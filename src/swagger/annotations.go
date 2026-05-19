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
			{"name": "server", "description": "Server health and metadata endpoints"},
		},
		"paths": map[string]interface{}{
			// Airport endpoints
			"/airports": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "listAirports",
					"summary":     "List airports",
					"description": "Get a paginated list of airports",
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "description": "Page number (default: 1)", "schema": map[string]string{"type": "integer"}},
						{"name": "limit", "in": "query", "description": "Results per page (default: 250, max: 1000)", "schema": map[string]string{"type": "integer"}},
						{"name": "format", "in": "query", "description": "Response format: json, csv, geojson", "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Paginated list of airports"},
					},
				},
			},
			"/airports/{ident}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "getAirport",
					"summary":     "Get airport by ICAO or IATA code",
					"parameters": []map[string]interface{}{
						{"name": "ident", "in": "path", "required": true, "description": "ICAO or IATA code", "schema": map[string]string{"type": "string"}},
						{"name": "format", "in": "query", "description": "Response format: json, csv, geojson, text", "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Airport object"},
						"404": map[string]interface{}{"description": "Airport not found"},
					},
				},
			},
			"/airports/search": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "searchAirports",
					"summary":     "Search airports",
					"description": "Full-text search across airport name, city, country, and codes",
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "required": true, "description": "Search query", "schema": map[string]string{"type": "string"}},
						{"name": "limit", "in": "query", "description": "Max results (default: 50)", "schema": map[string]string{"type": "integer"}},
						{"name": "format", "in": "query", "description": "Response format: json, csv, geojson, text", "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List of matching airports"},
					},
				},
			},
			"/airports/nearby": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "nearbyAirports",
					"summary":     "Airports within radius",
					"description": "Find airports within a given radius of a lat/lon point",
					"parameters": []map[string]interface{}{
						{"name": "lat", "in": "query", "required": true, "description": "Latitude", "schema": map[string]string{"type": "number"}},
						{"name": "lon", "in": "query", "required": true, "description": "Longitude", "schema": map[string]string{"type": "number"}},
						{"name": "radius", "in": "query", "description": "Radius in km (default: 50)", "schema": map[string]string{"type": "number"}},
						{"name": "limit", "in": "query", "description": "Max results (default: 20)", "schema": map[string]string{"type": "integer"}},
						{"name": "units", "in": "query", "description": "Distance units: metric (km) or imperial (mi)", "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Airports sorted by distance"},
					},
				},
			},
			"/airports/within": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "airportsWithin",
					"summary":     "Airports within bounding box",
					"description": "Find all airports within a lat/lon bounding box",
					"parameters": []map[string]interface{}{
						{"name": "lat_min", "in": "query", "required": true, "description": "Minimum latitude", "schema": map[string]string{"type": "number"}},
						{"name": "lat_max", "in": "query", "required": true, "description": "Maximum latitude", "schema": map[string]string{"type": "number"}},
						{"name": "lon_min", "in": "query", "required": true, "description": "Minimum longitude", "schema": map[string]string{"type": "number"}},
						{"name": "lon_max", "in": "query", "required": true, "description": "Maximum longitude", "schema": map[string]string{"type": "number"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Airports within the bounding box"},
					},
				},
			},
			"/airports/autocomplete": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "autocompleteAirports",
					"summary":     "Airport autocomplete",
					"description": "Autocomplete suggestions for airport name or code",
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "required": true, "description": "Prefix query", "schema": map[string]string{"type": "string"}},
						{"name": "limit", "in": "query", "description": "Max suggestions (default: 10)", "schema": map[string]string{"type": "integer"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Autocomplete suggestions"},
					},
				},
			},
			// Country endpoints
			"/countries": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "listCountries",
					"summary":     "List countries",
					"description": "Get all countries that have airports in the dataset",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List of countries"},
					},
				},
			},
			// GeoIP endpoints
			"/geoip": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"operationId": "geoipCaller",
					"summary":     "GeoIP lookup for caller",
					"description": "Returns GeoIP location for the caller's IP address",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "GeoIP result"},
					},
				},
			},
			"/geoip/{ip}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"operationId": "geoipLookup",
					"summary":     "GeoIP lookup for IP",
					"parameters": []map[string]interface{}{
						{"name": "ip", "in": "path", "required": true, "description": "IPv4 or IPv6 address", "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "GeoIP result"},
					},
				},
			},
			// Stats endpoint
			"/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "airportStats",
					"summary":     "Dataset statistics",
					"description": "Returns aggregate statistics about the airport dataset",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Statistics object"},
					},
				},
			},
			// Server endpoints
			"/server/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"server"},
					"operationId": "healthCheck",
					"summary":     "Health check",
					"description": "Returns server health status and loaded dataset counts",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Health status"},
					},
				},
			},
			"/server/about": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"server"},
					"operationId": "serverAbout",
					"summary":     "Server version info",
					"description": "Returns build metadata including version, commit, and build date",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Server metadata"},
					},
				},
			},
		},
		"components": map[string]interface{}{},
	}
}
