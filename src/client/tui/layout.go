package tui

import "github.com/apimgr/airports/src/common/terminal"

type LayoutConfig struct {
	ShowBorders    bool
	ShowHeader     bool
	ShowFooter     bool
	ShowSidebar    bool
	SidebarWidth   int
	MaxColumns     int
	TruncateAt     int
	UseAbbrev      bool
	VerticalScroll bool
	MultiPane      bool
	TileLayout     bool
}

func GetLayoutConfig(mode terminal.SizeMode) LayoutConfig {
	switch mode {
	case terminal.SizeModeMicro:
		return LayoutConfig{MaxColumns: 2, TruncateAt: 30, UseAbbrev: true, VerticalScroll: true}
	case terminal.SizeModeMinimal:
		return LayoutConfig{ShowHeader: true, ShowFooter: true, MaxColumns: 3, TruncateAt: 40, UseAbbrev: true, VerticalScroll: true}
	case terminal.SizeModeCompact:
		return LayoutConfig{ShowBorders: true, ShowHeader: true, ShowFooter: true, MaxColumns: 4, TruncateAt: 60, VerticalScroll: true}
	case terminal.SizeModeWide:
		return LayoutConfig{ShowBorders: true, ShowHeader: true, ShowFooter: true, ShowSidebar: true, SidebarWidth: 30, MaxColumns: 8, TruncateAt: 120}
	case terminal.SizeModeUltrawide:
		return LayoutConfig{ShowBorders: true, ShowHeader: true, ShowFooter: true, ShowSidebar: true, SidebarWidth: 40, MaxColumns: 12, TruncateAt: 200, MultiPane: true}
	case terminal.SizeModeMassive:
		return LayoutConfig{ShowBorders: true, ShowHeader: true, ShowFooter: true, ShowSidebar: true, SidebarWidth: 40, MaxColumns: 12, TruncateAt: 200, MultiPane: true, TileLayout: true}
	default:
		return LayoutConfig{ShowBorders: true, ShowHeader: true, ShowFooter: true, MaxColumns: 6, TruncateAt: 80, VerticalScroll: true}
	}
}

func terminalSizeMode(width, height int) terminal.SizeMode {
	if width < 40 || height < 10 {
		return terminal.SizeModeMicro
	}
	if width < 60 || height < 16 {
		return terminal.SizeModeMinimal
	}
	if width < 80 || height < 24 {
		return terminal.SizeModeCompact
	}
	if width < 120 || height < 40 {
		return terminal.SizeModeStandard
	}
	if width < 200 || height < 60 {
		return terminal.SizeModeWide
	}
	if width < 400 || height < 80 {
		return terminal.SizeModeUltrawide
	}
	return terminal.SizeModeMassive
}

func truncate(value string, maxWidth int) string {
	runes := []rune(value)
	if maxWidth <= 0 || len(runes) <= maxWidth {
		return value
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

func truncateMiddle(value string, maxWidth int) string {
	runes := []rune(value)
	if maxWidth <= 0 || len(runes) <= maxWidth {
		return value
	}
	if maxWidth <= 5 {
		return string(runes[:maxWidth])
	}
	half := (maxWidth - 3) / 2
	return string(runes[:half]) + "..." + string(runes[len(runes)-half:])
}
