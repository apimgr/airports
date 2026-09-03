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
	"github.com/apimgr/airports/src/geoip"
)

//go:embed assets/*
var assetsFS embed.FS

// AirportSource is the minimal surface the GraphQL resolvers need from
// the airport service. Defining it as an interface keeps the graphql
// package free of any cyclic dependency on the server package.
//
// Per AI.md PART 14 ("Swagger & GraphQL Sync"), GraphQL must expose the
// same functionality as the REST API, so this mirrors every read
// capability REST exposes via src/server/handlers.go.
type AirportSource interface {
	GetByCode(code string) (*airports.Airport, error)
	GetNearbyWithDistance(lat, lon, radiusKm float64, limit int, units string) []airports.AirportWithDistance
	GetAll(limit, offset int) []*airports.Airport
	Search(query string, limit, offset int) []*airports.Airport
	GetInBoundingBox(minLat, maxLat, minLon, maxLon float64) []*airports.Airport
	GetCountries() map[string]int
	GetStatesInCountry(country string) map[string]int
	Stats() map[string]interface{}
}

// GeoIPSource is the minimal surface the GraphQL `geoip` resolver needs
// from the GeoIP service, mirroring handleGeoIPLookup/handleGeoIPLookupIP.
type GeoIPSource interface {
	LookupString(ipStr string) (*geoip.GeoLocation, error)
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
// against the supplied airport data source and (optional) GeoIP source.
//
// This is a minimal hand-rolled resolver — not a full GraphQL runtime —
// covering the documented queries in schema.go: `airport`, `nearby`,
// `airports`, `search`, `within`, `autocomplete`, `countries`, `states`,
// `stats`, and `geoip`. Anything else returns a structured GraphQL error.
// geo may be nil (e.g. GeoIP disabled) — the `geoip` query then reports a
// lookup error instead of panicking.
func QueryHandler(src AirportSource, geo GeoIPSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveQuery(w, r, src, geo)
	}
}
