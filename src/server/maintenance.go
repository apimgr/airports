package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const defaultMaintenanceRetryAfter = 30

// StartupMaintenanceHandler keeps the translated health page and its assets
// available while database open/schema recovery is running. Every other route
// is closed with the canonical maintenance error response.
func StartupMaintenanceHandler(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReadOnlyMethod(r.Method) && (isMaintenanceHealthPath(r.URL.Path) || strings.HasPrefix(r.URL.Path, "/static/")) {
			s.Router().ServeHTTP(w, r)
			return
		}
		state := currentHealthRuntimeState()
		reason := "database_connection"
		message := "Database connection failed"
		if state.Maintenance != nil {
			reason = state.Maintenance.Reason
			message = state.Maintenance.Message
		}
		w.Header().Set("Retry-After", maintenanceRetryAfter(s.maintenanceRetryInterval()))
		w.Header().Set("X-Maintenance-Mode", "true")
		w.Header().Set("X-Maintenance-Reason", reason)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		response := ErrorResponse{OK: false, Error: "MAINTENANCE", Message: "Server is in maintenance mode due to: " + message, Details: map[string]interface{}{"reason": reason, "self_healing": true}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("startup maintenance response encode failed: %v", err)
		}
	})
}

func isMaintenanceHealthPath(path string) bool {
	return path == "/healthz" || path == "/healthz.txt" || path == "/server/healthz" || path == "/server/healthz.txt" ||
		path == "/api/healthz" || path == "/api/healthz.txt" || strings.HasSuffix(path, "/server/healthz") || strings.HasSuffix(path, "/server/healthz.txt")
}

// maintenanceWriteGate allows safe HTTP methods through and rejects all
// other methods while the process is in maintenance state.
func (s *Server) maintenanceWriteGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReadOnlyMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		state := currentHealthRuntimeState()
		if state.Maintenance == nil {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Retry-After", maintenanceRetryAfter(s.maintenanceRetryInterval()))
		w.Header().Set("X-Maintenance-Mode", "true")
		w.Header().Set("X-Maintenance-Reason", state.Maintenance.Reason)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)

		details := map[string]interface{}{
			"reason":       state.Maintenance.Reason,
			"self_healing": state.Maintenance.SelfHealing.Enabled,
		}
		if requestID := middleware.GetReqID(r.Context()); requestID != "" {
			details["request_id"] = requestID
		}
		response := ErrorResponse{
			OK:      false,
			Error:   "MAINTENANCE",
			Message: "Server is in maintenance mode due to: " + state.Maintenance.Message,
			Details: details,
		}
		if err := encodeJSON(w, response); err != nil {
			log.Printf("maintenanceWriteGate: encode error response: %v", err)
		}
	})
}

func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func (s *Server) maintenanceRetryInterval() string {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	if s.config == nil {
		return ""
	}
	return s.config.Server.Maintenance.SelfHealing.RetryInterval
}

func maintenanceRetryAfter(retryInterval string) string {
	duration, err := time.ParseDuration(strings.TrimSpace(retryInterval))
	if err != nil || duration <= 0 {
		return strconv.Itoa(defaultMaintenanceRetryAfter)
	}
	seconds := int(duration.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
