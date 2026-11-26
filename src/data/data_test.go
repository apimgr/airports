package data

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadFile_Existing(t *testing.T) {
	got, err := ReadFile("airports.json")
	if err != nil {
		t.Fatalf("ReadFile(airports.json): unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("ReadFile(airports.json) returned empty data")
	}
	if !json.Valid(got) {
		t.Error("ReadFile(airports.json) did not return valid JSON")
	}
}

func TestReadFile_Missing(t *testing.T) {
	_, err := ReadFile("does-not-exist.json")
	if err == nil {
		t.Fatal("ReadFile(does-not-exist.json): expected error, got nil")
	}
}

func TestReadFile_EmptyName(t *testing.T) {
	_, err := ReadFile("")
	if err == nil {
		t.Fatal("ReadFile(\"\"): expected error, got nil")
	}
}

func TestReadFile_PathTraversalRejected(t *testing.T) {
	// embed.FS rejects paths that escape the embedded root.
	_, err := ReadFile("../data.go")
	if err == nil {
		t.Fatal("ReadFile with a traversal path: expected error, got nil")
	}
}

func TestEmbeddedDataDirect(t *testing.T) {
	entries, err := EmbeddedData.ReadDir(".")
	if err != nil {
		t.Fatalf("EmbeddedData.ReadDir: unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("EmbeddedData.ReadDir(\".\") returned no entries")
	}

	found := false
	for _, e := range entries {
		if e.Name() == "airports.json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("airports.json not present in EmbeddedData")
	}
}

func TestAirportDatasetHasCurrentProvenance(t *testing.T) {
	type record struct {
		Type   string `json:"type"`
		TZ     string `json:"tz"`
		Source string `json:"source"`
	}
	var records map[string]record
	if err := json.Unmarshal(AirportsJSON, &records); err != nil {
		t.Fatalf("decode embedded airport data: %v", err)
	}
	if len(records) != 48071 {
		t.Fatalf("embedded airport count = %d, want 48071 for the pinned snapshot", len(records))
	}
	for ident, airport := range records {
		if airport.Type != "small_airport" && airport.Type != "medium_airport" && airport.Type != "large_airport" {
			t.Errorf("%s has excluded airport type %q", ident, airport.Type)
		}
		if airport.Source != "OurAirports" && airport.Source != "OurAirports; mwgg/Airports (timezone)" {
			t.Errorf("%s has invalid provenance source %q", ident, airport.Source)
		}
		if airport.TZ == "" && strings.Contains(airport.Source, "mwgg/Airports") {
			t.Errorf("%s names a timezone source but has no timezone value", ident)
		}
		if airport.TZ != "" && !strings.Contains(airport.Source, "mwgg/Airports") {
			t.Errorf("%s has a timezone without naming its source", ident)
		}
	}
}
