package paths

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

const (
	// Organization name for directory structure
	OrgName = "apimgr"
)

// goosFunc returns the OS target for every Get*Dir branch below, held in a
// package-level variable (defaulting to runtime.GOOS) so tests can force
// each Windows/Darwin/BSD/Linux branch on a single CI platform — those
// branches are otherwise unreachable when compiled and tested on Linux-only
// CI, since runtime.GOOS is fixed at compile time.
var goosFunc = func() string {
	return runtime.GOOS
}

// isBSD reports whether the current goosFunc() target is one of the BSD
// family targets this project builds for (freebsd/openbsd/netbsd/dragonfly).
// Per AI.md PART 4, BSD has its own privileged path set distinct from Linux.
func isBSD() bool {
	switch goosFunc() {
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return true
	default:
		return false
	}
}

// GetDefaultDirs returns OS-specific default directories based on privileges
// Uses {org}/{name} structure: /etc/apimgr/airports/, ~/.config/apimgr/airports/
// Per AI.md PART 4, Docker/container paths (/config, /data) take priority
// over the host-OS privileged/user split whenever running inside a container.
func GetDefaultDirs(projectName string) (configDir, dataDir, logsDir string) {
	if IsRunningInContainer() {
		configDir = filepath.Join("/config", projectName)
		dataDir = filepath.Join("/data", projectName)
		logsDir = filepath.Join("/data", "log", projectName)
		return configDir, dataDir, logsDir
	}

	if isRootOrAdmin() {
		// Running with elevated privileges - use system directories with org/name structure
		switch {
		case goosFunc() == "windows":
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			configDir = filepath.Join(programData, OrgName, projectName)
			dataDir = filepath.Join(programData, OrgName, projectName, "data")
			logsDir = filepath.Join(programData, OrgName, projectName, "logs")
		case goosFunc() == "darwin":
			configDir = filepath.Join("/Library/Application Support", OrgName, projectName)
			dataDir = filepath.Join("/Library/Application Support", OrgName, projectName, "data")
			logsDir = filepath.Join("/Library/Logs", OrgName, projectName)
		case isBSD():
			configDir = filepath.Join("/usr/local/etc", OrgName, projectName)
			dataDir = filepath.Join("/var/db", OrgName, projectName)
			logsDir = filepath.Join("/var/log", OrgName, projectName)
		default: // Linux
			configDir = filepath.Join("/etc", OrgName, projectName)
			dataDir = filepath.Join("/var/lib", OrgName, projectName)
			logsDir = filepath.Join("/var/log", OrgName, projectName)
		}
	} else {
		// Running as regular user - use user directories with org/name structure
		var homeDir string
		currentUser, err := user.Current()
		if err == nil {
			homeDir = currentUser.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
			if homeDir == "" {
				homeDir = os.Getenv("USERPROFILE") // Windows fallback
			}
		}

		switch goosFunc() {
		case "windows":
			appData := os.Getenv("APPDATA")
			if appData == "" {
				appData = filepath.Join(homeDir, "AppData", "Roaming")
			}
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData == "" {
				localAppData = filepath.Join(homeDir, "AppData", "Local")
			}
			configDir = filepath.Join(appData, OrgName, projectName)
			dataDir = filepath.Join(localAppData, OrgName, projectName)
			logsDir = filepath.Join(localAppData, OrgName, projectName, "logs")
		case "darwin": // macOS
			configDir = filepath.Join(homeDir, "Library", "Application Support", OrgName, projectName)
			dataDir = filepath.Join(homeDir, "Library", "Application Support", OrgName, projectName)
			logsDir = filepath.Join(homeDir, "Library", "Logs", OrgName, projectName)
		default: // Linux, BSD
			// Follow XDG Base Directory specification with org/name structure
			xdgConfig := os.Getenv("XDG_CONFIG_HOME")
			if xdgConfig == "" {
				xdgConfig = filepath.Join(homeDir, ".config")
			}
			xdgData := os.Getenv("XDG_DATA_HOME")
			if xdgData == "" {
				xdgData = filepath.Join(homeDir, ".local", "share")
			}

			configDir = filepath.Join(xdgConfig, OrgName, projectName)
			dataDir = filepath.Join(xdgData, OrgName, projectName)
			logsDir = filepath.Join(homeDir, ".local", "log", OrgName, projectName)
		}
	}

	return configDir, dataDir, logsDir
}

// EnsureDir creates a directory if it doesn't exist.
// Uses 0700 per AI.md PART 23/31 (config/data/log dirs are owner-only).
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0700)
}

// EnsureDirs creates all required directories
func EnsureDirs(configDir, dataDir, logsDir string) error {
	if err := EnsureDir(configDir); err != nil {
		return err
	}
	if err := EnsureDir(dataDir); err != nil {
		return err
	}
	if err := EnsureDir(logsDir); err != nil {
		return err
	}
	return nil
}

// IsRunningInContainer checks if running inside this project's own runtime
// container. A generic container signal (/.dockerenv, /run/.containerenv)
// alone is not enough — the casjaysdev/go:latest build/test toolchain image
// also runs tini as PID 1 and would false-positive. Per docker-rules.md the
// app's own runtime container always mounts exactly /config and /data, so
// isAppRuntimeContainer() requires both as a second, more specific signal.
func IsRunningInContainer() bool {
	return isRunningInContainerFunc()
}

// isRunningInContainerFunc is the real detection logic behind
// IsRunningInContainer, held in a package-level variable so tests can
// override it to exercise every Get*Dir helper's container-path branch
// without needing an actual container (tini PID 1 + real /config + /data +
// installed binary are otherwise impossible to fake safely in a unit test).
var isRunningInContainerFunc = func() bool {
	if !isGenericContainer() {
		return false
	}
	return isAppRuntimeContainer()
}

// isGenericContainer checks the standard container-detection markers
// (Docker's /.dockerenv, Podman/CRI-O's /run/.containerenv) without making
// any assumption about which container it is.
func isGenericContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}

// appBinaryPath is where docker/Dockerfile installs the built server binary
// in this project's own runtime image (project-rules.md, Binary row). The
// generic casjaysdev/go:latest build/test toolchain image mounts the source
// tree read/write but never installs a binary here, which is what makes this
// check specific to our own image rather than any tini-based container.
const appBinaryPath = "/usr/local/bin/airports"

// isAppRuntimeContainer reports whether this specific project's runtime
// container is active: tini as PID 1, both /config and /data mounted
// (the mandatory 2-volume layout in docker-rules.md), AND the built
// "airports" binary installed at its documented path. The casjaysdev/go:latest
// build/test toolchain image is also tini-as-PID1 and, incidentally, ships
// its own generic /config and /data directories — so the installed-binary
// check is required to avoid a false positive there.
func isAppRuntimeContainer() bool {
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	comm := string(data)
	if comm != "tini\n" && comm != "tini" {
		return false
	}
	if _, err := os.Stat("/config"); err != nil {
		return false
	}
	if _, err := os.Stat("/data"); err != nil {
		return false
	}
	info, err := os.Stat(appBinaryPath)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

// currentHomeDir resolves the invoking user's home directory, falling back
// from os/user to HOME/USERPROFILE env vars.
func currentHomeDir() string {
	currentUser, err := user.Current()
	if err == nil {
		return currentUser.HomeDir
	}
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}
	return homeDir
}

// isRootOrAdmin reports whether the process is running with elevated
// privileges, matching the check used in GetDefaultDirs.
func isRootOrAdmin() bool {
	return isRootOrAdminFunc()
}

// isRootOrAdminFunc is the real privilege-check logic behind isRootOrAdmin,
// held in a package-level variable so tests can force the unprivileged
// branch of every Get*Dir helper without needing a real non-root process.
var isRootOrAdminFunc = func() bool {
	if goosFunc() == "windows" {
		return os.Getenv("USERDOMAIN") == os.Getenv("COMPUTERNAME")
	}
	return os.Geteuid() == 0
}

// GetCacheDir returns the OS/privilege-appropriate cache directory per
// AI.md PART 4. Container paths take priority, matching GetDefaultDirs.
func GetCacheDir(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", projectName, "cache")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, "cache")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, OrgName, projectName, "cache")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Caches", OrgName, projectName)
		}
		return filepath.Join(currentHomeDir(), "Library", "Caches", OrgName, projectName)
	case isBSD(), goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/var/cache", OrgName, projectName)
		}
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache == "" {
			xdgCache = filepath.Join(currentHomeDir(), ".cache")
		}
		return filepath.Join(xdgCache, OrgName, projectName)
	default:
		return filepath.Join("/var/cache", OrgName, projectName)
	}
}

// GetPIDFile returns the OS/privilege-appropriate PID file path per AI.md
// PART 4.
func GetPIDFile(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", projectName, projectName+".pid")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		// Windows services are tracked by the Service Control Manager, not a
		// PID file, but a fallback location is still provided for tooling
		// that expects one.
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, projectName+".pid")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, OrgName, projectName, projectName+".pid")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/var/run", OrgName, projectName+".pid")
		}
		return filepath.Join(currentHomeDir(), "Library", "Application Support", OrgName, projectName, projectName+".pid")
	case isBSD(), goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/var/run", OrgName, projectName+".pid")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, projectName+".pid")
	default:
		return filepath.Join("/var/run", OrgName, projectName+".pid")
	}
}

// GetBackupDir returns the OS/privilege-appropriate backup directory per
// AI.md PART 4. Container paths take priority, matching GetDefaultDirs.
func GetBackupDir(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", "backups", projectName)
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, "Backups", OrgName, projectName)
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, "Backups", OrgName, projectName)
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Backups", OrgName, projectName)
		}
		return filepath.Join(currentHomeDir(), "Library", "Backups", OrgName, projectName)
	case isBSD():
		if isRoot {
			return filepath.Join("/var/backups", OrgName, projectName)
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, "Backups", OrgName, projectName)
	case goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/mnt/Backups", OrgName, projectName)
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, "Backups", OrgName, projectName)
	default:
		return filepath.Join("/mnt/Backups", OrgName, projectName)
	}
}

// GetSSLDir returns the OS/privilege-appropriate SSL certificate directory
// per AI.md PART 4 (contains letsencrypt/ and local/ subdirectories).
// Container paths take priority, matching GetDefaultDirs.
func GetSSLDir(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/config", projectName, "ssl")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, "ssl")
		}
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(currentHomeDir(), "AppData", "Roaming")
		}
		return filepath.Join(appData, OrgName, projectName, "ssl")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Application Support", OrgName, projectName, "ssl")
		}
		return filepath.Join(currentHomeDir(), "Library", "Application Support", OrgName, projectName, "ssl")
	case isBSD():
		if isRoot {
			return filepath.Join("/usr/local/etc", OrgName, projectName, "ssl")
		}
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(currentHomeDir(), ".config")
		}
		return filepath.Join(xdgConfig, OrgName, projectName, "ssl")
	case goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/etc", OrgName, projectName, "ssl")
		}
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(currentHomeDir(), ".config")
		}
		return filepath.Join(xdgConfig, OrgName, projectName, "ssl")
	default:
		return filepath.Join("/etc", OrgName, projectName, "ssl")
	}
}

// GetSecurityDir returns the OS/privilege-appropriate directory for
// downloaded security databases (geoip/, blocklists/, cve/, trivy/) per
// AI.md PART 4. Container paths take priority, matching GetDefaultDirs.
// Note: on Darwin the security dir lives under an explicit "data"
// subdirectory even in the user (non-privileged) case, unlike the rest of
// the Darwin user paths where data and config directories coincide.
func GetSecurityDir(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", projectName, "security")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, "data", "security")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, OrgName, projectName, "security")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Application Support", OrgName, projectName, "data", "security")
		}
		return filepath.Join(currentHomeDir(), "Library", "Application Support", OrgName, projectName, "data", "security")
	case isBSD():
		if isRoot {
			return filepath.Join("/var/db", OrgName, projectName, "security")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "security")
	case goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/var/lib", OrgName, projectName, "security")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "security")
	default:
		return filepath.Join("/var/lib", OrgName, projectName, "security")
	}
}

// GetTorDir returns the OS/privilege-appropriate directory for the
// dedicated Tor process this project's binary owns and manages (torrc,
// runtime data, and the hidden service's persistent ed25519 key) per
// AI.md PART 31 "TOR HIDDEN SERVICE". Container paths take priority,
// matching GetDefaultDirs.
func GetTorDir(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", projectName, "tor")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, "data", "tor")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, OrgName, projectName, "tor")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Application Support", OrgName, projectName, "data", "tor")
		}
		return filepath.Join(currentHomeDir(), "Library", "Application Support", OrgName, projectName, "data", "tor")
	case isBSD():
		if isRoot {
			return filepath.Join("/var/db", OrgName, projectName, "tor")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "tor")
	case goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/var/lib", OrgName, projectName, "tor")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "tor")
	default:
		return filepath.Join("/var/lib", OrgName, projectName, "tor")
	}
}

// GetSQLiteDBPath returns the OS/privilege-appropriate directory containing
// the SQLite database file per AI.md PART 4. The database file itself is
// always named "server.db" per docker-rules.md. Container paths take
// priority and, uniquely, omit the project name segment — the container's
// SQLite path is the fixed "/data/db/sqlite/" regardless of project.
func GetSQLiteDBPath(projectName string) string {
	if IsRunningInContainer() {
		return filepath.Join("/data", "db", "sqlite")
	}

	isRoot := isRootOrAdmin()

	switch {
	case goosFunc() == "windows":
		if isRoot {
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			return filepath.Join(programData, OrgName, projectName, "db")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(currentHomeDir(), "AppData", "Local")
		}
		return filepath.Join(localAppData, OrgName, projectName, "db")
	case goosFunc() == "darwin":
		if isRoot {
			return filepath.Join("/Library/Application Support", OrgName, projectName, "db")
		}
		return filepath.Join(currentHomeDir(), "Library", "Application Support", OrgName, projectName, "db")
	case isBSD():
		if isRoot {
			return filepath.Join("/var/db", OrgName, projectName, "db")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "db")
	case goosFunc() == "linux":
		if isRoot {
			return filepath.Join("/var/lib", OrgName, projectName, "db")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(currentHomeDir(), ".local", "share")
		}
		return filepath.Join(xdgData, OrgName, projectName, "db")
	default:
		return filepath.Join("/var/lib", OrgName, projectName, "db")
	}
}
