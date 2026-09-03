package tor

import "errors"

// ErrNotEnabled is returned by HealthCheck and other Manager methods when
// Tor hidden service support is not enabled and not configured. Callers
// (such as the scheduler) must treat this as a skip condition, not a
// failure.
var ErrNotEnabled = errors.New("tor: not enabled")

// ErrBinaryNotFound is returned when Tor is enabled (explicitly or via
// auto-detect) but no usable tor executable could be located on the
// system.
var ErrBinaryNotFound = errors.New("tor: binary not found")

// ErrNotRunning is returned when an operation requires a running Tor
// process (such as HealthCheck or OnionAddress) but Start has not been
// called, Start failed, or the process has already stopped.
var ErrNotRunning = errors.New("tor: process not running")
