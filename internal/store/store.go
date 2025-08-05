package store

import (
	"asdf/internal/types"
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	LookupBySubject(ctx context.Context, subject string) (*types.JRD, error)
	SearchSubjects(ctx context.Context, query string) ([]string, error)
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{db: db}
}

func (ps *PostgresStore) LookupBySubject(ctx context.Context, subject string) (*types.JRD, error) {
	const q = `
	SELECT subject, aliases, properties, links
	FROM users
	WHERE subject = $1
	`

	row := ps.db.QueryRow(ctx, q, subject)

	var aliases []string
	var propsJSON, linksJSON []byte
	var returnedSubject string

	if err := row.Scan(&returnedSubject, &aliases, &propsJSON, &linksJSON); err != nil {
		return nil, err
	}

	var properties map[string]interface{}
	if err := json.Unmarshal(propsJSON, &properties); err != nil {
		return nil, err
	}

	var links []types.Link
	if err := json.Unmarshal(linksJSON, &links); err != nil {
		return nil, err
	}

	return &types.JRD{
		Subject:    returnedSubject,
		Aliases:    aliases,
		Properties: properties,
		Links:      links,
	}, nil
}

func (ps *PostgresStore) InitSchemaAndSeed(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	domain TEXT NOT NULL,
	subject TEXT UNIQUE NOT NULL,
	aliases TEXT[],
	properties JSONB,
	links JSONB NOT NULL
);`

	_, err := ps.db.Exec(ctx, schema)
	if err != nil {
		return err
	}

	// Optional seed
	const check = `SELECT COUNT(*) FROM users WHERE subject = $1`
	var count int
	err = ps.db.QueryRow(ctx, check, "acct:example@example.com").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		const seed = `
INSERT INTO users (username, domain, subject, aliases, properties, links)
VALUES ($1, $2, $3, $4, $5, $6);`

		aliases := []string{"http://example.com/profile/example"}
		properties := `{"http://example.com/prop/name": "Example User"}`
		links := `[{"rel":"http://webfinger.net/rel/profile-page","type":"text/html","href":"http://example.com/profile/example"},
		           {"rel":"http://example.com/rel/blog","type":"text/html","href":"http://blogs.example.com/example/"}]`

		_, err = ps.db.Exec(ctx, seed,
			"example",
			"example.com",
			"acct:example@example.com",
			aliases,
			[]byte(properties),
			[]byte(links),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ps *PostgresStore) SearchSubjects(ctx context.Context, query string) ([]string, error) {
	rows, err := ps.db.Query(ctx, `
		SELECT subject
		FROM users
		WHERE LOWER(subject) LIKE '%' || $1 || '%'
		ORDER BY subject ASC
		LIMIT 25
	`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var subject string
		if err := rows.Scan(&subject); err == nil {
			results = append(results, subject)
		}
	}
	return results, nil
}
