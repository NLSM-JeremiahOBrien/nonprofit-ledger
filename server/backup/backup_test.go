package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/db"
	"github.com/tjcrowley/nonprofit-ledger/server/ledger"
)

// TestSnapshot verifies that Snapshot, backed by VACUUM INTO, captures
// committed writes even when they are still resident only in the WAL
// (not yet checkpointed into the main database file).
func TestSnapshot(t *testing.T) {
	conn, _ := newTestDB(t)

	if _, err := conn.Exec(
		`INSERT INTO accounts (code, name, type) VALUES ('1000', 'Cash', 'asset')`,
	); err != nil {
		t.Fatalf("seeding account: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "snapshot.db")
	if err := Snapshot(conn, destPath); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected snapshot file to exist: %v", err)
	}

	snapConn, err := db.Open(destPath)
	if err != nil {
		t.Fatalf("opening snapshot: %v", err)
	}
	defer snapConn.Close()

	var name string
	if err := snapConn.QueryRow(`SELECT name FROM accounts WHERE code = '1000'`).Scan(&name); err != nil {
		t.Fatalf("expected seeded account to be present in snapshot: %v", err)
	}
	if name != "Cash" {
		t.Fatalf("expected name 'Cash', got %q", name)
	}
}

// TestRotation verifies that Rotate deletes the oldest snapshot files
// beyond the retain count, keeping exactly the N most recent.
func TestRotation(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"ledger-20260101-000000.db",
		"ledger-20260102-000000.db",
		"ledger-20260103-000000.db",
		"ledger-20260104-000000.db",
		"ledger-20260105-000000.db",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("writing fixture %s: %v", n, err)
		}
	}

	const keepN = 3
	if err := Rotate(dir, keepN); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != keepN {
		t.Fatalf("expected %d files retained, got %d", keepN, len(entries))
	}

	remaining := make(map[string]bool, len(entries))
	for _, e := range entries {
		remaining[e.Name()] = true
	}
	for _, want := range names[len(names)-keepN:] {
		if !remaining[want] {
			t.Fatalf("expected %s to be retained, but it was deleted", want)
		}
	}
	for _, gone := range names[:len(names)-keepN] {
		if remaining[gone] {
			t.Fatalf("expected %s to be deleted, but it was retained", gone)
		}
	}
}

// TestRunScheduledRecordsHealth verifies RunScheduled records a
// backup_runs row on success with accurate file_path/size_bytes.
func TestRunScheduledRecordsHealth(t *testing.T) {
	conn, _ := newTestDB(t)
	backupDir := t.TempDir()

	if err := RunScheduled(conn, backupDir, 7); err != nil {
		t.Fatalf("RunScheduled: %v", err)
	}

	var status string
	var filePath sql.NullString
	var sizeBytes sql.NullInt64
	if err := conn.QueryRow(
		`SELECT status, file_path, size_bytes FROM backup_runs ORDER BY id DESC LIMIT 1`,
	).Scan(&status, &filePath, &sizeBytes); err != nil {
		t.Fatalf("querying backup_runs: %v", err)
	}

	if status != "success" {
		t.Fatalf("expected status 'success', got %q", status)
	}
	if !filePath.Valid || filePath.String == "" {
		t.Fatal("expected file_path to be set")
	}
	if !sizeBytes.Valid || sizeBytes.Int64 <= 0 {
		t.Fatal("expected size_bytes to be a positive number")
	}

	if _, err := os.Stat(filePath.String); err != nil {
		t.Fatalf("expected recorded file_path to exist on disk: %v", err)
	}
}

// TestRunScheduledRecordsFailure verifies RunScheduled still records a
// backup_runs row (status 'failed', error populated) when the snapshot
// destination is invalid.
func TestRunScheduledRecordsFailure(t *testing.T) {
	conn, _ := newTestDB(t)

	// A backup "directory" that is actually a regular file makes
	// os.MkdirAll fail, forcing Snapshot to fail deterministically.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing blocker file: %v", err)
	}
	badBackupDir := filepath.Join(blocker, "backups")

	if err := RunScheduled(conn, badBackupDir, 7); err == nil {
		t.Fatal("expected RunScheduled to return an error")
	}

	var status string
	var errMsg sql.NullString
	if err := conn.QueryRow(
		`SELECT status, error FROM backup_runs ORDER BY id DESC LIMIT 1`,
	).Scan(&status, &errMsg); err != nil {
		t.Fatalf("querying backup_runs: %v", err)
	}

	if status != "failed" {
		t.Fatalf("expected status 'failed', got %q", status)
	}
	if !errMsg.Valid || errMsg.String == "" {
		t.Fatal("expected error message to be recorded")
	}
}

// TestRestoreRoundTrip verifies that a snapshot taken of a DB with
// posted, balanced journal entries across multiple funds can be
// restored, passes PRAGMA integrity_check, and reproduces the exact
// same trial balance as the source database. This is the PLAT-03
// requirement that the restore path is proven, not just documented.
func TestRestoreRoundTrip(t *testing.T) {
	conn, _ := newTestDB(t)

	cashID, err := ledger.CreateAccount(conn, ledger.NewAccount{Code: "1000", Name: "Cash", Type: "asset"})
	if err != nil {
		t.Fatalf("CreateAccount(cash): %v", err)
	}
	revenueID, err := ledger.CreateAccount(conn, ledger.NewAccount{Code: "4000", Name: "Contributions", Type: "revenue"})
	if err != nil {
		t.Fatalf("CreateAccount(revenue): %v", err)
	}

	fundAID, err := ledger.CreateFund(conn, ledger.NewFund{Code: "GEN", Name: "General Fund", NetAssetClass: "without_donor_restrictions"})
	if err != nil {
		t.Fatalf("CreateFund(A): %v", err)
	}
	fundBID, err := ledger.CreateFund(conn, ledger.NewFund{Code: "REST", Name: "Restricted Fund", NetAssetClass: "with_donor_restrictions"})
	if err != nil {
		t.Fatalf("CreateFund(B): %v", err)
	}

	if _, err := ledger.PostJournalEntry(conn, ledger.NewEntry{
		EntryDate: "2026-07-01",
		Memo:      "donation to general fund",
		PostedBy:  1,
		Lines: []ledger.NewLine{
			{AccountID: cashID, FundID: fundAID, DebitAmount: 50000, CreditAmount: 0},
			{AccountID: revenueID, FundID: fundAID, DebitAmount: 0, CreditAmount: 50000},
		},
	}); err != nil {
		t.Fatalf("PostJournalEntry (fund A): %v", err)
	}

	if _, err := ledger.PostJournalEntry(conn, ledger.NewEntry{
		EntryDate: "2026-07-02",
		Memo:      "donation to restricted fund",
		PostedBy:  1,
		Lines: []ledger.NewLine{
			{AccountID: cashID, FundID: fundBID, DebitAmount: 20000, CreditAmount: 0},
			{AccountID: revenueID, FundID: fundBID, DebitAmount: 0, CreditAmount: 20000},
		},
	}); err != nil {
		t.Fatalf("PostJournalEntry (fund B): %v", err)
	}

	wantTrialBalance, err := ledger.TrialBalance(conn)
	if err != nil {
		t.Fatalf("TrialBalance (source): %v", err)
	}

	snapshotPath := filepath.Join(t.TempDir(), "snapshot.db")
	if err := Snapshot(conn, snapshotPath); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	restoredConn, err := Restore(snapshotPath, restoredPath)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	defer restoredConn.Close()

	gotTrialBalance, err := ledger.TrialBalance(restoredConn)
	if err != nil {
		t.Fatalf("TrialBalance (restored): %v", err)
	}

	if len(gotTrialBalance) != len(wantTrialBalance) {
		t.Fatalf("restored trial balance has %d rows, want %d", len(gotTrialBalance), len(wantTrialBalance))
	}
	for i := range wantTrialBalance {
		if gotTrialBalance[i] != wantTrialBalance[i] {
			t.Fatalf("restored trial balance row %d = %+v, want %+v", i, gotTrialBalance[i], wantTrialBalance[i])
		}
	}

	var total int64
	for _, row := range gotTrialBalance {
		total += row.Balance
	}
	if total != 0 {
		t.Fatalf("restored trial balance sums to %d, want 0", total)
	}
}
