# Project Research Summary

**Project:** Nonprofit Ledger (working name)
**Domain:** Self-hosted, single-tenant, local-first nonprofit fund-accounting PWA (QuickBooks Desktop replacement)
**Researched:** 2026-08-08
**Confidence:** MEDIUM-HIGH

## Executive Summary

This is a single-tenant, self-hosted fund-accounting system — not a browser-CRDT "local-first" app and not a multi-tenant SaaS. One instance runs per nonprofit on their own hardware; the server's database is the single source of truth; the PWA client is a thin, well-cached frontend with only shallow offline behavior. Experts building this class of system converge on a small, boring, ACID-correct stack: a single statically-linked server binary (Go, largely because of native cross-compilation and clean embedding of Tailscale's `tsnet` library), an embedded SQLite database in WAL mode (not Postgres, not TigerBeetle), and a React/Vite PWA frontend built around table/grid-heavy accounting UI. The single most load-bearing stack decision is embedding Tailscale via `tsnet`: it gives the app private, invite-only remote access with no port-forwarding and no public attack surface — solving the project's hardest UX problem.

The recommended approach treats the core ledger as an append-only, insert-only journal (corrections via reversing entries, never UPDATE/DELETE), with fund and functional-expense-category modeled as first-class dimensions from day one. Import from QuickBooks must go through a staging → reconciliation → human-approved-promotion pipeline, never write directly into the journal, so parallel-run against live QuickBooks with repeated, idempotent re-import actually works. Feature scope is unusually well-aligned with PROJECT.md already: double-entry GL, fund accounting, functional expense allocation, grant tracking, AR/AP, bank reconciliation, 990-ready statements, RBAC with a scoped external-accountant role, and the import/reconciliation pipeline are all P1; payroll, donor CRM, bank-feed aggregators, and multi-tenant SaaS are correctly out of scope.

Key risks cluster around trust and irreversibility, not raw engineering difficulty: mutable/stored balances that silently corrupt audited history; a one-shot, non-idempotent import that breaks the required parallel-run workflow; treating messy real QuickBooks exports as clean; skipping dedicated reconciliation tooling so two systems quietly drift apart; bolting remote access onto RBAC as an afterthought; and leaving backup/DR as the non-technical operator's problem for the org's sole financial record. All are addressed by getting the core ledger schema, import-adapter architecture, and backup mechanism right in the earliest phases — retrofitting any of them after real O'Brien data is in the system is expensive-to-impossible.

## Key Findings

### Recommended Stack

Go (single static binary) + embedded SQLite (via `modernc.org/sqlite`, WAL mode, no CGo) + React 19/Vite PWA frontend, with Tailscale's `tsnet` embedded directly in the Go process for remote access and `litestream` for continuous SQLite backup/point-in-time recovery. This is explicitly not a CRDT/sync-engine architecture — there is one authoritative server per org, so "offline-capable" means service-worker app-shell caching plus a small write queue, not multi-device conflict resolution. Postgres and TigerBeetle were both considered and rejected as operational overkill.

**Core technologies:**
- Go 1.26.x — backend server, single cross-compiled binary, no runtime to install on a NAS
- SQLite (`modernc.org/sqlite`) in WAL mode — embedded system of record, ACID, no separate DB server process
- React 19 + Vite — grid/table-heavy accounting UI (TanStack Table/Query), builds into a static bundle Go `embed`s directly
- `tailscale.com/tsnet` — embedded, userspace secure remote access; the single most load-bearing pick in the stack
- `litestream` — continuous SQLite backup, non-negotiable given the financial-data stakes
- `golang-migrate/migrate` — schema migrations, never hand-rolled for financial data

### Expected Features

Feature scope maps almost 1:1 onto PROJECT.md's "Active" requirements. The two genuinely novel differentiators (not offered by any competitor researched: Aplos, Sage Intacct, Blackbaud, QBO, GnuCash) are the bundled zero-config remote access and the parallel-run reconciliation/comparison tooling against live QuickBooks.

**Must have (table stakes):**
- Double-entry GL with customizable chart of accounts
- Fund accounting (restricted/temporarily restricted/unrestricted, FASB ASC 958-correct) with fund-tagged transactions
- Functional expense allocation (program/admin/fundraising) — required for Form 990 Part IX
- AR/AP, bank reconciliation (manual file import), core financial statements (Statement of Financial Position, Statement of Activities, Statement of Functional Expenses, likely Statement of Cash Flows)
- Multi-user roles (staff, admin, external accountant) with a scoped audit trail

**Should have (competitive differentiators):**
- Bundled secure remote access requiring no networking setup by the org
- Parallel-run reconciliation/comparison reporting against live QuickBooks (genuinely novel)
- Modular import-adapter architecture (QB Desktop, QBO, future Xero/Wave)
- Grant-to-fund linkage with spend-down/drawdown tracking, purpose-built for small orgs
- Zero recurring vendor dependency / no per-seat SaaS pricing

**Defer (v2+):**
- Real-time third-party bank-feed aggregation (Plaid-style) — conflicts with local-first constraint
- AI/ML auto-categorization, custom report builder, multi-entity support, hosted/managed SaaS tier
- Payroll, donor CRM/fundraising, full ERP features

### Architecture Approach

Layered single-process architecture: a PWA client talks over HTTPS (LAN or via the bundled tunnel) to a local webserver that owns auth/RBAC, a REST/RPC API, a framework-agnostic ledger/domain engine, a reporting/statement generator, and an import-adapter subsystem — all backed by an append-only journal plus derived/rebuildable projection tables in SQLite. The two rules that matter most: import adapters never write directly to the journal (staging → reconciliation → human-approved promotion), and the remote-access tunnel is a network-transport concern only, never an authorization boundary.

**Major components:**
1. Ledger/domain engine — enforces double-entry invariants, fund rules, and reversal-only corrections
2. Import-adapter subsystem + staging/reconciliation store — normalizes IIF/QBXML/QBO/CSV, idempotent re-import via upsert on external transaction ID
3. Auth/session/RBAC layer — staff/admin/external-accountant roles, sole authority on permissions regardless of network path
4. Reporting/statement generation — read-only queries against projections, produces 990-ready statements
5. Remote-access/tunnel component — bundled, outbound-only, swappable wrapper (Tailscale today) around the same API/auth layer

### Critical Pitfalls

1. **Mutable ledger rows / stored balances instead of derived** — enforce append-only journal at the DB layer; balances always computed from the journal, cached only as a disposable, rebuildable projection.
2. **Non-idempotent, one-shot QuickBooks import** — design for upsert semantics from day one (stable external-transaction-key per record); re-import must never touch transactions that originated inside the new system.
3. **Treating real QuickBooks exports as clean** — structural validation with row-level, fail-loud errors and a dry-run/preview mode; test against O'Brien's actual messy historical export.
4. **No reconciliation tooling during parallel-run** — first-class reconciliation report (matched / QB-only / app-only, balance comparison) before parallel-run begins with O'Brien.
5. **Remote access designed as an afterthought bolt-on to RBAC** — design auth/RBAC and remote access together; scope the external-accountant role narrowly, require MFA, log every session.
6. **Backup/DR designed for a technical operator, not a nonprofit volunteer** — automated scheduled backups, visible "last backup" status, tested one-click restore, as a named early phase.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Core Ledger & Fund-Accounting Data Model
**Rationale:** Foundational and expensive to retrofit — fund and functional-expense dimensions must be first-class on every transaction from day one (Pitfall 7); balances must be derived, never stored (Pitfall 1).
**Delivers:** Double-entry posting engine, chart of accounts, fund dimension, append-only journal with DB-level immutability enforcement, derived/rebuildable projection tables, reversal-only correction logic.
**Addresses:** Double-entry GL, Chart of Accounts, Fund accounting, Functional expense allocation
**Avoids:** Pitfall 1 (mutable ledger), Pitfall 7 (990-readiness bolted on late)

### Phase 2: Auth, RBAC & Backup/DR Foundations
**Rationale:** Trust-boundary and disaster-prevention concerns that must exist before real O'Brien data enters the system and before remote access is built (RBAC must be finalized first, per Pitfall 5).
**Delivers:** Session-based auth, staff/admin/external-accountant roles with scoped permissions, session/access audit logging, automated scheduled SQLite backups (litestream) with visible backup-health status and a tested restore flow.
**Uses:** `golang.org/x/crypto/argon2`, server-side cookie sessions, `litestream`
**Avoids:** Pitfall 5 (remote access as afterthought), Pitfall 6 (backup for non-technical operators)

### Phase 3: Bundled Secure Remote Access
**Rationale:** Depends on a finished RBAC model (Phase 2); the single most differentiated and highest-complexity stack integration (`tsnet`), deserving isolation as its own phase.
**Delivers:** Embedded Tailscale (`tsnet`) remote access, off by default, one-step enable, LAN-only baseline that always works without it.
**Implements:** Remote-access/tunnel component, scoped external-accountant access flow
**Avoids:** Pitfall 5 (tunnel treated as authorization boundary)

### Phase 4: Import-Adapter Framework + QuickBooks Desktop/Online Adapters
**Rationale:** The second-largest architectural bet and the most likely source of real-world edge-case effort. The staging→reconciliation→promotion pattern must be designed into the framework itself.
**Delivers:** Canonical `ImportAdapter` interface (parse/map/stage), staging schema keyed by external transaction ID, IIF and QBXML parsers, QBO/CSV adapter, idempotent upsert re-import, dry-run/preview mode, per-row error reporting.
**Addresses:** QuickBooks Desktop import, QBO import, modular import-adapter architecture, incremental/re-importable import
**Avoids:** Pitfall 2 (non-idempotent import), Pitfall 3 (fragile parsing of messy real exports)

### Phase 5: Reconciliation & Parallel-Run Reporting
**Rationale:** The entire point of the parallel-run strategy; reuses the same matching/diffing logic as bank reconciliation (Phase 6), so sequencing right after import gives the cleanest reuse.
**Delivers:** Reconciliation report (matched / QB-only / app-only buckets), balance-by-account/fund comparison, actionable discrepancy UI, reconciliation history over time.
**Addresses:** Parallel-run reconciliation/comparison reporting (the project's most novel differentiator)
**Avoids:** Pitfall 4 (no reconciliation tooling)

### Phase 6: AR/AP, Bank Reconciliation & Core Financial Statements
**Rationale:** Depends on the ledger core (Phase 1) and reuses the reconciliation matching engine (Phase 5). Financial statements are the ultimate validation of whether the Phase 1 schema is correct.
**Delivers:** AR/AP modules, manual bank statement import (CSV/OFX/QFX) with matching UI, Statement of Financial Position, Statement of Activities, Statement of Functional Expenses, Statement of Cash Flows — 990-ready against O'Brien's real chart of accounts.
**Addresses:** AR/AP, bank reconciliation, core financial statements, grant tracking

### Phase 7: PWA Client & Offline Shell
**Rationale:** Can be built incrementally against the API surface established in Phases 1-6; installability and offline-shell caching are additive UX, lower-risk relative to backend phases.
**Delivers:** Installable PWA shell, service-worker precaching, IndexedDB read cache, small offline write queue that flushes on reconnect, data-grid-heavy UI for ledgers/reports.
**Uses:** React 19, Vite, `vite-plugin-pwa`, `@tanstack/react-query`, `@tanstack/react-table`, `react-hook-form` + `zod`

### Phase Ordering Rationale

- Ledger/fund schema comes first because fund and functional-expense dimensions cannot be accurately retrofitted onto historical transactions later — every downstream feature depends on this schema being right.
- Auth/RBAC precedes remote access because remote access is only as safe as the RBAC model behind it — never ship the tunnel before roles are finalized.
- Backup/DR is pulled forward to Phase 2 rather than left as "polish," because O'Brien's real financial data becomes at-risk the moment the pilot import happens.
- Import adapters come before reconciliation/parallel-run reporting because reconciliation's diffing logic is built on top of the same staged/keyed transaction data the import framework establishes.
- Bank reconciliation and financial statements are grouped together after reconciliation tooling exists, so the same matching engine is reused rather than building two separate diffing systems.
- The PWA client is sequenced last among core phases because it is additive UX on a stable API, not a blocking dependency for financial correctness.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (Core Ledger & Fund-Accounting Data Model):** FASB ASC 958 net-asset terminology needs verification against current IRS Form 990 instructions before finalizing schema/UI labels.
- **Phase 4 (Import-Adapter Framework):** IIF is a fragile format with no maintained Go parser library and no stable transaction ID in many cases; QBXML/QBO real-world export quirks need a dedicated research/spike pass against O'Brien's actual export files.
- **Phase 3 (Bundled Secure Remote Access):** `tsnet` version-pinning strategy and the fallback UX for orgs that can't/won't create a Tailscale account should be nailed down with current Tailscale docs at implementation time.

Phases with standard patterns (skip research-phase):
- **Phase 2 (Auth, RBAC & Backup basics):** Server-side sessions, argon2, and litestream-based backup are well-documented, standard patterns.
- **Phase 7 (PWA Client & Offline Shell):** `vite-plugin-pwa` + React Query is a standard, well-documented pairing for exactly this use case.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM-HIGH | Core architecture (Go single binary + SQLite + embedded tsnet) is HIGH confidence and well-sourced from official docs; specific library versions are MEDIUM given 2026 mid-major-version churn |
| Features | MEDIUM | WebSearch-verified across multiple vendor/industry sources; no primary-source coverage for this product category, and FASB/IRS terminology specifics need direct verification |
| Architecture | MEDIUM | Patterns verified across multiple credible sources; no single canonical open-source project matches this exact shape, so the synthesis is informed inference |
| Pitfalls | MEDIUM-HIGH | Ledger/import/security patterns verified against multiple independent sources; nonprofit-specific fund-accounting norms verified against practitioner sources; QuickBooks format-specific quirks rest partly on general knowledge and need validation against real O'Brien files |

**Overall confidence:** MEDIUM-HIGH

### Gaps to Address

- **FASB ASC 958 terminology:** verify "with donor restrictions" / "without donor restrictions" against current IRS Form 990 instructions before finalizing the Phase 1 schema and UI labels.
- **IIF/QBXML real-world quirks:** obtain O'Brien's actual QuickBooks Desktop export early; treat Phase 4 as needing a dedicated spike/research pass against real, messy data.
- **Stack/architecture doc alignment:** STACK.md recommends Go + React/Vite; ARCHITECTURE.md's illustrative code examples are written in TypeScript/Node-style pseudocode. This is a documentation-consistency gap — treat STACK.md's Go recommendation as authoritative and adapt ARCHITECTURE.md's structural patterns to Go idioms during implementation planning.
- **TanStack Table v9 recency:** went stable Aug 4, 2026, very close to this research date — consider pinning to the better-documented v8 line initially if implementation starts soon.
- **License selection:** PROJECT.md notes open-source license is "TBD during research" — not resolved by any of the four research files; needs an explicit decision (tied to Pitfall 8's governance/security-disclosure recommendations) before or during Phase 1 project setup.
- **Statement of Cash Flows:** FEATURES.md flags this as "likely needed" but not explicitly listed in PROJECT.md's requirements — verify with O'Brien's CPA before finalizing the Phase 6 statement set.

## Sources

### Primary (HIGH confidence)
- [tsnet · Tailscale Docs](https://tailscale.com/docs/features/tsnet)
- [SQLite Write-Ahead Logging docs](https://www.sqlite.org/wal.html)
- [modernc.org/sqlite package docs](https://pkg.go.dev/modernc.org/sqlite)
- [go.dev — Go 1.26 release notes](https://go.dev/doc/go1.26)

### Secondary (MEDIUM confidence)
- Aplos, Sage Intacct, Blackbaud, QuickBooks Online nonprofit product/comparison pages — competitor feature landscape
- LedgerSMB, GnuCash, Ledger (plain-text accounting) — self-hosted double-entry reference architectures
- Modern Treasury (Enforcing Immutability in your Double-Entry Ledger; How to Scale a Ledger Part V) — append-only ledger design patterns
- Square Developer Blog — Books, an immutable double-entry accounting database service
- Tailscale vs Cloudflare Tunnel comparisons — remote-access trust-boundary analysis
- Nonprofit fund-accounting practitioner sources (NetSuite, Nonprofit Bookkeeping, Mission Edge, KLR, Wegner CPAs) — FASB ASC 958 and Form 990 functional-expense requirements

### Tertiary (LOW confidence)
- General training-data knowledge of QuickBooks IIF/QBXML/QBO export format structure and quirks — not independently verified; flagged for dedicated research before implementing importers
- IIF parser: no maintained Go library found; plan for a custom, well-tested parser as part of the import-adapter module

---
*Research completed: 2026-08-08*
*Ready for roadmap: yes*
