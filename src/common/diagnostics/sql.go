package diagnostics

import (
	"log/slog"
	"sync/atomic"
	"time"
)

var queryLoggingEnabled atomic.Bool
var debugModeEnabled atomic.Bool

// SetQueryLoggingEnabled updates the configuration gate for SQL diagnostics.
func SetQueryLoggingEnabled(enabled bool) {
	queryLoggingEnabled.Store(enabled)
}

// SetDebugModeEnabled tracks the independent runtime debug flag without
// coupling diagnostics to application configuration packages.
func SetDebugModeEnabled(enabled bool) {
	debugModeEnabled.Store(enabled)
}

// LogQuery records SQL text and timing without exposing bound argument values.
func LogQuery(query string, argumentCount int, started time.Time, rows int64, err error) {
	if !queryLoggingEnabled.Load() || !debugModeEnabled.Load() {
		return
	}
	attrs := []any{
		"query", query,
		"args_count", argumentCount,
		"duration_ms", time.Since(started).Milliseconds(),
		"rows", rows,
	}
	if err != nil {
		attrs = append(attrs, "failed", true)
		slog.Debug("db query failed", attrs...)
		return
	}
	slog.Debug("db query", attrs...)
}
