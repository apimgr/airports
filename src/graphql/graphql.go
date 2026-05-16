// Package graphql serves the GraphQL endpoint and GraphiQL UI.
//
// Per AI.md PART 14 ("Swagger & GraphQL Sync") this package is the
// canonical home for the GraphQL handler, schema, resolvers, and UI
// theming. All UI assets are embedded via Go's embed.FS so the binary
// is self-contained — no CDN fetches at runtime.
package graphql

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/apimgr/airports/src/airports"
)

//go:embed assets/*
var assetsFS embed.FS

// AirportSource is the minimal surface the GraphQL resolvers need from
// the airport service. Defining it as an interface keeps the graphql
// package free of any cyclic dependency on the server package.
type AirportSource interface {
	GetByCode(code string) (*airports.Airport, error)
	GetNearbyWithDistance(lat, lon, radiusKm float64, limit int, units string) []airports.AirportWithDistance
}

// AssetsHandler serves the vendored GraphiQL + React UMD bundles under
// the supplied URL prefix (e.g. "/server/docs/graphql/assets/").
func AssetsHandler(prefix string) http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		log.Printf("graphql: failed to sub assets fs: %v", err)
		return http.NotFoundHandler()
	}
	return http.StripPrefix(prefix, http.FileServer(http.FS(sub)))
}

// UIHandler serves the GraphiQL page that loads the embedded assets and
// points at the GraphQL endpoint at endpointPath.
func UIHandler(endpointPath, assetsPrefix string) http.HandlerFunc {
	page := renderGraphiQLHTML(endpointPath, assetsPrefix)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	}
}

// request is the standard GraphQL-over-HTTP request shape
// (https://graphql.org/learn/serving-over-http/#post-request).
type request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// response is the standard GraphQL-over-HTTP response shape.
type response struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []errorMessage `json:"errors,omitempty"`
}

type errorMessage struct {
	Message string `json:"message"`
}

// QueryHandler returns an http.HandlerFunc that services POST queries
// against the supplied airport data source.
//
// This is a minimal hand-rolled resolver covering the two documented
// queries (`airport(code:)` and `nearby(lat:, lon:, radius:)`) so the
// endpoint is functional without pulling in a heavy GraphQL runtime.
// Anything more elaborate returns a structured GraphQL error.
func QueryHandler(src AirportSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveQuery(w, r, src)
	}
}
