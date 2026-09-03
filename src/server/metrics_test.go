package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMetricsAuthMiddleware covers the optional bearer-token gate on
// /metrics per AI.md PART 20 "Access Control > Authentication options":
// empty token configured means the endpoint stays open (firewall-only
// deployments); a configured token requires an exact "Authorization: Bearer
// <token>" match and rejects everything else with 401.
func TestMetricsAuthMiddleware(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		token      string
		authHeader string
		wantStatus int
	}{
		{name: "no token configured allows any request", token: "", authHeader: "", wantStatus: http.StatusOK},
		{name: "no token configured allows request even with garbage header", token: "", authHeader: "Bearer wrong", wantStatus: http.StatusOK},
		{name: "token configured, missing header rejected", token: "secret123", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "token configured, wrong token rejected", token: "secret123", authHeader: "Bearer wrongtoken", wantStatus: http.StatusUnauthorized},
		{name: "token configured, non-bearer scheme rejected", token: "secret123", authHeader: "Basic secret123", wantStatus: http.StatusUnauthorized},
		{name: "token configured, correct token allowed", token: "secret123", authHeader: "Bearer secret123", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := metricsAuthMiddleware(tt.token, okHandler)

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusUnauthorized {
				if got := rr.Header().Get("WWW-Authenticate"); got == "" {
					t.Fatalf("expected WWW-Authenticate header on 401 response")
				}
			}
		})
	}
}
