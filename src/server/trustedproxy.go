package server

import (
	"log"
	"net"
	"net/netip"

	"github.com/apimgr/airports/src/config"
)

// defaultTrustedProxyCIDRs are always trusted, per AI.md PART 12 "Trusted
// Proxies" — loopback, RFC1918, RFC4193, and link-local ranges never need
// explicit configuration.
var defaultTrustedProxyCIDRs = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
	"169.254.0.0/16",
	"fe80::/10",
}

// resolveTrustedProxyCIDRs builds the full list of trusted-proxy CIDR
// prefixes for middleware.ClientIPFromXFF: the always-trusted private
// ranges plus cfg.Server.TrustedProxies.Additional. Additional entries may
// be a bare IP, a CIDR, or a DNS name (resolved to its A/AAAA addresses).
// Invalid or unresolvable entries are logged and skipped rather than
// causing startup failure.
func resolveTrustedProxyCIDRs(cfg *config.Config) []string {
	cidrs := make([]string, 0, len(defaultTrustedProxyCIDRs)+len(cfg.Server.TrustedProxies.Additional))
	cidrs = append(cidrs, defaultTrustedProxyCIDRs...)

	for _, entry := range cfg.Server.TrustedProxies.Additional {
		if entry == "" {
			continue
		}

		// Already a CIDR
		if _, _, err := net.ParseCIDR(entry); err == nil {
			cidrs = append(cidrs, entry)
			continue
		}

		// A bare IP - normalize to a single-address CIDR
		if addr, err := netip.ParseAddr(entry); err == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			cidrs = append(cidrs, netip.PrefixFrom(addr, bits).String())
			continue
		}

		// Otherwise treat as a DNS name and resolve to its addresses
		ips, err := net.LookupHost(entry)
		if err != nil {
			log.Printf("trusted_proxies.additional: could not resolve %q: %v", entry, err)
			continue
		}
		for _, ip := range ips {
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				continue
			}
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			cidrs = append(cidrs, netip.PrefixFrom(addr, bits).String())
		}
	}

	return cidrs
}
