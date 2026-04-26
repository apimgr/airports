package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/airports/src/admin"
	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/geoip"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Build info (set from main)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Server holds application dependencies
type Server struct {
	airports     *airports.Service
	geoip        *geoip.Service
	config       *config.Config
	router       *chi.Mux
	adminHandler *admin.Handler
}

// Response is the standard API response format
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorData  `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// ErrorData contains error information
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// New creates a new server instance
func New(airportSvc *airports.Service, geoipSvc *geoip.Service, cfg *config.Config) *Server {
	// Initialize templates
	if err := initTemplates(); err != nil {
		log.Printf("Warning: Failed to load templates: %v", err)
	}

	// Create admin handler
	adminHandler := admin.NewHandler(
		cfg.Server.Admin.Username,
		cfg.Server.Admin.Password,
		cfg.Server.Admin.APIToken,
		cfg.Server.Session.Timeout,
		cfg.Server.SSL.Enabled,
		Version,
		Commit,
		BuildDate,
	)

	s := &Server{
		airports:     airportSvc,
		geoip:        geoipSvc,
		config:       cfg,
		adminHandler: adminHandler,
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
	r.Get("/manifest.json", s.handleManifest)
	r.Get("/sw.js", s.handleServiceWorker)

	// Web routes (HTML - All Public, NO AUTH)
	r.Get("/", s.handleHome)
	r.Get("/search", s.handleSearch)
	r.Get("/nearby", s.handleNearby)
	r.Get("/airport/{code}", s.handleAirportDetail)
	r.Get("/stats", s.handleStats)
	r.Get("/geoip", s.handleGeoIPPage)
	r.Get("/healthz", s.handleHealth)

	// API Documentation routes (Public)
	r.Get("/openapi", s.handleSwaggerUI)
	r.Get("/graphql", s.handleGraphQLPlayground)

	// API v1 routes - ALL PUBLIC, NO AUTH per BASE.md
	r.Route("/api/v1", func(r chi.Router) {
		// API info
		r.Get("/", s.handleAPIInfo)

		// API Documentation endpoints
		r.Get("/openapi", s.handleSwaggerUI)
		r.Get("/openapi.json", s.handleOpenAPISpec)
		r.Get("/graphql", s.handleGraphQLPlayground)
		r.Post("/graphql", s.handleGraphQL)

		// Airport endpoints - JSON responses
		r.Get("/airports", s.handleGetAirports)
		r.Get("/airports.json", s.handleGetAirportsJSON)
		r.Get("/airports.csv", s.handleGetAirportsCSV)
		r.Get("/airports.geojson", s.handleGetAirportsGeoJSON)

		// Airport by code - with .txt extension support
		r.Get("/airport/{code}", s.handleGetAirportByCode)
		r.Get("/airport/{code}.txt", s.handleGetAirportByCodeText)

		// Search endpoints
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

	// Metrics endpoint (if enabled)
	if s.config.Server.Metrics.Enabled {
		r.Get(s.config.Server.Metrics.Endpoint, s.handleMetrics)
	}

	// Admin routes (session auth for web, bearer token for API)
	s.adminHandler.RegisterRoutes(r)

	s.router = r
}

// JSON helpers
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{
		Success:   status < 400,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{
		Success:   false,
		Error:     &ErrorData{Code: code, Message: message},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
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
			"airport":   "/api/v1/airport/{code}",
			"geoip":     "/api/v1/geoip",
			"stats":     "/api/v1/stats",
			"countries": "/api/v1/countries",
			"openapi":   "/api/v1/openapi",
			"graphql":   "/api/v1/graphql",
		},
		"documentation": "/openapi",
	}

	s.respondJSON(w, http.StatusOK, info)
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

// handleManifest returns PWA manifest.json
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
  '/static/css/style.css',
  '/static/js/app.js'
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

	s.respondJSON(w, http.StatusOK, settings)
}

// handleMetrics returns Prometheus-compatible metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()

	var sb strings.Builder
	sb.WriteString("# HELP airports_total Total number of airports\n")
	sb.WriteString("# TYPE airports_total gauge\n")
	sb.WriteString(fmt.Sprintf("airports_total %d\n", stats["total_airports"]))

	sb.WriteString("# HELP airports_countries Number of countries\n")
	sb.WriteString("# TYPE airports_countries gauge\n")
	sb.WriteString(fmt.Sprintf("airports_countries %d\n", stats["countries"]))

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(sb.String()))
}

// handleHealthText returns health status as text
func (s *Server) handleHealthText(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	text := fmt.Sprintf("status: healthy\nairports: %d\ncountries: %d\n",
		stats["total_airports"], stats["countries"])
	s.respondText(w, http.StatusOK, text)
}

// handleGeoIPPage renders the GeoIP lookup page
func (s *Server) handleGeoIPPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "geoip.html", map[string]interface{}{
		"Title": "GeoIP Lookup",
		"Theme": s.config.WebUI.Theme,
	})
}
