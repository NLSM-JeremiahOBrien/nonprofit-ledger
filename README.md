# Nonprofit Ledger

An open-source, self-hosted fund-accounting system for nonprofits — built as a QuickBooks Desktop replacement.

Nonprofit Ledger runs as a single local webserver on your own hardware. Your financial data never leaves your organization: no cloud sync, no third-party telemetry, no vendor who can raise prices, get acquired, or discontinue the product out from under you.

## Why this exists

QuickBooks Desktop is being discontinued, leaving nonprofits with cloud-only options that don't fit the data-sovereignty and cost requirements many small orgs need. Nonprofit Ledger is a full GAAP-compliant fund-accounting system — not a stripped-down bookkeeping tool — designed to actually replace it.

**Pilot deployment:** [SS Jeremiah O'Brien](https://ssjeremiahobrien.org), a historic Liberty ship museum in San Francisco, National Liberty Ship Memorial. The system is being built against O'Brien's real chart of accounts and run in parallel with their live QuickBooks Desktop instance for validation before cutover.

## Core value

Nonprofit financial data — funds, grants, restricted/unrestricted balances, transaction history — stays entirely under the organization's own control, on their own hardware, forever.

## Planned capabilities (v1)

- **Full fund accounting** — restricted/unrestricted funds (current FASB ASC 958 net-asset model), fund-level balances, functional expense allocation (program/admin/fundraising)
- **Core double-entry GL** — chart of accounts, AR/AP, bank reconciliation, immutable append-only journal with reversing-entry corrections
- **Grant tracking** — award amounts, spend-down, reporting periods, tied to restricted funds
- **990-ready financial statements** — Statement of Financial Position, Statement of Activities, Statement of Functional Expenses, Statement of Cash Flows
- **Role-based auth** — staff/bookkeeper, admin, and external accountant roles
- **Bundled secure remote access** — so an external accountant can log in without the org configuring VPNs or port-forwarding
- **QuickBooks import** — Desktop (IIF/QBXML) and Online (QBO/CSV) exports, with incremental re-import support for running in parallel with a live QuickBooks instance
- **CiviCRM integration** — import contribution/revenue batches as GL deposits, reusing the same adapter framework as the QuickBooks importers
- **Modular import-adapter architecture** — new sources (Xero, Wave, etc.) can be added without touching core ledger code
- **Reconciliation reporting** — compare this system's ledger against a live QuickBooks import during the parallel-run validation period

See [`.planning/REQUIREMENTS.md`](.planning/REQUIREMENTS.md) for the full requirement list and [`.planning/ROADMAP.md`](.planning/ROADMAP.md) for the phase-by-phase build plan.

## Status

**Phase 1 of 7 complete:** Core Ledger & Fund-Accounting Data Model.

| Phase | Status |
|-------|--------|
| 1. Core Ledger & Fund-Accounting Data Model | ✅ Complete |
| 2. Auth, RBAC & Backup/DR Foundations | ⬜ In progress |
| 3. Bundled Secure Remote Access | ⬜ Not started |
| 4. Import-Adapter Framework + QuickBooks & CiviCRM Adapters | ⬜ Not started |
| 5. Reconciliation & Parallel-Run Reporting | ⬜ Not started |
| 6. AR/AP, Bank Reconciliation, Grants & Financial Statements | ⬜ Not started |
| 7. PWA Client & Offline Shell | ⬜ Not started |

Not yet usable for real books. Do not point this at production financial data.

## Architecture

- **Backend:** Go, single cross-compiled binary — no C toolchain, no external services to install
- **Database:** SQLite (WAL mode), embedded — no separate database server to run
- **Ledger design:** append-only journal with database-enforced immutability (SQLite triggers block `UPDATE`/`DELETE` on posted rows); corrections happen only via reversing entries
- **Remote access (planned):** embedded Tailscale (`tsnet`) — no separate daemon, no manual networking setup for non-technical orgs
- **Frontend (planned):** React + Vite PWA

## Getting started (development)

```bash
go build ./...
go test ./...
go run ./cmd/server
```

The server opens (and migrates) a local SQLite database and serves a health check at `/healthz`. Configure with environment variables:

- `LEDGER_DB_PATH` — path to the SQLite database file (defaults to a local file in the working directory)
- `LEDGER_PORT` — port to listen on (default `8080`)

## License

[AGPL-3.0](LICENSE). Chosen deliberately: it lets any nonprofit self-host and use this software freely, but if someone runs a modified version as a hosted service for others, they must release their changes back. This protects against the exact vendor-lock-in dynamic this project exists to escape — self-hosting for your own org's internal use carries no additional obligation.

## Contributing

This project is in early development, built initially against one pilot organization's real books. Issues and discussion welcome; the architecture (import-adapter interface, ledger schema) is intentionally designed to generalize beyond the pilot org once the core is proven.
