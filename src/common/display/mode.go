// Package display detects the environment available to a command-line binary.
package display

// DisplayMode is the available user interface mode.
type DisplayMode int

const (
	DisplayModeHeadless DisplayMode = iota
	DisplayModeCLI
	DisplayModeTUI
	DisplayModeGUI
)

// DisplayEnv describes the current terminal and graphical session.
type DisplayEnv struct {
	Mode         DisplayMode
	HasDisplay   bool
	DisplayType  string
	IsTerminal   bool
	IsSSH        bool
	IsMosh       bool
	IsScreen     bool
	TerminalType string
	Cols         int
	Rows         int
}

// IsDumbTerminal reports whether TERM disables terminal control sequences.
func (e DisplayEnv) IsDumbTerminal() bool { return e.TerminalType == "dumb" }

// IsAutoDetectDisplayModeGUI reports whether a native graphical session exists.
func (e DisplayEnv) IsAutoDetectDisplayModeGUI() bool { return e.Mode == DisplayModeGUI }

// IsAutoDetectDisplayModeTUI reports whether an interactive terminal exists.
func (e DisplayEnv) IsAutoDetectDisplayModeTUI() bool { return e.Mode == DisplayModeTUI }

// IsAutoDetectDisplayModeCLI reports whether output should remain line based.
func (e DisplayEnv) IsAutoDetectDisplayModeCLI() bool { return e.Mode == DisplayModeCLI }

// IsAutoDetectDisplayModeHeadless reports whether no display or terminal exists.
func (e DisplayEnv) IsAutoDetectDisplayModeHeadless() bool { return e.Mode == DisplayModeHeadless }
