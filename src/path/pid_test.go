package paths

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// resetBinaryBaseName restores the package-level binaryBaseName after a test
// that changes it, so other tests are not affected by ordering.
func resetBinaryBaseName(t *testing.T) {
	t.Helper()
	orig := binaryBaseName
	t.Cleanup(func() { binaryBaseName = orig })
}

func TestCheckPIDFileMissing(t *testing.T) {
	dir := newScratchDir(t)
	pidPath := filepath.Join(dir, "missing.pid")

	running, pid, err := CheckPIDFile(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("running = true, want false for a missing PID file")
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

func TestCheckPIDFileCorruptRemovesFile(t *testing.T) {
	dir := newScratchDir(t)
	pidPath := filepath.Join(dir, "corrupt.pid")
	if err := os.WriteFile(pidPath, []byte("not-a-pid"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, _, err := CheckPIDFile(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("running = true, want false for a corrupt PID file")
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Errorf("corrupt PID file was not removed")
	}
}

func TestCheckPIDFileStaleDeadProcessRemovesFile(t *testing.T) {
	dir := newScratchDir(t)
	pidPath := filepath.Join(dir, "stale.pid")
	// PID 1 is always running (init/systemd) on any Unix, but a PID this
	// high is extremely unlikely to correspond to any live process,
	// simulating a crashed/kill -9'd instance.
	const deadPID = 999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(deadPID)), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, _, err := CheckPIDFile(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("running = true, want false for a dead PID")
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Errorf("stale PID file was not removed")
	}
}

func TestCheckPIDFileLiveOwnProcess(t *testing.T) {
	resetBinaryBaseName(t)
	// With no binaryBaseName set, isOurProcess is best-effort (always true),
	// so a PID file pointing at the current (definitely running) test
	// process must be reported as running.
	binaryBaseName = ""

	dir := newScratchDir(t)
	pidPath := filepath.Join(dir, "live.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid, err := CheckPIDFile(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Errorf("running = false, want true for the current process's own PID")
	}
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
	if _, statErr := os.Stat(pidPath); statErr != nil {
		t.Errorf("live PID file was unexpectedly removed: %v", statErr)
	}
}

func TestCheckPIDFileLiveButDifferentBinaryIsStale(t *testing.T) {
	resetBinaryBaseName(t)
	// A binary name that can never match this test binary's own exe/comm
	// name simulates PID reuse by an unrelated process - it must be treated
	// as stale even though the process (this test) is genuinely running.
	binaryBaseName = "definitely-not-this-test-binary"

	dir := newScratchDir(t)
	pidPath := filepath.Join(dir, "reused.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, _, err := CheckPIDFile(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("running = true, want false when the PID belongs to a different binary")
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Errorf("PID-reuse file was not removed")
	}
}

func TestSetBinaryBaseName(t *testing.T) {
	resetBinaryBaseName(t)
	SetBinaryBaseName("airports")
	if binaryBaseName != "airports" {
		t.Errorf("binaryBaseName = %q, want airports", binaryBaseName)
	}
}
