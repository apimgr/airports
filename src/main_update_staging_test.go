package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/service"
)

func TestStageReleaseBinaryVerifiesAndPublishesArtifacts(t *testing.T) {
	binary := []byte("verified staged update")
	digest := sha256.Sum256(binary)
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums":
			_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(digest[:]), assetName)
		case "/binary":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dataDir := t.TempDir()
	release := &releaseInfo{TagName: "v2.4.0", Assets: []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{
		{Name: assetName, BrowserDownloadURL: server.URL + "/binary"},
		{Name: "SHA256SUMS", BrowserDownloadURL: server.URL + "/checksums"},
	}}
	if err := stageReleaseBinary(server.Client(), release, dataDir); err != nil {
		t.Fatalf("stageReleaseBinary: %v", err)
	}

	stageDir := filepath.Join(dataDir, "update")
	for name, want := range map[string]string{
		ProjectName + ".new":              string(binary),
		ProjectName + ".sha256":           hex.EncodeToString(digest[:]) + "\n",
		ProjectName + ".version":          "2.4.0\n",
		ProjectName + ".previous_version": Version + "\n",
	} {
		got, err := os.ReadFile(filepath.Join(stageDir, name))
		if err != nil {
			t.Fatalf("read staged %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("staged %s = %q, want %q", name, got, want)
		}
	}
	if info, err := os.Stat(stageDir); err != nil || info.Mode().Perm() != 0700 {
		t.Errorf("staging directory permissions = %v, error = %v; want 0700", info, err)
	}
}

func TestStageReleaseBinaryRejectsMissingAndInvalidChecksums(t *testing.T) {
	dataDir := t.TempDir()
	if err := stageReleaseBinary(http.DefaultClient, &releaseInfo{}, dataDir); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing assets error = %v, want missing asset error", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checksums" {
			_, _ = fmt.Fprintln(w, "not-a-digest")
			return
		}
		_, _ = w.Write([]byte("binary"))
	}))
	defer server.Close()
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	release := &releaseInfo{TagName: "v2.4.0", Assets: []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{
		{Name: assetName, BrowserDownloadURL: server.URL + "/binary"},
		{Name: "SHA256SUMS", BrowserDownloadURL: server.URL + "/checksums"},
	}}
	if err := stageReleaseBinary(server.Client(), release, dataDir); err == nil || !strings.Contains(err.Error(), "no valid digest") {
		t.Fatalf("invalid checksum error = %v, want invalid digest error", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "update")); !os.IsNotExist(err) {
		t.Fatalf("invalid release created staging artifacts: stat error = %v", err)
	}
}

func TestStageReleaseBinaryRejectsHTTPAndChecksumFailures(t *testing.T) {
	binary := []byte("release binary")
	digest := sha256.Sum256(binary)
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	for _, test := range []struct {
		name       string
		checksum   int
		binaryCode int
		checksumOK bool
		want       string
	}{
		{"checksum endpoint status", http.StatusServiceUnavailable, http.StatusOK, true, "SHA256SUMS: HTTP 503"},
		{"binary endpoint status", http.StatusOK, http.StatusServiceUnavailable, true, "binary: HTTP 503"},
		{"checksum mismatch", http.StatusOK, http.StatusOK, false, "checksum mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/checksums" {
					if test.checksum != http.StatusOK {
						w.WriteHeader(test.checksum)
						return
					}
					if !test.checksumOK {
						_, _ = fmt.Fprintf(w, "%s  %s\n", strings.Repeat("0", sha256.Size*2), assetName)
						return
					}
					_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(digest[:]), assetName)
					return
				}
				if test.binaryCode != http.StatusOK {
					w.WriteHeader(test.binaryCode)
					return
				}
				_, _ = w.Write(binary)
			}))
			defer server.Close()
			release := &releaseInfo{TagName: "v2.4.0", Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: assetName, BrowserDownloadURL: server.URL + "/binary"},
				{Name: "SHA256SUMS", BrowserDownloadURL: server.URL + "/checksums"},
			}}
			if err := stageReleaseBinary(server.Client(), release, t.TempDir()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("stageReleaseBinary error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestStagedUpdateHelpers(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"v2.0.0", "1.9.9", true},
		{"1.0.0", "v1.0.0", false},
		{"devel-next", "devel", true},
		{"", "devel", false},
	}
	for _, test := range tests {
		if got := isNewerUpdateVersion(test.candidate, test.current); got != test.want {
			t.Errorf("isNewerUpdateVersion(%q, %q) = %v, want %v", test.candidate, test.current, got, test.want)
		}
	}
	if got := firstError(nil, fmt.Errorf("first"), fmt.Errorf("second")); got == nil || got.Error() != "first" {
		t.Errorf("firstError = %v, want first error", got)
	}
	if got := firstError(nil, nil); got != nil {
		t.Errorf("firstError with no errors = %v, want nil", got)
	}
	for manager, want := range map[service.ServiceType]string{
		service.ServiceOpenRC:   "rc-service " + ProjectName + " restart",
		service.ServiceSysVinit: "service " + ProjectName + " restart",
		service.ServiceBSDRC:    "service " + ProjectName + " restart",
		service.ServiceSystemd:  ProjectName + " --service restart",
	} {
		if got := stagedUpdateRestartCommand(manager); got != want {
			t.Errorf("stagedUpdateRestartCommand(%v) = %q, want %q", manager, got, want)
		}
	}
}

func TestApplyStagedUpdateRejectsIneligibleAndCorruptUpdates(t *testing.T) {
	dataDir := t.TempDir()
	if failure := applyStagedUpdate(dataDir, false); failure != nil {
		t.Fatalf("unprivileged staged update = %+v, want no action", failure)
	}
	stageDir := filepath.Join(dataDir, "update")
	if err := os.MkdirAll(stageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, ProjectName+".new"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, ProjectName+".version"), []byte("99.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, ProjectName+".previous_version"), []byte("1.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, ProjectName+".sha256"), []byte(strings.Repeat("0", sha256.Size*2)), 0600); err != nil {
		t.Fatal(err)
	}
	failure := applyStagedUpdate(dataDir, true)
	if failure == nil || !strings.Contains(failure.cause.Error(), "checksum mismatch") {
		t.Fatalf("corrupt staged update = %+v, want checksum failure", failure)
	}
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Errorf("corrupt staging directory remains: stat error = %v", err)
	}
}

func TestApplyStagedUpdateCleansIncompleteAndCurrentVersion(t *testing.T) {
	for _, test := range []struct {
		name    string
		version string
	}{
		{name: "incomplete metadata", version: "2.0.0"},
		{name: "already current", version: Version},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			stageDir := filepath.Join(dataDir, "update")
			if err := os.MkdirAll(stageDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stageDir, ProjectName+".new"), []byte("staged"), 0600); err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(filepath.Join(stageDir, ProjectName+".version"), []byte(test.version), 0600)
			if test.name == "already current" {
				_ = os.WriteFile(filepath.Join(stageDir, ProjectName+".previous_version"), []byte("older"), 0600)
				_ = os.WriteFile(filepath.Join(stageDir, ProjectName+".sha256"), []byte(strings.Repeat("0", sha256.Size*2)), 0600)
			}
			failure := applyStagedUpdate(dataDir, true)
			if test.name == "incomplete metadata" && (failure == nil || !strings.Contains(failure.cause.Error(), "metadata is incomplete")) {
				t.Fatalf("incomplete metadata failure = %+v", failure)
			}
			if test.name == "already current" && failure != nil {
				t.Fatalf("current version update failure = %+v, want no-op", failure)
			}
			if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
				t.Errorf("staging directory remains after %s: %v", test.name, err)
			}
		})
	}
}

func TestReportStagedUpdateRemovesCompletionMarker(t *testing.T) {
	dataDir, logsDir, configDir := t.TempDir(), t.TempDir(), t.TempDir()
	marker := filepath.Join(dataDir, "update_installed")
	if err := os.WriteFile(marker, []byte("1.0.0\n"+Version+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultApplicationConfig()
	reportStagedUpdate(dataDir, logsDir, configDir, cfg, nil)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("completion marker remains after reporting: %v", err)
	}
	if err := os.WriteFile(marker, []byte("1.0.0\nstale-version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reportStagedUpdate(dataDir, logsDir, configDir, cfg, nil)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("stale completion marker remains after reporting: %v", err)
	}
	reportStagedUpdate(dataDir, logsDir, configDir, cfg, &stagedUpdateFailure{version: "2.0.0", cause: errors.New("verification failed")})
}
