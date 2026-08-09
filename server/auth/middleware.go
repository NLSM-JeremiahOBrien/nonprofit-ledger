package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/alexedwards/scs/v2"

	"github.com/tjcrowley/nonprofit-ledger/server/users"
)

// CurrentUser is the authenticated user's identity and role, stored in
// the request context by RequireAuth for downstream handlers and
// middleware (e.g. a later RequireRole check) to consume.
type CurrentUser struct {
	ID   int64
	Role string
}

type currentUserKey struct{}

// RequireAuth wraps next with session-based authentication: it reads the
// "userID" session value, loads the corresponding user from db, and
// rejects the request with 401 if there is no session, the user cannot
// be loaded, or the user is inactive. On success it stores a
// CurrentUser in the request context for next to consume.
//
// This fails closed: any error path results in 401, never "allow by
// default."
func RequireAuth(sm *scs.SessionManager, db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := sm.Get(r.Context(), "userID").(int64)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := users.ByID(db, userID)
		if err != nil || !user.Active {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), currentUserKey{}, CurrentUser{ID: user.ID, Role: user.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the CurrentUser stored in ctx by RequireAuth, and
// whether one was present.
func FromContext(ctx context.Context) (CurrentUser, bool) {
	u, ok := ctx.Value(currentUserKey{}).(CurrentUser)
	return u, ok
}

// RequireRole returns middleware that only allows requests through when
// the CurrentUser stored in the request context (by a preceding
// RequireAuth) has one of the given roles. It fails closed: if
// RequireAuth has not run (no CurrentUser in context) or the role isn't
// in the allowed set, the request is rejected with 403.
//
// This is the single, generic place role checks live — callers should
// never write per-handler "if role != ..." checks; instead compose
// RequireAuth then RequireRole(allowed...) when registering routes.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cu, ok := FromContext(r.Context())
			if !ok || !allowedSet[cu.Role] {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
