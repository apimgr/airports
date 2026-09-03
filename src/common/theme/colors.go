// Package theme defines the shared color palette used across all client
// surfaces (Web CSS, Swagger, GraphiQL today; future CLI/TUI/GUI clients
// per AI.md PART 32).
package theme

// ThemePalette is the single source of truth for the app's color scheme,
// per AI.md PART 16 "Themes (NON-NEGOTIABLE - PROJECT-WIDE)" ->
// "Unified Color Palette". Colors are defined ONCE here as the literal hex
// source of truth; src/server/static/css/common.css's --color-* custom
// properties and src/swagger/theme.go mirror these same values and must be
// kept in sync manually.
type ThemePalette struct {
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Success    string `json:"success"`
	Warning    string `json:"warning"`
	Error      string `json:"error"`
	Info       string `json:"info"`
	Surface    string `json:"surface"`
	SurfaceAlt string `json:"surface_alt"`
	Border     string `json:"border"`
	Muted      string `json:"muted"`
}

// ThemePaletteDark is the Dracula-based dark palette (AI.md's literal spec).
var ThemePaletteDark = ThemePalette{
	Background: "#282a36",
	Foreground: "#f8f8f2",
	Primary:    "#bd93f9",
	Secondary:  "#50fa7b",
	Accent:     "#ff79c6",
	Success:    "#50fa7b",
	Warning:    "#ffb86c",
	Error:      "#ff5555",
	Info:       "#8be9fd",
	Surface:    "#2b2d3a",
	SurfaceAlt: "#21222c",
	Border:     "#44475a",
	Muted:      "#6272a4",
}

// ThemePaletteLight is the GitHub-Light-based light palette (AI.md's
// literal spec).
var ThemePaletteLight = ThemePalette{
	Background: "#ffffff",
	Foreground: "#1f2328",
	Primary:    "#0969da",
	Secondary:  "#1a7f37",
	Accent:     "#8250df",
	Success:    "#1a7f37",
	Warning:    "#9a6700",
	Error:      "#d1242f",
	Info:       "#0969da",
	Surface:    "#f6f8fa",
	SurfaceAlt: "#eff2f5",
	Border:     "#d1d9e0",
	Muted:      "#59636e",
}
