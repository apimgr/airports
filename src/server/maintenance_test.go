package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestMaintenanceWriteGateRejectsMutations(t *testing.T) {
	SetHealthRuntimeState(HealthRuntimeState{
		Maintenance: &MaintenanceHealthInfo{
			Reason:      "database_connection",
			Message:     "sensitive caller text must be replaced",
			SelfHealing: SelfHealingHealthStatus{Enabled: true, Attempts: 2},
		},
	})
	t.Cleanup(func() { SetHealthRuntimeState(HealthRuntimeState{}) })

	cfg := config.DefaultConfig()
	cfg.Server.Maintenance.SelfHealing.RetryInterval = "45s"
	s := &Server{config: cfg}
	nextCalled := false
	handler := s.maintenanceWriteGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/airports", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if nextCalled {
		t.Fatal("maintenance write gate passed a POST request to the next handler")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	for header, want := range map[string]string{
		"Retry-After":          "45",
		"X-Maintenance-Mode":   "true",
		"X-Maintenance-Reason": "database_connection",
		"Content-Type":         "application/json",
	} {
		if got := response.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	var body ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.OK || body.Error != "MAINTENANCE" || body.Message != "Server is in maintenance mode due to: Database connection failed" {
		t.Errorf("body = %+v, want canonical maintenance error", body)
	}
	details, ok := body.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details type = %T, want map", body.Details)
	}
	if details["reason"] != "database_connection" || details["self_healing"] != true {
		t.Errorf("details = %#v, want reason and self-healing", details)
	}
}

func TestMaintenanceWriteGateAllowsReadOnlyMethods(t *testing.T) {
	SetHealthRuntimeState(HealthRuntimeState{
		Maintenance: &MaintenanceHealthInfo{Reason: "database_connection"},
	})
	t.Cleanup(func() { SetHealthRuntimeState(HealthRuntimeState{}) })

	s := &Server{config: config.DefaultConfig()}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			nextCalled := false
			handler := s.maintenanceWriteGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, "/server/healthz", nil))
			if !nextCalled || response.Code != http.StatusNoContent {
				t.Errorf("method %s was blocked: called=%t status=%d", method, nextCalled, response.Code)
			}
		})
	}
}

func TestMaintenanceWriteGatePassesWhenInactive(t *testing.T) {
	SetHealthRuntimeState(HealthRuntimeState{})
	t.Cleanup(func() { SetHealthRuntimeState(HealthRuntimeState{}) })
	called := false
	handler := (&Server{}).maintenanceWriteGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/v1/airports", nil))
	if !called {
		t.Fatal("inactive maintenance gate blocked a PATCH request")
	}
}

func TestMaintenanceRetryAfter(t *testing.T) {
	for input, want := range map[string]string{
		"30s":    "30",
		"1500ms": "2",
		"1h":     "3600",
		"0s":     "30",
		"bad":    "30",
	} {
		if got := maintenanceRetryAfter(input); got != want {
			t.Errorf("maintenanceRetryAfter(%q) = %q, want %q", input, got, want)
		}
	}
}
