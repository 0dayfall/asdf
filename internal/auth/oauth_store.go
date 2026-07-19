package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OAuthAccount struct {
	ID                  int        `json:"id"`
	UserID              int        `json:"user_id"`
	Provider            string     `json:"provider"`
	ProviderUserID      string     `json:"provider_user_id"`
	ProviderUsername    string     `json:"provider_username"`
	ProviderDisplayName string     `json:"provider_display_name"`
	AccessToken         string     `json:"-"`
	RefreshToken        string     `json:"-"`
	TokenExpiresAt      *time.Time `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
}

type OAuthStore interface {
	FindAccount(ctx context.Context, provider, providerUserID string) (*OAuthAccount, error)
	UpsertAccount(ctx context.Context, acc *OAuthAccount) error
	GetUserAccounts(ctx context.Context, userID int) ([]OAuthAccount, error)
	DeleteAccount(ctx context.Context, userID int, provider string) error
}

type PostgresOAuthStore struct {
	db *pgxpool.Pool
}

func NewPostgresOAuthStore(db *pgxpool.Pool) *PostgresOAuthStore {
	return &PostgresOAuthStore{db: db}
}

func (s *PostgresOAuthStore) FindAccount(ctx context.Context, provider, providerUserID string) (*OAuthAccount, error) {
	var acc OAuthAccount
	var expiresAt sql.NullTime
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, provider, provider_user_id, provider_username,
		       provider_display_name, access_token, refresh_token, token_expires_at, created_at
		FROM oauth_accounts
		WHERE provider=$1 AND provider_user_id=$2
	`, provider, providerUserID).Scan(
		&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID,
		&acc.ProviderUsername, &acc.ProviderDisplayName,
		&acc.AccessToken, &acc.RefreshToken, &expiresAt, &acc.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find oauth account: %w", err)
	}
	if expiresAt.Valid {
		acc.TokenExpiresAt = &expiresAt.Time
	}
	return &acc, nil
}

func (s *PostgresOAuthStore) UpsertAccount(ctx context.Context, acc *OAuthAccount) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO oauth_accounts
			(user_id, provider, provider_user_id, provider_username, provider_display_name,
			 access_token, refresh_token, token_expires_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET
			user_id              = EXCLUDED.user_id,
			provider_username    = EXCLUDED.provider_username,
			provider_display_name= EXCLUDED.provider_display_name,
			access_token         = EXCLUDED.access_token,
			refresh_token        = EXCLUDED.refresh_token,
			token_expires_at     = EXCLUDED.token_expires_at,
			updated_at           = NOW()
	`, acc.UserID, acc.Provider, acc.ProviderUserID, acc.ProviderUsername,
		acc.ProviderDisplayName, acc.AccessToken, acc.RefreshToken, acc.TokenExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert oauth account: %w", err)
	}
	return nil
}

func (s *PostgresOAuthStore) GetUserAccounts(ctx context.Context, userID int) ([]OAuthAccount, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, provider, provider_user_id, provider_username,
		       provider_display_name, created_at
		FROM oauth_accounts WHERE user_id=$1 ORDER BY provider
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth accounts: %w", err)
	}
	defer rows.Close()

	var accs []OAuthAccount
	for rows.Next() {
		var acc OAuthAccount
		if err := rows.Scan(&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID,
			&acc.ProviderUsername, &acc.ProviderDisplayName, &acc.CreatedAt); err != nil {
			return nil, err
		}
		accs = append(accs, acc)
	}
	return accs, nil
}

func (s *PostgresOAuthStore) DeleteAccount(ctx context.Context, userID int, provider string) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM oauth_accounts WHERE user_id=$1 AND provider=$2", userID, provider)
	return err
}
