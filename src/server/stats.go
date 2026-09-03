package server

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// statsHourBuckets is the number of hourly buckets kept for the rolling
// 24-hour request counter. One extra bucket beyond 24 is kept so a bucket
// that is currently "in progress" is never mistaken for a fully-elapsed,
// stale hour when Requests24h() sums immediately after an hour boundary.
const statsHourBuckets = 25

// statsCollector tracks the lightweight, public-safe request counters
// surfaced by /server/healthz (AI.md PART 13 "stats.*"). It deliberately
// does not read back Prometheus counters (appMetrics) — CounterVec values
// are not cheaply queryable as a single scalar — and instead keeps its own
// minimal state: a lifetime total, a ring buffer of hourly buckets for the
// rolling 24h window, and a live active-connection gauge.
type statsCollector struct {
	requestsTotal int64 // atomic
	activeConns   int64 // atomic

	mu      sync.Mutex
	buckets [statsHourBuckets]int64 // indexed by (hour epoch) % statsHourBuckets
	stamps  [statsHourBuckets]int64 // hour epoch (hours since Unix epoch) owning each bucket slot
}

// newStatsCollector returns a ready-to-use, zero-valued statsCollector.
func newStatsCollector() *statsCollector {
	return &statsCollector{}
}

// currentHourEpoch returns the number of whole hours since the Unix epoch,
// used both as the bucket-selection key and the staleness check.
func currentHourEpoch(t time.Time) int64 {
	return t.UTC().Unix() / 3600
}

// recordRequest increments the lifetime total and the current hour's
// bucket. Called exactly once per request by the stats middleware.
func (c *statsCollector) recordRequest() {
	atomic.AddInt64(&c.requestsTotal, 1)

	hour := currentHourEpoch(time.Now())
	slot := int(hour % statsHourBuckets)

	c.mu.Lock()
	if c.stamps[slot] != hour {
		// Either the slot is unused (zero value) or it belongs to a
		// previous cycle through the ring — reset it for this hour.
		c.stamps[slot] = hour
		c.buckets[slot] = 0
	}
	c.buckets[slot]++
	c.mu.Unlock()
}

// requestsTotalCount returns the lifetime request count.
func (c *statsCollector) requestsTotalCount() int64 {
	return atomic.LoadInt64(&c.requestsTotal)
}

// requests24h sums every bucket whose stamped hour falls within the last
// 24 hours of the given reference time, ignoring stale/unused slots.
func (c *statsCollector) requests24h(now time.Time) int64 {
	currentHour := currentHourEpoch(now)
	cutoff := currentHour - 24

	var total int64
	c.mu.Lock()
	for i := 0; i < statsHourBuckets; i++ {
		if c.stamps[i] > cutoff && c.stamps[i] <= currentHour {
			total += c.buckets[i]
		}
	}
	c.mu.Unlock()
	return total
}

// activeConnections returns the current number of in-flight requests.
func (c *statsCollector) activeConnections() int {
	return int(atomic.LoadInt64(&c.activeConns))
}

// middleware wraps every request: increments the active-connection gauge
// on entry, decrements it on exit (via defer, so it is decremented even if
// a downstream handler panics and is recovered by middleware.Recoverer
// upstream in the chain), and records the request in the rolling counters.
func (c *statsCollector) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&c.activeConns, 1)
		defer atomic.AddInt64(&c.activeConns, -1)

		c.recordRequest()
		next.ServeHTTP(w, r)
	})
}
