package server

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestOpenPGPV4Fingerprint(t *testing.T) {
	keyBody := []byte{4, 0, 0, 0, 0, 1}
	packet := append([]byte{0xc6, byte(len(keyBody))}, keyBody...)
	framed := append([]byte{0x99, 0, byte(len(keyBody))}, keyBody...)
	wantDigest := sha1.Sum(framed)
	want := strings.ToUpper(hex.EncodeToString(wantDigest[:]))

	got, err := openPGPV4Fingerprint(packet)
	if err != nil {
		t.Fatalf("openPGPV4Fingerprint(binary) error = %v", err)
	}
	if got != want {
		t.Fatalf("openPGPV4Fingerprint(binary) = %q, want %q", got, want)
	}

	armored := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n" + base64.StdEncoding.EncodeToString(packet) + "\n-----END PGP PUBLIC KEY BLOCK-----\n"
	got, err = openPGPV4Fingerprint([]byte(armored))
	if err != nil {
		t.Fatalf("openPGPV4Fingerprint(armored) error = %v", err)
	}
	if got != want {
		t.Fatalf("openPGPV4Fingerprint(armored) = %q, want %q", got, want)
	}
}

func TestOpenPGPV4FingerprintRejectsInvalidPackets(t *testing.T) {
	for _, packet := range [][]byte{nil, {0xc6, 1, 3}} {
		if _, err := openPGPV4Fingerprint(packet); err == nil {
			t.Errorf("openPGPV4Fingerprint(%x) succeeded; want an error", packet)
		}
	}
}

func TestPGPPublicationFollowsKeypairPresence(t *testing.T) {
	s := newTestServer(t)
	publicPath := filepath.Join(s.configDir, "security", "pgp.pub.asc")
	request := httptest.NewRequest(http.MethodGet, "/.well-known/pgp-key.asc", nil)
	missing := httptest.NewRecorder()
	s.handleSecurityPGPKey(missing, request)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing public key status = %d, want 404", missing.Code)
	}
	if pgpRouteMounted(s) {
		t.Fatal("PGP route is listed as mounted without a keypair")
	}

	if err := os.MkdirAll(filepath.Dir(publicPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const publicKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\npublic\n"
	if err := os.WriteFile(publicPath, []byte(publicKey), 0o600); err != nil {
		t.Fatal(err)
	}
	publicOnly := httptest.NewRecorder()
	s.handleSecurityPGPKey(publicOnly, request)
	if publicOnly.Code != http.StatusNotFound {
		t.Fatalf("public-only key status = %d, want 404", publicOnly.Code)
	}
	if err := os.WriteFile(filepath.Join(s.configDir, "security", "pgp.priv.asc.enc"), []byte("encrypted private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	available := httptest.NewRecorder()
	s.handleSecurityPGPKey(available, request)
	if available.Code != http.StatusOK || available.Body.String() != publicKey {
		t.Fatalf("available public key response = %d %q", available.Code, available.Body.String())
	}
	if !pgpRouteMounted(s) {
		t.Fatal("PGP route did not become mounted after keypair creation")
	}

	securityTxt := httptest.NewRecorder()
	s.handleSecurityTxt(securityTxt, httptest.NewRequest(http.MethodGet, "/.well-known/security.txt", nil))
	if !strings.Contains(securityTxt.Body.String(), "Encryption: ") || !strings.Contains(securityTxt.Body.String(), "/.well-known/pgp-key.asc") {
		t.Fatalf("security.txt does not advertise the available public key: %q", securityTxt.Body.String())
	}

	if err := os.Remove(publicPath); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "unrelated.txt")
	if err := os.WriteFile(target, []byte("must not be served"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, publicPath); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if s.hasPGPKeypair() {
		t.Fatal("PGP keypair accepted a symlinked public key")
	}
	if pgpRouteMounted(s) {
		t.Fatal("PGP route remained mounted after the keypair became unsafe")
	}
	response := httptest.NewRecorder()
	s.handleSecurityPGPKey(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "must not be served") {
		t.Fatalf("symlinked public key response = %d %q", response.Code, response.Body.String())
	}
}

func pgpRouteMounted(s *Server) bool {
	for _, route := range s.openAPIRoutes() {
		if route.Path == "/.well-known/pgp-key.asc" && route.Method == http.MethodGet {
			return route.Mounted
		}
	}
	return false
}

func TestEnsureDisclosureSchemaMigratesLegacyRows(t *testing.T) {
	s := contactTestServer(t)
	if _, err := s.db.Exec(`CREATE TABLE security_reports (
		tracking_id TEXT PRIMARY KEY, body_ciphertext BLOB NOT NULL, severity TEXT NOT NULL,
		component TEXT NOT NULL, credit_preference TEXT NOT NULL, created_at TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'Received', status_token_hash TEXT NOT NULL DEFAULT '',
		status_token_used_at TEXT NOT NULL DEFAULT '', closed_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	legacyBody := []byte{1, 2, 3, 4}
	if _, err := s.db.Exec(`INSERT INTO security_reports(tracking_id, body_ciphertext, severity, component, credit_preference, created_at, status_token_hash) VALUES(?, ?, ?, ?, ?, ?, ?)`, "sec_legacy", legacyBody, "high", "API", "anonymous", "2026-01-02T03:04:05Z", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := s.ensureDisclosureSchema(); err != nil {
		t.Fatalf("ensureDisclosureSchema: %v", err)
	}
	var body []byte
	var bodyAlg, creditPref, tokenHash, createdAt, updatedAt string
	if err := s.db.QueryRow(`SELECT body_enc, body_alg, credit_pref, token_hash, created_at, updated_at FROM security_reports WHERE tracking_id = ?`, "sec_legacy").Scan(&body, &bodyAlg, &creditPref, &tokenHash, &createdAt, &updatedAt); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if string(body) != string(legacyBody) || bodyAlg != "aes-256-gcm" || creditPref != "anon" || tokenHash != strings.Repeat("a", 64) || updatedAt != createdAt {
		t.Fatalf("migrated row = body %v, alg %q, credit %q, token %q, updated %q; want preserved legacy values", body, bodyAlg, creditPref, tokenHash, updatedAt)
	}
}

func TestApplySecurityReportUpdateAdvancesAndAddsThanks(t *testing.T) {
	s := contactTestServer(t)
	if err := s.ensureDisclosureSchema(); err != nil {
		t.Fatalf("ensure disclosure schema: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO security_reports(tracking_id, security_id, severity, component, status, body_enc, body_alg, credit_pref, credit_name, token_hash, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sec_anon", "id", "high", "API", "received", []byte{1}, "aes-256-gcm", "anon", nil, strings.Repeat("a", 64), "2026-01-02T03:04:05Z", "2026-01-02T03:04:05Z"); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	status := "triaged"
	comment := "Confirmed in the API."
	date := "2026-03-04"
	if err := ApplySecurityReportUpdate(s.db, "sec_anon", SecurityReportUpdate{Status: &status, Comment: &comment, DisclosureDate: &date}, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)); err != nil {
		t.Fatalf("triage report: %v", err)
	}
	status = "disclosed"
	severity := "critical"
	if err := ApplySecurityReportUpdate(s.db, "sec_anon", SecurityReportUpdate{Status: &status, Severity: &severity}, time.Date(2026, 2, 4, 4, 5, 6, 0, time.UTC)); err != nil {
		t.Fatalf("disclose report: %v", err)
	}
	var count int
	var credit, disclosedOn, gotSeverity string
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM security_thanks WHERE tracking_id = ?`, "sec_anon").Scan(&count); err != nil {
		t.Fatalf("count thanks rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("hall-of-fame rows = %d, want 1", count)
	}
	if err := s.db.QueryRow(`SELECT credit, severity, disclosed_on FROM security_thanks WHERE tracking_id = ?`, "sec_anon").Scan(&credit, &gotSeverity, &disclosedOn); err != nil {
		t.Fatalf("read thanks row: %v", err)
	}
	if credit != "Anonymous Researcher #1" || gotSeverity != "critical" || disclosedOn != "2026-02-04" {
		t.Fatalf("thanks = %q, %q, %q", credit, gotSeverity, disclosedOn)
	}
	backward := "triaged"
	if err := ApplySecurityReportUpdate(s.db, "sec_anon", SecurityReportUpdate{Status: &backward}, time.Now()); err == nil {
		t.Fatal("closed report status was allowed to move backward")
	}
}

func TestApplySecurityReportUpdateRejectsMarkupAndBadDates(t *testing.T) {
	db, err := sql.Open("sqlite", "file:security-update-validation?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close security disclosure test database: %v", err)
		}
	})
	if _, err := db.Exec(`CREATE TABLE security_reports (tracking_id TEXT PRIMARY KEY, status TEXT, credit_pref TEXT, credit_name TEXT, severity TEXT, updated_at TEXT, researcher_comment TEXT, disclosure_date TEXT, closed_at TEXT)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE security_thanks (tracking_id TEXT PRIMARY KEY, credit TEXT, severity TEXT, disclosed_on TEXT)`); err != nil {
		t.Fatalf("create thanks schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO security_reports(tracking_id, status, credit_pref, severity) VALUES('sec_test', 'received', 'no', 'low')`); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	markup := "<script>no</script>"
	if err := ApplySecurityReportUpdate(db, "sec_test", SecurityReportUpdate{Comment: &markup}, time.Now()); err == nil {
		t.Fatal("markup comment was accepted")
	}
	badDate := "2026-13-40"
	if err := ApplySecurityReportUpdate(db, "sec_test", SecurityReportUpdate{DisclosureDate: &badDate}, time.Now()); err == nil {
		t.Fatal("invalid disclosure date was accepted")
	}
}

func TestDecryptSecurityContact(t *testing.T) {
	s := newTestServer(t)
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	encodedKey := base64.StdEncoding.EncodeToString(key)
	s.config.Server.Security.EncryptionKey = encodedKey
	plain, err := json.Marshal(map[string]string{"email": "researcher@example.com", "researcher_gpg": "-----BEGIN PGP PUBLIC KEY BLOCK-----"})
	if err != nil {
		t.Fatalf("marshal contact: %v", err)
	}
	ciphertext, err := s.encryptSecurityData(plain)
	if err != nil {
		t.Fatalf("encrypt contact: %v", err)
	}
	email, pubkey, err := DecryptSecurityContact(encodedKey, ciphertext)
	if err != nil {
		t.Fatalf("DecryptSecurityContact: %v", err)
	}
	if email != "researcher@example.com" || pubkey != "-----BEGIN PGP PUBLIC KEY BLOCK-----" {
		t.Fatalf("decrypted contact = %q, %q", email, pubkey)
	}
}

func TestSecurityReportStatusTokenIsSingleUsePerDay(t *testing.T) {
	s := contactTestServer(t)
	if err := s.ensureDisclosureSchema(); err != nil {
		t.Fatalf("ensure disclosure schema: %v", err)
	}
	token := strings.Repeat("a", 64)
	digest := sha256.Sum256([]byte(token))
	createdAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`INSERT INTO security_reports(tracking_id, severity, component, status, body_enc, body_alg, credit_pref, token_hash, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sec_status", "high", "API", "triaged", []byte{1}, "aes-256-gcm", "no", hex.EncodeToString(digest[:]), createdAt, createdAt); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	newRequest := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/server/security/report/sec_status?token="+token, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tracking_id", "sec_status")
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	first := httptest.NewRecorder()
	s.handleServerSecurityReportStatus(first, newRequest())
	if first.Code != http.StatusOK {
		t.Fatalf("first status request = %d, body %q", first.Code, first.Body.String())
	}
	if first.Header().Get("Cache-Control") != "no-store" || first.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("status privacy headers = %v", first.Header())
	}
	second := httptest.NewRecorder()
	s.handleServerSecurityReportStatus(second, newRequest())
	if second.Code != http.StatusNotFound {
		t.Fatalf("second same-day status request = %d, want 404", second.Code)
	}
}
