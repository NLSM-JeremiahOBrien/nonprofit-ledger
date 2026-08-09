package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

func TestLoginHandlerRejectsMalformedBody(t *testing.T) {
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

	sm := auth.NewSessionManager(conn)
	h := &api.AuthHandlers{DB: conn, SM: sm}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	h.LoginHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected non-empty error message")
	}
}
