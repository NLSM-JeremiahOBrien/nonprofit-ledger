---
phase: 01-core-ledger-fund-accounting-data-model
plan: 01
subsystem: database
tags: [go, sqlite, modernc-sqlite, golang-migrate, fasb-asc-958, tdd]

# Dependency graph
requires: []
provides:
  - "Go module github.com/tjcrowley/nonprofit-ledger with locked dependencies (modernc.org/sqlite, golang-migrate/migrate v4)"
  - "server/db.Open / server/db.RunMigrations — SQLite connection + migration runner with foreign_keys/WAL PRAGMAs"
  - "server/ledger.newTestDB(t) — reusable real on-disk-SQLite test fixture for all future ledger package tests"
  - "accounts table (chart of accounts, 5-value type CHECK, self-referencing parent_id) + CreateAccount domain function"
  - "funds table (current FASB ASC 958 two-category net_asset_class CHECK) + CreateFund domain function"
affects: [01-02-journal-entries, 02-auth-rbac-backup, 04-import-reconciliation, 06-ar-ap-statements]

# Tech tracking
tech-stack:
  added: ["modernc.org/sqlite v1.56.0 (pure-Go, no CGo)", "github.com/golang-migrate/migrate/v4 v4.19.1"]
  patterns:
    - "Money stored as integer cents everywhere (convention established for later plans; no money columns yet)"
    - "PRAGMA foreign_keys=ON and PRAGMA journal_mode=WAL set explicitly per-connection in db.Open"
    - "Domain functions (CreateAccount, CreateFund) validate in Go before issuing SQL, never rely solely on DB CHECK constraints"
    - "Test fixture uses real on-disk SQLite file via t.TempDir(), not :memory: — required for later immutability-trigger tests"

key-files:
  created:
    - go.mod
    - go.sum
    - server/db/sqlite.go
    - server/db/migrations/0001_accounts.up.sql
    - server/db/migrations/0001_accounts.down.sql
    - server/db/migrations/0002_funds.up.sql
    - server/db/migrations/0002_funds.down.sql
    - server/ledger/testutil_test.go
    - server/ledger/coa.go
    - server/ledger/coa_test.go
    - server/ledger/funds.go
    - server/ledger/funds_test.go
  modified: []

key-decisions:
  - "Go 1.26.5 installed via Homebrew (was not present on the host); matches locked research decision of Go 1.26.x"
  - "FASB ASC 958 two-category model (without_donor_restrictions / with_donor_restrictions) enforced both by SQL CHECK and Go-side validation; deprecated three-category model explicitly rejected, including as aliases"

patterns-established:
  - "TDD RED/GREEN cycle: write test file, confirm compile-failure (RED), commit test, implement, confirm PASS (GREEN), commit implementation — used for both coa.go and funds.go"
  - "server/ledger package convention: NewX input struct with IsActiveSet bool flag to distinguish 'not provided' from 'explicitly false' for IsActive defaulting"

requirements-completed: [LEDG-01, FUND-01]

# Metrics
duration: 20min
completed: 2026-08-08
---

# Phase 1 Plan 1: Go Module Scaffold, Chart of Accounts & Funds Schema Summary

**Go module with modernc.org/sqlite + golang-migrate foundation, chart-of-accounts and funds tables enforcing current FASB ASC 958 two-category net-asset classification, both built TDD-first with a reusable real-SQLite test fixture.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-08-08T19:04:00Z (approx)
- **Completed:** 2026-08-08T19:24:04Z
- **Tasks:** 3
- **Files modified:** 12 created

## Accomplishments
- Bootstrapped the Go module (`github.com/tjcrowley/nonprofit-ledger`) with the locked dependency set from research: `modernc.org/sqlite` (pure-Go, no CGo) and `golang-migrate/migrate/v4`.
- Built `server/db.Open`/`RunMigrations` establishing the connection + migration pattern every later phase reuses, with `PRAGMA foreign_keys=ON` and `PRAGMA journal_mode=WAL` set explicitly per the locked SQLite decisions.
- Built `newTestDB(t)` — a real on-disk-SQLite (not `:memory:`) test fixture in `server/ledger`, reusable by every subsequent plan in this phase.
- Chart of accounts: `accounts` table with 5-value type CHECK and self-referencing `parent_id`; `CreateAccount` validates type and parent existence in Go before hitting SQL.
- Funds: `funds` table enforcing the current FASB ASC 958 two-category net-asset model only (`without_donor_restrictions`/`with_donor_restrictions`); `CreateFund` explicitly rejects the deprecated three-category model, including as aliases.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 followed TDD: test commit → feat commit):

1. **Task 1: Scaffold Go module, SQLite connection, and migration test fixture** - `cfd8652` (feat)
2. **Task 2 (RED): failing tests for CreateAccount** - `dc32258` (test)
   **Task 2 (GREEN): chart-of-accounts schema + CreateAccount** - `1442e97` (feat)
3. **Task 3 (RED): failing tests for CreateFund** - `78bd182` (test)
   **Task 3 (GREEN): funds schema + CreateFund** - `570dca0` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `go.mod` / `go.sum` - Go module with modernc.org/sqlite and golang-migrate/migrate v4 dependencies
- `server/db/sqlite.go` - `Open()` (PRAGMA foreign_keys/WAL) and `RunMigrations()`
- `server/db/migrations/0001_accounts.{up,down}.sql` - accounts table, 5-value type CHECK, self-referencing parent_id
- `server/db/migrations/0002_funds.{up,down}.sql` - funds table, ASC 958 two-category net_asset_class CHECK
- `server/ledger/testutil_test.go` - `newTestDB(t)` real-SQLite fixture, resolves migrations dir via `runtime.Caller`
- `server/ledger/coa.go` / `coa_test.go` - `Account`, `NewAccount`, `CreateAccount`; 6 test cases
- `server/ledger/funds.go` / `funds_test.go` - `Fund`, `NewFund`, `CreateFund`; 5 test cases

## Decisions Made
- Go was not installed on the execution host; installed Go 1.26.5 via Homebrew, satisfying the locked "Go 1.26.x" research decision with no version deviation.
- `NewAccount`/`NewFund` use an `IsActiveSet bool` companion field alongside `IsActive bool` so callers can distinguish "not specified" (defaults to active) from "explicitly set to inactive" — plan only specified the default-true behavior, this was the minimal way to support both without a pointer-heavy API.

## Deviations from Plan

None - plan executed exactly as written. (Go toolchain installation was an environment prerequisite, not a plan deviation — no plan task assumed Go was pre-installed.)

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `newTestDB(t)` fixture, migration pattern, and `server/db.Open`/`RunMigrations` are ready for Plan 02 (journal entries), which will foreign-key against `accounts` and `funds`.
- Migration sequence is at `0002`; Plan 02 should continue with `0003_*`.
- No blockers.

---
*Phase: 01-core-ledger-fund-accounting-data-model*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 12 created files verified present on disk. All 5 task commit hashes (cfd8652, dc32258, 1442e97, 78bd182, 570dca0) verified present in git log.
