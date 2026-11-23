package service

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/apimgr/airports/src/path"
)

const (
	appName = "airports"
	orgName = "apimgr"

	// serviceGecos is the Gecos/RealName field used when creating the
	// dedicated service account.
	serviceGecos = "airports service account"

	// linuxSafeTop is the top of the safe UID/GID search range on Linux/BSD.
	linuxSafeTop = 899
	// darwinSafeTop is the top of the safe UID/GID search range on macOS.
	darwinSafeTop = 399
	// safeBottom is the bottom of the safe UID/GID search range on all platforms.
	safeBottom = 200
)

// ServiceType represents the type of service manager
type ServiceType int

const (
	ServiceUnknown ServiceType = iota
	ServiceSystemd
	ServiceOpenRC
	ServiceSysVinit
	ServiceRunit
	ServiceLaunchd
	ServiceWindows
	ServiceBSDRC
	ServiceS6
)

// DetectServiceManager detects the system's service manager.
// Linux priority order follows AI.md: systemd -> OpenRC -> runit/s6 -> SysVinit.
func DetectServiceManager() ServiceType {
	return detectServiceManager(runtime.GOOS, func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}, func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}, s6ScanDirectory() != "")
}

func detectServiceManager(goos string, exists func(string) bool, commandAvailable func(string) bool, s6Running bool) ServiceType {
	switch goos {
	case "linux":
		if exists("/run/systemd/system") {
			return ServiceSystemd
		}
		if exists("/sbin/openrc-run") && exists("/run/openrc") {
			return ServiceOpenRC
		}
		if exists("/etc/runit") || commandAvailable("sv") {
			return ServiceRunit
		}
		if commandAvailable("s6-svc") && s6Running {
			return ServiceS6
		}
		if exists("/etc/init.d") && (commandAvailable("update-rc.d") || commandAvailable("chkconfig")) {
			return ServiceSysVinit
		}
		return ServiceUnknown

	case "darwin":
		return ServiceLaunchd

	case "windows":
		return ServiceWindows

	case "freebsd", "openbsd", "netbsd":
		return ServiceBSDRC

	default:
		return ServiceUnknown
	}
}

func s6ScanDirectory() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		if _, statErr := os.Stat("/run/service"); statErr == nil {
			return "/run/service"
		}
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		processDir := filepath.Join("/proc", entry.Name())
		comm, commErr := os.ReadFile(filepath.Join(processDir, "comm"))
		if commErr == nil && strings.TrimSpace(string(comm)) != "s6-svscan" {
			continue
		}
		cmdline, readErr := os.ReadFile(filepath.Join(processDir, "cmdline"))
		if readErr != nil || len(cmdline) == 0 {
			if commErr == nil && strings.TrimSpace(string(comm)) == "s6-svscan" {
				return "/run/service"
			}
			continue
		}
		args := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
		if len(args) == 0 || filepath.Base(args[0]) != "s6-svscan" {
			if commErr != nil || strings.TrimSpace(string(comm)) != "s6-svscan" {
				continue
			}
			return "/run/service"
		}
		for _, arg := range args[1:] {
			if filepath.IsAbs(arg) {
				return filepath.Clean(arg)
			}
		}
		return "/run/service"
	}
	return ""
}

func s6ServicePath() string {
	scanDir := s6ScanDirectory()
	if scanDir == "" {
		scanDir = "/run/service"
	}
	return filepath.Join(scanDir, appName)
}

// Install installs the service for the detected service manager.
// Install creates the dedicated system account before installing a service
// whose init script may set directory ownership during its first start.
func InstallSystemService() error {
	if runtime.GOOS != "windows" && IsElevated() {
		if err := EnsureSystemUser(); err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}
	}

	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return installSystemd()
	case ServiceOpenRC:
		return installOpenRC()
	case ServiceSysVinit:
		return installSysVinit()
	case ServiceRunit:
		return installRunit()
	case ServiceS6:
		return installS6()
	case ServiceLaunchd:
		return installLaunchd()
	case ServiceWindows:
		return installWindows()
	case ServiceBSDRC:
		return installBSDRC()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Uninstall removes the service. Per spec this also deletes config, data,
// cache, log, and backup directories, the PID file, and the dedicated
// system user/group — but never the binary itself. It requires an explicit
// [y/N] confirmation before any destructive action is taken.
func UninstallSystemService() error {
	if !confirmUninstall() {
		fmt.Println("Uninstall cancelled.")
		return nil
	}

	serviceType := DetectServiceManager()

	var err error
	switch serviceType {
	case ServiceSystemd:
		err = uninstallSystemd()
	case ServiceOpenRC:
		err = uninstallOpenRC()
	case ServiceSysVinit:
		err = uninstallSysVinit()
	case ServiceRunit:
		err = uninstallRunit()
	case ServiceS6:
		err = uninstallS6()
	case ServiceLaunchd:
		err = uninstallLaunchd()
	case ServiceWindows:
		err = uninstallWindows()
	case ServiceBSDRC:
		err = uninstallBSDRC()
	default:
		err = fmt.Errorf("unsupported service manager")
	}
	if err != nil {
		return err
	}

	removeAllData()
	binPath := GetSystemServiceBinaryPath()
	fmt.Println()
	fmt.Printf("Config, data, cache, log, and backup directories, the PID file, and the %q system user/group have been removed.\n", appName)
	fmt.Printf("The binary at %s was left in place — remove it manually if no longer needed.\n", binPath)

	return nil
}

// confirmUninstall prompts the operator for explicit [y/N] confirmation.
func confirmUninstall() bool {
	fmt.Print("This will delete ALL data, configs, and the system user. Continue? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// removeAllData deletes config/data/logs/cache/backup dirs, the PID file,
// and the dedicated system user/group. Errors are logged, not fatal — best
// effort cleanup so a single missing path never blocks the rest.
func removeAllData() {
	configDir, dataDir, logsDir := paths.GetDefaultDirs(appName)
	backupDir := paths.GetBackupDir("", dataDir)
	cacheDir := paths.GetCacheDir(appName)
	pidFile := paths.GetPIDFile(appName)

	for _, dir := range []string{configDir, dataDir, logsDir, cacheDir, backupDir} {
		if dir == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			fmt.Printf("warning: failed to remove %s: %v\n", dir, err)
		}
	}
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("warning: failed to remove %s: %v\n", pidFile, err)
	}

	if err := DeleteSystemUser(); err != nil {
		fmt.Printf("warning: failed to remove system user/group: %v\n", err)
	}
}

// GetBinaryPath returns the path where the binary should be installed
func GetSystemServiceBinaryPath() string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf(`C:\Program Files\%s\%s\%s.exe`, orgName, appName, appName)
	default:
		return fmt.Sprintf("/usr/local/bin/%s", appName)
	}
}

// ---------------------------------------------------------------------
// System user/group management (PART 23)
// ---------------------------------------------------------------------

// isReservedID reports whether id is a well-known/reserved UID or GID that
// must never be assigned to the service account, even if it looks free.
func isReservedID(id int) bool {
	switch {
	case id == 65534:
		return true
	case id >= 980 && id <= 999:
		return true
	case id >= 101 && id <= 110:
		return true
	case id >= 170 && id <= 179:
		return true
	default:
		return false
	}
}

// idAvailable reports whether both a UID and a GID of the same value are
// currently unused on the system.
func idAvailable(id int) bool {
	idStr := strconv.Itoa(id)
	if _, err := user.LookupId(idStr); err == nil {
		return false
	}
	if _, err := user.LookupGroupId(idStr); err == nil {
		return false
	}
	return true
}

// findAvailableID searches the safe range top-down, skipping reserved and
// already-used IDs, and returns the first free value.
func findAvailableID(top int) (int, error) {
	for id := top; id >= safeBottom; id-- {
		if isReservedID(id) {
			continue
		}
		if idAvailable(id) {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no available UID/GID in safe range %d-%d", safeBottom, top)
}

// nologinShell returns the platform-appropriate no-login shell path.
func nologinShell() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/bin/false"
	default:
		if _, err := os.Stat("/sbin/nologin"); err == nil {
			return "/sbin/nologin"
		}
		if _, err := os.Stat("/usr/sbin/nologin"); err == nil {
			return "/usr/sbin/nologin"
		}
		return "/usr/sbin/nologin"
	}
}

// serviceHomeDir returns the home directory used for the dedicated system
// account — the config dir by default.
func serviceHomeDir() string {
	_, dataDir, _ := paths.GetDefaultDirs(appName)
	return dataDir
}

// EnsureSystemUser creates the dedicated "airports" system user/group if it
// does not already exist. It searches the safe UID/GID range top-down,
// skipping reserved IDs, creates the home directory before the user, and
// uses a no-login shell. Idempotent — safe to call on every startup.
func EnsureSystemUser() error {
	if runtime.GOOS == "windows" {
		// Windows uses a Virtual Service Account managed by the SCM —
		// nothing to create here.
		return nil
	}

	if _, err := user.Lookup(appName); err == nil {
		// Already exists.
		return nil
	}

	top := linuxSafeTop
	if runtime.GOOS == "darwin" {
		top = darwinSafeTop
	}

	id, err := findAvailableID(top)
	if err != nil {
		return err
	}

	homeDir := serviceHomeDir()
	if err := createSystemGroup(id); err != nil {
		return err
	}
	if err := createSystemUser(id, homeDir); err != nil {
		return err
	}

	return nil
}

// createSystemGroup creates the "airports" group with the given GID.
func createSystemGroup(gid int) error {
	switch runtime.GOOS {
	case "darwin":
		gidStr := strconv.Itoa(gid)
		steps := [][]string{
			{"dscl", ".", "-create", "/Groups/" + appName},
			{"dscl", ".", "-create", "/Groups/" + appName, "PrimaryGroupID", gidStr},
			// Lock the group account — no usable password.
			{"dscl", ".", "-create", "/Groups/" + appName, "Password", "*"},
		}
		for _, args := range steps {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
				return fmt.Errorf("failed to create macOS group (%v): %w", args, err)
			}
		}
		return nil
	default:
		cmd := exec.Command("groupadd", "-g", strconv.Itoa(gid), "-r", appName)
		return cmd.Run()
	}
}

// createSystemUser creates the "airports" user with the given UID/GID,
// no-login shell, the required Gecos string, and the given home directory.
func createSystemUser(id int, homeDir string) error {
	shell := nologinShell()

	switch runtime.GOOS {
	case "darwin":
		userPath := "/Users/" + appName
		idStr := strconv.Itoa(id)
		steps := [][]string{
			{"dscl", ".", "-create", userPath},
			{"dscl", ".", "-create", userPath, "UserShell", shell},
			{"dscl", ".", "-create", userPath, "RealName", serviceGecos},
			{"dscl", ".", "-create", userPath, "UniqueID", idStr},
			{"dscl", ".", "-create", userPath, "PrimaryGroupID", idStr},
			{"dscl", ".", "-create", userPath, "NFSHomeDirectory", homeDir},
			// Lock the account — no usable password (non-login system account).
			{"dscl", ".", "-create", userPath, "Password", "*"},
			// Hide the account from the macOS login window.
			{"dscl", ".", "-create", userPath, "IsHidden", "1"},
		}
		for _, args := range steps {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
				return fmt.Errorf("failed to create macOS user (%v): %w", args, err)
			}
		}
		return nil
	default:
		cmd := exec.Command("useradd",
			"-u", strconv.Itoa(id),
			"-g", strconv.Itoa(id),
			"-d", homeDir,
			"-s", shell,
			"-c", serviceGecos,
			"-M",
			"-r",
			appName,
		)
		return cmd.Run()
	}
}

// DeleteSystemUser removes the dedicated "airports" system user and group.
// Used by Uninstall — never called during normal operation.
func DeleteSystemUser() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if _, err := user.Lookup(appName); err != nil {
		// Already gone.
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		userErr := exec.Command("dscl", ".", "-delete", "/Users/"+appName).Run()
		groupErr := exec.Command("dscl", ".", "-delete", "/Groups/"+appName).Run()
		return errors.Join(userErr, groupErr)
	default:
		userErr := exec.Command("userdel", appName).Run()
		groupErr := exec.Command("groupdel", appName).Run()
		return errors.Join(userErr, groupErr)
	}
}

// ---------------------------------------------------------------------
// systemd
// ---------------------------------------------------------------------

// installSystemd creates systemd service file
func installSystemd() error {
	binaryPath := GetSystemServiceBinaryPath()

	serviceContent := fmt.Sprintf(`[Unit]
Description=Airports API Server
Documentation=https://airports.apimgr.us
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=%s
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

# The process starts elevated for privileged setup and drops privileges after startup.
# ProtectSystem=true keeps /etc writable for account creation while protecting /usr and /boot.
ProtectSystem=true
ProtectHome=yes
PrivateTmp=yes
ExecStartPre=+/usr/bin/install -d -m 0755 /etc/%s/%s /var/lib/%s/%s /var/cache/%s/%s /var/log/%s/%s /var/run/%s/%s /mnt/Backups/%s/%s
ReadWritePaths=-/etc/%s/%s
ReadWritePaths=-/var/lib/%s/%s
ReadWritePaths=-/var/cache/%s/%s
ReadWritePaths=-/var/log/%s/%s
ReadWritePaths=-/var/run/%s/%s
ReadWritePaths=-/mnt/Backups/%s/%s
ReadWritePaths=-/usr/local/bin

[Install]
WantedBy=multi-user.target
`, binaryPath,
		orgName, appName, orgName, appName, orgName, appName,
		orgName, appName, orgName, appName, orgName, appName,
		orgName, appName, orgName, appName, orgName, appName,
		orgName, appName, orgName, appName, orgName, appName)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", appName)

	// Create directories
	dirs := []string{
		fmt.Sprintf("/var/lib/%s/%s", orgName, appName),
		fmt.Sprintf("/var/cache/%s/%s", orgName, appName),
		fmt.Sprintf("/var/log/%s/%s", orgName, appName),
		fmt.Sprintf("/var/run/%s/%s", orgName, appName),
		fmt.Sprintf("/mnt/Backups/%s/%s", orgName, appName),
		fmt.Sprintf("/etc/%s/%s", orgName, appName),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write service file
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Copy binary if not already in place
	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	// Reload systemd
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := exec.Command("systemctl", "enable", appName).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Printf("Service installed at: %s\n", servicePath)
	fmt.Printf("Binary installed at: %s\n", binaryPath)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  sudo systemctl start %s\n", appName)
	fmt.Println()
	fmt.Println("To check status:")
	fmt.Printf("  sudo systemctl status %s\n", appName)

	return nil
}

// uninstallSystemd removes systemd service
func uninstallSystemd() error {
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", appName)

	// Stop service if running
	_ = exec.Command("systemctl", "stop", appName).Run()

	// Disable service
	_ = exec.Command("systemctl", "disable", appName).Run()

	// Remove service file
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	_ = exec.Command("systemctl", "daemon-reload").Run()

	fmt.Printf("Service uninstalled: %s\n", servicePath)
	return nil
}

// ---------------------------------------------------------------------
// OpenRC
// ---------------------------------------------------------------------

// installOpenRC creates an OpenRC init script
func installOpenRC() error {
	binaryPath := GetSystemServiceBinaryPath()
	scriptPath := fmt.Sprintf("/etc/init.d/%s", appName)

	scriptContent := fmt.Sprintf(`#!/sbin/openrc-run

name="%s"
description="Airports API Server"
command="%s"
command_background=true
pidfile="/var/run/%s/%s/%s.pid"
output_log="/var/log/%s/%s/server.log"
error_log="/var/log/%s/%s/error.log"

depend() {
	need net
	after firewall
}

start_pre() {
	checkpath -d -m 0755 /var/run/%s
	checkpath -d -m 0755 /var/log/%s
	checkpath -d -m 0755 /var/lib/%s
	checkpath -d -m 0755 /etc/%s
	checkpath -d -m 0755 -o %s:%s /var/run/%s/%s
	checkpath -d -m 0755 -o %s:%s /var/log/%s/%s
	checkpath -d -m 0755 -o %s:%s /var/lib/%s/%s
	checkpath -d -m 0755 -o %s:%s /etc/%s/%s
}
`, appName, binaryPath, orgName, appName, appName, orgName, appName, orgName, appName, orgName, orgName, orgName, orgName, appName, appName, orgName, appName, appName, appName, orgName, appName, appName, appName, orgName, appName, appName, appName, orgName, appName)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write OpenRC script: %w", err)
	}

	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	if err := exec.Command("rc-update", "add", appName, "default").Run(); err != nil {
		return fmt.Errorf("failed to enable OpenRC service: %w", err)
	}

	fmt.Printf("OpenRC service installed at: %s\n", scriptPath)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  rc-service %s start\n", appName)

	return nil
}

// uninstallOpenRC removes the OpenRC init script
func uninstallOpenRC() error {
	scriptPath := fmt.Sprintf("/etc/init.d/%s", appName)

	_ = exec.Command("rc-service", appName, "stop").Run()
	_ = exec.Command("rc-update", "del", appName, "default").Run()

	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove OpenRC script: %w", err)
	}

	fmt.Printf("OpenRC service uninstalled: %s\n", scriptPath)
	return nil
}

// ---------------------------------------------------------------------
// SysVinit
// ---------------------------------------------------------------------

// installSysVinit creates a SysVinit init.d script
func installSysVinit() error {
	binaryPath := GetSystemServiceBinaryPath()
	scriptPath := fmt.Sprintf("/etc/init.d/%s", appName)

	scriptContent := fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          %s
# Required-Start:    $network $remote_fs
# Required-Stop:     $network $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Airports API Server
### END INIT INFO

NAME=%s
DAEMON=%s
PIDFILE=/var/run/$NAME.pid

case "$1" in
  start)
    start-stop-daemon --start --background --make-pidfile --pidfile $PIDFILE --exec $DAEMON
    ;;
  stop)
    start-stop-daemon --stop --pidfile $PIDFILE
    ;;
  restart)
    $0 stop
    $0 start
    ;;
  status)
    start-stop-daemon --status --pidfile $PIDFILE
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
`, appName, appName, binaryPath)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write SysVinit script: %w", err)
	}

	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	if _, err := exec.LookPath("update-rc.d"); err == nil {
		if err := exec.Command("update-rc.d", appName, "defaults").Run(); err != nil {
			return fmt.Errorf("failed to register SysVinit service: %w", err)
		}
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		_ = exec.Command("chkconfig", "--add", appName).Run()
		if err := exec.Command("chkconfig", appName, "on").Run(); err != nil {
			return fmt.Errorf("failed to register SysVinit service: %w", err)
		}
	}

	fmt.Printf("SysVinit script installed at: %s\n", scriptPath)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  service %s start\n", appName)

	return nil
}

// uninstallSysVinit removes the SysVinit init.d script
func uninstallSysVinit() error {
	scriptPath := fmt.Sprintf("/etc/init.d/%s", appName)

	_ = exec.Command("service", appName, "stop").Run()

	if _, err := exec.LookPath("update-rc.d"); err == nil {
		_ = exec.Command("update-rc.d", "-f", appName, "remove").Run()
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		_ = exec.Command("chkconfig", appName, "off").Run()
		_ = exec.Command("chkconfig", "--del", appName).Run()
	}

	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove SysVinit script: %w", err)
	}

	fmt.Printf("SysVinit service uninstalled: %s\n", scriptPath)
	return nil
}

// ---------------------------------------------------------------------
// runit
// ---------------------------------------------------------------------

// installRunit creates runit service
func installRunit() error {
	svDir := fmt.Sprintf("/etc/sv/%s", appName)
	binaryPath := GetSystemServiceBinaryPath()

	// Create service directory
	if err := os.MkdirAll(svDir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	runScript := fmt.Sprintf(`#!/bin/sh
exec %s 2>&1
`, binaryPath)

	runPath := filepath.Join(svDir, "run")
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	// Create log directory
	logDir := filepath.Join(svDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Resolve the app log directory so svlogd writes to the canonical
	// {log_dir} (e.g. /var/log/apimgr/airports), never a relative path.
	_, _, appLogDir := paths.GetDefaultDirs(appName)
	if appLogDir == "" {
		appLogDir = fmt.Sprintf("/var/log/%s/%s", orgName, appName)
	}

	logRunScript := fmt.Sprintf(`#!/bin/sh
mkdir -p %s
exec svlogd -tt %s
`, appLogDir, appLogDir)
	logRunPath := filepath.Join(logDir, "run")
	if err := os.WriteFile(logRunPath, []byte(logRunScript), 0755); err != nil {
		return fmt.Errorf("failed to write log run script: %w", err)
	}

	// Link to service directory
	linkPath := fmt.Sprintf("/var/service/%s", appName)
	if err := os.Symlink(svDir, linkPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to link runit service directory: %w", err)
	}

	fmt.Printf("Runit service installed at: %s\n", svDir)
	return nil
}

// uninstallRunit removes runit service
func uninstallRunit() error {
	svDir := fmt.Sprintf("/etc/sv/%s", appName)
	linkPath := fmt.Sprintf("/var/service/%s", appName)

	// Stop service
	_ = exec.Command("sv", "stop", appName).Run()

	// Remove link
	_ = os.Remove(linkPath)

	// Remove service directory
	_ = os.RemoveAll(svDir)

	fmt.Printf("Runit service uninstalled\n")
	return nil
}

func installS6() error {
	scanDir := s6ScanDirectory()
	if scanDir == "" {
		return fmt.Errorf("could not find the running s6-svscan directory")
	}
	svDir := filepath.Join("/etc/s6/sv", appName)
	if err := os.MkdirAll(filepath.Join(svDir, "log"), 0755); err != nil {
		return fmt.Errorf("failed to create s6 service directory: %w", err)
	}
	runScript := fmt.Sprintf("#!/bin/sh\nexec %s 2>&1\n", shellQuote(GetSystemServiceBinaryPath()))
	if err := os.WriteFile(filepath.Join(svDir, "run"), []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write s6 run script: %w", err)
	}
	_, _, appLogDir := paths.GetDefaultDirs(appName)
	if appLogDir == "" {
		appLogDir = filepath.Join("/var/log", orgName, appName)
	}
	logRun := fmt.Sprintf("#!/bin/sh\nexec s6-log n10 s1000000 %s\n", shellQuote(appLogDir))
	if err := os.WriteFile(filepath.Join(svDir, "log", "run"), []byte(logRun), 0755); err != nil {
		return fmt.Errorf("failed to write s6 log run script: %w", err)
	}
	if err := enableS6(); err != nil {
		return err
	}
	fmt.Printf("s6 service installed at: %s\n", svDir)
	return nil
}

func enableS6() error {
	scanDir := s6ScanDirectory()
	if scanDir == "" {
		return fmt.Errorf("could not find the running s6-svscan directory")
	}
	svDir := filepath.Join("/etc/s6/sv", appName)
	linkPath := filepath.Join(scanDir, appName)
	if err := os.Symlink(svDir, linkPath); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to enable s6 service: %w", err)
		}
		linkedDirectory, resolveErr := filepath.EvalSymlinks(linkPath)
		if resolveErr != nil || linkedDirectory != svDir {
			return fmt.Errorf("s6 service path already exists: %s", linkPath)
		}
	}
	if err := exec.Command("s6-svscanctl", "-a", scanDir).Run(); err != nil {
		return fmt.Errorf("failed to reload s6 scan directory: %w", err)
	}
	if err := exec.Command("s6-svc", "-u", linkPath).Run(); err != nil {
		return fmt.Errorf("failed to start s6 service: %w", err)
	}
	return nil
}

func disableS6() error {
	scanDir := s6ScanDirectory()
	if scanDir == "" {
		scanDir = "/run/service"
	}
	linkPath := filepath.Join(scanDir, appName)
	_ = exec.Command("s6-svc", "-d", linkPath).Run()
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to disable s6 service: %w", err)
	}
	if err := exec.Command("s6-svscanctl", "-a", scanDir).Run(); err != nil {
		return fmt.Errorf("failed to reload s6 scan directory: %w", err)
	}
	return nil
}

func uninstallS6() error {
	if err := disableS6(); err != nil {
		return err
	}
	svDir := filepath.Join("/etc/s6/sv", appName)
	if err := os.RemoveAll(svDir); err != nil {
		return fmt.Errorf("failed to remove s6 service directory: %w", err)
	}
	fmt.Println("s6 service uninstalled")
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// ---------------------------------------------------------------------
// launchd (macOS)
// ---------------------------------------------------------------------

// installLaunchd creates macOS launchd plist
func installLaunchd() error {
	binaryPath := GetSystemServiceBinaryPath()
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.github.%s.%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/Library/Logs/%s/%s/error.log</string>
    <key>StandardOutPath</key>
    <string>/Library/Logs/%s/%s/output.log</string>
</dict>
</plist>
`, orgName, appName, binaryPath, orgName, appName, orgName, appName)

	// Create directories
	dirs := []string{
		fmt.Sprintf("/Library/Application Support/%s/%s", orgName, appName),
		fmt.Sprintf("/Library/Logs/%s/%s", orgName, appName),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Copy binary
	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	fmt.Printf("LaunchDaemon installed at: %s\n", plistPath)
	fmt.Println()
	fmt.Println("To load the service:")
	fmt.Printf("  sudo launchctl load %s\n", plistPath)

	return nil
}

// uninstallLaunchd removes macOS launchd plist
func uninstallLaunchd() error {
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)

	// Unload if running
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	// Remove plist
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	fmt.Printf("LaunchDaemon uninstalled\n")
	return nil
}

// ---------------------------------------------------------------------
// Windows
// ---------------------------------------------------------------------

// windowsVSA is the Virtual Service Account used for the Windows service —
// never Local System or a logged-in user account.
const windowsVSA = `NT SERVICE\airports`

// installWindows creates Windows service running under a Virtual Service
// Account (VSA), never LocalSystem/Administrator/a logged-in user.
func installWindows() error {
	binaryPath := GetSystemServiceBinaryPath()

	// Copy binary
	binDir := filepath.Dir(binaryPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	// Create service using sc.exe under the Virtual Service Account —
	// no password is required or supported for VSAs.
	displayName := cases.Title(language.English).String(appName) + " API"
	cmd := exec.Command("sc.exe", "create", appName,
		"binPath=", binaryPath,
		"DisplayName=", displayName,
		"obj=", windowsVSA,
		"start=", "auto")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}

	fmt.Printf("Windows service '%s' installed under %s\n", appName, windowsVSA)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  sc.exe start %s\n", appName)

	return nil
}

// uninstallWindows removes Windows service
func uninstallWindows() error {
	// Stop service
	_ = exec.Command("sc.exe", "stop", appName).Run()

	// Delete service
	if err := exec.Command("sc.exe", "delete", appName).Run(); err != nil {
		return fmt.Errorf("failed to delete Windows service: %w", err)
	}

	fmt.Printf("Windows service '%s' uninstalled\n", appName)
	return nil
}

// ---------------------------------------------------------------------
// BSD rc.d
// ---------------------------------------------------------------------

// installBSDRC creates BSD rc.d script
func installBSDRC() error {
	binaryPath := GetSystemServiceBinaryPath()
	rcPath := fmt.Sprintf("/usr/local/etc/rc.d/%s", appName)

	rcContent := fmt.Sprintf(`#!/bin/sh

# PROVIDE: %s
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="%s"
rcvar="%s_enable"
command="%s"
pidfile="/var/run/%s.pid"

load_rc_config $name
: ${%s_enable:="NO"}

run_rc_command "$1"
`, appName, appName, appName, binaryPath, appName, appName)

	if err := os.WriteFile(rcPath, []byte(rcContent), 0755); err != nil {
		return fmt.Errorf("failed to write rc.d script: %w", err)
	}

	// Copy binary
	if exePath, err := os.Executable(); err == nil && exePath != binaryPath {
		if err := copyBinary(exePath, binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	fmt.Printf("BSD rc.d script installed at: %s\n", rcPath)
	fmt.Println()
	fmt.Printf("Add '%s_enable=\"YES\"' to /etc/rc.conf\n", appName)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  service %s start\n", appName)

	return nil
}

// uninstallBSDRC removes BSD rc.d script
func uninstallBSDRC() error {
	rcPath := fmt.Sprintf("/usr/local/etc/rc.d/%s", appName)

	// Stop service
	_ = exec.Command("service", appName, "stop").Run()

	// Remove script
	if err := os.Remove(rcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove rc.d script: %w", err)
	}

	fmt.Printf("BSD rc.d script uninstalled\n")
	return nil
}

// ---------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------

// copyBinary copies the binary to the destination
func copyBinary(src, dst string) error {
	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Read source
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Write to destination
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return err
	}

	return nil
}

// StartSystemService starts the configured system service.
func StartSystemService() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "start", appName).Run()
	case ServiceOpenRC:
		cmd := exec.Command("rc-service", appName, "start")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceSysVinit:
		return exec.Command("service", appName, "start").Run()
	case ServiceRunit:
		return exec.Command("sv", "start", appName).Run()
	case ServiceS6:
		return exec.Command("s6-svc", "-u", s6ServicePath()).Run()
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "load", plistPath).Run()
	case ServiceWindows:
		return exec.Command("sc.exe", "start", appName).Run()
	case ServiceBSDRC:
		return exec.Command("service", appName, "start").Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// StopSystemService stops the configured system service.
func StopSystemService() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "stop", appName).Run()
	case ServiceOpenRC:
		return exec.Command("rc-service", appName, "stop").Run()
	case ServiceSysVinit:
		return exec.Command("service", appName, "stop").Run()
	case ServiceRunit:
		return exec.Command("sv", "stop", appName).Run()
	case ServiceS6:
		return exec.Command("s6-svc", "-d", s6ServicePath()).Run()
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "unload", plistPath).Run()
	case ServiceWindows:
		return exec.Command("sc.exe", "stop", appName).Run()
	case ServiceBSDRC:
		return exec.Command("service", appName, "stop").Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// RestartSystemService restarts the configured system service.
func RestartSystemService() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "restart", appName).Run()
	case ServiceOpenRC:
		return exec.Command("rc-service", appName, "restart").Run()
	case ServiceSysVinit:
		return exec.Command("service", appName, "restart").Run()
	case ServiceRunit:
		return exec.Command("sv", "restart", appName).Run()
	case ServiceS6:
		return exec.Command("s6-svc", "-r", s6ServicePath()).Run()
	case ServiceLaunchd:
		if err := StopSystemService(); err != nil {
			return err
		}
		return StartSystemService()
	case ServiceWindows:
		if err := exec.Command("sc.exe", "stop", appName).Run(); err != nil {
			return err
		}
		return exec.Command("sc.exe", "start", appName).Run()
	case ServiceBSDRC:
		return exec.Command("service", appName, "restart").Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// ReloadSystemService sends a reload signal to the configured system service.
func ReloadSystemService() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "reload", appName).Run()
	case ServiceOpenRC:
		return exec.Command("rc-service", appName, "reload").Run()
	case ServiceRunit:
		return exec.Command("sv", "hup", appName).Run()
	default:
		// For others, restart is the fallback
		return RestartSystemService()
	}
}

// PrintSystemServiceStatus prints the configured system service status.
func PrintSystemServiceStatus() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		cmd := exec.Command("systemctl", "status", appName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // systemctl status exits non-zero when stopped
		return nil
	case ServiceOpenRC:
		cmd := exec.Command("rc-service", appName, "status")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceSysVinit:
		cmd := exec.Command("service", appName, "status")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceRunit:
		cmd := exec.Command("sv", "status", appName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceS6:
		cmd := exec.Command("s6-svstat", s6ServicePath())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceLaunchd:
		cmd := exec.Command("launchctl", "list", fmt.Sprintf("io.github.%s.%s", orgName, appName))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceWindows:
		cmd := exec.Command("sc.exe", "query", appName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceBSDRC:
		cmd := exec.Command("service", appName, "status")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Enable marks the service for boot-time auto-start
func EnableSystemService() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "enable", appName).Run()
	case ServiceOpenRC:
		return exec.Command("rc-update", "add", appName, "default").Run()
	case ServiceSysVinit:
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			return exec.Command("update-rc.d", appName, "defaults").Run()
		}
		if _, err := exec.LookPath("chkconfig"); err == nil {
			return exec.Command("chkconfig", appName, "on").Run()
		}
		return fmt.Errorf("no SysVinit registration tool found")
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "load", "-w", plistPath).Run()
	case ServiceWindows:
		return exec.Command("sc.exe", "config", appName, "start=", "auto").Run()
	case ServiceBSDRC:
		return fmt.Errorf("add '%s_enable=\"YES\"' to /etc/rc.conf to enable at boot", appName)
	case ServiceS6:
		return enableS6()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Disable removes the service from boot-time auto-start. This only stops
// and disables the service — the service file, data, and system user/group
// are left intact (unlike Uninstall).
func DisableSystemService() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		_ = exec.Command("systemctl", "stop", appName).Run()
		return exec.Command("systemctl", "disable", appName).Run()
	case ServiceOpenRC:
		_ = exec.Command("rc-service", appName, "stop").Run()
		return exec.Command("rc-update", "del", appName, "default").Run()
	case ServiceSysVinit:
		_ = exec.Command("service", appName, "stop").Run()
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			return exec.Command("update-rc.d", "-f", appName, "remove").Run()
		}
		if _, err := exec.LookPath("chkconfig"); err == nil {
			return exec.Command("chkconfig", appName, "off").Run()
		}
		return fmt.Errorf("no SysVinit registration tool found")
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "unload", "-w", plistPath).Run()
	case ServiceWindows:
		_ = exec.Command("sc.exe", "stop", appName).Run()
		return exec.Command("sc.exe", "config", appName, "start=", "demand").Run()
	case ServiceBSDRC:
		return fmt.Errorf("remove '%s_enable=\"YES\"' from /etc/rc.conf to disable at boot", appName)
	case ServiceS6:
		return disableS6()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Logs tails recent logs from the service manager
func PrintRecentSystemServiceLogs() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		cmd := exec.Command("journalctl", "-u", appName, "-n", "50", "--no-pager")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceOpenRC, ServiceSysVinit:
		logPath := fmt.Sprintf("/var/log/%s/%s/output.log", orgName, appName)
		cmd := exec.Command("tail", "-n", "50", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceLaunchd:
		logPath := fmt.Sprintf("/Library/Logs/%s/%s/stdout.log", orgName, appName)
		cmd := exec.Command("tail", "-n", "50", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceWindows:
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-EventLog -LogName Application -Source %s -Newest 50 | Format-Table -AutoSize", appName))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceBSDRC:
		logPath := fmt.Sprintf("/var/log/%s/%s/server.log", orgName, appName)
		cmd := exec.Command("tail", "-n", "50", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case ServiceS6:
		_, _, logDir := paths.GetDefaultDirs(appName)
		if logDir == "" {
			logDir = filepath.Join("/var/log", orgName, appName)
		}
		cmd := exec.Command("tail", "-n", "50", filepath.Join(logDir, "current"))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}
