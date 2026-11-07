package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func TestRequestIDMiddlewareUsesValidHeaderFallbacks(t *testing.T) {
	validID := "550e8400-e29b-41d4-a716-446655440000"
	for _, header := range []string{"X-Request-ID", "X-Correlation-ID", "X-Trace-ID"} {
		t.Run(header, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(header, validID)
			response := httptest.NewRecorder()
			var contextID string
			RequestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				contextID = middleware.GetReqID(r.Context())
			})).ServeHTTP(response, request)
			if got := response.Header().Get("X-Request-ID"); got != validID {
				t.Errorf("response request ID = %q, want %q", got, validID)
			}
			if contextID != validID {
				t.Errorf("context request ID = %q, want %q", contextID, validID)
			}
		})
	}
}

func TestRequestIDMiddlewareGeneratesUUIDv4ForInvalidOrMissingID(t *testing.T) {
	for _, supplied := range []string{"", "not-a-uuid", "550e8400-e29b-11d4-a716-446655440000"} {
		t.Run(supplied, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if supplied != "" {
				request.Header.Set("X-Request-ID", supplied)
			}
			response := httptest.NewRecorder()
			RequestIDMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(response, request)
			id := response.Header().Get("X-Request-ID")
			if id == supplied {
				t.Fatalf("invalid request ID was retained: %q", id)
			}
			if !isUUIDv4(id) {
				t.Errorf("generated request ID %q is not UUID v4", id)
			}
		})
	}
}

func isUUIDv4(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.Version() == uuid.Version(4) && parsed.Variant() == uuid.RFC4122
}
