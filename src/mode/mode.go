package mode

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

const appName = "airports"

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
)

// Get returns the current application mode
func Get() Mode {
	mu.RLock()
	defer mu.RUnlock()
	return currentMode
}

// Set sets the application mode
// Valid values: "production", "prod", "development", "dev"
func Set(mode string) error {
	parsed, err := ParseMode(mode)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	currentMode = parsed
	return nil
}

// ParseMode parses a mode string into a Mode constant
// Accepts: "dev", "development", "prod", "production" (case-insensitive)
func ParseMode(s string) (Mode, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))

	switch normalized {
	case "development", "dev":
		return Development, nil
	case "production", "prod":
		return Production, nil
	default:
		return "", fmt.Errorf("invalid mode: %q (expected: production, prod, development, or dev)", s)
	}
}

// IsDevelopment returns true if the current mode is Development
func IsDevelopment() bool {
	return Get() == Development
}

// IsProduction returns true if the current mode is Production
func IsProduction() bool {
	return Get() == Production
}

// Initialize sets the mode based on priority order:
// 1. CLI flag (passed as parameter)
// 2. MODE environment variable
// 3. Default: production
func Initialize(cliMode string) error {
	// Priority 1: CLI flag
	if cliMode != "" {
		return Set(cliMode)
	}

	// Priority 2: Environment variable
	if envMode := os.Getenv("MODE"); envMode != "" {
		return Set(envMode)
	}

	// Priority 3: Default (already set to Production)
	return nil
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

// ShouldShowDebugEndpoints returns true if debug endpoints should be enabled
// Debug endpoints include /debug/pprof/* and /debug/vars
func ShouldShowDebugEndpoints() bool {
	return IsDevelopment()
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

// GetLogLevel returns the recommended log level for the current mode
func GetLogLevel() string {
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

// ShouldEnableProfiling returns true if profiling endpoints should be enabled
func ShouldEnableProfiling() bool {
	return IsDevelopment()
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
