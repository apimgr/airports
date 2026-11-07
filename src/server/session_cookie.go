package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
)

func (s *Server) newSessionCookie(r *http.Request, cfg config.SessionConfig, value string) *http.Cookie {
	name := strings.TrimSpace(cfg.CookieName)
	if name == "" {
		name = "session"
	}
	maxAge := 0
	if age, err := time.ParseDuration(strings.TrimSpace(cfg.MaxAge)); err == nil && age > 0 {
		maxAge = int(age.Seconds())
	}
	secure := csrfCookieSecure(r, cfg.Secure, s.urlResolver)
	if s.isTorRequest(r) {
		secure = true
	}
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: cfg.HTTPOnly,
		SameSite: sessionSameSite(cfg.SameSite),
	}
}

func sessionSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
