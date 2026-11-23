package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	cliUpdateOldVersionEnv = "APIMGR_AIRPORTS_CLI_UPDATE_OLD"
	cliUpdateNewVersionEnv = "APIMGR_AIRPORTS_CLI_UPDATE_NEW"
)

func cliLogFilePath() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, err := os.UserHomeDir()
			if err != nil || home == "" {
				return ""
			}
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "apimgr", "airports", "log", "cli.log")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "log", "apimgr", "airports", "cli.log")
}

func writeCLIAudit(event, fields string) {
	line := "timestamp=" + time.Now().UTC().Format(time.RFC3339) + " event=" + event + " " + fields
	if logPath := cliLogFilePath(); logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0700); err == nil {
			if file, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); openErr == nil {
				_, _ = fmt.Fprintln(file, line)
				_ = file.Close()
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "CLI audit log unavailable: %v\n", openErr)
			}
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "CLI audit log unavailable: %v\n", err)
		}
	}
	_, _ = fmt.Fprintln(os.Stderr, line)
}

func withCLIUpdateRestartMarker(env []string, oldVersion, newVersion string) []string {
	result := make([]string, 0, len(env)+2)
	for _, item := range env {
		if strings.HasPrefix(item, cliUpdateOldVersionEnv+"=") || strings.HasPrefix(item, cliUpdateNewVersionEnv+"=") {
			continue
		}
		result = append(result, item)
	}
	return append(result, cliUpdateOldVersionEnv+"="+oldVersion, cliUpdateNewVersionEnv+"="+newVersion)
}

func logCLIUpdateRestart() {
	oldVersion := os.Getenv(cliUpdateOldVersionEnv)
	newVersion := os.Getenv(cliUpdateNewVersionEnv)
	_ = os.Unsetenv(cliUpdateOldVersionEnv)
	_ = os.Unsetenv(cliUpdateNewVersionEnv)
	if oldVersion == "" || newVersion == "" || normalizeSemver(newVersion) != normalizeSemver(Version) {
		return
	}
	writeCLIAudit("cli.update_completed", "old_version="+strconv.Quote(oldVersion)+" new_version="+strconv.Quote(newVersion)+" os_arch="+strconv.Quote(runtime.GOOS+"-"+runtime.GOARCH))
}
