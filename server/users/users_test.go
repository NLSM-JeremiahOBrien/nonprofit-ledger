package users_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
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

// openTestDB returns a fresh, fully-migrated database connection backed
// by a temp file, closed automatically at test cleanup.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	f := newTestDB(t)
	f.Close()

	conn, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.RunMigrations(conn, "../db/migrations"); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return conn
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

func TestCreateUser(t *testing.T) {
	t.Run("all three valid roles succeed with hashed password", func(t *testing.T) {
		conn := openTestDB(t)
		roles := []string{"staff_bookkeeper", "admin", "external_accountant"}
		for _, role := range roles {
			hash, err := auth.HashPassword("s3cret-password")
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}

			u, err := users.CreateUser(conn, "user-"+role, hash, role)
			if err != nil {
				t.Fatalf("CreateUser(%q): %v", role, err)
			}
			if u.Role != role {
				t.Fatalf("expected role %q, got %q", role, u.Role)
			}
			if u.PasswordHash == "s3cret-password" {
				t.Fatalf("password hash must never equal the plaintext password")
			}
			if !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
				t.Fatalf("expected argon2id-encoded hash, got %q", u.PasswordHash)
			}
		}
	})

	t.Run("invalid role rejected by Go validation before reaching SQL", func(t *testing.T) {
		conn := openTestDB(t)
		hash, err := auth.HashPassword("password")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}

		_, err = users.CreateUser(conn, "baduser", hash, "superuser")
		if err == nil {
			t.Fatal("expected error creating user with invalid role")
		}
		if !strings.Contains(err.Error(), "invalid role") {
			t.Fatalf("expected invalid role error, got: %v", err)
		}

		// Confirm nothing was inserted.
		var count int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, "baduser").Scan(&count); err != nil {
			t.Fatalf("count query: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no row inserted for invalid role, found %d", count)
		}
	})

	t.Run("SQL CHECK constraint independently rejects invalid role", func(t *testing.T) {
		conn := openTestDB(t)
		hash, err := auth.HashPassword("password")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}

		// Bypass Go-side validation entirely with a direct SQL insert.
		_, err = conn.Exec(
			`INSERT INTO users (username, password_hash, role, active, must_change_password)
			 VALUES (?, ?, ?, 1, 0)`,
			"directinsert", hash, "superuser",
		)
		if err == nil {
			t.Fatal("expected SQL CHECK constraint to reject invalid role on direct insert")
		}
	})

	t.Run("duplicate username rejected cleanly", func(t *testing.T) {
		conn := openTestDB(t)
		hash, err := auth.HashPassword("password")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}

		if _, err := users.CreateUser(conn, "dupeuser", hash, "admin"); err != nil {
			t.Fatalf("first CreateUser: %v", err)
		}

		_, err = users.CreateUser(conn, "dupeuser", hash, "staff_bookkeeper")
		if err == nil {
			t.Fatal("expected error creating user with duplicate username")
		}
		if !strings.Contains(err.Error(), "duplicate username") {
			t.Fatalf("expected duplicate username error, got: %v", err)
		}
	})
}
