package rest

import (
	"asdf/internal/auth"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AuthHandler struct {
	authService *auth.AuthService
	userStore   auth.UserStore
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string     `json:"token"`
	User      *auth.User `json:"user"`
	ExpiresAt time.Time  `json:"expires_at"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *auth.AuthService, userStore auth.UserStore) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userStore:   userStore,
	}
}

// HandleLogin handles user login
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		h.writeError(w, http.StatusBadRequest, "Username and password required", "")
		return
	}

	// Find user by username or email
	var user *auth.User
	var err error

	if strings.Contains(req.Username, "@") {
		user, err = h.userStore.GetUserByEmail(r.Context(), req.Username)
	} else {
		user, err = h.userStore.GetUserByUsername(r.Context(), req.Username)
	}

	if err != nil || user == nil {
		h.writeError(w, http.StatusUnauthorized, "Invalid credentials", "")
		return
	}

	// Check if user is active
	if !user.IsActive {
		h.writeError(w, http.StatusForbidden, "Account is disabled", "")
		return
	}

	// Verify password
	if !h.authService.VerifyPassword(req.Password, user.PasswordHash) {
		h.writeError(w, http.StatusUnauthorized, "Invalid credentials", "")
		return
	}

	// Update last login
	_ = h.userStore.UpdateLastLogin(r.Context(), user.ID)

	// Generate JWT token
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	token, err := h.authService.GenerateToken(r.Context(), user, ipAddress, userAgent)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(24 * time.Hour) // Should match config

	response := LoginResponse{
		Token:     token,
		User:      user,
		ExpiresAt: expiresAt,
	}

	// Remove password hash from response
	response.User.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRegister handles user registration
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		h.writeError(w, http.StatusBadRequest, "Username, email, and password required", "")
		return
	}

	// Basic validation
	if len(req.Password) < 8 {
		h.writeError(w, http.StatusBadRequest, "Password must be at least 8 characters", "")
		return
	}

	if !strings.Contains(req.Email, "@") {
		h.writeError(w, http.StatusBadRequest, "Invalid email format", "")
		return
	}

	// Check if username already exists
	existingUser, _ := h.userStore.GetUserByUsername(r.Context(), req.Username)
	if existingUser != nil {
		h.writeError(w, http.StatusConflict, "Username already exists", "")
		return
	}

	// Check if email already exists
	existingUser, _ = h.userStore.GetUserByEmail(r.Context(), req.Email)
	if existingUser != nil {
		h.writeError(w, http.StatusConflict, "Email already exists", "")
		return
	}

	// Hash password
	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Create user
	createReq := &auth.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: passwordHash,
	}

	user, err := h.userStore.CreateUser(r.Context(), createReq)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create user", err.Error())
		return
	}

	// Remove password hash from response
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User created successfully",
		"user":    user,
	})
}

// HandleLogout handles user logout
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from Authorization header
	token := h.extractToken(r)
	if token == "" {
		h.writeError(w, http.StatusBadRequest, "No token provided", "")
		return
	}

	// Revoke the token
	if err := h.authService.RevokeToken(r.Context(), token); err != nil {
		// Don't fail logout if token revocation fails
		// The token will expire naturally
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// HandleRefreshToken handles token refresh
func (h *AuthHandler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from Authorization header
	oldToken := h.extractToken(r)
	if oldToken == "" {
		h.writeError(w, http.StatusBadRequest, "No token provided", "")
		return
	}

	// Refresh the token
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	newToken, err := h.authService.RefreshToken(r.Context(), oldToken, ipAddress, userAgent)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "Invalid or expired token", err.Error())
		return
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(24 * time.Hour)

	response := map[string]interface{}{
		"token":      newToken,
		"expires_at": expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleProfile returns the current user's profile
func (h *AuthHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := r.Context().Value("user").(*auth.Claims)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "Not authenticated", "")
		return
	}

	// Get full user details
	fullUser, err := h.userStore.GetUserByID(r.Context(), user.UserID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get user details", err.Error())
		return
	}

	// Remove password hash
	fullUser.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fullUser)
}

// AuthMiddleware validates JWT tokens and adds user to context
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := h.extractToken(r)
		if token == "" {
			h.writeError(w, http.StatusUnauthorized, "No token provided", "")
			return
		}

		claims, err := h.authService.ValidateToken(r.Context(), token)
		if err != nil {
			h.writeError(w, http.StatusUnauthorized, "Invalid token", err.Error())
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), "user", claims)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// RequireAdmin middleware ensures user is an admin
func (h *AuthHandler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*auth.Claims)
		if !ok {
			h.writeError(w, http.StatusUnauthorized, "Not authenticated", "")
			return
		}

		if !user.IsAdmin {
			h.writeError(w, http.StatusForbidden, "Admin access required", "")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractToken extracts JWT token from Authorization header
func (h *AuthHandler) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

// writeError writes a JSON error response
func (h *AuthHandler) writeError(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   message,
		Message: details,
	}

	json.NewEncoder(w).Encode(response)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if ip := strings.Split(r.RemoteAddr, ":"); len(ip) > 0 {
		return ip[0]
	}

	return r.RemoteAddr
}
