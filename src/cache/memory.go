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
	c.mu.RLock()
	entry, ok := c.entries[c.fullKey(key)]
	c.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		c.mu.Lock()
		delete(c.entries, c.fullKey(key))
		c.mu.Unlock()
		return nil, ErrNotFound
	}
	out := make([]byte, len(entry.value))
	copy(out, entry.value)
	return out, nil
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
	c.entries[c.fullKey(key)] = entry
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
