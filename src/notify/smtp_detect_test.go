package notify

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// startFakeSMTPListener spins up a bare-bones SMTP-speaking TCP listener
// (EHLO/QUIT only) for exercising probeSMTP/Autodetect's success path
// without touching the real network, mirroring send_test.go's rationale
// for a hand-rolled fake server.
func startFakeSMTPListener(t *testing.T) (host string, port int) {
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
					switch {
					case strings.HasPrefix(strings.ToUpper(line), "EHLO"):
						_, _ = c.Write([]byte("250-fake.smtp greets you\r\n250 OK\r\n"))
					case strings.HasPrefix(strings.ToUpper(line), "QUIT"):
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

func TestProbeSMTPSucceedsAgainstFakeServer(t *testing.T) {
	host, port := startFakeSMTPListener(t)
	if !probeSMTP(host, port, probeTimeout) {
		t.Error("probeSMTP should succeed against a responsive fake SMTP listener")
	}
}

func TestProbeSMTPFailsWhenNothingListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()

	if probeSMTP("127.0.0.1", tcpAddr.Port, probeTimeout) {
		t.Error("probeSMTP should fail when nothing is listening on the port")
	}
}

func TestCandidateHostsIncludesFQDNVariants(t *testing.T) {
	hosts := candidateHosts("example.com")

	want := map[string]bool{
		"127.0.0.1":        false,
		"172.17.0.1":       false,
		"example.com":      false,
		"mail.example.com": false,
		"smtp.example.com": false,
	}
	for _, h := range hosts {
		if _, ok := want[h]; ok {
			want[h] = true
		}
	}
	for h, found := range want {
		if !found {
			t.Errorf("candidateHosts(%q) missing expected entry %q, got %v", "example.com", h, hosts)
		}
	}
}

func TestCandidateHostsOmitsFQDNVariantsWhenEmpty(t *testing.T) {
	hosts := candidateHosts("")
	for _, h := range hosts {
		if strings.HasPrefix(h, "mail.") || strings.HasPrefix(h, "smtp.") {
			t.Errorf("candidateHosts(\"\") should not synthesize mail./smtp. hosts from an empty fqdn, got %q", h)
		}
	}
}

func TestAutodetectFindsFakeServerOnFirstCandidate(t *testing.T) {
	origPorts := smtpPorts
	defer func() { smtpPorts = origPorts }()

	host, port := startFakeSMTPListener(t)
	if host != "127.0.0.1" {
		t.Fatalf("expected fake listener on 127.0.0.1, got %s", host)
	}
	// Autodetect always probes 127.0.0.1 first per the priority table; force
	// the fixed port list down to just the fake listener's ephemeral port so
	// the very first candidate/port combination is the one that succeeds.
	smtpPorts = []int{port}

	gotHost, gotPort, ok := Autodetect("")
	if !ok {
		t.Fatal("Autodetect should find the fake SMTP listener on 127.0.0.1")
	}
	if gotHost != "127.0.0.1" || gotPort != port {
		t.Errorf("Autodetect = (%q, %d), want (127.0.0.1, %d)", gotHost, gotPort, port)
	}
}

func TestAutodetectFailsWhenNoCandidateResponds(t *testing.T) {
	origPorts := smtpPorts
	defer func() { smtpPorts = origPorts }()

	// A closed, guaranteed-unused local port: nothing will answer.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()
	smtpPorts = []int{tcpAddr.Port}

	if _, _, ok := Autodetect(""); ok {
		t.Error("Autodetect should return ok=false when no candidate host responds")
	}
}

func TestEhloNameNeverEmpty(t *testing.T) {
	if ehloName() == "" {
		t.Error("ehloName should never return an empty string")
	}
}

func TestHexRouteToIP(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"loopback little-endian", "0100007F", "127.0.0.1"},
		{"default route zero", "00000000", "0.0.0.0"},
		{"invalid length", "ABC", ""},
		{"invalid hex", "ZZZZZZZZ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hexRouteToIP(tt.hex); got != tt.want {
				t.Errorf("hexRouteToIP(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}

func TestDefaultGatewayIPNonLinuxOrBestEffort(t *testing.T) {
	// Best-effort: on Linux this reads /proc/net/route (may or may not
	// resolve depending on the container's network), on other platforms it
	// always returns "". Either way it must never panic and must return a
	// syntactically valid result (empty or a parseable IP).
	got := defaultGatewayIP()
	if got != "" && net.ParseIP(got) == nil {
		t.Errorf("defaultGatewayIP() = %q, not empty and not a valid IP", got)
	}
}

func TestGlobalIPv4BestEffort(t *testing.T) {
	// Best-effort: either "" (no route / private) or a syntactically valid
	// non-private IPv4 address. Must never panic.
	got := globalIPv4()
	if got == "" {
		return
	}
	ip := net.ParseIP(got)
	if ip == nil || ip.To4() == nil {
		t.Errorf("globalIPv4() = %q, not a valid IPv4 address", got)
	}
}
