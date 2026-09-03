package shellcompl

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Covers Extract across all recognized forms plus the pass-through cases:
// no --shell flag at all, --shell with no following args, --shell with a
// subcommand but no shell name, and non-flag args being preserved in order.
func TestExtract(t *testing.T) {
	tests := []struct {
		name          string
		argv          []string
		wantFound     bool
		wantSubcmd    string
		wantShell     string
		wantRemaining []string
	}{
		{"absent", []string{"search", "kennedy"}, false, "", "", []string{"search", "kennedy"}},
		{"completions-no-shell", []string{"--shell", "completions"}, true, "completions", "", []string{}},
		{"completions-with-shell", []string{"--shell", "completions", "bash"}, true, "completions", "bash", []string{}},
		{"init-with-shell", []string{"--shell", "init", "zsh"}, true, "init", "zsh", []string{}},
		{"init-no-shell", []string{"--shell", "init"}, true, "init", "", []string{}},
		{"help", []string{"--shell", "help"}, true, "help", "", []string{}},
		{"help-via---help", []string{"--shell", "--help"}, true, "help", "", []string{}},
		{"help-via--h", []string{"--shell", "-h"}, true, "help", "", []string{}},
		{"single-dash-shell", []string{"-shell", "completions", "fish"}, true, "completions", "fish", []string{}},
		{"shell-flag-with-nothing-after", []string{"--shell"}, true, "help", "", []string{}},
		{"shell-flag-unrecognized-next", []string{"--shell", "banana"}, true, "help", "", []string{"banana"}},
		{"remaining-preserved-around-flag", []string{"before", "--shell", "completions", "bash", "after"}, true, "completions", "bash", []string{"before", "after"}},
		{"shell-name-not-consumed-if-flag-like", []string{"--shell", "completions", "--debug"}, true, "completions", "", []string{"--debug"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, subcmd, shell, remaining := Extract(tt.argv)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if subcmd != tt.wantSubcmd {
				t.Errorf("subcmd = %q, want %q", subcmd, tt.wantSubcmd)
			}
			if shell != tt.wantShell {
				t.Errorf("shell = %q, want %q", shell, tt.wantShell)
			}
			if len(remaining) != len(tt.wantRemaining) {
				t.Fatalf("remaining = %v, want %v", remaining, tt.wantRemaining)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemaining[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemaining[i])
				}
			}
		})
	}
}

// Covers DetectShell: $SHELL set to a full path vs unset (defaults to bash).
func TestDetectShell(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		os.Setenv("SHELL", "/usr/bin/zsh")
		defer os.Unsetenv("SHELL")
		if got := DetectShell(); got != "zsh" {
			t.Errorf("DetectShell() = %q, want zsh", got)
		}
	})
	t.Run("unset-defaults-bash", func(t *testing.T) {
		os.Unsetenv("SHELL")
		if got := DetectShell(); got != "bash" {
			t.Errorf("DetectShell() = %q, want bash", got)
		}
	})
}

// Covers Handle's four subcommand branches (completions, init, help,
// default/unknown) including the error paths for unsupported shells, and the
// auto-detect-shell-when-empty behavior.
func TestHandle(t *testing.T) {
	commands := []string{"search", "get"}
	flags := []string{"--debug", "--config"}

	t.Run("completions-ok", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "completions", "bash", "airports-cli", commands, flags)
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
		if !strings.Contains(out.String(), "airports-cli") {
			t.Error("output missing binary name")
		}
	})

	t.Run("completions-unsupported-shell", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "completions", "tcsh", "airports-cli", commands, flags)
		if code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if !strings.Contains(errOut.String(), "unsupported shell") {
			t.Errorf("errOut = %q, want unsupported shell message", errOut.String())
		}
	})

	t.Run("init-ok", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "init", "zsh", "airports-cli", commands, flags)
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
		if !strings.Contains(out.String(), "airports-cli") {
			t.Error("output missing binary name")
		}
	})

	t.Run("init-unsupported-shell", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "init", "tcsh", "airports-cli", commands, flags)
		if code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if !strings.Contains(errOut.String(), "unsupported shell") {
			t.Errorf("errOut = %q, want unsupported shell message", errOut.String())
		}
	})

	t.Run("help", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "help", "", "airports-cli", commands, flags)
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
		if !strings.Contains(out.String(), "airports-cli") {
			t.Error("help output missing binary name")
		}
	})

	t.Run("unknown-subcommand", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "frobnicate", "", "airports-cli", commands, flags)
		if code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if !strings.Contains(errOut.String(), "Usage") {
			t.Errorf("errOut = %q, want usage message", errOut.String())
		}
	})

	t.Run("empty-shell-auto-detects", func(t *testing.T) {
		os.Setenv("SHELL", "/bin/bash")
		defer os.Unsetenv("SHELL")
		var out, errOut bytes.Buffer
		code := Handle(&out, &errOut, "completions", "", "airports-cli", commands, flags)
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
		if !strings.Contains(out.String(), "bash completion") {
			t.Errorf("output = %q, want bash completion script", out.String())
		}
	})
}

// Covers GenerateInit for every supported shell plus the unsupported-shell
// error path.
func TestGenerateInit(t *testing.T) {
	tests := []struct {
		shell   string
		wantErr bool
		wantSub string
	}{
		{"bash", false, "source <(airports-cli --shell completions bash)"},
		{"zsh", false, "source <(airports-cli --shell completions zsh)"},
		{"fish", false, "airports-cli --shell completions fish | source"},
		{"sh", false, "eval \"$(airports-cli --shell completions sh)\""},
		{"dash", false, "eval \"$(airports-cli --shell completions dash)\""},
		{"ksh", false, "eval \"$(airports-cli --shell completions ksh)\""},
		{"powershell", false, "Invoke-Expression (& airports-cli --shell completions powershell)"},
		{"pwsh", false, "Invoke-Expression (& airports-cli --shell completions powershell)"},
		{"tcsh", true, ""},
		{"", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := GenerateInit(tt.shell, "airports-cli")
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateInit(%q) error = %v, wantErr %v", tt.shell, err, tt.wantErr)
			}
			if !tt.wantErr && !strings.Contains(got, tt.wantSub) {
				t.Errorf("GenerateInit(%q) = %q, want to contain %q", tt.shell, got, tt.wantSub)
			}
		})
	}
}

// Covers GenerateCompletions dispatch to each per-shell generator plus the
// unsupported-shell error path, verifying key structural markers in each
// generated script (golden-string style, without over-fitting exact bytes).
func TestGenerateCompletions(t *testing.T) {
	commands := []string{"search", "get", "nearby"}
	flags := []string{"--debug", "--config"}

	tests := []struct {
		shell   string
		wantErr bool
		wantSub []string
	}{
		{"bash", false, []string{"_airports_cli_completions()", "complete -F", "compgen -W"}},
		{"zsh", false, []string{"#compdef airports-cli", "_describe 'command'"}},
		{"fish", false, []string{"complete -c airports-cli -n '__fish_use_subcommand' -a 'search'", "complete -c airports-cli -l 'debug'"}},
		{"sh", false, []string{"POSIX shell completion", "search get nearby --debug --config"}},
		{"dash", false, []string{"POSIX shell completion"}},
		{"ksh", false, []string{"POSIX shell completion"}},
		{"powershell", false, []string{"Register-ArgumentCompleter", "'search'"}},
		{"pwsh", false, []string{"Register-ArgumentCompleter"}},
		{"tcsh", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := GenerateCompletions(tt.shell, "airports-cli", commands, flags)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateCompletions(%q) error = %v, wantErr %v", tt.shell, err, tt.wantErr)
			}
			for _, want := range tt.wantSub {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateCompletions(%q) = %q, want to contain %q", tt.shell, got, want)
				}
			}
		})
	}
}

// Covers funcSafeName's replacement of characters that are invalid in shell
// function/identifier names.
func TestFuncSafeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"airports-cli", "airports_cli"},
		{"airports.cli", "airports_cli"},
		{"airports-cli.exe", "airports_cli_exe"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		if got := funcSafeName(tt.in); got != tt.want {
			t.Errorf("funcSafeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Covers wordList joining commands and flags with a single space, including
// the empty-slice edge cases.
func TestWordList(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		flags    []string
		want     string
	}{
		{"both", []string{"search", "get"}, []string{"--debug"}, "search get --debug"},
		{"empty-commands", nil, []string{"--debug"}, "--debug"},
		{"empty-flags", []string{"search"}, nil, "search"},
		{"both-empty", nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wordList(tt.commands, tt.flags); got != tt.want {
				t.Errorf("wordList(%v, %v) = %q, want %q", tt.commands, tt.flags, got, tt.want)
			}
		})
	}
}

// Covers unsupportedShellErr's message content.
func TestUnsupportedShellErr(t *testing.T) {
	err := unsupportedShellErr("tcsh")
	if err == nil {
		t.Fatal("unsupportedShellErr returned nil")
	}
	if !strings.Contains(err.Error(), "tcsh") {
		t.Errorf("error = %q, want to mention tcsh", err.Error())
	}
	if !strings.Contains(err.Error(), "supported:") {
		t.Errorf("error = %q, want to list supported shells", err.Error())
	}
}
