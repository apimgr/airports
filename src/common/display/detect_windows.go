//go:build windows

package display

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func (e *DisplayEnv) detectPlatformDisplay() {
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessionID); err == nil && sessionID == 0 {
		e.DisplayType = "none"
		return
	}
	if session := os.Getenv("SESSIONNAME"); strings.HasPrefix(session, "RDP-") {
		e.HasDisplay, e.DisplayType = true, "windows-rdp"
		return
	}
	e.HasDisplay, e.DisplayType = true, "windows"
}
