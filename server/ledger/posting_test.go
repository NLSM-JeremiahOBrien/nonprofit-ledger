package ledger

import (
	"database/sql"
	"testing"
)

// postingFixture seeds one asset account, one expense account, and one
// fund, returning their IDs for use in posting tests.
type postingFixture struct {
	AssetAccountID   int64
	ExpenseAccountID int64
	FundID           int64
}

// seedFixture creates one asset account, one expense account, and one
// fund shared across posting tests.
func seedFixture(t *testing.T, db *sql.DB) postingFixture {
	t.Helper()

	assetID, err := CreateAccount(db, NewAccount{Code: "1000", Name: "Cash", Type: "asset"})
	if err != nil {
		t.Fatalf("seedFixture: create asset account: %v", err)
	}

	expenseID, err := CreateAccount(db, NewAccount{Code: "5000", Name: "Program Expense", Type: "expense"})
	if err != nil {
		t.Fatalf("seedFixture: create expense account: %v", err)
	}

	fundID, err := CreateFund(db, NewFund{Code: "GEN", Name: "General Fund", NetAssetClass: "without_donor_restrictions"})
	if err != nil {
		t.Fatalf("seedFixture: create fund: %v", err)
	}

	return postingFixture{
		AssetAccountID:   assetID,
		ExpenseAccountID: expenseID,
		FundID:           fundID,
	}
}

// assertNoJournalRows fails the test if any journal_entries or
// journal_lines rows exist, used to confirm rejected postings never
// partially write.
func assertNoJournalRows(t *testing.T, db *sql.DB) {
	t.Helper()

	var entryCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM journal_entries`).Scan(&entryCount); err != nil {
		t.Fatalf("assertNoJournalRows: counting journal_entries: %v", err)
	}
	if entryCount != 0 {
		t.Fatalf("assertNoJournalRows: expected 0 journal_entries rows, got %d", entryCount)
	}

	var lineCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM journal_lines`).Scan(&lineCount); err != nil {
		t.Fatalf("assertNoJournalRows: counting journal_lines: %v", err)
	}
	if lineCount != 0 {
		t.Fatalf("assertNoJournalRows: expected 0 journal_lines rows, got %d", lineCount)
	}
}

func TestPostJournalEntry_RejectsUnbalanced(t *testing.T) {
	db := newTestDB(t)
	fx := seedFixture(t, db)

	_, err := PostJournalEntry(db, NewEntry{
		EntryDate: "2026-07-01",
		Memo:      "unbalanced",
		PostedBy:  1,
		Lines: []NewLine{
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 50},
		},
	})
	if err == nil {
		t.Fatal("expected error for unbalanced entry, got nil")
	}

	assertNoJournalRows(t, db)
}

func TestPostJournalEntry_RejectsZeroLines(t *testing.T) {
	db := newTestDB(t)

	_, err := PostJournalEntry(db, NewEntry{
		EntryDate: "2026-07-01",
		Memo:      "no lines",
		PostedBy:  1,
		Lines:     []NewLine{},
	})
	if err == nil {
		t.Fatal("expected error for zero-line entry, got nil")
	}
}

func TestPostJournalEntry_PostsBalancedEntry(t *testing.T) {
	db := newTestDB(t)
	fx := seedFixture(t, db)

	id, err := PostJournalEntry(db, NewEntry{
		EntryDate: "2026-07-01",
		Memo:      "balanced",
		PostedBy:  1,
		Lines: []NewLine{
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
		},
	})
	if err != nil {
		t.Fatalf("PostJournalEntry: unexpected error: %v", err)
	}
	if id == 0 {
		t.Fatal("PostJournalEntry: expected non-zero entry id")
	}

	var entryCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM journal_entries WHERE id = ?`, id).Scan(&entryCount); err != nil {
		t.Fatalf("counting journal_entries: %v", err)
	}
	if entryCount != 1 {
		t.Fatalf("expected 1 journal_entries row, got %d", entryCount)
	}

	var lineCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM journal_lines WHERE entry_id = ?`, id).Scan(&lineCount); err != nil {
		t.Fatalf("counting journal_lines: %v", err)
	}
	if lineCount != 2 {
		t.Fatalf("expected 2 journal_lines rows, got %d", lineCount)
	}
}

func TestPeriodLock_PostingRejectsLockedDates(t *testing.T) {
	db := newTestDB(t)
	fx := seedFixture(t, db)

	if err := LockPeriod(db, "2026-06-30", 1); err != nil {
		t.Fatalf("LockPeriod: unexpected error: %v", err)
	}

	_, err := PostJournalEntry(db, NewEntry{
		EntryDate: "2026-06-15",
		Memo:      "before lock",
		PostedBy:  1,
		Lines: []NewLine{
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
		},
	})
	if err == nil {
		t.Fatal("expected error for entry dated on/before locked-through date, got nil")
	}

	_, err = PostJournalEntry(db, NewEntry{
		EntryDate: "2026-07-01",
		Memo:      "after lock",
		PostedBy:  1,
		Lines: []NewLine{
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
			{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
		},
	})
	if err != nil {
		t.Fatalf("PostJournalEntry: expected success for entry after lock, got error: %v", err)
	}
}

func TestFunctionalExpenseRequired(t *testing.T) {
	db := newTestDB(t)
	fx := seedFixture(t, db)

	t.Run("expense line missing functional category is rejected", func(t *testing.T) {
		_, err := PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-01",
			Memo:      "expense no category",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.ExpenseAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
				{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
			},
		})
		if err == nil {
			t.Fatal("expected error for expense line missing functional category, got nil")
		}
	})

	t.Run("expense line with functional category succeeds", func(t *testing.T) {
		category := "program"
		_, err := PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-01",
			Memo:      "expense with category",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.ExpenseAccountID, FundID: fx.FundID, FunctionalCategory: &category, DebitAmount: 100, CreditAmount: 0},
				{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry: unexpected error: %v", err)
		}
	})

	t.Run("non-expense line without functional category succeeds", func(t *testing.T) {
		_, err := PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-01",
			Memo:      "asset only",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 100, CreditAmount: 0},
				{AccountID: fx.AssetAccountID, FundID: fx.FundID, DebitAmount: 0, CreditAmount: 100},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry: unexpected error: %v", err)
		}
	})
}
