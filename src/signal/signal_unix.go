//go:build !windows

package signal

import (
	"os"
	"os/signal"
	"syscall"
)

// containerStopSignal is SIGRTMIN+3, the STOPSIGNAL configured for the
// runtime container. It is delivered only by Linux container runtimes.
const containerStopSignal = syscall.Signal(37)

func shutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, containerStopSignal}
}

func logReopenSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR1}
}

func statusDumpSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR2}
}

func ignoreConfigReloadSignal() {
	signal.Ignore(syscall.SIGHUP)
}
