package urlvars

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func testOptions() Options {
	return Options{
		FQDN: "api.example.net", ListenPort: "8081", ProjectName: "airports",
		Detection:   config.URLDetectionConfig{Learning: true, MinSamples: 3, SampleWindow: "5m", LiveReload: true},
		TrustedPeer: func(r *http.Request) bool { return r.RemoteAddr == "10.0.0.1:9000" },
	}
}

func TestGetURLVarsUsesTrustedProxyPriorityAndPortRules(t *testing.T) {
	options := testOptions()
	manager := NewManager(options)
	req := httptest.NewRequest(http.MethodGet, "http://internal:8080/path", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	req.Header.Set("X-Forwarded-Host", "api.example.com:8443")
	req.Header.Set("X-Forwarded-Proto", "https, http")
	req.Header.Set("X-Forwarded-Port", "9443")
	proto, fqdn, port := manager.GetURLVars(req)
	if proto != "https" || fqdn != "api.example.com" || port != "9443" {
		t.Fatalf("GetURLVars() = (%q,%q,%q), want (https,api.example.com,9443)", proto, fqdn, port)
	}
	req.Header.Set("X-Forwarded-Port", "443")
	_, _, port = manager.GetURLVars(req)
	if port != "" {
		t.Errorf("default forwarded port = %q, want empty", port)
	}
}

func TestGetURLVarsIgnoresUntrustedOrInvalidProxyHosts(t *testing.T) {
	manager := NewManager(testOptions())
	req := httptest.NewRequest(http.MethodGet, "http://internal:8080/path", nil)
	req.RemoteAddr = "198.51.100.9:9000"
	req.Header.Set("X-Forwarded-Host", "attacker.invalid")
	req.Header.Set("X-Forwarded-Proto", "https")
	proto, fqdn, port := manager.GetURLVars(req)
	if proto != "http" || fqdn != "api.example.net" || port != "8081" {
		t.Fatalf("untrusted URL vars = (%q,%q,%q), want (http,api.example.net,8081)", proto, fqdn, port)
	}
	req.RemoteAddr = "10.0.0.1:9000"
	req.Header.Set("X-Forwarded-Host", "bad host")
	_, fqdn, _ = manager.GetURLVars(req)
	if fqdn != "api.example.net" {
		t.Errorf("invalid trusted host resolved to %q, want configured fallback", fqdn)
	}
}

func TestTorHostOverridesTrustedProxyHeaders(t *testing.T) {
	options := testOptions()
	options.OnionAddress = "abcd1234.onion"
	manager := NewManager(options)
	req := httptest.NewRequest(http.MethodGet, "http://abcd1234.onion/", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	req = req.WithContext(WithOriginalRequestHost(req.Context(), req.Host))
	req.Host = "clearnet.example.com"
	req.Header.Set("X-Forwarded-Host", "clearnet.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	proto, fqdn, port := manager.GetURLVars(req)
	if proto != "http" || fqdn != options.OnionAddress || port != "" {
		t.Fatalf("Tor URL vars = (%q,%q,%q), want (http,%s,empty)", proto, fqdn, port, options.OnionAddress)
	}
}

func TestI2PHostIsTrustedOnlyWhenConfigured(t *testing.T) {
	options := testOptions()
	options.I2PAddress = "exampleb32.b32.i2p"
	manager := NewManager(options)
	req := httptest.NewRequest(http.MethodGet, "http://exampleb32.b32.i2p/", nil)
	req = req.WithContext(WithOriginalRequestHost(req.Context(), req.Host))
	req.Host = "clearnet.example.net"
	req.Header.Set("X-Forwarded-Host", "clearnet.example.net")
	proto, host, port := manager.GetURLVars(req)
	if proto != "http" || host != options.I2PAddress || port != "" {
		t.Fatalf("configured I2P URL vars = (%q,%q,%q), want (http,%s,empty)", proto, host, port, options.I2PAddress)
	}
	options.I2PAddress = ""
	manager.UpdateOptions(options)
	_, host, _ = manager.GetURLVars(req)
	if host == "exampleb32.b32.i2p" {
		t.Fatal("unconfigured I2P host was trusted")
	}
}

func TestDomainLearningInfersRegistrableAndWildcardDomains(t *testing.T) {
	manager := NewManager(testOptions())
	for _, host := range []string{"example.com", "www.example.com", "api.example.com"} {
		req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
		req.RemoteAddr = "10.0.0.1:9000"
		req.Header.Set("X-Forwarded-Host", host)
		manager.GetURLVars(req)
	}
	if got := manager.GetBaseDomain(); got != "example.com" {
		t.Errorf("GetBaseDomain() = %q, want example.com", got)
	}
	if got := manager.GetWildcardDomain(); got != "*.example.com" {
		t.Errorf("GetWildcardDomain() = %q, want *.example.com", got)
	}
}

func TestDomainLearningNeedsMinimumDistinctSamplesAndExpires(t *testing.T) {
	options := testOptions()
	options.Detection.MinSamples = 3
	options.Detection.SampleWindow = "1s"
	manager := NewManager(options)
	now := time.Now()
	manager.observeDomain("www.example.com", now)
	manager.observeDomain("api.example.com", now)
	if got := manager.GetWildcardDomain(); got != "" {
		t.Fatalf("wildcard after two samples = %q, want empty", got)
	}
	manager.observeDomain("example.com", now.Add(-2*time.Second))
	if got := manager.GetWildcardDomain(); got != "" {
		t.Errorf("expired samples yielded wildcard %q, want empty", got)
	}
}

func TestDomainEnvironmentSeedsBaseAndWildcard(t *testing.T) {
	options := testOptions()
	options.DomainEnv = "example.co.uk,www.example.co.uk,api.example.co.uk"
	manager := NewManager(options)
	if got := manager.GetBaseDomain(); got != "example.co.uk" {
		t.Errorf("GetBaseDomain() = %q, want example.co.uk", got)
	}
	if got := manager.GetWildcardDomain(); got != "*.example.co.uk" {
		t.Errorf("GetWildcardDomain() = %q, want *.example.co.uk", got)
	}
}

func TestBuildURLUsesResolvedVarsAndBasePath(t *testing.T) {
	options := testOptions()
	options.BaseURL = func(*http.Request) string { return "/edge" }
	manager := NewManager(options)
	req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	req.Header.Set("X-Forwarded-Host", "api.example.com:443")
	req.Header.Set("X-Forwarded-Proto", "https")
	if got, want := manager.BuildURL(req, "/server/healthz"), "https://api.example.com/edge/server/healthz"; got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
	}
}

func TestListenPortFollowsResolvedProtocol(t *testing.T) {
	options := testOptions()
	options.ListenPort = "8080,8443"
	manager := NewManager(options)
	req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
	req.RemoteAddr = "198.51.100.9:9000"
	proto, _, port := manager.GetURLVars(req)
	if proto != "http" || port != "8080" {
		t.Fatalf("HTTP URL vars = (%q,%q), want (http,8080)", proto, port)
	}
	req = httptest.NewRequest(http.MethodGet, "https://internal/", nil)
	proto, _, port = manager.GetURLVars(req)
	if proto != "https" || port != "8443" {
		t.Fatalf("HTTPS URL vars = (%q,%q), want (https,8443)", proto, port)
	}
}

func TestUpdateOptionsRefreshesLiveResolverSettings(t *testing.T) {
	manager := NewManager(testOptions())
	options := testOptions()
	options.FQDN = "new.example.org"
	options.ListenPort = "9090"
	options.Detection.LiveReload = false
	manager.UpdateOptions(options)
	req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	req.Header.Set("X-Forwarded-Host", "proxy.example.net")
	_, fqdn, port := manager.GetURLVars(req)
	if fqdn != "new.example.org" || port != "9090" {
		t.Fatalf("updated URL vars = (%q,%q), want (new.example.org,9090)", fqdn, port)
	}
}

func TestResolveProtoPriorityAndUntrustedTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
	req.Header.Set("X-Forwarded-Proto", "invalid")
	req.Header.Set("X-Forwarded-Ssl", "on")
	req.Header.Set("X-Url-Scheme", "http")
	if got := ResolveProto(req, true); got != "https" {
		t.Errorf("ResolveProto(trusted) = %q, want https", got)
	}
	req.TLS = &tls.ConnectionState{}
	if got := ResolveProto(req, false); got != "https" {
		t.Errorf("ResolveProto(untrusted TLS) = %q, want https", got)
	}
	req.TLS = nil
	if got := ResolveProto(req, false); got != "http" {
		t.Errorf("ResolveProto(untrusted cleartext) = %q, want http", got)
	}
}

func TestHostValidationSharedRules(t *testing.T) {
	if !IsValidHost("api.example.co.uk", false, "airports") || IsValidHost("192.168.1.1", true, "airports") || IsValidHost("app.test", false, "airports") {
		t.Fatal("shared host validation did not enforce the documented production/development rules")
	}
	if IsValidSSLHost("node.onion") || !IsValidSSLHost("api.example.com") {
		t.Fatal("SSL host validation did not reject overlay domains or accept public domains")
	}
}

func TestPublicIPIsAllowedOnlyAsAutomaticFallback(t *testing.T) {
	if IsValidHost("203.0.113.10", false, "airports") {
		t.Fatal("IsValidHost accepted an IP address, want hostname validation to reject it")
	}
	if !IsPublicIP(net.ParseIP("203.0.113.10")) {
		t.Fatal("IsPublicIP rejected a global-unicast non-private address")
	}
	options := testOptions()
	options.FQDN = "203.0.113.10"
	options.DomainEnv = ""
	manager := NewManager(options)
	_, host, _ := manager.GetURLVars(httptest.NewRequest(http.MethodGet, "http://internal/", nil))
	if host != "203.0.113.10" {
		t.Fatalf("automatic fallback host = %q, want public interface IP", host)
	}
	options.FQDN = "2001:db8::10"
	manager = NewManager(options)
	if got, want := manager.BuildURL(nil, "/health"), "http://[2001:db8::10]/health"; got != want {
		t.Fatalf("IPv6 BuildURL() = %q, want %q", got, want)
	}
}
