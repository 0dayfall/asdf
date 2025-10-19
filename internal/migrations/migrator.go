package migrations

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Migrator struct {
	db          *sql.DB
	migrate     *migrate.Migrate
	sourceURL   string
	databaseURL string
}

// NewMigrator creates a new migration instance
func NewMigrator(databaseURL string) (*Migrator, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	sourceURL := "file://internal/migrations/sql"
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{
		db:          db,
		migrate:     m,
		sourceURL:   sourceURL,
		databaseURL: databaseURL,
	}, nil
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	err := m.migrate.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Migrations completed successfully")
	return nil
}

// Down rolls back one migration
func (m *Migrator) Down() error {
	err := m.migrate.Steps(-1)
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}
	log.Println("Migration rollback completed")
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// Force sets the migration version without running migrations
func (m *Migrator) Force(version int) error {
	err := m.migrate.Force(version)
	if err != nil {
		return fmt.Errorf("failed to force migration version: %w", err)
	}
	log.Printf("Forced migration version to %d", version)
	return nil
}

// Close closes the migrator and database connection
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return fmt.Errorf("failed to close migration source: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("failed to close database: %w", dbErr)
	}
	return nil
}

// CreateMigration creates new up and down migration files
func CreateMigration(name string) error {
	// This would typically use migrate CLI or implement file creation
	log.Printf("Create migration files for: %s", name)
	log.Println("Run: migrate create -ext sql -dir internal/migrations/sql -seq " + name)
	return nil
}
