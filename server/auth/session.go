package auth

import (
	"database/sql"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

// NewSessionManager constructs a *scs.SessionManager backed by a
// SQLite-backed session store (server/db/migrations/0006_sessions).
// Sessions persist across process restarts because state lives in the
// database, not in memory.
//
// Cookie.Secure defaults to true and can be disabled only via the
// LEDGER_COOKIE_SECURE=false environment variable (e.g. for local HTTP
// development).
func NewSessionManager(db *sql.DB) *scs.SessionManager {
	sm := scs.New()
	sm.Store = sqlite3store.New(db)
	sm.Lifetime = 12 * time.Hour
	sm.IdleTimeout = 2 * time.Hour
	sm.Cookie.Name = "ledger_session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = cookieSecure()
	sm.Cookie.Persist = true

	return sm
}

func cookieSecure() bool {
	v := os.Getenv("LEDGER_COOKIE_SECURE")
	if v == "" {
		return true
	}
	return v != "false"
}
