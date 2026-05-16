package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// graphQLArgPattern builds a regex that matches `field(... name: VALUE ...)`.
// The argument value is captured in group 1. This is a minimalist parser —
// it intentionally does not aspire to be a real GraphQL grammar.
func graphQLArgPattern(field, arg, valueRE string) *regexp.Regexp {
	return regexp.MustCompile(
		`\b` + regexp.QuoteMeta(field) + `\s*\([^)]*\b` + regexp.QuoteMeta(arg) + `\s*:\s*` + valueRE + `[^)]*\)`,
	)
}

// extractGraphQLStringArg parses a quoted string argument out of
// `field(arg: "value", ...)`. Returns "" when not found.
func extractGraphQLStringArg(query, field, arg string) string {
	m := graphQLArgPattern(field, arg, `"([^"]*)"`).FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractGraphQLFloatArg parses a numeric argument out of
// `field(arg: 12.34, ...)`. Returns (0, false) when not found.
func extractGraphQLFloatArg(query, field, arg string) (float64, bool) {
	m := graphQLArgPattern(field, arg, `(-?\d+(?:\.\d+)?)`).FindStringSubmatch(query)
	if len(m) < 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// handleSwaggerUI serves the Swagger UI for API documentation with site theme
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation - Airports API</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100vh; }
        #swagger-container { flex: 1; }
        .swagger-ui { background: var(--bg-primary); }
        .swagger-ui .topbar { display: none; }
        .swagger-ui .info { color: var(--text-primary); }
        .swagger-ui .scheme-container { background: var(--bg-secondary); }
        .swagger-ui .opblock { background: var(--bg-secondary); border-color: var(--border-color); }
        .swagger-ui .opblock-tag { color: var(--text-primary); border-color: var(--border-color); }
        .swagger-ui .opblock-summary { background: var(--bg-tertiary); }
        .swagger-ui .opblock-description { color: var(--text-secondary); }
        .swagger-ui table thead tr td, .swagger-ui table thead tr th { color: var(--text-primary); border-color: var(--border-color); }
        .swagger-ui .parameter__name { color: var(--accent-primary); }
        .swagger-ui .response-col_status { color: var(--accent-success); }
        .swagger-ui input, .swagger-ui select, .swagger-ui textarea { background: var(--bg-tertiary); color: var(--text-primary); border-color: var(--border-color); }
        .swagger-ui .btn { background: var(--accent-primary); color: white; }
    </style>
</head>
<body>
    <header id="main-header">
        <div class="header-container">
            <div class="header-left">
                <button class="mobile-menu-toggle" onclick="toggleMobileMenu()">☰</button>
                <a class="logo" href="/">✈️ Airports API</a>
            </div>
            <nav id="main-nav" class="header-center">
                <a href="/">Home</a>
                <a href="/search">Search</a>
                <a href="/nearby">Nearby</a>
                <a href="/stats">Stats</a>
                <a href="/server/docs/swagger" class="active">API Docs</a>
                <a href="/server/docs/graphql">GraphQL</a>
            </nav>
            <div class="header-right">
                <button class="theme-toggle" onclick="toggleTheme()">
                    <span class="theme-icon">🌙</span>
                </button>
            </div>
        </div>
    </header>

    <div id="swagger-container">
        <div id="swagger-ui"></div>
    </div>

    <script src="/static/js/main.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "/api/v1/server/swagger",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`

	t, err := template.New("swagger").Parse(tmpl)
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, nil)
}

// handleOpenAPISpec serves the OpenAPI specification JSON
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Airports API",
			"description": "Global airport location information API with GeoIP integration",
			"version":     "1.0.0",
			"contact": map[string]string{
				"name": "Airports API",
				"url":  "https://github.com/apimgr/airports",
			},
			"license": map[string]string{
				"name": "MIT",
				"url":  "https://opensource.org/licenses/MIT",
			},
		},
		"servers": []map[string]string{
			{"url": "/api/v1", "description": "API v1"},
		},
		"tags": []map[string]string{
			{"name": "airports", "description": "Airport data endpoints"},
			{"name": "geoip", "description": "GeoIP location endpoints"},
			{"name": "admin", "description": "Admin endpoints (authentication required)"},
		},
		"paths": map[string]interface{}{
			"/airports": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"summary":     "List all airports",
					"description": "Get a paginated list of all airports",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"description": "Results per page (default: 50, max: 1000)",
							"schema":      map[string]string{"type": "integer"},
						},
						{
							"name":        "offset",
							"in":          "query",
							"description": "Pagination offset",
							"schema":      map[string]string{"type": "integer"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
						},
					},
				},
			},
			"/airports/search": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"summary":     "Search airports",
					"description": "Search airports by name, city, or code",
					"parameters": []map[string]interface{}{
						{
							"name":        "q",
							"in":          "query",
							"description": "Search query",
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]string{
					"type":   "http",
					"scheme": "bearer",
				},
			},
		},
	}

	s.respondJSON(w, http.StatusOK, spec)
}

// handleGraphQLPlayground serves the GraphQL Playground with site theme
func (s *Server) handleGraphQLPlayground(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GraphQL Playground - Airports API</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <style>
        body { margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100vh; }
        #graphql-container { flex: 1; display: flex; flex-direction: column; }
        #root { flex: 1; }
    </style>
</head>
<body>
    <header id="main-header">
        <div class="header-container">
            <div class="header-left">
                <button class="mobile-menu-toggle" onclick="toggleMobileMenu()">☰</button>
                <a class="logo" href="/">✈️ Airports API</a>
            </div>
            <nav id="main-nav" class="header-center">
                <a href="/">Home</a>
                <a href="/search">Search</a>
                <a href="/nearby">Nearby</a>
                <a href="/stats">Stats</a>
                <a href="/server/docs/swagger">API Docs</a>
                <a href="/server/docs/graphql" class="active">GraphQL</a>
            </nav>
            <div class="header-right">
                <button class="theme-toggle" onclick="toggleTheme()">
                    <span class="theme-icon">🌙</span>
                </button>
            </div>
        </div>
    </header>

    <div id="graphql-container">
        <div id="root"></div>
    </div>

    <link rel="stylesheet" href="https://unpkg.com/graphql-playground-react@1.7.28/build/static/css/index.css">
    <script src="/static/js/main.js"></script>
    <script src="https://unpkg.com/graphql-playground-react@1.7.28/build/static/js/middleware.js"></script>
    <script>
        window.addEventListener('load', function (event) {
            GraphQLPlayground.init(document.getElementById('root'), {
                endpoint: '/api/v1/server/graphql',
                settings: {
                    'editor.theme': 'dark',
                    'editor.cursorShape': 'line',
                    'theme': 'dark'
                },
                tabs: [
                    {
                        endpoint: '/api/v1/server/graphql',
                        query: '# Welcome to Airports GraphQL API\n# Press the Play button to run a query\n\nquery GetAirport {\n  airport(code: "KJFK") {\n    icao\n    iata\n    name\n    city\n    country\n    coordinates {\n      lat\n      lon\n    }\n  }\n}\n\nquery SearchNearby {\n  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {\n    icao\n    name\n    distance\n  }\n}'
                    }
                ]
            });
        });
    </script>
</body>
</html>`

	t, err := template.New("graphql").Parse(tmpl)
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, nil)
}

// graphQLRequest is the standard GraphQL-over-HTTP request shape
// (https://graphql.org/learn/serving-over-http/#post-request).
type graphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// graphQLResponse is the standard GraphQL-over-HTTP response shape.
type graphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// handleGraphQL services POST queries against the airport dataset.
//
// This is a minimal hand-rolled resolver covering the two documented
// queries (`airport(code:)` and `nearby(lat:, lon:, radius:)`) so the
// endpoint is functional without pulling in a heavy GraphQL runtime.
// Anything more elaborate returns a structured GraphQL error.
func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	// Cap request body to a sane upper bound — protects against an
	// attacker streaming an unbounded "query" body to exhaust memory.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req graphQLRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, graphQLResponse{
			Errors: []graphQLError{{Message: "invalid GraphQL request body: " + err.Error()}},
		})
		return
	}

	q := strings.TrimSpace(req.Query)
	if q == "" {
		s.respondJSON(w, http.StatusBadRequest, graphQLResponse{
			Errors: []graphQLError{{Message: "query is required"}},
		})
		return
	}

	// `airport(code: "XXXX") { ... }` — extract the code argument.
	if code := extractGraphQLStringArg(q, "airport", "code"); code != "" {
		apt, err := s.airports.GetByCode(code)
		if err != nil {
			s.respondJSON(w, http.StatusOK, graphQLResponse{
				Errors: []graphQLError{{Message: "airport not found: " + code}},
			})
			return
		}
		s.respondJSON(w, http.StatusOK, graphQLResponse{
			Data: map[string]interface{}{"airport": apt},
		})
		return
	}

	// `nearby(lat: N, lon: N, radius: N) { ... }` — extract numeric args.
	if lat, latOK := extractGraphQLFloatArg(q, "nearby", "lat"); latOK {
		lon, lonOK := extractGraphQLFloatArg(q, "nearby", "lon")
		if !lonOK {
			s.respondJSON(w, http.StatusBadRequest, graphQLResponse{
				Errors: []graphQLError{{Message: "nearby query requires lon"}},
			})
			return
		}
		radius, ok := extractGraphQLFloatArg(q, "nearby", "radius")
		if !ok {
			radius = 50
		}
		results := s.airports.GetNearbyWithDistance(lat, lon, radius, 20, "metric")
		s.respondJSON(w, http.StatusOK, graphQLResponse{
			Data: map[string]interface{}{"nearby": results},
		})
		return
	}

	s.respondJSON(w, http.StatusOK, graphQLResponse{
		Errors: []graphQLError{{Message: "unsupported query — supported: airport(code:), nearby(lat:, lon:, radius:)"}},
	})
}
