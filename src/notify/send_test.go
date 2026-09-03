package notify

import (
	"bufio"
	"net"
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
	defer conn.Close()
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

func testConfigWithSMTP(host string, port int) *config.Config {
	cfg := &config.Config{}
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
	cfg := &config.Config{}
	cfg.Server.Notifications.Email.Enabled = false
	cfg.Server.Notifications.Email.SMTP.Host = "127.0.0.1"
	if CanSend(cfg) {
		t.Error("CanSend should be false when Enabled is false")
	}
}

func TestCanSendFalseWhenNoHost(t *testing.T) {
	cfg := &config.Config{}
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
	cfg := &config.Config{}
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
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for fake SMTP transcript")
	}
}

func TestEventEnabledDefaultsTrue(t *testing.T) {
	cfg := &config.Config{}
	if !eventEnabled(cfg, "backup_complete") {
		t.Error("eventEnabled should default to true when key absent")
	}
}

func TestEventEnabledExplicitFalse(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Notifications.Email.Events = map[string]bool{"backup_complete": false}
	if eventEnabled(cfg, "backup_complete") {
		t.Error("eventEnabled should honor explicit false")
	}
}

func TestBuildMessageOmitsReplyToWhenEmpty(t *testing.T) {
	msg := buildMessage("Airports", "no-reply@example.com", "to@example.com", "", "Subject line", "Body text")
	if strings.Contains(string(msg), "Reply-To:") {
		t.Error("Reply-To header should be omitted when replyTo is empty")
	}
}

func TestBuildMessageIncludesReplyToWhenSet(t *testing.T) {
	msg := buildMessage("Airports", "no-reply@example.com", "to@example.com", "reply@example.com", "Subject line", "Body text")
	if !strings.Contains(string(msg), "Reply-To: reply@example.com") {
		t.Error("Reply-To header should be present when replyTo is set")
	}
}
