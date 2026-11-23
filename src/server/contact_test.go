package server

import (
	"container/list"
	"database/sql"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
)

var contactCaptchaQuestionPattern = regexp.MustCompile(`What is ([0-9]+) ([+-]) ([0-9]+)\?`)

func TestContactCaptchaIsSingleUse(t *testing.T) {
	s := contactTestServer(t)
	question, id, err := s.newContactCaptcha()
	if err != nil {
		t.Fatalf("newContactCaptcha() error = %v", err)
	}
	parts := contactCaptchaQuestionPattern.FindStringSubmatch(question)
	if len(parts) != 4 {
		t.Fatalf("challenge question = %q, want arithmetic question", question)
	}
	left := mustParseContactOperand(t, parts[1])
	right := mustParseContactOperand(t, parts[3])
	if left < 1 || left > 20 || right < 1 || right > 20 {
		t.Fatalf("challenge operands = %d and %d, want values from 1 to 20", left, right)
	}
	answerValue := left + right
	if parts[2] == "-" {
		if right > left {
			t.Fatalf("subtraction challenge has a negative result: %q", question)
		}
		answerValue = left - right
	}
	answer := fmt.Sprintf("%d", answerValue)
	other := newTestServer(t)
	other.db = s.db
	if !other.consumeContactCaptcha(id, answer) {
		t.Fatal("consumeContactCaptcha() rejected the correct answer")
	}
	if s.consumeContactCaptcha(id, answer) {
		t.Fatal("consumeContactCaptcha() accepted a replayed challenge")
	}
}

func TestContactCaptchaRejectsWrongAndExpiredAnswers(t *testing.T) {
	s := contactTestServer(t)
	_, id, err := s.newContactCaptcha()
	if err != nil {
		t.Fatalf("newContactCaptcha() error = %v", err)
	}
	if s.consumeContactCaptcha(id, "wrong") {
		t.Fatal("consumeContactCaptcha() accepted a non-numeric answer")
	}

	if err := s.ensureContactCaptchaSchema(); err != nil {
		t.Fatalf("ensure contact CAPTCHA schema: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO captcha_challenges(id, answer_hash, expires_at) VALUES(?, ?, ?)`, "expired", "", time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatalf("insert expired challenge: %v", err)
	}
	if s.consumeContactCaptcha("expired", "2") {
		t.Fatal("consumeContactCaptcha() accepted an expired challenge")
	}
}

func TestContactPageChallengesEveryAttempt(t *testing.T) {
	s := contactTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/contact?type=abuse", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.handleServerContact(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "What is ") || !strings.Contains(w.Body.String(), `name="captcha_id"`) {
		t.Fatal("contact page did not challenge a visitor on the first attempt")
	}
}

func TestContactPageChallengesAfterFailedAttempt(t *testing.T) {
	s := contactTestServer(t)
	failed := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader("invalid"))
	failed.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleContactSubmission(httptest.NewRecorder(), failed)

	r := httptest.NewRequest(http.MethodGet, "/server/contact?type=abuse", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.handleServerContact(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "What is ") || !strings.Contains(w.Body.String(), `name="captcha_id"`) {
		t.Fatalf("contact page did not render a challenge after a failure: status=%d", w.Code)
	}
}

func contactTestServer(t *testing.T) *Server {
	t.Helper()
	s := newTestServer(t)
	database, err := sql.Open("sqlite", "file:contact-"+strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open contact test database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.EnsureAppSecrets(database); err != nil {
		t.Fatalf("initialize contact test secrets: %v", err)
	}
	s.db = database
	s.config.Server.Pages.Contact.Enabled = true
	return s
}

func TestContactSubmissionRejectsMissingReportType(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{
		"name": {"Visitor"}, "email": {"visitor@example.com"},
		"subject": {"Question"}, "message": {"Hello"},
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleContactSubmission(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Choose a valid report type") {
		t.Fatalf("status/body = %d %q, want a missing report type validation error", w.Code, w.Body.String())
	}
}

func mustParseContactOperand(t *testing.T, value string) int {
	t.Helper()
	var operand int
	if _, err := fmt.Sscanf(value, "%d", &operand); err != nil {
		t.Fatalf("parse challenge operand %q: %v", value, err)
	}
	return operand
}

func TestWebhookFailureLogsAreRateLimitedPerClientIP(t *testing.T) {
	webhookFailureLogState.Lock()
	delete(webhookFailureLogState.last, "192.0.2.44")
	delete(webhookFailureLogState.last, "192.0.2.45")
	webhookFailureLogState.Unlock()
	t.Cleanup(func() {
		webhookFailureLogState.Lock()
		delete(webhookFailureLogState.last, "192.0.2.44")
		delete(webhookFailureLogState.last, "192.0.2.45")
		webhookFailureLogState.Unlock()
	})
	first := time.Now().UTC()
	if !allowWebhookFailureLog("192.0.2.44", first) {
		t.Fatal("first failure for an IP was rate limited")
	}
	if allowWebhookFailureLog("192.0.2.44", first.Add(30*time.Second)) {
		t.Fatal("second failure inside the rate-limit window was logged")
	}
	if !allowWebhookFailureLog("192.0.2.45", first.Add(30*time.Second)) {
		t.Fatal("failure from a different IP was rate limited")
	}
	if !allowWebhookFailureLog("192.0.2.44", first.Add(time.Minute)) {
		t.Fatal("failure after the rate-limit window was still limited")
	}
}

func TestContactDuplicateUsesNormalizedBoundedKey(t *testing.T) {
	contactAntispamState.Lock()
	contactAntispamState.duplicates = make(map[string]*list.Element)
	contactAntispamState.order.Init()
	contactAntispamState.Unlock()
	settings := config.ContactAntispamConfig{DuplicateWindowHours: 1}
	now := time.Now()
	first := map[string]string{"email": " Person@Example.com ", "subject": "Hello   there", "message": " A\n B "}
	second := map[string]string{"email": "person@example.com", "subject": "hello there", "message": "a b"}
	if recordContactDuplicate(first, settings, now) {
		t.Fatal("first submission was considered a duplicate")
	}
	if !recordContactDuplicate(second, settings, now.Add(time.Minute)) {
		t.Fatal("normalized repeat was not considered a duplicate")
	}
}

func TestContactContentUsesConfiguredRules(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "https://airports.example/server/contact", nil)
	values := map[string]string{"name": "Visitor", "email": "visitor@example.com", "subject": "Question", "message": "Plain text"}
	settings := config.ContactAntispamConfig{RejectMarkup: true, MaxLinks: 1, BlockedWords: []string{"solicitation"}, BlockedEmailDomains: []string{"blocked.example"}}
	cases := []struct {
		name   string
		update func(map[string]string)
		reason string
	}{
		{name: "markup", update: func(v map[string]string) { v["message"] = "<a href='x'>link</a>" }, reason: "markup"},
		{name: "blocked word", update: func(v map[string]string) { v["message"] = "SOLICITATION" }, reason: "content"},
		{name: "blocked domain", update: func(v map[string]string) { v["email"] = "person@mail.blocked.example" }, reason: "content"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := maps.Clone(values)
			tc.update(copy)
			reason, err := validateContactContentReason(s, r, copy, "privacy", settings)
			if err == nil || reason != tc.reason {
				t.Fatalf("reason/error = %q/%v, want %q and an error", reason, err, tc.reason)
			}
		})
	}
}

func TestContactStrikeBanAndOverlayException(t *testing.T) {
	contactAntispamState.Lock()
	contactAntispamState.strikes = make(map[string][]time.Time)
	contactAntispamState.bans = make(map[string]time.Time)
	contactAntispamState.Unlock()
	settings := config.ContactAntispamConfig{Strikes: 3, BanHours: 2, Window: "1h"}
	for i := 0; i < 3; i++ {
		recordContactStrike("192.0.2.88", settings, time.Now(), false)
	}
	if got := contactAntispamState.bans[contactIdentity("192.0.2.88")]; time.Until(got) < time.Hour {
		t.Fatal("third strike did not create the expected ban")
	}
	key := contactIdentity("192.0.2.89")
	for i := 0; i < 5; i++ {
		recordContactStrike("192.0.2.89", settings, time.Now(), true)
	}
	if _, banned := contactAntispamState.bans[key]; banned {
		t.Fatal("overlay client was banned")
	}
}

func TestSecurityWebhookFallsBackToAdminPerTransport(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Contact.Admin.Webhooks = map[string]string{"slack": "https://hooks.example/admin", "slack_secret": "admin-secret"}
	s.config.Server.Contact.Security.Webhooks = map[string]string{"email": "https://mail.example/security", "email_secret": "security-secret"}
	targets := s.contactWebhookTargets("security")
	if len(targets) != 2 {
		t.Fatalf("security targets = %#v, want security and admin-fallback transports", targets)
	}
	seen := map[string]string{}
	for _, target := range targets {
		seen[target.Transport] = target.URL
	}
	if seen["slack"] != "https://hooks.example/admin" || seen["email"] != "https://mail.example/security" {
		t.Fatalf("security fallback targets = %#v", seen)
	}
}
