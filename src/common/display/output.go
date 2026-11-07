package display

import (
	"os"

	"golang.org/x/term"
)

// ColorEnabled applies CLI, config, NO_COLOR, and terminal detection priority.
func ColorEnabled(forceColor, configColor *bool) bool {
	env := DisplayEnv{IsTerminal: term.IsTerminal(int(os.Stdout.Fd())), TerminalType: os.Getenv("TERM")}
	return ColorEnabledFor(forceColor, configColor, os.Getenv("NO_COLOR"), env)
}

// EmojiEnabled applies the output.emoji override, NO_COLOR, and TERM priority.
// TERM=dumb always disables emojis, even when config requests them.
func EmojiEnabled(configEmoji *bool) bool {
	return EmojiEnabledFor(configEmoji, os.Getenv("NO_COLOR"), os.Getenv("TERM"))
}

// ColorEnabledFor applies color priority against an explicit display snapshot.
func ColorEnabledFor(forceColor, configColor *bool, noColor string, env DisplayEnv) bool {
	return colorEnabled(forceColor, configColor, noColor, env.IsTerminal, env.TerminalType)
}

// EmojiEnabledFor applies emoji priority against explicit environment values.
func EmojiEnabledFor(configEmoji *bool, noColor, terminalType string) bool {
	return emojiEnabled(configEmoji, noColor, terminalType)
}

func colorEnabled(forceColor, configColor *bool, noColor string, isTerminal bool, terminalType string) bool {
	if terminalType == "dumb" {
		return false
	}
	if forceColor != nil {
		return *forceColor
	}
	if configColor != nil {
		return *configColor
	}
	if noColor != "" || !isTerminal {
		return false
	}
	return true
}

func emojiEnabled(configEmoji *bool, noColor, terminalType string) bool {
	if terminalType == "dumb" {
		return false
	}
	if configEmoji != nil && *configEmoji {
		return true
	}
	return noColor == ""
}
