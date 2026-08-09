---
phase: 02-auth-rbac-backup-dr-foundations
plan: 02
subsystem: auth
tags: [rbac, go, sqlite, middleware, rest-api]

# Dependency graph
requires:
  - phase: 02-auth-rbac-backup-dr-foundations (plan 01)
    provides: users table, argon2id password hashing, SQLite-backed sessions, RequireAuth middleware, CurrentUser context
provides:
  - Full user CRUD (CreateUser, ListUsers, UpdateRole, Deactivate) with fixed-role validation matching the SQL CHECK
  - Generic RequireRole(allowed...) middleware for fail-closed role gating
  - Admin-only /api/users/* REST endpoints
  - server/api/routes.go: single RegisterRoutes function defining adminOnlyGroup vs allRolesGroup, first REST exposure of ledger.LockPeriod/TrialBalance/PostJournalEntry
affects: [phase-03-remote-access, phase-04-import-reconciliation, phase-06-ar-ap-statements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RequireRole(allowed...) composes with RequireAuth: RequireAuth(sm, db, RequireRole(roles...)(handler))"
    - "Route groups defined once in server/api/routes.go (RegisterRoutes), never per-handler role checks"
    - "server/users package never imports server/auth (avoids import cycle since auth already imports users) — callers hash passwords via auth.HashPassword and pass the hash into CreateUser"

key-files:
  created:
    - server/api/users.go
    - server/api/routes.go
    - server/auth/user_mgmt_test.go
    - server/auth/external_accountant_scope_test.go
  modified:
    - server/users/users.go
    - server/users/users_test.go
    - server/auth/middleware.go
    - cmd/server/main.go

key-decisions:
  - "CreateUser accepts a pre-hashed password rather than calling auth.HashPassword itself, to avoid a server/users <-> server/auth import cycle (server/auth/middleware.go already imports server/users for RequireAuth's user lookup)"
  - "TestUserMgmtRequiresAdmin and TestExternalAccountantScope live in server/auth (auth_test package) rather than server/api, matching the plan's own verify commands (`go test ./server/auth/... -run ...`)"
  - "external_accountant is scoped by omission from adminOnlyGroup, not a special-cased denial — the fail-closed pattern lives entirely in RequireRole + route grouping"
  - "Exposed ledger.LockPeriod, ledger.TrialBalance, and ledger.PostJournalEntry as the first REST routes (/api/org/period-lock, /api/reports/trial-balance, /api/ledger/entries) to give the route-group boundary real, non-stub routes to enforce against"

patterns-established:
  - "Pattern: role enum validation happens in Go (server/users) mirroring the SQL CHECK exactly, confirmed independently at both layers by tests"
  - "Pattern: admin-only vs all-roles route groups are the only two RBAC surfaces; new routes join one or the other in server/api/routes.go, never a bespoke check"

requirements-completed: [AUTH-02, AUTH-03]

# Metrics
duration: 12min
completed: 2026-08-09
---

# Phase 2 Plan 02: User Management & External-Accountant Scope Summary

**Full user CRUD across three fixed roles plus a single explicit route-group file (adminOnlyGroup vs allRolesGroup) that fail-closed enforces the external accountant's narrower scope.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-09T13:42:09Z
- **Completed:** 2026-08-09T13:54:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Admin can create, list, update the role of, and deactivate users across all three fixed roles (staff_bookkeeper, admin, external_accountant), with role validation enforced independently at both the Go layer and the SQL CHECK constraint from migration 0005
- Generic `RequireRole(allowed...)` middleware added to server/auth, composing with the existing `RequireAuth` — every future admin-only or role-scoped route reuses this, no per-handler checks
- `server/api/routes.go` is now the single place route groups are defined: `adminOnlyGroup` (user management, org/period-lock settings) and `allRolesGroup` (ledger read/write, trial-balance report), each wrapped once in `RequireAuth` + `RequireRole`
- External accountant is confirmed (by test, against the real production route wiring) to be blocked from `/api/users/*` and `/api/org/period-lock`, and allowed on `/api/reports/trial-balance` and `/api/ledger/entries`

## Task Commits

Each task was committed atomically:

1. **Task 1: User CRUD with role validation** - `2d226ad` (feat)
2. **Task 2: RequireRole middleware + admin-only user-management handlers** - `f2d8c47` (feat)
3. **Task 3: Explicit route groups enforcing external-accountant scope** - `3400e5a` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified
- `server/users/users.go` - CreateUser, ListUsers, UpdateRole, Deactivate added; role validation mirrors the SQL CHECK constraint exactly
- `server/users/users_test.go` - TestCreateUser covers all three valid roles, invalid-role rejection at both Go and SQL layers, and duplicate-username rejection
- `server/auth/middleware.go` - Added `RequireRole(allowed ...string) func(http.Handler) http.Handler`
- `server/auth/user_mgmt_test.go` - TestUserMgmtRequiresAdmin: non-admin 403s on all four user-mgmt routes, admin succeeds on all four
- `server/auth/external_accountant_scope_test.go` - TestExternalAccountantScope: exercises the real `api.RegisterRoutes` wiring to confirm the external-accountant boundary
- `server/api/users.go` - Admin-only REST handlers: list/create/update-role/deactivate, never expose PasswordHash
- `server/api/routes.go` - `RegisterRoutes(mux, sm, db)`: single source of truth for adminOnlyGroup vs allRolesGroup; also wires the first REST exposure of `ledger.LockPeriod`, `ledger.TrialBalance`, `ledger.PostJournalEntry`
- `cmd/server/main.go` - Now calls `api.RegisterRoutes` instead of wiring routes inline

## Decisions Made
- Avoided a `server/users` <-> `server/auth` import cycle by having `CreateUser` accept a pre-hashed password (caller — `server/api/users.go` — calls `auth.HashPassword` first), since `server/auth/middleware.go` already imports `server/users` for its own lookups
- Placed `TestUserMgmtRequiresAdmin` and `TestExternalAccountantScope` in `server/auth` (as `auth_test` package) rather than `server/api`, because the plan's own `<verify>` commands target `go test ./server/auth/...` — matching the verify command took priority over the file listed under `<files>`
- Gave the route-group boundary real routes to enforce against (`/api/org/period-lock`, `/api/reports/trial-balance`, `/api/ledger/entries`) backed by existing Phase 1 `server/ledger` functions, rather than stub handlers, since those functions already existed and unused stub routes would be less representative of the actual boundary

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Avoided import cycle between server/users and server/auth**
- **Found during:** Task 1 (User CRUD)
- **Issue:** Plan's action text said `CreateUser` should call `auth.HashPassword` directly, but `server/auth/middleware.go` already imports `server/users` (for `RequireAuth`'s user lookup), so `server/users` importing `server/auth` back would create a compile-time import cycle
- **Fix:** `CreateUser(db, username, passwordHash, role)` now takes an already-hashed password; `server/api/users.go`'s `CreateUserHandler` calls `auth.HashPassword` before calling `users.CreateUser`
- **Files modified:** server/users/users.go, server/api/users.go
- **Verification:** `go build ./...` succeeds; `TestCreateUser` confirms hashes are argon2id-encoded and never equal the plaintext password
- **Committed in:** 2d226ad (Task 1 commit)

**2. [Rule 3 - Blocking] Placed integration tests in server/auth to match verify commands**
- **Found during:** Task 2 and Task 3
- **Issue:** Plan's `<files>` list said `server/api/users_test.go`, but the plan's own `<verify>` automated commands are `go test ./server/auth/... -run TestUserMgmtRequiresAdmin` and `go test ./server/auth/... -run TestExternalAccountantScope` — a test file under `server/api` would never be found by those commands
- **Fix:** Created `server/auth/user_mgmt_test.go` and `server/auth/external_accountant_scope_test.go` (package `auth_test`) instead, importing `server/api` where needed (no cycle — this is test-only code)
- **Files modified:** server/auth/user_mgmt_test.go (new), server/auth/external_accountant_scope_test.go (new)
- **Verification:** Both verify commands pass exactly as specified in the plan
- **Committed in:** f2d8c47, 3400e5a

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking issues, both resolving contradictions within the plan text itself)
**Impact on plan:** No scope creep; both fixes were necessary to make the plan's own verify commands pass. AUTH-02 and AUTH-03 behavior is unchanged from what the plan specified.

## Issues Encountered
None beyond the two deviations documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `RequireRole` and the `adminOnlyGroup`/`allRolesGroup` pattern in `server/api/routes.go` are ready for reuse by later phases (remote access, import/reconciliation, AR/AP) needing admin-only or role-scoped routes — no new middleware should be needed
- `server/api/routes.go` currently exposes minimal REST surface for `ledger.LockPeriod`, `ledger.TrialBalance`, and `ledger.PostJournalEntry`; a full ledger REST API (more report types, journal entry listing/reversal endpoints) remains for a later phase
- Full test suite (`go test ./...`) and `go build ./...` are green

---
*Phase: 02-auth-rbac-backup-dr-foundations*
*Completed: 2026-08-09*

## Self-Check: PASSED

All created/modified files and all three task commit hashes (2d226ad, f2d8c47, 3400e5a) confirmed present.
