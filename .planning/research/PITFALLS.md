# Pitfalls Research

**Domain:** Self-hosted local-first PWA / fund-accounting system (QuickBooks Desktop replacement) for nonprofits
**Researched:** 2026-08-08
**Confidence:** MEDIUM-HIGH (ledger/import/security patterns verified against multiple independent sources; nonprofit-specific fund accounting norms verified against practitioner sources; no Context7 library involved since this is architecture/domain research, not a specific library)

## Critical Pitfalls

### Pitfall 1: Mutable ledger rows / balances stored instead of derived

**What goes wrong:**
Developers new to accounting systems build a ledger where account balances are stored as a mutable column (`accounts.balance`) that gets incremented/decremented on each transaction, and/or allow `UPDATE`/`DELETE` on posted transaction rows to "fix mistakes." Once this happens, the audit trail is gone — an inconsistency can never be explained after the fact, and a bug (or malicious edit) silently corrupts historical financial statements that may already have been filed with the IRS or handed to an auditor.

**Why it happens:**
It's the "obvious" naive schema (a bank balance sitting on the account row), it's faster to code, and it works fine in demos. The failure mode only appears once two writers race, or once someone needs to explain "why does this Statement of Financial Position not match what we filed last year."

**How to avoid:**
- Ledger entries table is **append-only**: no UPDATE, no DELETE on posted entries (enforce at the DB layer with triggers/permissions, not just app logic).
- Every transaction must be a balanced double-entry (debits == credits, or fund-balanced equivalent) enforced at the DB transaction boundary — reject unbalanced writes.
- Balances are **always derived** (SUM over ledger entries as-of a date), never stored as the source of truth. Materialized/cached balances are fine for performance but must be demonstrably rebuildable from the ledger and periodically reconciled against it.
- Corrections are made via reversing/adjusting entries that reference the original entry, never in-place edits. This mirrors GAAP audit-trail expectations and is what a real accountant/auditor will expect to see.

**Warning signs:**
- Any code path that does `UPDATE ledger_entries SET amount = ...` after posting.
- A "balance" column on accounts/funds with no way to prove it matches `SUM(entries)`.
- No concept of "posted" vs "draft" transaction state.

**Phase to address:**
Core ledger/schema design phase (earliest phase, before import or UI work) — this is foundational and expensive to retrofit.

---

### Pitfall 2: One-shot IIF/QBXML/QBO import instead of idempotent, re-runnable import

**What goes wrong:**
The import pipeline is built as "read file → insert rows," which works for the single pilot migration demo but breaks the stated requirement of running in parallel with live QuickBooks and re-importing repeatedly. Re-running the import duplicates every transaction, or (if someone tries to "fix" this by wiping and reloading) destroys any manual corrections/reconciliation notes made in the new system between imports.

**Why it happens:**
QuickBooks export formats (IIF, QBXML, QBO/CSV) have no stable, guaranteed-unique transaction ID that's meaningful across exports in every case — IIF in particular has weak/optional identifiers, and QBO exports may reformat/renumber. Teams often don't design for the "run this 20 times over 3 months" use case until it's too late, because the first import "just works."

**How to avoid:**
- Design for idempotent **upsert** semantics from day one: derive (or synthesize) a stable external-transaction-key per source record (QuickBooks TxnID from QBXML where available; a composite hash of date+amount+account+memo+line-items as fallback for IIF, since IIF often lacks stable IDs) and store it alongside every imported ledger entry.
- Re-import = match on external key → update if changed, insert if new, flag (don't silently drop) if a previously-imported transaction is now missing from the source export (could mean it was voided in QuickBooks).
- Never allow a re-import to touch/overwrite transactions that originated inside the new system (only touch rows tagged as `source=quickbooks_import`).
- Log every import run (file, timestamp, counts of inserted/updated/skipped/conflicted) so staff can audit what happened without being a developer.

**Warning signs:**
- Import script has no concept of "have I seen this transaction before."
- Testing the importer only once per test fixture instead of running it 2-3 times against overlapping exports.
- No visible import history/log in the UI.

**Phase to address:**
Import-adapter architecture phase, before building the QuickBooks Desktop/Online adapters — the idempotency contract belongs in the adapter framework, not bolted onto each format parser later.

---

### Pitfall 3: Treating real-world IIF/QBXML/QBO exports as clean, well-formed data

**What goes wrong:**
IIF is a strict, whitespace-sensitive, tab-delimited format where a single missing tab, wrong header, or mismatched name silently corrupts or fails the parse — and QuickBooks itself is known to fail either loudly (generic errors) or **silently** (data imports incomplete/wrong with no error at all). Files generated or re-saved on non-Windows tooling can pick up a UTF-8 BOM that breaks the first line, and encoding/line-ending mismatches (LF vs CRLF) cause older-tooling failures. Building the parser against one clean sample file and assuming production exports look the same is the single most common way this class of importer breaks.

**Why it happens:**
Developers test against one export from one QuickBooks version/company file and generalize. Real orgs' QuickBooks files accumulate 10-20 years of manual data entry cruft: inconsistent account naming, split/multi-line transactions, negative-vs-positive sign conventions that vary by transaction type, memo fields used inconsistently, voided/deleted transactions that leave gaps, and custom list items.

**How to avoid:**
- Build the importer to validate structurally (headers present, correct column counts, parseable dates/amounts) and **fail loud with a specific line/row error**, never partial-silent-import.
- Provide a "dry run" / preview mode showing exactly what will be created/changed before committing, and a diff report after.
- Test against O'Brien's actual real export (already planned as pilot data) plus at least one deliberately messy/edge-case file (split transactions, voided txns, unusual account structures) before calling the importer done.
- Explicitly handle: BOM stripping, CRLF/LF normalization, and QuickBooks' AR/AP account import restrictions (QuickBooks itself won't let you IIF-import directly into A/R-type accounts — the importer needs an explicit strategy for those transaction types, likely via QBXML or manual entry acceptance).

**Warning signs:**
- Importer only ever tested against a single synthetic/sample file.
- No preview/dry-run step before committing an import.
- No per-row error reporting — failures are all-or-nothing.

**Phase to address:**
QuickBooks import adapter build-out phase — should include a dedicated "messy real export" test pass using O'Brien's actual historical data before the parallel-run milestone.

---

### Pitfall 4: No reconciliation tooling, so "parallel run" is just two separate systems no one compares

**What goes wrong:**
The org runs the new system and QuickBooks side-by-side as instructed, but without built-in comparison tooling, "parallel run" becomes "two ledgers that quietly drift apart," and nobody notices until the cutover date when trial balances don't match and there's no way to tell which of hundreds of transactions is the discrepancy.

**Why it happens:**
Reconciliation is treated as a manual/future task ("we'll just eyeball the reports") rather than a first-class feature, because it doesn't feel like "the app" — it feels like an ops/QA process. But per the project's own explicit requirement, this has to be a built-in feature, not a manual spreadsheet exercise.

**How to avoid:**
- Build a reconciliation report that runs on-demand: pulls current QuickBooks export + current app state, matches transactions by the same external-key logic as import, and surfaces three buckets — matched, in-QuickBooks-not-in-app, in-app-not-in-QuickBooks — plus balance-by-account/fund comparison.
- Make discrepancies actionable in the UI (not just a diff dump): show enough context (date, amount, memo, account) for a bookkeeper to identify and resolve without touching a database.
- Track reconciliation history over time so staff can see "we were in sync as of last Tuesday" rather than only a point-in-time snapshot.

**Warning signs:**
- Reconciliation described only as "compare the reports at the end" with no tooling.
- No automated way to detect an account whose derived balance disagrees with QuickBooks' reported balance for the same period.

**Phase to address:**
Should be its own phase (or a clearly-scoped feature within the import phase) that ships before the parallel-run validation period begins with O'Brien — it's the entire point of the parallel-run strategy and has to exist before real dependence on it starts.

---

### Pitfall 5: External accountant remote access designed as an afterthought bolt-on

**What goes wrong:**
"Secure remote access" gets implemented as a generic VPN/tunnel exposing the whole app to anyone with the link/credentials, with the external accountant given the same trust level as internal admin staff (or worse, a shared login). If the accountant's device or credentials are compromised, the attacker has full read/write access to the org's complete financial ledger and potentially a network foothold into the org's other local systems (this is a real external party with less operational trust than staff — the actual threat model here is closer to "vendor with remote access" than "trusted employee").

**Why it happens:**
Remote access and RBAC are usually built as two separate concerns bolted together late — "let's add Tailscale" and "let's add roles" — rather than designed together from the start as one trust boundary. Teams also default to "simplest tunnel that works" (e.g., exposing the whole local server on the tunnel) rather than scoping what's reachable.

**How to avoid:**
- Decide the tunnel model deliberately: Cloudflare-Tunnel-style (outbound-only connection, no open inbound ports, but public-URL-reachable and only as safe as app-layer auth) vs. Tailscale-style (private overlay network, only enrolled/authenticated devices can even reach the server at the network layer). For a lower-trust external party, network-layer restriction (Tailscale-style, invite the accountant's device onto the tailnet with an ACL) is meaningfully safer than "public URL + password," because it removes the app from internet-wide attack surface entirely.
- Enforce role scoping at the application layer regardless of tunnel choice: external-accountant role should be defined with the narrowest privileges that make the role useful (e.g., read + specific write actions like reconciliation notes, not admin/user-management, not raw DB access, not ability to void/delete posted transactions).
- Every external-accountant session should be logged (login time, IP/device, actions taken) and ideally time-boxed (e.g., access windows, not standing always-on access) — non-technical staff need a way to see "who logged in and when" without reading server logs.
- Require MFA for the external-accountant role specifically, even if internal staff roles get a lighter bar initially — the accountant is genuinely the higher-risk credential to lose.

**Warning signs:**
- Design docs mention "give the accountant a login" without a distinct permission model from staff/admin.
- Tunnel setup exposes the full app surface (including admin panel) rather than a scoped view.
- No session/access audit log visible to org admins.

**Phase to address:**
Auth + roles phase should be designed jointly with the remote-access phase (or sequenced so remote access follows and consumes a finished RBAC model) — never ship remote access before roles are finalized.

---

### Pitfall 6: Backup/disaster-recovery designed for a technical operator, not a nonprofit volunteer/staff member

**What goes wrong:**
The app relies on the person running the server (often a non-technical volunteer or part-time staff, on a random NAS/old PC/laptop) to configure their own backups, understand restore procedures, or notice when backups have silently stopped working. Financial data for a 990-filing nonprofit — the org's entire audit trail — lives on a single machine with no offsite/immutable copy, and the first time anyone tests "restore" is during an actual disaster (hardware failure, ransomware, theft, spilled coffee), by which point it's too late. This is the single highest-consequence failure mode for a local-first financial app, because unlike QuickBooks Online, there's no vendor safety net.

**Why it happens:**
Backups are treated as a deployment/ops afterthought rather than a core product feature, and "local-first" gets conflated with "backups are the user's problem." The classic 3-2-1 pattern (three copies, two media, one offsite) is second nature to IT teams but invisible to a museum volunteer running the server. Recovery workflows are usually never tested until a real disaster, at which point manual/unclear restore steps cause extended data loss or downtime.

**How to avoid:**
- Ship automated, scheduled backups as a built-in feature, not a manual task — e.g., nightly encrypted DB dump to a location distinct from the primary disk (external drive, or optional user-configured offsite/cloud target), with a visible "last successful backup: [timestamp]" indicator on the admin dashboard so staff notice immediately if it stops.
- Provide a one-click/guided restore flow, tested as a first-class feature (not just an internal dev script) — the org's IT advisor (or even non-technical staff, worst case) needs to be able to execute it under stress without reading source code.
- Because this is single-tenant financial data with real audit/compliance stakes, recommend (and make easy) at least one offsite/off-machine copy — don't let "self-hosted" become "single point of failure with no copy at all."
- Periodically self-verify backup integrity (e.g., checksum, or actually attempt a restore-to-scratch-DB test on a schedule) rather than assuming a backup file that exists is a backup file that works.

**Warning signs:**
- Backup strategy described only as "the user should back up their data" with no built-in mechanism.
- No dashboard indicator of backup health/recency.
- Restore procedure only exists as a developer runbook, never tested by a non-developer.

**Phase to address:**
Should be a named phase early in the roadmap (soon after core ledger + before the parallel-run milestone) — not deferred to "polish," because O'Brien's real financial data will be at risk starting the moment the pilot import happens.

---

### Pitfall 7: Audit trail and compliance framing bolted on instead of designed for 990-readiness from the start

**What goes wrong:**
The team builds "a nice ledger" and assumes financial statements can be generated later, then discovers close to launch that the data model can't actually produce what Form 990 prep requires: net assets split by "with donor restrictions" vs. "without donor restrictions" (990 Part X lines 27-28), a functional expense statement (program/management/fundraising allocation) with a defensible allocation methodology, and a full change-history/audit trail an accountant or auditor can rely on. Retrofitting fund-restriction tracking and functional expense categorization onto transactions that were never tagged with that metadata at entry time is a painful, error-prone historical-data cleanup project.

**Why it happens:**
Generic double-entry ledger designs (and most tutorials/open-source ledger examples) model businesses, not nonprofits — they don't have a native concept of "fund" as a first-class dimension alongside account, or of functional expense classification, because for-profit accounting doesn't need either. It's easy to build "QuickBooks but simpler" and only discover the gap when someone tries to actually prepare a 990.

**How to avoid:**
- Model fund (restricted/unrestricted/temporarily-restricted) and functional-expense-category as first-class, required dimensions on every relevant ledger entry from day one — not optional metadata added later. Every expense transaction should require a program/admin/fundraising allocation at entry time (or a documented allocation rule), because retrofitting this onto years of historical transactions is effectively impossible to do accurately.
- Build the core financial statements (Statement of Financial Position, Statement of Activities, functional expense statement) as an early deliverable/validation target, not a late "reporting" phase — if the ledger schema can't produce these correctly against O'Brien's real chart of accounts early, the schema is wrong and needs to change before more data/history depends on it.
- Every posted transaction needs a durable audit trail: who entered it, when, from what source (manual entry vs. which import run), and full history of any reversing/adjusting entries — this is what a 990 preparer or auditor will ask for, and "we can query the database" is not the same as "there's a report for this."
- Do not mix up "restricted fund tracking" with a generic tagging/label system that could accidentally let restricted and unrestricted funds be posted to the same bucket — the classic real-world mistake (mixing funds, "borrowing" from restricted funds) needs to be structurally prevented, not just discouraged in the UI copy.

**Warning signs:**
- Ledger schema has "account" and "amount" but no first-class "fund" or "restriction type" dimension.
- No functional-expense-category field required on expense entries.
- Financial statement generation treated as a late/"reporting" phase rather than an early correctness check on the schema.

**Phase to address:**
Core ledger/schema design phase (same phase as Pitfall 1) — fund + functional-expense modeling is inseparable from the base data model, not a feature layered on top.

---

### Pitfall 8: Open-source single-maintainer project handling real financial/PII data without a security disclosure or succession plan

**What goes wrong:**
As a solo/small-team open-source project handling nonprofits' real financial data (and potentially donor/vendor PII depending on chart-of-accounts detail), there's no clear path for a security researcher to report a vulnerability responsibly, no plan for what happens to deployed instances if the maintainer becomes unavailable, and dependency/security patching lags because there's no dedicated bandwidth — this is a well-documented pattern in small open-source projects (contributed to real-world incidents like the XZ Utils backdoor, where maintainer burnout and isolation created an opening for a malicious "helpful" contributor).

**Why it happens:**
Financial-software-specific security process (disclosure policy, dependency scanning, threat modeling for the remote-access surface) is exactly the kind of unglamorous, non-feature work that gets deprioritized on a volunteer timeline, especially under the real deadline pressure from QuickBooks Desktop's discontinuation.

**How to avoid:**
- Publish a lightweight SECURITY.md with a disclosure contact/process from the first public release, even if it's just "email X, will respond within N days."
- Given the project handles real financial data for real organizations (not a toy), be deliberate about dependency hygiene (automated dependency update/scan tooling from the start) since this is cheap to set up early and expensive to retrofit after multiple orgs are running unpatched instances.
- Because self-hosted orgs won't proactively update software themselves (see Pitfall 6's non-technical-operator theme), consider a simple in-app "update available" / version-check notice so orgs know when a security patch exists, without requiring auto-update (which would conflict with "local control" values).
- Keep the trust/contribution model conservative early on (limited maintainers with commit access) precisely because this class of software is a high-value target if compromised at the source level — this is a governance decision, not a code decision, but worth stating explicitly in project docs.

**Warning signs:**
- No SECURITY.md or disclosure process by first public/pilot release.
- Dependencies not pinned or scanned; no visibility into known CVEs in the stack.
- Single point of failure for both maintenance and any credential/signing keys used in releases.

**Phase to address:**
Should be addressed as part of initial open-source project setup (license/governance decisions already flagged as pending in PROJECT.md) — cheap to do at project inception, disproportionately costly to retrofit once other orgs beyond O'Brien are self-hosting real financial data.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Storing account balances as a mutable column instead of deriving from ledger | Faster reads, less code upfront | Balance drift bugs impossible to audit/debug; breaks trust in the numbers | Never for the ledger core; OK only as a rebuildable cache with a documented rebuild/verify job |
| One-shot import script (no idempotency) | Ships the pilot demo faster | Breaks the explicitly-required parallel-run workflow; forces a risky big-bang cutover later | Never — parallel-run is a stated hard requirement, not optional |
| Single shared "remote access" tunnel/credential for all external users | Faster to set up remote access | No per-user audit trail, can't revoke one compromised accountant without breaking all remote access | Never for the external-accountant role; acceptable only for a true single-operator dev/test instance |
| Deferring fund/functional-expense modeling to "add later" | Simpler initial schema, faster MVP ledger | Historical transactions can't be retroactively and accurately fund/function-tagged; 990-readiness becomes a rewrite | Never — this is core to the product's reason for existing (990 prep) |
| Manual/undocumented backup process, "org configures their own backups" | Saves build time on ops tooling | Real risk of unrecoverable data loss for a 990-filing nonprofit's sole financial record | Only acceptable temporarily in a dev/internal-testing phase, never once O'Brien's real data is in the system |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| QuickBooks IIF export | Assuming one clean sample file represents all real exports; not handling BOM/line-ending variance | Build structural validation + row-level error reporting; test against multiple real, messy exports including split/voided transactions |
| QuickBooks IIF import limitation | Not knowing QuickBooks itself blocks IIF import directly into A/R-type accounts | Design the adapter's transaction-type handling explicitly around this constraint (route those txn types through QBXML or a documented manual-entry path) |
| QuickBooks Online (QBO/CSV) export | Treating QBO/CSV as equivalent in structure/fidelity to QBXML | Build a separate adapter with its own field-mapping and gap analysis — CSV especially loses structured metadata QBXML/IIF may retain |
| Cloudflare-Tunnel-style remote access | Exposing the full app surface (including admin) through the tunnel with only app-password protection | Scope what's reachable through the tunnel per role, and prefer network-layer restriction (e.g., Tailscale-style ACLs) for the external-accountant trust boundary specifically |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| External-accountant role granted same privileges as internal admin | A compromised accountant credential/device = full org financial data + network foothold | Narrowest-useful-privilege role definition, MFA required, session logging, time-boxed access where feasible |
| No audit log of who changed/entered what in the ledger | Can't investigate a discrepancy or prove data integrity to an auditor | Append-only ledger + entry-level metadata (who, when, source) as a core schema requirement, not a later feature |
| Backups stored on the same machine/network as production | Ransomware or hardware failure destroys both live data and backups simultaneously | Built-in support for at least one offsite/off-machine backup destination, surfaced clearly in the UI |
| No disclosure process for security reports on the OSS repo | Vulnerabilities found by researchers go unreported or get publicly disclosed with no coordinated fix | SECURITY.md + documented response process from first public release |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Import run gives no preview/dry-run before committing | Non-technical bookkeeper can't tell what an import will do until it's already done, causing anxiety and mistrust of the tool | Dry-run/preview step showing exactly what will be inserted/updated/skipped, with a plain-language summary |
| Reconciliation discrepancies shown as raw data diffs | Bookkeeper can't act on "row 4821 mismatch" without a developer | Present discrepancies with human-meaningful context (date, payee, amount, account/fund) and a suggested resolution action |
| No visible backup status | Staff assume backups are happening; find out otherwise only after data loss | Persistent, obvious "last backup: [time]" status with alerting if it goes stale |
| External accountant access setup requires technical networking knowledge | Non-technical org staff can't actually onboard the accountant, defeating the built-in-remote-access value prop | Guided in-app flow to invite/approve an accountant device/account, no manual firewall/DNS/VPN config required |

## "Looks Done But Isn't" Checklist

- [ ] **Ledger core:** Often missing enforcement that debits==credits at the DB layer — verify an unbalanced transaction is rejected, not just discouraged in the UI.
- [ ] **QuickBooks import:** Often missing repeat-import handling — verify running the same import file twice produces zero duplicate transactions.
- [ ] **Reconciliation:** Often missing balance-level comparison, only transaction-level — verify it can show "QuickBooks says fund X = $Y, app says fund X = $Z" not just transaction matching.
- [ ] **External accountant role:** Often missing distinct audit logging from staff — verify you can answer "what did the accountant do during their last login" without reading raw server logs.
- [ ] **Backups:** Often missing an actual tested restore — verify a full restore-from-backup has been executed successfully at least once, not just that backup files exist.
- [ ] **Financial statements (990-readiness):** Often missing net-assets-with/without-restrictions split — verify Statement of Financial Position correctly separates these per Form 990 Part X lines 27-28 against O'Brien's real chart of accounts.
- [ ] **Functional expense allocation:** Often missing an enforced allocation at transaction entry — verify every expense transaction has (or defaults to) a program/admin/fundraising split, not an optional afterthought field.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|-----------------|------------------|
| Mutable balances found to have drifted from ledger | MEDIUM | Rebuild all balances from the append-only ledger as the recovery/verification job (this is exactly why balances must be derivable — treat any drift discovery as a trigger to run the rebuild + investigate the write path that caused it) |
| Duplicate transactions from a non-idempotent import already run in production | HIGH | Requires a one-time dedup pass keyed on best-available match (date/amount/memo/account) with human review before deletion, since blind auto-delete risks removing legitimate similar transactions; then retrofit idempotent import before any further re-imports |
| Discovered late that fund/functional-expense data wasn't captured historically | HIGH | Requires manual accountant-led reclassification pass against QuickBooks source data before those periods can be considered 990-ready; strongly motivates addressing Pitfall 7 before any real data migration begins |
| Backup never tested, disaster occurs, restore fails | HIGH (potentially unrecoverable) | If backup exists but restore untested, treat every restore attempt as exploratory — document steps as discovered; if no working backup exists, the org's financial history reconstruction falls back to whatever QuickBooks parallel-run data still exists elsewhere (another reason the parallel-run period has real value beyond validation) |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| Mutable ledger / stored balances (P1) | Core ledger & schema design (earliest phase) | Attempt an UPDATE on a posted entry and confirm it's rejected; confirm balances can be fully rebuilt from entries and match cached values |
| Non-idempotent import (P2) | Import-adapter architecture phase | Run the same import file 3x against the same DB state; confirm zero duplicate transactions each time |
| Fragile parsing of messy real exports (P3) | QuickBooks import adapter build-out phase | Run importer against O'Brien's actual historical export plus a deliberately malformed/edge-case file; confirm row-level errors, no silent partial import |
| No reconciliation tooling (P4) | Dedicated reconciliation phase, before parallel-run milestone begins | Run reconciliation report against a known-mismatched pair of datasets; confirm it surfaces the specific discrepancy, not just "totals differ" |
| External accountant access as afterthought (P5) | Auth/RBAC phase, sequenced before remote-access phase | Confirm accountant role cannot access admin/user-management functions; confirm a session log entry is created and visible to org admin per login |
| Backup/DR designed for technical operator (P6) | Dedicated backup/DR phase, early (before parallel-run milestone) | Execute a full restore-from-backup by someone who didn't write the backup code; confirm it succeeds and data matches |
| 990-readiness not designed in from the start (P7) | Core ledger & schema design (same phase as P1) | Generate a real Statement of Financial Position + functional expense statement against O'Brien's actual chart of accounts early; confirm it structurally matches 990 Part X / functional expense requirements |
| No security disclosure/succession plan for OSS project (P8) | Project setup / license & governance decisions (already pending in PROJECT.md) | Confirm SECURITY.md exists and dependency scanning is enabled before first public/pilot release |

## Sources

- [IIF Overview: import kit, sample files, and headers — QuickBooks/Intuit](https://quickbooks.intuit.com/learn-support/en-us/help-article/list-management/iif-overview-import-kit-sample-files-headers/L5CZIpJne_US_en_US)
- [QuickBooks IIF File Format Explained (2026) — Data Conversion Center](https://www.dataconversioncenter.com/blog/iif-file-format-explained/)
- [A Complete Guide to the IIF File Format — Cloudvara](https://cloudvara.com/iif-file-format/)
- [Fix QuickBooks IIF Import Error — Dancing Numbers](https://www.dancingnumbers.com/iif-import-error-in-quickbooks-desktop/)
- [Fix QuickBooks IIF Import & Export Errors — Vision Computers](https://www.visioncomputers.com/quickbooks-data-export-iif-errors)
- [Books, an immutable double-entry accounting database service — Square Developer Blog](https://developer.squareup.com/blog/books-an-immutable-double-entry-accounting-database-service/)
- [Enforcing Immutability in your Double-Entry Ledger — Modern Treasury](https://www.moderntreasury.com/journal/enforcing-immutability-in-your-double-entry-ledger)
- [How to Scale a Ledger, Part V: Immutability and Double-Entry — Modern Treasury](https://www.moderntreasury.com/journal/how-to-scale-a-ledger-part-v)
- [Defining Double-Entry Accounting: A Formal Model for Engineers — Formance](https://www.formance.com/blog/engineering/defining-double-entry)
- [Tailscale vs Cloudflare Tunnel: Which Should You Use? (2026) — Need to Know IT](https://needtoknowit.com.au/blog/tailscale-vs-cloudflare-tunnels-for-remote-access/)
- [Secure Self-Hosting with Cloudflare Tunnels and Docker (Zero Trust) — DEV Community](https://dev.to/mihailtd/secure-self-hosting-with-cloudflare-tunnels-and-docker-zero-trust-security-5bbn)
- [Accounting for Restricted Funds: What Nonprofits Need to Know — NetSuite](https://www.netsuite.com/portal/resource/articles/accounting/restricted-funds-nonprofit-accounting.shtml)
- [Fund Accounting for Nonprofits: Restricted Funds Guide — Nonprofit Bookkeeping](https://nonprofitbookkeeping.com/nonprofit-bookkeeping/fund-accounting-for-nonprofits-understanding-restricted-funds/)
- [Common Nonprofit Accounting Mistakes — Mission Edge](https://www.missionedge.org/news-and-resources/common-nonprofit-accounting-mistakes-and-how-to-avoid-them)
- [Top Form 990 Mistakes Nonprofits Make and How to Avoid Them — Complete Balance CPA](https://www.completebalancecpa.com/blog/avoiding-common-form-990-mistakes)
- [Common IRS Form 990 Mistakes Non-Profits Must Avoid — IKRG CPA](https://www.ikrgcpa.com/common-irs-form-990-mistakes-non-profits-must-avoid-non-profit/)
- [How to make your data pipeline idempotent — Medium](https://medium.com/@iamanjlikaur/ensuring-idempotency-in-data-ingestion-pipelines-33301cf917fb)
- [Idempotency & Deduplication — System Design Sandbox](https://www.systemdesignsandbox.com/learn/idempotency-deduplication)
- [10 Common Mistakes Organizations Make with Their Backup and Recovery Strategies — CDW](https://www.cdw.com/content/cdw/en/articles/services/10-common-mistakes-organizations-their-backup-recovery-strategies.html)
- [Common Mistakes in Disaster Recovery (And How To Avoid Them) — Fusion Risk Management](https://www.fusionrm.com/blogs/common-mistakes-in-it-disaster-recovery-and-how-to-avoid-them/)
- [The Unpaid Backbone of Open Source: Solo Maintainers Face Increasing Risk — Socket.dev](https://socket.dev/blog/the-unpaid-backbone-of-open-source)
- [Open Source Maintainer Burnout and Its Security Implications — Safeguard](https://safeguard.sh/resources/blog/maintainer-burnout-security-implications)
- [Small open source projects pose significant security risks — TechTarget](https://www.techtarget.com/searchsoftwarequality/news/252527749/Small-open-source-projects-pose-significant-security-risks)

---
*Pitfalls research for: self-hosted local-first nonprofit fund-accounting PWA (QuickBooks Desktop replacement)*
*Researched: 2026-08-08*
