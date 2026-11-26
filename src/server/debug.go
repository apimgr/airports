package server

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/config"
	"github.com/go-chi/chi/v5"
)

// registerDebugRoutes registers debug endpoints (--debug/DEBUG=true only).
func (s *Server) registerDebugRoutes(r chi.Router) {
	if s.config == nil {
		return
	}
	if s.routeRegistry == nil {
		s.routeRegistry = &routeRegistry{router: r}
	}

	debugConfig := s.config.Server.Debug
	runtime.SetBlockProfileRate(debugConfig.BlockProfileRate)
	runtime.SetMutexProfileFraction(debugConfig.MutexProfileFraction)
	register := func(path string, handler http.Handler) {
		protected := s.requireOperatorToken(handler)
		s.routeRegistry.Add(Route{
			Method: http.MethodGet, Path: "/debug" + path, Auth: "operator",
			Internal: true, DebugOnly: true, Handler: protected.ServeHTTP,
		})
	}
	if debugConfig.Pprof {
		register("/pprof", http.HandlerFunc(s.handleDebugPprofIndex))
		register("/pprof/cmdline", http.HandlerFunc(s.handleDebugPprofCmdline))
		register("/pprof/profile", http.HandlerFunc(pprof.Profile))
		register("/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		register("/pprof/trace", http.HandlerFunc(pprof.Trace))
		for _, name := range []string{"heap", "goroutine", "allocs", "block", "mutex", "threadcreate"} {
			register("/pprof/"+name, pprof.Handler(name))
		}
	}
	if debugConfig.RuntimeEndpoints {
		register("/vars", http.HandlerFunc(s.handleDebugVars))
	}
	for _, endpoint := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{path: "/config", handler: s.handleDebugConfig},
		{path: "/routes", handler: s.handleDebugRoutes},
		{path: "/cache", handler: s.handleDebugCache},
		{path: "/db", handler: s.handleDebugDB},
		{path: "/scheduler", handler: s.handleDebugScheduler},
		{path: "/memory", handler: s.handleDebugMemory},
		{path: "/goroutines", handler: s.handleDebugGoroutines},
		{path: "/error500", handler: s.handleDebugError500},
	} {
		register(endpoint.path, endpoint.handler)
	}
}

func (s *Server) handleDebugPprofIndex(w http.ResponseWriter, r *http.Request) {
	request := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path += "/"
	request.URL = &urlCopy
	pprof.Index(w, request)
}

// debugBinaryName returns only the executable name, never its command-line arguments.
func debugBinaryName() string {
	if executable, err := os.Executable(); err == nil {
		if name := filepath.Base(executable); name != "." && name != string(filepath.Separator) && name != "" {
			return name
		}
	}
	if len(os.Args) > 0 {
		if name := filepath.Base(os.Args[0]); name != "." && name != string(filepath.Separator) && name != "" {
			return name
		}
	}
	return "airports"
}

// handleDebugPprofCmdline preserves pprof's NUL-terminated response format while
// ensuring process arguments (which may contain credentials) are never exposed.
func (s *Server) handleDebugPprofCmdline(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = fmt.Fprint(w, debugBinaryName(), "\x00")
}

// handleDebugVars serves expvar's JSON document with its unsafe cmdline value
// replaced by the binary name before any bytes are sent to the client.
func (s *Server) handleDebugVars(w http.ResponseWriter, r *http.Request) {
	buffer := &debugResponseBuffer{header: make(http.Header)}
	request := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.RawQuery = ""
	request.URL = &urlCopy
	expvar.Handler().ServeHTTP(buffer, request)

	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(buffer.body.Bytes(), &values); err != nil {
		http.Error(w, "Unable to encode runtime variables", http.StatusInternalServerError)
		return
	}
	cmdline, err := json.Marshal(debugBinaryName())
	if err != nil {
		http.Error(w, "Unable to encode runtime variables", http.StatusInternalServerError)
		return
	}
	values["cmdline"] = cmdline
	body, err := json.Marshal(values)
	if err != nil {
		http.Error(w, "Unable to encode runtime variables", http.StatusInternalServerError)
		return
	}
	for name, entries := range buffer.header {
		if strings.EqualFold(name, "Content-Length") {
			continue
		}
		for _, entry := range entries {
			w.Header().Add(name, entry)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	status := buffer.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

type debugResponseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *debugResponseBuffer) Header() http.Header            { return b.header }
func (b *debugResponseBuffer) WriteHeader(status int)         { b.status = status }
func (b *debugResponseBuffer) Write(data []byte) (int, error) { return b.body.Write(data) }

// handleDebugError500 uses the themed renderer to provide a deterministic
// 500 response for the E2E error-page check.
func (s *Server) handleDebugError500(w http.ResponseWriter, r *http.Request) {
	s.renderErrorPage(w, r, http.StatusInternalServerError)
}

func (s *Server) requireOperatorToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := bearerCredential(r)
		expected := strings.TrimSpace(s.config.Server.Token)
		if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			s.writeErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "A valid operator Bearer token is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleDebugDB returns connection pool statistics without exposing the
// database connection string or filesystem path.
func (s *Server) handleDebugDB(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		s.respondItem(w, http.StatusOK, map[string]interface{}{
			"configured": false,
		})
		return
	}

	stats := s.db.Stats()
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"configured":           true,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_open_connections": stats.MaxOpenConnections,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	})
}

// handleDebugCache returns cache backend type, availability, and the exact
// usage statistics exposed by the configured backend.
func (s *Server) handleDebugCache(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.respondItem(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"status":     "error",
		})
		return
	}

	driver := ""
	if s.config != nil {
		driver = s.config.Server.Cache.Type
	}
	result := map[string]interface{}{
		"configured": true,
		"driver":     driver,
		"status":     s.checkCache(),
	}
	if provider, ok := s.cache.(cachepkg.MetricsProvider); ok {
		stats := provider.MetricsStats()
		result["statistics_available"] = stats.Enabled
		if stats.Enabled {
			result["hits"] = stats.Hits
			result["misses"] = stats.Misses
			if stats.HasEvictionCount {
				result["evictions"] = stats.Evictions
			}
			if stats.HasSize {
				result["items"] = stats.Items
				result["bytes"] = stats.Bytes
			}
		}
	}
	s.respondItem(w, http.StatusOK, result)
}

// handleDebugConfig returns the current configuration with sensitive
// values (DNS API keys, RFC2136 TSIG secrets) redacted.
func (s *Server) handleDebugConfig(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, config.GetSanitizedApplicationConfig())
}

// handleDebugRoutes returns all registered routes.
func (s *Server) handleDebugRoutes(w http.ResponseWriter, r *http.Request) {
	routes := []map[string]string{}

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, map[string]string{
			"method": method,
			"route":  route,
		})
		return nil
	}

	if err := chi.Walk(s.router, walkFunc); err != nil {
		s.respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "errors.route_walk_failed")
		return
	}

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"count":  len(routes),
		"routes": routes,
	})
}

// handleDebugMemory returns memory statistics.
func (s *Server) handleDebugMemory(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"alloc_mb":       m.Alloc / 1024 / 1024,
		"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
		"sys_mb":         m.Sys / 1024 / 1024,
		"num_gc":         m.NumGC,
		"heap_objects":   m.HeapObjects,
		"goroutines":     runtime.NumGoroutine(),
	})
}

// handleDebugGoroutines returns goroutine count and stack traces.
func (s *Server) handleDebugGoroutines(w http.ResponseWriter, r *http.Request) {
	// 1MB buffer for stack traces
	buf := make([]byte, 1024*1024)
	// true = include all goroutines
	n := runtime.Stack(buf, true)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf[:n])
}

// handleDebugScheduler returns scheduler task status.
func (s *Server) handleDebugScheduler(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		s.respondItem(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"tasks":      []interface{}{},
		})
		return
	}

	tasks := s.scheduler.List()
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"configured": true,
		"count":      len(tasks),
		"tasks":      tasks,
	})
}
