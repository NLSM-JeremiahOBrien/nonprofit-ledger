# Phase 1: Core Ledger & Fund-Accounting Data Model - Research

**Researched:** 2026-08-08
**Domain:** Double-entry general ledger schema design, fund accounting (FASB ASC 958), append-only/immutable data modeling in SQLite, Go backend
**Confidence:** HIGH (schema/immutability patterns, FASB terminology); MEDIUM (Go-specific idioms — synthesized from general Go/SQLite research, not a single canonical open-source reference)

## Summary

Phase 1 builds the foundation everything else depends on: a double-entry general ledger where every transaction line carries both an account and a fund dimension, where posted rows are physically immutable (enforced at the SQLite layer, not just in application code), and where corrections happen exclusively via reversing entries. This phase also requires getting nonprofit-specific accounting terminology exactly right from day one, since UI labels and enum values are expensive to retrofit onto historical data later.

The critical finding from this research pass: **the FASB ASC 958 net-asset model that applies today (and has applied since fiscal years beginning after Dec 15, 2017) is the two-category model** — "net assets without donor restrictions" and "net assets with donor restrictions" — not the older three-category model (unrestricted / temporarily restricted / permanently restricted). This is confirmed independently by both FASB ASU 2016-14 and the current IRS Form 990 Part X instructions (Line 27 = without donor restrictions, Line 28 = with donor restrictions). The fund classification enum, database columns, and UI labels should use this terminology throughout. "Permanently restricted" and "temporarily restricted" survive only as informal/internal sub-classifications a fund can optionally carry (e.g., for tracking endowment vs. time-restricted grants) — they are not separate balance-sheet categories anymore.

The second critical finding: SQLite's `CREATE TRIGGER` syntax takes exactly one DML event per trigger (`BEFORE UPDATE`, `BEFORE DELETE`, or `BEFORE INSERT` — not `BEFORE UPDATE OR DELETE`). Two separate triggers are needed per protected table to block both UPDATE and DELETE. This is a small but load-bearing detail for the immutability enforcement task.

**Primary recommendation:** Model the ledger as an append-only `journal_entries` (header) + `journal_lines` (one row per debit/credit leg, each carrying `account_id`, `fund_id`, `functional_category`, `debit`/`credit` amount) pair of tables, enforce immutability with per-table SQLite triggers blocking UPDATE/DELETE on posted rows, enforce debit=credit balance and fund-integrity at the application-transaction boundary (wrapped in a single SQLite transaction), and use "without donor restrictions" / "with donor restrictions" as the two fund/net-asset category values with functional expense categories of "program services", "management and general", and "fundraising" per Form 990 Part IX.

## User Constraints

No CONTEXT.md exists for this phase (not run through `/gsd:discuss-phase`). No locked decisions, discretion notes, or deferred ideas to carry forward beyond what's already in PROJECT.md/STATE.md/REQUIREMENTS.md (already reflected in this research). Proceed using STACK.md/ARCHITECTURE.md/PITFALLS.md project-level research as binding technical direction (Go + SQLite + WAL, append-only ledger, fund as first-class dimension).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| LEDG-01 | Customizable chart of accounts (asset/liability/equity/revenue/expense, parent/sub hierarchy, active/inactive) | See Architecture Patterns — COA table design; Standard Stack — `golang-migrate` for schema evolution |
| LEDG-02 | Double-entry journal transactions must balance (debits=credits) before posting | See Pattern 1 (append-only ledger) and Code Examples — balance-check enforced in a single DB transaction, both at app layer and via CHECK/trigger |
| LEDG-03 | Posted transactions immutable — corrections via reversing entries only, never edit/delete | See Pattern 1 and Code Examples — SQLite BEFORE UPDATE/DELETE triggers with RAISE(ABORT, ...) |
| LEDG-04 | Period close/lock — posted transactions before lock date cannot be altered | See Architecture Patterns — period/lock table design; Common Pitfalls — must be enforced at posting-time validation, not just UI |
| LEDG-05 | Every transaction line carries a fund dimension so fund-level balances are always derivable | See Standard Stack — schema puts `fund_id` on `journal_lines`, not `journal_entries`; Don't Hand-Roll — balances always derived, never stored |
| FUND-01 | Funds classified per current FASB ASC 958 net-asset categories | See State of the Art — verified two-category model (with/without donor restrictions), current as of ASU 2016-14 and 2025 Form 990 instructions |
| FUND-02 | Real-time fund balance view; fund balances reconcile to GL total | See Don't Hand-Roll — derived balance projections, rebuild-and-verify pattern |
| FUND-03 | Expense transactions allocated across functional categories (program/M&G/fundraising) | See State of the Art — Form 990 Part IX functional expense categories; Architecture Patterns — `functional_category` as required field on expense journal lines |
| PLAT-01 | Runs as a local webserver on org's own hardware, no required outbound cloud dependency for core accounting | Already satisfied by project-level STACK.md (Go single binary + embedded SQLite); Phase 1 just needs to not introduce any cloud dependency in the ledger/schema layer — no action beyond what's already decided |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.x | Backend language, ledger domain engine | Already locked in project STACK.md; single static binary, no separate runtime |
| `modernc.org/sqlite` | latest (tracks SQLite 3.46+) | Embedded database driver (pure Go, no CGo) | Already locked in project STACK.md; required for cross-compilation to NAS/ARM targets |
| `golang-migrate/migrate` v4 | v4.x | Schema migrations | Already locked in project STACK.md; never hand-roll migrations for a financial schema — this phase creates the first migration files and establishes the migration workflow for the whole project |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `database/sql` (Go stdlib) | stdlib | Transaction boundary for posting | Wrap every journal-entry post (header + all lines) in a single `sql.Tx`; commit only if balanced and fund rules pass |
| SQLite `PRAGMA foreign_keys = ON` | — | Referential integrity | Must be explicitly enabled per-connection in SQLite (off by default) — required so `account_id`/`fund_id` foreign keys on journal lines are actually enforced |
| SQLite `PRAGMA journal_mode = WAL` | — | Write-ahead logging | Already decided in STACK.md; set at connection-open time, persists in the DB file after first set |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Two-table (entries + lines) schema | Single flat table with one row per debit or credit (Ledger-cli/Beancount style) | Flat single-table is simpler but makes "get me both legs of transaction X" and header-level metadata (memo, posted-by, source) awkward; two-table header+lines is the conventional double-entry-system schema and matches how QuickBooks/GnuCash/most GL systems model it — recommended |
| SQLite triggers for immutability | Application-layer-only enforcement (no DB trigger) | Rejected — Pitfall 1 from project research explicitly calls out that app-only enforcement is not defense-in-depth; a future direct-DB-access bug, ad hoc SQL script, or migration mistake could silently mutate posted rows with no trigger to stop it |
| Derived/computed balances only, no cache table | Persistent mutable `balance` column on accounts/funds | Rejected as the source of truth per Pitfall 1 and Anti-Pattern 3 in project ARCHITECTURE.md; a cached/materialized balance table is fine for read performance as long as it's clearly documented as rebuildable and periodically verified against the journal, never treated as authoritative |

**Installation:**
```bash
go get modernc.org/sqlite
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/sqlite
go get github.com/golang-migrate/migrate/v4/source/file
```

## Architecture Patterns

### Recommended Project Structure (Phase 1 slice)

```
server/
├── ledger/                    # core domain engine — no I/O, pure accounting logic
│   ├── posting.go             # PostJournalEntry: validates balance, fund rules, period lock
│   ├── reversal.go            # ReverseJournalEntry: creates a new offsetting entry
│   ├── coa.go                 # Chart of accounts model + validation
│   ├── funds.go                # Fund model, net-asset classification enum
│   └── period.go               # Period close/lock logic
├── db/
│   ├── migrations/
│   │   ├── 0001_accounts.up.sql
│   │   ├── 0002_funds.up.sql
│   │   ├── 0003_journal.up.sql        # journal_entries + journal_lines + immutability triggers
│   │   ├── 0004_periods.up.sql
│   │   └── 0005_balance_projections.up.sql
│   └── sqlite.go                # connection setup: WAL, foreign_keys pragma
└── api/
    └── ledger.go               # thin HTTP handlers calling into server/ledger
```

### Pattern 1: Header + Lines Double-Entry Schema, Fund as a Line-Level Dimension

**What:** `journal_entries` holds one row per transaction (date, memo, posted-by, source, status). `journal_lines` holds one row per debit/credit leg, each with `account_id`, `fund_id`, `functional_category` (nullable except for expense-type accounts), `debit_amount`, `credit_amount` (use two non-negative columns, not a signed single amount — this makes the debit=credit SUM check trivial and matches how every real GL schema does it).

**When to use:** This phase, as the foundational schema — every other phase's data (AR/AP, bank rec, imports, statements) ultimately reads from or writes into `journal_lines`.

**Example:**
```sql
-- Source: synthesized from double-entry ledger design literature (Modern Treasury,
-- Formance, pgledger) + standard nonprofit fund-accounting field requirements
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,          -- e.g. "1000", "4100"
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('asset','liability','equity','revenue','expense')),
    parent_id INTEGER REFERENCES accounts(id),
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE funds (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    net_asset_class TEXT NOT NULL
        CHECK (net_asset_class IN ('without_donor_restrictions','with_donor_restrictions')),
    restriction_detail TEXT,             -- optional free-text/sub-type, e.g. "endowment", "time-restricted"
    is_active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE journal_entries (
    id INTEGER PRIMARY KEY,
    entry_date TEXT NOT NULL,            -- ISO 8601 date
    memo TEXT,
    posted_by INTEGER NOT NULL,          -- user id (FK once auth exists in Phase 2)
    posted_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual', -- manual | import:<adapter> | reversal
    reverses_entry_id INTEGER REFERENCES journal_entries(id)
);

CREATE TABLE journal_lines (
    id INTEGER PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES journal_entries(id),
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    fund_id INTEGER NOT NULL REFERENCES funds(id),
    functional_category TEXT
        CHECK (functional_category IN ('program','management_general','fundraising')),
    debit_amount INTEGER NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),  -- integer cents
    credit_amount INTEGER NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    CHECK (NOT (debit_amount > 0 AND credit_amount > 0))  -- a line is a debit OR a credit, not both
);
```

**Note on amounts:** store money as integer cents (`INTEGER`), never `REAL`/float — SQLite has no fixed-point decimal type and floating-point rounding errors are unacceptable in a ledger. This is standard practice, not unique to this project, but worth stating explicitly since it's easy to default to `REAL`.

### Pattern 2: Immutability via Paired BEFORE Triggers (not "OR")

**What:** SQLite's `CREATE TRIGGER` syntax accepts exactly one DML event per trigger definition (`INSERT`, `UPDATE [OF columns]`, or `DELETE` — not a combined `UPDATE OR DELETE`). Verified against the official SQLite `CREATE TRIGGER` documentation. Two separate triggers are required per protected table.

**When to use:** On `journal_entries` and `journal_lines` (and eventually any other posted/immutable table, e.g. reconciliation approvals in later phases).

**Example:**
```sql
-- Source: SQLite official docs (sqlite.org/lang_createtrigger.html) — RAISE(ABORT, ...) pattern,
-- adapted to two separate triggers since SQLite doesn't support "UPDATE OR DELETE" as one event
CREATE TRIGGER journal_lines_no_update
BEFORE UPDATE ON journal_lines
BEGIN
    SELECT RAISE(ABORT, 'journal_lines is append-only: posted lines cannot be updated');
END;

CREATE TRIGGER journal_lines_no_delete
BEFORE DELETE ON journal_lines
BEGIN
    SELECT RAISE(ABORT, 'journal_lines is append-only: posted lines cannot be deleted');
END;

-- Same pair repeated for journal_entries
CREATE TRIGGER journal_entries_no_update
BEFORE UPDATE ON journal_entries
BEGIN
    SELECT RAISE(ABORT, 'journal_entries is append-only: posted entries cannot be updated');
END;

CREATE TRIGGER journal_entries_no_delete
BEFORE DELETE ON journal_entries
BEGIN
    SELECT RAISE(ABORT, 'journal_entries is append-only: posted entries cannot be deleted');
END;
```

SQLite docs note BEFORE triggers that themselves modify the row being changed leave the outcome of the original statement "undefined" — not a concern here since these triggers only `RAISE(ABORT, ...)` and never touch `NEW`/`OLD` rows, but AFTER triggers are the documented general preference when a trigger needs to inspect post-change state. For a pure abort-on-any-attempt trigger, BEFORE is fine and is the pattern used in the reference sources for this exact use case.

**Migration-time caveat:** these triggers will also block `golang-migrate` schema migrations that try to `ALTER`/backfill data in these tables later. Plan for migrations that need to touch historical journal data (rare, but e.g. a bug-fix backfill) to explicitly `DROP TRIGGER` / recreate as part of the migration, with that migration reviewed with extra scrutiny — this is a deliberate, logged exception path, not a routine one.

### Pattern 3: Correction via Reversing Entry, Never Edit

**What:** A "correction" creates a brand-new `journal_entries` row with `reverses_entry_id` pointing at the original, and lines that are the original's lines with debit/credit swapped. The original stays untouched and both are visible in history.

**When to use:** Any time a posted transaction needs to be fixed (wrong account, wrong fund, wrong amount).

**Example:**
```go
// Source: pattern synthesized from project ARCHITECTURE.md Pattern 1 + Modern Treasury
// "Enforcing Immutability in your Double-Entry Ledger" (moderntreasury.com/journal)
func ReverseJournalEntry(tx *sql.Tx, originalID int64, reason string, postedBy int64) (int64, error) {
    lines, err := getLines(tx, originalID) // read-only fetch of original lines
    if err != nil {
        return 0, err
    }
    newID, err := insertEntry(tx, entryFields{
        Memo:            "Reversal: " + reason,
        PostedBy:        postedBy,
        Source:          "reversal",
        ReversesEntryID: &originalID,
    })
    if err != nil {
        return 0, err
    }
    for _, l := range lines {
        // swap debit/credit — same account, same fund, same functional category
        if err := insertLine(tx, newID, l.AccountID, l.FundID, l.FunctionalCategory,
            l.CreditAmount, l.DebitAmount); err != nil {
            return 0, err
        }
    }
    return newID, nil
}
```

### Pattern 4: Period Close/Lock Enforced at Posting Time, Not Just UI

**What:** A `periods` table (or a single `locked_through_date` setting) records the latest date before which no new postings or reversals may be entered. Enforcement lives in the same `PostJournalEntry` function every entry (manual, reversal, or future import-promotion) goes through — never only a UI-level disabled button.

**When to use:** LEDG-04. Design this now even though period-close UI/workflow polish may be minimal in Phase 1 — the enforcement point (reject any entry dated on/before the lock date) must exist in the domain engine from day one, since later phases (import, bank rec) will post through the same function and must respect it too.

**Example:**
```sql
CREATE TABLE accounting_periods (
    id INTEGER PRIMARY KEY,
    locked_through_date TEXT,   -- NULL = nothing locked yet
    locked_by INTEGER,
    locked_at TEXT
);
```
```go
func PostJournalEntry(tx *sql.Tx, e NewEntry) (int64, error) {
    lockedThrough, _ := getLockedThroughDate(tx)
    if lockedThrough != nil && e.EntryDate <= *lockedThrough {
        return 0, ErrPeriodLocked
    }
    if !balances(e.Lines) { // sum(debits) == sum(credits), per currency
        return 0, ErrUnbalanced
    }
    // ... insert entry + lines inside tx
}
```

### Anti-Patterns to Avoid

- **Signed single `amount` column instead of separate `debit_amount`/`credit_amount`:** makes the balance check and account-type sign conventions ambiguous and error-prone; use two non-negative columns.
- **`fund_id` on `journal_entries` (header) instead of `journal_lines`:** a single transaction can legitimately touch multiple funds (e.g., an allocation entry moving cost between two grants) — fund must be a line-level dimension per LEDG-05, not a header-level one.
- **Storing money as `REAL`:** floating-point rounding errors are unacceptable for audited financial data; use integer cents.
- **Treating `functional_category` as required on every line:** it's meaningful for expense-type accounts (Form 990 Part IX) but not for balance-sheet accounts (assets/liabilities) — make it required at the application-validation layer specifically for expense-account lines, not a blanket NOT NULL column constraint that would break non-expense postings.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Schema versioning | Custom "run these SQL files in order" script | `golang-migrate/migrate` v4 | Already decided in project STACK.md; hand-rolled migration runners are a common source of subtle drift between dev/prod schemas, unacceptable for financial data |
| Row immutability | Only application-layer "don't call UPDATE" convention | SQLite `BEFORE UPDATE`/`BEFORE DELETE` triggers with `RAISE(ABORT, ...)` | Defense in depth — a future bug, ad hoc script, or even a well-intentioned migration must be physically prevented from mutating posted history, not just discouraged by code review |
| Current balances | Mutable `balance` column updated in place | `SUM(debit_amount) - SUM(credit_amount)` derived query, optionally cached in a clearly-labeled, rebuildable projection table | Anti-Pattern 3 in project ARCHITECTURE.md; the only way to guarantee balances are provably correct against the audit trail |
| Money math | Floats or hand-rolled decimal string parsing | Integer cents (`INTEGER` column, convert to display currency only at the presentation layer) | Standard practice for financial software; avoids floating-point rounding entirely |

**Key insight:** every "don't hand-roll" item above is really the same principle applied at a different layer: the journal is the single source of truth, and every other representation of the data (balances, statements, projections) must be mechanically derivable from it and never independently mutable.

## Common Pitfalls

(Consolidated from project-level PITFALLS.md, scoped to what Phase 1 must specifically address — see that file for the full write-up including warning signs and recovery costs.)

### Pitfall 1: Mutable ledger rows / stored balances
**What goes wrong:** Storing a `balance` column that's incremented in place, or allowing UPDATE/DELETE on posted rows, silently breaks the audit trail.
**How to avoid:** Append-only schema (Pattern 1) + DB-level triggers (Pattern 2) + always-derived balances (Don't Hand-Roll table).
**Warning signs:** Any code path doing `UPDATE journal_lines SET ...`; a `balance` column with no rebuild/verify job.

### Pitfall 7: Fund/functional-expense modeling bolted on later
**What goes wrong:** Building "a nice ledger" first and adding fund/restriction tracking later means historical transactions can never be accurately retrofitted.
**How to avoid:** `fund_id` and (for expense lines) `functional_category` are schema-level, non-optional dimensions from the very first migration in this phase — not added in a later phase.
**Warning signs:** A ledger schema with `account_id` but no `fund_id`; financial-statement generation treated as a distant future phase rather than an early correctness check against the schema (recommend a smoke-test query in this phase that computes a trial balance sheet split by net-asset class, even before Phase 6 builds the real statement UI).

### Pitfall (new, this pass): SQLite single-event trigger syntax
**What goes wrong:** Assuming `BEFORE UPDATE OR DELETE ON table` is valid SQLite syntax (it reads naturally but SQLite's grammar only accepts one DML event per `CREATE TRIGGER`) leads to either a migration failure or, worse, a trigger that silently only covers one of the two operations if the syntax is coerced incorrectly.
**Why it happens:** Other DB engines (and English) make "UPDATE OR DELETE" read as one natural clause; SQLite's docs page even shows single-event examples that are easy to skim past.
**How to avoid:** Always write two triggers per protected table, one `BEFORE UPDATE`, one `BEFORE DELETE`, as shown in Pattern 2. Verify both are present with a migration test (attempt an UPDATE, attempt a DELETE, confirm both are rejected — matches the "Looks Done But Isn't" checklist item in project PITFALLS.md).
**Warning signs:** A migration file with only one trigger per table, or a trigger definition that fails to apply and gets silently skipped.

## Code Examples

See Architecture Patterns section above for the full worked SQL schema, trigger pair, reversal function, and period-lock enforcement — all sourced from official SQLite docs plus project-level ARCHITECTURE.md/PITFALLS.md patterns.

### Enforcing balance (debits == credits) at commit time

```go
// Source: standard double-entry validation pattern (Formance "Defining Double-Entry
// Accounting: A Formal Model for Engineers"; project ARCHITECTURE.md Pattern 1)
func balances(lines []NewLine) bool {
    var totalDebit, totalCredit int64
    for _, l := range lines {
        totalDebit += l.DebitAmount
        totalCredit += l.CreditAmount
    }
    return totalDebit == totalCredit && totalDebit > 0
}
```

Enforce this both in Go (fail fast with a clear error) and, if practical, as a SQLite deferred-constraint-style check within the same transaction (SQLite doesn't support statement-level CHECK across rows directly, so the practical enforcement point is the application transaction wrapper — document this as an application-layer, not schema-layer, invariant, unlike the per-row CHECK constraints which the schema does enforce).

## State of the Art

| Old Approach (pre-2018) | Current Approach (now) | When Changed | Impact |
|--------------------------|--------------------------|---------------|--------|
| Three net-asset classes: unrestricted, temporarily restricted, permanently restricted | Two net-asset classes: without donor restrictions, with donor restrictions | FASB ASU 2016-14, effective for fiscal years beginning after Dec 15, 2017 (all nonprofits have been under this model for years by 2026) | Fund classification enum, Form 990 Part X (lines 27-28 replace old lines 27-29), and all UI labels must use the two-category model; "temporarily/permanently restricted" survive only as an optional descriptive sub-type on a "with donor restrictions" fund, not a distinct balance-sheet category |

**Deprecated/outdated:**
- "Unrestricted net assets" / "temporarily restricted net assets" / "permanently restricted net assets" as balance-sheet-category labels — replaced by "without donor restrictions" / "with donor restrictions" (ASU 2016-14). Any tutorial, older textbook, or generic nonprofit-accounting content referencing the three-category model is describing the pre-2018 standard and should not be used as a terminology source for this project.

## Open Questions

1. **Should `restriction_detail` (endowment vs. time-restricted vs. purpose-restricted) be a free-text field or its own controlled enum in Phase 1?**
   - What we know: Form 990/ASC 958 only require the two-category split at the balance-sheet level; sub-classification of restriction type is informational/disclosure-level, not structurally required.
   - What's unclear: Whether O'Brien's grants (Phase 6, FUND-04) will need a more structured restriction-type field for reporting, which would be much cheaper to add now than retrofit later.
   - Recommendation: Add `restriction_detail TEXT` as a free-text/nullable field now (low cost, per Pattern 1 schema above); revisit as a proper enum only if Phase 6 grant-tracking research surfaces a concrete reporting need for it.

2. **Multi-currency support?**
   - What we know: Not mentioned anywhere in PROJECT.md/REQUIREMENTS.md; O'Brien is a US-based museum with USD-only operations.
   - What's unclear: Nothing — out of scope.
   - Recommendation: Design the schema single-currency (integer cents, no currency column) for v1; do not add currency-handling complexity that has no requirement driving it.

3. **Should account `type` support the more granular sub-types some GL systems use (e.g., "current asset" vs "fixed asset") or just the five top-level GAAP types?**
   - What we know: LEDG-01 only requires "asset/liability/equity/revenue/expense types" plus parent/sub-account hierarchy.
   - What's unclear: Whether Statement of Financial Position generation (Phase 6) will need finer-grained typing to correctly bucket accounts into current/long-term sections.
   - Recommendation: Use the five top-level types now (matches LEDG-01 literally) and rely on the parent/sub-account hierarchy plus account `code` numbering (standard COA numbering convention: 1000s=assets, 2000s=liabilities, etc.) to handle finer distinctions; revisit only if Phase 6 statement research finds this insufficient.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package (no external test framework needed for Go) |
| Config file | none yet — greenfield project, Wave 0 must scaffold `go.mod`, `server/ledger/*_test.go` structure |
| Quick run command | `go test ./server/ledger/... -run TestXxx -v` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LEDG-01 | Chart of accounts supports type/hierarchy/active flag | unit | `go test ./server/ledger/... -run TestChartOfAccounts -v` | ❌ Wave 0 |
| LEDG-02 | Unbalanced entry rejected before posting | unit | `go test ./server/ledger/... -run TestPostJournalEntry_RejectsUnbalanced -v` | ❌ Wave 0 |
| LEDG-03 | UPDATE/DELETE on posted journal rows is rejected by DB trigger | integration (real SQLite file, not mock) | `go test ./server/ledger/... -run TestImmutability -v` | ❌ Wave 0 |
| LEDG-04 | Posting on/before locked-through date is rejected | unit | `go test ./server/ledger/... -run TestPeriodLock -v` | ❌ Wave 0 |
| LEDG-05 | Every journal line requires a valid fund_id; fund balances derivable via SUM query | unit + integration | `go test ./server/ledger/... -run TestFundDimension -v` | ❌ Wave 0 |
| FUND-01 | Fund `net_asset_class` CHECK constraint only accepts the two ASC 958 values | integration (schema-level) | `go test ./server/db/... -run TestFundsSchema -v` | ❌ Wave 0 |
| FUND-02 | Fund balance query result matches SUM(journal_lines) for that fund; total across funds reconciles to GL trial balance | integration | `go test ./server/ledger/... -run TestFundBalanceReconciliation -v` | ❌ Wave 0 |
| FUND-03 | Expense-type journal lines require a functional_category value | unit | `go test ./server/ledger/... -run TestFunctionalExpenseRequired -v` | ❌ Wave 0 |
| PLAT-01 | Server starts and serves without any outbound network call in its startup path | manual-only (smoke test, not worth automating in Phase 1) | n/a — verify via `go run ./cmd/server` with network disabled/airplane mode during manual QA | n/a |

### Sampling Rate
- **Per task commit:** `go test ./server/ledger/... -v` (quick run, seconds)
- **Per wave merge:** `go test ./... -v` (full suite)
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `go.mod` / module scaffolding — no Go module exists yet in this repo
- [ ] `server/db/sqlite.go` — connection setup with WAL + foreign_keys pragmas, needed by every test
- [ ] `server/db/migrations/0001..0005_*.sql` — the schema itself (this phase's primary deliverable, not a pre-req, but test files depend on it existing)
- [ ] `server/ledger/testutil_test.go` — shared test fixture: spin up a temp SQLite file, run migrations, return a `*sql.DB` for each test
- [ ] Test framework install: none needed beyond Go stdlib; `golang-migrate` CLI useful for dev (`go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

## Sources

### Primary (HIGH confidence)
- [FASB ASU 2016-14 — Not-for-Profit Entities (Topic 958)](https://storage.fasb.org/ASU_2016-14.pdf) — official FASB standard, confirms two-category net-asset model
- [SQLite CREATE TRIGGER documentation](https://www.sqlite.org/lang_createtrigger.html) — official docs, confirms single-DML-event-per-trigger syntax and BEFORE-trigger/RAISE(ABORT) behavior
- [IRS Instructions for Form 990 (current)](https://www.irs.gov/instructions/i990) — confirms Part X Line 27 ("without donor restrictions") / Line 28 ("with donor restrictions") current terminology

### Secondary (MEDIUM confidence)
- [Simplifying implementation of FASB's not-for-profit financial reporting standard — Journal of Accountancy](https://www.journalofaccountancy.com/news/2018/dec/fasb-not-for-profit-financial-reporting-standard-201819721/) — corroborates ASU 2016-14 effective dates and two-category model
- [Understanding the Balance Sheet Section of Form 990 — Tax990 blog](https://blog.tax990.com/2026/05/11/understanding-the-balance-sheet-section-of-form-990/) — corroborates current Line 27/28 labeling, recent (2026) source
- [Understanding FASB ASU 2016-14: A Guide for Non-Profit Financial Reporting — Calvetti Ferguson](https://calvettiferguson.com/fasb-asu-2016-14-nonprofit-financial-reporting/) — CPA-firm secondary source corroborating terminology
- Project-level research files (already HIGH/MEDIUM confidence per their own sourcing): `.planning/research/STACK.md`, `ARCHITECTURE.md`, `PITFALLS.md`

### Tertiary (LOW confidence)
- None new in this pass — all Phase 1-specific claims were verified against official FASB/IRS/SQLite sources above.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Go/SQLite/migrate already locked at project level; this phase's additions (trigger syntax, schema shape) verified against official SQLite docs
- Architecture: HIGH — header+lines double-entry schema is the conventional pattern for this domain, cross-checked against project ARCHITECTURE.md and general double-entry-ledger literature
- FASB/fund terminology: HIGH — verified against primary FASB and IRS sources, cross-checked with two independent secondary sources including one from 2026
- Pitfalls: HIGH — carried forward from project-level PITFALLS.md (already MEDIUM-HIGH) plus one new verified pitfall (SQLite trigger syntax) found and confirmed against official docs during this pass

**Research date:** 2026-08-08
**Valid until:** ~180 days for FASB/IRS terminology (stable regulatory domain, unlikely to change again soon); ~30 days for library/tooling specifics (Go/SQLite driver versions move faster)
