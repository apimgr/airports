package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/backup"
	cachepkg "github.com/apimgr/airports/src/cache"
	commonhttp "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/apimgr/airports/src/tor"
)

// healthCheckTimeout bounds every individual component probe (database,
// cache) so a single stalled dependency can never hang the healthz
// response itself.
const healthCheckTimeout = 2 * time.Second

// healthCacheProbeKey is the cache key used to round-trip a throwaway
// value through the configured cache backend as a liveness probe. It is
// namespaced under "healthz:" so it never collides with real cached data
// and expires quickly so it never lingers.
const healthCacheProbeKey = "healthz:probe"

// healthDiskUsageThreshold is the maximum fraction of a filesystem that
// may be in use before checks.disk reports "error". 95% mirrors the
// common operational rule of thumb that a disk is functionally full
// (and at real risk of write failures) before it hits 100%.
const healthDiskUsageThreshold = 0.95

// HealthResponse is the canonical /server/healthz response shape per
// AI.md PART 13 "Field Order & Structure". Field order here IS the JSON
// field order (encoding/json preserves struct field order for objects),
// so this order must never be reshuffled without re-reading the spec.
type HealthResponse struct {
	// 1. Project identification (PART 16: branding config)
	Project ProjectInfo `json:"project"`

	// 2. Overall status includes health, restart, maintenance, and shutdown state.
	Status string `json:"status"`
	// PendingRestart is true when a loaded config change affects a setting
	// that cannot take effect until the process restarts.
	PendingRestart bool `json:"pending_restart,omitempty"`
	// RestartReason lists the names of restart-required settings that changed.
	RestartReason []string `json:"restart_reason,omitempty"`

	// 3. Version & build info (PART 7: binary requirements)
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	Build     BuildInfo `json:"build"`

	// 4. Runtime info (PART 6: application modes)
	Uptime    string    `json:"uptime"`
	Mode      string    `json:"mode"`
	Timestamp time.Time `json:"timestamp"`
	// Maintenance appears only while the process is in maintenance state.
	Maintenance *MaintenanceHealthInfo `json:"maintenance,omitempty"`

	// 5. Features - PUBLIC only (PARTS 19, 31)
	Features FeaturesInfo `json:"features"`

	// 6. Component health checks
	Checks ChecksInfo `json:"checks"`

	// 7. Statistics (public-safe aggregates)
	Stats StatsInfo `json:"stats"`
}

// MaintenanceHealthInfo carries the public-safe maintenance state. The
// message is selected from a fixed set so internal errors and paths cannot
// be copied into the public health response.
type MaintenanceHealthInfo struct {
	Reason      string                  `json:"reason"`
	Message     string                  `json:"message"`
	Since       time.Time               `json:"since"`
	SelfHealing SelfHealingHealthStatus `json:"self_healing"`
}

// SelfHealingHealthStatus reports only operational state, never an error or
// a path that could expose internal details.
type SelfHealingHealthStatus struct {
	Enabled     bool       `json:"enabled"`
	Attempts    int        `json:"attempts"`
	LastAttempt *time.Time `json:"last_attempt,omitempty"`
	NextAttempt *time.Time `json:"next_attempt,omitempty"`
}

// HealthRuntimeState holds lifecycle state that is not derivable from
// dependency checks. Callers update it when restart, maintenance, or
// shutdown state changes.
type HealthRuntimeState struct {
	ShuttingDown   bool
	PendingRestart bool
	RestartReason  []string
	Maintenance    *MaintenanceHealthInfo
}

var (
	healthRuntimeStateMu sync.RWMutex
	healthRuntimeState   HealthRuntimeState
)

// SetHealthRuntimeState updates health lifecycle state. Maintenance messages
// are replaced with safe fixed text and reasons are reduced to known codes.
func SetHealthRuntimeState(state HealthRuntimeState) {
	state.RestartReason = sanitizeRestartReasons(state.RestartReason)
	state.PendingRestart = state.PendingRestart && len(state.RestartReason) > 0
	if state.Maintenance != nil {
		maintenance := *state.Maintenance
		maintenance.Reason, maintenance.Message = safeMaintenanceMessage(maintenance.Reason)
		if maintenance.Since.IsZero() {
			maintenance.Since = time.Now().UTC()
		}
		if maintenance.SelfHealing.Attempts < 0 {
			maintenance.SelfHealing.Attempts = 0
		}
		if maintenance.SelfHealing.LastAttempt != nil {
			lastAttempt := maintenance.SelfHealing.LastAttempt.UTC()
			maintenance.SelfHealing.LastAttempt = &lastAttempt
		}
		if maintenance.SelfHealing.NextAttempt != nil {
			nextAttempt := maintenance.SelfHealing.NextAttempt.UTC()
			maintenance.SelfHealing.NextAttempt = &nextAttempt
		}
		state.Maintenance = &maintenance
	}
	healthRuntimeStateMu.Lock()
	healthRuntimeState = state
	healthRuntimeStateMu.Unlock()
}

// SetHealthShuttingDown marks the process as draining while preserving other
// reported lifecycle state.
func SetHealthShuttingDown(shuttingDown bool) {
	state := currentHealthRuntimeState()
	state.ShuttingDown = shuttingDown
	SetHealthRuntimeState(state)
}

// SetPendingRestartSettings replaces the process's accumulated restart-only
// config differences. An empty list clears restart-required status.
func SetPendingRestartSettings(settings []string) {
	state := currentHealthRuntimeState()
	state.RestartReason = sanitizeRestartReasons(settings)
	state.PendingRestart = len(state.RestartReason) > 0
	SetHealthRuntimeState(state)
}

// SetHealthMaintenance updates maintenance state without changing shutdown
// or pending-restart state. Passing nil marks maintenance as recovered.
func SetHealthMaintenance(maintenance *MaintenanceHealthInfo) {
	state := currentHealthRuntimeState()
	state.Maintenance = maintenance
	SetHealthRuntimeState(state)
}

func sanitizeRestartReasons(reasons []string) []string {
	result := make([]string, 0, len(reasons))
	seen := make(map[string]bool, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" || len(reason) > 128 || seen[reason] {
			continue
		}
		valid := true
		for _, r := range reason {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-' {
				valid = false
				break
			}
		}
		if valid {
			result = append(result, reason)
			seen[reason] = true
		}
	}
	return result
}

func currentHealthRuntimeState() HealthRuntimeState {
	healthRuntimeStateMu.RLock()
	defer healthRuntimeStateMu.RUnlock()
	state := healthRuntimeState
	state.RestartReason = append([]string(nil), state.RestartReason...)
	if state.Maintenance != nil {
		maintenance := *state.Maintenance
		state.Maintenance = &maintenance
	}
	return state
}

func safeMaintenanceMessage(reason string) (string, string) {
	switch reason {
	case "database_connection":
		return reason, "Database connection failed"
	case "database_write":
		return reason, "Database write failed"
	case "disk_full":
		return reason, "Available disk space is critically low"
	case "file_permission":
		return reason, "Required file access failed"
	case "operator_requested":
		return reason, "Maintenance mode was started by an operator"
	default:
		return "unknown", "A critical service failure occurred"
	}
}

// ProjectInfo carries branding config (AI.md PART 16).
type ProjectInfo struct {
	Name        string `json:"name"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
}

// BuildInfo carries build-time variables (AI.md PART 7).
type BuildInfo struct {
	Commit string `json:"commit"`
	Date   string `json:"date"`
}

// FeaturesInfo lists PUBLIC, non-negotiable features only. /metrics
// (PART 20, internal-only) is never surfaced here.
type FeaturesInfo struct {
	Tor   TorInfo `json:"tor"`
	I2P   I2PInfo `json:"i2p"`
	GeoIP bool    `json:"geoip"`
}

// TorInfo reflects the dedicated Tor manager's live state (AI.md PART 31).
type TorInfo struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
}

// I2PInfo reflects the opt-in I2P feature and current provider lifecycle.
type I2PInfo struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
	Provider string `json:"provider"`
}

// ChecksInfo reports component health as "ok"/"error" only — never
// connection strings, paths, or any other implementation detail
// (AI.md PART 13 "Security: Public Info Only").
type ChecksInfo struct {
	Database  string `json:"database"`
	Cache     string `json:"cache"`
	Disk      string `json:"disk"`
	Scheduler string `json:"scheduler"`
	Tor       string `json:"tor,omitempty"`
	I2P       string `json:"i2p,omitempty"`
}

// StatsInfo carries public-safe aggregate counters only.
type StatsInfo struct {
	RequestsTotal int64 `json:"requests_total"`
	Requests24h   int64 `json:"requests_24h"`
	ActiveConns   int   `json:"active_connections"`
}

// formatUptime renders d as a compact, human-readable duration such as
// "2d 5h 30m", per AI.md PART 13's "uptime" example. Leading zero-value
// units are omitted, but the least significant unit (minutes) is always
// shown — including "0m" for an uptime under a minute — so the field is
// never empty.
func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalMinutes := int64(d / time.Minute)
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd ", days)
	}
	if days > 0 || hours > 0 {
		fmt.Fprintf(&b, "%dh ", hours)
	}
	fmt.Fprintf(&b, "%dm", minutes)
	return b.String()
}

// checkDatabase pings db and reports "ok"/"error" only — per PART 13,
// no connection string, host, or path ever leaves this function. A nil
// db (healthz built without a database dependency, e.g. in tests) is
// reported as "error" since the component is not actually reachable.
func checkDatabase(db *sql.DB) string {
	if db == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return "error"
	}
	return "ok"
}

// checkCache round-trips a small, short-lived probe value through the
// configured cache backend. A nil cache (never expected in production —
// New() always falls back to an in-memory cache — but possible in tests)
// reports "error".
func checkCache(cache cachepkg.Cache) string {
	if cache == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	if err := cache.Set(ctx, healthCacheProbeKey, []byte("1"), 5*time.Second); err != nil {
		return "error"
	}
	if _, err := cache.Get(ctx, healthCacheProbeKey); err != nil {
		return "error"
	}
	return "ok"
}

func (s *Server) checkCache() string {
	if s == nil || s.cache == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	setErr := s.cache.Set(ctx, healthCacheProbeKey, []byte("1"), 5*time.Second)
	if setErr != nil {
		return "error"
	}
	_, getErr := s.cache.Get(ctx, healthCacheProbeKey)
	if getErr != nil {
		return "error"
	}
	return "ok"
}

// checkDisk reports "error" when dataDir's filesystem cannot be statted,
// or is at/above healthDiskUsageThreshold full. Only ok/error ever
// leaves this function — never the path, total size, or free space.
func checkDisk(dataDir string) string {
	if dataDir == "" {
		return "error"
	}
	total, free, err := backup.DiskUsage(dataDir)
	if err != nil || total == 0 {
		return "error"
	}
	used := 1 - (float64(free) / float64(total))
	if used >= healthDiskUsageThreshold {
		return "error"
	}
	return "ok"
}

// checkScheduler reports "ok" when the scheduler is present and able to
// report its task list without panicking, "error" otherwise. A nil
// scheduler (e.g. in unit tests that don't wire one up) is "error".
func checkScheduler(sched *scheduler.Scheduler) (status string) {
	if sched == nil {
		return "error"
	}
	defer func() {
		if recover() != nil {
			status = "error"
		}
	}()
	sched.List()
	return "ok"
}

// checkTor reports "ok"/"error" for the Tor component check
// (checks.tor), or "" when Tor is disabled/not configured — the caller
// relies on ChecksInfo.Tor's `omitempty` tag to drop the field entirely
// in that case, matching AI.md PART 13's example where checks.tor is
// only present "(if enabled)".
func checkTor(mgr *tor.Manager) string {
	if mgr == nil || !mgr.Enabled() {
		return ""
	}
	if err := mgr.HealthCheck(); err != nil {
		return "error"
	}
	return "ok"
}

func checkI2P(mgr *i2p.Manager) string {
	if mgr == nil || !mgr.Enabled() {
		return ""
	}
	if !mgr.Running() {
		return "error"
	}
	return "ok"
}

func i2pFeatureInfo(mgr *i2p.Manager) I2PInfo {
	if mgr == nil || !mgr.Enabled() {
		return I2PInfo{Status: "disabled", Provider: "none"}
	}
	info := I2PInfo{Enabled: true, Running: mgr.Running(), Hostname: mgr.EepsiteAddress(), Provider: string(mgr.Provider())}
	if info.Running {
		info.Status = "healthy"
	} else {
		info.Status = "error"
	}
	return info
}

// torFeatureInfo builds the features.tor object. Per AI.md PART 13, this
// object is always present (unlike checks.tor) — Enabled/Running reflect
// actual state even when Tor is fully disabled.
func torFeatureInfo(mgr *tor.Manager) TorInfo {
	if mgr == nil || !mgr.Enabled() {
		return TorInfo{Status: "disabled"}
	}

	info := TorInfo{
		Enabled:  true,
		Hostname: mgr.OnionAddress(),
	}
	info.Running = info.Hostname != ""

	switch err := mgr.HealthCheck(); {
	case err == nil:
		info.Status = "healthy"
	case errors.Is(err, tor.ErrNotRunning):
		info.Status = "starting"
	default:
		info.Status = "error:" + err.Error()
	}
	return info
}

// overallStatus derives the top-level "status" field from the component
// checks: any hard failure among database/cache/scheduler makes the
// whole server "unhealthy"; a disk or (enabled) Tor failure alone is
// treated as "degraded" since the server keeps serving requests either
// way; otherwise "healthy".
func overallStatus(checks ChecksInfo) string {
	if checks.Database == "error" || checks.Cache == "error" || checks.Scheduler == "error" {
		return "unhealthy"
	}
	if checks.Disk == "error" || checks.Tor == "error" || checks.I2P == "error" {
		return "degraded"
	}
	return "healthy"
}

func statusWithRuntimeState(checks ChecksInfo, state HealthRuntimeState) string {
	switch {
	case state.ShuttingDown:
		return "shutting_down"
	case state.Maintenance != nil:
		return "maintenance"
	case overallStatus(checks) == "unhealthy":
		return "unhealthy"
	case state.PendingRestart:
		return "restart_required"
	default:
		return overallStatus(checks)
	}
}

func healthHTTPStatus(status string) int {
	switch status {
	case "unhealthy", "maintenance", "shutting_down":
		return http.StatusServiceUnavailable
	default:
		return http.StatusOK
	}
}

// buildHealthResponse assembles the full HealthResponse from the
// server's live dependencies. Every data source is documented inline
// against the AI.md PART 13 "Data Sources" table.
func buildHealthResponse(s *Server) HealthResponse {
	branding := s.config.Server.Branding
	runtimeState := currentHealthRuntimeState()

	checks := ChecksInfo{
		Database:  checkDatabase(s.db),
		Cache:     s.checkCache(),
		Disk:      checkDisk(s.dataDir),
		Scheduler: checkScheduler(s.scheduler),
		Tor:       checkTor(s.tor),
		I2P:       checkI2P(s.i2p),
	}

	var uptime string
	if s.startTime.IsZero() {
		uptime = formatUptime(0)
	} else {
		uptime = formatUptime(time.Since(s.startTime))
	}

	var requestsTotal int64
	var requests24h int64
	var activeConns int
	if s.stats != nil {
		requestsTotal = s.stats.requestsTotalCount()
		requests24h = s.stats.requests24h(time.Now())
		activeConns = s.stats.activeConnections()
	}

	response := HealthResponse{
		Project: ProjectInfo{
			Name:        branding.Title,
			Tagline:     branding.Tagline,
			Description: branding.Description,
		},
		Status:    statusWithRuntimeState(checks, runtimeState),
		Version:   Version,
		GoVersion: runtime.Version(),
		Build: BuildInfo{
			Commit: CommitID,
			Date:   BuildDate,
		},
		Uptime:      uptime,
		Mode:        s.config.Server.Mode,
		Timestamp:   time.Now().UTC(),
		Maintenance: runtimeState.Maintenance,
		Features: FeaturesInfo{
			Tor:   torFeatureInfo(s.tor),
			I2P:   i2pFeatureInfo(s.i2p),
			GeoIP: s.config.Server.GeoIP.Enabled,
		},
		Checks: checks,
		Stats: StatsInfo{
			RequestsTotal: requestsTotal,
			Requests24h:   requests24h,
			ActiveConns:   activeConns,
		},
	}
	if response.Status == "restart_required" {
		response.PendingRestart = true
		response.RestartReason = runtimeState.RestartReason
	}
	return response
}

// healthResponseText renders resp as flattened dot-notation plain text,
// in the exact field order, section comments, and grouping given by
// AI.md PART 13's "Plain Text (Accept: text/plain)" example — including
// its section-6 header appearing directly after "features.geoip" with
// no blank line, unlike every other section boundary. That placement is
// reproduced verbatim/intentionally to match the spec's example exactly.
func healthResponseText(resp HealthResponse) string {
	var b strings.Builder

	b.WriteString("# 1. Project (PART 16: branding)\n")
	fmt.Fprintf(&b, "project.name: %s\n", resp.Project.Name)
	fmt.Fprintf(&b, "project.tagline: %s\n", resp.Project.Tagline)
	fmt.Fprintf(&b, "project.description: %s\n\n", resp.Project.Description)

	b.WriteString("# 2. Status\n")
	fmt.Fprintf(&b, "status: %s\n", resp.Status)
	if resp.PendingRestart {
		b.WriteString("pending_restart: true\n")
		if len(resp.RestartReason) > 0 {
			fmt.Fprintf(&b, "restart_reason: %s\n", strings.Join(resp.RestartReason, ","))
		}
	}
	b.WriteString("\n")

	b.WriteString("# 3. Version & Build (PART 7)\n")
	fmt.Fprintf(&b, "version: %s\n", resp.Version)
	fmt.Fprintf(&b, "go_version: %s\n", resp.GoVersion)
	fmt.Fprintf(&b, "build.commit: %s\n", resp.Build.Commit)
	fmt.Fprintf(&b, "build.date: %s\n\n", resp.Build.Date)

	b.WriteString("# 4. Runtime (PART 6)\n")
	fmt.Fprintf(&b, "uptime: %s\n", resp.Uptime)
	fmt.Fprintf(&b, "mode: %s\n", resp.Mode)
	fmt.Fprintf(&b, "timestamp: %s\n", resp.Timestamp.Format(time.RFC3339))
	if resp.Maintenance != nil {
		fmt.Fprintf(&b, "maintenance.reason: %s\n", resp.Maintenance.Reason)
		fmt.Fprintf(&b, "maintenance.message: %s\n", resp.Maintenance.Message)
		fmt.Fprintf(&b, "maintenance.since: %s\n", resp.Maintenance.Since.UTC().Format(time.RFC3339))
		fmt.Fprintf(&b, "maintenance.self_healing.enabled: %t\n", resp.Maintenance.SelfHealing.Enabled)
		fmt.Fprintf(&b, "maintenance.self_healing.attempts: %d\n", resp.Maintenance.SelfHealing.Attempts)
		if resp.Maintenance.SelfHealing.LastAttempt != nil {
			fmt.Fprintf(&b, "maintenance.self_healing.last_attempt: %s\n", resp.Maintenance.SelfHealing.LastAttempt.UTC().Format(time.RFC3339))
		}
		if resp.Maintenance.SelfHealing.NextAttempt != nil {
			fmt.Fprintf(&b, "maintenance.self_healing.next_attempt: %s\n", resp.Maintenance.SelfHealing.NextAttempt.UTC().Format(time.RFC3339))
		}
	}
	b.WriteString("\n")

	b.WriteString("# 5. Features - NON-NEGOTIABLE only (show actual status)\n")
	fmt.Fprintf(&b, "features.tor.enabled: %t\n", resp.Features.Tor.Enabled)
	fmt.Fprintf(&b, "features.tor.running: %t\n", resp.Features.Tor.Running)
	fmt.Fprintf(&b, "features.tor.status: %s\n", resp.Features.Tor.Status)
	fmt.Fprintf(&b, "features.tor.hostname: %s\n", resp.Features.Tor.Hostname)
	fmt.Fprintf(&b, "features.i2p.enabled: %t\n", resp.Features.I2P.Enabled)
	fmt.Fprintf(&b, "features.i2p.running: %t\n", resp.Features.I2P.Running)
	fmt.Fprintf(&b, "features.i2p.status: %s\n", resp.Features.I2P.Status)
	fmt.Fprintf(&b, "features.i2p.hostname: %s\n", resp.Features.I2P.Hostname)
	fmt.Fprintf(&b, "features.i2p.provider: %s\n", resp.Features.I2P.Provider)
	fmt.Fprintf(&b, "features.geoip: %t\n", resp.Features.GeoIP)
	b.WriteString("# 6. Checks\n")
	fmt.Fprintf(&b, "checks.database: %s\n", resp.Checks.Database)
	fmt.Fprintf(&b, "checks.cache: %s\n", resp.Checks.Cache)
	fmt.Fprintf(&b, "checks.disk: %s\n", resp.Checks.Disk)
	fmt.Fprintf(&b, "checks.scheduler: %s\n", resp.Checks.Scheduler)
	if resp.Checks.Tor != "" {
		fmt.Fprintf(&b, "checks.tor: %s\n", resp.Checks.Tor)
	}
	if resp.Checks.I2P != "" {
		fmt.Fprintf(&b, "checks.i2p: %s\n", resp.Checks.I2P)
	}
	b.WriteString("\n")

	b.WriteString("# 7. Stats\n")
	fmt.Fprintf(&b, "stats.requests_total: %d\n", resp.Stats.RequestsTotal)
	fmt.Fprintf(&b, "stats.requests_24h: %d\n", resp.Stats.Requests24h)
	fmt.Fprintf(&b, "stats.active_connections: %d\n", resp.Stats.ActiveConns)

	return b.String()
}

// wantsHealthzText reports whether r should receive the flattened
// plain-text representation instead of HTML or JSON, per AI.md PART 14's
// standard content-negotiation rules: an explicit Accept: text/plain, or
// a non-interactive client (curl/wget/httpie identify via User-Agent and
// never send an HTML Accept header).
func wantsHealthzText(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return commonhttp.GetAPIResponseFormat(r) == "text"
	}
	return commonhttp.GetFrontendResponseFormat(r) == "text"
}

// handleServerHealthz serves /server/healthz, /healthz (optional root
// alias), /api/healthz, and /api/{api_version}/server/healthz — all four
// routes are mounted to this single handler (AI.md PART 13: "No
// redirect, no forked logic, no separate formatter path"). Content
// negotiation follows AI.md PART 14: browsers get HTML, text/plain or
// non-interactive clients get flattened dot-notation text, everything
// else (including the API's JSON default) gets 2-space indented JSON.
func (s *Server) handleServerHealthz(w http.ResponseWriter, r *http.Request) {
	resp := buildHealthResponse(s)
	statusCode := healthHTTPStatus(resp.Status)
	apiRoute := strings.HasPrefix(r.URL.Path, "/api/")
	format := commonhttp.GetFrontendResponseFormat(r)
	if apiRoute {
		format = commonhttp.GetAPIResponseFormat(r)
	}
	switch {
	case !apiRoute && (format == "html" || format == "nojs"):
		w.WriteHeader(statusCode)
		lang := i18n.FromContext(r.Context())
		s.renderTemplate(w, r, "server_healthz", map[string]interface{}{
			"Title":    fmt.Sprintf("%s - %s", resp.Project.Name, i18n.T(lang, "health.title")),
			"Health":   resp,
			"Requests": formatWithCommas(resp.Stats.RequestsTotal),
			"Req24h":   formatWithCommas(resp.Stats.Requests24h),
		})
	case wantsHealthzText(r):
		s.respondText(w, statusCode, healthResponseText(resp))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
	}
}

// formatWithCommas renders n with thousands separators (e.g. 1234567 ->
// "1,234,567") for the HTML stats display per AI.md PART 13's "Stat
// number" display rule. Negative values are not expected for these
// counters, but are handled by prefixing the sign before grouping.
func formatWithCommas(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	return out
}
