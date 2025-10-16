package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/airports"
	"github.com/go-chi/chi/v5"
)

// Note: handleHome is now in web_handlers.go

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": "2024-01-01T12:00:00Z",
		"version":   "dev",
		"uptime_seconds": 0,
		"checks": map[string]interface{}{
			"airports": map[string]interface{}{
				"status": "loaded",
				"total":  stats["total_airports"],
			},
			"geoip": map[string]interface{}{
				"status": "loaded",
			},
		},
	}

	s.respondJSON(w, http.StatusOK, health)
}

// handleGetAirports returns paginated list of airports
func (s *Server) handleGetAirports(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	airports := s.airports.GetAll(limit, offset)
	stats := s.airports.Stats()

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"airports": airports,
		"total":    stats["total_airports"],
		"limit":    limit,
		"offset":   offset,
	})
}

// handleGetAirportsJSON returns the full airport database as JSON
func (s *Server) handleGetAirportsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=airports.json")

	data := s.airports.GetRawData()
	json.NewEncoder(w).Encode(data)
}

// handleGetAirportByCode returns a single airport by code
func (s *Server) handleGetAirportByCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	airport, err := s.airports.GetByCode(code)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Airport not found: %s", code))
		return
	}

	s.respondJSON(w, http.StatusOK, airport)
}

// handleSearchAirports searches for airports
func (s *Server) handleSearchAirports(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	airports := s.airports.Search(query, limit, offset)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"airports": airports,
		"query":    query,
		"total":    len(airports),
		"limit":    limit,
		"offset":   offset,
	})
}

// handleNearbyAirports finds airports near coordinates
func (s *Server) handleNearbyAirports(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")
	limitStr := r.URL.Query().Get("limit")
	unitsParam := r.URL.Query().Get("units")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "INVALID_PARAM", "Invalid latitude")
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "INVALID_PARAM", "Invalid longitude")
		return
	}

	radius := 50.0
	if radiusStr != "" {
		radius, _ = strconv.ParseFloat(radiusStr, 64)
	}
	if radius > 500 {
		radius = 500
	}

	limit := 20
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	// Parse unit system (default: imperial)
	units := airports.ParseUnits(unitsParam)

	// Get airports with distance information
	airportsWithDist := s.airports.GetNearbyWithDistance(lat, lon, radius, limit, units)

	// Convert radius for display
	displayRadius, radiusUnit := airports.ConvertDistance(radius, units)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"airports": airportsWithDist,
		"center":   map[string]float64{"lat": lat, "lon": lon},
		"radius":   displayRadius,
		"radius_unit": radiusUnit,
		"units":    units,
		"count":    len(airportsWithDist),
	})
}

// handleBBoxAirports finds airports in bounding box
func (s *Server) handleBBoxAirports(w http.ResponseWriter, r *http.Request) {
	minLat, _ := strconv.ParseFloat(r.URL.Query().Get("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(r.URL.Query().Get("maxLat"), 64)
	minLon, _ := strconv.ParseFloat(r.URL.Query().Get("minLon"), 64)
	maxLon, _ := strconv.ParseFloat(r.URL.Query().Get("maxLon"), 64)

	airports := s.airports.GetInBoundingBox(minLat, maxLat, minLon, maxLon)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"airports": airports,
		"count":    len(airports),
	})
}

// handleAutocomplete provides autocomplete suggestions
func (s *Server) handleAutocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if len(query) < 2 {
		s.respondError(w, http.StatusBadRequest, "INVALID_QUERY", "Query too short (minimum 2 characters)")
		return
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	airports := s.airports.Search(query, limit, 0)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": airports,
		"query":       query,
	})
}

// handleGetCountries returns list of countries
func (s *Server) handleGetCountries(w http.ResponseWriter, r *http.Request) {
	countries := s.airports.GetCountries()
	s.respondJSON(w, http.StatusOK, countries)
}

// handleGetStates returns list of states in a country
func (s *Server) handleGetStates(w http.ResponseWriter, r *http.Request) {
	country := chi.URLParam(r, "country")
	states := s.airports.GetStatesInCountry(country)
	s.respondJSON(w, http.StatusOK, states)
}

// handleAirportStats returns database statistics
func (s *Server) handleAirportStats(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	s.respondJSON(w, http.StatusOK, stats)
}

// handleGeoIPLookup looks up current request IP
func (s *Server) handleGeoIPLookup(w http.ResponseWriter, r *http.Request) {
	// Extract real IP from headers
	ipStr := r.Header.Get("X-Forwarded-For")
	if ipStr == "" {
		ipStr = r.Header.Get("X-Real-IP")
	}
	if ipStr == "" {
		ipStr = r.RemoteAddr
	}

	// Strip port
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		s.respondError(w, http.StatusBadRequest, "INVALID_IP", "Invalid IP address")
		return
	}

	location, err := s.geoip.Lookup(ip)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "LOOKUP_FAILED", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, location)
}

// handleGeoIPLookupIP looks up specific IP
func (s *Server) handleGeoIPLookupIP(w http.ResponseWriter, r *http.Request) {
	ipStr := chi.URLParam(r, "ip")

	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "LOOKUP_FAILED", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, location)
}

// handleGeoIPNearbyAirports finds airports near IP location
func (s *Server) handleGeoIPNearbyAirports(w http.ResponseWriter, r *http.Request) {
	// Get IP to lookup
	ipStr := r.URL.Query().Get("ip")
	if ipStr == "" {
		ipStr = r.Header.Get("X-Forwarded-For")
		if ipStr == "" {
			ipStr = r.Header.Get("X-Real-IP")
		}
		if ipStr == "" {
			ipStr = r.RemoteAddr
		}

		// Strip port
		if host, _, err := net.SplitHostPort(ipStr); err == nil {
			ipStr = host
		}
	}

	// Lookup location
	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "LOOKUP_FAILED", err.Error())
		return
	}

	// Get radius, limit, and units
	radius := 100.0
	if r.URL.Query().Get("radius") != "" {
		radius, _ = strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
	}

	limit := 10
	if r.URL.Query().Get("limit") != "" {
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	}

	unitsParam := r.URL.Query().Get("units")
	units := airports.ParseUnits(unitsParam)

	// Find nearby airports with distance
	airportsNearby := s.airports.GetNearbyWithDistance(location.Latitude, location.Longitude, radius, limit, units)

	// Convert radius for display
	displayRadius, radiusUnit := airports.ConvertDistance(radius, units)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"location":        location,
		"nearby_airports": airportsNearby,
		"radius":          displayRadius,
		"radius_unit":     radiusUnit,
		"units":           units,
	})
}

// handleDebugRoutes shows all registered routes
func (s *Server) handleDebugRoutes(w http.ResponseWriter, r *http.Request) {
	routes := []string{}
	chi.Walk(s.router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, fmt.Sprintf("%s %s", method, route))
		return nil
	})

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Registered Routes:\n\n%s\n", strings.Join(routes, "\n"))
}
