//go:build !windows

package paths

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// isProcessRunning reports whether a process with the given PID exists
// (Unix), per AI.md PART 8 "PID File Handling".
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds - signal 0 is the actual probe.
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but belongs to another user - it IS running.
	return errors.Is(err, syscall.EPERM)
}

// isOurProcess verifies the process at pid is actually this binary (Unix),
// per AI.md PART 8 ("Exact match - substring matching would also match
// {project_name}-cli"). When SetBinaryBaseName was never called, presence of
// the process is treated as sufficient (best-effort).
func isOurProcess(pid int) bool {
	if binaryBaseName == "" {
		return true
	}

	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		// /proc is unavailable (macOS/BSD) - fall back to ps.
		return isOurProcessViaPS(pid)
	}
	return filepath.Base(exePath) == binaryBaseName
}

// isOurProcessViaPS checks process identity via `ps` on platforms without
// /proc/{pid}/exe (macOS/BSD), per AI.md PART 8.
func isOurProcessViaPS(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == binaryBaseName
}
