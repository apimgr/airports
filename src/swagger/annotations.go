package swagger

import "github.com/apimgr/airports/src/common/i18n"

// BuildSpec returns the hand-maintained OpenAPI 3.0 specification for the
// Airports API. Until full auto-generation from handler annotations is
// wired up, this file is the single source of truth for the spec served
// at /api/{api_version}/server/swagger and the unversioned alias
// /api/swagger (per AI.md PART 14).
// BuildSpec returns the OpenAPI 3.0 spec for the Airports API.
// version is injected at runtime from the build-time Version variable so
// the spec always reflects the running binary version. apiVersion is the
// configured API path segment (server.api_version, default "v1") and must
// never be hardcoded (PART 14, "NEVER hardcode v1"). lang selects the
// translated description/summary text (PART 30) — an empty lang falls
// back to English via i18n's own fallback chain.
func BuildSpec(version, apiVersion string, lang string) map[string]interface{} {
	if version == "" {
		version = "dev"
	}
	if apiVersion == "" {
		apiVersion = "v1"
	}
	t := func(key string) string { return i18n.T(lang, key) }
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Airports API",
			"description": t("swagger.info_description"),
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
			{"url": "/api/" + apiVersion, "description": "API " + apiVersion},
		},
		"tags": []map[string]string{
			{"name": "airports", "description": t("swagger.tag_airports")},
			{"name": "geoip", "description": t("swagger.tag_geoip")},
			{"name": "server", "description": t("swagger.tag_server")},
		},
		"paths": map[string]interface{}{
			// Airport endpoints
			"/airports": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "listAirports",
					"summary":     t("swagger.list_airports_summary"),
					"description": t("swagger.list_airports_description"),
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "description": t("swagger.param_page"), "schema": map[string]string{"type": "integer"}},
						{"name": "limit", "in": "query", "description": t("swagger.param_limit_250"), "schema": map[string]string{"type": "integer"}},
						{"name": "format", "in": "query", "description": t("swagger.param_format_json_csv_geojson"), "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_airports_list")},
					},
				},
			},
			"/airports/{ident}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "getAirport",
					"summary":     t("swagger.get_airport_summary"),
					"parameters": []map[string]interface{}{
						{"name": "ident", "in": "path", "required": true, "description": t("swagger.param_ident"), "schema": map[string]string{"type": "string"}},
						{"name": "format", "in": "query", "description": t("swagger.param_format_json_csv_geojson_text"), "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_airport_object")},
						"404": map[string]interface{}{"description": t("swagger.resp_airport_not_found")},
					},
				},
			},
			"/airports/search": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "searchAirports",
					"summary":     t("swagger.search_airports_summary"),
					"description": t("swagger.search_airports_description"),
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "required": true, "description": t("swagger.param_query"), "schema": map[string]string{"type": "string"}},
						{"name": "limit", "in": "query", "description": t("swagger.param_limit_50"), "schema": map[string]string{"type": "integer"}},
						{"name": "format", "in": "query", "description": t("swagger.param_format_json_csv_geojson_text"), "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_airports_matching")},
					},
				},
			},
			"/airports/nearby": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "nearbyAirports",
					"summary":     t("swagger.nearby_summary"),
					"description": t("swagger.nearby_description"),
					"parameters": []map[string]interface{}{
						{"name": "lat", "in": "query", "required": true, "description": t("swagger.param_lat"), "schema": map[string]string{"type": "number"}},
						{"name": "lon", "in": "query", "required": true, "description": t("swagger.param_lon"), "schema": map[string]string{"type": "number"}},
						{"name": "radius", "in": "query", "description": t("swagger.param_radius"), "schema": map[string]string{"type": "number"}},
						{"name": "limit", "in": "query", "description": t("swagger.param_limit_20"), "schema": map[string]string{"type": "integer"}},
						{"name": "units", "in": "query", "description": t("swagger.param_units"), "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_airports_by_distance")},
					},
				},
			},
			"/airports/within": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "airportsWithin",
					"summary":     t("swagger.within_summary"),
					"description": t("swagger.within_description"),
					"parameters": []map[string]interface{}{
						{"name": "lat_min", "in": "query", "required": true, "description": t("swagger.param_lat_min"), "schema": map[string]string{"type": "number"}},
						{"name": "lat_max", "in": "query", "required": true, "description": t("swagger.param_lat_max"), "schema": map[string]string{"type": "number"}},
						{"name": "lon_min", "in": "query", "required": true, "description": t("swagger.param_lon_min"), "schema": map[string]string{"type": "number"}},
						{"name": "lon_max", "in": "query", "required": true, "description": t("swagger.param_lon_max"), "schema": map[string]string{"type": "number"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_airports_within_box")},
					},
				},
			},
			"/airports/autocomplete": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "autocompleteAirports",
					"summary":     t("swagger.autocomplete_summary"),
					"description": t("swagger.autocomplete_description"),
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "required": true, "description": t("swagger.param_prefix_query"), "schema": map[string]string{"type": "string"}},
						{"name": "limit", "in": "query", "description": t("swagger.param_limit_10"), "schema": map[string]string{"type": "integer"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_autocomplete_suggestions")},
					},
				},
			},
			// Country endpoints
			"/countries": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "listCountries",
					"summary":     t("swagger.list_countries_summary"),
					"description": t("swagger.list_countries_description"),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_countries_list")},
					},
				},
			},
			// GeoIP endpoints
			"/geoip": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"operationId": "geoipCaller",
					"summary":     t("swagger.geoip_caller_summary"),
					"description": t("swagger.geoip_caller_description"),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_geoip_result")},
					},
				},
			},
			"/geoip/{ip}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"geoip"},
					"operationId": "geoipLookup",
					"summary":     t("swagger.geoip_lookup_summary"),
					"parameters": []map[string]interface{}{
						{"name": "ip", "in": "path", "required": true, "description": t("swagger.param_ip"), "schema": map[string]string{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_geoip_result")},
					},
				},
			},
			// Stats endpoint
			"/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"airports"},
					"operationId": "airportStats",
					"summary":     t("swagger.stats_summary"),
					"description": t("swagger.stats_description"),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_stats_object")},
					},
				},
			},
			// Server endpoints
			"/server/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"server"},
					"operationId": "healthCheck",
					"summary":     t("swagger.health_summary"),
					"description": t("swagger.health_description"),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_health_status")},
					},
				},
			},
			"/server/about": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"server"},
					"operationId": "serverAbout",
					"summary":     t("swagger.about_summary"),
					"description": t("swagger.about_description"),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": t("swagger.resp_server_metadata")},
					},
				},
			},
		},
		"components": map[string]interface{}{},
	}
}
