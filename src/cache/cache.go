// Package cache implements the cache backend described in AI.md PART 9
// "Caching" and PART 12 "Cache Configuration": an in-process memory cache by
// default, with optional Valkey/Redis backends for production deployments
// that need counters/sessions to survive a restart or be shared across
// instances.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apimgr/airports/src/config"
)

// ErrNotFound is returned by Get when the key does not exist or has expired.
var ErrNotFound = errors.New("cache: key not found")

// Cache is the backend-agnostic interface used throughout the application.
// Keys should follow the hierarchical naming convention from AI.md PART 9
// "Cache Key Naming" (e.g. "rate:api:192.168.1.1", "geoip:asn:1.2.3.4") —
// callers are responsible for building descriptive, colon-separated,
// lowercase keys; the backend applies the configured prefix automatically.
type Cache interface {
	// Get returns the stored value for key, or ErrNotFound if it does not
	// exist or has expired.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores value under key with the given TTL. A zero TTL uses the
	// backend's configured default TTL; a negative TTL means "no expiry".
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error

	// GetOrSet returns the cached value for key if present, otherwise calls
	// loader, stores the result with ttl, and returns it. This is the
	// standard read-through pattern for API response caching (PART 9).
	GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error)

	// Close releases any resources (connections, background goroutines) held
	// by the backend.
	Close() error
}

// New constructs the configured Cache backend. Type "none" or an unknown
// type disables caching entirely by returning a no-op backend so callers
// never need a nil check.
func New(cfg config.CacheConfig) (Cache, error) {
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "airports:"
	}

	defaultTTL, err := parseDurationOrDefault(cfg.TTL, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid ttl %q: %w", cfg.TTL, err)
	}

	switch cfg.Type {
	case "", "none":
		return newNoopCache(), nil
	case "memory":
		return newMemoryCache(prefix, defaultTTL), nil
	case "valkey", "redis":
		return newRedisCache(cfg, prefix, defaultTTL)
	default:
		return nil, fmt.Errorf("cache: unknown type %q (expected none, memory, valkey, or redis)", cfg.Type)
	}
}

// parseDurationOrDefault parses a Go duration string, falling back to def
// when s is empty. Per AI.md PART 5 "Live Reload", invalid config values
// never crash startup — callers surface the error and the caller decides
// whether to fall back.
func parseDurationOrDefault(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	return time.ParseDuration(s)
}
