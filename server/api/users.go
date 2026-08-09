package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/tjcrowley/nonprofit-ledger/server/auth"
	"github.com/tjcrowley/nonprofit-ledger/server/users"
)

// UserHandlers bundles the dependencies the admin-only user-management
// handlers need. All routes registered against these handlers must be
// wrapped in RequireAuth then RequireRole("admin") — see
// server/api/routes.go.
type UserHandlers struct {
	DB *sql.DB
}

// userResponse is the JSON shape returned for a user. It never includes
// PasswordHash.
type userResponse struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"must_change_password"`
}

func toUserResponse(u users.User) userResponse {
	return userResponse{
		ID:                 u.ID,
		Username:           u.Username,
		Role:               u.Role,
		Active:             u.Active,
		MustChangePassword: u.MustChangePassword,
	}
}

// ListUsersHandler handles GET /api/users, returning every user in the
// system (admin-only — enforced by route wiring, not here).
func (h *UserHandlers) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	list, err := users.ListUsers(h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	resp := make([]userResponse, 0, len(list))
	for _, u := range list {
		resp = append(resp, toUserResponse(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

// createUserRequest is the JSON body expected by CreateUserHandler.
type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUserHandler handles POST /api/users, creating a new user with
// one of the three fixed roles (admin-only — enforced by route wiring).
func (h *UserHandlers) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Username == "" || req.Password == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username, password, and role are required"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	u, err := users.CreateUser(h.DB, req.Username, hash, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, users.ErrInvalidRole):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		case errors.Is(err, users.ErrDuplicateUsername):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, toUserResponse(*u))
}

// updateRoleRequest is the JSON body expected by UpdateRoleHandler.
type updateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateRoleHandler handles PATCH /api/users/{id}/role, changing a
// user's role to one of the three fixed roles (admin-only — enforced by
// route wiring).
func (h *UserHandlers) UpdateRoleHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserIDFromPath(r.URL.Path, "/role")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := users.UpdateRole(h.DB, id, req.Role); err != nil {
		switch {
		case errors.Is(err, users.ErrInvalidRole):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		case errors.Is(err, sql.ErrNoRows):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeactivateUserHandler handles POST /api/users/{id}/deactivate,
// marking a user inactive without deleting the row (admin-only —
// enforced by route wiring).
func (h *UserHandlers) DeactivateUserHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserIDFromPath(r.URL.Path, "/deactivate")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	if err := users.Deactivate(h.DB, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// parseUserIDFromPath extracts the {id} segment from a path of the form
// /api/users/{id}<suffix>, e.g. /api/users/42/role with suffix "/role".
func parseUserIDFromPath(path, suffix string) (int64, bool) {
	const prefix = "/api/users/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}
	idStr := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
