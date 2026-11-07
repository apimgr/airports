package urlvars

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/config"
	"golang.org/x/net/publicsuffix"
)

const maxDomainSamples = 2048

type originalRequestHostKey struct{}

// WithOriginalRequestHost stores the HTTP Host value before trusted-proxy
// normalization so overlay-host checks retain priority over forwarded hosts.
func WithOriginalRequestHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, originalRequestHostKey{}, host)
}

// OriginalRequestHost returns the captured pre-normalization Host value.
func OriginalRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if host, ok := r.Context().Value(originalRequestHostKey{}).(string); ok && host != "" {
		return host
	}
	return r.Host
}

// Options supplies URL sources and trust checks to a resolver.
type Options struct {
	FQDN         string
	DomainEnv    string
	Hostname     string
	HostEnv      string
	ListenPort   string
	OnionAddress string
	I2PAddress   string
	ProjectName  string
	Development  bool
	Detection    config.URLDetectionConfig
	TrustedPeer  func(*http.Request) bool
	BaseURL      func(*http.Request) string
}

type domainSample struct {
	host string
	at   time.Time
}

// Manager resolves request URL variables and learns domains only from trusted
// reverse-proxy observations.
type Manager struct {
	mu                  sync.RWMutex
	options             Options
	samples             []domainSample
	baseDomain          string
	wildcard            string
	primaryHost         string
	lastProto           string
	lastHost            string
	lastConflictWarning time.Time
}

// NewManager creates a resolver with the configured URL-detection policy.
func NewManager(options Options) *Manager {
	if options.Hostname == "" {
		options.Hostname, _ = os.Hostname()
	}
	if options.HostEnv == "" {
		options.HostEnv = os.Getenv("HOSTNAME")
	}
	if options.Detection.MinSamples <= 0 {
		options.Detection.MinSamples = 3
	}
	if options.Detection.SampleWindow == "" {
		options.Detection.SampleWindow = "5m"
	}
	if _, err := time.ParseDuration(options.Detection.SampleWindow); err != nil {
		options.Detection.SampleWindow = "5m"
	}
	m := &Manager{options: options}
	m.seedConfiguredDomains()
	return m
}

// UpdateOptions replaces runtime URL settings while preserving observations
// that remain within the configured sample window.
func (m *Manager) UpdateOptions(options Options) {
	m.mu.Lock()
	if options.Hostname == "" {
		options.Hostname = m.options.Hostname
	}
	if options.HostEnv == "" {
		options.HostEnv = m.options.HostEnv
	}
	if options.DomainEnv == "" {
		options.DomainEnv = m.options.DomainEnv
	}
	if options.Detection.MinSamples <= 0 {
		options.Detection.MinSamples = 3
	}
	if _, err := time.ParseDuration(options.Detection.SampleWindow); err != nil {
		options.Detection.SampleWindow = "5m"
	}
	m.options = options
	m.pruneSamplesLocked(time.Now())
	m.recalculateLocked()
	m.mu.Unlock()
}

func (m *Manager) seedConfiguredDomains() {
	valid := m.configuredDomains()
	if len(valid) == 0 {
		return
	}
	m.baseDomain, _ = baseDomain(valid[0])
	m.primaryHost = valid[0]
	if len(valid) > 1 {
		for _, host := range valid[1:] {
			base, ok := baseDomain(host)
			if !ok || base != m.baseDomain {
				return
			}
		}
		m.wildcard = "*." + m.baseDomain
	}
}

func (m *Manager) configuredDomains() []string {
	m.mu.RLock()
	options := m.options
	m.mu.RUnlock()
	return configuredDomains(options)
}

func configuredDomains(options Options) []string {
	values := strings.Split(options.DomainEnv, ",")
	if strings.TrimSpace(options.DomainEnv) == "" {
		values = []string{options.FQDN}
	}
	seen := make(map[string]bool)
	var result []string
	for _, value := range values {
		host, _, ok := parseHostPort(strings.TrimSpace(value))
		if !ok || !IsValidHost(host, options.Development, options.ProjectName) || isOverlayHost(host) {
			continue
		}
		host = strings.ToLower(host)
		if !seen[host] {
			seen[host] = true
			result = append(result, host)
		}
	}
	if len(result) == 0 && options.FQDN != "" {
		host, _, ok := parseHostPort(options.FQDN)
		ip := net.ParseIP(host)
		if ok && ((ip != nil && IsPublicIP(ip)) || IsValidHost(host, options.Development, options.ProjectName)) && !isOverlayHost(host) {
			result = append(result, strings.ToLower(host))
		}
	}
	return result
}

// GetURLVars resolves protocol, FQDN, and non-default port for a request.
func (m *Manager) GetURLVars(r *http.Request) (proto, fqdn, port string) {
	m.mu.RLock()
	options := m.options
	m.mu.RUnlock()
	if r == nil {
		m.mu.RLock()
		primary := m.primaryHost
		m.mu.RUnlock()
		if primary == "" {
			primary = configuredPrimary(options)
		}
		return "http", primary, ""
	}
	requestHost, requestPort, _ := parseHostPort(r.Host)
	originalHost, _, _ := parseHostPort(OriginalRequestHost(r))
	if onion := normalizeHost(options.OnionAddress); onion != "" && strings.EqualFold(originalHost, onion) {
		return "http", onion, ""
	}
	if i2p := normalizeHost(options.I2PAddress); i2p != "" && strings.EqualFold(originalHost, i2p) {
		return "http", i2p, ""
	}
	trusted := options.TrustedPeer != nil && options.TrustedPeer(r)
	proto = requestProto(r, trusted)
	proxyHost, proxyPort := "", ""
	if trusted {
		for _, name := range []string{"X-Forwarded-Host", "X-Real-Host", "X-Original-Host"} {
			if value := firstHeaderValue(r.Header.Get(name)); value != "" {
				host, hostPort, ok := parseHostPort(value)
				if ok && IsValidHost(host, options.Development, options.ProjectName) && validObservedHost(host, options) {
					proxyHost, proxyPort = strings.ToLower(host), hostPort
					break
				}
			}
		}
	}
	if proxyHost != "" {
		// A single configured DOMAIN remains eligible for subdomain learning,
		// as specified; an explicit multi-domain list already defines the
		// domain set and needs no further learning.
		if options.Detection.Learning && (options.DomainEnv == "" || len(configuredDomains(options)) == 1) {
			m.observeDomain(proxyHost, time.Now())
		}
		if options.Detection.LiveReload {
			fqdn = proxyHost
		} else {
			fqdn = configuredPrimary(options)
		}
	}
	if fqdn == "" {
		m.mu.RLock()
		fqdn = m.primaryHost
		m.mu.RUnlock()
	}
	if fqdn == "" {
		fqdn = configuredPrimary(options)
	}
	if fqdn == "" {
		for _, candidate := range []string{options.Hostname, options.HostEnv} {
			candidate = strings.ToLower(strings.TrimSpace(candidate))
			if candidate != "" && IsValidHost(candidate, options.Development, options.ProjectName) {
				fqdn = candidate
				break
			}
		}
	}
	if fqdn == "" {
		fqdn = "localhost"
	}
	port = firstHeaderValue(r.Header.Get("X-Forwarded-Port"))
	if !trusted {
		port = ""
	}
	if !validPort(port) {
		port = proxyPort
	}
	if port == "" && trusted {
		port = requestPort
	}
	if port == "" && !trusted && strings.EqualFold(requestHost, fqdn) {
		port = requestPort
	}
	if port == "" {
		port = listenPort(options.ListenPort, proto)
	}
	if port == "80" || port == "443" || !validPort(port) {
		port = ""
	}
	if trusted && proxyHost != "" && options.Detection.LiveReload && options.Detection.LogChanges {
		m.observeResolution(proto, proxyHost)
	}
	return proto, fqdn, port
}

// BuildURL builds an absolute URL using the current request and configured base path.
func (m *Manager) BuildURL(r *http.Request, path string) string {
	proto, fqdn, port := m.GetURLVars(r)
	base := ""
	m.mu.RLock()
	baseURL := m.options.BaseURL
	m.mu.RUnlock()
	if r != nil && baseURL != nil {
		base = strings.TrimSuffix(baseURL(r), "/")
	}
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	host := fqdn
	if port != "" {
		host = net.JoinHostPort(fqdn, port)
	} else if ip := net.ParseIP(fqdn); ip != nil && ip.To4() == nil {
		host = "[" + fqdn + "]"
	}
	return proto + "://" + host + base + path
}

// GetBaseDomain returns the configured or inferred registrable domain.
func (m *Manager) GetBaseDomain() string {
	m.mu.Lock()
	m.pruneSamplesLocked(time.Now())
	m.recalculateLocked()
	base := m.baseDomain
	m.mu.Unlock()
	return base
}

// GetWildcardDomain returns the inferred wildcard domain, if available.
func (m *Manager) GetWildcardDomain() string {
	m.mu.Lock()
	m.pruneSamplesLocked(time.Now())
	m.recalculateLocked()
	wildcard := m.wildcard
	m.mu.Unlock()
	return wildcard
}

func (m *Manager) observeDomain(host string, now time.Time) {
	m.mu.Lock()
	beforeBase, beforeWildcard, beforePrimary := m.baseDomain, m.wildcard, m.primaryHost
	m.pruneSamplesLocked(now)
	m.samples = append(m.samples, domainSample{host: host, at: now})
	if len(m.samples) > maxDomainSamples {
		copy(m.samples, m.samples[len(m.samples)-maxDomainSamples:])
		m.samples = m.samples[:maxDomainSamples]
	}
	m.recalculateLocked()
	changed := beforeBase != m.baseDomain || beforeWildcard != m.wildcard || beforePrimary != m.primaryHost
	logChanges := m.options.Detection.LogChanges
	base, wildcard, primary := m.baseDomain, m.wildcard, m.primaryHost
	m.mu.Unlock()
	if changed && logChanges {
		log.Printf("URL domain learning updated: fqdn=%q base_domain=%q wildcard=%q", primary, base, wildcard)
	}
}

func (m *Manager) pruneSamplesLocked(now time.Time) {
	window, err := time.ParseDuration(m.options.Detection.SampleWindow)
	if err != nil || window <= 0 {
		window = 5 * time.Minute
	}
	cutoff := now.Add(-window)
	kept := m.samples[:0]
	for _, sample := range m.samples {
		if !sample.at.Before(cutoff) {
			kept = append(kept, sample)
		}
	}
	m.samples = kept
}

func (m *Manager) recalculateLocked() {
	configured := configuredDomains(m.options)
	explicitBase, explicitWildcard := "", ""
	if m.options.DomainEnv != "" && len(configured) > 0 {
		explicitBase, _ = baseDomain(configured[0])
		if len(configured) > 1 {
			sameBase := true
			for _, host := range configured[1:] {
				base, ok := baseDomain(host)
				if !ok || base != explicitBase {
					sameBase = false
					break
				}
			}
			if sameBase && explicitBase != "" {
				explicitWildcard = "*." + explicitBase
			}
		}
	}
	if len(m.samples) < m.options.Detection.MinSamples {
		if explicitBase == "" && len(configured) > 0 {
			explicitBase, _ = baseDomain(configured[0])
		}
		m.baseDomain, m.wildcard = explicitBase, explicitWildcard
		m.primaryHost = ""
		if len(configured) > 0 {
			m.primaryHost = configured[0]
		}
		return
	}
	baseCounts := make(map[string]int)
	distinct := make(map[string]map[string]bool)
	for _, sample := range m.samples {
		base, ok := baseDomain(sample.host)
		if !ok {
			continue
		}
		baseCounts[base]++
		if distinct[base] == nil {
			distinct[base] = make(map[string]bool)
		}
		distinct[base][sample.host] = true
	}
	type baseCandidate struct {
		name   string
		count  int
		latest time.Time
	}
	var bases []baseCandidate
	for base, count := range baseCounts {
		latest := time.Time{}
		for _, sample := range m.samples {
			if sampleBase, ok := baseDomain(sample.host); ok && sampleBase == base && sample.at.After(latest) {
				latest = sample.at
			}
		}
		bases = append(bases, baseCandidate{name: base, count: count, latest: latest})
	}
	sort.Slice(bases, func(i, j int) bool {
		if bases[i].count == bases[j].count {
			if bases[i].latest.Equal(bases[j].latest) {
				return bases[i].name < bases[j].name
			}
			return bases[i].latest.After(bases[j].latest)
		}
		return bases[i].count > bases[j].count
	})
	if len(bases) == 0 {
		m.baseDomain, m.wildcard = explicitBase, explicitWildcard
		m.primaryHost = ""
		if len(configured) > 0 {
			m.primaryHost = configured[0]
		}
		return
	}
	base := bases[0].name
	if len(bases) > 1 && m.options.Detection.LogChanges && time.Since(m.lastConflictWarning) >= time.Minute {
		m.lastConflictWarning = time.Now()
		log.Printf("Warning: URL domain learning observed conflicting domains; using %q", base)
	}
	if explicitBase != "" {
		base = explicitBase
	}
	m.baseDomain, m.wildcard = base, explicitWildcard
	if explicitBase != "" && len(configured) > 0 {
		m.primaryHost = configured[0]
	} else if distinct[base][base] {
		m.primaryHost = base
	} else {
		hosts := make([]string, 0, len(distinct[base]))
		for host := range distinct[base] {
			hosts = append(hosts, host)
		}
		sort.Slice(hosts, func(i, j int) bool {
			if strings.HasPrefix(hosts[i], "www.") != strings.HasPrefix(hosts[j], "www.") {
				return !strings.HasPrefix(hosts[i], "www.")
			}
			return hosts[i] < hosts[j]
		})
		if len(hosts) > 0 {
			m.primaryHost = hosts[0]
		}
	}
	if len(distinct[base]) >= 2 && baseCounts[base] >= m.options.Detection.MinSamples {
		for host := range distinct[base] {
			if host != base && strings.HasSuffix(host, "."+base) {
				m.wildcard = "*." + base
				break
			}
		}
	}
}

func (m *Manager) observeResolution(proto, host string) {
	m.mu.Lock()
	changed := m.lastProto != "" && (m.lastProto != proto || m.lastHost != host)
	m.lastProto, m.lastHost = proto, host
	m.mu.Unlock()
	if changed {
		log.Printf("URL detection changed: proto=%q host=%q", proto, host)
	}
}

func configuredPrimary(options Options) string {
	for _, candidate := range strings.Split(options.DomainEnv, ",") {
		host, _, ok := parseHostPort(strings.TrimSpace(candidate))
		if ok && IsValidHost(host, options.Development, options.ProjectName) && !isOverlayHost(host) {
			return strings.ToLower(host)
		}
	}
	if host, _, ok := parseHostPort(options.FQDN); ok && !isOverlayHost(host) {
		ip := net.ParseIP(host)
		if (ip != nil && IsPublicIP(ip)) || IsValidHost(host, options.Development, options.ProjectName) {
			return strings.ToLower(host)
		}
	}
	return ""
}

func validObservedHost(host string, options Options) bool {
	if !isOverlayHost(host) {
		return true
	}
	return (options.OnionAddress != "" && strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(options.OnionAddress, "."))) ||
		(normalizeHost(options.I2PAddress) != "" && strings.EqualFold(strings.TrimSuffix(host, "."), normalizeHost(options.I2PAddress)))
}

func baseDomain(host string) (string, bool) {
	if isOverlayHost(host) || net.ParseIP(host) != nil {
		return "", false
	}
	base, err := publicsuffix.EffectiveTLDPlusOne(host)
	return base, err == nil
}

func isOverlayHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(host, ".onion") || strings.HasSuffix(host, ".i2p") || strings.HasSuffix(host, ".exit")
}

func normalizeHost(host string) string {
	host, _, ok := parseHostPort(host)
	if !ok {
		return ""
	}
	return strings.ToLower(host)
}

func parseHostPort(value string) (host, port string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/\\@?#\r\n\t ") {
		return "", "", false
	}
	if ip := net.ParseIP(strings.Trim(value, "[]")); ip != nil && ip.To4() == nil {
		return strings.Trim(value, "[]"), "", true
	}
	if parsedHost, parsedPort, err := net.SplitHostPort(value); err == nil {
		if !validPort(parsedPort) {
			return "", "", false
		}
		return strings.Trim(strings.TrimSpace(parsedHost), "[]"), parsedPort, parsedHost != ""
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.Trim(value, "[]")
	}
	if strings.Contains(value, ":") {
		return "", "", false
	}
	return strings.TrimSuffix(value, "."), "", true
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port > 0 && port <= 65535
}

func firstHeaderValue(value string) string {
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func requestProto(r *http.Request, trusted bool) string {
	if trusted {
		if proto := validProtocol(firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))); proto != "" {
			return proto
		}
		switch strings.ToLower(firstHeaderValue(r.Header.Get("X-Forwarded-Ssl"))) {
		case "on", "true", "1", "https":
			return "https"
		case "off", "false", "0", "http":
			return "http"
		}
		if proto := validProtocol(firstHeaderValue(r.Header.Get("X-Url-Scheme"))); proto != "" {
			return proto
		}
		if strings.EqualFold(firstHeaderValue(r.Header.Get("Front-End-Https")), "on") {
			return "https"
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// ResolveProto applies the shared trusted-proxy/TLS protocol priority.
func ResolveProto(r *http.Request, trusted bool) string {
	return requestProto(r, trusted)
}

func validProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http", "https":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func listenPort(value, proto string) string {
	parts := strings.Split(value, ",")
	index := 0
	if proto == "https" && len(parts) > 1 {
		index = 1
	}
	if len(parts) == 0 {
		return ""
	}
	port := strings.TrimSpace(parts[index])
	if validPort(port) {
		return port
	}
	return ""
}

var defaultMu sync.RWMutex
var defaultManager = NewManager(Options{DomainEnv: os.Getenv("DOMAIN")})

// SetDefaultManager changes the process-wide resolver used by package helpers.
func SetDefaultManager(manager *Manager) {
	if manager == nil {
		manager = NewManager(Options{DomainEnv: os.Getenv("DOMAIN")})
	}
	defaultMu.Lock()
	defaultManager = manager
	defaultMu.Unlock()
}

// GetURLVars resolves URL variables using the current process resolver.
func GetURLVars(r *http.Request) (string, string, string) {
	defaultMu.RLock()
	manager := defaultManager
	defaultMu.RUnlock()
	return manager.GetURLVars(r)
}

// BuildURL builds an absolute URL using the current process resolver.
func BuildURL(r *http.Request, path string) string {
	defaultMu.RLock()
	manager := defaultManager
	defaultMu.RUnlock()
	return manager.BuildURL(r, path)
}

// GetBaseDomain returns the current process resolver's inferred base domain.
func GetBaseDomain() string {
	defaultMu.RLock()
	manager := defaultManager
	defaultMu.RUnlock()
	return manager.GetBaseDomain()
}

// GetWildcardDomain returns the current process resolver's inferred wildcard.
func GetWildcardDomain() string {
	defaultMu.RLock()
	manager := defaultManager
	defaultMu.RUnlock()
	return manager.GetWildcardDomain()
}
