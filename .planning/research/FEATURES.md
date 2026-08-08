# Feature Research

**Domain:** Nonprofit fund accounting software (self-hosted QuickBooks Desktop replacement)
**Researched:** 2026-08-08
**Confidence:** MEDIUM (WebSearch-verified across multiple vendor/industry sources; no Context7/official-doc coverage exists for this product category, so treat individual vendor feature claims as directional, not exact)

## Feature Landscape

### Table Stakes (Users Expect These)

These are non-negotiable for a bookkeeper or ED to trust this as a real QuickBooks Desktop replacement for a 990-filing nonprofit. Missing any of these makes the product a toy, not a books system.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Double-entry general ledger | Foundation of all accounting software; nothing above it works without it | MEDIUM | Standard debit/credit engine, journal entries, posting periods, period close/lock |
| Chart of accounts (customizable) | Every product (QB, Aplos, Sage Intacct, Xero) lets orgs define their own COA structure; O'Brien's existing COA must import cleanly | LOW-MEDIUM | Needs account types (asset/liability/equity/revenue/expense), parent/sub-account hierarchy, active/inactive flags |
| Fund accounting: restricted / temporarily restricted / unrestricted | This is *the* feature that separates nonprofit accounting software from generic small-business tools (QuickBooks proper famously lacks it — cited repeatedly as the reason nonprofits outgrow it) | HIGH | Requires net-asset classification per FASB ASC 958 (post-2018 GAAP collapsed "temp restricted" + "unrestricted" into "with donor restrictions" / "without donor restrictions" — verify current terminology before building UI labels) |
| Fund-level balances & fund-tagged transactions | Bookkeepers need to answer "what's our balance in the Capital Campaign fund right now" instantly | MEDIUM | Every transaction line needs a fund dimension, not just an account; fund balance = running sum, needs to reconcile against GL total |
| Functional expense allocation (program/admin/fundraising) | Directly required for Form 990 Part IX and the Statement of Functional Expenses; charity watchdogs (Charity Navigator, BBB) score orgs on this ratio | HIGH | Needs allocation rules (percentage split, per-transaction tagging, or class/tag-based like QBO's workaround) — this is one of the harder features to get right; multiple vendors treat it as a premium/differentiating feature |
| Accounts Receivable | Standard bookkeeping expectation; pledges/invoices, aging | MEDIUM | Nonprofits often also need "pledges receivable" as a variant — verify if O'Brien needs pledge tracking or just standard AR |
| Accounts Payable | Standard bookkeeping expectation; vendor bills, aging, payment tracking | MEDIUM | |
| Bank reconciliation | Universal expectation across every product researched (Aplos, QBO, Sage Intacct, Xero, GnuCash) — 2026 vendors are adding AI-assisted matching, but manual reconciliation is the floor | MEDIUM-HIGH | Needs bank statement import (CSV/OFX/QFX), transaction matching UI, reconciled/unreconciled state, discrepancy reporting |
| Core financial statements: Statement of Financial Position (Balance Sheet), Statement of Activities (Income Statement), Statement of Functional Expenses | These three reports are what a CPA needs to prepare Form 990; without them the system cannot actually replace QuickBooks for this org | HIGH | Statement of Cash Flows is also typically expected as a fourth standard nonprofit statement — flag as likely table-stakes even though not explicitly listed in PROJECT.md |
| Multi-user roles/permissions (staff, admin, external accountant) | Already scoped in PROJECT.md; every competitor product (Sage Intacct, Blackbaud, QBO Accountant) has an "accountant/read-write-scoped" role because outside CPAs routinely need books access at year-end and for 990 prep | MEDIUM | External-accountant role should default to a narrower scope than admin (e.g., can view all, edit GL/reports, but not manage users or org settings) |
| Audit trail / immutable transaction log | Aplos explicitly calls this out as a 2026 feature tied to FASB compliance; auditors and CPAs expect a "who changed what, when" log, especially since QuickBooks Desktop itself has this | MEDIUM | Needed for credibility with an outside auditor/CPA reviewing books, and for the parallel-run reconciliation story (proving nothing was silently edited) |
| Data export (CSV, PDF reports, standard journal export) | Baseline expectation — bookkeepers need to hand a CPA a trial balance or export data for tax prep software | LOW-MEDIUM | |
| Attachments/receipts on transactions | Standard modern-accounting-software expectation (QBO, Xero, Aplos all support this); useful for grant documentation and audit support | LOW-MEDIUM | Store as local files given the self-hosted, local-first design |

### Differentiators (Competitive Advantage)

These align directly with the Core Value in PROJECT.md (data sovereignty, no vendor lock-in) and are genuinely where this product can beat the incumbents, not just match them.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Local-first, self-hosted, offline-capable | No competitor researched (Aplos, Sage Intacct, Blackbaud, QBO, Xero) offers this — they are all cloud SaaS. This is the single biggest differentiator and the entire reason the project exists (QB Desktop sunset leaves nonprofits with only cloud options) | HIGH | This is the core bet; everything else supports it |
| Bundled secure remote access (no networking setup required) | Solves the actual reason nonprofits fear self-hosting: "how does our outside accountant log in without me configuring a VPN." Competitors don't need this because they're already cloud-hosted, so there's no existing playbook to copy — must be designed carefully | HIGH | Tailscale/Cloudflare Tunnel-style bundling; must "just work" for a non-technical ED/bookkeeper. This is a genuine UX innovation, not just a technical add-on |
| Parallel-run reconciliation/comparison against live QuickBooks | No mainstream competitor supports "run alongside your existing system with repeated re-import and diff reporting" — QBO/QBDT migration tools are all one-shot, one-directional cutover tools. This directly de-risks adoption during the QB Desktop sunset crunch | HIGH | This is a genuinely novel feature in the category; it is also the highest-complexity item and should get dedicated phase-level research later (idempotent re-import, transaction matching/diffing logic, handling of edits made in QB after initial import) |
| Modular import-adapter architecture (QB Desktop, QBO, and future Xero/Wave) | Vendors don't build for "leaving a competitor easily" — most competitor onboarding tools are QBO-only or paid-consultant-driven migrations. Making this modular and open is a real edge for the broader small-nonprofit market beyond O'Brien | MEDIUM-HIGH | Adapter interface should be scoped/designed early even if only QB adapters ship in v1, since PROJECT.md explicitly wants this extensible without core rework |
| No per-org/per-seat SaaS pricing, zero recurring vendor dependency | Aplos, Sage Intacct, Blackbaud are all subscription-priced (Sage Intacct and Blackbaud notably expensive/enterprise-tier); self-hosted OSS eliminates this permanently | LOW (business model, not engineering) | Directly maps to Core Value: "no dependency on a vendor that can raise prices, get acquired, or discontinue the product" |
| Grant-to-fund linkage with spend-down/drawdown tracking, purpose-built for small orgs | Sage Intacct and dedicated grant tools (AmpliFund, Instrumentl) do this well but only at enterprise price points; Aplos/QBO nonprofit workarounds are weak here. A focused, free, well-designed grant module for small orgs is a real gap | MEDIUM-HIGH | Award amount, remaining balance, reporting-period tracking (which may not match fiscal year — verified as a real requirement from grant-tracking research), tie grant restrictions to specific funds |
| Immutable audit log surfaced as a trust feature, not just compliance checkbox | Because this software is self-hosted (no vendor watching over your shoulder), a strong, user-visible audit trail becomes a trust-building differentiator rather than a back-office compliance detail — it's the equivalent of "prove to the board/auditor nothing was tampered with" | MEDIUM | Builds on the table-stakes audit trail item; differentiator is making it visible/exportable/verifiable rather than just present |

### Anti-Features (Commonly Requested, Often Problematic)

Explicitly scoped OUT in PROJECT.md, confirmed by ecosystem research as legitimate scope traps.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Multi-tenant hosted SaaS | Seems like the obvious way to reach more nonprofits and generate revenue/sustainability for the OSS project; every competitor is SaaS | Contradicts the entire "your data stays on your hardware" value proposition; multiplies security/compliance/ops burden (each tenant's financial + PII data now sits behind your uptime and breach risk); turns a lean open-source tool into an operations business overnight | Ship self-hosted OSS with excellent one-command deploy (Docker/NAS-friendly); consider a *future*, clearly separate hosted-option product only after core validates, not v1 |
| Payroll processing | QuickBooks Desktop did payroll, so "feature parity" instinct says to build it | Payroll has enormous compliance surface: federal/state tax withholding tables, quarterly filings, W-2/1099 generation, state-by-state unemployment rules — a single bug can create real regulatory liability for a nonprofit; already explicitly out of scope in PROJECT.md | Integrate with or recommend existing payroll providers (Gusto, QuickBooks Payroll standalone, ADP) via journal-entry import of payroll summary transactions into the GL |
| Donor CRM / fundraising / POS features | "It's all money in and out, why not track donors too" is a natural feature-creep request, especially since Aplos and Blackbaud bundle CRM+accounting | O'Brien already runs CiviCRM for this; duplicating donor management invites two systems of record for the same donor data (classic data-integrity trap), and CRM is a totally different product surface (event registration, email campaigns, membership tiers) that dilutes focus from the accounting core | Keep strictly to books; support clean import of *revenue transactions* (e.g., a CiviCRM contribution batch export becoming a GL deposit) rather than reimplementing donor tracking |
| Real-time bank feed auto-sync via third-party aggregators (Plaid-style) | Modern SaaS competitors (Aplos 2026, QBO, Xero) all tout automatic bank feed connections and AI-assisted matching, so it "feels" like table stakes | Requires trusting a third-party financial-data aggregator with bank credentials/API access — directly contradicts the local-first, no-cloud-dependency, no-third-party-telemetry constraint in PROJECT.md; also adds an ongoing vendor dependency (aggregator API changes/pricing) that this whole project exists to avoid | Support manual bank statement file import (CSV/OFX/QFX) with smart transaction-matching UI — gets 90% of the reconciliation benefit without a cloud dependency |
| Comprehensive built-in AI/ML features (auto-categorization copilot, natural-language reporting, etc.) | Competitive pressure — Aplos and others are actively marketing AI bank-matching and automation for 2026 | High implementation cost, unclear reliability for financial categorization where errors have real audit/compliance consequences, and risks becoming a black box that undermines the "you can trust and verify these books" value prop of a transparent local system | Simple rule-based auto-categorization (memorized vendor mappings, recurring transaction templates) covers most of the practical benefit without ML risk; revisit AI assistance as an optional, clearly-labeled v2+ feature once core trust is established |
| Full ERP/ecosystem features (inventory, fixed-asset depreciation schedules, multi-entity consolidation, budgeting/forecasting modules) | Sage Intacct and Blackbaud bundle these as "nonprofit ERP" and it's tempting to chase feature parity with the high end of the market | O'Brien is a single-entity small museum nonprofit; multi-entity consolidation, complex fixed-asset depreciation engines, and inventory modules are enterprise-tier features that add massive complexity for zero benefit to the pilot org and most small-nonprofit target users | Ship a simple fixed-asset register (asset name, cost, date, straight-line depreciation) if genuinely needed for the 990/balance sheet, but do not build a full asset-management module; defer multi-entity entirely — single-tenant-per-instance already assumes one org |
| Custom report builder / BI dashboard tooling | Enterprise nonprofit tools (Sage Intacct's "dimensional reporting") market this heavily as a differentiator | Building a flexible ad-hoc report/query builder is a large, open-ended engineering investment (arguably its own product) that isn't needed to prepare a 990 or run day-to-day fund accounting for a small museum | Ship a fixed, well-designed set of standard reports (the four core financial statements + fund balance report + budget-vs-actual + grant spend-down report) covering documented needs; add a generic report builder only if concrete unmet reporting needs emerge post-launch |

## Feature Dependencies

```
Double-entry GL + Chart of Accounts
    └──requires──> (foundation for everything below)

Fund accounting (restricted/unrestricted/temp-restricted)
    └──requires──> Double-entry GL (fund is a dimension on each transaction line)

Functional expense allocation (program/admin/fundraising)
    └──requires──> Chart of Accounts + Fund accounting (allocation rules need account + fund context)

Grant tracking (award, spend-down, reporting periods)
    └──requires──> Fund accounting (grants are typically implemented as restricted funds with extra metadata)

Financial statements (Statement of Financial Position, Statement of Activities, Statement of Functional Expenses)
    └──requires──> Double-entry GL
    └──requires──> Fund accounting
    └──requires──> Functional expense allocation

AR / AP
    └──requires──> Double-entry GL + Chart of Accounts

Bank reconciliation
    └──requires──> Double-entry GL
    └──enhances──> Parallel-run reconciliation/comparison (same matching engine, different comparison target)

QuickBooks import adapters (IIF/QBXML, QBO/CSV)
    └──requires──> Double-entry GL + Chart of Accounts + Fund accounting (import must map into fund/COA structures)

Incremental/re-importable import
    └──requires──> QuickBooks import adapters
    └──requires──> Audit trail (need to know what came from import vs. manual entry, to avoid duplicate/conflicting edits)

Parallel-run reconciliation/comparison reporting
    └──requires──> Incremental/re-importable import
    └──requires──> Bank reconciliation matching logic (reused for QB-vs-ledger diffing, not just bank-vs-ledger)

Bundled secure remote access
    └──requires──> Multi-user roles/permissions (external accountant role must exist before remote login is meaningful)

Audit trail / immutable log
    └──enhances──> Multi-user roles/permissions (attributes changes to specific users)
    └──enhances──> Parallel-run reconciliation (proves import integrity)

Real-time third-party bank feeds (ANTI-FEATURE) ──conflicts──> Local-first / no-cloud-dependency constraint
Multi-tenant SaaS (ANTI-FEATURE) ──conflicts──> Single-tenant self-hosted architecture
```

### Dependency Notes

- **Fund accounting requires double-entry GL:** funds are not a separate ledger — they are a dimension/tag on every GL transaction line, so the core posting engine must support multi-dimensional tagging (account + fund, minimum) from day one. Retrofitting fund-dimension support onto a GL not designed for it is a classic rewrite trigger — build this in from the start, not bolted on later.
- **Financial statements require fund accounting + functional expense allocation:** the Statement of Activities and Statement of Financial Position must both be presentable "with donor restrictions" / "without donor restrictions" per FASB ASC 958, and the Statement of Functional Expenses is impossible without allocation data existing. These three table-stakes reporting features cannot ship before their dependencies — this strongly implies GL + funds + allocation must be an early phase, reports a later phase.
- **Grant tracking requires fund accounting:** the cleanest architecture (seen implicitly across Sage Intacct and Aplos's approach) is to model each grant as (or as tightly linked to) a restricted fund, with additional metadata (award amount, reporting period, funder). Building grant tracking before fund accounting exists would mean re-doing the grant model later.
- **Parallel-run reconciliation enhances (reuses) bank reconciliation:** the matching/diffing algorithm needed to compare "our ledger vs. bank statement" is structurally the same problem as "our ledger vs. QuickBooks's ledger." Building the bank-rec matching engine with this reuse in mind avoids building two separate diffing systems.
- **Incremental re-import requires audit trail:** without a clear record of which transactions originated from an import (and which import run), re-importing QuickBooks data repeatedly during the parallel-run period risks silent duplication or overwriting manual corrections. This dependency is easy to underestimate and should be flagged for deeper phase-specific research.
- **Real-time bank feeds conflicts with local-first constraint:** this is the clearest anti-feature conflict — any third-party financial aggregator (Plaid, Finicity, etc.) requires a live external API relationship that contradicts "no dependency on a vendor" and "no cloud sync of financial data by default." Manual file import is the correct alternative.

## MVP Definition

### Launch With (v1)

Minimum to actually replace QuickBooks Desktop for a 990-filing nonprofit — matches PROJECT.md's "Active" requirements almost exactly, which is a good sign the scoping is already close to right.

- [ ] Double-entry GL with customizable chart of accounts — nothing works without this
- [ ] Fund accounting (restricted / temporarily restricted / unrestricted, FASB-ASC-958-correct net asset classification) — the core differentiator vs. plain QuickBooks
- [ ] Fund-level balances and fund-tagged transactions
- [ ] Functional expense allocation (program/admin/fundraising) — required for 990
- [ ] AR and AP — baseline bookkeeping expectation
- [ ] Bank reconciliation via manual statement import (CSV/OFX/QFX) — no third-party feed dependency
- [ ] Grant tracking (award amount, spend-down, reporting periods, tied to restricted funds)
- [ ] Statement of Financial Position, Statement of Activities, Statement of Functional Expenses (990-ready)
- [ ] Statement of Cash Flows — likely needed alongside the above three; verify with O'Brien's CPA whether their 990 prep workflow requires it
- [ ] Roles: staff/bookkeeper, admin, external accountant (scoped read/write)
- [ ] Bundled secure remote access mechanism
- [ ] Audit trail / immutable transaction log — needed for CPA/auditor trust and for parallel-run integrity
- [ ] QuickBooks Desktop import (IIF/QBXML)
- [ ] QuickBooks Online export import (QBO/CSV)
- [ ] Modular import-adapter interface (even if only QB adapters ship in v1)
- [ ] Incremental/re-importable import support
- [ ] Parallel-run reconciliation/comparison reporting against live QuickBooks
- [ ] Local-only data storage, no telemetry
- [ ] PWA shell: installable, works offline for viewing/light entry

### Add After Validation (v1.x)

Add once the pilot org (O'Brien) has successfully run parallel and cut over.

- [ ] Attachments/receipts on transactions — real value, but not blocking for parallel-run validation
- [ ] Budget vs. actual reporting (org-wide and per-grant) — commonly requested once basic fund/grant tracking is trusted
- [ ] Simple fixed-asset register with straight-line depreciation — needed for accurate 990/balance sheet, but can be manually journal-entered in v1
- [ ] Recurring transaction templates / memorized transactions — quality-of-life once daily use begins
- [ ] Rule-based (non-ML) auto-categorization for bank transactions
- [ ] Additional import adapters (Xero, Wave) — trigger: a second pilot/adopter org needs one

### Future Consideration (v2+)

Defer until the core product has proven trustworthy for at least one full fiscal year / one 990 filing cycle.

- [ ] Multi-entity support — only relevant if a future adopter has parent/subsidiary structure; O'Brien is single-entity
- [ ] Optional hosted/managed-instance offering — a fundamentally different product (multi-tenant SaaS), deliberately deferred per PROJECT.md
- [ ] AI-assisted bank matching / categorization copilot — wait until rule-based matching proves insufficient and trust in the core system is established
- [ ] Custom/ad-hoc report builder — wait for concrete unmet reporting requests beyond the fixed standard report set
- [ ] Pledge receivable tracking (distinct from standard AR) — only if a future org's revenue model needs it; confirm with O'Brien whether this applies to them

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| Double-entry GL + Chart of Accounts | HIGH | HIGH | P1 |
| Fund accounting (restricted/unrestricted) | HIGH | HIGH | P1 |
| Functional expense allocation | HIGH | HIGH | P1 |
| AR / AP | HIGH | MEDIUM | P1 |
| Bank reconciliation (manual import) | HIGH | MEDIUM | P1 |
| Grant tracking (award/spend-down/reporting periods) | HIGH | MEDIUM-HIGH | P1 |
| Core financial statements (990-ready) | HIGH | HIGH | P1 |
| Roles (staff/admin/external accountant) | HIGH | MEDIUM | P1 |
| Bundled secure remote access | HIGH | HIGH | P1 |
| Audit trail / immutable log | HIGH | MEDIUM | P1 |
| QB Desktop import (IIF/QBXML) | HIGH | HIGH | P1 |
| QBO export import (QBO/CSV) | HIGH | MEDIUM-HIGH | P1 |
| Modular import-adapter architecture | MEDIUM (v1), HIGH (long-term) | MEDIUM | P1 |
| Incremental re-import support | HIGH | HIGH | P1 |
| Parallel-run reconciliation/comparison | HIGH | HIGH | P1 |
| Attachments/receipts | MEDIUM | LOW-MEDIUM | P2 |
| Budget vs. actual reporting | MEDIUM-HIGH | MEDIUM | P2 |
| Fixed-asset register | MEDIUM | LOW-MEDIUM | P2 |
| Recurring transaction templates | MEDIUM | LOW | P2 |
| Rule-based auto-categorization | MEDIUM | LOW-MEDIUM | P2 |
| Additional adapters (Xero, Wave) | LOW (v1), MEDIUM (long-term) | MEDIUM | P3 |
| Multi-entity support | LOW | HIGH | P3 |
| Hosted/managed offering | LOW (contradicts core value for v1) | HIGH | P3 |
| AI-assisted categorization | LOW-MEDIUM | HIGH | P3 |
| Custom report builder | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (matches PROJECT.md "Active" requirements almost 1:1)
- P2: Should have, add when possible after pilot validates core
- P3: Nice to have, future consideration — several deliberately excluded per anti-features analysis

## Competitor Feature Analysis

| Feature | Aplos | Sage Intacct Nonprofit | Blackbaud Financial Edge NXT | QuickBooks Online (Nonprofit workaround) | GnuCash | Our Approach |
|---------|-------|-------------------------|-------------------------------|---------------------------------------------|---------|---------------|
| Fund accounting | Native, tag/fund-based | Native, dimension-based (strong) | Native, purpose-built (strongest legacy fit) | Not native — workaround via Classes/Locations | Not fund-aware; requires manual multi-book workarounds | Native fund dimension on every GL line, built in from v1 |
| Functional expense allocation | Automated allocation rules | Strong, dimensional reporting | Native | Manual Class/Tag workaround | Not supported natively | Rule-based allocation (percentage split + per-transaction tagging), P1 |
| Grant tracking | Basic-to-moderate | Strong (billing, drawdown, indirect cost rates) | Strong (integrated with Raiser's Edge ecosystem) | Weak/manual | None | Grants modeled as metadata on restricted funds; award/spend-down/reporting period tracked directly |
| Deployment model | Cloud SaaS only | Cloud SaaS only | Cloud SaaS only | Cloud SaaS only | Desktop, single-user, no web | Self-hosted PWA, local-first, offline-capable, multi-user |
| Remote/external accountant access | Standard SaaS login (cloud-hosted by vendor) | Standard SaaS login | Standard SaaS login | Standard SaaS login (Accountant view) | Not supported (single local file) | Bundled secure tunnel (Tailscale/Cloudflare-style), no networking config required |
| QuickBooks Desktop migration | Manual import/CSV, consultant-assisted | Manual import, often via consultant/3rd-party migration service | Manual import, often via consultant | QBDT→QBO official migration tool (one-shot, size-limited) | No native import | Native IIF/QBXML + QBO/CSV adapters, incremental & re-importable, built for parallel run |
| Ongoing cost model | Subscription (per-org tier) | Subscription (enterprise-tier pricing) | Subscription (enterprise-tier, notably high) | Subscription | Free (no vendor, but no support/updates guarantee beyond community) | Free/open-source, self-hosted, no recurring vendor fee |
| Audit trail | Added as 2026 feature (immutable log) | Standard/strong | Standard/strong | Present (QBO has activity log) | Basic | Immutable audit log, P1, positioned as a visible trust feature |
| Bank feeds | AI-assisted auto bank feed matching (2026) | Bank feed integrations | Bank feed integrations | Bank feed integrations | Manual/OFX import only | Manual file import (CSV/OFX/QFX) by design — no third-party aggregator dependency |

## Sources

- [Aplos Fund Accounting Software](https://www.aplos.com/fund-accounting-software)
- [Aplos: Why Fund Accounting Matters as Your Nonprofit Grows](https://www.aplos.com/academy/why-fund-accounting-matters-as-your-nonprofit-grows)
- [Aplos: Statement of Functional Expenses Guide](https://www.aplos.com/academy/statement-of-functional-expenses)
- [Aplos Software Overview 2026 - Software Advice](https://www.softwareadvice.com/nonprofit/aplos-profile/)
- [Sage: Blackbaud vs Sage Intacct Comparison](https://www.sage.com/en-us/sage-business-cloud/intacct/switch-to-sage/blackbaud-alternative/)
- [Sage Intacct Grant Tracking & Management](https://www.sage.com/en-us/sage-business-cloud/intacct/product-capabilities/extended-capabilities/grants-tracking-billing/)
- [Blackbaud Financial Edge NXT vs. QuickBooks](https://www.blackbaud.com/competitors/quickbooks-comparison)
- [Charity Charge: 6 Best Nonprofit Accounting Software Solutions (2026)](https://www.charitycharge.com/nonprofit-resources/nonprofit-accounting-software/)
- [Brazell Consulting: Nonprofit Accounting 101 — Comparing QuickBooks, Aplos, Sage Intacct, Financial Edge](https://www.brazellconsulting.org/blog/nonprofit-accounting-101)
- [KLR: Form 990 Statement of Functional Expenses](https://kahnlitwin.com/blogs/mission-matters-blog/form-990-statement-of-functional-expenses1)
- [Nonprofit Accounting Basics: The Statement of Functional Expenses](https://www.nonprofitaccountingbasics.org/about-us/statement-functional-expenses)
- [Wegner CPAs: Cost Allocation on the IRS Form 990](https://www.wegnercpas.com/cost-allocation-on-the-irs-form-900/)
- [Purple Margins: 990 Functional Expense Breakdown in QuickBooks Online](https://purplemargins.com/nonprofit-990-functional-expenses/)
- [SelectHub: GnuCash vs Odoo Accounting](https://www.selecthub.com/accounting-software/gnucash-vs-odoo-accounting/)
- [Shopify: What Is Open-Source Accounting?](https://www.shopify.com/blog/what-is-open-source-accounting)
- [inFlow: QuickBooks Desktop Discontinued — Best Alternatives 2026](https://www.inflowinventory.com/blog/quickbooks-desktop-discontinued/)
- [Method: QuickBooks Desktop discontinued — Next steps](https://www.method.me/blog/quickbooks-desktop-discontinued/)
- [Mighty Nonprofits: QuickBooks Desktop for Nonprofits Is Gone — Move to Online](https://www.mightynonprofits.com/single-post/why-quickbooks-online-is-now-the-only-option-for-nonprofits-and-how-to-transition-smoothly)
- [Instrumentl: Best Grant Management Software for Nonprofits](https://www.instrumentl.com/blog/best-grant-management-software)
- [Actually: Best Grant Budget Tracking Software for Nonprofits (2026)](https://actuallyfi.com/post/best-grant-budget-tracking-software-for-nonprofits-2026)

**Confidence caveats:**
- Vendor feature claims (Aplos "2026 AI reconciliation," specific FASB compliance labels) are MEDIUM confidence — sourced from vendor marketing/industry blogs via WebSearch, not verified against primary FASB/IRS documentation or vendor changelogs directly.
- FASB ASC 958 net-asset terminology ("with donor restrictions" / "without donor restrictions" replacing the older three-category unrestricted/temp-restricted/permanently-restricted model since 2018) should be double-checked against current IRS Form 990 instructions and FASB ASU 2016-14 text before finalizing UI/data-model labels — this is flagged as a gap requiring dedicated verification, likely during the phase that implements fund accounting.
- No open-source product researched (GnuCash, Odoo, Akaunting, Manager.io) has native nonprofit fund accounting — this space is genuinely underserved by OSS, reinforcing the market gap this project targets, but also means there is no existing OSS codebase/architecture to model the fund-accounting data model after.

---
*Feature research for: Nonprofit fund accounting software (self-hosted)*
*Researched: 2026-08-08*
