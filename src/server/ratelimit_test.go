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
	var previous int
	if err := database.QueryRow(`SELECT prev_count FROM rate_limits LIMIT 0`).Scan(&previous); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("prev_count schema probe err = %v, want sql.ErrNoRows", err)
	}
}

func TestEnsureRateLimitSchemaAddsPreviousCounterToExistingTable(t *testing.T) {
	database, err := sql.Open("sqlite", "file:ratelimit-schema-migration?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = database.Close() }()
	if _, err := database.Exec(`CREATE TABLE rate_limits (key TEXT PRIMARY KEY, count INTEGER NOT NULL DEFAULT 1, window_start INTEGER NOT NULL, updated_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create legacy rate_limits table: %v", err)
	}
	if err := ensureRateLimitSchema(database); err != nil {
		t.Fatalf("migrate rate_limits table: %v", err)
	}
	var previous int
	if err := database.QueryRow(`SELECT prev_count FROM rate_limits LIMIT 0`).Scan(&previous); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("migrated prev_count schema probe err = %v, want sql.ErrNoRows", err)
	}
}

func TestRateWindowCounterUsesWeightedPreviousWindow(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	counter := rateWindowCounter{count: 5, previous: 4, start: start.Unix()}
	if got := counter.weighted(start.Add(30*time.Second), time.Minute); got != 7 {
		t.Errorf("weighted count halfway through window = %v, want 7", got)
	}
	rolled, err := loadRateCounterValue(counter.count, counter.previous, counter.start, start.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("roll counter window: %v", err)
	}
	if rolled.count != 0 || rolled.previous != 5 || rolled.start != start.Add(time.Minute).Unix() {
		t.Errorf("counter after one window = %+v, want count 0, previous 5 and advanced start", rolled)
	}
	if got := rolled.weighted(start.Add(90*time.Second), time.Minute); got != 2.5 {
		t.Errorf("weighted count halfway through previous window = %v, want 2.5", got)
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

func TestRedisFailureFallsBackToProcessLocalCounters(t *testing.T) {
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
		t.Fatalf("first local fallback request status = %d, want 204", response.Code)
	}
	if !limiter.ExternalCacheDegraded() {
		t.Fatal("external cache failure did not mark rate-limit health degraded")
	}
	if response := request(); response.Code != http.StatusTooManyRequests {
		t.Fatalf("second local fallback request status = %d, want 429", response.Code)
	}
	if failing.calls != 1 {
		t.Errorf("cache calls after fallback = %d, want 1 (fallback is sticky until restart)", failing.calls)
	}
	var persistentTables int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='rate_limits'`).Scan(&persistentTables); err != nil {
		t.Fatalf("check persistent rate-limit table: %v", err)
	}
	if persistentTables != 0 {
		t.Errorf("external-cache fallback created %d persistent rate-limit tables, want 0", persistentTables)
	}

	failing.err = nil
	restarted := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = restarted.ShutdownRateLimitCounters() }()
	if err := restarted.SetPersistence(database, failing, true); err != nil {
		t.Fatalf("SetPersistence after restart: %v", err)
	}
	if restarted.ExternalCacheDegraded() || restarted.cache == nil {
		t.Fatal("available external cache was not selected after process restart")
	}
	handler := restarted.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	req.RemoteAddr = "203.0.113.82:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusNoContent {
		t.Errorf("request after process restart status = %d, want 204", response.Code)
	}
}

func TestExternalCacheFallbackPreservesMirroredSecurityCounters(t *testing.T) {
	cache, err := cachepkg.NewCache(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer func() { _ = cache.Close() }()
	fake := &fakeRateLimitCache{Cache: cache}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.RateLimit.AuthFailures = config.RateLimitBucket{Requests: 1, Window: 900}
	limiter := NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = limiter.ShutdownRateLimitCounters() }()
	if err := limiter.SetPersistence(nil, fake, true); err != nil {
		t.Fatalf("SetPersistence: %v", err)
	}
	now := time.Now()
	strict := config.RateLimitBucket{Requests: 1, Window: 900}
	if allowed, _, _, _, _ := limiter.allowConfiguredBucket("owner:abc", "strict", strict, now); !allowed {
		t.Fatal("first strict action was rejected by external cache")
	}
	fake.err = errors.New("cache unavailable")
	if allowed, _, _, _, _ := limiter.allowConfiguredBucket("owner:abc", "strict", strict, now.Add(time.Second)); allowed {
		t.Fatal("strict action exceeded limit after external-cache failure")
	}
	if !limiter.ExternalCacheDegraded() {
		t.Fatal("external-cache failure did not report degraded state")
	}

	// Failed-authentication counters are mirrored too, so fallback cannot lift
	// a lockout accumulated immediately before the cache became unavailable.
	limiter = NewConfiguredRateLimiter(cfg, "", nil)
	defer func() { _ = limiter.ShutdownRateLimitCounters() }()
	fake.err = nil
	if err := limiter.SetPersistence(nil, fake, true); err != nil {
		t.Fatalf("SetPersistence for auth counter: %v", err)
	}
	authFailures := config.RateLimitBucket{Requests: 1, Window: 900}
	if allowed, _, _, _, _ := limiter.allowConfiguredBucket("203.0.113.83", "auth_failures", authFailures, now); !allowed {
		t.Fatal("first failed-auth attempt was rejected by external cache")
	}
	fake.err = errors.New("cache unavailable")
	limiter.allowConfiguredBucket("203.0.113.83", "read", cfg.Server.RateLimit.Read, now.Add(time.Second))
	if locked, _ := limiter.authFailureLocked("203.0.113.83", now.Add(time.Second)); !locked {
		t.Fatal("failed-auth lockout was lifted after external-cache failure")
	}
}

func TestDatabaseRateLimitCountersPersistImmediatelyAndRejectedRequestDoesNotIncrement(t *testing.T) {
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
	allowed, _, _, _, _, err := limiter.allowConfiguredDatabase(database, 20, "203.0.113.99", "read", cfg.Server.RateLimit.Read, time.Minute, now, nil)
	if err != nil || !allowed {
		t.Fatalf("first request allowed=%t err=%v, want allowed", allowed, err)
	}
	allowed, _, _, _, _, err = limiter.allowConfiguredDatabase(database, 20, "203.0.113.99", "read", cfg.Server.RateLimit.Read, time.Minute, now.Add(time.Second), nil)
	if err != nil || allowed {
		t.Fatalf("second request allowed=%t err=%v, want rejected", allowed, err)
	}
	var rows, count int
	if err := database.QueryRow(`SELECT COUNT(*), COALESCE(SUM(count),0) FROM rate_limits WHERE key LIKE 'v1:%'`).Scan(&rows, &count); err != nil {
		t.Fatalf("count persisted after requests: %v", err)
	}
	if rows != 2 || count != 2 {
		t.Fatalf("persisted rows/count = %d/%d, want 2/2 (one class and one global increment)", rows, count)
	}
	var previous int
	if err := database.QueryRow(`SELECT SUM(prev_count) FROM rate_limits WHERE key LIKE 'v1:%'`).Scan(&previous); err != nil || previous != 0 {
		t.Errorf("previous-window count = %d, err=%v; want 0", previous, err)
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
