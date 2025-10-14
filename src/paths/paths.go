package paths

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

// GetDefaultDirs returns OS-specific default directories based on privileges
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
		// Running with elevated privileges
		switch runtime.GOOS {
		case "windows":
			programData := os.Getenv("ProgramData")
			if programData == "" {
				programData = "C:\\ProgramData"
			}
			configDir = filepath.Join(programData, projectName, "config")
			dataDir = filepath.Join(programData, projectName, "data")
			logsDir = filepath.Join(programData, projectName, "logs")
		default: // Linux, BSD, macOS
			configDir = filepath.Join("/etc", projectName)
			dataDir = filepath.Join("/var/lib", projectName)
			logsDir = filepath.Join("/var/log", projectName)
		}
	} else {
		// Running as regular user
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
			configDir = filepath.Join(appData, projectName)
			dataDir = filepath.Join(localAppData, projectName)
			logsDir = filepath.Join(localAppData, projectName, "logs")
		case "darwin": // macOS
			configDir = filepath.Join(homeDir, "Library", "Application Support", projectName)
			dataDir = filepath.Join(homeDir, "Library", "Application Support", projectName, "data")
			logsDir = filepath.Join(homeDir, "Library", "Logs", projectName)
		default: // Linux, BSD
			// Follow XDG Base Directory specification
			xdgConfig := os.Getenv("XDG_CONFIG_HOME")
			if xdgConfig == "" {
				xdgConfig = filepath.Join(homeDir, ".config")
			}
			xdgData := os.Getenv("XDG_DATA_HOME")
			if xdgData == "" {
				xdgData = filepath.Join(homeDir, ".local", "share")
			}
			xdgCache := os.Getenv("XDG_CACHE_HOME")
			if xdgCache == "" {
				xdgCache = filepath.Join(homeDir, ".cache")
			}

			configDir = filepath.Join(xdgConfig, projectName)
			dataDir = filepath.Join(xdgData, projectName)
			logsDir = filepath.Join(xdgCache, projectName, "logs")
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
