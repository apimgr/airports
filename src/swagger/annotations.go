package swagger

// BuildSpec returns the hand-maintained OpenAPI 3.0 specification for the
// Airports API. Until full auto-generation from handler annotations is
// wired up, this file is the single source of truth for the spec served
// at /api/{api_version}/server/swagger and the unversioned alias
// /api/swagger (per AI.md PART 14).
// BuildSpec returns the OpenAPI 3.0 spec for the Airports API.
// version is injected at runtime from the build-time Version variable so
// the spec always reflects the running binary version.
func BuildSpec(version string) map[string]interface{} {
	if version == "" {
		version = "dev"
	}
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Airports API",
			"description": "Global airport location information API with GeoIP integration",
			"version":     version,
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
						"200": map[string]interface{}{"description": "Successful response"},
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
						"200": map[string]interface{}{"description": "Successful response"},
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
}
