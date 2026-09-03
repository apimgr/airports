package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/airports"
	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/geoip"
	graphqlpkg "github.com/apimgr/airports/src/graphql"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/scheduler"
	swaggerpkg "github.com/apimgr/airports/src/swagger"
	"github.com/apimgr/airports/src/tor"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Asset URL prefixes for the embedded Swagger UI and GraphiQL bundles.
// The UI HTML templates reference these — keep in sync with route mounts.
const (
	swaggerAssetsPrefix = "/server/docs/swagger/assets/"
	graphqlAssetsPrefix = "/server/docs/graphql/assets/"
)

// Build info (set from main)
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

// Server holds application dependencies
type Server struct {
	airports    *airports.Service
	geoip       *geoip.Service
	config      *config.Config
	configMu    sync.RWMutex // guards config hot-reload
	router      *chi.Mux
	rateLimiter *RateLimiter
	metrics     *appMetrics
	scheduler   *scheduler.Scheduler
	cache       cachepkg.Cache
	blockStore  *BlockStore
	allowlist   *AllowlistLookup

	// db, dataDir, and tor back /server/healthz component checks (AI.md
	// PART 13). db and tor may be nil (e.g. in unit tests that don't wire
	// them up) — every consumer treats that as a hard "error"/"disabled"
	// state rather than panicking.
	db        *sql.DB
	dataDir   string
	tor       *tor.Manager
	stats     *statsCollector
	startTime time.Time
}

// ErrorResponse is the canonical error envelope per AI.md PART 14:
// {"ok": false, "error": "CODE", "message": "...", "details": {}}.
// The HTTP status code carries status — it is intentionally NOT in the body.
// Details is never omitted: callers that have nothing to add must still
// serialize an empty object, never a missing field.
type ErrorResponse struct {
	OK      bool        `json:"ok"`
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details"`
}

// New creates a new server instance. db, dataDir, and torMgr feed the
// /server/healthz component checks (AI.md PART 13); db and torMgr may be
// nil when the caller has no such dependency (e.g. tests).
func New(airportSvc *airports.Service, geoipSvc *geoip.Service, cfg *config.Config, sched *scheduler.Scheduler, db *sql.DB, dataDir, configDir string, torMgr *tor.Manager) *Server {
	// Initialize templates
	if err := initTemplates(); err != nil {
		log.Printf("Warning: Failed to load templates: %v", err)
	}

	// Cache backend per AI.md PART 9/12: memory by default, valkey/redis
	// opt-in. Never crash startup on a bad/unreachable cache config — fall
	// back to memory and log a warning (config file is the source of truth,
	// but invalid values must degrade gracefully per PART 5).
	cache, err := cachepkg.New(cfg.Server.Cache)
	if err != nil {
		log.Printf("Warning: cache backend %q unavailable (%v) — falling back to memory", cfg.Server.Cache.Type, err)
		cache, err = cachepkg.New(config.CacheConfig{Type: "memory", Prefix: cfg.Server.Cache.Prefix, TTL: cfg.Server.Cache.TTL})
		if err != nil {
			log.Printf("Warning: memory cache fallback failed (%v) — caching disabled", err)
			cache, _ = cachepkg.New(config.CacheConfig{Type: "none"})
		}
	}

	blockStore := NewBlockStore(cfg.Server.Security.BlockedIPs)
	startTime := time.Now()

	s := &Server{
		airports:    airportSvc,
		geoip:       geoipSvc,
		config:      cfg,
		rateLimiter: NewRateLimiter(60, 120, cfg, configDir, blockStore), // 60 req/s, burst 120
		metrics:     newMetrics(cfg.Server.Metrics, buildInfo{version: Version, commit: CommitID, buildDate: BuildDate}, startTime),
		scheduler:   sched,
		cache:       cache,
		blockStore:  blockStore,
		allowlist:   NewAllowlistLookup(cfg.Server.Security.Allowlist),
		db:          db,
		dataDir:     dataDir,
		tor:         torMgr,
		stats:       newStatsCollector(),
		startTime:   startTime,
	}

	s.setupRouter()
	return s
}

// BlockStore returns the server's shared IP blocklist store, used by the
// scheduler's blocklist_update task so scheduled downloads take effect on
// the already-running server's middleware without a restart.
func (s *Server) BlockStore() *BlockStore {
	return s.blockStore
}

// Allowlist returns the server's shared IP allowlist lookup.
func (s *Server) Allowlist() *AllowlistLookup {
	return s.allowlist
}

// Cache returns the configured cache backend (never nil).
func (s *Server) Cache() cachepkg.Cache {
	return s.cache
}

// CloseCache releases the cache backend's resources (connections,
// background goroutines). Safe to call during graceful shutdown.
func (s *Server) CloseCache() error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Close()
}

// Router returns the configured HTTP router
func (s *Server) Router() http.Handler {
	return s.router
}

// ReloadConfig swaps in a new configuration atomically. Called on SIGHUP.
// The router is not rebuilt — settings take effect on the next request.
func (s *Server) ReloadConfig(cfg *config.Config) {
	s.configMu.Lock()
	s.config = cfg
	s.configMu.Unlock()
}

// setupRouter configures all routes - NO AUTHENTICATION per BASE.md spec
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	// Resolve the client IP from X-Forwarded-For only when it is walked back
	// past trusted proxy hops (private ranges + server.trusted_proxies.additional
	// per AI.md PART 12 "Trusted Proxies"). Unlike the deprecated middleware.RealIP,
	// this never blindly trusts the header and does not mutate r.RemoteAddr.
	r.Use(middleware.ClientIPFromXFF(resolveTrustedProxyCIDRs(s.config)...))
	r.Use(middleware.Logger)
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
	r.Use(i18n.LanguageMiddleware)

	// IP allowlist/blocklist per AI.md PART 11 "IP Block Management".
	// Allowlist must run first and bypass blocklist + rate limiting (but
	// never CSRF/path-security/TLS, which stay unconditionally enforced
	// elsewhere) — allowlisted IPs are marked in the request context so
	// BlocklistMiddleware and the rate limiter can skip enforcement.
	r.Use(AllowlistMiddleware(s.allowlist))
	r.Use(BlocklistMiddleware(s.blockStore))

	// Per-IP rate limiting per AI.md PART 13.
	r.Use(s.rateLimiter.Middleware)

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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Stateless double-submit CSRF protection per AI.md PART 16 "CSRF
	// Protection" — issues/refreshes the csrf_token cookie on every request
	// (so server-rendered forms always have a token to embed) and validates
	// it on mutating browser requests without a bearer credential. Must run
	// after i18n.LanguageMiddleware (translated failure message) and before
	// any route handler.
	r.Use(s.csrfMiddleware)

	// Static files — serve with long-lived cache (1 year) per AI.md PART 9 caching rules.
	// Files are embedded at build time; content is immutable per build.
	staticFiles, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// AI.md PART 6: static file caching is aggressive in production and
		// disabled in development so edited assets are visible immediately.
		headers := mode.GetCacheHeaders()
		w.Header().Set("Cache-Control", headers.CacheControl)
		if headers.Pragma != "" {
			w.Header().Set("Pragma", headers.Pragma)
		}
		if headers.Expires != "" {
			w.Header().Set("Expires", headers.Expires)
		}
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))).ServeHTTP(w, r)
	}))

	// Translation files — served from the same embedded locale JSON the
	// server and CLI use, per AI.md PART 30 "WebUI JavaScript" row.
	r.Get("/locales/{lang}.json", s.handleLocaleJSON)

	// Special files
	r.Get("/robots.txt", s.handleRobotsTxt)
	r.Get("/security.txt", s.handleSecurityTxt)
	r.Get("/.well-known/security.txt", s.handleSecurityTxt)
	r.Get("/manifest.webmanifest", s.handleManifest)
	r.Get("/sw.js", s.handleServiceWorker)

	// Web routes (HTML - All Public, NO AUTH)
	// Canonical paths per AI.md route table: /airports/search, /airports/nearby.
	r.Get("/", s.handleHome)
	r.Get("/airports/search", s.handleSearch)
	r.Get("/airports/nearby", s.handleNearby)
	r.Get("/airports/{ident}", s.handleAirportDetail)
	r.Get("/stats", s.handleStats)
	r.Get("/geoip", s.handleGeoIPPage)
	if s.config.Server.Healthz.Root.Enabled {
		r.Get("/healthz", s.handleServerHealthz) // optional root alias for /server/healthz
	}

	// Flat web aliases retained for existing links and bookmarks.
	r.Get("/search", s.handleSearch)
	r.Get("/nearby", s.handleNearby)

	// /server/* pages (required by IDEA.md and AI.md spec)
	r.Get("/server/about", s.handleServerAbout)
	r.Get("/server/metrics", metricsAuthMiddleware(s.config.Server.Metrics.Token, s.metrics.handler()).ServeHTTP)
	r.Get("/server/help", s.handleServerHelp)
	r.Get("/server/healthz", s.handleServerHealthz)
	r.Get("/server/privacy", s.handleServerPrivacy)
	r.Get("/server/terms", s.handleServerTerms)
	r.Get("/server/contact", s.handleServerContact)
	r.Post("/server/consent", s.handleServerConsent)
	// /server/tor/* — INTERNAL CLI-to-running-server control channel per
	// AI.md PART 31 "CLI-to-running-server control channel". Loopback-gated
	// (404 on any non-loopback peer), never versioned, never in Swagger/
	// GraphQL/well-known/FeaturesInfo — same tier as /server/metrics but
	// without a bearer token (there is no legitimate remote caller).
	r.Get("/server/tor/status", torLoopbackOnly(s.handleTorStatus))
	r.Post("/server/tor/validate", torLoopbackOnly(s.handleTorValidate))
	r.Post("/server/tor/restart", torLoopbackOnly(s.handleTorRestart))
	r.Post("/server/tor/regenerate", torLoopbackOnly(s.handleTorRegenerate))
	r.Post("/server/tor/vanity/start", torLoopbackOnly(s.handleTorVanityStart))
	r.Post("/server/tor/vanity/apply", torLoopbackOnly(s.handleTorVanityApply))
	r.Post("/server/tor/import-keys", torLoopbackOnly(s.handleTorImportKeys))
	r.Post("/announcements/dismiss", s.handleAnnouncementDismiss)
	r.Post("/theme", s.handleSetTheme)
	r.Get("/sitemap.xml", s.handleSitemap)
	// Swagger UI + GraphiQL — handlers live in src/swagger and src/graphql
	// per AI.md PART 14 "Standardized File Locations". UI assets are
	// served from embed.FS in those packages (no CDN, single binary).
	apiBase := s.APIBasePath()
	swaggerUI := swaggerpkg.Handler(apiBase+"/server/swagger", swaggerAssetsPrefix)
	swaggerSpec := swaggerpkg.SpecHandler(Version, s.config.Server.APIVersion)
	graphqlUI := graphqlpkg.UIHandler(apiBase+"/server/graphql", graphqlAssetsPrefix)
	graphqlQuery := graphqlpkg.QueryHandler(s.airports, s.geoip)

	r.Get("/server/docs/swagger", swaggerUI)
	r.Get("/server/docs/graphql", graphqlUI)
	r.Mount(swaggerAssetsPrefix, swaggerpkg.AssetsHandler(swaggerAssetsPrefix))
	r.Mount(graphqlAssetsPrefix, graphqlpkg.AssetsHandler(graphqlAssetsPrefix))

	// Unversioned API aliases (mounted to the same handlers as the current version,
	// per AI.md PART 14 "Unversioned API aliases" — never redirect, never fork behavior).
	r.Get("/api/swagger", swaggerSpec)
	r.Get("/api/swagger.json", swaggerSpec) // canonical .json alias per AI.md PART 14
	r.Get("/api/healthz", s.handleServerHealthz)
	r.Post("/api/graphql", graphqlQuery)
	// AI.md note: "Old paths removed: /openapi, /openapi.json, /graphql (GET and POST
	// at root) are no longer served." The deprecated legacy aliases have therefore
	// been deleted from both the root and the versioned tree.

	// /api/autodiscover — unversioned, per AI.md PART 14. Returns server settings,
	// config schema, and options for CLI/agent consumers.
	r.Get("/api/autodiscover", s.handleAutodiscover)

	// Versioned API routes - ALL PUBLIC, NO AUTH
	r.Route(apiBase, func(r chi.Router) {
		// Add API version + no-cache headers to all versioned responses per AI.md PART 9.
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-API-Version", s.config.Server.APIVersion)
				w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
				next.ServeHTTP(w, r)
			})
		})

		// API info
		r.Get("/", s.handleAPIInfo)

		// Versioned server API surface (canonical paths per AI.md PART 14).
		r.Get("/server/healthz", s.handleServerHealthz)
		r.Get("/server/about", s.handleServerAboutAPI)
		r.Get("/server/help", s.handleServerHelpAPI)
		r.Get("/server/privacy", s.handleServerPrivacyAPI)
		r.Get("/server/terms", s.handleServerTermsAPI)
		r.Get("/server/contact", s.handleServerContactAPI)
		r.Get("/server/swagger", swaggerSpec)
		r.Get("/server/swagger.json", swaggerSpec) // canonical .json alias per AI.md PART 14
		r.Post("/server/graphql", graphqlQuery)

		// Airport endpoints - JSON responses
		r.Get("/airports", s.handleGetAirports)
		r.Get("/airports.json", s.handleGetAirportsJSON)
		r.Get("/airports.csv", s.handleGetAirportsCSV)
		r.Get("/airports.geojson", s.handleGetAirportsGeoJSON)

		// Airport by ident - with .txt extension support
		r.Get("/airports/{ident}", s.handleGetAirportByIdent)
		r.Get("/airports/{ident}.txt", s.handleGetAirportByIdentText)

		// Search / nearby / within — canonical paths per AI.md route table.
		r.Get("/airports/search", s.handleSearchAirports)
		r.Get("/airports/search.txt", s.handleSearchAirportsText)
		r.Get("/airports/nearby", s.handleNearbyAirports)
		r.Get("/airports/nearby.txt", s.handleNearbyAirportsText)
		r.Get("/airports/within", s.handleBBoxAirports)
		r.Get("/airports/autocomplete", s.handleAutocomplete)

		// Unversioned flat aliases retained for clients that already use them.
		r.Get("/search", s.handleSearchAirports)
		r.Get("/search.txt", s.handleSearchAirportsText)
		r.Get("/nearby", s.handleNearbyAirports)
		r.Get("/nearby.txt", s.handleNearbyAirportsText)
		r.Get("/bbox", s.handleBBoxAirports)
		r.Get("/autocomplete", s.handleAutocomplete)

		// Countries and states
		r.Get("/countries", s.handleGetCountries)
		r.Get("/countries.txt", s.handleGetCountriesText)
		r.Get("/states/{country}", s.handleGetStates)

		// Statistics
		r.Get("/stats", s.handleAirportStats)
		r.Get("/stats.txt", s.handleAirportStatsText)

		// GeoIP endpoints
		r.Get("/geoip", s.handleGeoIPLookup)
		r.Get("/geoip.txt", s.handleGeoIPLookupText)
		r.Get("/geoip/{ip}", s.handleGeoIPLookupIP)
		r.Get("/geoip/{ip}.txt", s.handleGeoIPLookupIPText)
		r.Get("/geoip/airports/nearby", s.handleGeoIPNearbyAirports)

		// Health
		r.Get("/health", s.handleHealth)
		r.Get("/health.txt", s.handleHealthText)

		// Settings (read-only, no auth needed)
		r.Get("/settings", s.handleGetSettings)
	})

	// Metrics endpoint — only mounted when server.metrics.enabled is true
	// (AI.md PART 20). The /server/metrics alias above serves browsers; this
	// one is for Prometheus scrapers that follow the standard convention.
	// Internal-only regardless of the optional bearer token below — deployments
	// are still expected to firewall this path/port from public access. When
	// disabled, the path is unregistered and falls through to the themed 404.
	if s.config.Server.Metrics.Enabled {
		metricsPath := s.config.Server.Metrics.Endpoint
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		r.Get(metricsPath, metricsAuthMiddleware(s.config.Server.Metrics.Token, s.metrics.handler()).ServeHTTP)
	}

	// Debug endpoints — only mounted when --debug/DEBUG=true, per AI.md PART 6.
	s.registerDebugRoutes(r)

	// Themed error pages for unmatched routes and methods (AI.md PART 16 —
	// no generic/unstyled browser error pages). Content-negotiated: JSON for
	// API/CLI clients, themed HTML for browsers, plain text otherwise. Chi
	// runs these through the router middleware chain, so the resolved language
	// and request id are available.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		s.renderErrorPage(w, req, http.StatusNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		s.renderErrorPage(w, req, http.StatusMethodNotAllowed)
	})

	s.router = r
}

// APIBasePath returns the versioned API base path (e.g. "/api/v1"), built
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

// JSON helpers

// respondItem sends a single resource wrapped in the canonical success
// envelope per AI.md PART 14: {"ok": true, "data": ...}.
func (s *Server) respondItem(w http.ResponseWriter, status int, item interface{}) {
	resp := map[string]interface{}{
		"ok":   true,
		"data": item,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondItem: encode failed: %v", err)
	}
}

// respondList sends a paginated list wrapped in the canonical success
// envelope per AI.md PART 14: {"ok": true, "data": [...], "pagination": {...}}.
func (s *Server) respondList(w http.ResponseWriter, status int, data interface{}, page, limit, total int) {
	pages := 0
	if limit > 0 {
		pages = (total + limit - 1) / limit
	}
	resp := map[string]interface{}{
		"ok":   true,
		"data": data,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": pages,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondList: encode failed: %v", err)
	}
}

// respondError sends the canonical JSON error envelope. msgKey is an i18n
// key ("errors.*") looked up in the request's resolved language; args are
// optional Tf-style "{name}" placeholder substitutions. Never pass raw
// English or err.Error() here — translate first, log the underlying error
// server-side if it must not leak (AI.md PART 9/11 Tier 1 rule).
func (s *Server) respondError(w http.ResponseWriter, r *http.Request, status int, code, msgKey string, args ...interface{}) {
	lang := i18n.FromContext(r.Context())
	s.writeErrorJSON(w, r, status, code, i18n.Tf(lang, msgKey, args...))
}

// writeErrorJSON encodes the canonical error envelope (AI.md PART 14) with an
// already-translated message. The request id (from middleware.GetReqID, the
// same value echoed in the X-Request-ID header) is surfaced in details so a
// failed API call can be correlated with server logs (AI.md PART 9). Details
// is never omitted — an empty object is serialized when there is no request id.
func (s *Server) writeErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	details := map[string]interface{}{}
	if id := middleware.GetReqID(r.Context()); id != "" {
		details["request_id"] = id
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := ErrorResponse{
		OK:      false,
		Error:   code,
		Message: message,
		Details: details,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("writeErrorJSON: encode failed: %v", err)
	}
}

// errorCodeForStatus maps an HTTP status to the canonical PART 9/14 error code.
func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusServiceUnavailable:
		return "MAINTENANCE"
	default:
		return "SERVER_ERROR"
	}
}

// statusMessageKey maps an HTTP status to an errors.* i18n key, or "" when
// no dedicated key exists (the caller falls back to http.StatusText).
func statusMessageKey(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "errors.bad_request"
	case http.StatusUnauthorized:
		return "errors.unauthorized"
	case http.StatusForbidden:
		return "errors.forbidden"
	case http.StatusNotFound:
		return "errors.not_found"
	case http.StatusTooManyRequests:
		return "errors.rate_limited"
	case http.StatusServiceUnavailable:
		return "errors.maintenance"
	case http.StatusInternalServerError:
		return "errors.internal"
	default:
		return ""
	}
}

// renderErrorPage sends a themed, content-negotiated error response per AI.md
// PART 14/16: the canonical JSON envelope for API/CLI clients, the themed
// error.html page (extending base.html) for browsers, and plain text
// otherwise. All three carry the correct HTTP status code; JSON additionally
// carries the request id (PART 9). It is safe to call only before any response
// body has been written.
func (s *Server) renderErrorPage(w http.ResponseWriter, r *http.Request, status int) {
	lang := i18n.FromContext(r.Context())
	code := errorCodeForStatus(status)

	message := http.StatusText(status)
	if key := statusMessageKey(status); key != "" {
		message = i18n.T(lang, key)
	}

	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "application/json"):
		s.writeErrorJSON(w, r, status, code, message)
	case strings.Contains(accept, "text/html"):
		statusText := http.StatusText(status)
		data := map[string]interface{}{
			"Title":      statusText,
			"StatusCode": status,
			"StatusText": statusText,
			"Message":    message,
		}
		// On a pre-write template failure, fall back to a bare (unthemed)
		// response — never recurse into themed rendering.
		if err := s.executeTemplateStatus(w, r, "error", status, data); err != nil {
			log.Printf("renderErrorPage: render error template failed: %v", err)
			http.Error(w, message, status)
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprintf(w, "ERROR: %s: %s\n", code, message)
	}
}

func (s *Server) respondText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(text))
}

// handleAPIInfo returns API information
func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	base := s.APIBasePath()
	info := map[string]interface{}{
		"name":        "Airports API",
		"version":     s.config.Server.APIVersion,
		"description": "Global airport location information API with GeoIP integration",
		"endpoints": map[string]string{
			"airports":     base + "/airports",
			"search":       base + "/airports/search?q=query",
			"nearby":       base + "/airports/nearby?lat=40.64&lon=-73.78&radius=50",
			"within":       base + "/airports/within?lat_min=&lat_max=&lon_min=&lon_max=",
			"autocomplete": base + "/airports/autocomplete?q=",
			"airport":      base + "/airports/{ident}",
			"geoip":        base + "/geoip",
			"stats":        base + "/stats",
			"countries":    base + "/countries",
			"swagger":      base + "/server/swagger",
			"graphql":      base + "/server/graphql",
			"healthz":      base + "/server/healthz",
			"autodiscover": "/api/autodiscover",
		},
		"documentation": "/server/docs/swagger",
	}

	s.respondItem(w, http.StatusOK, info)
}

// handleRobotsTxt returns robots.txt based on config, per AI.md PART 11
// "robots.txt". Denied AI crawlers get their own stanza ahead of the wildcard
// block; bots resolving to allow are covered by "User-agent: *" and get none.
func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder

	for _, bot := range s.deniedAIBots() {
		sb.WriteString(fmt.Sprintf("User-agent: %s\nDisallow: /\n\n", bot))
	}

	sb.WriteString("User-agent: *\n")
	for _, path := range s.config.Web.Robots.Allow {
		sb.WriteString(fmt.Sprintf("Allow: %s\n", path))
	}
	for _, path := range s.config.Web.Robots.Deny {
		sb.WriteString(fmt.Sprintf("Disallow: %s\n", path))
	}
	sb.WriteString(fmt.Sprintf("Sitemap: %s/sitemap.xml\n", s.publicBaseURL(r)))

	s.respondText(w, http.StatusOK, sb.String())
}

// deniedAIBots returns the sorted list of configured AI crawlers that resolve
// to deny — either an explicit "deny" value, or no explicit value while
// web.robots.ai_bots.default is "deny". Explicit entries always win over the
// default, per AI.md PART 11 "AI Crawler Rules".
func (s *Server) deniedAIBots() []string {
	aiBots := s.config.Web.Robots.AIBots
	denied := make([]string, 0, len(aiBots.Bots))
	for bot, access := range aiBots.Bots {
		switch access {
		case "deny":
			denied = append(denied, bot)
		case "allow":
			// explicitly allowed — covered by the wildcard block
		default:
			if aiBots.Default == "deny" {
				denied = append(denied, bot)
			}
		}
	}
	sort.Strings(denied)
	return denied
}

// handleLocaleJSON serves the embedded translation file for {lang} so
// WebUI JavaScript can fetch /locales/{lang}.json, per AI.md PART 30
// "WebUI JavaScript" row. Unsupported languages fall back to English
// rather than erroring (PART 30 "Unsupported language fallback").
func (s *Server) handleLocaleJSON(w http.ResponseWriter, r *http.Request) {
	lang := strings.TrimSuffix(chi.URLParam(r, "lang"), ".json")

	raw, err := i18n.RawLocale(lang)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
	w.Write([]byte("\n"))
}

// publicBaseURL resolves scheme://host for the current request. The scheme is
// derived from the actual TLS state; X-Forwarded-Proto is honored ONLY when
// the request arrived through a recognized trusted proxy (AI.md PART 12).
// The trusted-proxy signal reused here is the same one getClientIP/realIP
// rely on: middleware.ClientIPFromXFF resolves a client IP into the context
// only after walking past trusted proxy hops, so a non-empty value means the
// peer was trusted. Untrusted peers can spoof the header, so their forwarded
// proto is ignored. The default port for the resolved scheme (:80 for http,
// :443 for https) is stripped from the host per AI.md PART 13/15.
func (s *Server) publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if middleware.GetClientIP(r.Context()) != "" {
		if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			// X-Forwarded-Proto may be a comma-separated list; the first entry
			// is the original client-facing scheme.
			if comma := strings.IndexByte(fwdProto, ','); comma >= 0 {
				fwdProto = fwdProto[:comma]
			}
			scheme = strings.ToLower(strings.TrimSpace(fwdProto))
		}
	}
	return scheme + "://" + stripDefaultPort(r.Host, scheme)
}

// stripDefaultPort removes the scheme's default port (:80 for http, :443 for
// https) from host, leaving other ports intact. IPv6 literals keep their
// bracket notation. Hosts without a port are returned unchanged.
func stripDefaultPort(host, scheme string) string {
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		if strings.Contains(h, ":") {
			return "[" + h + "]"
		}
		return h
	}
	return host
}

// sitemapEntry describes one <url> block in sitemap.xml per AI.md PART 16
// "Sitemap Generation Rules" (priority/changefreq by page type).
type sitemapEntry struct {
	Path       string
	Priority   string
	ChangeFreq string
}

// publicSitemapEntries is the single list of routes considered public content:
// it drives both sitemap.xml and the per-route "index,follow" robots meta tag
// (AI.md PART 16 "Robots Directive" — every other route fails closed to
// "noindex,nofollow"). Never add API, health, debug, or internal routes here.
var publicSitemapEntries = []sitemapEntry{
	{Path: "/", Priority: "1.0", ChangeFreq: "daily"},
	{Path: "/airports/search", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/airports/nearby", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/stats", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/geoip", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/about", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/help", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/privacy", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/terms", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/contact", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/docs/swagger", Priority: "0.7", ChangeFreq: "weekly"},
	{Path: "/server/docs/graphql", Priority: "0.7", ChangeFreq: "weekly"},
}

// handleSitemap returns a dynamically generated sitemap.xml listing every
// public frontend route. API endpoints and any authenticated/server-management
// pages are NEVER included per AI.md PART 16.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	base := s.publicBaseURL(r)
	lastmod := time.Now().UTC().Format("2006-01-02")

	entries := publicSitemapEntries

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		sb.WriteString("  <url>\n")
		sb.WriteString("    <loc>" + base + e.Path + "</loc>\n")
		sb.WriteString("    <lastmod>" + lastmod + "</lastmod>\n")
		sb.WriteString("    <changefreq>" + e.ChangeFreq + "</changefreq>\n")
		sb.WriteString("    <priority>" + e.Priority + "</priority>\n")
		sb.WriteString("  </url>\n")
	}
	sb.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

// handleSecurityTxt returns security.txt based on config
func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	contact := s.config.Web.Security.Admin
	if contact == "" {
		contact = "security@apimgr.us"
	}

	content := fmt.Sprintf(`Contact: mailto:%s
Expires: %s
Preferred-Languages: en
`, contact, time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z"))

	s.respondText(w, http.StatusOK, content)
}

// handleManifest returns the PWA web app manifest.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	manifest := map[string]interface{}{
		"name":             "Airports API",
		"short_name":       "Airports",
		"description":      "Global airport location information API",
		"start_url":        "/",
		"scope":            "/",
		"display":          "standalone",
		"orientation":      "any",
		"background_color": "#282a36",
		"theme_color":      "#bd93f9",
		"categories":       []string{"utilities", "travel"},
		"icons": []map[string]string{
			{"src": "/static/icons/icon-72.png", "sizes": "72x72", "type": "image/png"},
			{"src": "/static/icons/icon-96.png", "sizes": "96x96", "type": "image/png"},
			{"src": "/static/icons/icon-128.png", "sizes": "128x128", "type": "image/png"},
			{"src": "/static/icons/icon-144.png", "sizes": "144x144", "type": "image/png"},
			{"src": "/static/icons/icon-152.png", "sizes": "152x152", "type": "image/png"},
			{"src": "/static/icons/icon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "/static/icons/icon-384.png", "sizes": "384x384", "type": "image/png"},
			{"src": "/static/icons/icon-512.png", "sizes": "512x512", "type": "image/png"},
			{"src": "/static/icons/icon-maskable-192.png", "sizes": "192x192", "type": "image/png", "purpose": "maskable"},
			{"src": "/static/icons/icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	}

	w.Header().Set("Content-Type", "application/manifest+json")
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		log.Printf("handleManifest: encode failed: %v", err)
	}
}

// handleServiceWorker returns the PWA service worker
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	sw := `// Service Worker for Airports API
const CACHE_NAME = 'airports-v1';
const urlsToCache = [
  '/',
  '/static/css/common.css',
  '/static/css/components.css',
  '/static/css/public.css',
  '/static/js/app.js',
  '/static/offline.html'
];

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => cache.addAll(urlsToCache))
  );
});

self.addEventListener('fetch', event => {
  event.respondWith(
    caches.match(event.request)
      .then(response => response || fetch(event.request)
        .catch(() => {
          if (event.request.mode === 'navigate') {
            return caches.match('/static/offline.html');
          }
          return Response.error();
        }))
  );
});
`
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(sw))
}

// handleGetSettings returns current configuration (read-only)
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// Return safe subset of settings (no sensitive data)
	settings := map[string]interface{}{
		"theme":   s.config.Web.UI.Theme,
		"cors":    s.config.Server.CORS.AllowedOrigins,
		"metrics": s.config.Server.Metrics.Enabled,
	}

	s.respondItem(w, http.StatusOK, settings)
}

// handleHealthText returns health status as text
func (s *Server) handleHealthText(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	text := fmt.Sprintf("status: healthy\nairports: %d\ncountries: %d\n",
		stats["total_airports"], stats["countries"])
	s.respondText(w, http.StatusOK, text)
}

// panicRecoverer recovers from handler panics per AI.md PART 6's
// mode-dependent panic recovery: "graceful" in production (log the error,
// return a generic 500) and "verbose" in development (also write the
// recovered value and stack trace into the response so a local developer
// sees the failure immediately). Replaces chi's generic middleware.Recoverer
// so the response body can vary by mode.
func (s *Server) panicRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				if mode.GetPanicRecoveryMode() == "verbose" {
					fmt.Fprintf(w, "panic: %v\n\n%s", rec, debug.Stack())
					return
				}
				fmt.Fprint(w, "An internal error occurred. Please contact support if the problem persists.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders applies the baseline HTTP security headers required by
// AI.md PART 11 ("Security Headers" / "Cross-Origin Isolation Headers" /
// "Reporting API" / "Content Security Policy"). HSTS is set conditionally
// on TLS-terminated requests.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Server: and X-App-Version expose the app name and version per AI.md PART 14.
		h.Set("Server", "airports/"+Version)
		h.Set("X-App-Version", Version)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Origin-Agent-Cluster", "?1")
		// Cross-Origin Isolation headers (AI.md PART 11 §"Cross-Origin
		// Isolation Headers"). COOP/COEP stay at their spec defaults
		// (unsafe-none) since this app neither embeds SharedArrayBuffer
		// nor handles payment/banking flows; CORP stays cross-origin so
		// the public API/static assets remain embeddable.
		h.Set("Cross-Origin-Opener-Policy", "unsafe-none")
		h.Set("Cross-Origin-Embedder-Policy", "unsafe-none")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Permissions-Policy: spec default table (AI.md PART 11 §"Permissions-Policy
		// Configuration"), with geolocation scoped to self since the app's
		// "use my location" / nearby-airports feature actively uses it.
		h.Set("Permissions-Policy", strings.Join([]string{
			"accelerometer=()",
			"ambient-light-sensor=()",
			"attribution-reporting=()",
			"autoplay=(self)",
			"battery=()",
			"browsing-topics=()",
			"camera=()",
			"display-capture=()",
			"encrypted-media=(self)",
			"fullscreen=(self)",
			"geolocation=(self)",
			"gyroscope=()",
			"hid=()",
			"idle-detection=()",
			"interest-cohort=()",
			"magnetometer=()",
			"microphone=()",
			"midi=()",
			"payment=(self)",
			"picture-in-picture=(self)",
			"publickey-credentials-get=(self)",
			"screen-wake-lock=()",
			"serial=()",
			"storage-access=(self)",
			"usb=()",
			"web-share=(self)",
			"xr-spatial-tracking=()",
		}, ", "))

		// Reporting API (AI.md PART 11 §"Reporting API (Modern + Legacy)"):
		// modern Reporting-Endpoints plus the legacy Report-To/NEL headers,
		// both pointing at the same default report group.
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		}
		reportURL := scheme + "://" + r.Host + s.APIBasePath() + "/server/reports/default"
		h.Set("Reporting-Endpoints", `default="`+reportURL+`"`)
		h.Set("Report-To", `{"group":"default","max_age":10886400,"endpoints":[{"url":"`+reportURL+`"}]}`)
		h.Set("NEL", `{"report_to":"default","max_age":2592000,"include_subdomains":true}`)

		// Content-Security-Policy (AI.md PART 11 §"Content Security Policy",
		// default per-directive policy). script-src is 'self' only — all JS
		// lives in static/js/app.js, no inline scripts or inline handlers
		// remain anywhere in the frontend (AI.md PART 16). style-src keeps
		// 'unsafe-inline' per the documented spec default (inline style=""
		// attributes are unavoidable).
		cspReportURI := s.APIBasePath() + "/server/reports/csp"
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob: https:; "+
				"font-src 'self' https:; "+
				"connect-src 'self'; "+
				"media-src 'self' blob:; "+
				"worker-src 'self' blob:; "+
				"manifest-src 'self'; "+
				"frame-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"object-src 'none'; "+
				"upgrade-insecure-requests; "+
				"report-to default; "+
				"report-uri "+cspReportURI,
		)
		// HSTS only when the request actually arrived over TLS (either
		// direct or via a trusted reverse-proxy that set X-Forwarded-Proto).
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

// requestIDHeader echoes the chi-generated request ID back to the client
// in the X-Request-ID response header so it can be quoted in bug reports.
func (s *Server) requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}

// handleGeoIPPage renders the GeoIP lookup page. Reads "ip" from the query
// string and performs the lookup server-side so the page works as a plain
// GET form with JavaScript disabled (AI.md PART 16 "No JavaScript-Disabled
// Broken State") — the JS-enhanced form posts back the same "ip" param and
// preventDefault()s to render the result via AJAX instead.
func (s *Server) handleGeoIPPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "GeoIP Lookup",
		"Theme": s.config.Web.UI.Theme,
	}

	ipStr := strings.TrimSpace(r.URL.Query().Get("ip"))
	if ipStr != "" {
		data["IP"] = ipStr
		if loc, err := s.geoip.LookupString(ipStr); err != nil {
			data["LookupError"] = mode.GetErrorDetail(err)
		} else {
			data["Location"] = loc
			if loc.Latitude != 0 || loc.Longitude != 0 {
				data["Nearby"] = s.airports.GetNearbyWithDistance(loc.Latitude, loc.Longitude, 100, 10, airports.UnitImperial)
			}
		}
	}

	s.renderTemplate(w, r, "geoip", data)
}
