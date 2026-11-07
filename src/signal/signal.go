// Package signal provides platform-aware process signal groups.
package signal

import "os"

// ShutdownSignals returns the signals that request graceful shutdown.
func ShutdownSignals() []os.Signal {
	return shutdownSignals()
}

// LogReopenSignals returns signals that ask managed log files to reopen.
func LogReopenSignals() []os.Signal {
	return logReopenSignals()
}

// StatusDumpSignals returns signals that request a runtime status dump.
func StatusDumpSignals() []os.Signal {
	return statusDumpSignals()
}

// IgnoreConfigReloadSignal disables the legacy reload signal because the
// configuration file watcher owns reloads.
func IgnoreConfigReloadSignal() {
	ignoreConfigReloadSignal()
}
