package database

import (
	"fmt"
	"os"
	"path/filepath"
)

// SaveCredentialsToFile saves admin credentials to a file
// This is shown ONCE on first run for the user to save
func SaveCredentialsToFile(creds *AdminCredentials, configDir string) error {
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

	content := fmt.Sprintf(`AIRPORTS API - ADMIN CREDENTIALS
========================================
SAVE THESE CREDENTIALS SECURELY!
This file is generated once and will not be shown again.

WEB UI LOGIN:
  URL:      http://localhost:8080/config
  Username: %s
  Password: (see below)

API TOKEN:
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
- Change password at /config after first login
========================================
`, creds.Username, creds.Token, creds.Username,
   "(check environment or use token)", creds.Token, creds.CreatedAt.Format("2006-01-02 15:04:05"))

	// Write with restricted permissions
	if err := os.WriteFile(credFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}
