package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Config holds SSL/TLS configuration
type Config struct {
	Enabled     bool
	CertPath    string
	LetsEncrypt LetsEncryptConfig
}

// LetsEncryptConfig holds Let's Encrypt settings. DNSProvider is any
// lego-supported provider name (see providers/dns.NewDNSChallengeProviderByName)
// and DNSCredentials holds that provider's own env-var-name -> value pairs,
// already decrypted by the caller from server.ssl.letsencrypt.dns_credentials
// using server.security.encryption_key, per AI.md PART 15 "DNS-01 Provider
// Configuration". Never log or serialize DNSCredentials.
type LetsEncryptConfig struct {
	Enabled        bool
	Email          string
	Challenge      string // http-01, tls-alpn-01, dns-01
	Staging        bool
	DNSProvider    string
	DNSCredentials map[string]string
}

// Manager handles SSL/TLS certificates
type Manager struct {
	config       Config
	certManager  *autocert.Manager
	dns01Cert    *tls.Certificate
	dns01Domains []string
	dns01Expiry  time.Time
	// autocertExpiry tracks the last-observed NotAfter per domain for the
	// autocert (http-01/tls-alpn-01) path, so RenewIfNeeded can detect that
	// a renewal actually happened (autocert exposes no renewal signal of
	// its own — GetCertificate silently returns the cached cert unless one
	// is due).
	autocertExpiry map[string]time.Time
	mu             sync.RWMutex
}

// NewManager creates a new SSL manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// GetTLSConfig returns TLS configuration for the server
func (m *Manager) GetTLSConfig(domains []string) (*tls.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, nil
	}

	// Check ALL existing certificate locations first, in the mandated lookup
	// order (AI.md PART 15 "Certificate Lookup Order"), before ever requesting
	// a new Let's Encrypt certificate:
	//   1-2. /etc/letsencrypt/live/domain/ and /etc/letsencrypt/live/{fqdn}/  (findExistingCerts)
	//   3.   {config_dir}/ssl/letsencrypt/{fqdn}/                             (findManualCerts)
	//   4.   {config_dir}/ssl/local/{fqdn}/                                   (findManualCerts)
	// A new LE cert is only requested when none of the four locations exist.
	if cert, key := m.findExistingCerts(domains); cert != "" && key != "" {
		log.Printf("Using existing certificate: %s", cert)
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}

	// Check for app-managed and user-provided certificates (priorities 3-4).
	if cert, key := m.findManualCerts(domains); cert != "" && key != "" {
		log.Printf("Using manual certificate: %s", cert)
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}

	// No existing certificate found anywhere - request a new one via Let's
	// Encrypt if enabled.
	if m.config.LetsEncrypt.Enabled {
		return m.getLetsEncryptTLSConfig(domains)
	}

	return nil, fmt.Errorf("no certificates available and Let's Encrypt not enabled")
}

// getLetsEncryptTLSConfig configures Let's Encrypt issuance for the given
// domains. HTTP-01 and TLS-ALPN-01 use autocert (needs port 80/443 reachable
// from the CA). DNS-01 has no port requirement and supports wildcard certs,
// so it is obtained directly via lego (see dns01.go) using the configured
// DNSProvider/DNSCredentials, per AI.md PART 15 "Supported Challenge Types".
func (m *Manager) getLetsEncryptTLSConfig(domains []string) (*tls.Config, error) {
	if m.config.LetsEncrypt.Challenge == "dns-01" {
		return m.getDNS01TLSConfig(domains)
	}

	// Autocert's own cache lives under the app-managed letsencrypt/
	// subdirectory (AI.md PART 15 "Certificate Directory Structure") even
	// though its internal file naming is autocert's own, not fullchain.pem/
	// privkey.pem.
	cacheDir := filepath.Join(m.config.CertPath, "letsencrypt", "autocert")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cert cache dir: %w", err)
	}

	m.certManager = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cacheDir),
		Email:      m.config.LetsEncrypt.Email,
	}

	return m.certManager.TLSConfig(), nil
}

// getDNS01TLSConfig obtains (or reuses a cached) certificate via the DNS-01
// challenge and returns a static tls.Config serving it. Unlike autocert,
// lego has no long-running GetCertificate hook, so the certificate is cached
// on the Manager and refreshed by RenewIfNeeded.
func (m *Manager) getDNS01TLSConfig(domains []string) (*tls.Config, error) {
	cert, err := m.obtainDNS01Certificate(domains)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// obtainDNS01Certificate builds the configured DNS-01 provider, runs the
// lego ACME flow, writes the resulting cert/key under
// {CertPath}/letsencrypt/{fqdn}/ (so findManualCerts also picks them up on
// future restarts as an app-managed, auto-renewing Let's Encrypt cert per
// AI.md PART 15 "Certificate Directory Structure"), and caches the parsed
// certificate plus its expiry for RenewIfNeeded.
func (m *Manager) obtainDNS01Certificate(domains []string) (*tls.Certificate, error) {
	le := m.config.LetsEncrypt
	provider, err := BuildDNSProvider(le.DNSProvider, le.DNSCredentials)
	if err != nil {
		return nil, err
	}

	accountDir := filepath.Join(m.config.CertPath, "letsencrypt", "dns01-accounts")
	certPEM, keyPEM, err := ObtainCertificateDNS01(accountDir, le.Email, le.Staging, provider, domains)
	if err != nil {
		return nil, err
	}

	if len(domains) > 0 {
		certDir := filepath.Join(m.config.CertPath, "letsencrypt", domains[0])
		if mkErr := os.MkdirAll(certDir, 0700); mkErr != nil {
			return nil, fmt.Errorf("failed to create cert dir: %w", mkErr)
		}
		if writeErr := os.WriteFile(filepath.Join(certDir, "fullchain.pem"), certPEM, 0644); writeErr != nil {
			return nil, fmt.Errorf("failed to write dns-01 certificate: %w", writeErr)
		}
		if writeErr := os.WriteFile(filepath.Join(certDir, "privkey.pem"), keyPEM, 0600); writeErr != nil {
			return nil, fmt.Errorf("failed to write dns-01 private key: %w", writeErr)
		}
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dns-01 certificate: %w", err)
	}

	expiry, err := certificateExpiry(&tlsCert)
	if err != nil {
		return nil, err
	}

	m.dns01Cert = &tlsCert
	m.dns01Domains = domains
	m.dns01Expiry = expiry
	return &tlsCert, nil
}

// RenewIfNeeded proactively triggers Let's Encrypt issuance/renewal checks
// for the given domains, per AI.md PART 22's daily ssl_renewal scheduled
// task (auto-renew app-managed certs 7 days before expiry). Only applies
// to app-managed autocert certificates — certs found under
// /etc/letsencrypt/live/** (certbot-managed) or manually placed certs are
// never touched here, per PART 22 "never auto-renew certs found under
// /etc/letsencrypt/live/**". autocert.Manager.GetCertificate internally
// checks expiry and fetches a fresh certificate when one is needed, so
// calling it here for each domain is sufficient to drive renewal without
// waiting for the next incoming TLS handshake.
func (m *Manager) RenewIfNeeded(domains []string) (bool, error) {
	m.mu.RLock()
	certManager := m.certManager
	dns01Active := m.config.LetsEncrypt.Challenge == "dns-01" && m.dns01Cert != nil
	dns01Expiry := m.dns01Expiry
	enabled := m.config.Enabled && m.config.LetsEncrypt.Enabled
	m.mu.RUnlock()

	if !enabled {
		return false, nil
	}

	if dns01Active {
		if time.Until(dns01Expiry) > 7*24*time.Hour {
			return false, nil
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if time.Until(m.dns01Expiry) > 7*24*time.Hour {
			return false, nil
		}
		if _, err := m.obtainDNS01Certificate(domains); err != nil {
			return false, fmt.Errorf("dns-01 ssl renewal failed: %w", err)
		}
		return true, nil
	}

	if certManager == nil {
		return false, nil
	}

	renewed := false
	for _, domain := range domains {
		hello := &tls.ClientHelloInfo{ServerName: domain}
		cert, err := certManager.GetCertificate(hello)
		if err != nil {
			return renewed, fmt.Errorf("ssl renewal check failed for %s: %w", domain, err)
		}
		expiry, expErr := certificateExpiry(cert)
		if expErr != nil {
			continue
		}
		m.mu.Lock()
		if m.autocertExpiry == nil {
			m.autocertExpiry = make(map[string]time.Time)
		}
		previous, known := m.autocertExpiry[domain]
		m.autocertExpiry[domain] = expiry
		m.mu.Unlock()
		if known && expiry.After(previous) {
			renewed = true
		}
	}
	return renewed, nil
}

// CertificateExpiry returns the current managed certificate's expiry for
// domain, across whichever challenge path is active (dns-01 or
// autocert-backed http-01/tls-alpn-01), for use by the ssl_expiring /
// ssl_renewed / ssl_renewal_failed notification events (PART 17). Returns
// an error if no managed certificate is currently known for domain — this
// only covers certificates this Manager issues/renews, never certbot-managed
// or manually placed certs (PART 22 "never auto-renew certs found under
// /etc/letsencrypt/live/**").
func (m *Manager) CertificateExpiry(domain string) (time.Time, error) {
	m.mu.RLock()
	dns01Active := m.config.LetsEncrypt.Challenge == "dns-01" && m.dns01Cert != nil
	dns01Expiry := m.dns01Expiry
	certManager := m.certManager
	cached, cachedKnown := m.autocertExpiry[domain]
	m.mu.RUnlock()

	if dns01Active {
		return dns01Expiry, nil
	}

	if certManager != nil {
		hello := &tls.ClientHelloInfo{ServerName: domain}
		cert, err := certManager.GetCertificate(hello)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load certificate for %s: %w", domain, err)
		}
		expiry, err := certificateExpiry(cert)
		if err != nil {
			return time.Time{}, err
		}
		return expiry, nil
	}

	if cachedKnown {
		return cached, nil
	}

	return time.Time{}, fmt.Errorf("no managed certificate known for %s", domain)
}

// GetHTTPHandler returns HTTP handler for ACME challenges
func (m *Manager) GetHTTPHandler(fallback http.Handler) http.Handler {
	if m.certManager != nil {
		return m.certManager.HTTPHandler(fallback)
	}
	return fallback
}

// findExistingCerts looks for system (certbot-managed) certificates under
// /etc/letsencrypt/live, per AI.md PART 15 "Certificate Lookup Order"
// priorities 1-2. These are used but never renewed by this app.
func (m *Manager) findExistingCerts(domains []string) (certPath, keyPath string) {
	// Priority 1: /etc/letsencrypt/live/domain/ - literal "domain" directory,
	// a common shared certbot setup covering multiple/all vhosts.
	if cert, key, ok := certbotCert("domain"); ok {
		return cert, key
	}

	// Priority 2: /etc/letsencrypt/live/{fqdn}/ - FQDN-named directory.
	for _, domain := range domains {
		if cert, key, ok := certbotCert(domain); ok {
			return cert, key
		}
	}

	return "", ""
}

// certbotCert checks for a certbot-style fullchain.pem/privkey.pem pair
// under /etc/letsencrypt/live/{dirName}/.
func certbotCert(dirName string) (certPath, keyPath string, ok bool) {
	cert := filepath.Join("/etc/letsencrypt/live", dirName, "fullchain.pem")
	key := filepath.Join("/etc/letsencrypt/live", dirName, "privkey.pem")
	if fileExists(cert) && fileExists(key) {
		return cert, key, true
	}
	return "", "", false
}

// findManualCerts looks for app-managed certificates under CertPath
// ({config_dir}/ssl), per AI.md PART 15 "Certificate Lookup Order"
// priorities 3-4 and "Certificate Directory Structure":
//
//	{config_dir}/ssl/letsencrypt/{fqdn}/{fullchain.pem,privkey.pem}  (priority 3, app-managed, auto-renews)
//	{config_dir}/ssl/local/{fqdn}/{cert.pem,key.pem}                 (priority 4, self-signed/user-provided, no auto-renew)
func (m *Manager) findManualCerts(domains []string) (certPath, keyPath string) {
	if m.config.CertPath == "" {
		return "", ""
	}

	// Priority 3: app-managed Let's Encrypt certificates.
	for _, domain := range domains {
		cert := filepath.Join(m.config.CertPath, "letsencrypt", domain, "fullchain.pem")
		key := filepath.Join(m.config.CertPath, "letsencrypt", domain, "privkey.pem")
		if fileExists(cert) && fileExists(key) {
			return cert, key
		}
	}

	// Priority 4: self-signed or user-provided certificates.
	for _, domain := range domains {
		cert := filepath.Join(m.config.CertPath, "local", domain, "cert.pem")
		key := filepath.Join(m.config.CertPath, "local", domain, "key.pem")
		if fileExists(cert) && fileExists(key) {
			return cert, key
		}
	}

	return "", ""
}

// ChallengeServer handles ACME HTTP-01 challenges
type ChallengeServer struct {
	tokens map[string]string
	mu     sync.RWMutex
}

// NewChallengeServer creates a challenge server
func NewChallengeServer() *ChallengeServer {
	return &ChallengeServer{
		tokens: make(map[string]string),
	}
}

// SetToken sets a challenge token
func (cs *ChallengeServer) SetToken(token, auth string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.tokens[token] = auth
}

// ClearToken removes a challenge token
func (cs *ChallengeServer) ClearToken(token string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.tokens, token)
}

// ServeHTTP handles ACME challenge requests
func (cs *ChallengeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		return false
	}

	token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
	cs.mu.RLock()
	auth, ok := cs.tokens[token]
	cs.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return true
	}

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(auth)); err != nil {
		log.Printf("failed to write acme challenge response: %v", err)
	}
	return true
}

// ParseChallenge parses challenge type from string
func ParseChallenge(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "http-01", "http01", "http":
		return "http-01"
	case "tls-alpn-01", "tlsalpn01", "tls-alpn", "tls":
		return "tls-alpn-01"
	case "dns-01", "dns01", "dns":
		return "dns-01"
	default:
		return "http-01"
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// certificateExpiry parses the leaf certificate's NotAfter time, used to
// decide when a DNS-01-obtained certificate needs renewal.
func certificateExpiry(cert *tls.Certificate) (time.Time, error) {
	if len(cert.Certificate) == 0 {
		return time.Time{}, fmt.Errorf("certificate has no leaf")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate leaf: %w", err)
	}
	return leaf.NotAfter, nil
}
