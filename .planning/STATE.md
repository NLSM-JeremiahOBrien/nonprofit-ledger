# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-08)

**Core value:** Nonprofit financial data stays entirely under the organization's own control, on their own hardware, forever, with no dependency on a vendor that can raise prices, get acquired, or discontinue the product.
**Current focus:** Phase 1 — Core Ledger & Fund-Accounting Data Model

## Current Position

Phase: 1 of 7 (Core Ledger & Fund-Accounting Data Model)
Plan: None yet
Status: Ready to plan
Last activity: 2026-08-08 — Roadmap created, 36/36 v1 requirements mapped across 7 phases

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Phase order chosen goal-backward: ledger schema first (hardest to retrofit), then auth/RBAC + backup before real O'Brien data enters the system, then remote access on top of finalized RBAC, then import/reconciliation, then AR/AP + statements, then PWA shell last.
- Grant tracking (FUND-04) grouped with Phase 6 (AR/AP/statements) rather than Phase 1, since it's a reporting/tracking feature best delivered alongside financial statements.
- PLAT-01 (local webserver, no cloud dependency) assigned to Phase 1 as foundational; PLAT-03/PLAT-04 (backups, no telemetry) assigned to Phase 2 as trust-boundary concerns; PLAT-02 (PWA/offline) assigned to Phase 7.

### Pending Todos

None yet.

### Blockers/Concerns

- Research flags open items to resolve during Phase 1 planning: verify FASB ASC 958 net-asset terminology against current IRS Form 990 instructions before finalizing schema/UI labels.
- Research flags open items to resolve during Phase 4 planning: IIF/QBXML have no maintained Go parser and real-world export quirks need validation against O'Brien's actual export files.
- Open-source license selection is still TBD — needs an explicit decision before or during Phase 1 project setup.
- Statement of Cash Flows (STMT-04) needs verification with O'Brien's CPA that it's actually required — currently included in Phase 6 scope per research recommendation.

## Session Continuity

Last session: 2026-08-08
Stopped at: Roadmap created and written to disk; awaiting user approval before planning Phase 1
Resume file: None
