package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/config"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRateLimitAlwaysOn(t *testing.T) {
	rl := &RateLimiter{windows: make(map[string]map[string][]time.Time), read: config.RateLimitBucket{Requests: 1, Window: 60}, write: config.RateLimitBucket{Requests: 1, Window: 60}, health: config.RateLimitBucket{Requests: 1, Window: 60}, global: 10}
	calls := 0
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.25:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		want := http.StatusOK
		if i == 1 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Errorf("request %d status = %d, want %d", i+1, response.Code, want)
		}
	}
	if calls != 1 {
		t.Errorf("downstream calls = %d, want 1", calls)
	}
}

func TestAuthFailureLockoutBlocksSubsequentRequests(t *testing.T) {
	now := time.Now()
	ip := "203.0.113.26"
	limiter := &RateLimiter{
		windows:      make(map[string]map[string][]time.Time),
		read:         config.RateLimitBucket{Requests: 20, Window: 900},
		write:        config.RateLimitBucket{Requests: 20, Window: 900},
		health:       config.RateLimitBucket{Requests: 20, Window: 900},
		authFailures: config.RateLimitBucket{Requests: 5, Window: 900},
		global:       100,
	}
	limiter.windows[ip] = map[string][]time.Time{"auth_failures": {now.Add(-5 * time.Minute), now.Add(-4 * time.Minute), now.Add(-3 * time.Minute), now.Add(-2 * time.Minute), now.Add(-time.Minute)}}
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = ip + ":1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if got := len(limiter.windows[ip]["auth_failures"]); got != 5 {
		t.Errorf("lockout consumed auth failure count; got %d, want 5", got)
	}
}

func TestConfiguredRateLimitPersistsAcrossLimiterRestartWithoutRawIP(t *testing.T) {
	database, err := dbpkg.Open(config.DatabaseConfig{Driver: "sqlite", URL: "file:ratelimit-persistence?mode=memory&cache=shared"}, "airports-test")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 1, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 20
	ip := "203.0.113.81"
	request := func(limiter *RateLimiter) *httptest.ResponseRecorder {
		handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
		req.RemoteAddr = net.JoinHostPort(ip, "1234")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	first := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = first.ShutdownRateLimitCounters() }()
	if err := first.SetPersistence(database, nil, false); err != nil {
		t.Fatalf("SetPersistence(first): %v", err)
	}
	if response := request(first); response.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want 204", response.Code)
	}
	if err := first.FlushRateLimitCounters(); err != nil {
		t.Fatalf("flush first limiter: %v", err)
	}
	second := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = second.ShutdownRateLimitCounters() }()
	if err := second.SetPersistence(database, nil, false); err != nil {
		t.Fatalf("SetPersistence(second): %v", err)
	}
	if response := request(second); response.Code != http.StatusTooManyRequests {
		t.Fatalf("request after limiter restart status = %d, want 429", response.Code)
	}
	rows, err := database.Query(`SELECT key FROM rate_limits`)
	if err != nil {
		t.Fatalf("query rate_limits: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatalf("scan rate limit key: %v", err)
		}
		if strings.Contains(key, ip) {
			t.Errorf("rate-limit key contains raw client IP %q", key)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rate-limit keys: %v", err)
	}
}

func TestEnsureRateLimitSchemaIsIdempotent(t *testing.T) {
	database, err := sql.Open("sqlite", "file:ratelimit-schema?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := ensureRateLimitSchema(database); err != nil {
		t.Fatalf("first ensureRateLimitSchema: %v", err)
	}
	if err := ensureRateLimitSchema(database); err != nil {
		t.Fatalf("second ensureRateLimitSchema: %v", err)
	}
}

func TestRateLimitCutoffRetainsFractionalBoundaryBucket(t *testing.T) {
	now := time.Unix(2000, 900_000_000)
	cutoff := rateLimitCutoffUnix(now, time.Minute)
	if cutoff != 1939 {
		t.Fatalf("rateLimitCutoffUnix() = %d, want 1939 to retain the boundary second conservatively", cutoff)
	}
}

type fakeRateLimitCache struct {
	cachepkg.Cache
	err   error
	calls int
}

func (f *fakeRateLimitCache) AllowRateLimit(context.Context, string, string, time.Time, time.Duration, time.Duration, int, int) (bool, int, int, time.Time, time.Time, error) {
	f.calls++
	if f.err != nil {
		return false, 0, 0, time.Time{}, time.Time{}, f.err
	}
	return true, 1, 1, time.Time{}, time.Time{}, nil
}

func TestRedisFailureFallsBackToDBAndKeepsDBAuthoritativeThroughRecovery(t *testing.T) {
	database, err := dbpkg.Open(config.DatabaseConfig{Driver: "sqlite", URL: "file:ratelimit-failover?mode=memory&cache=shared"}, "airports-test")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	cache, err := cachepkg.NewCache(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer func() { _ = cache.Close() }()
	failing := &fakeRateLimitCache{Cache: cache, err: errors.New("cache unavailable")}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 1, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 20
	limiter := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = limiter.ShutdownRateLimitCounters() }()
	if err := limiter.SetPersistence(database, failing, true); err != nil {
		t.Fatalf("SetPersistence: %v", err)
	}
	request := func() *httptest.ResponseRecorder {
		handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
		req.RemoteAddr = "203.0.113.82:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if response := request(); response.Code != http.StatusNoContent {
		t.Fatalf("fallback request status = %d, want 204", response.Code)
	}
	failing.err = nil
	if response := request(); response.Code != http.StatusTooManyRequests {
		t.Fatalf("recovered-cache request status = %d, want 429 from DB authority", response.Code)
	}
	if err := limiter.FlushRateLimitCounters(); err != nil {
		t.Fatalf("flush fallback counters: %v", err)
	}
	if failing.calls != 1 {
		t.Errorf("cache calls after in-process recovery = %d, want 1 (sticky DB fallback)", failing.calls)
	}

	restarted := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = restarted.ShutdownRateLimitCounters() }()
	if err := restarted.SetPersistence(database, failing, true); err != nil {
		t.Fatalf("SetPersistence after restart: %v", err)
	}
	if restarted.cache != nil {
		t.Fatal("Redis cache reselected before the persisted fallback window expired")
	}
	restartedHandler := restarted.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = "203.0.113.82:1234"
	restartedResponse := httptest.NewRecorder()
	restartedHandler.ServeHTTP(restartedResponse, req)
	if restartedResponse.Code != http.StatusTooManyRequests {
		t.Errorf("request after restart during fallback window status = %d, want 429", restartedResponse.Code)
	}
	if failing.calls != 1 {
		t.Errorf("cache calls after restart during fallback window = %d, want 1", failing.calls)
	}

	if _, err := database.Exec(`UPDATE rate_limits SET window_start=?, updated_at=? WHERE key=?`, time.Now().Add(-2*time.Hour).Unix(), time.Now().Unix(), redisFallbackMarkerKey); err != nil {
		t.Fatalf("expire fallback marker: %v", err)
	}
	recovered := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = recovered.ShutdownRateLimitCounters() }()
	if err := recovered.SetPersistence(database, failing, true); err != nil {
		t.Fatalf("SetPersistence after fallback window: %v", err)
	}
	if recovered.cache == nil {
		t.Fatal("Redis cache was not reselected after the fallback window expired")
	}
	if response := func() *httptest.ResponseRecorder {
		handler := recovered.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
		req.RemoteAddr = "203.0.113.82:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}(); response.Code != http.StatusNoContent {
		t.Errorf("request after fallback window status = %d, want 204", response.Code)
	}
	if failing.calls != 2 {
		t.Errorf("cache calls after fallback window = %d, want 2", failing.calls)
	}
}

func TestDatabaseRateLimitCountersFlushAndRejectedRequestWithdrawsIncrement(t *testing.T) {
	database, err := dbpkg.Open(config.DatabaseConfig{Driver: "sqlite", URL: "file:ratelimit-hot-counter?mode=memory&cache=shared"}, "airports-test")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 1, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 20
	limiter := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = limiter.ShutdownRateLimitCounters() }()
	if err := limiter.SetPersistence(database, nil, false); err != nil {
		t.Fatalf("SetPersistence: %v", err)
	}
	now := time.Now()
	allowed, _, _, _, _, err := limiter.allowConfiguredDatabase(database, 20, "203.0.113.99", "read", cfg.Server.RateLimit.Read, time.Minute, now, false, nil)
	if err != nil || !allowed {
		t.Fatalf("first request allowed=%t err=%v, want allowed", allowed, err)
	}
	allowed, _, _, _, _, err = limiter.allowConfiguredDatabase(database, 20, "203.0.113.99", "read", cfg.Server.RateLimit.Read, time.Minute, now.Add(time.Second), false, nil)
	if err != nil || allowed {
		t.Fatalf("second request allowed=%t err=%v, want rejected", allowed, err)
	}
	var beforeFlush int
	if err := database.QueryRow(`SELECT COUNT(*) FROM rate_limits WHERE key LIKE 'v1:%'`).Scan(&beforeFlush); err != nil {
		t.Fatalf("count before flush: %v", err)
	}
	if beforeFlush != 0 {
		t.Fatalf("database rows before flush = %d, want 0", beforeFlush)
	}
	if err := limiter.FlushRateLimitCounters(); err != nil {
		t.Fatalf("FlushRateLimitCounters: %v", err)
	}
	var rows, count int
	if err := database.QueryRow(`SELECT COUNT(*), COALESCE(SUM(count),0) FROM rate_limits WHERE key LIKE 'v1:%'`).Scan(&rows, &count); err != nil {
		t.Fatalf("count after flush: %v", err)
	}
	if rows != 2 || count != 2 {
		t.Fatalf("flushed rows/count = %d/%d, want 2/2 (one class and one global increment)", rows, count)
	}
}

func TestRateLimitMetricsUseBoundedLimitLabels(t *testing.T) {
	metrics := newMetrics(config.DefaultApplicationConfig().Server.Metrics, buildInfo{}, time.Now(), "")
	read := config.RateLimitBucket{Requests: 1, Window: 60}
	limiter := &RateLimiter{
		windows:  make(map[string]map[string][]time.Time),
		limiters: make(map[string]*ipLimiter),
		read:     read,
		write:    config.RateLimitBucket{Requests: 20, Window: 60},
		health:   config.RateLimitBucket{Requests: 20, Window: 60},
		global:   20,
		metrics:  metrics,
	}
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
		req.RemoteAddr = "203.0.113.25:1234"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if got := testutil.ToFloat64(metrics.rateLimitRequestsTotal.WithLabelValues("per_ip", "allowed")); got != 1 {
		t.Errorf("per_ip allowed count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.rateLimitRequestsTotal.WithLabelValues("per_ip", "limited")); got != 1 {
		t.Errorf("per_ip limited count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.rateLimitBlockedTotal.WithLabelValues("per_ip")); got != 1 {
		t.Errorf("per_ip blocked count = %v, want 1", got)
	}
}

func TestRateLimitMetricsIdentifyGlobalCeiling(t *testing.T) {
	metrics := newMetrics(config.DefaultApplicationConfig().Server.Metrics, buildInfo{}, time.Now(), "")
	limiter := &RateLimiter{
		windows:  make(map[string]map[string][]time.Time),
		limiters: make(map[string]*ipLimiter),
		read:     config.RateLimitBucket{Requests: 20, Window: 60},
		write:    config.RateLimitBucket{Requests: 20, Window: 60},
		health:   config.RateLimitBucket{Requests: 20, Window: 60},
		global:   1,
		metrics:  metrics,
	}
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
		req.RemoteAddr = "203.0.113.26:1234"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if got := testutil.ToFloat64(metrics.rateLimitRequestsTotal.WithLabelValues("global", "limited")); got != 1 {
		t.Errorf("global limited count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.rateLimitBlockedTotal.WithLabelValues("global")); got != 1 {
		t.Errorf("global blocked count = %v, want 1", got)
	}
}

func TestConfiguredRateLimitUsesRouteBuckets(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 1, Window: 60}
	cfg.Server.RateLimit.Write = config.RateLimitBucket{Requests: 2, Window: 60}
	cfg.Server.RateLimit.Health = config.RateLimitBucket{Requests: 3, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 20
	rl := NewConfiguredRateLimiter(cfg, "", nil)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name   string
		method string
		path   string
		count  int
		limit  int
	}{
		{name: "read", method: http.MethodGet, path: "/api/v1/airports", count: 2, limit: 1},
		{name: "write", method: http.MethodPost, path: "/api/v1/airports", count: 3, limit: 2},
		{name: "health", method: http.MethodGet, path: "/api/v1/server/healthz", count: 4, limit: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.count; i++ {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				req.RemoteAddr = tc.name + ".example:1234"
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				want := http.StatusNoContent
				if i >= tc.limit {
					want = http.StatusTooManyRequests
				}
				if response.Code != want {
					t.Errorf("request %d status = %d, want %d", i+1, response.Code, want)
				}
				if i >= tc.limit && response.Header().Get("X-RateLimit-Limit") != fmt.Sprint(tc.limit) {
					t.Errorf("rate limit header = %q, want %d", response.Header().Get("X-RateLimit-Limit"), tc.limit)
				}
			}
		})
	}
}

func TestConfiguredRateLimitAppliesGlobalCeiling(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 10, Window: 60}
	cfg.Server.RateLimit.Write = config.RateLimitBucket{Requests: 10, Window: 60}
	cfg.Server.RateLimit.Health = config.RateLimitBucket{Requests: 10, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 2
	handler := NewConfiguredRateLimiter(cfg, "", nil).Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i, path := range []string{"/api/v1/airports", "/api/v1/airports", "/api/v1/server/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "203.0.113.88:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		want := http.StatusNoContent
		if i == 2 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Errorf("request %d status = %d, want %d", i+1, response.Code, want)
		}
	}
}
