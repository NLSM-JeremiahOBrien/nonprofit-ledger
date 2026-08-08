// Command server is the single-binary local webserver entrypoint for the
// nonprofit ledger. It opens a local SQLite database, runs migrations,
// and serves the HTTP API — with no outbound network call, DNS lookup
// for a remote service, or telemetry/analytics SDK anywhere in its
// startup path or normal operation. This is the literal PLAT-01
// guarantee: the app runs entirely on local hardware with no required
// third-party cloud dependency.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tjcrowley/nonprofit-ledger/server/api"
	"github.com/tjcrowley/nonprofit-ledger/server/db"
)

func main() {
	dbPath := envOrDefault("LEDGER_DB_PATH", "./ledger.db")
	port := envOrDefault("LEDGER_PORT", "8080")
	migrationsDir := envOrDefault("LEDGER_MIGRATIONS_DIR", "server/db/migrations")

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("server: opening database %s: %v", dbPath, err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn, migrationsDir); err != nil {
		log.Fatalf("server: running migrations: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.HealthHandler)

	addr := ":" + port
	log.Printf("server: listening on %s (db=%s)", addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// envOrDefault returns the value of the environment variable named key,
// or fallback if it is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
