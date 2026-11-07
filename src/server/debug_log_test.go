package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugLogRedactsSensitiveQueryAndRefererValues(t *testing.T) {
	query := redactSensitiveQuery("page=2&Access_Token=private&password=hunter2")
	if !strings.Contains(query, "page=2") || strings.Contains(query, "private") || strings.Contains(query, "hunter2") {
		t.Fatalf("redacted query = %q", query)
	}
	referer := redactLoggedURL("https://user:pass@example.test/path?token=private&view=full#secret")
	for _, secret := range []string{"user", "pass", "private", "secret"} {
		if strings.Contains(referer, secret) {
			t.Errorf("redacted referer %q contains %q", referer, secret)
		}
	}
}

func TestSanitizeDebugBodyRedactsCredentialFields(t *testing.T) {
	got := sanitizeDebugBody([]byte(`{"user":"visitor","nested":{"api_key":"private","ok":true}}`), "application/json")
	if !strings.Contains(got, `"user":"visitor"`) || !strings.Contains(got, `"api_key":"[redacted]"`) || strings.Contains(got, "private") {
		t.Fatalf("sanitized JSON body = %s", got)
	}
	form := sanitizeDebugBody([]byte("name=visitor&csrf_token=private"), "application/x-www-form-urlencoded")
	if !strings.Contains(form, "name=visitor") || strings.Contains(form, "private") {
		t.Fatalf("sanitized form body = %s", form)
	}
	if got := sanitizeDebugBody([]byte("password=private"), "text/plain"); got != "[redacted: unsupported body type]" {
		t.Fatalf("sanitized text body = %q", got)
	}
}

func TestDebugResponseWriterCapturesOnlyConfiguredBound(t *testing.T) {
	response := httptest.NewRecorder()
	writer := &debugResponseWriter{ResponseWriter: response, status: 200, bodyLimit: 3}
	if _, err := writer.Write([]byte("abcdef")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if response.Body.String() != "abcdef" || string(writer.body) != "abcd" || !writer.bodyTruncated {
		t.Fatalf("captured response body=%q truncated=%t; sent body=%q", writer.body, writer.bodyTruncated, response.Body.String())
	}
}
