package server

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	"github.com/go-chi/chi/v5"
)

const ownerTokenCookieName = "airports_owner_token_6kP9qV2x"
const ownerTokenViewCookieName = "airports_token_view"
const ownerTokenNoticeCookieName = "airports_token_notice"
const ownerTokenAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const encryptionKeyPreviousPrefix = "airports-encryption-key-previous:v1:"

var errOwnerTokenUnauthorized = errors.New("owner token authorization failed")
var errOwnerTokenInput = errors.New("invalid token management input")

// ownerTokenRecord contains the public token metadata returned to the owner.
type ownerTokenRecord struct {
	ID             int64  `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name,omitempty"`
	Prefix         string `json:"prefix"`
	CreatedAt      int64  `json:"created_at"`
	LastUsedAt     *int64 `json:"last_used_at,omitempty"`
	ExpiresAt      *int64 `json:"expires_at,omitempty"`
	Status         string `json:"status"`
	Masked         string `json:"masked"`
	CreatedLabel   string `json:"-"`
	LastUsedLabel  string `json:"-"`
	ExpiresLabel   string `json:"-"`
	ExpiryChoice   string `json:"-"`
	ExpiresOnInput string `json:"-"`
}

// ownerTokenIdentity identifies a validated primary or linked token.
type ownerTokenIdentity struct {
	ID     int64
	Owner  string
	Kind   string
	Prefix string
}

func (s *Server) ensureOwnerTokenSchema() error {
	return ensureOwnerTokenSchema(s.db)
}

func ensureOwnerTokenSchema(db dbpkg.Handle) error {
	if db == nil {
		return errors.New("owner token database is unavailable")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS owners (
			id TEXT PRIMARY KEY,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			last_seen_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS api_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id TEXT NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
			kind TEXT NOT NULL CHECK (kind IN ('primary', 'linked')),
			name TEXT,
			token_hash TEXT NOT NULL UNIQUE,
			token_prefix TEXT NOT NULL,
			token_suffix TEXT NOT NULL DEFAULT '',
			token_enc TEXT NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			rotated_at INTEGER,
			expires_at INTEGER,
			last_used_at INTEGER,
			revoked_at INTEGER,
			revoked_reason TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_primary ON api_tokens(owner_id) WHERE kind = 'primary' AND revoked_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(token_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_owner ON api_tokens(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_active ON api_tokens(revoked_at) WHERE revoked_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_linked_name ON api_tokens(owner_id, name) WHERE kind = 'linked' AND revoked_at IS NULL`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("create owner token schema: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE api_tokens ADD COLUMN token_suffix TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return fmt.Errorf("add owner token suffix metadata: %w", err)
	}
	return nil
}

// OwnerTokenSummary contains CLI-safe owner-token metadata and never includes
// the token value or its encrypted copy.
type OwnerTokenSummary struct {
	OwnerID   string `json:"owner_id"`
	Prefix    string `json:"prefix"`
	Kind      string `json:"kind"`
	Name      string `json:"name,omitempty"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  *int64 `json:"last_used_at,omitempty"`
	Status    string `json:"status"`
}

// ListOwnerTokens returns token metadata for the maintenance CLI without
// exposing raw or encrypted token values.
func ListOwnerTokens(db dbpkg.Handle) (items []OwnerTokenSummary, resultErr error) {
	if err := ensureOwnerTokenSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT owner_id, token_prefix, kind, COALESCE(name,''), created_at, last_used_at, expires_at, revoked_at
		FROM api_tokens ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close owner token rows: %w", err))
		}
	}()
	items = make([]OwnerTokenSummary, 0)
	now := time.Now().Unix()
	for rows.Next() {
		var item OwnerTokenSummary
		var lastUsed, expires, revoked sql.NullInt64
		if err := rows.Scan(&item.OwnerID, &item.Prefix, &item.Kind, &item.Name, &item.CreatedAt, &lastUsed, &expires, &revoked); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			value := lastUsed.Int64
			item.LastUsed = &value
		}
		switch {
		case revoked.Valid:
			item.Status = "revoked"
		case expires.Valid && expires.Int64 <= now:
			item.Status = "expired"
		default:
			item.Status = "active"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// RevokeOwnerTokenByPrefix revokes a token by its displayed prefix. Revoking
// a primary token revokes all tokens belonging to that owner.
func RevokeOwnerTokenByPrefix(db dbpkg.Handle, prefix string) error {
	return RevokeOwnerTokenByPrefixWithAudit(db, prefix, nil, "api", "")
}

// RevokeOwnerTokenByPrefixWithAudit revokes by the displayed prefix and writes
// the required token.revoked event when a configured log writer is supplied.
func RevokeOwnerTokenByPrefixWithAudit(db dbpkg.Handle, prefix string, auditWriter interface {
	WriteAudit(logging.AuditCategory, string) error
}, source, actorIP string) error {
	if err := ensureOwnerTokenSchema(db); err != nil {
		return err
	}
	prefix = strings.TrimSpace(prefix)
	if len(prefix) != 12 || !strings.HasPrefix(prefix, "own_") {
		return errOwnerTokenInput
	}
	var owner, kind string
	if err := db.QueryRow(`SELECT owner_id, kind FROM api_tokens WHERE token_prefix = ? AND revoked_at IS NULL`, prefix).Scan(&owner, &kind); err != nil {
		return errOwnerTokenUnauthorized
	}
	if kind == "primary" {
		tokenCount, resourceCount, err := deleteOwnerTokenIdentity(db, owner)
		if err == nil {
			writeOwnerTokenAuditEntry(auditWriter, "revoked", owner, prefix, "owner", tokenCount, resourceCount, source, auditActor(source, owner), actorIP)
		}
		return err
	}
	now := time.Now().Unix()
	_, err := db.Exec(`UPDATE api_tokens SET revoked_at = ?, revoked_reason = 'operator_action' WHERE token_prefix = ? AND revoked_at IS NULL`, now, prefix)
	if err == nil {
		writeOwnerTokenAuditEntry(auditWriter, "revoked", owner, prefix, "token", 1, 0, source, auditActor(source, owner), actorIP)
	}
	return err
}

func deleteOwnerTokenIdentity(db dbpkg.Handle, owner string) (tokenCount, resourceCount int, resultErr error) {
	if err := dbpkg.Transaction(context.Background(), db, func(tx *dbpkg.TransactionHandle) error {
		deletedResources, err := dbpkg.DeleteOwnerResources(tx, owner)
		if err != nil {
			return err
		}
		resourceCount = int(deletedResources)
		if err := tx.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE owner_id = ?`, owner).Scan(&tokenCount); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM api_tokens WHERE owner_id = ?`, owner); err != nil {
			return err
		}
		result, err := tx.Exec(`DELETE FROM owners WHERE id = ?`, owner)
		if err != nil {
			return err
		}
		deletedOwners, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if deletedOwners != 1 {
			return errors.New("owner record disappeared during deletion")
		}
		return nil
	}); err != nil {
		return 0, 0, err
	}
	return tokenCount, resourceCount, nil
}

func ownerTokenRevocationDetails(db dbpkg.Handle, owner string) (tokenCount, linkedCount int, resourceCount int64, last4 string, resultErr error) {
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN kind = 'linked' THEN 1 ELSE 0 END), 0), COALESCE(MAX(CASE WHEN kind = 'primary' THEN token_suffix ELSE '' END), '') FROM api_tokens WHERE owner_id = ?`, owner).Scan(&tokenCount, &linkedCount, &last4); err != nil {
		return 0, 0, 0, "", err
	}
	resourceCount, err := dbpkg.CountOwnerResources(db, owner)
	if err != nil {
		return 0, 0, 0, "", err
	}
	return tokenCount, linkedCount, resourceCount, last4, nil
}

func CountOwnerResources(database dbpkg.Handle, owner string) (int64, error) {
	return dbpkg.CountOwnerResources(database, owner)
}

func writeOwnerTokenAuditEntry(writer interface {
	WriteAudit(logging.AuditCategory, string) error
}, event, owner, prefix, scope string, tokenCount, resourceCount int, source, actor, actorIP string) {
	if writer == nil {
		return
	}
	line := fmt.Sprintf("event=token.%s owner_id=%q token_prefix=%q scope=%q token_count=%d resource_count=%d actor=%q source=%q details_source=%q ip=%q", event, owner, prefix, scope, tokenCount, resourceCount, actor, source, source, actorIP)
	writeOwnerTokenAuditLine(writer, line)
}

func writeOwnerTokenAuditLine(writer interface {
	WriteAudit(logging.AuditCategory, string) error
}, line string) {
	if writer != nil {
		if err := writer.WriteAudit(logging.AuditCategorySecurity, line); err != nil {
			log.Printf("Warning: failed to write token audit event: %v", err)
		}
	}
}

func auditActor(source, owner string) string {
	if source == "cli" {
		return "operator"
	}
	return "owner:" + owner
}

// CleanupOwnerTokenData applies the owner idle-purge policy. Airports has no
// owner-owned resource tables, so all owners older than the idle threshold
// have no resources and can be removed with their tokens.
func CleanupOwnerTokenData(db dbpkg.Handle, idlePurgeDays int, now time.Time) (resultErr error) {
	if err := ensureOwnerTokenSchema(db); err != nil {
		return err
	}
	if idlePurgeDays <= 0 {
		return nil
	}
	cutoff := now.Add(-time.Duration(idlePurgeDays) * 24 * time.Hour).Unix()
	return dbpkg.Transaction(context.Background(), db, func(tx *dbpkg.TransactionHandle) error {
		if _, err := tx.Exec(`DELETE FROM api_tokens WHERE owner_id IN (SELECT id FROM owners WHERE last_seen_at < ?)`, cutoff); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM owners WHERE last_seen_at < ?`, cutoff); err != nil {
			return err
		}
		return nil
	})
}

// validateOwnerToken validates a raw owner Bearer token and refreshes its use
// timestamps only when they are at least five minutes old. It is shared by MCP and API auth.
func (s *Server) validateOwnerToken(raw string) (ownerID string, kind string, ok bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) != 36 || !strings.HasPrefix(raw, "own_") || s.ensureOwnerTokenSchema() != nil {
		return "", "", false
	}
	hash := tokenHash(raw)
	var identity ownerTokenIdentity
	var expires sql.NullInt64
	var revoked sql.NullInt64
	err := s.db.QueryRow(`SELECT id, owner_id, kind, token_prefix, expires_at, revoked_at
		FROM api_tokens WHERE token_hash = ?`, hash).
		Scan(&identity.ID, &identity.Owner, &identity.Kind, &identity.Prefix, &expires, &revoked)
	if err != nil || revoked.Valid || (expires.Valid && expires.Int64 <= time.Now().Unix()) {
		return "", "", false
	}
	now := time.Now().Unix()
	_, _ = s.db.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE id = ? AND (last_used_at IS NULL OR last_used_at <= ?)`, now, identity.ID, now-300)
	_, _ = s.db.Exec(`UPDATE owners SET last_seen_at = ? WHERE id = ? AND last_seen_at <= ?`, now, identity.Owner, now-300)
	return identity.Owner, identity.Kind, true
}

func (s *Server) ownerTokenFromBearer(r *http.Request, primaryOnly bool) (*ownerTokenIdentity, bool) {
	raw := bearerCredential(r)
	operator := strings.TrimSpace(s.config.Server.Token)
	if operator != "" && len(raw) == len(operator) && subtle.ConstantTimeCompare([]byte(raw), []byte(operator)) == 1 {
		return &ownerTokenIdentity{Kind: "operator"}, true
	}
	ownerID, kind, ok := s.validateOwnerToken(raw)
	if !ok {
		return nil, false
	}
	var identity ownerTokenIdentity
	identity.Owner = ownerID
	identity.Kind = kind
	identity.Prefix = tokenDisplayPrefix(raw)
	if err := s.db.QueryRow(`SELECT id, token_prefix FROM api_tokens WHERE token_hash = ?`, tokenHash(raw)).Scan(&identity.ID, &identity.Prefix); err != nil {
		return nil, false
	}
	_ = primaryOnly
	return &identity, true
}

func (s *Server) rateLimitBearerIdentity(r *http.Request) string {
	path := r.URL.Path
	if path != "/api" && !strings.HasPrefix(path, "/api/") && path != "/mcp" && !strings.HasPrefix(path, "/mcp/") {
		return ""
	}
	raw := bearerCredential(r)
	if raw == "" || !s.validAPIBearer(r) {
		return ""
	}
	return "token:" + tokenHash(raw)
}

func (s *Server) ownerTokenFromCookie(r *http.Request) (*ownerTokenIdentity, string, bool) {
	cookie, err := r.Cookie(ownerTokenCookieName)
	if err != nil || cookie.Value == "" {
		return nil, "", false
	}
	ownerID, kind, ok := s.validateOwnerToken(cookie.Value)
	if !ok {
		return nil, cookie.Value, false
	}
	var identity ownerTokenIdentity
	identity.Owner, identity.Kind = ownerID, kind
	if err := s.db.QueryRow(`SELECT id, token_prefix FROM api_tokens WHERE token_hash = ?`, tokenHash(cookie.Value)).Scan(&identity.ID, &identity.Prefix); err != nil {
		return nil, cookie.Value, false
	}
	return &identity, cookie.Value, identity.ID > 0
}

func (s *Server) issueOwnerToken() (ownerResult, tokenResult string, resultErr error) {
	if err := s.ensureOwnerTokenSchema(); err != nil {
		return "", "", err
	}
	ownerID, err := randomHexID(16)
	if err != nil {
		return "", "", err
	}
	token, err := newOwnerTokenValue()
	if err != nil {
		return "", "", err
	}
	ciphertext, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", "", err
	}
	err = dbpkg.Transaction(context.Background(), s.db, func(tx *dbpkg.TransactionHandle) error {
		if _, err := tx.Exec(`INSERT INTO owners (id) VALUES (?)`, ownerID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO api_tokens (owner_id, kind, token_hash, token_prefix, token_suffix, token_enc) VALUES (?, 'primary', ?, ?, ?, ?)`, ownerID, tokenHash(token), tokenDisplayPrefix(token), tokenDisplaySuffix(token), ciphertext); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return ownerID, token, nil
}

func (s *Server) encryptOwnerToken(token string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s.config.Server.Security.EncryptionKey))
	if err != nil || len(key) != 32 {
		return "", errors.New("owner token encryption key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Server) decryptOwnerTokenWithFallback(encoded string, currentKey []byte) ([]byte, bool, error) {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, false, errors.New("owner token ciphertext is invalid")
	}
	plain, err := decryptOwnerTokenCiphertext(currentKey, sealed)
	if err == nil {
		return plain, false, nil
	}
	previousKey, err := s.previousEncryptionKey(currentKey)
	if err != nil {
		return nil, false, errors.New("owner token ciphertext is invalid")
	}
	plain, err = decryptOwnerTokenCiphertext(previousKey, sealed)
	if err != nil {
		return nil, false, errors.New("owner token ciphertext is invalid")
	}
	return plain, true, nil
}

func decryptOwnerTokenCiphertext(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(sealed) < gcm.NonceSize() {
		return nil, errors.New("owner token ciphertext is invalid")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *Server) previousEncryptionKey(currentKey []byte) ([]byte, error) {
	if s.db == nil {
		return nil, errors.New("previous encryption key is unavailable")
	}
	var wrapped []byte
	if err := s.db.QueryRow(`SELECT value FROM app_secrets WHERE name = 'encryption_key_previous'`).Scan(&wrapped); err != nil {
		return nil, err
	}
	wrapped = dbpkg.DecodeAppSecret("encryption_key_previous", wrapped)
	var expires string
	if err := s.db.QueryRow(`SELECT CAST(value AS TEXT) FROM app_secrets WHERE name = 'encryption_key_previous_until'`).Scan(&expires); err != nil {
		return nil, err
	}
	until, err := time.Parse(time.RFC3339, expires)
	if err != nil || !time.Now().Before(until) {
		return nil, errors.New("previous encryption key grace period expired")
	}
	encoded := strings.TrimSpace(string(wrapped))
	if !strings.HasPrefix(encoded, encryptionKeyPreviousPrefix) {
		return nil, errors.New("previous encryption key envelope has an unsupported format")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encryptionKeyPreviousPrefix))
	if err != nil {
		return nil, err
	}
	return decryptOwnerTokenCiphertext(currentKey, ciphertext)
}

func (s *Server) decryptOwnerTokenRow(id int64, encoded string) (string, error) {
	currentKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s.config.Server.Security.EncryptionKey))
	if err != nil || len(currentKey) != 32 {
		return "", errors.New("owner token encryption key is invalid")
	}
	plain, usedPrevious, err := s.decryptOwnerTokenWithFallback(encoded, currentKey)
	if err != nil {
		s.logOwnerTokenDecryptFailure(id)
		return "", err
	}
	if usedPrevious {
		reencrypted, err := s.encryptOwnerToken(string(plain))
		if err != nil {
			return "", err
		}
		result, err := s.db.Exec(`UPDATE api_tokens SET token_enc = ? WHERE id = ? AND token_enc = ?`, reencrypted, id, encoded)
		if err != nil {
			return "", err
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return "", errors.New("owner token changed during encryption-key recovery")
		}
	}
	return string(plain), nil
}

func (s *Server) logOwnerTokenDecryptFailure(id int64) {
	if s.securityLog == nil {
		return
	}
	if err := s.securityLog.Write(fmt.Sprintf("security.decrypt_failed table=api_tokens row=%d", id)); err != nil {
		log.Printf("Warning: failed to write token decrypt failure event: %v", err)
	}
}

func (s *Server) createLinkedOwnerToken(owner string, name, expiry string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(expiry) == "custom" {
		return "", errOwnerTokenInput
	}
	if !validOwnerTokenName(name) {
		return "", errOwnerTokenInput
	}
	expiresAt, err := parseOwnerTokenExpiry(expiry)
	if err != nil {
		return "", errOwnerTokenInput
	}
	if err := s.ensureOwnerTokenSchema(); err != nil {
		return "", err
	}
	limit := s.config.Server.Tokens.MaxLinked
	if limit < 1 {
		limit = 25
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`, owner, time.Now().Unix()).Scan(&count); err != nil {
		return "", err
	}
	if count >= limit {
		return "", errOwnerTokenInput
	}
	var duplicate int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND name = ? AND revoked_at IS NULL`, owner, name).Scan(&duplicate); err != nil {
		return "", err
	}
	if duplicate != 0 {
		return "", errOwnerTokenInput
	}
	token, err := newOwnerTokenValue()
	if err != nil {
		return "", err
	}
	ciphertext, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", err
	}
	var expires interface{}
	if expiresAt != nil {
		expires = *expiresAt
	}
	if _, err := s.db.Exec(`INSERT INTO api_tokens (owner_id, kind, name, token_hash, token_prefix, token_suffix, token_enc, expires_at) VALUES (?, 'linked', ?, ?, ?, ?, ?, ?)`, owner, name, tokenHash(token), tokenDisplayPrefix(token), tokenDisplaySuffix(token), ciphertext, expires); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) ownerTokenRecords(owner string) (records []ownerTokenRecord, resultErr error) {
	if err := s.ensureOwnerTokenSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, kind, COALESCE(name,''), token_prefix, token_suffix, created_at, last_used_at, expires_at, revoked_at
		FROM api_tokens WHERE owner_id = ? ORDER BY CASE kind WHEN 'primary' THEN 0 ELSE 1 END, created_at DESC, id DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close owner token records: %w", err))
		}
	}()
	records = make([]ownerTokenRecord, 0)
	now := time.Now().Unix()
	for rows.Next() {
		var item ownerTokenRecord
		var lastUsed, expires, revoked sql.NullInt64
		var suffix string
		if err := rows.Scan(&item.ID, &item.Kind, &item.Name, &item.Prefix, &suffix, &item.CreatedAt, &lastUsed, &expires, &revoked); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			value := lastUsed.Int64
			item.LastUsedAt = &value
			item.LastUsedLabel = time.Unix(value, 0).UTC().Format(time.RFC3339)
		}
		if expires.Valid {
			value := expires.Int64
			item.ExpiresAt = &value
			item.ExpiresLabel = time.Unix(value, 0).UTC().Format(time.RFC3339)
			item.ExpiryChoice = "custom"
			item.ExpiresOnInput = time.Unix(value, 0).UTC().Format("2006-01-02")
		} else {
			item.ExpiryChoice = "never"
		}
		item.CreatedLabel = time.Unix(item.CreatedAt, 0).UTC().Format(time.RFC3339)
		switch {
		case revoked.Valid:
			item.Status = "revoked"
		case expires.Valid && expires.Int64 <= now:
			item.Status = "expired"
		default:
			item.Status = "active"
		}
		item.Masked = maskOwnerTokenSuffix(suffix)
		records = append(records, item)
	}
	return records, rows.Err()
}

func (s *Server) ownerTokenByID(owner string, id int64) (kind, ciphertext string, err error) {
	err = s.db.QueryRow(`SELECT kind, token_enc FROM api_tokens WHERE id = ? AND owner_id = ?`, id, owner).Scan(&kind, &ciphertext)
	return
}

func (s *Server) ownerTokenIDByRaw(raw string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM api_tokens WHERE token_hash = ?`, tokenHash(raw)).Scan(&id)
	return id, err
}

func (s *Server) revealOwnerToken(owner string, id int64, expectedKind string) (string, error) {
	kind, ciphertext, err := s.ownerTokenByID(owner, id)
	if err != nil || (expectedKind != "" && kind != expectedKind) {
		return "", errOwnerTokenUnauthorized
	}
	token, err := s.decryptOwnerTokenRow(id, ciphertext)
	if err != nil {
		return "", err
	}
	var prefix string
	_ = s.db.QueryRow(`SELECT token_prefix FROM api_tokens WHERE id = ? AND owner_id = ?`, id, owner).Scan(&prefix)
	return token, nil
}

func (s *Server) rotateOwnerToken(owner string, id int64, expectedKind string) (string, error) {
	kind, _, err := s.ownerTokenByID(owner, id)
	if err != nil || kind != expectedKind {
		return "", errOwnerTokenUnauthorized
	}
	token, err := newOwnerTokenValue()
	if err != nil {
		return "", err
	}
	ciphertext, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", err
	}
	result, err := s.db.Exec(`UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_suffix = ?, token_enc = ?, rotated_at = ?
		WHERE id = ? AND owner_id = ? AND kind = ? AND revoked_at IS NULL`, tokenHash(token), tokenDisplayPrefix(token), tokenDisplaySuffix(token), ciphertext, time.Now().Unix(), id, owner, expectedKind)
	if err != nil {
		return "", err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return "", errOwnerTokenUnauthorized
	}
	return token, nil
}

func (s *Server) updateLinkedOwnerToken(owner string, id int64, name, expiry string) error {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(expiry) == "custom" {
		return errOwnerTokenInput
	}
	if !validOwnerTokenName(name) {
		return errOwnerTokenInput
	}
	expiresAt, err := parseOwnerTokenExpiry(expiry)
	if err != nil {
		return errOwnerTokenInput
	}
	var duplicate int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND name = ? AND id != ? AND revoked_at IS NULL`, owner, name, id).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate != 0 {
		return errOwnerTokenInput
	}
	var expires interface{}
	if expiresAt != nil {
		expires = *expiresAt
	}
	result, err := s.db.Exec(`UPDATE api_tokens SET name = ?, expires_at = ? WHERE id = ? AND owner_id = ? AND kind = 'linked' AND revoked_at IS NULL`, name, expires, id, owner)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return errOwnerTokenUnauthorized
	}
	return nil
}

func (s *Server) revokeLinkedOwnerToken(owner string, id int64) (string, error) {
	var prefix string
	if err := s.db.QueryRow(`SELECT token_prefix FROM api_tokens WHERE id = ? AND owner_id = ? AND kind = 'linked' AND revoked_at IS NULL`, id, owner).Scan(&prefix); err != nil {
		return "", errOwnerTokenUnauthorized
	}
	result, err := s.db.Exec(`UPDATE api_tokens SET revoked_at = ?, revoked_reason = 'owner_revoked' WHERE id = ? AND owner_id = ? AND kind = 'linked' AND revoked_at IS NULL`, time.Now().Unix(), id, owner)
	if err != nil {
		return "", err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return "", errOwnerTokenUnauthorized
	}
	return prefix, nil
}

func (s *Server) setOwnerCookie(w http.ResponseWriter, r *http.Request, raw string) {
	maxAge := ownerCookieMaxAge(s.config.Server.Tokens.CookieMaxAge)
	replaceOwnerCookie(w, &http.Cookie{Name: ownerTokenCookieName, Value: raw, Path: "/", MaxAge: maxAge, Expires: time.Now().Add(time.Duration(maxAge) * time.Second), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func (s *Server) clearOwnerCookie(w http.ResponseWriter, r *http.Request) {
	replaceOwnerCookie(w, &http.Cookie{Name: ownerTokenCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func (s *Server) setOwnerTokenViewCookie(w http.ResponseWriter, card string, clear bool) {
	cookie := &http.Cookie{Name: ownerTokenViewCookieName, Value: card, Path: "/server/preferences/tokens", MaxAge: 60, Expires: time.Now().Add(60 * time.Second), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	if clear {
		cookie.Value = ""
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}

func (s *Server) setOwnerTokenNoticeCookie(w http.ResponseWriter, notice string, clear bool) {
	cookie := &http.Cookie{Name: ownerTokenNoticeCookieName, Value: notice, Path: "/server/preferences/tokens", MaxAge: 60, Expires: time.Now().Add(60 * time.Second), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	if clear {
		cookie.Value = ""
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}

func replaceOwnerCookie(w http.ResponseWriter, cookie *http.Cookie) {
	values := w.Header().Values("Set-Cookie")
	w.Header().Del("Set-Cookie")
	for _, value := range values {
		if !strings.HasPrefix(value, ownerTokenCookieName+"=") {
			w.Header().Add("Set-Cookie", value)
		}
	}
	http.SetCookie(w, cookie)
}

// ownerTokenCookieLifecycle refreshes valid owner cookies and clears stale
// credentials on every web response without using cookies for API auth.
func (s *Server) ownerTokenCookieLifecycle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiBase := strings.TrimSuffix(s.APIBasePath(), "/")
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") || r.URL.Path == apiBase || strings.HasPrefix(r.URL.Path, apiBase+"/") {
			next.ServeHTTP(w, r)
			return
		}
		writer := &ownerTokenLifecycleWriter{ResponseWriter: w, server: s, request: r}
		next.ServeHTTP(writer, r)
	})
}

type ownerTokenLifecycleWriter struct {
	http.ResponseWriter
	server  *Server
	request *http.Request
	done    bool
}

func (w *ownerTokenLifecycleWriter) apply(status int) {
	if w.done {
		return
	}
	w.done = true
	if _, err := w.request.Cookie(ownerTokenCookieName); err != nil || hasOwnerCookieResponse(w.Header()) {
		return
	}
	cookie, _ := w.request.Cookie(ownerTokenCookieName)
	_, kind, valid := w.server.validateOwnerToken(cookie.Value)
	if valid && kind == "primary" {
		if status != http.StatusTooManyRequests {
			w.server.setOwnerCookie(w, w.request, cookie.Value)
		}
		return
	}
	w.server.clearOwnerCookie(w, w.request)
}

func hasOwnerCookieResponse(header http.Header) bool {
	for _, value := range header.Values("Set-Cookie") {
		if strings.HasPrefix(value, ownerTokenCookieName+"=") {
			return true
		}
	}
	return false
}

func (w *ownerTokenLifecycleWriter) WriteHeader(status int) {
	w.apply(status)
	w.ResponseWriter.WriteHeader(status)
}

func (w *ownerTokenLifecycleWriter) Write(data []byte) (int, error) {
	w.apply(http.StatusOK)
	return w.ResponseWriter.Write(data)
}

func (w *ownerTokenLifecycleWriter) Flush() {
	w.apply(http.StatusOK)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *ownerTokenLifecycleWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.apply(http.StatusSwitchingProtocols)
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *ownerTokenLifecycleWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *ownerTokenLifecycleWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func ownerCookieMaxAge(value string) int {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "y") {
		years, err := strconv.Atoi(strings.TrimSuffix(value, "y"))
		if err == nil && years > 0 && years <= 10 {
			return years * 365 * 24 * 60 * 60
		}
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err == nil && days > 0 && days <= 3650 {
			return days * 24 * 60 * 60
		}
	}
	if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
		return int(duration.Seconds())
	}
	return 365 * 24 * 60 * 60
}

func parseOwnerTokenExpiry(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "", "never":
		return nil, nil
	case "7d":
		return int64Pointer(time.Now().Add(7 * 24 * time.Hour).Unix()), nil
	case "30d":
		return int64Pointer(time.Now().Add(30 * 24 * time.Hour).Unix()), nil
	case "90d":
		return int64Pointer(time.Now().Add(90 * 24 * time.Hour).Unix()), nil
	case "1y":
		return int64Pointer(time.Now().Add(365 * 24 * time.Hour).Unix()), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Before(time.Now().Add(-24*time.Hour)) {
		return nil, errOwnerTokenInput
	}
	return int64Pointer(parsed.Add(24*time.Hour - time.Second).Unix()), nil
}

func int64Pointer(value int64) *int64 { return &value }

func validOwnerTokenName(value string) bool {
	count := 0
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
		count++
	}
	return count > 0 && count <= 64
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func tokenDisplayPrefix(raw string) string {
	if len(raw) > 12 {
		return raw[:12]
	}
	return raw
}

func tokenDisplaySuffix(raw string) string {
	if len(raw) < 4 {
		return ""
	}
	return raw[len(raw)-4:]
}

func maskOwnerTokenSuffix(suffix string) string {
	if len(suffix) != 4 {
		return "own_••••••••••••••••••••••••"
	}
	return "own_••••••••••••••••••••••••" + suffix
}

func newOwnerTokenValue() (string, error) {
	const prefix = "own_"
	var value strings.Builder
	value.Grow(32)
	for i := 0; i < 32; i++ {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(ownerTokenAlphabet))))
		if err != nil {
			return "", err
		}
		value.WriteByte(ownerTokenAlphabet[index.Int64()])
	}
	return prefix + value.String(), nil
}

func randomHexID(length int) (string, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *Server) logOwnerTokenEvent(event, owner, prefix, source, actorIP string, detailsSource ...string) {
	line := fmt.Sprintf("event=token.%s owner_id=%q token_prefix=%q actor=%q source=%q ip=%q", event, owner, prefix, auditActor(source, owner), source, actorIP)
	if len(detailsSource) != 0 && detailsSource[0] != "" {
		line += fmt.Sprintf(" details_source=%q", detailsSource[0])
	}
	writeOwnerTokenAuditLine(s.auditLogger, line)
}

func (s *Server) enforceOwnerTokenStrict(w http.ResponseWriter, r *http.Request) bool {
	if s.rateLimiter == nil {
		return true
	}
	return s.enforceOwnerTokenBucket(w, r, "strict", s.rateLimiter.strict)
}

func (s *Server) enforceOwnerTokenStrictForOwner(w http.ResponseWriter, r *http.Request, owner string) bool {
	if s.rateLimiter == nil {
		return true
	}
	return s.enforceOwnerTokenBucketIdentity(w, r, "strict", s.rateLimiter.strict, "owner:"+owner)
}

func (s *Server) enforceOwnerTokenStrictManagement(w http.ResponseWriter, r *http.Request) bool {
	var identity *ownerTokenIdentity
	var ok bool
	if strings.HasPrefix(r.URL.Path, "/api/") {
		identity, ok = s.ownerTokenFromBearer(r, false)
	} else {
		identity, _, ok = s.ownerTokenFromCookie(r)
	}
	if ok && identity.Owner != "" {
		return s.enforceOwnerTokenStrictForOwner(w, r, identity.Owner)
	}
	return s.enforceOwnerTokenStrict(w, r)
}

func (s *Server) enforceOwnerTokenAuthFailure(w http.ResponseWriter, r *http.Request) bool {
	if s.rateLimiter == nil {
		return false
	}
	if bearerCredential(r) == "" {
		cookie, err := r.Cookie(ownerTokenCookieName)
		if err != nil || cookie.Value == "" {
			return false
		}
	}
	return !s.enforceOwnerTokenBucket(w, r, "auth_failures", s.rateLimiter.authFailures)
}

func (s *Server) enforceOwnerTokenBucket(w http.ResponseWriter, r *http.Request, class string, bucket config.RateLimitBucket) bool {
	return s.enforceOwnerTokenBucketIdentity(w, r, class, bucket, realIP(r))
}

func (s *Server) enforceOwnerTokenBucketIdentity(w http.ResponseWriter, r *http.Request, class string, bucket config.RateLimitBucket, identity string) bool {
	limiter := s.rateLimiter
	if limiter == nil || IsAllowlisted(r.Context()) {
		return true
	}
	allowed, limit, _, reset, limitType := limiter.allowConfiguredBucket(identity, class, bucket, time.Now())
	if limiter.metrics != nil {
		limiter.metrics.recordRateLimit(limitType, allowed)
	}
	if allowed {
		return true
	}
	seconds := int(time.Until(reset).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
	s.writeErrorJSON(w, r, http.StatusTooManyRequests, "RATE_LIMITED", i18n.TranslateText(i18n.FromContext(r.Context()), "errors.rate_limited"))
	return false
}

func decodeOwnerTokenForm(r *http.Request, target interface{}) error {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		return decoder.Decode(target)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	values, ok := target.(*ownerTokenInput)
	if !ok {
		return errOwnerTokenInput
	}
	values.Name = r.FormValue("name")
	values.Expiry = r.FormValue("expiry")
	values.ExpiresOn = r.FormValue("expires_on")
	values.Token = r.FormValue("token")
	values.Confirm = r.FormValue("confirm")
	return nil
}

type ownerTokenInput struct {
	Name      string `json:"name"`
	Expiry    string `json:"expiry"`
	ExpiresOn string `json:"expires_on"`
	Token     string `json:"token"`
	Confirm   string `json:"confirm"`
}

func (input ownerTokenInput) expiryValue() string {
	if strings.TrimSpace(input.Expiry) == "custom" {
		return strings.TrimSpace(input.ExpiresOn)
	}
	return input.Expiry
}

func (s *Server) handleOwnerTokenIssueAPI(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrict(w, r) {
		return
	}
	if bearerCredential(r) != "" && !s.validAPIBearer(r) && s.enforceOwnerTokenAuthFailure(w, r) {
		return
	}
	if s.validAPIBearer(r) {
		s.writeErrorJSON(w, r, http.StatusConflict, "CONFLICT", "This caller already has a token identity.")
		return
	}
	ownerID, token, err := s.issueOwnerToken()
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", i18n.TranslateText(i18n.FromContext(r.Context()), "errors.internal"))
		return
	}
	prefix := tokenDisplayPrefix(token)
	s.logOwnerTokenEvent("created", ownerID, prefix, "api", realIP(r), "issue")
	w.Header().Set("Cache-Control", "no-store")
	s.respondItem(w, http.StatusCreated, map[string]string{"owner_token": token})
}

func (s *Server) handleOwnerTokenListAPI(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	items, err := s.ownerTokenRecords(identity.Owner)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", i18n.TranslateText(i18n.FromContext(r.Context()), "errors.internal"))
		return
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{"tokens": items})
}

func (s *Server) ownerTokenUnauthorized(w http.ResponseWriter, r *http.Request) {
	s.logRevokedCLIToken(r)
	w.Header().Set("WWW-Authenticate", `Bearer realm="airports", error="invalid_token"`)
	s.writeErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "A valid primary owner token is required.")
}

func (s *Server) logRevokedCLIToken(r *http.Request) {
	if r == nil || !strings.HasPrefix(r.UserAgent(), "airports-cli/") {
		return
	}
	if prefix, revoked := s.revokedOwnerTokenPrefix(bearerCredential(r)); revoked {
		s.writeCLIAudit("cli.token_revoked_detected", "token_prefix="+strconv.Quote(prefix)+" client_ip="+strconv.Quote(realIP(r)))
	}
}

func (s *Server) revokedOwnerTokenPrefix(token string) (string, bool) {
	if s == nil || s.db == nil || !strings.HasPrefix(token, "own_") {
		return "", false
	}
	var prefix string
	var revokedAt sql.NullInt64
	if err := s.db.QueryRow("SELECT token_prefix, revoked_at FROM api_tokens WHERE token_hash = ?", tokenHash(token)).Scan(&prefix, &revokedAt); err != nil {
		return "", false
	}
	return prefix, revokedAt.Valid && prefix != ""
}

func (s *Server) ownerTokenForbidden(w http.ResponseWriter, r *http.Request) {
	s.writeErrorJSON(w, r, http.StatusForbidden, "FORBIDDEN", "A primary owner token is required for token management.")
}

func (s *Server) ownerTokenBadInput(w http.ResponseWriter, r *http.Request) {
	s.writeErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The token management request is invalid.")
}

func (s *Server) handleOwnerTokenPrimaryRevealAPI(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	token, err := s.revealOwnerToken(identity.Owner, identity.ID, "primary")
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Token not found.")
		return
	}
	s.logOwnerTokenEvent("revealed", identity.Owner, identity.Prefix, "api", realIP(r))
	w.Header().Set("Cache-Control", "no-store")
	s.respondItem(w, http.StatusOK, map[string]string{"owner_token": token})
}

func (s *Server) handleOwnerTokenPrimaryRotateAPI(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	token, err := s.rotateOwnerToken(identity.Owner, identity.ID, "primary")
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to rotate token.")
		return
	}
	s.logOwnerTokenEvent("rotated", identity.Owner, tokenDisplayPrefix(token), "api", realIP(r))
	w.Header().Set("Cache-Control", "no-store")
	s.respondItem(w, http.StatusOK, map[string]string{"owner_token": token})
}

func (s *Server) handleOwnerTokenPrimaryRevokeAPI(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	var input ownerTokenInput
	if err := decodeOwnerTokenForm(r, &input); err != nil {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "The confirmation value is invalid.")
		return
	}
	_, _, _, last4, err := ownerTokenRevocationDetails(s.db, identity.Owner)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to revoke owner token.")
		return
	}
	if len(input.Confirm) != len(last4) || len(last4) != 4 || subtle.ConstantTimeCompare([]byte(input.Confirm), []byte(last4)) != 1 {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "The confirmation value is invalid.")
		return
	}
	tokenCount, resourceCount, err := deleteOwnerTokenIdentity(s.db, identity.Owner)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to revoke owner token.")
		return
	}
	writeOwnerTokenAuditEntry(s.auditLogger, "revoked", identity.Owner, identity.Prefix, "owner", tokenCount, resourceCount, "api", auditActor("api", identity.Owner), realIP(r))
	s.respondItem(w, http.StatusOK, map[string]int{"tokens": tokenCount, "resources": resourceCount})
}

func (s *Server) handleOwnerTokenCreateLinkedAPI(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	var input ownerTokenInput
	if err := decodeOwnerTokenForm(r, &input); err != nil {
		s.ownerTokenBadInput(w, r)
		return
	}
	token, err := s.createLinkedOwnerToken(identity.Owner, input.Name, input.expiryValue())
	if err != nil {
		if errors.Is(err, errOwnerTokenInput) {
			s.ownerTokenBadInput(w, r)
			return
		}
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create linked token.")
		return
	}
	s.logOwnerTokenEvent("created", identity.Owner, tokenDisplayPrefix(token), "api", realIP(r))
	w.Header().Set("Cache-Control", "no-store")
	s.respondItem(w, http.StatusCreated, map[string]string{"owner_token": token})
}

func (s *Server) handleOwnerTokenLinkedActionAPI(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/reveal") || strings.HasSuffix(r.URL.Path, "/rotate") {
		if !s.enforceOwnerTokenStrictManagement(w, r) {
			return
		}
	}
	identity, ok := s.ownerTokenFromBearer(r, true)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.ownerTokenUnauthorized(w, r)
		return
	}
	if identity.Kind != "primary" {
		s.ownerTokenForbidden(w, r)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		s.ownerTokenBadInput(w, r)
		return
	}
	var input ownerTokenInput
	switch r.Method {
	case http.MethodPatch, http.MethodPut, http.MethodPost:
		if r.Body != nil && r.ContentLength != 0 {
			if err := decodeOwnerTokenForm(r, &input); err != nil {
				s.ownerTokenBadInput(w, r)
				return
			}
		}
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/reveal"):
		token, err := s.revealOwnerToken(identity.Owner, id, "linked")
		if err != nil {
			s.writeErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Token not found.")
			return
		}
		s.logOwnerTokenEvent("revealed", identity.Owner, tokenDisplayPrefix(token), "api", realIP(r))
		w.Header().Set("Cache-Control", "no-store")
		s.respondItem(w, http.StatusOK, map[string]string{"owner_token": token})
	case strings.HasSuffix(r.URL.Path, "/rotate"):
		token, err := s.rotateOwnerToken(identity.Owner, id, "linked")
		if err != nil {
			s.writeErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Token not found.")
			return
		}
		s.logOwnerTokenEvent("rotated", identity.Owner, tokenDisplayPrefix(token), "api", realIP(r))
		w.Header().Set("Cache-Control", "no-store")
		s.respondItem(w, http.StatusOK, map[string]string{"owner_token": token})
	case r.Method == http.MethodPatch || strings.HasSuffix(r.URL.Path, "/update"):
		if err := s.updateLinkedOwnerToken(identity.Owner, id, input.Name, input.expiryValue()); err != nil {
			if errors.Is(err, errOwnerTokenInput) {
				s.ownerTokenBadInput(w, r)
				return
			}
			s.writeErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Token not found.")
			return
		}
		s.respondItem(w, http.StatusOK, map[string]bool{"updated": true})
	case r.Method == http.MethodDelete || strings.HasSuffix(r.URL.Path, "/revoke"):
		prefix, err := s.revokeLinkedOwnerToken(identity.Owner, id)
		if err != nil {
			s.writeErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Token not found.")
			return
		}
		writeOwnerTokenAuditEntry(s.auditLogger, "revoked", identity.Owner, prefix, "token", 1, 0, "api", auditActor("api", identity.Owner), realIP(r))
		s.respondItem(w, http.StatusOK, map[string]bool{"revoked": true})
	default:
		s.writeErrorJSON(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Unsupported token action.")
	}
}

func (s *Server) handleOwnerTokenIssueWeb(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrict(w, r) {
		return
	}
	if bearerCredential(r) != "" && !s.validAPIBearer(r) && s.enforceOwnerTokenAuthFailure(w, r) {
		return
	}
	if s.validAPIBearer(r) {
		s.renderErrorPage(w, r, http.StatusConflict)
		return
	}
	if _, _, ok := s.ownerTokenFromCookie(r); ok {
		s.renderErrorPage(w, r, http.StatusConflict)
		return
	}
	ownerID, token, err := s.issueOwnerToken()
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	s.logOwnerTokenEvent("created", ownerID, tokenDisplayPrefix(token), "web", realIP(r), "issue")
	s.setOwnerCookie(w, r, token)
	s.setOwnerTokenViewCookie(w, "primary", false)
	s.setOwnerTokenNoticeCookie(w, "created", false)
	http.Redirect(w, r, "/server/preferences/tokens#primary", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenRestoreWeb(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrict(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderErrorPage(w, r, http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(r.FormValue("token"))
	ownerID, kind, ok := s.validateOwnerToken(token)
	if !ok || kind != "primary" {
		if s.rateLimiter != nil && !s.enforceOwnerTokenBucket(w, r, "auth_failures", s.rateLimiter.authFailures) {
			return
		}
		s.ownerTokenBadInput(w, r)
		return
	}
	s.setOwnerCookie(w, r, token)
	s.logOwnerTokenEvent("restored", ownerID, tokenDisplayPrefix(token), "web", realIP(r))
	http.Redirect(w, r, "/server/preferences/tokens#primary", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenForgetWeb(w http.ResponseWriter, r *http.Request) {
	s.clearOwnerCookie(w, r)
	w.Header().Set("Clear-Site-Data", `"storage"`)
	http.Redirect(w, r, "/server/preferences/tokens", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenPrimaryRevealWeb(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, token, ok := s.ownerTokenFromCookie(r)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.renderErrorPage(w, r, http.StatusUnauthorized)
		return
	}
	if identity.Kind != "primary" {
		s.renderErrorPage(w, r, http.StatusForbidden)
		return
	}
	s.setOwnerCookie(w, r, token)
	kind, _, err := s.ownerTokenByID(identity.Owner, identity.ID)
	if err != nil || kind != "primary" {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}
	s.logOwnerTokenEvent("revealed", identity.Owner, identity.Prefix, "web", realIP(r))
	s.setOwnerTokenViewCookie(w, "primary", false)
	s.setOwnerTokenNoticeCookie(w, "revealed", false)
	http.Redirect(w, r, "/server/preferences/tokens#primary", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenPrimaryRotateWeb(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, _, ok := s.ownerTokenFromCookie(r)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.renderErrorPage(w, r, http.StatusUnauthorized)
		return
	}
	if identity.Kind != "primary" {
		s.renderErrorPage(w, r, http.StatusForbidden)
		return
	}
	token, err := s.rotateOwnerToken(identity.Owner, identity.ID, "primary")
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	s.logOwnerTokenEvent("rotated", identity.Owner, tokenDisplayPrefix(token), "web", realIP(r))
	s.setOwnerCookie(w, r, token)
	s.setOwnerTokenViewCookie(w, "primary", false)
	s.setOwnerTokenNoticeCookie(w, "rotated", false)
	http.Redirect(w, r, "/server/preferences/tokens#primary", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenPrimaryRevokeWeb(w http.ResponseWriter, r *http.Request) {
	if !s.enforceOwnerTokenStrictManagement(w, r) {
		return
	}
	identity, _, ok := s.ownerTokenFromCookie(r)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.renderErrorPage(w, r, http.StatusUnauthorized)
		return
	}
	if identity.Kind != "primary" {
		s.renderErrorPage(w, r, http.StatusForbidden)
		return
	}
	var input ownerTokenInput
	if err := decodeOwnerTokenForm(r, &input); err != nil {
		s.renderErrorPage(w, r, http.StatusBadRequest)
		return
	}
	_, _, _, last4, err := ownerTokenRevocationDetails(s.db, identity.Owner)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	if len(input.Confirm) != len(last4) || len(last4) != 4 || subtle.ConstantTimeCompare([]byte(input.Confirm), []byte(last4)) != 1 {
		s.renderErrorPage(w, r, http.StatusBadRequest)
		return
	}
	tokenCount, resourceCount, err := deleteOwnerTokenIdentity(s.db, identity.Owner)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	writeOwnerTokenAuditEntry(s.auditLogger, "revoked", identity.Owner, identity.Prefix, "owner", tokenCount, resourceCount, "web", auditActor("web", identity.Owner), realIP(r))
	s.clearOwnerCookie(w, r)
	s.setOwnerTokenNoticeCookie(w, "revoked", false)
	w.Header().Set("Clear-Site-Data", `"storage"`)
	http.Redirect(w, r, "/server/preferences/tokens", http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenCreateLinkedWeb(w http.ResponseWriter, r *http.Request) {
	identity, primary, ok := s.ownerTokenFromCookie(r)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.renderErrorPage(w, r, http.StatusUnauthorized)
		return
	}
	if identity.Kind != "primary" {
		s.renderErrorPage(w, r, http.StatusForbidden)
		return
	}
	s.setOwnerCookie(w, r, primary)
	var input ownerTokenInput
	if err := decodeOwnerTokenForm(r, &input); err != nil {
		s.renderErrorPage(w, r, http.StatusBadRequest)
		return
	}
	token, err := s.createLinkedOwnerToken(identity.Owner, input.Name, input.expiryValue())
	if err != nil {
		if errors.Is(err, errOwnerTokenInput) {
			s.renderErrorPage(w, r, http.StatusBadRequest)
			return
		}
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	s.logOwnerTokenEvent("created", identity.Owner, tokenDisplayPrefix(token), "web", realIP(r))
	id, err := s.ownerTokenIDByRaw(token)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	s.setOwnerTokenViewCookie(w, strconv.FormatInt(id, 10), false)
	s.setOwnerTokenNoticeCookie(w, "created", false)
	http.Redirect(w, r, fmt.Sprintf("/server/preferences/tokens#token-%d", id), http.StatusSeeOther)
}

func (s *Server) handleOwnerTokenLinkedActionWeb(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/reveal") || strings.HasSuffix(r.URL.Path, "/rotate") {
		if !s.enforceOwnerTokenStrictManagement(w, r) {
			return
		}
	}
	identity, primary, ok := s.ownerTokenFromCookie(r)
	if !ok {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.renderErrorPage(w, r, http.StatusUnauthorized)
		return
	}
	if identity.Kind != "primary" {
		s.renderErrorPage(w, r, http.StatusForbidden)
		return
	}
	s.setOwnerCookie(w, r, primary)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		s.renderErrorPage(w, r, http.StatusBadRequest)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/reveal"):
		kind, _, err := s.ownerTokenByID(identity.Owner, id)
		if err != nil || kind != "linked" {
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
		var prefix string
		if err := s.db.QueryRow(`SELECT token_prefix FROM api_tokens WHERE id = ? AND owner_id = ?`, id, identity.Owner).Scan(&prefix); err != nil {
			s.renderErrorPage(w, r, http.StatusInternalServerError)
			return
		}
		s.logOwnerTokenEvent("revealed", identity.Owner, prefix, "web", realIP(r))
		s.setOwnerTokenViewCookie(w, strconv.FormatInt(id, 10), false)
		s.setOwnerTokenNoticeCookie(w, "revealed", false)
		http.Redirect(w, r, fmt.Sprintf("/server/preferences/tokens#token-%d", id), http.StatusSeeOther)
	case strings.HasSuffix(r.URL.Path, "/rotate"):
		token, err := s.rotateOwnerToken(identity.Owner, id, "linked")
		if err != nil {
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
		s.logOwnerTokenEvent("rotated", identity.Owner, tokenDisplayPrefix(token), "web", realIP(r))
		s.setOwnerTokenViewCookie(w, strconv.FormatInt(id, 10), false)
		s.setOwnerTokenNoticeCookie(w, "rotated", false)
		http.Redirect(w, r, fmt.Sprintf("/server/preferences/tokens#token-%d", id), http.StatusSeeOther)
	case strings.HasSuffix(r.URL.Path, "/update"):
		var input ownerTokenInput
		if err := decodeOwnerTokenForm(r, &input); err != nil {
			s.renderErrorPage(w, r, http.StatusBadRequest)
			return
		}
		if err := s.updateLinkedOwnerToken(identity.Owner, id, input.Name, input.expiryValue()); err != nil {
			if errors.Is(err, errOwnerTokenInput) {
				s.renderErrorPage(w, r, http.StatusBadRequest)
				return
			}
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
		s.setOwnerTokenNoticeCookie(w, "updated", false)
		http.Redirect(w, r, fmt.Sprintf("/server/preferences/tokens#token-%d", id), http.StatusSeeOther)
	case strings.HasSuffix(r.URL.Path, "/revoke"):
		prefix, err := s.revokeLinkedOwnerToken(identity.Owner, id)
		if err != nil {
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
		writeOwnerTokenAuditEntry(s.auditLogger, "revoked", identity.Owner, prefix, "token", 1, 0, "web", auditActor("web", identity.Owner), realIP(r))
		s.setOwnerTokenNoticeCookie(w, "revoked", false)
		http.Redirect(w, r, "/server/preferences/tokens#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	default:
		s.renderErrorPage(w, r, http.StatusMethodNotAllowed)
	}
}
