package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/mod/semver"
)

type cliRelease struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

type cliAutodiscover struct {
	Versions   map[string]cliRelease `json:"cli_versions"`
	MinVersion string                `json:"cli_min_version"`
}

func checkCLIRelease(ctx context.Context, c *client) (cliAutodiscover, cliRelease, error) {
	var result cliAutodiscover
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return result, cliRelease{}, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/autodiscover"
	base.RawQuery, base.Fragment = "", ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return result, cliRelease{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "airports-cli/"+Version)
	req.Header.Set("X-Request-ID", uuid.NewString())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.do(req)
	if err != nil {
		return result, cliRelease{}, fmt.Errorf("autodiscover request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, cliRelease{}, fmt.Errorf("autodiscover returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return result, cliRelease{}, fmt.Errorf("decode autodiscover response: %w", err)
	}
	release := result.Versions[runtime.GOOS+"-"+runtime.GOARCH]
	return result, release, nil
}

func isNewerCLIVersion(current, available string) (bool, error) {
	current = normalizeSemver(current)
	available = normalizeSemver(available)
	if !semver.IsValid(current) || !semver.IsValid(available) {
		return false, fmt.Errorf("invalid CLI version metadata: current=%q available=%q", current, available)
	}
	return semver.Compare(current, available) < 0, nil
}

func normalizeSemver(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "dev") || strings.EqualFold(value, "unknown") {
		return "v0.0.0-dev"
	}
	if value != "" && !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return value
}

func downloadAndReplaceCLI(ctx context.Context, c *client, release cliRelease) error {
	if strings.TrimSpace(release.SHA256) == "" {
		return errors.New("server did not provide a SHA-256 checksum; update refused")
	}
	expected, err := hex.DecodeString(strings.TrimSpace(release.SHA256))
	if err != nil || len(expected) != sha256.Size {
		return errors.New("server provided an invalid SHA-256 checksum; update refused")
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	if !strings.EqualFold(base.Scheme, "https") {
		return errors.New("CLI updates require an HTTPS server URL")
	}
	binaryURL := release.URL
	if binaryURL == "" {
		base.Path = strings.TrimRight(base.Path, "/") + "/cli/binaries/airports-cli-" + runtime.GOOS + "-" + runtime.GOARCH
		base.RawQuery, base.Fragment = "", ""
		binaryURL = base.String()
	} else {
		parsed, parseErr := url.Parse(binaryURL)
		if parseErr != nil {
			return fmt.Errorf("invalid CLI download URL: %w", parseErr)
		}
		parsed = base.ResolveReference(parsed)
		if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Host, base.Host) {
			return errors.New("CLI download URL must use HTTPS on the configured server")
		}
		binaryURL = parsed.String()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, binaryURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "airports-cli/"+Version)
	request.Header.Set("X-Request-ID", uuid.NewString())
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.do(request)
	if err != nil {
		return fmt.Errorf("download CLI update: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("CLI download returned HTTP %d", response.StatusCode)
	}
	if response.Request != nil && response.Request.URL != nil &&
		(!strings.EqualFold(response.Request.URL.Scheme, "https") || !strings.EqualFold(response.Request.URL.Host, base.Host)) {
		return errors.New("CLI download redirected away from the configured HTTPS server")
	}

	tempBase := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(tempBase, 0700); err != nil {
		return fmt.Errorf("create project temp namespace: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempBase, "airports-")
	if err != nil {
		return fmt.Errorf("create update temp directory: %w", err)
	}
	tempPath := filepath.Join(tempDir, "cli.update.tmp")
	defer func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(tempDir)
	}()
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return fmt.Errorf("create update file: %w", err)
	}
	hasher := sha256.New()
	const maximumCLIBinarySize = 512 << 20
	written, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(response.Body, maximumCLIBinarySize+1))
	closeErr := tempFile.Close()
	if copyErr != nil {
		return fmt.Errorf("save CLI update: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close CLI update file: %w", closeErr)
	}
	if written == 0 || written > maximumCLIBinarySize {
		return errors.New("CLI update size is invalid")
	}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), hex.EncodeToString(expected)) {
		return errors.New("CLI update checksum mismatch; update refused")
	}

	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current CLI path: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(currentPath); resolveErr == nil {
		currentPath = resolved
	}
	if err := os.Chmod(tempPath, 0700); err != nil {
		return fmt.Errorf("set update permissions: %w", err)
	}
	if err := replaceCLI(currentPath, tempPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("you do not have permission to update %s; ask your admin or move the binary to a writable path", currentPath)
		}
		return fmt.Errorf("replace CLI binary: %w", err)
	}
	return reexecCLI(currentPath, os.Args, os.Environ())
}

func promptForCLIUpdate(version string) bool {
	if !isTTY(os.Stdin) || !isTTY(os.Stderr) {
		return false
	}
	fmt.Fprintf(os.Stderr, "CLI version %s is available. Update now? [y/N] ", version)
	var answer string
	if _, err := fmt.Fscan(os.Stdin, &answer); err != nil {
		return false
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

func updateSetting(cfg *cliConfig) bool {
	if cfg == nil {
		return false
	}
	return cfg.Update.Auto
}

func isInteractiveCLI() bool {
	return isTTY(os.Stdin) && isTTY(os.Stderr)
}

func checkAndMaybeUpdateCLI(c *client, cfg *cliConfig, action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	metadata, release, err := checkCLIRelease(ctx, c)
	if err != nil {
		return err
	}
	minRequired := false
	if metadata.MinVersion != "" {
		tooOld, compareErr := isNewerCLIVersion(Version, metadata.MinVersion)
		if compareErr != nil {
			return compareErr
		}
		minRequired = tooOld
	}
	newer := false
	if release.Version != "" {
		newer, err = isNewerCLIVersion(Version, release.Version)
		if err != nil {
			return err
		}
	}
	if action == "check" {
		if newer {
			_, _ = fmt.Fprintf(os.Stdout, "CLI update available: %s\n", release.Version)
		} else if minRequired {
			_, _ = fmt.Fprintf(os.Stdout, "CLI update required: server minimum is %s\n", metadata.MinVersion)
		} else if release.Version == "" {
			_, _ = fmt.Fprintf(os.Stdout, "No CLI release is available for %s-%s.\n", runtime.GOOS, runtime.GOARCH)
		} else {
			_, _ = fmt.Fprintln(os.Stdout, "CLI is up to date.")
		}
		return nil
	}
	if minRequired && action != "yes" {
		return fmt.Errorf("this CLI is too old; the server requires %s — run '%s --update yes' to upgrade", metadata.MinVersion, cliBinaryDisplayName())
	}
	if minRequired && release.Version == "" {
		return fmt.Errorf("this CLI is too old; the server requires %s but has no downloadable CLI release for %s-%s", metadata.MinVersion, runtime.GOOS, runtime.GOARCH)
	}
	if minRequired {
		releaseBelowMinimum, compareErr := isNewerCLIVersion(release.Version, metadata.MinVersion)
		if compareErr != nil {
			return compareErr
		}
		if releaseBelowMinimum {
			return fmt.Errorf("server CLI release %s is below its required minimum %s", release.Version, metadata.MinVersion)
		}
	}
	if !newer {
		return nil
	}
	shouldUpdate := action == "yes"
	if shouldUpdate && !isInteractiveCLI() {
		auto := updateSetting(cfg)
		if !auto {
			return errors.New("non-interactive CLI updates require update.auto: true in cli.yml")
		}
	}
	if action == "" && isInteractiveCLI() {
		shouldUpdate = promptForCLIUpdate(release.Version)
	}
	if !shouldUpdate {
		if minRequired {
			return fmt.Errorf("this CLI is too old; the server requires %s — run 'airports-cli --update yes' to upgrade", metadata.MinVersion)
		}
		return nil
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return downloadAndReplaceCLI(ctx, c, release)
}

func cliBinaryDisplayName() string {
	if len(os.Args) > 0 {
		return filepath.Base(os.Args[0])
	}
	return "airports-cli"
}
