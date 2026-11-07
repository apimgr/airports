package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestSessionCookieUsesConfiguredDefaults(t *testing.T) {
	s := newTestServer(t)
	cfg := s.config.Server.Session
	request := httptest.NewRequest(http.MethodGet, "https://airports.example.test/", nil)
	cookie := s.newSessionCookie(request, cfg, "opaque-session")
	if cookie.Name != "session" || cookie.Value != "opaque-session" || cookie.Path != "/" {
		t.Fatalf("session cookie identity = %+v", cookie)
	}
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("session cookie lifetime = max-age %d, expires %v; want browser-session lifetime", cookie.MaxAge, cookie.Expires)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie attributes = %+v, want Secure, HttpOnly, SameSite=Lax", cookie)
	}
}

func TestSessionCookieHonorsConfiguredAttributes(t *testing.T) {
	s := newTestServer(t)
	cfg := config.SessionConfig{
		CookieName: "custom_session",
		MaxAge:     "2h",
		Secure:     "false",
		HTTPOnly:   false,
		SameSite:   "strict",
	}
	cookie := s.newSessionCookie(httptest.NewRequest(http.MethodGet, "http://airports.example.test/", nil), cfg, "value")
	if cookie.Name != "custom_session" || cookie.MaxAge != 7200 || cookie.Secure || cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %+v, want configured cookie attributes", cookie)
	}
}
