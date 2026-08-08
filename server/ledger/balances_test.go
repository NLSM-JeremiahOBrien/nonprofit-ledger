package ledger

import (
	"database/sql"
	"testing"
)

// balancesFixture seeds two funds and a small chart of accounts shared
// across balance-reconciliation tests.
type balancesFixture struct {
	CashAccountID    int64
	RevenueAccountID int64
	FundAID          int64
	FundBID          int64
}

func seedBalancesFixture(t *testing.T, db *sql.DB) balancesFixture {
	t.Helper()

	cashID, err := CreateAccount(db, NewAccount{Code: "1000", Name: "Cash", Type: "asset"})
	if err != nil {
		t.Fatalf("seedBalancesFixture: create cash account: %v", err)
	}

	revenueID, err := CreateAccount(db, NewAccount{Code: "4000", Name: "Contributions", Type: "revenue"})
	if err != nil {
		t.Fatalf("seedBalancesFixture: create revenue account: %v", err)
	}

	fundAID, err := CreateFund(db, NewFund{Code: "GEN", Name: "General Fund", NetAssetClass: "without_donor_restrictions"})
	if err != nil {
		t.Fatalf("seedBalancesFixture: create fund A: %v", err)
	}

	fundBID, err := CreateFund(db, NewFund{Code: "REST", Name: "Restricted Fund", NetAssetClass: "with_donor_restrictions"})
	if err != nil {
		t.Fatalf("seedBalancesFixture: create fund B: %v", err)
	}

	return balancesFixture{
		CashAccountID:    cashID,
		RevenueAccountID: revenueID,
		FundAID:          fundAID,
		FundBID:          fundBID,
	}
}

func TestFundBalanceReconciliation(t *testing.T) {
	db := newTestDB(t)
	fx := seedBalancesFixture(t, db)

	t.Run("zero postings returns zero, not an error", func(t *testing.T) {
		bal, err := FundBalance(db, fx.FundAID)
		if err != nil {
			t.Fatalf("FundBalance: unexpected error: %v", err)
		}
		if bal != 0 {
			t.Fatalf("FundBalance: expected 0 for fund with no postings, got %d", bal)
		}
	})

	t.Run("balances reconcile across balanced postings touching both funds", func(t *testing.T) {
		// Entry 1: $500 contribution to Fund A (cash debit, revenue credit).
		_, err := PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-01",
			Memo:      "donation to general fund",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.CashAccountID, FundID: fx.FundAID, DebitAmount: 50000, CreditAmount: 0},
				{AccountID: fx.RevenueAccountID, FundID: fx.FundAID, DebitAmount: 0, CreditAmount: 50000},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry (entry 1): unexpected error: %v", err)
		}

		// Entry 2: $200 contribution to Fund B (cash debit, revenue credit).
		_, err = PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-02",
			Memo:      "donation to restricted fund",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.CashAccountID, FundID: fx.FundBID, DebitAmount: 20000, CreditAmount: 0},
				{AccountID: fx.RevenueAccountID, FundID: fx.FundBID, DebitAmount: 0, CreditAmount: 20000},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry (entry 2): unexpected error: %v", err)
		}

		// Entry 3: a single entry with lines touching both funds — $75
		// transferred in substance via cash debit against Fund A and a
		// revenue credit against Fund B (still balanced overall).
		_, err = PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-03",
			Memo:      "cross-fund balanced entry",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.CashAccountID, FundID: fx.FundAID, DebitAmount: 7500, CreditAmount: 0},
				{AccountID: fx.RevenueAccountID, FundID: fx.FundBID, DebitAmount: 0, CreditAmount: 7500},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry (entry 3): unexpected error: %v", err)
		}

		// Manually computed expected balances (debit - credit):
		// Fund A: cash debits (50000 + 7500) - revenue credits (50000) = 7500
		// Fund B: cash debits (20000) - revenue credits (20000 + 7500) = -7500
		wantFundA := int64(7500)
		wantFundB := int64(-7500)

		gotFundA, err := FundBalance(db, fx.FundAID)
		if err != nil {
			t.Fatalf("FundBalance(FundA): unexpected error: %v", err)
		}
		if gotFundA != wantFundA {
			t.Fatalf("FundBalance(FundA) = %d, want %d", gotFundA, wantFundA)
		}

		gotFundB, err := FundBalance(db, fx.FundBID)
		if err != nil {
			t.Fatalf("FundBalance(FundB): unexpected error: %v", err)
		}
		if gotFundB != wantFundB {
			t.Fatalf("FundBalance(FundB) = %d, want %d", gotFundB, wantFundB)
		}

		rows, err := TrialBalance(db)
		if err != nil {
			t.Fatalf("TrialBalance: unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("TrialBalance: expected 2 rows (one per fund with postings), got %d", len(rows))
		}

		var total int64
		byFund := map[int64]int64{}
		for _, r := range rows {
			byFund[r.FundID] = r.Balance
			total += r.Balance
		}
		if byFund[fx.FundAID] != wantFundA {
			t.Fatalf("TrialBalance fund A = %d, want %d", byFund[fx.FundAID], wantFundA)
		}
		if byFund[fx.FundBID] != wantFundB {
			t.Fatalf("TrialBalance fund B = %d, want %d", byFund[fx.FundBID], wantFundB)
		}
		if total != 0 {
			t.Fatalf("TrialBalance: sum of all fund balances = %d, want 0 (GL must balance)", total)
		}
	})

	t.Run("reversal restores pre-posting balance", func(t *testing.T) {
		db := newTestDB(t)
		fx := seedBalancesFixture(t, db)

		before, err := FundBalance(db, fx.FundAID)
		if err != nil {
			t.Fatalf("FundBalance (before): unexpected error: %v", err)
		}

		entryID, err := PostJournalEntry(db, NewEntry{
			EntryDate: "2026-07-01",
			Memo:      "entry to be reversed",
			PostedBy:  1,
			Lines: []NewLine{
				{AccountID: fx.CashAccountID, FundID: fx.FundAID, DebitAmount: 30000, CreditAmount: 0},
				{AccountID: fx.RevenueAccountID, FundID: fx.FundAID, DebitAmount: 0, CreditAmount: 30000},
			},
		})
		if err != nil {
			t.Fatalf("PostJournalEntry: unexpected error: %v", err)
		}

		afterPost, err := FundBalance(db, fx.FundAID)
		if err != nil {
			t.Fatalf("FundBalance (after post): unexpected error: %v", err)
		}
		if afterPost == before {
			t.Fatalf("FundBalance did not change after posting: still %d", afterPost)
		}

		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("db.Begin: unexpected error: %v", err)
		}
		if _, err := ReverseJournalEntry(tx, entryID, "test reversal", 1); err != nil {
			tx.Rollback()
			t.Fatalf("ReverseJournalEntry: unexpected error: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit: unexpected error: %v", err)
		}

		afterReverse, err := FundBalance(db, fx.FundAID)
		if err != nil {
			t.Fatalf("FundBalance (after reverse): unexpected error: %v", err)
		}
		if afterReverse != before {
			t.Fatalf("FundBalance after reversal = %d, want pre-posting value %d", afterReverse, before)
		}
	})
}
