package ledger

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// newTestDB creates a fresh on-disk SQLite database in a temp directory,
// runs all migrations against it, and returns the resulting *sql.DB.
// A real file (not ":memory:") is used deliberately: later plans'
// immutability trigger tests require durable on-disk behavior consistent
// with production.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("newTestDB: open: %v", err)
	}

	if err := db.RunMigrations(conn, migrationsDir(t)); err != nil {
		conn.Close()
		t.Fatalf("newTestDB: run migrations: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// migrationsDir resolves the absolute path to server/db/migrations
// relative to this source file, regardless of the working directory
// the test binary is invoked from.
func migrationsDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migrationsDir: unable to determine caller")
	}

	// this file lives at server/ledger/testutil_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "db", "migrations")
}
