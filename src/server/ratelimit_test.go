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

func TestRateLimitDisabledPassesRequests(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.RateLimit.Enabled = false
	rl := NewRateLimiter(0, 0, cfg, "", nil)
	called := false
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.25:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if !called {
		t.Fatal("disabled limiter blocked the downstream handler")
	}
	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestConfiguredRateLimitPersistsAcrossLimiterRestartWithoutRawIP(t *testing.T) {
	database, err := dbpkg.Open(config.DatabaseConfig{Driver: "sqlite", URL: "file:ratelimit-persistence?mode=memory&cache=shared"}, "airports-test")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	cfg := config.DefaultConfig()
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
	if err := first.SetPersistence(database, nil, false); err != nil {
		t.Fatalf("SetPersistence(first): %v", err)
	}
	if response := request(first); response.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want 204", response.Code)
	}
	second := NewConfiguredRateLimiter(cfg, "", nil)
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
	cache, err := cachepkg.New(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer func() { _ = cache.Close() }()
	failing := &fakeRateLimitCache{Cache: cache, err: errors.New("cache unavailable")}
	cfg := config.DefaultConfig()
	cfg.Server.RateLimit.Read = config.RateLimitBucket{Requests: 1, Window: 60}
	cfg.Server.RateLimit.GlobalBurst = 20
	limiter := NewConfiguredRateLimiter(cfg, "", nil)
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
	if failing.calls != 1 {
		t.Errorf("cache calls after in-process recovery = %d, want 1 (sticky DB fallback)", failing.calls)
	}

	restarted := NewConfiguredRateLimiter(cfg, "", nil)
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

func TestRateLimitMetricsUseBoundedLimitLabels(t *testing.T) {
	metrics := newMetrics(config.DefaultConfig().Server.Metrics, buildInfo{}, time.Now(), "")
	read := config.RateLimitBucket{Requests: 1, Window: 60}
	limiter := &RateLimiter{
		windows:  make(map[string]map[string][]time.Time),
		limiters: make(map[string]*ipLimiter),
		enabled:  true,
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
	metrics := newMetrics(config.DefaultConfig().Server.Metrics, buildInfo{}, time.Now(), "")
	limiter := &RateLimiter{
		windows:  make(map[string]map[string][]time.Time),
		limiters: make(map[string]*ipLimiter),
		enabled:  true,
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
	cfg := config.DefaultConfig()
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
	cfg := config.DefaultConfig()
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
