package cache

import (
	"context"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func TestNewNoneType(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "none"})
	if err != nil {
		t.Fatalf("NewCache(none) returned error: %v", err)
	}
	if _, ok := c.(*noopCache); !ok {
		t.Fatalf("expected *noopCache, got %T", c)
	}
}

func TestNewEmptyTypeDefaultsToNoop(t *testing.T) {
	c, err := NewCache(config.CacheConfig{})
	if err != nil {
		t.Fatalf("NewCache(empty) returned error: %v", err)
	}
	if _, ok := c.(*noopCache); !ok {
		t.Fatalf("expected *noopCache for empty type, got %T", c)
	}
}

func TestNewWithDebugWrapsCacheAndPreservesMetrics(t *testing.T) {
	c, err := NewCacheWithDebug(config.CacheConfig{Type: "memory", Prefix: "debug:"}, true)
	if err != nil {
		t.Fatalf("NewCacheWithDebug(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()
	if _, ok := c.(*debugCache); !ok {
		t.Fatalf("expected *debugCache, got %T", c)
	}
	if err := c.Set(context.Background(), "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	value, err := c.Get(context.Background(), "key")
	if err != nil || string(value) != "value" {
		t.Fatalf("Get returned %q, %v; want value", value, err)
	}
	provider, ok := c.(MetricsProvider)
	if !ok || !provider.MetricsStats().Enabled {
		t.Fatalf("debug wrapper did not preserve metrics provider: %T", c)
	}
}

func TestNewUnknownType(t *testing.T) {
	if _, err := NewCache(config.CacheConfig{Type: "bogus"}); err == nil {
		t.Fatal("expected error for unknown cache type")
	}
}

func TestNewInvalidTTL(t *testing.T) {
	if _, err := NewCache(config.CacheConfig{Type: "memory", TTL: "not-a-duration"}); err == nil {
		t.Fatal("expected error for invalid ttl")
	}
}

func TestMemoryCacheGetSetDelete(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatalf("NewCache(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()

	if _, err := c.Get(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing key, got %v", err)
	}

	if err := c.Set(ctx, "user:1", []byte("alice"), time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	v, err := c.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(v) != "alice" {
		t.Fatalf("expected %q, got %q", "alice", v)
	}

	if err := c.Delete(ctx, "user:1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := c.Get(ctx, "user:1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryCacheExpiry(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("NewCache(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if err := c.Set(ctx, "short", []byte("v"), time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := c.Get(ctx, "short"); err != ErrNotFound {
		t.Fatalf("expected expired entry to return ErrNotFound, got %v", err)
	}
}

func TestMemoryCacheNoExpiryWithNegativeTTL(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("NewCache(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if err := c.Set(ctx, "forever", []byte("v"), -1); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if _, err := c.Get(ctx, "forever"); err != nil {
		t.Fatalf("expected no-expiry entry to still be present, got %v", err)
	}
}

func TestMemoryCacheGetOrSet(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("NewCache(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	calls := 0
	loader := func() ([]byte, error) {
		calls++
		return []byte("loaded"), nil
	}

	v, err := c.GetOrSet(ctx, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if string(v) != "loaded" {
		t.Fatalf("expected %q, got %q", "loaded", v)
	}

	v2, err := c.GetOrSet(ctx, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("GetOrSet (cached) returned error: %v", err)
	}
	if string(v2) != "loaded" {
		t.Fatalf("expected %q, got %q", "loaded", v2)
	}
	if calls != 1 {
		t.Fatalf("expected loader to be called once, got %d calls", calls)
	}
}

func TestMemoryCacheDefaultTTLApplied(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "memory", TTL: "1ms"})
	if err != nil {
		t.Fatalf("NewCache(memory) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := c.Get(ctx, "k"); err != ErrNotFound {
		t.Fatalf("expected default TTL to expire entry, got %v", err)
	}
}

func TestNoopCache(t *testing.T) {
	c, err := NewCache(config.CacheConfig{Type: "none"})
	if err != nil {
		t.Fatalf("NewCache(none) returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if _, err := c.Get(ctx, "k"); err != ErrNotFound {
		t.Fatalf("expected noop cache to always miss, got %v", err)
	}

	calls := 0
	v, err := c.GetOrSet(ctx, "k", time.Minute, func() ([]byte, error) {
		calls++
		return []byte("v"), nil
	})
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if string(v) != "v" || calls != 1 {
		t.Fatalf("expected loader called once and value returned, got calls=%d v=%q", calls, v)
	}
}

func TestNewRedisTypeUnreachableFallsBackToError(t *testing.T) {
	_, err := NewCache(config.CacheConfig{
		Type:    "redis",
		Host:    "127.0.0.1",
		Port:    1, // nothing listens here
		Timeout: "50ms",
	})
	if err == nil {
		t.Fatal("expected error connecting to an unreachable redis host")
	}
}

func TestRedisMetricEvictionsExcludeExpiredAndExplicitlyDeletedKeys(t *testing.T) {
	cache := &redisCache{
		hits: 7, misses: 3,
		known: map[string]time.Time{
			"app:live":    {},
			"app:expired": time.Now().Add(-time.Minute),
		},
	}
	now := time.Now()
	cache.recordMetricSnapshot(now, map[string]time.Time{"app:live": {}}, true)
	cache.recordMetricSnapshot(now, map[string]time.Time{}, true)
	if cache.evictions != 1 {
		t.Fatalf("evictions = %d, want one missing non-expired key", cache.evictions)
	}
	if _, ok := cache.known["app:live"]; ok {
		t.Fatal("missing key retained in observed-key set")
	}
}

func TestRedisMetricIncompleteSnapshotDoesNotChangeEvictionBaseline(t *testing.T) {
	cache := &redisCache{
		evictions: 4,
		known:     map[string]time.Time{"app:live": {}},
	}
	cache.recordMetricSnapshot(time.Now(), map[string]time.Time{}, false)
	if cache.evictions != 4 {
		t.Fatalf("incomplete snapshot changed evictions to %d, want 4", cache.evictions)
	}
	if _, ok := cache.known["app:live"]; !ok {
		t.Fatal("incomplete snapshot removed the prior key baseline")
	}
}

func TestRedisMetricSnapshotCountsDuplicateScanKeysOnce(t *testing.T) {
	seen := make(map[string]time.Time)
	firstItems, firstBytes := addRedisMetricSample(seen, "app:key", time.Time{}, 100)
	duplicateItems, duplicateBytes := addRedisMetricSample(seen, "app:key", time.Time{}, 100)
	if firstItems != 1 || firstBytes != 107 || duplicateItems != 0 || duplicateBytes != 0 {
		t.Fatalf("first sample=(%d,%d), duplicate=(%d,%d); want (1,107) then (0,0)", firstItems, firstBytes, duplicateItems, duplicateBytes)
	}
}

func TestNormalizeCacheURL(t *testing.T) {
	if got := normalizeCacheURL("valkey://localhost:6379/0"); got != "redis://localhost:6379/0" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
	if got := normalizeCacheURL("redis://localhost:6379/0"); got != "redis://localhost:6379/0" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}
