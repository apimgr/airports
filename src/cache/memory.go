package cache

import (
	"context"
	"sync"
	"time"
)

// memoryCache is the default in-process backend per AI.md PART 9 "Cache
// Drivers" (memory: development, small deployments — lost on restart).
type memoryCache struct {
	prefix     string
	defaultTTL time.Duration

	mu      sync.RWMutex
	entries map[string]memoryEntry
	hits    uint64
	misses  uint64
	evicts  uint64
	onEvict func(string)

	stop chan struct{}
}

type memoryEntry struct {
	value   []byte
	expires time.Time // zero means no expiry
}

func newMemoryCache(prefix string, defaultTTL time.Duration) *memoryCache {
	c := &memoryCache{
		prefix:     prefix,
		defaultTTL: defaultTTL,
		entries:    make(map[string]memoryEntry),
		stop:       make(chan struct{}),
	}
	go c.evictLoop()
	return c
}

func (c *memoryCache) setEvictionLogger(logger func(string)) {
	c.mu.Lock()
	c.onEvict = logger
	c.mu.Unlock()
}

// evictLoop periodically sweeps expired entries so a long-running process
// does not accumulate unbounded stale keys between reads.
func (c *memoryCache) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case now := <-ticker.C:
			c.mu.Lock()
			for k, e := range c.entries {
				if !e.expires.IsZero() && now.After(e.expires) {
					delete(c.entries, k)
					c.evicts++
					if c.onEvict != nil {
						c.onEvict(k)
					}
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *memoryCache) fullKey(key string) string {
	return c.prefix + key
}

func (c *memoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	entry, ok := c.entries[c.fullKey(key)]
	if !ok {
		c.misses++
		c.mu.Unlock()
		return nil, ErrNotFound
	}
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		delete(c.entries, c.fullKey(key))
		c.misses++
		c.evicts++
		if c.onEvict != nil {
			c.onEvict(c.fullKey(key))
		}
		c.mu.Unlock()
		return nil, ErrNotFound
	}
	c.hits++
	out := make([]byte, len(entry.value))
	copy(out, entry.value)
	c.mu.Unlock()
	return out, nil
}

// MetricsStats returns exact logical entry and value-byte counts, excluding
// entries whose TTL has elapsed even if the periodic sweeper has not removed
// them yet.
func (c *memoryCache) MetricsStats() MetricsStats {
	now := time.Now()
	c.mu.RLock()
	defer c.mu.RUnlock()
	stats := MetricsStats{
		Hits:             c.hits,
		Misses:           c.misses,
		Evictions:        c.evicts,
		HasEvictionCount: true,
		HasSize:          true,
		Enabled:          true,
	}
	for key, entry := range c.entries {
		if !entry.expires.IsZero() && now.After(entry.expires) {
			continue
		}
		stats.Items++
		stats.Bytes += uint64(len(key) + len(entry.value))
	}
	return stats
}

func (c *memoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	entry := memoryEntry{value: append([]byte(nil), value...)}
	switch {
	case ttl > 0:
		entry.expires = time.Now().Add(ttl)
	case ttl == 0 && c.defaultTTL > 0:
		entry.expires = time.Now().Add(c.defaultTTL)
	default:
		// negative ttl (or zero defaultTTL) means no expiry
	}

	c.mu.Lock()
	fullKey := c.fullKey(key)
	if previous, ok := c.entries[fullKey]; ok && !previous.expires.IsZero() && time.Now().After(previous.expires) {
		c.evicts++
		if c.onEvict != nil {
			c.onEvict(fullKey)
		}
	}
	c.entries[fullKey] = entry
	c.mu.Unlock()
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.entries, c.fullKey(key))
	c.mu.Unlock()
	return nil
}

func (c *memoryCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	if v, err := c.Get(ctx, key); err == nil {
		return v, nil
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

func (c *memoryCache) Close() error {
	close(c.stop)
	return nil
}
