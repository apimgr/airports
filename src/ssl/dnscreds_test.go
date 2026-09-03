package ssl

import "testing"

// Covers EncryptDNSCredentials/DecryptDNSCredentials round trip: encrypting
// a credential map and decrypting it with the same key must reproduce the
// original values.
func TestEncryptDecryptDNSCredentialsRoundTrip(t *testing.T) {
	key := "test-encryption-key-value"
	creds := map[string]string{
		"CLOUDFLARE_DNS_API_TOKEN": "super-secret-token",
		"CLOUDFLARE_EMAIL":         "ops@example.com",
	}

	encoded, err := EncryptDNSCredentials(key, creds)
	if err != nil {
		t.Fatalf("EncryptDNSCredentials: %v", err)
	}
	if encoded == "" {
		t.Fatal("EncryptDNSCredentials returned empty string")
	}

	decoded, err := DecryptDNSCredentials(key, encoded)
	if err != nil {
		t.Fatalf("DecryptDNSCredentials: %v", err)
	}
	if len(decoded) != len(creds) {
		t.Fatalf("decoded = %+v, want %+v", decoded, creds)
	}
	for k, v := range creds {
		if decoded[k] != v {
			t.Errorf("decoded[%q] = %q, want %q", k, decoded[k], v)
		}
	}
}

// Covers DecryptDNSCredentials with an empty blob (nothing configured yet):
// must return an empty map, not an error.
func TestDecryptDNSCredentialsEmptyBlob(t *testing.T) {
	creds, err := DecryptDNSCredentials("some-key", "")
	if err != nil {
		t.Fatalf("DecryptDNSCredentials(empty): unexpected error: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("DecryptDNSCredentials(empty) = %+v, want empty map", creds)
	}
}

// Covers the missing-encryption-key error paths on both directions.
func TestDNSCredentialsMissingEncryptionKey(t *testing.T) {
	if _, err := EncryptDNSCredentials("", map[string]string{"A": "B"}); err == nil {
		t.Error("EncryptDNSCredentials(\"\") expected error, got nil")
	}
	if _, err := DecryptDNSCredentials("", "anything"); err == nil {
		t.Error("DecryptDNSCredentials(\"\") expected error, got nil")
	}
}

// Covers DecryptDNSCredentials rejecting a wrong key (AES-GCM auth failure)
// and malformed base64 input.
func TestDecryptDNSCredentialsInvalidInput(t *testing.T) {
	encoded, err := EncryptDNSCredentials("correct-key", map[string]string{"A": "B"})
	if err != nil {
		t.Fatalf("EncryptDNSCredentials: %v", err)
	}

	if _, err := DecryptDNSCredentials("wrong-key", encoded); err == nil {
		t.Error("DecryptDNSCredentials with wrong key: expected error, got nil")
	}

	if _, err := DecryptDNSCredentials("correct-key", "not-valid-base64!!!"); err == nil {
		t.Error("DecryptDNSCredentials with invalid base64: expected error, got nil")
	}

	if _, err := DecryptDNSCredentials("correct-key", "AA=="); err == nil {
		t.Error("DecryptDNSCredentials with too-short ciphertext: expected error, got nil")
	}
}
