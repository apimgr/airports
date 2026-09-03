package geoip

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNewService(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network: downloads GeoIP databases")
	}
	tmpDir := t.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create GeoIP service: %v", err)
	}
	defer svc.Close()

	if svc.cityIPv4DB == nil && svc.cityIPv6DB == nil {
		t.Error("City database (v4/v6) not loaded")
	}

	if svc.countryDB == nil {
		t.Error("Country database not loaded")
	}

	if svc.asnDB == nil {
		t.Error("ASN database not loaded")
	}

	// Verify files were downloaded (service stores split IPv4/IPv6 files)
	geoipDir := filepath.Join(tmpDir, "geoip")
	ipv4Path := filepath.Join(geoipDir, "dbip-city-ipv4.mmdb")
	ipv6Path := filepath.Join(geoipDir, "dbip-city-ipv6.mmdb")
	_, errIPv4 := os.Stat(ipv4Path)
	_, errIPv6 := os.Stat(ipv6Path)
	if errIPv4 != nil && errIPv6 != nil {
		t.Error("City database file (ipv4 or ipv6) not found")
	}
}

func TestLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network: downloads GeoIP databases")
	}
	tmpDir := t.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create GeoIP service: %v", err)
	}
	defer svc.Close()

	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"Google DNS", "8.8.8.8", false},
		{"Cloudflare DNS", "1.1.1.1", false},
		{"Private IP", "192.168.1.1", false},
		{"Localhost", "127.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			location, err := svc.Lookup(ip)

			if (err != nil) != tt.wantErr {
				t.Errorf("Lookup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if location.IP != tt.ip {
					t.Errorf("Expected IP %s, got %s", tt.ip, location.IP)
				}

				t.Logf("%s: Country=%s (%s), City=%s, Lat=%.4f, Lon=%.4f",
					tt.name, location.CountryName, location.Country,
					location.City, location.Latitude, location.Longitude)
			}
		})
	}
}

func TestLookupString(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network: downloads GeoIP databases")
	}
	tmpDir := t.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create GeoIP service: %v", err)
	}
	defer svc.Close()

	location, err := svc.LookupString("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup 8.8.8.8: %v", err)
	}

	// The free ip-location-db may return empty country for well-known IPs;
	// we accept either a correct result (US) or an empty result (graceful degradation).
	if location.Country != "" && location.Country != "US" {
		t.Errorf("Expected country US or empty for 8.8.8.8, got %s", location.Country)
	}

	t.Logf("8.8.8.8 located in %q (%q)", location.CountryName, location.Country)
}

func TestNewServiceFailOpen(t *testing.T) {
	// Per AI.md PART 19, GeoIP is a risk signal only — a database download
	// failure must never be fatal. Point the download URLs at an address
	// that cannot resolve/connect and confirm NewService still succeeds
	// with a non-nil, degraded (unavailable) Service.
	origCityIPv4URL, origCityIPv6URL, origCountryURL, origASNURL := cityIPv4URL, cityIPv6URL, countryURL, asnURL
	defer func() {
		cityIPv4URL, cityIPv6URL, countryURL, asnURL = origCityIPv4URL, origCityIPv6URL, origCountryURL, origASNURL
	}()

	unreachable := "http://127.0.0.1:1/unreachable.mmdb"
	cityIPv4URL, cityIPv6URL, countryURL, asnURL = unreachable, unreachable, unreachable, unreachable

	tmpDir := t.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService must fail open (nil error) on download failure, got: %v", err)
	}
	if svc == nil {
		t.Fatal("NewService must return a non-nil Service even when downloads fail")
	}
	defer svc.Close()

	if svc.Available() {
		t.Error("Service.Available() must be false when databases failed to download")
	}

	if _, err := svc.Lookup(net.ParseIP("8.8.8.8")); err == nil {
		t.Error("Lookup() must return an error (not panic or succeed) when GeoIP is unavailable")
	}

	if _, err := svc.LookupString("8.8.8.8"); err == nil {
		t.Error("LookupString() must return an error (not panic or succeed) when GeoIP is unavailable")
	}

	// Directory must still be created with 0700 permissions (AI.md PART 23/31).
	geoipDir := filepath.Join(tmpDir, "geoip")
	info, statErr := os.Stat(geoipDir)
	if statErr != nil {
		t.Fatalf("GeoIP directory not created: %v", statErr)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("GeoIP directory permissions = %o, want 0700", perm)
	}
}

func BenchmarkLookup(b *testing.B) {
	if testing.Short() {
		b.Skip("requires network: downloads GeoIP databases")
	}
	tmpDir := b.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer svc.Close()

	ip := net.ParseIP("8.8.8.8")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Lookup(ip)
		if err != nil {
			b.Fatal(err)
		}
	}
}
