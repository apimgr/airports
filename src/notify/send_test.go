package notify

import (
	"bufio"
	"bytes"
	"mime"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

// fakeSMTPServer is a minimal SMTP server sufficient to satisfy net/smtp's
// client expectations (EHLO/MAIL/RCPT/DATA/QUIT), used because this project
// has no established network-mocking pattern for outbound protocols yet
// (checked src/geoip and src/ssl test files; neither defines one). It runs
// on a local net.Listen("tcp", "127.0.0.1:0") socket, never the real
// network, and records the transcript for assertions.
type fakeSMTPServer struct {
	listener net.Listener
	received chan string
}

func startFakeSMTPServer(t *testing.T) (host string, port int, srv *fakeSMTPServer) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	srv = &fakeSMTPServer{listener: ln, received: make(chan string, 1)}
	go srv.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })

	tcpAddr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", tcpAddr.Port, srv
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	var transcript strings.Builder

	writeLine := func(line string) {
		_, _ = conn.Write([]byte(line + "\r\n"))
	}

	writeLine("220 fake.smtp ESMTP ready")

	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			transcript.WriteString(trimmed)
			transcript.WriteString("\n")
			if trimmed == "." {
				inData = false
				writeLine("250 OK: message accepted")
			}
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			writeLine("250-fake.smtp greets you")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "MAIL FROM"):
			transcript.WriteString(trimmed + "\n")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			transcript.WriteString(trimmed + "\n")
			writeLine("250 OK")
		case upper == "DATA":
			inData = true
			writeLine("354 Start mail input; end with <CRLF>.<CRLF>")
		case upper == "QUIT":
			writeLine("221 Bye")
			s.received <- transcript.String()
			return
		default:
			writeLine("500 unrecognized command")
		}
	}
	select {
	case s.received <- transcript.String():
	default:
	}
}

func testConfigWithSMTP(host string, port int) *config.ApplicationConfig {
	cfg := &config.ApplicationConfig{}
	cfg.Server.FQDN = "example.com"
	cfg.Server.Notifications.Email.Enabled = true
	cfg.Server.Notifications.Email.SMTP.Host = host
	cfg.Server.Notifications.Email.SMTP.Port = port
	cfg.Server.Notifications.Email.SMTP.TLS = "none"
	cfg.Server.Notifications.Email.From.Name = "Airports"
	cfg.Server.Notifications.Email.From.Email = "no-reply@example.com"
	return cfg
}

func TestCanSendFalseWhenDisabled(t *testing.T) {
	cfg := &config.ApplicationConfig{}
	cfg.Server.Notifications.Email.Enabled = false
	cfg.Server.Notifications.Email.SMTP.Host = "127.0.0.1"
	if CanSend(cfg) {
		t.Error("CanSend should be false when Enabled is false")
	}
}

func TestCanSendFalseWhenNoHost(t *testing.T) {
	cfg := &config.ApplicationConfig{}
	cfg.Server.Notifications.Email.Enabled = true
	cfg.Server.Notifications.Email.SMTP.Host = ""
	if CanSend(cfg) {
		t.Error("CanSend should be false when SMTP host is empty")
	}
}

func TestCanSendTrue(t *testing.T) {
	cfg := testConfigWithSMTP("127.0.0.1", 2525)
	if !CanSend(cfg) {
		t.Error("CanSend should be true when enabled and host set")
	}
}

func TestSendNoOpWhenCanSendFalse(t *testing.T) {
	cfg := &config.ApplicationConfig{}
	if err := Send(cfg, "", "test", "someone@example.com", nil); err != nil {
		t.Errorf("Send should be a silent no-op, got error: %v", err)
	}
}

func TestSendNoOpWhenEventDisabled(t *testing.T) {
	_, port, srv := startFakeSMTPServer(t)
	_ = srv
	cfg := testConfigWithSMTP("127.0.0.1", port)
	cfg.Server.Notifications.Email.Events = map[string]bool{"test": false}

	if err := Send(cfg, "", "test", "someone@example.com", nil); err != nil {
		t.Errorf("Send should no-op when event disabled, got error: %v", err)
	}
}

func TestSendRejectsEmptyRecipient(t *testing.T) {
	_, port, _ := startFakeSMTPServer(t)
	cfg := testConfigWithSMTP("127.0.0.1", port)

	if err := Send(cfg, "", "test", "", nil); err == nil {
		t.Error("expected error for empty recipient")
	}
}

func TestSendDeliversToFakeServer(t *testing.T) {
	host, port, srv := startFakeSMTPServer(t)
	cfg := testConfigWithSMTP(host, port)

	err := Send(cfg, "", "test", "recipient@example.com", map[string]string{"app_name": "Airports"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case transcript := <-srv.received:
		if !strings.Contains(transcript, "MAIL FROM") {
			t.Errorf("transcript missing MAIL FROM: %q", transcript)
		}
		if !strings.Contains(transcript, "recipient@example.com") {
			t.Errorf("transcript missing recipient: %q", transcript)
		}
		if !strings.Contains(transcript, "Subject: [TEST] Test Email - Airports") {
			t.Errorf("transcript missing [TEST] subject prefix: %q", transcript)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for fake SMTP transcript")
	}
}

func TestSendWithReplyToKeepsContactEmailOutOfMessageBody(t *testing.T) {
	host, port, srv := startFakeSMTPServer(t)
	cfg := testConfigWithSMTP(host, port)
	senderEmail := "submitter@example.net"
	err := SendWithReplyTo(cfg, "", "contact_submission", "abuse@example.com", senderEmail, map[string]string{
		"sender_name":     "Jane Doe",
		"contact_subject": "Airport data correction",
		"contact_message": "Please review this airport record.",
	})
	if err != nil {
		t.Fatalf("SendWithReplyTo: %v", err)
	}

	select {
	case transcript := <-srv.received:
		if !strings.Contains(transcript, `From: "Airports" <no-reply@example.com>`) {
			t.Errorf("message did not use the configured sender: %q", transcript)
		}
		if !strings.Contains(transcript, "Reply-To: <"+senderEmail+">") {
			t.Errorf("message did not set submitter as Reply-To: %q", transcript)
		}
		separator := strings.Index(transcript, "\n\n")
		if separator < 0 {
			t.Fatalf("message has no header/body separator: %q", transcript)
		}
		if strings.Contains(transcript[separator+2:], senderEmail) {
			t.Errorf("submitter email appeared in the message body: %q", transcript)
		}
		if !strings.Contains(transcript, "Subject: [airports contact] Airport data correction") {
			t.Errorf("message subject is missing the required contact prefix: %q", transcript)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for fake SMTP transcript")
	}
}

func TestTestEmailSubject(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"Test message", "[TEST] Test message"},
		{"[TEST] Test message", "[TEST] Test message"},
	} {
		if got := TestEmailSubject(test.input); got != test.want {
			t.Errorf("TestEmailSubject(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestEventEnabledDefaultsTrue(t *testing.T) {
	cfg := &config.ApplicationConfig{}
	if !eventEnabled(cfg, "backup_complete") {
		t.Error("eventEnabled should default to true when key absent")
	}
}

func TestEventEnabledExplicitFalse(t *testing.T) {
	cfg := &config.ApplicationConfig{}
	cfg.Server.Notifications.Email.Events = map[string]bool{"backup_complete": false}
	if eventEnabled(cfg, "backup_complete") {
		t.Error("eventEnabled should honor explicit false")
	}
}

func TestBuildMessageOmitsReplyToWhenEmpty(t *testing.T) {
	msg, err := buildMessage("Airports", "no-reply@example.com", "to@example.com", "", "Subject line", "Body text", "example.com")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	if strings.Contains(string(msg), "Reply-To:") {
		t.Error("Reply-To header should be omitted when replyTo is empty")
	}
}

func TestBuildMessageIncludesReplyToWhenSet(t *testing.T) {
	msg, err := buildMessage("Airports", "no-reply@example.com", "to@example.com", "reply@example.com", "Subject line", "Body text", "example.com")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	if !strings.Contains(string(msg), "Reply-To: <reply@example.com>") {
		t.Error("Reply-To header should be present when replyTo is set")
	}
}

func TestBuildMessageIncludesRequiredHeadersAndAlternatives(t *testing.T) {
	message, err := buildMessage("Airports", "no-reply@example.com", "to@example.com", "", "Airport update", "A useful update", "airports.example.com")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	value := string(message)
	for _, header := range []string{"Date:", "Message-ID: <", "MIME-Version: 1.0", "Auto-Submitted: auto-generated", "Content-Type: multipart/alternative;", "boundary=\""} {
		if !strings.Contains(value, header) {
			t.Errorf("message is missing %q: %s", header, value)
		}
	}
	for _, part := range []string{"Content-Type: text/plain; charset=UTF-8", "Content-Type: text/html; charset=UTF-8", "A useful update", "<p>A useful update</p>"} {
		if !strings.Contains(value, part) {
			t.Errorf("message is missing part content %q", part)
		}
	}
}

func TestBuildMessageRFC2047HeadersRoundTrip(t *testing.T) {
	message, err := buildMessage("空港", "no-reply@example.com", "to@example.com", "", "更新通知", "A useful update", "airports.example.com")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(message))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	decoder := new(mime.WordDecoder)
	subject, err := decoder.DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if subject != "更新通知" {
		t.Errorf("decoded subject = %q, want 更新通知", subject)
	}
	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse From: %v", err)
	}
	if from.Name != "空港" {
		t.Errorf("decoded From name = %q, want 空港", from.Name)
	}
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	for _, test := range []struct {
		name     string
		fromName string
		to       string
		subject  string
	}{
		{name: "name", fromName: "Airports\r\nBcc: victim@example.com", to: "to@example.com", subject: "Update"},
		{name: "recipient", fromName: "Airports", to: "to@example.com\nBcc: victim@example.com", subject: "Update"},
		{name: "subject", fromName: "Airports", to: "to@example.com", subject: "Update\r\nBcc: victim@example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildMessage(test.fromName, "no-reply@example.com", test.to, "", test.subject, "Body", "example.com"); err == nil {
				t.Fatal("buildMessage accepted header injection")
			}
		})
	}
}
