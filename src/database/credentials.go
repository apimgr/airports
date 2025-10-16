package database

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// SaveCredentialsToFile saves admin credentials to a file
// This is shown ONCE on first run for the user to save
func SaveCredentialsToFile(creds *AdminCredentials, configDir, port string) error {
	if configDir == "" {
		configDir = "./config"
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	credFile := filepath.Join(configDir, "admin_credentials")

	// Only write if token is present (first time initialization)
	if creds.Token == "" {
		return nil // Already saved, token is empty
	}

	// Get accessible URL
	serverURL := getAccessibleURL(port)

	content := fmt.Sprintf(`AIRPORTS API - ADMIN CREDENTIALS
========================================
SAVE THESE CREDENTIALS SECURELY!
This file is generated once and will not be shown again.

WEB UI LOGIN:
  URL:      %s/admin
  Username: %s
  Password: (see below)

API ACCESS:
  URL:      %s/api/v1/admin
  Header:   Authorization: Bearer %s

CREDENTIALS:
  Username: %s
  Password: %s (if auto-generated)
  Token:    %s

Created: %s

SECURITY NOTES:
- Keep this file secure (permissions: 0600)
- The password and token are hashed in the database
- To reset, delete the database and restart
- Change password at /admin/settings after first login
========================================
`, serverURL, creds.Username, serverURL, creds.Token, creds.Username,
   "(check environment or use token)", creds.Token, creds.CreatedAt.Format("2006-01-02 15:04:05"))

	// Write with restricted permissions
	if err := os.WriteFile(credFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// getAccessibleURL returns the most relevant URL for accessing the server
// Priority: FQDN > hostname > public IP > fallback
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
