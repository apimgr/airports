package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var contactCaptchaQuestionPattern = regexp.MustCompile(`What is ([0-9]+) \+ ([0-9]+)\?`)

func TestContactCaptchaIsSingleUse(t *testing.T) {
	s := newTestServer(t)
	question, id, err := s.newContactCaptcha()
	if err != nil {
		t.Fatalf("newContactCaptcha() error = %v", err)
	}
	parts := contactCaptchaQuestionPattern.FindStringSubmatch(question)
	if len(parts) != 3 {
		t.Fatalf("challenge question = %q, want arithmetic question", question)
	}
	answer := fmt.Sprintf("%d", mustParseContactOperand(t, parts[1])+mustParseContactOperand(t, parts[2]))
	if !s.consumeContactCaptcha(id, answer) {
		t.Fatal("consumeContactCaptcha() rejected the correct answer")
	}
	if s.consumeContactCaptcha(id, answer) {
		t.Fatal("consumeContactCaptcha() accepted a replayed challenge")
	}
}

func TestContactCaptchaRejectsWrongAndExpiredAnswers(t *testing.T) {
	s := newTestServer(t)
	_, id, err := s.newContactCaptcha()
	if err != nil {
		t.Fatalf("newContactCaptcha() error = %v", err)
	}
	if s.consumeContactCaptcha(id, "wrong") {
		t.Fatal("consumeContactCaptcha() accepted a non-numeric answer")
	}

	s.contactCaptchaChallenges["expired"] = contactCaptchaChallenge{answer: 2, expires: time.Now().Add(-time.Second)}
	if s.consumeContactCaptcha("expired", "2") {
		t.Fatal("consumeContactCaptcha() accepted an expired challenge")
	}
}

func TestContactPageDoesNotChallengeFirstAttempt(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.handleServerContact(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "What is ") || strings.Contains(w.Body.String(), `name="captcha_id"`) {
		t.Fatal("contact page challenged a visitor before a failed attempt")
	}
}

func TestContactPageChallengesAfterFailedAttempt(t *testing.T) {
	s := newTestServer(t)
	failed := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader("invalid"))
	failed.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleContactSubmission(httptest.NewRecorder(), failed)

	r := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.handleServerContact(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "What is ") || !strings.Contains(w.Body.String(), `name="captcha_id"`) {
		t.Fatalf("contact page did not render a challenge after a failure: status=%d", w.Code)
	}
}

func TestContactSubmissionRejectsInvalidCaptcha(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", nil)
	s.markContactCaptchaRequired(r)
	form := url.Values{
		"name": {"Visitor"}, "email": {"visitor@example.com"},
		"subject": {"Question"}, "message": {"Hello"},
		"captcha_id": {"missing"}, "captcha": {"4"},
	}
	r = httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleContactSubmission(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "CAPTCHA_FAILED") {
		t.Fatalf("status/body = %d %q, want 400 CAPTCHA_FAILED", w.Code, w.Body.String())
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
