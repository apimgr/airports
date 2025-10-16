package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/database"
	"github.com/apimgr/airports/src/geoip"
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
)

func main() {
	// Command line flags
	portFlag := flag.String("port", "", "HTTP port")
	showVersion := flag.Bool("version", false, "Show version and exit")
	showStatus := flag.Bool("status", false, "Show server status and exit")
	devMode := flag.Bool("dev", false, "Run in development mode")
	showHelp := flag.Bool("help", false, "Show help message")

	flag.Parse()

	// Port priority: Flag > ENV > Default (8080)
	port := *portFlag
	if port == "" {
		port = getEnv("PORT", "8080")
	}

	// Handle flags
	if *showHelp {
		printHelp()
		return
	}

	if *showVersion {
		fmt.Println(Version)
		return
	}

	if *showStatus {
		// TODO: Implement status check
		fmt.Println("✅ Server: Not running")
		os.Exit(1)
	}

	// Start server
	if err := run(port, *devMode); err != nil {
		log.Fatal(err)
	}
}

func run(port string, devMode bool) error {
	log.Printf("Starting airports API server v%s", Version)
	log.Printf("Commit: %s, Built: %s", Commit, BuildDate)

	// Get OS-specific default directories
	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs("airports")

	// Allow overrides via environment variables
	configDir := getEnv("CONFIG_DIR", defaultConfigDir)
	dataDir := getEnv("DATA_DIR", defaultDataDir)
	logsDir := getEnv("LOGS_DIR", defaultLogsDir)

	// Ensure directories exist
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	log.Printf("Config directory: %s", configDir)
	log.Printf("Data directory: %s", dataDir)
	log.Printf("Logs directory: %s", logsDir)

	// Initialize database
	log.Println("Initializing database...")

	var dbConfig database.Config
	var err error

	// Check for connection string first
	connStr := getEnv("DATABASE_URL", "")
	if connStr != "" {
		dbConfig, err = database.ParseConnectionString(connStr)
		if err != nil {
			return fmt.Errorf("failed to parse DATABASE_URL: %w", err)
		}
		log.Printf("Using database connection string (type: %s)", dbConfig.Type)
	} else {
		// Use individual environment variables with data directory default
		dbType := getEnv("DB_TYPE", "sqlite")
		dbPath := getEnv("DB_PATH", fmt.Sprintf("%s/db/airports.db", dataDir))

		// Ensure db directory exists
		dbDir := fmt.Sprintf("%s/db", dataDir)
		if err := paths.EnsureDir(dbDir); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}

		dbConfig = database.Config{
			Type: dbType,
			Path: dbPath,
		}
	}

	if err := database.Initialize(dbConfig); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()
	log.Println("Database initialized successfully")

	// Initialize admin authentication
	log.Println("Initializing admin authentication...")
	adminUser := getEnv("ADMIN_USER", "")
	adminPass := getEnv("ADMIN_PASSWORD", "")
	adminToken := getEnv("ADMIN_TOKEN", "")

	creds, err := database.InitializeAdminAuth(adminUser, adminPass, adminToken)
	if err != nil {
		return fmt.Errorf("failed to initialize admin auth: %w", err)
	}
	log.Println("Admin authentication initialized")

	// Port resolution priority: Flag > DB > ENV (first run only) > Random
	if port == "" {
		// No flag provided, check database
		if storedPort := database.GetSettingValue("server.http_port", ""); storedPort != "" {
			port = storedPort
			log.Printf("Using port from database: %s", port)
		} else {
			// No port in DB, check ENV (first run only)
			envPort := getEnv("PORT", "")
			if envPort != "" {
				port = envPort
				log.Printf("Using port from environment: %s", port)
			} else {
				// No ENV, find random unused port
				port, err = findRandomPort()
				if err != nil {
					return fmt.Errorf("failed to find available port: %w", err)
				}
				log.Printf("Selected random available port: %s", port)
			}
			// Save to database for persistence
			if err := database.SetSetting("server.http_port", port, "number", "server", "HTTP server port"); err != nil {
				log.Printf("Warning: Failed to save port to database: %v", err)
			} else {
				log.Printf("Port %s saved to database for future use", port)
			}
		}
	} else {
		log.Printf("Using port from command line flag: %s", port)
	}

	// Save credentials to file if this is first initialization (after port is determined)
	if creds.Token != "" {
		if err := database.SaveCredentialsToFile(creds, configDir, port); err != nil {
			log.Printf("Warning: Failed to save credentials file: %v", err)
		} else {
			log.Printf("⚠️  ADMIN CREDENTIALS SAVED TO: %s/admin_credentials", configDir)
			log.Printf("⚠️  Username: %s", creds.Username)
			log.Printf("⚠️  API Token: %s", creds.Token)
			log.Printf("⚠️  Access URL: %s", getAccessibleURL(port))
			log.Printf("⚠️  Save these credentials securely! They will not be shown again.")
		}
	}

	// Load airport data
	log.Println("Loading airport database...")
	airportSvc, err := airports.NewService(airportsData)
	if err != nil {
		return fmt.Errorf("failed to load airports: %w", err)
	}
	stats := airportSvc.Stats()
	log.Printf("Loaded %d airports from %d countries", stats["total_airports"], stats["countries"])

	// Load GeoIP data
	log.Println("Loading GeoIP databases...")
	geoipSvc, err := geoip.NewService(configDir)
	if err != nil {
		return fmt.Errorf("failed to load GeoIP: %w", err)
	}
	defer geoipSvc.Close()
	log.Println("GeoIP databases loaded successfully")

	// Initialize scheduler
	sched := scheduler.New()

	// Add GeoIP weekly update task (Sunday at 3:00 AM)
	sched.AddTask("geoip-update", "0 3 * * 0", func() error {
		return geoipSvc.UpdateDatabases()
	})

	// Start scheduler
	sched.Start()
	defer sched.Stop()
	log.Println("Scheduler started")

	// Create HTTP server
	srv := server.New(airportSvc, geoipSvc, devMode)
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", getAccessibleURL(port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

func printHelp() {
	fmt.Println("airports - Airport location information API server")
	fmt.Println()
	fmt.Println("Usage: airports [OPTIONS]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help            Show this help message")
	fmt.Println("  --version         Show version information")
	fmt.Println("  --status          Show server status and exit with code")
	fmt.Println("  --port PORT       Set port (default: 8080)")
	fmt.Println("  --address ADDR    Listen address (default: 0.0.0.0)")
	fmt.Println("  --config DIR      Config directory (OS-specific default)")
	fmt.Println("  --data DIR        Data directory (OS-specific default)")
	fmt.Println("  --logs DIR        Logs directory (OS-specific default)")
	fmt.Println("  --dev             Run in development mode")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CONFIG_DIR        Config directory path")
	fmt.Println("  DATA_DIR          Data directory path")
	fmt.Println("  LOGS_DIR          Logs directory path")
	fmt.Println("  PORT              Server port")
	fmt.Println("  ADDRESS           Listen address")
	fmt.Println()
	fmt.Println("  DATABASE_URL      Database connection string")
	fmt.Println("                    Examples:")
	fmt.Println("                      sqlite:/data/db/airports.db")
	fmt.Println("                      mysql://user:pass@<host>:3306/dbname")
	fmt.Println("                      postgres://user:pass@<host>:5432/dbname")
	fmt.Println("  DB_TYPE           Database type (sqlite, mysql, postgres)")
	fmt.Println("  DB_PATH           SQLite database file path (default: {DATA_DIR}/db/airports.db)")
	fmt.Println()
	fmt.Println("  ADMIN_USER        Admin username (default: administrator, first run only)")
	fmt.Println("  ADMIN_PASSWORD    Admin password (default: random, first run only)")
	fmt.Println("  ADMIN_TOKEN       Admin API token (default: random, first run only)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  airports                          # Start with OS defaults")
	fmt.Println("  airports --port 8080              # Start on port 8080")
	fmt.Println("  airports --data /var/lib/airports # Use custom data directory")
	fmt.Println("  airports --dev                    # Start in development mode")
	fmt.Println()
	fmt.Println("Admin Panel:")
	fmt.Println("  Web UI:  http://<your-host>:<port>/admin")
	fmt.Println("  API:     http://<your-host>:<port>/api/v1/admin")
	fmt.Println()
	fmt.Println("Actual URL will be shown when server starts.")
	fmt.Println("Credentials are saved to {CONFIG_DIR}/admin_credentials on first run.")
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// findRandomPort finds an available port in the 64000-64999 range
func findRandomPort() (string, error) {
	rand.Seed(time.Now().UnixNano())

	// Try up to 100 times to find an available port
	for i := 0; i < 100; i++ {
		port := 64000 + rand.Intn(1000) // 64000-64999

		// Try to listen on the port
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			// Port is available, close and return it
			ln.Close()
			return fmt.Sprintf("%d", port), nil
		}
	}

	return "", fmt.Errorf("could not find available port in range 64000-64999 after 100 attempts")
}

// getAccessibleURL returns the most relevant URL for accessing the server
// Priority: FQDN > hostname > public IP > private IP
func getAccessibleURL(port string) string {
	// Try to get hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		// Try to resolve hostname to see if it's a valid FQDN
		if addrs, err := net.LookupHost(hostname); err == nil && len(addrs) > 0 {
			return fmt.Sprintf("http://%s:%s", hostname, port)
		}
	}

	// Try to get outbound IP (most likely accessible IP)
	if ip := getOutboundIP(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}

	// Fallback to hostname if we have one
	if hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("http://%s:%s", hostname, port)
	}

	// Last resort: use a generic message
	return fmt.Sprintf("http://<your-host>:%s", port)
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
