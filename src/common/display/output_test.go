package display

import "testing"

func TestDetectDisplayModeForDumbTerminal(t *testing.T) {
	if got := detectDisplayMode(DisplayEnv{TerminalType: "dumb", HasDisplay: true}); got != DisplayModeCLI {
		t.Fatalf("detectDisplayMode() = %v, want CLI when TERM=dumb", got)
	}
	if got := detectDisplayMode(DisplayEnv{TerminalType: "dumb"}); got != DisplayModeCLI {
		t.Fatalf("detectDisplayMode() = %v, want CLI rather than headless when TERM=dumb", got)
	}
}

func TestColorEnabledPriority(t *testing.T) {
	forceOn := true
	forceOff := false
	configOn := true
	configOff := false
	tests := []struct {
		name       string
		force      *bool
		configured *bool
		noColor    string
		terminal   bool
		term       string
		want       bool
	}{
		{"TERM dumb disables even CLI override", &forceOn, &configOff, "1", false, "dumb", false},
		{"CLI off overrides config", &forceOff, &configOn, "", true, "xterm", false},
		{"config on cannot override TERM dumb", nil, &configOn, "1", false, "dumb", false},
		{"config off overrides terminal", nil, &configOff, "", true, "xterm", false},
		{"nonempty NO_COLOR disables", nil, nil, "anything", true, "xterm", false},
		{"TERM dumb disables", nil, nil, "", true, "dumb", false},
		{"nonterminal disables", nil, nil, "", false, "xterm", false},
		{"supported terminal enables", nil, nil, "", true, "xterm-256color", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := colorEnabled(test.force, test.configured, test.noColor, test.terminal, test.term)
			if got != test.want {
				t.Errorf("colorEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEmojiEnabledPriority(t *testing.T) {
	forceOn := true
	forceOff := false
	tests := []struct {
		name       string
		configured *bool
		noColor    string
		term       string
		want       bool
	}{
		{"TERM dumb disables configured emoji", &forceOn, "1", "dumb", false},
		{"configured false follows NO_COLOR", &forceOff, "1", "xterm", false},
		{"NO_COLOR disables", nil, "1", "xterm", false},
		{"TERM dumb disables", nil, "", "dumb", false},
		{"default enables", nil, "", "xterm", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := emojiEnabled(test.configured, test.noColor, test.term)
			if got != test.want {
				t.Errorf("emojiEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}
