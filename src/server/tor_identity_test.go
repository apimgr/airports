package server

import (
	"context"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/pires/go-proxyproto"
)

func TestTorConnContextReadsCircuitIDFromProxyHeader(t *testing.T) {
	client, backend := net.Pipe()
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client pipe: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close backend pipe: %v", err)
		}
	})
	proxyConn := proxyproto.NewConn(backend, proxyproto.WithPolicy(proxyproto.REQUIRE))
	go func() {
		_, _ = client.Write([]byte("PROXY TCP6 fc00::1234 fc00::1 443 80\r\n"))
	}()

	s := newTestServer(t)
	ctx := s.TorConnContext(context.Background(), proxyConn)
	r := httptest.NewRequest("GET", "http://service.onion/", nil).WithContext(ctx)
	if got := TorCircuitID(r); got != "fc00::1234" {
		t.Fatalf("TorCircuitID() = %q, want %q", got, "fc00::1234")
	}
	if got := OriginalTCPPeer(r); got != "tor:fc00::1234" {
		t.Fatalf("OriginalTCPPeer() = %q, want Tor circuit identity", got)
	}
}

func TestOnionHostAloneDoesNotEstablishTorIdentity(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Tor.OnionAddress = "service.onion"
	r := httptest.NewRequest("GET", "http://service.onion/", nil)
	if s.isTorRequest(r) {
		t.Fatal("isTorRequest() accepted a client-controlled Host without PROXY metadata")
	}
}
