package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestContactFieldContracts(t *testing.T) {
	for _, kind := range []string{"abuse", "dmca", "privacy", "security"} {
		if len(contactFormFields(kind)) == 0 {
			t.Errorf("contactFormFields(%q) is empty", kind)
		}
	}
	if contactFormFields("unknown") != nil {
		t.Fatal("contactFormFields accepted an unknown type")
	}
	if !contactTypeAllowed("security", true) || contactTypeAllowed("abuse", true) || !contactTypeAllowed("privacy", false) || contactTypeAllowed("security", false) {
		t.Fatal("contactTypeAllowed accepted an invalid mode/type pair")
	}
	if strings.Join(contactAllowedTypes(true), ",") != "security" || strings.Join(contactAllowedTypes(false), ",") != "abuse,dmca,privacy" {
		t.Fatal("contactAllowedTypes returned an unexpected type set")
	}

	valid := map[string]string{"name": "Visitor", "email": "visitor@example.com", "reported_url": "https://example.com/item", "reason": "other", "subject": "Report", "message": "Please review this."}
	if err := validateContactFields(valid, "abuse"); err != nil {
		t.Fatalf("valid abuse report rejected: %v", err)
	}
	invalid := map[string]string{"name": "Visitor", "email": "not-an-email", "subject": "Report", "message": "Hello", "request_kind": "invalid"}
	if err := validateContactFields(invalid, "privacy"); err == nil {
		t.Fatal("invalid privacy request was accepted")
	}
	security := map[string]string{"name": "Researcher", "email": "r@example.com", "component": "API", "severity": "high", "summary": "Issue", "steps_to_reproduce": "Step", "impact": "Impact", "credit_preference": "handle", "disclosure_agreement": "true"}
	if err := validateContactFields(security, "security"); err == nil || !strings.Contains(err.Error(), "credit handle") {
		t.Fatalf("missing conditional credit handle error = %v", err)
	}
	security["credit_handle"] = "researcher"
	if err := validateContactFields(security, "security"); err != nil {
		t.Fatalf("valid security report rejected: %v", err)
	}
}

func TestContactSignedReviewAndFormTokens(t *testing.T) {
	s := contactTestServer(t)
	now := time.Now().UTC()
	issuedAt := now.Add(-4 * time.Second)
	formToken, err := contactFormToken(s, issuedAt)
	if err != nil {
		t.Fatalf("issue form token: %v", err)
	}
	if !verifyContactFormToken(s, formToken, now, false) || !verifyContactFormToken(s, formToken, now, true) || verifyContactFormToken(s, formToken, now, true) {
		t.Fatal("form token validity or single-use behavior is incorrect")
	}
	if verifyContactFormToken(s, formToken, now.Add(13*time.Hour), false) || verifyContactFormToken(s, "malformed", now, false) {
		t.Fatal("expired or malformed form token was accepted")
	}

	values := map[string]string{"type": "abuse", "name": "Visitor", "email": "visitor@example.com", "message": "Report", "step": "submit"}
	reviewIssuedAt := now.Add(-3 * time.Second)
	reviewToken, err := contactReviewToken(s, values, reviewIssuedAt)
	if err != nil {
		t.Fatalf("issue review token: %v", err)
	}
	changed := make(map[string]string, len(values))
	for key, value := range values {
		changed[key] = value
	}
	changed["message"] = "tampered"
	if consumeContactReviewToken(s, reviewToken, changed, now) {
		t.Fatal("review token accepted modified values")
	}
	if !consumeContactReviewToken(s, reviewToken, values, now) || consumeContactReviewToken(s, reviewToken, values, now) {
		t.Fatal("review token validity or single-use behavior is incorrect")
	}
	if contactFieldsHash(map[string]string{"b": "two spaces", "a": "one"}) != contactFieldsHash(map[string]string{"a": "one", "b": "two   spaces"}) {
		t.Fatal("contactFieldsHash is not stable across order and whitespace")
	}
	if got := firstNonEmpty(" ", " value "); got != " value " {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
}

func TestContactRequestDecoding(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/server/contact", strings.NewReader(`{"type":"privacy","confirmed":true}`))
	r.Header.Set("Content-Type", "application/json")
	values, err := contactValuesFromRequest(r)
	if err != nil || values["type"] != "privacy" || values["confirmed"] != "true" {
		t.Fatalf("decode JSON contact values = %#v, %v", values, err)
	}
	r = httptest.NewRequest("POST", "/api/v1/server/contact", strings.NewReader(`{"value":[]}`))
	r.Header.Set("Content-Type", "application/json")
	if _, err := contactValuesFromRequest(r); err == nil {
		t.Fatal("contact decoder accepted a non-scalar JSON field")
	}
	r = httptest.NewRequest("POST", "/api/v1/server/contact", strings.NewReader("name=a&name=b"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := contactValuesFromRequest(r); err == nil {
		t.Fatal("contact decoder accepted a duplicate field")
	}
}

func TestContactRequestAndContentValidation(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest("POST", "https://example.com/server/contact", nil)
	r.Host = "example.com"
	r.Header.Set("Origin", "https://example.com")
	if !contactOriginValid(r) || !contactBrowserPost(r) || !contactAllowedHost(r, "example.com") {
		t.Fatalf("same-origin browser request was rejected: origin=%v browser=%v host=%v base=%q", contactOriginValid(r), contactBrowserPost(r), contactAllowedHost(r, "example.com"), RequestBaseURL(r))
	}
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	if contactOriginValid(r) {
		t.Fatal("cross-site browser request was accepted")
	}
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Content-Type", "application/json")
	if contactBrowserPost(r) {
		t.Fatal("JSON API request was classified as a browser form")
	}

	if !contactShapeValid(map[string]string{"type": "abuse", "message": "ok"}) || contactShapeValid(map[string]string{"unexpected": "x"}) || contactShapeValid(map[string]string{"message": "\u202e"}) {
		t.Fatal("contactShapeValid did not enforce the field and Unicode contract")
	}
	if contactContainsLinks("person@example.com") || !contactContainsLinks("visit https://elsewhere.example") {
		t.Fatal("contactContainsLinks mishandled email and URL text")
	}
	if err := validateContactContent(s, r, map[string]string{"message": "<script>alert(1)</script>"}, "abuse"); err == nil {
		t.Fatal("contact content accepted markup")
	}
	if err := validateContactContent(s, r, map[string]string{"reported_url": "https://example.com/api/private"}, "abuse"); err == nil {
		t.Fatal("contact content accepted a non-reportable API URL")
	}
	if err := validateContactContent(s, r, map[string]string{"reported_url": "https://example.com/airports/FAKE"}, "abuse"); err == nil {
		t.Fatal("contact content accepted a route whose airport resource does not exist")
	}
	if err := validateContactContent(s, r, map[string]string{"reported_url": "https://example.com/airports/KJFK"}, "abuse"); err != nil {
		t.Fatalf("contact content rejected an existing airport resource: %v", err)
	}
	body := contactPlainTextBody(map[string]string{"subject": "Issue", "message": "Details"}, "abuse")
	if !strings.Contains(body, "Type: abuse") || !strings.Contains(body, "subject: Issue") || !strings.Contains(body, "message: Details") {
		t.Fatalf("contact email body = %q", body)
	}
	if got := firstNonEmpty("", "second", "third"); got != "second" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
}
