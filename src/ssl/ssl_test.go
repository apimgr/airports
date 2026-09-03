package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testTempDir creates an isolated temp dir under /tmp/apimgr/airports-XXXXXX
// per project convention, and returns it plus automatic cleanup.
func testTempDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll base: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// writeTestCert generates a self-signed cert/key pair for the given domain
// and writes them to certPath/keyPath so findManualCerts/GetTLSConfig can
// load real, parseable PEM data without any network/ACME dependency.
func writeTestCert(t *testing.T, certPath, keyPath, domain string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{domain},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatalf("pem.Encode cert: %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("pem.Encode key: %v", err)
	}
}

// Covers NewManager: returns a non-nil Manager carrying the given config.
func TestNewManager(t *testing.T) {
	cfg := Config{Enabled: true, CertPath: "/some/path"}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config.Enabled != true || m.config.CertPath != "/some/path" {
		t.Errorf("NewManager stored config = %+v, want %+v", m.config, cfg)
	}
}

// Covers GetTLSConfig's disabled short-circuit: returns nil, nil without
// touching the filesystem.
func TestGetTLSConfigDisabled(t *testing.T) {
	m := NewManager(Config{Enabled: false})
	cfg, err := m.GetTLSConfig([]string{"example.com"})
	if err != nil {
		t.Fatalf("GetTLSConfig: unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("GetTLSConfig(disabled) = %+v, want nil", cfg)
	}
}

// Covers GetTLSConfig's manual-cert branch using the local/{fqdn}/cert.pem
// + key.pem layout (AI.md PART 15 "Certificate Directory Structure",
// priority 4: self-signed/user-provided, no auto-renew).
func TestGetTLSConfigManualCertsLocalForm(t *testing.T) {
	dir := testTempDir(t)
	domain := "manual.example.com"
	domainDir := filepath.Join(dir, "local", domain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestCert(t, filepath.Join(domainDir, "cert.pem"), filepath.Join(domainDir, "key.pem"), domain)

	m := NewManager(Config{Enabled: true, CertPath: dir})
	tlsCfg, err := m.GetTLSConfig([]string{domain})
	if err != nil {
		t.Fatalf("GetTLSConfig: unexpected error: %v", err)
	}
	if tlsCfg == nil || len(tlsCfg.Certificates) != 1 {
		t.Fatalf("GetTLSConfig = %+v, want a config with one certificate", tlsCfg)
	}
	if tlsCfg.MinVersion != 0x0303 {
		t.Errorf("MinVersion = %x, want TLS 1.2 (0x0303)", tlsCfg.MinVersion)
	}
}

// Covers GetTLSConfig's manual-cert branch using the app-managed
// letsencrypt/{fqdn}/fullchain.pem + privkey.pem layout (priority 3).
func TestGetTLSConfigManualCertsLetsEncryptForm(t *testing.T) {
	dir := testTempDir(t)
	domain := "fullchain.example.com"
	domainDir := filepath.Join(dir, "letsencrypt", domain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestCert(t, filepath.Join(domainDir, "fullchain.pem"), filepath.Join(domainDir, "privkey.pem"), domain)

	m := NewManager(Config{Enabled: true, CertPath: dir})
	tlsCfg, err := m.GetTLSConfig([]string{domain})
	if err != nil {
		t.Fatalf("GetTLSConfig: unexpected error: %v", err)
	}
	if tlsCfg == nil || len(tlsCfg.Certificates) != 1 {
		t.Fatalf("GetTLSConfig = %+v, want a config with one certificate", tlsCfg)
	}
}

// Covers GetTLSConfig's error path: enabled, no existing/manual certs found,
// and Let's Encrypt not enabled.
func TestGetTLSConfigNoCertsAvailable(t *testing.T) {
	dir := testTempDir(t)
	m := NewManager(Config{Enabled: true, CertPath: dir})
	_, err := m.GetTLSConfig([]string{"nocert.example.com"})
	if err == nil {
		t.Fatal("GetTLSConfig: expected error when no certs available and LE disabled")
	}
}

// Covers GetTLSConfig's Let's Encrypt branch: verifies it returns a non-nil
// TLS config built from autocert without making any network calls (autocert
// only performs network I/O lazily, during actual handshakes).
func TestGetTLSConfigLetsEncrypt(t *testing.T) {
	dir := testTempDir(t)
	m := NewManager(Config{
		Enabled:     true,
		CertPath:    dir,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Email: "ops@example.com"},
	})
	tlsCfg, err := m.GetTLSConfig([]string{"le.example.com"})
	if err != nil {
		t.Fatalf("GetTLSConfig: unexpected error: %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("GetTLSConfig: expected non-nil autocert TLS config")
	}
	cacheDir := filepath.Join(dir, "letsencrypt", "autocert")
	if info, err := os.Stat(cacheDir); err != nil || !info.IsDir() {
		t.Errorf("expected autocert cache dir at %s to be created", cacheDir)
	}
}

// Covers findExistingCerts: with no real /etc/letsencrypt/live entries for a
// synthetic domain, it must return empty strings rather than erroring.
func TestFindExistingCertsNotPresent(t *testing.T) {
	m := NewManager(Config{})
	cert, key := m.findExistingCerts([]string{"definitely-not-a-real-domain.invalid"})
	if cert != "" || key != "" {
		t.Errorf("findExistingCerts = (%q, %q), want empty strings", cert, key)
	}
}

// Covers findManualCerts: empty CertPath short-circuits to empty results,
// and a directory with no matching cert files also returns empty.
func TestFindManualCertsMissing(t *testing.T) {
	t.Run("empty-cert-path", func(t *testing.T) {
		m := NewManager(Config{})
		cert, key := m.findManualCerts([]string{"example.com"})
		if cert != "" || key != "" {
			t.Errorf("findManualCerts = (%q, %q), want empty strings", cert, key)
		}
	})

	t.Run("cert-path-set-no-matching-files", func(t *testing.T) {
		dir := testTempDir(t)
		m := NewManager(Config{CertPath: dir})
		cert, key := m.findManualCerts([]string{"nomatch.example.com"})
		if cert != "" || key != "" {
			t.Errorf("findManualCerts = (%q, %q), want empty strings", cert, key)
		}
	})

	t.Run("multiple-domains-second-matches", func(t *testing.T) {
		dir := testTempDir(t)
		domain := "second.example.com"
		domainDir := filepath.Join(dir, "local", domain)
		if err := os.MkdirAll(domainDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeTestCert(t, filepath.Join(domainDir, "cert.pem"), filepath.Join(domainDir, "key.pem"), domain)
		m := NewManager(Config{CertPath: dir})
		cert, key := m.findManualCerts([]string{"first.example.com", domain})
		if cert == "" || key == "" {
			t.Error("findManualCerts: expected match on second domain")
		}
	})

	t.Run("letsencrypt-priority-over-local", func(t *testing.T) {
		dir := testTempDir(t)
		domain := "priority.example.com"
		leDir := filepath.Join(dir, "letsencrypt", domain)
		localDir := filepath.Join(dir, "local", domain)
		if err := os.MkdirAll(leDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.MkdirAll(localDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeTestCert(t, filepath.Join(leDir, "fullchain.pem"), filepath.Join(leDir, "privkey.pem"), domain)
		writeTestCert(t, filepath.Join(localDir, "cert.pem"), filepath.Join(localDir, "key.pem"), domain)

		m := NewManager(Config{CertPath: dir})
		cert, _ := m.findManualCerts([]string{domain})
		if cert != filepath.Join(leDir, "fullchain.pem") {
			t.Errorf("findManualCerts = %q, want app-managed letsencrypt cert %q", cert, filepath.Join(leDir, "fullchain.pem"))
		}
	})
}

// Covers GetHTTPHandler: falls back to the provided handler when no
// autocert manager has been initialized (never called GetTLSConfig with LE
// enabled).
func TestGetHTTPHandlerFallback(t *testing.T) {
	m := NewManager(Config{})
	called := false
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := m.GetHTTPHandler(fallback)

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("GetHTTPHandler(fallback) did not invoke the fallback handler")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// Covers GetHTTPHandler when an autocert manager has been initialized: it
// must wrap requests through certManager.HTTPHandler rather than calling the
// fallback directly for ACME challenge paths.
func TestGetHTTPHandlerWithCertManager(t *testing.T) {
	dir := testTempDir(t)
	m := NewManager(Config{
		Enabled:     true,
		CertPath:    dir,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Email: "ops@example.com"},
	})
	if _, err := m.GetTLSConfig([]string{"le.example.com"}); err != nil {
		t.Fatalf("GetTLSConfig: %v", err)
	}

	fallbackCalled := false
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handler := m.GetHTTPHandler(fallback)

	req := httptest.NewRequest(http.MethodGet, "/normal-path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !fallbackCalled {
		t.Error("expected fallback to be invoked for a non-ACME-challenge path")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// Covers ChallengeServer's full lifecycle: unmatched path (not handled),
// unknown token (404, but handled=true), and a known token (200 with the
// auth body served as text/plain), plus token clearing.
func TestChallengeServer(t *testing.T) {
	cs := NewChallengeServer()

	t.Run("non-acme-path-not-handled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/normal-path", nil)
		rec := httptest.NewRecorder()
		handled := cs.ServeHTTP(rec, req)
		if handled {
			t.Error("ServeHTTP handled a non-ACME-challenge path")
		}
	})

	t.Run("unknown-token-404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/unknown-token", nil)
		rec := httptest.NewRecorder()
		handled := cs.ServeHTTP(rec, req)
		if !handled {
			t.Error("ServeHTTP did not handle a well-known ACME challenge path")
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("known-token-served", func(t *testing.T) {
		cs.SetToken("mytoken", "mytoken.authkey")
		req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/mytoken", nil)
		rec := httptest.NewRecorder()
		handled := cs.ServeHTTP(rec, req)
		if !handled {
			t.Error("ServeHTTP did not handle a known token path")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != "mytoken.authkey" {
			t.Errorf("body = %q, want mytoken.authkey", rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", ct)
		}
	})

	t.Run("cleared-token-404s-again", func(t *testing.T) {
		cs.SetToken("cleartoken", "cleartoken.authkey")
		cs.ClearToken("cleartoken")
		req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/cleartoken", nil)
		rec := httptest.NewRecorder()
		cs.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status after ClearToken = %d, want 404", rec.Code)
		}
	})
}

// Covers ParseChallenge: every recognized alias for each of the three
// challenge types, plus the default-to-http-01 fallback for unknown input,
// case-insensitivity, and whitespace trimming.
func TestParseChallenge(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http-01", "http-01"},
		{"http01", "http-01"},
		{"http", "http-01"},
		{"HTTP-01", "http-01"},
		{"  http  ", "http-01"},
		{"tls-alpn-01", "tls-alpn-01"},
		{"tlsalpn01", "tls-alpn-01"},
		{"tls-alpn", "tls-alpn-01"},
		{"tls", "tls-alpn-01"},
		{"TLS", "tls-alpn-01"},
		{"dns-01", "dns-01"},
		{"dns01", "dns-01"},
		{"dns", "dns-01"},
		{"DNS", "dns-01"},
		{"", "http-01"},
		{"unknown-value", "http-01"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := ParseChallenge(tt.in); got != tt.want {
				t.Errorf("ParseChallenge(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Covers RenewIfNeeded's disabled short-circuit: no certManager/dns01
// state, must return nil without touching either renewal path.
func TestRenewIfNeededDisabled(t *testing.T) {
	m := NewManager(Config{Enabled: false})
	renewed, err := m.RenewIfNeeded([]string{"example.com"})
	if err != nil {
		t.Errorf("RenewIfNeeded(disabled) = %v, want nil", err)
	}
	if renewed {
		t.Error("RenewIfNeeded(disabled) renewed = true, want false")
	}
}

// Covers RenewIfNeeded's autocert branch when no certManager has been
// initialized yet (GetTLSConfig never called): must return nil.
func TestRenewIfNeededNoCertManager(t *testing.T) {
	m := NewManager(Config{
		Enabled:     true,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Challenge: "http-01"},
	})
	renewed, err := m.RenewIfNeeded([]string{"example.com"})
	if err != nil {
		t.Errorf("RenewIfNeeded(no cert manager) = %v, want nil", err)
	}
	if renewed {
		t.Error("RenewIfNeeded(no cert manager) renewed = true, want false")
	}
}

// Covers RenewIfNeeded's dns-01 branch when the cached certificate is not
// yet within the renewal window: must return nil without attempting a new
// ACME issuance (which would require network access).
func TestRenewIfNeededDNS01NotDue(t *testing.T) {
	m := NewManager(Config{
		Enabled:     true,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Challenge: "dns-01"},
	})
	m.dns01Cert = &tls.Certificate{}
	m.dns01Expiry = time.Now().Add(60 * 24 * time.Hour)

	renewed, err := m.RenewIfNeeded([]string{"dns01.example.com"})
	if err != nil {
		t.Errorf("RenewIfNeeded(dns-01 not due) = %v, want nil", err)
	}
	if renewed {
		t.Error("RenewIfNeeded(dns-01 not due) renewed = true, want false")
	}
}

// Covers CertificateExpiry's dns-01 branch and its not-found error when no
// managed certificate exists for the given domain yet.
func TestCertificateExpiryDNS01(t *testing.T) {
	m := NewManager(Config{
		Enabled:     true,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Challenge: "dns-01"},
	})
	m.dns01Cert = &tls.Certificate{}
	want := time.Now().Add(45 * 24 * time.Hour)
	m.dns01Expiry = want

	got, err := m.CertificateExpiry("dns01.example.com")
	if err != nil {
		t.Fatalf("CertificateExpiry(dns-01) error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("CertificateExpiry(dns-01) = %v, want %v", got, want)
	}
}

func TestCertificateExpiryUnknown(t *testing.T) {
	m := NewManager(Config{Enabled: true})
	if _, err := m.CertificateExpiry("unmanaged.example.com"); err == nil {
		t.Error("CertificateExpiry(unmanaged) = nil error, want error")
	}
}

// Covers certificateExpiry: a real leaf certificate's NotAfter is returned,
// and a certificate with no leaf bytes errors instead of panicking.
func TestCertificateExpiry(t *testing.T) {
	dir := testTempDir(t)
	certPath := filepath.Join(dir, "expiry.crt")
	keyPath := filepath.Join(dir, "expiry.key")
	writeTestCert(t, certPath, keyPath, "expiry.example.com")

	tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}

	expiry, err := certificateExpiry(&tlsCert)
	if err != nil {
		t.Fatalf("certificateExpiry: %v", err)
	}
	if time.Until(expiry) <= 0 || time.Until(expiry) > 25*time.Hour {
		t.Errorf("certificateExpiry = %v, want ~24h from now", expiry)
	}

	if _, err := certificateExpiry(&tls.Certificate{}); err == nil {
		t.Error("certificateExpiry(no leaf) expected error, got nil")
	}
}

// Covers fileExists: present vs absent file.
func TestFileExists(t *testing.T) {
	dir := testTempDir(t)
	present := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(present, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !fileExists(present) {
		t.Error("fileExists(present) = false, want true")
	}
	if fileExists(filepath.Join(dir, "absent.txt")) {
		t.Error("fileExists(absent) = true, want false")
	}
}
