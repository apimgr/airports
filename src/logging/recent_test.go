package logging

import (
	"strings"
	"testing"
	"time"
)

func TestRecentLogBufferRedactsCredentials(t *testing.T) {
	buffer := NewRecentLogBuffer(10)
	_, _ = buffer.Write([]byte("Authorization: Bearer bearer-secret\nAuthorization: Custom scheme-secret value\npassword=plain-secret token: \"json-secret\"\nCookie: owner_token=owner-secret; csrf_token=csrf-secret\nhttps://user:url-secret@example.test/private\n"))
	keyStart := strings.Join([]string{"-----BEGIN ", "PRIVATE KEY-----"}, "")
	keyEnd := strings.Join([]string{"-----END ", "PRIVATE KEY-----"}, "")
	_, _ = buffer.Write([]byte(keyStart + "\nprivate-key-secret\n" + keyEnd + "\nnext event\n"))

	entries := buffer.Recent(10, time.Hour, time.Now().Add(time.Second))
	if len(entries) != 9 {
		t.Fatalf("Recent() returned %d entries, want 9", len(entries))
	}
	joined := make([]string, 0, len(entries))
	for _, entry := range entries {
		joined = append(joined, entry.Line)
	}
	output := strings.Join(joined, "\n")
	for _, secret := range []string{"bearer-secret", "scheme-secret", "plain-secret", "json-secret", "owner-secret", "csrf-secret", "private-key-secret", "url-secret"} {
		if strings.Contains(output, secret) {
			t.Errorf("recent logs leaked %q: %s", secret, output)
		}
	}
	if strings.Count(output, "Authorization: xxxxx") != 2 || !strings.Contains(output, "password=xxxxx") || !strings.Contains(output, "[redacted credential block]") {
		t.Errorf("credential redaction markers missing: %s", output)
	}
}

func TestRecentLogBufferBoundsEntriesAndAge(t *testing.T) {
	buffer := NewRecentLogBuffer(3)
	_, _ = buffer.Write([]byte("one\ntwo\nthree\nfour\n"))
	now := time.Now().Add(time.Second)
	entries := buffer.Recent(2, time.Hour, now)
	if len(entries) != 2 || entries[0].Line != "three" || entries[1].Line != "four" {
		t.Fatalf("Recent(2) = %#v, want the newest two entries", entries)
	}
	if got := buffer.Recent(3, -time.Second, now); len(got) != 0 {
		t.Fatalf("Recent with expired age returned %d entries, want 0", len(got))
	}
}

func TestRecentLogBufferZeroValue(t *testing.T) {
	var buffer RecentLogBuffer
	if _, err := buffer.Write([]byte("ready\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buffer.Recent(1, time.Hour, time.Now().Add(time.Second)); len(got) != 1 || got[0].Line != "ready" {
		t.Fatalf("Recent() = %#v, want the zero-value entry", got)
	}
}
