package display

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// DetectDisplayEnv detects terminal, remote-session, and platform display state.
func DetectDisplayEnv() DisplayEnv {
	env := DisplayEnv{TerminalType: os.Getenv("TERM")}
	env.IsTerminal = term.IsTerminal(int(os.Stdout.Fd()))
	if env.IsTerminal {
		env.Cols, env.Rows, _ = term.GetSize(int(os.Stdout.Fd()))
	}
	env.IsSSH = os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""
	env.IsMosh = os.Getenv("MOSH") != "" || strings.Contains(strings.ToLower(env.TerminalType), "mosh")
	env.IsScreen = os.Getenv("STY") != "" || os.Getenv("TMUX") != ""
	env.detectPlatformDisplay()

	env.Mode = detectDisplayMode(env)
	return env
}

func detectDisplayMode(env DisplayEnv) DisplayMode {
	switch {
	case env.IsDumbTerminal():
		return DisplayModeCLI
	case !env.IsTerminal && !env.HasDisplay:
		return DisplayModeHeadless
	case env.HasDisplay && !env.IsSSH && !env.IsMosh:
		return DisplayModeGUI
	case env.IsTerminal:
		return DisplayModeTUI
	default:
		return DisplayModeCLI
	}
}

// CanUseANSI reports whether cursor and color sequences are safe for stdout.
func CanUseANSI(env DisplayEnv) bool {
	return ColorEnabledFor(nil, nil, os.Getenv("NO_COLOR"), env)
}
