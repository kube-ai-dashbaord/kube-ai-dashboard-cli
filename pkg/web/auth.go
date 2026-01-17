package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin, user, viewer
	Email        string    `json:"email,omitempty"`
	DisplayName  string    `json:"display_name,omitempty"`
	Source       string    `json:"source"` // local, ldap
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login,omitempty"`
}

// Session represents an authenticated session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Source    string    `json:"source"` // local, ldap
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AuthManager handles authentication
type AuthManager struct {
	users        map[string]*User    // username -> User
	sessions     map[string]*Session // session ID -> Session
	mu           sync.RWMutex
	config       *AuthConfig
	ldapProvider *LDAPProvider
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	SessionDuration time.Duration `yaml:"session_duration" json:"session_duration"`
	DefaultAdmin    string        `yaml:"default_admin" json:"default_admin"`
	DefaultPassword string        `yaml:"default_password" json:"-"`
	LDAP            *LDAPConfig   `yaml:"ldap" json:"ldap"`
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(cfg *AuthConfig) *AuthManager {
	am := &AuthManager{
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
		config:   cfg,
	}

	// Initialize LDAP provider if configured
	if cfg.LDAP != nil && cfg.LDAP.Enabled {
		am.ldapProvider = NewLDAPProvider(cfg.LDAP)
	}

	// Create default admin user if enabled (for local auth fallback)
	if cfg.Enabled {
		adminUser := cfg.DefaultAdmin
		if adminUser == "" {
			adminUser = "admin"
		}
		adminPass := cfg.DefaultPassword
		if adminPass == "" {
			adminPass = "admin123" // Default password - should be changed
		}
		am.createLocalUser(adminUser, adminPass, "admin")
	}

	return am
}

// createLocalUser creates a local user (internal use)
func (am *AuthManager) createLocalUser(username, password, role string) error {
	if _, exists := am.users[username]; exists {
		return fmt.Errorf("user already exists: %s", username)
	}

	am.users[username] = &User{
		ID:           generateSessionID()[:16],
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         role,
		Source:       "local",
		CreatedAt:    time.Now(),
	}

	return nil
}

// hashPassword creates a SHA256 hash of the password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// generateSessionID creates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// CreateUser creates a new local user
func (am *AuthManager) CreateUser(username, password, role string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.users[username]; exists {
		return fmt.Errorf("user already exists: %s", username)
	}

	am.users[username] = &User{
		ID:           generateSessionID()[:16],
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         role,
		Source:       "local",
		CreatedAt:    time.Now(),
	}

	return nil
}

// Authenticate validates credentials and creates a session
func (am *AuthManager) Authenticate(username, password string) (*Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Try LDAP authentication first if enabled
	if am.ldapProvider != nil && am.ldapProvider.IsEnabled() {
		ldapUser, err := am.ldapProvider.Authenticate(username, password)
		if err == nil {
			// LDAP auth successful - create or update local user cache
			user, exists := am.users[username]
			if !exists {
				user = &User{
					ID:          generateSessionID()[:16],
					Username:    ldapUser.Username,
					Email:       ldapUser.Email,
					DisplayName: ldapUser.DisplayName,
					Role:        ldapUser.Role,
					Source:      "ldap",
					CreatedAt:   time.Now(),
				}
				am.users[username] = user
			} else {
				// Update user info from LDAP
				user.Email = ldapUser.Email
				user.DisplayName = ldapUser.DisplayName
				user.Role = ldapUser.Role
				user.Source = "ldap"
			}
			user.LastLogin = time.Now()

			// Create session
			session := &Session{
				ID:        generateSessionID(),
				UserID:    user.ID,
				Username:  user.Username,
				Role:      user.Role,
				Source:    "ldap",
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(am.config.SessionDuration),
			}
			am.sessions[session.ID] = session
			return session, nil
		}
		// LDAP auth failed, fall through to local auth
	}

	// Try local authentication
	user, exists := am.users[username]
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Only allow local auth for local users
	if user.Source != "local" && user.Source != "" {
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.PasswordHash != hashPassword(password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	user.LastLogin = time.Now()

	// Create session
	session := &Session{
		ID:        generateSessionID(),
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		Source:    "local",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.config.SessionDuration),
	}

	am.sessions[session.ID] = session

	return session, nil
}

// AuthenticateLDAP authenticates specifically against LDAP
func (am *AuthManager) AuthenticateLDAP(username, password string) (*Session, error) {
	if am.ldapProvider == nil || !am.ldapProvider.IsEnabled() {
		return nil, fmt.Errorf("LDAP authentication is not enabled")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	ldapUser, err := am.ldapProvider.Authenticate(username, password)
	if err != nil {
		return nil, err
	}

	// Create or update local user cache
	user, exists := am.users[username]
	if !exists {
		user = &User{
			ID:          generateSessionID()[:16],
			Username:    ldapUser.Username,
			Email:       ldapUser.Email,
			DisplayName: ldapUser.DisplayName,
			Role:        ldapUser.Role,
			Source:      "ldap",
			CreatedAt:   time.Now(),
		}
		am.users[username] = user
	} else {
		user.Email = ldapUser.Email
		user.DisplayName = ldapUser.DisplayName
		user.Role = ldapUser.Role
		user.Source = "ldap"
	}
	user.LastLogin = time.Now()

	session := &Session{
		ID:        generateSessionID(),
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		Source:    "ldap",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.config.SessionDuration),
	}
	am.sessions[session.ID] = session

	return session, nil
}

// IsLDAPEnabled returns whether LDAP is enabled
func (am *AuthManager) IsLDAPEnabled() bool {
	return am.ldapProvider != nil && am.ldapProvider.IsEnabled()
}

// TestLDAPConnection tests the LDAP connection
func (am *AuthManager) TestLDAPConnection() error {
	if am.ldapProvider == nil {
		return fmt.Errorf("LDAP is not configured")
	}
	return am.ldapProvider.TestConnection()
}

// GetLDAPConfig returns the LDAP configuration (without sensitive data)
func (am *AuthManager) GetLDAPConfig() *LDAPConfig {
	if am.ldapProvider == nil {
		return nil
	}
	return am.ldapProvider.GetConfig()
}

// ValidateSession checks if a session is valid
func (am *AuthManager) ValidateSession(sessionID string) (*Session, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	session, exists := am.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	if time.Now().After(session.ExpiresAt) {
		delete(am.sessions, sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// InvalidateSession removes a session
func (am *AuthManager) InvalidateSession(sessionID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.sessions, sessionID)
}

// GetUsers returns all users (without passwords)
func (am *AuthManager) GetUsers() []*User {
	am.mu.RLock()
	defer am.mu.RUnlock()

	users := make([]*User, 0, len(am.users))
	for _, u := range am.users {
		users = append(users, u)
	}
	return users
}

// ChangePassword updates a user's password
func (am *AuthManager) ChangePassword(username, oldPassword, newPassword string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[username]
	if !exists {
		return fmt.Errorf("user not found")
	}

	if user.PasswordHash != hashPassword(oldPassword) {
		return fmt.Errorf("invalid current password")
	}

	user.PasswordHash = hashPassword(newPassword)
	return nil
}

// DeleteUser removes a user
func (am *AuthManager) DeleteUser(username string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.users[username]; !exists {
		return fmt.Errorf("user not found")
	}

	delete(am.users, username)
	return nil
}

// AuthMiddleware wraps handlers to require authentication
func (am *AuthManager) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !am.config.Enabled {
			next(w, r)
			return
		}

		// Check for session cookie or Authorization header
		sessionID := ""

		// Try cookie first
		if cookie, err := r.Cookie("k13s_session"); err == nil {
			sessionID = cookie.Value
		}

		// Try Authorization header
		if sessionID == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				sessionID = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if sessionID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		session, err := am.ValidateSession(sessionID)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Add session info to request context
		r.Header.Set("X-User-ID", session.UserID)
		r.Header.Set("X-Username", session.Username)
		r.Header.Set("X-User-Role", session.Role)

		next(w, r)
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HandleLogin handles login requests
func (am *AuthManager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := am.Authenticate(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "k13s_session",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Expires:  session.ExpiresAt,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:     session.ID,
		Username:  session.Username,
		Role:      session.Role,
		ExpiresAt: session.ExpiresAt,
	})
}

// HandleLogout handles logout requests
func (am *AuthManager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session from cookie or header
	sessionID := ""
	if cookie, err := r.Cookie("k13s_session"); err == nil {
		sessionID = cookie.Value
	}

	if sessionID != "" {
		am.InvalidateSession(sessionID)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "k13s_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// HandleCurrentUser returns the current user info
func (am *AuthManager) HandleCurrentUser(w http.ResponseWriter, r *http.Request) {
	if !am.config.Enabled {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"username":     "anonymous",
			"role":         "admin",
			"auth_enabled": false,
			"ldap_enabled": false,
		})
		return
	}

	username := r.Header.Get("X-Username")
	role := r.Header.Get("X-User-Role")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"username":     username,
		"role":         role,
		"auth_enabled": true,
		"ldap_enabled": am.IsLDAPEnabled(),
	})
}

// HandleLDAPStatus returns LDAP configuration status
func (am *AuthManager) HandleLDAPStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !am.IsLDAPEnabled() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
		})
		return
	}

	ldapConfig := am.GetLDAPConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":      true,
		"host":         ldapConfig.Host,
		"port":         ldapConfig.Port,
		"use_tls":      ldapConfig.UseTLS,
		"base_dn":      ldapConfig.BaseDN,
		"admin_groups": ldapConfig.AdminGroups,
		"user_groups":  ldapConfig.UserGroups,
	})
}

// HandleLDAPTest tests the LDAP connection
func (am *AuthManager) HandleLDAPTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := am.TestLDAPConnection(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}
