package airports

import (
	"testing"
)

// Covers LoadAirports error path and BuildIndexes behavior with airports
// missing optional fields (IATA/City/Country/State) - these must not be
// indexed and must not panic.
func TestLoadAirportsInvalidJSON(t *testing.T) {
	_, err := LoadAirports([]byte("not valid json"))
	if err == nil {
		t.Fatal("LoadAirports with invalid JSON should return an error")
	}
}

func TestBuildIndexesSkipsEmptyFields(t *testing.T) {
	raw := []byte(`{
		"KXXX": {"icao":"KXXX","iata":"","name":"No Extras Field","city":"","state":"","country":"","elevation":0,"lat":1,"lon":2,"tz":"UTC"}
	}`)
	data, err := LoadAirports(raw)
	if err != nil {
		t.Fatalf("LoadAirports: %v", err)
	}
	indexes := BuildIndexes(data)

	if _, ok := indexes.ByICAO["KXXX"]; !ok {
		t.Error("expected KXXX to be indexed by ICAO")
	}
	if len(indexes.ByIATA) != 0 {
		t.Errorf("expected no IATA index entries, got %d", len(indexes.ByIATA))
	}
	if len(indexes.ByCity) != 0 {
		t.Errorf("expected no city index entries, got %d", len(indexes.ByCity))
	}
	if len(indexes.ByCountry) != 0 {
		t.Errorf("expected no country index entries, got %d", len(indexes.ByCountry))
	}
	if len(indexes.ByState) != 0 {
		t.Errorf("expected no state index entries, got %d", len(indexes.ByState))
	}
}

func TestGetByCodeNotFound(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.GetByCode("ZZZZ"); err == nil {
		t.Error("expected error for unknown code, got nil")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if results := svc.Search("", 10, 0); results != nil {
		t.Errorf("expected nil for empty query, got %d results", len(results))
	}
	if results := svc.Search("   ", 10, 0); results != nil {
		t.Errorf("expected nil for whitespace-only query, got %d results", len(results))
	}
}

func TestSearchPaginationEdges(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// offset beyond total results must return an empty (non-nil) slice.
	results := svc.Search("Airport", 10, 1000)
	if len(results) != 0 {
		t.Errorf("expected 0 results for far offset, got %d", len(results))
	}

	// limit larger than the remaining results must clamp, not panic/overrun.
	all := svc.Search("Airport", 1000, 0)
	if len(all) < 2 {
		t.Fatalf("expected at least 2 airports matching Airport, got %d", len(all))
	}
	partial := svc.Search("Airport", 1000, len(all)-1)
	if len(partial) != 1 {
		t.Errorf("expected 1 result when offset = len-1, got %d", len(partial))
	}
}

func TestGetByCity(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	results := svc.GetByCity("New York")
	if len(results) == 0 {
		t.Error("expected results for New York")
	}
	// Case-insensitivity.
	if results2 := svc.GetByCity("NEW YORK"); len(results2) != len(results) {
		t.Errorf("case-insensitive lookup mismatch: %d vs %d", len(results2), len(results))
	}
	if results := svc.GetByCity("Nowhere"); len(results) != 0 {
		t.Errorf("expected no results for unknown city, got %d", len(results))
	}
}

func TestGetByCountry(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	results := svc.GetByCountry("US")
	if len(results) == 0 {
		t.Error("expected results for US")
	}
	if results := svc.GetByCountry("us"); len(results) == 0 {
		t.Error("expected lowercase country lookup to be normalized to uppercase")
	}
	if results := svc.GetByCountry("ZZ"); len(results) != 0 {
		t.Errorf("expected no results for unknown country, got %d", len(results))
	}
}

func TestGetByState(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	results := svc.GetByState("NY")
	if len(results) == 0 {
		t.Error("expected results for NY state")
	}
	if results := svc.GetByState("zz"); len(results) != 0 {
		t.Errorf("expected no results for unknown state, got %d", len(results))
	}
}

func TestGetNearbyWithDistance(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	metric := svc.GetNearbyWithDistance(40.6398, -73.7789, 50, 10, UnitMetric)
	if len(metric) == 0 {
		t.Fatal("expected metric results")
	}
	if metric[0].DistanceUnit != "km" {
		t.Errorf("expected unit km, got %s", metric[0].DistanceUnit)
	}

	imperial := svc.GetNearbyWithDistance(40.6398, -73.7789, 50, 10, UnitImperial)
	if len(imperial) == 0 {
		t.Fatal("expected imperial results")
	}
	if imperial[0].DistanceUnit != "mi" {
		t.Errorf("expected unit mi, got %s", imperial[0].DistanceUnit)
	}

	// Results should be sorted ascending by distance.
	for i := 1; i < len(metric); i++ {
		if metric[i].Distance < metric[i-1].Distance {
			t.Errorf("results not sorted ascending by distance at index %d", i)
		}
	}
}

func TestGetAll(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	all := svc.GetAll(1000, 0)
	if len(all) == 0 {
		t.Fatal("expected airports from GetAll")
	}
	for i := 1; i < len(all); i++ {
		if all[i].ICAO < all[i-1].ICAO {
			t.Errorf("GetAll not sorted ascending by ICAO at index %d", i)
		}
	}

	if results := svc.GetAll(1000, len(all)+10); len(results) != 0 {
		t.Errorf("expected 0 results for offset beyond total, got %d", len(results))
	}

	if results := svc.GetAll(1, 0); len(results) != 1 {
		t.Errorf("expected exactly 1 result with limit=1, got %d", len(results))
	}
}

func TestGetCountries(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	countries := svc.GetCountries()
	if len(countries) == 0 {
		t.Fatal("expected at least one country")
	}
	count, ok := countries["US"]
	if !ok || count == 0 {
		t.Errorf("expected US country with non-zero count, got %v", countries)
	}
}

func TestGetStatesInCountry(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	states := svc.GetStatesInCountry("US")
	if len(states) == 0 {
		t.Fatal("expected at least one state for US")
	}

	// Unknown country must return an empty, non-nil map.
	unknown := svc.GetStatesInCountry("ZZ")
	if unknown == nil {
		t.Error("expected non-nil empty map for unknown country")
	}
	if len(unknown) != 0 {
		t.Errorf("expected empty map for unknown country, got %v", unknown)
	}

	// Lowercase input must be normalized.
	if lower := svc.GetStatesInCountry("us"); len(lower) != len(states) {
		t.Errorf("case-insensitive country lookup mismatch: %d vs %d", len(lower), len(states))
	}
}

func TestGetRawData(t *testing.T) {
	svc, err := NewService(testAirportsJSON)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	raw := svc.GetRawData()
	if len(raw) == 0 {
		t.Fatal("expected non-empty raw data")
	}
	if _, ok := raw["KJFK"]; !ok {
		t.Error("expected KJFK in raw data")
	}
}

func TestConvertDistance(t *testing.T) {
	tests := []struct {
		units    string
		wantUnit string
		wantKm   bool
	}{
		{UnitMetric, "km", true},
		{UnitImperial, "mi", false},
		{"", "mi", false},
		{"bogus", "mi", false},
	}
	for _, tt := range tests {
		t.Run(tt.units, func(t *testing.T) {
			dist, unit := ConvertDistance(100, tt.units)
			if unit != tt.wantUnit {
				t.Errorf("ConvertDistance(100, %q) unit = %q, want %q", tt.units, unit, tt.wantUnit)
			}
			if tt.wantKm {
				if dist != 100 {
					t.Errorf("ConvertDistance metric = %f, want 100", dist)
				}
			} else {
				// Compare with a small epsilon rather than exact equality:
				// the compiler constant-folds "100 * KmToMiles" at higher
				// precision than the runtime float64 multiplication inside
				// ConvertDistance, so the two can differ by a single ULP.
				want := 100 * KmToMiles
				const epsilon = 1e-9
				if diff := dist - want; diff > epsilon || diff < -epsilon {
					t.Errorf("ConvertDistance imperial = %f, want %f", dist, want)
				}
			}
		})
	}
}

func TestConvertElevation(t *testing.T) {
	tests := []struct {
		units    string
		wantUnit string
	}{
		{UnitMetric, "m"},
		{UnitImperial, "ft"},
		{"", "ft"},
	}
	for _, tt := range tests {
		t.Run(tt.units, func(t *testing.T) {
			elev, unit := ConvertElevation(1000, tt.units)
			if unit != tt.wantUnit {
				t.Errorf("ConvertElevation(1000, %q) unit = %q, want %q", tt.units, unit, tt.wantUnit)
			}
			if tt.units == UnitMetric {
				want := 1000 * FeetToMeters
				if elev != want {
					t.Errorf("ConvertElevation metric = %f, want %f", elev, want)
				}
			} else if elev != 1000 {
				t.Errorf("ConvertElevation imperial = %f, want 1000", elev)
			}
		})
	}
}

func TestParseUnits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"metric", UnitMetric},
		{"METRIC", UnitMetric},
		{"m", UnitMetric},
		{"km", UnitMetric},
		{"kilometers", UnitMetric},
		{"imperial", UnitImperial},
		{"miles", UnitImperial},
		{"", UnitImperial},
		{"bogus", UnitImperial},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseUnits(tt.input); got != tt.want {
				t.Errorf("ParseUnits(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
