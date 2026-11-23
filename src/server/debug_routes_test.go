package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/mode"
)

func TestDebugAPIAliasesRequireOperatorToken(t *testing.T) {
	wasEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(true)
	t.Cleanup(func() { mode.SetDebugEnabled(wasEnabled) })

	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	server.config.Server.Debug.Pprof = false
	server.config.Server.Debug.RuntimeEndpoints = false
	server.setupRouter()

	for _, path := range []string{"/api/v1/debug/config", "/api/debug/config", "/debug/config"} {
		path := path
		t.Run(path, func(t *testing.T) {
			unauthorized := httptest.NewRecorder()
			server.router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, path, nil))
			if unauthorized.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
			}

			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer adm_test-token")
			response := httptest.NewRecorder()
			server.router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("authenticated status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
			if path == "/api/v1/debug/config" && response.Header().Get("X-API-Version") != server.config.Server.APIVersion {
				t.Errorf("X-API-Version = %q, want %q", response.Header().Get("X-API-Version"), server.config.Server.APIVersion)
			}
		})
	}

	paths := []string{"/api/v1/debug/config", "/api/debug/config"}
	responses := make([]*httptest.ResponseRecorder, 0, len(paths))
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer adm_test-token")
		responses = append(responses, httptest.NewRecorder())
		server.router.ServeHTTP(responses[len(responses)-1], request)
	}
	for i, response := range responses[1:] {
		if response.Code != responses[0].Code || response.Body.String() != responses[0].Body.String() {
			t.Errorf("%s response differs from the canonical debug API route", paths[i+1])
		}
		for _, header := range []string{"Content-Type", "Cache-Control", "X-API-Version"} {
			if response.Header().Get(header) != responses[0].Header().Get(header) {
				t.Errorf("%s %s = %q, canonical = %q", paths[i+1], header, response.Header().Get(header), responses[0].Header().Get(header))
			}
		}
	}
}
