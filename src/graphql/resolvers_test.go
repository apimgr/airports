package graphql

import "testing"

// TestExtractStringArg covers the minimalist string-argument parser used
// to pull `code: "XXXX"` out of a raw GraphQL query string.
func TestExtractStringArg(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string
		arg   string
		want  string
	}{
		{
			name:  "simple match",
			query: `{ airport(code: "KJFK") { icao } }`,
			field: "airport",
			arg:   "code",
			want:  "KJFK",
		},
		{
			name:  "empty string value",
			query: `{ airport(code: "") { icao } }`,
			field: "airport",
			arg:   "code",
			want:  "",
		},
		{
			name:  "wrong field name",
			query: `{ nearby(code: "KJFK") { icao } }`,
			field: "airport",
			arg:   "code",
			want:  "",
		},
		{
			name:  "wrong arg name",
			query: `{ airport(id: "KJFK") { icao } }`,
			field: "airport",
			arg:   "code",
			want:  "",
		},
		{
			name:  "empty query",
			query: "",
			field: "airport",
			arg:   "code",
			want:  "",
		},
		{
			name:  "multiple args order does not matter",
			query: `{ airport(foo: 1, code: "KLAX", bar: 2) { icao } }`,
			field: "airport",
			arg:   "code",
			want:  "KLAX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStringArg(tt.query, tt.field, tt.arg)
			if got != tt.want {
				t.Errorf("extractStringArg(%q, %q, %q) = %q, want %q", tt.query, tt.field, tt.arg, got, tt.want)
			}
		})
	}
}

// TestExtractFloatArg covers the numeric-argument parser, including
// negative numbers, decimals, and the not-found (0, false) case.
func TestExtractFloatArg(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		field    string
		arg      string
		wantVal  float64
		wantFlag bool
	}{
		{
			name:     "positive decimal",
			query:    `{ nearby(lat: 40.64, lon: -73.78) { icao } }`,
			field:    "nearby",
			arg:      "lat",
			wantVal:  40.64,
			wantFlag: true,
		},
		{
			name:     "negative decimal",
			query:    `{ nearby(lat: 40.64, lon: -73.78) { icao } }`,
			field:    "nearby",
			arg:      "lon",
			wantVal:  -73.78,
			wantFlag: true,
		},
		{
			name:     "integer value",
			query:    `{ nearby(lat: 40, lon: -73) { icao } }`,
			field:    "nearby",
			arg:      "lat",
			wantVal:  40,
			wantFlag: true,
		},
		{
			name:     "missing arg",
			query:    `{ nearby(lat: 40.64) { icao } }`,
			field:    "nearby",
			arg:      "lon",
			wantVal:  0,
			wantFlag: false,
		},
		{
			name:     "zero value still matches",
			query:    `{ nearby(lat: 0, lon: 0) { icao } }`,
			field:    "nearby",
			arg:      "lat",
			wantVal:  0,
			wantFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotFlag := extractFloatArg(tt.query, tt.field, tt.arg)
			if gotFlag != tt.wantFlag {
				t.Fatalf("extractFloatArg(...) flag = %v, want %v", gotFlag, tt.wantFlag)
			}
			if gotFlag && gotVal != tt.wantVal {
				t.Errorf("extractFloatArg(...) val = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

// TestGraphQLArgPattern exercises the regex builder directly for the
// boundary case of an arg name that also happens to be a regex metachar
// substring, ensuring QuoteMeta is actually applied.
func TestGraphQLArgPattern(t *testing.T) {
	re := graphQLArgPattern("air.port", "co.de", `"([^"]*)"`)
	if !re.MatchString(`air.port(co.de: "X")`) {
		t.Error("expected literal-dot field/arg names to match literally")
	}
	if re.MatchString(`airXport(coYde: "X")`) {
		t.Error("expected dots to be treated literally, not as regex wildcards")
	}
}
