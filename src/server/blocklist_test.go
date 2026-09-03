package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func TestParseCIDROrIP_SingleIPExpansion(t *testing.T) {
	prefix, ok := parseCIDROrIP("198.51.100.10", "test")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if prefix.Bits() != 32 {
		t.Errorf("IPv4 single IP should expand to /32, got /%d", prefix.Bits())
	}

	prefix6, ok := parseCIDROrIP("2001:db8::1", "test")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if prefix6.Bits() != 128 {
		t.Errorf("IPv6 single IP should expand to /128, got /%d", prefix6.Bits())
	}
}

func TestParseCIDROrIP_RejectsOverlyBroadRanges(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		ok   bool
	}{
		{"ipv4 /0 rejected", "0.0.0.0/0", false},
		{"ipv4 /7 rejected", "0.0.0.0/7", false},
		{"ipv4 /8 allowed", "10.0.0.0/8", true},
		{"ipv6 /0 rejected", "::/0", false},
		{"ipv6 /15 rejected", "2001::/15", false},
		{"ipv6 /16 allowed", "2001::/16", true},
		{"malformed rejected", "not-a-cidr", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseCIDROrIP(tt.cidr, "test")
			if ok != tt.ok {
				t.Errorf("parseCIDROrIP(%q) ok = %v, want %v", tt.cidr, ok, tt.ok)
			}
		})
	}
}

func TestAllowlistLookup_Contains(t *testing.T) {
	al := NewAllowlistLookup([]config.AllowlistEntry{
		{CIDR: "10.0.0.0/8", Description: "internal"},
		{CIDR: "203.0.113.50", Description: "admin home"},
		{CIDR: "::1/128", Description: "localhost"},
		{CIDR: "fd00::/16", Description: "ula"},
		{CIDR: "0.0.0.0/0", Description: "should be rejected"},
	})

	tests := []struct {
		ip   string
		want bool
	}{
		{"10.1.2.3", true},
		{"203.0.113.50", true},
		{"203.0.113.51", false},
		{"::1", true},
		{"fd00::1", true},
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		if got := al.Contains(tt.ip); got != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestAllowlistLookup_NilSafe(t *testing.T) {
	var al *AllowlistLookup
	if al.Contains("1.2.3.4") {
		t.Errorf("nil AllowlistLookup.Contains should return false")
	}
}

func TestBlockStore_PermanentConfigBlock(t *testing.T) {
	bs := NewBlockStore([]config.BlockedIPEntry{
		{CIDR: "198.51.100.0/24", Reason: "known abuser"},
	})

	blocked, block := bs.IsBlocked("198.51.100.5")
	if !blocked {
		t.Fatalf("expected 198.51.100.5 to be blocked")
	}
	if block.Type != BlockTypePermanent {
		t.Errorf("expected permanent block, got %s", block.Type)
	}
	if block.Reason != "known abuser" {
		t.Errorf("reason = %q, want %q", block.Reason, "known abuser")
	}

	if blocked, _ := bs.IsBlocked("198.51.101.5"); blocked {
		t.Errorf("198.51.101.5 should not be blocked")
	}
}

func TestBlockStore_TemporaryBlockAndExpiry(t *testing.T) {
	bs := NewBlockStore(nil)
	bs.AddTemporary("192.0.2.1", "rate_limit_abuse", 50*time.Millisecond)

	blocked, block := bs.IsBlocked("192.0.2.1")
	if !blocked {
		t.Fatalf("expected 192.0.2.1 to be blocked immediately after AddTemporary")
	}
	if block.Type != BlockTypeTemporary {
		t.Errorf("expected temporary block, got %s", block.Type)
	}
	if block.OffenseCount != 1 {
		t.Errorf("offense count = %d, want 1", block.OffenseCount)
	}

	// Second offense should bump the counter rather than duplicate.
	bs.AddTemporary("192.0.2.1", "rate_limit_abuse", time.Hour)
	if _, block := bs.IsBlocked("192.0.2.1"); block.OffenseCount != 2 {
		t.Errorf("offense count after 2nd block = %d, want 2", block.OffenseCount)
	}

	// Force expiry and release.
	bs.mu.Lock()
	expired := time.Now().Add(-time.Second)
	bs.temporary["192.0.2.1"].ExpiresAt = &expired
	bs.mu.Unlock()

	if blocked, _ := bs.IsBlocked("192.0.2.1"); blocked {
		t.Errorf("expired temporary block should no longer report as blocked")
	}

	released := bs.ReleaseExpired(NewAllowlistLookup(nil))
	if released != 1 {
		t.Errorf("ReleaseExpired() = %d, want 1", released)
	}
	bs.mu.RLock()
	_, stillPresent := bs.temporary["192.0.2.1"]
	bs.mu.RUnlock()
	if stillPresent {
		t.Errorf("expired block should have been removed from the store")
	}
}

func TestBlockStore_ReleaseExpired_AllowlistedIP(t *testing.T) {
	bs := NewBlockStore(nil)
	bs.AddTemporary("203.0.113.9", "rate_limit_abuse", time.Hour)

	al := NewAllowlistLookup([]config.AllowlistEntry{
		{CIDR: "203.0.113.9", Description: "reclassified as trusted"},
	})

	released := bs.ReleaseExpired(al)
	if released != 1 {
		t.Fatalf("ReleaseExpired() = %d, want 1 (allowlisted IP should be released)", released)
	}
	if blocked, _ := bs.IsBlocked("203.0.113.9"); blocked {
		t.Errorf("allowlisted IP should have been released from the temporary block store")
	}
}

func TestBlockStore_DownloadedNetset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "firehol_level1.netset")
	content := "# comment line\n\n1.2.3.0/24\n5.6.7.8\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write netset: %v", err)
	}

	bs := NewBlockStore(nil)
	if err := bs.LoadDownloadedNetset(path); err != nil {
		t.Fatalf("LoadDownloadedNetset: %v", err)
	}

	if blocked, _ := bs.IsBlocked("1.2.3.4"); !blocked {
		t.Errorf("1.2.3.4 should be blocked via downloaded netset CIDR")
	}
	if blocked, _ := bs.IsBlocked("5.6.7.8"); !blocked {
		t.Errorf("5.6.7.8 should be blocked via downloaded netset single IP")
	}
	if blocked, _ := bs.IsBlocked("9.9.9.9"); blocked {
		t.Errorf("9.9.9.9 should not be blocked")
	}
}

func TestUpdateBlocklist_DownloadFailureFailsOpenWithNoPreviousList(t *testing.T) {
	old := fireholLevel1URL
	fireholLevel1URL = "http://127.0.0.1:0/unreachable"
	defer func() { fireholLevel1URL = old }()

	bs := NewBlockStore(nil)
	err := UpdateBlocklist(bs, "airports-blocklist-test-nofile")
	if err == nil {
		t.Fatalf("expected an error when download fails and no previous list exists")
	}
}

func TestAllowlistMiddleware_SetsContextFlag(t *testing.T) {
	al := NewAllowlistLookup([]config.AllowlistEntry{
		{CIDR: "192.0.2.0/24", Description: "trusted range"},
	})

	var sawAllowlisted bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAllowlisted = IsAllowlisted(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = "192.0.2.5:1234"
	rw := httptest.NewRecorder()

	AllowlistMiddleware(al)(next).ServeHTTP(rw, req)

	if !sawAllowlisted {
		t.Errorf("expected downstream handler to see IsAllowlisted=true")
	}
}

func TestBlocklistMiddleware_BlocksAndPassesThrough(t *testing.T) {
	bs := NewBlockStore([]config.BlockedIPEntry{
		{CIDR: "198.51.100.0/24", Reason: "test block"},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := BlocklistMiddleware(bs)(next)

	// Blocked IP, API request -> 403 JSON, reason not leaked.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = "198.51.100.7:5555"
	req.Header.Set("Accept", "application/json")
	rw := httptest.NewRecorder()
	mw.ServeHTTP(rw, req)

	if rw.Code != http.StatusForbidden {
		t.Fatalf("blocked request status = %d, want %d", rw.Code, http.StatusForbidden)
	}
	if body := rw.Body.String(); !strings.Contains(body, `"ok":false`) || !strings.Contains(body, `"error":"IP_BLOCKED"`) {
		t.Errorf("body = %q, missing expected envelope fields", body)
	}
	if strings.Contains(rw.Body.String(), "test block") {
		t.Errorf("block reason must never be exposed to the client")
	}

	// Non-blocked IP passes through.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req2.RemoteAddr = "203.0.113.99:5555"
	rw2 := httptest.NewRecorder()
	mw.ServeHTTP(rw2, req2)
	if rw2.Code != http.StatusOK {
		t.Errorf("non-blocked request status = %d, want %d", rw2.Code, http.StatusOK)
	}
}

func TestBlocklistMiddleware_AllowlistBypass(t *testing.T) {
	bs := NewBlockStore([]config.BlockedIPEntry{
		{CIDR: "198.51.100.0/24", Reason: "test block"},
	})
	al := NewAllowlistLookup([]config.AllowlistEntry{
		{CIDR: "198.51.100.0/24", Description: "trusted despite blocklist"},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := AllowlistMiddleware(al)(BlocklistMiddleware(bs)(next))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = "198.51.100.7:5555"
	rw := httptest.NewRecorder()
	chain.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("allowlisted IP should bypass blocklist, got status %d", rw.Code)
	}
}

func TestBlocklistMiddleware_WebPathThemedErrorPage(t *testing.T) {
	bs := NewBlockStore([]config.BlockedIPEntry{
		{CIDR: "198.51.100.0/24", Reason: "test block"},
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := BlocklistMiddleware(bs)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.7:5555"
	req.Header.Set("Accept", "text/html")
	rw := httptest.NewRecorder()
	mw.ServeHTTP(rw, req)

	if rw.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rw.Code, http.StatusForbidden)
	}
	if ct := rw.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	if strings.Contains(rw.Body.String(), "test block") {
		t.Errorf("block reason must never be exposed to the client")
	}
}

