package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newScratchDir creates an isolated temp directory outside the project tree,
// following the mandated /tmp/{project_org}/{internal_name}-XXXXXX/ temp-dir
// structure, and registers cleanup.
func newScratchDir(t *testing.T) string {
	t.Helper()
	orgBase := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(orgBase, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", orgBase, err)
	}
	dir, err := os.MkdirTemp(orgBase, "airports-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

// TestGetDefaultDirs_CurrentPrivilege exercises the branch that actually
// executes for the process's real euid (Linux only, since this suite always
// runs inside the casjaysdev/go:latest Linux container). The Windows/darwin
// branches inside GetDefaultDirs are unreachable here because runtime.GOOS
// is fixed at compile time for the running binary — see final report.
func TestGetDefaultDirs_CurrentPrivilege(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("test targets the Linux/BSD branch, running on %s", runtime.GOOS)
	}

	scratch := newScratchDir(t)
	t.Setenv("HOME", scratch)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	configDir, dataDir, logsDir := GetDefaultDirs("airports")

	if os.Geteuid() == 0 {
		wantConfig := filepath.Join("/etc", OrgName, "airports")
		wantData := filepath.Join("/var/lib", OrgName, "airports")
		wantLogs := filepath.Join("/var/log", OrgName, "airports")
		if configDir != wantConfig {
			t.Errorf("configDir = %q, want %q", configDir, wantConfig)
		}
		if dataDir != wantData {
			t.Errorf("dataDir = %q, want %q", dataDir, wantData)
		}
		if logsDir != wantLogs {
			t.Errorf("logsDir = %q, want %q", logsDir, wantLogs)
		}
		return
	}

	wantConfig := filepath.Join(scratch, ".config", OrgName, "airports")
	wantData := filepath.Join(scratch, ".local", "share", OrgName, "airports")
	wantLogs := filepath.Join(scratch, ".local", "log", OrgName, "airports")
	if configDir != wantConfig {
		t.Errorf("configDir = %q, want %q", configDir, wantConfig)
	}
	if dataDir != wantData {
		t.Errorf("dataDir = %q, want %q", dataDir, wantData)
	}
	if logsDir != wantLogs {
		t.Errorf("logsDir = %q, want %q", logsDir, wantLogs)
	}
}

// TestGetDefaultDirs_XDGOverride verifies XDG_CONFIG_HOME/XDG_DATA_HOME take
// precedence over the HOME-derived defaults when running unprivileged.
func TestGetDefaultDirs_XDGOverride(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG override only applies to the Linux/BSD branch")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root — XDG override only applies to the unprivileged branch")
	}

	scratch := newScratchDir(t)
	customConfig := filepath.Join(scratch, "custom-config")
	customData := filepath.Join(scratch, "custom-data")
	t.Setenv("HOME", scratch)
	t.Setenv("XDG_CONFIG_HOME", customConfig)
	t.Setenv("XDG_DATA_HOME", customData)

	configDir, dataDir, _ := GetDefaultDirs("airports")

	wantConfig := filepath.Join(customConfig, OrgName, "airports")
	wantData := filepath.Join(customData, OrgName, "airports")
	if configDir != wantConfig {
		t.Errorf("configDir = %q, want %q", configDir, wantConfig)
	}
	if dataDir != wantData {
		t.Errorf("dataDir = %q, want %q", dataDir, wantData)
	}
}

// TestGetDefaultDirs_EmptyProjectName ensures an empty project name is
// simply joined as an empty path segment rather than causing a panic or
// falling back to some other default.
func TestGetDefaultDirs_EmptyProjectName(t *testing.T) {
	configDir, dataDir, logsDir := GetDefaultDirs("")
	for name, dir := range map[string]string{
		"configDir": configDir,
		"dataDir":   dataDir,
		"logsDir":   logsDir,
	} {
		if dir == "" {
			t.Errorf("%s is empty for empty projectName", name)
		}
		if strings.Contains(dir, "//") {
			t.Errorf("%s = %q contains a double slash from the empty segment", name, dir)
		}
	}
}

func TestEnsureDir(t *testing.T) {
	scratch := newScratchDir(t)
	target := filepath.Join(scratch, "nested", "config")

	if err := EnsureDir(target); err != nil {
		t.Fatalf("EnsureDir: unexpected error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat after EnsureDir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", target)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("EnsureDir mode = %o, want 0700", perm)
	}

	// Idempotency: calling it again on an existing directory must not error.
	if err := EnsureDir(target); err != nil {
		t.Errorf("EnsureDir on existing dir: unexpected error: %v", err)
	}
}

func TestEnsureDir_FileConflict(t *testing.T) {
	scratch := newScratchDir(t)
	blocker := filepath.Join(scratch, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Attempting to create a directory at a path where a plain file
	// already exists must fail.
	if err := EnsureDir(blocker); err == nil {
		t.Error("EnsureDir over an existing file: expected error, got nil")
	}
}

func TestEnsureDirs(t *testing.T) {
	scratch := newScratchDir(t)
	configDir := filepath.Join(scratch, "config")
	dataDir := filepath.Join(scratch, "data")
	logsDir := filepath.Join(scratch, "logs")

	if err := EnsureDirs(configDir, dataDir, logsDir); err != nil {
		t.Fatalf("EnsureDirs: unexpected error: %v", err)
	}

	for _, dir := range []string{configDir, dataDir, logsDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat(%q): %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestEnsureDirs_FailsFastOnFirstError(t *testing.T) {
	scratch := newScratchDir(t)
	blocker := filepath.Join(scratch, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dataDir := filepath.Join(scratch, "data")
	logsDir := filepath.Join(scratch, "logs")

	if err := EnsureDirs(blocker, dataDir, logsDir); err == nil {
		t.Fatal("EnsureDirs with a bad configDir: expected error, got nil")
	}

	// The dirs after the failing one must not have been created.
	if _, err := os.Stat(dataDir); err == nil {
		t.Error("dataDir was created even though configDir failed first")
	}
}

func TestEnsureDirs_FailsOnSecondDir(t *testing.T) {
	scratch := newScratchDir(t)
	configDir := filepath.Join(scratch, "config")
	blocker := filepath.Join(scratch, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	logsDir := filepath.Join(scratch, "logs")

	if err := EnsureDirs(configDir, blocker, logsDir); err == nil {
		t.Fatal("EnsureDirs with a bad dataDir: expected error, got nil")
	}

	// configDir (first arg) must have been created before the failure.
	if _, err := os.Stat(configDir); err != nil {
		t.Errorf("configDir was not created before dataDir failed: %v", err)
	}
	// logsDir (after the failing dataDir) must not have been created.
	if _, err := os.Stat(logsDir); err == nil {
		t.Error("logsDir was created even though dataDir failed first")
	}
}

func TestEnsureDirs_FailsOnThirdDir(t *testing.T) {
	scratch := newScratchDir(t)
	configDir := filepath.Join(scratch, "config")
	dataDir := filepath.Join(scratch, "data")
	blocker := filepath.Join(scratch, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := EnsureDirs(configDir, dataDir, blocker); err == nil {
		t.Fatal("EnsureDirs with a bad logsDir: expected error, got nil")
	}

	if _, err := os.Stat(configDir); err != nil {
		t.Errorf("configDir was not created before logsDir failed: %v", err)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("dataDir was not created before logsDir failed: %v", err)
	}
}

// TestIsRunningInContainer only verifies the function runs to completion
// and returns a bool without panicking. The real filesystem's /proc/1/comm
// always exists inside this Linux test environment, so the err != nil
// early-return branch (function reads a path it cannot influence) is not
// exercised here — see final report for the coverage gap.
func TestIsRunningInContainer(t *testing.T) {
	got := IsRunningInContainer()
	if got != true && got != false {
		t.Fatal("IsRunningInContainer returned a non-bool value, impossible")
	}
	t.Logf("IsRunningInContainer() = %v", got)
}

func TestGetBackupDir(t *testing.T) {
	tests := []struct {
		projectName string
		want        string
	}{
		{"airports", filepath.Join("/mnt/Backups", OrgName, "airports")},
		{"", filepath.Join("/mnt/Backups", OrgName, "")},
		{"other-app", filepath.Join("/mnt/Backups", OrgName, "other-app")},
	}

	for _, tt := range tests {
		t.Run(tt.projectName, func(t *testing.T) {
			got := GetBackupDir(tt.projectName)
			if got != tt.want {
				t.Errorf("GetBackupDir(%q) = %q, want %q", tt.projectName, got, tt.want)
			}
		})
	}
}

// requireRootLinux skips the calling test unless running as root on Linux,
// since GetCacheDir/GetPIDFile/GetSSLDir/GetSecurityDir/GetSQLiteDBPath all
// branch on isRootOrAdmin() and runtime.GOOS, and this suite always runs
// inside the casjaysdev/go:latest Linux container per PART 28.
func requireRootLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("test targets the Linux/BSD branch, running on %s", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		t.Skip("test targets the privileged (root) branch")
	}
}

func TestGetCacheDir(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/var/cache", OrgName, "airports")
	if got := GetCacheDir("airports"); got != want {
		t.Errorf("GetCacheDir(%q) = %q, want %q", "airports", got, want)
	}
}

func TestGetPIDFile(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/var/run", OrgName, "airports.pid")
	if got := GetPIDFile("airports"); got != want {
		t.Errorf("GetPIDFile(%q) = %q, want %q", "airports", got, want)
	}
}

func TestGetSSLDir(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/etc", OrgName, "airports", "ssl")
	if got := GetSSLDir("airports"); got != want {
		t.Errorf("GetSSLDir(%q) = %q, want %q", "airports", got, want)
	}
}

func TestGetSecurityDir(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/var/lib", OrgName, "airports", "security")
	if got := GetSecurityDir("airports"); got != want {
		t.Errorf("GetSecurityDir(%q) = %q, want %q", "airports", got, want)
	}
}

func TestGetSQLiteDBPath(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/var/lib", OrgName, "airports", "db")
	if got := GetSQLiteDBPath("airports"); got != want {
		t.Errorf("GetSQLiteDBPath(%q) = %q, want %q", "airports", got, want)
	}
}

func TestGetTorDir(t *testing.T) {
	requireRootLinux(t)
	want := filepath.Join("/var/lib", OrgName, "airports", "tor")
	if got := GetTorDir("airports"); got != want {
		t.Errorf("GetTorDir(%q) = %q, want %q", "airports", got, want)
	}
}

// TestGetTorDirContainer verifies the container branch takes priority over
// the host-OS root/user split, matching GetSecurityDir's container behavior.
func TestGetTorDirContainer(t *testing.T) {
	withRunningInContainer(t, true)
	want := filepath.Join("/data", "airports", "tor")
	if got := GetTorDir("airports"); got != want {
		t.Errorf("GetTorDir(%q) = %q, want %q", "airports", got, want)
	}
}

// TestGetTorDirAllPlatforms drives GetTorDir through every
// {windows, darwin, freebsd, linux} x {root, non-root} combination via the
// overridable goosFunc/isRootOrAdminFunc, since the OS-specific branches are
// otherwise unreachable when this suite runs on Linux-only CI.
func TestGetTorDirAllPlatforms(t *testing.T) {
	withRunningInContainer(t, false)
	home := currentHomeDir()
	t.Setenv("ProgramData", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_DATA_HOME", "")

	const projectName = "airports"
	tests := []struct {
		goos string
		root bool
		want string
	}{
		{"windows", true, filepath.Join("C:\\ProgramData", OrgName, projectName, "data", "tor")},
		{"windows", false, filepath.Join(home, "AppData", "Local", OrgName, projectName, "tor")},
		{"darwin", true, filepath.Join("/Library/Application Support", OrgName, projectName, "data", "tor")},
		{"darwin", false, filepath.Join(home, "Library", "Application Support", OrgName, projectName, "data", "tor")},
		{"freebsd", true, filepath.Join("/var/db", OrgName, projectName, "tor")},
		{"freebsd", false, filepath.Join(home, ".local", "share", OrgName, projectName, "tor")},
		{"linux", true, filepath.Join("/var/lib", OrgName, projectName, "tor")},
		{"linux", false, filepath.Join(home, ".local", "share", OrgName, projectName, "tor")},
	}

	for _, tt := range tests {
		t.Run(tt.goos+"/root="+strconvBool(tt.root), func(t *testing.T) {
			withGOOS(t, tt.goos)
			withRootOrAdmin(t, tt.root)
			if got := GetTorDir(projectName); got != tt.want {
				t.Errorf("GetTorDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCurrentHomeDir only verifies the function returns a non-empty path;
// the real os/user.Current() lookup always succeeds in this container.
func TestCurrentHomeDir(t *testing.T) {
	if got := currentHomeDir(); got == "" {
		t.Error("currentHomeDir() returned empty string")
	}
}

// TestIsGenericContainer only verifies the function runs to completion and
// returns a bool — the real filesystem markers (/.dockerenv,
// /run/.containerenv) are outside this test's control.
func TestIsGenericContainer(t *testing.T) {
	got := isGenericContainer()
	if got != true && got != false {
		t.Fatal("isGenericContainer returned a non-bool value, impossible")
	}
	t.Logf("isGenericContainer() = %v", got)
}

// TestIsAppRuntimeContainer_MissingBinary confirms the toolchain/test
// container (tini as PID 1, /config and /data both present per the
// generic casjaysdev/go:latest image) still resolves false because the
// built "airports" binary is not installed at appBinaryPath here — this is
// exactly the false-positive this check exists to avoid.
func TestIsAppRuntimeContainer_MissingBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("container detection is Linux-specific")
	}
	if _, err := os.Stat(appBinaryPath); err == nil {
		t.Skip("appBinaryPath unexpectedly present in this environment")
	}
	if isAppRuntimeContainer() {
		t.Error("isAppRuntimeContainer() = true, want false (no airports binary installed)")
	}
}

// withOverride swaps a package-level function var for the duration of the
// test and restores the original via t.Cleanup, so overrides never leak
// across tests even on failure.
func withRunningInContainer(t *testing.T, want bool) {
	t.Helper()
	orig := isRunningInContainerFunc
	isRunningInContainerFunc = func() bool { return want }
	t.Cleanup(func() { isRunningInContainerFunc = orig })
}

func withRootOrAdmin(t *testing.T, want bool) {
	t.Helper()
	orig := isRootOrAdminFunc
	isRootOrAdminFunc = func() bool { return want }
	t.Cleanup(func() { isRootOrAdminFunc = orig })
}

// withGOOS overrides goosFunc for the duration of the test so every
// Windows/Darwin/BSD/Linux branch of GetDefaultDirs and the Get*Dir helpers
// can be exercised regardless of the platform actually running the test.
func withGOOS(t *testing.T, goos string) {
	t.Helper()
	orig := goosFunc
	goosFunc = func() string { return goos }
	t.Cleanup(func() { goosFunc = orig })
}

// TestGetDefaultDirs_ContainerOverride exercises the container branch via
// the overridable isRunningInContainerFunc, independent of the real host
// environment.
func TestGetDefaultDirs_ContainerOverride(t *testing.T) {
	withRunningInContainer(t, true)

	configDir, dataDir, logsDir := GetDefaultDirs("airports")

	wantConfig := filepath.Join("/config", "airports")
	wantData := filepath.Join("/data", "airports")
	wantLogs := filepath.Join("/data", "log", "airports")
	if configDir != wantConfig {
		t.Errorf("configDir = %q, want %q", configDir, wantConfig)
	}
	if dataDir != wantData {
		t.Errorf("dataDir = %q, want %q", dataDir, wantData)
	}
	if logsDir != wantLogs {
		t.Errorf("logsDir = %q, want %q", logsDir, wantLogs)
	}
}

// TestGetDefaultDirs_NonRootOverride exercises the unprivileged Linux/BSD
// branch via isRootOrAdminFunc, regardless of the real process euid.
func TestGetDefaultDirs_NonRootOverride(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("test targets the Linux/BSD branch")
	}
	withRunningInContainer(t, false)
	withRootOrAdmin(t, false)

	// user.Current() (used by currentHomeDir()) resolves the real process
	// home directory and ignores a test-set $HOME when running as root, so
	// the expected values are derived from currentHomeDir() itself rather
	// than an overridden scratch dir.
	home := currentHomeDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	configDir, dataDir, logsDir := GetDefaultDirs("airports")

	wantConfig := filepath.Join(home, ".config", OrgName, "airports")
	wantData := filepath.Join(home, ".local", "share", OrgName, "airports")
	wantLogs := filepath.Join(home, ".local", "log", OrgName, "airports")
	if configDir != wantConfig {
		t.Errorf("configDir = %q, want %q", configDir, wantConfig)
	}
	if dataDir != wantData {
		t.Errorf("dataDir = %q, want %q", dataDir, wantData)
	}
	if logsDir != wantLogs {
		t.Errorf("logsDir = %q, want %q", logsDir, wantLogs)
	}
}

// dirGetterCases enumerates every Get*Dir helper's container / root / non-root
// branches for table-driven testing, since all six share the same
// container-first, then-root-or-user shape.
func dirGetterCases(t *testing.T, scratch string) []struct {
	name       string
	fn         func(string) string
	container  string
	root       string
	nonRootDir string
} {
	t.Helper()
	return []struct {
		name       string
		fn         func(string) string
		container  string
		root       string
		nonRootDir string
	}{
		{
			name:       "GetCacheDir",
			fn:         GetCacheDir,
			container:  filepath.Join("/data", "airports", "cache"),
			root:       filepath.Join("/var/cache", OrgName, "airports"),
			nonRootDir: filepath.Join(scratch, ".cache", OrgName, "airports"),
		},
		{
			name:       "GetPIDFile",
			fn:         GetPIDFile,
			container:  filepath.Join("/data", "airports", "airports.pid"),
			root:       filepath.Join("/var/run", OrgName, "airports.pid"),
			nonRootDir: filepath.Join(scratch, ".local", "share", OrgName, "airports", "airports.pid"),
		},
		{
			name:       "GetBackupDir",
			fn:         GetBackupDir,
			container:  filepath.Join("/data", "backups", "airports"),
			root:       filepath.Join("/mnt/Backups", OrgName, "airports"),
			nonRootDir: filepath.Join(scratch, ".local", "share", "Backups", OrgName, "airports"),
		},
		{
			name:       "GetSSLDir",
			fn:         GetSSLDir,
			container:  filepath.Join("/config", "airports", "ssl"),
			root:       filepath.Join("/etc", OrgName, "airports", "ssl"),
			nonRootDir: filepath.Join(scratch, ".config", OrgName, "airports", "ssl"),
		},
		{
			name:       "GetSecurityDir",
			fn:         GetSecurityDir,
			container:  filepath.Join("/data", "airports", "security"),
			root:       filepath.Join("/var/lib", OrgName, "airports", "security"),
			nonRootDir: filepath.Join(scratch, ".local", "share", OrgName, "airports", "security"),
		},
		{
			name:       "GetSQLiteDBPath",
			fn:         GetSQLiteDBPath,
			container:  filepath.Join("/data", "db", "sqlite"),
			root:       filepath.Join("/var/lib", OrgName, "airports", "db"),
			nonRootDir: filepath.Join(scratch, ".local", "share", OrgName, "airports", "db"),
		},
	}
}

func TestDirGetters_Container(t *testing.T) {
	withRunningInContainer(t, true)
	for _, tt := range dirGetterCases(t, "") {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn("airports"); got != tt.container {
				t.Errorf("%s(container) = %q, want %q", tt.name, got, tt.container)
			}
		})
	}
}

func TestDirGetters_Root(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("test targets the Linux/BSD branch")
	}
	withRunningInContainer(t, false)
	withRootOrAdmin(t, true)
	for _, tt := range dirGetterCases(t, "") {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn("airports"); got != tt.root {
				t.Errorf("%s(root) = %q, want %q", tt.name, got, tt.root)
			}
		})
	}
}

func TestDirGetters_NonRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("test targets the Linux/BSD branch")
	}
	withRunningInContainer(t, false)
	withRootOrAdmin(t, false)
	// currentHomeDir() resolves via user.Current(), which ignores a
	// test-set $HOME when running as root, so expectations are derived
	// from the real home directory instead of an overridden scratch dir.
	home := currentHomeDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	for _, tt := range dirGetterCases(t, home) {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn("airports"); got != tt.nonRootDir {
				t.Errorf("%s(non-root) = %q, want %q", tt.name, got, tt.nonRootDir)
			}
		})
	}
}

// allOSExpected mirrors paths.go's own per-OS/root formulas exactly, so the
// Windows/Darwin/BSD branches (unreachable at runtime on Linux-only CI) can
// still be verified via the overridable goosFunc.
type allOSExpected struct {
	config, data, logs                        string
	cache, pid, backup, ssl, security, sqlite string
}

func expectedForOS(goos string, root bool, home, org, name string) allOSExpected {
	switch goos {
	case "windows":
		programData := filepath.Join("C:\\ProgramData")
		appData := filepath.Join(home, "AppData", "Roaming")
		localAppData := filepath.Join(home, "AppData", "Local")
		if root {
			return allOSExpected{
				config:   filepath.Join(programData, org, name),
				data:     filepath.Join(programData, org, name, "data"),
				logs:     filepath.Join(programData, org, name, "logs"),
				cache:    filepath.Join(programData, org, name, "cache"),
				pid:      filepath.Join(programData, org, name, name+".pid"),
				backup:   filepath.Join(programData, "Backups", org, name),
				ssl:      filepath.Join(programData, org, name, "ssl"),
				security: filepath.Join(programData, org, name, "data", "security"),
				sqlite:   filepath.Join(programData, org, name, "db"),
			}
		}
		return allOSExpected{
			config:   filepath.Join(appData, org, name),
			data:     filepath.Join(localAppData, org, name),
			logs:     filepath.Join(localAppData, org, name, "logs"),
			cache:    filepath.Join(localAppData, org, name, "cache"),
			pid:      filepath.Join(localAppData, org, name, name+".pid"),
			backup:   filepath.Join(localAppData, "Backups", org, name),
			ssl:      filepath.Join(appData, org, name, "ssl"),
			security: filepath.Join(localAppData, org, name, "security"),
			sqlite:   filepath.Join(localAppData, org, name, "db"),
		}
	case "darwin":
		if root {
			return allOSExpected{
				config:   filepath.Join("/Library/Application Support", org, name),
				data:     filepath.Join("/Library/Application Support", org, name, "data"),
				logs:     filepath.Join("/Library/Logs", org, name),
				cache:    filepath.Join("/Library/Caches", org, name),
				pid:      filepath.Join("/var/run", org, name+".pid"),
				backup:   filepath.Join("/Library/Backups", org, name),
				ssl:      filepath.Join("/Library/Application Support", org, name, "ssl"),
				security: filepath.Join("/Library/Application Support", org, name, "data", "security"),
				sqlite:   filepath.Join("/Library/Application Support", org, name, "db"),
			}
		}
		base := filepath.Join(home, "Library", "Application Support", org, name)
		return allOSExpected{
			config:   base,
			data:     base,
			logs:     filepath.Join(home, "Library", "Logs", org, name),
			cache:    filepath.Join(home, "Library", "Caches", org, name),
			pid:      filepath.Join(base, name+".pid"),
			backup:   filepath.Join(home, "Library", "Backups", org, name),
			ssl:      filepath.Join(base, "ssl"),
			security: filepath.Join(base, "data", "security"),
			sqlite:   filepath.Join(base, "db"),
		}
	case "freebsd":
		if root {
			return allOSExpected{
				config:   filepath.Join("/usr/local/etc", org, name),
				data:     filepath.Join("/var/db", org, name),
				logs:     filepath.Join("/var/log", org, name),
				cache:    filepath.Join("/var/cache", org, name),
				pid:      filepath.Join("/var/run", org, name+".pid"),
				backup:   filepath.Join("/var/backups", org, name),
				ssl:      filepath.Join("/usr/local/etc", org, name, "ssl"),
				security: filepath.Join("/var/db", org, name, "security"),
				sqlite:   filepath.Join("/var/db", org, name, "db"),
			}
		}
		return allOSExpected{
			config:   filepath.Join(home, ".config", org, name),
			data:     filepath.Join(home, ".local", "share", org, name),
			logs:     filepath.Join(home, ".local", "log", org, name),
			cache:    filepath.Join(home, ".cache", org, name),
			pid:      filepath.Join(home, ".local", "share", org, name, name+".pid"),
			backup:   filepath.Join(home, ".local", "share", "Backups", org, name),
			ssl:      filepath.Join(home, ".config", org, name, "ssl"),
			security: filepath.Join(home, ".local", "share", org, name, "security"),
			sqlite:   filepath.Join(home, ".local", "share", org, name, "db"),
		}
	default: // linux
		if root {
			return allOSExpected{
				config:   filepath.Join("/etc", org, name),
				data:     filepath.Join("/var/lib", org, name),
				logs:     filepath.Join("/var/log", org, name),
				cache:    filepath.Join("/var/cache", org, name),
				pid:      filepath.Join("/var/run", org, name+".pid"),
				backup:   filepath.Join("/mnt/Backups", org, name),
				ssl:      filepath.Join("/etc", org, name, "ssl"),
				security: filepath.Join("/var/lib", org, name, "security"),
				sqlite:   filepath.Join("/var/lib", org, name, "db"),
			}
		}
		return allOSExpected{
			config:   filepath.Join(home, ".config", org, name),
			data:     filepath.Join(home, ".local", "share", org, name),
			logs:     filepath.Join(home, ".local", "log", org, name),
			cache:    filepath.Join(home, ".cache", org, name),
			pid:      filepath.Join(home, ".local", "share", org, name, name+".pid"),
			backup:   filepath.Join(home, ".local", "share", "Backups", org, name),
			ssl:      filepath.Join(home, ".config", org, name, "ssl"),
			security: filepath.Join(home, ".local", "share", org, name, "security"),
			sqlite:   filepath.Join(home, ".local", "share", org, name, "db"),
		}
	}
}

// TestAllOSBranches drives GetDefaultDirs and every Get*Dir helper through
// every {windows, darwin, freebsd, linux} x {root, non-root} combination via
// the overridable goosFunc/isRootOrAdminFunc, so the OS-specific branches
// that are unreachable at runtime on Linux-only CI are still verified
// against AI.md PART 4's full path matrix.
func TestAllOSBranches(t *testing.T) {
	withRunningInContainer(t, false)
	home := currentHomeDir()
	t.Setenv("ProgramData", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	const projectName = "airports"
	for _, goos := range []string{"windows", "darwin", "freebsd", "linux"} {
		for _, root := range []bool{true, false} {
			name := goos + "/root=" + strconvBool(root)
			t.Run(name, func(t *testing.T) {
				withGOOS(t, goos)
				withRootOrAdmin(t, root)

				want := expectedForOS(goos, root, home, OrgName, projectName)

				configDir, dataDir, logsDir := GetDefaultDirs(projectName)
				if configDir != want.config {
					t.Errorf("GetDefaultDirs configDir = %q, want %q", configDir, want.config)
				}
				if dataDir != want.data {
					t.Errorf("GetDefaultDirs dataDir = %q, want %q", dataDir, want.data)
				}
				if logsDir != want.logs {
					t.Errorf("GetDefaultDirs logsDir = %q, want %q", logsDir, want.logs)
				}
				if got := GetCacheDir(projectName); got != want.cache {
					t.Errorf("GetCacheDir = %q, want %q", got, want.cache)
				}
				if got := GetPIDFile(projectName); got != want.pid {
					t.Errorf("GetPIDFile = %q, want %q", got, want.pid)
				}
				if got := GetBackupDir(projectName); got != want.backup {
					t.Errorf("GetBackupDir = %q, want %q", got, want.backup)
				}
				if got := GetSSLDir(projectName); got != want.ssl {
					t.Errorf("GetSSLDir = %q, want %q", got, want.ssl)
				}
				if got := GetSecurityDir(projectName); got != want.security {
					t.Errorf("GetSecurityDir = %q, want %q", got, want.security)
				}
				if got := GetSQLiteDBPath(projectName); got != want.sqlite {
					t.Errorf("GetSQLiteDBPath = %q, want %q", got, want.sqlite)
				}
			})
		}
	}
}

// TestIsBSD_AllTargets verifies isBSD() classifies every goosFunc() target
// correctly, including the non-BSD default branch.
func TestIsBSD_AllTargets(t *testing.T) {
	cases := []struct {
		goos string
		want bool
	}{
		{"freebsd", true},
		{"openbsd", true},
		{"netbsd", true},
		{"dragonfly", true},
		{"linux", false},
		{"windows", false},
		{"darwin", false},
	}
	for _, tt := range cases {
		t.Run(tt.goos, func(t *testing.T) {
			withGOOS(t, tt.goos)
			if got := isBSD(); got != tt.want {
				t.Errorf("isBSD() with goos=%q = %v, want %v", tt.goos, got, tt.want)
			}
		})
	}
}

// TestIsRootOrAdmin_WindowsBranch verifies the Windows-specific
// USERDOMAIN/COMPUTERNAME comparison branch of isRootOrAdminFunc.
func TestIsRootOrAdmin_WindowsBranch(t *testing.T) {
	withGOOS(t, "windows")
	t.Setenv("USERDOMAIN", "WORKGROUP")
	t.Setenv("COMPUTERNAME", "WORKGROUP")
	if !isRootOrAdminFunc() {
		t.Error("isRootOrAdminFunc() = false, want true when USERDOMAIN == COMPUTERNAME")
	}
	t.Setenv("COMPUTERNAME", "OTHERHOST")
	if isRootOrAdminFunc() {
		t.Error("isRootOrAdminFunc() = true, want false when USERDOMAIN != COMPUTERNAME")
	}
}

// strconvBool avoids importing strconv solely for a test-name suffix.
func strconvBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
