package geoip

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadVerifiedFile(t *testing.T) {
	content := []byte("verified GeoIP fixture")
	sum := sha256.Sum256(content)
	checksum := hex.EncodeToString(sum[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database":
			_, _ = w.Write(content)
		case "/checksum":
			_, _ = fmt.Fprintln(w, checksum+"  fixture.mmdb")
		case "/bad-checksum":
			_, _ = fmt.Fprintln(w, strings.Repeat("0", sha256.Size*2))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "fixture.mmdb")
	if err := downloadVerifiedFile(dest, server.URL+"/database", server.URL+"/checksum"); err != nil {
		t.Fatalf("valid database download failed: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read installed database: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("installed database = %q, want %q", got, content)
	}

	if err := downloadVerifiedFile(dest, server.URL+"/database", server.URL+"/bad-checksum"); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
	got, err = os.ReadFile(dest)
	if err != nil {
		t.Fatalf("existing verified database was lost after failed download: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("existing database changed after checksum failure: %q", got)
	}
}

func TestNewGeoIPService(t *testing.T) {
	tmpDir := t.TempDir()
	geoipDir := filepath.Join(tmpDir, "geoip")
	if err := os.MkdirAll(geoipDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dbip-city-ipv4.mmdb", "dbip-city-ipv6.mmdb", "iptoasn-country.mmdb", "iptoasn-asn.mmdb"} {
		if err := os.WriteFile(filepath.Join(geoipDir, name), []byte("invalid database"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := NewGeoIPService(tmpDir)
	if err != nil || svc == nil {
		t.Fatalf("NewGeoIPService must return a non-nil service when stored databases cannot be loaded: svc=%v err=%v", svc, err)
	}
	if svc.IsGeoIPAvailable() {
		t.Fatal("service with invalid local databases must be unavailable")
	}
}

func TestLookup(t *testing.T) {
	svc := &GeoIPService{}
	if _, err := svc.Lookup(net.ParseIP("8.8.8.8")); err == nil {
		t.Fatal("Lookup must report unavailable databases")
	}
}

func TestLookupString(t *testing.T) {
	svc := &GeoIPService{}
	if _, err := svc.LookupString("8.8.8.8"); err == nil {
		t.Fatal("LookupString must report unavailable databases")
	}
}

func TestNewGeoIPServiceFailOpen(t *testing.T) {
	// Per AI.md PART 19, GeoIP is a risk signal only — a database download
	// failure must never be fatal. Point the download URLs at an address
	// that cannot resolve/connect and confirm NewGeoIPService still succeeds
	// with a non-nil, degraded (unavailable) GeoIPService.
	origCityIPv4URL, origCityIPv6URL, origCountryURL, origASNURL := cityIPv4URL, cityIPv6URL, countryURL, asnURL
	defer func() {
		cityIPv4URL, cityIPv6URL, countryURL, asnURL = origCityIPv4URL, origCityIPv6URL, origCountryURL, origASNURL
	}()

	unreachable := "http://127.0.0.1:1/unreachable.mmdb"
	cityIPv4URL, cityIPv6URL, countryURL, asnURL = unreachable, unreachable, unreachable, unreachable

	tmpDir := t.TempDir()
	svc, err := NewGeoIPService(tmpDir)
	if err != nil {
		t.Fatalf("NewGeoIPService must fail open (nil error) on download failure, got: %v", err)
	}
	if svc == nil {
		t.Fatal("NewGeoIPService must return a non-nil GeoIPService even when downloads fail")
	}
	defer func() { _ = svc.Close() }()

	if svc.IsGeoIPAvailable() {
		t.Error("GeoIPService.IsGeoIPAvailable() must be false when databases failed to download")
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
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("GeoIP directory permissions = %o, want 0700", perm)
	}
}

func BenchmarkLookup(b *testing.B) {
	if testing.Short() {
		b.Skip("requires network: downloads GeoIP databases")
	}
	tmpDir := b.TempDir()
	svc, err := NewGeoIPService(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = svc.Close() }()

	ip := net.ParseIP("8.8.8.8")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Lookup(ip)
		if err != nil {
			b.Fatal(err)
		}
	}
}
