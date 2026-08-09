package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/backup"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// TestBackupStatus verifies GET /api/backup/status is admin-only
// (RequireRole("admin"), enforced by route wiring) and reports accurate
// backup health data for a role that is allowed through.
func TestBackupStatus(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "ledger-*.db")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	f.Close()

	conn, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn, "../db/migrations"); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups")
	if err := backup.RunScheduled(conn, backupDir, 7); err != nil {
		t.Fatalf("RunScheduled (seeding backup health): %v", err)
	}

	sm := auth.NewSessionManager(conn)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, sm, conn, backupDir)

	// Test-only login endpoint, mirroring the pattern used in
	// server/auth/external_accountant_scope_test.go: starts a session for
	// a given username without exercising the real password login flow.
	mux.HandleFunc("/test-login", func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		var userID int64
		if err := conn.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&userID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sm.Put(r.Context(), "userID", userID)
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(sm.LoadAndSave(mux))
	defer srv.Close()

	for _, role := range []string{"admin", "staff_bookkeeper", "external_accountant"} {
		hash, err := auth.HashPassword("password")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if _, err := conn.Exec(
			`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, ?, 1)`,
			role, hash, role,
		); err != nil {
			t.Fatalf("seed user %s: %v", role, err)
		}
	}

	loginAs := func(username string) *http.Client {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookiejar.New: %v", err)
		}
		client := &http.Client{Jar: jar}
		resp, err := client.Get(srv.URL + "/test-login?username=" + username)
		if err != nil {
			t.Fatalf("test-login: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("test-login for %s: expected 200, got %d", username, resp.StatusCode)
		}
		return client
	}

	t.Run("admin sees backup status", func(t *testing.T) {
		client := loginAs("admin")
		resp, err := client.Get(srv.URL + "/api/backup/status")
		if err != nil {
			t.Fatalf("GET /api/backup/status: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var body struct {
			LastSuccessAt         *string `json:"last_success_at"`
			LastStatus            *string `json:"last_status"`
			RetainedSnapshotCount int     `json:"retained_snapshot_count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if body.LastStatus == nil || *body.LastStatus != "success" {
			t.Fatalf("expected last_status 'success', got %v", body.LastStatus)
		}
		if body.LastSuccessAt == nil || *body.LastSuccessAt == "" {
			t.Fatal("expected last_success_at to be set")
		}
		if body.RetainedSnapshotCount != 1 {
			t.Fatalf("expected retained_snapshot_count 1, got %d", body.RetainedSnapshotCount)
		}
	})

	t.Run("staff_bookkeeper is forbidden", func(t *testing.T) {
		client := loginAs("staff_bookkeeper")
		resp, err := client.Get(srv.URL + "/api/backup/status")
		if err != nil {
			t.Fatalf("GET /api/backup/status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("external_accountant is forbidden", func(t *testing.T) {
		client := loginAs("external_accountant")
		resp, err := client.Get(srv.URL + "/api/backup/status")
		if err != nil {
			t.Fatalf("GET /api/backup/status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}
