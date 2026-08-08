package ledger

import (
	"database/sql"
	"fmt"
)

// NewLine is a single debit or credit leg of a new journal entry.
// FunctionalCategory is required only when AccountID resolves to an
// expense-type account; it must remain nil for balance-sheet accounts
// (asset/liability/equity) and is optional for revenue accounts.
type NewLine struct {
	AccountID          int64
	FundID             int64
	FunctionalCategory *string
	DebitAmount        int64 // integer cents
	CreditAmount       int64 // integer cents
}

// NewEntry is the input to PostJournalEntry.
type NewEntry struct {
	EntryDate string // ISO 8601 (YYYY-MM-DD)
	Memo      string
	PostedBy  int64
	Source    string // defaults to "manual" if empty
	Lines     []NewLine
}

// PostJournalEntry is the single sanctioned entry point for creating new
// (non-reversal) journal entries. Every posting path in the system —
// manual entry now, imports/reversals/bank-rec in later phases — must go
// through this function so balance, period-lock, and functional-category
// invariants are enforced in exactly one place.
//
// It enforces, in order:
//  1. at least one line is present
//  2. debits sum to credits (and the total is non-zero)
//  3. EntryDate is strictly after the locked-through date, if any
//  4. every line against an expense-type account carries a
//     FunctionalCategory
//
// All checks that require a DB read happen inside a single transaction
// that is rolled back on any failure, so no partial entry is ever
// written.
func PostJournalEntry(db *sql.DB, e NewEntry) (int64, error) {
	if len(e.Lines) == 0 {
		return 0, fmt.Errorf("ledger: journal entry must have at least one line")
	}
	if !balances(e.Lines) {
		return 0, fmt.Errorf("ledger: journal entry is not balanced (debits must equal credits and be non-zero)")
	}

	source := e.Source
	if source == "" {
		source = "manual"
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("ledger: post journal entry: begin tx: %w", err)
	}
	defer tx.Rollback()

	lockedThrough, err := getLockedThroughDateTx(tx)
	if err != nil {
		return 0, fmt.Errorf("ledger: post journal entry: checking period lock: %w", err)
	}
	if lockedThrough != nil && e.EntryDate <= *lockedThrough {
		return 0, fmt.Errorf("ledger: entry date %s falls on or before locked-through date %s", e.EntryDate, *lockedThrough)
	}

	for i, line := range e.Lines {
		accountType, err := lookupAccountType(tx, line.AccountID)
		if err != nil {
			return 0, fmt.Errorf("ledger: post journal entry: line %d: %w", i, err)
		}
		if accountType == "expense" && line.FunctionalCategory == nil {
			return 0, fmt.Errorf("ledger: line %d: functional category is required for expense-type account %d", i, line.AccountID)
		}
	}

	res, err := tx.Exec(
		`INSERT INTO journal_entries (entry_date, memo, posted_by, posted_at, source)
		 VALUES (?, ?, ?, datetime('now'), ?)`,
		e.EntryDate, e.Memo, e.PostedBy, source,
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: post journal entry: insert entry: %w", err)
	}

	entryID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ledger: post journal entry: entry id: %w", err)
	}

	for i, line := range e.Lines {
		_, err := tx.Exec(
			`INSERT INTO journal_lines (entry_id, account_id, fund_id, functional_category, debit_amount, credit_amount)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			entryID, line.AccountID, line.FundID, line.FunctionalCategory, line.DebitAmount, line.CreditAmount,
		)
		if err != nil {
			return 0, fmt.Errorf("ledger: post journal entry: insert line %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ledger: post journal entry: commit: %w", err)
	}

	return entryID, nil
}

// balances reports whether the sum of debits equals the sum of credits
// across lines, and that total is non-zero (a zero-total entry, e.g. all
// lines with amount 0, is not a meaningful posting).
func balances(lines []NewLine) bool {
	var debits, credits int64
	for _, l := range lines {
		debits += l.DebitAmount
		credits += l.CreditAmount
	}
	return debits == credits && debits != 0
}

// getLockedThroughDateTx mirrors GetLockedThroughDate but reads within an
// existing transaction so the period-lock check is part of the same
// atomic unit as the entry insert.
func getLockedThroughDateTx(tx *sql.Tx) (*string, error) {
	var date sql.NullString
	err := tx.QueryRow(`SELECT locked_through_date FROM accounting_periods WHERE id = 1`).Scan(&date)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !date.Valid {
		return nil, nil
	}

	d := date.String
	return &d, nil
}

// lookupAccountType resolves the type of an account within tx, returning
// an error if the account does not exist.
func lookupAccountType(tx *sql.Tx, accountID int64) (string, error) {
	var accountType string
	err := tx.QueryRow(`SELECT type FROM accounts WHERE id = ?`, accountID).Scan(&accountType)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("account %d does not exist", accountID)
	}
	if err != nil {
		return "", fmt.Errorf("looking up account %d type: %w", accountID, err)
	}
	return accountType, nil
}
