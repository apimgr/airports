package database

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// AdminCredentials holds admin authentication credentials
type AdminCredentials struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	Token        string    `json:"token"`
	TokenHash    string    `json:"-"` // Never expose in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// InitializeAdminAuth ensures admin credentials exist
func InitializeAdminAuth(envUser, envPassword, envToken string) (*AdminCredentials, error) {
	// Check if admin credentials already exist
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM settings WHERE key = 'admin.username')").Scan(&exists)
	if err != nil {
		return nil, err
	}

	if exists {
		// Load existing credentials
		return loadAdminCredentials()
	}

	// Create new credentials
	username := envUser
	if username == "" {
		username = "administrator"
	}

	password := envPassword
	if password == "" {
		password = generateRandomPassword(16)
	}

	token := envToken
	if token == "" {
		token = generateRandomToken(32)
	}

	// Hash password and token
	passwordHash := hashPassword(password)
	tokenHash := hashToken(token)

	// Store in database
	now := time.Now()
	if err := SetSetting("admin.username", username, "string", "admin", "Admin username"); err != nil {
		return nil, err
	}
	if err := SetSetting("admin.password_hash", passwordHash, "string", "admin", "Admin password hash"); err != nil {
		return nil, err
	}
	if err := SetSetting("admin.token_hash", tokenHash, "string", "admin", "Admin API token hash"); err != nil {
		return nil, err
	}
	if err := SetSetting("admin.created_at", now.Format(time.RFC3339), "string", "admin", "Admin created at"); err != nil {
		return nil, err
	}

	return &AdminCredentials{
		Username:     username,
		PasswordHash: passwordHash,
		Token:        token, // Return plaintext token ONCE
		TokenHash:    tokenHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// loadAdminCredentials loads existing admin credentials from database
func loadAdminCredentials() (*AdminCredentials, error) {
	username := GetSettingValue("admin.username", "administrator")
	passwordHash := GetSettingValue("admin.password_hash", "")
	tokenHash := GetSettingValue("admin.token_hash", "")
	createdAtStr := GetSettingValue("admin.created_at", time.Now().Format(time.RFC3339))

	createdAt, _ := time.Parse(time.RFC3339, createdAtStr)

	return &AdminCredentials{
		Username:     username,
		PasswordHash: passwordHash,
		TokenHash:    tokenHash,
		Token:        "", // Never return stored token
		CreatedAt:    createdAt,
		UpdatedAt:    time.Now(),
	}, nil
}

// ValidatePassword checks if password matches stored hash
func ValidatePassword(password string) bool {
	storedHash := GetSettingValue("admin.password_hash", "")
	return hashPassword(password) == storedHash
}

// ValidateToken checks if token matches stored hash
func ValidateToken(token string) bool {
	storedHash := GetSettingValue("admin.token_hash", "")
	return hashToken(token) == storedHash
}

// UpdateAdminPassword updates the admin password
func UpdateAdminPassword(newPassword string) error {
	passwordHash := hashPassword(newPassword)
	return SetSetting("admin.password_hash", passwordHash, "string", "admin", "Admin password hash")
}

// RegenerateAdminToken generates a new admin token
func RegenerateAdminToken() (string, error) {
	token := generateRandomToken(32)
	tokenHash := hashToken(token)
	err := SetSetting("admin.token_hash", tokenHash, "string", "admin", "Admin API token hash")
	if err != nil {
		return "", err
	}
	return token, nil
}

// hashPassword creates a SHA-256 hash of the password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// hashToken creates a SHA-256 hash of the token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// generateRandomToken generates a cryptographically secure random token
func generateRandomToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
