package ssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
)

// acmeUser implements registration.User for the DNS-01 issuance flow. The
// registration resource is intentionally not persisted across process
// restarts: re-registering an already-existing account with the same
// private key is idempotent per RFC 8555 §7.3.1 and simply returns the
// existing account, so it is safe (and simpler) to re-register on every run.
type acmeUser struct {
	email string
	reg   *registration.Resource
	key   crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.reg }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// BuildDNSProvider dynamically constructs a lego DNS-01 challenge provider
// for ANY lego-supported provider name, per AI.md PART 15 "DNS-01 Provider
// Configuration" and api-rules.md "NEVER let a project's DNS-01 provider
// list be limited". creds is a map of the provider's own environment
// variable names (as documented at https://go-acme.github.io/lego/dns/) to
// values; each lego provider's constructor reads its env vars once at
// construction time, so the values are set as process environment variables
// only for the duration of this call and cleared again afterward.
func BuildDNSProvider(providerName string, creds map[string]string) (challenge.Provider, error) {
	if providerName == "" {
		return nil, fmt.Errorf("server.ssl.letsencrypt.dns_provider is not configured")
	}

	var set []string
	for k, v := range creds {
		if err := os.Setenv(k, v); err != nil {
			return nil, fmt.Errorf("failed to set env %q: %w", k, err)
		}
		set = append(set, k)
	}
	defer func() {
		for _, k := range set {
			_ = os.Unsetenv(k)
		}
	}()

	provider, err := dns.NewDNSChallengeProviderByName(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to build dns-01 provider %q: %w", providerName, err)
	}
	return provider, nil
}

// ValidateDNSProviderCredentials confirms the configured provider/credential
// pair is well-formed by attempting to construct the provider, per PART 15
// "Server validates credentials on startup and before certificate requests".
// lego providers validate their required fields during construction.
func ValidateDNSProviderCredentials(providerName string, creds map[string]string) error {
	_, err := BuildDNSProvider(providerName, creds)
	return err
}

// loadOrCreateDNS01Account loads a persisted ACME account private key from
// accountDir, or generates and persists a new one. Reusing the same key
// across runs means re-registration (see ObtainCertificateDNS01) always
// resolves to the same ACME account.
func loadOrCreateDNS01Account(accountDir, email string) (*acmeUser, error) {
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create acme account dir: %w", err)
	}
	keyPath := filepath.Join(accountDir, "account.key")

	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid acme account key at %s", keyPath)
		}
		key, parseErr := x509.ParseECPrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse acme account key: %w", parseErr)
		}
		return &acmeUser{email: email, key: key}, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate acme account key: %w", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal acme account key: %w", err)
	}
	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, fmt.Errorf("failed to persist acme account key: %w", err)
	}
	return &acmeUser{email: email, key: key}, nil
}

// ObtainCertificateDNS01 runs the full ACME DNS-01 flow via lego: it loads
// or creates a persistent account key under accountDir, registers (or
// reuses) the ACME account, sets the given DNS-01 provider, and requests a
// certificate for the given domains (the first domain may be a wildcard,
// e.g. "*.example.com", since DNS-01 has no port/host reachability
// requirement per PART 15 "Supported Challenge Types"). Returns PEM-encoded
// certificate (fullchain, since Bundle is requested) and private key bytes
// ready to write to disk.
func ObtainCertificateDNS01(accountDir, email string, staging bool, provider challenge.Provider, domains []string) (certPEM, keyPEM []byte, err error) {
	if len(domains) == 0 {
		return nil, nil, fmt.Errorf("no domains specified for dns-01 certificate request")
	}

	user, err := loadOrCreateDNS01Account(accountDir, email)
	if err != nil {
		return nil, nil, err
	}

	cfg := lego.NewConfig(user)
	if staging {
		cfg.CADirURL = lego.LEDirectoryStaging
	}
	cfg.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create acme client: %w", err)
	}

	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return nil, nil, fmt.Errorf("failed to set dns-01 provider: %w", err)
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to register acme account: %w", err)
	}
	user.reg = reg

	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to obtain dns-01 certificate: %w", err)
	}

	return resource.Certificate, resource.PrivateKey, nil
}
