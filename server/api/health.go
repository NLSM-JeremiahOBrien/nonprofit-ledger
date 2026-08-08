// Package api holds HTTP handlers for the local webserver. No handler in
// this package makes an outbound network call, DNS lookup for a remote
// service, or telemetry/analytics call of any kind — the app runs
// entirely on local hardware (PLAT-01).
package api

import (
	"encoding/json"
	"net/http"
)

// HealthHandler responds 200 {"status":"ok"} for any request. It performs
// no database access, no outbound network calls, and no external
// dependency checks of any kind — it exists solely to prove the local
// server process is up and responding.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
