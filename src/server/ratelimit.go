package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/notify"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

// requestFloodWindow bounds how recent rate-limit rejections must be to
// count toward the abuse-detection "10x rate limit in short burst" trigger
// (AI.md PART 11 "Abuse Detection"); a gap longer than this resets the
// per-IP rejection counter rather than accumulating forever.
const requestFloodWindow = time.Minute

// ipLimiter holds a rate limiter and the last-seen time for an IP, plus the
// request-flood abuse-detection rejection counter.
type ipLimiter struct {
	limiter        *rate.Limiter
	lastSeen       time.Time
	rejections     int
	windowStart    time.Time
	floodTriggered bool
}

// RateLimiter is a per-IP rate limiter store. When cfg/blockStore are set
// (via NewRateLimiter) it also implements the request-flood abuse-detection
// auto-block/auto-alert trigger from AI.md PART 11 "Abuse Detection".
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	r        rate.Limit
	b        int

	cfg        *config.Config
	configDir  string
	blockStore *BlockStore
}

// NewRateLimiter creates a new per-IP rate limiter.
// r = requests per second; b = burst size. cfg/configDir/blockStore wire up
// the request-flood abuse-detection auto-block/auto-alert trigger (AI.md
// PART 11 "Abuse Detection"); pass nil/"" /nil to disable it (e.g. tests
// that only exercise plain rate limiting).
func NewRateLimiter(r rate.Limit, b int, cfg *config.Config, configDir string, blockStore *BlockStore) *RateLimiter {
	rl := &RateLimiter{
		limiters:   make(map[string]*ipLimiter),
		r:          r,
		b:          b,
		cfg:        cfg,
		configDir:  configDir,
		blockStore: blockStore,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.limiters[ip]
	if !ok {
		entry = &ipLimiter{limiter: rate.NewLimiter(rl.r, rl.b)}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// recordRejection tracks a rate-limit rejection for ip and, once it crosses
// server.security.abuse_detection.request_flood.multiplier within
// requestFloodWindow, fires the "Request Flood" auto-block/auto-alert per
// AI.md PART 11 "Abuse Detection". Allowlisted IPs never reach here (the
// Middleware bypasses rate limiting for them entirely), so no allowlist
// check is needed on this path.
func (rl *RateLimiter) recordRejection(ip string) {
	if rl.cfg == nil {
		return
	}
	ad := rl.cfg.Server.Security.AbuseDetection
	multiplier := ad.RequestFlood.Multiplier
	if multiplier <= 0 {
		multiplier = 10
	}

	rl.mu.Lock()
	entry, ok := rl.limiters[ip]
	if !ok {
		// Should not normally happen (getLimiter always creates the entry
		// first), but guard defensively rather than panic.
		entry = &ipLimiter{limiter: rate.NewLimiter(rl.r, rl.b)}
		rl.limiters[ip] = entry
	}

	now := time.Now()
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) > requestFloodWindow {
		entry.windowStart = now
		entry.rejections = 0
		entry.floodTriggered = false
	}
	entry.rejections++

	trigger := !entry.floodTriggered && entry.rejections >= multiplier
	if trigger {
		entry.floodTriggered = true
	}
	rejections := entry.rejections
	rl.mu.Unlock()

	if trigger {
		rl.triggerRequestFlood(ip, rejections, multiplier)
	}
}

// triggerRequestFlood performs the "Request Flood" auto-actions (block IP,
// alert via email) per AI.md PART 11 "Abuse Detection". Both actions are
// individually gated by their own config toggle and never applied to an
// allowlisted IP (the allowlist bypasses auto IP blocking entirely).
func (rl *RateLimiter) triggerRequestFlood(ip string, rejections, multiplier int) {
	ad := rl.cfg.Server.Security.AbuseDetection
	details := fmt.Sprintf("%d rate-limit rejections within %s (threshold: %dx)", rejections, requestFloodWindow, multiplier)

	if ad.AutoBlockIP && rl.blockStore != nil {
		duration, err := time.ParseDuration(ad.RequestFlood.BlockDuration)
		if err != nil || duration <= 0 {
			duration = time.Hour
		}
		rl.blockStore.AddTemporary(ip, "request_flood", duration)
		log.Printf("security.ip_blocked: auto-blocked ip=%s reason=request_flood duration=%s offenses=%d", ip, duration, rejections)
	}

	if ad.AutoAlert {
		if err := notify.Send(rl.cfg, rl.configDir, "security_alert", abuseNotifyRecipient(rl.cfg), map[string]string{
			"event":   "Request flood detected",
			"ip":      ip,
			"details": details,
		}); err != nil {
			log.Printf("Warning: failed to send security_alert notification email: %v", err)
		}
	}
}

// abuseNotifyRecipient mirrors main.go's notifyRecipient (unexported,
// package-local there) so the server package's abuse-detection trigger can
// resolve the same operator notification recipient per AI.md PART 17:
// server.notifications.email.reply_to > server.notifications.email.from.
// email > server.web.security.admin (security.txt contact).
func abuseNotifyRecipient(cfg *config.Config) string {
	for _, candidate := range []string{
		cfg.Server.Notifications.Email.ReplyTo,
		cfg.Server.Notifications.Email.From.Email,
		cfg.Web.Security.Admin,
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// cleanupLoop removes stale limiters every 5 minutes.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.limiters {
			if time.Since(entry.lastSeen) > 10*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that rate-limits by remote IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allowlisted IPs bypass rate limiting per AI.md PART 11 "IP Block
		// Management" allowlist-bypass table (monitoring/CI/load balancers).
		if IsAllowlisted(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		ip := realIP(r)
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			rl.recordRejection(ip)
			reset := time.Now().Add(60 * time.Second).Unix()
			w.Header().Set("Retry-After", "60")
			w.Header().Set("X-RateLimit-Limit", "60")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", reset))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			lang := i18n.FromContext(r.Context())
			resp := ErrorResponse{
				OK:      false,
				Error:   "RATE_LIMITED",
				Message: i18n.T(lang, "errors.rate_limited"),
				Details: map[string]interface{}{},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				log.Printf("RateLimiter.Middleware: encode failed: %v", err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// realIP extracts the real client IP. It prefers the IP resolved by
// middleware.ClientIPFromXFF (walked back past trusted proxy hops per
// AI.md PART 12 "Trusted Proxies"); if the request context has no resolved
// value — meaning the request did not come through a recognized trusted
// proxy — it falls back to the raw TCP peer address (r.RemoteAddr), which
// IS the real client in that case. Proxy headers are never trusted
// directly here, since that would let any client spoof its IP and evade
// rate limiting.
func realIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	// Strip port from RemoteAddr, handling both IPv4 (1.2.3.4:5678) and
	// IPv6 bracket notation ([2001:db8::1]:1234)
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
