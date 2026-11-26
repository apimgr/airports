package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	if err := CreateBackupArchive(archive, BackupArchiveOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		AppVersion: "1.2.3",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	restoreConfig := filepath.Join(restoreRoot, "config")
	restoreData := filepath.Join(restoreRoot, "data")

	err := RestoreBackupArchive(archive, BackupRestoreOptions{
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

	if err := CreateBackupArchive(archive, BackupArchiveOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		AppVersion: "1.0.0",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	err := RestoreBackupArchive(archive, BackupRestoreOptions{
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

	if err := CreateBackupArchive(archive, BackupArchiveOptions{
		ConfigDir:          configDir,
		DataDir:            dataDir,
		EncryptionPassword: "correct-password",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreRoot := testTempDir(t)
	err := RestoreBackupArchive(archive, BackupRestoreOptions{
		ConfigDir: filepath.Join(restoreRoot, "config"),
		DataDir:   filepath.Join(restoreRoot, "data"),
	})
	if !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("expected ErrPasswordRequired, got %v", err)
	}
}

// buildMaliciousContainer hand-crafts a container whose tar payload contains
// a path-traversal entry, bypassing CreateBackupArchive() entirely so the zip-slip guard
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
	fileDigest := sha256.Sum256(evil)
	file := BackupFile{Path: "config/../../../../etc/passwd", Size: int64(len(evil)), SHA256: hex.EncodeToString(fileDigest[:])}
	checksum := sha256.Sum256([]byte(file.SHA256 + "  " + file.Path + "\n"))
	manifest := &Manifest{
		Version:    ManifestVersion,
		CreatedBy:  "test",
		Type:       "full",
		SnapshotAt: "2025-01-15T10:30:00Z",
		Files:      []BackupFile{file},
		Checksum:   hex.EncodeToString(checksum[:]),
	}
	manifestJSON, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(containerMagic[:]); err != nil {
		t.Fatalf("write container magic: %v", err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(manifestJSON)))
	for _, data := range [][]byte{lenBuf[:], manifestJSON, payload} {
		if _, err := f.Write(data); err != nil {
			t.Fatalf("write container data: %v", err)
		}
	}
}

func TestRestore_RejectsZipSlip(t *testing.T) {
	dir := testTempDir(t)
	archive := filepath.Join(dir, "malicious.tar.gz")
	buildMaliciousContainer(t, archive)

	restoreRoot := testTempDir(t)
	restoreConfig := filepath.Join(restoreRoot, "config")
	restoreData := filepath.Join(restoreRoot, "data")

	err := RestoreBackupArchive(archive, BackupRestoreOptions{
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

func TestExtractTarGzRejectsSymlinkedDestinationAncestor(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	outsideDir := filepath.Join(root, "outside")
	for _, dir := range []string{configDir, dataDir, outsideDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outsideDir, filepath.Join(configDir, "linked")); err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	content := []byte("must not escape")
	if err := tw.WriteHeader(&tar.Header{Name: "config/linked/secret.txt", Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := extractTarGzTo(archive.Bytes(), func(name string) (string, string, bool) {
		if strings.HasPrefix(name, "config/") {
			return filepath.Join(configDir, strings.TrimPrefix(name, "config/")), configDir, true
		}
		return "", "", false
	})
	if err == nil {
		t.Fatal("extraction through a symlinked ancestor succeeded")
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "secret.txt")); !os.IsNotExist(err) {
		t.Fatalf("symlink target was modified, stat error = %v", err)
	}
}

func TestValidateExpandedArchiveLimits(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct{ name, data string }{{"config/a", "abc"}, {"data/b", "def"}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o600, Size: int64(len(entry.data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	data := archive.Bytes()
	base := RestoreLimits{MaxArchiveSize: 1 << 20, MaxExpandedSize: 16, MaxFiles: 2, MaxFileSize: 3, MaxRatio: 100}
	if err := validateExpandedArchive(data, 1<<20, base); err != nil {
		t.Fatalf("valid archive: %v", err)
	}
	if err := validateExpandedArchive(data, 1<<20, RestoreLimits{MaxExpandedSize: 16, MaxFiles: 1, MaxFileSize: 3, MaxRatio: 100}); err == nil {
		t.Fatal("max_files limit was not enforced")
	}
	if err := validateExpandedArchive(data, 1<<20, RestoreLimits{MaxExpandedSize: 5, MaxFiles: 2, MaxFileSize: 3, MaxRatio: 100}); err == nil {
		t.Fatal("max_expanded_size limit was not enforced")
	}
	if err := validateExpandedArchive(data, 1<<20, RestoreLimits{MaxExpandedSize: 16, MaxFiles: 2, MaxFileSize: 2, MaxRatio: 100}); err == nil {
		t.Fatal("max_file_size limit was not enforced")
	}
	if err := validateExpandedArchive(data, 1, RestoreLimits{MaxExpandedSize: 16, MaxFiles: 2, MaxFileSize: 3, MaxRatio: 1}); err == nil {
		t.Fatal("max_ratio limit was not enforced")
	}
}

func TestValidateExpandedArchiveRejectsUnsafeEntries(t *testing.T) {
	cases := []tar.Header{
		{Name: "", Typeflag: tar.TypeReg},
		{Name: "/absolute", Typeflag: tar.TypeReg},
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "C:/drive", Typeflag: tar.TypeReg},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target"},
		{Name: "hardlink", Typeflag: tar.TypeLink, Linkname: "target"},
		{Name: "device", Typeflag: tar.TypeChar},
		{Name: "pipe", Typeflag: tar.TypeFifo},
		{Name: "setuid", Typeflag: tar.TypeReg, Mode: 0o4755},
	}
	for _, header := range cases {
		var archive bytes.Buffer
		gz := gzip.NewWriter(&archive)
		tarWriter := tar.NewWriter(gz)
		if err := tarWriter.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if err := tarWriter.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := validateExpandedArchive(archive.Bytes(), 1<<20, RestoreLimits{}); err == nil {
			t.Errorf("unsafe entry %q was accepted", header.Name)
		}
	}
}
