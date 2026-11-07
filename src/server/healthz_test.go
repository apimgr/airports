package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0m"},
		{"negative floors to zero", -5 * time.Minute, "0m"},
		{"sub-minute", 30 * time.Second, "0m"},
		{"minutes only", 45 * time.Minute, "45m"},
		{"exact hour", 2 * time.Hour, "2h 0m"},
		{"hours and minutes", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"multi-day", 50*time.Hour + 5*time.Minute, "2d 2h 5m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatUptime(tt.d); got != tt.want {
				t.Errorf("formatUptime(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestHealthHTTPStatus(t *testing.T) {
	tests := map[string]int{
		"healthy":          http.StatusOK,
		"degraded":         http.StatusOK,
		"restart_required": http.StatusOK,
		"unhealthy":        http.StatusServiceUnavailable,
		"maintenance":      http.StatusServiceUnavailable,
		"shutting_down":    http.StatusServiceUnavailable,
	}
	for status, want := range tests {
		if got := healthHTTPStatus(status); got != want {
			t.Errorf("healthHTTPStatus(%q) = %d, want %d", status, got, want)
		}
	}
}

func TestStatusWithRuntimeState(t *testing.T) {
	checks := ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok"}
	tests := []struct {
		name   string
		state  HealthRuntimeState
		want   string
		checks ChecksInfo
	}{
		{name: "healthy", want: "healthy"},
		{name: "restart required", state: HealthRuntimeState{PendingRestart: true}, want: "restart_required"},
		{name: "unhealthy outranks restart", state: HealthRuntimeState{PendingRestart: true}, want: "unhealthy", checks: ChecksInfo{Database: "error", Cache: "ok", Disk: "ok", Scheduler: "ok"}},
		{name: "maintenance", state: HealthRuntimeState{Maintenance: &MaintenanceHealthInfo{}}, want: "maintenance"},
		{name: "shutdown outranks maintenance", state: HealthRuntimeState{ShuttingDown: true, Maintenance: &MaintenanceHealthInfo{}}, want: "shutting_down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputChecks := tt.checks
			if inputChecks == (ChecksInfo{}) {
				inputChecks = checks
			}
			if got := statusWithRuntimeState(inputChecks, tt.state); got != tt.want {
				t.Errorf("statusWithRuntimeState(%+v, %+v) = %q, want %q", inputChecks, tt.state, got, tt.want)
			}
		})
	}
}

func TestSetHealthRuntimeStateSanitizesPublicMaintenanceData(t *testing.T) {
	SetHealthRuntimeState(HealthRuntimeState{
		PendingRestart: true,
		RestartReason:  []string{"ssl.cert", "server.port", "secret=private", "ssl.cert"},
		Maintenance: &MaintenanceHealthInfo{
			Reason:      "database_connection",
			Message:     "database failed at /private/path with token secret",
			SelfHealing: SelfHealingHealthStatus{Enabled: true, Attempts: -1},
		},
	})
	defer SetHealthRuntimeState(HealthRuntimeState{})

	state := currentHealthRuntimeState()
	if len(state.RestartReason) != 2 || state.RestartReason[0] != "ssl.cert" || state.RestartReason[1] != "server.port" {
		t.Errorf("RestartReason = %v, want safe unique setting names", state.RestartReason)
	}
	if state.Maintenance == nil {
		t.Fatal("Maintenance = nil, want sanitized maintenance state")
	}
	if state.Maintenance.Message != "Database connection failed" || state.Maintenance.Reason != "database_connection" {
		t.Errorf("Maintenance = %+v, want fixed public-safe fields", state.Maintenance)
	}
	if state.Maintenance.SelfHealing.Attempts != 0 {
		t.Errorf("SelfHealing.Attempts = %d, want 0 for negative input", state.Maintenance.SelfHealing.Attempts)
	}
}

func TestCheckDatabaseNil(t *testing.T) {
	if got := checkDatabase(nil); got != "error" {
		t.Errorf("checkDatabase(nil) = %q, want %q", got, "error")
	}
}

func TestCheckCacheNil(t *testing.T) {
	if got := checkCache(nil); got != "error" {
		t.Errorf("checkCache(nil) = %q, want %q", got, "error")
	}
}

func TestCheckDiskEmptyPath(t *testing.T) {
	if got := checkDisk(""); got != "error" {
		t.Errorf("checkDisk(\"\") = %q, want %q", got, "error")
	}
}

func TestCheckDiskValidPath(t *testing.T) {
	if got := checkDisk(t.TempDir()); got != "ok" {
		t.Errorf("checkDisk(tempdir) = %q, want %q", got, "ok")
	}
}

func TestCheckSchedulerNil(t *testing.T) {
	if got := checkScheduler(nil); got != "error" {
		t.Errorf("checkScheduler(nil) = %q, want %q", got, "error")
	}
}

func TestCheckTorNil(t *testing.T) {
	if got := checkTor(nil); got != "" {
		t.Errorf("checkTor(nil) = %q, want empty string (omitted)", got)
	}
}

func TestTorFeatureInfoNil(t *testing.T) {
	info := torFeatureInfo(nil)
	if info.Enabled {
		t.Errorf("torFeatureInfo(nil).Enabled = true, want false")
	}
	if info.Running {
		t.Errorf("torFeatureInfo(nil).Running = true, want false")
	}
	if info.Status != "disabled" {
		t.Errorf("torFeatureInfo(nil).Status = %q, want %q", info.Status, "disabled")
	}
	if info.Hostname != "" {
		t.Errorf("torFeatureInfo(nil).Hostname = %q, want empty", info.Hostname)
	}
}

func TestOverallStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks ChecksInfo
		want   string
	}{
		{"all ok", ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok"}, "healthy"},
		{"database error is unhealthy", ChecksInfo{Database: "error", Cache: "ok", Disk: "ok", Scheduler: "ok"}, "unhealthy"},
		{"cache error is unhealthy", ChecksInfo{Database: "ok", Cache: "error", Disk: "ok", Scheduler: "ok"}, "unhealthy"},
		{"scheduler error is unhealthy", ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "error"}, "unhealthy"},
		{"disk error alone is degraded", ChecksInfo{Database: "ok", Cache: "ok", Disk: "error", Scheduler: "ok"}, "degraded"},
		{"tor error alone is degraded", ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok", Tor: "error"}, "degraded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := overallStatus(tt.checks); got != tt.want {
				t.Errorf("overallStatus(%+v) = %q, want %q", tt.checks, got, tt.want)
			}
		})
	}
}

// TestBuildHealthResponseNilDependencies verifies buildHealthResponse never
// panics and reports the documented "everything unreachable" shape when the
// server has no db/cache/scheduler/tor wired up (the newTestServer() case).
func TestBuildHealthResponseNilDependencies(t *testing.T) {
	s := newTestServer(t)
	resp := buildHealthResponse(s)

	if resp.Checks.Database != "error" {
		t.Errorf("Checks.Database = %q, want %q", resp.Checks.Database, "error")
	}
	// New() always wires a real in-memory cache (PART 9 default), even
	// when the caller passes no explicit cache config, so this reports
	// "ok" rather than "error" — unlike db/scheduler/tor which stay nil
	// in newTestServer().
	if resp.Checks.Cache != "ok" {
		t.Errorf("Checks.Cache = %q, want %q", resp.Checks.Cache, "ok")
	}
	if resp.Checks.Scheduler != "error" {
		t.Errorf("Checks.Scheduler = %q, want %q", resp.Checks.Scheduler, "error")
	}
	if resp.Checks.Disk != "ok" {
		t.Errorf("Checks.Disk = %q, want %q (real temp dir)", resp.Checks.Disk, "ok")
	}
	if resp.Checks.Tor != "" {
		t.Errorf("Checks.Tor = %q, want empty (disabled -> omitted)", resp.Checks.Tor)
	}
	if resp.Features.Tor.Enabled {
		t.Errorf("Features.Tor.Enabled = true, want false")
	}
	if resp.Features.Tor.Status != "disabled" {
		t.Errorf("Features.Tor.Status = %q, want %q", resp.Features.Tor.Status, "disabled")
	}
	if resp.Features.I2P.Enabled || resp.Features.I2P.Running || resp.Features.I2P.Status != "disabled" || resp.Features.I2P.Provider != "none" {
		t.Errorf("Features.I2P = %+v, want disabled feature with no provider", resp.Features.I2P)
	}
	if resp.Status != "unhealthy" {
		t.Errorf("Status = %q, want %q (db/cache/scheduler all failing)", resp.Status, "unhealthy")
	}
	if resp.Uptime == "" {
		t.Errorf("Uptime is empty, want a formatted duration even for zero startTime")
	}
	if resp.GoVersion == "" {
		t.Errorf("GoVersion is empty, want runtime.Version()")
	}
	if resp.PendingRestart {
		t.Errorf("PendingRestart = true, want false (never set today)")
	}
	if len(resp.RestartReason) != 0 {
		t.Errorf("RestartReason = %v, want empty", resp.RestartReason)
	}
}

func TestHealthResponseTextFormat(t *testing.T) {
	resp := HealthResponse{
		Project:   ProjectInfo{Name: "Airports API", Tagline: "Fly", Description: "Desc"},
		Status:    "healthy",
		Version:   "1.0.0",
		GoVersion: "go1.23.0",
		Build:     BuildInfo{Commit: "abc1234", Date: "2026-01-01T00:00:00Z"},
		Uptime:    "2d 5h 30m",
		Mode:      "production",
		Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Features: FeaturesInfo{
			Tor:   TorInfo{Status: "disabled"},
			I2P:   I2PInfo{Status: "disabled", Provider: "none"},
			GeoIP: true,
		},
		Checks: ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok"},
		Stats:  StatsInfo{RequestsTotal: 10, Requests24h: 5, ActiveConns: 1},
	}

	text := healthResponseText(resp)

	// Section header order (verbatim per AI.md PART 13 example).
	sections := []string{
		"# 1. Project (PART 16: branding)",
		"# 2. Status",
		"# 3. Version & Build (PART 7)",
		"# 4. Runtime (PART 6)",
		"# 5. Features - NON-NEGOTIABLE only (show actual status)",
		"# 6. Checks",
		"# 7. Stats",
	}
	lastIdx := -1
	for _, section := range sections {
		idx := strings.Index(text, section)
		if idx == -1 {
			t.Fatalf("missing section header %q in:\n%s", section, text)
		}
		if idx <= lastIdx {
			t.Fatalf("section header %q out of order", section)
		}
		lastIdx = idx
	}

	// checks.tor is omitted entirely when the Tor check never ran.
	if strings.Contains(text, "checks.tor:") {
		t.Errorf("text output contains checks.tor when resp.Checks.Tor is empty:\n%s", text)
	}

	// The spec's example places "# 6. Checks" immediately after
	// "features.geoip" with no blank line, unlike every other section
	// boundary — verify that quirk is reproduced verbatim.
	want := "features.geoip: true\n# 6. Checks\n"
	if !strings.Contains(text, want) {
		t.Errorf("expected no blank line between features.geoip and # 6. Checks, got:\n%s", text)
	}
	if !strings.Contains(text, "features.i2p.provider: none\n") {
		t.Errorf("missing disabled I2P feature details:\n%s", text)
	}

	if !strings.Contains(text, "project.name: Airports API\n") {
		t.Errorf("missing project.name line:\n%s", text)
	}
	if !strings.Contains(text, "stats.active_connections: 1\n") {
		t.Errorf("missing stats.active_connections line:\n%s", text)
	}
}

func TestHealthResponseTextIncludesTorCheckWhenPresent(t *testing.T) {
	resp := HealthResponse{
		Checks: ChecksInfo{Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok", Tor: "ok"},
	}
	text := healthResponseText(resp)
	if !strings.Contains(text, "checks.tor: ok\n") {
		t.Errorf("expected checks.tor line when Checks.Tor is set:\n%s", text)
	}
}

func TestHealthResponseTextShowsRuntimeState(t *testing.T) {
	lastAttempt := time.Date(2026, 1, 2, 3, 3, 0, 0, time.UTC)
	nextAttempt := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	resp := HealthResponse{
		Status:         "restart_required",
		PendingRestart: true,
		RestartReason:  []string{"server.port", "ssl.enabled"},
		Maintenance: &MaintenanceHealthInfo{
			Reason:  "disk_full",
			Message: "Available disk space is critically low",
			Since:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			SelfHealing: SelfHealingHealthStatus{
				Enabled: true, Attempts: 2,
				LastAttempt: &lastAttempt,
				NextAttempt: &nextAttempt,
			},
		},
	}
	text := healthResponseText(resp)
	for _, field := range []string{
		"status: restart_required\n",
		"pending_restart: true\n",
		"restart_reason: server.port,ssl.enabled\n",
		"maintenance.reason: disk_full\n",
		"maintenance.self_healing.attempts: 2\n",
	} {
		if !strings.Contains(text, field) {
			t.Errorf("health text missing %q:\n%s", field, text)
		}
	}
}

func TestWantsHealthzText(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		ua     string
		want   bool
	}{
		{"explicit text/plain", "text/plain", "", true},
		{"explicit text/html", "text/html", "", false},
		{"browser application/json accept", "application/json", "Mozilla/5.0", false},
		{"curl user agent, no accept", "", "curl/8.0.0", true},
		{"wget user agent, no accept", "", "Wget/1.21", true},
		{"browser user agent, no accept", "", "Mozilla/5.0", false},
		{"no accept, no user agent", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
			req.Header.Del("User-Agent")
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if tt.ua != "" {
				req.Header.Set("User-Agent", tt.ua)
			}
			if got := wantsHealthzText(req); got != tt.want {
				t.Errorf("wantsHealthzText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatWithCommas(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{5, "5"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
	}
	for _, tt := range tests {
		if got := formatWithCommas(tt.n); got != tt.want {
			t.Errorf("formatWithCommas(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestHandleServerHealthzJSONShape verifies the default (no Accept header)
// response is valid, 2-space-indented JSON containing the documented
// top-level fields, and that omitempty actually drops pending_restart /
// restart_reason / checks.tor when they are zero-valued.
func TestHandleServerHealthzJSONShape(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("User-Agent", "airports-cli/integration-test")
	rec := httptest.NewRecorder()

	s.handleServerHealthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body := rec.Body.String()
	if !strings.HasPrefix(body, "{\n  ") {
		n := len(body)
		if n > 20 {
			n = 20
		}
		t.Errorf("body is not 2-space-indented JSON, starts with: %q", body[:n])
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	for _, field := range []string{"project", "status", "version", "go_version", "build", "uptime", "mode", "timestamp", "features", "checks", "stats"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing top-level field %q in JSON response", field)
		}
	}
	for _, field := range []string{"pending_restart", "restart_reason"} {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q present but should be omitted (omitempty, zero value)", field)
		}
	}

	var checks map[string]json.RawMessage
	if err := json.Unmarshal(raw["checks"], &checks); err != nil {
		t.Fatalf("checks is not an object: %v", err)
	}
	if _, ok := checks["tor"]; ok {
		t.Errorf("checks.tor present but should be omitted (Tor disabled in test server)")
	}
}

func TestHandleServerHealthzTextNegotiation(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	s.handleServerHealthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# 1. Project (PART 16: branding)") {
		t.Errorf("text response missing section header, got:\n%s", body)
	}
	if strings.HasPrefix(body, "{") {
		t.Errorf("text negotiation returned JSON body")
	}
}

func TestHandleServerHealthzHTMLNegotiation(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	s.handleServerHealthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHealthzAPIRouteUsesAPIFormats(t *testing.T) {
	s := newTestServer(t)
	tests := []struct {
		name        string
		path        string
		userAgent   string
		accept      string
		contentType string
		status      int
	}{
		{name: "HTML accept stays JSON on API", path: "/api/v1/server/healthz", userAgent: "Mozilla/5.0", accept: "text/html", contentType: "application/json", status: http.StatusServiceUnavailable},
		{name: "HTTP tool gets text", path: "/api/v1/server/healthz", userAgent: "curl/8.0", contentType: "text/plain", status: http.StatusServiceUnavailable},
		{name: "txt suffix gets text", path: "/api/v1/server/healthz.txt", userAgent: "Mozilla/5.0", accept: "text/html", contentType: "text/plain", status: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			r.Header.Set("User-Agent", tt.userAgent)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.status, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("Content-Type = %q, want %q", got, tt.contentType)
			}
		})
	}
}
