package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB is the database connection
var DB *sql.DB

// Config holds database configuration
type Config struct {
	Type     string // sqlite, mysql, postgres
	Path     string // for SQLite
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

// Initialize sets up the database connection
func Initialize(config Config) error {
	var err error

	switch config.Type {
	case "sqlite", "":
		// Default to SQLite
		if config.Path == "" {
			// Use ./data/airports.db as default
			config.Path = "./data/airports.db"
		}

		// Ensure directory exists
		dir := filepath.Dir(config.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}

		DB, err = sql.Open("sqlite", config.Path)
		if err != nil {
			return fmt.Errorf("failed to open SQLite database: %w", err)
		}

		// Enable WAL mode for better concurrency
		if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}

	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	// Test connection
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Initialize schema
	if _, err := DB.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// ParseConnectionString parses a database connection string
// Formats supported:
//   sqlite:./data/db.sqlite
//   sqlite:/path/to/db.sqlite
//   mysql://user:pass@localhost:3306/dbname
//   postgres://user:pass@localhost:5432/dbname?sslmode=disable
func ParseConnectionString(connStr string) (Config, error) {
	config := Config{}

	// Handle sqlite special case (may not have //)
	if strings.HasPrefix(connStr, "sqlite:") {
		config.Type = "sqlite"
		config.Path = strings.TrimPrefix(connStr, "sqlite:")
		if config.Path == "" {
			config.Path = "./data/airports.db"
		}
		return config, nil
	}

	// Parse as URL
	u, err := url.Parse(connStr)
	if err != nil {
		return config, fmt.Errorf("invalid connection string: %w", err)
	}

	config.Type = u.Scheme

	switch u.Scheme {
	case "sqlite":
		config.Path = u.Path
		if config.Path == "" {
			config.Path = "./data/airports.db"
		}

	case "mysql", "postgres", "postgresql":
		config.Host = u.Hostname()
		if portStr := u.Port(); portStr != "" {
			config.Port, _ = strconv.Atoi(portStr)
		} else {
			// Default ports
			if u.Scheme == "mysql" {
				config.Port = 3306
			} else {
				config.Port = 5432
			}
		}

		config.Name = strings.TrimPrefix(u.Path, "/")
		if u.User != nil {
			config.User = u.User.Username()
			config.Password, _ = u.User.Password()
		}

	default:
		return config, fmt.Errorf("unsupported database type: %s", u.Scheme)
	}

	return config, nil
}

// Ping checks if the database connection is alive
func Ping() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Ping()
}

// GetType returns the current database type
func GetType() string {
	// This is a simple implementation - in production you'd want to store this
	// For now, detect from the driver
	if DB == nil {
		return "unknown"
	}
	return "sqlite" // Default assumption
}
