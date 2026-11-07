package main

import (
	"crypto/subtle"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
	paths "github.com/apimgr/airports/src/path"
	"github.com/apimgr/airports/src/service"
)

func runMaintenanceMode(configDir, requestedMode string) error {
	appMode, err := mode.ParseAppMode(requestedMode)
	if err != nil {
		return err
	}
	if strings.TrimSpace(configDir) == "" {
		defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
		configDir = firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load server configuration: %w", err)
	}

	runningAsRoot := service.IsElevated()
	currentUser, err := user.Current()
	if err != nil && !runningAsRoot {
		return fmt.Errorf("cannot determine current user for mode authorization: %w", err)
	}
	username := ""
	if currentUser != nil {
		username = currentUser.Username
	}
	providedToken := ""
	if !runningAsRoot {
		if !isConfiguredServiceUser(cfg, username) {
			return fmt.Errorf("mode change requires root or the configured service user")
		}
		providedToken, err = promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
	}
	if err := authorizeMaintenanceModeChange(cfg, runningAsRoot, username, providedToken); err != nil {
		return err
	}
	if runningAsRoot {
		fmt.Fprintln(os.Stderr, "Warning: changing application mode can relax security controls.")
	}

	if err := persistConfiguredServerMode(configPath, cfg, appMode); err != nil {
		return fmt.Errorf("failed to save server mode: %w", err)
	}
	fmt.Printf("Application mode set to %s. Restart the server for the change to take effect.\n", appMode.String())
	return nil
}

func persistConfiguredServerMode(configPath string, cfg *config.Config, appMode mode.AppMode) error {
	if cfg == nil {
		return fmt.Errorf("server configuration is nil")
	}
	cfg.Server.Mode = appMode.String()
	return config.Save(configPath, cfg)
}

func isConfiguredServiceUser(cfg *config.Config, username string) bool {
	if cfg == nil || strings.TrimSpace(username) == "" {
		return false
	}
	serviceUser := strings.TrimSpace(cfg.Server.User)
	if serviceUser == "" {
		serviceUser = ProjectName
	}
	return sameOperatingSystemUser(serviceUser, username)
}

func sameOperatingSystemUser(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	if runtime.GOOS == "windows" {
		if index := strings.LastIndex(actual, `\`); index >= 0 {
			actual = actual[index+1:]
		}
		return strings.EqualFold(expected, actual)
	}
	return expected == actual
}

func authorizeMaintenanceModeChange(cfg *config.Config, runningAsRoot bool, username, providedToken string) error {
	if runningAsRoot {
		return nil
	}
	if !isConfiguredServiceUser(cfg, username) {
		return fmt.Errorf("mode change requires root or the configured service user")
	}
	if cfg.Server.Token == "" || providedToken == "" || subtle.ConstantTimeCompare([]byte(providedToken), []byte(cfg.Server.Token)) != 1 {
		return fmt.Errorf("mode change denied: invalid operator token")
	}
	return nil
}
