package server

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/logging"
)

func TestShouldSuppressHealthAccessLog(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
		want   bool
	}{
		{name: "frontend health", path: "/server/healthz", status: http.StatusOK, want: true},
		{name: "versioned API health", path: "/api/v1/server/healthz", status: http.StatusOK, want: true},
		{name: "versioned API text health", path: "/api/v1/server/healthz.txt", status: http.StatusOK, want: true},
		{name: "failed health", path: "/server/healthz", status: http.StatusServiceUnavailable},
		{name: "ordinary route", path: "/server/about", status: http.StatusOK},
		{name: "lookalike route", path: "/api/v1/server/healthz/private", status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSuppressHealthAccessLog(test.path, test.status); got != test.want {
				t.Errorf("shouldSuppressHealthAccessLog(%q, %d) = %t, want %t", test.path, test.status, got, test.want)
			}
		})
	}
}

func TestAccessLoggerSuppressesHealthyChecksAndLogsFailures(t *testing.T) {
	dir := t.TempDir()
	access := logging.NewWriter(dir, "access", logging.FileConfig{Enabled: true, Filename: "access.log"})
	errors := logging.NewWriter(dir, "error", logging.FileConfig{Enabled: true, Filename: "error.log"})
	t.Cleanup(func() {
		_ = access.Close()
		_ = errors.Close()
	})
	server := &Server{accessLog: access, errorLog: errors}
	previousOutput := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	healthy := httptest.NewRequest(http.MethodGet, "/api/v1/server/healthz", nil)
	server.accessLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), healthy)
	if _, err := os.Stat(filepath.Join(dir, "access.log")); !os.IsNotExist(err) {
		t.Fatalf("successful health check created access.log: stat error = %v", err)
	}

	failed := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	server.accessLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})).ServeHTTP(httptest.NewRecorder(), failed)
	accessContent, err := os.ReadFile(filepath.Join(dir, "access.log"))
	if err != nil {
		t.Fatalf("read access.log: %v", err)
	}
	errorContent, err := os.ReadFile(filepath.Join(dir, "error.log"))
	if err != nil {
		t.Fatalf("read error.log: %v", err)
	}
	if !strings.Contains(string(accessContent), "/server/healthz") || !strings.Contains(string(errorContent), "/server/healthz") {
		t.Fatalf("failed health check missing from access/error logs: access=%q error=%q", accessContent, errorContent)
	}
}
