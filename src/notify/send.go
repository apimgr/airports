package notify

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
func CanSend(cfg *config.Config) bool {
	return cfg.Server.Notifications.Email.Enabled && cfg.Server.Notifications.Email.SMTP.Host != ""
}

// eventEnabled reports whether templateName's per-event email switch is on,
// per AI.md PART 17 "Configuration" § events. An absent key defaults to
// true (matches the doc comment on config.NotifyEmailConfig.Events).
func eventEnabled(cfg *config.Config, templateName string) bool {
	if v, ok := cfg.Server.Notifications.Email.Events[templateName]; ok {
		return v
	}
	return true
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
func Send(cfg *config.Config, configDir, templateName, to string, vars map[string]string) error {
	if !CanSend(cfg) {
		return nil
	}
	if !eventEnabled(cfg, templateName) {
		return nil
	}
	if to == "" {
		return fmt.Errorf("notify: Send(%q): recipient address is empty", templateName)
	}

	subjectTpl, bodyTpl, err := LoadTemplate(configDir, templateName)
	if err != nil {
		return fmt.Errorf("notify: Send(%q): %w", templateName, err)
	}

	merged := GlobalVars(cfg)
	for k, v := range vars {
		merged[k] = v
	}

	subject, body := Render(subjectTpl, bodyTpl, merged)

	fromName := cfg.Server.Notifications.Email.From.Name
	if fromName == "" {
		fromName = merged["app_name"]
	}
	fromEmail := cfg.Server.Notifications.Email.From.Email
	if fromEmail == "" {
		fromEmail = "no-reply@" + merged["fqdn"]
	}

	message := buildMessage(fromName, fromEmail, to, cfg.Server.Notifications.Email.ReplyTo, subject, body)

	smtpCfg := cfg.Server.Notifications.Email.SMTP
	return sendMessage(smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, smtpCfg.Password, smtpCfg.TLS, fromEmail, to, message)
}

// buildMessage assembles an RFC 5322 plain-text email. replyTo is omitted
// when empty, per AI.md PART 17 "Default Sender".
func buildMessage(fromName, fromEmail, to, replyTo, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, fromEmail)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	if replyTo != "" {
		fmt.Fprintf(&b, "Reply-To: %s\r\n", replyTo)
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}

// sendMessage delivers message via a fresh SMTP connection, applying the
// same TLS/AUTH rules as TestConnection (so a passing `email test` reflects
// exactly how real notifications will be sent).
func sendMessage(host string, port int, username, password, tlsMode, from, to string, message []byte) error {
	client, err := dialSMTP(host, port, tlsMode)
	if err != nil {
		return err
	}
	defer client.Close()

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
