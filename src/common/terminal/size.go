// Package terminal provides terminal sizing and responsive breakpoints.
package terminal

import (
	"os"

	"golang.org/x/term"
)

// SizeMode identifies a terminal width and height class.
type SizeMode int

const (
	SizeModeMicro SizeMode = iota
	SizeModeMinimal
	SizeModeCompact
	SizeModeStandard
	SizeModeWide
	SizeModeUltrawide
	SizeModeMassive
)

// TerminalSize contains the terminal dimensions and their responsive class.
type TerminalSize struct {
	Cols int
	Rows int
	Mode SizeMode
}

// GetTerminalSize detects the stdout dimensions, using a standard fallback.
func GetTerminalSize() TerminalSize {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	return TerminalSize{Cols: cols, Rows: rows, Mode: calculateMode(cols, rows)}
}

func calculateMode(cols, rows int) SizeMode {
	switch {
	case cols < 40 || rows < 10:
		return SizeModeMicro
	case cols < 60 || rows < 16:
		return SizeModeMinimal
	case cols < 80 || rows < 24:
		return SizeModeCompact
	case cols < 120 || rows < 40:
		return SizeModeStandard
	case cols < 200 || rows < 60:
		return SizeModeWide
	case cols < 400 || rows < 80:
		return SizeModeUltrawide
	default:
		return SizeModeMassive
	}
}

// ShowASCIIArt reports whether the terminal is wide enough for larger text.
func (m SizeMode) ShowASCIIArt() bool { return m >= SizeModeStandard }

// ShowBorders reports whether the terminal is wide enough for box borders.
func (m SizeMode) ShowBorders() bool { return m >= SizeModeCompact }

// ShowSidebar reports whether the terminal is wide enough for a sidebar.
func (m SizeMode) ShowSidebar() bool { return m >= SizeModeWide }

// ShowIcons reports whether the terminal is wide enough for icon labels.
func (m SizeMode) ShowIcons() bool { return m >= SizeModeMinimal }
