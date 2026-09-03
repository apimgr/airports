package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleHealth covers the unversioned /api/v1/health handler.
// respondItem wraps the item in the canonical {"ok":true,"data":...}
// envelope per AI.md PART 14, so fields are asserted under body["data"].
func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("body[data] missing or wrong type: %v", body["data"])
	}
	if status, _ := data["status"].(string); status != "healthy" {
		t.Errorf("data[status] = %v, want %q", data["status"], "healthy")
	}
}

// TestHandleGetAirports covers pagination defaults and clamping.
func TestHandleGetAirports(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "default pagination", query: ""},
		{name: "explicit limit and page", query: "?limit=1&page=1"},
		{name: "limit clamped above max", query: "?limit=99999"},
		{name: "limit clamped below min", query: "?limit=0"},
		{name: "invalid page falls back", query: "?page=-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/airports"+tt.query, nil)
			w := httptest.NewRecorder()

			s.handleGetAirports(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}

// TestHandleSearchAirports covers the query-driven search handler.
func TestHandleSearchAirports(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "matching query", query: "?q=London"},
		{name: "empty query", query: ""},
		{name: "no match", query: "?q=zzzznonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/airports/search"+tt.query, nil)
			w := httptest.NewRecorder()

			s.handleSearchAirports(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}

// TestHandleNearbyAirports covers the required lat/lon validation, which
// responds 400 BAD_REQUEST on parse failure, unlike its text sibling.
func TestHandleNearbyAirports(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{name: "valid coordinates", query: "?lat=40.6&lon=-73.7", wantStatus: http.StatusOK},
		{name: "missing lat", query: "?lon=-73.7", wantStatus: http.StatusBadRequest},
		{name: "invalid lat", query: "?lat=abc&lon=-73.7", wantStatus: http.StatusBadRequest},
		{name: "invalid lon", query: "?lat=40.6&lon=xyz", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/airports/nearby"+tt.query, nil)
			w := httptest.NewRecorder()

			s.handleNearbyAirports(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus == http.StatusBadRequest {
				var body ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode error body: %v", err)
				}
				if body.Error != "BAD_REQUEST" {
					t.Errorf("body.Error = %q, want %q", body.Error, "BAD_REQUEST")
				}
			}
		})
	}
}

// TestHandleNearbyAirportsText covers input validation on the text handler:
// invalid lat/lon must return 400 with a descriptive plain-text body,
// matching the JSON handler's validation (AI.md PART 9 "validate/sanitize
// all input").
func TestHandleNearbyAirportsText(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/airports/nearby.txt?lat=notanumber&lon=notanumber", nil)
	w := httptest.NewRecorder()

	s.handleNearbyAirportsText(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (text handler validates lat/lon), body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestHandleBBoxAirports covers the bounding-box handler, which has no
// parameter validation at all (silently defaults to 0 on parse failure).
func TestHandleBBoxAirports(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "valid bbox", query: "?minLat=40&maxLat=41&minLon=-74&maxLon=-73"},
		{name: "missing params default to zero", query: ""},
		{name: "garbage params default to zero", query: "?minLat=x&maxLat=y&minLon=z&maxLon=w"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/airports/within"+tt.query, nil)
			w := httptest.NewRecorder()

			s.handleBBoxAirports(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}

// TestHandleAutocomplete covers the minimum-query-length validation.
func TestHandleAutocomplete(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  string
	}{
		{name: "valid query", query: "?q=Lon", wantStatus: http.StatusOK},
		{name: "too short", query: "?q=L", wantStatus: http.StatusBadRequest, wantError: "BAD_REQUEST"},
		{name: "empty query", query: "", wantStatus: http.StatusBadRequest, wantError: "BAD_REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/airports/autocomplete"+tt.query, nil)
			w := httptest.NewRecorder()

			s.handleAutocomplete(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantError != "" {
				var body ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode error body: %v", err)
				}
				if body.Error != tt.wantError {
					t.Errorf("body.Error = %q, want %q", body.Error, tt.wantError)
				}
			}
		})
	}
}

// TestHandleGetCountries and TestHandleAirportStats cover simple
// no-parameter data handlers.
func TestHandleGetCountries(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/countries", nil)
	w := httptest.NewRecorder()

	s.handleGetCountries(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleAirportStats(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	s.handleAirportStats(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestHandleGeoIPLookup covers the nil-GeoIP error path, which responds 500
// SERVER_ERROR (differing from the 400 BAD_REQUEST used by its sibling handlers).
func TestHandleGeoIPLookup(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantStatus int
		wantError  string
	}{
		{name: "valid ip, nil geoip service fails lookup", remoteAddr: "203.0.113.5:1234", wantStatus: http.StatusInternalServerError, wantError: "SERVER_ERROR"},
		{name: "unparseable remote addr", remoteAddr: "not-an-ip", wantStatus: http.StatusBadRequest, wantError: "BAD_REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/geoip", nil)
			r.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			s.handleGeoIPLookup(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			var body ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if body.Error != tt.wantError {
				t.Errorf("body.Error = %q, want %q", body.Error, tt.wantError)
			}
		})
	}
}

// TestHandleGeoIPLookupText covers the text sibling of handleGeoIPLookup,
// which responds 400 (not 500) on a failed lookup.
func TestHandleGeoIPLookupText(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/geoip.txt", nil)
	r.RemoteAddr = "203.0.113.5:1234"
	w := httptest.NewRecorder()

	s.handleGeoIPLookupText(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestHandleGeoIPNearbyAirports covers the optional ip query param and the
// getClientIP fallback, both of which fail against the nil GeoIP service.
func TestHandleGeoIPNearbyAirports(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "explicit ip param", query: "?ip=203.0.113.5"},
		{name: "fallback to remote addr", query: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/geoip/airports/nearby"+tt.query, nil)
			r.RemoteAddr = "203.0.113.5:1234"
			w := httptest.NewRecorder()

			s.handleGeoIPNearbyAirports(w, r)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (nil geoip service fails lookup), body=%s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}

// TestHandleGetAirportsExports covers the three full-database export
// handlers and their Content-Disposition headers.
func TestHandleGetAirportsExports(t *testing.T) {
	s := newTestServer(t)

	t.Run("json export", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/airports.json", nil)
		w := httptest.NewRecorder()
		s.handleGetAirportsJSON(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), "airports.json") {
			t.Errorf("Content-Disposition = %q, want it to mention airports.json", w.Header().Get("Content-Disposition"))
		}
	})

	t.Run("csv export", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/airports.csv", nil)
		w := httptest.NewRecorder()
		s.handleGetAirportsCSV(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), "airports.csv") {
			t.Errorf("Content-Disposition = %q, want it to mention airports.csv", w.Header().Get("Content-Disposition"))
		}
	})

	t.Run("geojson export", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/airports.geojson", nil)
		w := httptest.NewRecorder()
		s.handleGetAirportsGeoJSON(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), "airports.geojson") {
			t.Errorf("Content-Disposition = %q, want it to mention airports.geojson", w.Header().Get("Content-Disposition"))
		}
	})
}

// TestRouterChiParamHandlers routes requests through the real chi router so
// that chi.URLParam resolves correctly, covering the ident/country/ip
// path-param handlers.
func TestRouterChiParamHandlers(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "get airport by ident, found", method: http.MethodGet, path: "/api/v1/airports/KJFK", wantStatus: http.StatusOK},
		{name: "get airport by ident, not found", method: http.MethodGet, path: "/api/v1/airports/ZZZZ", wantStatus: http.StatusNotFound},
		{name: "get airport by ident text, found", method: http.MethodGet, path: "/api/v1/airports/KJFK.txt", wantStatus: http.StatusOK},
		{name: "get airport by ident text, not found", method: http.MethodGet, path: "/api/v1/airports/ZZZZ.txt", wantStatus: http.StatusNotFound},
		{name: "get states by country", method: http.MethodGet, path: "/api/v1/states/US", wantStatus: http.StatusOK},
		{name: "geoip lookup by ip path param", method: http.MethodGet, path: "/api/v1/geoip/203.0.113.5", wantStatus: http.StatusBadRequest},
		{name: "geoip lookup by ip path param text", method: http.MethodGet, path: "/api/v1/geoip/203.0.113.5.txt", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			s.Router().ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestHandleAirportDetailParamBug is a regression test documenting a known
// bug: handleAirportDetail reads chi.URLParam(r, "code") but the route is
// registered as /airports/{ident}, so code always resolves to "" and the
// requested airport's data is never actually loaded into the template,
// regardless of which ident is in the URL. See web_handlers.go:83.
func TestHandleAirportDetailParamBug(t *testing.T) {
	s := newTestServer(t)

	// Confirm the underlying lookup that the handler performs with the
	// wrong (always-empty) param actually fails, proving the airport data
	// is never populated no matter what /airports/{ident} is requested.
	if _, err := s.airports.GetByCode(""); err == nil {
		t.Fatal("GetByCode(\"\") unexpectedly succeeded; the param-name bug regression assumption no longer holds")
	}

	r := httptest.NewRequest(http.MethodGet, "/airports/KJFK", nil)
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, r)

	// The page still renders 200 because the handler treats the lookup
	// failure as "no data to display" rather than a 404 - but the airport
	// requested by the URL is silently never shown.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestHandleAPIInfo, TestHandleGetSettings, and TestHandleWellKnownFiles
// cover the remaining no-param JSON/text handlers in server.go.
func TestHandleAPIInfo(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1", nil)
	w := httptest.NewRecorder()

	s.handleAPIInfo(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// respondItem wraps the payload in the canonical {"ok":true,"data":...}
	// envelope per AI.md PART 14.
	if body["ok"] != true {
		t.Errorf("body[ok] = %v, want true", body["ok"])
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("body[data] is not an object: %#v", body["data"])
	}
	if data["version"] != s.config.Server.APIVersion {
		t.Errorf("data[version] = %v, want %q", data["version"], s.config.Server.APIVersion)
	}
}

func TestHandleGetSettings(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	w := httptest.NewRecorder()

	s.handleGetSettings(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleHealthText(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health.txt", nil)
	w := httptest.NewRecorder()

	s.handleHealthText(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "status: healthy") {
		t.Errorf("body = %q, want it to contain %q", w.Body.String(), "status: healthy")
	}
}

func TestHandleRobotsTxt(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()

	s.handleRobotsTxt(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "User-agent:") {
		t.Errorf("body = %q, want it to contain %q", w.Body.String(), "User-agent:")
	}
	for _, path := range s.config.Web.Robots.Deny {
		if !strings.Contains(w.Body.String(), "Disallow: "+path+"\n") {
			t.Errorf("body = %q, want it to contain %q", w.Body.String(), "Disallow: "+path)
		}
	}
}

// AI.md PART 11 "AI Crawler Rules": a bot resolving to deny gets its own
// stanza; a bot resolving to allow is covered by the wildcard block and gets
// none.
func TestHandleRobotsTxtAIBots(t *testing.T) {
	s := newTestServer(t)
	s.config.Web.Robots.AIBots.Default = "deny"
	s.config.Web.Robots.AIBots.Bots = map[string]string{
		"GPTBot":    "deny",
		"CCBot":     "allow",
		"ClaudeBot": "",
	}

	r := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()
	s.handleRobotsTxt(w, r)

	body := w.Body.String()
	for _, bot := range []string{"GPTBot", "ClaudeBot"} {
		if !strings.Contains(body, "User-agent: "+bot+"\nDisallow: /\n") {
			t.Errorf("body = %q, want a Disallow stanza for %q", body, bot)
		}
	}
	if strings.Contains(body, "User-agent: CCBot") {
		t.Errorf("body = %q, want no stanza for an explicitly allowed bot", body)
	}
}

func TestHandleLocaleJSON(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/locales/es.json", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(w.Body.String(), `"language": "es"`) {
		t.Errorf("body = %q, want it to contain the es meta block", w.Body.String())
	}
}

func TestHandleLocaleJSONUnsupportedFallsBackToEnglish(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/locales/xx.json", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `"language": "en"`) {
		t.Errorf("body = %q, want fallback to English meta block", w.Body.String())
	}
}

func TestHandleSecurityTxt(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/security.txt", nil)
	w := httptest.NewRecorder()

	s.handleSecurityTxt(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "Contact:") {
		t.Errorf("body = %q, want it to contain %q", w.Body.String(), "Contact:")
	}
}

func TestHandleManifest(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	w := httptest.NewRecorder()

	s.handleManifest(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/manifest+json")
	}
}

func TestHandleServiceWorker(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	w := httptest.NewRecorder()

	s.handleServiceWorker(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/javascript" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/javascript")
	}
}

// TestHandleServerHealthzContentNegotiation covers the 3-way Accept-header
// negotiation (html/text/json) in server_pages.go.
func TestHandleServerHealthzContentNegotiation(t *testing.T) {
	tests := []struct {
		name   string
		accept string
	}{
		{name: "html", accept: "text/html"},
		{name: "text", accept: "text/plain"},
		{name: "json default", accept: ""},
		{name: "json for unknown accept", accept: "application/xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			r := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}
			w := httptest.NewRecorder()

			s.handleServerHealthz(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}

// TestHandleAutodiscover covers the unversioned /api/autodiscover endpoint.
func TestHandleAutodiscover(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)
	r.Host = "airports.example.com"
	w := httptest.NewRecorder()

	s.handleAutodiscover(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// respondItem wraps the payload in the canonical {"ok":true,"data":...}
	// envelope per AI.md PART 14.
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("body[data] missing or wrong type: %v", body["data"])
	}
	api, ok := data["api"].(map[string]interface{})
	if !ok {
		t.Fatalf("data[api] missing or wrong type: %v", data["api"])
	}
	if api["current_version"] != s.config.Server.APIVersion {
		t.Errorf("api[current_version] = %v, want %q", api["current_version"], s.config.Server.APIVersion)
	}
}

// TestServerPagesStaticHandlers covers the static-content server_pages.go
// handlers (both HTML template and JSON variants).
func TestServerPagesStaticHandlers(t *testing.T) {
	s := newTestServer(t)

	htmlHandlers := map[string]func(http.ResponseWriter, *http.Request){
		"handleServerAbout":   s.handleServerAbout,
		"handleServerHelp":    s.handleServerHelp,
		"handleServerPrivacy": s.handleServerPrivacy,
		"handleServerTerms":   s.handleServerTerms,
	}
	for name, h := range htmlHandlers {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/server/x", nil)
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}

	jsonHandlers := map[string]func(http.ResponseWriter, *http.Request){
		"handleServerHelpAPI":    s.handleServerHelpAPI,
		"handleServerPrivacyAPI": s.handleServerPrivacyAPI,
		"handleServerTermsAPI":   s.handleServerTermsAPI,
		"handleServerAboutAPI":   s.handleServerAboutAPI,
	}
	for name, h := range jsonHandlers {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/server/x", nil)
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
			}
			var body map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
		})
	}
}

// TestWebHandlers covers the remaining web_handlers.go handlers not already
// covered by the param-bug regression test.
func TestWebHandlers(t *testing.T) {
	s := newTestServer(t)

	t.Run("home", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		s.handleHome(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("search with query", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/airports/search?q=London", nil)
		w := httptest.NewRecorder()
		s.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("search without query", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/airports/search", nil)
		w := httptest.NewRecorder()
		s.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("nearby with coordinates", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/airports/nearby?lat=40.6&lon=-73.7", nil)
		w := httptest.NewRecorder()
		s.handleNearby(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("nearby without coordinates", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/airports/nearby", nil)
		w := httptest.NewRecorder()
		s.handleNearby(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("stats", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/stats", nil)
		w := httptest.NewRecorder()
		s.handleStats(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
	})
}
