// Package backup implements automated, verifiable local backups of the
// SQLite ledger database (PLAT-03). Backups are plain files on local
// disk — this package never constructs or accepts an s3://, https://, or
// any other remote destination anywhere in its API (PLAT-04).
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// snapshotPrefix and snapshotTimeFormat together produce filenames like
// "ledger-20260808-153000.db" — timestamp-prefixed so lexical sort order
// matches chronological order, which Rotate relies on.
const (
	snapshotPrefix     = "ledger-"
	snapshotTimeFormat = "20060102-150405"
	snapshotExt        = ".db"
)

// Snapshot writes a consistent point-in-time copy of db to destPath using
// SQLite's VACUUM INTO. VACUUM INTO reads through the WAL, so it always
// captures every committed write, including writes still resident only
// in the WAL and not yet checkpointed into the main database file. Never
// hand-copy the .db/.db-wal files instead of this — that approach can
// silently omit committed-but-not-checkpointed data.
func Snapshot(db *sql.DB, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("backup: snapshot: creating backup directory: %w", err)
	}

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("backup: snapshot: %w", err)
	}

	return nil
}

// Rotate lists snapshot files in dir (matching the "ledger-*.db" naming
// pattern produced by RunScheduled), sorts them by filename — which
// sorts chronologically given the timestamp-prefixed naming scheme —
// and deletes all but the keepN most recent. It is a no-op if there are
// keepN or fewer snapshot files present.
func Rotate(dir string, keepN int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("backup: rotate: reading directory %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if len(n) > len(snapshotPrefix) && n[:len(snapshotPrefix)] == snapshotPrefix {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	if len(names) <= keepN {
		return nil
	}

	toDelete := names[:len(names)-keepN]
	for _, n := range toDelete {
		if err := os.Remove(filepath.Join(dir, n)); err != nil {
			return fmt.Errorf("backup: rotate: removing %s: %w", n, err)
		}
	}

	return nil
}

// RunScheduled performs one full backup cycle: it snapshots db into
// backupDir with a timestamp-prefixed filename, rotates old snapshots
// down to keepN, and records the outcome — success or failure — as a
// row in backup_runs. It always records a row, even on failure, so
// backup health is never silently unobserved.
func RunScheduled(db *sql.DB, backupDir string, keepN int) error {
	startedAt := time.Now().UTC()
	destPath := filepath.Join(backupDir, snapshotPrefix+startedAt.Format(snapshotTimeFormat)+snapshotExt)

	snapErr := Snapshot(db, destPath)

	var rotateErr error
	if snapErr == nil {
		rotateErr = Rotate(backupDir, keepN)
	}

	finishedAt := time.Now().UTC()

	runErr := snapErr
	if runErr == nil {
		runErr = rotateErr
	}

	status := "success"
	var errMsg *string
	var sizeBytes *int64
	if runErr != nil {
		status = "failed"
		msg := runErr.Error()
		errMsg = &msg
	} else {
		if info, statErr := os.Stat(destPath); statErr == nil {
			size := info.Size()
			sizeBytes = &size
		}
	}

	var filePath *string
	if snapErr == nil {
		filePath = &destPath
	}

	if _, err := db.Exec(
		`INSERT INTO backup_runs (started_at, finished_at, status, file_path, size_bytes, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		startedAt.Format(time.RFC3339), finishedAt.Format(time.RFC3339), status, filePath, sizeBytes, errMsg,
	); err != nil {
		return fmt.Errorf("backup: run scheduled: recording backup_runs row: %w", err)
	}

	return runErr
}
