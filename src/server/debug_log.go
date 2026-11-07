package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
)

// debugMiddleware records request metadata and, when explicitly enabled,
// bounded request bodies. It is installed only when the debug flag is active.
func (s *Server) debugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mode.IsDebugEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		var body []byte
		s.configMu.RLock()
		var debugConfig config.DebugConfig
		if s.config != nil {
			debugConfig = s.config.Server.Debug
		}
		s.configMu.RUnlock()
		if debugConfig.LogBodies && r.Body != nil {
			limit := debugBodyLogLimit(debugConfig.MaxBodyLogSize)
			body, _ = io.ReadAll(io.LimitReader(r.Body, limit+1))
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
		}

		wrapped := &debugResponseWriter{ResponseWriter: w, status: http.StatusOK}
		if debugConfig.LogBodies {
			wrapped.bodyLimit = debugBodyLogLimit(debugConfig.MaxBodyLogSize)
		}
		next.ServeHTTP(wrapped, r)
		s.debugLog(r, wrapped.status, time.Since(started), wrapped.size)
		if len(body) > 0 {
			attrs := []any{"request_body", sanitizeDebugBody(body, r.Header.Get("Content-Type"))}
			if int64(len(body)) > debugBodyLogLimit(debugConfig.MaxBodyLogSize) {
				attrs = append(attrs, "request_body_truncated", true)
			}
			slog.Debug("http request body", attrs...)
		}
		if len(wrapped.body) > 0 {
			attrs := []any{"response_body", sanitizeDebugBody(wrapped.body, w.Header().Get("Content-Type"))}
			if wrapped.bodyTruncated {
				attrs = append(attrs, "response_body_truncated", true)
			}
			slog.Debug("http response body", attrs...)
		}
	})
}

func (s *Server) debugLog(r *http.Request, status int, duration time.Duration, size int) {
	if !mode.IsDebugEnabled() || r == nil {
		return
	}
	slog.Debug("request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", redactSensitiveQuery(r.URL.RawQuery),
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"size", size,
		"remote_addr", requestRemoteIdentity(r),
		"user_agent", r.UserAgent(),
		"referer", redactLoggedURL(r.Referer()),
		"request_id", r.Header.Get("X-Request-ID"),
	)
}

func requestRemoteIdentity(r *http.Request) string {
	if TorCircuitID(r) != "" {
		return TorClientIdentity(r)
	}
	return r.RemoteAddr
}

func (s *Server) debugLogDB(query string, args []any, duration time.Duration, rows int64, err error) {
	if !s.debugOptionEnabled(func(settings config.DebugConfig) bool { return settings.LogQueries }) {
		return
	}
	attrs := []any{"query", query, "args_count", len(args), "duration_ms", duration.Milliseconds(), "rows", rows}
	if err != nil {
		attrs = append(attrs, "failed", true)
		slog.Debug("db query failed", attrs...)
		return
	}
	slog.Debug("db query", attrs...)
}

func (s *Server) debugOptionEnabled(enabled func(config.DebugConfig) bool) bool {
	if s == nil || !mode.IsDebugEnabled() {
		return false
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config != nil && enabled(s.config.Server.Debug)
}

type debugResponseWriter struct {
	http.ResponseWriter
	status        int
	size          int
	bodyLimit     int64
	body          []byte
	bodyTruncated bool
}

func (w *debugResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *debugResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	if w.bodyLimit > 0 && len(w.body) <= int(w.bodyLimit) {
		remaining := int(w.bodyLimit) + 1 - len(w.body)
		if remaining > len(data) {
			remaining = len(data)
		}
		w.body = append(w.body, data[:remaining]...)
		w.bodyTruncated = int64(len(w.body)) > w.bodyLimit
	}
	return n, err
}

func (w *debugResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *debugResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *debugResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *debugResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func debugBodyLogLimit(value string) int64 {
	value = strings.TrimSpace(strings.ToUpper(value))
	unit := int64(1)
	for _, suffix := range []struct {
		name       string
		multiplier int64
	}{{"KB", 1 << 10}, {"MB", 1 << 20}, {"B", 1}} {
		if strings.HasSuffix(value, suffix.name) {
			unit = suffix.multiplier
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix.name))
			break
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 || n > (1<<20)/unit {
		return 10 * 1024
	}
	return n * unit
}

func redactSensitiveQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "[redacted]"
	}
	for key := range values {
		if isSensitiveLogField(key) {
			values[key] = []string{"[redacted]"}
		}
	}
	return values.Encode()
}

func redactLoggedURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[redacted]"
	}
	parsed.User = nil
	parsed.RawQuery = redactSensitiveQuery(parsed.RawQuery)
	parsed.Fragment = ""
	value := parsed.String()
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

func sanitizeDebugBody(body []byte, contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "[redacted: unsupported body type]"
	}
	switch mediaType {
	case "application/x-www-form-urlencoded":
		return redactSensitiveQuery(string(body))
	case "application/json", "application/problem+json":
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return "[redacted: invalid JSON]"
		}
		redactJSONFields(value)
		redacted, err := json.Marshal(value)
		if err != nil {
			return "[redacted: invalid JSON]"
		}
		return string(redacted)
	default:
		return "[redacted: unsupported body type]"
	}
}

func redactJSONFields(value any) {
	switch object := value.(type) {
	case map[string]any:
		for key, field := range object {
			if isSensitiveLogField(key) {
				object[key] = "[redacted]"
				continue
			}
			redactJSONFields(field)
		}
	case []any:
		for _, field := range object {
			redactJSONFields(field)
		}
	}
}

func isSensitiveLogField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "token", "session", "code", "key", "password", "secret", "auth", "pwd", "api_key", "apikey", "access_token", "refresh_token", "csrf_token", "authorization", "client_secret":
		return true
	default:
		return false
	}
}
