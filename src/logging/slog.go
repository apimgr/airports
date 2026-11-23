package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type slogFileHandler struct {
	manager *LogManager
	level   slog.Level
	attrs   []slog.Attr
	group   []string
}

// ParseSlogLevel converts the configured minimum level to its slog value.
// Unknown values use info as the safe startup default.
func ParseSlogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewSlogHandler writes structured diagnostics to the configured server,
// error, app, and debug logs while enforcing the configured minimum level.
func NewSlogHandler(manager *LogManager, minimum slog.Level) slog.Handler {
	return &slogFileHandler{manager: manager, level: minimum}
}

func (h *slogFileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h != nil && h.manager != nil && level >= h.level
}

func (h *slogFileHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.Enabled(context.Background(), record.Level) {
		return nil
	}
	fields := map[string]any{"time": record.Time.Format(time.RFC3339), "level": strings.ToUpper(record.Level.String()), "msg": safeLogText(record.Message)}
	for _, attr := range h.attrs {
		addSlogAttr(fields, h.group, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		addSlogAttr(fields, h.group, attr)
		return true
	})
	if err := h.manager.Server().Write(formatSlogLineWithFormat(fields, h.manager.cfg.Server.Format)); err != nil {
		return err
	}
	if record.Level >= slog.LevelError {
		return h.manager.Error().Write(formatSlogLineWithFormat(fields, h.manager.cfg.Error.Format))
	}
	if record.Level >= slog.LevelInfo {
		return h.manager.App().Write(formatSlogLineWithFormat(fields, h.manager.cfg.App.Format))
	}
	return h.manager.Debug().Write(formatSlogLineWithFormat(fields, h.manager.cfg.Debug.Format))
}

func (h *slogFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *slogFileHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.group = append(append([]string(nil), h.group...), name)
	return &clone
}

func addSlogAttr(fields map[string]any, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			addSlogAttr(fields, append(append([]string(nil), groups...), attr.Key), child)
		}
		return
	}
	key := strings.Join(append(append([]string(nil), groups...), attr.Key), ".")
	if key == "" {
		return
	}
	if sensitiveSlogKey(key) {
		fields[key] = "xxxxx"
		return
	}
	fields[key] = safeLogText(fmt.Sprint(attr.Value.Any()))
}

func sensitiveSlogKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
	for _, part := range []string{"password", "passwd", "secret", "token", "credential", "authorization", "cookie", "private_key", "encryption_key", "security_id"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func formatSlogLineWithFormat(fields map[string]any, format string) string {
	if format == "json" {
		encoded, err := json.Marshal(fields)
		if err == nil {
			return string(encoded)
		}
	}
	if format == "logfmt" {
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		keys = sortLogFields(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+logfmtValue(fmt.Sprint(fields[key])))
		}
		return strings.Join(parts, " ")
	}
	message := safeLogText(fmt.Sprint(fields["msg"]))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key != "msg" {
			keys = append(keys, key)
		}
	}
	keys = sortLogFields(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+strconv.Quote(fmt.Sprint(fields[key])))
	}
	return fmt.Sprintf("%s [%s] %s %s", fields["time"], fields["level"], message, strings.Join(parts, " "))
}

func sortLogFields(keys []string) []string {
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func logfmtValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t=\"\\") {
		return strconv.Quote(value)
	}
	return value
}

func safeLogText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r > 0x7e {
			return ' '
		}
		return r
	}, value)
}
