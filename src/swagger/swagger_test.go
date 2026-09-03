package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAssetsHandler covers the embedded Swagger UI dist file server,
// including prefix stripping and the 404 case for an unknown asset.
func TestAssetsHandler(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		path       string
		wantStatus int
	}{
		{"known asset", "/assets/", "/assets/swagger-ui.css", http.StatusOK},
		{"unknown asset", "/assets/", "/assets/nope.js", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := AssetsHandler(tt.prefix)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// TestHandler verifies the Swagger UI HTML page renders with the correct
// content type and embeds the supplied spec path / assets prefix.
func TestHandler(t *testing.T) {
	h := Handler("/api/swagger", "/server/docs/swagger/assets/")
	req := httptest.NewRequest(http.MethodGet, "/server/docs/swagger", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api/swagger") {
		t.Error("body missing spec path")
	}
	if !strings.Contains(body, "/server/docs/swagger/assets/swagger-ui.css") {
		t.Error("body missing assets prefix in stylesheet link")
	}
}

// TestSpecHandler covers the OpenAPI JSON endpoint: content type, valid
// JSON body, version substitution, and the empty-version fallback to "dev".
func TestSpecHandler(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		wantVersion string
	}{
		{"explicit version", "1.2.3", "1.2.3"},
		{"empty version falls back to dev", "", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SpecHandler(tt.version, "v1")
			req := httptest.NewRequest(http.MethodGet, "/api/swagger.json", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var spec map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
				t.Fatalf("body is not valid JSON: %v", err)
			}
			info, ok := spec["info"].(map[string]interface{})
			if !ok {
				t.Fatal("spec missing info object")
			}
			if info["version"] != tt.wantVersion {
				t.Errorf("info.version = %v, want %v", info["version"], tt.wantVersion)
			}
			if !strings.HasSuffix(rec.Body.String(), "\n") {
				t.Error("response body does not end with a trailing newline")
			}
		})
	}
}

// TestSpecHandler_Idempotent verifies repeated calls to the same handler
// return identical bytes — the spec is built once and reused per request.
func TestSpecHandler_Idempotent(t *testing.T) {
	h := SpecHandler("9.9.9", "v1")

	req1 := httptest.NewRequest(http.MethodGet, "/api/swagger.json", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/swagger.json", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec1.Body.String() != rec2.Body.String() {
		t.Error("SpecHandler produced different bodies for identical requests")
	}
}
