package server

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestIDMiddleware accepts a valid upstream UUID v4 or creates one for
// every request, then exposes it in the response and chi request context.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := firstRequestID(r.Header)
		parsed, err := uuid.Parse(requestID)
		if requestID == "" || err != nil || parsed.Version() != uuid.Version(4) || parsed.Variant() != uuid.RFC4122 {
			if requestID != "" {
				log.Printf("Warning: invalid request ID header ignored")
			}
			requestID = uuid.NewString()
		} else {
			requestID = parsed.String()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func firstRequestID(header http.Header) string {
	for _, name := range []string{"X-Request-ID", "X-Correlation-ID", "X-Trace-ID"} {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}
