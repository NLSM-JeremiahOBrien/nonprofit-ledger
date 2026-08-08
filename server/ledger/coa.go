package ledger

import (
	"database/sql"
	"fmt"
)

// validAccountTypes is the fixed five-value chart-of-accounts type
// enumeration. Kept in sync with the CHECK constraint on the accounts
// table in server/db/migrations/0001_accounts.up.sql.
var validAccountTypes = map[string]bool{
	"asset":     true,
	"liability": true,
	"equity":    true,
	"revenue":   true,
	"expense":   true,
}

// Account is a single chart-of-accounts entry, optionally nested under
// a parent account to form a hierarchy.
type Account struct {
	ID       int64
	Code     string
	Name     string
	Type     string // "asset"|"liability"|"equity"|"revenue"|"expense"
	ParentID *int64
	IsActive bool
}

// NewAccount is the input to CreateAccount.
type NewAccount struct {
	Code     string
	Name     string
	Type     string
	ParentID *int64
	// IsActive defaults to true when not explicitly set to false via
	// IsActiveSet. Most callers can leave both fields zero-valued.
	IsActive    bool
	IsActiveSet bool
}

// CreateAccount validates and inserts a new chart-of-accounts row.
// It validates Type against the five allowed values and, when ParentID
// is set, confirms the parent account exists before inserting, ensuring
// hierarchy integrity.
func CreateAccount(db *sql.DB, a NewAccount) (int64, error) {
	if a.Code == "" {
		return 0, fmt.Errorf("ledger: account code is required")
	}
	if !validAccountTypes[a.Type] {
		return 0, fmt.Errorf("ledger: invalid account type %q", a.Type)
	}

	if a.ParentID != nil {
		var exists int
		err := db.QueryRow(`SELECT 1 FROM accounts WHERE id = ?`, *a.ParentID).Scan(&exists)
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("ledger: parent account %d does not exist", *a.ParentID)
		}
		if err != nil {
			return 0, fmt.Errorf("ledger: checking parent account: %w", err)
		}
	}

	isActive := 1
	if a.IsActiveSet && !a.IsActive {
		isActive = 0
	}

	res, err := db.Exec(
		`INSERT INTO accounts (code, name, type, parent_id, is_active) VALUES (?, ?, ?, ?, ?)`,
		a.Code, a.Name, a.Type, a.ParentID, isActive,
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: create account: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ledger: create account: %w", err)
	}

	return id, nil
}
