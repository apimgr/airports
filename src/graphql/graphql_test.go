package graphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/geoip"
)

// fakeAirportSource is a minimal in-memory AirportSource used to exercise
// the resolvers without depending on the real airports.Service.
type fakeAirportSource struct {
	byCode    map[string]*airports.Airport
	nearby    []airports.AirportWithDistance
	all       []*airports.Airport
	search    []*airports.Airport
	bbox      []*airports.Airport
	countries map[string]int
	states    map[string]int
	stats     map[string]interface{}
}

func (f *fakeAirportSource) GetByCode(code string) (*airports.Airport, error) {
	apt, ok := f.byCode[code]
	if !ok {
		return nil, errors.New("not found")
	}
	return apt, nil
}

func (f *fakeAirportSource) GetNearbyWithDistance(lat, lon, radiusKm float64, limit int, units string) []airports.AirportWithDistance {
	return f.nearby
}

func (f *fakeAirportSource) GetAll(limit, offset int) []*airports.Airport {
	return f.all
}

func (f *fakeAirportSource) Search(query string, limit, offset int) []*airports.Airport {
	return f.search
}

func (f *fakeAirportSource) GetInBoundingBox(minLat, maxLat, minLon, maxLon float64) []*airports.Airport {
	return f.bbox
}

func (f *fakeAirportSource) GetCountries() map[string]int {
	return f.countries
}

func (f *fakeAirportSource) GetStatesInCountry(country string) map[string]int {
	return f.states
}

func (f *fakeAirportSource) Stats() map[string]interface{} {
	return f.stats
}

// fakeGeoIPSource is a minimal in-memory GeoIPSource used to exercise the
// `geoip` resolver without depending on the real geoip.Service (and its
// MaxMind database files).
type fakeGeoIPSource struct {
	byIP map[string]*geoip.GeoLocation
}

func (f *fakeGeoIPSource) LookupString(ipStr string) (*geoip.GeoLocation, error) {
	loc, ok := f.byIP[ipStr]
	if !ok {
		return nil, errors.New("not found")
	}
	return loc, nil
}

func newFakeSource() *fakeAirportSource {
	jfk := &airports.Airport{ICAO: "KJFK", IATA: "JFK", Name: "John F Kennedy International Airport", City: "New York", Country: "US"}
	return &fakeAirportSource{
		byCode: map[string]*airports.Airport{
			"KJFK": jfk,
		},
		nearby: []airports.AirportWithDistance{
			{Airport: airports.Airport{ICAO: "KJFK", IATA: "JFK", Name: "John F Kennedy International Airport"}, Distance: 1.2, DistanceUnit: "km"},
		},
		all:    []*airports.Airport{jfk},
		search: []*airports.Airport{jfk},
		bbox:   []*airports.Airport{jfk},
		countries: map[string]int{
			"US": 1,
			"CA": 2,
		},
		states: map[string]int{
			"NY": 1,
		},
		stats: map[string]interface{}{
			"total_airports": 1,
			"countries":      1,
			"cities":         1,
			"with_iata":      1,
		},
	}
}

func newFakeGeoIPSource() *fakeGeoIPSource {
	return &fakeGeoIPSource{
		byIP: map[string]*geoip.GeoLocation{
			"1.2.3.4": {IP: "1.2.3.4", Country: "US", CountryName: "United States", City: "New York"},
		},
	}
}

// TestAssetsHandler covers the embedded-assets file server, including the
// prefix-stripping behavior and the 404 case for an unknown asset.
func TestAssetsHandler(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		path       string
		wantStatus int
	}{
		{"known asset", "/assets/", "/assets/graphiql.min.css", http.StatusOK},
		{"unknown asset", "/assets/", "/assets/does-not-exist.js", http.StatusNotFound},
		{"root under prefix serves directory listing", "/assets/", "/assets/", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := AssetsHandler(tt.prefix)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// TestUIHandler verifies the GraphiQL HTML page renders with the correct
// content type and embeds the supplied endpoint/asset prefix values.
func TestUIHandler(t *testing.T) {
	h := UIHandler("/api/graphql", "/server/docs/graphql/assets/")
	req := httptest.NewRequest(http.MethodGet, "/server/docs/graphql", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api/graphql") {
		t.Error("body missing endpoint path")
	}
	if !strings.Contains(body, "/server/docs/graphql/assets/graphiql.min.css") {
		t.Error("body missing assets prefix in stylesheet link")
	}
}

// TestQueryHandler_AirportQuery covers the `airport(code:)` resolver path,
// both found and not-found cases.
func TestQueryHandler_AirportQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "found",
			query:      `{"query":"query { airport(code: \"KJFK\") { icao name } }"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "not found",
			query:      `{"query":"query { airport(code: \"ZZZZ\") { icao } }"}`,
			wantStatus: http.StatusOK,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			gotError := len(resp.Errors) > 0
			if gotError != tt.wantError {
				t.Errorf("gotError = %v, want %v (errors: %+v)", gotError, tt.wantError, resp.Errors)
			}
			if !tt.wantError && resp.Data == nil {
				t.Error("expected data, got nil")
			}
		})
	}
}

// TestQueryHandler_NearbyQuery covers the `nearby(lat:, lon:, radius:)`
// resolver, including the default-radius branch and the missing-lon error.
func TestQueryHandler_NearbyQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "lat and lon and radius",
			query:      `{"query":"query { nearby(lat: 40.64, lon: -73.78, radius: 25) { icao distance } }"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "lat and lon without radius uses default",
			query:      `{"query":"query { nearby(lat: 40.64, lon: -73.78) { icao } }"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "lat without lon is an error",
			query:      `{"query":"query { nearby(lat: 40.64) { icao } }"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			gotError := len(resp.Errors) > 0
			if gotError != tt.wantError {
				t.Errorf("gotError = %v, want %v (errors: %+v)", gotError, tt.wantError, resp.Errors)
			}
		})
	}
}

// TestQueryHandler_Errors covers request-level error branches: malformed
// JSON, empty query, unknown fields, and an unsupported query shape.
func TestQueryHandler_Errors(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"malformed json", `{"query":`, http.StatusBadRequest},
		{"empty query", `{"query":"   "}`, http.StatusBadRequest},
		{"missing query field", `{}`, http.StatusBadRequest},
		{"unknown field rejected", `{"query":"{ airport(code: \"KJFK\") }","bogus":1}`, http.StatusBadRequest},
		{"unsupported query", `{"query":"{ somethingElse }"}`, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestQueryHandler_ContentType ensures every response is JSON, regardless
// of success or error branch.
func TestQueryHandler_ContentType(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(`{"query":"{ airport(code: \"KJFK\") { icao } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestQueryHandler_Idempotent verifies calling the handler twice with the
// same input yields the same result — the resolver has no hidden state.
func TestQueryHandler_Idempotent(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	body := `{"query":"{ airport(code: \"KJFK\") { icao name } }"}`

	var first, second string
	for i, dst := range []*string{&first, &second} {
		req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		*dst = rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200", i, rec.Code)
		}
	}
	if first != second {
		t.Errorf("responses differ between calls:\n1: %s\n2: %s", first, second)
	}
}

// TestQueryHandler_AirportsQuery covers the `airports(limit:, page:)`
// paginated-list resolver, including the default-args branch.
func TestQueryHandler_AirportsQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name  string
		query string
	}{
		{"with args", `{"query":"{ airports(limit: 10, page: 1) { icao } }"}`},
		{"without args uses defaults", `{"query":"{ airports { icao } }"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(resp.Errors) > 0 {
				t.Errorf("unexpected errors: %+v", resp.Errors)
			}
			if resp.Data == nil {
				t.Error("expected data, got nil")
			}
		})
	}
}

// TestQueryHandler_SearchQuery covers the `search(q:, limit:, page:)`
// resolver.
func TestQueryHandler_SearchQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(
		`{"query":"{ search(q: \"jfk\", limit: 5) { icao } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data, got nil")
	}
}

// TestQueryHandler_WithinQuery covers the bounding-box resolver, including
// the required-bounds error when only one edge is supplied.
func TestQueryHandler_WithinQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "all bounds supplied",
			query:      `{"query":"{ within(latMin: 40, latMax: 41, lonMin: -74, lonMax: -73) { icao } }"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "missing bounds is an error",
			query:      `{"query":"{ within(latMin: 40) { icao } }"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			gotError := len(resp.Errors) > 0
			if gotError != tt.wantError {
				t.Errorf("gotError = %v, want %v (errors: %+v)", gotError, tt.wantError, resp.Errors)
			}
		})
	}
}

// TestQueryHandler_AutocompleteQuery covers the autocomplete resolver,
// including the minimum-length validation error.
func TestQueryHandler_AutocompleteQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid query",
			query:      `{"query":"{ autocomplete(q: \"jf\", limit: 5) { icao } }"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "too short is an error",
			query:      `{"query":"{ autocomplete(q: \"j\") { icao } }"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			gotError := len(resp.Errors) > 0
			if gotError != tt.wantError {
				t.Errorf("gotError = %v, want %v (errors: %+v)", gotError, tt.wantError, resp.Errors)
			}
		})
	}
}

// TestQueryHandler_CountriesQuery covers the no-arg `countries` resolver.
func TestQueryHandler_CountriesQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(
		`{"query":"{ countries { name count } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data, got nil")
	}
}

// TestQueryHandler_StatesQuery covers the `states(country:)` resolver.
func TestQueryHandler_StatesQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(
		`{"query":"{ states(country: \"US\") { name count } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data, got nil")
	}
}

// TestQueryHandler_StatsQuery covers the no-arg `stats` resolver.
func TestQueryHandler_StatsQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(
		`{"query":"{ stats { totalAirports countries cities withIata } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data, got nil")
	}
}

// TestQueryHandler_GeoIPQuery covers the `geoip(ip:)` resolver, including
// the explicit-IP found/not-found cases and the omitted-IP fallback to
// the request's remote address.
func TestQueryHandler_GeoIPQuery(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, newFakeGeoIPSource())

	tests := []struct {
		name       string
		query      string
		remoteAddr string
		wantError  bool
	}{
		{
			name:      "known ip",
			query:     `{"query":"{ geoip(ip: \"1.2.3.4\") { ip country } }"}`,
			wantError: false,
		},
		{
			name:      "unknown ip",
			query:     `{"query":"{ geoip(ip: \"9.9.9.9\") { ip } }"}`,
			wantError: true,
		},
		{
			name:       "omitted ip falls back to remote addr",
			query:      `{"query":"{ geoip { ip } }"}`,
			remoteAddr: "1.2.3.4:5555",
			wantError:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(tt.query))
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
			}
			var resp response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			gotError := len(resp.Errors) > 0
			if gotError != tt.wantError {
				t.Errorf("gotError = %v, want %v (errors: %+v)", gotError, tt.wantError, resp.Errors)
			}
		})
	}
}

// TestQueryHandler_GeoIPNilSource covers the geo == nil fallback path
// (e.g. GeoIP disabled at startup) — it must report an error, not panic.
func TestQueryHandler_GeoIPNilSource(t *testing.T) {
	src := newFakeSource()
	h := QueryHandler(src, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", bytes.NewBufferString(
		`{"query":"{ geoip(ip: \"1.2.3.4\") { ip } }"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Errors) == 0 {
		t.Error("expected an error when geo source is nil")
	}
}
