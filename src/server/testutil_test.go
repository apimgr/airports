package server

import (
	"testing"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/config"
)

// fixtureJSON is a minimal inline airport dataset used to build a real
// *airports.Service without touching the network or the filesystem.
const fixtureJSON = `{
	"KJFK": {"icao":"KJFK","iata":"JFK","name":"John F Kennedy Intl","city":"New York","state":"NY","country":"US","elevation":13,"lat":40.6398,"lon":-73.7789,"tz":"America/New_York"},
	"KLAX": {"icao":"KLAX","iata":"LAX","name":"Los Angeles Intl","city":"Los Angeles","state":"CA","country":"US","elevation":125,"lat":33.9425,"lon":-118.4081,"tz":"America/Los_Angeles"},
	"EGLL": {"icao":"EGLL","iata":"LHR","name":"London Heathrow","city":"London","state":"","country":"GB","elevation":83,"lat":51.4700,"lon":-0.4543,"tz":"Europe/London"}
}`

// newTestServer builds a fully-wired *Server backed by the fixture dataset
// and the library default config, with a nil GeoIP service (safe: GeoIP
// methods are nil-receiver-safe and return a typed error instead of
// panicking).
func newTestServer(t *testing.T) *Server {
	t.Helper()

	svc, err := airports.NewService([]byte(fixtureJSON))
	if err != nil {
		t.Fatalf("airports.NewService: unexpected error: %v", err)
	}

	cfg := config.DefaultConfig()

	// db and tor are nil in tests — every healthz check function treats
	// that as a hard "error"/"disabled" state rather than panicking.
	// dataDir is a real writable temp dir so checkDisk() can succeed.
	return New(svc, nil, cfg, nil, nil, t.TempDir(), t.TempDir(), nil)
}
