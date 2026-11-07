//go:build !windows

package display

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func (e *DisplayEnv) detectPlatformDisplay() {
	if value := os.Getenv("WAYLAND_DISPLAY"); value != "" {
		e.HasDisplay, e.DisplayType = true, "wayland"
		return
	}
	if value := os.Getenv("DISPLAY"); value != "" {
		e.HasDisplay, e.DisplayType = true, "x11"
		return
	}
	if runtime.GOOS == "darwin" && !e.IsSSH {
		if os.Getenv("__CFBundleIdentifier") != "" {
			e.HasDisplay, e.DisplayType = true, "macos"
			return
		}
		if output, err := exec.Command("launchctl", "managername").Output(); err == nil && strings.Contains(string(output), "Aqua") {
			e.HasDisplay, e.DisplayType = true, "macos"
			return
		}
	}
	e.DisplayType = "none"
}
