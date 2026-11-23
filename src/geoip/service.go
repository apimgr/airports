package geoip

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// All GeoIP databases are downloaded from the canonical ip-location-db
// GitHub Release assets and verified against the matching published checksum.
var (
	cityIPv4URL    = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv4.mmdb"
	cityIPv6URL    = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv6.mmdb"
	countryURL     = "https://github.com/sapics/ip-location-db/releases/download/latest/iptoasn-country.mmdb"
	asnURL         = "https://github.com/sapics/ip-location-db/releases/download/latest/iptoasn-asn.mmdb"
	cityIPv4SumURL = "https://github.com/sapics/ip-location-db/releases/download/checksum/dbip-city-ipv4.mmdb.sha256"
	cityIPv6SumURL = "https://github.com/sapics/ip-location-db/releases/download/checksum/dbip-city-ipv6.mmdb.sha256"
	countrySumURL  = "https://github.com/sapics/ip-location-db/releases/download/checksum/iptoasn-country.mmdb.sha256"
	asnSumURL      = "https://github.com/sapics/ip-location-db/releases/download/checksum/iptoasn-asn.mmdb.sha256"
)

// mmdb record structs for ip-location-db format (GeoLite2-compatible fields,
// but with custom DatabaseType strings that geoip2-golang rejects at Open time;
// we use maxminddb directly to bypass that type check).

type mmdbCity struct {
	City        string  `maxminddb:"city"`
	CountryCode string  `maxminddb:"country_code"`
	State1      string  `maxminddb:"state1"`
	State2      string  `maxminddb:"state2"`
	Postcode    string  `maxminddb:"postcode"`
	Latitude    float64 `maxminddb:"latitude"`
	Longitude   float64 `maxminddb:"longitude"`
	TimeZone    string  `maxminddb:"timezone"`
}

type mmdbCountry struct {
	CountryCode string `maxminddb:"country_code"`
}

type mmdbASN struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// GeoIPService manages GeoIP lookups
type GeoIPService struct {
	cityIPv4DB *maxminddb.Reader // City database for IPv4 addresses
	cityIPv6DB *maxminddb.Reader // City database for IPv6 addresses
	countryDB  *maxminddb.Reader // Country database (combined IPv4/IPv6)
	asnDB      *maxminddb.Reader // ASN database (combined IPv4/IPv6)
	dataDir    string
	available  bool // true once databases are downloaded and loaded successfully
}

// IsGeoIPAvailable reports whether GeoIP databases are loaded and lookups can
// succeed. GeoIP is a risk signal only (AI.md PART 19) — callers must
// never treat an unavailable service as a reason to block a request.
func (s *GeoIPService) IsGeoIPAvailable() bool {
	return s != nil && s.available
}

// DatabaseHealthStatus reports a public-safe freshness state for one loaded
// database without returning its path or any parser error.
func (s *GeoIPService) DatabaseHealthStatus(filename string, maxAge time.Duration, now time.Time) string {
	if s == nil || !s.available || filepath.Base(filename) != filename || maxAge <= 0 {
		return "error"
	}
	info, err := os.Stat(filepath.Join(s.dataDir, filename))
	if err != nil || !info.Mode().IsRegular() {
		return "error"
	}
	if now.Sub(info.ModTime()) > maxAge {
		return "degraded"
	}
	return "ok"
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

// NewGeoIPService creates a new GeoIP service, downloading databases if needed.
//
// Per AI.md PART 19, GeoIP is a risk signal only — it must NEVER block
// startup or requests solely because a database download/load failed or
// the database is missing/stale. Any failure here is logged as a warning
// and NewGeoIPService still returns a valid, non-nil *GeoIPService with lookups
// disabled (Available() == false) rather than a fatal error.
func NewGeoIPService(baseDir string) (*GeoIPService, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory is required")
	}

	geoipDir := filepath.Join(baseDir, "geoip")
	s := &GeoIPService{dataDir: geoipDir}

	if err := os.MkdirAll(geoipDir, 0700); err != nil {
		log.Printf("Warning: failed to create GeoIP directory %s: %v", geoipDir, err)
		log.Println("GeoIP disabled for this session (risk signal only, requests are unaffected)")
		return s, nil
	}

	cityIPv4Path := filepath.Join(geoipDir, "dbip-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(geoipDir, "dbip-city-ipv6.mmdb")
	countryPath := filepath.Join(geoipDir, "iptoasn-country.mmdb")
	asnPath := filepath.Join(geoipDir, "iptoasn-asn.mmdb")

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
func (s *GeoIPService) LoadDatabases() error {
	cityIPv4Path := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
	cityIPv6Path := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
	countryPath := filepath.Join(s.dataDir, "iptoasn-country.mmdb")
	asnPath := filepath.Join(s.dataDir, "iptoasn-asn.mmdb")

	if err := s.Close(); err != nil {
		return fmt.Errorf("failed to close existing GeoIP databases: %w", err)
	}

	cityIPv4DB, err := maxminddb.Open(cityIPv4Path)
	if err != nil {
		return fmt.Errorf("failed to load city IPv4 database: %w", err)
	}
	s.cityIPv4DB = cityIPv4DB

	cityIPv6DB, err := maxminddb.Open(cityIPv6Path)
	if err != nil {
		_ = s.cityIPv4DB.Close()
		return fmt.Errorf("failed to load city IPv6 database: %w", err)
	}
	s.cityIPv6DB = cityIPv6DB

	countryDB, err := maxminddb.Open(countryPath)
	if err != nil {
		_ = s.cityIPv4DB.Close()
		_ = s.cityIPv6DB.Close()
		return fmt.Errorf("failed to load country database: %w", err)
	}
	s.countryDB = countryDB

	asnDB, err := maxminddb.Open(asnPath)
	if err != nil {
		_ = s.cityIPv4DB.Close()
		_ = s.cityIPv6DB.Close()
		_ = s.countryDB.Close()
		return fmt.Errorf("failed to load ASN database: %w", err)
	}
	s.asnDB = asnDB

	s.available = true
	return nil
}

// DownloadDatabases downloads and verifies each GeoIP database before installing it.
func (s *GeoIPService) DownloadDatabases() error {
	databases := []struct {
		filename, url, checksumURL string
	}{
		{"dbip-city-ipv4.mmdb", cityIPv4URL, cityIPv4SumURL},
		{"dbip-city-ipv6.mmdb", cityIPv6URL, cityIPv6SumURL},
		{"iptoasn-country.mmdb", countryURL, countrySumURL},
		{"iptoasn-asn.mmdb", asnURL, asnSumURL},
	}

	for _, db := range databases {
		path := filepath.Join(s.dataDir, db.filename)
		log.Printf("  Downloading %s...", db.filename)
		if err := downloadVerifiedFile(path, db.url, db.checksumURL); err != nil {
			return fmt.Errorf("failed to download %s: %w", db.filename, err)
		}
	}

	log.Println("GeoIP databases downloaded successfully")
	return nil
}

// UpdateDatabases downloads fresh copies of all databases. Per AI.md PART 19,
// a failed refresh must never disable an already-working GeoIP service — on
// error, the previously loaded databases (if any) are left untouched.
func (s *GeoIPService) UpdateDatabases() error {
	log.Println("Updating GeoIP databases...")

	tempDir := filepath.Join(s.dataDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Warning: could not remove GeoIP temp directory: %v", err)
		}
	}()

	databases := []struct {
		filename, url, checksumURL string
	}{
		{"dbip-city-ipv4.mmdb", cityIPv4URL, cityIPv4SumURL},
		{"dbip-city-ipv6.mmdb", cityIPv6URL, cityIPv6SumURL},
		{"iptoasn-country.mmdb", countryURL, countrySumURL},
		{"iptoasn-asn.mmdb", asnURL, asnSumURL},
	}

	for _, db := range databases {
		tempPath := filepath.Join(tempDir, db.filename)
		log.Printf("  Downloading %s...", db.filename)
		if err := downloadVerifiedFile(tempPath, db.url, db.checksumURL); err != nil {
			return fmt.Errorf("failed to download %s: %w", db.filename, err)
		}
	}

	if err := s.Close(); err != nil {
		return fmt.Errorf("failed to close existing GeoIP databases: %w", err)
	}

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
func (s *GeoIPService) Lookup(ip net.IP) (*GeoLocation, error) {
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
		if err := cityDB.Lookup(ip, &record); err == nil && record.CountryCode != "" {
			location := &GeoLocation{
				IP:         ip.String(),
				Country:    record.CountryCode,
				Region:     record.State1,
				RegionName: record.State2,
				City:       record.City,
				Latitude:   record.Latitude,
				Longitude:  record.Longitude,
				TimeZone:   record.TimeZone,
				PostalCode: record.Postcode,
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
		IP:      ip.String(),
		Country: countryRecord.CountryCode,
	}
	s.addASNInfo(ip, location)
	return location, nil
}

// addASNInfo adds ASN information to the location
func (s *GeoIPService) addASNInfo(ip net.IP, location *GeoLocation) {
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
func (s *GeoIPService) LookupString(ipStr string) (*GeoLocation, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}
	return s.Lookup(ip)
}

// Close closes all GeoIP databases
func (s *GeoIPService) Close() error {
	s.available = false
	var closeErrors []error
	if s.cityIPv4DB != nil {
		closeErrors = append(closeErrors, s.cityIPv4DB.Close())
		s.cityIPv4DB = nil
	}
	if s.cityIPv6DB != nil {
		closeErrors = append(closeErrors, s.cityIPv6DB.Close())
		s.cityIPv6DB = nil
	}
	if s.countryDB != nil {
		closeErrors = append(closeErrors, s.countryDB.Close())
		s.countryDB = nil
	}
	if s.asnDB != nil {
		closeErrors = append(closeErrors, s.asnDB.Close())
		s.asnDB = nil
	}
	return errors.Join(closeErrors...)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadVerifiedFile(dest, url, checksumURL string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".geoip-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if cleanupErr := os.Remove(tmpPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove temporary GeoIP file: %w", cleanupErr))
		}
	}()

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close GeoIP download response: %w", closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmp, hasher), resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	checksum, err := fetchChecksum(client, checksumURL)
	if err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", checksum, actual)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("install verified GeoIP database: %w", err)
	}
	return nil
}

func fetchChecksum(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close GeoIP checksum response: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", fmt.Errorf("checksum response exceeds size limit")
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return "", fmt.Errorf("checksum response is malformed")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", fmt.Errorf("checksum response is malformed: %w", err)
	}
	return strings.ToLower(fields[0]), nil
}
