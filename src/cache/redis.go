package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return v, nil
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
