package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPathSecurityMiddlewareRejectsTraversal(t *testing.T) {
	for _, target := range []string{"/static/../secret", "/static/%2e%2e/secret"} {
		t.Run(target, func(t *testing.T) {
			handler := URLNormalizeMiddleware(PathSecurityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestURLNormalizeMiddlewareCollapsesRepeatedSlashes(t *testing.T) {
	var got string
	handler := URLNormalizeMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "//server///healthz/", nil))
	if got != "/server/healthz/" {
		t.Fatalf("normalized path = %q, want %q", got, "/server/healthz/")
	}
}
