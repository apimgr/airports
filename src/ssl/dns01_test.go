package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// Covers BuildDNSProvider's empty-provider-name error path, per PART 15
// "server.tls.dns_provider is not configured".
func TestBuildDNSProviderEmptyName(t *testing.T) {
	if _, err := BuildDNSProvider("", nil); err == nil {
		t.Error("BuildDNSProvider(\"\") expected error, got nil")
	}
}

// Covers BuildDNSProvider's unknown-provider error path — proves the lookup
// is dynamic (any bad name errors) rather than a hardcoded allow-list.
func TestBuildDNSProviderUnknownName(t *testing.T) {
	if _, err := BuildDNSProvider("not-a-real-lego-provider", nil); err == nil {
		t.Error("BuildDNSProvider(unknown) expected error, got nil")
	}
}

// Covers BuildDNSProvider succeeding for a known lego provider (cloudflare)
// when its required env-var credential is supplied, and proves the env var
// is cleared again afterward (never leaked into the process environment).
func TestBuildDNSProviderCloudflareSuccess(t *testing.T) {
	creds := map[string]string{"CLOUDFLARE_DNS_API_TOKEN": "fake-token-for-test"}
	provider, err := BuildDNSProvider("cloudflare", creds)
	if err != nil {
		t.Fatalf("BuildDNSProvider(cloudflare): unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("BuildDNSProvider(cloudflare) returned nil provider")
	}
	if v := os.Getenv("CLOUDFLARE_DNS_API_TOKEN"); v != "" {
		t.Errorf("CLOUDFLARE_DNS_API_TOKEN leaked into environment after BuildDNSProvider: %q", v)
	}
}

// Covers BuildDNSProvider's failure path for a known provider missing its
// required credentials.
func TestBuildDNSProviderCloudflareMissingCreds(t *testing.T) {
	t.Setenv("CLOUDFLARE_DNS_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_EMAIL", "")
	t.Setenv("CLOUDFLARE_API_KEY", "")
	if _, err := BuildDNSProvider("cloudflare", nil); err == nil {
		t.Error("BuildDNSProvider(cloudflare, no creds) expected error, got nil")
	}
}

// Covers ValidateDNSProviderCredentials as a thin pass/fail wrapper around
// BuildDNSProvider, per PART 15 "Server validates credentials on startup
// and before certificate requests".
func TestValidateDNSProviderCredentials(t *testing.T) {
	if err := ValidateDNSProviderCredentials("cloudflare", map[string]string{"CLOUDFLARE_DNS_API_TOKEN": "fake"}); err != nil {
		t.Errorf("ValidateDNSProviderCredentials(valid): unexpected error: %v", err)
	}
	if err := ValidateDNSProviderCredentials("cloudflare", nil); err == nil {
		t.Error("ValidateDNSProviderCredentials(missing creds): expected error, got nil")
	}
}

func TestRFC2136DynamicUpdates(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.LocalAddr().String()
	var mu sync.Mutex
	var inserted, removed []string
	server := &dns.Server{
		PacketConn: listener,
		TsigSecret: map[string]string{"acme.example.": secret},
		MsgAcceptFunc: func(header dns.Header) dns.MsgAcceptAction {
			if header.Bits>>11&0xf == dns.OpcodeUpdate {
				return dns.MsgAccept
			}
			return dns.DefaultMsgAcceptFunc(header)
		},
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			response := new(dns.Msg)
			response.SetReply(req)
			response.Authoritative = true
			if req.IsTsig() != nil {
				response.SetTsig("acme.example.", dns.HmacSHA256, 300, time.Now().Unix())
				if w.TsigStatus() != nil {
					response.IsTsig().Error = dns.RcodeBadSig
					response.Rcode = dns.RcodeNotAuth
					_ = w.WriteMsg(response)
					return
				}
			}
			if req.Opcode != dns.OpcodeUpdate || req.IsTsig() == nil {
				response.Rcode = dns.RcodeRefused
				_ = w.WriteMsg(response)
				return
			}
			mu.Lock()
			for _, rr := range req.Ns {
				txt, ok := rr.(*dns.TXT)
				if !ok || txt.Hdr.Name != "_acme-challenge.example.test." {
					continue
				}
				value := strings.Join(txt.Txt, "")
				switch txt.Hdr.Class {
				case dns.ClassINET:
					inserted = append(inserted, value)
				case dns.ClassNONE:
					removed = append(removed, value)
				}
			}
			mu.Unlock()
			_ = w.WriteMsg(response)
		}),
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = server.ShutdownContext(context.Background())
		select {
		case err := <-serverErrors:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("RFC2136 test DNS server stopped with error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("RFC2136 test DNS server did not stop")
		}
	})

	tests := []struct {
		name       string
		namespace  string
		key        string
		secret     string
		wantError  string
		wantSigned bool
	}{
		{name: "DNSUPDATE signed insert and remove", namespace: "DNSUPDATE", key: "acme.example.", secret: secret, wantSigned: true},
		{name: "RFC2136 alias signed insert and remove", namespace: "RFC2136", key: "acme.example.", secret: secret, wantSigned: true},
		{name: "unsigned update rejected", namespace: "DNSUPDATE"},
		{name: "wrong TSIG secret rejected", namespace: "DNSUPDATE", key: "acme.example.", secret: base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210")), wantError: "BADSIG"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creds := map[string]string{
				tc.namespace + "_NAMESERVER": addr,
				tc.namespace + "_ZONES":      "example.test",
				tc.namespace + "_TTL":        "60",
			}
			if tc.key != "" {
				creds[tc.namespace+"_TSIG_KEY"] = tc.key
				creds[tc.namespace+"_TSIG_SECRET"] = tc.secret
				creds[tc.namespace+"_TSIG_ALGORITHM"] = "hmac-sha256."
			}
			provider, err := BuildDNSProvider("rfc2136", creds)
			if err != nil {
				t.Fatalf("BuildDNSProvider: %v", err)
			}
			challengeProvider := provider
			err = challengeProvider.Present("example.test", "token", "key-authorization")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(strings.ToUpper(err.Error()), tc.wantError) {
					t.Fatalf("Present() error = %v, want %s", err, tc.wantError)
				}
				return
			}
			if !tc.wantSigned {
				if err == nil {
					t.Fatal("unsigned update unexpectedly succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("Present(): %v", err)
			}
			if err := challengeProvider.CleanUp("example.test", "token", "key-authorization"); err != nil {
				t.Fatalf("CleanUp(): %v", err)
			}
		})
	}

	mu.Lock()
	defer mu.Unlock()
	if len(inserted) != 2 || len(removed) != 2 {
		t.Fatalf("record updates inserted=%v removed=%v; want two signed inserts and removes", inserted, removed)
	}
	for index := range inserted {
		if inserted[index] == "" || inserted[index] != removed[index] {
			t.Errorf("TXT record %d insert/remove mismatch: %q != %q", index, inserted[index], removed[index])
		}
	}
}

func TestRFC2136PortlessNameserverDefaultsTo53(t *testing.T) {
	provider, err := BuildDNSProvider("rfc2136", map[string]string{"DNSUPDATE_NAMESERVER": "127.0.0.1", "DNSUPDATE_ZONES": "example.test"})
	if err != nil {
		t.Fatalf("BuildDNSProvider: %v", err)
	}
	config := reflect.ValueOf(provider).Elem().FieldByName("provider").Elem().Elem().FieldByName("config").Elem()
	if got := config.FieldByName("Nameserver").String(); got != net.JoinHostPort("127.0.0.1", "53") {
		t.Errorf("portless nameserver normalized to %q, want %s", got, net.JoinHostPort("127.0.0.1", "53"))
	}
}

// Covers loadOrCreateDNS01Account: generates a new account key on first
// call, then reloads the identical key from disk on a second call.
func TestLoadOrCreateDNS01Account(t *testing.T) {
	dir := testTempDir(t)
	accountDir := filepath.Join(dir, "dns01")

	first, err := loadOrCreateDNS01Account(accountDir, "ops@example.com")
	if err != nil {
		t.Fatalf("loadOrCreateDNS01Account (create): %v", err)
	}
	if first.GetEmail() != "ops@example.com" {
		t.Errorf("GetEmail() = %q, want ops@example.com", first.GetEmail())
	}
	if first.GetPrivateKey() == nil {
		t.Fatal("GetPrivateKey() returned nil")
	}
	if first.GetRegistration() != nil {
		t.Error("GetRegistration() should be nil before any Register() call")
	}

	keyPath := filepath.Join(accountDir, "account.key")
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected account key persisted at %s: %v", keyPath, err)
	}

	second, err := loadOrCreateDNS01Account(accountDir, "ops@example.com")
	if err != nil {
		t.Fatalf("loadOrCreateDNS01Account (reload): %v", err)
	}
	firstKeyBytes := marshalTestKey(t, first.GetPrivateKey())
	secondKeyBytes := marshalTestKey(t, second.GetPrivateKey())
	if string(firstKeyBytes) != string(secondKeyBytes) {
		t.Error("reloaded account key does not match the originally generated key")
	}
}

// Covers ObtainCertificateDNS01's no-domains error path (no network I/O).
func TestObtainCertificateDNS01NoDomains(t *testing.T) {
	dir := testTempDir(t)
	provider, err := BuildDNSProvider("cloudflare", map[string]string{"CLOUDFLARE_DNS_API_TOKEN": "fake"})
	if err != nil {
		t.Fatalf("BuildDNSProvider: %v", err)
	}
	if _, _, err := ObtainCertificateDNS01(dir, "ops@example.com", true, provider, nil); err == nil {
		t.Error("ObtainCertificateDNS01(no domains) expected error, got nil")
	}
}

// marshalTestKey re-encodes an ECDSA private key so two keys can be
// compared by value rather than by pointer identity.
func marshalTestKey(t *testing.T, key crypto.PrivateKey) []byte {
	t.Helper()
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("key is not *ecdsa.PrivateKey: %T", key)
	}
	der, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	return der
}
