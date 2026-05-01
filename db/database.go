package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB represents a database connection
type DB struct {
	conn *sql.DB
}

// Open opens a database connection
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	// Open SQLite connection
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only allows one writer at a time; pin to a single connection so
	// PRAGMA settings remain in effect and concurrent writes don't race.
	conn.SetMaxOpenConns(1)

	// Verify connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{conn: conn}
	return db, nil
}

// OpenMemory creates an in-memory database for testing
func OpenMemory() (*DB, error) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{conn: conn}
	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, nil)
}

// ExecContext executes a query without returning rows
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query that returns a single row
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

// RunMigrations runs all pending migrations
func (db *DB) RunMigrations(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	_, err := db.ExecContext(ctx, SchemaMigrationsTable)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if _, ok := applied[m.Version]; ok {
			continue // already applied
		}

		if err := db.applyMigration(ctx, m); err != nil {
			return fmt.Errorf("applying migration %s: %w", m.Version, err)
		}
	}

	return nil
}

// getAppliedMigrations returns a map of applied migration versions
func (db *DB) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// applyMigration applies a single migration
func (db *DB) applyMigration(ctx context.Context, m struct {
	Version string
	SQL     []string
}) error {
	tx, err := db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Execute each SQL statement in the migration
	for _, sqlStmt := range m.SQL {
		_, err := tx.ExecContext(ctx, sqlStmt)
		if err != nil {
			return fmt.Errorf("executing SQL: %w", err)
		}
	}

	// Record the migration
	_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", m.Version)
	if err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}

	return nil
}

// IsMigrationApplied checks if a specific migration version has been applied
func (db *DB) IsMigrationApplied(ctx context.Context, version string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMigrationVersions returns all applied migration versions
func (db *DB) GetMigrationVersions(ctx context.Context) ([]Migration, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, version, applied_at FROM schema_migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var m Migration
		if err := rows.Scan(&m.ID, &m.Version, &m.AppliedAt); err != nil {
			return nil, err
		}
		migrations = append(migrations, m)
	}

	return migrations, rows.Err()
}

// DefaultDBPath returns the default database path
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.sharp-tools/sharp-tools.db"
	}
	return filepath.Join(home, ".sharp-tools", "sharp-tools.db")
}