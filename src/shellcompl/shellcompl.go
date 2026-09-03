// Package shellcompl implements the shared --shell completions / --shell init
// / --shell help universal flag per AI.md PART 32 "Shell Completions
// (Built-in, NON-NEGOTIABLE)". Both the airports server binary and the
// airports-cli client binary call into this package so completion scripts
// are generated identically and stay in sync with the actual binary name.
package shellcompl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract scans argv (before flag.Parse) for a --shell flag, since Go's flag
// package has no notion of an optional multi-value flag. It mirrors the
// extractUpdateFlag/extractConfigFlag pattern already used elsewhere in this
// project. Recognized forms: --shell completions, --shell completions bash,
// --shell init, --shell init zsh, --shell help, --shell --help.
//
// Returns found=true plus the subcommand ("completions", "init", or "help")
// and an optional shell name (empty means auto-detect from $SHELL). The
// remaining argv (with the consumed tokens stripped) is returned so callers
// can still hand the rest to flag.CommandLine.Parse().
func Extract(argv []string) (found bool, subcmd string, shell string, remaining []string) {
	remaining = make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg != "--shell" && arg != "-shell" {
			remaining = append(remaining, arg)
			continue
		}

		found = true
		if i+1 < len(argv) {
			next := argv[i+1]
			switch next {
			case "completions", "init", "help", "--help", "-h":
				if next == "--help" || next == "-h" {
					subcmd = "help"
				} else {
					subcmd = next
				}
				i++
				if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					shell = argv[i+1]
					i++
				}
			}
		}
		if subcmd == "" {
			subcmd = "help"
		}
	}
	return found, subcmd, shell, remaining
}

// DetectShell extracts the shell name from the $SHELL environment variable
// (e.g. "/bin/bash" -> "bash"), defaulting to "bash" when $SHELL is unset.
func DetectShell() string {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "bash"
	}
	return filepath.Base(shellPath)
}

// Handle implements the full --shell completions|init|help [SHELL] behavior
// and writes to w (normally os.Stdout for completions/init, os.Stderr for
// usage errors). commands and flags describe the binary's own command/flag
// surface and are embedded in the generated completion scripts.
func Handle(w io.Writer, errW io.Writer, subcmd, shell, binaryName string, commands []string, flags []string) int {
	if shell == "" {
		shell = DetectShell()
	}

	switch subcmd {
	case "completions":
		script, err := GenerateCompletions(shell, binaryName, commands, flags)
		if err != nil {
			fmt.Fprintln(errW, err.Error())
			return 1
		}
		fmt.Fprint(w, script)
		return 0
	case "init":
		script, err := GenerateInit(shell, binaryName)
		if err != nil {
			fmt.Fprintln(errW, err.Error())
			return 1
		}
		fmt.Fprint(w, script)
		return 0
	case "help":
		fmt.Fprintf(w, "Shell integration for %s:\n", binaryName)
		fmt.Fprintln(w, "  --shell completions [SHELL]  Print shell completions")
		fmt.Fprintln(w, "  --shell init [SHELL]         Print shell init command")
		fmt.Fprintln(w, "  SHELL: bash, zsh, fish, sh, dash, ksh, powershell, pwsh (auto-detect if omitted)")
		return 0
	default:
		fmt.Fprintln(errW, "Usage: --shell [completions|init|help] [SHELL]")
		return 1
	}
}

func unsupportedShellErr(shell string) error {
	return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, sh, dash, ksh, powershell, pwsh)", shell)
}

// GenerateInit returns the eval-ready command that sources the completions
// output for the given shell.
func GenerateInit(shell, binaryName string) (string, error) {
	switch shell {
	case "bash":
		return fmt.Sprintf("source <(%s --shell completions bash)\n", binaryName), nil
	case "zsh":
		return fmt.Sprintf("source <(%s --shell completions zsh)\n", binaryName), nil
	case "fish":
		return fmt.Sprintf("%s --shell completions fish | source\n", binaryName), nil
	case "sh", "dash", "ksh":
		return fmt.Sprintf("eval \"$(%s --shell completions %s)\"\n", binaryName, shell), nil
	case "powershell", "pwsh":
		return fmt.Sprintf("Invoke-Expression (& %s --shell completions powershell)\n", binaryName), nil
	default:
		return "", unsupportedShellErr(shell)
	}
}

// GenerateCompletions returns the completion script for the given shell.
func GenerateCompletions(shell, binaryName string, commands []string, flags []string) (string, error) {
	switch shell {
	case "bash":
		return generateBash(binaryName, commands, flags), nil
	case "zsh":
		return generateZsh(binaryName, commands, flags), nil
	case "fish":
		return generateFish(binaryName, commands, flags), nil
	case "sh", "dash", "ksh":
		return generatePosix(binaryName, commands, flags), nil
	case "powershell", "pwsh":
		return generatePowershell(binaryName, commands, flags), nil
	default:
		return "", unsupportedShellErr(shell)
	}
}

func wordList(commands, flags []string) string {
	return strings.Join(append(append([]string{}, commands...), flags...), " ")
}

func funcSafeName(binaryName string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(binaryName)
}

func generateBash(binaryName string, commands, flags []string) string {
	fn := "_" + funcSafeName(binaryName) + "_completions"
	words := wordList(commands, flags)
	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n", binaryName)
	fmt.Fprintf(&b, "%s() {\n", fn)
	b.WriteString("    local cur\n")
	b.WriteString("    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(&b, "    COMPREPLY=($(compgen -W \"%s\" -- \"$cur\"))\n", words)
	b.WriteString("}\n")
	fmt.Fprintf(&b, "complete -F %s %s\n", fn, binaryName)
	return b.String()
}

func generateZsh(binaryName string, commands, flags []string) string {
	words := wordList(commands, flags)
	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n", binaryName)
	fmt.Fprintf(&b, "_%s() {\n", funcSafeName(binaryName))
	fmt.Fprintf(&b, "    local -a words\n    words=(%s)\n", words)
	b.WriteString("    _describe 'command' words\n")
	b.WriteString("}\n")
	fmt.Fprintf(&b, "compdef _%s %s\n", funcSafeName(binaryName), binaryName)
	return b.String()
}

func generateFish(binaryName string, commands, flags []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s\n", binaryName)
	for _, c := range commands {
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -a '%s'\n", binaryName, c)
	}
	for _, f := range flags {
		name := strings.TrimLeft(f, "-")
		fmt.Fprintf(&b, "complete -c %s -l '%s'\n", binaryName, name)
	}
	return b.String()
}

func generatePosix(binaryName string, commands, flags []string) string {
	words := wordList(commands, flags)
	var b strings.Builder
	fmt.Fprintf(&b, "# POSIX shell completion for %s (basic)\n", binaryName)
	fmt.Fprintf(&b, "# Supported words: %s\n", words)
	return b.String()
}

func generatePowershell(binaryName string, commands, flags []string) string {
	words := wordList(commands, flags)
	quoted := make([]string, 0, len(commands)+len(flags))
	for _, w := range strings.Fields(words) {
		quoted = append(quoted, "'"+w+"'")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# PowerShell completion for %s\n", binaryName)
	fmt.Fprintf(&b, "Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {\n", binaryName)
	b.WriteString("    param($wordToComplete, $commandAst, $cursorPosition)\n")
	fmt.Fprintf(&b, "    @(%s) | Where-Object { $_ -like \"$wordToComplete*\" } | ForEach-Object {\n", strings.Join(quoted, ", "))
	b.WriteString("        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}
