package server

import (
	"expvar"
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
	"github.com/go-chi/chi/v5"
)

// registerDebugRoutes registers debug endpoints (--debug/DEBUG=true only).
func (s *Server) registerDebugRoutes(r chi.Router) {
	if !mode.ShouldShowDebugEndpoints() {
		return
	}

	r.Route("/debug", func(r chi.Router) {
		// pprof endpoints
		r.HandleFunc("/pprof/", pprof.Index)
		r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/pprof/profile", pprof.Profile)
		r.HandleFunc("/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/pprof/trace", pprof.Trace)
		r.Handle("/pprof/heap", pprof.Handler("heap"))
		r.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/pprof/allocs", pprof.Handler("allocs"))
		r.Handle("/pprof/block", pprof.Handler("block"))
		r.Handle("/pprof/mutex", pprof.Handler("mutex"))
		r.Handle("/pprof/threadcreate", pprof.Handler("threadcreate"))

		// expvar
		r.Handle("/vars", expvar.Handler())

		// Custom debug endpoints. This project has no separate DB or
		// cache layer (airports.Service holds the in-memory dataset
		// and there is no cache abstraction on Server), so /debug/db
		// and /debug/cache from the generic spec are replaced with
		// /debug/data (airports.Service.Stats()) and omitted respectively.
		r.Get("/config", s.handleDebugConfig)
		r.Get("/routes", s.handleDebugRoutes)
		r.Get("/data", s.handleDebugData)
		r.Get("/scheduler", s.handleDebugScheduler)
		r.Get("/memory", s.handleDebugMemory)
		r.Get("/goroutines", s.handleDebugGoroutines)
	})
}

// handleDebugConfig returns the current configuration with sensitive
// values (DNS API keys, RFC2136 TSIG secrets) redacted.
func (s *Server) handleDebugConfig(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, config.Sanitized())
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
	w.Write(buf[:n])
}

// handleDebugData returns airport data service statistics. This project
// has no separate DB layer — airports.Service holds the in-memory
// dataset, so its Stats() is the debug data source.
func (s *Server) handleDebugData(w http.ResponseWriter, r *http.Request) {
	if s.airports == nil {
		s.respondItem(w, http.StatusOK, map[string]interface{}{
			"configured": false,
		})
		return
	}

	stats := s.airports.Stats()
	s.respondItem(w, http.StatusOK, stats)
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
