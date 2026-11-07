package tui

import (
	"github.com/apimgr/airports/src/common/theme"
	"github.com/charmbracelet/lipgloss"
)

type themeStyles struct {
	app         lipgloss.Style
	heading     lipgloss.Style
	selected    lipgloss.Style
	muted       lipgloss.Style
	error       lipgloss.Style
	input       lipgloss.Style
	borderColor lipgloss.Color
	border      lipgloss.Border
}

func stylesForTheme(name string, useColor bool) themeStyles {
	palette := theme.ThemePaletteDark
	if name == "light" {
		palette = theme.ThemePaletteLight
	}
	styles := themeStyles{
		app:      lipgloss.NewStyle(),
		heading:  lipgloss.NewStyle().Bold(true),
		selected: lipgloss.NewStyle().Bold(true).Padding(0, 1),
		muted:    lipgloss.NewStyle(),
		error:    lipgloss.NewStyle(),
		input:    lipgloss.NewStyle(),
		border:   lipgloss.RoundedBorder(),
	}
	if useColor {
		styles.app = styles.app.Foreground(lipgloss.Color(palette.Foreground)).Background(lipgloss.Color(palette.Background))
		styles.heading = styles.heading.Foreground(lipgloss.Color(palette.Primary))
		styles.selected = styles.selected.Foreground(lipgloss.Color(palette.Background)).Background(lipgloss.Color(palette.Info))
		styles.muted = styles.muted.Foreground(lipgloss.Color(palette.Muted))
		styles.error = styles.error.Foreground(lipgloss.Color(palette.Error))
		styles.input = styles.input.Foreground(lipgloss.Color(palette.Foreground)).BorderForeground(lipgloss.Color(palette.Border))
		styles.borderColor = lipgloss.Color(palette.Border)
	}
	return styles
}
