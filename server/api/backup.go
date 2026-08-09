package api

import (
	"database/sql"
	"net/http"
	"os"
)

// BackupHandlers bundles the dependencies BackupStatusHandler needs: a
// database connection (for the backup_runs health table) and the local
// backup directory path (for counting retained snapshots).
type BackupHandlers struct {
	DB        *sql.DB
	BackupDir string
}

// backupStatusResponse is the JSON shape returned by
// GET /api/backup/status.
type backupStatusResponse struct {
	LastSuccessAt         *string `json:"last_success_at"`
	LastStatus            *string `json:"last_status"`
	LastError             *string `json:"last_error,omitempty"`
	RetainedSnapshotCount int     `json:"retained_snapshot_count"`
}

// BackupStatusHandler handles GET /api/backup/status, reporting backup
// health: the most recent successful run's timestamp, the most recent
// run's status (success or failed, regardless of which), and how many
// snapshot files are currently retained on disk. Admin-only (enforced by
// route wiring in RegisterRoutes, never here).
func (h *BackupHandlers) BackupStatusHandler(w http.ResponseWriter, r *http.Request) {
	resp := backupStatusResponse{}

	var lastStatus sql.NullString
	var lastError sql.NullString
	err := h.DB.QueryRow(
		`SELECT status, error FROM backup_runs ORDER BY id DESC LIMIT 1`,
	).Scan(&lastStatus, &lastError)
	switch {
	case err == sql.ErrNoRows:
		// No backup has run yet — respond with zero-value status rather
		// than an error.
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	default:
		if lastStatus.Valid {
			s := lastStatus.String
			resp.LastStatus = &s
		}
		if lastError.Valid && lastError.String != "" {
			e := lastError.String
			resp.LastError = &e
		}
	}

	var lastSuccessAt sql.NullString
	err = h.DB.QueryRow(
		`SELECT started_at FROM backup_runs WHERE status = 'success' ORDER BY id DESC LIMIT 1`,
	).Scan(&lastSuccessAt)
	if err != nil && err != sql.ErrNoRows {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if lastSuccessAt.Valid {
		s := lastSuccessAt.String
		resp.LastSuccessAt = &s
	}

	entries, err := os.ReadDir(h.BackupDir)
	if err != nil {
		if !os.IsNotExist(err) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		// Backup directory not created yet — zero retained snapshots.
	} else {
		count := 0
		for _, e := range entries {
			if !e.IsDir() {
				count++
			}
		}
		resp.RetainedSnapshotCount = count
	}

	writeJSON(w, http.StatusOK, resp)
}
