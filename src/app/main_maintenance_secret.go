package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	paths "github.com/apimgr/airports/src/paths"
	"github.com/apimgr/airports/src/security/pgp"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
	"github.com/apimgr/airports/src/ssl"
)

const encryptionPreviousPrefix = "airports-encryption-key-previous:v1:"

func runMaintenanceSecret(configDir, logsDir, rawArgs string) error {
	args := strings.Fields(rawArgs)
	if len(args) != 2 || args[0] != "rotate" || args[1] != "installation_secret" && args[1] != "encryption_key" {
		return errors.New("usage: --maintenance secret rotate {installation_secret|encryption_key}")
	}
	defaultConfigDir, _, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	configDir = firstNonEmpty(configDir, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	logsDir = firstNonEmpty(logsDir, os.Getenv("LOG_DIR"), defaultLogsDir)
	cfg, err := config.LoadApplicationConfig(filepath.Join(configDir, "server.yml"))
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	if !service.IsElevated() {
		provided, err := promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
		if cfg.Server.Token == "" || len(provided) != len(cfg.Server.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.Server.Token)) != 1 {
			return errors.New("secret rotation denied: invalid operator token")
		}
	}
	if err := confirmPGPAction("ROTATE " + strings.ToUpper(args[1])); err != nil {
		return err
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() { _ = database.Close() }()
	if err := ensureAppSecretsSchema(database); err != nil {
		return err
	}
	if err := refuseActivePreviousSecret(database, args[1], time.Now().UTC()); err != nil {
		return err
	}
	manager := logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
	defer func() { _ = manager.Close() }()
	if args[1] == "installation_secret" {
		return rotateInstallationSecret(database, configDir, args[1], manager)
	}
	return rotateEncryptionKey(database, cfg, filepath.Join(configDir, "server.yml"), configDir, manager)
}

func ensureAppSecretsSchema(database db.Handle) error {
	return db.EnsureAppSecrets(database)
}

func refuseActivePreviousSecret(database db.Handle, name string, now time.Time) error {
	var until string
	err := database.QueryRow(`SELECT CAST(value AS TEXT) FROM app_secrets WHERE name = ?`, name+"_previous_until").Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return fmt.Errorf("invalid previous %s expiration value", name)
	}
	if now.Before(expires) {
		return fmt.Errorf("cannot rotate %s while its previous value remains in the grace period", name)
	}
	_, err = database.Exec(`DELETE FROM app_secrets WHERE name IN (?, ?)`, name+"_previous", name+"_previous_until")
	return err
}

func rotateInstallationSecret(database db.Handle, configDir, name string, logs *logging.LogManager) error {
	oldSecret, err := loadInstallationSecret(database)
	if err != nil {
		return err
	}
	newSecret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newSecret); err != nil {
		return err
	}
	until := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
		if _, err := tx.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, name+"_previous", db.EncodeAppSecret(name+"_previous", oldSecret)); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, name+"_previous_until", db.EncodeAppSecret(name+"_previous_until", []byte(until))); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, name, db.EncodeAppSecret(name, newSecret))
		return err
	}); err != nil {
		return err
	}
	if err := resumeInstallationSecretRotation(database, configDir); err != nil {
		return fmt.Errorf("secret changed; startup recovery will retry PGP private-key re-encryption: %w", err)
	}
	return logs.WriteAudit(logging.AuditCategorySecurity, "security.installation_secret_rotated grace_until="+until)
}

func rotateEncryptionKey(database db.Handle, cfg *config.ApplicationConfig, configPath, configDir string, logs *logging.LogManager) error {
	oldKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.Server.Security.EncryptionKey))
	if err != nil || len(oldKey) != 32 {
		return errors.New("configured encryption key is invalid")
	}
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return err
	}
	wrappedPrevious, err := wrapPreviousEncryptionKey(newKey, oldKey)
	if err != nil {
		return err
	}
	until := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
		if _, err := tx.Exec(`INSERT INTO app_secrets(name, value) VALUES('encryption_key_previous', ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, db.EncodeAppSecret("encryption_key_previous", wrappedPrevious)); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO app_secrets(name, value) VALUES('encryption_key_previous_until', ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, db.EncodeAppSecret("encryption_key_previous_until", []byte(until)))
		return err
	}); err != nil {
		return err
	}
	original := *cfg
	newEncoded := base64.StdEncoding.EncodeToString(newKey)
	if cfg.Server.SSL.LetsEncrypt.DNSCredentials != "" {
		credentials, err := ssl.DecryptDNSCredentials(base64.StdEncoding.EncodeToString(oldKey), cfg.Server.SSL.LetsEncrypt.DNSCredentials)
		if err != nil {
			return fmt.Errorf("decrypt DNS provider credentials before key rotation: %w", err)
		}
		cfg.Server.SSL.LetsEncrypt.DNSCredentials, err = ssl.EncryptDNSCredentials(newEncoded, credentials)
		if err != nil {
			return err
		}
	}
	cfg.Server.Security.EncryptionKey = newEncoded
	cfg.Server.Security.EncryptionKeyVersion++
	if err := config.SaveApplicationConfig(configPath, cfg); err != nil {
		*cfg = original
		return err
	}
	if err := reencryptEncryptionKeyRows(database, oldKey, newKey); err != nil {
		return fmt.Errorf("encryption key rotated; resume unfinished data re-encryption at startup: %w", err)
	}
	return logs.WriteAudit(logging.AuditCategorySecurity, "security.encryption_key_rotated grace_until="+until)
}

func wrapPreviousEncryptionKey(newKey, oldKey []byte) ([]byte, error) {
	ciphertext, err := encryptAESGCM(newKey, oldKey)
	if err != nil {
		return nil, err
	}
	return []byte(encryptionPreviousPrefix + base64.StdEncoding.EncodeToString(ciphertext)), nil
}

func unwrapPreviousEncryptionKey(newKey, encoded []byte) ([]byte, error) {
	value := strings.TrimSpace(string(encoded))
	if !strings.HasPrefix(value, encryptionPreviousPrefix) {
		return nil, errors.New("previous encryption key envelope has an unsupported format")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptionPreviousPrefix))
	if err != nil {
		return nil, err
	}
	return decryptAESGCM(newKey, ciphertext)
}

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append(nonce, aead.Seal(nil, nonce, plaintext, nil)...), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("encrypted value is truncated")
	}
	nonce := ciphertext[:aead.NonceSize()]
	return aead.Open(nil, nonce, ciphertext[aead.NonceSize():], nil)
}

func decryptWithKeyFallback(current, previous, value []byte, encoded bool) ([]byte, bool, error) {
	input := value
	if encoded {
		var err error
		input, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(value)))
		if err != nil {
			return nil, false, err
		}
	}
	plain, err := decryptAESGCM(current, input)
	if err == nil {
		return plain, false, nil
	}
	plain, previousErr := decryptAESGCM(previous, input)
	if previousErr != nil {
		return nil, false, errors.New("encrypted value cannot be decrypted with current or previous key")
	}
	return plain, true, nil
}

func reencryptEncryptionKeyRows(database db.Handle, oldKey, newKey []byte) error {
	if _, err := server.ListOwnerTokens(database); err != nil {
		return err
	}
	rows, err := database.Query(`SELECT token_hash, token_enc FROM api_tokens ORDER BY created_at, token_hash`)
	if err != nil {
		return err
	}
	type tokenCipher struct {
		hash  string
		value string
	}
	var tokens []tokenCipher
	for rows.Next() {
		var row tokenCipher
		if err := rows.Scan(&row.hash, &row.value); err != nil {
			_ = rows.Close()
			return err
		}
		tokens = append(tokens, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for start := 0; start < len(tokens); start += 500 {
		end := min(start+500, len(tokens))
		batch := tokens[start:end]
		if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
			for _, row := range batch {
				plain, usedPrevious, err := decryptWithKeyFallback(newKey, oldKey, []byte(row.value), true)
				if err != nil {
					return fmt.Errorf("security.decrypt_failed table=api_tokens")
				}
				if !usedPrevious {
					continue
				}
				ciphertext, err := encryptAESGCM(newKey, plain)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(`UPDATE api_tokens SET token_enc = ? WHERE token_hash = ?`, base64.StdEncoding.EncodeToString(ciphertext), row.hash); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	var reportTable int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='security_reports'`).Scan(&reportTable); err != nil {
		return err
	}
	if reportTable == 0 {
		return nil
	}
	type rowCipher struct {
		id, algorithm string
		body, contact []byte
	}
	rows, err = database.Query(`SELECT tracking_id, body_alg, body_enc, contact_enc FROM security_reports WHERE body_alg = 'aes-256-gcm' OR contact_enc IS NOT NULL ORDER BY tracking_id`)
	if err != nil {
		return err
	}
	var reports []rowCipher
	for rows.Next() {
		var row rowCipher
		if err := rows.Scan(&row.id, &row.algorithm, &row.body, &row.contact); err != nil {
			_ = rows.Close()
			return err
		}
		reports = append(reports, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for start := 0; start < len(reports); start += 500 {
		end := min(start+500, len(reports))
		batch := reports[start:end]
		if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
			for _, row := range batch {
				bodyCipher := row.body
				bodyPrevious := false
				if row.algorithm == "aes-256-gcm" {
					newBody, usedPrevious, err := decryptWithKeyFallback(newKey, oldKey, row.body, false)
					if err != nil {
						return fmt.Errorf("security.decrypt_failed table=security_reports row=%s", row.id)
					}
					bodyPrevious = usedPrevious
					if bodyPrevious {
						bodyCipher, err = encryptAESGCM(newKey, newBody)
						if err != nil {
							return err
						}
					}
				}
				contactCipher := row.contact
				if len(row.contact) > 0 {
					plain, usedPrevious, err := decryptWithKeyFallback(newKey, oldKey, row.contact, false)
					if err != nil {
						return fmt.Errorf("security.decrypt_failed table=security_reports row=%s", row.id)
					}
					if usedPrevious {
						contactCipher, err = encryptAESGCM(newKey, plain)
						if err != nil {
							return err
						}
					}
				}
				if bodyPrevious || !equalBytes(contactCipher, row.contact) {
					if _, err := tx.Exec(`UPDATE security_reports SET body_enc = ?, contact_enc = ? WHERE tracking_id = ?`, bodyCipher, contactCipher, row.id); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func resumeSecretRotations(database db.Handle, cfg *config.ApplicationConfig, configDir string) error {
	if err := ensureAppSecretsSchema(database); err != nil {
		return err
	}
	if err := resumeInstallationSecretRotation(database, configDir); err != nil {
		return err
	}
	var wrapped []byte
	err := database.QueryRow(`SELECT value FROM app_secrets WHERE name = 'encryption_key_previous'`).Scan(&wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	wrapped = db.DecodeAppSecret("encryption_key_previous", wrapped)
	current, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.Server.Security.EncryptionKey))
	if err != nil || len(current) != 32 {
		return errors.New("current encryption key is invalid during recovery")
	}
	previous, err := unwrapPreviousEncryptionKey(current, wrapped)
	if err != nil {
		// If the configuration still contains the old key, the interrupted operation stopped before its atomic config replace.
		_, _ = database.Exec(`DELETE FROM app_secrets WHERE name IN ('encryption_key_previous','encryption_key_previous_until')`)
		return nil
	}
	return reencryptEncryptionKeyRows(database, previous, current)
}

func resumeInstallationSecretRotation(database db.Handle, configDir string) error {
	var current, previous []byte
	if err := database.QueryRow(`SELECT value FROM app_secrets WHERE name='installation_secret'`).Scan(&current); err != nil {
		return err
	}
	current = db.DecodeAppSecret("installation_secret", current)
	err := database.QueryRow(`SELECT value FROM app_secrets WHERE name='installation_secret_previous'`).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	previous = db.DecodeAppSecret("installation_secret_previous", previous)
	keyPath := filepath.Join(configDir, "security", "pgp.priv.asc.enc")
	ciphertext, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = pgp.DecryptPrivate(current, ciphertext)
	if err == nil {
		return nil
	}
	private, err := pgp.DecryptPrivate(previous, ciphertext)
	if err != nil {
		return fmt.Errorf("PGP private key cannot be decrypted with current or previous installation secret")
	}
	return pgp.WritePrivate(keyPath, current, private)
}

func cleanupPreviousSecrets(database db.Handle, now time.Time) error {
	if err := ensureAppSecretsSchema(database); err != nil {
		return err
	}
	rows, err := database.Query(`SELECT name, CAST(value AS TEXT) FROM app_secrets WHERE name LIKE '%_previous_until'`)
	if err != nil {
		return err
	}
	type expiredSecret struct {
		untilName, previousName string
		expires                 string
	}
	var items []expiredSecret
	for rows.Next() {
		var item expiredSecret
		if err := rows.Scan(&item.untilName, &item.expires); err != nil {
			_ = rows.Close()
			return err
		}
		item.previousName = strings.TrimSuffix(item.untilName, "_until")
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range items {
		until, err := time.Parse(time.RFC3339, item.expires)
		if err != nil {
			return fmt.Errorf("invalid previous secret expiration for %s", item.previousName)
		}
		if now.Before(until) {
			continue
		}
		if _, err := database.Exec(`DELETE FROM app_secrets WHERE name IN (?, ?)`, item.previousName, item.untilName); err != nil {
			return err
		}
	}
	return nil
}
