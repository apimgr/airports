package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

// TestRequestFloodAutoBlock exercises the AI.md PART 11 "Abuse Detection"
// request-flood trigger: once an IP crosses request_flood.multiplier
// rate-limit rejections within the flood window, it must be auto-blocked
// (when auto_block_ip is true) via the shared BlockStore.
func TestRequestFloodAutoBlock(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Security.AbuseDetection.RequestFlood.Multiplier = 3
	cfg.Server.Security.AbuseDetection.RequestFlood.BlockDuration = "1h"
	cfg.Server.Security.AbuseDetection.AutoBlockIP = true
	cfg.Server.Security.AbuseDetection.AutoAlert = false

	store := NewBlockStore(nil)
	rl := NewRateLimiter(1, 1, cfg, "", store)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	const ip = "203.0.113.99"
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	blocked, block := store.IsBlocked(ip)
	if !blocked {
		t.Fatalf("expected ip %s to be auto-blocked after repeated rate-limit rejections", ip)
	}
	if block.Reason != "request_flood" {
		t.Errorf("block reason = %q, want %q", block.Reason, "request_flood")
	}
	if !block.AutoBlocked {
		t.Error("expected AutoBlocked = true")
	}
}

// TestRequestFloodRespectsAutoBlockToggle verifies that auto_block_ip=false
// suppresses the block action even once the flood threshold is crossed.
func TestRequestFloodRespectsAutoBlockToggle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Security.AbuseDetection.RequestFlood.Multiplier = 2
	cfg.Server.Security.AbuseDetection.AutoBlockIP = false
	cfg.Server.Security.AbuseDetection.AutoAlert = false

	store := NewBlockStore(nil)
	rl := NewRateLimiter(1, 1, cfg, "", store)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	const ip = "203.0.113.100"
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	if blocked, _ := store.IsBlocked(ip); blocked {
		t.Fatal("expected ip to remain unblocked when auto_block_ip is false")
	}
}

// TestRequestFloodWindowReset verifies that rejections separated by more
// than requestFloodWindow do not accumulate toward the flood threshold.
func TestRequestFloodWindowReset(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Security.AbuseDetection.RequestFlood.Multiplier = 3
	cfg.Server.Security.AbuseDetection.AutoBlockIP = true
	cfg.Server.Security.AbuseDetection.AutoAlert = false

	store := NewBlockStore(nil)
	rl := NewRateLimiter(1, 1, cfg, "", store)

	const ip = "203.0.113.101"
	rl.recordRejection(ip)

	rl.mu.Lock()
	entry := rl.limiters[ip]
	entry.windowStart = time.Now().Add(-2 * requestFloodWindow)
	rl.mu.Unlock()

	rl.recordRejection(ip)
	rl.recordRejection(ip)

	if blocked, _ := store.IsBlocked(ip); blocked {
		t.Fatal("expected window reset to prevent premature auto-block")
	}
}
