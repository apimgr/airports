package swagger

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/apimgr/airports/src/common/i18n"
)

//go:embed openapi.json
var embeddedOpenAPI []byte

func BuildEmbeddedSpecWithRoutes(version, apiVersion, lang string, routes []OpenAPIRoute, baseURL ...string) map[string]interface{} {
	spec := BuildSpec(version, apiVersion, lang, baseURL...)
	var generated map[string]interface{}
	if err := json.Unmarshal(embeddedOpenAPI, &generated); err != nil {
		return BuildSpecWithRoutes(version, apiVersion, lang, routes, baseURL...)
	}
	paths, _ := generated["paths"].(map[string]interface{})
	if paths == nil {
		paths = make(map[string]interface{})
	}
	for path, baseItem := range spec["paths"].(map[string]interface{}) {
		baseMethods, _ := baseItem.(map[string]interface{})
		pathItem, _ := paths[path].(map[string]interface{})
		if pathItem == nil {
			pathItem = make(map[string]interface{})
			paths[path] = pathItem
		}
		for method, operation := range baseMethods {
			pathItem[method] = operation
		}
	}
	spec["paths"] = paths
	if components, ok := generated["components"].(map[string]interface{}); ok {
		spec["components"] = components
	}
	for _, route := range routes {
		method := strings.ToLower(route.Method)
		pathItem, _ := paths[route.Path].(map[string]interface{})
		operation, _ := pathItem[method].(map[string]interface{})
		if operation == nil {
			continue
		}
		if route.Auth {
			operation["security"] = []map[string][]string{{"BearerAuth": {}}}
		}
		if route.MountedWhen != "" {
			operation["x-mounted-when"] = route.MountedWhen
		}
		if route.Summary != "" {
			if summary, _ := operation["summary"].(string); summary == "" || summary == method+" "+route.Path {
				operation["summary"] = route.Summary
				operation["description"] = i18n.TranslateText(lang, "swagger.route_description")
			}
		}
	}
	return spec
}
