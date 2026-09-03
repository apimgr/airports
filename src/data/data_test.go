package data

import (
	"encoding/json"
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
