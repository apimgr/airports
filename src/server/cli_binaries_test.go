package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCLIReleaseCatalogAndDownloadUseGitHubReleaseAssets(t *testing.T) {
	const binary = "airports-cli-linux-amd64"
	assetBytes := []byte("cli fixture bytes")
	assetChecksum := sha256.Sum256(assetBytes)
	checksumText := fmt.Sprintf("%s  %s\n", hex.EncodeToString(assetChecksum[:]), binary)
	var github *httptest.Server
	github = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apimgr/airports/releases/tags/1.2.3":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"tag_name": "v1.2.3",
				"assets": []map[string]string{
					{"name": binary, "browser_download_url": github.URL + "/assets/" + binary},
					{"name": "sha256.txt", "browser_download_url": github.URL + "/assets/sha256.txt"},
				},
			})
		case "/assets/sha256.txt":
			_, _ = w.Write([]byte(checksumText))
		case "/assets/" + binary:
			_, _ = w.Write(assetBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()

	previousVersion := Version
	Version = "1.2.3"
	defer func() { Version = previousVersion }()
	s := newTestServer(t)
	s.cliReleaseAPIBase = github.URL
	metadata := s.cliReleaseVersions(httptest.NewRequest(http.MethodGet, "http://airports.example.com/api/autodiscover", nil))
	if got := metadata["linux-amd64"]["sha256"]; got != hex.EncodeToString(assetChecksum[:]) {
		t.Fatalf("release checksum = %q, want %q", got, hex.EncodeToString(assetChecksum[:]))
	}
	if !strings.HasSuffix(metadata["linux-amd64"]["url"], "/cli/binaries/"+binary) {
		t.Fatalf("release URL = %q, want local CLI binary route", metadata["linux-amd64"]["url"])
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
	if !strings.EqualFold(w.Body.String(), string(assetBytes)) {
		t.Fatalf("download body = %q, want %q", w.Body.String(), assetBytes)
	}
}

func TestCLIBinaryDownloadRequiresOperatorTokenWhenConfigured(t *testing.T) {
	s := newTestServer(t)
	s.config.CLI.BinaryDownload.RequireAuth = true
	s.config.Server.Token = "operator-secret"
	r := httptest.NewRequest(http.MethodGet, "/cli/binaries/airports-cli-linux-amd64", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("binary", "airports-cli-linux-amd64")
	r = r.WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, routeContext))
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

func TestCLIReleaseAssetURLsAndChecksumsAreValidated(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "https://github.com/apimgr/airports/releases/download/v1/asset", valid: true},
		{value: "https://release-assets.githubusercontent.com/asset", valid: true},
		{value: "http://127.0.0.1:1234/asset", valid: true},
		{value: "http://github.com/apimgr/airports/asset", valid: false},
		{value: "https://evil.example/asset", valid: false},
		{value: "https://github.com.evil.example/asset", valid: false},
		{value: "https://user@github.com/asset", valid: false},
	} {
		if got := validCLIReleaseAssetURL(test.value); got != test.valid {
			t.Errorf("validCLIReleaseAssetURL(%q) = %t, want %t", test.value, got, test.valid)
		}
	}
	if !isHexSHA256(strings.Repeat("a", 64)) || isHexSHA256(strings.Repeat("z", 64)) {
		t.Fatal("isHexSHA256() did not distinguish hexadecimal from non-hexadecimal digests")
	}
}
