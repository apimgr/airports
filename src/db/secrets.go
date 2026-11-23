package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const appSecretTimestamp = "2006-01-02T15:04:05Z07:00"

// initializeAppSecrets applies the app_secrets schema and creates missing
// generated secrets atomically without replacing values already in the DB.
func initializeAppSecrets(database Handle) error {
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS app_secrets (
		name TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`); err != nil {
		return err
	}

	columns := map[string]bool{}
	rows, err := database.Query(`PRAGMA table_info(app_secrets)`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var position int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&position, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !columns["name"] || !columns["value"] {
		return errors.New("db: app_secrets schema is missing required columns")
	}
	if !columns["updated_at"] {
		if _, err := database.Exec(`ALTER TABLE app_secrets ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := database.Exec(`CREATE TRIGGER IF NOT EXISTS app_secrets_updated_at_insert
		AFTER INSERT ON app_secrets WHEN NEW.updated_at = '' BEGIN
		UPDATE app_secrets SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE name = NEW.name;
		END`); err != nil {
		return err
	}
	if _, err := database.Exec(`CREATE TRIGGER IF NOT EXISTS app_secrets_updated_at_change
		AFTER UPDATE OF value ON app_secrets BEGIN
		UPDATE app_secrets SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE name = NEW.name;
		END`); err != nil {
		return err
	}

	return Transaction(context.Background(), database, func(tx *TransactionHandle) error {
		if _, err := tx.Exec(`UPDATE app_secrets SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE updated_at = '' OR updated_at IS NULL`); err != nil {
			return err
		}
		if err := migrateAppSecretValues(tx); err != nil {
			return err
		}
		for _, name := range []string{"installation_secret", "cookie_signing_key", "csrf_token_secret"} {
			value := make([]byte, 32)
			if _, err := rand.Read(value); err != nil {
				return fmt.Errorf("generate %s: %w", name, err)
			}
			encoded := base64.StdEncoding.EncodeToString(value)
			if _, err := tx.Exec(`INSERT INTO app_secrets(name, value, updated_at) VALUES(?, ?, ?) ON CONFLICT(name) DO NOTHING`, name, encoded, time.Now().UTC().Format(appSecretTimestamp)); err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureAppSecrets applies the app_secrets schema and generated-secret
// initialization to an already-open database handle.
func EnsureAppSecrets(database Handle) error {
	if database == nil {
		return errors.New("db: app_secrets database handle is nil")
	}
	return initializeAppSecrets(database)
}

func migrateAppSecretValues(tx *TransactionHandle) error {
	rows, err := tx.Query(`SELECT name, value, typeof(value) FROM app_secrets`)
	if err != nil {
		return err
	}
	type entry struct {
		name  string
		value []byte
		type_ string
	}
	var entries []entry
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.name, &item.value, &item.type_); err != nil {
			_ = rows.Close()
			return err
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range entries {
		if item.type_ != "blob" {
			continue
		}
		value := string(item.value)
		if !isAppSecretTimestamp(item.name) {
			value = base64.StdEncoding.EncodeToString(item.value)
		}
		if _, err := tx.Exec(`UPDATE app_secrets SET value = ? WHERE name = ?`, value, item.name); err != nil {
			return err
		}
	}
	return nil
}

// EncodeAppSecret stores secret bytes as base64 text; timestamp bookkeeping
// rows remain readable UTC ISO 8601 text.
func EncodeAppSecret(name string, value []byte) string {
	if isAppSecretTimestamp(name) {
		return string(value)
	}
	return base64.StdEncoding.EncodeToString(value)
}

// DecodeAppSecret reads base64 text while accepting legacy raw values during
// tests and while importing databases created by older releases.
func DecodeAppSecret(name string, value []byte) []byte {
	if isAppSecretTimestamp(name) {
		return value
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(value)))
	if err != nil {
		return value
	}
	return decoded
}

func isAppSecretTimestamp(name string) bool {
	return strings.HasSuffix(name, "_until") || strings.HasSuffix(name, "_at")
}
