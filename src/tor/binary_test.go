package tor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindTorBinaryConfiguredPathExists(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "tor")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	got, err := findTorBinary(fake)
	if err != nil {
		t.Fatalf("findTorBinary() error = %v", err)
	}
	if got != fake {
		t.Errorf("findTorBinary() = %q, want %q", got, fake)
	}
}

func TestFindTorBinaryConfiguredPathMissing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	_, err := findTorBinary(missing)
	if err != ErrBinaryNotFound {
		t.Errorf("findTorBinary() error = %v, want ErrBinaryNotFound", err)
	}
}

func TestFindTorBinaryConfiguredPathIsDirectory(t *testing.T) {
	dir := t.TempDir()

	_, err := findTorBinary(dir)
	if err != ErrBinaryNotFound {
		t.Errorf("findTorBinary() error = %v, want ErrBinaryNotFound for a directory", err)
	}
}

func TestCommonTorPathsNonEmpty(t *testing.T) {
	paths := commonTorPaths()
	if len(paths) == 0 {
		t.Error("commonTorPaths() returned no candidates for current GOOS")
	}
}

func TestFindTorBinaryAutoDetectNoTorInstalled(t *testing.T) {
	if _, err := exec.LookPath("tor"); err == nil {
		t.Skip("a real tor binary is installed on this machine; auto-detect success path is exercised instead")
	}

	_, err := findTorBinary("")
	if err != ErrBinaryNotFound {
		t.Errorf("findTorBinary(\"\") error = %v, want ErrBinaryNotFound when tor is not installed", err)
	}
}

func TestFindTorBinaryAutoDetectTorInstalled(t *testing.T) {
	path, err := exec.LookPath("tor")
	if err != nil {
		t.Skip("tor is not installed on this machine; not-found path is exercised instead")
	}

	got, err := findTorBinary("")
	if err != nil {
		t.Fatalf("findTorBinary(\"\") error = %v, want nil when tor is on PATH", err)
	}
	if got != path {
		t.Errorf("findTorBinary(\"\") = %q, want PATH-resolved %q", got, path)
	}
}
