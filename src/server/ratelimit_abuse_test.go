package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func TestAbuseNotifyRecipientFallbackOrder(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*config.Config)
		want  string
	}{
		{
			name: "abuse recipient wins",
			setup: func(cfg *config.Config) {
				cfg.Server.Privacy.Contact.Abuse = "abuse@example.com"
				cfg.Server.Privacy.Contact.General = "general@example.com"
				cfg.Server.Notifications.Email.ReplyTo = "admin@example.com"
			},
			want: "abuse@example.com",
		},
		{
			name: "general recipient is the abuse fallback",
			setup: func(cfg *config.Config) {
				cfg.Server.Privacy.Contact.General = "general@example.com"
				cfg.Server.Notifications.Email.ReplyTo = "admin@example.com"
			},
			want: "general@example.com",
		},
		{
			name: "admin contact is the final fallback",
			setup: func(cfg *config.Config) {
				cfg.Server.Notifications.Email.ReplyTo = "notification-reply@example.com"
				cfg.Server.Notifications.Email.From.Email = "notification-from@example.com"
				cfg.Web.Security.Admin = "admin@example.com"
			},
			want: "admin@example.com",
		},
		{
			name: "notification sender addresses are not admin contact fallbacks",
			setup: func(cfg *config.Config) {
				cfg.Server.Notifications.Email.ReplyTo = "notification-reply@example.com"
				cfg.Server.Notifications.Email.From.Email = "notification-from@example.com"
			},
			want: "",
		},
		{
			name:  "empty config has no recipient",
			setup: func(*config.Config) {},
			want:  "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Server.Privacy.Contact.Abuse = ""
			cfg.Server.Privacy.Contact.General = ""
			cfg.Server.Notifications.Email.ReplyTo = ""
			cfg.Server.Notifications.Email.From.Email = ""
			cfg.Server.Contact.Admin.Email = ""
			cfg.Web.Security.Admin = ""
			test.setup(cfg)
			if got := abuseNotifyRecipient(cfg); got != test.want {
				t.Errorf("abuseNotifyRecipient() = %q, want %q", got, test.want)
			}
		})
	}
	if got := abuseNotifyRecipient(nil); got != "" {
		t.Errorf("abuseNotifyRecipient(nil) = %q, want empty", got)
	}
}

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
