package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/notify"
)

func stageReleaseBinary(client *http.Client, release *releaseInfo, dataDir string) error {
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	var binaryURL, sumsURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			binaryURL = asset.BrowserDownloadURL
		case "SHA256SUMS":
			sumsURL = asset.BrowserDownloadURL
		}
	}
	if binaryURL == "" || sumsURL == "" {
		return fmt.Errorf("release is missing %s or SHA256SUMS", assetName)
	}

	sumsResponse, err := client.Get(sumsURL)
	if err != nil {
		return fmt.Errorf("fetch SHA256SUMS: %w", err)
	}
	defer func() { _ = sumsResponse.Body.Close() }()
	if sumsResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch SHA256SUMS: HTTP %d", sumsResponse.StatusCode)
	}
	sums, err := io.ReadAll(io.LimitReader(sumsResponse.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	var expected string
	for _, line := range strings.Split(string(sums), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expected = strings.ToLower(parts[0])
			break
		}
	}
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("SHA256SUMS has no valid digest for %s", assetName)
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("SHA256SUMS has an invalid digest for %s", assetName)
	}

	stageDir := filepath.Join(dataDir, "update")
	if err := os.MkdirAll(stageDir, 0700); err != nil {
		return fmt.Errorf("create update staging directory: %w", err)
	}
	if err := os.Chmod(stageDir, 0700); err != nil {
		return fmt.Errorf("secure update staging directory: %w", err)
	}
	stagedPath := filepath.Join(stageDir, ProjectName+".new")
	temporaryPath := stagedPath + ".tmp"
	_ = os.Remove(temporaryPath)
	output, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return fmt.Errorf("create staged binary: %w", err)
	}
	download, err := client.Get(binaryURL)
	if err != nil {
		_ = output.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("download update binary: %w", err)
	}
	if download.StatusCode != http.StatusOK {
		_ = download.Body.Close()
		_ = output.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("download update binary: HTTP %d", download.StatusCode)
	}
	hasher := sha256.New()
	const maxUpdateSize = 256 << 20
	written, copyErr := io.Copy(io.MultiWriter(output, hasher), io.LimitReader(download.Body, maxUpdateSize+1))
	_ = download.Body.Close()
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || written == 0 || written > maxUpdateSize {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("save staged update: %w", firstError(copyErr, closeErr, fmt.Errorf("invalid update size %d", written)))
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	if err := os.Rename(temporaryPath, stagedPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("publish staged binary: %w", err)
	}
	version := strings.TrimPrefix(release.TagName, "v")
	for filename, value := range map[string]string{
		ProjectName + ".sha256":           expected + "\n",
		ProjectName + ".version":          version + "\n",
		ProjectName + ".previous_version": Version + "\n",
	} {
		if err := os.WriteFile(filepath.Join(stageDir, filename), []byte(value), 0600); err != nil {
			_ = os.RemoveAll(stageDir)
			return fmt.Errorf("write staged update metadata: %w", err)
		}
	}
	return nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

type stagedUpdateFailure struct {
	version string
	cause   error
}

func applyStagedUpdate(dataDir string, startedElevated bool) *stagedUpdateFailure {
	if !startedElevated || runtime.GOOS == "windows" {
		return nil
	}
	stageDir := filepath.Join(dataDir, "update")
	stagedBinary := filepath.Join(stageDir, ProjectName+".new")
	if _, err := os.Stat(stagedBinary); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		failure := &stagedUpdateFailure{version: "unknown", cause: fmt.Errorf("inspect staged update: %w", err)}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	readMetadata := func(name string) string {
		value, _ := os.ReadFile(filepath.Join(stageDir, name))
		return strings.TrimSpace(string(value))
	}
	version := readMetadata(ProjectName + ".version")
	oldVersion := readMetadata(ProjectName + ".previous_version")
	expected := strings.ToLower(readMetadata(ProjectName + ".sha256"))
	if version == "" || oldVersion == "" || len(expected) != sha256.Size*2 {
		failure := &stagedUpdateFailure{version: firstNonEmpty(version, "unknown"), cause: errors.New("staged update metadata is incomplete")}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	if !isNewerUpdateVersion(version, Version) {
		_ = os.RemoveAll(stageDir)
		return nil
	}
	data, err := os.ReadFile(stagedBinary)
	if err != nil {
		failure := &stagedUpdateFailure{version: version, cause: fmt.Errorf("read staged binary: %w", err)}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != expected {
		failure := &stagedUpdateFailure{version: version, cause: errors.New("staged binary checksum mismatch")}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	currentPath, err := os.Executable()
	if err == nil {
		err = os.Chmod(stagedBinary, 0755)
	}
	if err == nil {
		err = replaceBinary(currentPath, stagedBinary)
	}
	if err != nil {
		failure := &stagedUpdateFailure{version: version, cause: fmt.Errorf("replace running binary: %w", err)}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	installedMarker := filepath.Join(dataDir, "update_installed")
	if err := os.WriteFile(installedMarker, []byte(oldVersion+"\n"+version+"\n"), 0600); err != nil {
		failure := &stagedUpdateFailure{version: version, cause: fmt.Errorf("record installed update: %w", err)}
		_ = os.RemoveAll(stageDir)
		return failure
	}
	_ = os.RemoveAll(stageDir)
	if err := execUpdatedBinary(currentPath); err != nil {
		_ = os.Remove(installedMarker)
		return &stagedUpdateFailure{version: version, cause: fmt.Errorf("re-execute updated binary: %w", err)}
	}
	return nil
}

func reportStagedUpdate(dataDir, logsDir, configDir string, cfg *config.ApplicationConfig, failure *stagedUpdateFailure) {
	installedMarker := filepath.Join(dataDir, "update_installed")
	if marker, err := os.ReadFile(installedMarker); err == nil {
		fields := strings.Fields(string(marker))
		if len(fields) == 2 && fields[1] == Version {
			if err := notify.Send(cfg, configDir, "update_installed", notifyRecipient(cfg), map[string]string{
				"old_version": fields[0],
				"new_version": fields[1],
			}); err != nil {
				log.Printf("Warning: failed to send update_installed notification email: %v", err)
			}
			if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "update.installed", "operator", "version="+strconv.Quote(fields[1])); err != nil {
				log.Printf("Warning: could not audit installed update: %v", err)
			}
		}
		_ = os.Remove(installedMarker)
	}
	if failure != nil {
		failStagedUpdate(cfg, configDir, logsDir, failure.version, failure.cause)
	}
}

func isNewerUpdateVersion(candidate, current string) bool {
	candidateVersion := "v" + strings.TrimPrefix(candidate, "v")
	currentVersion := "v" + strings.TrimPrefix(current, "v")
	if semver.IsValid(candidateVersion) && semver.IsValid(currentVersion) {
		return semver.Compare(candidateVersion, currentVersion) > 0
	}
	return candidate != "" && candidate != current
}

func failStagedUpdate(cfg *config.ApplicationConfig, configDir, logsDir, version string, cause error) {
	log.Printf("ERROR: staged update %s failed: %v", version, cause)
	if err := notify.Send(cfg, configDir, "update_failed", notifyRecipient(cfg), map[string]string{
		"new_version": version,
		"error":       cause.Error(),
	}); err != nil {
		log.Printf("Warning: failed to send update_failed notification email: %v", err)
	}
	if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "update.failed", "operator", "version="+strconv.Quote(version)); err != nil {
		log.Printf("Warning: could not audit failed update: %v", err)
	}
}
