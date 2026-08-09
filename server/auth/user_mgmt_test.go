package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// setupUserMgmtServer builds a fully-migrated test database, session
// manager, and an httptest server exposing the four user-management
// routes wired behind RequireAuth then RequireRole("admin") — exactly
// the composition production route wiring uses (see
// server/api/routes.go). It seeds one user per fixed role (username ==
// role, password "password") and returns the server plus a helper that
// logs in as a given username and returns an authenticated client.
func setupUserMgmtServer(t *testing.T) (*httptest.Server, func(username string) *http.Client) {
	t.Helper()

	dbPath := newTestDB(t)
	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.RunMigrations(conn, testMigrationsDir); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	sm := auth.NewSessionManager(conn)

	adminOnly := func(next http.Handler) http.Handler {
		return auth.RequireAuth(sm, conn, auth.RequireRole("admin")(next))
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	created := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mux := http.NewServeMux()
	mux.Handle("GET /api/users", adminOnly(ok))
	mux.Handle("POST /api/users", adminOnly(created))
	mux.Handle("PATCH /api/users/{id}/role", adminOnly(ok))
	mux.Handle("POST /api/users/{id}/deactivate", adminOnly(ok))

	// A login endpoint outside RequireAuth purely for test setup: it
	// starts a session for the given username without needing to go
	// through the real password-based login handler.
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
	t.Cleanup(srv.Close)

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
		jar := newCookieJar(t)
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

	return srv, loginAs
}

func TestUserMgmtRequiresAdmin(t *testing.T) {
	srv, loginAs := setupUserMgmtServer(t)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodPatch, "/api/users/1/role"},
		{http.MethodPost, "/api/users/1/deactivate"},
	}

	nonAdminRoles := []string{"staff_bookkeeper", "external_accountant"}
	for _, role := range nonAdminRoles {
		client := loginAs(role)
		for _, req := range requests {
			httpReq, err := http.NewRequest(req.method, srv.URL+req.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := client.Do(httpReq)
			if err != nil {
				t.Fatalf("%s %s: %v", req.method, req.path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("role=%s %s %s: expected 403, got %d", role, req.method, req.path, resp.StatusCode)
			}
		}
	}

	adminClient := loginAs("admin")
	for _, req := range requests {
		httpReq, err := http.NewRequest(req.method, srv.URL+req.path, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		resp, err := adminClient.Do(httpReq)
		if err != nil {
			t.Fatalf("%s %s: %v", req.method, req.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			t.Fatalf("admin %s %s: expected 2xx, got %d", req.method, req.path, resp.StatusCode)
		}
	}
}
