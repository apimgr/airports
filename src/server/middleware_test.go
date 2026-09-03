package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecurityHeaders covers the securityHeaders middleware, including the
// HSTS conditional (present only over TLS or with X-Forwarded-Proto: https).
func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name        string
		forwardedTo string
		useTLS      bool
		wantHSTS    bool
	}{
		{name: "plain http, no hsts", wantHSTS: false},
		{name: "forwarded https header sets hsts", forwardedTo: "https", wantHSTS: true},
		{name: "forwarded https case-insensitive", forwardedTo: "HTTPS", wantHSTS: true},
		{name: "forwarded http does not set hsts", forwardedTo: "http", wantHSTS: false},
		{name: "direct tls sets hsts", useTLS: true, wantHSTS: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.forwardedTo != "" {
				r.Header.Set("X-Forwarded-Proto", tt.forwardedTo)
			}
			if tt.useTLS {
				r.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()

			s.securityHeaders(next).ServeHTTP(w, r)

			if !called {
				t.Fatal("next handler was not called")
			}

			hsts := w.Header().Get("Strict-Transport-Security")
			if tt.wantHSTS && hsts == "" {
				t.Error("expected Strict-Transport-Security header to be set, got none")
			}
			if !tt.wantHSTS && hsts != "" {
				t.Errorf("expected no Strict-Transport-Security header, got %q", hsts)
			}

			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want %q", w.Header().Get("X-Content-Type-Options"), "nosniff")
			}
			if w.Header().Get("X-Frame-Options") == "" {
				t.Error("expected X-Frame-Options header to be set")
			}
			if w.Header().Get("Content-Security-Policy") == "" {
				t.Error("expected Content-Security-Policy header to be set")
			}
		})
	}
}

// TestRequestIDHeader covers echoing of the chi-generated request ID into
// the X-Request-ID response header, and the no-op case when no request ID
// is present in the context (calling the handler directly, bypassing chi's
// RequestID middleware).
func TestRequestIDHeader(t *testing.T) {
	s := newTestServer(t)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.requestIDHeader(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler was not called")
	}
	// No request ID was injected into the context (chi's own RequestID
	// middleware wasn't run), so no header should be set.
	if got := w.Header().Get("X-Request-ID"); got != "" {
		t.Errorf("X-Request-ID = %q, want empty (no request id in context)", got)
	}
}

// TestRequestIDHeaderViaRouter confirms the same middleware picks up a real
// chi-generated request ID when requests are routed through the full router.
func TestRequestIDHeaderViaRouter(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("expected X-Request-ID header to be set when routed through the full router")
	}
}

// TestRateLimiterMiddleware exercises a tight, dedicated limiter (distinct
// from the server's real 60 req/s, burst 120 limiter) so a 429 is reliably
// triggered within a handful of requests.
func TestRateLimiterMiddleware(t *testing.T) {
	rl := NewRateLimiter(1, 1, nil, "", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	// First request consumes the single burst token and should pass.
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.RemoteAddr = "192.0.2.10:1111"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second immediate request from the same IP should be rate limited.
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.RemoteAddr = "192.0.2.10:1111"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d, body=%s", w2.Code, http.StatusTooManyRequests, w2.Body.String())
	}
	if got := w2.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}
	if got := w2.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", got, "60")
	}
	if got := w2.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("X-RateLimit-Remaining = %q, want %q", got, "0")
	}
	if got := w2.Header().Get("X-RateLimit-Reset"); got == "" {
		t.Error("expected X-RateLimit-Reset header to be set")
	}

	// A request from a different IP is a separate bucket and should pass.
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	r3.RemoteAddr = "192.0.2.20:2222"
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("different-IP request status = %d, want %d", w3.Code, http.StatusOK)
	}
}
