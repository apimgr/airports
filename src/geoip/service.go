package geoip

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/geoip2-golang"
)

const (
	cityDBURL    = "https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-City.mmdb"
	countryDBURL = "https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-Country.mmdb"
	asnDBURL     = "https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-ASN.mmdb"
)

// Service manages GeoIP lookups
type Service struct {
	cityDB    *geoip2.Reader
	countryDB *geoip2.Reader
	asnDB     *geoip2.Reader
	dataDir   string
}

// GeoLocation contains geolocation information for an IP
type GeoLocation struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`            // ISO code (US, CA, etc.)
	CountryName string  `json:"country_name"`
	Region      string  `json:"region,omitempty"`   // State/Province code
	RegionName  string  `json:"region_name,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	TimeZone    string  `json:"timezone,omitempty"`
	PostalCode  string  `json:"postal_code,omitempty"`
	ASN         uint    `json:"asn,omitempty"`
	ASNOrg      string  `json:"asn_org,omitempty"`
}

// NewService creates a new GeoIP service, downloading databases if needed
func NewService(configDir string) (*Service, error) {
	if configDir == "" {
		return nil, fmt.Errorf("config directory is required")
	}

	geoipDir := filepath.Join(configDir, "geoip")
	if err := os.MkdirAll(geoipDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create geoip directory: %w", err)
	}

	s := &Service{dataDir: geoipDir}

	// Check if databases exist, download if not
	cityPath := filepath.Join(geoipDir, "GeoLite2-City.mmdb")
	countryPath := filepath.Join(geoipDir, "GeoLite2-Country.mmdb")
	asnPath := filepath.Join(geoipDir, "GeoLite2-ASN.mmdb")

	if !fileExists(cityPath) || !fileExists(countryPath) || !fileExists(asnPath) {
		fmt.Println("GeoIP databases not found, downloading...")
		if err := s.DownloadDatabases(); err != nil {
			return nil, fmt.Errorf("failed to download GeoIP databases: %w", err)
		}
	}

	// Load databases
	if err := s.LoadDatabases(); err != nil {
		return nil, err
	}

	return s, nil
}

// LoadDatabases loads GeoIP databases from disk
func (s *Service) LoadDatabases() error {
	cityPath := filepath.Join(s.dataDir, "GeoLite2-City.mmdb")
	countryPath := filepath.Join(s.dataDir, "GeoLite2-Country.mmdb")
	asnPath := filepath.Join(s.dataDir, "GeoLite2-ASN.mmdb")

	// Close existing databases if any
	s.Close()

	// Load City database
	cityDB, err := geoip2.Open(cityPath)
	if err != nil {
		return fmt.Errorf("failed to load city database: %w", err)
	}
	s.cityDB = cityDB

	// Load Country database (fallback)
	countryDB, err := geoip2.Open(countryPath)
	if err != nil {
		s.cityDB.Close()
		return fmt.Errorf("failed to load country database: %w", err)
	}
	s.countryDB = countryDB

	// Load ASN database
	asnDB, err := geoip2.Open(asnPath)
	if err != nil {
		s.cityDB.Close()
		s.countryDB.Close()
		return fmt.Errorf("failed to load ASN database: %w", err)
	}
	s.asnDB = asnDB

	return nil
}

// DownloadDatabases downloads all GeoIP databases from P3TERX
func (s *Service) DownloadDatabases() error {
	databases := map[string]string{
		"GeoLite2-City.mmdb":    cityDBURL,
		"GeoLite2-Country.mmdb": countryDBURL,
		"GeoLite2-ASN.mmdb":     asnDBURL,
	}

	for filename, url := range databases {
		path := filepath.Join(s.dataDir, filename)
		fmt.Printf("  Downloading %s...\n", filename)
		if err := downloadFile(path, url); err != nil {
			return fmt.Errorf("failed to download %s: %w", filename, err)
		}
	}

	fmt.Println("GeoIP databases downloaded successfully")
	return nil
}

// UpdateDatabases downloads fresh copies of all databases
func (s *Service) UpdateDatabases() error {
	fmt.Println("Updating GeoIP databases...")

	// Download to temporary files first
	tempDir := filepath.Join(s.dataDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	databases := map[string]string{
		"GeoLite2-City.mmdb":    cityDBURL,
		"GeoLite2-Country.mmdb": countryDBURL,
		"GeoLite2-ASN.mmdb":     asnDBURL,
	}

	// Download all databases to temp directory
	for filename, url := range databases {
		tempPath := filepath.Join(tempDir, filename)
		fmt.Printf("  Downloading %s...\n", filename)
		if err := downloadFile(tempPath, url); err != nil {
			return fmt.Errorf("failed to download %s: %w", filename, err)
		}
	}

	// Close current databases
	s.Close()

	// Move temp files to final location
	for filename := range databases {
		tempPath := filepath.Join(tempDir, filename)
		finalPath := filepath.Join(s.dataDir, filename)
		if err := os.Rename(tempPath, finalPath); err != nil {
			return fmt.Errorf("failed to move %s: %w", filename, err)
		}
	}

	// Reload databases
	if err := s.LoadDatabases(); err != nil {
		return fmt.Errorf("failed to reload databases: %w", err)
	}

	fmt.Println("GeoIP databases updated successfully")
	return nil
}

// Lookup performs a GeoIP lookup for the given IP address
func (s *Service) Lookup(ip net.IP) (*GeoLocation, error) {
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	if s.cityDB == nil || s.countryDB == nil {
		return nil, fmt.Errorf("GeoIP databases not loaded")
	}

	// Try city lookup first (most detailed)
	city, err := s.cityDB.City(ip)
	if err == nil {
		location := &GeoLocation{
			IP:          ip.String(),
			Country:     city.Country.IsoCode,
			CountryName: city.Country.Names["en"],
			Latitude:    city.Location.Latitude,
			Longitude:   city.Location.Longitude,
			TimeZone:    city.Location.TimeZone,
		}

		// City info
		if city.City.Names != nil {
			location.City = city.City.Names["en"]
		}

		// Region/State info
		if len(city.Subdivisions) > 0 {
			location.Region = city.Subdivisions[0].IsoCode
			if city.Subdivisions[0].Names != nil {
				location.RegionName = city.Subdivisions[0].Names["en"]
			}
		}

		// Postal code
		if city.Postal.Code != "" {
			location.PostalCode = city.Postal.Code
		}

		// Add ASN information
		s.addASNInfo(ip, location)

		return location, nil
	}

	// Fallback to country lookup
	country, err := s.countryDB.Country(ip)
	if err != nil {
		return nil, fmt.Errorf("geolocation failed: %w", err)
	}

	location := &GeoLocation{
		IP:          ip.String(),
		Country:     country.Country.IsoCode,
		CountryName: country.Country.Names["en"],
	}

	// Add ASN information
	s.addASNInfo(ip, location)

	return location, nil
}

// addASNInfo adds ASN information to the location
func (s *Service) addASNInfo(ip net.IP, location *GeoLocation) {
	if s.asnDB == nil {
		return
	}
	asn, err := s.asnDB.ASN(ip)
	if err == nil {
		location.ASN = asn.AutonomousSystemNumber
		location.ASNOrg = asn.AutonomousSystemOrganization
	}
}

// LookupString performs lookup for string IP address
func (s *Service) LookupString(ipStr string) (*GeoLocation, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}
	return s.Lookup(ip)
}

// Close closes all GeoIP databases
func (s *Service) Close() error {
	var errs []error

	if s.cityDB != nil {
		if err := s.cityDB.Close(); err != nil {
			errs = append(errs, err)
		}
		s.cityDB = nil
	}

	if s.countryDB != nil {
		if err := s.countryDB.Close(); err != nil {
			errs = append(errs, err)
		}
		s.countryDB = nil
	}

	if s.asnDB != nil {
		if err := s.asnDB.Close(); err != nil {
			errs = append(errs, err)
		}
		s.asnDB = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing GeoIP databases: %v", errs)
	}

	return nil
}

// ExtractIPFromRequest extracts the real client IP from request headers
func ExtractIPFromRequest(remoteAddr, xForwardedFor, xRealIP string) string {
	// Check X-Forwarded-For header (proxy)
	if xForwardedFor != "" {
		// Take first IP from comma-separated list
		for idx := 0; idx < len(xForwardedFor); idx++ {
			if xForwardedFor[idx] == ',' {
				return xForwardedFor[:idx]
			}
		}
		return xForwardedFor
	}

	// Check X-Real-IP header
	if xRealIP != "" {
		return xRealIP
	}

	// Use remote address (strip port)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}

	return remoteAddr
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	// Get the data
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
