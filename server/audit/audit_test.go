package audit_test

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/audit"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
	"github.com/tjcrowley/nonprofit-ledger/server/ledger"
)

// newTestDB creates a fresh on-disk SQLite database in a temp directory,
// runs all migrations against it, and returns the resulting *sql.DB. A
// real file (not ":memory:") is used deliberately, mirroring
// server/ledger/testutil_test.go, since the immutability triggers under
// test need durable on-disk behavior consistent with production.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("newTestDB: open: %v", err)
	}

	if err := db.RunMigrations(conn, migrationsDir(t)); err != nil {
		conn.Close()
		t.Fatalf("newTestDB: run migrations: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// migrationsDir resolves the absolute path to server/db/migrations
// relative to this source file, regardless of the working directory the
// test binary is invoked from.
func migrationsDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migrationsDir: unable to determine caller")
	}

	// this file lives at server/audit/audit_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "db", "migrations")
}

// seedUser inserts one users row and returns its ID, satisfying the
// audit_log.actor_user_id foreign key.
func seedUser(t *testing.T, conn *sql.DB) int64 {
	t.Helper()

	res, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		"tester", "not-a-real-hash", "admin",
	)
	if err != nil {
		t.Fatalf("seedUser: insert: %v", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seedUser: id: %v", err)
	}

	return id
}

func TestAuditImmutable(t *testing.T) {
	conn := newTestDB(t)
	userID := seedUser(t, conn)

	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("db.Begin: %v", err)
	}

	if err := audit.Write(tx, userID, "create", "journal_entry", 1, "test detail"); err != nil {
		t.Fatalf("audit.Write: unexpected error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit: %v", err)
	}

	var id int64
	if err := conn.QueryRow(`SELECT id FROM audit_log LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("querying inserted row: %v", err)
	}

	if _, err := conn.Exec(`UPDATE audit_log SET action = 'tampered' WHERE id = ?`, id); err == nil {
		t.Fatal("expected UPDATE on audit_log to fail, got nil error")
	}

	if _, err := conn.Exec(`DELETE FROM audit_log WHERE id = ?`, id); err == nil {
		t.Fatal("expected DELETE on audit_log to fail, got nil error")
	}

	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count); err != nil {
		t.Fatalf("counting audit_log rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 audit_log row to survive rejected UPDATE/DELETE, got %d", count)
	}
}

// assertAuditRow fails the test unless exactly one audit_log row matches
// the given action/entity/actor combination.
func assertAuditRow(t *testing.T, conn *sql.DB, action, entityType string, entityID, actorUserID int64) {
	t.Helper()

	var count int
	err := conn.QueryRow(
		`SELECT COUNT(*) FROM audit_log
		 WHERE action = ? AND entity_type = ? AND entity_id = ? AND actor_user_id = ?`,
		action, entityType, entityID, actorUserID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("assertAuditRow: query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 audit_log row for action=%s entity_type=%s entity_id=%d actor_user_id=%d, got %d",
			action, entityType, entityID, actorUserID, count)
	}
}

func TestAuditAttribution(t *testing.T) {
	conn := newTestDB(t)
	userID := seedUser(t, conn)

	assetID, err := ledger.CreateAccount(conn, ledger.NewAccount{Code: "1000", Name: "Cash", Type: "asset"})
	if err != nil {
		t.Fatalf("CreateAccount cash: %v", err)
	}
	revenueID, err := ledger.CreateAccount(conn, ledger.NewAccount{Code: "4000", Name: "Contributions", Type: "revenue"})
	if err != nil {
		t.Fatalf("CreateAccount revenue: %v", err)
	}
	fundID, err := ledger.CreateFund(conn, ledger.NewFund{Code: "GEN", Name: "General Fund", NetAssetClass: "without_donor_restrictions"})
	if err != nil {
		t.Fatalf("CreateFund: %v", err)
	}

	entryID, err := ledger.PostJournalEntry(conn, ledger.NewEntry{
		EntryDate: "2026-08-01",
		Memo:      "test entry",
		PostedBy:  userID,
		Lines: []ledger.NewLine{
			{AccountID: assetID, FundID: fundID, DebitAmount: 1000},
			{AccountID: revenueID, FundID: fundID, CreditAmount: 1000},
		},
	})
	if err != nil {
		t.Fatalf("PostJournalEntry: %v", err)
	}
	assertAuditRow(t, conn, "create", "journal_entry", entryID, userID)

	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("db.Begin: %v", err)
	}
	reversalID, err := ledger.ReverseJournalEntry(tx, entryID, "correction", userID)
	if err != nil {
		t.Fatalf("ReverseJournalEntry: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit: %v", err)
	}
	assertAuditRow(t, conn, "reversal", "journal_entry", reversalID, userID)

	if err := ledger.LockPeriod(conn, "2026-07-31", userID); err != nil {
		t.Fatalf("LockPeriod: %v", err)
	}
	assertAuditRow(t, conn, "lock_period", "accounting_period", 1, userID)
}
