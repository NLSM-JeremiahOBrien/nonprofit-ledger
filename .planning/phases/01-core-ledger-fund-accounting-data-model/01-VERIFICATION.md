---
phase: 01-core-ledger-fund-accounting-data-model
verified: 2026-08-08T00:00:00Z
status: passed
score: 8/8 must-haves verified
---

# Phase 1: Core Ledger & Fund-Accounting Data Model Verification Report

**Phase Goal:** Users can maintain a correct, immutable, fund-aware general ledger that forms the foundation of the entire system, running as a local webserver on the org's own hardware.
**Verified:** 2026-08-08
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can define a customizable chart of accounts with account types and parent/sub-account hierarchy | ✓ VERIFIED | `server/db/migrations/0001_accounts.up.sql` has `type CHECK (type IN ('asset','liability','equity','revenue','expense'))` and `parent_id INTEGER REFERENCES accounts(id)`. `server/ledger/coa.go` `CreateAccount` validates type against the 5-value enum and confirms parent existence before insert. Tested in `coa_test.go` (128 lines). |
| 2 | User can create funds classified per current FASB ASC 958 net-asset categories | ✓ VERIFIED | `0002_funds.up.sql` CHECK constrains `net_asset_class` to `without_donor_restrictions`/`with_donor_restrictions`. `server/ledger/funds.go` `CreateFund` validates the same two-value enum app-side, explicitly rejecting the deprecated 3-category model. Tested in `funds_test.go` (96 lines). |
| 3 | A posted transaction cannot be edited or deleted — corrections happen only via reversing/adjusting entries | ✓ VERIFIED | `0003_journal.up.sql` defines 4 `RAISE(ABORT...)` triggers (`journal_lines_no_update/delete`, `journal_entries_no_update/delete`). `server/ledger/reversal.go` `ReverseJournalEntry` only ever INSERTs (reads original read-only, inserts new entry with `reverses_entry_id` set, inserts swapped-sign lines) — never issues UPDATE/DELETE. `journal_test.go::TestImmutability` and `reversal_test.go::TestReverseJournalEntry` (231 lines) exercise this directly, including asserting trigger errors on attempted mutation. |
| 4 | Every transaction line carries a fund dimension so fund-level balances can always be derived | ✓ VERIFIED | `journal_lines.fund_id INTEGER NOT NULL REFERENCES funds(id)` — schema-enforced, not just convention. `journal_test.go::TestFundDimension` confirms NOT NULL rejection. |
| 5 | User can post a double-entry journal transaction tagged with a fund, and it is rejected unless debits equal credits | ✓ VERIFIED | `server/ledger/posting.go` `PostJournalEntry` calls `balances()` (sums debits/credits, requires equal and non-zero) before any DB write; wraps insert in a transaction rolled back on any failure. `posting_test.go::TestPostJournalEntry_RejectsUnbalanced/_RejectsZeroLines/_PostsBalancedEntry`. |
| 6 | User can close/lock an accounting period so transactions before that date can no longer be altered | ✓ VERIFIED | `0004_periods.up.sql` `accounting_periods.locked_through_date`; `server/ledger/period.go` `LockPeriod`/`GetLockedThroughDate`; `PostJournalEntry` checks `getLockedThroughDateTx` and rejects `EntryDate <= lockedThrough` inside the same transaction as the insert. `period_test.go` + `posting_test.go::TestPeriodLock_PostingRejectsLockedDates`. |
| 7 | User can view real-time balance for any fund, and fund balances always reconcile to the GL total | ✓ VERIFIED | `server/ledger/balances.go` `FundBalance`/`TrialBalance` computed purely via `SUM(debit_amount) - SUM(credit_amount)` against `journal_lines` at query time — no stored balance column (Anti-Pattern 3 explicitly avoided). `balances_test.go::TestFundBalanceReconciliation` (204 lines) posts multi-fund balanced entries via the real `PostJournalEntry` path, confirms `TrialBalance` sums to exactly zero, and confirms a `ReverseJournalEntry` restores pre-posting balance — full round-trip proof, not just schema-level. |
| 8 | The app runs entirely as a local webserver with no required third-party cloud dependency | ✓ VERIFIED | `cmd/server/main.go` opens local SQLite (`db.Open`), runs migrations, registers `/healthz`, calls `http.ListenAndServe` — no outbound calls, no external SDKs, no remote DNS anywhere in the startup path. `server/api/health.go` `HealthHandler` does zero external I/O. Verified via `lsof -a -p <pid> -i` showing exactly one `LISTEN` socket, zero outbound/established connections (documented in 01-04-SUMMARY.md, accepted per instructions as valid proxy for PLAT-01 given physical airplane-mode wasn't appropriate to trigger unilaterally in this environment). |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `server/db/migrations/0001_accounts.up.sql` | accounts table, type CHECK, parent_id self-ref | ✓ VERIFIED | Present, contains both required patterns |
| `server/db/migrations/0002_funds.up.sql` | funds table, net_asset_class CHECK | ✓ VERIFIED | Present, contains required pattern |
| `server/ledger/coa.go` | `CreateAccount` | ✓ VERIFIED | Exported, 84 lines, full validation logic, no stubs |
| `server/ledger/funds.go` | `CreateFund` | ✓ VERIFIED | Exported, 73 lines, full validation logic |
| `server/ledger/testutil_test.go` | `newTestDB` fixture | ✓ VERIFIED | Present, used across all `*_test.go` files |
| `server/db/migrations/0003_journal.up.sql` | journal_entries + journal_lines, immutability triggers, NOT NULL fund_id | ✓ VERIFIED | 4 `RAISE(ABORT` triggers present; `fund_id INTEGER NOT NULL REFERENCES funds` present |
| `server/ledger/reversal.go` | `ReverseJournalEntry` | ✓ VERIFIED | Exported, 105 lines, insert-only implementation |
| `server/ledger/posting.go` | `PostJournalEntry`, `balances` | ✓ VERIFIED | Both exported/present, 158 lines, enforces balance + lock + functional category in one transaction |
| `server/db/migrations/0004_periods.up.sql` | accounting_periods, locked_through_date | ✓ VERIFIED | Present, contains pattern |
| `server/ledger/period.go` | `LockPeriod`, `GetLockedThroughDate` | ✓ VERIFIED | Both exported, 51 lines |
| `server/ledger/balances.go` | `FundBalance`, `TrialBalance` | ✓ VERIFIED | Both exported, 65 lines, pure derived queries, no stored balance column |
| `cmd/server/main.go` | local webserver entrypoint, `http.ListenAndServe` | ✓ VERIFIED | Present, contains pattern, builds and runs |
| `server/api/health.go` | `HealthHandler` | ✓ VERIFIED | Exported, 20 lines, zero external I/O |

All 13 artifacts across the 4 plans: VERIFIED (exists, substantive, no stub markers).

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `coa.go` | accounts table | `INSERT INTO accounts` | ✓ WIRED | Present in `CreateAccount` |
| `funds.go` | funds table | `INSERT INTO funds` | ✓ WIRED | Present in `CreateFund` |
| `reversal.go` | `journal_entries.reverses_entry_id` | insert-only, sets on new row | ✓ WIRED | `reverses_entry_id` set on INSERT; original never mutated |
| `journal_lines.fund_id` | `funds.id` | NOT NULL FK | ✓ WIRED | Confirmed in schema and by `TestFundDimension` |
| `posting.go` | `period.go` | `GetLockedThroughDate` (tx variant) | ✓ WIRED | `getLockedThroughDateTx` called before insert, same transaction |
| `posting.go` | `journal_lines.functional_category` | expense-account line validation | ✓ WIRED | `lookupAccountType` + conditional check before insert |
| `balances.go` | `journal_lines` | `SUM(debit_amount) - SUM(credit_amount)` | ✓ WIRED | Present verbatim in both `FundBalance` and `TrialBalance` |
| `cmd/server/main.go` | `server/db/sqlite.go` | `db.Open` + `RunMigrations` at startup | ✓ WIRED | Present, called once before `ListenAndServe`, no outbound calls |

All 8 key links: WIRED.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| LEDG-01 | 01-01 | Customizable chart of accounts | ✓ SATISFIED | `coa.go`, `0001_accounts.up.sql` |
| LEDG-02 | 01-03 | Post balanced double-entry transactions | ✓ SATISFIED | `posting.go::PostJournalEntry` |
| LEDG-03 | 01-02 | Posted transactions immutable, reversal-only correction | ✓ SATISFIED | `0003_journal.up.sql` triggers, `reversal.go` |
| LEDG-04 | 01-03 | Close/lock accounting period | ✓ SATISFIED | `period.go`, `0004_periods.up.sql` |
| LEDG-05 | 01-02 | Every line carries fund dimension | ✓ SATISFIED | `fund_id NOT NULL` FK |
| FUND-01 | 01-01 | Funds classified per FASB ASC 958 | ✓ SATISFIED | `funds.go`, `0002_funds.up.sql` |
| FUND-02 | 01-04 | Real-time fund balance, reconciles to GL | ✓ SATISFIED | `balances.go`, `TestFundBalanceReconciliation` |
| FUND-03 | 01-03 | Functional category allocation | ✓ SATISFIED | `posting.go` functional_category enforcement |
| PLAT-01 | 01-04 | Local webserver, no cloud dependency | ✓ SATISFIED | `cmd/server/main.go`, `health.go`, lsof-verified in 01-04-SUMMARY.md |

No orphaned requirements — REQUIREMENTS.md maps exactly LEDG-01..05, FUND-01..03, PLAT-01 to Phase 1, and all 9 appear in plan frontmatter `requirements:` fields (01-01: LEDG-01, FUND-01; 01-02: LEDG-03, LEDG-05; 01-03: LEDG-02, LEDG-04, FUND-03; 01-04: FUND-02, PLAT-01). FUND-04 is correctly deferred to Phase 6 and out of scope here.

### Anti-Patterns Found

None. Grep for TODO/FIXME/XXX/HACK/PLACEHOLDER/"not implemented"/"coming soon" across `server/` and `cmd/` returned zero matches. No empty-return stub patterns found. `go build ./...` and `go vet ./...` both clean. `go test ./...` passes (server/ledger package, the only one with tests, `ok`).

### Human Verification Required

None required for this phase. All observable truths are verifiable programmatically via schema inspection, code review, and passing automated tests. The one item that would normally require human/manual verification — PLAT-01's offline network isolation — was verified via `lsof` socket inspection (documented in 01-04-SUMMARY.md) and is treated as a valid, equivalent verification method per project convention (physical airplane-mode toggling not appropriate to trigger unilaterally in this environment).

### Gaps Summary

No gaps found. All 8 observable truths verified, all 13 artifacts substantive and wired, all 8 key links confirmed, all 9 requirement IDs accounted for with no orphans, zero anti-patterns, full test suite green (1171 lines of test code across 8 test files), clean build/vet. Phase 1 goal is fully achieved.

---

*Verified: 2026-08-08*
*Verifier: Claude (gsd-verifier)*
