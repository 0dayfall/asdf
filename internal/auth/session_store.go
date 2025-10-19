package auth

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSessionStore struct {
	db *pgxpool.Pool
}

// NewPostgresSessionStore creates a new PostgreSQL session store
func NewPostgresSessionStore(db *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{db: db}
}

// CreateSession stores a new session in the database
func (s *PostgresSessionStore) CreateSession(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.CreatedAt,
		session.LastUsedAt,
		session.IPAddress,
		session.UserAgent,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by token hash
func (s *PostgresSessionStore) GetSession(ctx context.Context, tokenHash string) (*Session, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, last_used_at, 
		       COALESCE(ip_address::text, '') as ip_address, 
		       COALESCE(user_agent, '') as user_agent
		FROM sessions 
		WHERE token_hash = $1 AND expires_at > NOW()`

	var session Session
	err := s.db.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.IPAddress,
		&session.UserAgent,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Update last used time
	updateQuery := `UPDATE sessions SET last_used_at = NOW() WHERE token_hash = $1`
	_, _ = s.db.Exec(ctx, updateQuery, tokenHash)

	return &session, nil
}

// DeleteSession removes a session by token hash
func (s *PostgresSessionStore) DeleteSession(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM sessions WHERE token_hash = $1`

	result, err := s.db.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteUserSessions removes all sessions for a user
func (s *PostgresSessionStore) DeleteUserSessions(ctx context.Context, userID int) error {
	query := `DELETE FROM sessions WHERE user_id = $1`

	_, err := s.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all expired sessions
func (s *PostgresSessionStore) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at <= NOW()`

	result, err := s.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	// Log the number of cleaned up sessions (you might want to use structured logging here)
	_ = result.RowsAffected()

	return nil
}

// GetUserSessions returns all active sessions for a user
func (s *PostgresSessionStore) GetUserSessions(ctx context.Context, userID int) ([]*Session, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, last_used_at,
		       COALESCE(ip_address::text, '') as ip_address,
		       COALESCE(user_agent, '') as user_agent
		FROM sessions 
		WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY last_used_at DESC`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.TokenHash,
			&session.ExpiresAt,
			&session.CreatedAt,
			&session.LastUsedAt,
			&session.IPAddress,
			&session.UserAgent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}
