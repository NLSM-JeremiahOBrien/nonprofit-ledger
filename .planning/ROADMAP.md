# Roadmap: Nonprofit Ledger

## Overview

Nonprofit Ledger is built from the ground up: an immutable, fund-aware general ledger first (the schema hardest to retrofit later), then the trust boundary (auth/RBAC and backups) that must exist before real O'Brien financial data enters the system, then the bundled remote-access tunnel built on top of finalized RBAC. From there, the QuickBooks import-adapter framework and the parallel-run reconciliation tooling give O'Brien a safe path to validate the new system against their live QuickBooks Desktop books. AR/AP, bank reconciliation, grant tracking, and 990-ready financial statements reuse the ledger and reconciliation engines built earlier. The PWA installable client and offline shell are layered on last as additive UX against a stable, already-correct API.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Core Ledger & Fund-Accounting Data Model** - Immutable double-entry GL with fund and functional-expense dimensions as first-class citizens
- [ ] **Phase 2: Auth, RBAC & Backup/DR Foundations** - Scoped user roles, audit logging, automated backups, and no-telemetry-by-default trust guarantees
- [ ] **Phase 3: Bundled Secure Remote Access** - Zero-config remote login for external accountants, layered on finalized RBAC
- [ ] **Phase 4: Import-Adapter Framework + QuickBooks Adapters** - Idempotent, previewable import of QuickBooks Desktop and Online data via a documented adapter interface
- [ ] **Phase 5: Reconciliation & Parallel-Run Reporting** - Comparison reporting to validate this system's books against live QuickBooks
- [ ] **Phase 6: AR/AP, Bank Reconciliation, Grants & Financial Statements** - Receivables/payables, bank matching, grant spend-down tracking, and 990-ready statements
- [ ] **Phase 7: PWA Client & Offline Shell** - Installable app shell usable through brief network interruptions

## Phase Details

### Phase 1: Core Ledger & Fund-Accounting Data Model
**Goal**: Users can maintain a correct, immutable, fund-aware general ledger that forms the foundation of the entire system, running as a local webserver on the org's own hardware.
**Depends on**: Nothing (first phase)
**Requirements**: LEDG-01, LEDG-02, LEDG-03, LEDG-04, LEDG-05, FUND-01, FUND-02, FUND-03, PLAT-01
**Success Criteria** (what must be TRUE):
  1. User can define a customizable chart of accounts with account types and parent/sub-account hierarchy
  2. User can post a double-entry journal transaction tagged with a fund, and it is rejected unless debits equal credits
  3. A posted transaction cannot be edited or deleted — corrections happen only via reversing/adjusting entries
  4. User can close/lock an accounting period so transactions before that date can no longer be altered
  5. User can view real-time balance for any fund, and fund balances always reconcile to the GL total, with the app running entirely as a local webserver with no required third-party cloud dependency
**Plans**: TBD

### Phase 2: Auth, RBAC & Backup/DR Foundations
**Goal**: Users can securely log in with scoped roles, every action is attributable, and the organization's sole financial record is protected by automated, verifiable backups — all before real financial data enters the system.
**Depends on**: Phase 1
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, PLAT-03, PLAT-04
**Success Criteria** (what must be TRUE):
  1. User can log in with a username/password, and the session persists securely across browser refresh/restart
  2. Admin can create and manage user accounts with one of three roles (staff/bookkeeper, admin, external accountant), each scoped appropriately
  3. Every create/edit/delete action is attributed to the logged-in user in an immutable audit log
  4. The application performs automated local backups on a schedule, with visible backup health status and a documented, tested restore procedure
  5. No financial data or usage telemetry is transmitted to any third party by default
**Plans**: TBD

### Phase 3: Bundled Secure Remote Access
**Goal**: External accountants can securely connect to the app remotely, with the org never having to configure networking themselves, and remote access never bypassing the app's own RBAC.
**Depends on**: Phase 2
**Requirements**: REMT-01, REMT-02, REMT-03
**Success Criteria** (what must be TRUE):
  1. Admin can enable secure remote access for the external accountant role without manually configuring VPN, port-forwarding, or firewall rules
  2. Remote connections are authenticated and encrypted end-to-end, and remote users are still bound by the app's own auth/RBAC
  3. Admin can see currently-connected remote sessions and revoke a remote user's access immediately
**Plans**: TBD

### Phase 4: Import-Adapter Framework + QuickBooks Adapters
**Goal**: Users can bring existing QuickBooks Desktop and Online data into the system reliably, repeatedly, and safely, with a clean path for future import sources.
**Depends on**: Phase 1, Phase 2
**Requirements**: IMPT-01, IMPT-02, IMPT-03, IMPT-04
**Success Criteria** (what must be TRUE):
  1. User can import a QuickBooks Desktop export (IIF and/or QBXML) and preview mapped transactions before committing
  2. User can import a QuickBooks Online export (QBO/CSV) and preview mapped transactions before committing
  3. Re-running an import with overlapping data does not create duplicate transactions
  4. A new import source can be added against a documented adapter interface without modifying core ledger code
**Plans**: TBD

### Phase 5: Reconciliation & Parallel-Run Reporting
**Goal**: Users can verify that this system's books match live QuickBooks during the parallel-run validation period, so the org can trust the new system before fully cutting over.
**Depends on**: Phase 4
**Requirements**: RECN-01, RECN-02
**Success Criteria** (what must be TRUE):
  1. User can run a comparison report between this system's ledger and the most recent QuickBooks import, highlighting transactions present in one system but not the other
  2. User can mark a discrepancy from a comparison report as resolved, with the resolution recorded in the audit log
**Plans**: TBD

### Phase 6: AR/AP, Bank Reconciliation, Grants & Financial Statements
**Goal**: Users can manage receivables and payables, reconcile bank activity against the ledger, track grants against restricted funds, and produce financial statements sufficient to prepare a Form 990.
**Depends on**: Phase 1, Phase 5
**Requirements**: BANK-01, BANK-02, BANK-03, BANK-04, FUND-04, STMT-01, STMT-02, STMT-03, STMT-04, STMT-05
**Success Criteria** (what must be TRUE):
  1. User can record and track accounts receivable (invoices, aging) and accounts payable (bills, aging, payment status)
  2. User can import a bank statement file (CSV/OFX/QFX), match transactions against the ledger, and mark a period reconciled with discrepancies flagged
  3. User can create a grant record (funder, award amount, reporting period) linked to a restricted fund and track spend-down against the award
  4. User can generate a Statement of Financial Position, Statement of Activities, Statement of Functional Expenses, and Statement of Cash Flows for a given date/period
  5. User can export any generated statement as PDF and the underlying data as CSV
**Plans**: TBD

### Phase 7: PWA Client & Offline Shell
**Goal**: Users can install the app like a native app and keep working during brief local network interruptions.
**Depends on**: Phase 1, Phase 2, Phase 3, Phase 4, Phase 5, Phase 6
**Requirements**: PLAT-02
**Success Criteria** (what must be TRUE):
  1. User can install the application as a PWA on their device
  2. User can continue viewing data and drafting entries during brief network interruptions to the local server
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Core Ledger & Fund-Accounting Data Model | 0/TBD | Not started | - |
| 2. Auth, RBAC & Backup/DR Foundations | 0/TBD | Not started | - |
| 3. Bundled Secure Remote Access | 0/TBD | Not started | - |
| 4. Import-Adapter Framework + QuickBooks Adapters | 0/TBD | Not started | - |
| 5. Reconciliation & Parallel-Run Reporting | 0/TBD | Not started | - |
| 6. AR/AP, Bank Reconciliation, Grants & Financial Statements | 0/TBD | Not started | - |
| 7. PWA Client & Offline Shell | 0/TBD | Not started | - |
