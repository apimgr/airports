package notify

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// startAuthCapableFakeSMTPServer is like startFakeSMTPServer in send_test.go
// but also advertises AUTH PLAIN and STARTTLS in its EHLO response, so
// TestConnection/handshakeAndAuth's AUTH and STARTTLS branches can be
// exercised without a real network.
func startAuthCapableFakeSMTPServer(t *testing.T, advertiseAuth, advertiseStartTLS bool) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("220 fake.smtp ESMTP ready\r\n"))
				reader := bufio.NewReader(c)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					upper := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(upper, "EHLO"):
						var b strings.Builder
						b.WriteString("250-fake.smtp greets you\r\n")
						if advertiseStartTLS {
							b.WriteString("250-STARTTLS\r\n")
						}
						if advertiseAuth {
							b.WriteString("250-AUTH PLAIN\r\n")
						}
						b.WriteString("250 OK\r\n")
						_, _ = c.Write([]byte(b.String()))
					case strings.HasPrefix(upper, "AUTH"):
						_, _ = c.Write([]byte("235 Authentication succeeded\r\n"))
					case upper == "QUIT":
						_, _ = c.Write([]byte("221 Bye\r\n"))
						return
					default:
						_, _ = c.Write([]byte("500 unrecognized command\r\n"))
					}
				}
			}(conn)
		}
	}()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", tcpAddr.Port
}

func TestTestConnectionSucceedsPlain(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	if err := TestConnection(host, port, "", "", "none"); err != nil {
		t.Errorf("TestConnection should succeed against a plain EHLO/QUIT server, got: %v", err)
	}
}

func TestTestConnectionSucceedsWithAuth(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, true, false)
	if err := TestConnection(host, port, "user", "pass", "none"); err != nil {
		t.Errorf("TestConnection should succeed when AUTH is advertised and credentials are supplied, got: %v", err)
	}
}

func TestTestConnectionFailsWhenAuthRequiredButNotAdvertised(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	if err := TestConnection(host, port, "user", "pass", "none"); err == nil {
		t.Error("TestConnection should fail when a username is configured but the server does not advertise AUTH")
	}
}

func TestTestConnectionFailsWhenStartTLSRequiredButNotAdvertised(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	if err := TestConnection(host, port, "", "", "starttls"); err == nil {
		t.Error("TestConnection should fail when tlsMode=starttls but the server does not advertise STARTTLS")
	}
}

func TestTestConnectionAutoModeSkipsStartTLSWhenUnadvertised(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	if err := TestConnection(host, port, "", "", "auto"); err != nil {
		t.Errorf("TestConnection with tlsMode=auto should tolerate a server with no STARTTLS support, got: %v", err)
	}
}

func TestTestConnectionFailsWhenNothingListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()

	if err := TestConnection("127.0.0.1", tcpAddr.Port, "", "", "none"); err == nil {
		t.Error("TestConnection should fail to dial a closed port")
	}
}

func TestDialSMTPPlainConnectsSuccessfully(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	client, err := dialSMTP(host, port, "none")
	if err != nil {
		t.Fatalf("dialSMTP: %v", err)
	}
	defer client.Close()
}

func TestDialSMTPTLSModeFailsAgainstPlainServer(t *testing.T) {
	host, port := startAuthCapableFakeSMTPServer(t, false, false)
	// The fake server speaks plaintext only, so an implicit-TLS dial
	// ("tls" mode performs the TLS handshake immediately) must fail rather
	// than silently falling back to plaintext.
	if _, err := dialSMTP(host, port, "tls"); err == nil {
		t.Error("dialSMTP with tlsMode=tls should fail against a plaintext-only listener")
	}
}

func TestDialSMTPFailsWhenUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()

	if _, err := dialSMTP("127.0.0.1", tcpAddr.Port, "none"); err == nil {
		t.Error("dialSMTP should fail to connect to a closed port")
	}
}
