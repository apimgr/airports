package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	dbmetrics "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/notify"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

// requestFloodWindow bounds how recent rate-limit rejections must be to
// count toward the abuse-detection "10x rate limit in short burst" trigger
// (AI.md PART 11 "Abuse Detection"); a gap longer than this resets the
// per-IP rejection counter rather than accumulating forever.
const requestFloodWindow = time.Minute

const redisFallbackMarkerKey = "v1:backend:redis_fallback"

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
	windows  map[string]map[string][]time.Time
	r        rate.Limit
	b        int
	enabled  bool
	read     config.RateLimitBucket
	write    config.RateLimitBucket
	health   config.RateLimitBucket
	global   int

	cfg                *config.Config
	configDir          string
	blockStore         *BlockStore
	metrics            *appMetrics
	db                 *sql.DB
	cache              cachepkg.RateLimitStore
	configuredCache    cachepkg.RateLimitStore
	fallbackFromRedis  bool
	lastStorageWarning time.Time
	debugLogCache      atomic.Bool
	debugLogDB         func(string, []any, time.Duration, int64, error)
}

// NewConfiguredRateLimiter builds per-IP sliding-window buckets from
// server.rate_limit. Persistence is attached by Server.New.
func NewConfiguredRateLimiter(cfg *config.Config, configDir string, blockStore *BlockStore) *RateLimiter {
	defaults := config.DefaultConfig().Server.RateLimit
	limits := defaults
	enabled := true
	if cfg != nil {
		limits = cfg.Server.RateLimit
		enabled = limits.Enabled
	}
	if limits.Read.Requests <= 0 || limits.Read.Window <= 0 {
		limits.Read = defaults.Read
	}
	if limits.Write.Requests <= 0 || limits.Write.Window <= 0 {
		limits.Write = defaults.Write
	}
	if limits.Health.Requests <= 0 || limits.Health.Window <= 0 {
		limits.Health = defaults.Health
	}
	if limits.GlobalBurst <= 0 {
		limits.GlobalBurst = defaults.GlobalBurst
	}
	rl := &RateLimiter{
		limiters:   make(map[string]*ipLimiter),
		windows:    make(map[string]map[string][]time.Time),
		enabled:    enabled,
		read:       limits.Read,
		write:      limits.Write,
		health:     limits.Health,
		global:     limits.GlobalBurst,
		cfg:        cfg,
		configDir:  configDir,
		blockStore: blockStore,
	}
	rl.debugLogCache.Store(cfg != nil && cfg.Server.Debug.LogCache)
	go rl.cleanupLoop()
	return rl
}

// SetPersistence uses the configured durable cache when it supports atomic
// rate-limit operations, otherwise it persists counters in server.db.
func (rl *RateLimiter) SetPersistence(database *sql.DB, cache cachepkg.Cache, cacheConfigured bool) error {
	var databaseErr error
	databaseReady := false
	retention := rl.maxRateLimitWindow()
	if database != nil {
		if err := ensureRateLimitSchema(database); err != nil {
			databaseErr = err
		} else {
			databaseReady = true
			rl.mu.Lock()
			rl.db = database
			rl.mu.Unlock()
			if err := cleanupRateLimitRows(database, time.Now().Add(-retention).Unix(), 500); err != nil {
				databaseErr = err
			}
		}
	}
	fallbackActive := false
	if databaseReady {
		active, err := redisFallbackIsActive(database, time.Now(), retention, nil)
		if err != nil {
			databaseErr = err
			fallbackActive = true
		} else {
			fallbackActive = active
		}
	}
	if fallbackActive {
		rl.mu.Lock()
		rl.fallbackFromRedis = true
		rl.mu.Unlock()
	}
	if cacheConfigured {
		if store, ok := cache.(cachepkg.RateLimitStore); ok {
			rl.mu.Lock()
			rl.configuredCache = store
			rl.mu.Unlock()
			if !fallbackActive {
				rl.mu.Lock()
				rl.cache = store
				rl.mu.Unlock()
				return nil
			}
		}
	}
	if databaseReady {
		return nil
	}
	return databaseErr
}

func (rl *RateLimiter) maxRateLimitWindow() time.Duration {
	maxWindow := time.Minute
	for _, seconds := range []int{rl.read.Window, rl.write.Window, rl.health.Window, 60} {
		if duration := time.Duration(seconds) * time.Second; duration > maxWindow {
			maxWindow = duration
		}
	}
	return maxWindow
}

func redisFallbackIsActive(database *sql.DB, now time.Time, maxWindow time.Duration, debugLog func(string, []any, time.Duration, int64, error)) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var lastFallback int64
	started := time.Now()
	err := database.QueryRowContext(ctx, `SELECT window_start FROM rate_limits WHERE key=?`, redisFallbackMarkerKey).Scan(&lastFallback)
	dbmetrics.ObserveQuery("select", "rate_limits", started, err)
	if debugLog != nil {
		rows := int64(0)
		if err == nil {
			rows = 1
		}
		debugLog(`SELECT window_start FROM rate_limits WHERE key=?`, []any{redisFallbackMarkerKey}, time.Since(started), rows, err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// The marker is second-granular, so retain DB authority for one extra
	// second to avoid resuming Redis before a live fallback bucket expires.
	return now.Sub(time.Unix(lastFallback, 0)) <= maxWindow+time.Second, nil
}

func ensureRateLimitSchema(database *sql.DB) error {
	query := `CREATE TABLE IF NOT EXISTS rate_limits (
		key TEXT PRIMARY KEY,
		count INTEGER NOT NULL DEFAULT 1,
		window_start INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
	)`
	started := time.Now()
	result, err := database.Exec(query)
	logRateLimitSQL(query, 0, started, result, err)
	if err != nil {
		return fmt.Errorf("rate limit: create schema: %w", err)
	}
	query = `CREATE INDEX IF NOT EXISTS idx_rate_limits_window ON rate_limits(window_start)`
	started = time.Now()
	result, err = database.Exec(query)
	logRateLimitSQL(query, 0, started, result, err)
	if err != nil {
		return fmt.Errorf("rate limit: create window index: %w", err)
	}
	return nil
}

func cleanupRateLimitRows(database *sql.DB, before int64, batch int) error {
	if batch <= 0 || batch > 5000 {
		batch = 500
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	query := `DELETE FROM rate_limits WHERE key IN (
		SELECT key FROM rate_limits WHERE window_start < ? ORDER BY window_start LIMIT ?
	)`
	result, err := database.ExecContext(ctx, query, before, batch)
	logRateLimitSQL(query, 2, started, result, err)
	dbmetrics.ObserveQuery("delete", "rate_limits", started, err)
	return err
}

func logRateLimitSQL(query string, argumentCount int, started time.Time, result sql.Result, err error) {
	rows := int64(0)
	if result != nil {
		if affected, resultErr := result.RowsAffected(); resultErr == nil {
			rows = affected
		}
	}
	dbmetrics.LogQuery(query, argumentCount, started, rows, err)
}

// NewRateLimiter creates a new per-IP rate limiter.
// r = requests per second; b = burst size. cfg/configDir/blockStore wire up
// the request-flood abuse-detection auto-block/auto-alert trigger (AI.md
// PART 11 "Abuse Detection"); pass nil/"" /nil to disable it (e.g. tests
// that only exercise plain rate limiting).
func NewRateLimiter(r rate.Limit, b int, cfg *config.Config, configDir string, blockStore *BlockStore) *RateLimiter {
	enabled := true
	if cfg != nil {
		enabled = cfg.Server.RateLimit.Enabled
	}
	rl := &RateLimiter{
		limiters:   make(map[string]*ipLimiter),
		r:          r,
		b:          b,
		enabled:    enabled,
		cfg:        cfg,
		configDir:  configDir,
		blockStore: blockStore,
	}
	rl.debugLogCache.Store(cfg != nil && cfg.Server.Debug.LogCache)
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

// allowConfigured applies the route-class bucket and the absolute per-IP
// ceiling, returning the active limit, remaining requests, and reset time.
func (rl *RateLimiter) allowConfigured(ip, method, requestPath string, now time.Time) (bool, int, int, time.Time, string) {
	class, bucket := "read", rl.read
	if isHealthRateLimitPath(requestPath) {
		class, bucket = "health", rl.health
	} else if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
		class, bucket = "write", rl.write
	}
	window := time.Duration(bucket.Window) * time.Second
	rl.mu.Lock()
	cacheStore, database, fallbackFromRedis := rl.cache, rl.db, rl.fallbackFromRedis
	rl.mu.Unlock()
	if cacheStore == nil && fallbackFromRedis && database != nil {
		active, err := redisFallbackIsActive(database, now, rl.maxRateLimitWindow(), rl.debugLogDB)
		if err == nil && !active {
			rl.mu.Lock()
			if rl.cache == nil {
				rl.cache = rl.configuredCache
				rl.fallbackFromRedis = false
			}
			cacheStore, fallbackFromRedis = rl.cache, rl.fallbackFromRedis
			rl.mu.Unlock()
		}
	}
	if cacheStore != nil {
		identity := fmt.Sprintf("%x", sha256.Sum256([]byte(ip)))
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		started := time.Now()
		allowed, classCount, globalCount, classOldest, globalOldest, err := cacheStore.AllowRateLimit(ctx, identity, class, now, window, time.Minute, bucket.Requests, rl.global)
		cancel()
		rl.logRateLimitCache(identity, class, allowed, time.Since(started), err)
		if err == nil {
			if allowed {
				return true, bucket.Requests, bucket.Requests - classCount, now.Add(window), "per_ip"
			}
			limit, interval, limitType := bucket.Requests, window, "per_ip"
			oldest := classOldest
			if globalCount >= rl.global {
				limit, interval, limitType, oldest = rl.global, time.Minute, "global", globalOldest
			}
			reset := now.Add(interval)
			if !oldest.IsZero() {
				reset = oldest.Add(interval)
			}
			return false, limit, 0, reset, limitType
		}
		rl.warnStorageFallback("cache", err)
		rl.mu.Lock()
		rl.cache = nil
		rl.fallbackFromRedis = true
		rl.mu.Unlock()
		fallbackFromRedis = true
	}
	if database != nil {
		allowed, limit, remaining, reset, limitType, err := allowConfiguredDatabase(database, rl.global, ip, class, bucket, window, now, fallbackFromRedis, rl.debugLogDB)
		if err == nil {
			return allowed, limit, remaining, reset, limitType
		}
		rl.warnStorageFallback("database", err)
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	byClass := rl.windows[ip]
	if byClass == nil {
		byClass = make(map[string][]time.Time)
		rl.windows[ip] = byClass
	}
	classTimes := recentRateLimitTimes(byClass[class], now.Add(-window))
	globalTimes := recentRateLimitTimes(byClass["global"], now.Add(-time.Minute))
	byClass[class] = classTimes
	byClass["global"] = globalTimes
	limit, active, interval := bucket.Requests, classTimes, window
	limitType := "per_ip"
	if len(globalTimes) >= rl.global {
		limit, active, interval = rl.global, globalTimes, time.Minute
		limitType = "global"
	} else if len(classTimes) >= bucket.Requests {
		// Route-class limit is the rejection source.
	} else {
		byClass[class] = append(classTimes, now)
		byClass["global"] = append(globalTimes, now)
		remaining := bucket.Requests - len(classTimes) - 1
		return true, bucket.Requests, remaining, now.Add(window), limitType
	}
	reset := now.Add(interval)
	if len(active) > 0 {
		reset = active[0].Add(interval)
	}
	return false, limit, 0, reset, limitType
}

func (rl *RateLimiter) logRateLimitCache(key, operation string, allowed bool, duration time.Duration, err error) {
	if rl == nil || !rl.debugLogCache.Load() || !mode.IsDebugEnabled() {
		return
	}
	attrs := []any{
		"operation", "allow_rate_limit_" + operation,
		"key", key,
		"hit", err == nil,
		"allowed", allowed,
		"duration_us", duration.Microseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	slog.Debug("cache", attrs...)
}

func (rl *RateLimiter) setDebugLogCache(enabled bool) {
	if rl != nil {
		rl.debugLogCache.Store(enabled)
	}
}

func (rl *RateLimiter) warnStorageFallback(backend string, err error) {
	now := time.Now()
	rl.mu.Lock()
	shouldLog := rl.lastStorageWarning.IsZero() || now.Sub(rl.lastStorageWarning) >= time.Minute
	if shouldLog {
		rl.lastStorageWarning = now
	}
	rl.mu.Unlock()
	if shouldLog {
		scope := "cross-process"
		if backend == "cache" {
			scope = "cross-instance"
		}
		log.Printf("Warning: persistent %s rate-limit operation failed (%s); %s protection is unavailable during fallback", backend, rateLimitStorageErrorKind(err), scope)
	}
}

func rateLimitStorageErrorKind(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "connection") || strings.Contains(message, "closed") || strings.Contains(message, "busy") {
		return "unavailable"
	}
	return "storage_error"
}

func allowConfiguredDatabase(database *sql.DB, globalLimit int, ip, class string, bucket config.RateLimitBucket, window time.Duration, now time.Time, persistFallbackMarker bool, debugLog func(string, []any, time.Duration, int64, error)) (bool, int, int, time.Time, string, error) {
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(ip)))
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, time.Time{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if persistFallbackMarker {
		started := time.Now()
		query := `INSERT INTO rate_limits (key,count,window_start,updated_at) VALUES (?,1,?,?)
			ON CONFLICT(key) DO UPDATE SET count=1,window_start=excluded.window_start,updated_at=excluded.updated_at`
		result, err := tx.ExecContext(ctx, query, redisFallbackMarkerKey, now.Unix(), now.Unix())
		dbmetrics.ObserveQuery("insert", "rate_limits", started, err)
		if debugLog != nil {
			debugLog(query, []any{redisFallbackMarkerKey, now.Unix(), now.Unix()}, time.Since(started), sqlRowsAffected(result), err)
		}
		if err != nil {
			return false, 0, 0, time.Time{}, "", err
		}
	}
	// SQLite stores one-second buckets. Retain the preceding second when
	// rounding cutoffs so precision can only conservatively extend a limit.
	cutoff := func(d time.Duration) int64 { return rateLimitCutoffUnix(now, d) }
	second := now.Unix()
	keys := make([]string, 0, 2)
	for _, trackedClass := range []string{class, "global"} {
		key := fmt.Sprintf("v1:%s:%s:%d", identity, trackedClass, second)
		keys = append(keys, key)
		started := time.Now()
		query := `INSERT INTO rate_limits (key,count,window_start,updated_at) VALUES (?,1,?,?)
			ON CONFLICT(key) DO UPDATE SET count=count+1,updated_at=excluded.updated_at`
		result, execErr := tx.ExecContext(ctx, query, key, second, second)
		dbmetrics.ObserveQuery("insert", "rate_limits", started, execErr)
		if debugLog != nil {
			debugLog(query, []any{key, second, second}, time.Since(started), sqlRowsAffected(result), execErr)
		}
		if execErr != nil {
			return false, 0, 0, time.Time{}, "", execErr
		}
	}
	classCount, err := persistedRateCount(ctx, tx, identity, class, cutoff(window), debugLog)
	if err != nil {
		return false, 0, 0, time.Time{}, "", err
	}
	globalCount, err := persistedRateCount(ctx, tx, identity, "global", cutoff(time.Minute), debugLog)
	if err != nil {
		return false, 0, 0, time.Time{}, "", err
	}
	allowed := classCount <= bucket.Requests && globalCount <= globalLimit
	limit, interval, limitType, oldestClass := bucket.Requests, window, "per_ip", class
	if globalCount >= globalLimit {
		limit, interval, limitType, oldestClass = globalLimit, time.Minute, "global", "global"
	}
	if allowed {
		if err := tx.Commit(); err != nil {
			return false, 0, 0, time.Time{}, "", err
		}
		return true, bucket.Requests, bucket.Requests - classCount, now.Add(window), limitType, nil
	}
	for _, key := range keys {
		started := time.Now()
		query := `UPDATE rate_limits SET count=count-1 WHERE key=?`
		result, err := tx.ExecContext(ctx, query, key)
		dbmetrics.ObserveQuery("update", "rate_limits", started, err)
		if debugLog != nil {
			debugLog(query, []any{key}, time.Since(started), sqlRowsAffected(result), err)
		}
		if err != nil {
			return false, 0, 0, time.Time{}, "", err
		}
		started = time.Now()
		query = `DELETE FROM rate_limits WHERE key=? AND count<=0`
		result, err = tx.ExecContext(ctx, query, key)
		dbmetrics.ObserveQuery("delete", "rate_limits", started, err)
		if debugLog != nil {
			debugLog(query, []any{key}, time.Since(started), sqlRowsAffected(result), err)
		}
		if err != nil {
			return false, 0, 0, time.Time{}, "", err
		}
	}
	var oldest sql.NullInt64
	started := time.Now()
	query := `SELECT MIN(window_start) FROM rate_limits WHERE key LIKE ? AND window_start > ?`
	err = tx.QueryRowContext(ctx, query,
		"v1:"+identity+":"+oldestClass+":%", cutoff(interval)).Scan(&oldest)
	dbmetrics.ObserveQuery("select", "rate_limits", started, err)
	if debugLog != nil {
		rows := int64(0)
		if err == nil {
			rows = 1
		}
		debugLog(query, []any{"v1:" + identity + ":" + oldestClass + ":%", cutoff(interval)}, time.Since(started), rows, err)
	}
	if err != nil {
		return false, 0, 0, time.Time{}, "", err
	}
	reset := now.Add(interval)
	if oldest.Valid {
		reset = time.Unix(oldest.Int64, 0).Add(interval)
	}
	if err := tx.Commit(); err != nil {
		return false, 0, 0, time.Time{}, "", err
	}
	return false, limit, 0, reset, limitType, nil
}

func persistedRateCount(ctx context.Context, tx *sql.Tx, identity, class string, cutoff int64, debugLog func(string, []any, time.Duration, int64, error)) (int, error) {
	var count int
	started := time.Now()
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(count),0) FROM rate_limits WHERE key LIKE ? AND window_start > ?`,
		"v1:"+identity+":"+class+":%", cutoff).Scan(&count)
	dbmetrics.ObserveQuery("select", "rate_limits", started, err)
	if debugLog != nil {
		rows := int64(0)
		if err == nil {
			rows = 1
		}
		debugLog(`SELECT COALESCE(SUM(count),0) FROM rate_limits WHERE key LIKE ? AND window_start > ?`, []any{"v1:" + identity + ":" + class + ":%", cutoff}, time.Since(started), rows, err)
	}
	return count, err
}

func sqlRowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return -1
	}
	return rows
}

func rateLimitCutoffUnix(now time.Time, window time.Duration) int64 {
	return now.Add(-window).Unix() - 1
}

func recentRateLimitTimes(times []time.Time, after time.Time) []time.Time {
	first := 0
	for first < len(times) && !times[first].After(after) {
		first++
	}
	return append([]time.Time(nil), times[first:]...)
}

func isHealthRateLimitPath(requestPath string) bool {
	return requestPath == "/server/healthz" || requestPath == "/healthz" || requestPath == "/api/healthz" || strings.HasSuffix(requestPath, "/server/healthz")
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

// abuseNotifyRecipient follows AI.md PART 12's abuse -> general -> admin
// recipient chain, with legacy config values retained as a migration fallback.
func abuseNotifyRecipient(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	for _, candidate := range []string{
		cfg.Server.Contact.Abuse.Email,
		cfg.Server.Contact.General.Email,
		cfg.Server.Privacy.Contact.Abuse,
		cfg.Server.Privacy.Contact.General,
		cfg.Server.Contact.ResolveEmail("admin"),
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
		for ip, buckets := range rl.windows {
			for class, times := range buckets {
				window := time.Minute
				switch class {
				case "read":
					window = time.Duration(rl.read.Window) * time.Second
				case "write":
					window = time.Duration(rl.write.Window) * time.Second
				case "health":
					window = time.Duration(rl.health.Window) * time.Second
				}
				buckets[class] = recentRateLimitTimes(times, time.Now().Add(-window))
				if len(buckets[class]) == 0 {
					delete(buckets, class)
				}
			}
			if len(buckets) == 0 {
				delete(rl.windows, ip)
			}
		}
		rl.mu.Unlock()
		rl.mu.Lock()
		database := rl.db
		rl.mu.Unlock()
		if database != nil {
			retention := time.Minute
			for _, seconds := range []int{rl.read.Window, rl.write.Window, rl.health.Window, 60} {
				if duration := time.Duration(seconds) * time.Second; duration > retention {
					retention = duration
				}
			}
			if err := cleanupRateLimitRows(database, time.Now().Add(-retention).Unix(), 500); err != nil {
				log.Printf("Warning: rate-limit stale-row cleanup failed (%s)", rateLimitStorageErrorKind(err))
			}
		}
	}
}

// Middleware returns an HTTP middleware that rate-limits by remote IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.enabled {
			next.ServeHTTP(w, r)
			return
		}
		// Allowlisted IPs bypass rate limiting per AI.md PART 11 "IP Block
		// Management" allowlist-bypass table (monitoring/CI/load balancers).
		if IsAllowlisted(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		ip := realIP(r)
		var allowed bool
		limit, remaining := 0, 0
		var reset time.Time
		limitType := "per_ip"
		if rl.windows != nil {
			allowed, limit, remaining, reset, limitType = rl.allowConfigured(ip, r.Method, r.URL.Path, time.Now())
		} else {
			limiter := rl.getLimiter(ip)
			allowed = limiter.Allow()
			limit = int(rl.r * 60)
			remaining = int(limiter.Tokens())
			reset = time.Now().Add(time.Minute)
		}
		if rl.metrics != nil {
			rl.metrics.recordRateLimit(limitType, allowed)
		}
		if !allowed {
			rl.recordRejection(ip)
			resetUnix := reset.Unix()
			retryAfter := int(math.Ceil(time.Until(reset).Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetUnix))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			lang := i18n.FromContext(r.Context())
			resp := ErrorResponse{
				OK:      false,
				Error:   "RATE_LIMITED",
				Message: i18n.T(lang, "errors.rate_limited"),
				Details: map[string]interface{}{},
			}
			if err := encodeJSON(w, resp); err != nil {
				log.Printf("RateLimiter.Middleware: encode failed: %v", err)
			}
			return
		}
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
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
	if TorCircuitID(r) != "" {
		return TorClientIdentity(r)
	}
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
