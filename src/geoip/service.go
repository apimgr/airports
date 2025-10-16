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
	// sapics/ip-location-db databases via jsdelivr CDN (daily updates)
	cityIPv4URL  = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb"
	cityIPv6URL  = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv6.mmdb"
	countryURL   = "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
	asnURL       = "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
)

// Service manages GeoIP lookups
type Service struct {
	cityIPv4DB *geoip2.Reader // City database for IPv4 addresses
	cityIPv6DB *geoip2.Reader // City database for IPv6 addresses
	countryDB  *geoip2.Reader // Country database (combined IPv4/IPv6)
	asnDB      *geoip2.Reader // ASN database (combined IPv4/IPv6)
	dataDir    string
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
	cityIPv4Path := filepath.Join(geoipDir, "geolite2-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(geoipDir, "geolite2-city-ipv6.mmdb")
	countryPath := filepath.Join(geoipDir, "geo-whois-asn-country.mmdb")
	asnPath := filepath.Join(geoipDir, "asn.mmdb")

	if !fileExists(cityIPv4Path) || !fileExists(cityIPv6Path) || !fileExists(countryPath) || !fileExists(asnPath) {
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
	cityIPv4Path := filepath.Join(s.dataDir, "geolite2-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(s.dataDir, "geolite2-city-ipv6.mmdb")
	countryPath := filepath.Join(s.dataDir, "geo-whois-asn-country.mmdb")
	asnPath := filepath.Join(s.dataDir, "asn.mmdb")

	// Close existing databases if any
	s.Close()

	// Load City IPv4 database
	cityIPv4DB, err := geoip2.Open(cityIPv4Path)
	if err != nil {
		return fmt.Errorf("failed to load city IPv4 database: %w", err)
	}
	s.cityIPv4DB = cityIPv4DB

	// Load City IPv6 database
	cityIPv6DB, err := geoip2.Open(cityIPv6Path)
	if err != nil {
		s.cityIPv4DB.Close()
		return fmt.Errorf("failed to load city IPv6 database: %w", err)
	}
	s.cityIPv6DB = cityIPv6DB

	// Load Country database (fallback)
	countryDB, err := geoip2.Open(countryPath)
	if err != nil {
		s.cityIPv4DB.Close()
		s.cityIPv6DB.Close()
		return fmt.Errorf("failed to load country database: %w", err)
	}
	s.countryDB = countryDB

	// Load ASN database
	asnDB, err := geoip2.Open(asnPath)
	if err != nil {
		s.cityIPv4DB.Close()
		s.cityIPv6DB.Close()
		s.countryDB.Close()
		return fmt.Errorf("failed to load ASN database: %w", err)
	}
	s.asnDB = asnDB

	return nil
}

// DownloadDatabases downloads all GeoIP databases from sapics/ip-location-db via jsdelivr CDN
func (s *Service) DownloadDatabases() error {
	databases := map[string]string{
		"geolite2-city-ipv4.mmdb":      cityIPv4URL,
		"geolite2-city-ipv6.mmdb":      cityIPv6URL,
		"geo-whois-asn-country.mmdb":   countryURL,
		"asn.mmdb":                     asnURL,
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
		"geolite2-city-ipv4.mmdb":      cityIPv4URL,
		"geolite2-city-ipv6.mmdb":      cityIPv6URL,
		"geo-whois-asn-country.mmdb":   countryURL,
		"asn.mmdb":                     asnURL,
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

	if (s.cityIPv4DB == nil && s.cityIPv6DB == nil) || s.countryDB == nil {
		return nil, fmt.Errorf("GeoIP databases not loaded")
	}

	// Determine which city database to use based on IP version
	var cityDB *geoip2.Reader
	if ip.To4() != nil {
		// IPv4 address
		cityDB = s.cityIPv4DB
	} else {
		// IPv6 address
		cityDB = s.cityIPv6DB
	}

	// Try city lookup first (most detailed)
	if cityDB != nil {
		city, err := cityDB.City(ip)
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

	if s.cityIPv4DB != nil {
		if err := s.cityIPv4DB.Close(); err != nil {
			errs = append(errs, err)
		}
		s.cityIPv4DB = nil
	}

	if s.cityIPv6DB != nil {
		if err := s.cityIPv6DB.Close(); err != nil {
			errs = append(errs, err)
		}
		s.cityIPv6DB = nil
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
