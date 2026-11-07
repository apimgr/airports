package diagnostics

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLogQueryRequiresConfigAndDebugGates(t *testing.T) {
	previousLogger := slog.Default()
	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		SetQueryLoggingEnabled(false)
		SetDebugModeEnabled(false)
		slog.SetDefault(previousLogger)
	})

	SetQueryLoggingEnabled(true)
	SetDebugModeEnabled(false)
	LogQuery("SELECT * FROM airports WHERE name = ?", 1, time.Now(), 1, nil)
	if output.Len() != 0 {
		t.Fatalf("query logged while debug mode was disabled: %s", output.String())
	}
	SetDebugModeEnabled(true)
	LogQuery("SELECT * FROM airports WHERE name = ?", 1, time.Now(), 1, nil)
	if !strings.Contains(output.String(), "args_count=1") || strings.Contains(output.String(), "bound-value") {
		t.Fatalf("unexpected query log: %s", output.String())
	}
}
