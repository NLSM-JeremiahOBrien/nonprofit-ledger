// Package users provides the user lookup and management surface used by
// the authentication and admin layers: read paths that login requires,
// plus full CRUD (create/list/update-role/deactivate) for admin-only
// user management.
package users

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// User represents a row in the users table. PasswordHash is the
// argon2id-encoded hash; plaintext passwords are never stored or
// returned in this struct.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	Role               string
	Active             bool
	MustChangePassword bool
}

// validRoles mirrors the migration 0005 SQL CHECK constraint exactly.
// Any role string not in this set is rejected before it ever reaches
// SQL.
var validRoles = map[string]bool{
	"staff_bookkeeper":    true,
	"admin":               true,
	"external_accountant": true,
}

// ErrInvalidRole is returned when a caller supplies a role string
// outside the fixed enum (staff_bookkeeper, admin, external_accountant).
var ErrInvalidRole = errors.New("users: invalid role")

// ErrDuplicateUsername is returned when creating a user whose username
// already exists.
var ErrDuplicateUsername = errors.New("users: duplicate username")

// ByUsername looks up a user by username. It returns sql.ErrNoRows
// (wrapped) if no such user exists.
func ByUsername(db *sql.DB, username string) (*User, error) {
	row := db.QueryRow(
		`SELECT id, username, password_hash, role, active, must_change_password
		 FROM users WHERE username = ?`,
		username,
	)
	return scanUser(row)
}

// ByID looks up a user by id. It returns sql.ErrNoRows (wrapped) if no
// such user exists.
func ByID(db *sql.DB, id int64) (*User, error) {
	row := db.QueryRow(
		`SELECT id, username, password_hash, role, active, must_change_password
		 FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

// CreateUser creates a new user with the given username, pre-hashed
// password, and role. The role is validated against the fixed Go-side
// enum (mirroring the SQL CHECK constraint) before the row is inserted.
// Returns ErrInvalidRole for an unrecognized role and
// ErrDuplicateUsername if the username already exists.
//
// The password must already be hashed by the caller (see
// auth.HashPassword) — this package never hashes or handles plaintext
// passwords itself, since server/auth already imports server/users for
// its own lookups and this package cannot import server/auth back
// without creating an import cycle.
func CreateUser(db *sql.DB, username, passwordHash, role string) (*User, error) {
	if !validRoles[role] {
		return nil, fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}

	res, err := db.Exec(
		`INSERT INTO users (username, password_hash, role, active, must_change_password)
		 VALUES (?, ?, ?, 1, 0)`,
		username, passwordHash, role,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateUsername, username)
		}
		return nil, fmt.Errorf("users: create: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("users: create: %w", err)
	}

	return &User{
		ID:                 id,
		Username:           username,
		PasswordHash:       passwordHash,
		Role:               role,
		Active:             true,
		MustChangePassword: false,
	}, nil
}

// ListUsers returns every user in the system, ordered by id.
func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(
		`SELECT id, username, password_hash, role, active, must_change_password
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("users: list: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var active, mustChange int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &active, &mustChange); err != nil {
			return nil, fmt.Errorf("users: list: %w", err)
		}
		u.Active = active != 0
		u.MustChangePassword = mustChange != 0
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("users: list: %w", err)
	}
	return out, nil
}

// UpdateRole changes the role of the user with the given id. The new
// role is validated against the fixed enum before the update runs.
func UpdateRole(db *sql.DB, userID int64, newRole string) error {
	if !validRoles[newRole] {
		return fmt.Errorf("%w: %q", ErrInvalidRole, newRole)
	}

	res, err := db.Exec(`UPDATE users SET role = ? WHERE id = ?`, newRole, userID)
	if err != nil {
		return fmt.Errorf("users: update role: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("users: update role: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("users: update role: %w", sql.ErrNoRows)
	}
	return nil
}

// Deactivate marks the user with the given id as inactive (active = 0).
// It never deletes the row — deactivated users remain in the audit
// trail.
func Deactivate(db *sql.DB, userID int64) error {
	res, err := db.Exec(`UPDATE users SET active = 0 WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("users: deactivate: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("users: deactivate: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("users: deactivate: %w", sql.ErrNoRows)
	}
	return nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var active, mustChange int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &active, &mustChange); err != nil {
		return nil, fmt.Errorf("users: lookup: %w", err)
	}
	u.Active = active != 0
	u.MustChangePassword = mustChange != 0
	return &u, nil
}

// isUniqueConstraintErr reports whether err represents a SQLite unique
// constraint violation. This is checked by string matching on the
// driver error text since modernc.org/sqlite does not export a typed
// sentinel for this case.
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed: UNIQUE")
}
