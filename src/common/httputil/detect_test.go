package httputil

import (
	"net/http/httptest"
	"testing"
)

func TestClientDetection(t *testing.T) {
	tests := []struct {
		name               string
		userAgent          string
		wantCLI            bool
		wantText           bool
		wantHTTP           bool
		wantNonInteractive bool
	}{
		{name: "our client", userAgent: "airports-cli/1.0", wantCLI: true},
		{name: "text browser", userAgent: "Lynx/2.9", wantText: true},
		{name: "curl", userAgent: "curl/8.0", wantHTTP: true, wantNonInteractive: true},
		{name: "empty user agent", wantHTTP: true, wantNonInteractive: true},
		{name: "regular browser", userAgent: "Mozilla/5.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://example.test/", nil)
			r.Header.Set("User-Agent", tt.userAgent)
			if IsOurCLIClient(r) != tt.wantCLI || IsTextBrowser(r) != tt.wantText ||
				IsHTTPTool(r) != tt.wantHTTP || IsNonInteractiveClient(r) != tt.wantNonInteractive {
				t.Fatalf("unexpected classification for %q", tt.userAgent)
			}
		})
	}
}

func TestResponseFormatDetection(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		userAgent    string
		accept       string
		frontendWant string
		apiWant      string
	}{
		{name: "browser", path: "/", userAgent: "Mozilla/5.0", accept: "text/html", frontendWant: "html", apiWant: "json"},
		{name: "our CLI", path: "/", userAgent: "airports-cli/1.0", frontendWant: "json", apiWant: "json"},
		{name: "text browser", path: "/", userAgent: "Lynx/2.9", frontendWant: "nojs", apiWant: "json"},
		{name: "curl", path: "/", userAgent: "curl/8.0", frontendWant: "text", apiWant: "text"},
		{name: "plain text accept", path: "/", userAgent: "Mozilla/5.0", accept: "text/plain", frontendWant: "text", apiWant: "text"},
		{name: "JSON accept overrides CLI detection", path: "/api/v1/items", userAgent: "curl/8.0", accept: "application/json", frontendWant: "text", apiWant: "json"},
		{name: "txt suffix overrides html accept", path: "/page.txt", userAgent: "Mozilla/5.0", accept: "text/html", frontendWant: "text", apiWant: "text"},
		{name: "empty agent", path: "/", userAgent: "", frontendWant: "text", apiWant: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://example.test"+tt.path, nil)
			r.Header.Set("User-Agent", tt.userAgent)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}
			if got := GetFrontendResponseFormat(r); got != tt.frontendWant {
				t.Errorf("frontend format = %q, want %q", got, tt.frontendWant)
			}
			if got := GetAPIResponseFormat(r); got != tt.apiWant {
				t.Errorf("API format = %q, want %q", got, tt.apiWant)
			}
		})
	}
}
