// Package db provides the SQLite connection and migration-runner used
// across the application. Money is stored as integer cents everywhere;
// no REAL/float columns are ever used for monetary values.
package db

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"
)

// Open opens the SQLite database file at path using the pure-Go
// modernc.org/sqlite driver, enables foreign key enforcement and WAL
// journaling on the connection, and returns the resulting *sql.DB.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: enable foreign_keys: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: enable WAL journal mode: %w", err)
	}

	return conn, nil
}

// RunMigrations applies all pending "up" migrations found in
// migrationsDir to db using golang-migrate.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	driver, err := sqlitemigrate.WithInstance(db, &sqlitemigrate.Config{})
	if err != nil {
		return fmt.Errorf("db: create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"sqlite",
		driver,
	)
	if err != nil {
		return fmt.Errorf("db: create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("db: run migrations: %w", err)
	}

	return nil
}
