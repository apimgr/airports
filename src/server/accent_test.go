package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestAccentCSSUsesContrastAdjustedThemeColors(t *testing.T) {
	base, ok := parseAccentColor("#777777")
	if !ok {
		t.Fatal("parseAccentColor() rejected valid color")
	}
	darkBackground := accentColor{r: 40, g: 42, b: 54}
	lightBackground := accentColor{r: 255, g: 255, b: 255}
	dark := accentForBackground(base, darkBackground, true)
	light := accentForBackground(base, lightBackground, false)
	if accentContrast(dark, darkBackground) < 4.5 {
		t.Errorf("dark theme contrast = %.2f, want at least 4.5", accentContrast(dark, darkBackground))
	}
	if accentContrast(light, lightBackground) < 4.5 {
		t.Errorf("light theme contrast = %.2f, want at least 4.5", accentContrast(light, lightBackground))
	}

	server := &Server{config: config.DefaultApplicationConfig()}
	server.config.Web.UI.Accent = "#777777"
	response := httptest.NewRecorder()
	server.handleAccentCSS(response, httptest.NewRequest("GET", "/static/css/accent.css", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "html.theme-light") || !strings.Contains(response.Body.String(), "prefers-color-scheme: light") {
		t.Fatalf("accent CSS response = (%d, %s)", response.Code, response.Body.String())
	}
}

func TestParseAccentColorRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "#fff", "fff", "#12345g", "##123456", "#12345678"} {
		if _, ok := parseAccentColor(value); ok {
			t.Errorf("parseAccentColor(%q) accepted invalid value", value)
		}
	}
}
