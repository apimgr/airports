//go:build !windows

package service

import (
	"errors"
	"strings"
	"testing"
)

// fakeLookPath returns a lookPath-compatible function that reports the
// given names as present in PATH and everything else as missing.
func fakeLookPath(present ...string) func(string) (string, error) {
	set := make(map[string]bool, len(present))
	for _, p := range present {
		set[p] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestEscalationOrder(t *testing.T) {
	tests := []struct {
		goos string
		want []escalationMethod
	}{
		{"linux", []escalationMethod{methodSudo, methodSu, methodPkexec, methodDoas}},
		{"darwin", []escalationMethod{methodSudo, methodOsascript}},
		{"freebsd", []escalationMethod{methodDoas, methodSudo, methodSu}},
		{"openbsd", []escalationMethod{methodDoas, methodSudo, methodSu}},
		{"netbsd", []escalationMethod{methodDoas, methodSudo, methodSu}},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got := escalationOrder(tt.goos)
			if len(got) != len(tt.want) {
				t.Fatalf("escalationOrder(%q) = %v, want %v", tt.goos, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("escalationOrder(%q)[%d] = %q, want %q", tt.goos, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectEscalationMethod(t *testing.T) {
	tests := []struct {
		name string
		goos string
		lp   func(string) (string, error)
		want escalationMethod
	}{
		{"linux nothing available", "linux", fakeLookPath(), methodNone},
		{"linux only pkexec", "linux", fakeLookPath("pkexec"), methodPkexec},
		{"linux su before pkexec", "linux", fakeLookPath("su", "pkexec"), methodSu},
		{"linux doas is last resort", "linux", fakeLookPath("doas"), methodDoas},
		{"darwin only osascript", "darwin", fakeLookPath("osascript"), methodOsascript},
		{"darwin nothing available", "darwin", fakeLookPath(), methodNone},
		{"bsd doas preferred over su", "freebsd", fakeLookPath("doas", "su"), methodDoas},
		{"bsd su fallback", "freebsd", fakeLookPath("su"), methodSu},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectEscalationMethod(tt.goos, tt.lp)
			if got != tt.want {
				t.Errorf("detectEscalationMethod(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestDetectEscalationMethodSudoRequiresBinary(t *testing.T) {
	// sudo group membership / passwordless checks only matter if the sudo
	// binary itself is present; with no binaries at all, sudo must not be
	// selected even if the test-runner's own user happens to be in a
	// privileged group.
	got := detectEscalationMethod("linux", fakeLookPath())
	if got == methodSudo {
		t.Errorf("detectEscalationMethod selected sudo with no sudo binary present")
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"it's got a quote", `'it'\''s got a quote'`},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAppleScriptQuote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"simple", `"simple"`},
		{`has "quotes"`, `"has \"quotes\""`},
		{`back\slash`, `"back\\slash"`},
	}
	for _, tt := range tests {
		if got := appleScriptQuote(tt.in); got != tt.want {
			t.Errorf("appleScriptQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildElevationCommand(t *testing.T) {
	t.Run("sudo passes args directly", func(t *testing.T) {
		cmd, err := buildElevationCommand(methodSudo, []string{"/usr/bin/app", "--service", "start"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"sudo", "/usr/bin/app", "--service", "start"}
		if len(cmd.Args) != len(want) {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
		for i := range want {
			if cmd.Args[i] != want[i] {
				t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], want[i])
			}
		}
	})

	t.Run("pkexec passes args directly", func(t *testing.T) {
		cmd, err := buildElevationCommand(methodPkexec, []string{"/usr/bin/app", "--service", "start"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd.Args[0] != "pkexec" {
			t.Errorf("Args[0] = %q, want pkexec", cmd.Args[0])
		}
	})

	t.Run("doas passes args directly", func(t *testing.T) {
		cmd, err := buildElevationCommand(methodDoas, []string{"/usr/bin/app"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd.Args[0] != "doas" {
			t.Errorf("Args[0] = %q, want doas", cmd.Args[0])
		}
	})

	t.Run("su wraps args in a single -c shell string", func(t *testing.T) {
		cmd, err := buildElevationCommand(methodSu, []string{"/usr/bin/app", "--service", "start"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"su", "-c", "'/usr/bin/app' '--service' 'start'"}
		if len(cmd.Args) != len(want) {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
		for i := range want {
			if cmd.Args[i] != want[i] {
				t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], want[i])
			}
		}
	})

	t.Run("osascript wraps args in an AppleScript shell-script string", func(t *testing.T) {
		cmd, err := buildElevationCommand(methodOsascript, []string{"/usr/bin/app", "--service", "start"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd.Args[0] != "osascript" || cmd.Args[1] != "-e" {
			t.Fatalf("Args = %v, want [osascript -e ...]", cmd.Args)
		}
		script := cmd.Args[2]
		if !strings.Contains(script, "do shell script") || !strings.Contains(script, "with administrator privileges") {
			t.Errorf("script = %q, missing expected AppleScript wrapper", script)
		}
		if !strings.Contains(script, "/usr/bin/app") {
			t.Errorf("script = %q, missing target binary path", script)
		}
	})

	t.Run("unknown method errors", func(t *testing.T) {
		if _, err := buildElevationCommand(methodNone, []string{"app"}); err == nil {
			t.Error("expected error for methodNone, got nil")
		}
	})
}
