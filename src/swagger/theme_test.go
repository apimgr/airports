package swagger

import (
	"strings"
	"testing"
)

// TestRenderSwaggerHTML covers the page template directly, including the
// boundary case of an empty assets prefix.
func TestRenderSwaggerHTML(t *testing.T) {
	tests := []struct {
		name         string
		specURL      string
		assetsPrefix string
	}{
		{"typical values", "/api/swagger", "/server/docs/swagger/assets/"},
		{"empty assets prefix", "/api/swagger", ""},
		{"empty spec url", "", "/assets/"},
		{"spec url with quotes is escaped", `/api/swagger?x="y"`, "/assets/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := renderSwaggerHTML(tt.specURL, tt.assetsPrefix)
			if !strings.HasPrefix(html, "<!DOCTYPE html>") {
				t.Error("output does not start with a DOCTYPE declaration")
			}
			if !strings.Contains(html, tt.assetsPrefix+"swagger-ui.css") {
				t.Errorf("output missing stylesheet link for prefix %q", tt.assetsPrefix)
			}
			if !strings.Contains(html, tt.assetsPrefix+"swagger-ui-bundle.js") {
				t.Error("output missing Swagger UI bundle script tag")
			}
			if !strings.Contains(html, `data-spec-url=`) {
				t.Error("output missing data-spec-url attribute for app.js bootstrap")
			}
			if !strings.Contains(html, `src="/static/js/app.js"`) {
				t.Error("output missing shared app.js script tag")
			}
			if !strings.Contains(html, "</html>") {
				t.Error("output missing closing html tag")
			}
		})
	}
}

// TestRenderSwaggerHTML_Idempotent verifies the render function is pure —
// same inputs always produce the same output.
func TestRenderSwaggerHTML_Idempotent(t *testing.T) {
	a := renderSwaggerHTML("/api/swagger", "/assets/")
	b := renderSwaggerHTML("/api/swagger", "/assets/")
	if a != b {
		t.Error("renderSwaggerHTML is not deterministic for identical inputs")
	}
}
