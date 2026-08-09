---
phase: 02-auth-rbac-backup-dr-foundations
plan: 03
subsystem: database
tags: [sqlite, audit-log, immutability-triggers, ledger]

# Dependency graph
requires:
  - phase: 01-core-ledger-fund-accounting-data-model
    provides: journal_entries/journal_lines append-only trigger pattern (0003_journal migration) that this plan mirrors exactly for audit_log
  - phase: 02-auth-rbac-backup-dr-foundations (plan 02-01)
    provides: users table and the fact that PostedBy/postedBy/lockedBy parameters already carry server-side-resolved authenticated user IDs
provides:
  - append-only audit_log table with BEFORE UPDATE/DELETE RAISE(ABORT) triggers, database-enforced
  - server/audit.Write(tx, actorUserID, action, entityType, entityID, detail) helper, tx-scoped only
  - audit attribution wired into PostJournalEntry ("create"), ReverseJournalEntry ("reversal"), and LockPeriod ("lock_period")
affects: [phase-03-backup-dr, phase-04-import-reconciliation, any future ledger mutation entry point]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "New append-only tables mirror 0003_journal's paired BEFORE UPDATE/BEFORE DELETE RAISE(ABORT) trigger pattern rather than inventing new immutability mechanisms"
    - "audit.Write always takes the caller's *sql.Tx and never opens/commits/rolls back its own transaction, so the audit record is atomic with the action it documents"

key-files:
  created:
    - server/db/migrations/0007_audit_log.up.sql
    - server/db/migrations/0007_audit_log.down.sql
    - server/audit/audit.go
    - server/audit/audit_test.go
  modified:
    - server/ledger/posting.go
    - server/ledger/reversal.go
    - server/ledger/period.go
    - server/ledger/testutil_test.go

key-decisions:
  - "LockPeriod was restructured to open its own transaction (previously a bare db.Exec) so the settings write and its lock_period audit row commit atomically, matching the pattern already used by PostJournalEntry and ReverseJournalEntry"
  - "Only audit_log.actor_user_id gets a REFERENCES users(id) foreign key; journal_entries.posted_by and accounting_periods.locked_by remain plain integers as Phase 1 left them, per plan, to avoid disrupting existing Phase 1 fixtures"
  - "Seeded a single users row (id=1) in server/ledger/testutil_test.go's shared newTestDB helper rather than editing every existing PostedBy:1/lockedBy:1 call site across posting_test.go/reversal_test.go/period_test.go/balances_test.go, since the new audit_log FK made those hardcoded IDs need a real referenced row"

patterns-established:
  - "Pattern: append-only audit/ledger tables always get two single-event triggers (one BEFORE UPDATE, one BEFORE DELETE) each doing RAISE(ABORT, '<table> is append-only')"

requirements-completed: [AUTH-05]

# Metrics
duration: 12min
completed: 2026-08-09
---

# Phase 02 Plan 03: Immutable Audit Log Summary

**Append-only audit_log table (SQLite trigger-enforced) with a tx-scoped Write helper wired into PostJournalEntry, ReverseJournalEntry, and LockPeriod so every ledger mutation is attributed to a real authenticated user.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-09T13:52:00Z (approx, position 02-01 completion time in STATE.md as baseline)
- **Completed:** 2026-08-09
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- `audit_log` table with `BEFORE UPDATE`/`BEFORE DELETE` triggers that `RAISE(ABORT, 'audit_log is append-only')`, rejecting tampering at the database layer even via raw `db.Exec`, mirroring Phase 1's `journal_entries`/`journal_lines` pattern exactly
- `server/audit.Write` helper: single insert, always inside the caller's transaction, so the audit record and the action it describes commit or roll back together
- All three existing ledger mutation entry points (post, reverse, lock period) now write a correctly-attributed audit row before their own commit — `LockPeriod` was upgraded from a bare `db.Exec` to a proper transaction to make this atomic

## Task Commits

Each task was committed atomically:

1. **Task 1: audit_log schema, append-only triggers, and Write helper** - `e7ea81f` (feat, tdd)
2. **Task 2: Wire audit attribution into posting, reversal, and period lock** - `f347655` (feat, tdd)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `server/db/migrations/0007_audit_log.up.sql` - audit_log table, index, immutability triggers
- `server/db/migrations/0007_audit_log.down.sql` - reverse migration
- `server/audit/audit.go` - `Write(tx, actorUserID, action, entityType, entityID, detail)` helper
- `server/audit/audit_test.go` - `TestAuditImmutable` (DB-layer UPDATE/DELETE rejection) and `TestAuditAttribution` (end-to-end attribution across all three ledger entry points)
- `server/ledger/posting.go` - `audit.Write(tx, e.PostedBy, "create", "journal_entry", entryID, e.Memo)` before commit
- `server/ledger/reversal.go` - `audit.Write(tx, postedBy, "reversal", "journal_entry", newID, memo)` before returning
- `server/ledger/period.go` - restructured `LockPeriod` to use its own transaction; `audit.Write(tx, lockedBy, "lock_period", "accounting_period", 1, throughDate)`
- `server/ledger/testutil_test.go` - seeds a users row (id=1) in `newTestDB` so existing hardcoded actor IDs satisfy the new `audit_log.actor_user_id` foreign key

## Decisions Made
- `LockPeriod`'s signature is unchanged, but its internals now open/commit their own `*sql.Tx` instead of a bare `db.Exec`, for atomicity with the audit write. Callers are unaffected.
- Seeded the FK-satisfying user centrally in the shared ledger test helper instead of touching every individual test file that hardcodes `PostedBy: 1` / `lockedBy: 1` — keeps the diff small and avoids merge conflicts with the parallel 02-02 work touching `server/users`.
- `entity_id` for `lock_period` audit rows is hardcoded to `1`, matching `accounting_periods`' singleton-row design (always `id=1`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Seeded a users row in ledger test fixtures to satisfy the new audit_log foreign key**
- **Found during:** Task 2 (wiring audit calls into posting/reversal/period)
- **Issue:** `audit_log.actor_user_id` has a `REFERENCES users(id)` foreign key (enforced — `PRAGMA foreign_keys = ON`), but existing Phase 1 ledger tests hardcode `PostedBy: 1` / `lockedBy: 1` without any `users` row present. Once `audit.Write` started running inside `PostJournalEntry`/`ReverseJournalEntry`/`LockPeriod`, every one of those existing tests would fail with a foreign-key violation.
- **Fix:** Added a single `INSERT INTO users (id, username, ...) VALUES (1, ...)` to the shared `newTestDB` helper in `server/ledger/testutil_test.go`, run once per test DB right after migrations. This satisfies the FK for all existing call sites without touching each test file individually.
- **Files modified:** `server/ledger/testutil_test.go`
- **Verification:** `go test ./server/audit/... ./server/ledger/...` and full `go test ./...` both green after the change.
- **Committed in:** `f347655` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix, Rule 1)
**Impact on plan:** Necessary for correctness — the plan itself anticipated this exact FK-vs-existing-fixture tension ("only the new audit_log.actor_user_id column gets the REFERENCES users(id) FK... satisfied going forward") but didn't specify the test-fixture fix. No scope creep; confined to test infrastructure.

## Issues Encountered
None beyond the deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- AUTH-05 fully implemented and test-covered: every ledger mutation (post, reverse, lock period) now writes a database-enforced, tamper-proof audit row attributing the action to a real authenticated user.
- `server/audit.Write` is a stable, reusable helper — any future ledger/RBAC mutation entry point (e.g. Phase 4 import/reconciliation writes) should call it the same way, inside its own transaction.
- Full test suite (`go test ./...`) is green including the parallel 02-02 (user CRUD/RBAC) work, confirming no interference between the two plans' changes.
- No blockers for Phase 2's remaining plan(s) (backup/DR).

---
*Phase: 02-auth-rbac-backup-dr-foundations*
*Completed: 2026-08-09*

## Self-Check: PASSED

All created files and commit hashes verified present.
