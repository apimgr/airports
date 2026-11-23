package server

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParsePreferenceImportFormsAndOverrides(t *testing.T) {
	query := "lang=fr&theme=dark"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(query))
	cases := []struct {
		name string
		code string
	}{
		{name: "query", code: query},
		{name: "padded base64url", code: base64.URLEncoding.EncodeToString([]byte(query))},
		{name: "unpadded base64url", code: encoded},
		{name: "full URL", code: "https://example.test/server/preferences/sync/import?" + query},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := parsePreferenceImport(url.Values{"theme": {"light"}}, tc.code)
			if values.Get("theme") != "light" || values.Get("lang") != "fr" {
				t.Fatalf("parsed preferences = %v", values)
			}
		})
	}
	if values := parsePreferenceImport(nil, strings.Repeat("x", 2049)); len(values) != 0 {
		t.Fatalf("oversized code was accepted: %v", values)
	}
}

func TestPreferenceRedirectRejectsExternalTargets(t *testing.T) {
	for _, target := range []string{"//evil.test", `/\evil.test`, "https://evil.test/path", "/path\r\nX-Injected: yes"} {
		if got := safePreferenceRedirect(target); got != "" {
			t.Errorf("safePreferenceRedirect(%q) = %q", target, got)
		}
	}
	if got := safePreferenceRedirect("/server/preferences/sync?ok=1"); got != "/server/preferences/sync?ok=1" {
		t.Fatalf("relative redirect = %q", got)
	}
}

func TestPreferenceRedirectFallsThroughExcludedTargetToSameOriginReferer(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader("redirect=%2Fserver%2Fpreferences%3Fsaved%3D1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", "https://localhost/server/about?section=project")
	r.Host = "localhost"
	r.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	s.preferenceRedirect(w, r, "/")
	if got := w.Header().Get("Location"); got != "/server/about?section=project" {
		t.Fatalf("redirect after excluded target = %q", got)
	}
}

func TestPreferenceSaveDropsInvalidAndWritesConsent(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Privacy.Data.Sold = true
	r := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader("theme=invalid&lang=fr-CA&consent_form=1&consent_preferences=true&ccpa_opt_out=true"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	s.handleServerPreferencesSave(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("save response = %d, cache-control %q", w.Code, w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Location") != "/" {
		t.Fatalf("redirect = %q", w.Header().Get("Location"))
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookies = %#v, want lang, consent, CCPA", cookies)
	}
	for _, cookie := range cookies {
		if !cookie.Secure || cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie attributes for %q = %#v", cookie.Name, cookie)
		}
		if cookie.Name == "lang" && cookie.Value != "fr" {
			t.Errorf("regional language did not fall back to base tag: %q", cookie.Value)
		}
	}
}

func TestPreferenceSyncExportAndImport(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/preferences/sync", nil)
	r.Host = "airports.example.test"
	r.Header.Set("Accept", "text/html")
	r.Header.Set("User-Agent", "Mozilla/5.0")
	r.TLS = &tls.ConnectionState{}
	r.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	r.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	r.AddCookie(&http.Cookie{Name: "cookie_consent", Value: "not-exported"})
	w := httptest.NewRecorder()
	s.handleServerPreferencesSync(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "https://localhost/server/preferences/sync/import?lang=fr&amp;theme=dark") {
		t.Fatalf("sync export status/body = %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "not-exported") {
		t.Fatal("consent cookie appeared in export")
	}

	query := url.Values{"theme": {"light"}, "lang": {"de"}, "cookie_consent": {"forbidden"}}
	r = httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/sync/import?"+query.Encode(), nil)
	r.Header.Set("Accept", "application/json")
	w = httptest.NewRecorder()
	s.handleServerPreferencesImportAPI(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"theme":"light"`) || !strings.Contains(w.Body.String(), `"lang":"de"`) || !strings.Contains(w.Body.String(), `"cookie_consent"`) {
		t.Fatalf("sync import status/body = %d %s", w.Code, w.Body.String())
	}
}
