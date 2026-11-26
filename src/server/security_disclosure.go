package server

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/go-chi/chi/v5/middleware"
)

const securityIDWindow = 48 * time.Hour

// SecurityReportMu serializes security-report submissions and PGP key rotations.
var SecurityReportMu sync.Mutex

var securityReportStatusOrder = map[string]int{
	"received":  0,
	"triaged":   1,
	"confirmed": 2,
	"patching":  3,
	"disclosed": 4,
}

var securityReportMarkup = regexp.MustCompile(`(?i)<\s*/?\s*[a-z!][^>]*(?:>|$)|&(?:lt|gt|#0*60|#0*62|#x0*3c|#x0*3e);`)

func (s *Server) encryptSecurityData(plaintext []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.config.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid security encryption key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, plaintext, nil)...), nil
}

func (s *Server) decryptSecurityData(ciphertext []byte) ([]byte, error) {
	return decryptSecurityPayload(s.config.Server.Security.EncryptionKey, ciphertext)
}

// DecryptSecurityContact returns the email and researcher public key stored
// in a security report's independently encrypted contact field.
func DecryptSecurityContact(encryptionKey string, ciphertext []byte) (email, pubkey string, err error) {
	plain, err := decryptSecurityPayload(encryptionKey, ciphertext)
	if err != nil {
		return "", "", err
	}
	var contact struct {
		Email         string `json:"email"`
		ResearcherGPG string `json:"researcher_gpg"`
	}
	if err := json.Unmarshal(plain, &contact); err != nil {
		return "", "", err
	}
	return contact.Email, contact.ResearcherGPG, nil
}

func decryptSecurityPayload(encodedKey string, ciphertext []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid security encryption key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted security data")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], nil)
}

func (s *Server) installationSecret() ([]byte, error) {
	if s.db == nil {
		return nil, fmt.Errorf("security disclosure storage is unavailable")
	}
	var secret []byte
	if err := s.db.QueryRow(`SELECT value FROM app_secrets WHERE name = ?`, "installation_secret").Scan(&secret); err != nil {
		return nil, err
	}
	secret = dbpkg.DecodeAppSecret("installation_secret", secret)
	if len(secret) != 32 {
		return nil, fmt.Errorf("invalid installation secret length")
	}
	return secret, nil
}

func (s *Server) securityIDAt(t time.Time) (string, error) {
	secret, err := s.installationSecret()
	if err != nil {
		return "", err
	}
	return securityIDAtSecret(t, secret), nil
}

func securityIDAtSecret(t time.Time, secret []byte) string {
	epoch := t.Unix() / int64(securityIDWindow.Seconds())
	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%d", epoch)
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func (s *Server) validSecurityID(value string, now time.Time) bool {
	value = strings.TrimSpace(value)
	if len(value) != 16 {
		return false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return false
	}
	currentSecret, err := s.installationSecret()
	if err != nil {
		return false
	}
	secrets := [][]byte{currentSecret}
	if previousSecret, ok := s.previousInstallationSecret(now); ok {
		secrets = append(secrets, previousSecret)
	}
	for _, secret := range secrets {
		if hmac.Equal([]byte(value), []byte(securityIDAtSecret(now, secret))) || hmac.Equal([]byte(value), []byte(securityIDAtSecret(now.Add(-securityIDWindow), secret))) {
			return true
		}
	}
	return false
}

func (s *Server) previousInstallationSecret(now time.Time) ([]byte, bool) {
	if s.db == nil {
		return nil, false
	}
	var previous []byte
	if err := s.db.QueryRow(`SELECT value FROM app_secrets WHERE name = 'installation_secret_previous'`).Scan(&previous); err != nil {
		return nil, false
	}
	previous = dbpkg.DecodeAppSecret("installation_secret_previous", previous)
	if len(previous) != 32 {
		return nil, false
	}
	var expiration string
	if err := s.db.QueryRow(`SELECT CAST(value AS TEXT) FROM app_secrets WHERE name = 'installation_secret_previous_until'`).Scan(&expiration); err != nil {
		return nil, false
	}
	until, err := time.Parse(time.RFC3339, expiration)
	if err != nil || !now.Before(until) {
		return nil, false
	}
	return previous, true
}

func (s *Server) logInvalidSecurityID(r *http.Request, value string) {
	digest := sha256.Sum256([]byte(value))
	if s.securityLog == nil {
		return
	}
	line := fmt.Sprintf("security.security_id_invalid ip=%q user_agent=%q id_sha256_prefix=%s", middleware.GetClientIP(r.Context()), r.UserAgent(), hex.EncodeToString(digest[:4]))
	if err := s.securityLog.Write(line); err != nil {
		log.Printf("Warning: failed to write security log event: %v", err)
	}
}

func (s *Server) securityReportURL(r *http.Request) string {
	securityID, err := s.securityIDAt(time.Now())
	if err != nil {
		return ""
	}
	base := strings.TrimSuffix(RequestBaseURL(r), "/")
	return base + "/server/contact?security_id=" + url.QueryEscape(securityID)
}

func securityExpiry(configured string, now time.Time) string {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(configured)); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(configured)); err == nil {
		return parsed.UTC().Format("2006-01-02T15:04:05Z")
	}
	return now.AddDate(1, 0, 0).UTC().Format("2006-01-02T15:04:05Z")
}

func constantHexEqual(left, right string) bool {
	leftBytes, leftErr := hex.DecodeString(left)
	rightBytes, rightErr := hex.DecodeString(right)
	if leftErr != nil || rightErr != nil || len(leftBytes) != len(rightBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(leftBytes, rightBytes) == 1
}

func (s *Server) pgpPublicKey() ([]byte, error) {
	if !s.config.Web.Security.PublishPGPKey || s.configDir == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(s.configDir, "security", "pgp.pub.asc"))
}

func openPGPV4Fingerprint(armored []byte) (string, error) {
	binary := armored
	if bytes.Contains(armored, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) {
		lines := bytes.Split(armored, []byte("\n"))
		var encoded bytes.Buffer
		inBody := false
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if !inBody {
				if len(line) == 0 {
					inBody = true
				}
				continue
			}
			if bytes.HasPrefix(line, []byte("=")) || bytes.HasPrefix(line, []byte("-----END ")) {
				break
			}
			encoded.Write(line)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded.String())
		if err != nil {
			return "", fmt.Errorf("decode armored OpenPGP key: %w", err)
		}
		binary = decoded
	}
	for offset := 0; offset < len(binary); {
		header := binary[offset]
		offset++
		if header&0x80 == 0 {
			return "", fmt.Errorf("invalid OpenPGP packet header")
		}
		var tag int
		var length uint64
		if header&0x40 != 0 {
			tag = int(header & 0x3f)
			if offset >= len(binary) {
				return "", fmt.Errorf("truncated OpenPGP packet length")
			}
			first := binary[offset]
			offset++
			switch {
			case first < 192:
				length = uint64(first)
			case first < 224:
				if offset >= len(binary) {
					return "", fmt.Errorf("truncated OpenPGP packet length")
				}
				length = uint64(first-192)<<8 + uint64(binary[offset]) + 192
				offset++
			case first == 255:
				if offset+4 > len(binary) {
					return "", fmt.Errorf("truncated OpenPGP packet length")
				}
				length = uint64(binary[offset])<<24 | uint64(binary[offset+1])<<16 | uint64(binary[offset+2])<<8 | uint64(binary[offset+3])
				offset += 4
			default:
				return "", fmt.Errorf("partial OpenPGP packet lengths are unsupported")
			}
		} else {
			tag = int((header >> 2) & 0x0f)
			switch header & 3 {
			case 0:
				if offset+1 > len(binary) {
					return "", fmt.Errorf("truncated OpenPGP packet length")
				}
				length = uint64(binary[offset])
				offset++
			case 1:
				if offset+2 > len(binary) {
					return "", fmt.Errorf("truncated OpenPGP packet length")
				}
				length = uint64(binary[offset])<<8 | uint64(binary[offset+1])
				offset += 2
			case 2:
				if offset+4 > len(binary) {
					return "", fmt.Errorf("truncated OpenPGP packet length")
				}
				length = uint64(binary[offset])<<24 | uint64(binary[offset+1])<<16 | uint64(binary[offset+2])<<8 | uint64(binary[offset+3])
				offset += 4
			case 3:
				length = uint64(len(binary) - offset)
			}
		}
		if length > uint64(len(binary)-offset) {
			return "", fmt.Errorf("truncated OpenPGP packet body")
		}
		body := binary[offset : offset+int(length)]
		offset += int(length)
		if tag != 6 {
			continue
		}
		if len(body) < 6 || body[0] != 4 || len(body) > 65535 {
			return "", fmt.Errorf("OpenPGP primary key is not version 4")
		}
		framed := make([]byte, 3+len(body))
		framed[0] = 0x99
		framed[1] = byte(len(body) >> 8)
		framed[2] = byte(len(body))
		copy(framed[3:], body)
		fingerprint := sha1.Sum(framed)
		return strings.ToUpper(hex.EncodeToString(fingerprint[:])), nil
	}
	return "", fmt.Errorf("OpenPGP public key packet was not found")
}

func (s *Server) ensureDisclosureSchema() error {
	if s.db == nil {
		return sql.ErrConnDone
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS security_reports (
		tracking_id TEXT PRIMARY KEY,
		security_id TEXT NOT NULL DEFAULT '',
		severity TEXT NOT NULL,
		component TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'received',
		body_enc BLOB NOT NULL DEFAULT X'',
		body_alg TEXT NOT NULL DEFAULT 'aes-256-gcm',
		contact_enc BLOB,
		credit_pref TEXT NOT NULL DEFAULT 'no',
		credit_name TEXT,
		researcher_comment TEXT NOT NULL DEFAULT '',
		disclosure_date TEXT,
		cve_requested INTEGER NOT NULL DEFAULT 0,
		token_hash TEXT NOT NULL DEFAULT '',
		token_last_used TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT '',
		closed_at TEXT
	)`)
	if err != nil {
		return err
	}
	columns, err := disclosureColumns(s.db)
	if err != nil {
		return err
	}
	additions := []struct{ name, definition string }{
		{"security_id", `TEXT NOT NULL DEFAULT ''`},
		{"body_enc", `BLOB NOT NULL DEFAULT X''`},
		{"body_alg", `TEXT NOT NULL DEFAULT 'aes-256-gcm'`},
		{"contact_enc", `BLOB`},
		{"credit_pref", `TEXT NOT NULL DEFAULT 'no'`},
		{"credit_name", `TEXT`},
		{"researcher_comment", `TEXT NOT NULL DEFAULT ''`},
		{"disclosure_date", `TEXT`},
		{"cve_requested", `INTEGER NOT NULL DEFAULT 0`},
		{"token_hash", `TEXT NOT NULL DEFAULT ''`},
		{"token_last_used", `TEXT`},
		{"updated_at", `TEXT NOT NULL DEFAULT ''`},
	}
	for _, column := range additions {
		if columns[column.name] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE security_reports ADD COLUMN ` + column.name + ` ` + column.definition); err != nil {
			return err
		}
	}
	if columns["body_ciphertext"] {
		if _, err := s.db.Exec(`UPDATE security_reports SET body_enc = body_ciphertext WHERE length(body_enc) = 0`); err != nil {
			return err
		}
		if err := s.migrateLegacySecurityBodies(); err != nil {
			return err
		}
	}
	if columns["credit_preference"] {
		if _, err := s.db.Exec(`UPDATE security_reports SET credit_pref = CASE lower(credit_preference) WHEN 'anonymous' THEN 'anon' WHEN 'name' THEN 'name' WHEN 'handle' THEN 'handle' ELSE 'no' END WHERE credit_pref = 'no'`); err != nil {
			return err
		}
	}
	if columns["status_token_hash"] {
		if _, err := s.db.Exec(`UPDATE security_reports SET token_hash = status_token_hash WHERE token_hash = ''`); err != nil {
			return err
		}
	}
	if columns["status_token_used_at"] {
		if _, err := s.db.Exec(`UPDATE security_reports SET token_last_used = substr(status_token_used_at, 1, 10) WHERE token_last_used IS NULL AND status_token_used_at <> ''`); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`UPDATE security_reports SET updated_at = created_at WHERE updated_at = ''`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE security_reports SET status = lower(status)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_security_reports_status ON security_reports(status, created_at)`); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS security_thanks (
		tracking_id TEXT PRIMARY KEY,
		credit TEXT NOT NULL,
		severity TEXT NOT NULL,
		disclosed_on TEXT NOT NULL
	)`)
	return err
}

func (s *Server) migrateLegacySecurityBodies() error {
	rows, err := s.db.Query(`SELECT tracking_id, body_enc FROM security_reports WHERE length(body_enc) > 0 AND contact_enc IS NULL`)
	if err != nil {
		return err
	}
	type legacyBody struct {
		trackingID string
		ciphertext []byte
	}
	var legacy []legacyBody
	for rows.Next() {
		var item legacyBody
		if err := rows.Scan(&item.trackingID, &item.ciphertext); err != nil {
			return errors.Join(err, rows.Close())
		}
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		return errors.Join(err, rows.Close())
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range legacy {
		plain, err := s.decryptSecurityData(item.ciphertext)
		if err != nil {
			continue
		}
		var report map[string]interface{}
		if err := json.Unmarshal(plain, &report); err != nil {
			continue
		}
		contact := make(map[string]string)
		for _, key := range []string{"name", "email", "researcher_gpg", "credit_name"} {
			if value, ok := report[key].(string); ok && value != "" {
				contact[key] = value
			}
			delete(report, key)
		}
		body, err := json.Marshal(report)
		if err != nil {
			continue
		}
		contactJSON, err := json.Marshal(contact)
		if err != nil {
			continue
		}
		bodyCipher, err := s.encryptSecurityData(body)
		if err != nil {
			return err
		}
		contactCipher, err := s.encryptSecurityData(contactJSON)
		if err != nil {
			return err
		}
		securityID, _ := report["security_id"].(string)
		cveRequested, _ := report["cve_requested"].(bool)
		creditName := any(nil)
		if value := contact["credit_name"]; value != "" {
			creditName = value
		}
		if _, err := s.db.Exec(`UPDATE security_reports SET security_id = ?, body_enc = ?, contact_enc = ?, credit_name = ?, cve_requested = ? WHERE tracking_id = ?`, securityID, bodyCipher, contactCipher, creditName, cveRequested, item.trackingID); err != nil {
			return err
		}
		if _, err := s.db.Exec(`UPDATE security_reports SET body_ciphertext = X'' WHERE tracking_id = ?`, item.trackingID); err != nil {
			return err
		}
	}
	return nil
}

func disclosureColumns(db dbpkg.Handle) (columns map[string]bool, resultErr error) {
	rows, err := db.Query(`PRAGMA table_info(security_reports)`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close security disclosure columns: %w", err))
		}
	}()
	columns = make(map[string]bool)
	for rows.Next() {
		var ordinal int
		var name, dataType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&ordinal, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

// SecurityReportUpdate contains the optional fields accepted by the protected
// maintenance security update command.
type SecurityReportUpdate struct {
	Status         *string
	Comment        *string
	DisclosureDate *string
	Severity       *string
}

// ApplySecurityReportUpdate performs the state transition and any hall-of-fame
// insertion in one database transaction. Callers must authorize the operator.
func ApplySecurityReportUpdate(db dbpkg.Handle, trackingID string, update SecurityReportUpdate, now time.Time) (resultErr error) {
	if db == nil {
		return sql.ErrConnDone
	}
	trackingID = strings.TrimSpace(trackingID)
	if trackingID == "" {
		return fmt.Errorf("tracking id is required")
	}
	if update.Status == nil && update.Comment == nil && update.DisclosureDate == nil && update.Severity == nil {
		return fmt.Errorf("at least one report field must be updated")
	}
	if update.Comment != nil {
		if len([]rune(*update.Comment)) > 2000 || securityReportMarkup.MatchString(*update.Comment) {
			return fmt.Errorf("researcher comment must be plain text and at most 2000 characters")
		}
	}
	if update.DisclosureDate != nil && *update.DisclosureDate != "" {
		if _, err := time.Parse("2006-01-02", *update.DisclosureDate); err != nil {
			return fmt.Errorf("disclosure date must use YYYY-MM-DD")
		}
	}
	if update.Severity != nil {
		switch *update.Severity {
		case "critical", "high", "medium", "low", "informational":
		default:
			return fmt.Errorf("invalid severity")
		}
	}
	if update.Status != nil {
		normalizedStatus := strings.ToLower(strings.TrimSpace(*update.Status))
		if _, ok := securityReportStatusOrder[normalizedStatus]; !ok && normalizedStatus != "wont_fix" {
			return fmt.Errorf("invalid security report status")
		}
		update.Status = &normalizedStatus
	}

	return dbpkg.Transaction(context.Background(), db, func(tx *dbpkg.TransactionHandle) error {
		var oldStatus, creditPreference, severity string
		var creditName sql.NullString
		if err := tx.QueryRow(`SELECT status, credit_pref, credit_name, severity FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&oldStatus, &creditPreference, &creditName, &severity); err != nil {
			return err
		}
		oldStatus = strings.ToLower(oldStatus)
		if update.Status != nil {
			if oldStatus == "disclosed" || oldStatus == "wont_fix" {
				if *update.Status != oldStatus {
					return fmt.Errorf("closed security reports cannot change status")
				}
			}
			if *update.Status != "wont_fix" && securityReportStatusOrder[*update.Status] <= securityReportStatusOrder[oldStatus] {
				return fmt.Errorf("security report status cannot move backward")
			}
		}
		newStatus := oldStatus
		if update.Status != nil {
			newStatus = *update.Status
		}
		newSeverity := severity
		if update.Severity != nil {
			newSeverity = *update.Severity
		}
		now = now.UTC()
		updatedAt := now.Format(time.RFC3339)
		sets := []string{"updated_at = ?"}
		args := []any{updatedAt}
		if update.Status != nil {
			sets = append(sets, "status = ?")
			args = append(args, newStatus)
		}
		if update.Comment != nil {
			sets = append(sets, "researcher_comment = ?")
			args = append(args, *update.Comment)
		}
		if update.DisclosureDate != nil {
			sets = append(sets, "disclosure_date = ?")
			if *update.DisclosureDate == "" {
				args = append(args, nil)
			} else {
				args = append(args, *update.DisclosureDate)
			}
		}
		if update.Severity != nil {
			sets = append(sets, "severity = ?")
			args = append(args, newSeverity)
		}
		if update.Status != nil && (newStatus == "disclosed" || newStatus == "wont_fix") {
			sets = append(sets, "closed_at = ?")
			args = append(args, updatedAt)
		}
		args = append(args, trackingID)
		if _, err := tx.Exec(`UPDATE security_reports SET `+strings.Join(sets, ", ")+` WHERE tracking_id = ?`, args...); err != nil {
			return err
		}
		if update.Status != nil && newStatus == "disclosed" && creditPreference != "no" {
			credit := creditName.String
			if creditPreference == "anon" {
				var anonymousCount int
				if err := tx.QueryRow(`SELECT COUNT(*) FROM security_thanks WHERE credit LIKE 'Anonymous Researcher #%'`).Scan(&anonymousCount); err != nil {
					return err
				}
				credit = fmt.Sprintf("Anonymous Researcher #%d", anonymousCount+1)
			}
			if credit == "" {
				return fmt.Errorf("security report credit name is missing")
			}
			if _, err := tx.Exec(`INSERT INTO security_thanks(tracking_id, credit, severity, disclosed_on) VALUES(?, ?, ?, ?) ON CONFLICT(tracking_id) DO NOTHING`, trackingID, credit, newSeverity, now.Format("2006-01-02")); err != nil {
				return err
			}
		}
		return nil
	})
}
