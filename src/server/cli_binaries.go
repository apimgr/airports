package server

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	cliReleaseCacheTTL = 10 * time.Minute
	cliReleaseMaxSize  = 2 << 20
	cliBinaryMaxSize   = 512 << 20
	cliReleaseRepo     = "apimgr/airports"
)

var cliReleasePlatforms = []string{
	"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64",
	"windows-amd64", "windows-arm64", "freebsd-amd64", "freebsd-arm64",
}

type cliReleaseCache struct {
	version string
	expires time.Time
	assets  map[string]cliReleaseAsset
}

type cliReleaseAsset struct {
	name string
	url  string
	sum  string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (s *Server) loadCLIRelease(ctx context.Context) (*cliReleaseCache, error) {
	s.cliReleaseCacheMu.Lock()
	if s.cliReleaseCache != nil && time.Now().Before(s.cliReleaseCache.expires) {
		cached := s.cliReleaseCache
		s.cliReleaseCacheMu.Unlock()
		return cached, nil
	}
	s.cliReleaseCacheMu.Unlock()

	if s.cliReleaseAPIBase == "" {
		s.cliReleaseAPIBase = "https://api.github.com"
	}
	if s.cliReleaseHTTPClient == nil {
		s.cliReleaseHTTPClient = &http.Client{Timeout: 12 * time.Second}
	}
	client := s.cliReleaseHTTPClient
	version := strings.TrimSpace(Version)
	if version == "" || version == "unknown" || version == "dev" {
		return nil, errors.New("CLI release version is unavailable")
	}
	tags := []string{version}
	if !strings.HasPrefix(version, "v") {
		tags = append(tags, "v"+version)
	} else {
		tags = append(tags, strings.TrimPrefix(version, "v"))
	}
	base, err := url.Parse(strings.TrimRight(s.cliReleaseAPIBase, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub API base URL: %w", err)
	}
	var release githubRelease
	var found bool
	for _, tag := range tags {
		endpoint := *base
		endpoint.Path = path.Join(endpoint.Path, "repos", cliReleaseRepo, "releases", "tags", tag)
		request, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if reqErr != nil {
			return nil, reqErr
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("User-Agent", "airports/"+version)
		response, reqErr := doCLIReleaseRequest(client, request)
		if reqErr != nil {
			return nil, fmt.Errorf("fetch GitHub release metadata: %w", reqErr)
		}
		if response.StatusCode == http.StatusNotFound {
			_ = response.Body.Close()
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("GitHub release metadata returned HTTP %d", response.StatusCode)
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, cliReleaseMaxSize)).Decode(&release)
		_ = response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitHub release metadata: %w", decodeErr)
		}
		if release.Draft {
			return nil, errors.New("matching GitHub release is a draft")
		}
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("no GitHub release matches server version %q", version)
	}
	assets := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		if asset.Name != "" && asset.BrowserDownloadURL != "" {
			assets[asset.Name] = asset.BrowserDownloadURL
		}
	}
	checksumURL := assets["sha256.txt"]
	if checksumURL == "" {
		return nil, errors.New("matching GitHub release has no sha256.txt asset")
	}
	if !validCLIReleaseAssetURL(checksumURL) {
		return nil, errors.New("matching GitHub release has an invalid checksum asset URL")
	}
	checksums, err := s.fetchReleaseChecksums(ctx, checksumURL)
	if err != nil {
		return nil, err
	}
	catalog := &cliReleaseCache{version: version, expires: time.Now().Add(cliReleaseCacheTTL), assets: make(map[string]cliReleaseAsset)}
	for _, platform := range cliReleasePlatforms {
		name := "airports-cli-" + platform
		if strings.HasPrefix(platform, "windows-") {
			name += ".exe"
		}
		downloadURL := assets[name]
		checksum := checksums[name]
		if downloadURL != "" && checksum != "" && validCLIReleaseAssetURL(downloadURL) {
			catalog.assets[platform] = cliReleaseAsset{name: name, url: downloadURL, sum: checksum}
		}
	}
	s.cliReleaseCacheMu.Lock()
	s.cliReleaseCache = catalog
	s.cliReleaseCacheMu.Unlock()
	return catalog, nil
}

func (s *Server) fetchReleaseChecksums(ctx context.Context, checksumURL string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "airports/"+Version)
	response, err := doCLIReleaseRequest(s.cliReleaseHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub release checksums: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release checksums returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, cliReleaseMaxSize))
	if err != nil {
		return nil, fmt.Errorf("read GitHub release checksums: %w", err)
	}
	checksums := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) != 64 || !isHexSHA256(fields[0]) {
			continue
		}
		name := filepath.Base(strings.TrimLeft(fields[1], "*"))
		checksums[name] = strings.ToLower(fields[0])
	}
	return checksums, nil
}

func (s *Server) cliReleaseVersions(r *http.Request) map[string]map[string]string {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	catalog, err := s.loadCLIRelease(ctx)
	if err != nil {
		return map[string]map[string]string{}
	}
	result := make(map[string]map[string]string, len(catalog.assets))
	base := strings.TrimSuffix(s.publicBaseURL(r), "/")
	for platform, asset := range catalog.assets {
		result[platform] = map[string]string{
			"version": catalog.version,
			"url":     base + "/cli/binaries/" + asset.name,
			"sha256":  asset.sum,
		}
	}
	return result
}

func (s *Server) handleCLIBinaryDownload(w http.ResponseWriter, r *http.Request) {
	if s.config.CLI.BinaryDownload.RequireAuth {
		token := requestAuthToken(r)
		if s.config.Server.Token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.config.Server.Token)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	binary := chi.URLParam(r, "binary")
	platform, ok := parseCLIReleaseBinary(binary)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	catalog, err := s.loadCLIRelease(ctx)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusBadGateway, "CLI_RELEASE_UNAVAILABLE", "The matching CLI release is unavailable.")
		return
	}
	asset, ok := catalog.assets[platform]
	if !ok || asset.name != binary {
		http.NotFound(w, r)
		return
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.url, nil)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusBadGateway, "CLI_RELEASE_UNAVAILABLE", "The matching CLI release is unavailable.")
		return
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "airports/"+Version)
	response, err := doCLIReleaseRequest(s.cliReleaseHTTPClient, request)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusBadGateway, "CLI_RELEASE_UNAVAILABLE", "The matching CLI release is unavailable.")
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.ContentLength > cliBinaryMaxSize {
		s.writeErrorJSON(w, r, http.StatusBadGateway, "CLI_RELEASE_UNAVAILABLE", "The matching CLI release is unavailable.")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+asset.name+`"`)
	w.Header().Set("Cache-Control", "public, max-age=300")
	if response.ContentLength >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", response.ContentLength))
	}
	if _, err := io.Copy(w, io.LimitReader(response.Body, cliBinaryMaxSize+1)); err != nil {
		return
	}
}

func isHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCLIReleaseAssetURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme == "http" && isLocalCLIReleaseHost(host) {
		return true
	}
	if parsed.Scheme != "https" {
		return false
	}
	return host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

func isLocalCLIReleaseHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func doCLIReleaseRequest(client *http.Client, request *http.Request) (*http.Response, error) {
	if !validCLIReleaseAssetURL(request.URL.String()) {
		return nil, errors.New("refusing CLI release request to an untrusted URL")
	}
	guarded := *client
	priorRedirect := client.CheckRedirect
	guarded.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if !validCLIReleaseAssetURL(next.URL.String()) {
			return errors.New("refusing CLI release redirect to an untrusted URL")
		}
		if priorRedirect != nil {
			return priorRedirect(next, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 CLI release redirects")
		}
		return nil
	}
	return guarded.Do(request)
}

func parseCLIReleaseBinary(binary string) (string, bool) {
	for _, platform := range cliReleasePlatforms {
		name := "airports-cli-" + platform
		if strings.HasPrefix(platform, "windows-") {
			name += ".exe"
		}
		if binary == name {
			return platform, true
		}
	}
	return "", false
}
