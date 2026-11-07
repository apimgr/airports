package cache

import (
	"context"
	"time"
)

// RateLimitStore performs an atomic sliding-window decision in a durable
// cache. The identity argument must be a pseudonymous digest, never a raw IP.
type RateLimitStore interface {
	AllowRateLimit(ctx context.Context, identity, class string, now time.Time, classWindow, globalWindow time.Duration, classLimit, globalLimit int) (allowed bool, classCount, globalCount int, classOldest, globalOldest time.Time, err error)
}
