# Requirements: Nonprofit Ledger

**Defined:** 2026-08-08
**Core Value:** Nonprofit financial data stays entirely under the organization's own control, on their own hardware, forever, with no dependency on a vendor that can raise prices, get acquired, or discontinue the product.

## v1 Requirements

### Ledger (GL)

- [x] **LEDG-01**: User can define a customizable chart of accounts (asset/liability/equity/revenue/expense types, parent/sub-account hierarchy, active/inactive flags)
- [ ] **LEDG-02**: User can post double-entry journal transactions that must balance (debits = credits) before posting
- [ ] **LEDG-03**: Posted transactions are immutable — corrections are made via reversing/adjusting entries, never destructive edit or delete
- [ ] **LEDG-04**: User can close/lock an accounting period so posted transactions before that date can no longer be altered
- [ ] **LEDG-05**: Every transaction line carries a fund dimension (not just an account), so fund-level balances can always be derived from the ledger

### Fund Accounting

- [x] **FUND-01**: User can create funds classified per current FASB ASC 958 net-asset categories (with donor restrictions / without donor restrictions)
- [ ] **FUND-02**: User can view real-time balance for any individual fund, and fund balances reconcile to the GL total
- [ ] **FUND-03**: User can allocate expense transactions across functional categories (program / management & general / fundraising)
- [ ] **FUND-04**: User can create a grant record (funder, award amount, reporting period) linked to a restricted fund and track spend-down against the award

### Financial Statements

- [ ] **STMT-01**: User can generate a Statement of Financial Position (balance sheet) for a given date
- [ ] **STMT-02**: User can generate a Statement of Activities showing changes in net assets with/without donor restrictions for a period
- [ ] **STMT-03**: User can generate a Statement of Functional Expenses (program/management/fundraising breakdown) for a period
- [ ] **STMT-04**: User can generate a Statement of Cash Flows for a period
- [ ] **STMT-05**: User can export any generated statement as PDF and the underlying data as CSV

### AR / AP / Bank

- [ ] **BANK-01**: User can record and track accounts receivable (invoices, aging)
- [ ] **BANK-02**: User can record and track accounts payable (bills, aging, payment status)
- [ ] **BANK-03**: User can import a bank statement file (CSV/OFX/QFX) and match transactions against the ledger
- [ ] **BANK-04**: User can mark a period as reconciled once bank statement and ledger balances agree, with discrepancies flagged

### Auth & Access

- [ ] **AUTH-01**: User can log in with a username/password to a locally-hosted account
- [ ] **AUTH-02**: Admin can create and manage user accounts with one of three roles: staff/bookkeeper, admin, external accountant
- [ ] **AUTH-03**: External accountant role has a default scope narrower than admin (can view/edit ledger and reports, cannot manage users or org settings)
- [ ] **AUTH-04**: Session persists securely across browser refresh/restart without re-entering credentials constantly
- [ ] **AUTH-05**: Every create/edit/delete action is attributed to the logged-in user in an immutable audit log (who changed what, when)

### Remote Access

- [ ] **REMT-01**: Admin can enable secure remote access for the external accountant role without manually configuring VPN, port-forwarding, or firewall rules
- [ ] **REMT-02**: Remote connections are authenticated and encrypted end-to-end; the remote-access layer never bypasses the app's own auth/RBAC
- [ ] **REMT-03**: Admin can revoke a remote user's access immediately and see currently-connected remote sessions

### Import — QuickBooks

- [ ] **IMPT-01**: User can import a QuickBooks Desktop export (IIF and/or QBXML) and preview mapped transactions before committing
- [ ] **IMPT-02**: User can import a QuickBooks Online export (QBO/CSV) and preview mapped transactions before committing
- [ ] **IMPT-03**: Re-running an import with overlapping data does not create duplicate transactions (idempotent import keyed on stable external identifiers)
- [ ] **IMPT-04**: Import adapters are built against a documented internal interface so a new source (e.g. Xero, Wave) can be added without modifying core ledger code

### Integration — CiviCRM

- [ ] **CIVI-01**: User can import CiviCRM contribution/revenue batch exports as GL deposit transactions, mapped to the correct fund and account
- [ ] **CIVI-02**: CiviCRM contact and contribution IDs are preserved as external references on imported transactions, so re-importing a batch does not create duplicate revenue entries
- [ ] **CIVI-03**: CiviCRM adapter is built against the same import-adapter interface as the QuickBooks adapters (IMPT-04), not a one-off integration

### Parallel-Run Reconciliation

- [ ] **RECN-01**: User can run a comparison report between this system's ledger and the most recent QuickBooks import, highlighting transactions present in one system but not the other
- [ ] **RECN-02**: User can mark discrepancies from a comparison report as resolved, with the resolution recorded in the audit log

### Platform

- [ ] **PLAT-01**: The application runs as a local webserver on the organization's own hardware, with no required outbound dependency on a third-party cloud service for core accounting functions
- [ ] **PLAT-02**: The application is installable as a PWA and remains usable (viewing data, drafting entries) during brief network interruptions to the local server
- [ ] **PLAT-03**: The application performs automated local backups on a schedule, with visible backup health status and a documented, tested restore procedure
- [ ] **PLAT-04**: The application does not transmit financial data or usage telemetry to any third party by default

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Documents & Quality of Life

- **DOCS-01**: User can attach receipts/documents to transactions
- **DOCS-02**: User can set up recurring/memorized transaction templates

### Reporting

- **RPT-01**: User can generate budget-vs-actual reports at the org and per-grant level
- **RPT-02**: User can maintain a simple fixed-asset register with straight-line depreciation

### Import

- **IMPT-05**: Rule-based (non-ML) auto-categorization suggestions for imported/bank transactions
- **IMPT-06**: Additional import adapters (Xero, Wave, others) beyond QuickBooks

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-tenant hosted SaaS | Contradicts the local-first, single-org-per-instance data sovereignty value; a future hosted offering would be a separate product decision, not v1 |
| Payroll processing | Enormous compliance surface (federal/state withholding, quarterly filings, W-2/1099); recommend integrating with an existing payroll provider via journal-entry import instead |
| Donor CRM / fundraising / POS features | O'Brien already runs CiviCRM for this; duplicating donor data creates a two-systems-of-record integrity trap |
| Real-time third-party bank feed aggregation (Plaid-style) | Requires trusting a third-party financial aggregator with bank credentials, contradicting the no-cloud-dependency constraint; manual file import covers the same need |
| Comprehensive AI/ML auto-categorization or natural-language reporting | High implementation cost, unclear reliability where errors have audit consequences, undermines the "verifiable local system" trust story; simple rule-based matching covers most practical benefit |
| Full ERP features (inventory, multi-entity consolidation, complex fixed-asset depreciation engines) | Enterprise-tier features with no benefit to the pilot org or typical small-nonprofit target user |
| Custom/ad-hoc report builder | Large open-ended engineering investment; a fixed set of standard reports covers documented 990 and fund-accounting needs |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| LEDG-01 | Phase 1 | Complete |
| LEDG-02 | Phase 1 | Pending |
| LEDG-03 | Phase 1 | Pending |
| LEDG-04 | Phase 1 | Pending |
| LEDG-05 | Phase 1 | Pending |
| FUND-01 | Phase 1 | Complete |
| FUND-02 | Phase 1 | Pending |
| FUND-03 | Phase 1 | Pending |
| PLAT-01 | Phase 1 | Pending |
| AUTH-01 | Phase 2 | Pending |
| AUTH-02 | Phase 2 | Pending |
| AUTH-03 | Phase 2 | Pending |
| AUTH-04 | Phase 2 | Pending |
| AUTH-05 | Phase 2 | Pending |
| PLAT-03 | Phase 2 | Pending |
| PLAT-04 | Phase 2 | Pending |
| REMT-01 | Phase 3 | Pending |
| REMT-02 | Phase 3 | Pending |
| REMT-03 | Phase 3 | Pending |
| IMPT-01 | Phase 4 | Pending |
| IMPT-02 | Phase 4 | Pending |
| IMPT-03 | Phase 4 | Pending |
| IMPT-04 | Phase 4 | Pending |
| RECN-01 | Phase 5 | Pending |
| RECN-02 | Phase 5 | Pending |
| BANK-01 | Phase 6 | Pending |
| BANK-02 | Phase 6 | Pending |
| BANK-03 | Phase 6 | Pending |
| BANK-04 | Phase 6 | Pending |
| FUND-04 | Phase 6 | Pending |
| STMT-01 | Phase 6 | Pending |
| STMT-02 | Phase 6 | Pending |
| STMT-03 | Phase 6 | Pending |
| STMT-04 | Phase 6 | Pending |
| STMT-05 | Phase 6 | Pending |
| PLAT-02 | Phase 7 | Pending |
| CIVI-01 | Phase 4 | Pending |
| CIVI-02 | Phase 4 | Pending |
| CIVI-03 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 39 total
- Mapped to phases: 39
- Unmapped: 0 ✓

**Note:** CIVI-01..03 (CiviCRM integration) added 2026-08-08 after initial roadmap creation, per explicit request that this system fully integrate with CiviCRM (which O'Brien already runs for donor/membership management). These reuse the same import-adapter architecture as the QuickBooks adapters (IMPT-04) and are mapped into Phase 4 (Import-Adapter Framework + QuickBooks Adapters) as an additional adapter on the same interface, rather than a new phase. Scope is revenue-transaction import only (contribution batches → GL deposits) — donor/contact management itself remains CiviCRM's job, per the Out of Scope decision against duplicating CRM features.

---
*Requirements defined: 2026-08-08*
*Last updated: 2026-08-08 after adding CiviCRM integration requirements*
