package ledger

import (
	"database/sql"
	"fmt"
)

// FundBalanceRow is one fund's derived balance, as returned by
// TrialBalance.
type FundBalanceRow struct {
	FundID  int64
	Balance int64 // integer cents; positive/negative per normal debit-credit convention
}

// FundBalance returns the derived balance for fundID, computed as
// SUM(debit_amount) - SUM(credit_amount) across every journal_lines row
// for that fund. It is never a stored/mutable value — recomputed fresh
// from journal_lines on every call (Anti-Pattern 3: no balance column).
// A fund with no postings returns 0, not an error.
func FundBalance(db *sql.DB, fundID int64) (int64, error) {
	var balance int64
	err := db.QueryRow(
		`SELECT COALESCE(SUM(debit_amount) - SUM(credit_amount), 0)
		 FROM journal_lines WHERE fund_id = ?`,
		fundID,
	).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("ledger: fund balance for fund %d: %w", fundID, err)
	}
	return balance, nil
}

// TrialBalance returns the derived balance for every fund that has at
// least one journal_lines posting, computed as
// SUM(debit_amount) - SUM(credit_amount) GROUP BY fund_id. Like
// FundBalance, this is always derived directly from journal_lines, never
// a stored/mutable projection. The sum of every returned balance must
// equal zero whenever the underlying journal is balanced (every posted
// entry itself balanced) — this is the GL trial-balance invariant.
func TrialBalance(db *sql.DB) ([]FundBalanceRow, error) {
	rows, err := db.Query(
		`SELECT fund_id, SUM(debit_amount) - SUM(credit_amount) AS balance
		 FROM journal_lines
		 GROUP BY fund_id
		 ORDER BY fund_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("ledger: trial balance: %w", err)
	}
	defer rows.Close()

	var result []FundBalanceRow
	for rows.Next() {
		var r FundBalanceRow
		if err := rows.Scan(&r.FundID, &r.Balance); err != nil {
			return nil, fmt.Errorf("ledger: trial balance: scanning row: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: trial balance: iterating rows: %w", err)
	}

	return result, nil
}
