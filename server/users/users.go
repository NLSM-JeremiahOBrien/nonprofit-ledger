// Package users provides the minimal user lookup surface needed by the
// authentication layer. Full user CRUD (create/list/update-role) is
// added in a later plan; this package only supports the read path that
// login requires.
package users

import (
	"database/sql"
	"fmt"
)

// User represents a row in the users table.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	Role               string
	Active             bool
	MustChangePassword bool
}

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
