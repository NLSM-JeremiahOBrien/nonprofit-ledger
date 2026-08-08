package ledger

import (
	"database/sql"
	"fmt"
)

// reversalLine is a single journal_lines row read from the original
// entry, carrying just the fields needed to construct a swapped copy.
type reversalLine struct {
	AccountID          int64
	FundID             int64
	FunctionalCategory sql.NullString
	DebitAmount        int64
	CreditAmount       int64
}

// ReverseJournalEntry is the only sanctioned mechanism for correcting a
// posted journal entry. It never issues UPDATE/DELETE against
// journal_entries or journal_lines. Instead it reads the original
// entry's lines (read-only), inserts a brand-new journal_entries row
// with reverses_entry_id set to originalID and source "reversal", and
// inserts lines that mirror the original's lines with debit_amount and
// credit_amount swapped (same account_id, fund_id, functional_category).
//
// The caller owns the transaction: ReverseJournalEntry does not call
// Commit or Rollback.
func ReverseJournalEntry(tx *sql.Tx, originalID int64, reason string, postedBy int64) (int64, error) {
	var entryDate, postedAt string
	err := tx.QueryRow(
		`SELECT entry_date, posted_at FROM journal_entries WHERE id = ?`,
		originalID,
	).Scan(&entryDate, &postedAt)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("ledger: original journal entry %d does not exist", originalID)
	}
	if err != nil {
		return 0, fmt.Errorf("ledger: reading original entry: %w", err)
	}

	lines, err := readReversalLines(tx, originalID)
	if err != nil {
		return 0, err
	}
	if len(lines) == 0 {
		return 0, fmt.Errorf("ledger: original journal entry %d has no lines to reverse", originalID)
	}

	memo := "Reversal: " + reason

	res, err := tx.Exec(
		`INSERT INTO journal_entries (entry_date, memo, posted_by, posted_at, source, reverses_entry_id)
		 VALUES (?, ?, ?, ?, 'reversal', ?)`,
		entryDate, memo, postedBy, postedAt, originalID,
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: inserting reversal entry: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ledger: reversal entry id: %w", err)
	}

	for _, l := range lines {
		_, err := tx.Exec(
			`INSERT INTO journal_lines (entry_id, account_id, fund_id, functional_category, debit_amount, credit_amount)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			newID, l.AccountID, l.FundID, l.FunctionalCategory, l.CreditAmount, l.DebitAmount,
		)
		if err != nil {
			return 0, fmt.Errorf("ledger: inserting reversal line: %w", err)
		}
	}

	return newID, nil
}

// readReversalLines reads the original entry's lines read-only, in a
// stable order, for use in constructing the swapped reversal copy.
func readReversalLines(tx *sql.Tx, originalID int64) ([]reversalLine, error) {
	rows, err := tx.Query(
		`SELECT account_id, fund_id, functional_category, debit_amount, credit_amount
		 FROM journal_lines WHERE entry_id = ? ORDER BY id`,
		originalID,
	)
	if err != nil {
		return nil, fmt.Errorf("ledger: reading original lines: %w", err)
	}
	defer rows.Close()

	var lines []reversalLine
	for rows.Next() {
		var l reversalLine
		if err := rows.Scan(&l.AccountID, &l.FundID, &l.FunctionalCategory, &l.DebitAmount, &l.CreditAmount); err != nil {
			return nil, fmt.Errorf("ledger: scanning original line: %w", err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterating original lines: %w", err)
	}

	return lines, nil
}
