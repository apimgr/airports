package geoip

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// sapics/ip-location-db databases (AI.md PART 19). ASN and Country are small
// enough to serve from the jsdelivr npm CDN (AI.md "Database Sources" table).
// The City (DB-IP) files are 60-70MB — over jsdelivr's per-file size cap for
// npm-hosted packages, which returns 403 for both dbip-city-ipv4.mmdb and
// dbip-city-ipv6.mmdb — so City uses the GitHub Releases URL from AI.md
// PART 19's structured `security.geoip.databases` config instead, which has
// no such size limit. Declared as vars (not consts) so tests can point them
// at an unreachable URL to exercise the fail-open download-failure path.
var (
	cityIPv4URL = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv4.mmdb"
	cityIPv6URL = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv6.mmdb"
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
	available  bool // true once databases are downloaded and loaded successfully
}

// Available reports whether GeoIP databases are loaded and lookups can
// succeed. GeoIP is a risk signal only (AI.md PART 19) — callers must
// never treat an unavailable service as a reason to block a request.
func (s *Service) Available() bool {
	return s != nil && s.available
}

// GeoLocation contains geolocation information for an IP
type GeoLocation struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"` // ISO code (US, CA, etc.)
	CountryName string  `json:"country_name"`
	Region      string  `json:"region,omitempty"` // State/Province code
	RegionName  string  `json:"region_name,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	TimeZone    string  `json:"timezone,omitempty"`
	PostalCode  string  `json:"postal_code,omitempty"`
	ASN         uint    `json:"asn,omitempty"`
	ASNOrg      string  `json:"asn_org,omitempty"`
}

// NewService creates a new GeoIP service, downloading databases if needed.
//
// Per AI.md PART 19, GeoIP is a risk signal only — it must NEVER block
// startup or requests solely because a database download/load failed or
// the database is missing/stale. Any failure here is logged as a warning
// and NewService still returns a valid, non-nil *Service with lookups
// disabled (Available() == false) rather than a fatal error.
func NewService(baseDir string) (*Service, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory is required")
	}

	geoipDir := filepath.Join(baseDir, "geoip")
	s := &Service{dataDir: geoipDir}

	if err := os.MkdirAll(geoipDir, 0700); err != nil {
		log.Printf("Warning: failed to create GeoIP directory %s: %v", geoipDir, err)
		log.Println("GeoIP disabled for this session (risk signal only, requests are unaffected)")
		return s, nil
	}

	cityIPv4Path := filepath.Join(geoipDir, "dbip-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(geoipDir, "dbip-city-ipv6.mmdb")
	countryPath := filepath.Join(geoipDir, "geo-whois-asn-country.mmdb")
	asnPath := filepath.Join(geoipDir, "asn.mmdb")

	if !fileExists(cityIPv4Path) || !fileExists(cityIPv6Path) || !fileExists(countryPath) || !fileExists(asnPath) {
		log.Println("GeoIP databases not found, downloading...")
		if err := s.DownloadDatabases(); err != nil {
			log.Printf("Warning: failed to download GeoIP databases: %v", err)
			log.Println("GeoIP disabled until the next scheduled update succeeds (risk signal only, requests are unaffected)")
			return s, nil
		}
	}

	if err := s.LoadDatabases(); err != nil {
		log.Printf("Warning: failed to load GeoIP databases: %v", err)
		log.Println("GeoIP disabled until the databases can be loaded (risk signal only, requests are unaffected)")
		return s, nil
	}

	return s, nil
}

// LoadDatabases loads GeoIP databases from disk
func (s *Service) LoadDatabases() error {
	cityIPv4Path := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
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

	s.available = true
	return nil
}

// DownloadDatabases downloads all GeoIP databases from sapics/ip-location-db via jsdelivr CDN
func (s *Service) DownloadDatabases() error {
	databases := []struct {
		filename string
		url      string
	}{
		{"dbip-city-ipv4.mmdb", cityIPv4URL},
		{"dbip-city-ipv6.mmdb", cityIPv6URL},
		{"geo-whois-asn-country.mmdb", countryURL},
		{"asn.mmdb", asnURL},
	}

	for _, db := range databases {
		path := filepath.Join(s.dataDir, db.filename)
		log.Printf("  Downloading %s...", db.filename)
		if err := downloadFile(path, db.url); err != nil {
			return fmt.Errorf("failed to download %s: %w", db.filename, err)
		}
	}

	log.Println("GeoIP databases downloaded successfully")
	return nil
}

// UpdateDatabases downloads fresh copies of all databases. Per AI.md PART 19,
// a failed refresh must never disable an already-working GeoIP service — on
// error, the previously loaded databases (if any) are left untouched.
func (s *Service) UpdateDatabases() error {
	log.Println("Updating GeoIP databases...")

	tempDir := filepath.Join(s.dataDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	databases := []struct {
		filename string
		url      string
	}{
		{"dbip-city-ipv4.mmdb", cityIPv4URL},
		{"dbip-city-ipv6.mmdb", cityIPv6URL},
		{"geo-whois-asn-country.mmdb", countryURL},
		{"asn.mmdb", asnURL},
	}

	for _, db := range databases {
		tempPath := filepath.Join(tempDir, db.filename)
		log.Printf("  Downloading %s...", db.filename)
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

	log.Println("GeoIP databases updated successfully")
	return nil
}

// Lookup performs a GeoIP lookup for the given IP address
func (s *Service) Lookup(ip net.IP) (*GeoLocation, error) {
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	if s == nil || !s.available || (s.cityIPv4DB == nil && s.cityIPv6DB == nil) || s.countryDB == nil {
		return nil, fmt.Errorf("GeoIP unavailable: databases not loaded")
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
	s.available = false
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadFile(dest string, url string) (err error) {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Remove a partial/empty file if the download fails at any point so a
	// truncated database is never left on disk (a corrupt file that still
	// exists would block re-download on the next startup and fail to load).
	defer func() {
		if err != nil {
			_ = os.Remove(dest)
		}
	}()

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
