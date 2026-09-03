package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/server"
)

// Sample dataset is shared with src/airports/testdata/ - read at test
// runtime instead of duplicating and go:embed'ing a second copy here.
func loadTestAirportsJSON(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../src/airports/testdata/airports_sample.json")
	if err != nil {
		t.Fatalf("Failed to read shared testdata: %v", err)
	}
	return data
}

func setupTestServer(t *testing.T) *httptest.Server {
	if testing.Short() {
		t.Skip("integration tests require network: downloads GeoIP databases")
	}

	airportSvc, err := airports.NewService(loadTestAirportsJSON(t))
	if err != nil {
		t.Fatalf("Failed to create airport service: %v", err)
	}

	// Use temp directory for GeoIP data in tests
	tmpDir := t.TempDir()
	geoipSvc, err := geoip.NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create geoip service: %v", err)
	}

	srv := server.New(airportSvc, geoipSvc, &config.Config{}, nil, nil, tmpDir, tmpDir, nil)
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
		{"Search airports", "/api/v1/search?q=New+York", http.StatusOK},
		{"List airports", "/api/v1/airports?limit=10", http.StatusOK},
		{"Nearby airports", "/api/v1/nearby?lat=40.6398&lon=-73.7789&radius=50", http.StatusOK},
		{"Bounding box", "/api/v1/bbox?minLat=40&maxLat=41&minLon=-74&maxLon=-73", http.StatusOK},
		{"Autocomplete", "/api/v1/autocomplete?q=JFK", http.StatusOK},
		{"Get countries", "/api/v1/countries", http.StatusOK},
		{"Get states", "/api/v1/states/US", http.StatusOK},
		{"Airport stats", "/api/v1/stats", http.StatusOK},
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

			// Verify response is valid JSON
			var raw map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
				t.Fatalf("Failed to decode response as JSON: %v", err)
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

	// /healthz is an opt-in root alias (server.healthz.root.enabled,
	// AI.md PART 13) that setupTestServer's zero-value config leaves
	// disabled; /server/healthz is the always-on canonical endpoint.
	resp, err := http.Get(ts.URL + "/server/healthz")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// setupTestServer wires no db/scheduler, so checks.database and
	// checks.scheduler both legitimately report "error" here (AI.md
	// PART 13: real dependency probes, never hardcoded) — this harness
	// only asserts the response is well-formed JSON with a valid status
	// enum value, not a specific status.
	var healthResp struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	switch healthResp.Status {
	case "healthy", "degraded", "unhealthy":
	default:
		t.Errorf("status = %q, want one of healthy/degraded/unhealthy", healthResp.Status)
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

	if len(data) == 0 {
		t.Errorf("Expected airports in export, got empty response")
	}

	t.Logf("Exported %d airports", len(data))
}
