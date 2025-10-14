package airports

import (
	"testing"
)

func TestLoadAirports(t *testing.T) {
	data, err := LoadAirports()
	if err != nil {
		t.Fatalf("Failed to load airports: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("No airports loaded")
	}

	t.Logf("Loaded %d airports", len(data))
}

func TestBuildIndexes(t *testing.T) {
	data, err := LoadAirports()
	if err != nil {
		t.Fatalf("Failed to load airports: %v", err)
	}

	indexes := BuildIndexes(data)

	if len(indexes.ByICAO) == 0 {
		t.Error("No ICAO indexes built")
	}

	if len(indexes.ByCountry) == 0 {
		t.Error("No country indexes built")
	}

	t.Logf("Built indexes: %d ICAO, %d countries", len(indexes.ByICAO), len(indexes.ByCountry))
}

func TestNewService(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	stats := svc.Stats()
	total, ok := stats["total_airports"].(int)
	if !ok || total == 0 {
		t.Error("No airports in service")
	}

	t.Logf("Service initialized with %d airports", total)
}

func TestGetByCode(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		code     string
		expected string
	}{
		{"KJFK", "John F Kennedy International Airport"},
		{"JFK", "John F Kennedy International Airport"},
		{"KLAX", "Los Angeles International Airport"},
		{"LAX", "Los Angeles International Airport"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			airport, err := svc.GetByCode(tt.code)
			if err != nil {
				t.Errorf("Failed to get airport %s: %v", tt.code, err)
				return
			}

			if airport.Name != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, airport.Name)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		query    string
		minCount int
	}{
		{"New York", 1},
		{"JFK", 1},
		{"International", 100},
		{"Airport", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := svc.Search(tt.query, 1000, 0)
			if len(results) < tt.minCount {
				t.Errorf("Expected at least %d results, got %d", tt.minCount, len(results))
			}
		})
	}
}

func TestGetNearby(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// JFK coordinates
	results := svc.GetNearby(40.6398, -73.7789, 50, 10)

	if len(results) == 0 {
		t.Error("No nearby airports found")
		return
	}

	// First result should be JFK itself
	if results[0].ICAO != "KJFK" {
		t.Logf("Closest airport is %s (%s), not JFK", results[0].ICAO, results[0].Name)
	}

	t.Logf("Found %d airports within 50km of JFK", len(results))
}

func TestGetInBoundingBox(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Box around New York area
	results := svc.GetInBoundingBox(40.0, 41.0, -74.0, -73.0)

	if len(results) == 0 {
		t.Error("No airports in bounding box")
		return
	}

	t.Logf("Found %d airports in NYC area bounding box", len(results))
}

func TestHaversine(t *testing.T) {
	// Test distance between JFK and LAX (known distance ~3974 km)
	distance := haversine(40.6398, -73.7789, 33.9416, -118.4085)

	if distance < 3900 || distance > 4000 {
		t.Errorf("Expected ~3974 km between JFK and LAX, got %.2f km", distance)
	}

	t.Logf("Distance JFK to LAX: %.2f km", distance)
}

func BenchmarkLoadAirports(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := LoadAirports()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	svc, err := NewService()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Search("New York", 50, 0)
	}
}

func BenchmarkGetByCode(b *testing.B) {
	svc, err := NewService()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetByCode("KJFK")
	}
}

func BenchmarkGetNearby(b *testing.B) {
	svc, err := NewService()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetNearby(40.6398, -73.7789, 50, 10)
	}
}
