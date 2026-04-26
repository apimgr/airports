package ssl

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/acme/autocert"
)

// Config holds SSL/TLS configuration
type Config struct {
	Enabled     bool
	CertPath    string
	LetsEncrypt LetsEncryptConfig
}

// LetsEncryptConfig holds Let's Encrypt settings
type LetsEncryptConfig struct {
	Enabled         bool
	Email           string
	Challenge       string // http-01, tls-alpn-01, dns-01
	DNSProviderType string
	DNSProviderKey  string
	RFC2136Server   string
	RFC2136Name     string
	RFC2136Algo     string
}

// Manager handles SSL/TLS certificates
type Manager struct {
	config      Config
	certManager *autocert.Manager
	mu          sync.RWMutex
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

	// Check for existing certificates first (e.g., from /etc/letsencrypt/live)
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

	// Use Let's Encrypt if enabled
	if m.config.LetsEncrypt.Enabled {
		return m.getLetsEncryptTLSConfig(domains)
	}

	// Check for manual certificates
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

	return nil, fmt.Errorf("no certificates available and Let's Encrypt not enabled")
}

// getLetsEncryptTLSConfig configures autocert for Let's Encrypt
func (m *Manager) getLetsEncryptTLSConfig(domains []string) (*tls.Config, error) {
	cacheDir := filepath.Join(m.config.CertPath, "autocert")
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

// GetHTTPHandler returns HTTP handler for ACME challenges
func (m *Manager) GetHTTPHandler(fallback http.Handler) http.Handler {
	if m.certManager != nil {
		return m.certManager.HTTPHandler(fallback)
	}
	return fallback
}

// findExistingCerts looks for existing certificates (e.g., from certbot)
func (m *Manager) findExistingCerts(domains []string) (certPath, keyPath string) {
	// Check /etc/letsencrypt/live first
	for _, domain := range domains {
		cert := filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem")
		key := filepath.Join("/etc/letsencrypt/live", domain, "privkey.pem")
		if fileExists(cert) && fileExists(key) {
			return cert, key
		}
	}
	return "", ""
}

// findManualCerts looks for manually placed certificates
func (m *Manager) findManualCerts(domains []string) (certPath, keyPath string) {
	// Check configured cert path
	if m.config.CertPath != "" {
		for _, domain := range domains {
			cert := filepath.Join(m.config.CertPath, domain+".crt")
			key := filepath.Join(m.config.CertPath, domain+".key")
			if fileExists(cert) && fileExists(key) {
				return cert, key
			}

			// Also try fullchain format
			cert = filepath.Join(m.config.CertPath, domain, "fullchain.pem")
			key = filepath.Join(m.config.CertPath, domain, "privkey.pem")
			if fileExists(cert) && fileExists(key) {
				return cert, key
			}
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
	w.Write([]byte(auth))
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
