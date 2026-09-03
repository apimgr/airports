package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestStatsCollectorRecordRequestIncrementsTotal(t *testing.T) {
	c := newStatsCollector()
	if got := c.requestsTotalCount(); got != 0 {
		t.Fatalf("initial requestsTotalCount = %d, want 0", got)
	}

	c.recordRequest()
	c.recordRequest()
	c.recordRequest()

	if got := c.requestsTotalCount(); got != 3 {
		t.Fatalf("requestsTotalCount = %d, want 3", got)
	}
}

func TestStatsCollectorRequests24hSumsCurrentHour(t *testing.T) {
	c := newStatsCollector()
	c.recordRequest()
	c.recordRequest()

	if got := c.requests24h(time.Now()); got != 2 {
		t.Fatalf("requests24h = %d, want 2", got)
	}
}

func TestStatsCollectorRequests24hExcludesStaleBuckets(t *testing.T) {
	c := newStatsCollector()
	now := time.Now().UTC()

	// Simulate a request recorded exactly 30 hours ago — it lands in a
	// bucket slot but must not count toward the rolling 24h window.
	staleHour := currentHourEpoch(now) - 30
	slot := int(staleHour % statsHourBuckets)
	c.stamps[slot] = staleHour
	c.buckets[slot] = 5

	c.recordRequest() // one genuinely current request

	if got := c.requests24h(now); got != 1 {
		t.Fatalf("requests24h = %d, want 1 (stale bucket must be excluded)", got)
	}
}

func TestStatsCollectorRequests24hHandlesRingReuse(t *testing.T) {
	c := newStatsCollector()
	now := time.Now().UTC()
	currentHour := currentHourEpoch(now)

	// Fill every slot with a stale stamp so the ring has wrapped at least
	// once; each stale count must be excluded even though every slot has
	// been written.
	for i := 0; i < statsHourBuckets; i++ {
		c.stamps[i] = currentHour - 100 - int64(i)
		c.buckets[i] = 42
	}

	c.recordRequest()

	if got := c.requests24h(now); got != 1 {
		t.Fatalf("requests24h = %d, want 1", got)
	}
}

func TestStatsCollectorActiveConnections(t *testing.T) {
	c := newStatsCollector()
	if got := c.activeConnections(); got != 0 {
		t.Fatalf("initial activeConnections = %d, want 0", got)
	}

	block := make(chan struct{})
	done := make(chan struct{})
	handler := c.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))

	go func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		close(done)
	}()

	// Poll briefly until the handler has registered as active — avoids a
	// flaky fixed sleep while still bounding the wait.
	deadline := time.Now().Add(2 * time.Second)
	for c.activeConnections() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := c.activeConnections(); got != 1 {
		t.Fatalf("activeConnections during request = %d, want 1", got)
	}

	close(block)
	<-done

	if got := c.activeConnections(); got != 0 {
		t.Fatalf("activeConnections after request = %d, want 0", got)
	}
}

func TestStatsCollectorMiddlewareRecordsRequest(t *testing.T) {
	c := newStatsCollector()
	handler := c.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if got := c.requestsTotalCount(); got != 1 {
		t.Fatalf("requestsTotalCount after one request = %d, want 1", got)
	}
}

func TestStatsCollectorConcurrentRecordRequest(t *testing.T) {
	c := newStatsCollector()
	var wg sync.WaitGroup
	const n = 200
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			c.recordRequest()
		}()
	}
	wg.Wait()

	if got := c.requestsTotalCount(); got != n {
		t.Fatalf("requestsTotalCount = %d, want %d", got, n)
	}
}
