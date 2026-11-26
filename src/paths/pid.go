package paths

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// binaryBaseName is the exact (non-substring) process/executable name a PID
// file entry must match to be considered "our process" — set once by the
// server binary via SetBinaryBaseName. Left empty, isOurProcess treats every
// running PID as a match (best-effort identity check), matching the pre-PART-8
// behavior for callers that never opt in.
var binaryBaseName string

// SetBinaryBaseName records the exact binary name (e.g. "airports", not
// "airports-cli") used by isOurProcess to distinguish a genuine stale PID
// (process gone, or PID reused by an unrelated program) from a real running
// instance, per AI.md PART 8 "PID File Handling" ("Exact match - substring
// matching would also match {project_name}-cli").
func SetBinaryBaseName(name string) {
	binaryBaseName = name
}

// CheckPIDFile reports whether the PID file at pidPath refers to a still-running
// instance of this binary, per AI.md PART 8 "PID File Handling". A missing,
// corrupt, or stale (process gone / PID reused by a different binary) PID
// file is treated as "not running" and removed so the caller can proceed to
// write a fresh one - crash/kill -9 recovery is automatic on every startup.
func CheckPIDFile(pidPath string) (running bool, pid int, err error) {
	data, readErr := os.ReadFile(pidPath)
	if os.IsNotExist(readErr) {
		return false, 0, nil
	}
	if readErr != nil {
		return false, 0, fmt.Errorf("reading pid file: %w", readErr)
	}

	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if convErr != nil {
		// Corrupt PID file - remove it.
		_ = os.Remove(pidPath)
		return false, 0, nil
	}

	if !isProcessRunning(pid) {
		// Stale PID file (process gone) - remove it.
		_ = os.Remove(pidPath)
		return false, 0, nil
	}

	if !isOurProcess(pid) {
		// PID was reused by an unrelated process - remove the stale file.
		_ = os.Remove(pidPath)
		return false, 0, nil
	}

	return true, pid, nil
}
