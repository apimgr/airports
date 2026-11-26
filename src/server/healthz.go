package server

import (
	"context"
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
	commonformat "github.com/apimgr/airports/src/common/format"
	commonhttp "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/apimgr/airports/src/tor"
)

// healthCheckTimeout bounds every individual component probe (database,
// cache) so a single stalled dependency can never hang the healthz
// response itself.
const healthCheckTimeout = 2 * time.Second

var (
	databaseHealthProbeSlot  = make(chan struct{}, 1)
	cacheHealthProbeSlot     = make(chan struct{}, 1)
	diskHealthProbeSlot      = make(chan struct{}, 1)
	schedulerHealthProbeSlot = make(chan struct{}, 1)
	torHealthProbeSlot       = make(chan struct{}, 1)
	i2pHealthProbeSlot       = make(chan struct{}, 1)
)

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
	// Maintenance appears only while the process is in maintenance state.
	Maintenance *MaintenanceHealthInfo `json:"maintenance,omitempty"`

	// 3. Version & build info (PART 7: binary requirements)
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	Build     BuildInfo `json:"build"`

	// 4. Runtime info (PART 6: application modes)
	Uptime    string    `json:"uptime"`
	Mode      string    `json:"mode"`
	Timestamp time.Time `json:"timestamp"`

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
	Exhausted   bool       `json:"exhausted"`
	Attempts    int        `json:"attempts"`
	LastAttempt *time.Time `json:"last_attempt,omitempty"`
	NextAttempt *time.Time `json:"next_attempt"`
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
	State    string `json:"state"`
	Hostname string `json:"hostname"`
	Provider string `json:"provider"`
}

// ChecksInfo reports component health as "ok"/"error" only — never
// connection strings, paths, or any other implementation detail
// (AI.md PART 13 "Security: Public Info Only").
type ChecksInfo struct {
	Database  string               `json:"database"`
	Cache     string               `json:"cache"`
	Disk      string               `json:"disk"`
	Scheduler string               `json:"scheduler"`
	Tor       string               `json:"tor,omitempty"`
	I2P       string               `json:"i2p,omitempty"`
	Datasets  []DatasetHealthCheck `json:"-"`
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
func checkDatabase(db dbpkg.Handle) string {
	if db == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	if err := dbpkg.HealthProbe(ctx, db); err != nil {
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
	if s.rateLimiter != nil && s.rateLimiter.ExternalCacheDegraded() {
		return "degraded"
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
func checkTor(mgr *tor.TorManager) string {
	if mgr == nil || !mgr.IsTorEnabled() {
		return ""
	}
	if err := mgr.HealthCheck(); errors.Is(err, tor.ErrNotRunning) {
		return "ok"
	} else if err != nil {
		return "degraded"
	}
	return "ok"
}

func checkI2P(mgr *i2p.I2PManager) string {
	if mgr == nil {
		return ""
	}
	return checkI2PStatus(mgr.GetI2PStatus())
}

func checkI2PStatus(state string) string {
	switch state {
	case "disabled", "no_provider":
		return ""
	case "error":
		return "degraded"
	default:
		return "ok"
	}
}

func i2pFeatureInfo(mgr *i2p.I2PManager, expose bool) I2PInfo {
	if mgr == nil {
		return mapI2PFeatureInfo("disabled", false, false, "", i2p.I2PProviderNone)
	}
	hostname := ""
	if expose {
		hostname = mgr.EepsiteAddress()
	}
	return mapI2PFeatureInfo(mgr.GetI2PStatus(), mgr.IsI2PEnabled(), mgr.IsI2PRunning(), hostname, mgr.GetI2PProvider())
}

func mapI2PFeatureInfo(state string, enabled, running bool, hostname string, provider i2p.I2PProvider) I2PInfo {
	info := I2PInfo{State: state, Provider: string(provider)}
	if state == "disabled" || state == "no_provider" {
		info.Status, info.Provider = "disabled", "none"
		return info
	}
	info.Enabled = enabled
	info.Running = running
	info.Hostname = hostname
	switch state {
	case "running":
		info.Status = "healthy"
	case "starting", "building":
		info.Status = "starting"
	case "error":
		info.Status = "unavailable"
	default:
		info.Status = "unavailable"
	}
	return info
}

// torFeatureInfo builds the features.tor object. Per AI.md PART 13, this
// object is always present (unlike checks.tor) — Enabled/Running reflect
// actual state even when Tor is fully disabled.
func torFeatureInfo(mgr *tor.TorManager, check string, expose bool) TorInfo {
	if mgr == nil || !mgr.IsTorEnabled() {
		return TorInfo{Status: "disabled"}
	}

	info := TorInfo{Enabled: true}
	switch check {
	case "degraded":
		info.Status = "unavailable"
	case "ok":
		hostname := mgr.OnionAddress()
		if expose {
			info.Hostname = hostname
		}
		if hostname == "" {
			info.Status = "starting"
		} else {
			info.Status = "healthy"
			info.Running = true
		}
	default:
		info.Status = "starting"
	}
	return info
}

type healthProbe struct {
	name     string
	fallback string
	slot     chan struct{}
	check    func() string
}

type healthProbeResult struct {
	name   string
	status string
}

func collectHealthProbes(probes []healthProbe, timeout time.Duration) map[string]string {
	results := make(chan healthProbeResult, len(probes))
	for _, probe := range probes {
		probe := probe
		select {
		case probe.slot <- struct{}{}:
			go func() {
				status := probe.fallback
				defer func() {
					if recover() != nil {
						status = probe.fallback
					}
					<-probe.slot
					results <- healthProbeResult{name: probe.name, status: status}
				}()
				status = probe.check()
			}()
		default:
			results <- healthProbeResult{name: probe.name, status: probe.fallback}
		}
	}

	statuses := make(map[string]string, len(probes))
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for remaining := len(probes); remaining > 0; remaining-- {
		select {
		case result := <-results:
			statuses[result.name] = result.status
		case <-timer.C:
			for _, probe := range probes {
				if _, ok := statuses[probe.name]; !ok {
					statuses[probe.name] = probe.fallback
				}
			}
			return statuses
		}
	}
	return statuses
}

func collectServerHealthChecks(s *Server) ChecksInfo {
	probes := []healthProbe{
		{name: "database", fallback: "error", slot: databaseHealthProbeSlot, check: func() string { return checkDatabase(s.db) }},
		{name: "cache", fallback: "error", slot: cacheHealthProbeSlot, check: func() string { return s.checkCache() }},
		{name: "disk", fallback: "error", slot: diskHealthProbeSlot, check: func() string { return checkDisk(s.dataDir) }},
		{name: "scheduler", fallback: "error", slot: schedulerHealthProbeSlot, check: func() string { return checkScheduler(s.scheduler) }},
		{name: "tor", fallback: "degraded", slot: torHealthProbeSlot, check: func() string { return checkTor(s.tor) }},
		{name: "i2p", fallback: "degraded", slot: i2pHealthProbeSlot, check: func() string { return checkI2P(s.i2p) }},
	}
	statuses := collectHealthProbes(probes, healthCheckTimeout)
	checks := ChecksInfo{
		Database:  statuses["database"],
		Cache:     statuses["cache"],
		Disk:      statuses["disk"],
		Scheduler: statuses["scheduler"],
		Tor:       statuses["tor"],
		I2P:       statuses["i2p"],
		Datasets:  currentDatasetHealthChecks(),
	}
	if s.tor == nil || !s.tor.IsTorEnabled() {
		checks.Tor = ""
	}
	if s.i2p == nil || s.i2p.GetI2PStatus() == "disabled" || s.i2p.GetI2PStatus() == "no_provider" {
		checks.I2P = ""
	}
	return checks
}

// overallStatus derives the top-level "status" field from the component
// checks: database and disk errors are critical; non-critical component,
// overlay, and dataset failures degrade status unless a dataset is marked
// critical.
func overallStatus(checks ChecksInfo) string {
	if checks.Database == "error" || checks.Disk == "error" {
		return "unhealthy"
	}
	degraded := checks.Cache != "ok" || checks.Scheduler != "ok" || checks.Tor == "error" || checks.Tor == "degraded" || checks.I2P == "error" || checks.I2P == "degraded"
	for _, check := range checks.Datasets {
		if check.Status == "error" && check.Critical {
			return "unhealthy"
		}
		if check.Status != "ok" {
			degraded = true
		}
	}
	if degraded {
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
	case overallStatus(checks) == "degraded":
		return "degraded"
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

	checks := collectServerHealthChecks(s)

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
		Timestamp:   time.Now().UTC().Truncate(time.Second),
		Maintenance: runtimeState.Maintenance,
		Features: FeaturesInfo{
			Tor:   torFeatureInfo(s.tor, checks.Tor, s.config.Server.Tor.Expose),
			I2P:   i2pFeatureInfo(s.i2p, s.config.Server.I2P.Expose),
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
	fmt.Fprintf(&b, "features.tor: %s", resp.Features.Tor.Status)
	if resp.Features.Tor.Hostname != "" {
		fmt.Fprintf(&b, " %s", resp.Features.Tor.Hostname)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "features.i2p: %s", resp.Features.I2P.Status)
	if resp.Features.I2P.Hostname != "" {
		fmt.Fprintf(&b, " %s", resp.Features.I2P.Hostname)
	}
	fmt.Fprintf(&b, " provider=%s state=%s\n", resp.Features.I2P.Provider, resp.Features.I2P.State)
	if resp.Features.GeoIP {
		b.WriteString("features.geoip: enabled\n")
	} else {
		b.WriteString("features.geoip: disabled\n")
	}
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
	for _, check := range resp.Checks.Datasets {
		fmt.Fprintf(&b, "checks.%s: %s\n", check.Key, check.Status)
	}
	b.WriteString("\n")

	b.WriteString("# 7. Stats\n")
	fmt.Fprintf(&b, "stats.requests_total: %d\n", resp.Stats.RequestsTotal)
	fmt.Fprintf(&b, "stats.requests_24h: %d\n", resp.Stats.Requests24h)
	fmt.Fprintf(&b, "stats.active_connections: %d\n", resp.Stats.ActiveConns)

	return b.String()
}

// healthFrontendText renders the human-readable frontend text layout. API
// routes use healthResponseText, whose dot notation and raw values are a
// machine-facing contract.
func healthFrontendText(lang string, resp HealthResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s - %s\n\n", resp.Project.Name, resp.Project.Tagline)

	label := func(key string) string { return i18n.TranslateText(lang, "health."+key) }
	fmt.Fprintf(&b, "%-10s %s\n", label("status")+":", humanHealthStatus(lang, resp.Status))
	fmt.Fprintf(&b, "%-10s %s\n", label("version")+":", resp.Version)
	build := resp.Build.Commit
	if buildDate := humanHealthTimestamp(parseBuildTimestamp(resp.Build.Date)); buildDate != "" {
		build += " (" + buildDate + ")"
	}
	fmt.Fprintf(&b, "%-10s %s\n", label("build")+":", build)
	fmt.Fprintf(&b, "%-10s %s\n", label("go_version")+":", resp.GoVersion)
	fmt.Fprintf(&b, "%-10s %s\n", label("uptime")+":", commonformat.Duration(lang, parseCompactUptime(resp.Uptime)))
	fmt.Fprintf(&b, "%-10s %s\n", label("mode")+":", resp.Mode)
	checkedLabel := strings.TrimRight(label("last_checked"), ": ")
	fmt.Fprintf(&b, "%-10s %s\n\n", checkedLabel+":", humanHealthTimestamp(resp.Timestamp))

	b.WriteString(label("features") + "\n")
	fmt.Fprintf(&b, "  %-16s %s\n", label("tor")+":", humanFeatureStatus(lang, resp.Features.Tor.Status, resp.Features.Tor.Hostname))
	fmt.Fprintf(&b, "  %-16s %s\n", "I2P:", humanFeatureStatus(lang, resp.Features.I2P.Status, resp.Features.I2P.Hostname))
	featureState := "disabled"
	if resp.Features.GeoIP {
		featureState = "enabled"
	}
	fmt.Fprintf(&b, "  %-16s %s\n\n", label("geoip")+":", i18n.TranslateText(lang, "common."+featureState))

	b.WriteString(label("component_status") + "\n")
	checks := []struct{ key, status string }{
		{"database", resp.Checks.Database}, {"cache", resp.Checks.Cache}, {"disk", resp.Checks.Disk}, {"scheduler", resp.Checks.Scheduler},
	}
	if resp.Checks.Tor != "" {
		checks = append(checks, struct{ key, status string }{"tor", resp.Checks.Tor})
	}
	if resp.Checks.I2P != "" {
		checks = append(checks, struct{ key, status string }{"i2p", resp.Checks.I2P})
	}
	for _, check := range resp.Checks.Datasets {
		checks = append(checks, struct{ key, status string }{check.Key, check.Status})
	}
	for _, check := range checks {
		var checkLabel string
		switch check.key {
		case "database", "cache", "disk", "scheduler", "tor":
			checkLabel = label(check.key)
		case "i2p":
			checkLabel = "I2P"
		default:
			checkLabel = i18n.TranslateText(lang, "health.check."+check.key)
			if checkLabel == "health.check."+check.key {
				checkLabel = i18n.TranslateFormattedText(lang, "health.check.unknown", "key", check.key)
			}
		}
		fmt.Fprintf(&b, "  %-24s %s\n", checkLabel+":", humanCheckStatus(lang, check.status))
	}

	b.WriteString("\n" + label("server_statistics") + "\n")
	fmt.Fprintf(&b, "  %-24s %s\n", label("total_requests")+":", commonformat.Count(lang, resp.Stats.RequestsTotal))
	fmt.Fprintf(&b, "  %-24s %s\n", label("requests_24h")+":", commonformat.Count(lang, resp.Stats.Requests24h))
	fmt.Fprintf(&b, "  %-24s %s\n", label("active_connections")+":", commonformat.Count(lang, int64(resp.Stats.ActiveConns)))
	return b.String()
}

func humanFeatureStatus(lang, status, hostname string) string {
	if status == "disabled" {
		return i18n.TranslateText(lang, "common.disabled")
	}
	text := humanHealthStatus(lang, status)
	if hostname != "" {
		text += " - " + hostname
	}
	return text
}

func humanCheckStatus(lang, status string) string {
	return humanHealthStatus(lang, status)
}

func humanHealthStatus(lang, status string) string {
	switch status {
	case "healthy", "unhealthy", "degraded", "restart_required", "maintenance", "shutting_down":
		return i18n.TranslateText(lang, "health."+status)
	case "ok":
		return i18n.TranslateText(lang, "health.ok")
	case "starting":
		return i18n.TranslateText(lang, "common.loading")
	case "unavailable", "error":
		return i18n.TranslateText(lang, "common.error")
	case "disabled":
		return i18n.TranslateText(lang, "common.disabled")
	default:
		return status
	}
}

func humanHealthTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("January 02, 2006 at 15:04:05 MST")
}

func parseBuildTimestamp(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed
	}
	return time.Time{}
}

func parseCompactUptime(value string) time.Duration {
	var days, hours, minutes, seconds int64
	for _, part := range strings.Fields(value) {
		if len(part) < 2 {
			continue
		}
		amount, err := strconv.ParseInt(part[:len(part)-1], 10, 64)
		if err != nil {
			continue
		}
		switch part[len(part)-1] {
		case 'd':
			days = amount
		case 'h':
			hours = amount
		case 'm':
			minutes = amount
		case 's':
			seconds = amount
		}
	}
	return time.Duration(days)*24*time.Hour + time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
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
// alias), /api/healthz, /api/{api_version}/server/healthz, and its canonical
// .txt suffix — all routes use this single handler (AI.md PART 13: "No
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
		maintenanceSince, maintenanceLastAttempt, maintenanceNextAttempt, maintenanceAttempts := "", "", "", ""
		if resp.Maintenance != nil {
			maintenanceSince = humanHealthTimestamp(resp.Maintenance.Since)
			maintenanceAttempts = commonformat.Count(lang, int64(resp.Maintenance.SelfHealing.Attempts))
			if resp.Maintenance.SelfHealing.LastAttempt != nil {
				maintenanceLastAttempt = humanHealthTimestamp(*resp.Maintenance.SelfHealing.LastAttempt)
			}
			if resp.Maintenance.SelfHealing.NextAttempt != nil {
				maintenanceNextAttempt = humanHealthTimestamp(*resp.Maintenance.SelfHealing.NextAttempt)
			}
		}
		s.renderTemplate(w, r, "server_healthz", map[string]interface{}{
			"Title":                  fmt.Sprintf("%s - %s", resp.Project.Name, i18n.TranslateText(lang, "health.title")),
			"Health":                 resp,
			"Requests":               commonformat.Count(lang, resp.Stats.RequestsTotal),
			"Req24h":                 commonformat.Count(lang, resp.Stats.Requests24h),
			"ActiveConnections":      commonformat.Count(lang, int64(resp.Stats.ActiveConns)),
			"Uptime":                 commonformat.Duration(lang, parseCompactUptime(resp.Uptime)),
			"Timestamp":              humanHealthTimestamp(resp.Timestamp),
			"BuildTimestamp":         humanHealthTimestamp(parseBuildTimestamp(resp.Build.Date)),
			"MaintenanceSince":       maintenanceSince,
			"MaintenanceLastAttempt": maintenanceLastAttempt,
			"MaintenanceNextAttempt": maintenanceNextAttempt,
			"MaintenanceAttempts":    maintenanceAttempts,
		})
	case wantsHealthzText(r):
		body := healthResponseText(resp)
		if !apiRoute {
			body = healthFrontendText(i18n.FromContext(r.Context()), resp)
		}
		s.respondText(w, statusCode, body)
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
