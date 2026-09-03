package cache

import (
	"context"
	"time"
)

// noopCache backs server.cache.type: none — every read misses, writes are
// discarded. Keeps callers unconditional (no nil checks) when caching is
// disabled entirely.
type noopCache struct{}

func newNoopCache() *noopCache {
	return &noopCache{}
}

func (c *noopCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrNotFound
}

func (c *noopCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (c *noopCache) Delete(_ context.Context, _ string) error {
	return nil
}

func (c *noopCache) GetOrSet(_ context.Context, _ string, _ time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	return loader()
}

func (c *noopCache) Close() error {
	return nil
}
