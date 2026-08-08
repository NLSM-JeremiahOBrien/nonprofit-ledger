# Nonprofit Ledger (working name)

## What This Is

An open-source, self-hosted progressive web app that acts as a full fund-accounting system for small-to-mid nonprofits — a replacement for QuickBooks Desktop, which is being discontinued. It runs as a local webserver (single-tenant, one instance per organization, on the org's own machine/NAS/server), keeps all financial data local and never sends it to a third-party cloud, and provides gated authentication for internal staff/admins plus secure remote login for external accountants and bookkeepers. It ships with an importer for existing QuickBooks data and a modular import-adapter system so other cloud accounting platforms (Xero, Wave, etc.) can be supported later without core rework.

The SS Jeremiah O'Brien (historic Liberty ship museum, San Francisco) is the pilot deployment — the app will be built against their real chart of accounts and books, run in parallel with their existing QuickBooks Desktop instance during a validation period, and only fully replace it once trusted.

## Core Value

Nonprofit financial data — funds, grants, restricted/unrestricted balances, transaction history — stays entirely under the organization's own control, on their own hardware, forever, with no dependency on a vendor that can raise prices, get acquired, or discontinue the product out from under them.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Self-hosted PWA running on a local webserver, usable offline and installable like a native app
- [ ] Full nonprofit fund accounting: restricted/unrestricted/temporarily-restricted funds, fund-level balances, functional expense allocation (program/admin/fundraising)
- [ ] Core GL: double-entry bookkeeping, chart of accounts, AR/AP, bank reconciliation
- [ ] Grant tracking (award amounts, spend-down, reporting periods, restrictions tied to grant)
- [ ] Financial statements sufficient to prepare a Form 990 (Statement of Financial Position, Statement of Activities, functional expense statement)
- [ ] Auth system with roles: org staff/bookkeeper, admin, and external accountant (read/write scoped appropriately)
- [ ] Secure remote access built into the app (bundled tunnel, e.g. Tailscale/Cloudflare Tunnel style) so non-technical orgs don't need to configure networking themselves
- [ ] QuickBooks Desktop import (IIF and/or QBXML export formats)
- [ ] QuickBooks Online export import (QBO/CSV)
- [ ] Modular import-adapter architecture so future adapters (Xero, Wave, etc.) can be added without touching core ledger logic
- [ ] Re-importable / incremental import support so the app can run in parallel with a live QuickBooks instance during a transition period (not just one-shot migration)
- [ ] Reconciliation/comparison tooling to verify parity between this system's books and QuickBooks during the parallel-run period
- [ ] Local data storage only — no telemetry, no cloud sync of financial data by default
- [x] Open-source license: AGPL-3.0 — chosen specifically to close the SaaS loophole (prevents someone hosting a modified version as a service without contributing changes back), consistent with the anti-vendor-lock-in Core Value. Does not burden self-hosting orgs like O'Brien, since internal use isn't "distribution" under AGPL.
- [ ] Full CiviCRM integration: import CiviCRM contribution/revenue batches as GL deposits, built on the same import-adapter interface as QuickBooks (see CIVI-01..03 in REQUIREMENTS.md, mapped to Phase 4). Donor/contact management stays in CiviCRM — this is revenue-transaction import only, not a CRM rebuild.

### Out of Scope

- Multi-tenant hosted SaaS — contradicts the local-first, single-org-per-instance design (may revisit as a future hosted option, not v1)
- Payroll processing — high compliance surface area (tax filings, state-by-state rules); defer indefinitely, recommend integration with existing payroll providers instead
- Point-of-sale / donor CRM features — CiviCRM and similar tools already cover this for O'Brien; this project is books only

## Context

- Driving trigger: Intuit is discontinuing QuickBooks Desktop, and O'Brien (and likely other small nonprofits) need a non-cloud replacement before that cutoff.
- Darren is IT Advisor for the SS Jeremiah O'Brien (historic Liberty ship museum, Pier 35, San Francisco) — see workspace MEMORY.md for full org context. Ken Wright is the current point of contact on O'Brien matters.
- O'Brien already runs WordPress + CiviCRM for membership/donor management (separate system, separate server) — this project is strictly the accounting/books layer, not CRM.
- Pilot approach: build against O'Brien's real QuickBooks Desktop data, run the new system in parallel with live QuickBooks for a validation period before cutover — so the import pipeline needs to support repeated/incremental re-import and reconciliation reporting, not just a single one-time migration.
- No existing codebase — fully greenfield.

## Constraints

- **Deployment**: Single-tenant, self-hosted — each nonprofit runs its own instance on its own hardware. No central multi-org hosting in v1.
- **Data locality**: All financial data stays on the org's own server/device. No default cloud sync or third-party telemetry.
- **Remote access**: Must include a built-in secure remote-access mechanism (not a manual networking task) so a non-technical org can let an external accountant log in.
- **License**: Must be open-source (specific license TBD during research).
- **Migration compatibility**: Must be able to import from QuickBooks Desktop and QuickBooks Online export formats on day one, with an adapter architecture for more sources later.
- **Timeline pressure**: QuickBooks Desktop discontinuation creates real urgency for at least the pilot org.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single-tenant, self-hosted architecture | Matches "data stays local" requirement; avoids operating a multi-tenant service | — Pending |
| Bundle a secure remote-access mechanism into the app | External accountants need remote login; can't assume org IT can set up VPN/port-forwarding themselves | — Pending |
| Full fund accounting (not basic bookkeeping) for v1 | This is the real bar for replacing QuickBooks Desktop for a 990-filing nonprofit | — Pending |
| SS Jeremiah O'Brien as pilot deployment, built against real data | Gives a concrete production target instead of designing in the abstract; de-risks generalization later | — Pending |
| Support parallel-run against live QuickBooks (incremental re-import + reconciliation) | Org wants to validate the new system before fully cutting over, not do a risky one-shot migration | — Pending |
| AGPL-3.0 license | Closes the SaaS loophole (no re-hosting as a closed service) without burdening self-hosters; matches the no-vendor-lock-in Core Value | — Pending |
| Full CiviCRM integration via the import-adapter framework | O'Brien already runs CiviCRM for donor/membership management; revenue data should flow in without manual re-entry, but CRM itself stays out of scope to avoid duplicating a system of record | — Pending |

---
*Last updated: 2026-08-08 after initialization*
