package backup

import "testing"

func TestManifestMarshalUnmarshalRoundTrip(t *testing.T) {
	m := &Manifest{
		Version:          ManifestVersion,
		CreatedAt:        "2025-01-15T10:30:00Z",
		CreatedBy:        "operator",
		AppVersion:       "1.2.3",
		Contents:         []string{"server.yml", "server.db", "template/", "ssl/"},
		Encrypted:        true,
		EncryptionMethod: "AES-256-GCM",
		Checksum:         "sha256:abc123",
	}

	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := UnmarshalManifest(data)
	if err != nil {
		t.Fatalf("UnmarshalManifest: %v", err)
	}

	if got.Version != m.Version || got.CreatedBy != m.CreatedBy || got.AppVersion != m.AppVersion ||
		got.Encrypted != m.Encrypted || got.EncryptionMethod != m.EncryptionMethod || got.Checksum != m.Checksum {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, m)
	}
	if len(got.Contents) != len(m.Contents) {
		t.Fatalf("contents length mismatch: got %v want %v", got.Contents, m.Contents)
	}
}

func TestUnmarshalManifestInvalidJSON(t *testing.T) {
	if _, err := UnmarshalManifest([]byte("not json")); err == nil {
		t.Fatalf("expected error unmarshaling invalid JSON")
	}
}

func TestManifestUnencryptedOmitsEncryptionMethod(t *testing.T) {
	m := &Manifest{
		Version:   ManifestVersion,
		Encrypted: false,
		Checksum:  "sha256:xyz",
	}
	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := UnmarshalManifest(data)
	if err != nil {
		t.Fatalf("UnmarshalManifest: %v", err)
	}
	if got.EncryptionMethod != "" {
		t.Fatalf("expected empty encryption_method, got %q", got.EncryptionMethod)
	}
}
