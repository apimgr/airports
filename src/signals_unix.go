//go:build !windows

package main

import (
	"os"
	"syscall"
)

// containerStopSignal is SIGRTMIN+3, the STOPSIGNAL set in docker/Dockerfile
// (AI.md PART 26). Go's syscall package does not export SIGRTMIN, so it is
// expressed numerically: SIGRTMIN is 34 on Linux/glibc, so SIGRTMIN+3 = 37.
// This signal is only ever delivered by a Linux container runtime.
const containerStopSignal = syscall.Signal(37)

// shutdownSignals lists the OS signals that trigger a graceful shutdown on
// Unix per AI.md PART 8: SIGINT, SIGTERM, SIGQUIT, and SIGRTMIN+3 (the
// container STOPSIGNAL sent by Docker/Podman on `stop`).
func shutdownSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		containerStopSignal,
	}
}

// logReopenSignals lists the signals that ask the process to reopen its log
// files (SIGUSR1) after an external log-rotation tool moved them, per
// AI.md PART 8.
func logReopenSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR1}
}

// statusDumpSignals lists the signals that dump a runtime status snapshot to
// the log (SIGUSR2), per AI.md PART 8.
func statusDumpSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR2}
}
