package mode

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/apimgr/airports/src/common/diagnostics"
	"github.com/apimgr/airports/src/config"
)

// AppMode identifies the application's runtime behavior.
type AppMode int

const (
	Production AppMode = iota
	Development
	Debug
)

var (
	// currentMode stores the active application mode
	currentMode AppMode = Production
	// mu protects concurrent access to currentMode
	mu sync.RWMutex

	// debugEnabled stores whether debug mode is active
	debugEnabled bool
	// debugMu protects concurrent access to debugEnabled
	debugMu sync.RWMutex
)

// GetAppMode returns the current application mode
func GetAppMode() AppMode {
	mu.RLock()
	defer mu.RUnlock()
	return currentMode
}

// SetAppMode sets the application mode
// Valid values: "production", "prod", "development", "dev", "devel", "debug".
// Use SetAppModeFromValue instead when the raw --mode/MODE value may be "debug" —
// SetAppMode alone does not apply the debug-alias side effect.
func SetAppMode(mode string) error {
	parsed, err := ParseAppMode(mode)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	currentMode = parsed
	if parsed == Debug {
		SetDebugEnabled(true)
	}
	return nil
}

// SetAppModeFromValue sets the application mode from a raw --mode/MODE value,
// applying the "debug" alias: debug mode + diagnostics enabled.
// Per AI.md PART 6, an explicit --debug flag or DEBUG env var still wins
// over the alias's debug-on side effect — callers apply those separately
// after calling SetAppModeFromValue.
func SetAppModeFromValue(value string) error {
	if err := SetAppMode(value); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(value), "debug") {
		SetDebugEnabled(true)
	}
	return nil
}

// SetDebugEnabled enables or disables debug mode and toggles the runtime profiling
// rates that back the /debug/pprof/block and /debug/pprof/mutex endpoints.
// Per AI.md PART 6, block/mutex profiling is driven by the debug flag, not
// by the app mode.
func SetDebugEnabled(enabled bool) {
	debugMu.Lock()
	debugEnabled = enabled
	debugMu.Unlock()
	diagnostics.SetDebugModeEnabled(enabled)
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

// IsDebugEnabled returns true if debug mode is currently enabled
func IsDebugEnabled() bool {
	debugMu.RLock()
	defer debugMu.RUnlock()
	return debugEnabled
}

// ParseAppMode parses a mode string into a AppMode constant
// Accepts: "dev", "devel", "development", "prod", "production", "debug"
// (case-insensitive). SetAppModeFromValue also enables diagnostics for debug.
func ParseAppMode(s string) (AppMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))

	switch normalized {
	case "development", "dev", "devel":
		return Development, nil
	case "debug":
		return Debug, nil
	case "production", "prod":
		return Production, nil
	default:
		return Production, fmt.Errorf("invalid mode: %q (expected: production, prod, development, dev, devel, or debug)", s)
	}
}

// IsAppModeDev returns true if the current mode is Development
func IsAppModeDev() bool {
	return GetAppMode() == Development
}

// IsAppModeProd returns true if the current mode is Production
func IsAppModeProd() bool {
	return GetAppMode() == Production
}

// GetAppModeString returns the current mode with the diagnostics suffix.
func GetAppModeString() string {
	name := GetAppMode().String()
	if IsDebugEnabled() {
		name += " [debugging]"
	}
	return name
}

// Initialize sets the mode based on priority order:
// 1. CLI flag (passed as parameter)
// 2. MODE environment variable
// 3. Default: production
// A raw value of "debug" (either source) selects Debug mode and enables
// diagnostics — see SetAppModeFromValue. Callers should apply
// InitializeDebug afterward so an explicit --debug/DEBUG=true always wins
// over the alias.
func Initialize(cliMode string) error {
	// Priority 1: CLI flag
	if cliMode != "" {
		return SetAppModeFromValue(cliMode)
	}

	// Priority 2: Environment variable
	if envMode := os.Getenv("MODE"); envMode != "" {
		return SetAppModeFromValue(envMode)
	}

	// Priority 3: Default (already set to Production)
	return nil
}

// InitializeDebug resolves debug mode using the AI.md PART 6 priority order:
//  1. --debug CLI flag (cliDebug, highest priority)
//  2. DEBUG environment variable (truthy values)
//  3. --mode debug / MODE=debug alias (already applied by Initialize via
//     SetAppModeFromValue before this is called)
//  4. Default: false
//
// Call Initialize first so the MODE=debug alias's debug-on side effect is in
// place, then call InitializeDebug so an explicit --debug flag or DEBUG env
// var can override it in either direction (including turning it back off via
// MODE=debug DEBUG=false).
func InitializeDebug(cliDebug bool) {
	// Priority 1: CLI flag
	if cliDebug {
		SetDebugEnabled(true)
		return
	}

	// Priority 2: Environment variable (must distinguish unset from
	// explicitly false, so LookupEnv rather than Getenv)
	if envDebug, ok := os.LookupEnv("DEBUG"); ok {
		SetDebugEnabled(config.IsTruthy(envDebug))
		return
	}

	// Priority 3: MODE=debug alias already applied by Initialize/SetAppModeFromValue;
	// leave debugEnabled as-is.
	// Priority 4: Default false (already the zero value) when nothing set it.
}

// ShouldShowDebugEndpoints returns true if debug endpoints should be enabled.
// Debug endpoints include /debug/pprof/*, /debug/vars, /debug/config,
// /debug/routes, /debug/cache, /debug/db, and /debug/scheduler. Gated on the
// independent debug flag (per AI.md PART 6), never on the app mode — so
// Production+Debug is a reachable state.
func ShouldShowDebugEndpoints() bool {
	return IsDebugEnabled()
}

// ShouldEnableProfiling returns true if profiling endpoints should be enabled.
// Gated on the independent debug flag, not the app mode — see
// ShouldShowDebugEndpoints.
func ShouldEnableProfiling() bool {
	return IsDebugEnabled()
}

// String returns the stable display name for an application mode.
func (appMode AppMode) String() string {
	switch appMode {
	case Development:
		return "development"
	case Debug:
		return "debug"
	default:
		return "production"
	}
}

// ValidateAppMode checks that the enum contains a supported value.
func (appMode AppMode) ValidateAppMode() error {
	switch appMode {
	case Production, Development, Debug:
		return nil
	default:
		return errors.New("invalid mode")
	}
}
