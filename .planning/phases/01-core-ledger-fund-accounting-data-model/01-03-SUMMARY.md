---
phase: 01-core-ledger-fund-accounting-data-model
plan: 03
subsystem: ledger
tags: [go, sqlite, double-entry, period-lock, functional-expense, tdd]

# Dependency graph
requires: ["01-01", "01-02"]
provides:
  - "accounting_periods table + LockPeriod/GetLockedThroughDate — mutable period-lock settings row"
  - "server/ledger.PostJournalEntry — the single sanctioned entry point for new (non-reversal) journal entries"
  - "server/ledger.balances — pure balance-check helper (no DB hit, fail-fast)"
affects: [02-auth-rbac-backup, 04-import-reconciliation, 06-ar-ap-statements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PostJournalEntry is the sole choke point every future posting path (imports, reversals, bank-rec) must call through — balance/period-lock/functional-category invariants enforced in one place, not duplicated per caller"
    - "Fail-fast ordering: pure in-memory balance check runs before any DB transaction is opened; DB-dependent checks (period lock, account type lookup) run inside the same *sql.Tx as the inserts so failures never partially write"
    - "accounting_periods is a single-row-by-convention (id=1) mutable settings table, upserted via ON CONFLICT DO UPDATE — explicitly NOT subject to the Plan 01-02 append-only immutability triggers since re-locking to extend the boundary is normal usage, not a correction to a posted ledger row"

key-files:
  created:
    - server/db/migrations/0004_periods.up.sql
    - server/db/migrations/0004_periods.down.sql
    - server/ledger/period.go
    - server/ledger/period_test.go
    - server/ledger/posting.go
    - server/ledger/posting_test.go
  modified: []

key-decisions:
  - "functional_category enforcement lives entirely in PostJournalEntry's Go validation (per-line account-type lookup), never as a schema-level NOT NULL — matches the anti-pattern warning explicit in both 01-RESEARCH.md and this plan's context block"
  - "getLockedThroughDateTx/lookupAccountType are transaction-scoped duplicates of the *sql.DB-scoped GetLockedThroughDate, kept small and unexported, so the period-lock and account-type checks inside PostJournalEntry read within the same atomic unit as the entry/line inserts"

requirements-completed: [LEDG-02, LEDG-04, FUND-03]

# Metrics
duration: 10min
completed: 2026-08-08
---

# Phase 1 Plan 3: Period Lock & PostJournalEntry Summary

**`PostJournalEntry` — the single sanctioned entry point for new journal entries — enforcing debit/credit balance, period-lock date boundaries, and functional-expense-category requirements atomically in one `*sql.Tx`, backed by a new single-row `accounting_periods` lock table.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-08-08 (continuing from Plan 01-02)
- **Completed:** 2026-08-08
- **Tasks:** 2
- **Files modified:** 6 created

## Accomplishments
- Migration `0004_periods.up.sql`: `accounting_periods` single-row-by-convention table (`id`, `locked_through_date`, `locked_by`, `locked_at`).
- `server/ledger/period.go`: `LockPeriod` (upsert via `ON CONFLICT(id) DO UPDATE`, supports re-locking to a later date) and `GetLockedThroughDate` (returns `nil` when nothing locked yet).
- `server/ledger/posting.go`: `PostJournalEntry` — the single function every future posting path (manual entry now, imports/reversals/bank-rec later) must call through. Enforces, in order: at least one line; `balances()` fail-fast in-memory check (sum debits == sum credits, non-zero); entry date strictly after the locked-through date (checked inside the transaction); every expense-type account line carries a `FunctionalCategory`. Entry + all lines insert in a single `*sql.Tx`, committed only if every check passes.
- `balances(lines []NewLine) bool` — pure helper, no DB access, runs before any transaction is opened.

## Task Commits

Both tasks followed strict TDD RED → GREEN:

1. **Task 1 (RED): failing tests for period lock schema + helpers** - `85980a5` (test)
   **Task 1 (GREEN): period lock read/write helpers** - `5b6b2b0` (feat)
2. **Task 2 (RED): failing tests for PostJournalEntry** - `72c7112` (test)
   **Task 2 (GREEN): implement PostJournalEntry** - `f19c699` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `server/db/migrations/0004_periods.{up,down}.sql` - `accounting_periods` table
- `server/ledger/period.go` - `LockPeriod`, `GetLockedThroughDate`
- `server/ledger/period_test.go` - `TestPeriodLock` (3 subtests: no-lock-nil, round-trip, re-lock-extends-boundary)
- `server/ledger/posting.go` - `NewLine`, `NewEntry`, `PostJournalEntry`, `balances`, plus unexported tx-scoped helpers `getLockedThroughDateTx`/`lookupAccountType`
- `server/ledger/posting_test.go` - `seedFixture`/`assertNoJournalRows` helpers, `TestPostJournalEntry_RejectsUnbalanced`, `TestPostJournalEntry_RejectsZeroLines`, `TestPostJournalEntry_PostsBalancedEntry`, `TestPeriodLock_PostingRejectsLockedDates`, `TestFunctionalExpenseRequired` (3 subtests)

## Decisions Made
- Kept the period-lock and account-type lookups as small tx-scoped unexported duplicates (`getLockedThroughDateTx`, `lookupAccountType`) rather than threading `*sql.Tx` vs `*sql.DB` through a shared generic helper — Go's `database/sql` has no common interface covering both cleanly without an extra abstraction the plan didn't call for; kept minimal per YAGNI.
- `PostJournalEntry` rejects zero-total balanced entries (all-zero-amount lines) via `balances()`, treating `debits == credits == 0` as invalid — not explicitly required by the plan's interface doc but directly implied by "an entry with zero lines is rejected" and the balance invariant; a zero-amount posting is never a meaningful transaction (Rule 2: missing validation, minimal scope).

## Deviations from Plan

None — plan executed exactly as written. The zero-total-balance rejection noted above is a minimal Rule 2 addition within `balances()`'s own scope, consistent with the plan's explicit "zero lines is rejected" test case and not a structural change.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `PostJournalEntry` is the only function in the codebase that inserts into `journal_entries`/`journal_lines` for new postings; `ReverseJournalEntry` (Plan 01-02) remains the only path for corrections. Together these cover the full posting lifecycle Phase 1 targets.
- Migration sequence is at `0004`; later phases (imports in Phase 4, bank-rec) should call `PostJournalEntry` directly rather than reimplementing balance/period-lock/functional-category checks.
- Full test suite (`go build ./...`, `go vet ./...`, `go test ./...`) green across all three plans in this phase.
- No blockers.

---
*Phase: 01-core-ledger-fund-accounting-data-model*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 6 created files verified present on disk. All 4 task commit hashes (85980a5, 5b6b2b0, 72c7112, f19c699) verified present in git log.
