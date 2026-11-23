package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

func TestRunMaintenanceTokenListAndRevoke(t *testing.T) {
	if !service.IsElevated() {
		t.Skip("maintenance token command requires administrator authorization")
	}
	root := newMainTestDir(t)
	configDir := filepath.Join(root, "config")
	logsDir := filepath.Join(root, "logs")
	databasePath := filepath.Join(root, "server.db")
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Database.URL = databasePath
	if err := config.SaveApplicationConfig(filepath.Join(configDir, "server.yml"), cfg); err != nil {
		t.Fatalf("save maintenance test config: %v", err)
	}
	database := openRecoveryTestDB(t, databasePath, false)
	if _, err := server.ListOwnerTokens(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO owners(id) VALUES('owner-test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO api_tokens(owner_id, kind, token_hash, token_prefix, token_enc)
		VALUES('owner-test', 'primary', 'hash-test', 'own_12345678', 'encrypted-token')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	list := captureStdout(t, func() {
		if err := runMaintenanceToken(configDir, logsDir, "list"); err != nil {
			t.Errorf("runMaintenanceToken(list): %v", err)
		}
	})
	if !strings.Contains(list, "own_12345678") || !strings.Contains(list, "owner-test") {
		t.Fatalf("token list omitted metadata: %q", list)
	}
	if err := runMaintenanceToken(configDir, logsDir, "revoke own_12345678 --yes"); err != nil {
		t.Fatalf("runMaintenanceToken(revoke): %v", err)
	}
	if err := runMaintenanceToken(configDir, logsDir, "revoke own_missing"); err == nil {
		t.Fatal("revoke accepted an unknown token prefix")
	}
}
