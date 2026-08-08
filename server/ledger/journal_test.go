package ledger

import (
	"database/sql"
	"strings"
	"testing"
)

// seedAccountAndFund inserts one account and one fund directly via the
// Plan 01-01 domain functions, returning their IDs for use in journal
// line tests.
func seedAccountAndFund(t *testing.T, db *sql.DB) (accountID, fundID int64) {
	t.Helper()

	accountID, err := CreateAccount(db, NewAccount{
		Code: "1000",
		Name: "Cash",
		Type: "asset",
	})
	if err != nil {
		t.Fatalf("seedAccountAndFund: CreateAccount: %v", err)
	}

	fundID, err = CreateFund(db, NewFund{
		Code:          "GEN",
		Name:          "General Fund",
		NetAssetClass: "without_donor_restrictions",
	})
	if err != nil {
		t.Fatalf("seedAccountAndFund: CreateFund: %v", err)
	}

	return accountID, fundID
}

// insertEntryAndLine inserts a minimal balanced journal entry (one line)
// directly via raw SQL, returning the new entry ID and line ID.
func insertEntryAndLine(t *testing.T, db *sql.DB, accountID, fundID int64) (entryID, lineID int64) {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO journal_entries (entry_date, memo, posted_by, posted_at, source)
		 VALUES (?, ?, ?, ?, ?)`,
		"2026-08-08", "test entry", 1, "2026-08-08T00:00:00Z", "manual",
	)
	if err != nil {
		t.Fatalf("insertEntryAndLine: insert entry: %v", err)
	}
	entryID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("insertEntryAndLine: entry id: %v", err)
	}

	res, err = db.Exec(
		`INSERT INTO journal_lines (entry_id, account_id, fund_id, debit_amount, credit_amount)
		 VALUES (?, ?, ?, ?, ?)`,
		entryID, accountID, fundID, 1000, 0,
	)
	if err != nil {
		t.Fatalf("insertEntryAndLine: insert line: %v", err)
	}
	lineID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("insertEntryAndLine: line id: %v", err)
	}

	return entryID, lineID
}

func TestImmutability(t *testing.T) {
	db := newTestDB(t)
	accountID, fundID := seedAccountAndFund(t, db)
	entryID, lineID := insertEntryAndLine(t, db, accountID, fundID)

	t.Run("journal_lines UPDATE rejected", func(t *testing.T) {
		_, err := db.Exec(`UPDATE journal_lines SET debit_amount = 999 WHERE id = ?`, lineID)
		if err == nil {
			t.Fatal("expected UPDATE on journal_lines to be rejected, got nil error")
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Errorf("expected append-only abort message, got: %v", err)
		}
	})

	t.Run("journal_lines DELETE rejected", func(t *testing.T) {
		_, err := db.Exec(`DELETE FROM journal_lines WHERE id = ?`, lineID)
		if err == nil {
			t.Fatal("expected DELETE on journal_lines to be rejected, got nil error")
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Errorf("expected append-only abort message, got: %v", err)
		}
	})

	t.Run("journal_entries UPDATE rejected", func(t *testing.T) {
		_, err := db.Exec(`UPDATE journal_entries SET memo = 'changed' WHERE id = ?`, entryID)
		if err == nil {
			t.Fatal("expected UPDATE on journal_entries to be rejected, got nil error")
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Errorf("expected append-only abort message, got: %v", err)
		}
	})

	t.Run("journal_entries DELETE rejected", func(t *testing.T) {
		_, err := db.Exec(`DELETE FROM journal_entries WHERE id = ?`, entryID)
		if err == nil {
			t.Fatal("expected DELETE on journal_entries to be rejected, got nil error")
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Errorf("expected append-only abort message, got: %v", err)
		}
	})
}

func TestFundDimension(t *testing.T) {
	db := newTestDB(t)
	accountID, fundID := seedAccountAndFund(t, db)

	entryID, err := insertBareEntry(t, db)
	if err != nil {
		t.Fatalf("insertBareEntry: %v", err)
	}

	t.Run("NULL fund_id rejected", func(t *testing.T) {
		_, err := db.Exec(
			`INSERT INTO journal_lines (entry_id, account_id, fund_id, debit_amount, credit_amount)
			 VALUES (?, ?, NULL, ?, ?)`,
			entryID, accountID, 1000, 0,
		)
		if err == nil {
			t.Fatal("expected NULL fund_id to be rejected, got nil error")
		}
	})

	t.Run("invalid fund_id rejected", func(t *testing.T) {
		_, err := db.Exec(
			`INSERT INTO journal_lines (entry_id, account_id, fund_id, debit_amount, credit_amount)
			 VALUES (?, ?, ?, ?, ?)`,
			entryID, accountID, 999999, 1000, 0,
		)
		if err == nil {
			t.Fatal("expected invalid fund_id (no matching funds row) to be rejected, got nil error")
		}
	})

	t.Run("valid fund_id succeeds", func(t *testing.T) {
		_, err := db.Exec(
			`INSERT INTO journal_lines (entry_id, account_id, fund_id, debit_amount, credit_amount)
			 VALUES (?, ?, ?, ?, ?)`,
			entryID, accountID, fundID, 1000, 0,
		)
		if err != nil {
			t.Fatalf("expected valid fund_id insert to succeed, got error: %v", err)
		}
	})
}

func TestJournalLineDebitCreditExclusive(t *testing.T) {
	db := newTestDB(t)
	accountID, fundID := seedAccountAndFund(t, db)
	entryID, err := insertBareEntry(t, db)
	if err != nil {
		t.Fatalf("insertBareEntry: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO journal_lines (entry_id, account_id, fund_id, debit_amount, credit_amount)
		 VALUES (?, ?, ?, ?, ?)`,
		entryID, accountID, fundID, 500, 500,
	)
	if err == nil {
		t.Fatal("expected a line with both debit_amount>0 and credit_amount>0 to be rejected")
	}
}

// insertBareEntry inserts a journal_entries row with no lines, for tests
// that only need a valid entry_id to attach lines to.
func insertBareEntry(t *testing.T, db *sql.DB) (int64, error) {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO journal_entries (entry_date, memo, posted_by, posted_at, source)
		 VALUES (?, ?, ?, ?, ?)`,
		"2026-08-08", "bare entry", 1, "2026-08-08T00:00:00Z", "manual",
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
