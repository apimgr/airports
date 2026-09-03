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
	"time"

	"github.com/apimgr/airports/src/backup"
	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/common/i18n"
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

	// 2. Overall status: "healthy", "unhealthy", "degraded"
	Status string `json:"status"`
	// PendingRestart is true when a config change requires a restart to
	// take effect. This project has no such settings today (PART 5: all
	// settings live-reload except listen address/port and DB driver,
	// neither of which healthz can detect at runtime), so this always
	// omits per its `omitempty` tag.
	PendingRestart bool `json:"pending_restart,omitempty"`
	// RestartReason lists the specific settings that changed and require
	// a restart; always omitted alongside PendingRestart today.
	RestartReason []string `json:"restart_reason,omitempty"`

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
	GeoIP bool    `json:"geoip"`
}

// TorInfo reflects the dedicated Tor manager's live state (AI.md PART 31).
type TorInfo struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
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
	if checks.Disk == "error" || checks.Tor == "error" {
		return "degraded"
	}
	return "healthy"
}

// buildHealthResponse assembles the full HealthResponse from the
// server's live dependencies. Every data source is documented inline
// against the AI.md PART 13 "Data Sources" table.
func buildHealthResponse(s *Server) HealthResponse {
	branding := s.config.Server.Branding

	checks := ChecksInfo{
		Database:  checkDatabase(s.db),
		Cache:     checkCache(s.cache),
		Disk:      checkDisk(s.dataDir),
		Scheduler: checkScheduler(s.scheduler),
		Tor:       checkTor(s.tor),
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

	return HealthResponse{
		Project: ProjectInfo{
			Name:        branding.Title,
			Tagline:     branding.Tagline,
			Description: branding.Description,
		},
		Status:    overallStatus(checks),
		Version:   Version,
		GoVersion: runtime.Version(),
		Build: BuildInfo{
			Commit: CommitID,
			Date:   BuildDate,
		},
		Uptime:    uptime,
		Mode:      s.config.Server.Mode,
		Timestamp: time.Now().UTC(),
		Features: FeaturesInfo{
			Tor:   torFeatureInfo(s.tor),
			GeoIP: s.config.Server.GeoIP.Enabled,
		},
		Checks: checks,
		Stats: StatsInfo{
			RequestsTotal: requestsTotal,
			Requests24h:   requests24h,
			ActiveConns:   activeConns,
		},
	}
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
	fmt.Fprintf(&b, "status: %s\n\n", resp.Status)

	b.WriteString("# 3. Version & Build (PART 7)\n")
	fmt.Fprintf(&b, "version: %s\n", resp.Version)
	fmt.Fprintf(&b, "go_version: %s\n", resp.GoVersion)
	fmt.Fprintf(&b, "build.commit: %s\n", resp.Build.Commit)
	fmt.Fprintf(&b, "build.date: %s\n\n", resp.Build.Date)

	b.WriteString("# 4. Runtime (PART 6)\n")
	fmt.Fprintf(&b, "uptime: %s\n", resp.Uptime)
	fmt.Fprintf(&b, "mode: %s\n", resp.Mode)
	fmt.Fprintf(&b, "timestamp: %s\n\n", resp.Timestamp.Format(time.RFC3339))

	b.WriteString("# 5. Features - NON-NEGOTIABLE only (show actual status)\n")
	fmt.Fprintf(&b, "features.tor.enabled: %t\n", resp.Features.Tor.Enabled)
	fmt.Fprintf(&b, "features.tor.running: %t\n", resp.Features.Tor.Running)
	fmt.Fprintf(&b, "features.tor.status: %s\n", resp.Features.Tor.Status)
	fmt.Fprintf(&b, "features.tor.hostname: %s\n", resp.Features.Tor.Hostname)
	fmt.Fprintf(&b, "features.geoip: %t\n", resp.Features.GeoIP)
	b.WriteString("# 6. Checks\n")
	fmt.Fprintf(&b, "checks.database: %s\n", resp.Checks.Database)
	fmt.Fprintf(&b, "checks.cache: %s\n", resp.Checks.Cache)
	fmt.Fprintf(&b, "checks.disk: %s\n", resp.Checks.Disk)
	fmt.Fprintf(&b, "checks.scheduler: %s\n", resp.Checks.Scheduler)
	if resp.Checks.Tor != "" {
		fmt.Fprintf(&b, "checks.tor: %s\n", resp.Checks.Tor)
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
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/plain") {
		return true
	}
	if strings.Contains(accept, "text/html") || strings.Contains(accept, "application/json") {
		return false
	}
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	for _, tool := range []string{"curl", "wget", "httpie"} {
		if strings.Contains(ua, tool) {
			return true
		}
	}
	return false
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
	accept := r.Header.Get("Accept")

	switch {
	case strings.Contains(accept, "text/html"):
		lang := i18n.FromContext(r.Context())
		s.renderTemplate(w, r, "server_healthz", map[string]interface{}{
			"Title":    fmt.Sprintf("%s - %s", resp.Project.Name, i18n.T(lang, "health.title")),
			"Health":   resp,
			"Requests": formatWithCommas(resp.Stats.RequestsTotal),
			"Req24h":   formatWithCommas(resp.Stats.Requests24h),
		})
	case wantsHealthzText(r):
		s.respondText(w, http.StatusOK, healthResponseText(resp))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
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
