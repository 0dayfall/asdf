package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

type AuthService struct {
	jwtSecret    []byte
	tokenExpiry  time.Duration
	sessionStore SessionStore
}

type SessionStore interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, tokenHash string) (*Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID int) error
	CleanupExpiredSessions(ctx context.Context) error
}

type Session struct {
	ID         string    `json:"id"`
	UserID     int       `json:"user_id"`
	TokenHash  string    `json:"token_hash"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	IPAddress  string    `json:"ip_address,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
}

type User struct {
	ID            int        `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	IsAdmin       bool       `json:"is_admin"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
}

// NewAuthService creates a new authentication service
func NewAuthService(jwtSecret string, tokenExpiry time.Duration, sessionStore SessionStore) *AuthService {
	return &AuthService{
		jwtSecret:    []byte(jwtSecret),
		tokenExpiry:  tokenExpiry,
		sessionStore: sessionStore,
	}
}

// HashPassword creates a bcrypt hash of the password
func (a *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks if the password matches the hash
func (a *AuthService) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates a new JWT token and session
func (a *AuthService) GenerateToken(ctx context.Context, user *User, ipAddress, userAgent string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(a.tokenExpiry)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "asdf-webfinger",
			Subject:   fmt.Sprintf("user:%d", user.ID),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Create session record
	tokenHash := a.hashToken(tokenString)
	session := &Session{
		ID:         claims.RegisteredClaims.ID,
		UserID:     user.ID,
		TokenHash:  tokenHash,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		LastUsedAt: now,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}

	if err := a.sessionStore.CreateSession(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and checks if session exists
func (a *AuthService) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if session exists and is valid
	tokenHash := a.hashToken(tokenString)
	session, err := a.sessionStore.GetSession(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("session expired")
	}

	return claims, nil
}

// RevokeToken revokes a token by deleting its session
func (a *AuthService) RevokeToken(ctx context.Context, tokenString string) error {
	tokenHash := a.hashToken(tokenString)
	return a.sessionStore.DeleteSession(ctx, tokenHash)
}

// RevokeAllUserTokens revokes all tokens for a user
func (a *AuthService) RevokeAllUserTokens(ctx context.Context, userID int) error {
	return a.sessionStore.DeleteUserSessions(ctx, userID)
}

// CleanupExpiredSessions removes expired sessions
func (a *AuthService) CleanupExpiredSessions(ctx context.Context) error {
	return a.sessionStore.CleanupExpiredSessions(ctx)
}

// hashToken creates a SHA-256 hash of the token for storage
func (a *AuthService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// RefreshToken creates a new token for an existing valid session
func (a *AuthService) RefreshToken(ctx context.Context, oldToken string, ipAddress, userAgent string) (string, error) {
	// Validate the old token first
	claims, err := a.ValidateToken(ctx, oldToken)
	if err != nil {
		return "", fmt.Errorf("invalid token for refresh: %w", err)
	}

	// Create a new user object from claims
	user := &User{
		ID:       claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
		IsAdmin:  claims.IsAdmin,
	}

	// Revoke the old token
	if err := a.RevokeToken(ctx, oldToken); err != nil {
		// Log error but don't fail the refresh
		// The old token will expire naturally
	}

	// Generate a new token
	return a.GenerateToken(ctx, user, ipAddress, userAgent)
}
