package ssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
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
	os.Unsetenv("CLOUDFLARE_DNS_API_TOKEN")
	os.Unsetenv("CLOUDFLARE_EMAIL")
	os.Unsetenv("CLOUDFLARE_API_KEY")
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
