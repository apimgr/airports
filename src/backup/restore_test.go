package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorizeRestore_EmptyDB(t *testing.T) {
	auth, err := AuthorizeRestore(true, false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != RestoreAllowedEmptyDB {
		t.Fatalf("got %v, want RestoreAllowedEmptyDB", auth)
	}
}

func TestAuthorizeRestore_Root(t *testing.T) {
	auth, err := AuthorizeRestore(false, true, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != RestoreAllowedRoot {
		t.Fatalf("got %v, want RestoreAllowedRoot", auth)
	}
}

func TestAuthorizeRestore_ValidToken(t *testing.T) {
	auth, err := AuthorizeRestore(false, false, "tok_abc123", "tok_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != RestoreRequiresToken {
		t.Fatalf("got %v, want RestoreRequiresToken", auth)
	}
}

func TestAuthorizeRestore_WrongToken(t *testing.T) {
	auth, err := AuthorizeRestore(false, false, "tok_wrong", "tok_abc123")
	if auth != RestoreDenied {
		t.Fatalf("got %v, want RestoreDenied", auth)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthorizeRestore_NoToken(t *testing.T) {
	auth, err := AuthorizeRestore(false, false, "", "tok_abc123")
	if auth != RestoreDenied {
		t.Fatalf("got %v, want RestoreDenied", auth)
	}
	if !errors.Is(err, ErrRestoreDenied) {
		t.Fatalf("expected ErrRestoreDenied, got %v", err)
	}
}

func TestAuthorizeRestore_NoConfiguredToken(t *testing.T) {
	auth, err := AuthorizeRestore(false, false, "anything", "")
	if auth != RestoreDenied {
		t.Fatalf("got %v, want RestoreDenied", auth)
	}
	if !errors.Is(err, ErrRestoreDenied) {
		t.Fatalf("expected ErrRestoreDenied, got %v", err)
	}
}

func TestRestore_RoundTrip(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	archive := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")

	if err := Create(archive, CreateOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		AppVersion: "1.2.3",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	restoreConfig := filepath.Join(restoreRoot, "config")
	restoreData := filepath.Join(restoreRoot, "data")

	err := Restore(archive, RestoreOptions{
		ConfigDir:  restoreConfig,
		DataDir:    restoreData,
		AppVersion: "1.2.3",
	})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(restoreConfig, "server.yml")); statErr != nil {
		t.Fatalf("expected server.yml restored: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(restoreData, "db", "sqlite", "server.db")); statErr != nil {
		t.Fatalf("expected server.db restored: %v", statErr)
	}
}

func TestRestore_VersionMismatchWarning(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	archive := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")

	if err := Create(archive, CreateOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		AppVersion: "1.0.0",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	err := Restore(archive, RestoreOptions{
		ConfigDir:  filepath.Join(restoreRoot, "config"),
		DataDir:    filepath.Join(restoreRoot, "data"),
		AppVersion: "2.0.0",
	})
	if err == nil {
		t.Fatalf("expected a VersionMismatchWarning, got nil")
	}
	var mismatch *VersionMismatchWarning
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *VersionMismatchWarning, got %T: %v", err, err)
	}
	if mismatch.BackupVersion != "1.0.0" || mismatch.RunningVersion != "2.0.0" {
		t.Fatalf("unexpected mismatch fields: %+v", mismatch)
	}

	// Even though a warning was returned, the restore itself must have
	// actually happened.
	if _, statErr := os.Stat(filepath.Join(restoreRoot, "config", "server.yml")); statErr != nil {
		t.Fatalf("expected files to be restored despite version mismatch: %v", statErr)
	}
}

func TestRestore_EncryptedRequiresPassword(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	archive := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz.enc")

	if err := Create(archive, CreateOptions{
		ConfigDir:          configDir,
		DataDir:            dataDir,
		EncryptionPassword: "correct-password",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	err := Restore(archive, RestoreOptions{
		ConfigDir: filepath.Join(restoreRoot, "config"),
		DataDir:   filepath.Join(restoreRoot, "data"),
	})
	if !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("expected ErrPasswordRequired, got %v", err)
	}
}

// buildMaliciousContainer hand-crafts a container whose tar payload contains
// a path-traversal entry, bypassing Create() entirely so the zip-slip guard
// in extractTarGzTo/extractRestoreArchive is what has to catch it.
func buildMaliciousContainer(t *testing.T, path string) {
	t.Helper()

	var tarBuf bytes.Buffer
	gz := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gz)

	evil := []byte("pwned")
	hdr := &tar.Header{
		Name: "config/../../../../etc/passwd",
		Mode: 0o644,
		Size: int64(len(evil)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(evil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}

	payload := tarBuf.Bytes()
	manifest := &Manifest{
		Version:   ManifestVersion,
		CreatedBy: "test",
		Checksum:  checksumOf(payload),
	}
	manifestJSON, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	defer f.Close()

	f.Write(containerMagic[:])
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(manifestJSON)))
	f.Write(lenBuf[:])
	f.Write(manifestJSON)
	f.Write(payload)
}

func TestRestore_RejectsZipSlip(t *testing.T) {
	dir := testTempDir(t)
	archive := filepath.Join(dir, "malicious.tar.gz")
	buildMaliciousContainer(t, archive)

	restoreRoot := testTempDir(t)
	restoreConfig := filepath.Join(restoreRoot, "config")
	restoreData := filepath.Join(restoreRoot, "data")

	err := Restore(archive, RestoreOptions{
		ConfigDir: restoreConfig,
		DataDir:   restoreData,
	})
	if err == nil {
		t.Fatalf("expected zip-slip entry to be rejected")
	}

	if _, statErr := os.Stat("/etc/passwd-pwned"); statErr == nil {
		t.Fatalf("zip-slip entry escaped the target directory")
	}
}
