package server

import (
	"html/template"
	"net/http"
)

// handleSwaggerUI serves the Swagger UI for API documentation
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Airports API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .swagger-ui .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "/api/v1/openapi.json",
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
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{
										"$ref": "#/components/schemas/AirportListResponse",
									},
								},
							},
						},
					},
				},
			},
			"/airports/{code}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"summary":     "Get airport by code",
					"description": "Get airport details by ICAO or IATA code",
					"parameters": []map[string]interface{}{
						{
							"name":        "code",
							"in":          "path",
							"description": "ICAO or IATA code (e.g., KJFK or JFK)",
							"required":    true,
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
						},
						"404": map[string]interface{}{
							"description": "Airport not found",
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
						{
							"name":        "city",
							"in":          "query",
							"description": "Filter by city",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name":        "country",
							"in":          "query",
							"description": "Filter by country code (e.g., US)",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name":        "limit",
							"in":          "query",
							"description": "Max results (default: 50, max: 1000)",
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
			"/airports/nearby": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"summary":     "Find nearby airports",
					"description": "Find airports near coordinates",
					"parameters": []map[string]interface{}{
						{
							"name":        "lat",
							"in":          "query",
							"description": "Latitude",
							"required":    true,
							"schema":      map[string]string{"type": "number"},
						},
						{
							"name":        "lon",
							"in":          "query",
							"description": "Longitude",
							"required":    true,
							"schema":      map[string]string{"type": "number"},
						},
						{
							"name":        "radius",
							"in":          "query",
							"description": "Search radius in km (default: 50, max: 500)",
							"schema":      map[string]string{"type": "integer"},
						},
						{
							"name":        "limit",
							"in":          "query",
							"description": "Max results (default: 20)",
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
			"/geoip": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"summary":     "Lookup current IP",
					"description": "Get geolocation for request IP",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
						},
					},
				},
			},
			"/geoip/{ip}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"summary":     "Lookup specific IP",
					"description": "Get geolocation for specific IP address",
					"parameters": []map[string]interface{}{
						{
							"name":        "ip",
							"in":          "path",
							"description": "IPv4 or IPv6 address",
							"required":    true,
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
			"schemas": map[string]interface{}{
				"Airport": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"icao":      map[string]string{"type": "string"},
						"iata":      map[string]string{"type": "string"},
						"name":      map[string]string{"type": "string"},
						"city":      map[string]string{"type": "string"},
						"state":     map[string]string{"type": "string"},
						"country":   map[string]string{"type": "string"},
						"elevation": map[string]string{"type": "integer"},
						"lat":       map[string]string{"type": "number"},
						"lon":       map[string]string{"type": "number"},
						"tz":        map[string]string{"type": "string"},
					},
				},
			},
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

// handleGraphQLPlayground serves the GraphQL Playground
func (s *Server) handleGraphQLPlayground(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Airports GraphQL Playground</title>
    <link rel="stylesheet" href="https://unpkg.com/graphql-playground-react@1.7.28/build/static/css/index.css">
    <link rel="shortcut icon" href="https://unpkg.com/graphql-playground-react@1.7.28/build/favicon.png">
    <script src="https://unpkg.com/graphql-playground-react@1.7.28/build/static/js/middleware.js"></script>
</head>
<body>
    <div id="root"></div>
    <script>
        window.addEventListener('load', function (event) {
            GraphQLPlayground.init(document.getElementById('root'), {
                endpoint: '/api/v1/graphql',
                settings: {
                    'editor.theme': 'dark',
                    'editor.cursorShape': 'line'
                },
                tabs: [
                    {
                        endpoint: '/api/v1/graphql',
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

// handleGraphQL handles GraphQL queries
func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	// For now, return a simple message
	// Full GraphQL implementation would go here
	s.respondJSON(w, http.StatusOK, map[string]string{
		"message": "GraphQL endpoint - Full implementation coming soon",
		"playground": "/graphql",
	})
}
