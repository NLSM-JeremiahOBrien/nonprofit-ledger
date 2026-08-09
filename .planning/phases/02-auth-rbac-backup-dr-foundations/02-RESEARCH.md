# Phase 2: Auth, RBAC & Backup/DR Foundations - Research

**Researched:** 2026-08-08
**Domain:** Session-based auth, RBAC, immutable audit logging, and local-only SQLite backup/DR in a pure-Go single-binary app
**Confidence:** HIGH (auth/RBAC/audit), MEDIUM-HIGH (backup strategy)

## Summary

Phase 2 builds the trust boundary that must exist before O'Brien's real financial data enters the system: scoped login, attributable actions, and an automated, verifiable local backup of the org's sole financial record. Every recommendation here is constrained by two hard project rules that are already load-bearing in the Phase 1 code: (1) pure-Go SQLite via `modernc.org/sqlite` with **no CGo**, and (2) **no outbound network calls, no telemetry, no cloud** — the app runs entirely on local hardware (see `cmd/server/main.go` and `server/api/health.go`, which document this literally in package comments).

The recommended stack is deliberately boring and mostly stdlib. Use `alexedwards/scs/v2` for server-side session management with a SQLite-backed store (session token in a cookie, session data in the DB — supports immediate revocation, which JWTs do not); `golang.org/x/crypto/argon2` with argon2id at the OWASP-2026 baseline for password hashing; and the **new Go 1.25+ stdlib `http.CrossOriginProtection`** for CSRF (Sec-Fetch-Site/Origin header checks) — no third-party CSRF library needed, which aligns perfectly with the "prefer stdlib over rolling our own crypto" constraint. RBAC is a thin `net/http` middleware layer: an auth middleware loads the session user into the request `context`, and role-guard middleware wraps protected handlers. The three roles (staff/bookkeeper, admin, external accountant) are a fixed enum, not a general permission engine — do not pull in Casbin.

The audit log follows the exact append-only pattern Phase 1 already established for the journal: a plain table plus paired `BEFORE UPDATE`/`BEFORE DELETE` triggers that `RAISE(ABORT)`. For backup, the primary recommendation is **`VACUUM INTO` scheduled snapshots + rotation** — pure Go, zero new dependencies, transaction-consistent even under WAL, fully controllable, testable, and 100% local. Litestream is documented as an *optional* operator-installed continuous-replication layer (its v0.5 is now pure-Go and can replicate to a local file path), but its in-process library API is explicitly unstable and mixing it in-process with `modernc.org/sqlite` has locking caveats — so it should not be the required mechanism for the success criteria.

**Primary recommendation:** `scs/v2` server-side sessions + `argon2id` (x/crypto) + stdlib `http.CrossOriginProtection` for CSRF + a fixed 3-role context-based middleware; audit log as a Phase-1-style append-only trigger-protected table; automated backups via scheduled `VACUUM INTO` snapshots with rotation, health status surfaced in the DB and an API endpoint, and a documented+scripted restore procedure. No cloud, no telemetry, no CGo anywhere.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | User can log in with username/password to a locally-hosted account | `scs/v2` session cookie + `argon2id` password verify; login/logout handlers over the existing `net/http` mux |
| AUTH-02 | Admin can create/manage user accounts with one of three roles (staff/bookkeeper, admin, external accountant) | `users` table + role enum (SQL `CHECK`, mirrored in Go like Phase 1's net-asset model); admin-only user-management handlers |
| AUTH-03 | External accountant role scope narrower than admin (view/edit ledger + reports, cannot manage users or org settings) | Role-guard middleware; external accountant blocked from user-mgmt and settings routes at the handler layer |
| AUTH-04 | Session persists securely across browser refresh/restart | Server-side session store + persistent cookie (`MaxAge`/`Persist`), `HttpOnly`, `Secure`, `SameSite=Lax` |
| AUTH-05 | Every create/edit/delete attributed to logged-in user in an immutable audit log | Append-only `audit_log` table with Phase-1-style triggers; `posted_by` already exists on `journal_entries`; audit writes carry `context` user ID |
| PLAT-03 | Automated local backups on a schedule, visible backup health status, documented+tested restore | Scheduled `VACUUM INTO` snapshots + rotation; `backup_runs` health table + status endpoint; scripted, test-covered restore |
| PLAT-04 | No financial data or usage telemetry transmitted to any third party by default | Extend Phase 1's offline-isolation smoke test (lsof socket inspection) to cover the auth/backup paths; no telemetry SDK, no outbound calls, backups local-only |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/alexedwards/scs/v2` | v2 (latest) | Server-side HTTP session management | OWASP-aligned: random token in cookie, data server-side; supports immediate revocation; integrates cleanly with stdlib `net/http`; the de-facto Go session library |
| `github.com/alexedwards/scs/sqlite3store` | latest pseudo-version | SQLite-backed session store | Keeps sessions in the same single SQLite file; survives restart (AUTH-04). See compatibility note below |
| `golang.org/x/crypto/argon2` | latest x/crypto | argon2id password hashing | OWASP 2026 #1 recommendation; memory-hard; already the project research pick. Argon2 is NOT in the stdlib — x/crypto is the maintained home |
| `net/http` (`http.CrossOriginProtection`) | Go 1.25+ (have 1.26.5) | CSRF protection via Sec-Fetch-Site/Origin | Stdlib as of Go 1.25 — no third-party CSRF dep; satisfies "prefer stdlib over rolling our own" |
| `modernc.org/sqlite` | v1.56.0 (already in go.mod) | Pure-Go SQLite driver | Already the project driver; no CGo; required for the single-binary/cross-compile constraint |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 (already in go.mod) | Schema migrations | Phase 1 pattern — every new table/trigger goes through a numbered migration pair |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/rand` | stdlib | Generating secure tokens / initial admin password | Any secret/token generation — never `math/rand` |
| `github.com/google/uuid` | v1.6.0 (already indirect) | Stable IDs if needed for sessions/audit | Only if integer PKs are insufficient; Phase 1 uses integer PKs — stay consistent unless there's a reason |
| `github.com/benbjohnson/litestream` (CLI) | v0.5.x | OPTIONAL continuous replication to local disk/NAS | Operator-installed, layered on top of the required `VACUUM INTO` backups — NOT a required dependency (see Backup section) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `scs/v2` server-side sessions | JWT / stateless tokens | JWTs can't be immediately revoked without a server-side blocklist (which defeats the point); for a single-server app with an admin who must be able to kill an external accountant's access, server-side sessions are strictly better |
| argon2id | bcrypt (cost≥12) | bcrypt is still safe in 2026 but argon2id is memory-hard and OWASP's #1 pick; project research already chose argon2id. Do not migrate to bcrypt |
| stdlib `CrossOriginProtection` | `gorilla/csrf` / `filippo.io/csrf` | Third-party double-submit token approach; unnecessary given stdlib support in Go 1.26. `filippo.io/csrf` is a reasonable fallback only if you need to support pre-2020 browsers (not a concern here) |
| `VACUUM INTO` snapshots | Litestream (required) / SQLite Online Backup API | See Backup section — snapshots are simplest, zero-dep, fully local, and testable; the others add dependency/complexity or CGo concerns |
| Fixed 3-role enum | Casbin / general RBAC engine | Massive overkill for exactly three fixed roles; adds a dependency and a policy-file attack surface for no benefit |

**Installation:**
```bash
go get github.com/alexedwards/scs/v2
go get github.com/alexedwards/scs/sqlite3store
go get golang.org/x/crypto/argon2
# litestream is an optional operator tool, not a go.mod dependency
```

## Architecture Patterns

### Recommended Project Structure
Extends the existing Phase 1 layout (`server/api`, `server/db`, `server/ledger`):
```
server/
├── api/            # HTTP handlers (existing health.go); add auth, users, backup handlers
├── auth/           # NEW: password hashing, session manager wiring, middleware
│   ├── password.go       # argon2id hash/verify
│   ├── session.go        # scs.SessionManager construction
│   ├── middleware.go     # RequireAuth, RequireRole(...) context-based guards
│   └── *_test.go
├── users/          # NEW: user CRUD domain (CreateUser, roles, admin-only mgmt)
├── audit/          # NEW: audit_log write helper + query
├── backup/         # NEW: VACUUM INTO snapshot, rotation, health status, restore
└── db/
    └── migrations/ # NEW pairs: 0005_users, 0006_sessions, 0007_audit_log, 0008_backup_runs
```

### Pattern 1: Password hashing (argon2id)
**What:** Hash passwords with argon2id at OWASP-2026 baseline params; store the full encoded string (params + salt + hash) so verification is self-describing.
**When to use:** User creation and password change (hash); login (verify).
**Example:**
```go
// Source: golang.org/x/crypto/argon2 + OWASP Password Storage Cheat Sheet (2026)
// Baseline: m=19456 KiB (19 MiB), t=2, p=1  — ~tune to ~100ms on target hardware.
import "golang.org/x/crypto/argon2"

const (
    argonTime    = 2
    argonMemory  = 19456 // KiB
    argonThreads = 1
    argonKeyLen  = 32
    saltLen      = 16
)

func HashPassword(pw string) (string, error) {
    salt := make([]byte, saltLen)
    if _, err := rand.Read(salt); err != nil { // crypto/rand
        return "", err
    }
    key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
    // Encode as: $argon2id$v=19$m=...,t=...,p=...$<b64salt>$<b64hash>
    return encodePHC(salt, key), nil
}
// Verify: parse params+salt from stored string, recompute, subtle.ConstantTimeCompare.
```
Note: `argon2.IDKey` selects argon2**id** (the recommended variant). Use `subtle.ConstantTimeCompare` for the final comparison.

### Pattern 2: Session manager + secure cookie (AUTH-04)
**What:** One `scs.SessionManager`, SQLite store, persistent secure cookie.
**Example:**
```go
// Source: github.com/alexedwards/scs/v2 docs
sm := scs.New()
sm.Store = sqlite3store.New(sqlDB)      // reuses the app's *sql.DB
sm.Lifetime = 12 * time.Hour            // sliding; tune for the org
sm.IdleTimeout = 2 * time.Hour
sm.Cookie.Name = "ledger_session"
sm.Cookie.HttpOnly = true
sm.Cookie.SameSite = http.SameSiteLaxMode
sm.Cookie.Secure = true                 // served over HTTPS (LAN cert / Phase 3 tsnet)
sm.Cookie.Persist = true                // survives browser restart -> AUTH-04

// Wrap the mux: mux = sm.LoadAndSave(mux)
// On login:  sm.RenewToken(ctx); sm.Put(ctx, "userID", user.ID)
// On logout: sm.Destroy(ctx)
```
`RenewToken` on login prevents session fixation. Admin can force-revoke by deleting the user's rows from the `sessions` table (immediate revocation — the whole reason for server-side sessions).

### Pattern 3: Auth + RBAC middleware (AUTH-01/02/03)
**What:** Two-layer middleware. `RequireAuth` loads the session's user ID, fetches role, stores a typed value in `context`. `RequireRole` wraps handlers and 403s if the context role isn't allowed.
**Example:**
```go
// Source: standard Go RBAC middleware pattern (net/http handler wrapper + context)
type ctxKey int
const userCtxKey ctxKey = 0

type CurrentUser struct { ID int64; Role string }

func (a *Auth) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        uid, ok := a.sm.Get(r.Context(), "userID").(int64)
        if !ok || uid == 0 { http.Error(w, "unauthorized", 401); return }
        u, err := a.users.ByID(r.Context(), uid)
        if err != nil { http.Error(w, "unauthorized", 401); return }
        ctx := context.WithValue(r.Context(), userCtxKey, CurrentUser{u.ID, u.Role})
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func RequireRole(allowed ...string) func(http.Handler) http.Handler {
    set := map[string]bool{}
    for _, r := range allowed { set[r] = true }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            u, _ := r.Context().Value(userCtxKey).(CurrentUser)
            if !set[u.Role] { http.Error(w, "forbidden", 403); return }
            next.ServeHTTP(w, r.WithContext(r.Context()))
        })
    }
}
// Routing: user-mgmt & settings -> RequireRole("admin")
//          ledger read/write & reports -> RequireRole("admin","staff_bookkeeper","external_accountant")
```
**Fail-closed rule:** the default for any protected route is deny. External-accountant scope (AUTH-03) is enforced by simply NOT wiring that role into admin-only route groups — never by a per-handler `if role != "external"` check scattered around.

### Pattern 4: Immutable audit log (AUTH-05) — reuse Phase 1's trigger pattern
**What:** An `audit_log` table protected by the same paired `BEFORE UPDATE`/`BEFORE DELETE` `RAISE(ABORT)` triggers Phase 1 uses on `journal_lines`/`journal_entries`.
**Example (migration `0007_audit_log.up.sql`):**
```sql
-- Source: mirrors server/db/migrations/0003_journal.up.sql immutability triggers
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    actor_user_id INTEGER NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,          -- 'create' | 'update' | 'delete'
    entity_type TEXT NOT NULL,     -- 'journal_entry' | 'user' | 'account' | ...
    entity_id INTEGER,
    detail TEXT,                   -- JSON: before/after or summary
    occurred_at TEXT NOT NULL      -- datetime('now')
);
CREATE INDEX audit_log_entity_idx ON audit_log(entity_type, entity_id);

CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON audit_log
BEGIN SELECT RAISE(ABORT, 'audit_log is append-only'); END;
CREATE TRIGGER audit_log_no_delete BEFORE DELETE ON audit_log
BEGIN SELECT RAISE(ABORT, 'audit_log is append-only'); END;
```
Write audit rows inside the **same transaction** as the mutating operation (Phase 1's `PostJournalEntry` already wraps everything in one tx — thread the actor's user ID through and append the audit insert before commit). The actor ID comes from `context`, not from client input.

Note: Phase 1's `journal_entries.posted_by` and `accounting_periods.locked_by`/`locked_at` columns already exist as `INTEGER` with **no FK** to a users table. Phase 2 should add the `users` table and decide whether to backfill/point these at it. Existing test data uses arbitrary integer user IDs, so adding a FK may require a migration that seeds a system user or relaxes enforcement for pre-existing rows — flag for the planner.

### Anti-Patterns to Avoid
- **Per-handler role `if` checks:** scatter authorization logic and you get inconsistent enforcement. Centralize in route-group middleware, fail closed.
- **Storing role/permission in the cookie or client:** role must be re-read from the DB per request via the session's user ID. Never trust a client-supplied role.
- **JWT for this app:** no revocation without extra machinery; server-side sessions are the correct choice for a single-server, admin-revocable system.
- **`math/rand` for tokens/salts/initial passwords:** always `crypto/rand`.
- **Backups that copy only the main `.db` file under WAL:** silently loses the `-wal` contents. Use `VACUUM INTO` (or Online Backup API / litestream) which are WAL-consistent.
- **Rolling your own password hash or session token scheme:** use argon2id + scs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom PBKDF/salt scheme | `x/crypto/argon2` (argon2id) | Memory-hardness, correct params, timing-safe verify are easy to get subtly wrong |
| Session tokens/store | Custom cookie signing + token table | `scs/v2` + `sqlite3store` | Handles token rotation, expiry cleanup, fixation prevention, concurrency |
| CSRF | Custom token double-submit | stdlib `http.CrossOriginProtection` | Shipped and audited in Go 1.25+; header-based, no token plumbing |
| Consistent live backup | `cp ledger.db backup.db` | `VACUUM INTO` (or litestream) | Raw copy under WAL is corrupt/partial; `VACUUM INTO` is transaction-consistent |
| Append-only enforcement | App-layer "please don't edit" | SQLite `BEFORE UPDATE/DELETE` triggers | Phase 1 already proves DB-layer enforcement survives any code path/bug |

**Key insight:** This phase is where "prefer well-maintained libraries or stdlib over rolling our own crypto" is most load-bearing. The only genuinely custom code should be the PHC-string encode/decode around `argon2.IDKey`, the role-routing wiring, and the backup scheduler/rotation — all of which are thin glue, not cryptography or storage internals.

## Common Pitfalls

### Pitfall 1: `sqlite3store` expecting the CGo driver
**What goes wrong:** `scs/sqlite3store`'s docs/examples import `github.com/mattn/go-sqlite3` (CGo). Assuming you must adopt that driver would break the pure-Go/no-CGo constraint.
**Why it happens:** The store's example uses the traditional CGo driver; there's no official statement about `modernc.org/sqlite`.
**How to avoid:** `sqlite3store.New(db *sql.DB)` takes a generic `*sql.DB`, and its schema is plain SQL (`token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL` + an expiry index). The project's `db.Open` already registers `modernc.org/sqlite` under the driver name `"sqlite"`. **Verification spike required:** confirm `sqlite3store` runs against the existing `*sql.DB`. Fallback if any query is driver-specific: create the `sessions` table via a project golang-migrate migration (matching the store's schema) and/or use a different scs store — the schema is trivial. Do NOT add `mattn/go-sqlite3`.
**Warning signs:** any `go get` pulling in `mattn/go-sqlite3`; build attempting CGo.

### Pitfall 2: Backup that isn't actually restorable
**What goes wrong:** Backups run and "succeed," but the restore path was never tested, so the org discovers the backup is unusable during an actual disaster.
**Why it happens:** PLAT-03 explicitly requires a *tested* restore — teams skip it because the happy path (taking backups) looks done.
**How to avoid:** Ship a restore procedure as a real, test-covered code path (or CLI subcommand): given a snapshot file, produce a working DB and verify integrity (`PRAGMA integrity_check`, plus a ledger-invariant check that trial balance still nets to zero). Add an automated test that takes a snapshot, restores it into a temp path, and asserts the ledger data matches. This is a success criterion, not polish.
**Warning signs:** no test exercises the restore direction; restore is only documented prose.

### Pitfall 3: WAL checkpoint / backup interaction
**What goes wrong:** Long-lived WAL grows, or a snapshot misses recent committed writes still in the WAL.
**Why it happens:** Misunderstanding WAL — recent commits live in `ledger.db-wal` until checkpoint.
**How to avoid:** `VACUUM INTO` reads a transaction-consistent view and writes a fresh, defragmented file — it inherently includes committed WAL contents and needs no manual checkpoint. Don't hand-copy files. (If ever using the Online Backup API instead, it's also WAL-consistent.)
**Warning signs:** backup file smaller than expected; missing latest transactions after restore.

### Pitfall 4: Telemetry/outbound creep (PLAT-04)
**What goes wrong:** A convenience library (error reporting, update check, analytics) or a backup target that phones out to cloud storage silently violates the no-telemetry, local-only guarantee.
**Why it happens:** Common libraries add "helpful" outbound calls; litestream's default guides point at S3.
**How to avoid:** Keep backups on a **local file path / second local volume / NAS** only — never configure a cloud replica by default. Extend Phase 1's offline-isolation smoke test (it inspects sockets via `lsof`: single LISTEN socket, zero outbound/established) to run with the auth and backup paths active. No analytics/telemetry/error-reporting SDK enters `go.mod`.
**Warning signs:** any new dependency that opens a network connection; a backup config with an `s3://`/`https://` replica URL.

### Pitfall 5: First-admin bootstrap
**What goes wrong:** No users exist on first run, so no one can log in to create the first admin (chicken-and-egg), OR a hardcoded default password ships.
**Why it happens:** Auth systems need a seeded first account.
**How to avoid:** On first startup with an empty `users` table, generate a random initial admin password with `crypto/rand`, print it once to the server log/console, and force a change on first login. Never hardcode credentials. Document this in the setup procedure.
**Warning signs:** a literal default password in code; unable to log in on a fresh DB.

## Code Examples

### Backup: transaction-consistent snapshot (PLAT-03, pure Go, no dep)
```go
// Source: SQLite VACUUM INTO docs (requires SQLite >= 3.27; modernc bundles current SQLite)
// Runs safely against the live WAL database; produces a consistent, compacted copy.
func Snapshot(db *sql.DB, destPath string) error {
    // destPath must not already exist. VACUUM INTO refuses to overwrite.
    _, err := db.ExecContext(ctx, "VACUUM INTO ?", destPath)
    return err
}
// Scheduler: time.Ticker (or a cron-style schedule) invokes Snapshot to a
// timestamped file under a local backup dir, then rotation prunes old files
// (keep N most recent / GFS-style). Record each run in backup_runs.
```

### Backup health status (PLAT-03)
```sql
-- 0008_backup_runs.up.sql — mutable status table (like accounting_periods, NOT append-only)
CREATE TABLE backup_runs (
    id INTEGER PRIMARY KEY,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL,        -- 'success' | 'failed'
    file_path TEXT,
    size_bytes INTEGER,
    error TEXT
);
```
Expose `GET /api/backup/status` (admin-only) returning last success time, last status, count of retained snapshots, and next scheduled run — this is the "visible backup health status" PLAT-03 requires.

### CSRF (stdlib, Go 1.26)
```go
// Source: net/http http.CrossOriginProtection (Go 1.25+)
csrf := http.NewCrossOriginProtection()
// Optionally csrf.AddTrustedOrigin("https://ledger.local") for the LAN/tsnet origin.
handler := csrf.Handler(sm.LoadAndSave(mux)) // wrap the whole authenticated mux
```

## State of the Art

| Old Approach | Current Approach (2026) | When Changed | Impact |
|--------------|--------------------------|--------------|--------|
| bcrypt as default | argon2id (OWASP #1) | OWASP cheat sheet, ongoing | Use argon2id for new projects; bcrypt only for legacy migration |
| Third-party CSRF lib (gorilla/csrf) | stdlib `http.CrossOriginProtection` | Go 1.25 | No CSRF dependency needed; header-based |
| Litestream CGo (mattn) | Litestream v0.5 pure-Go (modernc) + LTX format | Litestream v0.5.0, Oct 2025 | Litestream is now cross-compile-friendly; faster point-in-time restore. BUT its `-tags vfs` read-replica still needs CGo, and its library API is unstable |
| Raw file copy backups | `VACUUM INTO` / Online Backup API | SQLite 3.27+ | WAL-safe consistent snapshots without stopping writers |

**Deprecated/outdated:**
- JWT-for-everything: wrong tool for a revocable single-server admin model.
- CGo SQLite (`mattn/go-sqlite3`): banned by the project's no-CGo constraint; do not reintroduce via a session store or backup tool.

## Open Questions

1. **`scs/sqlite3store` + `modernc.org/sqlite` compatibility**
   - What we know: store takes a generic `*sql.DB`; schema is plain SQL; project driver is already registered as `"sqlite"`.
   - What's unclear: no official confirmation the store runs on the pure-Go driver; store has only pseudo-versioned releases.
   - Recommendation: 30-minute verification spike in Wave 0. Fallback: create the `sessions` table via project migration and/or use `memstore`+periodic persistence or a hand-written minimal store against the schema. Do not adopt CGo.

2. **Litestream: required vs optional**
   - What we know: v0.5 is pure-Go and can replicate to a local file path (fully offline-compatible); library API is explicitly unstable; in-process use with modernc has POSIX lock caveats.
   - What's unclear: whether the pilot operator wants continuous PITR vs periodic snapshots.
   - Recommendation: make **`VACUUM INTO` snapshots the required PLAT-03 mechanism** (simplest, testable, zero-dep). Document litestream-to-local-disk as an optional, operator-run enhancement — not a Phase 2 deliverable dependency.

3. **Backfilling FK from Phase 1 `posted_by`/`locked_by` to new `users` table**
   - What we know: those columns exist as `INTEGER` with no FK; tests seed arbitrary IDs.
   - What's unclear: whether to add FKs now (data integrity) or defer to avoid disrupting Phase 1 tests/fixtures.
   - Recommendation: planner decides — likely seed a `system`/`import` user and add FKs in the same migration that creates `users`, updating Phase 1 test fixtures to use real user IDs.

4. **HTTPS/`Secure` cookie in dev vs prod**
   - What we know: `Secure` cookies require HTTPS; Phase 1 serves plain HTTP on `:8080`; Phase 3 brings tsnet (which provides TLS).
   - Recommendation: make `Secure` configurable (env), default on; document that LAN dev may need a local cert or a dev-only override. Never disable `HttpOnly`/`SameSite`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` (Go 1.26.5), table-driven — matches Phase 1 (`*_test.go` throughout `server/ledger`) |
| Config file | none — `go test` convention; shared helpers in `testutil_test.go` (Phase 1 pattern to extend) |
| Quick run command | `go test ./server/auth/... ./server/users/... ./server/audit/... ./server/backup/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | Correct password logs in, wrong password rejected; session cookie set | unit + handler | `go test ./server/auth/... -run TestLogin -x` | ❌ Wave 0 |
| AUTH-01 | argon2id hash round-trips; verify is timing-safe; wrong pw fails | unit | `go test ./server/auth/... -run TestPassword -x` | ❌ Wave 0 |
| AUTH-02 | Admin creates users with each of 3 roles; invalid role rejected (SQL CHECK + Go) | unit | `go test ./server/users/... -run TestCreateUser -x` | ❌ Wave 0 |
| AUTH-02 | Non-admin cannot create/manage users (403) | handler | `go test ./server/auth/... -run TestUserMgmtRequiresAdmin -x` | ❌ Wave 0 |
| AUTH-03 | External accountant blocked from user-mgmt & settings (403), allowed on ledger/reports | handler | `go test ./server/auth/... -run TestExternalAccountantScope -x` | ❌ Wave 0 |
| AUTH-04 | Session persists (persistent cookie); survives simulated restart; idle/lifetime expiry | integration | `go test ./server/auth/... -run TestSessionPersistence -x` | ❌ Wave 0 |
| AUTH-04 | Admin revocation kills active session immediately | integration | `go test ./server/auth/... -run TestRevokeSession -x` | ❌ Wave 0 |
| AUTH-05 | create/edit(reversal)/delete-attempt writes an audit row with correct actor from context | unit + integration | `go test ./server/audit/... -run TestAuditAttribution -x` | ❌ Wave 0 |
| AUTH-05 | UPDATE/DELETE on audit_log is rejected by trigger | unit | `go test ./server/audit/... -run TestAuditImmutable -x` | ❌ Wave 0 |
| PLAT-03 | `VACUUM INTO` produces a consistent snapshot including latest committed writes | unit | `go test ./server/backup/... -run TestSnapshot -x` | ❌ Wave 0 |
| PLAT-03 | Restore snapshot into temp DB; `PRAGMA integrity_check` ok; trial balance still nets zero | integration | `go test ./server/backup/... -run TestRestoreRoundTrip -x` | ❌ Wave 0 |
| PLAT-03 | Rotation keeps N snapshots, prunes older; `backup_runs` records success/failure | unit | `go test ./server/backup/... -run TestRotation -x` | ❌ Wave 0 |
| PLAT-03 | Backup status endpoint returns last-success/last-status (admin-only) | handler | `go test ./server/api/... -run TestBackupStatus -x` | ❌ Wave 0 |
| PLAT-04 | No outbound sockets with auth+backup active (extend Phase 1 lsof isolation smoke test) | smoke | `go test ./... -run TestOfflineIsolation -x` (extend existing) | ⚠️ extend existing |
| PLAT-04 | No telemetry/cloud dependency in module graph | manual/CI | `go list -deps ./... | grep -Ei 'analytics|sentry|s3|telemetry'` returns nothing | ❌ Wave 0 (CI check) |

### Sampling Rate
- **Per task commit:** `go test ./server/<touched-package>/... -x` (quick, < 30s)
- **Per wave merge:** `go test ./...` (full suite green)
- **Phase gate:** full suite green + restore round-trip test green + offline-isolation smoke green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `server/auth/password_test.go` — argon2id hash/verify (AUTH-01)
- [ ] `server/auth/session_test.go`, `middleware_test.go` — persistence, revocation, role guards (AUTH-01/03/04)
- [ ] `server/users/users_test.go` — user CRUD + role validation (AUTH-02)
- [ ] `server/audit/audit_test.go` — attribution + immutability trigger (AUTH-05)
- [ ] `server/backup/backup_test.go` — snapshot, restore round-trip, rotation, status (PLAT-03)
- [ ] Extend existing Phase 1 offline-isolation smoke test to cover auth+backup paths (PLAT-04)
- [ ] CI/manual dependency-graph check for telemetry/cloud packages (PLAT-04)
- [ ] Verification spike: `scs/sqlite3store` against `modernc.org/sqlite` (Open Question 1) — before committing to the store
- [ ] Shared test helper: extend `testutil_test.go` pattern with a seeded-user + logged-in-request fixture
- [ ] Framework install: none needed — Go stdlib `testing` already in use

## Sources

### Primary (HIGH confidence)
- [github.com/alexedwards/scs/v2 — Go Packages](https://pkg.go.dev/github.com/alexedwards/scs/v2) — session manager API, cookie config, LoadAndSave, RenewToken
- [github.com/alexedwards/scs/sqlite3store — Go Packages](https://pkg.go.dev/github.com/alexedwards/scs/sqlite3store) — `New(*sql.DB)`, required schema, driver note
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26) and [net/http csrf.go source](https://go.dev/src/net/http/csrf.go) — stdlib `CrossOriginProtection`
- [Litestream: Replicating to a Local File Path](https://litestream.io/guides/file/) and [Command: restore](https://litestream.io/reference/restore/) — local-only replication + restore
- [Litestream: Using as a Go Library](https://litestream.io/guides/go-library/) — API-stability caveat
- [Litestream VFS Reference](https://litestream.io/reference/vfs/) — VFS still needs CGo/mattn; main binary is pure-Go in v0.5
- [SQLite Online Backup API (backup_finish)](https://sqlite.org/c3ref/backup_finish.html) — WAL-consistent backup semantics
- Phase 1 code: `server/db/migrations/0003_journal.up.sql` (trigger pattern), `server/ledger/posting.go` (single-entry-point + single-tx pattern), `cmd/server/main.go` / `server/api/health.go` (no-outbound PLAT-01 guarantee), `server/db/sqlite.go` (WAL + modernc driver)

### Secondary (MEDIUM confidence)
- [OWASP Password Storage: Bcrypt vs Argon2id (2026)](https://www.onlinehashcrack.com/guides/password-recovery/bcrypt-vs-argon2-choosing-strong-hashing-today.php) and [Argon2 vs bcrypt 2026](https://blog.kestreltools.com/en/blog/argon2-vs-scrypt-vs-bcrypt-password-hashing-2026/) — argon2id params (m=19456, t=2, p=1)
- [A Modern Approach to Preventing CSRF in Go — Alex Edwards](https://www.alexedwards.net/blog/preventing-csrf-in-go) and [CSRF Protection in Go 1.25](https://samueladebayo.dev/posts/golang-cross-origin-protection/) — stdlib CSRF usage, browser caveats
- [Litestream v0.5.0 performance/PITR](https://biggo.com/news/202510041322_Litestream_v0.5.0_Performance_Update) — LTX format, pure-Go migration
- [Backup strategies for SQLite in production — Oldmoe's blog](https://oldmoe.blog/2024/04/30/backup-strategies-for-sqlite-in-production/) and [SQLite WAL consistent backups](https://sqlite.work/ensuring-consistent-backups-in-sqlite-wal-mode-without-disrupting-writers/) — VACUUM INTO vs raw copy
- [Building RBAC in Golang — Aserto](https://www.aserto.com/blog/building-rbac-in-go) and [Secure design pattern for RBAC in Go — RunReveal](https://blog.runreveal.com/owasp-oplease-a-secure-design-pattern-for-role-based-authorization-in-go/) — context + middleware, fail-closed pattern

### Tertiary (LOW confidence — flagged for validation)
- `scs/sqlite3store` running on `modernc.org/sqlite`: inferred from generic `*sql.DB` signature + plain-SQL schema; NOT officially documented — verification spike required (Open Question 1)
- Exact argon2id timing (~100ms) is hardware-dependent — tune on target NAS hardware during implementation

## Metadata

**Confidence breakdown:**
- Standard stack (scs, argon2id, stdlib CSRF): HIGH — official docs + OWASP + Go release notes; scs is de-facto standard
- Architecture (middleware RBAC, audit triggers): HIGH — directly mirrors proven Phase 1 patterns in this codebase
- Backup strategy: MEDIUM-HIGH — VACUUM INTO is well-documented and WAL-safe; the "which mechanism is required" call is a design recommendation, not a fact
- `sqlite3store` pure-Go compatibility: MEDIUM — needs a verification spike (only unknown that could shift the stack)

**Research date:** 2026-08-08
**Valid until:** ~2026-09-08 (30 days; stable domain, but re-check litestream v0.5.x API and scs releases if implementation slips)
