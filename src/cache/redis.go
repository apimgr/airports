package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/apimgr/airports/src/config"
)

// redisCache backs server.cache.type: valkey or redis (Valkey is wire-
// compatible with the Redis protocol, so both use the same client) per
// AI.md PART 9/12 — the preferred production driver, shared across
// instances and surviving restarts.
type redisCache struct {
	client     *redis.Client
	prefix     string
	defaultTTL time.Duration
	statsMu    sync.Mutex
	hits       uint64
	misses     uint64
	evictions  uint64
	known      map[string]time.Time
}

func newRedisCache(cfg config.CacheConfig, prefix string, defaultTTL time.Duration) (*redisCache, error) {
	timeout, err := parseDurationOrDefault(cfg.Timeout, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid timeout %q: %w", cfg.Timeout, err)
	}

	var opts *redis.Options
	if cfg.URL != "" {
		opts, err = redis.ParseURL(normalizeCacheURL(cfg.URL))
		if err != nil {
			return nil, fmt.Errorf("cache: invalid url: %w", err)
		}
	} else {
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 6379
		}
		opts = &redis.Options{
			Addr:     fmt.Sprintf("%s:%d", host, port),
			Username: cfg.Username,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdle > 0 {
		opts.MinIdleConns = cfg.MinIdle
	}
	opts.DialTimeout = timeout
	opts.ReadTimeout = timeout
	opts.WriteTimeout = timeout

	if cfg.TLS || opts.TLSConfig != nil {
		tlsCfg := opts.TLSConfig
		if tlsCfg == nil {
			tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tlsCfg.InsecureSkipVerify = cfg.TLSSkipVerify
		opts.TLSConfig = tlsCfg
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: connection test failed: %w", err)
	}

	return &redisCache{client: client, prefix: prefix, defaultTTL: defaultTTL, known: make(map[string]time.Time)}, nil
}

// normalizeCacheURL rewrites the valkey:// scheme (AI.md PART 12 URL format)
// to redis:// since Valkey is wire-protocol compatible and go-redis only
// recognizes redis:// and rediss://.
func normalizeCacheURL(url string) string {
	if len(url) >= 9 && url[:9] == "valkey://" {
		return "redis://" + url[9:]
	}
	return url
}

func (c *redisCache) fullKey(key string) string {
	return c.prefix + key
}

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := c.client.Get(ctx, c.fullKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		c.statsMu.Lock()
		c.misses++
		c.statsMu.Unlock()
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.statsMu.Lock()
	c.hits++
	c.statsMu.Unlock()
	return v, nil
}

// MetricsStats reports observed lookups and live key/byte counts scoped to the
// configured cache prefix. Redis has no prefix-scoped counters, so this scans
// only the application's key namespace. Raw keys remain internal and never
// become metric labels.
func (c *redisCache) MetricsStats() MetricsStats {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	seen := make(map[string]time.Time)
	var items, bytes uint64
	var cursor uint64
	complete := true
	for {
		keys, next, err := c.client.Scan(ctx, cursor, redisGlobPrefix(c.prefix)+"*", 500).Result()
		if err != nil {
			complete = false
			break
		}
		if len(keys) > 0 {
			pipe := c.client.Pipeline()
			valueLengths := make([]*redis.IntCmd, len(keys))
			ttls := make([]*redis.DurationCmd, len(keys))
			for i, key := range keys {
				valueLengths[i] = pipe.StrLen(ctx, key)
				ttls[i] = pipe.PTTL(ctx, key)
			}
			_, execErr := pipe.Exec(ctx)
			if execErr != nil && !errors.Is(execErr, redis.Nil) {
				complete = false
				break
			}
			for i, key := range keys {
				if _, duplicate := seen[key]; duplicate {
					continue
				}
				valueLength, valueErr := valueLengths[i].Result()
				pttl, ttlErr := ttls[i].Result()
				if pttl == -2*time.Nanosecond || errors.Is(valueErr, redis.Nil) || errors.Is(ttlErr, redis.Nil) {
					continue
				}
				if valueErr != nil || ttlErr != nil {
					complete = false
					break
				}
				expires := time.Time{}
				if pttl > 0 {
					expires = now.Add(pttl)
				}
				keyItems, keyBytes := addRedisMetricSample(seen, key, expires, valueLength)
				items += keyItems
				bytes += keyBytes
			}
			if !complete {
				break
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	c.recordMetricSnapshot(now, seen, complete)
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return MetricsStats{Hits: c.hits, Misses: c.misses, Evictions: c.evictions, Items: items, Bytes: bytes, HasEvictionCount: complete, HasSize: complete, Enabled: true}
}

func addRedisMetricSample(seen map[string]time.Time, key string, expires time.Time, valueLength int64) (uint64, uint64) {
	if _, exists := seen[key]; exists {
		return 0, 0
	}
	seen[key] = expires
	if valueLength < 0 {
		valueLength = 0
	}
	return 1, uint64(len(key)) + uint64(valueLength)
}

func redisGlobPrefix(prefix string) string {
	var escaped strings.Builder
	escaped.Grow(len(prefix))
	for _, r := range prefix {
		if strings.ContainsRune("*?[]\\", r) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

func (c *redisCache) recordMetricSnapshot(now time.Time, seen map[string]time.Time, complete bool) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	if !complete {
		return
	}
	for key, expiry := range c.known {
		if _, ok := seen[key]; ok {
			continue
		}
		if expiry.IsZero() || now.Before(expiry) {
			c.evictions++
		}
		delete(c.known, key)
	}
	for key, expiry := range seen {
		c.known[key] = expiry
	}
}

func (c *redisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var err error
	switch {
	case ttl > 0:
		err = c.client.Set(ctx, c.fullKey(key), value, ttl).Err()
	case ttl == 0:
		err = c.client.Set(ctx, c.fullKey(key), value, c.defaultTTL).Err()
	default:
		err = c.client.Set(ctx, c.fullKey(key), value, 0).Err()
	}
	if err == nil {
		duration := ttl
		if duration == 0 {
			duration = c.defaultTTL
		}
		expires := time.Time{}
		if duration > 0 {
			expires = time.Now().Add(duration)
		}
		c.statsMu.Lock()
		c.known[c.fullKey(key)] = expires
		c.statsMu.Unlock()
	}
	return err
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	fullKey := c.fullKey(key)
	err := c.client.Del(ctx, fullKey).Err()
	if err == nil {
		c.statsMu.Lock()
		delete(c.known, fullKey)
		c.statsMu.Unlock()
	}
	return err
}

const rateLimitScript = `
local now = tonumber(ARGV[1])
local class_window = tonumber(ARGV[2])
local global_window = tonumber(ARGV[3])
local class_limit = tonumber(ARGV[4])
local global_limit = tonumber(ARGV[5])
local function load_counter(key, window, limit)
  local values = redis.call('HMGET', key, 'count', 'prev_count', 'window_start')
  local count = tonumber(values[1]) or 0
  local previous = tonumber(values[2]) or 0
  local start = tonumber(values[3]) or now
  local elapsed = now - start
  if elapsed >= window then
    local periods = math.floor(elapsed / window)
    if periods == 1 then previous = count else previous = 0 end
    count = 0
    start = start + periods * window
  end
  local elapsed_window = now - start
  local weighted = previous * (1 - elapsed_window / window) + count
  local reset = start + window + 1
  if count < limit and previous > 0 then
    local needed = window * (1 - (limit - count) / previous)
    if needed <= elapsed_window then reset = now + 1
    elseif needed < window then reset = start + needed + 1 end
  elseif count >= limit and count > 0 then
    reset = start + window + window * (1 - limit / count) + 1
  end
  redis.call('HSET', key, 'count', count, 'prev_count', previous, 'window_start', start)
  redis.call('PEXPIRE', key, math.max(window * 3, 60000))
  return count, weighted, reset
end
local class_count, class_weight, class_reset = load_counter(KEYS[1], class_window, class_limit)
local global_count, global_weight, global_reset = load_counter(KEYS[2], global_window, global_limit)
local allowed = 0
if class_weight < class_limit and global_weight < global_limit then
  class_count = class_count + 1
  global_count = global_count + 1
  redis.call('HSET', KEYS[1], 'count', class_count)
  redis.call('HSET', KEYS[2], 'count', global_count)
  class_weight = class_weight + 1
  global_weight = global_weight + 1
  allowed = 1
end
return {allowed, math.ceil(class_weight), math.ceil(global_weight), class_reset, global_reset}
`

// AllowRateLimit atomically enforces weighted current/previous windows in Redis.
// Keys contain only a SHA-256 identity pseudonym.
func (c *redisCache) AllowRateLimit(ctx context.Context, identity, class string, now time.Time, classWindow, globalWindow time.Duration, classLimit, globalLimit int) (bool, int, int, time.Time, time.Time, error) {
	classKey := c.fullKey("ratelimit:{" + identity + "}:" + class)
	globalKey := c.fullKey("ratelimit:{" + identity + "}:global")
	nowMillis := now.UnixMilli()
	values, err := c.client.Eval(ctx, rateLimitScript, []string{classKey, globalKey}, nowMillis, classWindow.Milliseconds(), globalWindow.Milliseconds(), classLimit, globalLimit).Int64Slice()
	if err != nil {
		return false, 0, 0, time.Time{}, time.Time{}, err
	}
	if len(values) != 5 {
		return false, 0, 0, time.Time{}, time.Time{}, fmt.Errorf("cache: unexpected rate-limit script result")
	}
	reset := func(value int64) time.Time {
		if value == 0 {
			return time.Time{}
		}
		return time.UnixMilli(value)
	}
	return values[0] == 1, int(values[1]), int(values[2]), reset(values[3]), reset(values[4]), nil
}

const rateLimitStatusScript = `
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local values = redis.call('HMGET', KEYS[1], 'count', 'prev_count', 'window_start')
local count = tonumber(values[1]) or 0
local previous = tonumber(values[2]) or 0
local start = tonumber(values[3]) or now
local elapsed = now - start
if elapsed >= window then
  local periods = math.floor(elapsed / window)
  if periods == 1 then previous = count else previous = 0 end
  count = 0
  start = start + periods * window
end
local elapsed_window = now - start
local weighted = previous * (1 - elapsed_window / window) + count
local reset = start + window + 1
if count < limit and previous > 0 then
  local needed = window * (1 - (limit - count) / previous)
  if needed <= elapsed_window then reset = now + 1
  elseif needed < window then reset = start + needed + 1 end
elseif count >= limit and count > 0 then
  reset = start + window + window * (1 - limit / count) + 1
end
redis.call('HSET', KEYS[1], 'count', count, 'prev_count', previous, 'window_start', start)
redis.call('PEXPIRE', KEYS[1], math.max(window * 3, 60000))
return {math.floor(weighted), reset}
`

// CheckRateLimit reads the weighted window without adding an attempt.
func (c *redisCache) CheckRateLimit(ctx context.Context, identity, class string, now time.Time, window time.Duration, limit int) (int, time.Time, error) {
	key := c.fullKey("ratelimit:{" + identity + "}:" + class)
	values, err := c.client.Eval(ctx, rateLimitStatusScript, []string{key}, now.UnixMilli(), window.Milliseconds(), limit).Int64Slice()
	if err != nil {
		return 0, time.Time{}, err
	}
	if len(values) != 2 {
		return 0, time.Time{}, fmt.Errorf("cache: unexpected rate-limit status result")
	}
	return int(values[0]), time.UnixMilli(values[1]), nil
}

func (c *redisCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	if v, err := c.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	v, err := loader()
	if err != nil {
		return nil, err
	}
	if err := c.Set(ctx, key, v, ttl); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *redisCache) Close() error {
	return c.client.Close()
}
