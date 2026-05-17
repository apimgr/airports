package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"runtime"
)

// Server pages required by IDEA.md and AI.md spec:
//   /server/about    - About page sourced from IDEA.md project description
//   /server/help     - Help / API reference
//   /server/healthz  - Health check (HTML/JSON/text via content negotiation)
//   /server/privacy  - Privacy policy
//   /server/terms    - Terms of use
//   /server/docs/swagger  - Swagger UI (interactive)
//   /server/docs/graphql  - GraphiQL UI (interactive)

// handleServerAbout renders the About page using IDEA.md-derived content.
func (s *Server) handleServerAbout(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "server_about.html", map[string]interface{}{
		"Title":       "About",
		"Theme":       s.config.WebUI.Theme,
		"Tagline":     "Global airport reference data — free, open, and authentication-free.",
		"Description": "Airports is a full-stack Go web application providing comprehensive global airport information. It serves data on 35,000+ airports worldwide — ICAO/IATA codes, names, city, country, coordinates, elevation, and type — through a versioned REST API, a GraphQL endpoint, and a server-side rendered web UI.",
		"Features": []string{
			"Read-only REST API with JSON, CSV, and GeoJSON exports",
			"GraphQL endpoint with interactive playground",
			"OpenAPI / Swagger documentation",
			"GeoIP-based caller location to surface nearby airports",
			"Geographic queries: nearby (radius), bounding box, by code",
			"Server-side rendered web UI (dark / light / auto theme, mobile-first PWA)",
			"CLI client (airports-cli) for terminal use",
			"Single self-contained static binary",
		},
		"Repo":    "https://github.com/apimgr/airports",
		"License": "MIT",
		"Version": Version,
	})
}

// handleServerHelp renders the Help / API reference page.
func (s *Server) handleServerHelp(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "server_help.html", map[string]interface{}{
		"Title": "Help",
		"Theme": s.config.WebUI.Theme,
	})
}

// handleServerPrivacy renders the Privacy page.
func (s *Server) handleServerPrivacy(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "server_privacy.html", map[string]interface{}{
		"Title": "Privacy",
		"Theme": s.config.WebUI.Theme,
	})
}

// handleServerTerms renders the Terms of Use page.
func (s *Server) handleServerTerms(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "server_terms.html", map[string]interface{}{
		"Title": "Terms",
		"Theme": s.config.WebUI.Theme,
	})
}

// handleServerHelpAPI returns the API help reference as JSON per AI.md PART 14.
func (s *Server) handleServerHelpAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"name":    "Airports API Help",
		"version": "v1",
		"endpoints": []map[string]string{
			{"method": "GET", "path": "/api/v1/airports", "description": "List airports (paginated)"},
			{"method": "GET", "path": "/api/v1/airports/{ident}", "description": "Get airport by ICAO or IATA code"},
			{"method": "GET", "path": "/api/v1/search?q=", "description": "Full-text search"},
			{"method": "GET", "path": "/api/v1/nearby?lat=&lon=&radius=", "description": "Airports within radius"},
			{"method": "GET", "path": "/api/v1/bbox?lat_min=&lat_max=&lon_min=&lon_max=", "description": "Airports within bounding box"},
			{"method": "GET", "path": "/api/v1/countries", "description": "List countries"},
			{"method": "GET", "path": "/api/v1/stats", "description": "Dataset statistics"},
			{"method": "GET", "path": "/api/v1/geoip", "description": "Caller GeoIP lookup"},
			{"method": "GET", "path": "/api/v1/geoip/{ip}", "description": "GeoIP lookup for IP"},
			{"method": "GET", "path": "/api/v1/server/healthz", "description": "Health status"},
			{"method": "GET", "path": "/api/v1/server/about", "description": "Server version info"},
		},
		"formats": []string{"json", "csv", "geojson", "text"},
		"docs":    "/server/docs/swagger",
	})
}

// handleServerPrivacyAPI returns the privacy policy as JSON per AI.md PART 14.
func (s *Server) handleServerPrivacyAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]interface{}{
			"data_stored_on_server": false,
			"data_sold":             false,
			"user_control":          false,
		},
		"cookies": map[string]interface{}{
			"essential": map[string]interface{}{
				"enabled":     true,
				"description": "Theme preference (dark/light/auto) stored in localStorage only. No server-side session cookies.",
			},
			"preferences": map[string]interface{}{
				"enabled":     false,
				"description": "No user accounts or server-side preferences.",
			},
			"analytics": map[string]interface{}{
				"enabled":     false,
				"description": "No analytics or tracking of any kind.",
			},
		},
		"data": map[string]interface{}{
			"sold":             false,
			"stored_on_server": false,
			"sharing":          []interface{}{},
		},
		"tracking": map[string]interface{}{
			"enabled":   false,
			"type":      "",
			"type_name": "",
		},
		"retention": map[string]interface{}{
			"period":             "No personal data is collected or retained.",
			"export_available":   false,
			"deletion_available": false,
		},
		"third_party": map[string]interface{}{
			"services": []interface{}{},
		},
		"content": map[string]interface{}{
			"consent_message": "This service collects no personal data.",
			"data_usage":      "Airport data is public reference data served read-only. No user data is stored.",
		},
	})
}

// handleServerTermsAPI returns the terms of use as JSON per AI.md PART 14.
func (s *Server) handleServerTermsAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"service":  "Airports API",
		"version":  Version,
		"license":  "MIT",
		"data_source": map[string]string{
			"airports": "OurAirports (public domain)",
			"geoip":    "ip-location-db via jsDelivr CDN (CC0/PDDL)",
		},
		"usage": "Free, open, and authentication-free. No rate-limit keys required.",
		"repo":  "https://github.com/apimgr/airports",
	})
}

// handleServerAboutAPI returns build metadata as JSON per AI.md PART 14.
func (s *Server) handleServerAboutAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"name":        "Airports API",
		"version":     Version,
		"commit":      Commit,
		"build_date":  BuildDate,
		"repo":        "https://github.com/apimgr/airports",
		"license":     "MIT",
		"go_version":  runtime.Version(),
		"description": "Global airport reference data — free, open, and authentication-free.",
	})
}

// handleServerHealthz performs content negotiation:
//   - text/html for browsers (Accept contains text/html)
//   - text/plain when Accept contains text/plain or path ends in .txt
//   - application/json otherwise (default)
func (s *Server) handleServerHealthz(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	accept := r.Header.Get("Accept")

	switch {
	case strings.Contains(accept, "text/html"):
		s.renderTemplate(w, "server_healthz.html", map[string]interface{}{
			"Title":     "Health",
			"Theme":     s.config.WebUI.Theme,
			"Status":    "healthy",
			"Airports":  stats["total_airports"],
			"Countries": stats["countries"],
			"Version":   Version,
			"Time":      time.Now().UTC().Format(time.RFC3339),
		})
	case strings.Contains(accept, "text/plain"):
		s.respondText(w, http.StatusOK, fmt.Sprintf("status: healthy\nairports: %d\ncountries: %d\nversion: %s\n",
			stats["total_airports"], stats["countries"], Version))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"airports":  stats["total_airports"],
			"countries": stats["countries"],
			"version":   Version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
