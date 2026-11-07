package server

import (
	"net/http"
	"strings"

	commonhttputil "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
)

func URLNormalizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original := r.URL.Path
		cleaned := original
		for strings.Contains(cleaned, "//") {
			cleaned = strings.ReplaceAll(cleaned, "//", "/")
		}
		cleaned = strings.ReplaceAll(cleaned, "/./", "/")
		if strings.HasSuffix(cleaned, "/.") {
			cleaned = strings.TrimSuffix(cleaned, ".")
		}
		if cleaned == "" {
			cleaned = "/"
		}
		r.URL.Path = cleaned
		next.ServeHTTP(w, r)
	})
}

func PathSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original := r.URL.Path
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = original
		}
		if strings.Contains(original, "..") || strings.Contains(rawPath, "..") ||
			strings.Contains(strings.ToLower(rawPath), "%2e") {
			message := i18n.T(i18n.FromContext(r.Context()), "errors.bad_request")
			if commonhttputil.GetAPIResponseFormat(r) == "text" {
				writeTextError(w, http.StatusBadRequest, "BAD_REQUEST", message)
			} else {
				writeJSONError(w, r, http.StatusBadRequest, "BAD_REQUEST", message)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
