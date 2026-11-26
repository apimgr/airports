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
	if string(blob[:5]) != "BKUP1" {
		t.Fatalf("header magic = %q", blob[:5])
	}
	if header, err := ParseEncryptionHeader(blob); err != nil || header.BaseName != "" {
		t.Fatalf("ParseEncryptionHeader() = %+v, %v", header, err)
	}

	got, err := DecryptArchive(password, blob)
	if err != nil {
		t.Fatalf("DecryptArchive: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("round trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptArchiveWithBaseHeader(t *testing.T) {
	blob, err := EncryptArchiveWithBase("secret", []byte("payload"), "base.tar.gz.enc")
	if err != nil {
		t.Fatal(err)
	}
	header, err := ParseEncryptionHeader(blob)
	if err != nil {
		t.Fatal(err)
	}
	if header.BaseName != "base.tar.gz.enc" {
		t.Fatalf("BaseName = %q", header.BaseName)
	}
	match, err := PasswordMatchesHeader("secret", header)
	if err != nil || !match {
		t.Fatalf("password match = %v, %v", match, err)
	}
	match, err = PasswordMatchesHeader("wrong", header)
	if err != nil || match {
		t.Fatalf("wrong password match = %v, %v", match, err)
	}
	blob[53], blob[54] = 1, 0
	if _, err := ParseEncryptionHeader(blob); err == nil {
		t.Fatal("expected malformed base length error")
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
