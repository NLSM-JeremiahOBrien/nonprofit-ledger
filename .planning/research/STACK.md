# Stack Research

**Domain:** Self-hosted, offline-capable PWA fund-accounting system (single-tenant, local-first server + bundled secure remote access)
**Researched:** 2026-08-08
**Confidence:** MEDIUM-HIGH (core architecture HIGH; specific library picks MEDIUM — verify versions at implementation time since several packages are mid-major-version churn in 2026)

## Architectural Framing (read first)

This is **not** a browser-CRDT "local-first" app in the Kleppmann/Linear sense (no multi-device sync of independent local databases). It is a **single-tenant self-hosted server** (one instance per nonprofit, on their NAS/desktop) with a **PWA client that talks to that local server**. The server's database is the single source of truth. "Offline-capable" here mostly means: (1) the PWA app shell and static assets are precached by a service worker so the UI still loads if the LAN/tunnel blips, and (2) the client can queue a small number of writes (e.g., a bank rec checkbox) and flush them on reconnect — not a full offline ledger with conflict resolution. This distinction matters: it means you do **not** need PGlite/CRDT/sync-engine machinery (ElectricSQL, Turso Sync, Yjs). You need a boring, correct, ACID server + a well-cached PWA frontend. Treat any research/roadmap phase that starts reaching for CRDT sync tooling as a red flag — that's solving a different problem than the one in PROJECT.md.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26.x (current stable, released Feb 2026; patched through 1.26.5 as of Jul 2026) | Backend server language | Single statically-linked binary output (critical for non-technical self-hosters — no runtime to install), trivial cross-compilation to Linux/ARM (Synology/QNAP NAS), macOS, Windows from one dev machine, strong stdlib HTTP server, and it's the same language Tailscale ships `tsnet` in (see remote access below), so the tunnel is a native import, not a subprocess. Go's simplicity also matters given the project's timeline pressure (QuickBooks Desktop EOL) — faster to build and audit correctly than Rust, safer under concurrent writes than a dynamically-typed stack. **Confidence: HIGH** |
| SQLite (via `modernc.org/sqlite`, latest) | SQLite 3.46+ engine, `modernc.org/sqlite` pure-Go driver | Embedded database — the ledger's system of record | See "Database Choice" section below for full rationale. Short version: single-tenant + single-writer-process fits SQLite's design point exactly, WAL mode gives durability and read concurrency, and `modernc.org/sqlite` is CGo-free so it doesn't break Go's cross-compilation story (unlike `mattn/go-sqlite3`, which requires a C toolchain per target platform). **Confidence: HIGH** |
| React 19 + Vite 6/7 | React 19.x, Vite 6.x or 7.x | Frontend SPA framework + build tool | Accounting UI is grid/table/report-heavy (ledgers, trial balances, functional expense allocation). React has the deepest ecosystem for this (TanStack Table/Query, react-aria for accessibility, mature form libraries) and the largest pool of contributors an open-source nonprofit tool can draw volunteer devs from. Vite gives fast local dev and a clean static-asset build output that Go can `embed` directly into the binary. **Confidence: MEDIUM** (framework choice is a judgment call — see Alternatives) |
| `vite-plugin-pwa` | 1.x (current major as of 2026) | Service worker generation, manifest, offline app-shell caching, installability | Purpose-built for exactly this: Workbox-based precaching, auto-update prompts, zero-config manifest generation. Standard choice for Vite + PWA in 2025/2026. **Confidence: MEDIUM** |
| `tsnet` (Tailscale, `tailscale.com/tsnet`) | latest (track alongside Tailscale client releases, currently 1.8x series) | Embedded, userspace secure remote access | See "Remote Access" section below. This is the single most load-bearing and specific recommendation in this document. **Confidence: HIGH** |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang-migrate/migrate` | v4.x | SQLite schema migrations | Always — never hand-roll schema versioning for a financial data store; you need reliable, reviewable, reversible migrations from day one, especially since the ledger schema will evolve during the O'Brien pilot. |
| `litestream` (benbjohnson/litestream) | latest (0.3.x+) | Continuous SQLite backup / point-in-time recovery | Bundle as a sidecar process or embed via its Go library. Streams WAL changes to a local second disk, NAS share, or S3-compatible target (e.g., self-hosted MinIO or Backblaze B2) in near-real-time. This is the standard answer to "how do you get Postgres-grade durability guarantees out of a single SQLite file" — without it, a disk failure loses everything since the last manual backup. Non-negotiable for a financial system. |
| `go-chi/chi` | v5.x | HTTP router/middleware | Lightweight, stdlib-compatible router; avoids pulling in a heavier framework (Gin/Echo) you don't need for a single-tenant app. Optional — Go 1.22+ stdlib `net/http` mux is now capable enough for straightforward REST routes if you want to minimize dependencies further. |
| `golang.org/x/crypto/argon2` | current | Password hashing | Argon2id for local staff/admin credentials. Do not use bcrypt-only or (worse) SHA-based hashing for a financial app's auth. |
| `go-webauthn/webauthn` | v0.x (actively maintained) | Passkey/WebAuthn support | Recommended for the external accountant role specifically — accountants log in infrequently from their own devices; a passkey avoids password-reuse risk on a system holding a nonprofit's full financial history. Optional for v1, but flag as a near-term hardening item. |
| `@tanstack/react-query` | v5.x (5.101+) | Server state management/caching in the PWA | Standard pairing with a REST/JSON Go backend; handles caching, retries, and background refetch cleanly, which also gives you most of the "reconnect and resync" behavior you want for the offline-blip case without building a sync engine. |
| `@tanstack/react-table` | v9 (stable as of Aug 2026) or pin to v8 if v9 churn is a concern early on | Ledger/report data grids | Purpose-built for exactly this UI shape (sortable, filterable, groupable financial tables). v9 just went stable (Aug 4, 2026) — if the project starts before the ecosystem (docs, examples, plugins) catches up, pinning to the well-documented v8 line is a reasonable, lower-risk choice. |
| `react-hook-form` + `zod` | latest | Forms + validation | Standard pairing for data-entry-heavy apps (journal entries, chart-of-accounts edits); zod schemas can be shared/mirrored against Go-side validation logic conceptually (not literally shared code, but same validation rules kept in sync). |
| `encoding/xml` (Go stdlib) | stdlib | QBXML parsing | QBXML is just XML; no extra dependency needed for parsing, only for domain mapping logic. |
| `aclindsa/ofxgo` | latest | QBO (OFX-format) import | QBO exports are OFX-formatted. This is the most maintained Go OFX parser; avoid hand-rolling an OFX/SGML parser — OFX has enough historical format quirks (SGML vs XML variants) that a maintained library saves real time. |
| IIF parser | custom (tab-delimited format, no good existing Go library found) | QuickBooks Desktop IIF import | IIF is a simple tab-delimited text format with typed record blocks (`!TRNS`, `!SPL`, etc.) — no widely-used Go library exists; plan to write a small, well-tested parser as part of the import-adapter module rather than searching further for one. Flag this for phase-specific research/spike time. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| Go `embed` package (stdlib) | Bundle built frontend (Vite `dist/`) into the Go binary | This is what makes "single binary, no separate web server needed" possible — the Go binary serves its own embedded static assets and API from one process. |
| `air` (cosmtrek/air or air-verse/air fork) | Live reload for Go dev | Standard Go dev-loop tool; not for production. |
| Docker + a provided `Dockerfile`/`docker-compose.yml` | Optional secondary distribution path | Many self-hosters (Synology Container Manager, Unraid, TrueNAS Apps, Casaos) expect Docker even when a native binary exists. Ship both — native binary as the "just run it" path for the least technical orgs, Docker image as the path for orgs whose IT volunteer already runs a Docker-based NAS stack. |
| GitHub Actions + `goreleaser` | Cross-compiled release builds | `goreleaser` is the standard tool for producing signed, cross-platform Go release binaries (Linux amd64/arm64, macOS, Windows) plus Docker images from one CI config — appropriate for an open-source project that needs non-technical users to download a working binary for their exact NAS architecture. |

## Installation

```bash
# Backend (Go module)
go get modernc.org/sqlite
go get github.com/golang-migrate/migrate/v4
go get tailscale.com/tsnet
go get github.com/go-chi/chi/v5
go get golang.org/x/crypto
go get github.com/aclindsa/ofxgo
go get github.com/benbjohnson/litestream

# Frontend
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install @tanstack/react-query @tanstack/react-table react-hook-form zod
npm install -D vite-plugin-pwa

# Dev tooling
go install github.com/air-verse/air@latest
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Database Choice — Rationale Specific to Double-Entry Fund Accounting

This is the highest-stakes decision in the stack, so it gets its own section rather than just a table row.

**Recommendation: SQLite (file-based, WAL mode), not Postgres, not a specialized ledger database like TigerBeetle.**

Reasoning:
- **Deployment shape drives this decision.** The app is explicitly single-tenant, one instance per org, on the org's own NAS/desktop. Postgres is a fantastic database, but it's a *separate server process* that needs its own install, port, credentials, upgrade path, and backup tooling — exactly the kind of "org IT has to configure networking/services themselves" complexity the project is trying to avoid (per PROJECT.md's remote-access constraint, which applies just as much to the database). SQLite is a library, not a service: it's linked into the Go binary and lives as one file on disk. This is the same reasoning that leads most single-tenant self-hosted tools in this class (e.g., Miniflux optionally, many Beszel/uptime-style tools, Litestream's own target use case) to SQLite over Postgres.
- **Concurrency profile fits.** A single nonprofit's bookkeeping activity is low-concurrency (a handful of staff/bookkeeper/accountant users, not concurrent high-throughput writes). SQLite's single-writer/multiple-reader model under WAL mode is not a bottleneck at this scale — Postgres's MVCC concurrency advantages solve a problem this app doesn't have.
- **ACID guarantees are equivalent for this workload.** SQLite is fully ACID-compliant and transactional; WAL mode adds crash-safe durability with the write-ahead log flushed before commit acknowledgement. For a double-entry system, the actual integrity guarantee you need — "a transaction either posts both its debit and credit legs or neither" — is enforced by wrapping each journal-entry post in a single SQL transaction, which SQLite supports identically to Postgres. General industry commentary (e.g., pgledger, a reference double-entry implementation) defaults to Postgres because it's built for *multi-tenant, server-hosted* ledger systems — a different deployment shape than this project's.
- **Do not reach for TigerBeetle or another specialized financial OLTP database.** TigerBeetle is a purpose-built distributed ledger database designed for extremely high-throughput financial transaction processing (banks, payment processors) — it's a distributed system with its own consensus/replication model. That's the wrong tool for a single NAS-hosted nonprofit doing dozens to hundreds of transactions a day; it adds massive operational complexity (a whole additional service/cluster to run and understand) for guarantees this project doesn't need. Mentioned here explicitly so it doesn't get proposed later as "the correct choice for financial data" without this context.
- **Enforce ledger integrity at the schema level, not just in application code.** Recommended pattern (flag for the architecture/roadmap phase that designs the ledger schema): journal entries are posted as an atomic multi-row insert (one row per debit/credit leg) inside a single transaction; once posted, rows are immutable — corrections happen via reversing/adjusting entries, never `UPDATE`/`DELETE` on posted rows. Enforce the immutability with SQLite triggers that raise on `UPDATE`/`DELETE` against posted transaction tables, and enforce debit=credit balance with a trigger or application-transaction check before commit. This "insert-only ledger, corrections via reversal" pattern is standard double-entry-system practice (matches how Ledger-cli/Beancount and most real GL systems behave) and directly satisfies the audit/append-only requirement from the milestone context.
- **Backups are the one place SQLite needs a deliberate answer**, because "it's just a file" cuts both ways — trivial to copy, but also trivial to lose if nobody's doing it. `litestream` (see Supporting Libraries) solves this by continuously streaming WAL segments to a second location, giving near-Postgres-grade durability (point-in-time recovery) without running a second database server. This should be treated as a **required**, not optional, component of the stack — ship it bundled/pre-configured, not as a "docs tell you to set it up yourself" afterthought, given non-technical operators.
- **Driver choice matters for distribution:** use `modernc.org/sqlite` (pure Go, no CGo) rather than `mattn/go-sqlite3` (CGo-based). The project's distribution model depends on producing single cross-compiled binaries for ARM-based NAS devices, macOS, Windows, and Linux from CI — CGo dependencies make that materially harder (need a C cross-compiler toolchain per target) for a marginal performance gain this app's workload doesn't need.

## Remote Access — Bundling a Secure Tunnel

**Recommendation: embed Tailscale via `tsnet` (`tailscale.com/tsnet`), not Cloudflare Tunnel, not raw WireGuard.**

- `tsnet` is a Go library that runs a full Tailscale node **inside the application process**, using a userspace network stack (no TUN device, no root/admin privileges required, no separate `tailscaled` daemon to install). The Go binary itself joins the org's private "tailnet," gets its own stable identity/IP, and can request its own HTTPS certificate automatically. This is exactly the shape needed: the nonprofit's IT-non-technical staff installs and runs one binary, authenticates it to their (free-tier) Tailscale account once via a login link, and from then on the external accountant is granted access by being invited as a user on that tailnet — no port forwarding, no firewall rules, no VPN client configuration by the org.
- Access control lives at the network layer (Tailscale ACLs — who's allowed on the tailnet and what they can reach) *and* the app layer (your own RBAC), giving defense in depth for a financial system, without the org ever exposing the app to the public internet. This is a meaningfully safer default than a publicly reachable HTTPS endpoint.
- **Why not Cloudflare Tunnel (`cloudflared`):** Cloudflare Tunnel is a solid product, but as of 2025 there is no first-class Go SDK for embedding a tunnel *inside* your own process the way `tsnet` does — this has been an open feature request against `cloudflared` since 2023 (`cloudflare/cloudflared` issue #986, requesting an `ngrok-go`-style embeddable library) and isn't resolved. Using Cloudflare Tunnel today means either shelling out to a separately-installed `cloudflared` binary as a sidecar process, or running it as its own service — reintroducing the "org has to install and manage a second thing" problem this feature is trying to eliminate. It also requires the org to hold a Cloudflare account and (for custom domains) a registered domain, which is friction Tailscale's free tier avoids. Worth revisiting if Cloudflare ships a proper embeddable Go tunnel library later.
- **Why not raw WireGuard:** WireGuard is the protocol Tailscale itself is built on, but using it directly means the app (or the org) has to handle key exchange, peer configuration, and NAT traversal manually — there's no "invite your accountant" UX at that layer. Tailscale is WireGuard plus exactly the coordination/identity/NAT-traversal layer this use case needs; using raw WireGuard would mean re-building that layer from scratch for no benefit.
- **Practical detail to design for:** treat `tsnet` as optional/pluggable, not mandatory — some orgs may prefer LAN-only access (no remote accountant need, or they already have a VPN) and shouldn't be forced to create a Tailscale account. Ship it as a feature that's off by default and one config step to enable, with plain local HTTP/HTTPS on the LAN as the baseline that always works.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| Go + single binary | Node.js (Express/Fastify) + `pkg`/single-file bundling | If the team's existing expertise is much stronger in TypeScript/Node than Go and shipping velocity matters more than the cleanest cross-platform single-binary story. Node's single-binary tooling (`pkg`, Node's own SEA) is less mature/reliable than Go's native cross-compilation, and you lose the clean `tsnet` integration. |
| Go + single binary | Rust (Axum) + single binary | If maximal memory-safety and long-term correctness guarantees outweigh development speed. Rust would be defensible for a financial system on safety grounds, but given the project's stated timeline pressure (QuickBooks Desktop EOL) and that this needs to ship a working pilot against real O'Brien data soon, Go's faster iteration speed is the better tradeoff. Tailscale itself is written largely in Go, so `tsnet` integration is smoothest there. |
| SQLite (embedded) | PostgreSQL (self-hosted alongside app, e.g. via embedded-postgres or a bundled Postgres binary) | If the roadmap later adds true multi-instance/multi-tenant hosting (explicitly out of scope for v1 per PROJECT.md) or if concurrent write volume ever grows far beyond a single small nonprofit's bookkeeping activity. Not recommended for v1. |
| React + Vite | SvelteKit | If bundle size and simplicity are prioritized over ecosystem breadth. Svelte produces smaller PWAs and has a pleasant DX, but the accounting-grid/report tooling ecosystem (TanStack Table/Query, mature accessible data-grid components) is meaningfully deeper on React, which matters more for this domain's UI complexity than shipping a few hundred extra KB of JS. |
| React + Vite | Server-rendered Go templates + htmx | If the team wants to minimize frontend build complexity and the UI stays closer to simple CRUD forms/tables. Reasonable alternative, but weaker fit for "installable, offline-capable PWA" (service worker precaching and offline queueing are much more natural in an SPA architecture) and for complex financial reports/grids. |
| `tsnet` (Tailscale) | Cloudflare Tunnel (`cloudflared` as sidecar) | If the org already standardizes on Cloudflare (e.g., already uses Cloudflare for DNS/other services) and is comfortable running `cloudflared` as a separate managed process, or if truly public (non-tailnet) access is a hard requirement. |
| `tsnet` (Tailscale) | Headscale (self-hosted Tailscale coordination server) | If a nonprofit or umbrella organization wants to avoid dependency on Tailscale's (SaaS) coordination server entirely and self-host the coordination layer too. Adds real operational complexity for a single small org; more relevant if this project is later deployed at scale across many nonprofits by a coalition that wants to run shared infrastructure. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Electron | The requirement is a PWA served from a local webserver, installable via the browser — not a desktop app framework. Electron would mean bundling an entire Chromium + Node runtime per platform, massively bloating distribution size and update complexity, for a capability (installable app-like UI) the PWA spec already provides for free. | `vite-plugin-pwa` + browser-native "Install App" |
| A CRDT/local-first sync engine (Yjs, Automerge, ElectricSQL, PGlite+sync, Turso Sync/embedded replicas) | Solves multi-device/offline-first data divergence and merge — a problem this architecture doesn't have, since there's one server-side source of truth per org and the client is a thin PWA against it. Adopting one would add significant conceptual and implementation complexity (conflict resolution, merge semantics) with no corresponding requirement in PROJECT.md, and is actively risky for a financial ledger where "automatically merge two divergent edits" is close to the last thing you want. | Server-authoritative SQLite + React Query for client caching/refetch |
| `mattn/go-sqlite3` (CGo driver) as the default | Requires a C toolchain to cross-compile for each target OS/arch, complicating the "produce single binaries for NAS/macOS/Windows from CI" distribution goal. Fine for local dev if needed, but shouldn't be the default release driver. | `modernc.org/sqlite` (pure Go) |
| JWTs for the primary web session auth | This is a traditional server-rendered-session-adjacent web app (one server, one org, cookie-based browser sessions) — JWTs solve stateless auth across multiple services/APIs, a problem this single-server app doesn't have, and they add revocation complexity (a JWT can't be un-issued before expiry) that's a real liability for an app you may need to instantly cut off an ex-bookkeeper's access to. | Server-side sessions with secure, httpOnly, SameSite cookies |
| Raw WireGuard config for remote access | No account/identity layer, no "invite this specific accountant" UX, requires manual key exchange — reintroduces exactly the "org has to do networking themselves" problem the feature is meant to eliminate. | `tsnet`-embedded Tailscale |
| TigerBeetle or other distributed financial-OLTP databases | Massive operational overkill for a single-NAS, low-transaction-volume nonprofit; it's a clustered system designed for banks/payment processors. | SQLite with an insert-only, trigger-enforced ledger schema |
| Postgres run as a second self-managed service on the org's box | Reintroduces exactly the "install and maintain a database server" burden the single-binary distribution goal is trying to eliminate for non-technical self-hosters. | SQLite (embedded) + `litestream` for backup |

## Stack Patterns by Variant

**If the roadmap later adds a hosted/managed-service tier for orgs that don't want to self-host at all (explicitly out of scope for v1 per PROJECT.md, but worth flagging now):**
- Swap SQLite for Postgres and re-architect for true multi-tenancy at that point — don't try to make SQLite do multi-tenant duty.
- Because the two deployment models (single-tenant local file DB vs. multi-tenant hosted service) have genuinely different database requirements; trying to unify them prematurely will compromise the v1 simplicity that makes this approach viable for non-technical self-hosters.

**If accountant/bookkeeper users need to work from a location where installing/joining a Tailscale tailnet is genuinely not feasible (rare, but e.g. a locked-down corporate laptop):**
- Fall back to the LAN-only baseline plus a manually configured reverse proxy (nginx/Caddy) with the org's own TLS cert, documented as an advanced/manual path.
- Because the bundled-tunnel feature is explicitly optional-by-design (see Remote Access section) — there should always be a "just works on the LAN" fallback that doesn't depend on any third-party network service.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `vite-plugin-pwa` 1.x | Vite 5.x+ | Confirmed compatible from v0.17 onward per plugin docs; verify against whatever Vite major is current at implementation time. |
| `@tanstack/react-table` v9 | React 18/19 | v9 went stable Aug 4, 2026 — very recent at time of writing. If implementation starts soon after this research, consider pinning to the well-established v8 line to avoid working against thin early-v9 docs/examples, then upgrading once the ecosystem catches up. |
| `modernc.org/sqlite` | Go 1.21+ | No CGo requirement; verify current release notes for the SQLite engine version it embeds (it tracks upstream SQLite releases with some lag — check this isn't more than 1-2 minor SQLite releases behind at implementation time, since SQLite periodically ships performance/correctness fixes). |
| `tailscale.com/tsnet` | Go 1.22+ | Track against whatever Tailscale client version the org's tailnet is running; Tailscale ships frequent releases, pin a specific version in `go.mod` and update deliberately rather than always-latest, given this is a security-sensitive dependency. |

## Sources

- [tsnet · Tailscale Docs](https://tailscale.com/docs/features/tsnet) — HIGH confidence, official docs
- [Create Virtual Private Services with tsnet on Tailscale](https://tailscale.com/blog/tsnet-virtual-private-services) — HIGH confidence, official blog
- [cloudflared Issue #986 — Go SDK library request](https://github.com/cloudflare/cloudflared/issues/986) — MEDIUM confidence, confirms no embeddable Go SDK exists for Cloudflare Tunnel as of the open issue
- [SQLite Write-Ahead Logging docs](https://www.sqlite.org/wal.html) — HIGH confidence, official SQLite docs
- [modernc.org/sqlite package docs](https://pkg.go.dev/modernc.org/sqlite) — HIGH confidence, official package docs
- [Ledger Implementation in PostgreSQL — pgledger (Paul Gross)](https://www.pgrs.net/2025/03/24/pgledger-ledger-implementation-in-postgresql/) — MEDIUM confidence, informs why Postgres-first ledger examples exist (multi-tenant/server-hosted assumption) and why that doesn't transfer directly to this project's single-tenant deployment shape
- [Double-Entry Ledgers: The Missing Primitive in Modern Software (Paul Gross)](https://www.pgrs.net/2025/06/17/double-entry-ledgers-missing-primitive-in-modern-software/) — MEDIUM confidence, general double-entry-ledger design patterns (insert-only, immutability)
- [go.dev — Go 1.26 release notes / release history](https://go.dev/doc/go1.26) — HIGH confidence, official, confirms current Go version as of research date
- [vite-plugin-pwa GitHub / npm](https://github.com/vite-pwa/vite-plugin-pwa) — MEDIUM confidence, official repo
- [TanStack Table v9 announcement](https://tanstack.com/blog/announcing-tanstack-table-v9) — MEDIUM confidence, official blog, flags recency risk noted above
- [The Architecture Of Local-First Web Development — Smashing Magazine (2026)](https://www.smashingmagazine.com/2026/05/architecture-local-first-web-development/) — MEDIUM confidence, used to explicitly distinguish this project's architecture from browser-CRDT local-first patterns
- General training-data knowledge of Go, SQLite, litestream, OFX/QBXML/IIF formats — LOW-MEDIUM confidence where not cross-checked above; flagged inline where a claim rests primarily on this.

---
*Stack research for: self-hosted, offline-capable nonprofit fund-accounting PWA*
*Researched: 2026-08-08*
