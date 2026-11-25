package data

import _ "embed"

// AirportsJSON is the embedded airport catalogue used by the server.
//go:embed airports.json
var AirportsJSON []byte
