package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// csrfCookieValue extracts the csrf_token cookie value set on w, failing
// the test if it is absent.
func csrfCookieValue(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "csrf_token" {
			if c.Value == "" {
				t.Fatalf("csrf_token cookie set with empty value")
			}
			return c.Value
		}
	}
	t.Fatalf("csrf_token cookie was not set")
	return ""
}

// TestCSRFMiddlewareIssuesCookieOnGet covers token issuance: a GET request
// with no existing cookie gets a fresh csrf_token cookie (SameSite=Strict,
// not HttpOnly) and is not blocked (AI.md PART 16 "Cookie Posture").
func TestCSRFMiddlewareIssuesCookieOnGet(t *testing.T) {
	s := newTestServer(t)
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		if csrfTokenFromContext(r.Context()) == "" {
			t.Errorf("csrfTokenFromContext: expected a non-empty token in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("GET request was blocked; want pass-through")
	}
	token := csrfCookieValue(t, w)

	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name != "csrf_token" {
			continue
		}
		found = true
		if c.SameSite != http.SameSiteStrictMode {
			t.Errorf("csrf_token SameSite = %v, want Strict", c.SameSite)
		}
		if c.HttpOnly {
			t.Errorf("csrf_token HttpOnly = true, want false (JS/forms must read it)")
		}
	}
	if !found {
		t.Fatalf("csrf_token cookie not found")
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
}

// TestCSRFMiddlewarePostMissingTokenRejected covers the reject path: a
// mutating request with a valid cookie but no header/form token is rejected
// with 403 CSRF_FAILED (AI.md PART 16 "no silent fallback").
func TestCSRFMiddlewarePostMissingTokenRejected(t *testing.T) {
	s := newTestServer(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be reached when the CSRF token is missing")
	})

	r := httptest.NewRequest(http.MethodPost, "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "existing-cookie-token"})
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if !strings.Contains(w.Body.String(), "CSRF_FAILED") {
		t.Errorf("body = %q, want it to contain CSRF_FAILED", w.Body.String())
	}
}

// TestCSRFMiddlewarePostMatchingTokenAccepted covers the accept path: a
// mutating request presenting the same token via the X-CSRF-Token header as
// the csrf_token cookie is allowed through.
func TestCSRFMiddlewarePostMatchingTokenAccepted(t *testing.T) {
	s := newTestServer(t)
	token := "test-token-value-0123456789abcdef"

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	r.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("matching CSRF token was rejected; want pass-through, status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestCSRFMiddlewarePostMismatchedTokenRejected covers the constant-time
// mismatch path: header token differs from cookie token -> 403 CSRF_FAILED
// with the canonical error envelope (AI.md PART 16 "Implementation Rules").
func TestCSRFMiddlewarePostMismatchedTokenRejected(t *testing.T) {
	s := newTestServer(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be reached on CSRF mismatch")
	})

	form := url.Values{"theme": {"dark"}}
	r := httptest.NewRequest(http.MethodPost, "/theme", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
	r.Header.Set("X-CSRF-Token", "different-token")
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if !strings.Contains(w.Body.String(), "CSRF_FAILED") {
		t.Errorf("body = %q, want it to contain CSRF_FAILED", w.Body.String())
	}
}

// TestCSRFMiddlewareFormFieldAccepted covers the form-field fallback: a
// mutating request submitting the token via the csrf_token form field
// (rather than the header) is accepted, matching the noscript-form flow.
func TestCSRFMiddlewareFormFieldAccepted(t *testing.T) {
	s := newTestServer(t)
	token := "form-field-token-abc123"

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	form := url.Values{"theme": {"dark"}, "csrf_token": {token}}
	r := httptest.NewRequest(http.MethodPost, "/theme", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("matching form-field CSRF token was rejected; want pass-through, status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestCSRFMiddlewareBearerBypassed covers the bearer-auth bypass: a
// mutating request with an Authorization: Bearer header skips CSRF
// validation entirely, even with no cookie/token at all (AI.md PART 16
// "Bearer credentials are not auto-attached by browsers").
func TestCSRFMiddlewareBearerBypassed(t *testing.T) {
	s := newTestServer(t)
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/something", nil)
	r.Header.Set("Authorization", "Bearer some-api-token")
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("bearer-authenticated request was blocked; want pass-through, status=%d", w.Code)
	}
}

// TestCSRFMiddlewareExemptPathBypassed covers the operator-declared
// exempt_paths bypass (AI.md PART 16 "server.csrf.exempt_paths").
func TestCSRFMiddlewareExemptPathBypassed(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.CSRF.ExemptPaths = []string{"/api/graphql"}

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/api/graphql", nil)
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("exempt path was blocked; want pass-through, status=%d", w.Code)
	}
}

// TestCSRFMiddlewareDisabledSkipsValidation covers server.csrf.enabled:
// false — CSRF is entirely bypassed and no cookie is set.
func TestCSRFMiddlewareDisabledSkipsValidation(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.CSRF.Enabled = false

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/theme", nil)
	w := httptest.NewRecorder()
	s.csrfMiddleware(next).ServeHTTP(w, r)

	if !handlerCalled {
		t.Fatalf("disabled CSRF should always pass through, status=%d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "csrf_token" {
			t.Errorf("csrf_token cookie should not be set when CSRF is disabled")
		}
	}
}

// TestGenerateCSRFTokenLength covers the token_length config knob and the
// zero/negative-length fallback to the 32-byte default.
func TestGenerateCSRFTokenLength(t *testing.T) {
	tok, err := generateCSRFToken(16)
	if err != nil {
		t.Fatalf("generateCSRFToken: unexpected error: %v", err)
	}
	if len(tok) != 32 { // hex-encoded: 2 chars per byte
		t.Errorf("len(token) = %d, want 32 for 16 random bytes", len(tok))
	}

	tok2, err := generateCSRFToken(0)
	if err != nil {
		t.Fatalf("generateCSRFToken(0): unexpected error: %v", err)
	}
	if len(tok2) != 64 { // falls back to 32 bytes -> 64 hex chars
		t.Errorf("len(token) = %d, want 64 for the zero-length fallback", len(tok2))
	}
}

// TestCSRFCookieSecureModes covers the auto/true/false server.csrf.secure
// config values.
func TestCSRFCookieSecureModes(t *testing.T) {
	httpReq := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	httpsReq := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	httpsReq.TLS = &tls.ConnectionState{}

	cases := []struct {
		mode string
		req  *http.Request
		want bool
	}{
		{"true", httpReq, true},
		{"false", httpsReq, false},
		{"auto", httpReq, false},
		{"auto", httpsReq, true},
	}
	for _, c := range cases {
		got := csrfCookieSecure(c.req, c.mode)
		if got != c.want {
			t.Errorf("csrfCookieSecure(mode=%q, tls=%v) = %v, want %v", c.mode, c.req.TLS != nil, got, c.want)
		}
	}
}
