package service

import (
	"runtime"
	"testing"
)

func TestIsReservedID(t *testing.T) {
	cases := []struct {
		id       int
		reserved bool
	}{
		{65534, true},
		{980, true},
		{999, true},
		{990, true},
		{101, true},
		{110, true},
		{105, true},
		{170, true},
		{179, true},
		{175, true},
		{1000, false},
		{500, false},
		{200, false},
		{899, false},
		{111, false},
		{169, false},
		{180, false},
	}
	for _, c := range cases {
		if got := isReservedID(c.id); got != c.reserved {
			t.Errorf("isReservedID(%d) = %v, want %v", c.id, got, c.reserved)
		}
	}
}

func TestFindAvailableID(t *testing.T) {
	top := linuxSafeTop
	if runtime.GOOS == "darwin" {
		top = darwinSafeTop
	}
	id, err := findAvailableID(top)
	if err != nil {
		t.Fatalf("findAvailableID(%d) returned error: %v", top, err)
	}
	if id < safeBottom || id > top {
		t.Errorf("findAvailableID(%d) = %d, want in range [%d, %d]", top, id, safeBottom, top)
	}
	if isReservedID(id) {
		t.Errorf("findAvailableID(%d) = %d, want non-reserved", top, id)
	}
	if !idAvailable(id) {
		t.Errorf("findAvailableID(%d) = %d, want available", top, id)
	}
}

func TestFindAvailableIDExhaustedRange(t *testing.T) {
	if _, err := findAvailableID(safeBottom - 1); err == nil {
		t.Errorf("findAvailableID(%d) should error on an exhausted range", safeBottom-1)
	}
}

func TestNologinShell(t *testing.T) {
	shell := nologinShell()
	if shell == "" {
		t.Fatal("nologinShell() returned empty string")
	}
	if runtime.GOOS == "darwin" && shell != "/usr/bin/false" {
		t.Errorf("nologinShell() on darwin = %q, want /usr/bin/false", shell)
	}
}

func TestDetectServiceManagerReturnsKnownType(t *testing.T) {
	valid := map[ServiceType]bool{
		ServiceUnknown:  true,
		ServiceSystemd:  true,
		ServiceOpenRC:   true,
		ServiceSysVinit: true,
		ServiceRunit:    true,
		ServiceLaunchd:  true,
		ServiceWindows:  true,
		ServiceBSDRC:    true,
	}
	got := DetectServiceManager()
	if !valid[got] {
		t.Errorf("DetectServiceManager() = %v, not a known ServiceType", got)
	}
	switch runtime.GOOS {
	case "darwin":
		if got != ServiceLaunchd {
			t.Errorf("DetectServiceManager() on darwin = %v, want ServiceLaunchd", got)
		}
	case "windows":
		if got != ServiceWindows {
			t.Errorf("DetectServiceManager() on windows = %v, want ServiceWindows", got)
		}
	case "freebsd", "openbsd", "netbsd":
		if got != ServiceBSDRC {
			t.Errorf("DetectServiceManager() on %s = %v, want ServiceBSDRC", runtime.GOOS, got)
		}
	}
}

func TestGetBinaryPath(t *testing.T) {
	path := GetBinaryPath()
	if path == "" {
		t.Fatal("GetBinaryPath() returned empty string")
	}
	if runtime.GOOS == "windows" {
		if !contains(path, ".exe") {
			t.Errorf("GetBinaryPath() on windows = %q, want .exe suffix", path)
		}
	} else {
		if !contains(path, appName) {
			t.Errorf("GetBinaryPath() = %q, want it to contain %q", path, appName)
		}
	}
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
