package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserLink struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Platform   string    `json:"platform"`
	Rel        string    `json:"rel"`
	Href       string    `json:"href"`
	Title      string    `json:"title"`
	Visibility string    `json:"visibility"` // public | registered | friends | private
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

type LinkStore interface {
	GetLinksForViewer(ctx context.Context, subjectID int, viewerID *int, areFriends bool) ([]UserLink, error)
	GetUserLinks(ctx context.Context, userID int) ([]UserLink, error)
	AddLink(ctx context.Context, link *UserLink) (*UserLink, error)
	UpdateLink(ctx context.Context, id, userID int, link *UserLink) (*UserLink, error)
	DeleteLink(ctx context.Context, id, userID int) error
}

type PostgresLinkStore struct {
	db *pgxpool.Pool
}

func NewPostgresLinkStore(db *pgxpool.Pool) *PostgresLinkStore {
	return &PostgresLinkStore{db: db}
}

func (s *PostgresLinkStore) GetLinksForViewer(ctx context.Context, subjectID int, viewerID *int, areFriends bool) ([]UserLink, error) {
	levels := []string{"'public'"}
	if viewerID != nil {
		if *viewerID == subjectID {
			levels = []string{"'public'", "'registered'", "'friends'", "'private'"}
		} else {
			levels = append(levels, "'registered'")
			if areFriends {
				levels = append(levels, "'friends'")
			}
		}
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, platform, rel, href, title, visibility, sort_order, created_at
		FROM user_links
		WHERE user_id = $1 AND visibility IN (%s)
		ORDER BY sort_order ASC, id ASC
	`, strings.Join(levels, ","))

	return s.queryLinks(ctx, query, subjectID)
}

func (s *PostgresLinkStore) GetUserLinks(ctx context.Context, userID int) ([]UserLink, error) {
	return s.queryLinks(ctx, `
		SELECT id, user_id, platform, rel, href, title, visibility, sort_order, created_at
		FROM user_links WHERE user_id = $1 ORDER BY sort_order ASC, id ASC
	`, userID)
}

func (s *PostgresLinkStore) queryLinks(ctx context.Context, query string, args ...any) ([]UserLink, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query links: %w", err)
	}
	defer rows.Close()

	var links []UserLink
	for rows.Next() {
		var l UserLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.Platform, &l.Rel, &l.Href, &l.Title, &l.Visibility, &l.SortOrder, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, l)
	}
	return links, nil
}

func (s *PostgresLinkStore) AddLink(ctx context.Context, link *UserLink) (*UserLink, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO user_links (user_id, platform, rel, href, title, visibility, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, user_id, platform, rel, href, title, visibility, sort_order, created_at
	`, link.UserID, link.Platform, link.Rel, link.Href, link.Title, link.Visibility, link.SortOrder)

	var l UserLink
	if err := row.Scan(&l.ID, &l.UserID, &l.Platform, &l.Rel, &l.Href, &l.Title, &l.Visibility, &l.SortOrder, &l.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to add link: %w", err)
	}
	return &l, nil
}

func (s *PostgresLinkStore) UpdateLink(ctx context.Context, id, userID int, link *UserLink) (*UserLink, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE user_links
		SET platform=$3, rel=$4, href=$5, title=$6, visibility=$7, sort_order=$8
		WHERE id=$1 AND user_id=$2
		RETURNING id, user_id, platform, rel, href, title, visibility, sort_order, created_at
	`, id, userID, link.Platform, link.Rel, link.Href, link.Title, link.Visibility, link.SortOrder)

	var l UserLink
	if err := row.Scan(&l.ID, &l.UserID, &l.Platform, &l.Rel, &l.Href, &l.Title, &l.Visibility, &l.SortOrder, &l.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to update link: %w", err)
	}
	return &l, nil
}

func (s *PostgresLinkStore) DeleteLink(ctx context.Context, id, userID int) error {
	result, err := s.db.Exec(ctx, "DELETE FROM user_links WHERE id=$1 AND user_id=$2", id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}
