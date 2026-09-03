package swagger

import "testing"

// TestBuildSpec_TopLevelStructure verifies the required OpenAPI 3.0
// top-level fields are present with the expected fixed values.
func TestBuildSpec_TopLevelStructure(t *testing.T) {
	spec := BuildSpec("1.0.0", "v1", "en")

	if spec["openapi"] != "3.0.0" {
		t.Errorf("openapi = %v, want 3.0.0", spec["openapi"])
	}

	info, ok := spec["info"].(map[string]interface{})
	if !ok {
		t.Fatal("spec missing info object")
	}
	if info["title"] != "Airports API" {
		t.Errorf("info.title = %v, want Airports API", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("info.version = %v, want 1.0.0", info["version"])
	}

	license, ok := info["license"].(map[string]string)
	if !ok {
		t.Fatal("info missing license object")
	}
	if license["name"] != "MIT" {
		t.Errorf("license.name = %v, want MIT", license["name"])
	}

	if _, ok := spec["paths"].(map[string]interface{}); !ok {
		t.Fatal("spec missing paths object")
	}
	if _, ok := spec["components"].(map[string]interface{}); !ok {
		t.Fatal("spec missing components object")
	}
}

// TestBuildSpec_Version covers the version-injection behavior, including
// the empty-string boundary that falls back to "dev".
func TestBuildSpec_Version(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"explicit version", "2.5.0", "2.5.0"},
		{"empty version defaults to dev", "", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := BuildSpec(tt.version, "v1", "en")
			info := spec["info"].(map[string]interface{})
			if info["version"] != tt.want {
				t.Errorf("info.version = %v, want %v", info["version"], tt.want)
			}
		})
	}
}

// TestBuildSpec_RequiredPaths ensures every documented endpoint stays
// present in the hand-maintained spec — a regression here means a route
// was added/removed in the server without updating annotations.go.
func TestBuildSpec_RequiredPaths(t *testing.T) {
	spec := BuildSpec("dev", "v1", "en")
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec missing paths object")
	}

	required := []string{
		"/airports",
		"/airports/{ident}",
		"/airports/search",
		"/airports/nearby",
		"/airports/within",
		"/airports/autocomplete",
		"/countries",
		"/geoip",
		"/geoip/{ip}",
		"/stats",
		"/server/healthz",
		"/server/about",
	}
	for _, p := range required {
		if _, ok := paths[p]; !ok {
			t.Errorf("paths missing required entry: %s", p)
		}
	}

	if len(paths) != len(required) {
		t.Errorf("paths has %d entries, want %d (spec may have drifted, update the required list too)", len(paths), len(required))
	}
}

// TestBuildSpec_PathShape spot-checks that a representative path entry has
// the expected method, tags, and response structure — catching accidental
// structural breakage (e.g. tags becoming a string instead of []string).
func TestBuildSpec_PathShape(t *testing.T) {
	spec := BuildSpec("dev", "v1", "en")
	paths := spec["paths"].(map[string]interface{})

	entry, ok := paths["/airports/{ident}"].(map[string]interface{})
	if !ok {
		t.Fatal("/airports/{ident} entry missing or wrong type")
	}
	get, ok := entry["get"].(map[string]interface{})
	if !ok {
		t.Fatal("/airports/{ident} missing GET method")
	}
	if get["operationId"] != "getAirport" {
		t.Errorf("operationId = %v, want getAirport", get["operationId"])
	}

	tags, ok := get["tags"].([]string)
	if !ok || len(tags) == 0 {
		t.Fatal("tags missing or wrong type")
	}
	if tags[0] != "airports" {
		t.Errorf("tags[0] = %v, want airports", tags[0])
	}

	params, ok := get["parameters"].([]map[string]interface{})
	if !ok || len(params) == 0 {
		t.Fatal("parameters missing or wrong type")
	}

	responses, ok := get["responses"].(map[string]interface{})
	if !ok {
		t.Fatal("responses missing or wrong type")
	}
	if _, ok := responses["200"]; !ok {
		t.Error("responses missing 200 entry")
	}
	if _, ok := responses["404"]; !ok {
		t.Error("responses missing 404 entry for a lookup-by-identifier endpoint")
	}
}

// TestBuildSpec_ServersReflectsAPIVersion ensures the OpenAPI "servers" URL
// is built from the apiVersion parameter, not a hardcoded "v1" (PART 14,
// "NEVER hardcode v1").
func TestBuildSpec_ServersReflectsAPIVersion(t *testing.T) {
	spec := BuildSpec("dev", "v2", "en")
	servers, ok := spec["servers"].([]map[string]string)
	if !ok || len(servers) == 0 {
		t.Fatal("servers missing or wrong type")
	}
	if servers[0]["url"] != "/api/v2" {
		t.Errorf("servers[0].url = %v, want /api/v2", servers[0]["url"])
	}

	specDefault := BuildSpec("dev", "", "en")
	serversDefault := specDefault["servers"].([]map[string]string)
	if serversDefault[0]["url"] != "/api/v1" {
		t.Errorf("empty apiVersion should default to v1, got %v", serversDefault[0]["url"])
	}
}

// TestBuildSpec_Idempotent ensures repeated calls with the same version
// produce structurally identical output — BuildSpec must be pure.
func TestBuildSpec_Idempotent(t *testing.T) {
	a := BuildSpec("3.0.0", "v1", "en")
	b := BuildSpec("3.0.0", "v1", "en")

	aInfo := a["info"].(map[string]interface{})
	bInfo := b["info"].(map[string]interface{})
	if aInfo["version"] != bInfo["version"] {
		t.Error("BuildSpec is not deterministic for identical input")
	}

	aPaths := a["paths"].(map[string]interface{})
	bPaths := b["paths"].(map[string]interface{})
	if len(aPaths) != len(bPaths) {
		t.Errorf("path count differs between calls: %d vs %d", len(aPaths), len(bPaths))
	}
}
