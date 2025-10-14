package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/apimgr/airports/src/database"
)

type contextKey string

const adminAuthKey contextKey = "admin_authenticated"

// AdminAuthMiddleware checks for valid admin authentication
// Supports both Bearer token (API) and Basic auth (Web UI)
func AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")

		if authHeader != "" {
			// Try Bearer token first (API)
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if database.ValidateToken(token) {
					ctx := context.WithValue(r.Context(), adminAuthKey, true)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try Basic auth (Web UI)
			if strings.HasPrefix(authHeader, "Basic ") {
				payload := strings.TrimPrefix(authHeader, "Basic ")
				decoded, err := base64.StdEncoding.DecodeString(payload)
				if err == nil {
					parts := strings.SplitN(string(decoded), ":", 2)
					if len(parts) == 2 {
						username := parts[0]
						password := parts[1]

						// Validate credentials
						storedUsername := database.GetSettingValue("admin.username", "administrator")
						if username == storedUsername && database.ValidatePassword(password) {
							ctx := context.WithValue(r.Context(), adminAuthKey, true)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}
		}

		// Check for session cookie (after successful Basic auth)
		if cookie, err := r.Cookie("admin_session"); err == nil && cookie.Value != "" {
			if database.ValidateToken(cookie.Value) {
				ctx := context.WithValue(r.Context(), adminAuthKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// No valid authentication found
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin Area"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// IsAdminAuthenticated checks if the request is authenticated
func IsAdminAuthenticated(r *http.Request) bool {
	val := r.Context().Value(adminAuthKey)
	if val == nil {
		return false
	}
	authenticated, ok := val.(bool)
	return ok && authenticated
}

// RequireAdminAuth is a convenience wrapper for handlers
func (s *Server) RequireAdminAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminAuthenticated(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Admin Area"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}
