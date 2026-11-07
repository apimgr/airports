package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/common/urlvars"
	"github.com/apimgr/airports/src/config"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pires/go-proxyproto"
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

const trustedProxyDNSRefreshInterval = 5 * time.Minute

type originalTCPPeerContextKey struct{}
type torCircuitContextKey struct{}

// TorCircuitID returns the circuit token attached by the dedicated Tor
// listener's PROXY-protocol parser. Requests without that trusted listener
// metadata are never considered Tor requests based on Host alone.
func TorCircuitID(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(torCircuitContextKey{}).(string)
	return id
}

// TorClientIdentity formats the circuit token for logs and per-circuit
// rate-limit keys. It returns the literal sentinel when circuit export is
// unavailable.
func TorClientIdentity(r *http.Request) string {
	if id := TorCircuitID(r); id != "" {
		return "tor:" + id
	}
	return "tor"
}

// TorConnContext extracts validated circuit metadata from a PROXY-protocol
// connection accepted by the dedicated loopback Tor listener. The listener
// rejects connections without a valid fc00::/8 source address before any
// request handler runs.
func (s *Server) TorConnContext(ctx context.Context, conn net.Conn) context.Context {
	proxyConn, ok := conn.(*proxyproto.Conn)
	if !ok {
		return ctx
	}
	header := proxyConn.ProxyHeader()
	if header == nil || header.Version != 1 {
		return ctx
	}
	source, ok := header.SourceAddr.(*net.TCPAddr)
	if !ok || source.IP == nil || source.IP.To4() != nil {
		return ctx
	}
	ip := source.IP.To16()
	if ip == nil || ip[0] != 0xfc {
		return ctx
	}
	return context.WithValue(ctx, torCircuitContextKey{}, ip.String())
}

// OriginalTCPPeer returns the socket peer address captured before proxy
// normalization. If the request has not passed through trusted-proxy
// middleware, it returns the current RemoteAddr as the best available value.
func OriginalTCPPeer(r *http.Request) string {
	if r == nil {
		return ""
	}
	if TorCircuitID(r) != "" {
		return TorClientIdentity(r)
	}
	if peer, ok := r.Context().Value(originalTCPPeerContextKey{}).(string); ok {
		return peer
	}
	return r.RemoteAddr
}

type trustedProxyCIDRCache struct {
	config func() *config.Config
	lookup func(string) ([]string, error)
	now    func() time.Time

	mu        sync.Mutex
	configKey string
	refreshed time.Time
	cidrs     []string
}

func newTrustedProxyCIDRCache(configProvider func() *config.Config) *trustedProxyCIDRCache {
	return &trustedProxyCIDRCache{config: configProvider, lookup: net.LookupHost, now: time.Now}
}

func (c *trustedProxyCIDRCache) CIDRs() []string {
	cfg := c.config()
	configKey := trustedProxyConfigKey(cfg)
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cidrs == nil || c.configKey != configKey || now.Sub(c.refreshed) >= trustedProxyDNSRefreshInterval {
		c.cidrs = resolveTrustedProxyCIDRsWithLookup(cfg, c.lookup)
		c.configKey = configKey
		c.refreshed = now
	}
	return append([]string(nil), c.cidrs...)
}

func trustedProxyConfigKey(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Server.Address) + "\x00" + strings.Join(cfg.Server.TrustedProxies.Additional, "\x00")
}

// TrustedClientIPFromHeaders resolves the client address only through the
// trusted-proxy gate. It normalizes supported vendor headers into
// X-Forwarded-For so chi can walk the proxy chain using the configured CIDRs.
func TrustedClientIPFromHeaders(trustedCIDRs ...string) func(http.Handler) http.Handler {
	fixedCIDRs := append([]string(nil), trustedCIDRs...)
	return trustedClientIPFromCIDRSource(func() []string { return fixedCIDRs })
}

func trustedClientIPFromCIDRSource(cidrsForRequest func() []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := urlvars.WithOriginalRequestHost(r.Context(), r.Host)
			if TorCircuitID(r) != "" {
				ctx = context.WithValue(ctx, originalTCPPeerContextKey{}, TorClientIdentity(r))
				request := r.Clone(ctx)
				request.Header = r.Header.Clone()
				dropProxyHeaders(request.Header)
				next.ServeHTTP(w, request)
				return
			}
			if _, captured := ctx.Value(originalTCPPeerContextKey{}).(string); !captured {
				ctx = context.WithValue(ctx, originalTCPPeerContextKey{}, r.RemoteAddr)
			}
			r = r.WithContext(ctx)
			trustedCIDRs := cidrsForRequest()
			resolveXFF := middleware.ClientIPFromXFF(trustedCIDRs...)
			resolved := resolveXFF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			}))
			request := r.Clone(r.Context())
			request.Header = r.Header.Clone()
			if isTrustedProxyPeer(r.RemoteAddr, trustedCIDRs) {
				normalizeTrustedClientIPHeaders(request.Header)
				normalizeTrustedProxyRequest(request)
			} else {
				dropProxyHeaders(request.Header)
			}
			resolved.ServeHTTP(w, request)
		})
	}
}

func isTrustedProxyPeer(remoteAddr string, trustedCIDRs []string) bool {
	peer := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		peer = host
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return false
	}
	for _, cidr := range trustedCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err == nil && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func dropProxyHeaders(header http.Header) {
	for _, name := range []string{
		"CF-Connecting-IP", "True-Client-IP", "X-Real-IP", "X-Forwarded-For", "X-Client-IP",
		"X-Forwarded-Host", "X-Real-Host", "X-Original-Host", "X-Forwarded-Proto", "X-Forwarded-Ssl",
		"X-Url-Scheme", "Front-End-Https", "X-Forwarded-Port", "X-Forwarded-Prefix", "X-Forwarded-Path",
		"X-Real-Port", "X-Script-Name",
	} {
		header.Del(name)
	}
}

func normalizeTrustedProxyRequest(r *http.Request) {
	forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host"), r.Header.Get("X-Real-Host"), r.Header.Get("X-Original-Host"))
	port := firstForwardedPort(r.Header.Get("X-Forwarded-Port"), r.Header.Get("X-Real-Port"))
	host := forwardedHost
	if host == "" && port != "" {
		host = r.Host
	}
	if host != "" {
		if port != "" {
			if portedHost := hostWithPort(host, port); portedHost != "" {
				host = portedHost
			}
		}
		if forwardedHost != "" || port != "" {
			r.Host = host
		}
	}

	proto := forwardedProto(r)
	if proto != "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}
}

func forwardedProto(r *http.Request) string {
	return urlvars.ResolveProto(r, true)
}

func firstForwardedPort(values ...string) string {
	for _, value := range values {
		value = firstForwardedValue(value)
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 && port <= 65535 {
			return strconv.Itoa(port)
		}
	}
	return ""
}

func hostWithPort(host, port string) string {
	host = strings.TrimSpace(host)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	} else if _, err := netip.ParseAddr(host); err != nil && strings.Contains(host, ":") {
		return ""
	}
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, port)
}

func firstForwardedValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if comma := strings.IndexByte(value, ','); comma >= 0 {
			value = strings.TrimSpace(value[:comma])
		}
		if value != "" {
			return value
		}
	}
	return ""
}

// normalizeTrustedClientIPHeaders chooses the first supported proxy header
// in the AI.md precedence order. ClientIPFromXFF still applies the trust gate
// to the original TCP peer before it sets a client IP in request context.
func normalizeTrustedClientIPHeaders(header http.Header) {
	for _, name := range []string{
		"CF-Connecting-IP",
		"True-Client-IP",
		"X-Real-IP",
		"X-Forwarded-For",
		"X-Client-IP",
	} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			header.Set("X-Forwarded-For", value)
			return
		}
	}
}

// resolveTrustedProxyCIDRs builds the full list of trusted-proxy CIDR
// prefixes for middleware.ClientIPFromXFF: the always-trusted private
// ranges plus cfg.Server.TrustedProxies.Additional. Additional entries may
// be a bare IP, a CIDR, or a DNS name (resolved to its A/AAAA addresses).
// Invalid or unresolvable entries are logged and skipped rather than
// causing startup failure.
func resolveTrustedProxyCIDRs(cfg *config.Config) []string {
	return resolveTrustedProxyCIDRsWithLookup(cfg, net.LookupHost)
}

func resolveTrustedProxyCIDRsWithLookup(cfg *config.Config, lookup func(string) ([]string, error)) []string {
	if cfg == nil {
		cfg = &config.Config{}
	}
	cidrs := make([]string, 0, len(defaultTrustedProxyCIDRs)+len(cfg.Server.TrustedProxies.Additional))
	cidrs = append(cidrs, defaultTrustedProxyCIDRs...)
	if addr, err := netip.ParseAddr(strings.TrimSpace(cfg.Server.Address)); err == nil && addr.Is4() {
		cidrs = append(cidrs, netip.PrefixFrom(addr, 24).Masked().String())
	}

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
		ips, err := lookup(entry)
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
