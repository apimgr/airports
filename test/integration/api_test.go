package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/server"
)

type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func setupTestServer(t *testing.T) *httptest.Server {
	airportSvc, err := airports.NewService()
	if err != nil {
		t.Fatalf("Failed to create airport service: %v", err)
	}

	// Use temp directory for GeoIP data in tests
	tmpDir := t.TempDir()
	geoipSvc, err := geoip.NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create geoip service: %v", err)
	}

	srv := server.New(airportSvc, geoipSvc, false)
	return httptest.NewServer(srv.Router())
}

func TestAirportEndpoints(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	tests := []struct {
		name       string
		endpoint   string
		wantStatus int
	}{
		{"Get JFK by ICAO", "/api/v1/airports/KJFK", http.StatusOK},
		{"Get JFK by IATA", "/api/v1/airports/JFK", http.StatusOK},
		{"Search airports", "/api/v1/airports/search?q=New+York", http.StatusOK},
		{"List airports", "/api/v1/airports?limit=10", http.StatusOK},
		{"Nearby airports", "/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50", http.StatusOK},
		{"Bounding box", "/api/v1/airports/bbox?minLat=40&maxLat=41&minLon=-74&maxLon=-73", http.StatusOK},
		{"Autocomplete", "/api/v1/airports/autocomplete?q=JFK", http.StatusOK},
		{"Get countries", "/api/v1/airports/countries", http.StatusOK},
		{"Get states", "/api/v1/airports/states/US", http.StatusOK},
		{"Airport stats", "/api/v1/airports/stats", http.StatusOK},
		{"Not found", "/api/v1/airports/NOTFOUND", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.endpoint)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			var apiResp Response
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tt.wantStatus == http.StatusOK && !apiResp.Success {
				t.Error("Expected success=true")
			}
		})
	}
}

func TestGeoIPEndpoints(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	tests := []struct {
		name       string
		endpoint   string
		wantStatus int
	}{
		{"Lookup 8.8.8.8", "/api/v1/geoip/8.8.8.8", http.StatusOK},
		{"Nearby airports by IP", "/api/v1/geoip/airports/nearby?ip=8.8.8.8", http.StatusOK},
		{"Invalid IP", "/api/v1/geoip/invalid", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.endpoint)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var apiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("Expected success=true")
	}
}

func TestFullDatabaseExport(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/airports.json")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected application/json content type")
	}

	// Verify it's a large response (should be several MB)
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if len(data) < 1000 {
		t.Errorf("Expected at least 1000 airports in export, got %d", len(data))
	}

	t.Logf("Exported %d airports", len(data))
}
