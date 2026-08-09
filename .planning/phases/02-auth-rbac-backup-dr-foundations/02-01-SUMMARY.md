---
phase: 02-auth-rbac-backup-dr-foundations
plan: 01
subsystem: auth
tags: [argon2id, scs, sqlite3store, sessions, golang-migrate, modernc-sqlite]

# Dependency graph
requires:
  - phase: 01-core-ledger-fund-accounting-data-model
    provides: SQLite connection/migration runner (server/db), golang-migrate migration pattern, local-webserver entrypoint (cmd/server/main.go)
provides:
  - users table (users) with role CHECK constraint, active/must_change_password flags
  - argon2id password hashing (server/auth/password.go)
  - SQLite-backed server-side session manager (server/auth/session.go) surviving process restart
  - login/logout API (server/api/auth.go) with anti-enumeration generic 401
  - RequireAuth middleware (server/auth/middleware.go) with fail-closed auth + CurrentUser context
  - first-run admin bootstrap wired into cmd/server/main.go
affects: [02-auth-rbac-backup-dr-foundations (plan 02 RBAC, plan 03 audit), 03-remote-access]

# Tech tracking
tech-stack:
  added: [golang.org/x/crypto/argon2, github.com/alexedwards/scs/v2, github.com/alexedwards/scs/sqlite3store]
  patterns:
    - "PHC-style self-describing password hash string ($argon2id$v=...$m=...,t=...,p=...$salt$hash) so verify never needs separately stored params"
    - "scs.SessionManager backed by a migration-defined `sessions` table (token/data/expiry) via sqlite3store, spike-verified against modernc.org/sqlite — no CGo introduced"
    - "Fail-closed auth middleware: any error path (no session, lookup failure, inactive user) returns 401, never allow-by-default"
    - "Anti-enumeration: unknown username and wrong password return byte-identical 401 body/status"

key-files:
  created:
    - server/db/migrations/0005_users.up.sql / .down.sql
    - server/db/migrations/0006_sessions.up.sql / .down.sql
    - server/auth/password.go
    - server/auth/session.go
    - server/auth/middleware.go
    - server/users/users.go
    - server/api/auth.go
  modified:
    - cmd/server/main.go
    - go.mod
    - go.sum

key-decisions:
  - "Spiked alexedwards/scs/sqlite3store against modernc.org/sqlite (pure-Go) before committing to it; it worked without CGo, so the plan's fallback path (custom migration-backed session table) was used directly since sqlite3store expects the table to pre-exist rather than create it itself."
  - "RequireAuth takes (sm, db, next) explicitly rather than closing over package-level state, keeping server/auth free of global mutable state and testable with per-test databases."
  - "First-admin bootstrap password uses a 20-character crypto/rand alphabet excluding visually ambiguous characters (0/O, 1/l/I), printed once via log.Printf and never persisted anywhere else."

requirements-completed: [AUTH-01, AUTH-04]

# Metrics
duration: 8min
completed: 2026-08-09
---

# Phase 02 Plan 01: Auth Foundation (Users, Sessions, Login/Logout, Bootstrap) Summary

**Argon2id password hashing, SQLite-backed server-side sessions via alexedwards/scs (spike-verified against modernc.org/sqlite, no CGo), login/logout endpoints with anti-enumeration protection, fail-closed RequireAuth middleware, and a first-run admin bootstrap that prints a one-time crypto/rand password.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-09T13:33:52Z
- **Completed:** 2026-08-09T13:41:23Z
- **Tasks:** 3
- **Files modified:** 17

## Accomplishments
- `users` table (migration 0005) mirroring the `accounting_periods` mutable-table pattern, with a role CHECK constraint covering `staff_bookkeeper`/`admin`/`external_accountant`
- Self-describing argon2id `HashPassword`/`VerifyPassword` (PHC-style string, random salt per call, `subtle.ConstantTimeCompare`, never panics on malformed input)
- Server-side session manager (`server/auth/session.go`) using `alexedwards/scs/v2` + `sqlite3store` against a migration-defined `sessions` table (migration 0006) — spike-verified compatible with the pure-Go `modernc.org/sqlite` driver, so no CGo dependency was introduced
- Login/logout API (`server/api/auth.go`) that renews the session token on login (fixation prevention) and returns an identical generic 401 for both unknown username and wrong password
- `RequireAuth` middleware (`server/auth/middleware.go`) that fails closed on any error path and stores `CurrentUser{ID, Role}` in the request context for later RBAC use
- First-run admin bootstrap in `cmd/server/main.go`: on an empty `users` table, generates a `crypto/rand` one-time password, creates one `admin` user with `must_change_password=true`, and prints the credentials to the console exactly once

## Task Commits

Each task was committed atomically (TDD RED→GREEN per task):

1. **Task 1: Users table migration + argon2id password hashing**
   - `9264b8a` (test) — failing tests for password hashing + users migration
   - `1043f87` (feat) — argon2id HashPassword/VerifyPassword implementation
2. **Task 2: Session manager, minimal user lookup, and login/logout handlers** - `d5e1177` (feat)
3. **Task 3: RequireAuth middleware, session persistence/revocation, and first-admin bootstrap** - `07ec01b` (feat)

**Plan metadata:** (this commit, see below)

## Files Created/Modified
- `server/db/migrations/0005_users.up.sql` / `.down.sql` - users table
- `server/db/migrations/0006_sessions.up.sql` / `.down.sql` - sessions table for sqlite3store
- `server/auth/password.go` - argon2id HashPassword/VerifyPassword
- `server/auth/password_test.go` - hash round-trip, PHC prefix, random salt, malformed-hash tests
- `server/auth/session.go` - scs.SessionManager construction (lifetime/idle/cookie config)
- `server/auth/session_test.go` - login/logout, persistence-across-restart, revocation tests
- `server/auth/middleware.go` - RequireAuth, CurrentUser, FromContext
- `server/auth/middleware_test.go` - RequireAuth rejects no-session and inactive-user cases
- `server/auth/testutil_test.go` - shared test helpers (cookie jar, URL parse, cookie assertions)
- `server/users/users.go` - User struct, ByUsername, ByID
- `server/users/users_test.go` - lookup round-trip test
- `server/api/auth.go` - LoginHandler, LogoutHandler
- `server/api/auth_test.go` - malformed-body rejection test
- `cmd/server/main.go` - wired bootstrap, session manager, login/logout routes, sm.LoadAndSave
- `go.mod` / `go.sum` - added golang.org/x/crypto, alexedwards/scs/v2, alexedwards/scs/sqlite3store

## Decisions Made
- Confirmed `alexedwards/scs/sqlite3store` works against `modernc.org/sqlite` via a standalone spike program before wiring it into the codebase; since the store expects its table to already exist (it does not create one), migration 0006 defines the `sessions` table exactly matching the store's expected schema (`token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL`).
- `RequireAuth(sm, db, next)` takes its dependencies as explicit parameters rather than package-level globals, keeping `server/auth` stateless and each test's session manager/database isolated.
- First-admin bootstrap password alphabet excludes ambiguous characters (0/O/1/l/I) for easier manual transcription during first login.

## Deviations from Plan

None - plan executed exactly as written. The plan's own contingency ("if scs/sqlite3store fails, fall back to a migration-defined table") applied in its non-failure branch: the spike succeeded, but sqlite3store still required a pre-existing table (it doesn't self-create one), so migration 0006 was written regardless — this was anticipated by the plan's task 2 action text, not a deviation from it.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. On first run, the server prints a one-time admin password to its own console; no manual credential setup is needed beyond reading that log line.

## Next Phase Readiness
- `CurrentUser{ID, Role}` is available in request context via `auth.FromContext`, ready for plan 02's `RequireRole` middleware.
- `server/users` currently only exposes the read path (`ByUsername`/`ByID`); plan 02 must add Create/List/UpdateRole.
- `sessions` table and `users` table are both plain mutable tables (matching `accounting_periods`), explicitly outside the append-only immutability trigger pattern used for `audit_log` in plan 03.

---
*Phase: 02-auth-rbac-backup-dr-foundations*
*Completed: 2026-08-09*

## Self-Check: PASSED

All 16 claimed files verified present on disk; all 4 task commits (9264b8a, 1043f87, d5e1177, 07ec01b) verified present in git log.
