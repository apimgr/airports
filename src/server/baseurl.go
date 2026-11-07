package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/apimgr/airports/src/config"
)

type baseURLContextKey struct{}

// TrustedBaseURLMiddleware resolves a request's public path prefix from a
// trusted proxy header or the configured fallback, then removes that prefix
// from the request path when the proxy preserved it.
func TrustedBaseURLMiddleware(configuredBaseURL string, trustedCIDRs ...string) func(http.Handler) http.Handler {
	fixedCIDRs := append([]string(nil), trustedCIDRs...)
	return trustedBaseURLMiddlewareWithCIDRSource(configuredBaseURL, func() []string { return fixedCIDRs })
}

func trustedBaseURLMiddlewareWithCIDRSource(configuredBaseURL string, cidrsForRequest func() []string) func(http.Handler) http.Handler {
	fallback, valid := config.NormalizeBaseURL(configuredBaseURL)
	if !valid {
		fallback = "/"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request := r.Clone(r.Context())
			request.Header = r.Header.Clone()
			baseURL := fallback
			if isTrustedProxyPeer(r.RemoteAddr, cidrsForRequest()) {
				for _, name := range []string{"X-Forwarded-Prefix", "X-Forwarded-Path", "X-Script-Name"} {
					value := firstForwardedValue(r.Header.Get(name))
					if value == "" {
						continue
					}
					if normalized, ok := config.NormalizeBaseURL(value); ok {
						baseURL = normalized
						break
					}
				}
			}
			request = request.WithContext(context.WithValue(request.Context(), baseURLContextKey{}, baseURL))
			request.URL.Path = removeBaseURLPrefix(request.URL.Path, baseURL)
			request.URL.RawPath = ""
			next.ServeHTTP(w, request)
		})
	}
}

// RequestBaseURL returns the resolved public path prefix for URL generation.
// It returns "/" when the request did not pass through the base URL middleware.
func RequestBaseURL(r *http.Request) string {
	if r == nil {
		return "/"
	}
	if baseURL, ok := r.Context().Value(baseURLContextKey{}).(string); ok && baseURL != "" {
		return baseURL
	}
	return "/"
}

func removeBaseURLPrefix(requestPath, baseURL string) string {
	if baseURL == "/" {
		return requestPath
	}
	if requestPath == baseURL {
		return "/"
	}
	if strings.HasPrefix(requestPath, baseURL+"/") {
		return strings.TrimPrefix(requestPath, baseURL)
	}
	return requestPath
}
