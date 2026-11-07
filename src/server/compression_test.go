package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestGzipCompressionMiddlewareCompressesConfiguredContent(t *testing.T) {
	settings := config.CompressionConfig{
		Enabled: true,
		Level:   5,
		Types:   []string{"application/json"},
	}
	body := strings.Repeat(`{"ok":true}`, 100)
	handler := gzipCompressionMiddleware(func() config.CompressionConfig { return settings })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Vary", "Accept")
		w.Header().Set("Content-Length", "1100")
		w.Header().Set("ETag", `"original"`)
		_, _ = io.WriteString(w, body)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "br, gzip;q=0.8")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := response.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length = %q, want removed for compressed response", got)
	}
	if got := response.Header().Get("ETag"); got != "" {
		t.Errorf("ETag = %q, want removed for transformed response", got)
	}
	if !varyContains(response.Header(), "Accept") || !varyContains(response.Header(), "Accept-Encoding") {
		t.Errorf("Vary = %v, want Accept and Accept-Encoding", response.Header().Values("Vary"))
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read compressed body: %v", err)
	}
	if string(decoded) != body {
		t.Errorf("decoded body = %q, want original payload", decoded)
	}
}

func varyContains(header http.Header, want string) bool {
	for _, value := range header.Values("Vary") {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}

func TestGzipCompressionMiddlewarePassesThroughIneligibleResponses(t *testing.T) {
	settings := config.CompressionConfig{
		Enabled: true,
		Level:   5,
		Types:   []string{"text/*"},
	}
	tests := []struct {
		name           string
		method         string
		acceptEncoding string
		contentType    string
		status         int
		wantEncoding   string
	}{
		{name: "disabled by client quality", method: http.MethodGet, acceptEncoding: "gzip;q=0, *;q=1", contentType: "text/plain", status: http.StatusOK},
		{name: "unsupported MIME type", method: http.MethodGet, acceptEncoding: "gzip", contentType: "image/png", status: http.StatusOK},
		{name: "head request", method: http.MethodHead, acceptEncoding: "gzip", contentType: "text/plain", status: http.StatusOK},
		{name: "no content", method: http.MethodGet, acceptEncoding: "gzip", contentType: "text/plain", status: http.StatusNoContent},
		{name: "partial response", method: http.MethodGet, acceptEncoding: "gzip", contentType: "text/plain", status: http.StatusPartialContent},
		{name: "not modified", method: http.MethodGet, acceptEncoding: "gzip", contentType: "text/plain", status: http.StatusNotModified},
		{name: "no transform directive", method: http.MethodGet, acceptEncoding: "gzip", contentType: "text/plain", status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := gzipCompressionMiddleware(func() config.CompressionConfig { return settings })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				if test.name == "no transform directive" {
					w.Header().Set("Cache-Control", "public, no-transform")
				}
				w.WriteHeader(test.status)
				if test.status != http.StatusNoContent && test.status != http.StatusNotModified {
					_, _ = io.WriteString(w, "body")
				}
			}))
			request := httptest.NewRequest(test.method, "/", nil)
			request.Header.Set("Accept-Encoding", test.acceptEncoding)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Errorf("status = %d, want %d", response.Code, test.status)
			}
			if got := response.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
			if got := response.Header().Get("Vary"); got != "Accept-Encoding" {
				t.Errorf("Vary = %q, want Accept-Encoding", got)
			}
		})
	}
}

func TestGzipCompressionMiddlewareReadsUpdatedSettings(t *testing.T) {
	settings := config.CompressionConfig{Enabled: false}
	handler := gzipCompressionMiddleware(func() config.CompressionConfig { return settings })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "body")
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request)
	if got := first.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding before enabling = %q, want empty", got)
	}

	settings = config.CompressionConfig{Enabled: true, Level: 5, Types: []string{"text/plain"}}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if got := second.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding after enabling = %q, want gzip", got)
	}
}
