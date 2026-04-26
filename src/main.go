package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/paths"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/apimgr/airports/src/server"
)

//go:embed data/airports.json
var airportsData []byte

var (
	// Injected at build time via ldflags
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"

	// Project info
	ProjectName = "airports"
	ProjectOrg  = "apimgr"
)

func main() {
	// Command line flags
	portFlag := flag.String("port", "", "HTTP port")
	addressFlag := flag.String("address", "", "Listen address")
	configDirFlag := flag.String("config", "", "Configuration directory")
	dataDirFlag := flag.String("data", "", "Data directory")
	modeFlag := flag.String("mode", "", "Application mode: production, development")
	showVersion := flag.Bool("version", false, "Show version and exit")
	showStatus := flag.Bool("status", false, "Show server status and exit")
	showHelp := flag.Bool("help", false, "Show help message")

	// Service commands
	serviceCmd := flag.String("service", "", "Service commands: start, stop, restart, reload, status, --install, --uninstall, --disable, --help")

	// Maintenance commands
	maintenanceCmd := flag.String("maintenance", "", "Maintenance commands: backup, restore, update, mode, setup")

	// Update command
	updateCmd := flag.String("update", "", "Update commands: check, yes, branch")

	flag.Parse()

	// Handle help (can run without privileges)
	if *showHelp {
		printHelp()
		return
	}

	// Handle version (can run without privileges)
	if *showVersion {
		fmt.Printf("%s\n", Version)
		return
	}

	// Handle status (can run without privileges)
	if *showStatus {
		exitCode := checkStatus()
		os.Exit(exitCode)
	}

	// Handle mode flag (sets mode and exits)
	if *modeFlag != "" {
		if err := setApplicationMode(*modeFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle update command
	if *updateCmd != "" {
		// Get optional branch from remaining args
		var branch string
		if flag.NArg() > 0 {
			branch = flag.Arg(0)
		}
		if err := handleUpdateCommand(*updateCmd, branch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle service commands
	if *serviceCmd != "" {
		if err := handleServiceCommand(*serviceCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle maintenance commands
	if *maintenanceCmd != "" {
		// Get optional file location from remaining args
		var fileLocation string
		if flag.NArg() > 0 {
			fileLocation = flag.Arg(0)
		}
		if err := handleMaintenanceCommand(*maintenanceCmd, fileLocation); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start server
	if err := run(*portFlag, *addressFlag, *configDirFlag, *dataDirFlag); err != nil {
		log.Fatal(err)
	}
}

func run(portFlag, addressFlag, configDirFlag, dataDirFlag string) error {
	log.Printf("Starting %s API server v%s", ProjectName, Version)
	log.Printf("Commit: %s, Built: %s", Commit, BuildDate)

	// Get OS-specific default directories
	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs(ProjectName)

	// Allow overrides via flags or environment variables
	configDir := firstNonEmpty(configDirFlag, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	dataDir := firstNonEmpty(dataDirFlag, os.Getenv("DATA_DIR"), defaultDataDir)
	logsDir := firstNonEmpty(os.Getenv("LOGS_DIR"), defaultLogsDir)

	// Ensure directories exist
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	log.Printf("Config directory: %s", configDir)
	log.Printf("Data directory: %s", dataDir)
	log.Printf("Logs directory: %s", logsDir)

	// Load configuration from YAML file (.yml per BASE.md)
	configPath := filepath.Join(configDir, "server.yml")
	log.Printf("Loading configuration from: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	log.Println("Configuration loaded successfully")

	// Determine port: Flag > Config > ENV > Random
	port := firstNonEmpty(portFlag, cfg.Server.Port, os.Getenv("PORT"))
	if port == "" {
		port, err = findRandomPort()
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		log.Printf("Selected random available port: %s", port)
		// Save to config for persistence
		if err := config.SetPort(port); err != nil {
			log.Printf("Warning: Failed to save port to config: %v", err)
		}
	}

	// Determine listen address
	address := firstNonEmpty(addressFlag, cfg.Server.Address, os.Getenv("ADDRESS"), "0.0.0.0")

	// Check if running in container
	if paths.IsRunningInContainer() {
		log.Println("Running in container environment")
	}

	// Load airport data
	log.Println("Loading airport database...")
	airportSvc, err := airports.NewService(airportsData)
	if err != nil {
		return fmt.Errorf("failed to load airports: %w", err)
	}
	stats := airportSvc.Stats()
	log.Printf("Loaded %d airports from %d countries", stats["total_airports"], stats["countries"])

	// Load GeoIP data (optional - continue without if fails)
	log.Println("Loading GeoIP databases...")
	geoipSvc, err := geoip.NewService(configDir)
	if err != nil {
		log.Printf("Warning: GeoIP initialization failed: %v", err)
		log.Println("GeoIP features will be unavailable")
		geoipSvc = nil
	} else {
		defer geoipSvc.Close()
		log.Println("GeoIP databases loaded successfully")
	}

	// Initialize scheduler
	sched := scheduler.New()

	// Add GeoIP update task based on config (only if GeoIP loaded successfully)
	if geoipSvc != nil && cfg.Server.Schedule.Enabled {
		schedule := "0 3 * * 0" // Default: Sunday at 3:00 AM
		switch cfg.Server.Schedule.GeoIPUpdate {
		case "daily":
			schedule = "0 3 * * *"
		case "weekly":
			schedule = "0 3 * * 0"
		}
		sched.AddTask("geoip-update", schedule, func() error {
			return geoipSvc.UpdateDatabases()
		})
	}

	// Start scheduler
	sched.Start()
	defer sched.Stop()
	log.Println("Scheduler started")

	// Create HTTP server
	srv := server.New(airportSvc, geoipSvc, cfg)
	httpServer := &http.Server{
		Addr:         address + ":" + port,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		url := getAccessibleURL(port)
		log.Printf("Server listening on %s", url)
		fmt.Printf("\n  %s Airports API Server v%s\n", getEmoji("plane"), Version)
		fmt.Printf("  %s %s\n\n", getEmoji("link"), url)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

func printHelp() {
	fmt.Printf("%s - Airport location information API server\n", ProjectName)
	fmt.Println()
	fmt.Printf("Usage: %s [OPTIONS]\n", ProjectName)
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help                Show this help message")
	fmt.Println("  --version             Show version information")
	fmt.Println("  --status              Show server status and exit with code")
	fmt.Println("  --mode MODE           Set application mode (production, development)")
	fmt.Println("  --port PORT           Set port (default: random 64xxx)")
	fmt.Println("  --address ADDR        Listen address (default: 0.0.0.0)")
	fmt.Println("  --config DIR          Configuration directory")
	fmt.Println("  --data DIR            Data directory")
	fmt.Println()
	fmt.Println("Update Commands:")
	fmt.Println("  --update check        Check for updates")
	fmt.Println("  --update yes          Install available updates")
	fmt.Println("  --update branch NAME  Set update branch (stable, beta, daily)")
	fmt.Println()
	fmt.Println("Service Commands:")
	fmt.Println("  --service start       Start the service")
	fmt.Println("  --service stop        Stop the service")
	fmt.Println("  --service restart     Restart the service")
	fmt.Println("  --service reload      Reload configuration")
	fmt.Println("  --service status      Show service status")
	fmt.Println("  --service --install   Install as system service")
	fmt.Println("  --service --uninstall Remove system service")
	fmt.Println("  --service --disable   Disable system service")
	fmt.Println("  --service --help      Show service help")
	fmt.Println()
	fmt.Println("Maintenance Commands:")
	fmt.Println("  --maintenance backup [file]   Backup config and data")
	fmt.Println("  --maintenance restore [file]  Restore from backup")
	fmt.Println("  --maintenance update          Check and install updates")
	fmt.Println("  --maintenance mode [MODE]     Show or set application mode")
	fmt.Println("  --maintenance setup           Run initial setup wizard")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CONFIG_DIR            Configuration directory path")
	fmt.Println("  DATA_DIR              Data directory path")
	fmt.Println("  LOGS_DIR              Logs directory path")
	fmt.Println("  PORT                  Server port")
	fmt.Println("  ADDRESS               Listen address")
	fmt.Println("  MODE                  Application mode (production, development)")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Root: /etc/%s/%s/server.yml\n", ProjectOrg, ProjectName)
	fmt.Printf("  User: ~/.config/%s/%s/server.yml\n", ProjectOrg, ProjectName)
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s                          # Start with defaults\n", ProjectName)
	fmt.Printf("  %s --port 8080              # Start on port 8080\n", ProjectName)
	fmt.Printf("  %s --maintenance backup     # Create backup\n", ProjectName)
	fmt.Printf("  %s --service --install      # Install as service\n", ProjectName)
}

func checkStatus() int {
	// Try to connect to health endpoint
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yaml")

	cfg, err := config.Load(configPath)
	if err != nil || cfg.Server.Port == "" {
		fmt.Println("Status: Not configured")
		return 1
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", cfg.Server.Port))
	if err != nil {
		fmt.Printf("Status: Not running (port %s)\n", cfg.Server.Port)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var health map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&health)
		fmt.Printf("Status: Running on port %s\n", cfg.Server.Port)
		if data, ok := health["data"].(map[string]interface{}); ok {
			if status, ok := data["status"].(string); ok {
				fmt.Printf("Health: %s\n", status)
			}
		}
		return 0
	}

	fmt.Printf("Status: Unhealthy (HTTP %d)\n", resp.StatusCode)
	return 1
}

func handleServiceCommand(cmd string) error {
	switch cmd {
	case "start":
		return serviceControl("start")
	case "stop":
		return serviceControl("stop")
	case "restart":
		return serviceControl("restart")
	case "reload":
		return serviceControl("reload")
	case "status":
		return serviceControl("status")
	case "--install":
		return installService()
	case "--uninstall":
		return uninstallService()
	case "--disable":
		return disableService()
	case "--help":
		printServiceHelp()
		return nil
	default:
		return fmt.Errorf("unknown service command: %s", cmd)
	}
}

func serviceControl(action string) error {
	switch runtime.GOOS {
	case "linux":
		// Try systemctl first, then runit
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.Command("systemctl", action, ProjectName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		if _, err := exec.LookPath("sv"); err == nil {
			cmd := exec.Command("sv", action, ProjectName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("no supported service manager found (systemd or runit)")
	case "darwin":
		cmd := exec.Command("launchctl", action, fmt.Sprintf("com.%s.%s", ProjectOrg, ProjectName))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "windows":
		var scAction string
		switch action {
		case "start":
			scAction = "start"
		case "stop":
			scAction = "stop"
		case "restart":
			// Windows doesn't have restart, do stop then start
			exec.Command("sc", "stop", ProjectName).Run()
			time.Sleep(2 * time.Second)
			scAction = "start"
		case "status":
			scAction = "query"
		default:
			return fmt.Errorf("unsupported action for Windows: %s", action)
		}
		cmd := exec.Command("sc", scAction, ProjectName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func installService() error {
	fmt.Printf("Installing %s service...\n", ProjectName)
	// This would be handled by the install scripts
	fmt.Println("Please use the installation scripts in the scripts/ directory")
	fmt.Printf("  Linux:   scripts/linux.sh\n")
	fmt.Printf("  macOS:   scripts/macos.sh\n")
	fmt.Printf("  Windows: scripts/windows.ps1\n")
	return nil
}

func uninstallService() error {
	fmt.Printf("Uninstalling %s service...\n", ProjectName)
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			exec.Command("systemctl", "stop", ProjectName).Run()
			exec.Command("systemctl", "disable", ProjectName).Run()
			os.Remove(fmt.Sprintf("/etc/systemd/system/%s.service", ProjectName))
			exec.Command("systemctl", "daemon-reload").Run()
			fmt.Println("Service uninstalled")
			return nil
		}
	}
	return fmt.Errorf("manual uninstallation required for this platform")
}

func disableService() error {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			return exec.Command("systemctl", "disable", ProjectName).Run()
		}
	}
	return fmt.Errorf("unsupported on this platform")
}

func printServiceHelp() {
	fmt.Println("Service Commands:")
	fmt.Println()
	fmt.Println("  start       Start the service")
	fmt.Println("  stop        Stop the service")
	fmt.Println("  restart     Restart the service")
	fmt.Println("  reload      Reload configuration")
	fmt.Println("  status      Show service status")
	fmt.Println("  --install   Install as system service")
	fmt.Println("  --uninstall Remove system service")
	fmt.Println("  --disable   Disable system service")
	fmt.Println()
	fmt.Println("Supported service managers:")
	fmt.Println("  Linux:   systemd, runit")
	fmt.Println("  macOS:   launchd")
	fmt.Println("  Windows: Windows Service Manager")
	fmt.Println("  BSD:     rc.d")
}

func handleMaintenanceCommand(cmd, fileLocation string) error {
	switch cmd {
	case "backup":
		return createBackup(fileLocation)
	case "restore":
		return restoreBackup(fileLocation)
	case "update":
		return checkAndUpdate()
	case "mode":
		// fileLocation is actually the mode value (production/development)
		if fileLocation == "" {
			return showCurrentMode()
		}
		return setApplicationMode(fileLocation)
	case "setup":
		return runInitialSetup()
	default:
		return fmt.Errorf("unknown maintenance command: %s", cmd)
	}
}

func createBackup(fileLocation string) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)

	// Default backup location
	if fileLocation == "" {
		backupDir := paths.GetBackupDir(ProjectName)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
		timestamp := time.Now().Format("20060102150405")
		fileLocation = filepath.Join(backupDir, fmt.Sprintf("%s.tar.gz", timestamp))
	}

	fmt.Printf("Creating backup: %s\n", fileLocation)

	// Create tar.gz archive
	file, err := os.Create(fileLocation)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add config directory
	if err := addDirToTar(tarWriter, configDir, "config"); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}

	// Add data directory
	if err := addDirToTar(tarWriter, dataDir, "data"); err != nil {
		return fmt.Errorf("failed to backup data: %w", err)
	}

	fmt.Printf("Backup created successfully: %s\n", fileLocation)
	return nil
}

func addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.Join(prefix, relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			return err
		}
		return nil
	})
}

func restoreBackup(fileLocation string) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)

	// If no file specified, find most recent backup
	if fileLocation == "" {
		backupDir := paths.GetBackupDir(ProjectName)
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("no backups found in %s", backupDir)
		}

		var latest string
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tar.gz") {
				if latest == "" || entry.Name() > latest {
					latest = entry.Name()
				}
			}
		}
		if latest == "" {
			return fmt.Errorf("no backup files found")
		}
		fileLocation = filepath.Join(backupDir, latest)
	}

	fmt.Printf("Restoring from: %s\n", fileLocation)

	file, err := os.Open(fileLocation)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		var destDir string
		if strings.HasPrefix(header.Name, "config/") {
			destDir = configDir
			header.Name = strings.TrimPrefix(header.Name, "config/")
		} else if strings.HasPrefix(header.Name, "data/") {
			destDir = dataDir
			header.Name = strings.TrimPrefix(header.Name, "data/")
		} else {
			continue
		}

		destPath := filepath.Join(destDir, header.Name)

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(destPath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(destPath), 0755)
			outFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			io.Copy(outFile, tarReader)
			outFile.Close()
		}
	}

	fmt.Println("Restore completed successfully")
	return nil
}

func checkAndUpdate() error {
	fmt.Println("Checking for updates...")

	// Get latest release from GitHub
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ProjectOrg, ProjectName))
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == Version {
		fmt.Printf("Already running latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, Version)

	// Find correct asset for this platform
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("Downloading %s...\n", assetName)

	// Download to temp file
	tmpFile, err := os.CreateTemp("", ProjectName+"-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer dlResp.Body.Close()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		return fmt.Errorf("failed to save update: %w", err)
	}
	tmpFile.Close()

	// Get current binary path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Replace binary
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), currentPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Updated to version %s\n", latestVersion)
	fmt.Println("Please restart the service to apply the update")
	return nil
}

// Helper functions

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func findRandomPort() (string, error) {
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 100; i++ {
		port := 64000 + rand.Intn(1000)
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return fmt.Sprintf("%d", port), nil
		}
	}

	return "", fmt.Errorf("could not find available port in range 64000-64999")
}

func getAccessibleURL(port string) string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		if addrs, err := net.LookupHost(hostname); err == nil && len(addrs) > 0 {
			return fmt.Sprintf("http://%s:%s", hostname, port)
		}
	}

	if ip := getOutboundIP(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}

	if hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("http://%s:%s", hostname, port)
	}

	return fmt.Sprintf("http://<your-host>:%s", port)
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func getEmoji(name string) string {
	emojis := map[string]string{
		"plane":    "✈️",
		"link":     "🔗",
		"check":    "✓",
		"cross":    "✗",
		"warning":  "⚠️",
		"info":     "ℹ️",
		"success":  "✅",
		"error":    "❌",
		"folder":   "📁",
		"file":     "📄",
		"gear":     "⚙️",
		"rocket":   "🚀",
	}
	if e, ok := emojis[name]; ok {
		return e
	}
	return ""
}

// setApplicationMode sets the application mode (production/development)
func setApplicationMode(modeStr string) error {
	// Validate the mode
	parsedMode, err := mode.ParseMode(modeStr)
	if err != nil {
		return err
	}

	// Get config path
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	// Load existing config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Update mode
	cfg.Server.Mode = string(parsedMode)

	// Save config
	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Also set in runtime
	if err := mode.Set(modeStr); err != nil {
		return err
	}

	fmt.Printf("Application mode set to: %s\n", parsedMode)
	fmt.Println("Restart the service for the change to take full effect")
	return nil
}

// showCurrentMode displays the current application mode
func showCurrentMode() error {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	currentMode := cfg.Server.Mode
	if currentMode == "" {
		currentMode = "production"
	}

	fmt.Printf("Current mode: %s\n", currentMode)
	return nil
}

// runInitialSetup runs the initial setup wizard
func runInitialSetup() error {
	fmt.Printf("Running %s initial setup...\n\n", ProjectName)

	// Get config path
	configDir, dataDir, logsDir := paths.GetDefaultDirs(ProjectName)

	// Create directories
	fmt.Println("Creating directories...")
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	fmt.Printf("  Config: %s\n", configDir)
	fmt.Printf("  Data:   %s\n", dataDir)
	fmt.Printf("  Logs:   %s\n", logsDir)

	// Create default config
	configPath := filepath.Join(configDir, "server.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("\nCreating default configuration...")
		cfg, err := config.Load(configPath) // This creates default if not exists
		if err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
		fmt.Printf("  Config file: %s\n", configPath)
		fmt.Printf("  Default port: %s\n", cfg.Server.Port)
	} else {
		fmt.Printf("\nConfiguration already exists: %s\n", configPath)
	}

	fmt.Println("\nSetup complete!")
	fmt.Printf("Start the server: %s\n", ProjectName)
	fmt.Printf("Or install as service: %s --service --install\n", ProjectName)
	return nil
}

// handleUpdateCommand handles the --update command with subcommands
func handleUpdateCommand(cmd, branch string) error {
	switch cmd {
	case "check":
		return checkForUpdate()
	case "yes":
		return checkAndUpdate()
	case "branch":
		if branch == "" {
			return fmt.Errorf("branch name required: stable, beta, or daily")
		}
		return setUpdateBranch(branch)
	default:
		return fmt.Errorf("unknown update command: %s (expected: check, yes, branch)", cmd)
	}
}

// checkForUpdate checks for updates without installing
func checkForUpdate() error {
	fmt.Println("Checking for updates...")

	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ProjectOrg, ProjectName))
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Body        string `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == Version {
		fmt.Printf("You are running the latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, Version)
	fmt.Printf("Published: %s\n", release.PublishedAt)
	if release.Body != "" {
		fmt.Printf("\nRelease notes:\n%s\n", release.Body)
	}
	fmt.Printf("\nRun '%s --update yes' to install the update\n", ProjectName)
	return nil
}

// setUpdateBranch sets the update branch preference
func setUpdateBranch(branch string) error {
	validBranches := map[string]bool{"stable": true, "beta": true, "daily": true}
	if !validBranches[branch] {
		return fmt.Errorf("invalid branch: %s (expected: stable, beta, or daily)", branch)
	}

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Server.UpdateBranch = branch

	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Update branch set to: %s\n", branch)
	return nil
}
