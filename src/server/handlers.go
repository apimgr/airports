package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/mode"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()

	health := map[string]interface{}{
		"status":  "healthy",
		"version": Version,
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

	s.respondItem(w, http.StatusOK, health)
}

// handleGetAirports returns paginated list of airports
func (s *Server) handleGetAirports(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	if page < 1 {
		page = 1
	}
	stats := s.airports.Stats()
	total, _ := stats["total_airports"].(int)

	// Guard against integer overflow and out-of-range pages: a very large
	// page value makes (page-1)*limit overflow to a negative int, which would
	// panic the slice inside GetAll. Compute the offset in 64-bit space and,
	// when it lands outside [0, total), return a valid empty page (HTTP 200)
	// with correct pagination metadata instead of panicking.
	offset := int64(page-1) * int64(limit)
	results := []*airports.Airport{}
	if offset >= 0 && offset < int64(total) {
		results = s.airports.GetAll(limit, int(offset))
	}
	s.respondList(w, http.StatusOK, results, page, limit, total)
}

// handleGetAirportsJSON returns the full airport database as JSON
func (s *Server) handleGetAirportsJSON(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportExportTotal.WithLabelValues("json").Inc()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=airports.json")

	data := s.airports.GetRawData()
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("handleGetAirportsJSON: encode failed: %v", err)
	}
}

// handleGetAirportsCSV returns the full airport database as CSV
func (s *Server) handleGetAirportsCSV(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportExportTotal.WithLabelValues("csv").Inc()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=airports.csv")

	writer := csv.NewWriter(w)

	// Write header
	_ = writer.Write([]string{"icao", "iata", "name", "city", "state", "country", "elevation", "lat", "lon", "tz"})

	// Write data
	airports := s.airports.GetAll(100000, 0) // Get all
	for _, a := range airports {
		_ = writer.Write([]string{
			a.ICAO, a.IATA, a.Name, a.City, a.State, a.Country,
			fmt.Sprintf("%d", a.Elevation),
			fmt.Sprintf("%.6f", a.Lat),
			fmt.Sprintf("%.6f", a.Lon),
			a.Tz,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("handleGetAirportsCSV: write failed: %v", err)
	}
}

// handleGetAirportsGeoJSON returns the full airport database as GeoJSON
func (s *Server) handleGetAirportsGeoJSON(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportExportTotal.WithLabelValues("geojson").Inc()
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

	if err := json.NewEncoder(w).Encode(geojson); err != nil {
		log.Printf("handleGetAirportsGeoJSON: encode failed: %v", err)
	}
}

// handleGetAirportByIdent returns a single airport by ICAO/IATA ident
func (s *Server) handleGetAirportByIdent(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportLookupTotal.WithLabelValues("json").Inc()
	ident := chi.URLParam(r, "ident")

	airport, err := s.airports.GetByCode(ident)
	if err != nil {
		s.respondError(w, r, http.StatusNotFound, "NOT_FOUND", "errors.airport_not_found", "ident", ident)
		return
	}

	s.respondItem(w, http.StatusOK, airport)
}

// handleGetAirportByIdentText returns a single airport as plain text
func (s *Server) handleGetAirportByIdentText(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportLookupTotal.WithLabelValues("text").Inc()
	code := chi.URLParam(r, "ident")
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
	s.metrics.airportSearchTotal.WithLabelValues("json").Inc()
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	total := s.airports.SearchCount(query)

	// Guard against integer overflow and out-of-range pages: a very large
	// page value makes (page-1)*limit overflow to a negative int, which would
	// panic the slice inside Search. Compute the offset in 64-bit space and,
	// when it lands outside [0, total), return a valid empty page (HTTP 200)
	// with correct pagination metadata instead of panicking.
	offset := int64(page-1) * int64(limit)
	results := []*airports.Airport{}
	if offset >= 0 && offset < int64(total) {
		results = s.airports.Search(query, limit, int(offset))
	}
	s.respondList(w, http.StatusOK, results, page, limit, total)
}

// handleSearchAirportsText returns search results as plain text
func (s *Server) handleSearchAirportsText(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportSearchTotal.WithLabelValues("text").Inc()
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
	s.metrics.airportNearbyTotal.WithLabelValues("json").Inc()
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")
	limitStr := r.URL.Query().Get("limit")
	unitsParam := r.URL.Query().Get("units")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_latitude")
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_longitude")
		return
	}

	// Reject non-finite or out-of-range coordinates: ParseFloat accepts
	// "NaN"/"Inf", so a successful parse is not enough — a bad coordinate here
	// would produce nonsensical distance math.
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_latitude")
		return
	}
	if math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_longitude")
		return
	}

	radius := 50.0
	if radiusStr != "" {
		radius, _ = strconv.ParseFloat(radiusStr, 64)
	}
	// Bounds-check the user-supplied radius: reject non-positive/NaN values
	// (parse failures leave radius at 0) and cap the upper bound so an
	// oversized query can't scan the whole dataset per request.
	if !(radius > 0) {
		radius = 50
	}
	if radius > 500 {
		radius = 500
	}

	limit := 20
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	// Bounds-check limit: parse failures/negatives fall back to the default,
	// and the count is capped so a single request stays cheap.
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Parse unit system (default: imperial)
	units := airports.ParseUnits(unitsParam)

	// Get airports with distance information
	airportsWithDist := s.airports.GetNearbyWithDistance(lat, lon, radius, limit, units)

	// Convert radius for display
	displayRadius, radiusUnit := airports.ConvertDistance(radius, units)

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"data":        airportsWithDist,
		"center":      map[string]float64{"lat": lat, "lon": lon},
		"radius":      displayRadius,
		"radius_unit": radiusUnit,
		"units":       units,
		"count":       len(airportsWithDist),
	})
}

// handleNearbyAirportsText returns nearby airports as plain text
func (s *Server) handleNearbyAirportsText(w http.ResponseWriter, r *http.Request) {
	s.metrics.airportNearbyTotal.WithLabelValues("text").Inc()
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		s.respondText(w, http.StatusBadRequest, "Invalid latitude: must be a number in [-90, 90]\n")
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil || math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
		s.respondText(w, http.StatusBadRequest, "Invalid longitude: must be a number in [-180, 180]\n")
		return
	}
	radius := 50.0
	if radiusStr != "" {
		radius, _ = strconv.ParseFloat(radiusStr, 64)
	}
	// Bounds-check radius (parse failures leave it 0): keep it positive and
	// capped so the query stays bounded.
	if !(radius > 0) {
		radius = 50
	}
	if radius > 500 {
		radius = 500
	}

	airportsWithDist := s.airports.GetNearbyWithDistance(lat, lon, radius, 20, "metric")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Nearby airports (%.1f km from %.4f, %.4f)\n\n", radius, lat, lon))

	for _, a := range airportsWithDist {
		sb.WriteString(fmt.Sprintf("%s (%s) - %s (%.1f km)\n", a.ICAO, a.IATA, a.Name, a.Distance))
	}

	s.respondText(w, http.StatusOK, sb.String())
}

// handleBBoxAirports finds airports in bounding box.
// Accepts both documented parameter namings: the swagger/api-info form
// (lat_min/lat_max/lon_min/lon_max) and the docs/api.md form
// (minLat/maxLat/minLon/maxLon), so a client following either works.
func (s *Server) handleBBoxAirports(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bound := func(canonical, legacy string) float64 {
		v := q.Get(canonical)
		if v == "" {
			v = q.Get(legacy)
		}
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	minLat := bound("lat_min", "minLat")
	maxLat := bound("lat_max", "maxLat")
	minLon := bound("lon_min", "minLon")
	maxLon := bound("lon_max", "maxLon")

	results := s.airports.GetInBoundingBox(minLat, maxLat, minLon, maxLon)

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"data":  results,
		"count": len(results),
	})
}

// handleAutocomplete provides autocomplete suggestions
func (s *Server) handleAutocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if len(query) < 2 {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.query_too_short")
		return
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	results := s.airports.Search(query, limit, 0)

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"suggestions": results,
		"query":       query,
	})
}

// handleGetCountries returns list of countries
func (s *Server) handleGetCountries(w http.ResponseWriter, r *http.Request) {
	countries := s.airports.GetCountries()
	s.respondItem(w, http.StatusOK, countries)
}

// handleGetCountriesText returns list of countries as text
func (s *Server) handleGetCountriesText(w http.ResponseWriter, r *http.Request) {
	countries := s.airports.GetCountries()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Countries with airports: %d\n\n", len(countries)))

	for name, count := range countries {
		sb.WriteString(fmt.Sprintf("%s\t%d\n", name, count))
	}

	s.respondText(w, http.StatusOK, sb.String())
}

// handleGetStates returns list of states in a country
func (s *Server) handleGetStates(w http.ResponseWriter, r *http.Request) {
	country := chi.URLParam(r, "country")
	states := s.airports.GetStatesInCountry(country)
	s.respondItem(w, http.StatusOK, states)
}

// handleAirportStats returns database statistics
func (s *Server) handleAirportStats(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	s.respondItem(w, http.StatusOK, stats)
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
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_ip")
		return
	}

	location, err := s.geoip.Lookup(ip)
	if err != nil {
		log.Printf("handleGeoIPLookup: lookup failed for %s: %v", ipStr, err)
		s.respondError(w, r, http.StatusInternalServerError, "SERVER_ERROR", "errors.internal")
		return
	}

	s.respondItem(w, http.StatusOK, location)
}

// handleGeoIPLookupText returns GeoIP lookup as text
func (s *Server) handleGeoIPLookupText(w http.ResponseWriter, r *http.Request) {
	ipStr := getClientIP(r)

	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondText(w, http.StatusBadRequest, fmt.Sprintf("Error: %s", mode.GetErrorDetail(err)))
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
		log.Printf("handleGeoIPLookupIP: lookup failed for %s: %v", ipStr, err)
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_ip")
		return
	}

	s.respondItem(w, http.StatusOK, location)
}

// handleGeoIPLookupIPText returns specific IP lookup as text
func (s *Server) handleGeoIPLookupIPText(w http.ResponseWriter, r *http.Request) {
	ipStr := chi.URLParam(r, "ip")
	ipStr = strings.TrimSuffix(ipStr, ".txt")

	location, err := s.geoip.LookupString(ipStr)
	if err != nil {
		s.respondText(w, http.StatusBadRequest, fmt.Sprintf("Error: %s", mode.GetErrorDetail(err)))
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
		log.Printf("handleGeoIPNearbyAirports: lookup failed for %s: %v", ipStr, err)
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.invalid_ip")
		return
	}

	// Get radius, limit, and units
	radius := 100.0
	if r.URL.Query().Get("radius") != "" {
		radius, _ = strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
	}
	// Bounds-check radius (parse failures leave it 0): keep it positive and
	// capped so the query stays bounded.
	if !(radius > 0) {
		radius = 100
	}
	if radius > 500 {
		radius = 500
	}

	limit := 10
	if r.URL.Query().Get("limit") != "" {
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	}
	// Bounds-check limit: parse failures/negatives fall back to the default,
	// and the count is capped so a single request stays cheap.
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	unitsParam := r.URL.Query().Get("units")
	units := airports.ParseUnits(unitsParam)

	// Find nearby airports with distance
	airportsNearby := s.airports.GetNearbyWithDistance(location.Latitude, location.Longitude, radius, limit, units)

	// Convert radius for display
	displayRadius, radiusUnit := airports.ConvertDistance(radius, units)

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"location":        location,
		"nearby_airports": airportsNearby,
		"radius":          displayRadius,
		"radius_unit":     radiusUnit,
		"units":           units,
	})
}

// getClientIP extracts the real client IP. It prefers the IP resolved by
// middleware.ClientIPFromXFF (walked back past trusted proxy hops per
// AI.md PART 12 "Trusted Proxies"); if the request context has no resolved
// value — meaning the request did not come through a recognized trusted
// proxy — it falls back to the raw TCP peer address (r.RemoteAddr), which
// IS the real client in that case. X-Forwarded-For/X-Real-IP are never
// trusted directly here, since that would let any client spoof its IP.
func getClientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}

	ipStr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}

	return ipStr
}
