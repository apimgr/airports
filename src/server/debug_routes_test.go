package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/mode"
)

func TestDebugRoutesRequireOperatorTokenAndHaveNoAPIAliases(t *testing.T) {
	wasEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(true)
	t.Cleanup(func() { mode.SetDebugEnabled(wasEnabled) })

	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	server.config.Server.Debug.Pprof = false
	server.config.Server.Debug.RuntimeEndpoints = false
	server.setupRouter()

	for _, path := range []string{"/debug/config"} {
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
		})
	}
	for _, path := range []string{"/api/v1/debug/config", "/api/debug/config"} {
		response := httptest.NewRecorder()
		server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("debug alias %s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}
}

func TestDebugPprofIndexUsesSlashFreeCanonicalRoute(t *testing.T) {
	wasEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(true)
	t.Cleanup(func() { mode.SetDebugEnabled(wasEnabled) })

	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	server.config.Server.Debug.Pprof = true
	server.setupRouter()

	for _, path := range []string{"/debug/pprof", "/debug/pprof/"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer adm_test-token")
		response := httptest.NewRecorder()
		server.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "goroutine") {
			t.Errorf("%s returned (%d, %q), want pprof index", path, response.Code, response.Body.String())
		}
	}
}

func TestDebugSensitiveEndpointsSanitizeAndRender(t *testing.T) {
	wasEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(true)
	t.Cleanup(func() { mode.SetDebugEnabled(wasEnabled) })

	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	server.config.Server.Debug.Pprof = true
	server.config.Server.Debug.RuntimeEndpoints = true
	server.setupRouter()

	for _, path := range []string{"/debug/pprof/cmdline", "/debug/vars", "/debug/error500"} {
		unauthorized := httptest.NewRecorder()
		server.router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, path, nil))
		if unauthorized.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s status = %d, want %d", path, unauthorized.Code, http.StatusUnauthorized)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	request.Header.Set("Authorization", "Bearer adm_test-token")
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != debugBinaryName()+"\x00" {
		t.Errorf("sanitized pprof cmdline = (%d, %q), want (%d, %q)", response.Code, response.Body.String(), http.StatusOK, debugBinaryName()+"\x00")
	}

	request = httptest.NewRequest(http.MethodGet, "/debug/vars", nil)
	request.Header.Set("Authorization", "Bearer adm_test-token")
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	var variables map[string]interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &variables); err != nil {
		t.Fatalf("decode expvar response: %v; body=%s", err, response.Body.String())
	}
	if got, ok := variables["cmdline"].(string); !ok || got != debugBinaryName() {
		t.Errorf("sanitized expvar cmdline = %#v, want %q", variables["cmdline"], debugBinaryName())
	}

	request = httptest.NewRequest(http.MethodGet, "/debug/error500", nil)
	request.Header.Set("Authorization", "Bearer adm_test-token")
	request.Header.Set("Accept", "text/html")
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Errorf("error500 status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Errorf("error500 content type = %q, want themed HTML", contentType)
	}
}
