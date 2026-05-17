package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// appMetrics holds all registered Prometheus metrics for the airports server.
type appMetrics struct {
	registry *prometheus.Registry

	// HTTP request counter — labels: method, path (pattern), status_code.
	httpRequestsTotal *prometheus.CounterVec
	// HTTP request duration histogram — labels: method, path, status_code.
	httpRequestDuration *prometheus.HistogramVec

	// Airport-level business metrics.
	airportSearchTotal   *prometheus.CounterVec
	airportNearbyTotal   *prometheus.CounterVec
	airportLookupTotal   *prometheus.CounterVec
	airportExportTotal   *prometheus.CounterVec
}

// newMetrics creates and registers all metrics into a dedicated registry.
// Using a non-default registry keeps the binary's metric namespace clean and
// avoids collisions with unrelated default Go metrics when running tests.
func newMetrics() *appMetrics {
	reg := prometheus.NewRegistry()

	// Standard Go runtime + process collectors (go_*, process_*).
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &appMetrics{
		registry: reg,

		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests partitioned by method, route, and status code.",
			},
			[]string{"method", "path", "status_code"},
		),

		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency distribution in seconds.",
				Buckets: prometheus.DefBuckets, // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
			},
			[]string{"method", "path", "status_code"},
		),

		airportSearchTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "airports_search_total",
				Help: "Total airport search requests partitioned by format.",
			},
			[]string{"format"},
		),

		airportNearbyTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "airports_nearby_total",
				Help: "Total nearby-airport requests partitioned by format.",
			},
			[]string{"format"},
		),

		airportLookupTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "airports_lookup_total",
				Help: "Total single-airport lookup requests partitioned by format.",
			},
			[]string{"format"},
		),

		airportExportTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "airports_export_total",
				Help: "Total full-database export requests partitioned by format.",
			},
			[]string{"format"},
		),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.airportSearchTotal,
		m.airportNearbyTotal,
		m.airportLookupTotal,
		m.airportExportTotal,
	)

	return m
}

// responseCapture is a minimal http.ResponseWriter wrapper that records the
// status code so the metrics middleware can label each request accurately.
type responseCapture struct {
	http.ResponseWriter
	statusCode int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

// instrumentMiddleware records http_requests_total and
// http_request_duration_seconds for every request that passes through it.
func (m *appMetrics) instrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := &responseCapture{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rc, r)
		elapsed := time.Since(start).Seconds()

		// Use the chi route pattern when available (set by chi after ServeHTTP
		// returns), falling back to the raw path. Using the pattern prevents
		// cardinality explosion from dynamic path segments like /airports/{ident}.
		routePattern := r.URL.Path
		if rctx := r.Context().Value(struct{ key string }{key: "routePattern"}); rctx != nil {
			if p, ok := rctx.(string); ok && p != "" {
				routePattern = p
			}
		}

		statusStr := strconv.Itoa(rc.statusCode)
		m.httpRequestsTotal.WithLabelValues(r.Method, routePattern, statusStr).Inc()
		m.httpRequestDuration.WithLabelValues(r.Method, routePattern, statusStr).Observe(elapsed)
	})
}

// handler returns an http.Handler that serves the Prometheus text exposition.
func (m *appMetrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
