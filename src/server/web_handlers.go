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
	s.renderTemplate(w, r, "home", data)
}

// handleSearch serves the search page. Reads "q" and "limit" from the query
// string so the page works as a plain GET form with JavaScript disabled
// (AI.md PART 16 "No JavaScript-Disabled Broken State") — the JS-enhanced
// search-results select posts back the same "limit" param.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var results []*airports.Airport

	if query != "" {
		results = s.airports.Search(query, limit, 0)
	}

	data := map[string]interface{}{
		"Title":   "Search",
		"Query":   query,
		"Limit":   limit,
		"Results": results,
		"MapJSON": mapPointsJSON(results),
	}
	s.renderTemplate(w, r, "search", data)
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

	bboxKeys := []string{"minLat", "maxLat", "minLon", "maxLon"}
	bboxRequested := true
	for _, key := range bboxKeys {
		if r.URL.Query().Get(key) == "" {
			bboxRequested = false
			break
		}
	}
	if bboxRequested {
		data["BBoxSearch"] = true
		values := make([]float64, len(bboxKeys))
		valid := true
		for i, key := range bboxKeys {
			value, err := strconv.ParseFloat(r.URL.Query().Get(key), 64)
			if err != nil {
				valid = false
				break
			}
			values[i] = value
		}
		if valid && values[0] >= -90 && values[1] <= 90 && values[0] <= values[1] &&
			values[2] >= -180 && values[3] <= 180 && values[2] <= values[3] {
			results := s.airports.GetInBoundingBox(values[0], values[1], values[2], values[3])
			total := len(results)
			limit := 100
			if limitStr != "" {
				if requested, err := strconv.Atoi(limitStr); err == nil && requested > 0 {
					limit = requested
				}
			}
			if limit > 100 {
				limit = 100
			}
			if len(results) > limit {
				results = results[:limit]
			}
			data["BBoxResults"] = results
			data["BBoxTotal"] = total
			data["MapJSON"] = mapPointsJSON(results)
			data["Limit"] = limit
		}
	} else if latStr != "" && lonStr != "" {
		lat, latErr := strconv.ParseFloat(latStr, 64)
		lon, lonErr := strconv.ParseFloat(lonStr, 64)
		radius := 50.0
		var radiusErr error
		if radiusStr != "" {
			radius, radiusErr = strconv.ParseFloat(radiusStr, 64)
		}
		if latErr == nil && lonErr == nil && radiusErr == nil &&
			lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 && radius >= 1 && radius <= 500 {
			limit := 20
			if limitStr != "" {
				if requested, err := strconv.Atoi(limitStr); err == nil && requested > 0 {
					limit = requested
				}
			}
			if limit > 100 {
				limit = 100
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
			data["MapJSON"] = nearbyMapPointsJSON(airportsNearby)
		}

	}

	s.renderTemplate(w, r, "nearby", data)
}

// handleAirportDetail serves the airport detail page
func (s *Server) handleAirportDetail(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "ident")

	airport, err := s.airports.GetByCode(code)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Title": code,
		"Code":  code,
	}

	// Calculate elevation in meters
	elevMeters := float64(airport.Elevation) * 0.3048

	// Convert airport to JSON for display
	jsonBytes, _ := json.MarshalIndent(airport, "", "  ")

	data["Airport"] = airport
	data["ElevationMeters"] = int(elevMeters)
	data["JSON"] = string(jsonBytes)
	data["MapJSON"] = mapPointsJSON([]*airports.Airport{airport})

	s.renderTemplate(w, r, "airport", data)
}

type airportMapPoint struct {
	Code string  `json:"code"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

func mapPointsJSON(airportsList []*airports.Airport) string {
	points := make([]airportMapPoint, 0, len(airportsList))
	for _, airport := range airportsList {
		if airport == nil {
			continue
		}
		code := airport.ICAO
		if code == "" {
			code = airport.IATA
		}
		points = append(points, airportMapPoint{Code: code, Name: airport.Name, Lat: airport.Lat, Lon: airport.Lon})
	}
	data, err := json.Marshal(points)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func nearbyMapPointsJSON(airportsList []airports.AirportWithDistance) string {
	points := make([]airportMapPoint, 0, len(airportsList))
	for _, airport := range airportsList {
		code := airport.ICAO
		if code == "" {
			code = airport.IATA
		}
		points = append(points, airportMapPoint{Code: code, Name: airport.Name, Lat: airport.Lat, Lon: airport.Lon})
	}
	data, err := json.Marshal(points)
	if err != nil {
		return "[]"
	}
	return string(data)
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

	s.renderTemplate(w, r, "stats", data)
}
