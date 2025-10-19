package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore interface {
	CreateUser(ctx context.Context, user *CreateUserRequest) (*User, error)
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, id int, updates *UpdateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, id int) error
	ListUsers(ctx context.Context, filters *UserFilters) ([]*User, int, error)
	UpdateLastLogin(ctx context.Context, id int) error
	VerifyEmail(ctx context.Context, email string) error
}

type CreateUserRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8"`
	DisplayName string `json:"display_name,omitempty" validate:"max=100"`
	Bio         string `json:"bio,omitempty" validate:"max=500"`
	Website     string `json:"website,omitempty" validate:"url"`
	Location    string `json:"location,omitempty" validate:"max=100"`
	Domain      string `json:"domain,omitempty"`
}

type UpdateUserRequest struct {
	DisplayName   *string `json:"display_name,omitempty"`
	Bio           *string `json:"bio,omitempty"`
	Website       *string `json:"website,omitempty"`
	Location      *string `json:"location,omitempty"`
	AvatarURL     *string `json:"avatar_url,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	IsAdmin       *bool   `json:"is_admin,omitempty"`
	EmailVerified *bool   `json:"email_verified,omitempty"`
}

type UserFilters struct {
	Search   string `json:"search,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
	IsAdmin  *bool  `json:"is_admin,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type PostgresUserStore struct {
	db *pgxpool.Pool
}

// NewPostgresUserStore creates a new PostgreSQL user store
func NewPostgresUserStore(db *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

// CreateUser creates a new user
func (s *PostgresUserStore) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Default domain if not provided
	domain := req.Domain
	if domain == "" {
		parts := strings.Split(req.Email, "@")
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	subject := fmt.Sprintf("acct:%s@%s", req.Username, domain)

	query := `
		INSERT INTO users (username, domain, subject, email, password_hash, display_name, bio, website, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, username, domain, subject, email, display_name, bio, website, location, 
		          avatar_url, is_admin, is_active, email_verified, created_at, updated_at, last_login`

	var user User
	var displayName, bio, website, location, avatarURL sql.NullString
	var lastLogin sql.NullTime

	err := s.db.QueryRow(ctx, query,
		req.Username,
		domain,
		subject,
		req.Email,
		"", // Password hash will be set separately
		req.DisplayName,
		req.Bio,
		req.Website,
		req.Location,
	).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&displayName,
		&bio,
		&website,
		&location,
		&avatarURL,
		&user.IsAdmin,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Handle nullable fields
	if displayName.Valid {
		// Add DisplayName to User struct if needed
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *PostgresUserStore) GetUserByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, is_admin, is_active, email_verified, created_at, last_login
		FROM users 
		WHERE id = $1`

	var user User
	var lastLogin sql.NullTime

	err := s.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *PostgresUserStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, is_admin, is_active, email_verified, created_at, last_login
		FROM users 
		WHERE username = $1`

	var user User
	var lastLogin sql.NullTime

	err := s.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *PostgresUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, is_admin, is_active, email_verified, created_at, last_login
		FROM users 
		WHERE email = $1`

	var user User
	var lastLogin sql.NullTime

	err := s.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// UpdateUser updates a user's information
func (s *PostgresUserStore) UpdateUser(ctx context.Context, id int, req *UpdateUserRequest) (*User, error) {
	// Build dynamic query based on provided fields
	setParts := []string{"updated_at = NOW()"}
	args := []interface{}{id}
	argIndex := 2

	if req.DisplayName != nil {
		setParts = append(setParts, fmt.Sprintf("display_name = $%d", argIndex))
		args = append(args, *req.DisplayName)
		argIndex++
	}

	if req.Bio != nil {
		setParts = append(setParts, fmt.Sprintf("bio = $%d", argIndex))
		args = append(args, *req.Bio)
		argIndex++
	}

	if req.Website != nil {
		setParts = append(setParts, fmt.Sprintf("website = $%d", argIndex))
		args = append(args, *req.Website)
		argIndex++
	}

	if req.Location != nil {
		setParts = append(setParts, fmt.Sprintf("location = $%d", argIndex))
		args = append(args, *req.Location)
		argIndex++
	}

	if req.AvatarURL != nil {
		setParts = append(setParts, fmt.Sprintf("avatar_url = $%d", argIndex))
		args = append(args, *req.AvatarURL)
		argIndex++
	}

	if req.IsActive != nil {
		setParts = append(setParts, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	if req.IsAdmin != nil {
		setParts = append(setParts, fmt.Sprintf("is_admin = $%d", argIndex))
		args = append(args, *req.IsAdmin)
		argIndex++
	}

	if req.EmailVerified != nil {
		setParts = append(setParts, fmt.Sprintf("email_verified = $%d", argIndex))
		args = append(args, *req.EmailVerified)
		argIndex++
	}

	query := fmt.Sprintf(`
		UPDATE users 
		SET %s
		WHERE id = $1
		RETURNING id, username, email, is_admin, is_active, email_verified, created_at, updated_at, last_login`,
		strings.Join(setParts, ", "))

	var user User
	var lastLogin sql.NullTime

	err := s.db.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.IsAdmin,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// DeleteUser soft deletes a user (sets is_active to false)
func (s *PostgresUserStore) DeleteUser(ctx context.Context, id int) error {
	query := `UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1`

	result, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// ListUsers returns a paginated list of users with optional filtering
func (s *PostgresUserStore) ListUsers(ctx context.Context, filters *UserFilters) ([]*User, int, error) {
	// Build WHERE clauses
	whereParts := []string{"1=1"}
	args := []interface{}{}
	argIndex := 1

	if filters.Search != "" {
		whereParts = append(whereParts, fmt.Sprintf("(username ILIKE $%d OR email ILIKE $%d OR display_name ILIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, "%"+filters.Search+"%")
		argIndex++
	}

	if filters.IsActive != nil {
		whereParts = append(whereParts, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *filters.IsActive)
		argIndex++
	}

	if filters.IsAdmin != nil {
		whereParts = append(whereParts, fmt.Sprintf("is_admin = $%d", argIndex))
		args = append(args, *filters.IsAdmin)
		argIndex++
	}

	if filters.Domain != "" {
		whereParts = append(whereParts, fmt.Sprintf("domain = $%d", argIndex))
		args = append(args, filters.Domain)
		argIndex++
	}

	whereClause := strings.Join(whereParts, " AND ")

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereClause)
	var total int
	err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Main query with pagination
	limit := filters.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, username, email, is_admin, is_active, email_verified, created_at, updated_at, last_login
		FROM users 
		WHERE %s 
		ORDER BY created_at DESC 
		LIMIT $%d OFFSET $%d`,
		whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.IsAdmin,
			&user.IsActive,
			&user.EmailVerified,
			&user.CreatedAt,
			&lastLogin,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, &user)
	}

	return users, total, nil
}

// UpdateLastLogin updates the user's last login time
func (s *PostgresUserStore) UpdateLastLogin(ctx context.Context, id int) error {
	query := `UPDATE users SET last_login = NOW() WHERE id = $1`
	_, err := s.db.Exec(ctx, query, id)
	return err
}

// VerifyEmail marks a user's email as verified
func (s *PostgresUserStore) VerifyEmail(ctx context.Context, email string) error {
	query := `UPDATE users SET email_verified = true, updated_at = NOW() WHERE email = $1`
	result, err := s.db.Exec(ctx, query, email)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
