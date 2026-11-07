package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedBaseURLMiddlewareHeaderPriorityAndPathRewrite(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://internal/proxy-prefix/server/healthz", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("X-Forwarded-Prefix", "/proxy-prefix/")
	request.Header.Set("X-Forwarded-Path", "/fallback")
	var gotBaseURL, gotPath string
	handler := TrustedBaseURLMiddleware("/configured", defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBaseURL = RequestBaseURL(r)
		gotPath = r.URL.Path
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if gotBaseURL != "/proxy-prefix" {
		t.Errorf("RequestBaseURL() = %q, want /proxy-prefix", gotBaseURL)
	}
	if gotPath != "/server/healthz" {
		t.Errorf("request path = %q, want /server/healthz", gotPath)
	}
}

func TestTrustedBaseURLMiddlewareStripsMatchingPrefix(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/myproject", want: "/"},
		{path: "/myproject/", want: "/"},
		{path: "/myproject/server/healthz", want: "/server/healthz"},
		{path: "/myprojected/server/healthz", want: "/myprojected/server/healthz"},
	} {
		request := httptest.NewRequest(http.MethodGet, "http://internal"+tc.path, nil)
		request.RemoteAddr = "10.0.0.5:443"
		request.Header.Set("X-Forwarded-Prefix", "/myproject")
		var got string
		handler := TrustedBaseURLMiddleware("/configured", defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = r.URL.Path
		}))
		handler.ServeHTTP(httptest.NewRecorder(), request)
		if got != tc.want {
			t.Errorf("path %q normalized to %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestTrustedBaseURLMiddlewareIgnoresUntrustedHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://internal/configured/path", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Forwarded-Prefix", "/spoofed")
	var gotBaseURL, gotPath string
	handler := TrustedBaseURLMiddleware("/configured", defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBaseURL = RequestBaseURL(r)
		gotPath = r.URL.Path
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if gotBaseURL != "/configured" || gotPath != "/path" {
		t.Errorf("untrusted proxy result = (%q, %q), want (/configured, /path)", gotBaseURL, gotPath)
	}
}

func TestTrustedBaseURLMiddlewareUsesConfigWhenHeaderMissing(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://internal/myproject/server/healthz", nil)
	request.RemoteAddr = "198.51.100.10:443"
	var gotBaseURL, gotPath string
	handler := TrustedBaseURLMiddleware("/myproject/", defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBaseURL = RequestBaseURL(r)
		gotPath = r.URL.Path
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if gotBaseURL != "/myproject" || gotPath != "/server/healthz" {
		t.Errorf("configured base path result = (%q, %q), want (/myproject, /server/healthz)", gotBaseURL, gotPath)
	}
}

func TestTrustedBaseURLMiddlewareHeaderFallbacks(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "forwarded path fallback", headers: map[string]string{"X-Forwarded-Path": "/path/"}, want: "/path"},
		{name: "script name fallback", headers: map[string]string{"X-Script-Name": "/script"}, want: "/script"},
		{name: "invalid header skipped", headers: map[string]string{"X-Forwarded-Prefix": "//evil", "X-Forwarded-Path": "/fallback"}, want: "/fallback"},
		{name: "first valid header wins", headers: map[string]string{"X-Forwarded-Prefix": "/prefix", "X-Forwarded-Path": "/path"}, want: "/prefix"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
			request.RemoteAddr = "10.0.0.5:443"
			for name, value := range tc.headers {
				request.Header.Set(name, value)
			}
			var got string
			handler := TrustedBaseURLMiddleware("/configured", defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = RequestBaseURL(r)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if got != tc.want {
				t.Errorf("RequestBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
