package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/geoip"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server holds application dependencies
type Server struct {
	airports *airports.Service
	geoip    *geoip.Service
	devMode  bool
	router   *chi.Mux
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
func New(airportSvc *airports.Service, geoipSvc *geoip.Service, devMode bool) *Server {
	// Initialize templates
	if err := initTemplates(); err != nil {
		log.Printf("Warning: Failed to load templates: %v", err)
	}

	s := &Server{
		airports: airportSvc,
		geoip:    geoipSvc,
		devMode:  devMode,
	}

	s.setupRouter()
	return s
}

// Router returns the configured HTTP router
func (s *Server) Router() http.Handler {
	return s.router
}

// setupRouter configures all routes
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
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

	// Web routes (HTML - Public)
	r.Get("/", s.handleHome)
	r.Get("/search", s.handleSearch)
	r.Get("/nearby", s.handleNearby)
	r.Get("/airport/{code}", s.handleAirportDetail)
	r.Get("/stats", s.handleStats)
	r.Get("/healthz", s.handleHealth)

	// Documentation routes (Public)
	r.Get("/docs", s.handleSwaggerUI)
	r.Get("/graphql", s.handleGraphQLPlayground)

	// Admin routes (Protected - Web UI with Basic Auth)
	r.Group(func(r chi.Router) {
		r.Use(AdminAuthMiddleware)
		r.Get("/admin", s.handleAdminDashboard)
		r.Get("/admin/settings", s.handleAdminSettings)
		r.Post("/admin/settings", s.handleAdminSettingsUpdate)
		r.Get("/admin/database", s.handleAdminDatabase)
		r.Post("/admin/database/test", s.handleAdminDatabaseTest)
		r.Get("/admin/logs", s.handleAdminLogs)
		r.Get("/admin/health", s.handleAdminHealth)
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Documentation endpoints
		r.Get("/docs", s.handleSwaggerUI)
		r.Get("/openapi.json", s.handleOpenAPISpec)
		r.Get("/graphql", s.handleGraphQLPlayground)
		r.Post("/graphql", s.handleGraphQL)

		// Airport endpoints
		r.Get("/airports", s.handleGetAirports)
		r.Get("/airports.json", s.handleGetAirportsJSON)
		r.Get("/airports/{code}", s.handleGetAirportByCode)
		r.Get("/airports/search", s.handleSearchAirports)
		r.Get("/airports/nearby", s.handleNearbyAirports)
		r.Get("/airports/bbox", s.handleBBoxAirports)
		r.Get("/airports/autocomplete", s.handleAutocomplete)
		r.Get("/airports/countries", s.handleGetCountries)
		r.Get("/airports/states/{country}", s.handleGetStates)
		r.Get("/airports/stats", s.handleAirportStats)

		// GeoIP endpoints
		r.Get("/geoip", s.handleGeoIPLookup)
		r.Get("/geoip/{ip}", s.handleGeoIPLookupIP)
		r.Get("/geoip/airports/nearby", s.handleGeoIPNearbyAirports)

		// Health
		r.Get("/health", s.handleHealth)

		// Admin API (Protected - Bearer Token)
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware)
			r.Get("/admin", s.handleAdminAPI)
			r.Get("/admin/settings", s.handleAdminSettingsAPI)
			r.Put("/admin/settings", s.handleAdminSettingsUpdateAPI)
			r.Get("/admin/database", s.handleAdminDatabaseAPI)
			r.Post("/admin/database/test", s.handleAdminDatabaseTestAPI)
			r.Get("/admin/logs", s.handleAdminLogsAPI)
			r.Get("/admin/health", s.handleAdminHealthAPI)
		})
	})

	// Development routes
	if s.devMode {
		r.Get("/debug/routes", s.handleDebugRoutes)
	}

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
