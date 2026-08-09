package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/alexedwards/scs/v2"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/users"
)

// loginRequest is the JSON body expected by LoginHandler.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// invalidCredentialsBody is returned for both an unknown username and a
// correct username with the wrong password, so no response ever leaks
// which of the two was wrong (no user-enumeration signal).
var invalidCredentialsBody = map[string]string{"error": "invalid credentials"}

// AuthHandlers bundles the dependencies LoginHandler and LogoutHandler
// need: a database connection (for user lookup) and the session manager.
type AuthHandlers struct {
	DB *sql.DB
	SM *scs.SessionManager
}

// LoginHandler authenticates a username/password pair and, on success,
// starts a new session (renewing the session token first to prevent
// session fixation). On any failure — unknown username or wrong
// password — it returns an identical generic 401.
func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := users.ByUsername(h.DB, req.Username)
	if err != nil || !user.Active {
		writeJSON(w, http.StatusUnauthorized, invalidCredentialsBody)
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		writeJSON(w, http.StatusUnauthorized, invalidCredentialsBody)
		return
	}

	if err := h.SM.RenewToken(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	h.SM.Put(r.Context(), "userID", user.ID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// LogoutHandler destroys the current session.
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.SM.Destroy(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
