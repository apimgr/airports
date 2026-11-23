package notify

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/apimgr/airports/src/config"
)

// ErrSuppressScheduler is wrapped into a scheduled task's returned error by
// callers that have already sent a more specific failure notification
// (backup_failed, ssl_renewal_failed) for the same execution, per AI.md
// PART 17 "Suppression": "One notification, not two." The generic
// scheduler_error wrapper checks errors.Is(err, ErrSuppressScheduler) and
// skips sending its own email when present.
var ErrSuppressScheduler = errors.New("notify: scheduler_error suppressed by a more specific event")

// CanSend reports whether email notifications may be attempted at all, per
// AI.md PART 17 "SMTP Check Before Sending": email is enabled AND a host is
// configured. This is the single gate — no email is ever attempted,
// queued, or logged as "would have sent" when it returns false.
func CanSend(cfg *config.ApplicationConfig) bool {
	return cfg.Server.Notifications.Email.Enabled && cfg.Server.Notifications.Email.SMTP.Host != ""
}

// eventEnabled reports whether templateName's per-event email switch is on,
// per AI.md PART 17 "Configuration" § events. An absent key defaults to
// true (matches the doc comment on config.NotifyEmailConfig.Events).
func eventEnabled(cfg *config.ApplicationConfig, templateName string) bool {
	if v, ok := cfg.Server.Notifications.Email.Events[templateName]; ok {
		return v
	}
	return true
}

// TestEmailSubject marks an operator-requested test message in the recipient's
// inbox while preserving the rendered subject text.
func TestEmailSubject(subject string) string {
	if strings.HasPrefix(subject, "[TEST]") {
		return subject
	}
	return "[TEST] " + subject
}

// Send renders and delivers templateName to recipient to, per AI.md PART 17.
// configDir is used to resolve a custom template override
// ({configDir}/template/email/{templateName}.txt); pass "" to always use the
// embedded default. vars are merged over GlobalVars(cfg) (caller-supplied
// values win on key collision) before rendering. Returns nil only once the
// message has been fully accepted by the SMTP server (through DATA/CRLF.CRLF
// and QUIT). Send is a no-op (returns nil, does nothing) when CanSend is
// false or the per-event switch for templateName is off — per the spec,
// this is never an error, just nothing to do.
func Send(cfg *config.ApplicationConfig, configDir, templateName, to string, vars map[string]string) error {
	return sendTemplate(cfg, configDir, templateName, to, vars, nil)
}

// SendWithReplyTo renders and sends a notification with a per-message Reply-To
// address, overriding the global notification address.
func SendWithReplyTo(cfg *config.ApplicationConfig, configDir, templateName, to, replyTo string, vars map[string]string) error {
	return sendTemplate(cfg, configDir, templateName, to, vars, &replyTo)
}

func sendTemplate(cfg *config.ApplicationConfig, configDir, templateName, to string, vars map[string]string, replyToOverride *string) error {
	if !CanSend(cfg) {
		return nil
	}
	if !eventEnabled(cfg, templateName) {
		return nil
	}
	if to == "" {
		return fmt.Errorf("notify: Send(%q): recipient address is empty", templateName)
	}

	subjectTpl, bodyTpl, err := LoadEmailTemplate(configDir, templateName)
	if err != nil {
		return fmt.Errorf("notify: Send(%q): %w", templateName, err)
	}

	merged := GlobalVars(cfg)
	for k, v := range vars {
		merged[k] = v
	}

	subjectTpl, bodyTpl = TranslateEmailTemplate(configDir, templateName, subjectTpl, bodyTpl, cfg.Server.I18n.DefaultLanguage, merged)
	subject, body := Render(subjectTpl, bodyTpl, merged)
	if templateName == "test" {
		subject = TestEmailSubject(subject)
	}

	fromName := cfg.Server.Notifications.Email.From.Name
	if fromName == "" {
		fromName = merged["app_name"]
	}
	fromEmail := cfg.Server.Notifications.Email.From.Email
	if fromEmail == "" {
		fromEmail = "no-reply@" + merged["fqdn"]
	}

	replyTo := cfg.Server.Notifications.Email.ReplyTo
	if replyToOverride != nil {
		replyTo = *replyToOverride
	}
	message, err := buildMessage(fromName, fromEmail, to, replyTo, subject, body, merged["fqdn"])
	if err != nil {
		return fmt.Errorf("notify: build %q message: %w", templateName, err)
	}

	smtpCfg := cfg.Server.Notifications.Email.SMTP
	return sendMessage(smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, smtpCfg.Password, smtpCfg.TLS, fromEmail, to, message)
}

// SendEncryptedSecurityReport delivers an encrypted report as a MIME attachment.
// The message body contains no report content; when the project PGP key is not
// configured, the body warns the operator that the attachment uses AES-GCM.
func SendEncryptedSecurityReport(cfg *config.ApplicationConfig, to, algorithm string, ciphertext []byte) error {
	if !CanSend(cfg) || !eventEnabled(cfg, "security_report_received") {
		return nil
	}
	if strings.TrimSpace(to) == "" {
		return errors.New("notify: security report recipient address is empty")
	}
	if len(ciphertext) == 0 {
		return errors.New("notify: encrypted security report is empty")
	}
	attachment := "security-report.pgp"
	body := "An encrypted security report is attached. Decrypt it with the configured project PGP private key."
	if algorithm == "aes-256-gcm" {
		attachment = "security-report.aes256"
		body = "Warning: no project PGP key is configured. The attached security report is encrypted with the server AES-256-GCM key."
	} else if algorithm != "pgp" {
		return errors.New("notify: unsupported security report encryption algorithm")
	}
	subject := "Encrypted security report"
	fromName := cfg.Server.Notifications.Email.From.Name
	if fromName == "" {
		fromName = cfg.Server.Branding.Title
	}
	fromEmail := cfg.Server.Notifications.Email.From.Email
	if fromEmail == "" {
		fqdn := cfg.Server.FQDN
		if fqdn == "" {
			fqdn = "localhost"
		}
		fromEmail = "no-reply@" + fqdn
	}
	message, err := buildEncryptedAttachmentMessage(fromName, fromEmail, to, cfg.Server.Notifications.Email.ReplyTo, subject, body, attachment, ciphertext, cfg.Server.FQDN)
	if err != nil {
		return err
	}
	smtpCfg := cfg.Server.Notifications.Email.SMTP
	return sendMessage(smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, smtpCfg.Password, smtpCfg.TLS, fromEmail, to, message)
}

func buildEncryptedAttachmentMessage(fromName, fromEmail, to, replyTo, subject, body, filename string, ciphertext []byte, fqdn string) ([]byte, error) {
	if strings.TrimSpace(body) == "" {
		return nil, errors.New("notify: email body is empty")
	}
	from, recipient, reply, subject, err := validateEmailHeaders(fromName, fromEmail, to, replyTo, subject)
	if err != nil {
		return nil, err
	}
	var message bytes.Buffer
	writer := multipart.NewWriter(&message)
	if err := writeMessageHeaders(&message, from, recipient, reply, subject, fqdn, fmt.Sprintf("multipart/mixed; boundary=%q", writer.Boundary())); err != nil {
		return nil, err
	}
	altBoundary := multipart.NewWriter(&bytes.Buffer{}).Boundary()
	altPart, err := writer.CreatePart(map[string][]string{"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%q", altBoundary)}})
	if err != nil {
		return nil, err
	}
	alternative := multipart.NewWriter(altPart)
	if err := alternative.SetBoundary(altBoundary); err != nil {
		return nil, err
	}
	if err := writeAlternativeParts(alternative, body); err != nil {
		return nil, err
	}
	if err := alternative.Close(); err != nil {
		return nil, err
	}
	header := map[string][]string{
		"Content-Type":              {"application/octet-stream; name=" + quoteMIME(filename)},
		"Content-Disposition":       {"attachment; filename=" + quoteMIME(filename)},
		"Content-Transfer-Encoding": {"base64"},
	}
	attachment, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)
	for len(encoded) > 76 {
		line := append([]byte(nil), encoded[:76]...)
		line = append(line, '\r', '\n')
		if _, err := attachment.Write(line); err != nil {
			return nil, err
		}
		encoded = encoded[76:]
	}
	if _, err := attachment.Write(encoded); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return message.Bytes(), nil
}

func quoteMIME(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", "", "\n", "").Replace(value) + `"`
}

// buildMessage assembles an RFC 5322 plain-text email. replyTo is omitted
// when empty, per AI.md PART 17 "Default Sender".
func buildMessage(fromName, fromEmail, to, replyTo, subject, body, fqdn string) ([]byte, error) {
	if strings.TrimSpace(body) == "" {
		return nil, errors.New("notify: email body is empty")
	}
	from, recipient, reply, subject, err := validateEmailHeaders(fromName, fromEmail, to, replyTo, subject)
	if err != nil {
		return nil, err
	}
	var message bytes.Buffer
	writer := multipart.NewWriter(&message)
	contentType := fmt.Sprintf("multipart/alternative; boundary=%q", writer.Boundary())
	if err := writeMessageHeaders(&message, from, recipient, reply, subject, fqdn, contentType); err != nil {
		return nil, err
	}
	if err := writeAlternativeParts(writer, body); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return message.Bytes(), nil
}

func validateEmailHeaders(fromName, fromEmail, to, replyTo, subject string) (mail.Address, mail.Address, *mail.Address, string, error) {
	for name, value := range map[string]string{"From name": fromName, "From email": fromEmail, "To": to, "Reply-To": replyTo, "Subject": subject} {
		if !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n") {
			return mail.Address{}, mail.Address{}, nil, "", fmt.Errorf("notify: invalid %s header", name)
		}
	}
	if strings.TrimSpace(subject) == "" {
		return mail.Address{}, mail.Address{}, nil, "", errors.New("notify: email subject is empty")
	}
	if len(subject) > 32*1024 || len(fromName) > 1024 {
		return mail.Address{}, mail.Address{}, nil, "", errors.New("notify: email header exceeds maximum length")
	}
	parseMailbox := func(name, value string) (mail.Address, error) {
		address, err := mail.ParseAddress(strings.TrimSpace(value))
		if err != nil || address.Name != "" || address.Address != strings.TrimSpace(value) {
			return mail.Address{}, fmt.Errorf("notify: invalid %s address", name)
		}
		return *address, nil
	}
	from, err := parseMailbox("From", fromEmail)
	if err != nil {
		return mail.Address{}, mail.Address{}, nil, "", err
	}
	recipient, err := parseMailbox("To", to)
	if err != nil {
		return mail.Address{}, mail.Address{}, nil, "", err
	}
	var reply *mail.Address
	if strings.TrimSpace(replyTo) != "" {
		parsed, err := parseMailbox("Reply-To", replyTo)
		if err != nil {
			return mail.Address{}, mail.Address{}, nil, "", err
		}
		reply = &parsed
	}
	return mail.Address{Name: fromName, Address: from.Address}, recipient, reply, subject, nil
}

func writeMessageHeaders(message *bytes.Buffer, from, recipient mail.Address, reply *mail.Address, subject, fqdn, contentType string) error {
	messageID, err := newMessageID(fqdn)
	if err != nil {
		return err
	}
	headers := [][2]string{
		{"From", from.String()},
		{"To", recipient.String()},
	}
	if reply != nil {
		headers = append(headers, [2]string{"Reply-To", reply.String()})
	}
	headers = append(headers,
		[2]string{"Subject", mime.QEncoding.Encode("UTF-8", subject)},
		[2]string{"Date", time.Now().Format(time.RFC1123Z)},
		[2]string{"Message-ID", messageID},
		[2]string{"MIME-Version", "1.0"},
		[2]string{"Auto-Submitted", "auto-generated"},
		[2]string{"Content-Type", contentType},
	)
	for _, header := range headers {
		if err := writeFoldedHeader(message, header[0], header[1]); err != nil {
			return err
		}
	}
	message.WriteString("\r\n")
	return nil
}

func writeFoldedHeader(message *bytes.Buffer, name, value string) error {
	line := name + ": " + value
	for len(line) > 78 {
		cut := strings.LastIndex(line[:78], " ")
		if cut <= len(name)+1 {
			if len(line) > 998 {
				return fmt.Errorf("notify: %s header exceeds 998 octets", name)
			}
			break
		}
		message.WriteString(line[:cut] + "\r\n")
		line = " " + strings.TrimLeft(line[cut+1:], " ")
	}
	if len(line) > 998 {
		return fmt.Errorf("notify: %s header exceeds 998 octets", name)
	}
	message.WriteString(line + "\r\n")
	return nil
}

func writeAlternativeParts(writer *multipart.Writer, body string) error {
	plainPart, err := writer.CreatePart(map[string][]string{
		"Content-Type":              {"text/plain; charset=UTF-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	plainWriter := quotedprintable.NewWriter(plainPart)
	if _, err := io.WriteString(plainWriter, body); err != nil {
		_ = plainWriter.Close()
		return err
	}
	if err := plainWriter.Close(); err != nil {
		return err
	}
	htmlPart, err := writer.CreatePart(map[string][]string{
		"Content-Type":              {"text/html; charset=UTF-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	htmlBody := "<html><body><p>" + strings.ReplaceAll(html.EscapeString(body), "\n", "<br>\n") + "</p></body></html>"
	htmlWriter := quotedprintable.NewWriter(htmlPart)
	if _, err := io.WriteString(htmlWriter, htmlBody); err != nil {
		_ = htmlWriter.Close()
		return err
	}
	return htmlWriter.Close()
}

func newMessageID(fqdn string) (string, error) {
	if strings.TrimSpace(fqdn) == "" || strings.ContainsAny(fqdn, "\r\n <>@") {
		fqdn = "localhost"
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("notify: generate Message-ID: %w", err)
	}
	return "<" + hex.EncodeToString(random[:]) + "@" + fqdn + ">", nil
}

// sendMessage delivers message via a fresh SMTP connection, applying the
// same TLS/AUTH rules as TestConnection (so a passing `email test` reflects
// exactly how real notifications will be sent).
func sendMessage(host string, port int, username, password, tlsMode, from, to string, message []byte) error {
	tlsMode = effectiveTLSMode(tlsMode, port)
	client, err := dialSMTP(host, port, tlsMode)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err := handshakeAndAuth(client, host, tlsMode, username, password); err != nil {
		return err
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		_ = w.Close()
		return fmt.Errorf("writing message body failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing message body failed: %w", err)
	}

	return client.Quit()
}
