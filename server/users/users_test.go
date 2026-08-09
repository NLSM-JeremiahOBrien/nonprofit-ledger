package users_test

import (
	"os"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/db"
	"github.com/tjcrowley/nonprofit-ledger/server/users"
)

func newTestDB(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ledger-*.db")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	return f
}

func TestByUsernameAndByID(t *testing.T) {
	f := newTestDB(t)
	f.Close()

	conn, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn, "../db/migrations"); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	res, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'staff_bookkeeper', 1)`,
		"carol", "some-hash",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}

	byUsername, err := users.ByUsername(conn, "carol")
	if err != nil {
		t.Fatalf("ByUsername: %v", err)
	}
	if byUsername.ID != id || byUsername.Username != "carol" || byUsername.Role != "staff_bookkeeper" {
		t.Fatalf("unexpected user: %+v", byUsername)
	}
	if !byUsername.Active {
		t.Fatal("expected Active=true")
	}

	byID, err := users.ByID(conn, id)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if byID.Username != "carol" {
		t.Fatalf("unexpected user: %+v", byID)
	}

	if _, err := users.ByUsername(conn, "nobody"); err == nil {
		t.Fatal("expected error for unknown username")
	}
}
