package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/geoip"
)

// Covers countryAllowed's decision table per AI.md PART 19 "Country
// Blocking Behavior": both empty allows everything, deny_countries blocks
// only listed codes, allow_countries permits only listed codes, and
// allow_countries wins when both are set.
func TestCountryAllowed(t *testing.T) {
	tests := []struct {
		name    string
		country string
		allow   []string
		deny    []string
		want    bool
	}{
		{"both empty allows all", "CN", nil, nil, true},
		{"deny list blocks match", "CN", nil, []string{"CN", "RU"}, false},
		{"deny list allows non-match", "US", nil, []string{"CN", "RU"}, true},
		{"deny list case-insensitive", "cn", nil, []string{"CN"}, false},
		{"allow list permits match", "US", []string{"US", "CA", "GB"}, nil, true},
		{"allow list blocks non-match", "CN", []string{"US", "CA", "GB"}, nil, false},
		{"allow list wins over deny when both set", "US", []string{"US"}, []string{"US"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countryAllowed(tt.country, tt.allow, tt.deny); got != tt.want {
				t.Errorf("countryAllowed(%q, %v, %v) = %v, want %v", tt.country, tt.allow, tt.deny, got, tt.want)
			}
		})
	}
}

// callGeoIPMiddleware wires GeoIPMiddleware in front of a handler that
// records whether it was reached, then executes a single request.
func callGeoIPMiddleware(svc *geoip.Service, enabled bool, allow, deny []string, remoteAddr string, allowlisted bool) (reached bool, status int) {
	mw := GeoIPMiddleware(svc, enabled, allow, deny)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if allowlisted {
		req = req.WithContext(context.WithValue(req.Context(), ctxKeyAllowlisted, true))
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return reached, rec.Code
}

// Covers every fail-open path per AI.md PART 19 "Failure mode" — GeoIP
// must never block a request when it cannot make a definitive decision.
func TestGeoIPMiddlewareFailsOpen(t *testing.T) {
	unavailableSvc := &geoip.Service{}

	tests := []struct {
		name        string
		svc         *geoip.Service
		enabled     bool
		allow       []string
		deny        []string
		remoteAddr  string
		allowlisted bool
	}{
		{"disabled", unavailableSvc, false, nil, []string{"CN"}, "8.8.8.8:1234", false},
		{"no lists configured", unavailableSvc, true, nil, nil, "8.8.8.8:1234", false},
		{"already allowlisted", unavailableSvc, true, nil, []string{"CN"}, "8.8.8.8:1234", true},
		{"private IP source", unavailableSvc, true, nil, []string{"CN"}, "192.168.1.5:1234", false},
		{"loopback IP source", unavailableSvc, true, nil, []string{"CN"}, "127.0.0.1:1234", false},
		{"service unavailable (nil DBs)", unavailableSvc, true, nil, []string{"CN"}, "8.8.8.8:1234", false},
		{"nil service", nil, true, nil, []string{"CN"}, "8.8.8.8:1234", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached, status := callGeoIPMiddleware(tt.svc, tt.enabled, tt.allow, tt.deny, tt.remoteAddr, tt.allowlisted)
			if !reached {
				t.Errorf("expected request to reach downstream handler, but it was blocked (status %d)", status)
			}
			if status != http.StatusOK {
				t.Errorf("status = %d, want 200", status)
			}
		})
	}
}
