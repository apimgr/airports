package httputil

import (
	"net/http"
	"strings"
)

func IsOurCLIClient(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("User-Agent"), "airports-cli/")
}

func IsTextBrowser(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	for _, marker := range []string{"lynx/", "w3m/", "links ", "links/", "elinks/", "browsh/", "carbonyl/", "netsurf"} {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
}

func IsHTTPTool(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	for _, marker := range []string{"curl/", "wget/", "httpie/", "libcurl/", "python-requests/", "go-http-client/", "axios/", "node-fetch/"} {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return ua == ""
}

func IsNonInteractiveClient(r *http.Request) bool {
	if IsOurCLIClient(r) || IsTextBrowser(r) {
		return false
	}
	return IsHTTPTool(r)
}

func GetAPIResponseFormat(r *http.Request) string {
	if strings.HasSuffix(r.URL.Path, ".txt") {
		return "text"
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/plain") {
		return "text"
	}
	if strings.Contains(accept, "application/json") {
		return "json"
	}
	if IsNonInteractiveClient(r) {
		return "text"
	}
	return "json"
}

func GetFrontendResponseFormat(r *http.Request) string {
	if strings.HasSuffix(r.URL.Path, ".txt") {
		return "text"
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return "html"
	}
	if strings.Contains(accept, "text/plain") {
		return "text"
	}
	if IsOurCLIClient(r) {
		return "json"
	}
	if IsTextBrowser(r) {
		return "nojs"
	}
	if IsHTTPTool(r) {
		return "text"
	}
	return "html"
}
