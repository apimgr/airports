package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"
)

// connectTimeout bounds TestConnection's TCP dial and handshake, matching
// AI.md PART 17 "Connection Test (when host is set)" — this runs once per
// startup and must not hang the boot sequence.
const connectTimeout = 5 * time.Second

// TestConnection performs a real connect + EHLO(+AUTH if username is set)
// handshake against host:port per AI.md PART 17 "Connection Test (when
// host is set)". tlsMode is one of "auto", "starttls", "tls", "none" per
// the server.notifications.email.smtp.tls config field. Returns nil only
// on a fully successful handshake (including AUTH, when credentials are
// configured).
func TestConnection(host string, port int, username, password, tlsMode string) error {
	client, err := dialSMTP(host, port, tlsMode)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := handshakeAndAuth(client, host, tlsMode, username, password); err != nil {
		return err
	}

	return client.Quit()
}

// handshakeAndAuth runs EHLO, optional STARTTLS upgrade, and optional AUTH
// against an already-dialed client. Shared by TestConnection and the
// message-sending path in send.go so both apply identical TLS/AUTH rules.
func handshakeAndAuth(client *smtp.Client, host, tlsMode, username, password string) error {
	if err := client.Hello(ehloName()); err != nil {
		return fmt.Errorf("EHLO failed: %w", err)
	}

	if tlsMode == "starttls" || tlsMode == "auto" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("STARTTLS failed: %w", err)
			}
		} else if tlsMode == "starttls" {
			return fmt.Errorf("server does not support STARTTLS")
		}
	}

	if username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", username, password, host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP authentication failed: %w", err)
			}
		} else {
			return fmt.Errorf("server does not support AUTH but username is configured")
		}
	}

	return nil
}

// dialSMTP connects to host:port per tlsMode: "tls" dials directly over
// TLS (implicit TLS, typically port 465); all other modes ("auto",
// "starttls", "none") dial plaintext and let TestConnection/sendMail
// upgrade via STARTTLS afterward when applicable.
func dialSMTP(host string, port int, tlsMode string) (*smtp.Client, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if tlsMode == "tls" {
		dialer := &net.Dialer{Timeout: connectTimeout}
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, fmt.Errorf("TLS connect to %s failed: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return nil, fmt.Errorf("SMTP handshake with %s failed: %w", addr, err)
		}
		return client, nil
	}

	conn, err := net.DialTimeout("tcp", addr, connectTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect to %s failed: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(connectTimeout))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, fmt.Errorf("SMTP handshake with %s failed: %w", addr, err)
	}
	return client, nil
}
