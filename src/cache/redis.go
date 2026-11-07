package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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

	return &redisCache{client: client, prefix: prefix, defaultTTL: defaultTTL}, nil
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

// MetricsStats reports exact cache-key hits and misses observed by this client.
// go-redis PoolStats.Hits/Misses describe connection reuse, not cache lookups.
// Redis INFO evicted_keys and DBSIZE are server/database-wide, not scoped to
// this cache prefix. Scans cost work proportional to the database and the
// prefix may be shared by other instances; keyspace notifications require
// server-wide configuration and can lose events across disconnects. These
// APIs therefore cannot provide bounded, exact ownership-scoped size/bytes/
// evictions, so those metrics are omitted instead of reporting misleading data.
func (c *redisCache) MetricsStats() MetricsStats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return MetricsStats{Hits: c.hits, Misses: c.misses, Enabled: true}
}

func (c *redisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	switch {
	case ttl > 0:
		return c.client.Set(ctx, c.fullKey(key), value, ttl).Err()
	case ttl == 0:
		return c.client.Set(ctx, c.fullKey(key), value, c.defaultTTL).Err()
	default:
		return c.client.Set(ctx, c.fullKey(key), value, 0).Err()
	}
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.fullKey(key)).Err()
}

const rateLimitScript = `
local now = tonumber(ARGV[1])
local member = ARGV[2]
local class_cutoff = tonumber(ARGV[3])
local global_cutoff = tonumber(ARGV[4])
local class_ttl = tonumber(ARGV[5])
local global_ttl = tonumber(ARGV[6])
local class_limit = tonumber(ARGV[7])
local global_limit = tonumber(ARGV[8])
local sequence = redis.call('INCR', KEYS[3])
local unique_member = member .. ':' .. tostring(sequence)
redis.call('PEXPIRE', KEYS[3], global_ttl)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', class_cutoff)
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', global_cutoff)
local class_count = redis.call('ZCARD', KEYS[1])
local global_count = redis.call('ZCARD', KEYS[2])
local class_oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
local global_oldest = redis.call('ZRANGE', KEYS[2], 0, 0, 'WITHSCORES')
local allowed = 0
if class_count < class_limit and global_count < global_limit then
  redis.call('ZADD', KEYS[1], now, unique_member)
  redis.call('ZADD', KEYS[2], now, unique_member)
  redis.call('PEXPIRE', KEYS[1], class_ttl)
  redis.call('PEXPIRE', KEYS[2], global_ttl)
  class_count = class_count + 1
  global_count = global_count + 1
  allowed = 1
end
return {allowed, class_count, global_count, tonumber(class_oldest[2]) or 0, tonumber(global_oldest[2]) or 0}
`

// AllowRateLimit atomically enforces both the route-class and global sliding
// windows with Redis sorted sets. Keys contain only a SHA-256 IP pseudonym.
func (c *redisCache) AllowRateLimit(ctx context.Context, identity, class string, now time.Time, classWindow, globalWindow time.Duration, classLimit, globalLimit int) (bool, int, int, time.Time, time.Time, error) {
	classKey := c.fullKey("ratelimit:{" + identity + "}:" + class)
	globalKey := c.fullKey("ratelimit:{" + identity + "}:global")
	sequenceKey := c.fullKey("ratelimit:{" + identity + "}:sequence")
	nowMillis := now.UnixMilli()
	member := fmt.Sprintf("%d", now.UnixNano())
	// Redis scores use millisecond precision; retaining the preceding
	// millisecond at the cutoff avoids prematurely expiring a live event.
	classCutoff := now.Add(-classWindow).UnixMilli() - 1
	globalCutoff := now.Add(-globalWindow).UnixMilli() - 1
	classTTL := classWindow.Milliseconds() + time.Minute.Milliseconds()
	globalTTL := globalWindow.Milliseconds() + time.Minute.Milliseconds()
	values, err := c.client.Eval(ctx, rateLimitScript, []string{classKey, globalKey, sequenceKey}, nowMillis, member, classCutoff, globalCutoff, classTTL, globalTTL, classLimit, globalLimit).Int64Slice()
	if err != nil {
		return false, 0, 0, time.Time{}, time.Time{}, err
	}
	if len(values) != 5 {
		return false, 0, 0, time.Time{}, time.Time{}, fmt.Errorf("cache: unexpected rate-limit script result")
	}
	oldest := func(value int64) time.Time {
		if value == 0 {
			return time.Time{}
		}
		return time.UnixMilli(value)
	}
	return values[0] == 1, int(values[1]), int(values[2]), oldest(values[3]), oldest(values[4]), nil
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
