package ledger

import (
	"database/sql"
	"testing"
)

// insertBalancedEntry inserts a posted journal entry with two balanced
// lines (one debit, one credit) directly via raw SQL, returning the
// entry ID and the two line IDs (debit line, credit line).
func insertBalancedEntry(t *testing.T, db *sql.DB, debitAccountID, creditAccountID, fundID int64) (entryID, debitLineID, creditLineID int64) {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO journal_entries (entry_date, memo, posted_by, posted_at, source)
		 VALUES (?, ?, ?, ?, ?)`,
		"2026-08-08", "original entry", 1, "2026-08-08T00:00:00Z", "manual",
	)
	if err != nil {
		t.Fatalf("insertBalancedEntry: insert entry: %v", err)
	}
	entryID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("insertBalancedEntry: entry id: %v", err)
	}

	res, err = db.Exec(
		`INSERT INTO journal_lines (entry_id, account_id, fund_id, functional_category, debit_amount, credit_amount)
		 VALUES (?, ?, ?, NULL, ?, ?)`,
		entryID, debitAccountID, fundID, 1000, 0,
	)
	if err != nil {
		t.Fatalf("insertBalancedEntry: insert debit line: %v", err)
	}
	debitLineID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("insertBalancedEntry: debit line id: %v", err)
	}

	res, err = db.Exec(
		`INSERT INTO journal_lines (entry_id, account_id, fund_id, functional_category, debit_amount, credit_amount)
		 VALUES (?, ?, ?, NULL, ?, ?)`,
		entryID, creditAccountID, fundID, 0, 1000,
	)
	if err != nil {
		t.Fatalf("insertBalancedEntry: insert credit line: %v", err)
	}
	creditLineID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("insertBalancedEntry: credit line id: %v", err)
	}

	return entryID, debitLineID, creditLineID
}

type journalLineRow struct {
	AccountID           int64
	FundID              int64
	FunctionalCategory  sql.NullString
	DebitAmount         int64
	CreditAmount        int64
}

func fetchLines(t *testing.T, db *sql.DB, entryID int64) []journalLineRow {
	t.Helper()

	rows, err := db.Query(
		`SELECT account_id, fund_id, functional_category, debit_amount, credit_amount
		 FROM journal_lines WHERE entry_id = ? ORDER BY id`,
		entryID,
	)
	if err != nil {
		t.Fatalf("fetchLines: query: %v", err)
	}
	defer rows.Close()

	var result []journalLineRow
	for rows.Next() {
		var l journalLineRow
		if err := rows.Scan(&l.AccountID, &l.FundID, &l.FunctionalCategory, &l.DebitAmount, &l.CreditAmount); err != nil {
			t.Fatalf("fetchLines: scan: %v", err)
		}
		result = append(result, l)
	}
	return result
}

func TestReverseJournalEntry(t *testing.T) {
	db := newTestDB(t)
	cashID, err := CreateAccount(db, NewAccount{Code: "1000", Name: "Cash", Type: "asset"})
	if err != nil {
		t.Fatalf("CreateAccount cash: %v", err)
	}
	revenueID, err := CreateAccount(db, NewAccount{Code: "4000", Name: "Contributions", Type: "revenue"})
	if err != nil {
		t.Fatalf("CreateAccount revenue: %v", err)
	}
	fundID, err := CreateFund(db, NewFund{Code: "GEN", Name: "General Fund", NetAssetClass: "without_donor_restrictions"})
	if err != nil {
		t.Fatalf("CreateFund: %v", err)
	}

	entryID, _, _ := insertBalancedEntry(t, db, cashID, revenueID, fundID)
	originalLinesBefore := fetchLines(t, db, entryID)

	t.Run("creates linked reversal entry", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("db.Begin: %v", err)
		}
		defer tx.Rollback()

		newID, err := ReverseJournalEntry(tx, entryID, "correction test", 1)
		if err != nil {
			t.Fatalf("ReverseJournalEntry: %v", err)
		}
		if newID == entryID {
			t.Fatal("expected a new entry ID distinct from the original")
		}

		var reversesEntryID sql.NullInt64
		var source string
		err = tx.QueryRow(`SELECT reverses_entry_id, source FROM journal_entries WHERE id = ?`, newID).
			Scan(&reversesEntryID, &source)
		if err != nil {
			t.Fatalf("querying new entry: %v", err)
		}
		if !reversesEntryID.Valid || reversesEntryID.Int64 != entryID {
			t.Errorf("expected reverses_entry_id=%d, got %+v", entryID, reversesEntryID)
		}
		if source != "reversal" {
			t.Errorf("expected source=reversal, got %q", source)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit: %v", err)
		}
	})

	t.Run("reversal lines are swapped copies of original", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("db.Begin: %v", err)
		}
		defer tx.Rollback()

		newID, err := ReverseJournalEntry(tx, entryID, "correction test 2", 1)
		if err != nil {
			t.Fatalf("ReverseJournalEntry: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit: %v", err)
		}

		originalLines := fetchLines(t, db, entryID)
		reversalLines := fetchLines(t, db, newID)

		if len(reversalLines) != len(originalLines) {
			t.Fatalf("expected %d reversal lines, got %d", len(originalLines), len(reversalLines))
		}

		for i, orig := range originalLines {
			rev := reversalLines[i]
			if rev.AccountID != orig.AccountID {
				t.Errorf("line %d: account_id mismatch: orig=%d rev=%d", i, orig.AccountID, rev.AccountID)
			}
			if rev.FundID != orig.FundID {
				t.Errorf("line %d: fund_id mismatch: orig=%d rev=%d", i, orig.FundID, rev.FundID)
			}
			if rev.DebitAmount != orig.CreditAmount {
				t.Errorf("line %d: expected reversal debit=%d (orig credit), got %d", i, orig.CreditAmount, rev.DebitAmount)
			}
			if rev.CreditAmount != orig.DebitAmount {
				t.Errorf("line %d: expected reversal credit=%d (orig debit), got %d", i, orig.DebitAmount, rev.CreditAmount)
			}
		}
	})

	t.Run("original entry unchanged after reversal", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("db.Begin: %v", err)
		}
		defer tx.Rollback()

		_, err = ReverseJournalEntry(tx, entryID, "correction test 3", 1)
		if err != nil {
			t.Fatalf("ReverseJournalEntry: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit: %v", err)
		}

		originalLinesAfter := fetchLines(t, db, entryID)
		if len(originalLinesAfter) != len(originalLinesBefore) {
			t.Fatalf("original line count changed: before=%d after=%d", len(originalLinesBefore), len(originalLinesAfter))
		}
		for i, before := range originalLinesBefore {
			after := originalLinesAfter[i]
			if before != after {
				t.Errorf("original line %d changed: before=%+v after=%+v", i, before, after)
			}
		}
	})

	t.Run("non-existent original ID returns error and inserts nothing", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("db.Begin: %v", err)
		}
		defer tx.Rollback()

		var countBefore int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM journal_entries`).Scan(&countBefore); err != nil {
			t.Fatalf("count before: %v", err)
		}

		_, err = ReverseJournalEntry(tx, 999999, "should fail", 1)
		if err == nil {
			t.Fatal("expected error for non-existent originalID, got nil")
		}

		var countAfter int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM journal_entries`).Scan(&countAfter); err != nil {
			t.Fatalf("count after: %v", err)
		}
		if countAfter != countBefore {
			t.Errorf("expected no rows inserted on error, before=%d after=%d", countBefore, countAfter)
		}
	})
}
