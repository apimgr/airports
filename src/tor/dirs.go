package tor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apimgr/airports/src/path"
)

// Dirs holds every filesystem path the Tor manager reads from or writes to,
// resolved once at startup per AI.md PART 31 "Storage Locations".
type Dirs struct {
	// ConfigDir is {config_dir}/tor/ — holds the generated torrc.
	ConfigDir string
	// DataDir is {data_dir}/tor/ — Tor's own runtime data directory.
	DataDir string
	// SiteDir is {data_dir}/tor/site/ — the hidden service key directory.
	SiteDir string
	// TorrcPath is {config_dir}/tor/torrc.
	TorrcPath string
	// KeyPath is {data_dir}/tor/site/hs_ed25519_secret_key — the
	// persisted key file this package writes/reads (stored as a
	// "type:base64blob" string produced by bine's control.Key).
	KeyPath string
	// LogFile is {log_dir}/tor.log.
	LogFile string
}

// resolveDirs computes all Tor paths for projectName using the shared
// paths package, mirroring GetSSLDir's config-dir pattern for the torrc
// location and GetTorDir for the data-dir location.
func resolveDirs(projectName string) Dirs {
	configDir, _, logDir := paths.GetDefaultDirs(projectName)
	dataDir := paths.GetTorDir(projectName)

	torConfigDir := filepath.Join(configDir, "tor")
	siteDir := filepath.Join(dataDir, "site")

	return Dirs{
		ConfigDir: torConfigDir,
		DataDir:   dataDir,
		SiteDir:   siteDir,
		TorrcPath: filepath.Join(torConfigDir, "torrc"),
		KeyPath:   filepath.Join(siteDir, "hs_ed25519_secret_key"),
		LogFile:   filepath.Join(logDir, "tor.log"),
	}
}

// ensureDir creates dir if missing and idempotently enforces 0700
// permissions and current-user ownership per AI.md PART 31 "Runtime
// Directory Handling". Chown is skipped on Windows, which relies on
// inherited ACLs from the user profile instead.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod dir %s: %w", dir, err)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chown(dir, os.Getuid(), os.Getgid()); err != nil {
		return fmt.Errorf("chown dir %s: %w", dir, err)
	}
	return nil
}

// ensureDirs creates and enforces permissions on all three Tor directories
// (config, data, site). Safe to call on every startup.
func ensureDirs(d Dirs) error {
	for _, dir := range []string{d.ConfigDir, d.DataDir, d.SiteDir} {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// ensureFile writes content to path with 0600 permissions if the file does
// not already exist, then idempotently enforces those permissions and
// ownership either way. Returns true if the file was newly created.
func ensureFile(path string, content []byte) (bool, error) {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return false, err
	}

	created := false
	if _, err := os.Stat(path); err != nil {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return false, fmt.Errorf("write file %s: %w", path, err)
		}
		created = true
	}

	if err := os.Chmod(path, 0o600); err != nil {
		return created, fmt.Errorf("chmod file %s: %w", path, err)
	}
	if runtime.GOOS == "windows" {
		return created, nil
	}
	if err := os.Chown(path, os.Getuid(), os.Getgid()); err != nil {
		return created, fmt.Errorf("chown file %s: %w", path, err)
	}
	return created, nil
}

// writeFile overwrites path with content unconditionally (used when the
// operator changes Tor settings and the torrc must be regenerated), then
// enforces the same 0600 permissions/ownership as ensureFile.
func writeFile(path string, content []byte) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod file %s: %w", path, err)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chown(path, os.Getuid(), os.Getgid()); err != nil {
		return fmt.Errorf("chown file %s: %w", path, err)
	}
	return nil
}
