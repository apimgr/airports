package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/config"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	torpkg "github.com/apimgr/airports/src/tor"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsAuthMiddleware covers mandatory per-service bearer auth.
func TestMetricsAuthMiddleware(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name                 string
		token                string
		allowUnauthenticated bool
		authHeader           string
		tokenHeader          string
		queryToken           string
		wantStatus           int
	}{
		{name: "empty token disables service", token: "", authHeader: "", wantStatus: http.StatusForbidden},
		{name: "empty token disables service even with garbage header", token: "", authHeader: "Bearer wrong", wantStatus: http.StatusForbidden},
		{name: "firewalled escape hatch allows empty token", allowUnauthenticated: true, wantStatus: http.StatusOK},
		{name: "token configured, missing header rejected", token: "secret123", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "token configured, wrong token rejected", token: "secret123", authHeader: "Bearer wrongtoken", wantStatus: http.StatusUnauthorized},
		{name: "token configured, non-bearer scheme rejected", token: "secret123", authHeader: "Basic secret123", wantStatus: http.StatusUnauthorized},
		{name: "token configured, correct token allowed", token: "secret123", authHeader: "Bearer secret123", wantStatus: http.StatusOK},
		{name: "API key header allowed", token: "secret123", tokenHeader: "X-API-Key", wantStatus: http.StatusOK},
		{name: "API-Key alias allowed", token: "secret123", tokenHeader: "API-Key", wantStatus: http.StatusOK},
		{name: "auth token header allowed", token: "secret123", tokenHeader: "X-Auth-Token", wantStatus: http.StatusOK},
		{name: "access token header allowed", token: "secret123", tokenHeader: "X-Access-Token", wantStatus: http.StatusOK},
		{name: "short token header allowed", token: "secret123", tokenHeader: "X-Token", wantStatus: http.StatusOK},
		{name: "token header allowed", token: "secret123", tokenHeader: "Token", wantStatus: http.StatusOK},
		{name: "query token forbidden", token: "secret123", queryToken: "secret123", wantStatus: http.StatusUnauthorized},
		{name: "authorization takes priority", token: "secret123", authHeader: "Bearer wrong", tokenHeader: "X-API-Key", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), "")
			h := metricsAuthMiddleware(tt.token, tt.allowUnauthenticated, metrics, okHandler)

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.tokenHeader != "" {
				req.Header.Set(tt.tokenHeader, tt.token)
			}
			if tt.queryToken != "" {
				req = httptest.NewRequest(http.MethodGet, "/metrics?token="+tt.queryToken, nil)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.token == "" && !tt.allowUnauthenticated && rr.Body.Len() != 0 {
				t.Fatalf("disabled service body = %q, want empty", rr.Body.String())
			}
			if tt.wantStatus == http.StatusUnauthorized {
				if got := rr.Header().Get("WWW-Authenticate"); got == "" {
					t.Fatalf("expected WWW-Authenticate header on 401 response")
				}
			}
			if !tt.allowUnauthenticated {
				status := "failed"
				if tt.wantStatus == http.StatusOK {
					status = "success"
				}
				if got := testutil.ToFloat64(metrics.authAttemptsTotal.WithLabelValues("service_token", status)); got != 1 {
					t.Errorf("auth attempts (%s) = %v, want 1", status, got)
				}
			}
		})
	}
}

func TestAuthMetricsReportNoUserSessions(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), "")
	if got := testutil.ToFloat64(metrics.authSessionsActive); got != 0 {
		t.Fatalf("active user sessions = %v, want 0 for the no-login product", got)
	}
}

func TestTorMetricsRegisterManagerLifecycleGauges(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), "")
	manager := torpkg.NewManager("airports-test", 8080, torpkg.Config{
		Binary: filepath.Join(t.TempDir(), "tor-not-installed"),
	})
	metrics.setTorManager(manager)

	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := map[string]bool{
		"airports_tor_enabled":             false,
		"airports_tor_running":             false,
		"airports_tor_circuit_established": false,
		"airports_tor_requests_total":      false,
	}
	for _, family := range families {
		if _, ok := found[family.GetName()]; ok {
			found[family.GetName()] = true
		}
	}
	for name, present := range found {
		if !present {
			t.Errorf("metric %s was not registered", name)
		}
	}
}

func TestTorRequestsMetricCountsOnlyCircuitMetadata(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), "")
	manager := torpkg.NewManager("airports-test", 8080, torpkg.Config{
		Binary: filepath.Join(t.TempDir(), "tor-not-installed"),
	})
	metrics.setTorManager(manager)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := metrics.instrumentMiddleware(next)

	spoofedHost := httptest.NewRequest(http.MethodGet, "http://exampleexampleexampleexampleexampleexampleexampleexample.onion/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), spoofedHost)
	if got := testutil.ToFloat64(metrics.torRequestsTotal); got != 0 {
		t.Fatalf("Tor request count after Host-only request = %v, want 0", got)
	}

	torRequest := httptest.NewRequest(http.MethodGet, "http://exampleexampleexampleexampleexampleexampleexampleexample.onion/", nil)
	torRequest = torRequest.WithContext(context.WithValue(torRequest.Context(), torCircuitContextKey{}, "fc00::1234"))
	handler.ServeHTTP(httptest.NewRecorder(), torRequest)
	if got := testutil.ToFloat64(metrics.torRequestsTotal); got != 1 {
		t.Fatalf("Tor request count after circuit metadata = %v, want 1", got)
	}
}

func TestLokiServiceReturnsBoundedRecentStreams(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{
		Loki: config.MetricsLokiConfig{MaxEntries: 2, MaxAge: "1h"},
	}, buildInfo{}, time.Now(), "")
	logs := logging.NewRecentLogBuffer(4)
	_, _ = logs.Write([]byte("first\nsecond\nthird\n"))
	metrics.setLokiLogs(logs)

	response := httptest.NewRecorder()
	metrics.serviceHandler("loki").ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/server/metrics/loki", nil))
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var payload lokiResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode Loki stream: %v", err)
	}
	if len(payload.Streams) != 1 || len(payload.Streams[0].Values) != 2 {
		t.Fatalf("Loki stream values = %#v, want one stream with two entries", payload.Streams)
	}
	if payload.Streams[0].Values[0][1] != "second" || payload.Streams[0].Values[1][1] != "third" {
		t.Errorf("Loki values = %#v, want newest two lines", payload.Streams[0].Values)
	}
}

func TestDatabaseMetricsExposeConnectionPoolStats(t *testing.T) {
	database, err := dbpkg.Open(config.DatabaseConfig{Driver: "sqlite", URL: ":memory:"}, "airports")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), t.TempDir())
	metrics.setDatabase(database)
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	want := map[string]bool{
		"airports_db_connections_open":   false,
		"airports_db_connections_in_use": false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("database metric %q was not gathered", name)
		}
	}
}

func TestCacheMetricsExposeMeasuredBackendStats(t *testing.T) {
	cache, err := cachepkg.New(config.CacheConfig{Type: "memory", Prefix: "metrics:"})
	if err != nil {
		t.Fatalf("create memory cache: %v", err)
	}
	defer func() { _ = cache.Close() }()
	if err := cache.Set(t.Context(), "airport", []byte("value"), -1); err != nil {
		t.Fatalf("set cache value: %v", err)
	}
	if _, err := cache.Get(t.Context(), "airport"); err != nil {
		t.Fatalf("get cache hit: %v", err)
	}
	if _, err := cache.Get(t.Context(), "missing"); err == nil {
		t.Fatal("cache miss unexpectedly returned a value")
	}

	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), t.TempDir())
	metrics.setCache(cache)
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	want := map[string]bool{
		"airports_cache_hits_total":      false,
		"airports_cache_misses_total":    false,
		"airports_cache_evictions_total": false,
		"airports_cache_size":            false,
		"airports_cache_bytes":           false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("cache metric %q was not gathered", name)
		}
	}
}

func TestGrafanaMetricsDashboardImportShapeAndCoverage(t *testing.T) {
	dashboard := grafanaMetricsDashboard()
	encoded, err := json.Marshal(dashboard)
	if err != nil {
		t.Fatalf("marshal dashboard: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode dashboard JSON: %v", err)
	}
	if decoded["schemaVersion"] != float64(42) {
		t.Errorf("schemaVersion = %v, want current Classic dashboard schema 42", decoded["schemaVersion"])
	}
	if decoded["id"] != nil || decoded["uid"] != "airports-metrics" || decoded["title"] != "Airports Metrics" {
		t.Errorf("dashboard identity = id:%v uid:%v title:%v", decoded["id"], decoded["uid"], decoded["title"])
	}

	templating, ok := decoded["templating"].(map[string]interface{})
	if !ok {
		t.Fatalf("templating = %T, want object", decoded["templating"])
	}
	variables, ok := templating["list"].([]interface{})
	if !ok || len(variables) != 1 {
		t.Fatalf("templating.list = %v, want one datasource variable", templating["list"])
	}
	datasourceVariable, ok := variables[0].(map[string]interface{})
	if !ok || datasourceVariable["type"] != "datasource" || datasourceVariable["query"] != "prometheus" {
		t.Fatalf("datasource variable = %v, want Prometheus datasource variable", variables[0])
	}

	panels, ok := decoded["panels"].([]interface{})
	if !ok || len(panels) == 0 {
		t.Fatalf("panels = %v, want non-empty array", decoded["panels"])
	}
	categories := map[string]bool{}
	queries := map[string]bool{}
	ids := map[float64]bool{}
	for i, value := range panels {
		panel, ok := value.(map[string]interface{})
		if !ok {
			t.Fatalf("panels[%d] = %T, want object", i, value)
		}
		if panel["type"] != "timeseries" {
			t.Errorf("panels[%d].type = %v, want timeseries", i, panel["type"])
		}
		if id, ok := panel["id"].(float64); !ok || id <= 0 || ids[id] {
			t.Errorf("panels[%d].id = %v, want unique positive ID", i, panel["id"])
		} else {
			ids[id] = true
		}
		if panel["gridPos"] == nil || panel["fieldConfig"] == nil || panel["options"] == nil {
			t.Errorf("panels[%d] lacks Grafana layout or rendering fields", i)
		}
		title, _ := panel["title"].(string)
		if category, _, found := strings.Cut(title, " · "); found {
			categories[category] = true
		}
		targets, ok := panel["targets"].([]interface{})
		if !ok || len(targets) == 0 {
			t.Errorf("panels[%d].targets = %v, want a query target", i, panel["targets"])
			continue
		}
		target, ok := targets[0].(map[string]interface{})
		if !ok {
			t.Errorf("panels[%d].targets[0] = %T, want object", i, targets[0])
			continue
		}
		expr, _ := target["expr"].(string)
		if expr == "" {
			t.Errorf("panels[%d] has empty PromQL expression", i)
		}
		queries[expr] = true
		datasource, ok := target["datasource"].(map[string]interface{})
		if !ok || datasource["type"] != "prometheus" || datasource["uid"] != "${datasource}" {
			t.Errorf("panels[%d] datasource = %v, want templated Prometheus datasource", i, target["datasource"])
		}
	}

	for _, category := range []string{"HTTP", "Database", "Cache", "Scheduler", "System", "Authentication", "Rate limiting", "Go runtime", "Tor", "Business"} {
		if !categories[category] {
			t.Errorf("dashboard has no %q category panel", category)
		}
	}
	for _, metric := range []string{
		"airports_http_requests_total", "airports_http_request_duration_seconds_bucket", "airports_http_request_size_bytes_bucket", "airports_http_response_size_bytes_bucket", "airports_http_active_requests",
		"airports_db_queries_total", "airports_db_query_duration_seconds_bucket", "airports_db_connections_open", "airports_db_connections_in_use", "airports_db_errors_total",
		"airports_cache_hits_total", "airports_cache_misses_total", "airports_cache_evictions_total", "airports_cache_size", "airports_cache_bytes",
		"airports_scheduler_tasks_total", "airports_scheduler_task_duration_seconds_bucket", "airports_scheduler_tasks_running", "airports_scheduler_last_run_timestamp",
		"airports_system_cpu_usage_percent", "airports_system_memory_usage_percent", "airports_system_memory_used_bytes", "airports_system_memory_total_bytes", "airports_system_disk_usage_percent", "airports_system_disk_used_bytes", "airports_system_disk_total_bytes",
		"airports_auth_attempts_total", "airports_auth_sessions_active", "airports_ratelimit_requests_total", "airports_ratelimit_blocked_total",
		"airports_go_goroutines", "airports_go_mem_alloc_bytes", "airports_go_mem_sys_bytes", "airports_go_gc_runs_total", "airports_go_gc_pause_total_seconds",
		"airports_tor_enabled", "airports_tor_running", "airports_tor_circuit_established", "airports_tor_requests_total",
		"airports_search_total", "airports_nearby_total", "airports_lookup_total", "airports_export_total",
	} {
		found := false
		for query := range queries {
			if strings.Contains(query, metric) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dashboard has no panel query for required metric %s", metric)
		}
	}
}

func TestHTTPRequestDurationUsesBoundedLabels(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), t.TempDir())
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	metrics.instrumentMiddleware(next).ServeHTTP(httptest.NewRecorder(), req)

	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "airports_http_request_duration_seconds" {
			continue
		}
		labels := family.GetMetric()[0].GetLabel()
		if len(labels) != 2 || labels[0].GetName() != "method" || labels[1].GetName() != "path" {
			t.Fatalf("request duration labels = %v, want method and path only", labels)
		}
		return
	}
	t.Fatal("request duration metric not gathered")
}

func TestHTTPRequestCounterUsesSpecStatusLabel(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), t.TempDir())
	metrics.instrumentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/airports", nil))

	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "airports_http_requests_total" {
			continue
		}
		labels := family.GetMetric()[0].GetLabel()
		if len(labels) != 3 || labels[0].GetName() != "method" || labels[1].GetName() != "path" || labels[2].GetName() != "status" || labels[2].GetValue() != "201" {
			t.Fatalf("request counter labels = %v, want method, path, status=201", labels)
		}
		return
	}
	t.Fatal("request counter metric not gathered")
}

func TestDebugExpvarsRecordRequestsAndErrors(t *testing.T) {
	requestsBefore := debugRequestCount.Value()
	errorsBefore := debugErrorCount.Value()
	durationBefore := debugRequestDuration.Value()
	metrics := newMetrics(config.MetricsConfig{}, buildInfo{}, time.Now(), t.TempDir())
	metrics.instrumentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/failure", nil))
	if got := debugRequestCount.Value(); got != requestsBefore+1 {
		t.Errorf("requests_total = %d, want %d", got, requestsBefore+1)
	}
	if got := debugErrorCount.Value(); got != errorsBefore+1 {
		t.Errorf("errors_total = %d, want %d", got, errorsBefore+1)
	}
	if got := debugRequestDuration.Value(); got <= durationBefore {
		t.Errorf("requests_duration_seconds = %f, want greater than %f", got, durationBefore)
	}
}

func TestSystemMetricsCollectorExportsAllMetricsWhenEnabled(t *testing.T) {
	dataDir := t.TempDir()
	metrics := newMetrics(config.MetricsConfig{IncludeSystem: true}, buildInfo{}, time.Now(), dataDir)
	time.Sleep(20 * time.Millisecond)
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather system metrics: %v", err)
	}

	want := map[string]bool{
		"airports_system_cpu_usage_percent":    false,
		"airports_system_memory_usage_percent": false,
		"airports_system_memory_used_bytes":    false,
		"airports_system_memory_total_bytes":   false,
		"airports_system_disk_usage_percent":   false,
		"airports_system_disk_used_bytes":      false,
		"airports_system_disk_total_bytes":     false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
		}
		if strings.HasPrefix(family.GetName(), "airports_system_disk_") {
			labels := family.GetMetric()[0].GetLabel()
			if len(labels) != 1 || labels[0].GetName() != "path" || labels[0].GetValue() != dataDir {
				t.Errorf("%s labels = %v, want path=%q", family.GetName(), labels, dataDir)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("system metric %s was not gathered", name)
		}
	}
}

func TestSystemMetricsCollectorOmitsMetricsWhenDisabled(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{IncludeSystem: false}, buildInfo{}, time.Now(), t.TempDir())
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "airports_system_") {
			t.Errorf("system metric %s was gathered while include_system is false", family.GetName())
		}
	}
}

func TestGoRuntimeMetricsCollectorExportsSpecFamilies(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{IncludeRuntime: true}, buildInfo{}, time.Now(), t.TempDir())
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather runtime metrics: %v", err)
	}
	want := map[string]bool{
		"airports_go_goroutines":             false,
		"airports_go_mem_alloc_bytes":        false,
		"airports_go_mem_sys_bytes":          false,
		"airports_go_gc_runs_total":          false,
		"airports_go_gc_pause_total_seconds": false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = len(family.GetMetric()) == 1
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("runtime metric %s was not gathered", name)
		}
	}
}

func TestRegisteredMetricFamiliesUseProjectPrefix(t *testing.T) {
	metrics := newMetrics(config.MetricsConfig{IncludeRuntime: true, IncludeSystem: true}, buildInfo{}, time.Now(), "")
	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), "airports_") {
			t.Errorf("metric family %q does not use the required airports_ prefix", family.GetName())
		}
	}
}

func TestMetricsServiceRoutesAreRegisteredAndDisabledWithoutTokens(t *testing.T) {
	s := newTestServer(t)
	paths := []string{
		"/server/metrics", "/server/metrics/prometheus", "/server/metrics/grafana", "/server/metrics/loki",
		"/api/metrics", "/api/metrics/prometheus", "/api/metrics/grafana", "/api/metrics/loki",
		"/api/v1/server/metrics", "/api/v1/server/metrics/prometheus", "/api/v1/server/metrics/grafana", "/api/v1/server/metrics/loki",
		"/metrics", "/metrics/prometheus", "/metrics/grafana", "/metrics/loki",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status for %s = %d, want %d; body=%s", path, w.Code, http.StatusForbidden, w.Body.String())
			}
			if w.Body.Len() != 0 {
				t.Fatalf("disabled service response must be empty: %q", w.Body.String())
			}
		})
	}
}

func TestLegacyAirportRoutesAreRemoved(t *testing.T) {
	s := newTestServer(t)
	paths := []string{"/search", "/nearby", "/api/v1/search", "/api/v1/nearby", "/api/v1/bbox", "/api/v1/autocomplete"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Header.Set("User-Agent", "Mozilla/5.0")
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != http.StatusNotFound {
				t.Fatalf("status for legacy route %s = %d, want %d", path, w.Code, http.StatusNotFound)
			}
		})
	}
}
