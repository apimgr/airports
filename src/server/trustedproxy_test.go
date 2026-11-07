package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func TestIsTrustedProxyRequestUsesRouterCIDRs(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	srv.proxyCIDRs = newTrustedProxyCIDRCache(func() *config.Config { return srv.config })
	tests := []struct {
		name       string
		remoteAddr string
		want       bool
	}{
		{name: "private proxy", remoteAddr: "10.0.0.5:8080", want: true},
		{name: "public unlisted peer", remoteAddr: "203.0.113.8:8080", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = test.remoteAddr
			if got := srv.IsTrustedProxyRequest(r); got != test.want {
				t.Errorf("IsTrustedProxyRequest() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestTrustedClientIPFromHeadersPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "cloudflare", header: "CF-Connecting-IP", want: "203.0.113.1"},
		{name: "true client", header: "True-Client-IP", want: "203.0.113.2"},
		{name: "real ip", header: "X-Real-IP", want: "203.0.113.3"},
		{name: "forwarded for", header: "X-Forwarded-For", want: "203.0.113.4"},
		{name: "client ip", header: "X-Client-IP", want: "203.0.113.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = "10.0.0.5:443"
			if tc.header != "X-Client-IP" {
				request.Header.Set("X-Forwarded-For", "203.0.113.4")
			}
			request.Header.Set(tc.header, tc.want)
			var got string
			handler := TrustedClientIPFromHeaders(defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = getClientIP(r)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if got != tc.want {
				t.Errorf("client IP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveTrustedProxyCIDRsIncludesListenAddressSubnet(t *testing.T) {
	cidrs := resolveTrustedProxyCIDRs(&config.Config{Server: config.ServerConfig{Address: "192.168.44.17"}})
	for _, cidr := range cidrs {
		if cidr == "192.168.44.0/24" {
			return
		}
	}
	t.Fatalf("trusted CIDRs %v do not include the listen address /24", cidrs)
}

func TestTrustedProxyDNSIsCachedThenRefreshed(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{TrustedProxies: config.TrustedProxiesConfig{Additional: []string{"proxy.example"}}}}
	lookups := 0
	source := newTrustedProxyCIDRCache(func() *config.Config { return cfg })
	source.lookup = func(string) ([]string, error) {
		lookups++
		if lookups == 1 {
			return []string{"203.0.113.1"}, nil
		}
		return []string{"203.0.113.2"}, nil
	}

	contains := func(cidrs []string, wanted string) bool {
		for _, cidr := range cidrs {
			if cidr == wanted {
				return true
			}
		}
		return false
	}
	first := source.CIDRs()
	if !contains(first, "203.0.113.1/32") || lookups != 1 {
		t.Fatalf("initial CIDRs=%v lookups=%d, want 203.0.113.1/32 and one lookup", first, lookups)
	}
	second := source.CIDRs()
	if !contains(second, "203.0.113.1/32") || lookups != 1 {
		t.Fatalf("cached CIDRs=%v lookups=%d, want cached first result", second, lookups)
	}

	source.refreshed = time.Now().Add(-trustedProxyDNSRefreshInterval)
	third := source.CIDRs()
	if !contains(third, "203.0.113.2/32") || lookups != 2 {
		t.Fatalf("refreshed CIDRs=%v lookups=%d, want 203.0.113.2/32 and two lookups", third, lookups)
	}
}

func TestBothProxyMiddlewarePathsUseRefreshedDNS(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{TrustedProxies: config.TrustedProxiesConfig{Additional: []string{"proxy.example"}}}}
	lookups := 0
	source := newTrustedProxyCIDRCache(func() *config.Config { return cfg })
	source.lookup = func(string) ([]string, error) {
		lookups++
		if lookups == 1 {
			return []string{"203.0.113.1"}, nil
		}
		return []string{"203.0.113.2"}, nil
	}
	checkClientIP := func(peer string) string {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = peer + ":443"
		request.Header.Set("X-Forwarded-For", "198.51.100.22")
		var got string
		handler := trustedClientIPFromCIDRSource(source.CIDRs)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = getClientIP(r)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), request)
		return got
	}
	if got := checkClientIP("203.0.113.1"); got != "198.51.100.22" {
		t.Fatalf("client IP before DNS refresh = %q, want forwarded client", got)
	}
	source.refreshed = time.Now().Add(-trustedProxyDNSRefreshInterval)
	if got := checkClientIP("203.0.113.2"); got != "198.51.100.22" {
		t.Fatalf("client IP after DNS refresh = %q, want forwarded client", got)
	}

	request := httptest.NewRequest(http.MethodGet, "/site/health", nil)
	request.RemoteAddr = "203.0.113.2:443"
	request.Header.Set("X-Forwarded-Prefix", "/site")
	var gotBaseURL, gotPath string
	handler := trustedBaseURLMiddlewareWithCIDRSource("/", source.CIDRs)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBaseURL = RequestBaseURL(r)
		gotPath = r.URL.Path
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if gotBaseURL != "/site" || gotPath != "/health" {
		t.Fatalf("refreshed prefix result = base %q path %q, want base /site path /health", gotBaseURL, gotPath)
	}
}

func TestTrustedClientIPFromHeadersIgnoresUntrustedPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("CF-Connecting-IP", "203.0.113.99")
	var got, original string
	handler := TrustedClientIPFromHeaders(defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = getClientIP(r)
		original = OriginalTCPPeer(r)
		r.RemoteAddr = "203.0.113.200:443"
		if peer := OriginalTCPPeer(r); peer != original {
			t.Errorf("OriginalTCPPeer() changed after RemoteAddr mutation: got %q, want %q", peer, original)
		}
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if got != "198.51.100.10" {
		t.Errorf("client IP = %q, want original untrusted TCP peer", got)
	}
	if original != "198.51.100.10:443" {
		t.Errorf("OriginalTCPPeer() = %q, want original socket peer", original)
	}
}

func TestTrustedClientIPFromHeadersPreservesOriginalPeerWithSpoofedForwardedIP(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	var gotClient, gotPeer string
	handler := TrustedClientIPFromHeaders(defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotClient = getClientIP(r)
		gotPeer = OriginalTCPPeer(r)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if gotClient != "203.0.113.99" {
		t.Errorf("resolved client IP = %q, want forwarded client IP", gotClient)
	}
	if gotPeer != "10.0.0.5:443" {
		t.Errorf("OriginalTCPPeer() = %q, want trusted proxy socket peer", gotPeer)
	}
}

func TestTrustedProxyHeadersResolvePublicOrigin(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "http://internal:8080/", nil)
	request.RemoteAddr = "10.0.0.5:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	request.Header.Set("X-Forwarded-Host", "public.example.com:443")
	request.Header.Set("X-Forwarded-Proto", "https, http")
	var got string
	handler := TrustedClientIPFromHeaders(defaultTrustedProxyCIDRs...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = s.publicBaseURL(r)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if got != "https://public.example.com" {
		t.Errorf("publicBaseURL() = %q, want https://public.example.com", got)
	}
}

func TestNormalizeTrustedProxyURLHeaders(t *testing.T) {
	tests := []struct {
		name     string
		host     []string
		port     []string
		wantHost string
	}{
		{name: "standard host and port", host: []string{"X-Forwarded-Host", "public.example"}, port: []string{"X-Forwarded-Port", "8443"}, wantHost: "public.example:8443"},
		{name: "real host and real port fallback", host: []string{"X-Real-Host", "public.example"}, port: []string{"X-Real-Port", "9443"}, wantHost: "public.example:9443"},
		{name: "forwarded port takes precedence", host: []string{"X-Forwarded-Host", "public.example:443"}, port: []string{"X-Forwarded-Port", "8443"}, wantHost: "public.example:8443"},
		{name: "ipv6 host", host: []string{"X-Forwarded-Host", "[2001:db8::1]"}, port: []string{"X-Forwarded-Port", "8080"}, wantHost: "[2001:db8::1]:8080"},
		{name: "invalid port is ignored", host: []string{"X-Forwarded-Host", "public.example"}, port: []string{"X-Forwarded-Port", "65536"}, wantHost: "public.example"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
			request.Header.Set(tc.host[0], tc.host[1])
			request.Header.Set(tc.port[0], tc.port[1])
			normalizeTrustedProxyRequest(request)
			if request.Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", request.Host, tc.wantHost)
			}
		})
	}
}

func TestNormalizeTrustedProxyPortWithoutForwardedHost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://internal:8080/", nil)
	request.Header.Set("X-Forwarded-Port", "8443")
	normalizeTrustedProxyRequest(request)
	if request.Host != "internal:8443" {
		t.Errorf("Host = %q, want internal:8443", request.Host)
	}
}

func TestNormalizeTrustedProxyProtocolPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		header map[string]string
		tls    bool
		want   string
	}{
		{name: "forwarded proto", header: map[string]string{"X-Forwarded-Proto": "HTTPS, http", "X-Forwarded-Ssl": "off"}, want: "https"},
		{name: "ssl off precedes url scheme", header: map[string]string{"X-Forwarded-Ssl": "off", "X-Url-Scheme": "https"}, want: "http"},
		{name: "url scheme", header: map[string]string{"X-Url-Scheme": "https"}, want: "https"},
		{name: "frontend https", header: map[string]string{"Front-End-Https": "on"}, want: "https"},
		{name: "tls fallback", tls: true, want: "https"},
		{name: "default", want: "http"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
			for name, value := range tc.header {
				request.Header.Set(name, value)
			}
			if tc.tls {
				request.TLS = &tls.ConnectionState{}
			}
			normalizeTrustedProxyRequest(request)
			if got := request.Header.Get("X-Forwarded-Proto"); got != tc.want {
				t.Errorf("X-Forwarded-Proto = %q, want %q", got, tc.want)
			}
		})
	}
}
