package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestBodyLimitRejectsOversizedContentLength(t *testing.T) {
	called := false
	handler := requestBodyLimitMiddleware(4)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if called {
		t.Fatal("downstream handler ran for oversized Content-Length")
	}
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestRequestBodyLimitBoundsStreamingBody(t *testing.T) {
	var body []byte
	var readErr error
	handler := requestBodyLimitMiddleware(4)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, readErr = io.ReadAll(r.Body)
	}))
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	request.ContentLength = -1
	handler.ServeHTTP(httptest.NewRecorder(), request)
	var maxBytesErr *http.MaxBytesError
	if !errors.As(readErr, &maxBytesErr) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
	if string(body) != "1234" {
		t.Errorf("body = %q, want capped body %q", body, "1234")
	}
}
