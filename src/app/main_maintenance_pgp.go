package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	paths "github.com/apimgr/airports/src/paths"
	"github.com/apimgr/airports/src/security/pgp"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

const pgpPrivateExportCooldown = time.Hour

type pgpMetadata struct {
	Fingerprint         string                   `json:"fingerprint"`
	CreatedAt           string                   `json:"created_at"`
	ExpiresAt           string                   `json:"expires_at"`
	LastRotatedAt       string                   `json:"last_rotated_at,omitempty"`
	KeyserversPublished map[string]string        `json:"keyservers_published"`
	KeyserverRetries    map[string]pgpRetryState `json:"keyserver_retries"`
	Revoked             bool                     `json:"revoked"`
	PrivateExportedAt   string                   `json:"private_exported_at,omitempty"`
}

type pgpRetryState struct {
	Attempts    int    `json:"attempts"`
	NextAttempt string `json:"next_attempt"`
	LastError   string `json:"last_error"`
}

// runMaintenancePGP handles the security keypair's authorized CLI operations.
func runMaintenancePGP(configDir, logsDir, rawArgs string) (resultErr error) {
	args := strings.Fields(rawArgs)
	defaultConfigDir, _, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	if configDir == "" {
		configDir = firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	}
	if logsDir == "" {
		logsDir = firstNonEmpty(os.Getenv("LOG_DIR"), defaultLogsDir)
	}
	if len(args) == 0 {
		return errors.New("usage: --maintenance pgp {generate|rotate|publish|export|import|delete}")
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	if !service.IsElevated() {
		provided, err := promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
		if cfg.Server.Token == "" || len(provided) != len(cfg.Server.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.Server.Token)) != 1 {
			return errors.New("PGP maintenance denied: invalid operator token")
		}
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close PGP maintenance database: %w", err))
		}
	}()
	if err := ensurePGPMetadataSchema(database); err != nil {
		return err
	}
	secret, err := loadInstallationSecret(database)
	if err != nil {
		return err
	}
	keyDir := filepath.Join(configDir, "security")
	privatePath := filepath.Join(keyDir, "pgp.priv.asc.enc")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	logs := logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
	defer func() {
		if err := logs.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close PGP maintenance log manager: %w", err))
		}
	}()

	switch args[0] {
	case "generate":
		if len(args) != 1 {
			return errors.New("usage: --maintenance pgp generate")
		}
		return generatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, logs)
	case "rotate":
		if len(args) != 1 {
			return errors.New("usage: --maintenance pgp rotate")
		}
		if err := confirmPGPAction("ROTATE PGP KEY"); err != nil {
			return err
		}
		return rotatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, logs)
	case "publish":
		if len(args) != 1 {
			return errors.New("usage: --maintenance pgp publish")
		}
		return publishPGPKey(database, keyDir, publicPath, cfg.Web.Security.Keyservers, logs)
	case "export":
		if len(args) < 2 || len(args) > 3 {
			return errors.New("usage: --maintenance pgp export {public [path]|private <path>}")
		}
		switch args[1] {
		case "public":
			if len(args) == 3 {
				return exportPublicPGPKey(publicPath, args[2])
			}
			content, err := os.ReadFile(publicPath)
			if err != nil {
				return fmt.Errorf("read public key: %w", err)
			}
			_, err = os.Stdout.Write(content)
			return err
		case "private":
			if len(args) != 3 {
				return errors.New("usage: --maintenance pgp export private <path>")
			}
			if err := confirmPGPAction("EXPORT PRIVATE KEY"); err != nil {
				return err
			}
			reason, err := readRequiredPGPInput("Reason for private key export: ", 500)
			if err != nil {
				return err
			}
			return exportPrivatePGPKey(database, secret, privatePath, args[2], logs, reason)
		default:
			return errors.New("usage: --maintenance pgp export {public [path]|private <path>}")
		}
	case "import":
		if len(args) != 2 {
			return errors.New("usage: --maintenance pgp import <file>")
		}
		if err := confirmPGPAction("IMPORT PRIVATE KEY"); err != nil {
			return err
		}
		return importPrivatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, args[1], logs)
	case "delete":
		if len(args) != 1 {
			return errors.New("usage: --maintenance pgp delete")
		}
		if err := confirmPGPAction("DELETE PGP KEY"); err != nil {
			return err
		}
		return deletePGPKey(database, keyDir, logs)
	default:
		return fmt.Errorf("unknown PGP action %q; use generate, rotate, publish, export, import, or delete", args[0])
	}
}

func ensurePGPMetadataSchema(database db.Handle) error {
	_, err := database.Exec(`CREATE TABLE IF NOT EXISTS pgp_metadata (
		id INTEGER PRIMARY KEY CHECK (id = 1), fingerprint TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '', expires_at TEXT NOT NULL DEFAULT '',
		last_rotated_at TEXT, keyservers_published TEXT NOT NULL DEFAULT '{}',
		keyserver_retries TEXT NOT NULL DEFAULT '{}',
		revoked INTEGER NOT NULL DEFAULT 0, private_exported_at TEXT
	)`)
	if err != nil {
		return err
	}
	if _, err := database.Exec(`ALTER TABLE pgp_metadata ADD COLUMN keyserver_retries TEXT NOT NULL DEFAULT '{}'`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS pgp_rotation_journal (id INTEGER PRIMARY KEY CHECK (id = 1), old_private BLOB NOT NULL, old_public BLOB NOT NULL, new_private BLOB NOT NULL, new_public BLOB NOT NULL, metadata TEXT NOT NULL)`); err != nil {
		return err
	}
	return nil
}

func loadInstallationSecret(database db.Handle) ([]byte, error) {
	if err := db.EnsureAppSecrets(database); err != nil {
		return nil, err
	}
	var secret []byte
	if err := database.QueryRow(`SELECT value FROM app_secrets WHERE name = ?`, "installation_secret").Scan(&secret); err != nil {
		return nil, err
	}
	secret = db.DecodeAppSecret("installation_secret", secret)
	if len(secret) != 32 {
		return nil, errors.New("invalid installation secret length")
	}
	return secret, nil
}

func generatePGPKey(database db.Handle, cfg *config.ApplicationConfig, keyDir, privatePath, publicPath string, secret []byte, logs *logging.LogManager) error {
	if _, err := os.Stat(privatePath); err == nil {
		return errors.New("an OpenPGP private key already exists; use --maintenance pgp rotate")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	contact := cfg.Server.Contact.ResolveEmail("security")
	if contact == "" {
		return errors.New("configure a security contact before generating the PGP key")
	}
	key, err := pgp.Generate(cfg.Server.Branding.Title+" Security", contact, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := pgp.WritePrivate(privatePath, secret, key.Private); err != nil {
		return err
	}
	if err := atomicPGPWrite(publicPath, key.Public, 0o600); err != nil {
		_ = os.Remove(privatePath)
		return err
	}
	if err := savePGPMetadata(database, pgpMetadata{Fingerprint: key.Fingerprint, CreatedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: key.ExpiresAt.Format(time.RFC3339), KeyserversPublished: map[string]string{}}); err != nil {
		return err
	}
	if err := writePGPAudit(logs, "security.pgp_generated", key.Fingerprint); err != nil {
		return err
	}
	if err := publishPGPKey(database, keyDir, publicPath, cfg.Web.Security.Keyservers, logs); err != nil {
		return fmt.Errorf("keypair generated but publishing did not complete: %w", err)
	}
	return nil
}

func rotatePGPKey(database db.Handle, cfg *config.ApplicationConfig, keyDir, privatePath, publicPath string, secret []byte, logs *logging.LogManager) error {
	server.SecurityReportMu.Lock()
	defer server.SecurityReportMu.Unlock()
	oldCipher, err := os.ReadFile(privatePath)
	if err != nil {
		return fmt.Errorf("read current private key: %w", err)
	}
	oldPrivate, err := pgp.DecryptPrivate(secret, oldCipher)
	if err != nil {
		return err
	}
	oldEntity, err := pgp.ParsePrivate(oldPrivate)
	if err != nil {
		return err
	}
	contact := cfg.Server.Contact.ResolveEmail("security")
	if contact == "" {
		return errors.New("configure a security contact before rotating the PGP key")
	}
	newKey, err := pgp.Generate(cfg.Server.Branding.Title+" Security", contact, time.Now().UTC())
	if err != nil {
		return err
	}
	newKey.Private, newKey.Public, err = pgp.SignNewIdentity(newKey.Private, oldPrivate)
	if err != nil {
		return err
	}
	newEntity, err := pgp.ParsePrivate(newKey.Private)
	if err != nil {
		return err
	}
	oldPublic, err := os.ReadFile(publicPath)
	if err != nil {
		return err
	}
	meta, err := readPGPMetadata(database)
	if err != nil {
		return err
	}
	meta.Fingerprint = newKey.Fingerprint
	meta.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	meta.ExpiresAt = newKey.ExpiresAt.Format(time.RFC3339)
	meta.LastRotatedAt = time.Now().UTC().Format(time.RFC3339)
	meta.Revoked = false
	newCipher, err := pgp.EncryptPrivate(secret, newKey.Private)
	if err != nil {
		return err
	}
	if err := rotatePGPReportBodiesWithJournal(database, oldEntity, newEntity, oldCipher, oldPublic, newCipher, newKey.Public, meta); err != nil {
		return err
	}
	if err := applyPGPRotationJournal(database, filepath.Dir(keyDir)); err != nil {
		return fmt.Errorf("PGP key rotation is committed and will resume on restart: %w", err)
	}
	if err := writePGPAudit(logs, "security.pgp_rotated", newKey.Fingerprint); err != nil {
		return err
	}
	if err := publishPGPKey(database, keyDir, publicPath, cfg.Web.Security.Keyservers, logs); err != nil {
		return fmt.Errorf("keypair rotated but keyserver publication did not complete: %w", err)
	}
	return nil
}

func rotatePGPReportBodiesWithJournal(database db.Handle, oldEntity, newEntity *openpgp.Entity, oldPrivate, oldPublic, newPrivate, newPublic []byte, metadata pgpMetadata) error {
	if err := ensurePGPMetadataSchema(database); err != nil {
		return err
	}
	var exists int
	err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'security_reports'`).Scan(&exists)
	if err != nil {
		return err
	}
	type encryptedReport struct {
		id   string
		body []byte
	}
	var reports []encryptedReport
	if exists > 0 {
		rows, err := database.Query(`SELECT tracking_id, body_enc FROM security_reports WHERE body_alg = 'pgp'`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var report encryptedReport
			if err := rows.Scan(&report.id, &report.body); err != nil {
				_ = rows.Close()
				return err
			}
			reports = append(reports, report)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	var encryptedReports []encryptedReport
	for _, report := range reports {
		message, err := openpgp.ReadMessage(bytes.NewReader(report.body), openpgp.EntityList{oldEntity}, func([]openpgp.Key, bool) ([]byte, error) {
			return nil, errors.New("symmetric OpenPGP report decryption is not permitted")
		}, nil)
		if err != nil {
			return fmt.Errorf("decrypt PGP report %s: %w", report.id, err)
		}
		plain, err := io.ReadAll(message.UnverifiedBody)
		if err != nil || message.SignatureError != nil {
			return fmt.Errorf("verify PGP report %s: %w", report.id, errors.Join(err, message.SignatureError))
		}
		var encrypted bytes.Buffer
		writer, err := openpgp.Encrypt(&encrypted, []*openpgp.Entity{newEntity}, nil, nil, nil)
		if err != nil {
			return err
		}
		if _, err := writer.Write(plain); err != nil {
			_ = writer.Close()
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		encryptedReports = append(encryptedReports, encryptedReport{id: report.id, body: encrypted.Bytes()})
	}
	if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
		if len(newPrivate) > 0 {
			metadataBytes, err := json.Marshal(metadata)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO pgp_rotation_journal(id, old_private, old_public, new_private, new_public, metadata) VALUES(1, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET old_private=excluded.old_private, old_public=excluded.old_public, new_private=excluded.new_private, new_public=excluded.new_public, metadata=excluded.metadata`, oldPrivate, oldPublic, newPrivate, newPublic, string(metadataBytes)); err != nil {
				return err
			}
		}
		for _, report := range encryptedReports {
			if _, err := tx.Exec(`UPDATE security_reports SET body_enc = ? WHERE tracking_id = ?`, report.body, report.id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func applyPGPRotationJournal(database db.Handle, configDir string) error {
	if err := ensurePGPMetadataSchema(database); err != nil {
		return err
	}
	var oldPrivate, newPrivate, newPublic []byte
	var metadataJSON string
	err := database.QueryRow(`SELECT old_private, new_private, new_public, metadata FROM pgp_rotation_journal WHERE id = 1`).Scan(&oldPrivate, &newPrivate, &newPublic, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	metadata := pgpMetadata{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return err
	}
	keyDir := filepath.Join(configDir, "security")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return err
	}
	if err := atomicPGPWrite(filepath.Join(keyDir, "pgp.priv.old.asc.enc"), oldPrivate, 0o600); err != nil {
		return err
	}
	if err := atomicPGPWrite(filepath.Join(keyDir, "pgp.priv.asc.enc"), newPrivate, 0o600); err != nil {
		return err
	}
	if err := atomicPGPWrite(filepath.Join(keyDir, "pgp.pub.asc"), newPublic, 0o600); err != nil {
		return err
	}
	if err := savePGPMetadata(database, metadata); err != nil {
		return err
	}
	_, err = database.Exec(`DELETE FROM pgp_rotation_journal WHERE id = 1`)
	return err
}

func publishPGPKey(database db.Handle, keyDir, publicPath string, keyservers []string, logs *logging.LogManager) error {
	public, err := os.ReadFile(publicPath)
	if err != nil {
		return err
	}
	meta, err := readPGPMetadata(database)
	if err != nil {
		return err
	}
	if meta.KeyserversPublished == nil {
		meta.KeyserversPublished = make(map[string]string)
	}
	if meta.KeyserverRetries == nil {
		meta.KeyserverRetries = make(map[string]pgpRetryState)
	}
	var failures []string
	for _, target := range keyservers {
		endpoint, err := pgpKeyserverEndpoint(target)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", target, err))
			meta.KeyserverRetries[target] = pgpRetryState{Attempts: 3, NextAttempt: time.Now().UTC().Add(4 * time.Second).Format(time.RFC3339), LastError: err.Error()}
			continue
		}
		var publishErr error
		for attempt := 0; attempt < 3; attempt++ {
			publishErr = postPGPKey(endpoint, public)
			if publishErr == nil {
				break
			}
			if attempt < 2 {
				time.Sleep(time.Duration(1<<attempt) * time.Second)
			}
		}
		if publishErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", target, publishErr))
			meta.KeyserverRetries[target] = pgpRetryState{Attempts: 3, NextAttempt: time.Now().UTC().Add(4 * time.Second).Format(time.RFC3339), LastError: publishErr.Error()}
			continue
		}
		meta.KeyserversPublished[target] = time.Now().UTC().Format(time.RFC3339)
		delete(meta.KeyserverRetries, target)
	}
	if err := savePGPMetadata(database, meta); err != nil {
		return err
	}
	state, err := json.MarshalIndent(struct {
		Published map[string]string        `json:"published"`
		Retries   map[string]pgpRetryState `json:"retries"`
	}{Published: meta.KeyserversPublished, Retries: meta.KeyserverRetries}, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicPGPWrite(filepath.Join(keyDir, "keyservers.state"), append(state, '\n'), 0o600); err != nil {
		return err
	}
	if len(failures) > 0 {
		line := "security.pgp_publish_failed targets=" + strconv.Quote(strings.Join(failures, "; "))
		if err := logs.Security().Write(line); err != nil {
			return errors.Join(errors.New(strings.Join(failures, "; ")), err)
		}
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func pgpKeyserverEndpoint(target string) (string, error) {
	value := strings.TrimSpace(target)
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("keyserver must be a valid HTTPS URL")
	}
	if strings.TrimRight(u.Path, "/") == "" {
		switch strings.ToLower(u.Hostname()) {
		case "keys.openpgp.org":
			u.Path = "/vks/v1/upload"
		default:
			u.Path = "/pks/add"
		}
	}
	return u.String(), nil
}

func retryPendingPGPKeyservers(database db.Handle, configDir string, keyservers []string, logs *logging.LogManager, now time.Time) error {
	if err := ensurePGPMetadataSchema(database); err != nil {
		return err
	}
	meta, err := readPGPMetadata(database)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(meta.KeyserverRetries) == 0 {
		return nil
	}
	public, err := os.ReadFile(filepath.Join(configDir, "security", "pgp.pub.asc"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	configured := make(map[string]struct{}, len(keyservers))
	for _, target := range keyservers {
		configured[target] = struct{}{}
	}
	var failures []string
	for target, retry := range meta.KeyserverRetries {
		if _, ok := configured[target]; !ok {
			delete(meta.KeyserverRetries, target)
			continue
		}
		next, parseErr := time.Parse(time.RFC3339, retry.NextAttempt)
		if parseErr != nil {
			return parseErr
		}
		if now.Before(next) {
			continue
		}
		endpoint, err := pgpKeyserverEndpoint(target)
		if err == nil {
			err = postPGPKey(endpoint, public)
		}
		if err == nil {
			meta.KeyserversPublished[target] = now.UTC().Format(time.RFC3339)
			delete(meta.KeyserverRetries, target)
			continue
		}
		retry.Attempts++
		delay := time.Second * time.Duration(1<<min(retry.Attempts, 16))
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
		retry.NextAttempt = now.UTC().Add(delay).Format(time.RFC3339)
		retry.LastError = err.Error()
		meta.KeyserverRetries[target] = retry
		failures = append(failures, target+": "+err.Error())
	}
	if err := savePGPMetadata(database, meta); err != nil {
		return err
	}
	if err := savePGPKeyserverState(filepath.Join(configDir, "security", "keyservers.state"), meta); err != nil {
		return err
	}
	if len(failures) != 0 && logs != nil {
		if err := logs.Security().Write("security.pgp_publish_retry_failed targets=" + strconv.Quote(strings.Join(failures, "; "))); err != nil {
			return err
		}
	}
	return nil
}

func savePGPKeyserverState(path string, meta pgpMetadata) error {
	state, err := json.MarshalIndent(struct {
		Published map[string]string        `json:"published"`
		Retries   map[string]pgpRetryState `json:"retries"`
	}{Published: meta.KeyserversPublished, Retries: meta.KeyserverRetries}, "", "  ")
	if err != nil {
		return err
	}
	return atomicPGPWrite(path, append(state, '\n'), 0o600)
}

func postPGPKey(endpoint string, public []byte) (resultErr error) {
	form := url.Values{"keytext": {string(public)}}
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close keyserver response: %w", err))
		}
	}()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("keyserver returned HTTP %d", response.StatusCode)
	}
	return nil
}

func exportPublicPGPKey(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return atomicPGPWrite(target, content, 0o600)
}

func exportPrivatePGPKey(database db.Handle, secret []byte, source, target string, logs *logging.LogManager, reason string) error {
	meta, err := readPGPMetadata(database)
	if err != nil {
		return err
	}
	if meta.PrivateExportedAt != "" {
		last, parseErr := time.Parse(time.RFC3339, meta.PrivateExportedAt)
		if parseErr != nil {
			return errors.New("invalid private key export audit timestamp")
		}
		if time.Since(last) < pgpPrivateExportCooldown {
			return errors.New("private-key export is limited to once per hour")
		}
	}
	private, err := pgp.ReadPrivate(source, secret)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("private key export target already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeExclusivePrivateFile(target, private); err != nil {
		return err
	}
	meta.PrivateExportedAt = time.Now().UTC().Format(time.RFC3339)
	if err := savePGPMetadata(database, meta); err != nil {
		_ = os.Remove(target)
		return err
	}
	if err := logs.WriteAudit(logging.AuditCategorySecurity, "security.private_key_exported operator_ip="+strconv.Quote(maintenanceOperatorIP())+" reason="+strconv.Quote(reason)); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func maintenanceOperatorIP() string {
	if value := strings.TrimSpace(os.Getenv("SSH_CONNECTION")); value != "" {
		fields := strings.Fields(value)
		if len(fields) >= 4 && net.ParseIP(fields[0]) != nil {
			return fields[0]
		}
	}
	return "127.0.0.1"
}

func importPrivatePGPKey(database db.Handle, cfg *config.ApplicationConfig, keyDir, privatePath, publicPath string, secret []byte, source string, logs *logging.LogManager) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	entity, err := pgp.ParsePrivate(input)
	if err != nil {
		return err
	}
	identity := entity.PrimaryIdentity()
	expectedName := cfg.Server.Branding.Title + " Security"
	expectedEmail := cfg.Server.Contact.ResolveEmail("security")
	if identity == nil || identity.UserId == nil || identity.UserId.Name != expectedName || !strings.EqualFold(identity.UserId.Email, expectedEmail) {
		fmt.Fprintln(os.Stderr, "Warning: imported key identity does not match the configured security identity.")
		answer, err := readRequiredPGPInput("Type IMPORT KEY ANYWAY to continue: ", 40)
		if err != nil || answer != "IMPORT KEY ANYWAY" {
			return errors.New("private key import cancelled")
		}
	}
	public, err := pgp.SerializePublicArmor(entity)
	if err != nil {
		return err
	}
	fingerprint, err := pgp.Fingerprint(public)
	if err != nil {
		return err
	}
	oldCipher, privateErr := os.ReadFile(privatePath)
	if privateErr != nil && !errors.Is(privateErr, os.ErrNotExist) {
		return privateErr
	}
	oldPublic, publicErr := os.ReadFile(publicPath)
	if publicErr != nil && !errors.Is(publicErr, os.ErrNotExist) {
		return publicErr
	}
	oldMeta, metaErr := readPGPMetadata(database)
	if metaErr != nil && !errors.Is(metaErr, sql.ErrNoRows) {
		return metaErr
	}
	rotated := privateErr == nil
	oldPath := filepath.Join(keyDir, "pgp.priv.old.asc.enc")
	if rotated && len(oldPublic) == 0 {
		return errors.New("cannot replace an active PGP key without its public key file")
	}
	rollback := func() {
		if len(oldCipher) > 0 {
			_ = atomicPGPWrite(privatePath, oldCipher, 0o600)
		} else {
			_ = os.Remove(privatePath)
		}
		if len(oldPublic) > 0 {
			_ = atomicPGPWrite(publicPath, oldPublic, 0o600)
		} else {
			_ = os.Remove(publicPath)
		}
		if rotated {
			_ = os.Remove(oldPath)
		}
		if metaErr == nil {
			_ = savePGPMetadata(database, oldMeta)
		}
	}
	newCipher, err := pgp.EncryptPrivate(secret, input)
	if err != nil {
		return err
	}
	meta := pgpMetadata{Fingerprint: fingerprint, CreatedAt: entity.PrimaryKey.CreationTime.UTC().Format(time.RFC3339), ExpiresAt: entity.PrimaryKey.CreationTime.AddDate(2, 0, 0).UTC().Format(time.RFC3339), KeyserversPublished: map[string]string{}}
	if rotated {
		meta.LastRotatedAt = time.Now().UTC().Format(time.RFC3339)
		meta.KeyserversPublished = oldMeta.KeyserversPublished
	}
	if rotated {
		oldPrivate, err := pgp.DecryptPrivate(secret, oldCipher)
		if err != nil {
			rollback()
			return err
		}
		oldEntity, err := pgp.ParsePrivate(oldPrivate)
		if err != nil {
			rollback()
			return err
		}
		if err := rotatePGPReportBodiesWithJournal(database, oldEntity, entity, oldCipher, oldPublic, newCipher, public, meta); err != nil {
			rollback()
			return err
		}
		if err := applyPGPRotationJournal(database, filepath.Dir(keyDir)); err != nil {
			return fmt.Errorf("PGP import committed; startup recovery will finish key-file activation: %w", err)
		}
	} else {
		if err := atomicPGPWrite(privatePath, newCipher, 0o600); err != nil {
			rollback()
			return err
		}
		if err := atomicPGPWrite(publicPath, public, 0o600); err != nil {
			rollback()
			return err
		}
		if err := savePGPMetadata(database, meta); err != nil {
			rollback()
			return err
		}
	}
	if err := writePGPAudit(logs, "security.pgp_imported", fingerprint); err != nil {
		return err
	}
	return publishPGPKey(database, keyDir, publicPath, cfg.Web.Security.Keyservers, logs)
}

func deletePGPKey(database db.Handle, keyDir string, logs *logging.LogManager) error {
	if _, err := os.Stat(filepath.Join(keyDir, "pgp.priv.asc.enc")); errors.Is(err, os.ErrNotExist) {
		return errors.New("no PGP keypair exists")
	}
	meta, err := readPGPMetadata(database)
	if err != nil {
		return err
	}
	originalMeta := meta
	originalMeta.KeyserversPublished = make(map[string]string, len(meta.KeyserversPublished))
	for target, publishedAt := range meta.KeyserversPublished {
		originalMeta.KeyserversPublished[target] = publishedAt
	}
	stagingDir, err := os.MkdirTemp(keyDir, ".pgp-delete-*")
	if err != nil {
		return err
	}
	if err := os.Chmod(stagingDir, 0o700); err != nil {
		_ = os.RemoveAll(stagingDir)
		return err
	}
	var staged []string
	rollback := func(cause error, metadataSaved bool) error {
		var rollbackErrs []error
		for _, name := range staged {
			if err := os.Rename(filepath.Join(stagingDir, name), filepath.Join(keyDir, name)); err != nil {
				rollbackErrs = append(rollbackErrs, err)
			}
		}
		if metadataSaved {
			if err := savePGPMetadata(database, originalMeta); err != nil {
				rollbackErrs = append(rollbackErrs, err)
			}
		}
		if err := os.RemoveAll(stagingDir); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
		return errors.Join(append([]error{cause}, rollbackErrs...)...)
	}
	for _, name := range []string{"pgp.priv.asc.enc", "pgp.priv.old.asc.enc", "pgp.pub.asc", "pgp.pub.asc.sig", "keyservers.state"} {
		path := filepath.Join(keyDir, name)
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return rollback(err, false)
		}
		if err := os.Rename(path, filepath.Join(stagingDir, name)); err != nil {
			return rollback(err, false)
		}
		staged = append(staged, name)
	}
	meta.Revoked = true
	meta.LastRotatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := savePGPMetadata(database, meta); err != nil {
		return rollback(err, true)
	}
	if err := writePGPAudit(logs, "security.pgp_deleted", meta.Fingerprint); err != nil {
		return rollback(err, true)
	}
	return os.RemoveAll(stagingDir)
}

func savePGPMetadata(database db.Handle, value pgpMetadata) error {
	if value.KeyserversPublished == nil {
		value.KeyserversPublished = make(map[string]string)
	}
	published, err := json.Marshal(value.KeyserversPublished)
	if err != nil {
		return err
	}
	retries, err := json.Marshal(value.KeyserverRetries)
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO pgp_metadata(id, fingerprint, created_at, expires_at, last_rotated_at, keyservers_published, keyserver_retries, revoked, private_exported_at)
		VALUES(1, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''))
		ON CONFLICT(id) DO UPDATE SET fingerprint=excluded.fingerprint, created_at=excluded.created_at,
		expires_at=excluded.expires_at, last_rotated_at=excluded.last_rotated_at,
		keyservers_published=excluded.keyservers_published, keyserver_retries=excluded.keyserver_retries, revoked=excluded.revoked,
		private_exported_at=excluded.private_exported_at`, value.Fingerprint, value.CreatedAt, value.ExpiresAt, value.LastRotatedAt, string(published), string(retries), value.Revoked, value.PrivateExportedAt)
	return err
}

func readPGPMetadata(database db.Handle) (pgpMetadata, error) {
	var value pgpMetadata
	var published string
	var revoked int
	var lastRotated, privateExported sql.NullString
	var retries string
	err := database.QueryRow(`SELECT fingerprint, created_at, expires_at, last_rotated_at, keyservers_published, keyserver_retries, revoked, private_exported_at FROM pgp_metadata WHERE id = 1`).Scan(&value.Fingerprint, &value.CreatedAt, &value.ExpiresAt, &lastRotated, &published, &retries, &revoked, &privateExported)
	if err != nil {
		return value, err
	}
	if lastRotated.Valid {
		value.LastRotatedAt = lastRotated.String
	}
	if privateExported.Valid {
		value.PrivateExportedAt = privateExported.String
	}
	value.Revoked = revoked != 0
	if err := json.Unmarshal([]byte(published), &value.KeyserversPublished); err != nil {
		return value, err
	}
	if err := json.Unmarshal([]byte(retries), &value.KeyserverRetries); err != nil {
		return value, err
	}
	return value, nil
}

func writePGPAudit(logs *logging.LogManager, event, fingerprint string) error {
	return logs.WriteAudit(logging.AuditCategorySecurity, event+" fingerprint="+strconv.Quote(fingerprint))
}

func confirmPGPAction(expected string) error {
	answer, err := readRequiredPGPInput("Type "+expected+" to confirm: ", 80)
	if err != nil {
		return err
	}
	if answer != expected {
		return errors.New("PGP operation cancelled")
	}
	return nil
}

func readRequiredPGPInput(prompt string, maxLength int) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	input := bufio.NewScanner(os.Stdin)
	input.Buffer(make([]byte, 256), maxLength+1)
	if !input.Scan() {
		if err := input.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no confirmation entered")
	}
	value := strings.TrimSpace(input.Text())
	if value == "" || len(value) > maxLength {
		return "", errors.New("confirmation is empty or too long")
	}
	return value, nil
}

func atomicPGPWrite(path string, content []byte, mode os.FileMode) (resultErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".pgp-write-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary PGP file: %w", err))
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func writeExclusivePrivateFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

// CleanupRetiredPGPKey deletes the previous private key after its 30-day grace period.
func CleanupRetiredPGPKey(database db.Handle, configDir string, now time.Time) error {
	if err := ensurePGPMetadataSchema(database); err != nil {
		return err
	}
	meta, err := readPGPMetadata(database)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if meta.LastRotatedAt == "" {
		return nil
	}
	rotated, err := time.Parse(time.RFC3339, meta.LastRotatedAt)
	if err != nil {
		return err
	}
	if now.Before(rotated.Add(30 * 24 * time.Hour)) {
		return nil
	}
	err = os.Remove(filepath.Join(configDir, "security", "pgp.priv.old.asc.enc"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func CleanupSecurityReports(database db.Handle, configDir string, now time.Time, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 365
	}
	var exists int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='security_reports'`).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		cutoff := now.UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour).Format(time.RFC3339)
		for {
			var deleted int64
			if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
				result, err := tx.Exec(`DELETE FROM security_reports WHERE tracking_id IN (SELECT tracking_id FROM security_reports WHERE closed_at IS NOT NULL AND closed_at <= ? ORDER BY closed_at LIMIT 500)`, cutoff)
				if err != nil {
					return err
				}
				deleted, err = result.RowsAffected()
				return err
			}); err != nil {
				return err
			}
			if deleted < 500 {
				break
			}
		}
	}
	if err := cleanupPreviousSecrets(database, now); err != nil {
		return err
	}
	return CleanupRetiredPGPKey(database, configDir, now)
}
