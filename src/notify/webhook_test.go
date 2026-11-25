package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildWebhookRequestAdapters(t *testing.T) {
	message := WebhookMessage{
		Event: "backup.failed", Subject: "Backup failed", Body: "Disk full", Severity: "warning",
		Timestamp:   time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		ProjectName: "airports", ProjectVersion: "1.2.3", AppURL: "https://example.test", TrackingID: "sec_123",
	}
	tests := []struct {
		name, rawURL, wantType, wantBody string
	}{
		{"telegram", "https://notify.test/send?chat_id=123", "", ""},
		{"discord", "https://notify.test/webhook", "application/json", `"username":"airports"`},
		{"slack", "https://notify.test/slack", "application/json", `"text":"Backup failed\nSeverity: warning\nTracking ID: sec_123\nReport: https://example.test/server/security/report/sec_123"`},
		{"mattermost", "https://notify.test/mattermost", "application/json", `"text":"Backup failed\nSeverity: warning\nTracking ID: sec_123\nReport: https://example.test/server/security/report/sec_123"`},
		{"pushover", "https://notify.test/1/messages.json?user=u&token=t", "application/x-www-form-urlencoded", "priority=0"},
		{"gotify", "https://notify.test/base?token=t", "application/json", `"priority":5`},
		{"generic", "https://notify.test/events", "application/json", `"tracking_id":"sec_123"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint, body, contentType, err := buildWebhookRequest(WebhookTarget{Role: "admin", Transport: test.name, URL: test.rawURL}, message)
			if err != nil {
				t.Fatalf("buildWebhookRequest() error = %v", err)
			}
			parsed, err := url.Parse(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			if contentType != test.wantType {
				t.Errorf("content type = %q, want %q", contentType, test.wantType)
			}
			if test.name == "telegram" {
				if parsed.Query().Get("chat_id") != "123" || parsed.Query().Get("text") != "Backup failed\nSeverity: warning\nTracking ID: sec_123\nReport: https://example.test/server/security/report/sec_123" {
					t.Errorf("Telegram query = %v", parsed.Query())
				}
				return
			}
			if test.name == "pushover" {
				if strings.Contains(endpoint, "token=") || !strings.Contains(string(body), test.wantBody) || !strings.Contains(string(body), "message=Backup+failed") || strings.Contains(string(body), "Disk+full") {
					t.Errorf("Pushover request leaked query or omitted fields: endpoint=%q body=%q", endpoint, body)
				}
				return
			}
			if test.name == "gotify" && (parsed.Path != "/base/message" || parsed.Query().Get("token") != "t") {
				t.Errorf("Gotify endpoint = %q", endpoint)
			}
			if test.wantBody != "" && !strings.Contains(string(body), test.wantBody) {
				t.Errorf("request body %s missing %q", body, test.wantBody)
			}
			if test.name == "generic" {
				var payload webhookPayload
				if err := json.Unmarshal(body, &payload); err != nil || payload.Role != "admin" || payload.Event != message.Event || payload.TrackingID != message.TrackingID {
					t.Errorf("generic payload = (%+v, %v)", payload, err)
				}
				if payload.Body != "Backup failed\nSeverity: warning\nTracking ID: sec_123\nReport: https://example.test/server/security/report/sec_123" || strings.Contains(payload.Body, "Disk full") {
					t.Errorf("generic body is not a summary: %q", payload.Body)
				}
				if payload.Timestamp.Format(time.RFC3339) != "2026-01-02T03:04:05Z" || payload.Severity != "warning" {
					t.Errorf("generic timestamp/severity = %q/%q", payload.Timestamp.Format(time.RFC3339), payload.Severity)
				}
			}
		})
	}
}

func TestWebhookValidationSigningAndFormatting(t *testing.T) {
	for _, raw := range []string{
		"http://notify.test/path", "https://user:pass@notify.test/path", "https://notify.test/path#fragment", "https://",
	} {
		if _, err := parseWebhookURL(raw); err == nil {
			t.Errorf("parseWebhookURL(%q) accepted an invalid destination", raw)
		}
	}
	if _, _, _, err := buildWebhookRequest(WebhookTarget{Transport: "telegram", URL: "https://notify.test/send"}, WebhookMessage{}); err == nil {
		t.Fatal("Telegram adapter accepted a URL without chat_id")
	}
	if _, _, _, err := buildWebhookRequest(WebhookTarget{Transport: "pushover", URL: "https://notify.test/send?user=u"}, WebhookMessage{}); err == nil {
		t.Fatal("Pushover adapter accepted a URL without token")
	}
	if _, _, _, err := buildWebhookRequest(WebhookTarget{Transport: "gotify", URL: "https://notify.test"}, WebhookMessage{}); err == nil {
		t.Fatal("Gotify adapter accepted a URL without token")
	}
	if _, _, _, err := buildWebhookRequest(WebhookTarget{Transport: "unknown", URL: "https://notify.test"}, WebhookMessage{}); err == nil {
		t.Fatal("unknown adapter was accepted")
	}

	body := []byte(`{"event":"ready"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	wantSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := webhookSignature("secret", body); got != wantSignature {
		t.Errorf("webhookSignature() = %q, want %q", got, wantSignature)
	}
	message := WebhookMessage{Subject: "subject", Body: "private contact details", ProjectName: " airports ", ProjectVersion: " 1.0 ", AppURL: " https://example.test "}
	if got := webhookText(message); got != "subject\nSeverity: info" {
		t.Errorf("webhookText() = %q", got)
	}
	message.Subject = ""
	if got := webhookText(message); got != "Severity: info" {
		t.Errorf("webhookText(body only) = %q", got)
	}
	message.Body = ""
	if got := webhookText(message); got != "" {
		t.Errorf("webhookText(empty body and subject) = %q", got)
	}
	if got := webhookUserAgent(message); got != "airports/1.0 (+https://example.test)" {
		t.Errorf("webhookUserAgent() = %q", got)
	}
	for severity, want := range map[string]string{"critical": "1", "ERROR": "1", "warning": "0", "warn": "0", "info": "0"} {
		if got := pushoverPriority(severity); got != want {
			t.Errorf("pushoverPriority(%q) = %q, want %q", severity, got, want)
		}
	}
	for severity, want := range map[string]int{"critical": 8, "error": 8, "warning": 5, "warn": 5, "info": 2} {
		if got := gotifyPriority(severity); got != want {
			t.Errorf("gotifyPriority(%q) = %d, want %d", severity, got, want)
		}
	}
}
