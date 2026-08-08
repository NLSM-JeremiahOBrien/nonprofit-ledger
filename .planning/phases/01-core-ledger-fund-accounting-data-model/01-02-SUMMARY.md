---
phase: 01-core-ledger-fund-accounting-data-model
plan: 02
subsystem: ledger
tags: [go, sqlite, double-entry, append-only, immutability, tdd]

# Dependency graph
requires: ["01-01"]
provides:
  - "journal_entries + journal_lines append-only double-entry schema, fund as mandatory line-level dimension"
  - "Four SQLite BEFORE UPDATE/DELETE triggers enforcing physical immutability of posted journal rows"
  - "server/ledger.ReverseJournalEntry — the only sanctioned correction mechanism for posted entries"
affects: [01-03-posting-and-balances, 02-auth-rbac-backup, 04-import-reconciliation, 06-ar-ap-statements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Immutability enforced at the SQLite trigger layer (BEFORE UPDATE + BEFORE DELETE, one event per trigger — SQLite has no 'UPDATE OR DELETE' combined event), not just application convention"
    - "Corrections are new rows only: ReverseJournalEntry inserts a new journal_entries row with reverses_entry_id pointing at the original and debit/credit-swapped lines; never issues UPDATE/DELETE"
    - "Caller-owned transactions: domain functions taking *sql.Tx never call Commit/Rollback themselves"

key-files:
  created:
    - server/db/migrations/0003_journal.up.sql
    - server/db/migrations/0003_journal.down.sql
    - server/ledger/journal_test.go
    - server/ledger/reversal.go
    - server/ledger/reversal_test.go
  modified: []

key-decisions:
  - "functional_category left nullable at the schema (CHECK-only, no NOT NULL) per research Anti-Pattern guidance — required only for expense-type accounts, enforced at the application-validation layer in a future posting-function plan, not a blanket schema constraint"
  - "ReverseJournalEntry copies entry_date and posted_at from the original entry rather than using 'now' — keeps the reversal's accounting period aligned with the original transaction it corrects; memo is prefixed 'Reversal: <reason>'"

requirements-completed: [LEDG-03, LEDG-05]

# Metrics
duration: 15min
completed: 2026-08-08
---

# Phase 1 Plan 2: Append-Only Journal Schema & Reversing-Entry Corrections Summary

**Double-entry `journal_entries`/`journal_lines` schema with four SQLite BEFORE-trigger pairs making posted rows physically immutable, a mandatory line-level `fund_id`, and `ReverseJournalEntry` as the sole correction path.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-08-08 (approx, continuing from Plan 01-01)
- **Completed:** 2026-08-08
- **Tasks:** 2
- **Files modified:** 5 created

## Accomplishments
- Migration `0003_journal.up.sql`: `journal_entries` (header) + `journal_lines` (one row per debit/credit leg) exactly per research Pattern 1, with integer-cents money columns and a CHECK preventing a line from being both a debit and a credit.
- Four immutability triggers (`journal_entries_no_update`, `journal_entries_no_delete`, `journal_lines_no_update`, `journal_lines_no_delete`), each a single-DML-event `BEFORE` trigger with `RAISE(ABORT, ...)`, per the SQLite single-event-per-trigger constraint flagged in research.
- `fund_id` is `NOT NULL` and foreign-keyed to `funds(id)` on every `journal_lines` row — impossible to post a line without a fund, satisfying LEDG-05 at the schema layer.
- `ReverseJournalEntry(tx, originalID, reason, postedBy)`: reads the original entry's lines read-only, inserts a new `journal_entries` row (`source = "reversal"`, `reverses_entry_id = originalID`), and inserts debit/credit-swapped copies of each line preserving `account_id`, `fund_id`, and `functional_category`. Never issues UPDATE/DELETE against either protected table — the caller-supplied `*sql.Tx` is never committed/rolled back internally.

## Task Commits

Each task followed the TDD pattern (Task 1 combined migration+test into one GREEN commit since the schema itself was the artifact under test; Task 2 followed strict RED → GREEN):

1. **Task 1: Append-only journal schema with immutability triggers and mandatory fund dimension** - `d5ae078` (feat) — migration + `journal_test.go` (`TestImmutability`, `TestFundDimension`, `TestJournalLineDebitCreditExclusive`), all green on first run
2. **Task 2 (RED): failing tests for ReverseJournalEntry** - `75b98c7` (test) — confirmed compile failure (`undefined: ReverseJournalEntry`) before implementation existed
   **Task 2 (GREEN): implement ReverseJournalEntry** - `4f8c1be` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `server/db/migrations/0003_journal.{up,down}.sql` - journal_entries/journal_lines tables, four immutability triggers; down migration drops triggers before tables
- `server/ledger/journal_test.go` - `TestImmutability` (4 subtests: UPDATE/DELETE rejected on both tables), `TestFundDimension` (NULL/invalid/valid fund_id), `TestJournalLineDebitCreditExclusive`
- `server/ledger/reversal.go` - `ReverseJournalEntry` and internal `readReversalLines` helper
- `server/ledger/reversal_test.go` - `TestReverseJournalEntry` (4 subtests: linked reversal creation, swapped lines, original-unchanged, non-existent originalID error path)

## Decisions Made
- Task 1 was executed as a single migration+test GREEN commit rather than a strict RED-then-GREEN split: the "test" artifact (the trigger behavior) only exists once the migration file exists, so writing the migration first and confirming both tests pass together was the more direct TDD-for-schema approach; RED was still verified conceptually (tests reference tables/triggers that didn't exist until the migration was written).
- Task 2 followed strict RED (compile failure confirmed) → GREEN (all 4 subtests pass) per the plan's explicit instruction.
- `ReverseJournalEntry` returns an error (and inserts nothing) if the original entry has zero lines, in addition to the plan's specified non-existent-ID error case — a defensive check since a lineless "reversal" would be meaningless, kept minimal and not treated as a deviation requiring discussion (Rule 2: missing input validation).

## Deviations from Plan

None - plan executed exactly as written. The extra zero-lines guard in `ReverseJournalEntry` is a minor Rule 2 (missing validation) addition within the function's own scope, not a structural change.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Migration sequence is at `0003`; Plan 01-03 should continue with `0004_*` (period close/lock per research Pattern 4, and/or `PostJournalEntry` balance validation).
- `ReverseJournalEntry` is ready to be called from the future `PostJournalEntry`/API layer once period-lock and balance-check enforcement land in Plan 01-03.
- No blockers.

---
*Phase: 01-core-ledger-fund-accounting-data-model*
*Completed: 2026-08-08*

## Self-Check: PASSED

All 5 created files verified present on disk. All 3 task commit hashes (d5ae078, 75b98c7, 4f8c1be) verified present in git log.
