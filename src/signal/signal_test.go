package signal

import (
	"runtime"
	"testing"
)

func TestSignalGroupsMatchPlatformSupport(t *testing.T) {
	if len(ShutdownSignals()) == 0 {
		t.Fatal("no graceful shutdown signals are configured")
	}
	if runtime.GOOS == "windows" {
		if len(LogReopenSignals()) != 0 || len(StatusDumpSignals()) != 0 {
			t.Fatal("unsupported Windows signals are configured")
		}
		return
	}
	if len(LogReopenSignals()) == 0 || len(StatusDumpSignals()) == 0 {
		t.Fatal("Unix log reopen and status dump signals are missing")
	}
}
