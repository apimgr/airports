package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/go-chi/chi/v5/middleware"
)

// csrfContextKey is the request-context key csrfMiddleware stores the
// per-request CSRF token under so renderTemplate can inject it into
// server-rendered forms without every handler wiring it manually.
type csrfContextKey struct{}

// csrfTokenFromContext returns the CSRF token issued for this request, or
// "" when csrfMiddleware did not run (e.g. CSRF disabled).
func csrfTokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(csrfContextKey{}).(string); ok {
		return v
	}
	return ""
}

// generateCSRFToken returns a random hex-encoded token of length random
// bytes (AI.md PART 16 "Configuration" — server.csrf.token_length).
func generateCSRFToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// csrfCookieSecure resolves the csrf_token cookie's Secure attribute from
// server.csrf.secure. "auto" follows the resolved request scheme (TLS or a
// trusted X-Forwarded-Proto: https), mirroring canonicalURL's detection.
func csrfCookieSecure(r *http.Request, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	default:
		return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	}
}

// isBearerRequest reports whether r carries a bearer-style Authorization or
// API-token header. Such requests are API/CLI clients, not browser forms —
// bearer credentials are never auto-attached by a browser to a cross-site
// request, so there is no CSRF vector to defend against (AI.md PART 16
// "When CSRF Validation Runs").
func isBearerRequest(r *http.Request) bool {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) != "" {
			return true
		}
	}
	return strings.TrimSpace(r.Header.Get("X-API-Token")) != ""
}

// csrfPathExempt reports whether p matches one of the operator-declared
// server.csrf.exempt_paths glob patterns.
func csrfPathExempt(p string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, err := path.Match(pattern, p); err == nil && ok {
			return true
		}
	}
	return false
}

// csrfMiddleware implements the stateless double-submit CSRF check from
// AI.md PART 16 "CSRF Protection". Every request gets/keeps a csrf_token
// cookie (SameSite=Strict, Secure per config, NOT HttpOnly so a
// server-rendered form's hidden field can carry the same value) and the
// resolved token is placed in the request context so renderTemplate can
// inject it into forms automatically. Only POST/PUT/PATCH/DELETE requests
// without a bearer credential are validated; GET/HEAD/OPTIONS, WebSocket
// upgrades, and exempt_paths are never checked. There is no Origin-header
// bypass.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.config.Server.CSRF
		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		cookieName := cfg.CookieName
		if cookieName == "" {
			cookieName = "csrf_token"
		}

		token := ""
		if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
			token = c.Value
		}
		if token == "" {
			newToken, err := generateCSRFToken(cfg.TokenLength)
			if err != nil {
				log.Printf("csrfMiddleware: token generation failed: %v", err)
				s.renderErrorPage(w, r, http.StatusInternalServerError)
				return
			}
			token = newToken
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    token,
				Path:     "/",
				MaxAge:   int((365 * 24 * time.Hour).Seconds()),
				Secure:   csrfCookieSecure(r, cfg.Secure),
				HttpOnly: false,
				SameSite: http.SameSiteStrictMode,
			})
		}

		r = r.WithContext(context.WithValue(r.Context(), csrfContextKey{}, token))

		mutating := r.Method == http.MethodPost || r.Method == http.MethodPut ||
			r.Method == http.MethodPatch || r.Method == http.MethodDelete
		if !mutating || isBearerRequest(r) ||
			strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			csrfPathExempt(r.URL.Path, cfg.ExemptPaths) {
			next.ServeHTTP(w, r)
			return
		}

		headerName := cfg.HeaderName
		if headerName == "" {
			headerName = "X-CSRF-Token"
		}
		submitted := r.Header.Get(headerName)
		if submitted == "" {
			submitted = r.FormValue("csrf_token")
		}

		if submitted == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(token)) != 1 {
			ip := middleware.GetClientIP(r.Context())
			log.Printf("security.csrf_failure: rejected request ip=%s path=%s method=%s", ip, r.URL.Path, r.Method)
			lang := i18n.FromContext(r.Context())
			s.writeErrorJSON(w, r, http.StatusForbidden, "CSRF_FAILED", i18n.T(lang, "errors.csrf_failed"))
			return
		}

		next.ServeHTTP(w, r)
	})
}
