package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const buildStampCookieName = "airports_build"

func currentBuildStamp() string {
	return Version + "-" + CommitID
}

func applyBuildStampResponse(w http.ResponseWriter, r *http.Request, status int, contentType string) {
	if w == nil || r == nil || !strings.HasPrefix(contentType, "text/html") {
		return
	}
	stamp := currentBuildStamp()
	if r.Method == http.MethodGet && status == http.StatusOK {
		if cookie, err := r.Cookie(buildStampCookieName); err == nil && cookie.Value != stamp && isTopLevelDocumentRequest(r) {
			w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: buildStampCookieName, Value: stamp, Path: "/", MaxAge: 31536000,
		Expires: time.Now().Add(365 * 24 * time.Hour), Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

func isTopLevelDocumentRequest(r *http.Request) bool {
	destination := r.Header.Get("Sec-Fetch-Dest")
	mode := r.Header.Get("Sec-Fetch-Mode")
	return (destination == "" && mode == "") || (destination == "document" && mode == "navigate")
}

// registerMiddleware installs the global middleware chain in its required order.
func (s *Server) registerMiddleware(r chi.Router) {
	// Request paths are normalized before request IDs and traversal checks.
	r.Use(URLNormalizeMiddleware)
	r.Use(RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Build", Version+"-"+CommitID)
			next.ServeHTTP(w, r)
		})
	})
	r.Use(PathSecurityMiddleware)
	r.Use(gzipCompressionMiddleware(func() config.CompressionConfig {
		s.configMu.RLock()
		defer s.configMu.RUnlock()
		settings := s.config.Server.Compression
		settings.Types = append([]string(nil), settings.Types...)
		return settings
	}))
	r.Use(s.seoNoIndexMiddleware)
	r.Use(redactSecurityReportTokenMiddleware)
	r.Use(requestBodyLimitMiddleware(s.config.Server.Limits.MaxBodyBytes()))
	r.Use(s.debugMiddleware)
	// Resolve trusted proxy entries dynamically so DNS-backed entries refresh
	// every five minutes and config reloads take effect immediately.
	s.proxyCIDRs = newTrustedProxyCIDRCache(func() *config.ApplicationConfig {
		s.configMu.RLock()
		defer s.configMu.RUnlock()
		return s.config
	})
	// Resolve the configured client-IP header chain only when the immediate
	// peer is trusted. The middleware normalizes supported proxy headers before
	// chi walks the chain and never mutates r.RemoteAddr.
	r.Use(trustedClientIPFromCIDRSource(s.proxyCIDRs.CIDRs))
	// Resolve and strip the trusted proxy's external path prefix before routing.
	r.Use(trustedBaseURLMiddlewareWithCIDRSource(s.config.Server.BaseURL, s.proxyCIDRs.CIDRs))
	r.Use(s.panicRecoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Prometheus HTTP instrumentation — records http_requests_total and
	// http_request_duration_seconds for every request.
	r.Use(s.metrics.instrumentMiddleware)

	// Lightweight healthz stats counters (AI.md PART 13 stats.*) — kept
	// separate from Prometheus since CounterVec values aren't cheaply
	// queryable back out as a single scalar.
	r.Use(s.stats.middleware)

	// Security headers — mandated by AI.md PART 11 "Security Headers".
	// Applied to every response so even errors and static assets get them.
	r.Use(s.securityHeaders)

	// Echo the (chi-generated or client-supplied) request ID back to the
	// caller so it can be quoted in bug reports and correlated with logs.
	// AI.md PART 9 mandates X-Request-ID on every response.
	r.Use(s.requestIDHeader)

	// Resolve the request language per AI.md PART 30's fallback chain
	// (?lang= query param sets cookie -> lang cookie -> Accept-Language ->
	// en default) and store it in the request context for handlers/
	// templates to read via i18n.FromContext().
	// Resolve consent once per request so analytics and preference decisions
	// use the same effective state across rendering and handlers.
	r.Use(s.consentMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return i18n.LanguageMiddlewareWithPreferenceGate(next, CanSetPreferenceCookie)
	})
	// The response-writer wrapper defers owner-cookie validation until after
	// rate limiting has decided whether the request may proceed.
	r.Use(s.ownerTokenCookieLifecycle)
	// Maintenance mode rejects writes before IP/rate-limit middleware so every
	// mutation receives the canonical maintenance response.
	r.Use(s.maintenanceWriteGate)

	// IP allowlist/blocklist per AI.md PART 11 "IP Block Management".
	// Allowlist must run first and bypass blocklist + rate limiting (but
	// never CSRF/path-security/TLS, which stay unconditionally enforced
	// elsewhere) — allowlisted IPs are marked in the request context so
	// BlocklistMiddleware and the rate limiter can skip enforcement.
	r.Use(AllowlistMiddleware(s.allowlist))
	r.Use(BlocklistMiddleware(s.blockStore))

	// Per-IP anonymous and per-token authenticated rate limiting per AI.md PART 12.
	r.Use(func(next http.Handler) http.Handler {
		return s.rateLimiter.MiddlewareWithIdentity(next, s.rateLimitBearerIdentity)
	})
	// Country allow/deny-list enforcement per AI.md PART 19 "GeoIP" — a
	// risk signal only, fails open on missing data/lookup errors, and never
	// runs ahead of allowlist/blocklist/rate-limit.
	r.Use(GeoIPMiddleware(s.geoip, s.config.Server.GeoIP.Enabled, s.config.Server.GeoIP.AllowCountries, s.config.Server.GeoIP.DenyCountries))

	// CORS - configurable, defaults to "*"
	corsOrigin := "*"
	if len(s.config.Server.CORS.AllowedOrigins) > 0 {
		corsOrigin = s.config.Server.CORS.AllowedOrigins[0]
	}
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, API-Key, ApiKey, X-Auth-Token, X-Access-Token, X-Token, Token, X-CSRF-Token, X-XSRF-Token, X-Session-ID, X-Service-Token, X-Internal-Token, X-Request-ID, X-Correlation-ID, X-Trace-ID")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Stateless double-submit CSRF protection per AI.md PART 16 "CSRF
	// Protection" — issues/refreshes the csrf_token cookie for frontend
	// requests (so server-rendered forms always have a token to embed) and
	// validates mutating browser requests without a bearer credential. The
	// middleware bypasses backend API paths entirely. It runs after
	// i18n.LanguageMiddleware and before any route handler.
	r.Use(s.csrfMiddleware)
	r.Use(s.accessLogger)
}

func (s *Server) seoNoIndexMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.configMu.RLock()
		noIndex := s.config != nil && s.config.Server.SEO.NoIndex
		s.configMu.RUnlock()
		if noIndex {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		next.ServeHTTP(w, r)
	})
}

type securityReportTokenContextKey struct{}

func securityStatusToken(r *http.Request) string {
	token, _ := r.Context().Value(securityReportTokenContextKey{}).(string)
	return token
}

func redactSecurityReportTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/server/security/report/") {
			next.ServeHTTP(w, r)
			return
		}
		query := r.URL.Query()
		token := query.Get("token")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		query.Del("token")
		requestURL := *r.URL
		requestURL.RawQuery = query.Encode()
		ctx := context.WithValue(r.Context(), securityReportTokenContextKey{}, token)
		request := r.WithContext(ctx)
		request.URL = &requestURL
		next.ServeHTTP(w, request)
	})
}

func requestBodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > maxBytes {
				writeJSONError(w, r, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body exceeds configured maximum size")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// APIBasePath returns the versioned API base path (e.g. "/api/{api_version}"), built
// from server.api_version. Per AI.md PART 14 "API Versioning", code must
// never hardcode "v1" — always resolve through this helper or
// s.config.Server.APIVersion.
func (s *Server) APIBasePath() string {
	v := s.config.Server.APIVersion
	if v == "" {
		v = "v1"
	}
	return "/api/" + v
}
