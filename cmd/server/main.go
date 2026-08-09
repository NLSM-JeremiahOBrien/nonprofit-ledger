// Command server is the single-binary local webserver entrypoint for the
// nonprofit ledger. It opens a local SQLite database, runs migrations,
// and serves the HTTP API — with no outbound network call, DNS lookup
// for a remote service, or telemetry/analytics SDK anywhere in its
// startup path or normal operation. This is the literal PLAT-01
// guarantee: the app runs entirely on local hardware with no required
// third-party cloud dependency.
package main

import (
	"crypto/rand"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/backup"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

func main() {
	dbPath := envOrDefault("LEDGER_DB_PATH", "./ledger.db")
	port := envOrDefault("LEDGER_PORT", "8080")
	migrationsDir := envOrDefault("LEDGER_MIGRATIONS_DIR", "server/db/migrations")

	// Backup configuration. backupDir is always a local filesystem path —
	// this code path never accepts or constructs an s3://, https://, or
	// any other remote destination (PLAT-04).
	backupDir := envOrDefault("LEDGER_BACKUP_DIR", "./backups")
	backupInterval, err := time.ParseDuration(envOrDefault("LEDGER_BACKUP_INTERVAL", "24h"))
	if err != nil {
		log.Fatalf("server: invalid LEDGER_BACKUP_INTERVAL: %v", err)
	}
	backupRetain, err := strconv.Atoi(envOrDefault("LEDGER_BACKUP_RETAIN", "7"))
	if err != nil {
		log.Fatalf("server: invalid LEDGER_BACKUP_RETAIN: %v", err)
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("server: opening database %s: %v", dbPath, err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn, migrationsDir); err != nil {
		log.Fatalf("server: running migrations: %v", err)
	}

	if err := bootstrapFirstAdmin(conn); err != nil {
		log.Fatalf("server: bootstrapping first admin: %v", err)
	}

	startBackupScheduler(conn, backupDir, backupInterval, backupRetain)

	sm := auth.NewSessionManager(conn)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, sm, conn, backupDir)

	handler := sm.LoadAndSave(mux)

	addr := ":" + port
	log.Printf("server: listening on %s (db=%s)", addr, dbPath)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// startBackupScheduler runs backup.RunScheduled once immediately (so a
// fresh install has a backup within the first run) and then again on
// every tick of interval, for as long as the process is alive. It runs
// in its own goroutine so backup activity never blocks server startup
// or request handling.
func startBackupScheduler(conn *sql.DB, backupDir string, interval time.Duration, retain int) {
	runOnce := func() {
		if err := backup.RunScheduled(conn, backupDir, retain); err != nil {
			log.Printf("server: scheduled backup failed: %v", err)
		}
	}

	runOnce()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	}()
}

// bootstrapFirstAdmin checks whether the users table is empty and, if
// so, creates exactly one admin account with a randomly generated
// one-time password. The password is printed to the server console
// exactly once — it is never written to a file, logged again, or
// hardcoded as a default.
func bootstrapFirstAdmin(conn *sql.DB) error {
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password, err := generateOneTimePassword(20)
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	if _, err := conn.Exec(
		`INSERT INTO users (username, password_hash, role, active, must_change_password)
		 VALUES (?, ?, 'admin', 1, 1)`,
		"admin", hash,
	); err != nil {
		return err
	}

	log.Printf("server: bootstrapped first admin account — username=%q password=%q (change this immediately; it will not be shown again)", "admin", password)

	return nil
}

// generateOneTimePassword returns a random, human-typeable password of
// the given length using crypto/rand (never math/rand).
func generateOneTimePassword(length int) (string, error) {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

// envOrDefault returns the value of the environment variable named key,
// or fallback if it is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
