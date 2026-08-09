package ledger

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tjcrowley/nonprofit-ledger/server/audit"
)

// LockPeriod locks the ledger through throughDate (inclusive): entries
// dated on or before this date will be rejected by PostJournalEntry.
// accounting_periods is a mutable settings row (always id=1), not a
// posted ledger row, so it is intentionally NOT subject to the
// journal_entries/journal_lines immutability triggers — re-locking to a
// later date to extend the boundary is expected, ordinary usage.
//
// The settings write and its audit_log attribution happen in one
// transaction, matching the atomicity pattern used by PostJournalEntry
// and ReverseJournalEntry.
func LockPeriod(db *sql.DB, throughDate string, lockedBy int64) error {
	lockedAt := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("ledger: lock period: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO accounting_periods (id, locked_through_date, locked_by, locked_at)
		 VALUES (1, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   locked_through_date = excluded.locked_through_date,
		   locked_by = excluded.locked_by,
		   locked_at = excluded.locked_at`,
		throughDate, lockedBy, lockedAt,
	)
	if err != nil {
		return fmt.Errorf("ledger: lock period: %w", err)
	}

	if err := audit.Write(tx, lockedBy, "lock_period", "accounting_period", 1, throughDate); err != nil {
		return fmt.Errorf("ledger: lock period: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger: lock period: commit: %w", err)
	}

	return nil
}

// GetLockedThroughDate returns the current locked-through date, or nil
// if no period has ever been locked.
func GetLockedThroughDate(db *sql.DB) (*string, error) {
	var date sql.NullString
	err := db.QueryRow(`SELECT locked_through_date FROM accounting_periods WHERE id = 1`).Scan(&date)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get locked-through date: %w", err)
	}
	if !date.Valid {
		return nil, nil
	}

	d := date.String
	return &d, nil
}
