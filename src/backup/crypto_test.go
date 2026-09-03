package backup

import "testing"

func TestEncryptDecryptArchiveRoundTrip(t *testing.T) {
	plaintext := []byte("hello backup world, this is a fake tar.gz payload")
	password := "correct horse battery staple"

	blob, err := EncryptArchive(password, plaintext)
	if err != nil {
		t.Fatalf("EncryptArchive: %v", err)
	}
	if len(blob) == 0 {
		t.Fatalf("EncryptArchive returned empty blob")
	}

	got, err := DecryptArchive(password, blob)
	if err != nil {
		t.Fatalf("DecryptArchive: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("round trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestDecryptArchiveWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	blob, err := EncryptArchive("correct-password", plaintext)
	if err != nil {
		t.Fatalf("EncryptArchive: %v", err)
	}

	if _, err := DecryptArchive("wrong-password", blob); err == nil {
		t.Fatalf("expected error decrypting with wrong password, got nil")
	}
}

func TestDecryptArchiveTruncatedBlob(t *testing.T) {
	plaintext := []byte("secret data")
	blob, err := EncryptArchive("a-password", plaintext)
	if err != nil {
		t.Fatalf("EncryptArchive: %v", err)
	}

	truncated := blob[:len(blob)/2]
	if _, err := DecryptArchive("a-password", truncated); err == nil {
		t.Fatalf("expected error decrypting truncated blob, got nil")
	}

	tooShort := []byte{1, 2, 3}
	if _, err := DecryptArchive("a-password", tooShort); err == nil {
		t.Fatalf("expected error decrypting too-short blob, got nil")
	}
}

func TestEncryptArchiveEmptyPassword(t *testing.T) {
	if _, err := EncryptArchive("", []byte("data")); err == nil {
		t.Fatalf("expected error encrypting with empty password")
	}
}

func TestDecryptArchiveEmptyPassword(t *testing.T) {
	if _, err := DecryptArchive("", []byte("data")); err == nil {
		t.Fatalf("expected error decrypting with empty password")
	}
}
