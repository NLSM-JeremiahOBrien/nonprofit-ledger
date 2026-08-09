package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

func TestRequireAuthRejectsWithoutSession(t *testing.T) {
	dbPath := newTestDB(t)
	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()
	if err := db.RunMigrations(conn, testMigrationsDir); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	sm := auth.NewSessionManager(conn)
	protected := auth.RequireAuth(sm, conn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(sm.LoadAndSave(protected))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d", resp.StatusCode)
	}
}

func TestRequireAuthRejectsInactiveUser(t *testing.T) {
	dbPath := newTestDB(t)
	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()
	if err := db.RunMigrations(conn, testMigrationsDir); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	hash, err := auth.HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	res, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 0)`,
		"deactivated", hash,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()

	sm := auth.NewSessionManager(conn)
	protected := auth.RequireAuth(sm, conn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	mux := http.NewServeMux()
	mux.HandleFunc("/set-session", func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", userID)
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/protected", protected)

	srv := httptest.NewServer(sm.LoadAndSave(mux))
	defer srv.Close()

	jar := newCookieJar(t)
	client := &http.Client{Jar: jar}

	setResp, err := client.Get(srv.URL + "/set-session")
	if err != nil {
		t.Fatalf("GET /set-session: %v", err)
	}
	setResp.Body.Close()

	resp, err := client.Get(srv.URL + "/protected")
	if err != nil {
		t.Fatalf("GET /protected: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for inactive user, got %d", resp.StatusCode)
	}
}
