package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/apimgr/airports/src/airports"
	"github.com/go-chi/chi/v5"
)

// handleHome serves the homepage (now using template)
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Home",
	}
	s.renderTemplate(w, "base.html", data)
}

// handleSearch serves the search page
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	var results []*airports.Airport

	if query != "" {
		results = s.airports.Search(query, 100, 0)
	}

	data := map[string]interface{}{
		"Title":   "Search",
		"Query":   query,
		"Results": results,
	}
	s.renderTemplate(w, "base.html", data)
}

// handleNearby serves the nearby airports page
func (s *Server) handleNearby(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")
	limitStr := r.URL.Query().Get("limit")
	unitsParam := r.URL.Query().Get("units")

	data := map[string]interface{}{
		"Title": "Find Nearby",
		"Units": airports.ParseUnits(unitsParam),
	}

	if latStr != "" && lonStr != "" {
		lat, _ := strconv.ParseFloat(latStr, 64)
		lon, _ := strconv.ParseFloat(lonStr, 64)

		radius := 50.0
		if radiusStr != "" {
			radius, _ = strconv.ParseFloat(radiusStr, 64)
		}

		limit := 20
		if limitStr != "" {
			limit, _ = strconv.Atoi(limitStr)
		}

		units := airports.ParseUnits(unitsParam)
		airportsNearby := s.airports.GetNearbyWithDistance(lat, lon, radius, limit, units)

		displayRadius, radiusUnit := airports.ConvertDistance(radius, units)

		data["Lat"] = lat
		data["Lon"] = lon
		data["Radius"] = radius
		data["DisplayRadius"] = displayRadius
		data["RadiusUnit"] = radiusUnit
		data["Limit"] = limit
		data["Airports"] = airportsNearby
	}

	s.renderTemplate(w, "base.html", data)
}

// handleAirportDetail serves the airport detail page
func (s *Server) handleAirportDetail(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	airport, err := s.airports.GetByCode(code)

	data := map[string]interface{}{
		"Title": code,
		"Code":  code,
	}

	if err == nil {
		// Calculate elevation in meters
		elevMeters := float64(airport.Elevation) * 0.3048

		// Convert airport to JSON for display
		jsonBytes, _ := json.MarshalIndent(airport, "", "  ")

		data["Airport"] = airport
		data["ElevationMeters"] = int(elevMeters)
		data["JSON"] = string(jsonBytes)
	}

	s.renderTemplate(w, "base.html", data)
}

// handleStats serves the statistics page
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.airports.Stats()
	countries := s.airports.GetCountries()

	data := map[string]interface{}{
		"Title": "Statistics",
		"Stats": map[string]interface{}{
			"TotalAirports": stats["total_airports"],
			"Countries":     stats["countries"],
			"Cities":        stats["cities"],
			"WithIATA":      stats["with_iata"],
		},
		"Countries": countries,
	}

	s.renderTemplate(w, "base.html", data)
}
