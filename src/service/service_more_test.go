package service

import (
	"os"
	"path/filepath"
	"testing"
)

// withStdin temporarily replaces os.Stdin with a pipe pre-loaded with input,
// restoring the original os.Stdin when the test completes.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}
	w.Close()

	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

// confirmUninstall must only treat exact "y"/"yes" (case-insensitive, with
// surrounding whitespace trimmed) as confirmation - everything else,
// including empty input, must cancel the destructive operation.
func TestConfirmUninstall(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"lowercase-y", "y\n", true},
		{"lowercase-yes", "yes\n", true},
		{"uppercase-Y", "Y\n", true},
		{"uppercase-YES", "YES\n", true},
		{"mixed-case-Yes", "Yes\n", true},
		{"padded-whitespace", "  y  \n", true},
		{"lowercase-n", "n\n", false},
		{"uppercase-N", "N\n", false},
		{"no", "no\n", false},
		{"empty-line", "\n", false},
		{"whitespace-only", "   \n", false},
		{"garbage", "maybe\n", false},
		{"yesplease-not-exact", "yesplease\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withStdin(t, tt.input)
			if got := confirmUninstall(); got != tt.want {
				t.Errorf("confirmUninstall() with input %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// confirmUninstall reads a single line via bufio.Reader.ReadString('\n') -
// EOF with no trailing newline (e.g. stdin closed/piped from a non-terminal
// with no final newline) must return false, not panic or hang.
func TestConfirmUninstallEOFNoNewline(t *testing.T) {
	withStdin(t, "y")
	// ReadString('\n') returns an error (io.EOF) when the final newline is
	// missing, so confirmUninstall must treat this as "not confirmed".
	if got := confirmUninstall(); got != false {
		t.Errorf("confirmUninstall() with unterminated input = %v, want false", got)
	}
}

// confirmUninstall on a completely empty stdin (immediate EOF) must also
// return false rather than blocking or panicking.
func TestConfirmUninstallEmptyStdin(t *testing.T) {
	withStdin(t, "")
	if got := confirmUninstall(); got != false {
		t.Errorf("confirmUninstall() with empty stdin = %v, want false", got)
	}
}

// newServiceTestDir creates an isolated temp directory under
// /tmp/apimgr/airports-XXXXXX/ per project testing rules, cleaned up
// automatically when the test completes.
func newServiceTestDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll /tmp/apimgr: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-service-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// copyBinary must copy file contents byte-for-byte, create the destination
// directory if missing, and preserve executability (0755).
func TestCopyBinary(t *testing.T) {
	dir := newServiceTestDir(t)
	src := filepath.Join(dir, "src-binary")
	content := []byte("fake binary contents\x00\x01\x02")
	if err := os.WriteFile(src, content, 0755); err != nil {
		t.Fatalf("WriteFile(src): %v", err)
	}

	// Destination directory does not exist yet - copyBinary must create it.
	dst := filepath.Join(dir, "nested", "subdir", "dst-binary")
	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("copyBinary: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(dst): %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copyBinary content mismatch: got %q, want %q", got, content)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat(dst): %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Errorf("copyBinary dst mode = %v, want executable bit set", info.Mode())
	}
}

// copyBinary with a missing source file must return an error, not panic,
// and must not create a partial/empty destination file.
func TestCopyBinaryMissingSource(t *testing.T) {
	dir := newServiceTestDir(t)
	src := filepath.Join(dir, "does-not-exist")
	dst := filepath.Join(dir, "dst-binary")

	if err := copyBinary(src, dst); err == nil {
		t.Fatal("copyBinary with missing source: expected error, got nil")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("copyBinary with missing source should not create dst, stat err = %v", err)
	}
}

// copyBinary called twice with the same destination (re-install/upgrade
// scenario) must overwrite the previous content, not append or fail.
func TestCopyBinaryOverwritesExisting(t *testing.T) {
	dir := newServiceTestDir(t)
	src := filepath.Join(dir, "src-binary")
	dst := filepath.Join(dir, "dst-binary")

	if err := os.WriteFile(src, []byte("version-one"), 0755); err != nil {
		t.Fatalf("WriteFile(src v1): %v", err)
	}
	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("copyBinary (first): %v", err)
	}

	if err := os.WriteFile(src, []byte("version-two-longer-content"), 0755); err != nil {
		t.Fatalf("WriteFile(src v2): %v", err)
	}
	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("copyBinary (second): %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(dst): %v", err)
	}
	if string(got) != "version-two-longer-content" {
		t.Errorf("copyBinary did not overwrite: got %q", got)
	}
}

// idAvailable must not panic on the current process's real UID and must
// return a plain bool - this exercises the LookupId code path without
// requiring root or mutating the system user database.
func TestIdAvailableCurrentUser(t *testing.T) {
	uid := os.Getuid()
	if uid < 0 {
		t.Skip("no numeric UID available on this platform")
	}
	_ = idAvailable(uid)
}

// serviceHomeDir must return a non-empty path derived from paths.GetDefaultDirs,
// and must be stable (idempotent) across repeated calls.
func TestServiceHomeDir(t *testing.T) {
	first := serviceHomeDir()
	if first == "" {
		t.Fatal("serviceHomeDir() returned empty string")
	}
	if second := serviceHomeDir(); second != first {
		t.Errorf("serviceHomeDir() not stable: %q vs %q", first, second)
	}
}

// windowsVSA must be the documented Virtual Service Account identity -
// never Local System, Administrator, or a plain username. This guards
// against an accidental regression toward a less-secure account.
func TestWindowsVSAConstant(t *testing.T) {
	want := `NT SERVICE\airports`
	if windowsVSA != want {
		t.Errorf("windowsVSA = %q, want %q", windowsVSA, want)
	}
}

// NOTE: the following service.go functions are intentionally NOT covered by
// unit tests in this package, because they require root privileges, mutate
// real system state (user/group database, /etc/systemd, /etc/init.d,
// /Library/LaunchDaemons, Windows service registry, etc.), or shell out to
// real service managers (systemctl, rc-service, launchctl, sc.exe) with no
// injectable/mockable interface:
//   EnsureSystemUser, createSystemGroup, createSystemUser, DeleteSystemUser,
//   removeAllData, Install, Uninstall,
//   installSystemd/uninstallSystemd, installOpenRC/uninstallOpenRC,
//   installSysVinit/uninstallSysVinit, installRunit/uninstallRunit,
//   installLaunchd/uninstallLaunchd, installWindows/uninstallWindows,
//   installBSDRC/uninstallBSDRC,
//   Start, Stop, Restart, Reload, Status, Enable, Disable, Logs.
// Reaching 60% coverage on this file without refactoring toward an
// injectable exec.Command runner (or running as root in a disposable
// container) is not achievable safely.
