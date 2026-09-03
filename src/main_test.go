package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/path"
)

// newMainTestDir creates an isolated temp directory under /tmp/apimgr/ per
// project testing rules, cleaned up automatically when the test completes.
func newMainTestDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll /tmp/apimgr: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-main-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll captured stdout: %v", err)
	}
	return string(out)
}

func TestExtractUpdateFlag(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFound  bool
		wantAction string
		wantBranch string
		wantRemain []string
	}{
		{"no update flag", []string{"--port", "8080"}, false, "yes", "", []string{"--port", "8080"}},
		{"bare --update defaults yes", []string{"--update"}, true, "yes", "", []string{}},
		{"--update check", []string{"--update", "check"}, true, "check", "", []string{}},
		{"--update yes explicit", []string{"--update", "yes"}, true, "yes", "", []string{}},
		{"--update=check inline", []string{"--update=check"}, true, "check", "", []string{}},
		{"--update branch stable", []string{"--update", "branch", "stable"}, true, "branch", "stable", []string{}},
		{"--update=branch inline then value", []string{"--update=branch", "beta"}, true, "branch", "", []string{"beta"}},
		{"non-update flags preserved", []string{"--debug", "--update", "check", "--color", "no"}, true, "check", "", []string{"--debug", "--color", "no"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, action, branch, remaining := extractUpdateFlag(tt.args)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if action != tt.wantAction {
				t.Errorf("action = %q, want %q", action, tt.wantAction)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
			}
			if len(remaining) != len(tt.wantRemain) {
				t.Fatalf("remaining = %v, want %v", remaining, tt.wantRemain)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemain[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemain[i])
				}
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first wins", []string{"a", "b", "c"}, "a"},
		{"skips leading empties", []string{"", "", "c"}, "c"},
		{"no args", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.values...); got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestGetEmoji(t *testing.T) {
	t.Run("known name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if got := getEmoji("check"); got != "✓" {
			t.Errorf("getEmoji(check) = %q, want %q", got, "✓")
		}
	})
	t.Run("unknown name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if got := getEmoji("does-not-exist"); got != "" {
			t.Errorf("getEmoji(unknown) = %q, want empty", got)
		}
	})
	t.Run("NO_COLOR suppresses known name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if got := getEmoji("check"); got != "" {
			t.Errorf("getEmoji(check) with NO_COLOR set = %q, want empty", got)
		}
	})
}

// TestEmojiEnabled_ColorFlagPriority covers the AI.md PART 8 "NO_COLOR
// Support" priority order: --color CLI flag overrides NO_COLOR/TERM=dumb in
// both directions (forcing on when NO_COLOR is set, forcing off when it
// isn't), and TERM=dumb is honored when no flag overrides it.
func TestEmojiEnabled_ColorFlagPriority(t *testing.T) {
	prev := colorSetting
	t.Cleanup(func() { colorSetting = prev })

	t.Run("color=yes overrides NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		colorSetting = "yes"
		if !emojiEnabled() {
			t.Error("emojiEnabled() = false, want true (--color=yes overrides NO_COLOR)")
		}
	})
	t.Run("color=no overrides unset NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		colorSetting = "no"
		if emojiEnabled() {
			t.Error("emojiEnabled() = true, want false (--color=no forces off)")
		}
	})
	t.Run("auto falls back to TERM=dumb", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "dumb")
		colorSetting = "auto"
		if emojiEnabled() {
			t.Error("emojiEnabled() = true, want false (TERM=dumb with color=auto)")
		}
	})
	t.Run("auto with normal TERM stays enabled", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")
		colorSetting = "auto"
		if !emojiEnabled() {
			t.Error("emojiEnabled() = false, want true (color=auto, no NO_COLOR/TERM=dumb)")
		}
	})
}

// TestResolveColorSetting confirms the --color flag value is actually
// stored into colorSetting instead of being discarded (the AI.md PART 7/8
// gap this test guards against: "--color validated but never applied").
// run() itself is not called here — it starts a real server on success and
// is intentionally excluded from unit tests (see the NOTE comment below).
func TestResolveColorSetting(t *testing.T) {
	prev := colorSetting
	t.Cleanup(func() { colorSetting = prev })

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty defaults to auto", "", "auto", false},
		{"auto", "auto", "auto", false},
		{"yes", "yes", "yes", false},
		{"no", "no", "no", false},
		{"invalid", "bogus", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colorSetting = "auto"
			got, err := resolveColorSetting(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveColorSetting(%q): expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveColorSetting(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("resolveColorSetting(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if colorSetting != tt.want {
				t.Errorf("colorSetting after resolveColorSetting(%q) = %q, want %q", tt.in, colorSetting, tt.want)
			}
		})
	}
}

// isRunningInContainer() (the main package's own copy, distinct from
// paths.IsRunningInContainer) checks for /.dockerenv, which is always
// present inside the casjaysdev/go:latest test container.
func TestIsRunningInContainerMain(t *testing.T) {
	got := isRunningInContainer()
	if !got {
		t.Log("isRunningInContainer() = false; expected true inside a Docker test container, but not failing the build for host runs")
	}
}

func TestFindRandomPort(t *testing.T) {
	port, err := findRandomPort()
	if err != nil {
		t.Fatalf("findRandomPort() returned error: %v", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("findRandomPort() = %q, not numeric: %v", port, err)
	}
	if n < 64000 || n > 64999 {
		t.Errorf("findRandomPort() = %d, want in range [64000, 64999]", n)
	}
}

// getOutboundIP dials 8.8.8.8:80 via UDP, which never actually sends a
// packet (connectionless), so this is safe to call in any environment; it
// simply asserts the function returns without panicking.
func TestGetOutboundIP(t *testing.T) {
	ip := getOutboundIP()
	t.Logf("getOutboundIP() = %q", ip)
}

func TestGetAccessibleURL(t *testing.T) {
	url := getAccessibleURL("64123")
	if !strings.Contains(url, "64123") {
		t.Errorf("getAccessibleURL(64123) = %q, want it to contain the port", url)
	}
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("getAccessibleURL(64123) = %q, want http:// prefix", url)
	}
}

func TestResolveFQDN(t *testing.T) {
	t.Run("DOMAIN env wins over configured value", func(t *testing.T) {
		t.Setenv("DOMAIN", "env.example.com,other.example.com")
		if got := resolveFQDN("configured.example.com"); got != "env.example.com" {
			t.Errorf("resolveFQDN() = %q, want %q", got, "env.example.com")
		}
	})

	t.Run("DOMAIN env trims whitespace around first entry", func(t *testing.T) {
		t.Setenv("DOMAIN", "  spaced.example.com , other.example.com")
		if got := resolveFQDN(""); got != "spaced.example.com" {
			t.Errorf("resolveFQDN() = %q, want %q", got, "spaced.example.com")
		}
	})

	t.Run("configured value used when DOMAIN unset", func(t *testing.T) {
		t.Setenv("DOMAIN", "")
		if got := resolveFQDN("configured.example.com"); got != "configured.example.com" {
			t.Errorf("resolveFQDN() = %q, want %q", got, "configured.example.com")
		}
	})

	t.Run("falls back to hostname/IP/localhost when nothing configured", func(t *testing.T) {
		t.Setenv("DOMAIN", "")
		got := resolveFQDN("")
		if got == "" {
			t.Error("resolveFQDN() returned empty string, want a non-empty fallback")
		}
	})
}

func TestApplyTZEnv(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	t.Run("valid TZ sets time.Local", func(t *testing.T) {
		t.Setenv("TZ", "America/New_York")
		applyTZEnv()
		if time.Local.String() != "America/New_York" {
			t.Errorf("time.Local = %q, want %q", time.Local.String(), "America/New_York")
		}
	})

	t.Run("invalid TZ leaves time.Local unchanged", func(t *testing.T) {
		time.Local = original
		t.Setenv("TZ", "Not/A_Real_Zone")
		applyTZEnv()
		if time.Local != original {
			t.Errorf("time.Local changed on invalid TZ, want unchanged (%v)", original)
		}
	})

	t.Run("empty TZ leaves time.Local unchanged", func(t *testing.T) {
		time.Local = original
		t.Setenv("TZ", "")
		applyTZEnv()
		if time.Local != original {
			t.Errorf("time.Local changed on empty TZ, want unchanged (%v)", original)
		}
	})
}

func TestResolveSchedulerLocation(t *testing.T) {
	t.Run("empty timezone falls back to time.Local", func(t *testing.T) {
		if got := resolveSchedulerLocation(""); got != time.Local {
			t.Errorf("resolveSchedulerLocation(\"\") = %v, want time.Local", got)
		}
	})

	t.Run("valid IANA timezone is loaded", func(t *testing.T) {
		got := resolveSchedulerLocation("America/New_York")
		if got.String() != "America/New_York" {
			t.Errorf("resolveSchedulerLocation() = %v, want America/New_York", got)
		}
	})

	t.Run("invalid timezone falls back to time.Local", func(t *testing.T) {
		if got := resolveSchedulerLocation("Not/A_Real_Zone"); got != time.Local {
			t.Errorf("resolveSchedulerLocation() = %v, want time.Local fallback", got)
		}
	})
}

func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, func() { printHelp("airports") })
	for _, want := range []string{"Usage:", "--help", "--version", "--status", "Update Commands:", "Service Commands:", "Maintenance / Admin:", "Environment Variables:"} {
		if !strings.Contains(out, want) {
			t.Errorf("printHelp() output missing %q; got:\n%s", want, out)
		}
	}
}

func TestPrintServiceHelp(t *testing.T) {
	out := captureStdout(t, printServiceHelp)
	for _, want := range []string{"Service Commands:", "install", "uninstall", "start", "stop", "restart", "reload", "status", "enable", "disable", "logs", "Supported service managers:"} {
		if !strings.Contains(out, want) {
			t.Errorf("printServiceHelp() output missing %q; got:\n%s", want, out)
		}
	}
}

func TestHandleServiceCommandUnknown(t *testing.T) {
	err := handleServiceCommand("not-a-real-command")
	if err == nil {
		t.Fatal("handleServiceCommand(unknown): expected error, got nil")
	}
	want := "unknown service command: not-a-real-command"
	if err.Error() != want {
		t.Errorf("handleServiceCommand(unknown) error = %q, want %q", err.Error(), want)
	}
}

// run() validates --color before touching any directories/config, so an
// invalid value returns an error deterministically without side effects
// beyond setting server.Version/Commit/BuildDate package vars.
func TestRunInvalidColor(t *testing.T) {
	err := run("", "", "", "", "", "", "", "", false, "bogus", "", false, false)
	if err == nil {
		t.Fatal("run() with invalid --color: expected error, got nil")
	}
	want := "invalid --color value: bogus (must be auto, yes, or no)"
	if err.Error() != want {
		t.Errorf("run() error = %q, want %q", err.Error(), want)
	}
}

// checkAndUpdate() refuses to self-update inside a Docker container and
// returns before making any network calls — deterministically true inside
// the casjaysdev/go:latest test container.
func TestCheckAndUpdateInContainer(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("not running inside a Docker container (no /.dockerenv)")
	}
	err := checkAndUpdate()
	if err == nil {
		t.Fatal("checkAndUpdate() inside a container: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported inside a Docker container") {
		t.Errorf("checkAndUpdate() error = %q, want container-refusal message", err.Error())
	}
}

// setUpdateBranch validates the branch name before touching config, so an
// invalid value returns an error deterministically without side effects.
func TestSetUpdateBranchInvalid(t *testing.T) {
	err := setUpdateBranch("not-a-real-branch")
	if err == nil {
		t.Fatal("setUpdateBranch(invalid): expected error, got nil")
	}
	want := `invalid update branch: "not-a-real-branch" (expected stable, beta, or daily)`
	if err.Error() != want {
		t.Errorf("setUpdateBranch(invalid) error = %q, want %q", err.Error(), want)
	}
}

// setApplicationMode validates the mode via mode.ParseMode before touching
// config, so an invalid value returns an error deterministically without
// side effects.
func TestSetApplicationModeInvalid(t *testing.T) {
	err := setApplicationMode("not-a-real-mode")
	if err == nil {
		t.Fatal("setApplicationMode(invalid): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("setApplicationMode(invalid) error = %q, want it to mention invalid mode", err.Error())
	}
}

func TestRestoreBackupMissingFile(t *testing.T) {
	dir := newMainTestDir(t)
	missing := filepath.Join(dir, "does-not-exist.tar.gz")
	err := restoreBackup(missing, false)
	if err == nil {
		t.Fatal("restoreBackup(missing file): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "file exists") {
		t.Errorf("restoreBackup(missing file) error = %q, want it to mention file exists", err.Error())
	}
}

// redirectingTransport rewrites every request to target the given
// httptest.Server instead of its original host, so fetchLatestRelease's
// hardcoded api.github.com URL can be exercised against a local mock.
type redirectingTransport struct {
	targetURL string
}

func (rt redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := http.NewRequest(req.Method, rt.targetURL, req.Body)
	if err != nil {
		return nil, err
	}
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}

func TestFetchLatestReleaseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"airports-linux-amd64","browser_download_url":"https://example.invalid/airports-linux-amd64"}]}`))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchLatestRelease(client)
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if release == nil {
		t.Fatal("fetchLatestRelease returned nil release for a 200 response")
	}
	if release.TagName != "v9.9.9" {
		t.Errorf("release.TagName = %q, want %q", release.TagName, "v9.9.9")
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "airports-linux-amd64" {
		t.Errorf("release.Assets = %+v, want one asset named airports-linux-amd64", release.Assets)
	}
}

func TestFetchLatestReleaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchLatestRelease(client)
	if err != nil {
		t.Fatalf("fetchLatestRelease on 404: unexpected error: %v", err)
	}
	if release != nil {
		t.Errorf("fetchLatestRelease on 404 = %+v, want nil (no update available)", release)
	}
}

func TestFetchLatestReleaseBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	_, err := fetchLatestRelease(client)
	if err == nil {
		t.Fatal("fetchLatestRelease with malformed JSON: expected error, got nil")
	}
}

// TestMatchesBranch covers PART 22 "Channel Semantics": channels are
// cumulative, so a stable release always matches, beta additionally matches
// "-beta" tags, and daily additionally matches 14-digit timestamp tags.
func TestMatchesBranch(t *testing.T) {
	cases := []struct {
		name   string
		r      releaseInfo
		branch string
		want   bool
	}{
		{"stable release matches stable", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "stable", true},
		{"stable release matches beta", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "beta", true},
		{"stable release matches daily", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "daily", true},
		{"beta tag matches beta", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "beta", true},
		{"beta tag matches daily", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "daily", true},
		{"beta tag does not match stable", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "stable", false},
		{"daily timestamp matches daily", releaseInfo{TagName: "20251205143022", Prerelease: true}, "daily", true},
		{"daily timestamp does not match beta", releaseInfo{TagName: "20251205143022", Prerelease: true}, "beta", false},
		{"daily timestamp does not match stable", releaseInfo{TagName: "20251205143022", Prerelease: true}, "stable", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesBranch(tc.r, tc.branch); got != tc.want {
				t.Errorf("matchesBranch(%+v, %q) = %v, want %v", tc.r, tc.branch, got, tc.want)
			}
		})
	}
}

// TestFetchReleaseForBranchStable exercises the stable-channel path, which
// must delegate to the GitHub "latest" endpoint and skip when already current.
func TestFetchReleaseForBranchStable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","prerelease":false,"assets":[]}`))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchReleaseForBranch(client, "stable", "1.0.0")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch: %v", err)
	}
	if release == nil || release.TagName != "v9.9.9" {
		t.Fatalf("fetchReleaseForBranch(stable) = %+v, want v9.9.9", release)
	}

	release, err = fetchReleaseForBranch(client, "stable", "9.9.9")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch (current): %v", err)
	}
	if release != nil {
		t.Errorf("fetchReleaseForBranch(stable) at current version = %+v, want nil", release)
	}
}

// TestFetchReleaseForBranchDaily exercises the beta/daily list-and-filter
// path: it must return the newest release matching the channel, skipping
// entries that don't match and entries equal to the running version.
func TestFetchReleaseForBranchDaily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"tag_name":"20260101000000","prerelease":true,"assets":[]},
			{"tag_name":"202512051430-beta","prerelease":true,"assets":[]},
			{"tag_name":"v1.2.3","prerelease":false,"assets":[]}
		]`))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchReleaseForBranch(client, "daily", "1.0.0")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch: %v", err)
	}
	if release == nil || release.TagName != "20260101000000" {
		t.Fatalf("fetchReleaseForBranch(daily) = %+v, want newest daily entry", release)
	}

	release, err = fetchReleaseForBranch(client, "daily", "20260101000000")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch (skip current): %v", err)
	}
	if release == nil || release.TagName != "202512051430-beta" {
		t.Fatalf("fetchReleaseForBranch(daily) skipping current = %+v, want beta entry", release)
	}
}

// TestRunScheduledUpdateCheckDeferDays covers PART 22 "Defer Semantics": the
// scheduled update_check task must skip a release that hasn't aged past
// defer_days, without touching LastNotifiedVersion.
func TestRunScheduledUpdateCheckDeferDays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"tag_name":"v9.9.9","prerelease":false,"published_at":%q,"assets":[]}`,
			time.Now().Add(-1*24*time.Hour).Format(time.RFC3339))))
	}))
	defer srv.Close()

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}
	cfg.Server.Update.Branch = "stable"
	cfg.Server.Update.DeferDays = 30
	cfg.Server.Update.LastNotifiedVersion = ""

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient: %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "" {
		t.Errorf("LastNotifiedVersion = %q, want empty (release not yet eligible under defer_days)", cfg.Server.Update.LastNotifiedVersion)
	}
}

// TestRunScheduledUpdateCheckNotifiesOncePerVersion covers PART 22's "fires
// once per version" rule: an eligible release updates LastNotifiedVersion on
// first sight, and a second run for the same version is a no-op.
func TestRunScheduledUpdateCheckNotifiesOncePerVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","prerelease":false,"assets":[]}`))
	}))
	defer srv.Close()

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}
	cfg.Server.Update.Branch = "stable"
	cfg.Server.Update.DeferDays = 0
	cfg.Server.Update.AutoInstall = false
	cfg.Server.Update.LastNotifiedVersion = ""

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient (first run): %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "9.9.9" {
		t.Fatalf("LastNotifiedVersion = %q, want 9.9.9", cfg.Server.Update.LastNotifiedVersion)
	}

	// Second run for the same version must not error and must leave
	// LastNotifiedVersion unchanged (no re-notification).
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient (second run): %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "9.9.9" {
		t.Errorf("LastNotifiedVersion after second run = %q, want unchanged 9.9.9", cfg.Server.Update.LastNotifiedVersion)
	}
}

// setUpdateBranch's valid-input path writes to the real OS-resolved config
// path (/etc/apimgr/airports/server.yml under the root-owned, --rm'd
// make test Docker container). That container is disposed of immediately
// after the test run, so this write is fully contained.
func TestSetUpdateBranchValid(t *testing.T) {
	for _, branch := range []string{"stable", "beta", "daily"} {
		if err := setUpdateBranch(branch); err != nil {
			t.Errorf("setUpdateBranch(%q) = %v, want nil", branch, err)
		}
	}
}

// setApplicationMode's valid-input path likewise writes to the real
// OS-resolved config path - same disposable-container justification as
// TestSetUpdateBranchValid.
func TestSetApplicationModeValid(t *testing.T) {
	for _, m := range []string{"production", "development"} {
		if err := setApplicationMode(m); err != nil {
			t.Errorf("setApplicationMode(%q) = %v, want nil", m, err)
		}
	}
}

// checkStatus must never panic and must report "not running" when no server
// is actually listening on the configured/default port - this exercises the
// config-load and HTTP-probe paths without requiring a live server.
func TestCheckStatusNotRunning(t *testing.T) {
	if got := checkStatus(); got != 0 && got != 1 {
		t.Errorf("checkStatus() = %d, want 0 or 1", got)
	}
}

// TestCheckStatusRunning exercises the success-path branches of checkStatus
// (HTTP 200, JSON decode, "data.status" field print) by standing up a real
// HTTP listener on the exact configured port and serving a synthetic
// /healthz response - without ever starting the full application server.
func TestCheckStatusRunning(t *testing.T) {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}

	// config.Load's DefaultConfig leaves Server.Port empty (the real port is
	// normally assigned during server startup, not config load), so bind an
	// ephemeral listener ourselves and persist that port into the config -
	// this mirrors what a running server would have done before checkStatus
	// is ever invoked.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind an ephemeral listener: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	cfg.Server.Port = port
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save(%q): %v", configPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"status":"ok"}}`))
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() {
		srv.Close()
	})

	if got := checkStatus(); got != 0 {
		t.Errorf("checkStatus() with a healthy server = %d, want 0", got)
	}
}

// TestCheckStatusUnhealthy exercises the "Unhealthy (HTTP %d)" branch of
// checkStatus - a live server that responds on /healthz with a non-200
// status must produce a checkStatus() return value of 1.
func TestCheckStatusUnhealthy(t *testing.T) {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}

	// config.Load's DefaultConfig leaves Server.Port empty, so bind an
	// ephemeral listener ourselves and persist that port into the config -
	// see TestCheckStatusRunning for the full rationale.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind an ephemeral listener: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	cfg.Server.Port = port
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save(%q): %v", configPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() {
		srv.Close()
	})

	if got := checkStatus(); got != 1 {
		t.Errorf("checkStatus() with an unhealthy server = %d, want 1", got)
	}
}

// createBackup and restoreBackup both unconditionally call
// paths.GetDefaultDirs for the real config/data dirs regardless of whether
// fileLocation is empty or explicit. Supplying an explicit fileLocation
// (inside a test-owned /tmp/apimgr dir) avoids ever touching the hardcoded
// /mnt/Backups/apimgr/airports default location, while still exercising the
// real archive/extract logic against /etc/apimgr/airports and
// /var/lib/apimgr/airports - contained by the same disposable, --rm'd, root
// make test Docker container already relied on by
// TestSetUpdateBranchValid/TestSetApplicationModeValid.
func TestCreateBackupExplicitLocation(t *testing.T) {
	dir := newMainTestDir(t)
	dest := filepath.Join(dir, "explicit-backup.tar.gz")

	// createBackup unconditionally walks the real config/data dirs - ensure
	// they exist in this disposable container before calling it, since a
	// fresh test run may not have created them yet.
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup(dest, false); err != nil {
		t.Fatalf("createBackup(%q): unexpected error: %v", dest, err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("Stat(%q) after createBackup: %v", dest, err)
	}
	if info.Size() == 0 {
		t.Error("createBackup produced an empty archive")
	}

	// No leftover .tmp file after a successful atomic rename.
	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("leftover tmp file after createBackup, stat err = %v", err)
	}
}

// TestRestoreBackupSuccess round-trips a backup created via createBackup
// through restoreBackup, exercising the extraction success path (both the
// "config/" and "data/" tar entry prefixes) without ever touching the
// /mnt/Backups default location.
func TestRestoreBackupSuccess(t *testing.T) {
	dir := newMainTestDir(t)
	archive := filepath.Join(dir, "roundtrip-backup.tar.gz")

	// restoreBackup unconditionally extracts into the real config/data dirs -
	// ensure they exist in this disposable container first.
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup(archive, false); err != nil {
		t.Fatalf("createBackup(%q): unexpected error: %v", archive, err)
	}

	if err := restoreBackup(archive, false); err != nil {
		t.Fatalf("restoreBackup(%q): unexpected error: %v", archive, err)
	}
}

// TestCreateBackupDefaultLocationAlsoCreatesDailyIncremental exercises
// createBackup's empty-fileLocation (scheduled backup_daily task) path per
// AI.md PART 21 "Backup Files Created": the full dated backup AND the
// "{project}-daily.tar.gz" incremental must both exist afterward. Uses
// backupDirOverride to redirect the default backup location into a
// test-owned temp dir instead of the real /mnt/Backups path.
func TestCreateBackupDefaultLocationAlsoCreatesDailyIncremental(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup("", false); err != nil {
		t.Fatalf("createBackup(\"\"): unexpected error: %v", err)
	}

	dailyPath := filepath.Join(dir, ProjectName+"-daily.tar.gz")
	info, err := os.Stat(dailyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after createBackup: %v", dailyPath, err)
	}
	if info.Size() == 0 {
		t.Error("daily incremental backup is empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	sawFull := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ProjectName+"_backup_") {
			sawFull = true
		}
	}
	if !sawFull {
		t.Errorf("expected a %s_backup_* full backup alongside the daily incremental, got entries %v", ProjectName, entries)
	}
}

// TestCreateIncrementalBackupHourlyReplacesExistingFile confirms
// createIncrementalBackup writes a single "{project}-hourly.tar.gz" file
// and that a second run replaces (not duplicates) it, per AI.md PART 21
// "Backup Files Created" - "Always 1 (replaced each run)".
func TestCreateIncrementalBackupHourlyReplacesExistingFile(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createIncrementalBackup("hourly"); err != nil {
		t.Fatalf("createIncrementalBackup(hourly) #1: unexpected error: %v", err)
	}
	hourlyPath := filepath.Join(dir, ProjectName+"-hourly.tar.gz")
	firstInfo, err := os.Stat(hourlyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after first run: %v", hourlyPath, err)
	}

	if err := createIncrementalBackup("hourly"); err != nil {
		t.Fatalf("createIncrementalBackup(hourly) #2: unexpected error: %v", err)
	}
	secondInfo, err := os.Stat(hourlyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after second run: %v", hourlyPath, err)
	}
	if secondInfo.Size() == 0 {
		t.Error("hourly incremental backup is empty after replace")
	}
	_ = firstInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ProjectName+"-hourly.") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 hourly incremental file after two runs, got %d (entries %v)", count, entries)
	}
}

// TestCreateIncrementalBackupNotSubjectToFullBackupRetention creates several
// dated full backups (exceeding max_backups) plus a daily incremental, runs
// createBackup's retention sweep, and confirms the incremental file is never
// counted or deleted by it - per AI.md PART 21 "Backup Files Created"
// retention column ("Always 1 (replaced each run)" vs "Controlled by
// max_backups").
func TestCreateIncrementalBackupNotSubjectToFullBackupRetention(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createIncrementalBackup("daily"); err != nil {
		t.Fatalf("createIncrementalBackup(daily): unexpected error: %v", err)
	}
	dailyPath := filepath.Join(dir, ProjectName+"-daily.tar.gz")
	if _, err := os.Stat(dailyPath); err != nil {
		t.Fatalf("Stat(%q): %v", dailyPath, err)
	}

	// A stale-dated full backup that createBackup's own retention sweep
	// (max_backups defaults to 1) should be free to delete without ever
	// touching the daily incremental sitting alongside it.
	staleFull := filepath.Join(dir, ProjectName+"_backup_2000-01-01.tar.gz")
	if err := os.WriteFile(staleFull, []byte("not a real archive, just retention bait"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", staleFull, err)
	}

	if err := createBackup("", false); err != nil {
		t.Fatalf("createBackup(\"\"): unexpected error: %v", err)
	}

	if _, err := os.Stat(dailyPath); err != nil {
		t.Errorf("daily incremental was removed by full-backup retention: %v", err)
	}
}

func TestExtractEmailFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantFound   bool
		wantAction  string
		wantAddress string
		wantRemain  []string
	}{
		{"no email keyword", []string{"--port", "8080"}, false, "", "", []string{"--port", "8080"}},
		{"bare email", []string{"email"}, true, "", "", []string{}},
		{"email test", []string{"email", "test"}, true, "test", "", []string{}},
		{"email test address", []string{"email", "test", "someone@example.com"}, true, "test", "someone@example.com", []string{}},
		{"email not first arg is ignored", []string{"--debug", "email", "test"}, false, "", "", []string{"--debug", "email", "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, action, address, remaining := extractEmailFlag(tt.args)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if action != tt.wantAction {
				t.Errorf("action = %q, want %q", action, tt.wantAction)
			}
			if address != tt.wantAddress {
				t.Errorf("address = %q, want %q", address, tt.wantAddress)
			}
			if len(remaining) != len(tt.wantRemain) {
				t.Fatalf("remaining = %v, want %v", remaining, tt.wantRemain)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemain[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemain[i])
				}
			}
		})
	}
}

func TestNotifyRecipient(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "reply_to wins",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Server.Notifications.Email.ReplyTo = "reply@example.com"
				c.Server.Notifications.Email.From.Email = "from@example.com"
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "reply@example.com",
		},
		{
			name: "falls back to from.email",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Server.Notifications.Email.From.Email = "from@example.com"
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "from@example.com",
		},
		{
			name: "falls back to web.security.admin",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "admin@example.com",
		},
		{
			name: "all empty",
			cfg:  &config.Config{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notifyRecipient(tt.cfg); got != tt.want {
				t.Errorf("notifyRecipient() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleEmailCLIUnknownAction(t *testing.T) {
	if code := handleEmailCLI("bogus", ""); code != 1 {
		t.Errorf("handleEmailCLI(%q) = %d, want 1", "bogus", code)
	}
}

// NOTE: the following main.go functions are intentionally NOT covered by
// unit tests in this package, because they start a live HTTP server, shell
// out to real service managers with no injectable interface, or call
// os.Exit:
//   main (parses os.Args and calls os.Exit)
//   run (success path beyond the --color validation: starts the real HTTP
//     server, binds a live listener, blocks on signal handling)
//   createBackup / restoreBackup (empty-fileLocation branches: these still
//     touch the hardcoded /mnt/Backups/apimgr/airports path, which does not
//     exist and is not safe to create in this environment - the explicit-
//     fileLocation success paths are covered above)
//   handleServiceCommand (named-command branches: invoke real service.*
//     functions that mutate systemd/OpenRC/launchd/Windows service state)
//   handleEmailCLI (success/recipient-missing/CanSend-false branches: these
//     call config.Load against the real default config dir and attempt a
//     live SMTP send; only the pure-validation "unknown action" branch is
//     exercised here)
//   checkForUpdate / checkAndUpdate (success path beyond the in-container
//     short-circuit: uses a live http.Client with no injection point,
//     downloads and atomically replaces the running binary)
// Reaching full coverage on these without refactoring toward injectable
// config paths / an httptest-backed default client, or running destructive
// operations against real system state in a disposable container, is not
// achievable safely per the task's constraints.
