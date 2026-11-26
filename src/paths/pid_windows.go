//go:build windows

package paths

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// isProcessRunning reports whether a process with the given PID exists
// (Windows), per AI.md PART 8 "PID File Handling".
func isProcessRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == 259 // STILL_ACTIVE
}

// isOurProcess verifies the process at pid is actually this binary
// (Windows), per AI.md PART 8 ("Exact match - substring matching would also
// match {project_name}-cli.exe"). When SetBinaryBaseName was never called,
// presence of the process is treated as sufficient (best-effort).
func isOurProcess(pid int) bool {
	if binaryBaseName == "" {
		return true
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return false
	}
	exePath := windows.UTF16ToString(buf[:size])
	base := filepath.Base(exePath)
	return strings.EqualFold(base, binaryBaseName) || strings.EqualFold(base, binaryBaseName+".exe")
}
