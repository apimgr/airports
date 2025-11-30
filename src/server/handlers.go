package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/airports"
	"github.com/go-chi/chi/v5"
)

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()

	health := map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
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

// handleGetAirportsCSV returns the full airport database as CSV
func (s *Server) handleGetAirportsCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=airports.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"icao", "iata", "name", "city", "state", "country", "elevation", "lat", "lon", "tz"})

	// Write data
	airports := s.airports.GetAll(100000, 0) // Get all
	for _, a := range airports {
		writer.Write([]string{
			a.ICAO, a.IATA, a.Name, a.City, a.State, a.Country,
			fmt.Sprintf("%d", a.Elevation),
			fmt.Sprintf("%.6f", a.Lat),
			fmt.Sprintf("%.6f", a.Lon),
			a.Tz,
		})
	}
}

// handleGetAirportsGeoJSON returns the full airport database as GeoJSON
func (s *Server) handleGetAirportsGeoJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/geo+json")
	w.Header().Set("Content-Disposition", "attachment; filename=airports.geojson")

	airports := s.airports.GetAll(100000, 0) // Get all
	features := make([]map[string]interface{}, 0, len(airports))

	for _, a := range airports {
		feature := map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{a.Lon, a.Lat},
			},
			"properties": map[string]interface{}{
				"icao":      a.ICAO,
				"iata":      a.IATA,
				"name":      a.Name,
				"city":      a.City,
				"country":   a.Country,
				"elevation": a.Elevation,
				"timezone":  a.Tz,
			},
		}
		features = append(features, feature)
	}

	geojson := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	}

	json.NewEncoder(w).Encode(geojson)
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

// handleGetAirportByCodeText returns a single airport as text
func (s *Server) handleGetAirportByCodeText(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	// Remove .txt extension if present
	code = strings.TrimSuffix(code, ".txt")

	airport, err := s.airports.GetByCode(code)
	if err != nil {
		s.respondText(w, http.StatusNotFound, fmt.Sprintf("Airport not found: %s", code))
		return
	}

	text := fmt.Sprintf("ICAO: %s\nIATA: %s\nName: %s\nCity: %s\nCountry: %s\nLatitude: %.6f\nLongitude: %.6f\nElevation: %d ft\nTimezone: %s\n",
		airport.ICAO, airport.IATA, airport.Name, airport.City, airport.Country,
		airport.Lat, airport.Lon, airport.Elevation, airport.Tz)

	s.respondText(w, http.StatusOK, text)
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

// handleSearchAirportsText returns search results as text
func (s *Server) handleSearchAirportsText(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	airports := s.airports.Search(query, limit, 0)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search: %s\nResults: %d\n\n", query, len(airports)))

	for _, a := range airports {
		sb.WriteString(fmt.Sprintf("%s (%s) - %s, %s\n", a.ICAO, a.IATA, a.Name, a.City))
	}

	s.respondText(w, http.StatusOK, sb.String())
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
		"airports":    airportsWithDist,
		"center":      map[string]float64{"lat": lat, "lon": lon},
		"radius":      displayRadius,
		"radius_unit": radiusUnit,
		"units":       units,
		"count":       len(airportsWithDist),
	})
}

// handleNearbyAirportsText returns nearby airports as text
func (s *Server) handleNearbyAirportsText(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")

	lat, _ := strconv.ParseFloat(latStr, 64)
	lon, _ := strconv.ParseFloat(lonStr, 64)
	radius := 50.0
	if radiusStr != "" {
		radius, _ = strconv.ParseFloat(radiusStr, 64)
	}

	airportsWithDist := s.airports.GetNearbyWithDistance(lat, lon, radius, 20, "metric")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Nearby airports (%.1f km from %.4f, %.4f)\n\n", radius, lat, lon))

	for _, a := range airportsWithDist {
		sb.WriteString(fmt.Sprintf("%s (%s) - %s (%.1f km)\n", a.ICAO, a.IATA, a.Name, a.Distance))
	}

	s.respondText(w, http.StatusOK, sb.String())
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

// handleGetCountriesText returns list of countries as text
func (s *Server) handleGetCountriesText(w http.ResponseWriter, r *http.Request) {
	countries := s.airports.GetCountries()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Countries with airports: %d\n\n", len(countries)))

	for _, c := range countries {
		sb.WriteString(fmt.Sprintf("%s\n", c))
	}

	s.respondText(w, http.StatusOK, sb.String())
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

// handleAirportStatsText returns statistics as text
func (s *Server) handleAirportStatsText(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()

	text := fmt.Sprintf("Airports Statistics\n\nTotal Airports: %d\nCountries: %d\n",
		stats["total_airports"], stats["countries"])

	s.respondText(w, http.StatusOK, text)
}

// handleGeoIPLookup looks up current request IP
func (s *Server) handleGeoIPLookup(w http.ResponseWriter, r *http.Request) {
	ipStr := getClientIP(r)

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

// handleGeoIPLookupText returns GeoIP lookup as text
func (s *Server) handleGeoIPLookupText(w http.ResponseWriter, r *http.Request) {
	ipStr := getClientIP(r)

	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondText(w, http.StatusBadRequest, fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	text := fmt.Sprintf("IP: %s\nCountry: %s\nCity: %s\nLatitude: %.4f\nLongitude: %.4f\n",
		ipStr, location.Country, location.City, location.Latitude, location.Longitude)

	s.respondText(w, http.StatusOK, text)
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

// handleGeoIPLookupIPText returns specific IP lookup as text
func (s *Server) handleGeoIPLookupIPText(w http.ResponseWriter, r *http.Request) {
	ipStr := chi.URLParam(r, "ip")
	ipStr = strings.TrimSuffix(ipStr, ".txt")

	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondText(w, http.StatusBadRequest, fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	text := fmt.Sprintf("IP: %s\nCountry: %s\nCity: %s\nLatitude: %.4f\nLongitude: %.4f\n",
		ipStr, location.Country, location.City, location.Latitude, location.Longitude)

	s.respondText(w, http.StatusOK, text)
}

// handleGeoIPNearbyAirports finds airports near IP location
func (s *Server) handleGeoIPNearbyAirports(w http.ResponseWriter, r *http.Request) {
	// Get IP to lookup
	ipStr := r.URL.Query().Get("ip")
	if ipStr == "" {
		ipStr = getClientIP(r)
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

// getClientIP extracts the real client IP from headers
func getClientIP(r *http.Request) string {
	ipStr := r.Header.Get("X-Forwarded-For")
	if ipStr == "" {
		ipStr = r.Header.Get("X-Real-IP")
	}
	if ipStr == "" {
		ipStr = r.RemoteAddr
	}

	// Handle X-Forwarded-For which can be comma-separated
	if strings.Contains(ipStr, ",") {
		ipStr = strings.TrimSpace(strings.Split(ipStr, ",")[0])
	}

	// Strip port
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}

	return ipStr
}
