package backup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// Restore copies the snapshot at snapshotPath to destPath, opens it, and
// runs PRAGMA integrity_check against it. It returns the opened *sql.DB
// on success so the caller can run further verification against it (for
// example, ledger.TrialBalance) — this package deliberately does not
// duplicate any ledger-domain logic itself.
//
// snapshotPath is left untouched; destPath receives an independent copy,
// so a caller can restore into a fresh location without risking the
// original snapshot file.
func Restore(snapshotPath, destPath string) (*sql.DB, error) {
	if err := copyFile(snapshotPath, destPath); err != nil {
		return nil, fmt.Errorf("backup: restore: copying snapshot: %w", err)
	}

	conn, err := db.Open(destPath)
	if err != nil {
		return nil, fmt.Errorf("backup: restore: opening restored database: %w", err)
	}

	var result string
	if err := conn.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		conn.Close()
		return nil, fmt.Errorf("backup: restore: running integrity_check: %w", err)
	}
	if result != "ok" {
		conn.Close()
		return nil, fmt.Errorf("backup: restore: integrity_check failed: %s", result)
	}

	return conn, nil
}

// copyFile copies the file at src to dst, creating any necessary parent
// directories.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying: %w", err)
	}

	return out.Close()
}
