package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
	"github.com/go-chi/chi/v5"
)

func TestRouteRegistryMountsAndSeparatesPublicAndMCPSurfaces(t *testing.T) {
	router := chi.NewRouter()
	registry := &routeRegistry{router: router}
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/v1/airports", Aliases: []string{"/api/airports"},
		OperationID: "listAirports", Summary: "List airports", Auth: "none", Listed: true,
		Request: map[string]interface{}{"query": "query schema"}, Response: map[string]interface{}{"200": "response schema"}, Handler: handler,
	})
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/v1/private", OperationID: "privateRead",
		Summary: "Private read", Auth: "operator", Listed: false, Handler: handler,
	})
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/v1/service", OperationID: "serviceMetrics",
		Summary: "Service metrics", Auth: "service", Listed: false, Handler: handler,
	})
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/v1/internal", OperationID: "internalDebug",
		Summary: "Internal debug", Auth: "operator", Listed: true, Internal: true, Handler: handler,
	})
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/v1/unlisted", OperationID: "unlistedRead",
		Summary: "Unlisted read", Auth: "none", Listed: false, Handler: handler,
	})

	for _, path := range []string{"/api/v1/airports", "/api/airports"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNoContent {
			t.Errorf("registered route %s status = %d, want %d", path, response.Code, http.StatusNoContent)
		}
	}

	publicPaths := make(map[string]bool)
	for _, route := range registry.OpenAPIRoutes("/api/v1") {
		publicPaths[route.Path] = true
		if route.Path == "/airports" && (route.Request == nil || route.Response == nil) {
			t.Error("route schema models were not projected into the OpenAPI surface")
		}
	}
	for _, path := range []string{"/airports", "/api/airports"} {
		if !publicPaths[path] {
			t.Errorf("public docs omitted %s", path)
		}
	}
	for _, path := range []string{"/private", "/service", "/internal", "/unlisted"} {
		if publicPaths[path] {
			t.Errorf("public docs included %s", path)
		}
	}

	mcpPaths := make(map[string]bool)
	for _, route := range registry.MCPRoutes("/api/v1") {
		mcpPaths[route.Path] = true
	}
	for _, path := range []string{"/airports", "/private", "/unlisted"} {
		if !mcpPaths[path] {
			t.Errorf("MCP candidates omitted %s", path)
		}
	}
	for _, path := range []string{"/service", "/internal"} {
		if mcpPaths[path] {
			t.Errorf("MCP candidates included %s", path)
		}
	}
}

func TestRouteRegistryMountsDebugOnlyRoutesConditionally(t *testing.T) {
	wasDebug := mode.IsDebugEnabled()
	defer mode.SetDebugEnabled(wasDebug)
	mode.SetDebugEnabled(false)
	router := chi.NewRouter()
	registry := &routeRegistry{router: router}
	registry.Add(Route{Method: http.MethodGet, Path: "/debug/only", Internal: true, DebugOnly: true, Handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }})
	if snapshot := registry.Snapshot(); len(snapshot) != 1 || snapshot[0].Mounted {
		t.Fatalf("debug route snapshot = %#v, want unmounted route", snapshot)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/only", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("debug route status = %d, want %d", response.Code, http.StatusNotFound)
	}
	mode.SetDebugEnabled(true)
	debugRouter := chi.NewRouter()
	debugRegistry := &routeRegistry{router: debugRouter}
	debugRegistry.Add(Route{Method: http.MethodGet, Path: "/debug/only", Internal: true, DebugOnly: true, Handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }})
	response = httptest.NewRecorder()
	debugRouter.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/only", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("debug route status with debug enabled = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestMCPRouteAvailabilityUsesTokenClass(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Token = "operator-token"
	server := &Server{config: cfg}
	operatorRequest := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	operatorRequest.Header.Set("Authorization", "Bearer operator-token")

	if !server.mcpRouteAvailable(Route{Auth: "none"}, nil) {
		t.Fatal("public route was hidden from an unauthenticated help listing")
	}
	if server.mcpRouteAvailable(Route{Auth: "owner"}, nil) {
		t.Fatal("owner route was exposed without a token")
	}
	if !server.mcpRouteAvailable(Route{Auth: "operator"}, operatorRequest) {
		t.Fatal("operator route was hidden from the operator token")
	}
	if server.mcpRouteAvailable(Route{Auth: "owner"}, operatorRequest) {
		t.Fatal("owner route was exposed to the operator token")
	}
	if server.mcpRouteAvailable(Route{Auth: "service"}, operatorRequest) {
		t.Fatal("service route was exposed to an MCP bearer token")
	}
	if server.mcpRouteAvailable(Route{Auth: "token"}, operatorRequest) != true {
		t.Fatal("token route was hidden from a valid operator bearer token")
	}
}
