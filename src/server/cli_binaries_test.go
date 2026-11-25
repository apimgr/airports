package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func writeCLIReleaseFixture(t *testing.T, directory, binary string, contents []byte, checksum string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, binary), contents, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "version.txt"), []byte("1.2.3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "sha256.txt"), []byte(fmt.Sprintf("%s  %s\n", checksum, binary)), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCLIReleaseCatalogAndDownloadUseProvisionedFiles(t *testing.T) {
	const binary = "airports-cli-linux-amd64"
	assetBytes := []byte("cli fixture bytes")
	assetChecksum := sha256.Sum256(assetBytes)
	s := newTestServer(t)
	directory := filepath.Join(s.dataDir, "cli")
	writeCLIReleaseFixture(t, directory, binary, assetBytes, hex.EncodeToString(assetChecksum[:]))
	metadata := s.cliReleaseVersions(httptest.NewRequest(http.MethodGet, "https://airports.example.com/api/autodiscover", nil))
	entry := metadata["linux-amd64"]
	if entry["sha256"] != hex.EncodeToString(assetChecksum[:]) || entry["version"] != "1.2.3" {
		t.Fatalf("release metadata = %v, want version/checksum", entry)
	}
	if _, exists := entry["url"]; exists {
		t.Fatal("autodiscover CLI entry contains a URL; clients build it from their configured server")
	}

	r := httptest.NewRequest(http.MethodGet, "/cli/binaries/"+binary, nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("binary", binary)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeContext))
	w := httptest.NewRecorder()
	s.handleCLIBinaryDownload(w, r)
	if w.Code != http.StatusOK || !strings.EqualFold(w.Header().Get("Content-Type"), "application/octet-stream") {
		t.Fatalf("download status/content type = %d %q, want 200 application/octet-stream; body=%q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if w.Body.String() != string(assetBytes) {
		t.Fatalf("download body = %q, want %q", w.Body.String(), assetBytes)
	}
}

func TestCLIBinaryCatalogRejectsChecksumMismatchAndEmptyDirectoryIs404(t *testing.T) {
	s := newTestServer(t)
	if got := s.cliReleaseVersions(httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)); len(got) != 0 {
		t.Fatalf("empty release directory metadata = %v, want empty", got)
	}
	const binary = "airports-cli-linux-amd64"
	directory := filepath.Join(s.dataDir, "cli")
	writeCLIReleaseFixture(t, directory, binary, []byte("wrong payload"), strings.Repeat("a", 64))
	if got := s.cliReleaseVersions(httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)); len(got) != 0 {
		t.Fatalf("mismatched release metadata = %v, want empty", got)
	}
	r := httptest.NewRequest(http.MethodGet, "/cli/binaries/"+binary, nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("binary", binary)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeContext))
	w := httptest.NewRecorder()
	s.handleCLIBinaryDownload(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("mismatched binary download status = %d, want 404", w.Code)
	}
}

func TestCLIBinaryDownloadRequiresOperatorTokenWhenConfigured(t *testing.T) {
	s := newTestServer(t)
	s.config.CLI.BinaryDownload.RequireAuth = true
	s.config.Server.Token = "operator-secret"
	r := httptest.NewRequest(http.MethodGet, "/cli/binaries/airports-cli-linux-amd64", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("binary", "airports-cli-linux-amd64")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeContext))
	w := httptest.NewRecorder()
	s.handleCLIBinaryDownload(w, r)
	if w.Code != http.StatusForbidden || w.Body.Len() != 0 {
		t.Fatalf("unauthenticated download = %d %q, want empty 403", w.Code, w.Body.String())
	}
}

func TestParseCLIReleaseBinaryRejectsUnknownNames(t *testing.T) {
	if _, ok := parseCLIReleaseBinary("../../secret"); ok {
		t.Fatal("parseCLIReleaseBinary() accepted path traversal")
	}
	if platform, ok := parseCLIReleaseBinary("airports-cli-windows-amd64.exe"); !ok || platform != "windows-amd64" {
		t.Fatalf("parseCLIReleaseBinary() = %q, %v, want windows-amd64, true", platform, ok)
	}
}

func TestCLIChecksumsAreValidated(t *testing.T) {
	if !isHexSHA256(strings.Repeat("a", 64)) || isHexSHA256(strings.Repeat("z", 64)) {
		t.Fatal("isHexSHA256() did not distinguish hexadecimal from non-hexadecimal digests")
	}
	checksums := parseCLIChecksumFile([]byte(strings.Repeat("a", 64) + " *airports-cli-linux-amd64\nnot-a-checksum  bad\n"))
	if checksums["airports-cli-linux-amd64"] != strings.Repeat("a", 64) || len(checksums) != 1 {
		t.Fatalf("parsed checksums = %v", checksums)
	}
}
