package cache

import (
	"context"
	"time"
)

// RateLimitStore performs an atomic rate-limit decision in a durable cache.
// The identity argument must be a pseudonymous digest, never a raw IP.
type RateLimitStore interface {
	AllowRateLimit(ctx context.Context, identity, class string, now time.Time, classWindow, globalWindow time.Duration, classLimit, globalLimit int) (allowed bool, classCount, globalCount int, classReset, globalReset time.Time, err error)
}

// RateLimitStatusStore reads a bucket without consuming an attempt.
type RateLimitStatusStore interface {
	CheckRateLimit(ctx context.Context, identity, class string, now time.Time, window time.Duration, limit int) (count int, reset time.Time, err error)
}
