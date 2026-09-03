package server

import (
	"crypto/subtle"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// appMetrics holds all registered Prometheus metrics for the airports server.
type appMetrics struct {
	registry *prometheus.Registry

	// Application info/uptime — AI.md PART 20 "Required Metrics >
	// Application Info".
	appInfo            *prometheus.GaugeVec
	appUptimeSeconds   prometheus.GaugeFunc
	appStartTimestamp  prometheus.Gauge

	// HTTP request counter — labels: method, path (pattern), status_code.
	httpRequestsTotal *prometheus.CounterVec
	// HTTP request duration histogram — labels: method, path, status_code.
	httpRequestDuration *prometheus.HistogramVec
	// HTTP request/response body size histograms — labels: method, path.
	httpRequestSize  *prometheus.HistogramVec
	httpResponseSize *prometheus.HistogramVec
	// HTTP in-flight request gauge — AI.md PART 20 "Required Metrics > HTTP".
	httpActiveRequests prometheus.Gauge

	// Airport-level business metrics.
	airportSearchTotal *prometheus.CounterVec
	airportNearbyTotal *prometheus.CounterVec
	airportLookupTotal *prometheus.CounterVec
	airportExportTotal *prometheus.CounterVec
}

// buildInfo carries the version metadata newMetrics stamps onto
// airports_app_info per AI.md PART 20 "Required Metrics > Application Info".
type buildInfo struct {
	version   string
	commit    string
	buildDate string
}

// newMetrics creates and registers all metrics into a dedicated registry.
// Using a non-default registry keeps the binary's metric namespace clean and
// avoids collisions with unrelated default Go metrics when running tests.
// cfg supplies the operator-configurable histogram bucket boundaries (AI.md
// PART 20 "Configuration" duration_buckets/size_buckets); build carries the
// version metadata for airports_app_info; startTime anchors uptime/start
// timestamp.
func newMetrics(cfg config.MetricsConfig, build buildInfo, startTime time.Time) *appMetrics {
	reg := prometheus.NewRegistry()

	// Standard Go runtime + process collectors (go_*, process_*).
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	durationBuckets := cfg.DurationBuckets
	if len(durationBuckets) == 0 {
		durationBuckets = prometheus.DefBuckets // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
	}
	sizeBuckets := cfg.SizeBuckets
	if len(sizeBuckets) == 0 {
		sizeBuckets = []float64{100, 1000, 10000, 100000, 1000000, 10000000}
	}

	m := &appMetrics{
		registry: reg,

		appInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "airports_app_info",
				Help: "Application build information. Always 1, labels carry the version details.",
			},
			[]string{"version", "commit", "build_date", "go_version"},
		),

		appStartTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "airports_app_start_timestamp",
				Help: "Unix timestamp when the application started.",
			},
		),

		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "airports_http_requests_total",
				Help: "Total number of HTTP requests partitioned by method, route, and status code.",
			},
			[]string{"method", "path", "status_code"},
		),

		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "airports_http_request_duration_seconds",
				Help:    "HTTP request latency distribution in seconds.",
				Buckets: durationBuckets,
			},
			[]string{"method", "path", "status_code"},
		),

		httpRequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "airports_http_request_size_bytes",
				Help:    "HTTP request body size distribution in bytes.",
				Buckets: sizeBuckets,
			},
			[]string{"method", "path"},
		),

		httpResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "airports_http_response_size_bytes",
				Help:    "HTTP response body size distribution in bytes.",
				Buckets: sizeBuckets,
			},
			[]string{"method", "path"},
		),

		httpActiveRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "airports_http_active_requests",
				Help: "Number of HTTP requests currently being processed.",
			},
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

	m.appUptimeSeconds = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "airports_app_uptime_seconds",
			Help: "Seconds since the application started.",
		},
		func() float64 { return time.Since(startTime).Seconds() },
	)

	m.appInfo.WithLabelValues(build.version, build.commit, build.buildDate, runtime.Version()).Set(1)
	m.appStartTimestamp.Set(float64(startTime.Unix()))

	reg.MustRegister(
		m.appInfo,
		m.appUptimeSeconds,
		m.appStartTimestamp,
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpRequestSize,
		m.httpResponseSize,
		m.httpActiveRequests,
		m.airportSearchTotal,
		m.airportNearbyTotal,
		m.airportLookupTotal,
		m.airportExportTotal,
	)

	return m
}

// responseCapture is a minimal http.ResponseWriter wrapper that records the
// status code and bytes written so the metrics middleware can label and size
// each request accurately.
type responseCapture struct {
	http.ResponseWriter
	statusCode  int
	bytesWritten int64
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	n, err := rc.ResponseWriter.Write(b)
	rc.bytesWritten += int64(n)
	return n, err
}

// instrumentMiddleware records http_requests_total,
// http_request_duration_seconds, http_request_size_bytes,
// http_response_size_bytes, and http_active_requests for every request that
// passes through it, per AI.md PART 20 "Required Metrics > HTTP".
func (m *appMetrics) instrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.httpActiveRequests.Inc()
		defer m.httpActiveRequests.Dec()

		rc := &responseCapture{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rc, r)
		elapsed := time.Since(start).Seconds()

		// Use the chi route pattern when available (populated after ServeHTTP
		// matches the route), falling back to the raw path. Using the pattern
		// prevents cardinality explosion from dynamic path segments like
		// /airports/{ident}.
		routePattern := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if p := rctx.RoutePattern(); p != "" {
				routePattern = p
			}
		}

		statusStr := strconv.Itoa(rc.statusCode)
		m.httpRequestsTotal.WithLabelValues(r.Method, routePattern, statusStr).Inc()
		m.httpRequestDuration.WithLabelValues(r.Method, routePattern, statusStr).Observe(elapsed)

		if r.ContentLength > 0 {
			m.httpRequestSize.WithLabelValues(r.Method, routePattern).Observe(float64(r.ContentLength))
		}
		m.httpResponseSize.WithLabelValues(r.Method, routePattern).Observe(float64(rc.bytesWritten))
	})
}

// handler returns an http.Handler that serves the Prometheus text exposition.
func (m *appMetrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// metricsAuthMiddleware enforces the optional "Authorization: Bearer <token>"
// check on /metrics per AI.md PART 20 "Access Control > Authentication
// options". When token is empty (the default), metrics stay open to any
// caller that can reach the endpoint — deployments are expected to firewall
// /metrics instead, per PART 20 "Access Control": /metrics is internal only
// regardless of whether a token is configured.
func metricsAuthMiddleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, prefix)), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, i18n.T(i18n.FromContext(r.Context()), "errors.unauthorized"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
