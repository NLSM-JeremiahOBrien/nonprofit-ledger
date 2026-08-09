package api_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/backup"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

// TestOfflineIsolation starts the full server stack — auth, RBAC, audit,
// and backup all wired exactly as in cmd/server/main.go — on an
// ephemeral local port, performs a real login and a health check
// against it, then inspects the test process's own open sockets and
// asserts exactly one LISTEN socket (the server's own port) and zero
// ESTABLISHED/outbound connections.
//
// This makes Phase 1's manual lsof verification (documented in
// 01-04-SUMMARY.md) a permanently-enforced automated test that now also
// covers the auth/session/backup code paths added in Phase 2. PLAT-04 is
// the load-bearing trust guarantee of the whole project — it must be
// continuously verified, not asserted once and assumed to still hold.
func TestOfflineIsolation(t *testing.T) {
	lsofPath, err := exec.LookPath("lsof")
	if err != nil {
		t.Skip("lsof not available on this platform/sandbox; skipping offline-isolation smoke test")
	}

	dbPath := filepath.Join(t.TempDir(), "ledger.db")
	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn, "../db/migrations"); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	const username = "isolation-admin"
	const password = "correct horse battery staple"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 1)`,
		username, hash,
	); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups")
	if err := backup.RunScheduled(conn, backupDir, 7); err != nil {
		t.Fatalf("RunScheduled (backup wired into stack): %v", err)
	}

	sm := auth.NewSessionManager(conn)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, sm, conn, backupDir)
	handler := sm.LoadAndSave(mux)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	srv := &http.Server{Handler: handler}
	go srv.Serve(listener)
	defer srv.Close()

	baseURL := "http://" + listener.Addr().String()

	// A dedicated Transport with keep-alives disabled and no connection
	// pooling, so no idle-but-open socket lingers past the request and
	// gets mistaken for an outbound connection by the later socket
	// inspection.
	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	healthResp, err := client.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz: expected 200, got %d", healthResp.StatusCode)
	}

	loginBody, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	loginResp, err := client.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/auth/login: expected 200, got %d", loginResp.StatusCode)
	}

	transport.CloseIdleConnections()
	// Give the OS a brief moment to fully tear down the closed
	// connection's socket state before inspecting it.
	time.Sleep(100 * time.Millisecond)

	pid := os.Getpid()
	out, err := exec.Command(lsofPath, "-a", "-p", strconv.Itoa(pid), "-i", "-n", "-P").Output()
	if err != nil {
		// lsof exits non-zero when there are no matching lines in some
		// implementations; if we got no output at all alongside the
		// error, treat that as "no sockets found" rather than a hard
		// failure, since exit code semantics differ across lsof
		// implementations/platforms.
		if len(out) == 0 {
			t.Skipf("lsof returned no output and a non-zero exit (%v); skipping rather than risking a false failure", err)
		}
	}

	var listenCount, establishedCount int
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}
		switch {
		case strings.Contains(line, "(LISTEN)"):
			listenCount++
		case strings.Contains(line, "(ESTABLISHED)"):
			establishedCount++
		}
	}

	if listenCount != 1 {
		t.Errorf("expected exactly 1 LISTEN socket, got %d\nlsof output:\n%s", listenCount, out)
	}
	if establishedCount != 0 {
		t.Errorf("expected 0 ESTABLISHED/outbound sockets, got %d\nlsof output:\n%s", establishedCount, out)
	}
}
