package web

import (
	"context"
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

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	users          map[string]*User    // username -> User
	sessions       map[string]*Session // session ID -> Session
	tokenSessions  map[string]*Session // K8s token -> Session (cached)
	mu             sync.RWMutex
	config         *AuthConfig
	ldapProvider   *LDAPProvider
	tokenValidator *K8sTokenValidator
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	SessionDuration time.Duration `yaml:"session_duration" json:"session_duration"`
	DefaultAdmin    string        `yaml:"default_admin" json:"default_admin"`
	DefaultPassword string        `yaml:"default_password" json:"-"`
	LDAP            *LDAPConfig   `yaml:"ldap" json:"ldap"`
	// AuthMode: "token" (K8s RBAC token - default), "local" (username/password), "ldap"
	AuthMode string `yaml:"auth_mode" json:"auth_mode"`
}

// K8sTokenValidator validates Kubernetes service account tokens
type K8sTokenValidator struct {
	clientset *kubernetes.Clientset
}

// NewK8sTokenValidator creates a new K8s token validator
func NewK8sTokenValidator() (*K8sTokenValidator, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sTokenValidator{clientset: clientset}, nil
}

// ValidateToken validates a Kubernetes token and returns user info
func (v *K8sTokenValidator) ValidateToken(ctx context.Context, token string) (*TokenReview, error) {
	review := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := v.clientset.AuthenticationV1().TokenReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("token review failed: %w", err)
	}

	if !result.Status.Authenticated {
		return nil, fmt.Errorf("token not authenticated")
	}

	return &TokenReview{
		Authenticated: result.Status.Authenticated,
		Username:      result.Status.User.Username,
		UID:           result.Status.User.UID,
		Groups:        result.Status.User.Groups,
	}, nil
}

// TokenReview represents the result of a token validation
type TokenReview struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username"`
	UID           string   `json:"uid"`
	Groups        []string `json:"groups"`
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(cfg *AuthConfig) *AuthManager {
	am := &AuthManager{
		users:         make(map[string]*User),
		sessions:      make(map[string]*Session),
		tokenSessions: make(map[string]*Session),
		config:        cfg,
	}

	// Set default auth mode to "token" if not specified
	if cfg.AuthMode == "" {
		cfg.AuthMode = "token"
	}

	// Initialize K8s token validator for token auth mode
	if cfg.AuthMode == "token" {
		validator, err := NewK8sTokenValidator()
		if err != nil {
			// Token validation may fail outside cluster, that's OK for dev mode
			fmt.Printf("  K8s token validator: Not available (running outside cluster)\n")
		} else {
			am.tokenValidator = validator
			fmt.Printf("  K8s token validator: Ready\n")
		}
	}

	// Initialize LDAP provider if configured
	if cfg.LDAP != nil && cfg.LDAP.Enabled {
		am.ldapProvider = NewLDAPProvider(cfg.LDAP)
	}

	// Create default admin user only for local auth mode
	if cfg.Enabled && cfg.AuthMode == "local" {
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

// GetAuthMode returns the current authentication mode
func (am *AuthManager) GetAuthMode() string {
	return am.config.AuthMode
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
		token := ""

		// Try cookie first
		if cookie, err := r.Cookie("k13s_session"); err == nil {
			sessionID = cookie.Value
		}

		// Try Authorization header
		if sessionID == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		// For token auth mode, try K8s token validation first
		if am.config.AuthMode == "token" && token != "" {
			session, err := am.ValidateK8sToken(r.Context(), token)
			if err == nil {
				r.Header.Set("X-User-ID", session.UserID)
				r.Header.Set("X-Username", session.Username)
				r.Header.Set("X-User-Role", session.Role)
				next(w, r)
				return
			}
			// Fall through to session validation
			sessionID = token
		}

		if sessionID == "" && token != "" {
			sessionID = token
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

// ValidateK8sToken validates a Kubernetes service account token
func (am *AuthManager) ValidateK8sToken(ctx context.Context, token string) (*Session, error) {
	// Check cache first
	am.mu.RLock()
	if session, exists := am.tokenSessions[token]; exists {
		if time.Now().Before(session.ExpiresAt) {
			am.mu.RUnlock()
			return session, nil
		}
	}
	am.mu.RUnlock()

	// Validate with K8s API
	if am.tokenValidator == nil {
		return nil, fmt.Errorf("K8s token validator not available")
	}

	review, err := am.tokenValidator.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Determine role from groups
	role := "viewer"
	for _, group := range review.Groups {
		if strings.Contains(group, "admin") || strings.Contains(group, "cluster-admin") {
			role = "admin"
			break
		}
		if strings.Contains(group, "edit") || strings.Contains(group, "developer") {
			role = "user"
		}
	}

	// Create cached session
	session := &Session{
		ID:        generateSessionID(),
		UserID:    review.UID,
		Username:  review.Username,
		Role:      role,
		Source:    "k8s-token",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.config.SessionDuration),
	}

	// Cache the session
	am.mu.Lock()
	am.tokenSessions[token] = session
	am.mu.Unlock()

	return session, nil
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"` // K8s service account token
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	AuthMode  string    `json:"auth_mode"`
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

	var session *Session
	var err error

	// Handle token-based login (K8s RBAC)
	if req.Token != "" {
		session, err = am.ValidateK8sToken(r.Context(), req.Token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid K8s token: " + err.Error(),
			})
			return
		}
	} else {
		// Handle username/password login (local or LDAP)
		session, err = am.Authenticate(req.Username, req.Password)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
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
		AuthMode:  session.Source,
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
			"auth_mode":    "none",
		})
		return
	}

	username := r.Header.Get("X-Username")
	role := r.Header.Get("X-User-Role")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"username":        username,
		"role":            role,
		"auth_enabled":    true,
		"ldap_enabled":    am.IsLDAPEnabled(),
		"auth_mode":       am.config.AuthMode,
		"token_available": am.tokenValidator != nil,
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

// AdminMiddleware ensures the user has admin role
func (am *AuthManager) AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-User-Role")
		if role != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// UserRequest represents a user creation/update request
type UserRequest struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role"`
	Email    string `json:"email,omitempty"`
}

// HandleListUsers returns list of all users (admin only)
func (am *AuthManager) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users := am.GetUsers()

	// Sanitize user data (remove sensitive fields)
	safeUsers := make([]map[string]interface{}, len(users))
	for i, u := range users {
		safeUsers[i] = map[string]interface{}{
			"id":           u.ID,
			"username":     u.Username,
			"role":         u.Role,
			"email":        u.Email,
			"display_name": u.DisplayName,
			"source":       u.Source,
			"created_at":   u.CreatedAt,
			"last_login":   u.LastLogin,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": safeUsers,
		"total": len(safeUsers),
	})
}

// HandleCreateUser creates a new user (admin only)
func (am *AuthManager) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" && req.Role != "viewer" {
		http.Error(w, "Invalid role. Must be admin, user, or viewer", http.StatusBadRequest)
		return
	}

	if err := am.CreateUser(req.Username, req.Password, req.Role); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// Update email if provided
	if req.Email != "" {
		am.mu.Lock()
		if user, exists := am.users[req.Username]; exists {
			user.Email = req.Email
		}
		am.mu.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "created",
		"username": req.Username,
	})
}

// HandleUpdateUser updates an existing user (admin only)
func (am *AuthManager) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get username from URL path
	username := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[username]
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Only allow updating local users
	if user.Source != "local" && user.Source != "" {
		http.Error(w, "Cannot update non-local user", http.StatusBadRequest)
		return
	}

	// Update fields
	if req.Role != "" {
		if req.Role != "admin" && req.Role != "user" && req.Role != "viewer" {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		user.Role = req.Role
	}

	if req.Email != "" {
		user.Email = req.Email
	}

	if req.Password != "" {
		user.PasswordHash = hashPassword(req.Password)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "updated",
		"username": username,
	})
}

// HandleDeleteUser deletes a user (admin only)
func (am *AuthManager) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get username from URL path
	username := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Prevent deleting the current user
	currentUser := r.Header.Get("X-Username")
	if username == currentUser {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := am.DeleteUser(username); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "deleted",
		"username": username,
	})
}

// HandleResetPassword resets a user's password (admin only)
func (am *AuthManager) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.NewPassword == "" {
		http.Error(w, "Username and new password are required", http.StatusBadRequest)
		return
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[req.Username]
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.Source != "local" && user.Source != "" {
		http.Error(w, "Cannot reset password for non-local user", http.StatusBadRequest)
		return
	}

	user.PasswordHash = hashPassword(req.NewPassword)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "password_reset",
		"username": req.Username,
	})
}

// HandleAuthStatus returns authentication system status
func (am *AuthManager) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auth_enabled":    am.config.Enabled,
		"auth_mode":       am.config.AuthMode,
		"ldap_enabled":    am.IsLDAPEnabled(),
		"token_available": am.tokenValidator != nil,
		"session_duration": am.config.SessionDuration.String(),
		"total_users":     len(am.users),
		"active_sessions": len(am.sessions),
	})
}
