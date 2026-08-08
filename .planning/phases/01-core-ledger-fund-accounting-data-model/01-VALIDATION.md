---
phase: 1
slug: core-ledger-fund-accounting-data-model
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-08-08
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` package (no external test framework needed) |
| **Config file** | none yet — greenfield project; Wave 0 must scaffold `go.mod` and `server/ledger/*_test.go` structure |
| **Quick run command** | `go test ./server/ledger/... -run TestXxx -v` |
| **Full suite command** | `go test ./... -v` |
| **Estimated runtime** | ~5-15 seconds (small Go unit/integration suite against a temp SQLite file) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./server/ledger/... -v`
- **After every plan wave:** Run `go test ./... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 0 | (scaffolding) | n/a | `go build ./...` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | LEDG-01 | unit | `go test ./server/ledger/... -run TestChartOfAccounts -v` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | LEDG-02 | unit | `go test ./server/ledger/... -run TestPostJournalEntry_RejectsUnbalanced -v` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 1 | LEDG-03 | integration (real SQLite file) | `go test ./server/ledger/... -run TestImmutability -v` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 1 | LEDG-04 | unit | `go test ./server/ledger/... -run TestPeriodLock -v` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 1 | LEDG-05 | unit + integration | `go test ./server/ledger/... -run TestFundDimension -v` | ❌ W0 | ⬜ pending |
| 01-01-07 | 01 | 1 | FUND-01 | integration (schema-level) | `go test ./server/db/... -run TestFundsSchema -v` | ❌ W0 | ⬜ pending |
| 01-01-08 | 01 | 1 | FUND-02 | integration | `go test ./server/ledger/... -run TestFundBalanceReconciliation -v` | ❌ W0 | ⬜ pending |
| 01-01-09 | 01 | 1 | FUND-03 | unit | `go test ./server/ledger/... -run TestFunctionalExpenseRequired -v` | ❌ W0 | ⬜ pending |
| 01-01-10 | 01 | 1 | PLAT-01 | manual-only | n/a — see Manual-Only Verifications | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` — no Go module exists yet in this repo
- [ ] `server/db/sqlite.go` — connection setup with WAL + `foreign_keys` pragmas, needed by every test
- [ ] `server/db/migrations/0001..000N_*.sql` — schema migrations (this phase's primary deliverable; test files depend on these existing)
- [ ] `server/ledger/testutil_test.go` — shared fixture: spin up a temp SQLite file, run migrations, return a `*sql.DB` per test
- [ ] `golang-migrate` CLI (dev dependency) — `go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Server starts and serves with no outbound network call in its startup path | PLAT-01 | Network-isolation smoke test isn't worth automating in Phase 1 (no remote-access feature exists yet — that's Phase 3) | Disconnect network / enable airplane mode, run `go run ./cmd/server`, confirm it starts and serves a local request successfully |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-08 (YOLO mode — auto-approved per config.json)
