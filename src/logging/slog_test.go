package logging

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlogHandlerRoutesFormatsAndRedactsSensitiveAttributes(t *testing.T) {
	base := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "airports-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cfg := DefaultLogManagerConfig()
	cfg.Server.Format = "json"
	cfg.Error.Format = "json"
	cfg.App.Format = "logfmt"
	cfg.Debug.Enabled = true
	manager := NewLogManager(dir, cfg)
	t.Cleanup(func() { _ = manager.Close() })
	logger := slog.New(NewSlogHandler(manager, slog.LevelWarn))
	logger.Info("filtered")
	logger.Warn("contact submitted", "token", "secret-value", "request_id", "req-123")
	logger.Error("delivery failed", "error", "smtp timeout")

	serverData, err := os.ReadFile(manager.Server().Path())
	if err != nil {
		t.Fatal(err)
	}
	serverLines := strings.Split(strings.TrimSpace(string(serverData)), "\n")
	if len(serverLines) != 2 || strings.Contains(string(serverData), "filtered") {
		t.Fatalf("server log lines = %q", serverData)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(serverLines[0]), &record); err != nil {
		t.Fatalf("server log is not JSON: %v (%s)", err, serverLines[0])
	}
	if record["token"] != "xxxxx" || record["request_id"] != "req-123" {
		t.Fatalf("server log fields = %#v", record)
	}
	appData, err := os.ReadFile(manager.App().Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appData), `level=WARN`) || !strings.Contains(string(appData), `token=xxxxx`) {
		t.Fatalf("app log = %q", appData)
	}
	errorData, err := os.ReadFile(manager.Error().Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(errorData), `"msg":"delivery failed"`) {
		t.Fatalf("error log = %q", errorData)
	}
	if err := NewSlogHandler(manager, slog.LevelError).Handle(context.Background(), slog.Record{}); err != nil {
		t.Fatalf("empty record handle: %v", err)
	}
}

func TestParseSlogLevel(t *testing.T) {
	tests := []struct {
		value string
		want  slog.Level
	}{
		{value: "debug", want: slog.LevelDebug},
		{value: "info", want: slog.LevelInfo},
		{value: "WARN", want: slog.LevelWarn},
		{value: "error", want: slog.LevelError},
		{value: "invalid", want: slog.LevelInfo},
	}
	for _, test := range tests {
		if got := ParseSlogLevel(test.value); got != test.want {
			t.Errorf("ParseSlogLevel(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}
