package urlvars

import (
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

var devOnlyTLDs = map[string]bool{
	"localhost": true, "test": true, "example": true, "invalid": true,
	"local": true, "lan": true, "internal": true, "home": true,
	"localdomain": true, "home.arpa": true, "intranet": true,
	"corp": true, "private": true,
}

// IsValidHost checks a hostname against production or development FQDN rules.
func IsValidHost(host string, devMode bool, projectName string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	if lower == "" || net.ParseIP(strings.Trim(lower, "[]")) != nil {
		return false
	}
	if lower == "localhost" {
		return devMode
	}
	if !strings.Contains(lower, ".") {
		return false
	}
	for _, suffix := range []string{".onion", ".i2p", ".exit"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	if projectName != "" && strings.HasSuffix(lower, "."+strings.ToLower(projectName)) {
		return devMode
	}
	suffix, icann := publicsuffix.PublicSuffix(lower)
	if devOnlyTLDs[suffix] {
		return devMode
	}
	if !devMode && !icann {
		return false
	}
	_, err := publicsuffix.EffectiveTLDPlusOne(lower)
	return err == nil
}

// IsValidSSLHost permits only public production hostnames suitable for TLS issuance.
func IsValidSSLHost(host string) bool {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(host)), ".onion") {
		return false
	}
	return IsValidHost(host, false, "")
}

// IsPublicIP reports whether ip is a public interface address suitable for
// the final automatic URL fallback. Hostname validation still rejects IPs in
// DOMAIN, proxy headers, and TLS names.
func IsPublicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
