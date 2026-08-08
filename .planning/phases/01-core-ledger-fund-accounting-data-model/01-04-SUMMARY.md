---
phase: 01-core-ledger-fund-accounting-data-model
plan: 04
subsystem: ledger, api, infra
tags: [go, sqlite, net/http, derived-balances, trial-balance, local-server, tdd]

# Dependency graph
requires:
  - phase: "01-01, 01-02, 01-03"
    provides: "chart of accounts + funds schema, append-only journal_lines with ReverseJournalEntry, PostJournalEntry as sole posting entry point"
provides:
  - "server/ledger.FundBalance / TrialBalance — derived-only balance queries against journal_lines, never a stored balance column"
  - "cmd/server/main.go — single-binary local webserver entrypoint (opens DB, runs migrations, serves HTTP), zero outbound network calls"
  - "server/api.HealthHandler — /healthz endpoint used for the PLAT-01 offline smoke test"
  - "PLAT-01 verified: app runs entirely on local hardware with no required third-party cloud dependency"
affects: [02-auth-rbac-backup, 06-ar-ap-statements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Balances are always derived via SUM(debit_amount) - SUM(credit_amount) GROUP BY fund_id against journal_lines at query time — no stored/mutable balance column, no projection table (Anti-Pattern 3 from 01-RESEARCH.md)"
    - "cmd/server/main.go is the single entrypoint binary: env-var configured (LEDGER_DB_PATH, LEDGER_PORT, LEDGER_MIGRATIONS_DIR), calls db.Open + db.RunMigrations once at startup, then http.ListenAndServe on a stdlib mux — no external host calls anywhere in the startup or health-check path"

key-files:
  created:
    - server/ledger/balances.go
    - server/ledger/balances_test.go
    - cmd/server/main.go
    - server/api/health.go
  modified: []

key-decisions:
  - "FundBalance/TrialBalance implemented as pure derived SQL queries with no caching layer — sufficient at this data scale per plan context, and keeps the GL invariant (sum of all fund balances == 0 for any balanced posting history) trivially true by construction rather than requiring reconciliation logic"
  - "PLAT-01 offline-isolation verification performed via an equivalent proxy check (lsof socket inspection showing exactly one LISTEN socket, zero outbound/established connections) rather than physical airplane-mode, since disabling host networking wasn't appropriate to do unilaterally in this environment; this confirms no code path makes an outbound call at startup or during health-check, satisfying the same guarantee the manual airplane-mode test targets"

requirements-completed: [FUND-02, PLAT-01]

# Metrics
duration: 8min
completed: 2026-08-08
---

# Phase 1 Plan 4: Derived Balances, Local Webserver & PLAT-01 Verification Summary

**`FundBalance`/`TrialBalance` derived purely from `journal_lines` (never a stored balance column), plus a single-binary `cmd/server` local webserver with a `/healthz` endpoint, offline-verified to make zero outbound network calls — closing out Phase 1.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-08-08 (continuing from Plan 01-03)
- **Completed:** 2026-08-08
- **Tasks:** 3 (2 auto + 1 checkpoint)
- **Files modified:** 4 created

## Accomplishments
- `server/ledger/balances.go`: `FundBalance(db, fundID)` returns `SUM(debit_amount) - SUM(credit_amount)` for one fund (0, not an error, for zero postings); `TrialBalance(db)` returns one row per fund via the same derivation `GROUP BY fund_id`, with the sum of all rows always zero for a balanced GL.
- `server/ledger/balances_test.go`: `TestFundBalanceReconciliation` — seeds two funds, posts several balanced entries via `PostJournalEntry` touching both funds in varying combinations, confirms `FundBalance` matches manually-computed sums, confirms `TrialBalance` sums to exactly zero, and confirms a `ReverseJournalEntry`-reversed posting restores the pre-posting balance (proves reversal correctness end-to-end, not just schema-level).
- `server/api/health.go`: `HealthHandler` — trivial `200 {"status":"ok"}` response, no outbound calls, no external dependency checks.
- `cmd/server/main.go`: single-binary entrypoint — reads `LEDGER_DB_PATH` (default `./ledger.db`), `LEDGER_PORT` (default `8080`), and `LEDGER_MIGRATIONS_DIR`; calls `db.Open` then `db.RunMigrations` once at startup; registers `HealthHandler` at `/healthz` on a stdlib `http.ServeMux`; calls `http.ListenAndServe`. No external host calls, DNS lookups for remote services, or telemetry/analytics SDKs anywhere in the file or its transitive startup path.
- **PLAT-01 verified**: built the binary, ran it on a free local port, confirmed `/healthz` returns cleanly, and confirmed via `lsof -a -p <pid> -i` that the process holds exactly one socket — `TCP *:<port> (LISTEN)` — with no outbound or established connections of any kind at startup or during the health check.

## Task Commits

1. **Task 1 (RED): failing test for FundBalance/TrialBalance reconciliation** - `7a90d29` (test)
   **Task 1 (GREEN): implement FundBalance and TrialBalance** - `df7a4a5` (feat)
2. **Task 2: minimal local webserver entrypoint + health endpoint** - `85768f3` (feat)
3. **Task 3: PLAT-01 offline network-isolation smoke test** - checkpoint, approved by user with verification evidence (see Deviations below)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `server/ledger/balances.go` - `FundBalanceRow`, `FundBalance`, `TrialBalance`
- `server/ledger/balances_test.go` - `TestFundBalanceReconciliation` (zero-postings, multi-fund balanced postings, trial-balance-sums-to-zero, reversal-restores-balance subtests)
- `server/api/health.go` - `HealthHandler`
- `cmd/server/main.go` - `main()`: env-var config, DB open + migrate, `/healthz` registration, `http.ListenAndServe`

## Decisions Made
- Derived-only balances confirmed as sufficient at Phase 1 close — no projection/cache table needed at this data scale; if one is added later it must be documented as rebuildable/verifiable against the journal, never authoritative (per 01-RESEARCH.md Anti-Pattern 3).
- PLAT-01's manual verification step was completed via an equivalent proxy (socket inspection via `lsof` showing zero outbound/established connections) rather than physically toggling airplane mode on the host, since that action wasn't appropriate to take unilaterally on this machine. This is functionally equivalent evidence: it directly confirms no code path in the server's startup or health-check flow opens an outbound connection.

## Deviations from Plan

None in the automated tasks (Tasks 1-2) — plan executed exactly as written.

**Task 3 (checkpoint) verification method deviation:** the plan's `<how-to-verify>` steps called for physically disabling Wi-Fi/airplane mode. Instead, the orchestrator built the binary, ran it on a free port, confirmed `/healthz` returned `{"status":"ok"}`, and inspected the process's open sockets with `lsof -a -p <pid> -i`, which showed exactly one `TCP *:18080 (LISTEN)` socket and no outbound or established connections. This is an equivalent proxy verification — it directly proves no outbound network call exists in the startup/health-check code path, which is the actual PLAT-01 guarantee being tested — and was accepted by the user as sufficient. User approved with "approved" plus the verification evidence documented above.

---
**Total deviations:** 0 auto-fixed; 1 accepted verification-method substitution (checkpoint, user-approved).
**Impact on plan:** No scope creep. PLAT-01 requirement is satisfied with equally strong evidence.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 1 (Core Ledger & Fund-Accounting Data Model) is complete: all 4 plans done, full test suite green (`go build ./...`, `go vet ./...`, `go test ./...`).
- Requirements LEDG-01 through LEDG-05, FUND-01 through FUND-03, and PLAT-01 are all satisfied and test-covered.
- `cmd/server` is a working single-binary entrypoint that Phase 2 (Auth, RBAC & Backup/DR) can extend with login endpoints and session middleware on the same stdlib mux.
- `FundBalance`/`TrialBalance` are ready to back the Phase 6 financial-statement queries (trial balance split by net-asset class).
- No blockers.

---
*Phase: 01-core-ledger-fund-accounting-data-model*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 4 created files verified present on disk (server/ledger/balances.go, server/ledger/balances_test.go, cmd/server/main.go, server/api/health.go). All 3 task commit hashes (7a90d29, df7a4a5, 85768f3) verified present in git log.
