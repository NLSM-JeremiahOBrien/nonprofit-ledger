package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// setupFullRouteServer builds a fully-migrated test database and an
// httptest server using the exact production route wiring
// (api.RegisterRoutes), so this test exercises the real adminOnlyGroup /
// allRolesGroup boundary defined in server/api/routes.go rather than a
// re-implementation of it.
func setupFullRouteServer(t *testing.T) (*httptest.Server, func(username string) *http.Client) {
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

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, sm, conn)

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

	srv := httptest.NewServer(withSessionMiddleware(sm, mux))
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

func withSessionMiddleware(sm *scs.SessionManager, next http.Handler) http.Handler {
	return sm.LoadAndSave(next)
}

func TestExternalAccountantScope(t *testing.T) {
	srv, loginAs := setupFullRouteServer(t)
	client := loginAs("external_accountant")

	t.Run("blocked from user management and org settings", func(t *testing.T) {
		blocked := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/users"},
			{http.MethodPost, "/api/org/period-lock"},
		}
		for _, b := range blocked {
			req, err := http.NewRequest(b.method, srv.URL+b.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", b.method, b.path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s: expected 403, got %d", b.method, b.path, resp.StatusCode)
			}
		}
	})

	t.Run("allowed on ledger and report routes", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/api/reports/trial-balance")
		if err != nil {
			t.Fatalf("GET trial-balance: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden {
			t.Fatalf("GET trial-balance: expected non-403, got %d", resp.StatusCode)
		}

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/entries", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		resp2, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST ledger/entries: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode == http.StatusForbidden {
			t.Fatalf("POST ledger/entries: expected non-403, got %d", resp2.StatusCode)
		}
	})
}
