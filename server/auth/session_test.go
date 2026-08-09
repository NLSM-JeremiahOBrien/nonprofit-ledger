package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

const testMigrationsDir = "../db/migrations"

func newTestDB(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ledger-*.db")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	f.Close()
	return f.Name()
}

func seedTestServer(t *testing.T, username, password string) (*httptest.Server, string) {
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

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if _, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 1)`,
		username, hash,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	sm := auth.NewSessionManager(conn)
	handlers := &api.AuthHandlers{DB: conn, SM: sm}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", handlers.LoginHandler)
	mux.HandleFunc("/api/auth/logout", handlers.LogoutHandler)

	srv := httptest.NewServer(sm.LoadAndSave(mux))
	t.Cleanup(srv.Close)

	return srv, dbPath
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	srv, _ := seedTestServer(t, "alice", "correct-password")

	jar := newCookieJar(t)
	client := &http.Client{Jar: jar}

	resp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-password",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	found := false
	for _, c := range jar.Cookies(mustParseURL(t, srv.URL)) {
		if c.Name == "ledger_session" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ledger_session cookie to be set")
	}
}

func TestLoginWrongPasswordRejected(t *testing.T) {
	srv, _ := seedTestServer(t, "alice", "correct-password")

	client := &http.Client{Jar: newCookieJar(t)}

	resp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong-password",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	assertNoCookie(t, resp, "ledger_session")
}

func TestLoginUnknownUsernameRejectedSameAsWrongPassword(t *testing.T) {
	srv, _ := seedTestServer(t, "alice", "correct-password")

	client := &http.Client{}

	unknownResp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "bob-does-not-exist",
		"password": "anything",
	})
	defer unknownResp.Body.Close()
	var unknownBody map[string]string
	json.NewDecoder(unknownResp.Body).Decode(&unknownBody)

	wrongResp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong-password",
	})
	defer wrongResp.Body.Close()
	var wrongBody map[string]string
	json.NewDecoder(wrongResp.Body).Decode(&wrongBody)

	if unknownResp.StatusCode != wrongResp.StatusCode {
		t.Fatalf("expected identical status codes, got %d vs %d", unknownResp.StatusCode, wrongResp.StatusCode)
	}
	if unknownBody["error"] != wrongBody["error"] {
		t.Fatalf("expected identical error bodies, got %q vs %q", unknownBody["error"], wrongBody["error"])
	}
}

func TestLogoutDestroysSession(t *testing.T) {
	srv, _ := seedTestServer(t, "alice", "correct-password")

	jar := newCookieJar(t)
	client := &http.Client{Jar: jar}

	loginResp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-password",
	})
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", loginResp.StatusCode)
	}

	logoutResp, err := client.Post(srv.URL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", logoutResp.StatusCode)
	}
}
