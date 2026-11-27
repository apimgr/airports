package server

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/mode"
	swaggerpkg "github.com/apimgr/airports/src/swagger"
	"github.com/go-chi/chi/v5"
)

func TestAPIRouteDrift(t *testing.T) {
	wasDebugEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(false)
	t.Cleanup(func() { mode.SetDebugEnabled(wasDebugEnabled) })
	server := contactTestServer(t)
	server.setupRouter()

	projectRoutes := readIDEAAPIRoutes(t)
	templateRoutes := readAITemplateAPIRoutes(t)
	expectedSuperset := make(map[string]struct{}, len(templateRoutes)+len(projectRoutes))
	for _, route := range templateRoutes {
		expectedSuperset[route] = struct{}{}
	}
	for _, route := range projectRoutes {
		expectedSuperset[route] = struct{}{}
	}
	debugRoutes := readAIDebugRoutes(t)
	for _, route := range debugRoutes {
		expectedSuperset[route] = struct{}{}
	}
	expectedRouter := make(map[string]struct{}, len(expectedSuperset))
	for route := range expectedSuperset {
		expectedRouter[route] = struct{}{}
	}
	for _, route := range []string{
		"GET /healthz", "GET /metrics", "GET /metrics/prometheus", "GET /metrics/grafana", "GET /metrics/loki",
		"GET /llms.txt", "GET /.well-known/security.txt", "GET /.well-known/llms.txt", "GET /.well-known/pgp-key.asc",
		"GET /api/swagger", "GET /api/healthz", "POST /api/graphql", "GET /api/graphql", "POST /api/mcp", "POST /mcp", "GET /api/autodiscover",
		"GET /api/metrics", "GET /api/metrics/prometheus", "GET /api/metrics/grafana", "GET /api/metrics/loki",
	} {
		expectedSuperset[route] = struct{}{}
	}
	expected := make(map[string]struct{}, len(expectedSuperset))
	for route := range expectedSuperset {
		if strings.HasPrefix(route, "GET /debug/") && (!mode.ShouldShowDebugEndpoints() || (route == "GET /debug/pprof" || strings.Contains(route, "/debug/pprof/")) && !server.config.Server.Debug.Pprof || strings.HasSuffix(route, "/debug/vars") && !server.config.Server.Debug.RuntimeEndpoints) {
			continue
		}
		if (strings.HasPrefix(route, "GET /server/metrics") || strings.HasPrefix(route, "GET /api/metrics")) && !server.config.Server.Metrics.Enabled {
			continue
		}
		if route == "GET /healthz" && !server.config.Server.Healthz.Root.Enabled ||
			strings.HasPrefix(route, "GET /metrics") && strings.HasPrefix(route, "GET /metrics/") && (!server.config.Server.Metrics.Enabled || !server.config.Server.Metrics.Root.Enabled) ||
			route == "GET /metrics" && (!server.config.Server.Metrics.Enabled || !server.config.Server.Metrics.Root.Enabled) ||
			route == "GET /.well-known/pgp-key.asc" && !server.hasPGPKeypair() {
			continue
		}
		expected[route] = struct{}{}
	}
	expectedDocsSuperset := publicDocumentRoutes(expectedSuperset)
	expectedDocs := publicDocumentRoutes(expected)
	expectedRouterServed := make(map[string]struct{}, len(expectedRouter))
	for route := range expectedRouter {
		if _, ok := expected[route]; ok {
			expectedRouterServed[route] = struct{}{}
		}
	}

	registeredDocs := make(map[string]struct{})
	for _, route := range server.openAPIRoutes() {
		registeredDocs[route.Method+" "+route.Path] = struct{}{}
	}
	registeredRouter := make(map[string]struct{})
	apiBase := server.APIBasePath()
	_ = chi.Walk(server.router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(path, apiBase+"/") {
			path = strings.TrimPrefix(path, apiBase)
			registeredRouter[method+" "+path] = struct{}{}
		} else if strings.HasPrefix(path, "/debug/") {
			registeredRouter[method+" "+path] = struct{}{}
		}
		return nil
	})
	supersetRoutes := server.openAPIRouteSuperset()
	spec := swaggerpkg.BuildSpecWithRoutes(Version, server.config.Server.APIVersion, "en", supersetRoutes)
	paths := spec["paths"].(map[string]interface{})
	for _, route := range supersetRoutes {
		if route.MountedWhen == "" {
			continue
		}
		operation := paths[route.Path].(map[string]interface{})[strings.ToLower(route.Method)].(map[string]interface{})
		if operation["x-mounted-when"] != route.MountedWhen {
			t.Errorf("%s %s missing x-mounted-when %q", route.Method, route.Path, route.MountedWhen)
		}
	}
	documentedSuperset := routeSetFromSpec(paths, t)
	registeredSuperset := make(map[string]struct{}, len(supersetRoutes))
	for _, route := range supersetRoutes {
		registeredSuperset[route.Method+" "+route.Path] = struct{}{}
	}
	assertRouteSet(t, "AI.md+IDEA.md public route superset", expectedDocsSuperset, registeredSuperset)
	assertRouteSet(t, "OpenAPI public superset", expectedDocsSuperset, documentedSuperset)
	embedded := swaggerpkg.BuildEmbeddedSpecWithRoutes(Version, server.config.Server.APIVersion, "en", supersetRoutes)
	embeddedPaths := embedded["paths"].(map[string]interface{})
	assertRouteSet(t, "embedded OpenAPI public superset", expectedDocsSuperset, routeSetFromSpec(embeddedPaths, t))
	swaggerpkg.FilterMountedRoutes(embedded, supersetRoutes)
	assertRouteSet(t, "embedded served OpenAPI", expectedDocs, routeSetFromSpec(embeddedPaths, t))
	swaggerpkg.FilterMountedRoutes(spec, supersetRoutes)
	documented := routeSetFromSpec(paths, t)

	assertRouteSet(t, "router API/internal routes", expectedRouterServed, registeredRouter)
	assertRouteSet(t, "registered public docs routes", expectedDocsSuperset, registeredDocs)
	assertRouteSet(t, "OpenAPI mounted public routes", expectedDocs, documented)
}

func publicDocumentRoutes(routes map[string]struct{}) map[string]struct{} {
	public := make(map[string]struct{}, len(routes))
	for route := range routes {
		if strings.HasPrefix(route, "GET /debug/") ||
			strings.HasPrefix(route, "GET /server/metrics") ||
			strings.HasPrefix(route, "GET /api/metrics") ||
			strings.HasPrefix(route, "GET /metrics") {
			continue
		}
		public[route] = struct{}{}
	}
	return public
}

func routeSetFromSpec(paths map[string]interface{}, t *testing.T) map[string]struct{} {
	t.Helper()
	routes := make(map[string]struct{})
	for path, item := range paths {
		methods, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("OpenAPI path %q has invalid operations", path)
			continue
		}
		for method := range methods {
			if method == "parameters" || method == "servers" {
				continue
			}
			routes[strings.ToUpper(method)+" "+path] = struct{}{}
		}
	}
	return routes
}

func readAIDebugRoutes(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "AI.md"))
	if err != nil {
		t.Fatalf("read AI.md debug route contract: %v", err)
	}
	var routes []string
	inInventory := false
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "### pprof Endpoints" || trimmed == "### Debug API Endpoints" {
			inInventory = true
			continue
		}
		if strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") {
			inInventory = false
			continue
		}
		if !inInventory || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		match := regexp.MustCompile("`(/debug/[^`]+)`").FindStringSubmatch(line)
		if match != nil {
			routes = append(routes, "GET "+match[1])
		}
	}
	implementationEndpoint := regexp.MustCompile(`r\.Get\("/(memory|goroutines)"`)
	for _, match := range implementationEndpoint.FindAllStringSubmatch(string(contents), -1) {
		routes = append(routes, "GET /debug/"+match[1])
	}
	if len(routes) == 0 {
		t.Fatal("AI.md has no debug endpoint route contract")
	}
	return routes
}

func readAITemplateAPIRoutes(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "AI.md"))
	if err != nil {
		t.Fatalf("read AI.md API route contract: %v", err)
	}
	var routes []string
	inInventory := false
	seenInventory := false
	inTokenRoutes := false
	tokenRoutePattern := regexp.MustCompile("`(GET|POST|PUT|PATCH|DELETE) /api/\\{api_version\\}(/server/preferences/tokens[^` ]*)`")
	healthTextRoute := regexp.MustCompile(`/api/\{api_version\}/server/healthz\.txt`)
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**Routes** (web forms are cookie-authenticated") {
			inTokenRoutes = true
		} else if inTokenRoutes && strings.HasPrefix(trimmed, "#### ") {
			inTokenRoutes = false
		}
		if inTokenRoutes {
			for _, match := range tokenRoutePattern.FindAllStringSubmatch(line, -1) {
				routes = append(routes, match[1]+" "+match[2])
			}
		}
		if healthTextRoute.MatchString(line) {
			routes = append(routes, "GET /server/healthz.txt")
		}
		if trimmed == "### /server/ API Endpoints" || trimmed == "## Root-Level Endpoints" {
			inInventory = true
			seenInventory = true
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			inInventory = false
			continue
		}
		if !inInventory || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		path := strings.Trim(strings.TrimSpace(cells[0]), "`")
		methods := strings.Split(strings.TrimSpace(cells[1]), ",")
		if !strings.HasPrefix(path, "/api/{api_version}/") {
			continue
		}
		if strings.Contains(path, "*") {
			continue
		}
		path = strings.TrimPrefix(path, "/api/{api_version}")
		if strings.HasSuffix(path, "[.txt]") {
			base := strings.TrimSuffix(path, "[.txt]")
			for _, method := range methods {
				routes = append(routes, strings.TrimSpace(method)+" "+base, strings.TrimSpace(method)+" "+base+".txt")
			}
			continue
		}
		if strings.Contains(path, "[/{service}]") {
			base := strings.TrimSuffix(path, "[/{service}]")
			for _, method := range methods {
				for _, service := range []string{"", "/prometheus", "/grafana", "/loki"} {
					routes = append(routes, strings.TrimSpace(method)+" "+base+service)
				}
			}
			continue
		}
		for _, method := range methods {
			method = strings.TrimSpace(method)
			if method == "ALL" {
				continue
			}
			routes = append(routes, method+" "+path)
		}
	}
	if !seenInventory {
		t.Fatal("AI.md has no ### /server/ API Endpoints route contract")
	}
	routes = append(routes, "GET /", "POST /server/mcp")
	return routes
}

func readIDEAAPIRoutes(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "IDEA.md"))
	if err != nil {
		t.Fatalf("read IDEA.md API route inventory: %v", err)
	}
	routePattern := regexp.MustCompile("`(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS) (/api/\\{api_version\\}/[^`]+)`")
	var routes []string
	inInventory := false
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSpace(line) == "### API routes" {
			inInventory = true
			continue
		}
		if inInventory && strings.HasPrefix(strings.TrimSpace(line), "### ") {
			break
		}
		if !inInventory {
			continue
		}
		for _, match := range routePattern.FindAllStringSubmatch(line, -1) {
			path := strings.TrimPrefix(match[2], "/api/{api_version}")
			if path == "" {
				path = "/"
			}
			if strings.HasSuffix(path, "[.txt]") {
				base := strings.TrimSuffix(path, "[.txt]")
				routes = append(routes, match[1]+" "+base, match[1]+" "+base+".txt")
				continue
			}
			routes = append(routes, match[1]+" "+path)
		}
	}
	if !inInventory {
		t.Fatal("IDEA.md has no ### API routes inventory")
	}
	return routes
}

func assertRouteSet(t *testing.T, name string, expected, actual map[string]struct{}) {
	t.Helper()
	var missing, extra []string
	for route := range expected {
		if _, ok := actual[route]; !ok {
			missing = append(missing, route)
		}
	}
	for route := range actual {
		if _, ok := expected[route]; !ok {
			extra = append(extra, route)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s route drift: missing=%v extra=%v", name, missing, extra)
	}
}
