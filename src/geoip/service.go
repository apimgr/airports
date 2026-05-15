package geoip

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

const (
	// sapics/ip-location-db databases via jsdelivr CDN (daily updates)
	cityIPv4URL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb"
	cityIPv6URL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv6.mmdb"
	countryURL  = "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
	asnURL      = "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
)

// mmdb record structs for ip-location-db format (GeoLite2-compatible fields,
// but with custom DatabaseType strings that geoip2-golang rejects at Open time;
// we use maxminddb directly to bypass that type check).

type mmdbCity struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		IsoCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Subdivisions []struct {
		IsoCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

type mmdbCountry struct {
	Country struct {
		IsoCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

type mmdbASN struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// Service manages GeoIP lookups
type Service struct {
	cityIPv4DB *maxminddb.Reader // City database for IPv4 addresses
	cityIPv6DB *maxminddb.Reader // City database for IPv6 addresses
	countryDB  *maxminddb.Reader // Country database (combined IPv4/IPv6)
	asnDB      *maxminddb.Reader // ASN database (combined IPv4/IPv6)
	dataDir    string
}

// GeoLocation contains geolocation information for an IP
type GeoLocation struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`              // ISO code (US, CA, etc.)
	CountryName string  `json:"country_name"`
	Region      string  `json:"region,omitempty"`     // State/Province code
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

	s.Close()

	cityIPv4DB, err := maxminddb.Open(cityIPv4Path)
	if err != nil {
		return fmt.Errorf("failed to load city IPv4 database: %w", err)
	}
	s.cityIPv4DB = cityIPv4DB

	cityIPv6DB, err := maxminddb.Open(cityIPv6Path)
	if err != nil {
		s.cityIPv4DB.Close()
		return fmt.Errorf("failed to load city IPv6 database: %w", err)
	}
	s.cityIPv6DB = cityIPv6DB

	countryDB, err := maxminddb.Open(countryPath)
	if err != nil {
		s.cityIPv4DB.Close()
		s.cityIPv6DB.Close()
		return fmt.Errorf("failed to load country database: %w", err)
	}
	s.countryDB = countryDB

	asnDB, err := maxminddb.Open(asnPath)
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
	databases := []struct {
		filename string
		url      string
	}{
		{"geolite2-city-ipv4.mmdb", cityIPv4URL},
		{"geolite2-city-ipv6.mmdb", cityIPv6URL},
		{"geo-whois-asn-country.mmdb", countryURL},
		{"asn.mmdb", asnURL},
	}

	for _, db := range databases {
		path := filepath.Join(s.dataDir, db.filename)
		fmt.Printf("  Downloading %s...\n", db.filename)
		if err := downloadFile(path, db.url); err != nil {
			return fmt.Errorf("failed to download %s: %w", db.filename, err)
		}
	}

	fmt.Println("GeoIP databases downloaded successfully")
	return nil
}

// UpdateDatabases downloads fresh copies of all databases
func (s *Service) UpdateDatabases() error {
	fmt.Println("Updating GeoIP databases...")

	tempDir := filepath.Join(s.dataDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	databases := []struct {
		filename string
		url      string
	}{
		{"geolite2-city-ipv4.mmdb", cityIPv4URL},
		{"geolite2-city-ipv6.mmdb", cityIPv6URL},
		{"geo-whois-asn-country.mmdb", countryURL},
		{"asn.mmdb", asnURL},
	}

	for _, db := range databases {
		tempPath := filepath.Join(tempDir, db.filename)
		fmt.Printf("  Downloading %s...\n", db.filename)
		if err := downloadFile(tempPath, db.url); err != nil {
			return fmt.Errorf("failed to download %s: %w", db.filename, err)
		}
	}

	s.Close()

	for _, db := range databases {
		tempPath := filepath.Join(tempDir, db.filename)
		finalPath := filepath.Join(s.dataDir, db.filename)
		if err := os.Rename(tempPath, finalPath); err != nil {
			return fmt.Errorf("failed to move %s: %w", db.filename, err)
		}
	}

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

	// Choose city DB by IP version
	var cityDB *maxminddb.Reader
	if ip.To4() != nil {
		cityDB = s.cityIPv4DB
	} else {
		cityDB = s.cityIPv6DB
	}

	// Try city lookup first
	if cityDB != nil {
		var record mmdbCity
		if err := cityDB.Lookup(ip, &record); err == nil && record.Country.IsoCode != "" {
			location := &GeoLocation{
				IP:          ip.String(),
				Country:     record.Country.IsoCode,
				CountryName: record.Country.Names["en"],
				Latitude:    record.Location.Latitude,
				Longitude:   record.Location.Longitude,
				TimeZone:    record.Location.TimeZone,
				PostalCode:  record.Postal.Code,
			}
			if record.City.Names != nil {
				location.City = record.City.Names["en"]
			}
			if len(record.Subdivisions) > 0 {
				location.Region = record.Subdivisions[0].IsoCode
				location.RegionName = record.Subdivisions[0].Names["en"]
			}
			s.addASNInfo(ip, location)
			return location, nil
		}
	}

	// Fallback to country lookup
	var countryRecord mmdbCountry
	if err := s.countryDB.Lookup(ip, &countryRecord); err != nil {
		return nil, fmt.Errorf("geolocation failed: %w", err)
	}

	location := &GeoLocation{
		IP:          ip.String(),
		Country:     countryRecord.Country.IsoCode,
		CountryName: countryRecord.Country.Names["en"],
	}
	s.addASNInfo(ip, location)
	return location, nil
}

// addASNInfo adds ASN information to the location
func (s *Service) addASNInfo(ip net.IP, location *GeoLocation) {
	if s.asnDB == nil {
		return
	}
	var record mmdbASN
	if err := s.asnDB.Lookup(ip, &record); err == nil {
		location.ASN = record.Number
		location.ASNOrg = record.Org
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
	if s.cityIPv4DB != nil {
		s.cityIPv4DB.Close()
		s.cityIPv4DB = nil
	}
	if s.cityIPv6DB != nil {
		s.cityIPv6DB.Close()
		s.cityIPv6DB = nil
	}
	if s.countryDB != nil {
		s.countryDB.Close()
		s.countryDB = nil
	}
	if s.asnDB != nil {
		s.asnDB.Close()
		s.asnDB = nil
	}
	return nil
}

// ExtractIPFromRequest extracts the real client IP from request headers
func ExtractIPFromRequest(remoteAddr, xForwardedFor, xRealIP string) string {
	if xForwardedFor != "" {
		for idx := 0; idx < len(xForwardedFor); idx++ {
			if xForwardedFor[idx] == ',' {
				return xForwardedFor[:idx]
			}
		}
		return xForwardedFor
	}
	if xRealIP != "" {
		return xRealIP
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadFile(dest string, url string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}
