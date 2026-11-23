package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type accessLogRecord struct {
	Time      string `json:"time"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Size      int    `json:"size"`
	Remote    string `json:"ip"`
	UserAgent string `json:"ua"`
	Referer   string `json:"referer"`
}

func (s *Server) accessLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		if shouldSuppressHealthAccessLog(r.URL.Path, status) {
			return
		}
		line := s.formatAccessLog(r, status, wrapped.BytesWritten(), started)
		if s.accessLog != nil {
			if err := s.accessLog.Write(line); err != nil {
				log.Printf("Warning: failed to write access log: %v", err)
			}
		}
		log.Print(line)
		if isHealthCheckPath(r.URL.Path) && (status < http.StatusOK || status >= http.StatusMultipleChoices) {
			if s.errorLog != nil {
				if err := s.errorLog.Write(line); err != nil {
					log.Printf("Warning: failed to write health-check error log: %v", err)
				}
			}
			log.Printf("ERROR: health check returned HTTP %d for %s", status, r.URL.Path)
		}
	})
}

func isHealthCheckPath(path string) bool {
	switch path {
	case "/server/healthz", "/server/healthz.txt", "/healthz", "/healthz.txt", "/api/healthz", "/api/healthz.txt":
		return true
	}
	parts := strings.Split(path, "/")
	return len(parts) == 5 && parts[1] == "api" && parts[2] != "" && parts[3] == "server" && (parts[4] == "healthz" || parts[4] == "healthz.txt")
}

func shouldSuppressHealthAccessLog(path string, status int) bool {
	return isHealthCheckPath(path) && status >= http.StatusOK && status < http.StatusMultipleChoices
}

func (s *Server) formatAccessLog(r *http.Request, status, size int, started time.Time) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	requestURL := *r.URL
	requestURL.RawQuery = redactSensitiveQuery(requestURL.RawQuery)
	requestLine := r.Method + " " + requestURL.RequestURI() + " " + r.Proto

	s.configMu.RLock()
	format := "apache"
	if s.config != nil && s.config.Server.Logging.Access.Format != "" {
		format = s.config.Server.Logging.Access.Format
	}
	s.configMu.RUnlock()
	switch format {
	case "nginx":
		return fmt.Sprintf("%s - - [%s] %q %d %d", remote, started.Format("02/Jan/2006:15:04:05 -0700"), requestLine, status, size)
	case "json":
		encoded, err := json.Marshal(accessLogRecord{
			Time: started.UTC().Format(time.RFC3339), Method: r.Method, Path: requestURL.RequestURI(),
			Status: status, Size: size, Remote: remote, UserAgent: r.UserAgent(), Referer: redactLoggedURL(r.Referer()),
		})
		if err == nil {
			return string(encoded)
		}
	}
	referer := r.Referer()
	if referer == "" {
		referer = "-"
	} else {
		referer = redactLoggedURL(referer)
	}
	userAgent := r.UserAgent()
	if userAgent == "" {
		userAgent = "-"
	}
	return fmt.Sprintf("%s - - [%s] %q %d %s %q %q", remote, started.Format("02/Jan/2006:15:04:05 -0700"), requestLine, status, strconv.Itoa(size), referer, userAgent)
}
