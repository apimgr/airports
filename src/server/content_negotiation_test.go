package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderTemplateNegotiatesFrontendFormat(t *testing.T) {
	tests := []struct {
		name        string
		userAgent   string
		accept      string
		contentType string
		contains    string
		omits       string
	}{
		{name: "browser", userAgent: "Mozilla/5.0", accept: "text/html", contentType: "text/html", contains: "<!doctype html"},
		{name: "first party CLI", userAgent: "airports-cli/1.0", contentType: "application/json", contains: `"Title":"Airport"`, omits: "CSRFToken"},
		{name: "HTTP tool", userAgent: "curl/8.0", contentType: "text/plain", contains: "Airport"},
		{name: "text browser", userAgent: "Lynx/2.9", contentType: "text/html", contains: "Airport"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("User-Agent", tt.userAgent)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}
			w := httptest.NewRecorder()
			s.renderTemplate(w, r, "home", map[string]interface{}{"Title": "Airport"})
			if got := w.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("Content-Type = %q, want to contain %q", got, tt.contentType)
			}
			if !strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(tt.contains)) {
				t.Fatalf("response missing %q: %s", tt.contains, w.Body.String())
			}
			if tt.omits != "" && strings.Contains(w.Body.String(), tt.omits) {
				t.Fatalf("response unexpectedly contains %q", tt.omits)
			}
			if tt.name == "text browser" && strings.Contains(w.Body.String(), "<script") {
				t.Fatalf("no-JS response contains script: %s", w.Body.String())
			}
		})
	}
}

func TestRenderTemplatePrefixesURLsForBaseURL(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest("GET", "/server/help", nil)
	r.RemoteAddr = "198.51.100.10:443"
	r.Header.Set("User-Agent", "Mozilla/5.0")
	r.Header.Set("Accept", "text/html")
	s.config.Server.BaseURL = "/myproject"
	var response *httptest.ResponseRecorder
	handler := TrustedBaseURLMiddleware(s.config.Server.BaseURL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response = httptest.NewRecorder()
		s.renderTemplate(response, r, "home", map[string]interface{}{"Title": "Airport"})
	}))
	handler.ServeHTTP(httptest.NewRecorder(), r)
	body := response.Body.String()
	for _, want := range []string{
		`data-api-base="/myproject/api/v1"`,
		`href="/myproject/static/css/common.css"`,
		`href="/myproject/airports/search"`,
		`href="/myproject/stats"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("base URL rendered page missing %q", want)
		}
	}
}

func TestGeneratedResourcesHonorBaseURL(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.BaseURL = "/myproject"
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    []string
	}{
		{name: "manifest", handler: s.handleManifest, want: []string{`"start_url": "/myproject/"`, `"scope": "/myproject/"`, `"src": "/myproject/static/icons/icon-192.png"`}},
		{name: "service worker", handler: s.handleServiceWorker, want: []string{`const BASE_URL = "/myproject";`, `BASE_URL + '/static/css/common.css'`, `BASE_URL + '/static/offline.html'`}},
		{name: "API discovery", handler: s.handleAPIInfo, want: []string{`/myproject/api/v1/airports`, `/myproject/api/autodiscover`, `/myproject/server/docs/swagger`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			request.RemoteAddr = "198.51.100.10:443"
			response := httptest.NewRecorder()
			handler := TrustedBaseURLMiddleware(s.config.Server.BaseURL)(tc.handler)
			handler.ServeHTTP(response, request)
			for _, want := range tc.want {
				if !strings.Contains(response.Body.String(), want) {
					t.Errorf("response missing %q", want)
				}
			}
		})
	}
	routes := []struct {
		path string
		want []string
	}{
		{path: "/server/docs/swagger", want: []string{`data-spec-url="/myproject/api/swagger"`, `href="/myproject/static/css/common.css"`, `href="/myproject/server/docs/graphql"`}},
		{path: "/server/docs/graphql", want: []string{`data-endpoint-url="/myproject/api/graphql"`, `src="/myproject/server/docs/graphql/assets/react.production.min.js"`, `href="/myproject/server/docs/swagger"`}},
		{path: "/api/swagger", want: []string{`"url": "/myproject/api/v1"`}},
	}
	for _, tc := range routes {
		t.Run(tc.path, func(t *testing.T) {
			request := httptest.NewRequest("GET", tc.path, nil)
			request.RemoteAddr = "10.0.0.5:443"
			request.Header.Set("X-Forwarded-Prefix", "/myproject")
			response := httptest.NewRecorder()
			s.Router().ServeHTTP(response, request)
			for _, want := range tc.want {
				if !strings.Contains(response.Body.String(), want) {
					t.Errorf("response missing %q", want)
				}
			}
		})
	}
}
