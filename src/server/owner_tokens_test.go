package server

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/logging"
)

type ownerTokenAuditRecorder struct {
	events []string
}

func (r *ownerTokenAuditRecorder) WriteAudit(_ logging.AuditCategory, line string) error {
	r.events = append(r.events, line)
	return nil
}

func TestLegacyOwnerCookieMigratesAfterTokenValidation(t *testing.T) {
	s := newTestServer(t)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy-cookie.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	s.db = database
	s.config.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x51}, 32))
	ownerID, token, err := s.issueOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	if ownerID == "" {
		t.Fatal("owner token did not create an owner")
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: ownerTokenLegacyCookieName, Value: token})
	response := httptest.NewRecorder()
	s.ownerTokenCookieLifecycle(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d, want 200", response.Code)
	}
	var newCookie, legacyCleared bool
	for _, cookie := range response.Result().Cookies() {
		switch cookie.Name {
		case ownerTokenCookieName:
			newCookie = cookie.Value == token && cookie.MaxAge > 0 && cookie.HttpOnly && cookie.Secure && cookie.SameSite == http.SameSiteStrictMode
		case ownerTokenLegacyCookieName:
			legacyCleared = cookie.Value == "" && cookie.MaxAge < 0
		}
	}
	if !newCookie || !legacyCleared {
		t.Fatalf("legacy cookie migration missing: new=%t legacy_cleared=%t", newCookie, legacyCleared)
	}
}

func TestOwnerCookieNamePrefersCompliantName(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: ownerTokenLegacyCookieName, Value: "legacy"})
	r.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: "current"})
	cookie, err := s.ownerTokenCookie(r)
	if err != nil || cookie.Name != ownerTokenCookieName || cookie.Value != "current" {
		t.Fatalf("ownerTokenCookie() = %#v, %v; want current cookie", cookie, err)
	}
}

func TestLegacyOwnerCookieFallbackExpiresAfterConfiguredMaxAge(t *testing.T) {
	s := newTestServer(t)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "expired-legacy-cookie.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	s.db = database
	if err := s.ensureOwnerTokenSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE owner_cookie_migrations SET started_at = 1 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: ownerTokenLegacyCookieName, Value: "legacy"})
	if cookie, err := s.ownerTokenCookie(r); err == nil || cookie != nil {
		t.Fatalf("expired legacy cookie fallback = %#v, %v; want no cookie", cookie, err)
	}
}

func TestOwnerTokenViewCookieRevealsOnlyOwnedCardOnce(t *testing.T) {
	s := newTestServer(t)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "token-view.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	s.db = database
	s.config.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x51}, 32))
	ownerID, primary, err := s.issueOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	linked, err := s.createLinkedOwnerToken(ownerID, "MCP client", "never")
	if err != nil {
		t.Fatal(err)
	}
	linkedID, err := s.ownerTokenIDByRaw(linked)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	request.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: primary})
	request.AddCookie(&http.Cookie{Name: ownerTokenViewCookieName, Value: fmt.Sprint(linkedID)})
	request.AddCookie(&http.Cookie{Name: ownerTokenNoticeCookieName, Value: "created"})
	response := httptest.NewRecorder()
	s.handleOwnerTokensPage(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("token page status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), linked) || strings.Contains(response.Body.String(), primary) {
		t.Fatal("one-use view cookie did not reveal only the designated linked token")
	}
	if !strings.Contains(response.Body.String(), "Token created. Save it somewhere safe.") {
		t.Fatal("post-redirect token feedback was not rendered for the no-JavaScript path")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	cleared := false
	noticeCleared := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == ownerTokenViewCookieName {
			cleared = cookie.MaxAge <= 0 && cookie.Value == "" && cookie.HttpOnly && cookie.Secure && cookie.SameSite == http.SameSiteStrictMode && cookie.Path == "/server/preferences/tokens"
		}
		if cookie.Name == ownerTokenNoticeCookieName {
			noticeCleared = cookie.MaxAge <= 0 && cookie.Value == "" && cookie.HttpOnly && cookie.Secure && cookie.SameSite == http.SameSiteStrictMode && cookie.Path == "/server/preferences/tokens"
		}
	}
	if !cleared {
		t.Fatal("one-use token view cookie was not cleared with the required attributes")
	}
	if !noticeCleared {
		t.Fatal("post-redirect notice cookie was not cleared")
	}

	reload := httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	reload.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: primary})
	reloadResponse := httptest.NewRecorder()
	s.handleOwnerTokensPage(reloadResponse, reload)
	if strings.Contains(reloadResponse.Body.String(), linked) || strings.Contains(reloadResponse.Body.String(), primary) {
		t.Fatal("a later page load revealed a raw owner token")
	}
}

func TestPrimaryTokenRevokeConfirmsAndDeletesOwnerResources(t *testing.T) {
	s := newTestServer(t)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "owners.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	s.db = database
	s.config.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	if err := s.ensureOwnerTokenSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE saved_airports (id INTEGER PRIMARY KEY, owner_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	ownerID, primary, err := s.issueOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.createLinkedOwnerToken(ownerID, "MCP client", "never"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO saved_airports(owner_id) VALUES (?), (?)`, ownerID, ownerID); err != nil {
		t.Fatal(err)
	}
	audit := &ownerTokenAuditRecorder{}
	s.SetAuditLogger(audit)

	pageRequest := httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	pageRequest.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: primary})
	pageResponse := httptest.NewRecorder()
	s.Router().ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("token page status = %d, want 200: %s", pageResponse.Code, pageResponse.Body.String())
	}
	if !strings.Contains(pageResponse.Body.String(), "1 linked tokens and 2 airports") || !strings.Contains(pageResponse.Body.String(), primary[len(primary)-4:]) {
		t.Fatalf("token page omitted live revoke counts or primary suffix")
	}

	request := func(confirm string) *httptest.ResponseRecorder {
		body := strings.NewReader(fmt.Sprintf(`{"confirm":%q}`, confirm))
		r := httptest.NewRequest(http.MethodDelete, "/api/v1/server/preferences/tokens/primary", body)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+primary)
		r.RemoteAddr = "203.0.113.5:4321"
		response := httptest.NewRecorder()
		s.handleOwnerTokenPrimaryRevokeAPI(response, r)
		return response
	}
	wrong := request("####")
	if wrong.Code != http.StatusBadRequest {
		t.Fatalf("wrong confirmation status = %d, want 400", wrong.Code)
	}
	var remaining int
	if err := database.QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("wrong confirmation changed owner data: count=%d err=%v", remaining, err)
	}

	response := request(primary[len(primary)-4:])
	if response.Code != http.StatusOK {
		t.Fatalf("valid confirmation status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Tokens    int `json:"tokens"`
			Resources int `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Tokens != 2 || body.Data.Resources != 2 {
		t.Fatalf("revoke response counts = %+v, want 2 tokens and 2 resources", body.Data)
	}
	for _, table := range []string{"owners", "api_tokens", "saved_airports"} {
		if err := database.QueryRow(`SELECT COUNT(*) FROM "` + table + `"`).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("%s contains rows after revoke: count=%d err=%v", table, remaining, err)
		}
	}
	if len(audit.events) != 1 || !strings.Contains(audit.events[0], "event=token.revoked") || !strings.Contains(audit.events[0], "scope=\"owner\"") || !strings.Contains(audit.events[0], "token_count=2") || !strings.Contains(audit.events[0], "resource_count=2") || !strings.Contains(audit.events[0], "source=\"api\"") || !strings.Contains(audit.events[0], "ip=\"203.0.113.5\"") {
		t.Fatalf("primary revoke audit events = %#v", audit.events)
	}
}
