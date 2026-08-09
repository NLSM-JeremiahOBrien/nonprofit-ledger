package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/alexedwards/scs/v2"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/ledger"
)

// RegisterRoutes is the single place every route group in the app is
// defined, per the fail-closed pattern: route groups are wrapped in
// RequireRole once here, rather than scattering per-handler
// "if role != ..." checks throughout individual handlers.
//
// Two groups exist:
//
//   - adminOnlyGroup: user management and org/period-lock settings,
//     wrapped in RequireRole("admin"). The external accountant is
//     deliberately never added to this group — the boundary is
//     enforced by omission, not a special-cased denial.
//   - allRolesGroup: ledger read/write and report routes, wrapped in
//     RequireRole("admin", "staff_bookkeeper", "external_accountant").
//
// Every route in both groups is wrapped in RequireAuth first — no route
// registered here is ever reachable without a valid session.
func RegisterRoutes(mux *http.ServeMux, sm *scs.SessionManager, db *sql.DB, backupDir string) {
	adminOnly := func(next http.Handler) http.Handler {
		return auth.RequireAuth(sm, db, auth.RequireRole("admin")(next))
	}
	allRoles := func(next http.Handler) http.Handler {
		return auth.RequireAuth(sm, db, auth.RequireRole("admin", "staff_bookkeeper", "external_accountant")(next))
	}

	// Public (unauthenticated) routes.
	mux.HandleFunc("/healthz", HealthHandler)
	authHandlers := &AuthHandlers{DB: db, SM: sm}
	mux.HandleFunc("/api/auth/login", authHandlers.LoginHandler)
	mux.HandleFunc("/api/auth/logout", authHandlers.LogoutHandler)

	// adminOnlyGroup: user management.
	userHandlers := &UserHandlers{DB: db}
	mux.Handle("GET /api/users", adminOnly(http.HandlerFunc(userHandlers.ListUsersHandler)))
	mux.Handle("POST /api/users", adminOnly(http.HandlerFunc(userHandlers.CreateUserHandler)))
	mux.Handle("PATCH /api/users/{id}/role", adminOnly(http.HandlerFunc(userHandlers.UpdateRoleHandler)))
	mux.Handle("POST /api/users/{id}/deactivate", adminOnly(http.HandlerFunc(userHandlers.DeactivateUserHandler)))

	// adminOnlyGroup: org/period-lock settings.
	mux.Handle("POST /api/org/period-lock", adminOnly(http.HandlerFunc(lockPeriodHandler(db))))

	// adminOnlyGroup: backup health status.
	backupHandlers := &BackupHandlers{DB: db, BackupDir: backupDir}
	mux.Handle("GET /api/backup/status", adminOnly(http.HandlerFunc(backupHandlers.BackupStatusHandler)))

	// allRolesGroup: ledger read/write and report routes.
	mux.Handle("GET /api/reports/trial-balance", allRoles(http.HandlerFunc(trialBalanceHandler(db))))
	mux.Handle("POST /api/ledger/entries", allRoles(http.HandlerFunc(postJournalEntryHandler(db))))
}

// lockPeriodRequest is the JSON body expected by the period-lock
// handler.
type lockPeriodRequest struct {
	ThroughDate string `json:"through_date"`
}

// lockPeriodHandler locks the accounting period through the given date.
// Admin-only (enforced by route wiring in RegisterRoutes, never here).
func lockPeriodHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req lockPeriodRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ThroughDate == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		cu, ok := auth.FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if err := ledger.LockPeriod(db, req.ThroughDate, cu.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// trialBalanceHandler returns the current trial balance (per-fund
// balances derived from posted journal lines). Reachable by all three
// roles (enforced by route wiring in RegisterRoutes).
func trialBalanceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := ledger.TrialBalance(db)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

// postJournalEntryRequest is the JSON body expected by the journal
// entry creation handler, mirroring ledger.NewEntry.
type postJournalEntryRequest struct {
	EntryDate string              `json:"entry_date"`
	Memo      string              `json:"memo"`
	Source    string              `json:"source"`
	Lines     []postJournalLineIn `json:"lines"`
}

type postJournalLineIn struct {
	AccountID          int64   `json:"account_id"`
	FundID             int64   `json:"fund_id"`
	FunctionalCategory *string `json:"functional_category"`
	DebitAmount        int64   `json:"debit_amount"`
	CreditAmount       int64   `json:"credit_amount"`
}

// postJournalEntryHandler posts a new journal entry via
// ledger.PostJournalEntry, the single sanctioned entry point that
// enforces balance/period-lock/functional-category invariants.
// Reachable by all three roles (enforced by route wiring in
// RegisterRoutes) — this is "ledger write" scope shared across admin,
// staff_bookkeeper, and external_accountant alike.
func postJournalEntryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		cu, ok := auth.FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		lines := make([]ledger.NewLine, 0, len(req.Lines))
		for _, l := range req.Lines {
			lines = append(lines, ledger.NewLine{
				AccountID:          l.AccountID,
				FundID:             l.FundID,
				FunctionalCategory: l.FunctionalCategory,
				DebitAmount:        l.DebitAmount,
				CreditAmount:       l.CreditAmount,
			})
		}

		id, err := ledger.PostJournalEntry(db, ledger.NewEntry{
			EntryDate: req.EntryDate,
			Memo:      req.Memo,
			PostedBy:  cu.ID,
			Source:    req.Source,
			Lines:     lines,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	}
}
