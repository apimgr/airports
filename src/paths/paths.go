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

// GetDefaultDirs returns OS-specific default directories based on privileges
// Uses {org}/{name} structure: /etc/apimgr/airports/, ~/.config/apimgr/airports/
func GetDefaultDirs(projectName string) (configDir, dataDir, logsDir string) {
	// Check if running as root/admin
	isRoot := false
	if runtime.GOOS == "windows" {
		// On Windows, check if running as Administrator
		isRoot = os.Getenv("USERDOMAIN") == os.Getenv("COMPUTERNAME")
	} else {
		// On Unix-like systems, check if UID is 0
		isRoot = os.Geteuid() == 0
	}

	if isRoot {
		// Running with elevated privileges - use system directories with org/name structure
		switch runtime.GOOS {
		case "windows":
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			configDir = filepath.Join(programData, OrgName, projectName)
			dataDir = filepath.Join(programData, OrgName, projectName, "data")
			logsDir = filepath.Join(programData, OrgName, projectName, "logs")
		default: // Linux, BSD, macOS
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

		switch runtime.GOOS {
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
			configDir = filepath.Join(homeDir, ".config", OrgName, projectName)
			dataDir = filepath.Join(homeDir, ".local", "share", OrgName, projectName)
			logsDir = filepath.Join(homeDir, ".local", "share", OrgName, projectName, "logs")
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
			logsDir = filepath.Join(xdgData, OrgName, projectName, "logs")
		}
	}

	return configDir, dataDir, logsDir
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
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

// IsRunningInContainer checks if running inside a container (tini as PID 1)
func IsRunningInContainer() bool {
	// Check if PID 1 is tini (container init system)
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	comm := string(data)
	return comm == "tini\n" || comm == "tini"
}

// GetBackupDir returns the default backup directory
func GetBackupDir(projectName string) string {
	return filepath.Join("/mnt/Backups", OrgName, projectName)
}
