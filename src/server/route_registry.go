package server

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/apimgr/airports/src/mode"
	swaggerpkg "github.com/apimgr/airports/src/swagger"
	"github.com/go-chi/chi/v5"
)

// Route defines one API operation and every surface that derives from it.
type Route struct {
	Method            string
	Path              string
	Aliases           []string
	AliasOperationIDs map[string]string
	OperationID       string
	Summary           string
	Auth              string
	Listed            bool
	Internal          bool
	DebugOnly         bool
	Request           any
	Response          any
	Primary           string
	Message           string
	Binary            string
	Handler           http.HandlerFunc
	MountedWhen       string
	Mounted           bool
}

// routeRegistry is the single source for API route mounting and metadata.
type routeRegistry struct {
	router chi.Router
	routes []Route
}

// Add mounts a route and its aliases, then records the shared route contract.
func (registry *routeRegistry) Add(route Route) {
	if route.Summary == "" {
		route.Summary = route.Method + " " + route.Path
	}
	if route.Auth == "" {
		route.Auth = "none"
	}
	mounted := route.MountedWhen == "" || route.Mounted
	if route.DebugOnly {
		mounted = mounted && mode.ShouldShowDebugEndpoints()
	}
	route.Mounted = mounted
	if mounted {
		handler := route.Handler
		if strings.Contains(route.Path, "/api/") && !strings.Contains(route.Path, "/server/healthz") && !strings.Contains(route.Path, "/server/mcp") && route.Binary == "" {
			handler = textNegotiatedHandler(route, handler)
		}
		registry.router.Method(route.Method, route.Path, handler)
		for _, alias := range route.Aliases {
			registry.router.Method(route.Method, alias, handler)
		}
		if route.Method == http.MethodGet && (strings.HasPrefix(route.Path, "/.well-known/") || route.Path == "/llms.txt") {
			registry.router.Method(http.MethodHead, route.Path, headOnlyHandler(handler))
			for _, alias := range route.Aliases {
				registry.router.Method(http.MethodHead, alias, headOnlyHandler(handler))
			}
		}
	}
	registry.routes = append(registry.routes, route)
}

type headOnlyResponseWriter struct {
	http.ResponseWriter
}

func (w headOnlyResponseWriter) Write(body []byte) (int, error) {
	return len(body), nil
}

func headOnlyHandler(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(headOnlyResponseWriter{ResponseWriter: w}, r)
	})
}

// Snapshot returns a stable copy of the registered route definitions.
func (registry *routeRegistry) Snapshot() []Route {
	if registry == nil {
		return nil
	}
	routes := make([]Route, len(registry.routes))
	copy(routes, registry.routes)
	return routes
}

// OpenAPIRoutes returns only routes allowed on the public documentation surfaces.
func (registry *routeRegistry) OpenAPIRoutes(apiBase string) []swaggerpkg.OpenAPIRoute {
	return registry.surfaceRoutes(apiBase, func(route Route) bool {
		return !route.Internal && route.Listed && route.Auth != "operator" && route.Auth != "service"
	})
}

// MCPRoutes returns every tool candidate, including routes hidden from the
// public documentation surface, while excluding service and internal routes.
func (registry *routeRegistry) MCPRoutes(apiBase string) []swaggerpkg.OpenAPIRoute {
	return registry.surfaceRoutes(apiBase, func(route Route) bool {
		return !route.Internal && route.Auth != "service"
	})
}

func (registry *routeRegistry) surfaceRoutes(apiBase string, include func(Route) bool) []swaggerpkg.OpenAPIRoute {
	var routes []swaggerpkg.OpenAPIRoute
	for _, route := range registry.Snapshot() {
		if !include(route) {
			continue
		}
		paths := append([]string{route.Path}, route.Aliases...)
		for _, path := range paths {
			operationID := route.OperationID
			if path != route.Path && route.AliasOperationIDs[path] != "" {
				operationID = route.AliasOperationIDs[path]
			}
			if strings.HasPrefix(path, apiBase) {
				path = strings.TrimPrefix(path, apiBase)
				if path == "" {
					path = "/"
				}
			}
			routes = append(routes, swaggerpkg.OpenAPIRoute{
				Method: route.Method, Path: path, Summary: route.Summary,
				Auth: route.Auth != "none", AuthType: route.Auth,
				OperationID: operationID, Internal: route.Internal,
				Listed: route.Listed, Request: route.Request, Response: route.Response,
				Primary: route.Primary, Message: route.Message, Binary: route.Binary,
				MountedWhen: route.MountedWhen, Mounted: route.Mounted || route.MountedWhen == "",
			})
		}
	}
	return routes
}

// routeOperationID creates a stable camelCase operation name for an unannotated route.
func routeOperationID(method, path string) string {
	var name strings.Builder
	name.WriteString(strings.ToLower(method))
	upper := true
	for _, r := range path {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if upper {
				r = unicode.ToUpper(r)
				upper = false
			}
			name.WriteRune(r)
		} else {
			upper = true
		}
	}
	return name.String()
}
