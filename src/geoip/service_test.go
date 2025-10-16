package geoip

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNewService(t *testing.T) {
	tmpDir := t.TempDir()
	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create GeoIP service: %v", err)
	}
	defer svc.Close()

	if svc.cityDB == nil {
		t.Error("City database not loaded")
	}

	if svc.countryDB == nil {
		t.Error("Country database not loaded")
	}

	if svc.asnDB == nil {
		t.Error("ASN database not loaded")
	}

	// Verify files were downloaded
	geoipDir := filepath.Join(tmpDir, "geoip")
	if _, err := os.Stat(filepath.Join(geoipDir, "GeoLite2-City.mmdb")); err != nil {
		t.Error("City database file not found")
	}
}

func TestLookup(t *testing.T) {
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

	if location.Country != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", location.Country)
	}

	t.Logf("8.8.8.8 located in %s (%s)", location.CountryName, location.Country)
}

func TestExtractIPFromRequest(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "Direct connection",
			remoteAddr: "1.2.3.4:12345",
			expectedIP: "1.2.3.4",
		},
		{
			name:          "X-Forwarded-For single",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "8.8.8.8",
			expectedIP:    "8.8.8.8",
		},
		{
			name:          "X-Forwarded-For multiple",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "8.8.8.8,1.1.1.1,192.168.1.1",
			expectedIP:    "8.8.8.8",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			xRealIP:    "8.8.8.8",
			expectedIP: "8.8.8.8",
		},
		{
			name:          "X-Forwarded-For priority over X-Real-IP",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "1.1.1.1",
			xRealIP:       "8.8.8.8",
			expectedIP:    "1.1.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := ExtractIPFromRequest(tt.remoteAddr, tt.xForwardedFor, tt.xRealIP)
			if ip != tt.expectedIP {
				t.Errorf("Expected %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func BenchmarkLookup(b *testing.B) {
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
