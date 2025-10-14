package airports

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

//go:embed data/airports.json
var airportDataJSON []byte

// Airport represents a single airport's data
type Airport struct {
	ICAO      string  `json:"icao"`
	IATA      string  `json:"iata"`
	Name      string  `json:"name"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	Country   string  `json:"country"`
	Elevation int     `json:"elevation"` // In feet (source data)
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Tz        string  `json:"tz"`
}

// AirportWithDistance includes distance from search point
type AirportWithDistance struct {
	Airport
	Distance     float64 `json:"distance"`
	DistanceUnit string  `json:"distance_unit"`
}

// AirportDatabase is the in-memory airport data (map key is ICAO code)
type AirportDatabase map[string]Airport

// AirportIndexes provides fast lookup capabilities
type AirportIndexes struct {
	ByICAO    map[string]*Airport
	ByIATA    map[string][]*Airport
	ByCity    map[string][]*Airport
	ByCountry map[string][]*Airport
	ByState   map[string][]*Airport
	mu        sync.RWMutex
}

// Service manages airport data and lookups
type Service struct {
	data    AirportDatabase
	indexes *AirportIndexes
}

// NewService loads and indexes all airport data
func NewService() (*Service, error) {
	data, err := LoadAirports()
	if err != nil {
		return nil, fmt.Errorf("failed to load airports: %w", err)
	}

	indexes := BuildIndexes(data)

	return &Service{
		data:    data,
		indexes: indexes,
	}, nil
}

// LoadAirports reads and parses the embedded airports.json
func LoadAirports() (AirportDatabase, error) {
	var airports AirportDatabase
	err := json.Unmarshal(airportDataJSON, &airports)
	if err != nil {
		return nil, fmt.Errorf("failed to parse airports.json: %w", err)
	}
	return airports, nil
}

// BuildIndexes creates fast lookup indexes for airports
func BuildIndexes(airports AirportDatabase) *AirportIndexes {
	indexes := &AirportIndexes{
		ByICAO:    make(map[string]*Airport),
		ByIATA:    make(map[string][]*Airport),
		ByCity:    make(map[string][]*Airport),
		ByCountry: make(map[string][]*Airport),
		ByState:   make(map[string][]*Airport),
	}

	for icao, airport := range airports {
		// Create a copy to avoid pointer issues
		apt := airport

		// Index by ICAO (primary key)
		indexes.ByICAO[strings.ToUpper(icao)] = &apt

		// Index by IATA (can be empty or duplicate)
		if apt.IATA != "" {
			iata := strings.ToUpper(apt.IATA)
			indexes.ByIATA[iata] = append(indexes.ByIATA[iata], &apt)
		}

		// Index by city (case-insensitive)
		if apt.City != "" {
			city := strings.ToLower(apt.City)
			indexes.ByCity[city] = append(indexes.ByCity[city], &apt)
		}

		// Index by country
		if apt.Country != "" {
			country := strings.ToUpper(apt.Country)
			indexes.ByCountry[country] = append(indexes.ByCountry[country], &apt)
		}

		// Index by state (case-insensitive)
		if apt.State != "" {
			state := strings.ToLower(apt.State)
			indexes.ByState[state] = append(indexes.ByState[state], &apt)
		}
	}

	return indexes
}

// GetByCode looks up an airport by ICAO or IATA code
func (s *Service) GetByCode(code string) (*Airport, error) {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	code = strings.ToUpper(code)

	// Try ICAO first
	if apt, ok := s.indexes.ByICAO[code]; ok {
		return apt, nil
	}

	// Try IATA
	if apts, ok := s.indexes.ByIATA[code]; ok && len(apts) > 0 {
		return apts[0], nil
	}

	return nil, fmt.Errorf("airport not found: %s", code)
}

// Search finds airports matching the query across multiple fields
func (s *Service) Search(query string, limit, offset int) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	results := make(map[string]*Airport)

	// Search in codes (exact match priority)
	queryUpper := strings.ToUpper(query)
	if apt, ok := s.indexes.ByICAO[queryUpper]; ok {
		results[apt.ICAO] = apt
	}
	if apts, ok := s.indexes.ByIATA[queryUpper]; ok {
		for _, apt := range apts {
			results[apt.ICAO] = apt
		}
	}

	// Search in names and cities (partial match)
	for _, apt := range s.data {
		if _, found := results[apt.ICAO]; found {
			continue
		}

		nameLower := strings.ToLower(apt.Name)
		cityLower := strings.ToLower(apt.City)

		if strings.Contains(nameLower, query) || strings.Contains(cityLower, query) {
			aptCopy := apt
			results[apt.ICAO] = &aptCopy
		}
	}

	// Convert to slice
	airports := make([]*Airport, 0, len(results))
	for _, apt := range results {
		airports = append(airports, apt)
	}

	// Sort by name
	sort.Slice(airports, func(i, j int) bool {
		return airports[i].Name < airports[j].Name
	})

	// Apply pagination
	if offset >= len(airports) {
		return []*Airport{}
	}
	end := offset + limit
	if end > len(airports) {
		end = len(airports)
	}

	return airports[offset:end]
}

// GetByCity returns all airports in a city
func (s *Service) GetByCity(city string) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	cityLower := strings.ToLower(city)
	return s.indexes.ByCity[cityLower]
}

// GetByCountry returns all airports in a country
func (s *Service) GetByCountry(country string) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	countryUpper := strings.ToUpper(country)
	return s.indexes.ByCountry[countryUpper]
}

// GetByState returns all airports in a state
func (s *Service) GetByState(state string) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	stateLower := strings.ToLower(state)
	return s.indexes.ByState[stateLower]
}

// GetNearby finds airports within radius (km) of coordinates
func (s *Service) GetNearby(lat, lon, radiusKm float64, limit int) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	type result struct {
		airport  *Airport
		distance float64
	}

	results := []result{}

	for _, apt := range s.data {
		dist := haversine(lat, lon, apt.Lat, apt.Lon)
		if dist <= radiusKm {
			aptCopy := apt
			results = append(results, result{&aptCopy, dist})
		}
	}

	// Sort by distance
	sort.Slice(results, func(i, j int) bool {
		return results[i].distance < results[j].distance
	})

	// Apply limit
	if limit > len(results) {
		limit = len(results)
	}

	airports := make([]*Airport, limit)
	for i := 0; i < limit; i++ {
		airports[i] = results[i].airport
	}

	return airports
}

// GetNearbyWithDistance finds airports within radius and includes distance information
func (s *Service) GetNearbyWithDistance(lat, lon, radiusKm float64, limit int, units string) []AirportWithDistance {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	type result struct {
		airport  Airport
		distance float64
	}

	results := []result{}

	for _, apt := range s.data {
		dist := haversine(lat, lon, apt.Lat, apt.Lon)
		if dist <= radiusKm {
			results = append(results, result{apt, dist})
		}
	}

	// Sort by distance
	sort.Slice(results, func(i, j int) bool {
		return results[i].distance < results[j].distance
	})

	// Apply limit
	if limit > len(results) {
		limit = len(results)
	}

	// Convert distances based on unit system
	airports := make([]AirportWithDistance, limit)
	for i := 0; i < limit; i++ {
		distance, unit := ConvertDistance(results[i].distance, units)
		airports[i] = AirportWithDistance{
			Airport:      results[i].airport,
			Distance:     distance,
			DistanceUnit: unit,
		}
	}

	return airports
}

// GetInBoundingBox finds airports within geographic bounds
func (s *Service) GetInBoundingBox(minLat, maxLat, minLon, maxLon float64) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	results := []*Airport{}

	for _, apt := range s.data {
		if apt.Lat >= minLat && apt.Lat <= maxLat &&
			apt.Lon >= minLon && apt.Lon <= maxLon {
			aptCopy := apt
			results = append(results, &aptCopy)
		}
	}

	return results
}

// GetAll returns all airports (paginated)
func (s *Service) GetAll(limit, offset int) []*Airport {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	airports := make([]*Airport, 0, len(s.data))
	for _, apt := range s.data {
		aptCopy := apt
		airports = append(airports, &aptCopy)
	}

	// Sort by ICAO code
	sort.Slice(airports, func(i, j int) bool {
		return airports[i].ICAO < airports[j].ICAO
	})

	// Apply pagination
	if offset >= len(airports) {
		return []*Airport{}
	}
	end := offset + limit
	if end > len(airports) {
		end = len(airports)
	}

	return airports[offset:end]
}

// GetCountries returns list of all countries with airport counts
func (s *Service) GetCountries() map[string]int {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	counts := make(map[string]int)
	for country, airports := range s.indexes.ByCountry {
		counts[country] = len(airports)
	}
	return counts
}

// GetStatesInCountry returns list of states in a country with counts
func (s *Service) GetStatesInCountry(country string) map[string]int {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	countryUpper := strings.ToUpper(country)
	airports, ok := s.indexes.ByCountry[countryUpper]
	if !ok {
		return map[string]int{}
	}

	counts := make(map[string]int)
	for _, apt := range airports {
		if apt.State != "" {
			counts[apt.State]++
		}
	}
	return counts
}

// Stats returns database statistics
func (s *Service) Stats() map[string]interface{} {
	s.indexes.mu.RLock()
	defer s.indexes.mu.RUnlock()

	return map[string]interface{}{
		"total_airports": len(s.data),
		"countries":      len(s.indexes.ByCountry),
		"cities":         len(s.indexes.ByCity),
		"with_iata":      len(s.indexes.ByIATA),
	}
}

// GetRawData returns the complete airport database as JSON
func (s *Service) GetRawData() AirportDatabase {
	return s.data
}

// Unit conversion constants
const (
	KmToMiles      = 0.621371
	MilesToKm      = 1.60934
	MetersToFeet   = 3.28084
	FeetToMeters   = 0.3048
)

// Unit system types
const (
	UnitImperial = "imperial"
	UnitMetric   = "metric"
)

// haversine calculates the distance between two points on Earth (in km)
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // km

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// ConvertDistance converts km to the specified unit system
func ConvertDistance(distanceKm float64, units string) (float64, string) {
	if units == UnitMetric {
		return distanceKm, "km"
	}
	// Default to imperial
	return distanceKm * KmToMiles, "mi"
}

// ConvertElevation converts feet to the specified unit system
func ConvertElevation(elevationFeet int, units string) (float64, string) {
	if units == UnitMetric {
		return float64(elevationFeet) * FeetToMeters, "m"
	}
	// Default to imperial
	return float64(elevationFeet), "ft"
}

// ParseUnits normalizes unit parameter to imperial or metric
func ParseUnits(unitParam string) string {
	switch strings.ToLower(unitParam) {
	case "metric", "m", "km", "kilometers":
		return UnitMetric
	default:
		return UnitImperial
	}
}
