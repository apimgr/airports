package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderFallbackError(t *testing.T) {
	t.Run("html page", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/missing", nil)
		response := httptest.NewRecorder()
		renderFallbackError(response, request, http.StatusInternalServerError)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
		}
		if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want HTML", got)
		}
		for _, want := range []string{"<!doctype html>", "<title>500 Internal Server Error</title>", "<main>", "<h1>500 Internal Server Error</h1>", "href=\"/\""} {
			if !strings.Contains(response.Body.String(), want) {
				t.Errorf("fallback body does not contain %q", want)
			}
		}
	})

	t.Run("json envelope", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()
		renderFallbackError(response, request, http.StatusInternalServerError)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
		}
		if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want JSON", got)
		}
		if got := response.Body.String(); !strings.Contains(got, `"ok":false`) || !strings.Contains(got, `"error":"SERVER_ERROR"`) || strings.Contains(got, `"status"`) {
			t.Fatalf("fallback JSON body has wrong envelope: %s", got)
		}
	})
}
