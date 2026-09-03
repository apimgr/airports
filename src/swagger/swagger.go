// Package swagger serves the OpenAPI specification and Swagger UI.
//
// Per AI.md PART 14 ("Swagger & GraphQL Sync") this package is the
// canonical home for the Swagger handler, UI theming, and OpenAPI
// annotations. All assets are embedded via Go's embed.FS so the
// resulting binary is self-contained — no CDN fetches at runtime.
package swagger

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	"github.com/apimgr/airports/src/common/i18n"
)

//go:embed assets/*
var assetsFS embed.FS

// AssetsHandler serves the vendored Swagger UI dist files under the
// supplied URL prefix (e.g. "/server/docs/swagger/assets/").
func AssetsHandler(prefix string) http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		log.Printf("swagger: failed to sub assets fs: %v", err)
		return http.NotFoundHandler()
	}
	return http.StripPrefix(prefix, http.FileServer(http.FS(sub)))
}

// Handler serves the Swagger UI HTML page that loads the embedded assets
// and points at the OpenAPI spec at specPath.
func Handler(specPath, assetsPrefix string) http.HandlerFunc {
	page := renderSwaggerHTML(specPath, assetsPrefix)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	}
}

// SpecHandler serves the OpenAPI 3.0 JSON specification.
// version is the running binary version (injected by server.go via Version var).
// apiVersion is the configured API path segment (server.api_version, default "v1").
// The spec's translated description/summary text (PART 30) is precomputed
// once per supported language at handler-creation time and selected per
// request via the resolved request language — this avoids re-marshaling
// JSON on every request while still serving each visitor's language.
func SpecHandler(version, apiVersion string) http.HandlerFunc {
	bodies := make(map[string][]byte, len(i18n.SupportedLanguages))
	for _, lang := range i18n.SupportedLanguages {
		spec := BuildSpec(version, apiVersion, lang)
		body, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			body = []byte(`{"error":"failed to encode openapi spec"}`)
		}
		body = append(body, '\n')
		bodies[lang] = body
	}
	return func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.FromContext(r.Context())
		body, ok := bodies[lang]
		if !ok {
			body = bodies["en"]
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}
