package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appName = "airports"
	orgName = "apimgr"
)

// ServiceType represents the type of service manager
type ServiceType int

const (
	ServiceUnknown ServiceType = iota
	ServiceSystemd
	ServiceRunit
	ServiceLaunchd
	ServiceWindows
	ServiceBSDRC
)

// DetectServiceManager detects the system's service manager
func DetectServiceManager() ServiceType {
	switch runtime.GOOS {
	case "linux":
		// Check for systemd
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return ServiceSystemd
		}
		// Check for runit
		if _, err := os.Stat("/run/runit"); err == nil {
			return ServiceRunit
		}
		// Fallback to systemd if /etc/systemd exists
		if _, err := os.Stat("/etc/systemd"); err == nil {
			return ServiceSystemd
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

// Install installs the service for the detected service manager
func Install() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return installSystemd()
	case ServiceRunit:
		return installRunit()
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

// Uninstall removes the service
func Uninstall() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return uninstallSystemd()
	case ServiceRunit:
		return uninstallRunit()
	case ServiceLaunchd:
		return uninstallLaunchd()
	case ServiceWindows:
		return uninstallWindows()
	case ServiceBSDRC:
		return uninstallBSDRC()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// GetBinaryPath returns the path where the binary should be installed
func GetBinaryPath() string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf(`C:\Program Files\%s\%s\%s.exe`, orgName, appName, appName)
	default:
		return fmt.Sprintf("/usr/local/bin/%s", appName)
	}
}

// installSystemd creates systemd service file
func installSystemd() error {
	binaryPath := GetBinaryPath()

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
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
ReadWritePaths=/var/lib/%s/%s /var/log/%s/%s /etc/%s/%s

[Install]
WantedBy=multi-user.target
`, binaryPath, orgName, appName, orgName, appName, orgName, appName)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", appName)

	// Create directories
	dirs := []string{
		fmt.Sprintf("/var/lib/%s/%s", orgName, appName),
		fmt.Sprintf("/var/log/%s/%s", orgName, appName),
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

	fmt.Printf("✅ Service installed at: %s\n", servicePath)
	fmt.Printf("✅ Binary installed at: %s\n", binaryPath)
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
	exec.Command("systemctl", "stop", appName).Run()

	// Disable service
	exec.Command("systemctl", "disable", appName).Run()

	// Remove service file
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Printf("✅ Service uninstalled: %s\n", servicePath)
	return nil
}

// installRunit creates runit service
func installRunit() error {
	svDir := fmt.Sprintf("/etc/sv/%s", appName)
	binaryPath := GetBinaryPath()

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

	logRunScript := `#!/bin/sh
exec svlogd -tt ./main
`
	logRunPath := filepath.Join(logDir, "run")
	if err := os.WriteFile(logRunPath, []byte(logRunScript), 0755); err != nil {
		return fmt.Errorf("failed to write log run script: %w", err)
	}

	// Link to service directory
	linkPath := fmt.Sprintf("/var/service/%s", appName)
	os.Symlink(svDir, linkPath)

	fmt.Printf("✅ Runit service installed at: %s\n", svDir)
	return nil
}

// uninstallRunit removes runit service
func uninstallRunit() error {
	svDir := fmt.Sprintf("/etc/sv/%s", appName)
	linkPath := fmt.Sprintf("/var/service/%s", appName)

	// Stop service
	exec.Command("sv", "stop", appName).Run()

	// Remove link
	os.Remove(linkPath)

	// Remove service directory
	os.RemoveAll(svDir)

	fmt.Printf("✅ Runit service uninstalled\n")
	return nil
}

// installLaunchd creates macOS launchd plist
func installLaunchd() error {
	binaryPath := GetBinaryPath()
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

	fmt.Printf("✅ LaunchDaemon installed at: %s\n", plistPath)
	fmt.Println()
	fmt.Println("To load the service:")
	fmt.Printf("  sudo launchctl load %s\n", plistPath)

	return nil
}

// uninstallLaunchd removes macOS launchd plist
func uninstallLaunchd() error {
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)

	// Unload if running
	exec.Command("launchctl", "unload", plistPath).Run()

	// Remove plist
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	fmt.Printf("✅ LaunchDaemon uninstalled\n")
	return nil
}

// installWindows creates Windows service
func installWindows() error {
	binaryPath := GetBinaryPath()

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

	// Create service using sc.exe
	displayName := strings.Title(appName) + " API"
	cmd := exec.Command("sc.exe", "create", appName,
		"binPath=", binaryPath,
		"DisplayName=", displayName,
		"start=", "auto")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}

	fmt.Printf("✅ Windows service '%s' installed\n", appName)
	fmt.Println()
	fmt.Println("To start the service:")
	fmt.Printf("  sc.exe start %s\n", appName)

	return nil
}

// uninstallWindows removes Windows service
func uninstallWindows() error {
	// Stop service
	exec.Command("sc.exe", "stop", appName).Run()

	// Delete service
	if err := exec.Command("sc.exe", "delete", appName).Run(); err != nil {
		return fmt.Errorf("failed to delete Windows service: %w", err)
	}

	fmt.Printf("✅ Windows service '%s' uninstalled\n", appName)
	return nil
}

// installBSDRC creates BSD rc.d script
func installBSDRC() error {
	binaryPath := GetBinaryPath()
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

	fmt.Printf("✅ BSD rc.d script installed at: %s\n", rcPath)
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
	exec.Command("service", appName, "stop").Run()

	// Remove script
	if err := os.Remove(rcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove rc.d script: %w", err)
	}

	fmt.Printf("✅ BSD rc.d script uninstalled\n")
	return nil
}

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

// Start starts the service
func Start() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "start", appName).Run()
	case ServiceRunit:
		return exec.Command("sv", "start", appName).Run()
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

// Stop stops the service
func Stop() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "stop", appName).Run()
	case ServiceRunit:
		return exec.Command("sv", "stop", appName).Run()
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

// Restart restarts the service
func Restart() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "restart", appName).Run()
	case ServiceRunit:
		return exec.Command("sv", "restart", appName).Run()
	case ServiceLaunchd:
		Stop()
		return Start()
	case ServiceWindows:
		exec.Command("sc.exe", "stop", appName).Run()
		return exec.Command("sc.exe", "start", appName).Run()
	case ServiceBSDRC:
		return exec.Command("service", appName, "restart").Run()
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Reload sends reload signal to the service
func Reload() error {
	serviceType := DetectServiceManager()

	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "reload", appName).Run()
	case ServiceRunit:
		return exec.Command("sv", "hup", appName).Run()
	default:
		// For others, restart is the fallback
		return Restart()
	}
}

// Status prints the service status
func Status() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		cmd := exec.Command("systemctl", "status", appName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // systemctl status exits non-zero when stopped
		return nil
	case ServiceRunit:
		cmd := exec.Command("sv", "status", appName)
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
func Enable() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "enable", appName).Run()
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "load", "-w", plistPath).Run()
	case ServiceWindows:
		return exec.Command("sc.exe", "config", appName, "start=", "auto").Run()
	case ServiceBSDRC:
		return fmt.Errorf("add '%s_enable=\"YES\"' to /etc/rc.conf to enable at boot", appName)
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Disable removes the service from boot-time auto-start
func Disable() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		return exec.Command("systemctl", "disable", appName).Run()
	case ServiceLaunchd:
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/io.github.%s.%s.plist", orgName, appName)
		return exec.Command("launchctl", "unload", "-w", plistPath).Run()
	case ServiceWindows:
		return exec.Command("sc.exe", "config", appName, "start=", "demand").Run()
	case ServiceBSDRC:
		return fmt.Errorf("remove '%s_enable=\"YES\"' from /etc/rc.conf to disable at boot", appName)
	default:
		return fmt.Errorf("unsupported service manager")
	}
}

// Logs tails recent logs from the service manager
func Logs() error {
	serviceType := DetectServiceManager()
	switch serviceType {
	case ServiceSystemd:
		cmd := exec.Command("journalctl", "-u", appName, "-n", "50", "--no-pager")
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
	default:
		return fmt.Errorf("unsupported service manager")
	}
}
