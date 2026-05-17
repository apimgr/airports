package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/geoip"
	graphqlpkg "github.com/apimgr/airports/src/graphql"
	swaggerpkg "github.com/apimgr/airports/src/swagger"
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
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Server holds application dependencies
type Server struct {
	airports    *airports.Service
	geoip       *geoip.Service
	config      *config.Config
	router      *chi.Mux
	rateLimiter *RateLimiter
	metrics     *appMetrics
}

// ErrorResponse is the canonical error envelope per AI.md PART 14:
// {"ok": false, "error": "CODE", "message": "...", "details": {...}?}.
// The HTTP status code carries status — it is intentionally NOT in the body.
type ErrorResponse struct {
	OK      bool        `json:"ok"`
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// New creates a new server instance
func New(airportSvc *airports.Service, geoipSvc *geoip.Service, cfg *config.Config) *Server {
	// Initialize templates
	if err := initTemplates(); err != nil {
		log.Printf("Warning: Failed to load templates: %v", err)
	}

	s := &Server{
		airports:    airportSvc,
		geoip:       geoipSvc,
		config:      cfg,
		rateLimiter: NewRateLimiter(60, 120), // 60 req/s, burst 120
		metrics:     newMetrics(),
	}

	s.setupRouter()
	return s
}

// Router returns the configured HTTP router
func (s *Server) Router() http.Handler {
	return s.router
}

// setupRouter configures all routes - NO AUTHENTICATION per BASE.md spec
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Prometheus HTTP instrumentation — records http_requests_total and
	// http_request_duration_seconds for every request.
	r.Use(s.metrics.instrumentMiddleware)

	// Security headers — mandated by AI.md PART 11 "Security Headers".
	// Applied to every response so even errors and static assets get them.
	r.Use(s.securityHeaders)

	// Echo the (chi-generated or client-supplied) request ID back to the
	// caller so it can be quoted in bug reports and correlated with logs.
	// AI.md PART 9 mandates X-Request-ID on every response.
	r.Use(s.requestIDHeader)

	// Per-IP rate limiting per AI.md PART 13.
	r.Use(s.rateLimiter.Middleware)

	// CORS - configurable, defaults to "*"
	corsOrigin := s.config.WebSecurity.CORS
	if corsOrigin == "" {
		corsOrigin = "*"
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

	// Static files
	staticFiles, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))

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
	r.Get("/healthz", s.handleServerHealthz) // optional root alias for /server/healthz

	// Flat web aliases retained for existing links and bookmarks.
	r.Get("/search", s.handleSearch)
	r.Get("/nearby", s.handleNearby)

	// /server/* pages (required by IDEA.md and AI.md spec)
	r.Get("/server/about", s.handleServerAbout)
	r.Get("/server/metrics", s.metrics.handler().ServeHTTP)
	r.Get("/server/help", s.handleServerHelp)
	r.Get("/server/healthz", s.handleServerHealthz)
	r.Get("/server/privacy", s.handleServerPrivacy)
	r.Get("/server/terms", s.handleServerTerms)
	// Swagger UI + GraphiQL — handlers live in src/swagger and src/graphql
	// per AI.md PART 14 "Standardized File Locations". UI assets are
	// served from embed.FS in those packages (no CDN, single binary).
	swaggerUI := swaggerpkg.Handler("/api/v1/server/swagger", swaggerAssetsPrefix)
	swaggerSpec := swaggerpkg.SpecHandler()
	graphqlUI := graphqlpkg.UIHandler("/api/v1/server/graphql", graphqlAssetsPrefix)
	graphqlQuery := graphqlpkg.QueryHandler(s.airports)

	r.Get("/server/docs/swagger", swaggerUI)
	r.Get("/server/docs/graphql", graphqlUI)
	r.Mount(swaggerAssetsPrefix, swaggerpkg.AssetsHandler(swaggerAssetsPrefix))
	r.Mount(graphqlAssetsPrefix, graphqlpkg.AssetsHandler(graphqlAssetsPrefix))

	// Unversioned API aliases (mounted to the same handlers as the current version,
	// per AI.md PART 14 "Unversioned API aliases" — never redirect, never fork behavior).
	r.Get("/api/swagger", swaggerSpec)
	r.Get("/api/healthz", s.handleServerHealthz)
	r.Post("/api/graphql", graphqlQuery)
	// AI.md note: "Old paths removed: /openapi, /openapi.json, /graphql (GET and POST
	// at root) are no longer served." The deprecated legacy aliases have therefore
	// been deleted from both the root and the versioned tree.

	// API v1 routes - ALL PUBLIC, NO AUTH
	r.Route("/api/v1", func(r chi.Router) {
		// Add API version header to all v1 responses.
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-API-Version", "v1")
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
		r.Get("/server/swagger", swaggerSpec)
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

	// Metrics endpoint — always mounted at the configured path (default /metrics).
	// The /server/metrics alias above serves browsers; this one is for Prometheus
	// scrapers that follow the standard convention.
	metricsPath := s.config.Server.Metrics.Endpoint
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	r.Get(metricsPath, s.metrics.handler().ServeHTTP)

	s.router = r
}

// JSON helpers

// respondItem sends a single resource directly (no wrapper) per AI.md PART 14.
func (s *Server) respondItem(w http.ResponseWriter, status int, item interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(item); err != nil {
		log.Printf("respondItem: encode failed: %v", err)
	}
}

// respondList sends a paginated list per AI.md PART 14.
func (s *Server) respondList(w http.ResponseWriter, status int, data interface{}, page, limit, total int) {
	pages := 0
	if limit > 0 {
		pages = (total + limit - 1) / limit
	}
	resp := map[string]interface{}{
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

// respondAction sends an action result per AI.md PART 14 ({"ok":true,"data":...}).
func (s *Server) respondAction(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{"ok": true, "data": data}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondAction: encode failed: %v", err)
	}
}

func (s *Server) respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := ErrorResponse{
		OK:      false,
		Error:   code,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondError: encode failed: %v", err)
	}
}

func (s *Server) respondText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(text))
}

// handleAPIInfo returns API information
func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"name":        "Airports API",
		"version":     "v1",
		"description": "Global airport location information API with GeoIP integration",
		"endpoints": map[string]string{
			"airports":  "/api/v1/airports",
			"search":    "/api/v1/search?q=query",
			"nearby":    "/api/v1/nearby?lat=40.64&lon=-73.78&radius=50",
			"airport":   "/api/v1/airports/{ident}",
			"geoip":     "/api/v1/geoip",
			"stats":     "/api/v1/stats",
			"countries": "/api/v1/countries",
			"swagger":   "/api/v1/server/swagger",
			"graphql":   "/api/v1/server/graphql",
			"healthz":   "/api/v1/server/healthz",
		},
		"documentation": "/server/docs/swagger",
	}

	s.respondItem(w, http.StatusOK, info)
}

// handleRobotsTxt returns robots.txt based on config
func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder
	sb.WriteString("User-agent: *\n")

	for _, path := range s.config.WebRobots.Allow {
		sb.WriteString(fmt.Sprintf("Allow: %s\n", path))
	}
	for _, path := range s.config.WebRobots.Deny {
		sb.WriteString(fmt.Sprintf("Disallow: %s\n", path))
	}

	s.respondText(w, http.StatusOK, sb.String())
}

// handleSecurityTxt returns security.txt based on config
func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	contact := s.config.WebSecurity.Admin
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
		"display":          "standalone",
		"background_color": "#282a36",
		"theme_color":      "#bd93f9",
		"icons": []map[string]string{
			{"src": "/static/icons/icon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "/static/icons/icon-512.png", "sizes": "512x512", "type": "image/png"},
		},
	}

	w.Header().Set("Content-Type", "application/manifest+json")
	json.NewEncoder(w).Encode(manifest)
}

// handleServiceWorker returns the PWA service worker
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	sw := `// Service Worker for Airports API
const CACHE_NAME = 'airports-v1';
const urlsToCache = [
  '/',
  '/static/css/main.css',
  '/static/js/main.js'
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
      .then(response => response || fetch(event.request))
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
		"theme":   s.config.WebUI.Theme,
		"cors":    s.config.WebSecurity.CORS,
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

// securityHeaders applies the baseline HTTP security headers required by
// AI.md PART 11. HSTS is set conditionally on TLS-terminated requests.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Server: header exposes the app name and version per AI.md PART 14.
		h.Set("Server", "airports/"+Version)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Origin-Agent-Cluster", "?1")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Permissions-Policy: lock down powerful APIs by default.
		h.Set("Permissions-Policy", "geolocation=(self), camera=(), microphone=(), payment=(), usb=()")
		// Content-Security-Policy: restrict dangerous capabilities.
		// 'unsafe-inline' for script/style is retained for compatibility with
		// the remaining template inline scripts. Dangerous sinks (object, base,
		// frame-ancestors) are blocked unconditionally.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"object-src 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'",
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

// handleGeoIPPage renders the GeoIP lookup page
func (s *Server) handleGeoIPPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "geoip.html", map[string]interface{}{
		"Title": "GeoIP Lookup",
		"Theme": s.config.WebUI.Theme,
	})
}
