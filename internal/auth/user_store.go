package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore interface {
	CreateUser(ctx context.Context, user *CreateUserRequest) (*User, error)
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserBySubject(ctx context.Context, subject string) (*User, error)
	UpdateUser(ctx context.Context, id int, updates *UpdateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, id int) error
	ListUsers(ctx context.Context, filters *UserFilters) ([]*User, int, error)
	UpdateLastLogin(ctx context.Context, id int) error
	VerifyEmail(ctx context.Context, email string) error
}

// fullUserCols is the canonical column list for a full user SELECT.
const fullUserCols = `
	id, username, domain, subject, email, password_hash,
	COALESCE(display_name,'') AS display_name,
	COALESCE(bio,'')          AS bio,
	COALESCE(website,'')      AS website,
	COALESCE(location,'')     AS location,
	location_visibility,
	COALESCE(avatar_url,'')   AS avatar_url,
	COALESCE(status,'')       AS status,
	status_visibility,
	is_admin, is_active, email_verified,
	created_at, updated_at, last_login`

// scanUser populates a User from a row that selected fullUserCols.
func scanUser(scan func(...any) error) (*User, error) {
	var u User
	var updatedAt time.Time
	var lastLogin sql.NullTime
	err := scan(
		&u.ID, &u.Username, &u.Domain, &u.Subject, &u.Email, &u.PasswordHash,
		&u.DisplayName, &u.Bio, &u.Website, &u.Location, &u.LocationVisibility,
		&u.AvatarURL, &u.Status, &u.StatusVisibility,
		&u.IsAdmin, &u.IsActive, &u.EmailVerified,
		&u.CreatedAt, &updatedAt, &lastLogin,
	)
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLogin = &lastLogin.Time
	}
	return &u, nil
}

type CreateUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"` // already hashed by caller
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Website     string `json:"website,omitempty"`
	Location    string `json:"location,omitempty"`
	Domain      string `json:"domain,omitempty"`
}

type UpdateUserRequest struct {
	DisplayName        *string `json:"display_name,omitempty"`
	Bio                *string `json:"bio,omitempty"`
	Website            *string `json:"website,omitempty"`
	Location           *string `json:"location,omitempty"`
	LocationVisibility *string `json:"location_visibility,omitempty"`
	AvatarURL          *string `json:"avatar_url,omitempty"`
	Status             *string `json:"status,omitempty"`
	StatusVisibility   *string `json:"status_visibility,omitempty"`
	IsActive           *bool   `json:"is_active,omitempty"`
	IsAdmin            *bool   `json:"is_admin,omitempty"`
	EmailVerified      *bool   `json:"email_verified,omitempty"`
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

func NewPostgresUserStore(db *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

func (s *PostgresUserStore) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	domain := req.Domain
	if domain == "" {
		parts := strings.Split(req.Email, "@")
		if len(parts) == 2 {
			domain = parts[1]
		}
	}
	subject := fmt.Sprintf("acct:%s@%s", req.Username, domain)

	query := `
		INSERT INTO users
			(username, domain, subject, email, password_hash, display_name, bio, website, location, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		RETURNING ` + fullUserCols

	row := s.db.QueryRow(ctx, query,
		req.Username, domain, subject, req.Email, req.Password,
		req.DisplayName, req.Bio, req.Website, req.Location,
	)
	u, err := scanUser(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) GetUserByID(ctx context.Context, id int) (*User, error) {
	row := s.db.QueryRow(ctx, "SELECT "+fullUserCols+" FROM users WHERE id = $1", id)
	u, err := scanUser(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRow(ctx, "SELECT "+fullUserCols+" FROM users WHERE username = $1", username)
	u, err := scanUser(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.db.QueryRow(ctx, "SELECT "+fullUserCols+" FROM users WHERE email = $1", email)
	u, err := scanUser(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) GetUserBySubject(ctx context.Context, subject string) (*User, error) {
	row := s.db.QueryRow(ctx, "SELECT "+fullUserCols+" FROM users WHERE subject = $1", subject)
	u, err := scanUser(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by subject: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) UpdateUser(ctx context.Context, id int, req *UpdateUserRequest) (*User, error) {
	setParts := []string{"updated_at = NOW()"}
	args := []interface{}{id}
	idx := 2

	add := func(col string, val interface{}) {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if req.DisplayName != nil {
		add("display_name", *req.DisplayName)
	}
	if req.Bio != nil {
		add("bio", *req.Bio)
	}
	if req.Website != nil {
		add("website", *req.Website)
	}
	if req.Location != nil {
		add("location", *req.Location)
	}
	if req.LocationVisibility != nil {
		add("location_visibility", *req.LocationVisibility)
	}
	if req.AvatarURL != nil {
		add("avatar_url", *req.AvatarURL)
	}
	if req.Status != nil {
		add("status", *req.Status)
	}
	if req.StatusVisibility != nil {
		add("status_visibility", *req.StatusVisibility)
	}
	if req.IsActive != nil {
		add("is_active", *req.IsActive)
	}
	if req.IsAdmin != nil {
		add("is_admin", *req.IsAdmin)
	}
	if req.EmailVerified != nil {
		add("email_verified", *req.EmailVerified)
	}

	query := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = $1 RETURNING %s",
		strings.Join(setParts, ", "), fullUserCols,
	)

	row := s.db.QueryRow(ctx, query, args...)
	u, err := scanUser(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}
	return u, nil
}

func (s *PostgresUserStore) DeleteUser(ctx context.Context, id int) error {
	result, err := s.db.Exec(ctx,
		"UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *PostgresUserStore) ListUsers(ctx context.Context, filters *UserFilters) ([]*User, int, error) {
	whereParts := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if filters.Search != "" {
		whereParts = append(whereParts,
			fmt.Sprintf("(username ILIKE $%d OR email ILIKE $%d OR display_name ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+filters.Search+"%")
		idx++
	}
	if filters.IsActive != nil {
		whereParts = append(whereParts, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *filters.IsActive)
		idx++
	}
	if filters.IsAdmin != nil {
		whereParts = append(whereParts, fmt.Sprintf("is_admin = $%d", idx))
		args = append(args, *filters.IsAdmin)
		idx++
	}
	if filters.Domain != "" {
		whereParts = append(whereParts, fmt.Sprintf("domain = $%d", idx))
		args = append(args, filters.Domain)
		idx++
	}

	where := strings.Join(whereParts, " AND ")

	var total int
	if err := s.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM users WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	limit := filters.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(
		"SELECT %s FROM users WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		fullUserCols, where, idx, idx+1,
	)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := scanUser(rows.Scan)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (s *PostgresUserStore) UpdateLastLogin(ctx context.Context, id int) error {
	_, err := s.db.Exec(ctx, "UPDATE users SET last_login = NOW() WHERE id = $1", id)
	return err
}

func (s *PostgresUserStore) VerifyEmail(ctx context.Context, email string) error {
	result, err := s.db.Exec(ctx,
		"UPDATE users SET email_verified = true, updated_at = NOW() WHERE email = $1", email)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
