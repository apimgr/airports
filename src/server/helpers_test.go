package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

// TestRealIP_IgnoresHeadersWithoutTrustedMiddleware is the anti-spoofing
// regression test for AI.md's trusted-proxy fix (GO-2026-5777/5775/5774):
// realIP (ratelimit.go) must NEVER read X-Real-IP/X-Forwarded-For directly.
// It only trusts middleware.GetClientIP(ctx), which is set exclusively by
// middleware.ClientIPFromXFF after CIDR-gating. Without that middleware in
// the chain, spoofed headers must be fully ignored and RemoteAddr used.
func TestRealIP_IgnoresHeadersWithoutTrustedMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		realIP     string
		forwarded  string
		remoteAddr string
		want       string
	}{
		{
			name:       "spoofed x-real-ip ignored",
			realIP:     "203.0.113.5",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "spoofed x-forwarded-for ignored",
			forwarded:  "198.51.100.9, 10.0.0.1",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "remote addr port stripped",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "remote addr without port unchanged",
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},
		{
			name:       "ipv6 remote addr port stripped",
			remoteAddr: "[2001:db8::1]:8080",
			want:       "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			got := realIP(r)
			if got != tt.want {
				t.Errorf("realIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGetClientIP_IgnoresHeadersWithoutTrustedMiddleware mirrors
// TestRealIP_IgnoresHeadersWithoutTrustedMiddleware for handlers.go's
// getClientIP — same anti-spoofing requirement.
func TestGetClientIP_IgnoresHeadersWithoutTrustedMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		forwarded  string
		realIP     string
		remoteAddr string
		want       string
	}{
		{
			name:       "spoofed x-forwarded-for ignored",
			forwarded:  "198.51.100.9, 10.0.0.1",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "spoofed x-real-ip ignored",
			realIP:     "203.0.113.5",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "remote addr fallback with port stripped",
			remoteAddr: "192.0.2.1:8080",
			want:       "192.0.2.1",
		},
		{
			name:       "remote addr without port returned as-is when SplitHostPort fails",
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}

			got := getClientIP(r)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClientIPFromXFF_TrustedProxyResolvesRealIP covers the intended,
// protected path: middleware.ClientIPFromXFF (wired in server.go via
// resolveTrustedProxyCIDRs, AI.md PART 12 "Trusted Proxies") walks the
// X-Forwarded-For chain right-to-left, skipping entries inside the trusted
// CIDR list, and stops at the first untrusted entry — the real client IP,
// as honestly appended by the trusted proxy. Both realIP and getClientIP
// must read that resolved value via middleware.GetClientIP.
func TestClientIPFromXFF_TrustedProxyResolvesRealIP(t *testing.T) {
	tests := []struct {
		name      string
		forwarded string
		want      string
	}{
		{
			name:      "single trusted proxy hop resolves real client",
			forwarded: "203.0.113.9, 10.0.0.5",
			want:      "203.0.113.9",
		},
		{
			name:      "client-injected fake hop ahead of trusted proxy is ignored",
			forwarded: "6.6.6.6, 203.0.113.9, 10.0.0.5",
			want:      "203.0.113.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = "10.0.0.5:1234"
			r.Header.Set("X-Forwarded-For", tt.forwarded)

			var gotRealIP, gotClientIP string
			handler := middleware.ClientIPFromXFF(defaultTrustedProxyCIDRs...)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotRealIP = realIP(r)
					gotClientIP = getClientIP(r)
				}),
			)
			handler.ServeHTTP(httptest.NewRecorder(), r)

			if gotRealIP != tt.want {
				t.Errorf("realIP() = %q, want %q", gotRealIP, tt.want)
			}
			if gotClientIP != tt.want {
				t.Errorf("getClientIP() = %q, want %q", gotClientIP, tt.want)
			}
		})
	}
}

// TestClientIPFromXFF_UntrustedSourceNotSpoofed shows that when the
// X-Forwarded-For chain's rightmost hop is not inside any trusted CIDR
// (no recognized trusted proxy in front of the request), the resolved
// IP is that rightmost entry itself — it is never possible for a client
// to make the middleware skip past an untrusted hop, so nothing further
// left in the chain (attacker-controlled) is ever trusted.
func TestClientIPFromXFF_UntrustedSourceNotSpoofed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.1:1234"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.9")

	var gotClientIP string
	handler := middleware.ClientIPFromXFF(defaultTrustedProxyCIDRs...)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotClientIP = getClientIP(r)
		}),
	)
	handler.ServeHTTP(httptest.NewRecorder(), r)

	// 198.51.100.9 is the rightmost entry and is not a trusted proxy CIDR,
	// so it is resolved as the client IP; 6.6.6.6 (further left, attacker
	// territory) is never reached or trusted.
	want := "198.51.100.9"
	if gotClientIP != want {
		t.Errorf("getClientIP() = %q, want %q", gotClientIP, want)
	}
}

// TestRobotsDirective covers AI.md PART 16 "Robots Directive": only routes
// published as public content are indexable, everything else fails closed.
func TestRobotsDirective(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/", "index,follow"},
		{"/airports/search", "index,follow"},
		{"/airports/KJFK", "index,follow"},
		{"/server/about", "index,follow"},
		{"/server/about/", "index,follow"},
		{"/api/v1/airports", "noindex,nofollow"},
		{"/server/healthz", "noindex,nofollow"},
		{"/debug/pprof/", "noindex,nofollow"},
		{"/search", "noindex,nofollow"},
	}
	for _, c := range cases {
		got := robotsDirective(httptest.NewRequest(http.MethodGet, c.path, nil))
		if got != c.want {
			t.Errorf("robotsDirective(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
