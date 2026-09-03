//go:build windows

package main

import "os"

// shutdownSignals on Windows is os.Interrupt only — Windows has no SIGQUIT,
// SIGRTMIN, or SIGUSR semantics (AI.md PART 8 "Windows" column).
func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// logReopenSignals is empty on Windows — SIGUSR1 does not exist.
func logReopenSignals() []os.Signal {
	return nil
}

// statusDumpSignals is empty on Windows — SIGUSR2 does not exist.
func statusDumpSignals() []os.Signal {
	return nil
}
