package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Connection struct {
	ID                int       `json:"id"`
	RequesterID       int       `json:"requester_id"`
	AddresseeID       int       `json:"addressee_id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	RequesterUsername string    `json:"requester_username,omitempty"`
	RequesterDisplay  string    `json:"requester_display_name,omitempty"`
	AddresseeUsername string    `json:"addressee_username,omitempty"`
	AddresseeDisplay  string    `json:"addressee_display_name,omitempty"`
}

type ConnectionStore interface {
	RequestConnection(ctx context.Context, requesterID, addresseeID int) error
	AcceptConnection(ctx context.Context, requesterID, addresseeID int) error
	RemoveConnection(ctx context.Context, userID, otherID int) error
	AreFriends(ctx context.Context, userID1, userID2 int) (bool, error)
	GetConnectionStatus(ctx context.Context, userID1, userID2 int) (string, error)
	GetConnections(ctx context.Context, userID int) ([]Connection, error)
	GetPendingRequests(ctx context.Context, userID int) ([]Connection, error)
}

type PostgresConnectionStore struct {
	db *pgxpool.Pool
}

func NewPostgresConnectionStore(db *pgxpool.Pool) *PostgresConnectionStore {
	return &PostgresConnectionStore{db: db}
}

func (s *PostgresConnectionStore) RequestConnection(ctx context.Context, requesterID, addresseeID int) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO connections (requester_id, addressee_id, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (requester_id, addressee_id) DO NOTHING
	`, requesterID, addresseeID)
	if err != nil {
		return fmt.Errorf("failed to request connection: %w", err)
	}
	return nil
}

func (s *PostgresConnectionStore) AcceptConnection(ctx context.Context, requesterID, addresseeID int) error {
	result, err := s.db.Exec(ctx, `
		UPDATE connections SET status='accepted', updated_at=NOW()
		WHERE requester_id=$1 AND addressee_id=$2 AND status='pending'
	`, requesterID, addresseeID)
	if err != nil {
		return fmt.Errorf("failed to accept connection: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("pending request not found")
	}
	return nil
}

func (s *PostgresConnectionStore) RemoveConnection(ctx context.Context, userID, otherID int) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM connections
		WHERE (requester_id=$1 AND addressee_id=$2)
		   OR (requester_id=$2 AND addressee_id=$1)
	`, userID, otherID)
	return err
}

func (s *PostgresConnectionStore) AreFriends(ctx context.Context, userID1, userID2 int) (bool, error) {
	status, err := s.GetConnectionStatus(ctx, userID1, userID2)
	if err != nil {
		return false, err
	}
	return status == "accepted", nil
}

func (s *PostgresConnectionStore) GetConnectionStatus(ctx context.Context, userID1, userID2 int) (string, error) {
	var status string
	err := s.db.QueryRow(ctx, `
		SELECT status FROM connections
		WHERE (requester_id=$1 AND addressee_id=$2)
		   OR (requester_id=$2 AND addressee_id=$1)
		LIMIT 1
	`, userID1, userID2).Scan(&status)
	if err != nil {
		return "", nil // no connection = empty string, not an error
	}
	return status, nil
}

func (s *PostgresConnectionStore) GetConnections(ctx context.Context, userID int) ([]Connection, error) {
	return s.queryConnections(ctx, `
		SELECT c.id, c.requester_id, c.addressee_id, c.status, c.created_at,
		       r.username, COALESCE(r.display_name,''),
		       a.username, COALESCE(a.display_name,'')
		FROM connections c
		JOIN users r ON r.id = c.requester_id
		JOIN users a ON a.id = c.addressee_id
		WHERE (c.requester_id=$1 OR c.addressee_id=$1) AND c.status='accepted'
		ORDER BY c.created_at DESC
	`, userID)
}

func (s *PostgresConnectionStore) GetPendingRequests(ctx context.Context, userID int) ([]Connection, error) {
	return s.queryConnections(ctx, `
		SELECT c.id, c.requester_id, c.addressee_id, c.status, c.created_at,
		       r.username, COALESCE(r.display_name,''),
		       a.username, COALESCE(a.display_name,'')
		FROM connections c
		JOIN users r ON r.id = c.requester_id
		JOIN users a ON a.id = c.addressee_id
		WHERE c.addressee_id=$1 AND c.status='pending'
		ORDER BY c.created_at DESC
	`, userID)
}

func (s *PostgresConnectionStore) queryConnections(ctx context.Context, query string, args ...any) ([]Connection, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %w", err)
	}
	defer rows.Close()

	var conns []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(
			&c.ID, &c.RequesterID, &c.AddresseeID, &c.Status, &c.CreatedAt,
			&c.RequesterUsername, &c.RequesterDisplay,
			&c.AddresseeUsername, &c.AddresseeDisplay,
		); err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, nil
}
