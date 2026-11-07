package server

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	torpkg "github.com/apimgr/airports/src/tor"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// appMetrics holds all registered Prometheus metrics for the airports server.
type appMetrics struct {
	registry   *prometheus.Registry
	lokiLogs   *logging.RecentLogBuffer
	lokiConfig config.MetricsLokiConfig

	// Application info/uptime — AI.md PART 20 "Required Metrics >
	// Application Info".
	appInfo           *prometheus.GaugeVec
	appUptimeSeconds  prometheus.GaugeFunc
	appStartTimestamp prometheus.Gauge

	// HTTP request counter — labels: method, path (pattern), status.
	httpRequestsTotal *prometheus.CounterVec
	// HTTP request duration histogram — labels: method, path.
	httpRequestDuration *prometheus.HistogramVec
	// HTTP request/response body size histograms — labels: method, path.
	httpRequestSize  *prometheus.HistogramVec
	httpResponseSize *prometheus.HistogramVec
	// HTTP in-flight request gauge — AI.md PART 20 "Required Metrics > HTTP".
	httpActiveRequests prometheus.Gauge
	// Tor hidden-service requests are counted only when PROXY-protocol
	// metadata from the dedicated loopback listener attached a circuit ID.
	torRequestsTotal prometheus.Counter
	// Rate-limit counters use fixed limit and outcome labels per AI.md PART 20.
	rateLimitRequestsTotal *prometheus.CounterVec
	rateLimitBlockedTotal  *prometheus.CounterVec
	// Metrics service-token checks are the only authentication flow in this
	// public, no-user-account product; labels never contain credentials.
	authAttemptsTotal  *prometheus.CounterVec
	authSessionsActive prometheus.Gauge

	// Airport-level business metrics.
	airportSearchTotal *prometheus.CounterVec
	airportNearbyTotal *prometheus.CounterVec
	airportLookupTotal *prometheus.CounterVec
	airportExportTotal *prometheus.CounterVec
	database           *sql.DB
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
func newMetrics(cfg config.MetricsConfig, build buildInfo, startTime time.Time, dataDir string) *appMetrics {
	reg := prometheus.NewRegistry()

	// Runtime and process collectors are independently controlled by config.
	if cfg.IncludeRuntime {
		reg.MustRegister(newGoRuntimeMetricsCollector())
	}
	if cfg.IncludeSystem {
		reg.MustRegister(newSystemMetricsCollector(dataDir))
	}

	durationBuckets := cfg.DurationBuckets
	if len(durationBuckets) == 0 {
		durationBuckets = prometheus.DefBuckets // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
	}
	sizeBuckets := cfg.SizeBuckets
	if len(sizeBuckets) == 0 {
		sizeBuckets = []float64{100, 1000, 10000, 100000, 1000000, 10000000}
	}

	m := &appMetrics{
		registry:   reg,
		lokiConfig: cfg.Loki,

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
			[]string{"method", "path", "status"},
		),

		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "airports_http_request_duration_seconds",
				Help:    "HTTP request latency distribution in seconds.",
				Buckets: durationBuckets,
			},
			[]string{"method", "path"},
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
		rateLimitRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_ratelimit_requests_total",
			Help: "Total requests evaluated by rate limiting, partitioned by limit and outcome.",
		}, []string{"limit", "status"}),
		rateLimitBlockedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_ratelimit_blocked_total",
			Help: "Total requests blocked by rate limiting, partitioned by limit type.",
		}, []string{"limit"}),
		authAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_auth_attempts_total",
			Help: "Metrics service-token authentication attempts partitioned by method and outcome.",
		}, []string{"method", "status"}),
		authSessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "airports_auth_sessions_active",
			Help: "Active authenticated user sessions; this product does not create user sessions.",
		}),

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
	// Authentication is limited to per-service metrics tokens. The public
	// airport API has no login or user-session model (IDEA.md Product scope).
	m.authSessionsActive.Set(0)

	reg.MustRegister(
		m.appInfo,
		m.appUptimeSeconds,
		m.appStartTimestamp,
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpRequestSize,
		m.httpResponseSize,
		m.httpActiveRequests,
		m.rateLimitRequestsTotal,
		m.rateLimitBlockedTotal,
		m.authAttemptsTotal,
		m.authSessionsActive,
		m.airportSearchTotal,
		m.airportNearbyTotal,
		m.airportLookupTotal,
		m.airportExportTotal,
		dbpkg.QueryMetricsCollector(),
	)

	return m
}

func (m *appMetrics) recordRateLimit(limit string, allowed bool) {
	status := "allowed"
	if !allowed {
		status = "limited"
	}
	m.rateLimitRequestsTotal.WithLabelValues(limit, status).Inc()
	if !allowed {
		m.rateLimitBlockedTotal.WithLabelValues(limit).Inc()
	}
}

func (m *appMetrics) setLokiLogs(buffer *logging.RecentLogBuffer) {
	m.lokiLogs = buffer
}

func (m *appMetrics) setSchedulerMetrics(collector prometheus.Collector) {
	if collector != nil {
		m.registry.MustRegister(collector)
	}
}

// setTorManager registers live lifecycle gauges from the Tor control manager.
// A request counter is deliberately not inferred from Host: only trusted
// proxy-protocol metadata on a dedicated hidden-service listener can identify
// actual onion traffic.
func (m *appMetrics) setTorManager(manager *torpkg.Manager) {
	if manager == nil {
		return
	}
	m.torRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "airports_tor_requests_total",
		Help: "Total requests accepted through the Tor hidden-service listener.",
	})
	m.registry.MustRegister(
		m.torRequestsTotal,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "airports_tor_enabled",
			Help: "Whether the Tor service is available and enabled.",
		}, func() float64 {
			if manager.Enabled() {
				return 1
			}
			return 0
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "airports_tor_running",
			Help: "Whether the dedicated Tor process is running.",
		}, func() float64 {
			if manager.HealthCheck() == nil {
				return 1
			}
			return 0
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "airports_tor_circuit_established",
			Help: "Whether Tor currently reports an established circuit.",
		}, func() float64 {
			if manager.CircuitEstablished() {
				return 1
			}
			return 0
		}),
	)
}

// setDatabase registers live connection-pool gauges for the shared SQLite
// database. Query counters and duration metrics are recorded at call sites.
func (m *appMetrics) setDatabase(database *sql.DB) {
	if database == nil {
		return
	}
	m.database = database
	m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "airports_db_connections_open",
		Help: "Number of open database connections.",
	}, func() float64 { return float64(database.Stats().OpenConnections) }))
	m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "airports_db_connections_in_use",
		Help: "Number of database connections currently in use.",
	}, func() float64 { return float64(database.Stats().InUse) }))
}

// setCache registers only cache measurements the configured backend can
// report accurately. Redis size and eviction metrics remain absent because
// server-wide values would include unrelated clients sharing that database.
func (m *appMetrics) setCache(cache cachepkg.Cache) {
	provider, ok := cache.(cachepkg.MetricsProvider)
	if !ok {
		return
	}
	m.registry.MustRegister(newCacheMetricsCollector(provider))
}

type cacheMetricsCollector struct {
	provider  cachepkg.MetricsProvider
	hits      *prometheus.Desc
	misses    *prometheus.Desc
	evictions *prometheus.Desc
	size      *prometheus.Desc
	bytes     *prometheus.Desc
}

func newCacheMetricsCollector(provider cachepkg.MetricsProvider) *cacheMetricsCollector {
	return &cacheMetricsCollector{
		provider:  provider,
		hits:      prometheus.NewDesc("airports_cache_hits_total", "Total successful cache lookups.", []string{"cache"}, nil),
		misses:    prometheus.NewDesc("airports_cache_misses_total", "Total cache lookups without a value.", []string{"cache"}, nil),
		evictions: prometheus.NewDesc("airports_cache_evictions_total", "Total cache entries removed after expiration.", []string{"cache"}, nil),
		size:      prometheus.NewDesc("airports_cache_size", "Current number of live entries in the cache.", []string{"cache"}, nil),
		bytes:     prometheus.NewDesc("airports_cache_bytes", "Current logical key and value bytes stored in the cache; excludes runtime allocation overhead.", []string{"cache"}, nil),
	}
}

func (c *cacheMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.hits
	ch <- c.misses
	ch <- c.evictions
	ch <- c.size
	ch <- c.bytes
}

func (c *cacheMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.provider.MetricsStats()
	if !stats.Enabled {
		return
	}
	const cacheName = "default"
	ch <- prometheus.MustNewConstMetric(c.hits, prometheus.CounterValue, float64(stats.Hits), cacheName)
	ch <- prometheus.MustNewConstMetric(c.misses, prometheus.CounterValue, float64(stats.Misses), cacheName)
	if stats.HasEvictionCount {
		ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(stats.Evictions), cacheName)
	}
	if stats.HasSize {
		ch <- prometheus.MustNewConstMetric(c.size, prometheus.GaugeValue, float64(stats.Items), cacheName)
		ch <- prometheus.MustNewConstMetric(c.bytes, prometheus.GaugeValue, float64(stats.Bytes), cacheName)
	}
}

type goRuntimeMetricsCollector struct {
	descs []*prometheus.Desc
}

func newGoRuntimeMetricsCollector() *goRuntimeMetricsCollector {
	return &goRuntimeMetricsCollector{descs: []*prometheus.Desc{
		prometheus.NewDesc("airports_go_goroutines", "Current number of goroutines.", nil, nil),
		prometheus.NewDesc("airports_go_mem_alloc_bytes", "Bytes allocated and still in use (heap).", nil, nil),
		prometheus.NewDesc("airports_go_mem_sys_bytes", "Total bytes obtained from the system.", nil, nil),
		prometheus.NewDesc("airports_go_gc_runs_total", "Total number of completed garbage collections.", nil, nil),
		prometheus.NewDesc("airports_go_gc_pause_total_seconds", "Total time spent in garbage collection pauses.", nil, nil),
	}}
}

func (c *goRuntimeMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *goRuntimeMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	ch <- prometheus.MustNewConstMetric(c.descs[0], prometheus.GaugeValue, float64(runtime.NumGoroutine()))
	ch <- prometheus.MustNewConstMetric(c.descs[1], prometheus.GaugeValue, float64(stats.Alloc))
	ch <- prometheus.MustNewConstMetric(c.descs[2], prometheus.GaugeValue, float64(stats.Sys))
	ch <- prometheus.MustNewConstMetric(c.descs[3], prometheus.CounterValue, float64(stats.NumGC))
	ch <- prometheus.MustNewConstMetric(c.descs[4], prometheus.CounterValue, float64(stats.PauseTotalNs)/float64(time.Second))
}

// systemMetricsCollector gathers the current system CPU, memory, and data
// directory filesystem values when Prometheus scrapes the registry.
type systemMetricsCollector struct {
	dataDir string
	mu      sync.Mutex
	lastCPU *cpu.TimesStat
	descs   []*prometheus.Desc
}

func newSystemMetricsCollector(dataDir string) *systemMetricsCollector {
	collector := &systemMetricsCollector{
		dataDir: dataDir,
		descs: []*prometheus.Desc{
			prometheus.NewDesc("airports_system_cpu_usage_percent", "Current system CPU usage percentage (0-100).", nil, nil),
			prometheus.NewDesc("airports_system_memory_usage_percent", "Current system memory usage percentage (0-100).", nil, nil),
			prometheus.NewDesc("airports_system_memory_used_bytes", "System memory currently in use in bytes.", nil, nil),
			prometheus.NewDesc("airports_system_memory_total_bytes", "Total system memory in bytes.", nil, nil),
			prometheus.NewDesc("airports_system_disk_usage_percent", "Data directory filesystem usage percentage.", []string{"path"}, nil),
			prometheus.NewDesc("airports_system_disk_used_bytes", "Data directory filesystem space used in bytes.", []string{"path"}, nil),
			prometheus.NewDesc("airports_system_disk_total_bytes", "Data directory filesystem total space in bytes.", []string{"path"}, nil),
		},
	}
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		first := times[0]
		collector.lastCPU = &first
	}
	return collector
}

func (c *systemMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *systemMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if usage, ok := c.cpuUsagePercent(); ok {
		ch <- prometheus.MustNewConstMetric(c.descs[0], prometheus.GaugeValue, usage)
	}
	if memory, err := mem.VirtualMemory(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.descs[1], prometheus.GaugeValue, memory.UsedPercent)
		ch <- prometheus.MustNewConstMetric(c.descs[2], prometheus.GaugeValue, float64(memory.Used))
		ch <- prometheus.MustNewConstMetric(c.descs[3], prometheus.GaugeValue, float64(memory.Total))
	}
	if c.dataDir != "" {
		if usage, err := disk.Usage(c.dataDir); err == nil {
			ch <- prometheus.MustNewConstMetric(c.descs[4], prometheus.GaugeValue, usage.UsedPercent, c.dataDir)
			ch <- prometheus.MustNewConstMetric(c.descs[5], prometheus.GaugeValue, float64(usage.Used), c.dataDir)
			ch <- prometheus.MustNewConstMetric(c.descs[6], prometheus.GaugeValue, float64(usage.Total), c.dataDir)
		}
	}
}

func (c *systemMetricsCollector) cpuUsagePercent() (float64, bool) {
	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		return 0, false
	}
	current := times[0]
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.lastCPU
	c.lastCPU = &current
	if previous == nil {
		return 0, false
	}
	totalDelta := cpuTotal(&current) - cpuTotal(previous)
	if totalDelta <= 0 {
		return 0, false
	}
	idleDelta := (current.Idle + current.Iowait) - (previous.Idle + previous.Iowait)
	usage := 100 * (totalDelta - idleDelta) / totalDelta
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}
	return usage, true
}

func cpuTotal(times *cpu.TimesStat) float64 {
	return times.User + times.Nice + times.System + times.Idle + times.Iowait + times.Irq + times.Softirq + times.Steal
}

// responseCapture is a minimal http.ResponseWriter wrapper that records the
// status code and bytes written so the metrics middleware can label and size
// each request accurately.
type responseCapture struct {
	http.ResponseWriter
	statusCode   int
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
		duration := time.Since(start)
		elapsed := duration.Seconds()

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
		if TorCircuitID(r) != "" && m.torRequestsTotal != nil {
			m.torRequestsTotal.Inc()
		}
		recordDebugRequest(duration, rc.statusCode)
		m.httpRequestsTotal.WithLabelValues(r.Method, routePattern, statusStr).Inc()
		m.httpRequestDuration.WithLabelValues(r.Method, routePattern).Observe(elapsed)

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

func (m *appMetrics) serviceHandler(service string) http.Handler {
	switch service {
	case "grafana":
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = encodeJSON(w, grafanaMetricsDashboard())
		})
	case "loki":
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = encodeJSON(w, m.lokiStreams())
		})
	default:
		return m.handler()
	}
}

type lokiResponse struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func (m *appMetrics) lokiStreams() lokiResponse {
	response := lokiResponse{Streams: []lokiStream{}}
	if m.lokiLogs == nil {
		return response
	}
	maxEntries := m.lokiConfig.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	maxAge, err := time.ParseDuration(m.lokiConfig.MaxAge)
	if err != nil || maxAge <= 0 {
		maxAge = time.Hour
	}
	entries := m.lokiLogs.Recent(maxEntries, maxAge, time.Now().UTC())
	if len(entries) == 0 {
		return response
	}
	values := make([][]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, []string{fmt.Sprintf("%d", entry.Timestamp.UnixNano()), entry.Line})
	}
	response.Streams = append(response.Streams, lokiStream{
		Stream: map[string]string{"app": "airports", "source": "server"},
		Values: values,
	})
	return response
}

func grafanaMetricsDashboard() map[string]interface{} {
	type panelDefinition struct {
		title       string
		expr        string
		legend      string
		unit        string
		description string
	}

	panelsSpec := []panelDefinition{
		{title: "HTTP · Request rate by status", expr: "sum by (status) (rate(airports_http_requests_total[5m]))", legend: "{{status}}", unit: "reqps"},
		{title: "HTTP · 95th percentile request duration", expr: "histogram_quantile(0.95, sum by (le) (rate(airports_http_request_duration_seconds_bucket[5m])))", unit: "s"},
		{title: "HTTP · 95th percentile request size", expr: "histogram_quantile(0.95, sum by (le) (rate(airports_http_request_size_bytes_bucket[5m])))", unit: "bytes"},
		{title: "HTTP · 95th percentile response size", expr: "histogram_quantile(0.95, sum by (le) (rate(airports_http_response_size_bytes_bucket[5m])))", unit: "bytes"},
		{title: "HTTP · Active requests", expr: "airports_http_active_requests", unit: "short"},
		{title: "Database · Query rate by operation", expr: "sum by (operation) (rate(airports_db_queries_total[5m]))", legend: "{{operation}}", unit: "reqps"},
		{title: "Database · 95th percentile query duration", expr: "histogram_quantile(0.95, sum by (le) (rate(airports_db_query_duration_seconds_bucket[5m])))", unit: "s"},
		{title: "Database · Open connections", expr: "airports_db_connections_open", unit: "short"},
		{title: "Database · Connections in use", expr: "airports_db_connections_in_use", unit: "short"},
		{title: "Database · Error rate by type", expr: "sum by (error_type) (rate(airports_db_errors_total[5m]))", legend: "{{error_type}}", unit: "reqps"},
		{title: "Cache · Hit ratio", expr: "sum by (cache) (rate(airports_cache_hits_total[5m])) / clamp_min(sum by (cache) (rate(airports_cache_hits_total[5m]) + rate(airports_cache_misses_total[5m])), 1e-9)", legend: "{{cache}}", unit: "percentunit"},
		{title: "Cache · Miss rate", expr: "sum by (cache) (rate(airports_cache_misses_total[5m]))", legend: "{{cache}}", unit: "reqps"},
		{title: "Cache · Eviction rate", expr: "sum by (cache) (rate(airports_cache_evictions_total[5m]))", legend: "{{cache}}", unit: "reqps"},
		{title: "Cache · Item count", expr: "airports_cache_size", legend: "{{cache}}", unit: "short"},
		{title: "Cache · Bytes used", expr: "airports_cache_bytes", legend: "{{cache}}", unit: "bytes"},
		{title: "Scheduler · Task rate by status", expr: "sum by (task, status) (rate(airports_scheduler_tasks_total[5m]))", legend: "{{task}} · {{status}}", unit: "reqps"},
		{title: "Scheduler · 95th percentile task duration", expr: "histogram_quantile(0.95, sum by (le) (rate(airports_scheduler_task_duration_seconds_bucket[5m])))", unit: "s"},
		{title: "Scheduler · Running tasks", expr: "sum by (task) (airports_scheduler_tasks_running)", legend: "{{task}}", unit: "short"},
		{title: "Scheduler · Last task run", expr: "airports_scheduler_last_run_timestamp", legend: "{{task}}", unit: "dateTimeAsIso"},
		{title: "System · CPU usage", expr: "airports_system_cpu_usage_percent", unit: "percent"},
		{title: "System · Memory usage", expr: "airports_system_memory_usage_percent", unit: "percent"},
		{title: "System · Memory used", expr: "airports_system_memory_used_bytes", unit: "bytes"},
		{title: "System · Memory total", expr: "airports_system_memory_total_bytes", unit: "bytes"},
		{title: "System · Disk usage by path", expr: "airports_system_disk_usage_percent", legend: "{{path}}", unit: "percent"},
		{title: "System · Disk used by path", expr: "airports_system_disk_used_bytes", legend: "{{path}}", unit: "bytes"},
		{title: "System · Disk total by path", expr: "airports_system_disk_total_bytes", legend: "{{path}}", unit: "bytes"},
		{title: "Authentication · Attempts by status", expr: "sum by (method, status) (rate(airports_auth_attempts_total[5m]))", legend: "{{method}} · {{status}}", unit: "reqps"},
		{title: "Authentication · Active sessions", expr: "airports_auth_sessions_active", unit: "short"},
		{title: "Rate limiting · Requests by status", expr: "sum by (limit, status) (rate(airports_ratelimit_requests_total[5m]))", legend: "{{limit}} · {{status}}", unit: "reqps"},
		{title: "Rate limiting · Blocked requests", expr: "sum by (limit) (rate(airports_ratelimit_blocked_total[5m]))", legend: "{{limit}}", unit: "reqps"},
		{title: "Go runtime · Goroutines", expr: "airports_go_goroutines", unit: "short"},
		{title: "Go runtime · Heap allocation", expr: "airports_go_mem_alloc_bytes", unit: "bytes"},
		{title: "Go runtime · Memory obtained", expr: "airports_go_mem_sys_bytes", unit: "bytes"},
		{title: "Go runtime · Garbage collections", expr: "rate(airports_go_gc_runs_total[5m])", unit: "reqps"},
		{title: "Go runtime · GC pause time", expr: "rate(airports_go_gc_pause_total_seconds[5m])", unit: "s"},
		{title: "Tor · Enabled", expr: "airports_tor_enabled", unit: "short"},
		{title: "Tor · Running", expr: "airports_tor_running", unit: "short"},
		{title: "Tor · Circuit established", expr: "airports_tor_circuit_established", unit: "short"},
		{title: "Tor · Request rate", expr: "rate(airports_tor_requests_total[5m])", unit: "reqps"},
		{title: "Business · Airport searches", expr: "sum(rate(airports_search_total[5m]))", unit: "reqps"},
		{title: "Business · Nearby airport requests", expr: "sum(rate(airports_nearby_total[5m]))", unit: "reqps"},
		{title: "Business · Airport lookups", expr: "sum(rate(airports_lookup_total[5m]))", unit: "reqps"},
		{title: "Business · Airport exports", expr: "sum(rate(airports_export_total[5m]))", unit: "reqps"},
	}
	datasource := map[string]interface{}{"type": "prometheus", "uid": "${datasource}"}
	panels := make([]map[string]interface{}, 0, len(panelsSpec))
	for i, panel := range panelsSpec {
		target := map[string]interface{}{
			"datasource":   datasource,
			"editorMode":   "code",
			"expr":         panel.expr,
			"instant":      false,
			"legendFormat": panel.legend,
			"range":        true,
			"refId":        "A",
		}
		panels = append(panels, map[string]interface{}{
			"datasource":  datasource,
			"description": panel.description,
			"fieldConfig": map[string]interface{}{
				"defaults": map[string]interface{}{
					"color": map[string]interface{}{"mode": "palette-classic"},
					"custom": map[string]interface{}{
						"axisBorderShow": false, "axisCenteredZero": false, "axisColorMode": "text", "axisLabel": "", "axisPlacement": "auto",
						"barAlignment": 0, "barWidthFactor": 0.6, "drawStyle": "line", "fillOpacity": 10, "gradientMode": "none",
						"hideFrom": map[string]bool{"legend": false, "tooltip": false, "viz": false}, "insertNulls": false,
						"lineInterpolation": "linear", "lineWidth": 1, "pointSize": 5, "scaleDistribution": map[string]string{"type": "linear"},
						"showPoints": "never", "spanNulls": false, "stacking": map[string]string{"group": "A", "mode": "none"},
						"thresholdsStyle": map[string]string{"mode": "off"},
					},
					"mappings":   []interface{}{},
					"thresholds": map[string]interface{}{"mode": "absolute", "steps": []map[string]interface{}{{"color": "green"}, {"color": "red", "value": 80}}},
					"unit":       panel.unit,
				},
				"overrides": []interface{}{},
			},
			"gridPos": map[string]int{"h": 8, "w": 12, "x": (i % 2) * 12, "y": (i / 2) * 8},
			"id":      i + 1,
			"options": map[string]interface{}{
				"alertThreshold": true,
				"legend":         map[string]interface{}{"calcs": []interface{}{}, "displayMode": "list", "placement": "bottom", "showLegend": true},
				"tooltip":        map[string]interface{}{"hideZeros": false, "mode": "multi", "sort": "none"},
			},
			"pluginVersion": "12.0.2",
			"targets":       []map[string]interface{}{target},
			"title":         panel.title,
			"type":          "timeseries",
		})
	}
	return map[string]interface{}{
		"__requires": []map[string]interface{}{
			{"type": "grafana", "id": "grafana", "name": "Grafana", "version": "12.0.2"},
			{"type": "datasource", "id": "prometheus", "name": "Prometheus", "version": "1.0.0"},
		},
		"annotations": map[string]interface{}{"list": []map[string]interface{}{{
			"builtIn": 1, "datasource": map[string]string{"type": "datasource", "uid": "grafana"}, "enable": true, "hide": true,
			"iconColor": "rgba(0, 211, 255, 1)", "name": "Annotations & Alerts", "target": map[string]interface{}{"limit": 100, "matchAny": false, "tags": []interface{}{}, "type": "dashboard"}, "type": "dashboard",
		}}},
		"editable":             true,
		"fiscalYearStartMonth": 0,
		"graphTooltip":         0,
		"id":                   nil,
		"links":                []interface{}{},
		"liveNow":              false,
		"panels":               panels,
		"preload":              false,
		"refresh":              "30s",
		"schemaVersion":        42,
		"tags":                 []string{"airports", "metrics"},
		"templating": map[string]interface{}{"list": []map[string]interface{}{{
			"current":    map[string]interface{}{"selected": true, "text": "Prometheus", "value": "prometheus"},
			"includeAll": false, "name": "datasource", "options": []interface{}{}, "query": "prometheus", "refresh": 1, "regex": "", "type": "datasource",
		}}},
		"time":       map[string]string{"from": "now-6h", "to": "now"},
		"timepicker": map[string]interface{}{"refresh_intervals": []string{"10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"}},
		"timezone":   "utc",
		"title":      "Airports Metrics",
		"uid":        "airports-metrics",
		"version":    1,
	}
}

func metricsServiceToken(cfg config.MetricsConfig, service string) string {
	switch service {
	case "grafana":
		return cfg.Auth.Tokens.Grafana
	case "loki":
		return cfg.Auth.Tokens.Loki
	default:
		return metricsPrometheusToken(cfg)
	}
}

func metricsPrometheusToken(cfg config.MetricsConfig) string {
	if cfg.Auth.Tokens.Prometheus != "" {
		return cfg.Auth.Tokens.Prometheus
	}
	return cfg.Token
}

// metricsAuthMiddleware requires a service token unless the operator has
// explicitly enabled the firewalled unauthenticated escape hatch.
func metricsAuthMiddleware(token string, allowUnauthenticated bool, metrics *appMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowUnauthenticated {
			next.ServeHTTP(w, r)
			return
		}
		if token == "" {
			recordAuthAttempt(metrics, false)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(requestAuthToken(r)), []byte(token)) != 1 {
			recordAuthAttempt(metrics, false)
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			writeTextError(w, http.StatusUnauthorized, "UNAUTHORIZED", i18n.T(i18n.FromContext(r.Context()), "errors.unauthorized"))
			return
		}
		recordAuthAttempt(metrics, true)
		next.ServeHTTP(w, r)
	})
}

func recordAuthAttempt(metrics *appMetrics, success bool) {
	if metrics == nil {
		return
	}
	status := "failed"
	if success {
		status = "success"
	}
	metrics.authAttemptsTotal.WithLabelValues("service_token", status).Inc()
}

// requestAuthToken follows AI.md's supported credential header priority. The
// metrics endpoints are header-only; query-string credentials are forbidden.
func requestAuthToken(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization != "" {
		if scheme, value, found := strings.Cut(authorization, " "); found && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(value)
		}
		return authorization
	}
	for _, name := range []string{"X-API-Key", "API-Key", "X-Auth-Token", "X-Access-Token", "X-Token", "Token"} {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}
