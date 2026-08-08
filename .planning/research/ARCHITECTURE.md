# Architecture Research

**Domain:** Self-hosted, single-tenant, local-first fund-accounting system (QuickBooks Desktop replacement) for nonprofits
**Researched:** 2026-08-08
**Confidence:** MEDIUM (patterns verified across multiple credible sources; no single canonical open-source project matches this exact shape — GnuCash/LedgerSMB/Ledger-cli inform the ledger layer, homelab remote-access literature informs the tunnel layer, PWA offline-first literature informs the client layer)

## Standard Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│  CLIENT — PWA (installed, runs in browser/webview)                    │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────────┐    │
│  │ UI Views   │ │ Local Cache│ │ Service     │ │ Auth/Session     │    │
│  │ (GL, funds,│ │ (IndexedDB │ │ Worker      │ │ token store      │    │
│  │ statements)│ │ read cache)│ │ (shell only)│ │                  │    │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └────────┬─────────┘    │
└────────┼──────────────┼──────────────┼─────────────────┼──────────────┘
         │              │              │                 │
         │        HTTPS (LAN direct OR via remote-access tunnel)         
         ▼              ▼              ▼                 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LOCAL WEBSERVER — single-tenant app process (one per org)            │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────────┐    │
│  │ Auth/       │ │ REST/RPC   │ │ Reporting/  │ │ Import-Adapter   │    │
│  │ Session svc │ │ API layer  │ │ Statement   │ │ subsystem        │    │
│  │ (roles,     │ │ (GL, funds,│ │ generation  │ │ (QBD, QBO, CSV,  │    │
│  │ RBAC)       │ │ grants,    │ │ (990-ready  │ │ future: Xero,    │    │
│  │             │ │ AR/AP)     │ │ statements) │ │ Wave)            │    │
│  └──────┬──────┘ └─────┬──────┘ └──────┬──────┘ └────────┬─────────┘    │
│         └──────────────┴──────────────┴─────────────────┘             │
│                              │                                         │
│                     ┌────────▼─────────┐                               │
│                     │  Ledger / Domain  │                               │
│                     │  Engine (double-  │                               │
│                     │  entry posting,   │                               │
│                     │  fund rules,      │                               │
│                     │  reversal logic)  │                               │
│                     └────────┬─────────┘                               │
├──────────────────────────────┼──────────────────────────────────────┤
│                        DATA LAYER                                     │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐     │
│  │ Append-only    │  │ Derived/      │  │ Import staging /       │     │
│  │ journal (GL    │  │ projection    │  │ reconciliation store    │     │
│  │ postings, event│  │ tables (COA   │  │ (raw imported rows,     │     │
│  │ log, immutable)│  │ balances, fund│  │ match/diff results)     │     │
│  │                │  │ ledgers, cache│  │                        │     │
│  └───────────────┘  └───────────────┘  └───────────────────────┘     │
│              SQLite (or Postgres) on local disk — single file/instance│
└──────────────────────────────────────────────────────────────────────┘
                              ▲
                              │ outbound-only connection
                     ┌────────┴─────────┐
                     │ Remote-Access     │
                     │ Tunnel component  │
                     │ (bundled, e.g.    │
                     │ Tailscale Funnel /│
                     │ Cloudflare Tunnel │
                     │ / self-hosted     │
                     │ WireGuard relay)  │
                     └────────┬─────────┘
                              │
                     External accountant's
                     browser (remote PWA client)
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| PWA client | Renders UI, caches read data for offline browsing, queues writes when offline, installable via manifest | React/Vue/Svelte SPA + service worker + IndexedDB (Dexie.js or similar) |
| Local webserver / API layer | Single-tenant app server; owns HTTP API, session validation, request routing | Node.js (Express/Fastify) or Python (FastAPI) — single process, single-org config |
| Auth/session service | Authenticates users, issues session tokens, enforces RBAC (staff, admin, external accountant), scopes external-accountant access | Server-side sessions or short-lived JWT + refresh; bcrypt/argon2 password hashing; role table |
| Ledger / domain engine | Enforces double-entry invariants (debits=credits), fund-accounting rules (restricted/unrestricted), reversal-only corrections, posting validation | Core domain module, framework-agnostic; the "business logic heart" — no ORM leakage of accounting rules |
| Data layer — append-only journal | System of record for every posted transaction; immutable once posted; corrections are new reversing entries, never UPDATE/DELETE | SQLite (or Postgres) table with insert-only constraint at the application layer, ideally reinforced with DB triggers denying UPDATE/DELETE on posted rows |
| Data layer — projections | Fast-read balances, fund ledgers, COA rollups derived from the journal (rebuildable) | Materialized/derived tables or views, recomputed on write or on schedule |
| Import-adapter subsystem | Normalizes external formats (IIF, QBXML, QBO, CSV) into a canonical staging schema before touching the ledger | Plugin-per-source pattern: each adapter implements a shared interface (`parse`, `map`, `stage`), output lands in a common staging schema |
| Import staging / reconciliation store | Holds raw + normalized imported rows separately from posted ledger; supports diffing against QuickBooks and repeated/incremental re-import | Separate tables keyed by external transaction ID + import batch/run ID; supports idempotent re-import (upsert on external ID) |
| Reporting/statement generation | Produces Statement of Financial Position, Statement of Activities, functional expense statement, 990-ready outputs | Read-only queries against projections + journal; templated report generator (server-rendered PDF/HTML) |
| Remote-access/tunnel component | Bundled, outbound-only secure channel so external accountant reaches the local server without org configuring VPN/port-forwarding | Embedded Tailscale Funnel/Serve, Cloudflare Tunnel (cloudflared), or self-hosted WireGuard relay (e.g., Pangolin-style) shipped and auto-configured by the app installer |

## Recommended Project Structure

```
nonprofit-ledger/
├── server/
│   ├── auth/                # session mgmt, RBAC, role checks
│   │   ├── roles.ts         # staff / admin / external-accountant definitions
│   │   └── session.ts
│   ├── ledger/               # core domain engine — no I/O, pure accounting logic
│   │   ├── posting.ts        # double-entry validation, reversal logic
│   │   ├── funds.ts          # restricted/unrestricted rules
│   │   └── coa.ts            # chart of accounts model
│   ├── api/                  # REST/RPC routes, thin controllers only
│   │   ├── gl.ts
│   │   ├── grants.ts
│   │   └── reports.ts
│   ├── reporting/             # statement generation (990-ready outputs)
│   ├── importers/             # adapter subsystem
│   │   ├── base-adapter.ts    # shared interface: parse/map/stage
│   │   ├── quickbooks-desktop/  # IIF, QBXML
│   │   ├── quickbooks-online/   # QBO/CSV
│   │   └── reconciliation.ts   # diff staged vs posted ledger
│   ├── db/
│   │   ├── migrations/
│   │   ├── journal-schema.ts   # append-only postings
│   │   └── projections-schema.ts
│   └── remote-access/          # tunnel bootstrap/config wrapper
├── client/                     # PWA
│   ├── service-worker.ts
│   ├── local-cache/             # IndexedDB read cache + offline write queue
│   └── views/
└── docs/
    └── architecture/
```

### Structure Rationale

- **server/ledger/ isolated from server/api/:** accounting correctness (double-entry, fund rules, no destructive edits) must be enforceable independent of HTTP concerns and independent of which import adapter fed it — this is the auditability boundary.
- **server/importers/ never writes directly to journal-schema:** adapters write only to staging; a separate reconciliation/posting step (reviewed by a human, at least initially) promotes staged data into real ledger postings. This keeps the "system of record" trustworthy even while parallel-running against live QuickBooks.
- **server/remote-access/ as a wrapper, not a fork:** treat the tunnel technology as a pluggable, swappable dependency (Tailscale today, something else later) — same adapter-pattern philosophy as import adapters, applied to networking.

## Architectural Patterns

### Pattern 1: Append-Only Ledger with Reversing Corrections

**What:** Every posted transaction is an immutable row (or event) in a journal table. There is no UPDATE or DELETE path for posted entries — only INSERT of new reversing/correcting entries that reference the original.
**When to use:** Any time financial correctness and audit trail matter (this project, always).
**Trade-offs:** Simpler audit story, trivial "what changed and when" queries, safe against accidental data loss. Costs: more rows over time, current-balance queries need derived/projection tables rather than reading raw state, requires discipline in the API layer (and ideally DB-level constraints) to actually prevent mutation.

**Example:**
```typescript
// Correction is a new entry, not a mutation
async function reverseTransaction(originalTxnId: string, reason: string) {
  const original = await journal.get(originalTxnId);
  return journal.insert({
    type: 'reversal',
    reverses: originalTxnId,
    lines: original.lines.map(l => ({ ...l, debit: l.credit, credit: l.debit })),
    memo: reason,
    postedAt: now(),
  });
}
```

### Pattern 2: Import Staging → Reconciliation → Promotion (never adapter-to-journal direct write)

**What:** Import adapters (QBD/IIF, QBO/CSV, future Xero/Wave) all normalize into one canonical staging schema. A separate reconciliation/comparison step diffs staged transactions against both (a) already-posted ledger entries and (b) the live QuickBooks export, and only then are transactions promoted into the real journal.
**When to use:** Any system needing repeated/incremental re-import and parallel-run validation against a source-of-truth system that's still live (this project's explicit requirement).
**Trade-offs:** Extra layer of indirection and a staging schema to maintain, but this is what makes incremental re-import idempotent (match on external transaction ID) and makes reconciliation reporting possible without corrupting the ledger. Skipping this and writing adapters straight into the journal would make "designed for future adapters without core rework" false in practice.

**Example:**
```typescript
interface ImportAdapter {
  sourceType: 'qb-desktop-iif' | 'qb-desktop-qbxml' | 'qb-online-csv' | 'xero' | 'wave';
  parse(rawFile: Buffer): RawRecord[];
  map(records: RawRecord[]): StagedTransaction[]; // canonical shape
}
// staging table keyed by (sourceType, externalTxnId, importBatchId)
// re-running an import upserts on externalTxnId — idempotent by design
```

### Pattern 3: Local-First PWA with Explicit Offline Scope

**What:** Since the "backend" is a local server (not a cloud API), the PWA's offline story is narrower than typical local-first apps: the service worker caches the app shell and recent read data (IndexedDB) for browsing/viewing when the LAN/server is briefly unreachable; writes are queued and replayed once the local server is reachable again. Full peer-to-peer offline editing (CRDT-style multi-device merge) is explicitly out of scope — there is one authoritative server per org, not a distributed mesh.
**When to use:** Single-tenant local-server deployments where "offline" primarily means "resilient to brief LAN hiccups and installable/app-like," not "fully functional with no server ever."
**Trade-offs:** Much simpler than true local-first sync engines (no conflict resolution needed since there's one write authority); but requires an explicit design decision documented early (this ambiguity is flagged in PROJECT.md) so the roadmap doesn't accidentally scope a full CRDT sync engine that isn't needed.

## Data Flow

### Request Flow (normal transaction entry)

```
[Staff enters transaction in PWA]
    ↓
[Client validates shape] → [POST /api/gl/transactions] → [Auth/session check + RBAC]
    ↓
[Ledger engine validates: debits=credits, fund rules, account exists]
    ↓ (valid)
[INSERT into append-only journal] → [recompute/update projections]
    ↓
[Response: posted transaction] ← [Client updates local cache + UI]
```

### Import/Reconciliation Flow

```
[QuickBooks export file (IIF/QBXML/QBO/CSV)]
    ↓
[Import adapter: parse → map to canonical staged shape]
    ↓
[Staging store] (upsert on external txn ID — safe to re-run)
    ↓
[Reconciliation engine: diff staged vs. posted journal + vs. QB balances]
    ↓
[Comparison report shown to bookkeeper] → [human approves promotion]
    ↓
[Promoted staged txns become real journal postings via normal ledger engine path]
```

### Remote Access Flow (external accountant)

```
[App startup] → [Remote-access component establishes outbound tunnel connection]
    ↓ (no inbound ports opened on org's router)
[External accountant browser] → [Tunnel provider's edge] → [tunnel] → [Local webserver]
    ↓
[Auth/session service: external-accountant role check — scoped permissions]
    ↓
[Same API layer as local staff, but role-gated: e.g., read + limited write, no admin/user-management]
```

### Key Data Flows

1. **Posting flow:** Client → API → Ledger engine → append-only journal → projections. One-directional, no backward mutation.
2. **Import flow:** External file → adapter → staging (isolated from journal) → reconciliation → human-approved promotion → journal. This isolation is what makes parallel-run against live QuickBooks safe.
3. **Remote-access flow:** External accountant traffic never touches the org's router config directly — it rides the bundled tunnel, but hits the exact same auth/RBAC/API layer as local traffic. The tunnel is a network-layer concern only; it must not become a second authorization system.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Single org, single instance (this project's actual target) | SQLite is very likely sufficient — one small nonprofit's transaction volume (thousands to tens of thousands of GL lines/year) is trivial for SQLite; avoids running a separate DB server process on donated/older hardware |
| Multiple concurrent local users (staff + bookkeeper + remote accountant simultaneously) | SQLite handles this fine with WAL mode for a handful of concurrent writers; if contention becomes real, Postgres is a drop-in-ish upgrade since the domain layer should be DB-agnostic |
| Multi-org / hosted future (explicitly out of scope for v1 per PROJECT.md) | Would require re-introducing tenancy at the data layer and rethinking the remote-access model (currently one org = one tunnel identity) — deliberately not designed for now, but note the ledger/domain engine should stay tenant-agnostic internally so it isn't a rewrite later |

### Scaling Priorities

1. **First bottleneck (real one):** Not performance — it's trust and correctness at parallel-run time. The reconciliation reporting needs to be accurate and clear enough that a nonprofit board/accountant will actually trust cutover. Prioritize reconciliation UX and correctness over any performance work.
2. **Second bottleneck:** Import adapter breadth/robustness — QuickBooks Desktop's IIF/QBXML export quirks vary by version and by how messy the org's existing books are (common in small nonprofits: miscategorized funds, negative balances, memorized transactions). Expect adapter edge cases to dominate early implementation effort, not the ledger core.

## Anti-Patterns

### Anti-Pattern 1: Import Adapters Writing Directly to the Journal

**What people do:** Build the QuickBooks importer to directly INSERT posted transactions into the ledger to "save a layer."
**Why it's wrong:** Breaks idempotent re-import (can't safely re-run without duplicating or requiring fragile dedup logic against posted, immutable rows), breaks the append-only/no-destructive-edit guarantee when an import needs correcting, and makes every future adapter (Xero, Wave) require touching ledger-write code — violating the "no core rework" requirement explicitly called out in PROJECT.md.
**Do this instead:** Always route through staging → reconciliation → human-approved promotion, using the same ledger-engine posting path that manual entry uses.

### Anti-Pattern 2: Treating the Remote-Access Tunnel as an Authorization Boundary

**What people do:** Assume "traffic came through the tunnel" implies "this is the trusted external accountant," and skip proper RBAC checks for tunnel-originated requests.
**Why it's wrong:** The tunnel is a network transport concern (getting bytes to the server without manual port-forwarding); it says nothing about who is actually authenticated. If the tunnel endpoint is even briefly misconfigured or the accountant's device is compromised, the app must still be the sole authority on what a given authenticated role can do.
**Do this instead:** Auth/session + RBAC layer is the only source of truth for permissions, applied identically regardless of whether the request arrived via LAN or via tunnel. Log/flag which requests came in via the remote path for audit purposes, but never grant privilege based on network origin alone.

### Anti-Pattern 3: Mutable "Current Balance" Columns as Source of Truth

**What people do:** Store a running `balance` column on accounts/funds and update it in place on every transaction (common in simpler bookkeeping tools) instead of deriving balances from the journal.
**Why it's wrong:** Breaks the tamper-evident/auditable requirement — a bug or bad migration can silently desync the "current balance" from the actual transaction history, and there's no way to prove correctness after the fact. This is exactly the failure mode fund-accounting audits are designed to catch.
**Do this instead:** Balances are always projections computed (and recomputable) from the append-only journal. Cache them for performance if needed, but treat the cache as disposable and rebuildable, never as authoritative.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| QuickBooks Desktop (IIF export / QBXML) | File-based import adapter, no live API connection (QBD has no modern cloud API) | IIF is a simple delimited text format; QBXML is richer but requires more parsing care around list vs. transaction elements; expect version-specific quirks |
| QuickBooks Online (QBO/CSV export) | File-based import adapter; QBO's actual live API (Intuit Developer) could be a v2 enhancement but v1 should target export-file import to avoid OAuth/cloud dependency, matching "no cloud dependency" ethos | CSV export formats vary by report type chosen in QBO — adapter needs to handle at least the standard transaction-list export |
| Tailscale Funnel / Cloudflare Tunnel (or self-hosted WireGuard relay) | Bundled binary/service, outbound-only connection initiated by the app, configured on first-run setup wizard | Both require an account/identity with the provider (Tailscale account or Cloudflare account) unless using a fully self-hosted relay (e.g., Pangolin-style) — decide during STACK research whether v1 accepts this external dependency or ships a self-hosted-only tunnel option, since "no cloud dependency" is a stated value |
| Future adapters (Xero, Wave) | Same `ImportAdapter` interface as QB adapters; likely API-based (both have REST APIs) rather than file-export-based | Deferred; the interface should already anticipate API-pull adapters, not just file-upload ones |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| PWA client ↔ API layer | HTTPS/JSON REST (or RPC), session-token authenticated | Client never talks to the DB directly; all writes validated server-side regardless of client-side validation |
| API layer ↔ Ledger/domain engine | In-process function calls (same server process) | Keep this boundary clean even though it's in-process — enables later extraction/testing in isolation and keeps accounting logic framework-agnostic |
| Import adapters ↔ Ledger engine | Never direct — always via staging + reconciliation + promotion (see Pattern 2) | This is the boundary most likely to be violated under time pressure; treat as a hard rule |
| Auth/session service ↔ everything else | Middleware layer in front of API routes; role checked per-route/per-action | External-accountant role is the trust-boundary-crossing case — scope it explicitly (e.g., no user management, no ability to delete/void without staff/admin co-sign) |
| Remote-access tunnel ↔ Local webserver | Local process bridges tunnel's local listener to the app's normal HTTP port | Tunnel component should be swappable/replaceable without touching API or auth code — it's purely a network front door |

## Sources

- [LedgerSMB](https://ledgersmb.org/) — self-hosted, open-source double-entry accounting/ERP for reference on web-based self-hosted accounting structure (MEDIUM confidence, project exists and is actively released as of May 2026)
- [GnuCash](https://en.wikipedia.org/wiki/GnuCash) — desktop double-entry bookkeeping reference for local-data-ownership model (MEDIUM confidence)
- [Ledger (plain-text accounting)](https://en.wikipedia.org/wiki/Ledger_(software)) — append-only plain-text ledger philosophy, informs journal/reversal pattern (MEDIUM confidence)
- [Architecting Immutable Ledger Design for Financial Systems](https://martinuke0.github.io/posts/2026-05-27-architecting-immutable-ledger-design-for-financial-systems-consistency-auditability-and-real-world-patterns/) — append-only, ACID, reversing-entry patterns (MEDIUM confidence, single-author blog but consistent with broader event-sourcing literature)
- [Event Sourcing and the History of Accounting](https://dev.to/dealeron/event-sourcing-and-the-history-of-accounting-1aah) — conceptual link between event sourcing and traditional double-entry bookkeeping (MEDIUM confidence)
- [Pangolin vs Cloudflare Tunnels vs Tailscale](https://contabo.com/blog/pangolin-vs-cloudflare-tunnels-vs-tailscale/) — comparison of bundleable remote-access tunnel approaches, including self-hosted alternative (MEDIUM confidence)
- [Tailscale vs Cloudflare Tunnel for home remote access](https://hometechops.com/guides/home-remote-access-tailscale-vs-cloudflare-tunnel) — practical breakdown of tunnel trade-offs for non-technical self-hosters, directly relevant to "org doesn't do manual networking" constraint (MEDIUM confidence)
- [Local-First Architecture for Progressive Web Apps](https://blog.openreplay.com/local-first-pwa-architecture/) — service worker + IndexedDB + background sync layering (MEDIUM confidence)
- [Offline-First PWA Patterns — Service Workers, IndexedDB, and Background Sync](https://rohitraj.tech/en/notes/pwa-offline-sync) — three-layer offline architecture, informs client-side offline scope decision (MEDIUM confidence)
- [The Architect's Guide to Data Integration Patterns](https://medium.com/@prayagvakharia/the-architects-guide-to-data-integration-patterns-migration-broadcast-bi-directional-a4c92b5f908d) — canonical data model / adapter pattern for multi-source ETL, informs import-adapter subsystem design (MEDIUM confidence)
- Training-data knowledge of QuickBooks Desktop IIF/QBXML export format structure and QuickBooks Online CSV export variability (LOW confidence — not independently verified against current Intuit documentation in this pass; flag for adapter-specific research before implementing importers)

---
*Architecture research for: Self-hosted nonprofit fund-accounting PWA (QuickBooks Desktop replacement)*
*Researched: 2026-08-08*
