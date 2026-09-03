package graphql

import (
	"strings"
	"testing"
)

// TestRenderGraphiQLHTML covers the page template directly, including the
// boundary case of an empty assets prefix (asset URLs collapse to bare
// filenames rather than panicking or producing malformed markup).
func TestRenderGraphiQLHTML(t *testing.T) {
	tests := []struct {
		name         string
		endpointURL  string
		assetsPrefix string
	}{
		{"typical values", "/api/graphql", "/server/docs/graphql/assets/"},
		{"empty assets prefix", "/api/graphql", ""},
		{"empty endpoint", "", "/assets/"},
		{"endpoint with quotes is escaped", `/api/graphql?x="y"`, "/assets/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := renderGraphiQLHTML(tt.endpointURL, tt.assetsPrefix)
			if !strings.HasPrefix(html, "<!DOCTYPE html>") {
				t.Error("output does not start with a DOCTYPE declaration")
			}
			if !strings.Contains(html, tt.assetsPrefix+"graphiql.min.css") {
				t.Errorf("output missing stylesheet link for prefix %q", tt.assetsPrefix)
			}
			if !strings.Contains(html, tt.assetsPrefix+"graphiql.min.js") {
				t.Error("output missing GraphiQL bundle script tag")
			}
			if !strings.Contains(html, `data-endpoint-url=`) {
				t.Error("output missing data-endpoint-url attribute for app.js bootstrap")
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

// TestRenderGraphiQLHTML_Idempotent verifies the render function is pure —
// same inputs always produce the same output.
func TestRenderGraphiQLHTML_Idempotent(t *testing.T) {
	a := renderGraphiQLHTML("/api/graphql", "/assets/")
	b := renderGraphiQLHTML("/api/graphql", "/assets/")
	if a != b {
		t.Error("renderGraphiQLHTML is not deterministic for identical inputs")
	}
}
