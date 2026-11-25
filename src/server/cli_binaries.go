package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/logging"
	"github.com/go-chi/chi/v5"
)

const cliBinaryMaxSize = 512 << 20

var cliReleasePlatforms = []string{
	"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64",
	"windows-amd64", "windows-arm64", "freebsd-amd64", "freebsd-arm64",
}

type cliReleaseAsset struct {
	name    string
	version string
	sum     string
	path    string
}

func (s *Server) cliBinaryDirectory() string {
	configured := strings.TrimSpace(s.config.CLI.BinaryDownload.Dir)
	if configured == "" {
		return filepath.Join(s.dataDir, "cli")
	}
	configured = strings.ReplaceAll(configured, "{data_dir}", s.dataDir)
	if !filepath.IsAbs(configured) {
		configured = filepath.Join(s.dataDir, configured)
	}
	return filepath.Clean(configured)
}

func (s *Server) loadCLIRelease(context.Context) (map[string]cliReleaseAsset, error) {
	directory := s.cliBinaryDirectory()
	info, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return map[string]cliReleaseAsset{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect CLI binary directory: %w", err)
	}
	if !info.IsDir() {
		return map[string]cliReleaseAsset{}, nil
	}

	versionBytes, err := os.ReadFile(filepath.Join(directory, "version.txt"))
	if err != nil {
		return map[string]cliReleaseAsset{}, nil
	}
	version := strings.TrimSpace(strings.SplitN(string(versionBytes), "\n", 2)[0])
	if version == "" {
		return map[string]cliReleaseAsset{}, nil
	}
	checksumBytes, err := os.ReadFile(filepath.Join(directory, "sha256.txt"))
	if err != nil {
		return map[string]cliReleaseAsset{}, nil
	}
	checksums := parseCLIChecksumFile(checksumBytes)
	assets := make(map[string]cliReleaseAsset)
	for _, platform := range cliReleasePlatforms {
		name := "airports-cli-" + platform
		if strings.HasPrefix(platform, "windows-") {
			name += ".exe"
		}
		binaryPath := filepath.Join(directory, name)
		fileInfo, statErr := os.Stat(binaryPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() < 1 || fileInfo.Size() > cliBinaryMaxSize {
			continue
		}
		expected := checksums[name]
		actual, hashErr := hashCLIFile(binaryPath)
		if expected == "" || hashErr != nil || !strings.EqualFold(actual, expected) {
			log.Printf("Warning: CLI release binary checksum mismatch or missing checksum: %s", name)
			continue
		}
		assets[platform] = cliReleaseAsset{name: name, version: version, sum: actual, path: binaryPath}
	}
	return assets, nil
}

func parseCLIChecksumFile(contents []byte) map[string]string {
	checksums := make(map[string]string)
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !isHexSHA256(fields[0]) {
			continue
		}
		name := filepath.Base(strings.TrimLeft(fields[1], "*"))
		checksums[name] = strings.ToLower(fields[0])
	}
	return checksums
}

func hashCLIFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	return hashCLIReader(file)
}

func hashCLIReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(reader, cliBinaryMaxSize+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Server) cliReleaseVersions(r *http.Request) map[string]map[string]string {
	assets, err := s.loadCLIRelease(r.Context())
	if err != nil {
		log.Printf("Warning: could not inspect CLI release directory: %v", err)
		return map[string]map[string]string{}
	}
	result := make(map[string]map[string]string, len(assets))
	for platform, asset := range assets {
		result[platform] = map[string]string{"version": asset.version, "sha256": asset.sum}
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
	assets, err := s.loadCLIRelease(r.Context())
	if err != nil {
		log.Printf("Warning: could not inspect CLI release directory: %v", err)
		http.NotFound(w, r)
		return
	}
	asset, ok := assets[platform]
	if !ok || asset.name != binary {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(asset.path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > cliBinaryMaxSize {
		http.NotFound(w, r)
		return
	}
	actual, err := hashCLIReader(file)
	if err != nil || !strings.EqualFold(actual, asset.sum) {
		log.Printf("Warning: CLI release binary checksum mismatch: %s", asset.name)
		http.NotFound(w, r)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+asset.name+`"`)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if _, err := io.Copy(w, io.LimitReader(file, cliBinaryMaxSize+1)); err != nil {
		return
	}
	s.writeCLIAudit("cli.update_started", "os_arch="+strconv.Quote(platform)+" version="+strconv.Quote(asset.version)+" client_ip="+strconv.Quote(realIP(r)))
}

func (s *Server) writeCLIAudit(event, fields string) {
	if s == nil || s.auditLogger == nil {
		return
	}
	if err := s.auditLogger.WriteAudit(logging.AuditCategoryServer, "event="+event+" "+fields); err != nil {
		log.Printf("Warning: could not write CLI audit event %s: %v", event, err)
	}
}

func isHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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
