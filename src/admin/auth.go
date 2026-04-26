package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// AuthManager handles admin authentication
type AuthManager struct {
	sessions     map[string]*Session
	tokens       map[string]*APIToken
	mu           sync.RWMutex
	adminUser    string
	adminPassHash string
	apiToken     string
	sessionTimeout int
	sslEnabled   bool
}

// Session represents an authenticated admin session
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
}

// APIToken represents a bearer token for API access
type APIToken struct {
	Token       string
	Name        string
	Permissions []string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastUsed    time.Time
}

// NewAuthManager creates a new auth manager
func NewAuthManager(adminUser, adminPass, apiToken string, sessionTimeout int, sslEnabled bool) *AuthManager {
	am := &AuthManager{
		sessions:       make(map[string]*Session),
		tokens:         make(map[string]*APIToken),
		adminUser:      adminUser,
		adminPassHash:  adminPass,
		apiToken:       apiToken,
		sessionTimeout: sessionTimeout,
		sslEnabled:     sslEnabled,
	}

	// Start cleanup goroutine
	go am.cleanupLoop()

	return am
}

// Authenticate validates username and password
func (am *AuthManager) Authenticate(username, password string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.adminUser == "" {
		return false
	}

	// Constant-time comparison for username
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(am.adminUser),
	) == 1

	if !usernameMatch {
		return false
	}

	// Check if stored password is an Argon2 hash
	if strings.HasPrefix(am.adminPassHash, "$argon2") {
		return verifyArgon2Hash(password, am.adminPassHash)
	}

	// Plain text comparison for initial setup
	return subtle.ConstantTimeCompare([]byte(password), []byte(am.adminPassHash)) == 1
}

// CreateSession creates a new admin session
func (am *AuthManager) CreateSession(username, ip string) *Session {
	am.mu.Lock()
	defer am.mu.Unlock()

	sessionID := generateSecureToken(32)
	session := &Session{
		ID:        sessionID,
		Username:  username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(am.sessionTimeout) * time.Second),
		IP:        ip,
	}

	am.sessions[sessionID] = session
	return session
}

// GetSession retrieves a session by ID
func (am *AuthManager) GetSession(sessionID string) (*Session, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	session, ok := am.sessions[sessionID]
	if !ok {
		return nil, false
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// DeleteSession removes a session
func (am *AuthManager) DeleteSession(sessionID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.sessions, sessionID)
}

// RefreshSession extends a session's expiration
func (am *AuthManager) RefreshSession(sessionID string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	session, ok := am.sessions[sessionID]
	if !ok {
		return false
	}

	session.ExpiresAt = time.Now().Add(time.Duration(am.sessionTimeout) * time.Second)
	return true
}

// ValidateAPIToken validates a bearer token
func (am *AuthManager) ValidateAPIToken(token string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Check static token from config
	if am.apiToken != "" {
		if subtle.ConstantTimeCompare([]byte(token), []byte(am.apiToken)) == 1 {
			return true
		}
	}

	// Check dynamic tokens
	apiToken, ok := am.tokens[token]
	if !ok {
		return false
	}

	if time.Now().After(apiToken.ExpiresAt) {
		return false
	}

	apiToken.LastUsed = time.Now()
	return true
}

// SetSessionCookie sets the admin session cookie
func (am *AuthManager) SetSessionCookie(w http.ResponseWriter, session *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    session.ID,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   am.sslEnabled,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   am.sessionTimeout,
	})
}

// ClearSessionCookie removes the admin session cookie
func (am *AuthManager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// GetSessionFromRequest extracts session from request cookie
func (am *AuthManager) GetSessionFromRequest(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return nil, false
	}
	return am.GetSession(cookie.Value)
}

// GetTokenFromRequest extracts bearer token from Authorization header
func GetTokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

// cleanupLoop periodically removes expired sessions
func (am *AuthManager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		am.cleanup()
	}
}

func (am *AuthManager) cleanup() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for id, session := range am.sessions {
		if now.After(session.ExpiresAt) {
			delete(am.sessions, id)
		}
	}

	for id, token := range am.tokens {
		if now.After(token.ExpiresAt) {
			delete(am.tokens, id)
		}
	}
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))[:length]
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// HashPassword creates an Argon2id hash of the password
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)

	// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return "$argon2id$v=19$m=65536,t=3,p=4$" + b64Salt + "$" + b64Hash, nil
}

// verifyArgon2Hash verifies a password against an Argon2id hash
func verifyArgon2Hash(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1
}
