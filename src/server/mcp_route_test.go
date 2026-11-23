package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPRouteAliasesMatchVersionedHeadersAndBody(t *testing.T) {
	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	server.setupRouter()
	paths := []string{"/api/v1/server/mcp", "/api/mcp", "/mcp"}
	responses := make([]*httptest.ResponseRecorder, 0, len(paths))
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.Host = "example.com"
		request.Header.Set("Authorization", "Bearer invalid-token")
		response := httptest.NewRecorder()
		server.router.ServeHTTP(response, request)
		responses = append(responses, response)
	}
	base := responses[0]
	if base.Code != http.StatusUnauthorized {
		t.Fatalf("canonical status = %d, want %d", base.Code, http.StatusUnauthorized)
	}
	for index, response := range responses[1:] {
		path := paths[index+1]
		if response.Code != base.Code || response.Body.String() != base.Body.String() {
			t.Errorf("%s response = (%d, %q), canonical = (%d, %q)", path, response.Code, response.Body.String(), base.Code, base.Body.String())
		}
		for _, header := range []string{"Content-Type", "Cache-Control", "X-API-Version", "WWW-Authenticate"} {
			if response.Header().Get(header) != base.Header().Get(header) {
				t.Errorf("%s %s = %q, canonical = %q", path, header, response.Header().Get(header), base.Header().Get(header))
			}
		}
	}
}
