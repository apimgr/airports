package server

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestMCPToolCallDispatchesAirportRouteWithFreshChiContext(t *testing.T) {
	s := newTestServer(t)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "mcp-route.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	s.db = database
	s.config.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x62}, 32))
	_, token, err := s.issueOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	s.setupRouter()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"getAirport","arguments":{"ident":"KJFK"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("MCP-Protocol-Version", "2026-07-28")
	r.Header.Set("Mcp-Method", "tools/call")
	r.Header.Set("Mcp-Name", "getAirport")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "KJFK") {
		t.Fatalf("MCP airport tool status=%d body=%q", w.Code, w.Body.String())
	}
}
