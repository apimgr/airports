package swagger

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/apimgr/airports/src/common/i18n"
)

// BuildSpec returns the annotated OpenAPI 3.0 operation metadata for the
// Airports API. The running router supplies the authoritative route and
// method set through BuildSpecWithRoutes.
// version is injected at runtime from the build-time Version variable so
// the spec always reflects the running binary version. apiVersion is the
// configured API path segment (server.api_version, default "v1") and must
// never be hardcoded (PART 14, "NEVER hardcode v1"). lang selects the
// translated description/summary text (PART 30) — an empty lang falls
// back to English via i18n's own fallback chain.
func BuildSpec(version, apiVersion string, lang string, baseURL ...string) map[string]interface{} {
	if version == "" {
		version = "dev"
	}
	if apiVersion == "" {
		apiVersion = "v1"
	}
	t := func(key string) string { return i18n.TranslateText(lang, key) }
	prefix := ""
	if len(baseURL) > 0 {
		prefix = strings.TrimSuffix(baseURL[0], "/")
	}
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
			{"url": prefix + "/api/" + apiVersion, "description": "API " + apiVersion},
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
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Airport": map[string]interface{}{
					"type":     "object",
					"required": []string{"icao", "iata", "name", "city", "state", "country", "elevation", "lat", "lon", "type"},
					"properties": map[string]interface{}{
						"icao":      map[string]string{"type": "string"},
						"iata":      map[string]string{"type": "string"},
						"name":      map[string]string{"type": "string"},
						"city":      map[string]string{"type": "string"},
						"state":     map[string]string{"type": "string"},
						"country":   map[string]string{"type": "string"},
						"elevation": map[string]string{"type": "integer"},
						"lat":       map[string]string{"type": "number", "format": "double"},
						"lon":       map[string]string{"type": "number", "format": "double"},
						"type":      map[string]string{"type": "string"},
						"tz":        map[string]string{"type": "string", "description": "IANA timezone when available; omitted when unknown"},
					},
				},
				"Pagination": map[string]interface{}{
					"type":     "object",
					"required": []string{"page", "limit", "total", "pages"},
					"properties": map[string]interface{}{
						"page":  map[string]string{"type": "integer"},
						"limit": map[string]string{"type": "integer"},
						"total": map[string]string{"type": "integer"},
						"pages": map[string]string{"type": "integer"},
					},
				},
				"AirportListResponse": map[string]interface{}{
					"type":     "object",
					"required": []string{"ok", "data", "pagination"},
					"properties": map[string]interface{}{
						"ok":         map[string]string{"type": "boolean"},
						"data":       map[string]interface{}{"type": "array", "items": map[string]string{"$ref": "#/components/schemas/Airport"}},
						"pagination": map[string]string{"$ref": "#/components/schemas/Pagination"},
					},
				},
				"AirportItemResponse": map[string]interface{}{
					"type":     "object",
					"required": []string{"ok", "data"},
					"properties": map[string]interface{}{
						"ok":   map[string]string{"type": "boolean"},
						"data": map[string]string{"$ref": "#/components/schemas/Airport"},
					},
				},
				"AirportDataset": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": map[string]string{"$ref": "#/components/schemas/Airport"},
				},
				"AirportWithDistance": map[string]interface{}{
					"allOf": []map[string]interface{}{
						{"$ref": "#/components/schemas/Airport"},
						{"type": "object", "required": []string{"distance", "distance_unit"}, "properties": map[string]interface{}{
							"distance":      map[string]string{"type": "number", "format": "double"},
							"distance_unit": map[string]string{"type": "string"},
						}},
					},
				},
				"Coordinate": map[string]interface{}{
					"type":     "object",
					"required": []string{"lat", "lon"},
					"properties": map[string]interface{}{
						"lat": map[string]string{"type": "number", "format": "double"},
						"lon": map[string]string{"type": "number", "format": "double"},
					},
				},
				"NearbyAirportsResult": map[string]interface{}{
					"type":     "object",
					"required": []string{"data", "center", "radius", "radius_unit", "units", "count"},
					"properties": map[string]interface{}{
						"data":        map[string]interface{}{"type": "array", "items": map[string]string{"$ref": "#/components/schemas/AirportWithDistance"}},
						"center":      map[string]string{"$ref": "#/components/schemas/Coordinate"},
						"radius":      map[string]string{"type": "number", "format": "double"},
						"radius_unit": map[string]string{"type": "string"},
						"units":       map[string]string{"type": "string"},
						"count":       map[string]string{"type": "integer"},
					},
				},
				"BoundingBoxResult": map[string]interface{}{
					"type":     "object",
					"required": []string{"data", "count"},
					"properties": map[string]interface{}{
						"data":  map[string]interface{}{"type": "array", "items": map[string]string{"$ref": "#/components/schemas/Airport"}},
						"count": map[string]string{"type": "integer"},
					},
				},
				"NearbyAirportsResponse": map[string]interface{}{
					"type": "object", "required": []string{"ok", "data"},
					"properties": map[string]interface{}{
						"ok":   map[string]string{"type": "boolean"},
						"data": map[string]string{"$ref": "#/components/schemas/NearbyAirportsResult"},
					},
				},
				"BoundingBoxResponse": map[string]interface{}{
					"type": "object", "required": []string{"ok", "data"},
					"properties": map[string]interface{}{
						"ok":   map[string]string{"type": "boolean"},
						"data": map[string]string{"$ref": "#/components/schemas/BoundingBoxResult"},
					},
				},
			},
		},
	}
}

// OpenAPIRoute is route metadata collected from the live HTTP router.
type OpenAPIRoute struct {
	Method      string
	Path        string
	Summary     string
	Auth        bool
	AuthType    string
	OperationID string
	Listed      bool
	Internal    bool
	Request     any
	Response    any
	Primary     string
	Message     string
	Binary      string
	MountedWhen string
	Mounted     bool
}

// BuildSpecWithRoutes combines operation annotations with the running route
// table so optional routes and compatibility surfaces stay in sync with the
// handlers actually mounted for this process.
func BuildSpecWithRoutes(version, apiVersion, lang string, routes []OpenAPIRoute, baseURL ...string) map[string]interface{} {
	spec := BuildSpec(version, apiVersion, lang, baseURL...)
	paths := spec["paths"].(map[string]interface{})
	for _, route := range routes {
		if route.Internal {
			continue
		}
		method := strings.ToLower(route.Method)
		if method != "get" && method != "post" && method != "put" && method != "patch" && method != "delete" && method != "head" && method != "options" {
			continue
		}
		pathItem, ok := paths[route.Path].(map[string]interface{})
		if !ok {
			pathItem = map[string]interface{}{}
			paths[route.Path] = pathItem
		}
		operation, ok := pathItem[method].(map[string]interface{})
		if !ok {
			operation = map[string]interface{}{
				"tags":        []string{openAPITag(route.Path)},
				"operationId": openAPIOperationID(method, route.Path),
				"summary":     route.Summary,
				"description": i18n.TranslateText(lang, "swagger.route_description"),
				"responses": map[string]interface{}{
					"200": map[string]interface{}{"description": "Successful response"},
					"400": map[string]interface{}{"description": "Invalid request"},
				},
			}
			if parameters := openAPIPathParameters(route.Path); len(parameters) > 0 {
				operation["parameters"] = parameters
			}
			if method == "get" && (route.Path == "/server/graphql" || route.Path == "/api/graphql") {
				operation["parameters"] = []map[string]interface{}{
					{"name": "query", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
					{"name": "variables", "in": "query", "required": false, "schema": map[string]interface{}{"type": "string"}},
					{"name": "operationName", "in": "query", "required": false, "schema": map[string]interface{}{"type": "string"}},
				}
			}
			if method == "post" || method == "put" || method == "patch" {
				operation["requestBody"] = map[string]interface{}{
					"required": method == "post" || method == "put",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "additionalProperties": true}},
					},
				}
			}
			pathItem[method] = operation
		}
		if route.Request != nil {
			operation["requestBody"] = cloneJSONValue(route.Request)
		}
		if route.Response != nil {
			operation["responses"] = cloneJSONValue(route.Response)
		} else {
			applyOpenAPIRouteSchema(operation, route.Path, method)
		}
		if route.OperationID != "" {
			operation["operationId"] = route.OperationID
		}
		if route.Auth {
			operation["security"] = []map[string][]string{{"BearerAuth": {}}}
		}
		if route.MountedWhen != "" {
			operation["x-mounted-when"] = route.MountedWhen
		}
	}
	components := spec["components"].(map[string]interface{})
	components["securitySchemes"] = map[string]interface{}{
		"BearerAuth": map[string]string{"type": "http", "scheme": "bearer"},
	}
	return spec
}

// CanonicalRouteModels extracts the established operation contract for one
// route so the server registry can own and reuse its OpenAPI models.
func CanonicalRouteModels(version, apiVersion, lang, method, path string) (operationID, summary string, request, response any) {
	spec := BuildSpec(version, apiVersion, lang)
	paths, _ := spec["paths"].(map[string]interface{})
	pathItem, _ := paths[path].(map[string]interface{})
	operation, _ := pathItem[strings.ToLower(method)].(map[string]interface{})
	if operation == nil {
		operation = map[string]interface{}{
			"operationId": openAPIOperationID(method, path),
			"summary":     method + " " + path,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "Successful response"},
				"400": map[string]interface{}{"description": "Invalid request"},
			},
		}
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			operation["requestBody"] = map[string]interface{}{
				"required": method == http.MethodPost || method == http.MethodPut,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "additionalProperties": true}},
				},
			}
		}
	}
	applyOpenAPIRouteSchema(operation, path, strings.ToLower(method))
	operationID, _ = operation["operationId"].(string)
	summary, _ = operation["summary"].(string)
	request = cloneJSONValue(operation["requestBody"])
	response = cloneJSONValue(operation["responses"])
	return operationID, summary, request, response
}

func cloneJSONValue(value any) any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var clone any
	if err := json.Unmarshal(encoded, &clone); err != nil {
		return value
	}
	return clone
}

func openAPIPathParameters(path string) []map[string]interface{} {
	var parameters []map[string]interface{}
	for {
		start := strings.Index(path, "{")
		if start < 0 {
			break
		}
		end := strings.Index(path[start:], "}")
		if end < 0 {
			break
		}
		name := path[start+1 : start+end]
		parameters = append(parameters, map[string]interface{}{
			"name": name, "in": "path", "required": true,
			"schema": map[string]string{"type": "string"},
		})
		path = path[start+end+1:]
	}
	return parameters
}

func applyOpenAPIRouteSchema(operation map[string]interface{}, path, method string) {
	if method == "get" {
		switch path {
		case "/airports", "/airports/search":
			setOpenAPIJSONResponse(operation, "#/components/schemas/AirportListResponse")
		case "/airports/nearby":
			setOpenAPIJSONResponse(operation, "#/components/schemas/NearbyAirportsResponse")
		case "/airports/within":
			setOpenAPIJSONResponse(operation, "#/components/schemas/BoundingBoxResponse")
		case "/airports/{ident}":
			setOpenAPIJSONResponse(operation, "#/components/schemas/AirportItemResponse")
		case "/airports.json":
			setOpenAPIJSONResponse(operation, "#/components/schemas/AirportDataset")
		}
	}
	if method == "post" && path == "/server/graphql" {
		operation["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object", "required": []string{"query"},
					"properties": map[string]interface{}{
						"query":         map[string]string{"type": "string"},
						"operationName": map[string]string{"type": "string"},
						"variables":     map[string]string{"type": "object"},
					}, "additionalProperties": false,
				},
			}},
		}
	}
	if method == "post" && path == "/server/preferences" {
		operation["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object", "properties": map[string]interface{}{
						"theme": map[string]string{"type": "string"},
						"lang":  map[string]string{"type": "string"},
					}, "additionalProperties": true,
				},
			}},
		}
	}
	if method == "post" && path == "/server/contact" {
		fields := strings.Fields("name email type reported_url reason copyrighted_work good_faith accuracy signature request_kind subject message captcha_id captcha_answer captcha captcha_response g-recaptcha-response h-captcha-response website form_token step review_token confirmed security_id component endpoint severity summary steps_to_reproduce impact suggested_fix cve_requested timeline_days credit_preference credit_handle researcher_gpg disclosure_agreement")
		properties := make(map[string]interface{}, len(fields))
		for _, field := range fields {
			properties[field] = map[string]string{"type": "string"}
		}
		operation["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{"application/json": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object", "properties": properties, "additionalProperties": false},
			}},
		}
	}
}

func setOpenAPIJSONResponse(operation map[string]interface{}, schemaRef string) {
	responses, _ := operation["responses"].(map[string]interface{})
	response, _ := responses["200"].(map[string]interface{})
	if response == nil {
		return
	}
	content, _ := response["content"].(map[string]interface{})
	if content == nil {
		content = make(map[string]interface{})
		response["content"] = content
	}
	content["application/json"] = map[string]interface{}{"schema": map[string]string{"$ref": schemaRef}}
}

func FilterMountedRoutes(spec map[string]interface{}, routes []OpenAPIRoute) {
	paths, _ := spec["paths"].(map[string]interface{})
	registered := make(map[string]OpenAPIRoute, len(routes))
	for _, route := range routes {
		if route.Internal {
			continue
		}
		registered[strings.ToLower(route.Method)+" "+route.Path] = route
	}
	for path, rawPathItem := range paths {
		pathItem, _ := rawPathItem.(map[string]interface{})
		for method := range pathItem {
			if _, ok := registered[strings.ToLower(method)+" "+path]; !ok {
				delete(pathItem, method)
			}
		}
		if len(pathItem) == 0 {
			delete(paths, path)
		}
	}
	for _, route := range routes {
		if route.MountedWhen == "" {
			continue
		}
		pathItem, ok := paths[route.Path].(map[string]interface{})
		if !ok {
			continue
		}
		method := strings.ToLower(route.Method)
		if !route.Mounted {
			delete(pathItem, method)
		} else if operation, ok := pathItem[method].(map[string]interface{}); ok {
			delete(operation, "x-mounted-when")
		}
		if len(pathItem) == 0 {
			delete(paths, route.Path)
		}
	}
}

func openAPITag(path string) string {
	first := strings.Trim(strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0], "{}")
	switch first {
	case "geoip":
		return "geoip"
	case "airports", "countries", "states", "stats", "settings":
		return "airports"
	default:
		return "server"
	}
}

func openAPIOperationID(method, path string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	upper := true
	for _, r := range path {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			if upper {
				r = unicode.ToUpper(r)
				upper = false
			}
			b.WriteRune(r)
		} else {
			upper = true
		}
	}
	return b.String()
}
