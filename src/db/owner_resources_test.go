package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestOwnerResourceCountAndDeletion(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	for _, statement := range []string{
		`CREATE TABLE owners (id TEXT PRIMARY KEY)`,
		`CREATE TABLE api_tokens (id INTEGER PRIMARY KEY, owner_id TEXT)`,
		`CREATE TABLE saved_airports (id INTEGER PRIMARY KEY, owner_id TEXT NOT NULL)`,
		`INSERT INTO owners(id) VALUES ('owner-a'), ('owner-b')`,
		`INSERT INTO saved_airports(owner_id) VALUES ('owner-a'), ('owner-a'), ('owner-b')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	count, err := CountOwnerResources(database, "owner-a")
	if err != nil || count != 2 {
		t.Fatalf("CountOwnerResources = (%d, %v), want (2, nil)", count, err)
	}
	if err := Transaction(context.Background(), database, func(tx *TransactionHandle) error {
		deleted, err := DeleteOwnerResources(tx, "owner-a")
		if err != nil {
			return err
		}
		if deleted != 2 {
			t.Fatalf("DeleteOwnerResources deleted %d rows, want 2", deleted)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	count, err = CountOwnerResources(database, "owner-a")
	if err != nil || count != 0 {
		t.Fatalf("CountOwnerResources after deletion = (%d, %v), want (0, nil)", count, err)
	}
	count, err = CountOwnerResources(database, "owner-b")
	if err != nil || count != 1 {
		t.Fatalf("CountOwnerResources for other owner = (%d, %v), want (1, nil)", count, err)
	}
}
