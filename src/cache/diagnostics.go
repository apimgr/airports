package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/apimgr/airports/src/mode"
)

type debugCache struct {
	backend Cache
	enabled atomic.Bool
}

// SetDebugLogging updates a debug cache wrapper when one is installed.
func SetDebugLogging(cache Cache, enabled bool) {
	if logger, ok := cache.(*debugCache); ok {
		logger.enabled.Store(enabled)
	}
}

func (c *debugCache) Get(ctx context.Context, key string) ([]byte, error) {
	started := time.Now()
	value, err := c.backend.Get(ctx, key)
	c.log("get", key, time.Since(started), err == nil, err)
	return value, err
}

func (c *debugCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	started := time.Now()
	err := c.backend.Set(ctx, key, value, ttl)
	c.log("set", key, time.Since(started), err == nil, err)
	return err
}

func (c *debugCache) Delete(ctx context.Context, key string) error {
	started := time.Now()
	err := c.backend.Delete(ctx, key)
	c.log("delete", key, time.Since(started), err == nil, err)
	return err
}

func (c *debugCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	if value, err := c.Get(ctx, key); err == nil {
		return value, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	value, err := loader()
	if err != nil {
		return nil, err
	}
	if err := c.Set(ctx, key, value, ttl); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *debugCache) Close() error {
	return c.backend.Close()
}

func (c *debugCache) MetricsStats() MetricsStats {
	if provider, ok := c.backend.(MetricsProvider); ok {
		return provider.MetricsStats()
	}
	return MetricsStats{}
}

func (c *debugCache) log(operation, key string, duration time.Duration, hit bool, err error) {
	if !c.enabled.Load() || !mode.IsDebugEnabled() {
		return
	}
	attrs := []any{
		"operation", operation,
		"key_hash", cacheKeyHash(key),
		"hit", hit,
		"duration_us", duration.Microseconds(),
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		attrs = append(attrs, "error", err.Error())
	}
	slog.Debug("cache", attrs...)
}

func cacheKeyHash(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:8])
}
