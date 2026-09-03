package mode

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/apimgr/airports/src/config"
)

// Mode represents the application execution mode
type Mode string

const (
	// Production mode - optimized for performance and security
	Production Mode = "production"
	// Development mode - optimized for debugging and development
	Development Mode = "development"
)

var (
	// currentMode stores the active application mode
	currentMode Mode = Production
	// mu protects concurrent access to currentMode
	mu sync.RWMutex

	// debugEnabled stores whether debug mode is active
	debugEnabled bool
	// debugMu protects concurrent access to debugEnabled
	debugMu sync.RWMutex
)

// GetAppMode returns the current application mode
func GetAppMode() Mode {
	mu.RLock()
	defer mu.RUnlock()
	return currentMode
}

// SetAppMode sets the application mode
// Valid values: "production", "prod", "development", "dev", "devel"
// Use SetFromValue instead when the raw --mode/MODE value may be "debug" —
// SetAppMode alone does not apply the debug-alias side effect.
func SetAppMode(mode string) error {
	parsed, err := ParseMode(mode)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	currentMode = parsed
	return nil
}

// SetFromValue sets the application mode from a raw --mode/MODE value,
// applying the "debug" alias: development mode + debug enabled.
// Per AI.md PART 6, an explicit --debug flag or DEBUG env var still wins
// over the alias's debug-on side effect — callers apply those separately
// after calling SetFromValue.
func SetFromValue(value string) error {
	if err := SetAppMode(value); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(value), "debug") {
		SetDebug(true)
	}
	return nil
}

// SetDebug enables or disables debug mode and toggles the runtime profiling
// rates that back the /debug/pprof/block and /debug/pprof/mutex endpoints.
// Per AI.md PART 6, block/mutex profiling is driven by the debug flag, not
// by the app mode.
func SetDebug(enabled bool) {
	debugMu.Lock()
	debugEnabled = enabled
	debugMu.Unlock()
	updateProfilingSettings(enabled)
}

// updateProfilingSettings enables or disables Go's block/mutex profilers.
// A rate of 1 samples every blocking/contention event (debug mode); 0
// disables sampling entirely (default, non-debug behavior).
func updateProfilingSettings(enabled bool) {
	if enabled {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		return
	}
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(0)
}

// IsDebug returns true if debug mode is currently enabled
func IsDebug() bool {
	debugMu.RLock()
	defer debugMu.RUnlock()
	return debugEnabled
}

// ParseMode parses a mode string into a Mode constant
// Accepts: "dev", "devel", "development", "prod", "production", "debug"
// (case-insensitive). "debug" resolves to Development — see SetFromValue
// for applying its debug-on side effect.
func ParseMode(s string) (Mode, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))

	switch normalized {
	case "development", "dev", "devel", "debug":
		return Development, nil
	case "production", "prod":
		return Production, nil
	default:
		return "", fmt.Errorf("invalid mode: %q (expected: production, prod, development, dev, devel, or debug)", s)
	}
}

// IsDevelopment returns true if the current mode is Development
func IsDevelopment() bool {
	return GetAppMode() == Development
}

// IsProduction returns true if the current mode is Production
func IsProduction() bool {
	return GetAppMode() == Production
}

// Initialize sets the mode based on priority order:
// 1. CLI flag (passed as parameter)
// 2. MODE environment variable
// 3. Default: production
// A raw value of "debug" (either source) expands to Development mode and
// also enables debug — see SetFromValue. Callers should apply
// InitializeDebug afterward so an explicit --debug/DEBUG=true always wins
// over the alias.
func Initialize(cliMode string) error {
	// Priority 1: CLI flag
	if cliMode != "" {
		return SetFromValue(cliMode)
	}

	// Priority 2: Environment variable
	if envMode := os.Getenv("MODE"); envMode != "" {
		return SetFromValue(envMode)
	}

	// Priority 3: Default (already set to Production)
	return nil
}

// InitializeDebug resolves debug mode using the AI.md PART 6 priority order:
//  1. --debug CLI flag (cliDebug, highest priority)
//  2. DEBUG environment variable (truthy values)
//  3. --mode debug / MODE=debug alias (already applied by Initialize via
//     SetFromValue before this is called)
//  4. Default: false
//
// Call Initialize first so the MODE=debug alias's debug-on side effect is in
// place, then call InitializeDebug so an explicit --debug flag or DEBUG env
// var can override it in either direction (including turning it back off via
// MODE=debug DEBUG=false).
func InitializeDebug(cliDebug bool) {
	// Priority 1: CLI flag
	if cliDebug {
		SetDebug(true)
		return
	}

	// Priority 2: Environment variable (must distinguish unset from
	// explicitly false, so LookupEnv rather than Getenv)
	if envDebug, ok := os.LookupEnv("DEBUG"); ok {
		SetDebug(config.IsTruthy(envDebug))
		return
	}

	// Priority 3: MODE=debug alias already applied by Initialize/SetFromValue;
	// leave debugEnabled as-is.
	// Priority 4: Default false (already the zero value) when nothing set it.
}

// GetErrorDetail returns error details based on the current mode
// In development mode: returns full error details with stack traces
// In production mode: returns generic error message without internal details
func GetErrorDetail(err error) string {
	if err == nil {
		return ""
	}

	if IsDevelopment() {
		// Development mode: return full error details
		return err.Error()
	}

	// Production mode: return generic error message
	return "An internal error occurred. Please contact support if the problem persists."
}

// ShouldShowDebugEndpoints returns true if debug endpoints should be enabled.
// Debug endpoints include /debug/pprof/*, /debug/vars, /debug/config,
// /debug/routes, /debug/cache, /debug/db, and /debug/scheduler. Gated on the
// independent debug flag (per AI.md PART 6), never on the app mode — so
// Production+Debug is a reachable state.
func ShouldShowDebugEndpoints() bool {
	return IsDebug()
}

// CacheHeaders represents HTTP cache control headers
type CacheHeaders struct {
	CacheControl string
	Pragma       string
	Expires      string
}

// GetCacheHeaders returns appropriate cache headers based on the current mode
// Development mode: no-cache headers to prevent caching
// Production mode: aggressive caching headers for static files
func GetCacheHeaders() CacheHeaders {
	if IsDevelopment() {
		// Development mode: disable caching
		return CacheHeaders{
			CacheControl: "no-cache, no-store, must-revalidate",
			Pragma:       "no-cache",
			Expires:      "0",
		}
	}

	// Production mode: enable caching (1 year for static assets)
	return CacheHeaders{
		CacheControl: "public, max-age=31536000, immutable",
		Pragma:       "",
		Expires:      "",
	}
}

// getLogLevel returns the recommended log level for the current mode
// (AI.md PART 6: "debug" in development, "info" in production). Unexported:
// the codebase logs via the stdlib "log" package, which has no concept of
// levels, so there is no call site to wire this into until a leveled logger
// is introduced.
func getLogLevel() string {
	if IsDevelopment() {
		return "debug"
	}
	return "info"
}

// ShouldCacheTemplates returns true if templates should be cached
func ShouldCacheTemplates() bool {
	return IsProduction()
}

// ShouldEnableAutoReload returns true if auto-reload should be enabled
func ShouldEnableAutoReload() bool {
	return IsDevelopment()
}

// ShouldEnableProfiling returns true if profiling endpoints should be enabled.
// Gated on the independent debug flag, not the app mode — see
// ShouldShowDebugEndpoints.
func ShouldEnableProfiling() bool {
	return IsDebug()
}

// GetPanicRecoveryMode returns the panic recovery behavior for the current mode
// Returns "verbose" for development, "graceful" for production
func GetPanicRecoveryMode() string {
	if IsDevelopment() {
		return "verbose"
	}
	return "graceful"
}

// String returns the string representation of the Mode
func (m Mode) String() string {
	return string(m)
}

// Validate returns an error if the mode is not valid
func (m Mode) Validate() error {
	switch m {
	case Production, Development:
		return nil
	default:
		return errors.New("invalid mode")
	}
}
