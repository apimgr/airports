package graphql

import (
	"strings"
	"testing"
)

// TestSchemaContents ensures the hand-maintained Schema string stays in
// sync with the resolvers it documents — a regression here means someone
// edited resolvers.go without updating schema.go (or vice versa).
func TestSchemaContents(t *testing.T) {
	if Schema == "" {
		t.Fatal("Schema is empty")
	}

	required := []string{
		"type Airport",
		"type Coordinates",
		"type AirportWithDistance",
		"type NameCount",
		"type Stats",
		"type GeoLocation",
		"type Query",
		"airport(code: String!): Airport",
		"nearby(lat: Float!, lon: Float!, radius: Float = 50): [AirportWithDistance!]!",
		"airports(limit: Int = 250, page: Int = 1): [Airport!]!",
		"search(q: String!, limit: Int = 50, page: Int = 1): [Airport!]!",
		"within(latMin: Float!, latMax: Float!, lonMin: Float!, lonMax: Float!): [Airport!]!",
		"autocomplete(q: String!, limit: Int = 10): [Airport!]!",
		"countries: [NameCount!]!",
		"states(country: String!): [NameCount!]!",
		"stats: Stats",
		"geoip(ip: String): GeoLocation",
	}
	for _, want := range required {
		if !strings.Contains(Schema, want) {
			t.Errorf("Schema missing expected fragment: %q", want)
		}
	}
}
