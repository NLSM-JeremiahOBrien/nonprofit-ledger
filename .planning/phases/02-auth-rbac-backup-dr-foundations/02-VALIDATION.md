---
phase: 2
slug: auth-rbac-backup-dr-foundations
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-08-09
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` (Go 1.26.5), table-driven — matches Phase 1 (`*_test.go` throughout `server/ledger`) |
| **Config file** | none — `go test` convention; shared helpers in `testutil_test.go` (Phase 1 pattern to extend) |
| **Quick run command** | `go test ./server/auth/... ./server/users/... ./server/audit/... ./server/backup/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./server/<touched-package>/... -x` (quick, < 30s)
- **After every plan wave:** Run `go test ./...` (full suite)
- **Before `/gsd:verify-work`:** Full suite green + restore round-trip test green + offline-isolation smoke green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | AUTH-01 | unit | `go test ./server/auth/... -run TestPassword -x` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | AUTH-01 | unit + handler | `go test ./server/auth/... -run TestLogin -x` | ❌ W0 | ⬜ pending |
| 02-01-03 | 01 | 1 | AUTH-04 | integration | `go test ./server/auth/... -run TestSessionPersistence -x` | ❌ W0 | ⬜ pending |
| 02-01-04 | 01 | 1 | AUTH-04 | integration | `go test ./server/auth/... -run TestRevokeSession -x` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | AUTH-02 | unit | `go test ./server/users/... -run TestCreateUser -x` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | AUTH-02 | handler | `go test ./server/auth/... -run TestUserMgmtRequiresAdmin -x` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 2 | AUTH-03 | handler | `go test ./server/auth/... -run TestExternalAccountantScope -x` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 2 | AUTH-05 | unit + integration | `go test ./server/audit/... -run TestAuditAttribution -x` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 2 | AUTH-05 | unit | `go test ./server/audit/... -run TestAuditImmutable -x` | ❌ W0 | ⬜ pending |
| 02-04-01 | 04 | 3 | PLAT-03 | unit | `go test ./server/backup/... -run TestSnapshot -x` | ❌ W0 | ⬜ pending |
| 02-04-02 | 04 | 3 | PLAT-03 | integration | `go test ./server/backup/... -run TestRestoreRoundTrip -x` | ❌ W0 | ⬜ pending |
| 02-04-03 | 04 | 3 | PLAT-03 | unit | `go test ./server/backup/... -run TestRotation -x` | ❌ W0 | ⬜ pending |
| 02-04-04 | 04 | 3 | PLAT-03 | handler | `go test ./server/api/... -run TestBackupStatus -x` | ❌ W0 | ⬜ pending |
| 02-04-05 | 04 | 3 | PLAT-04 | smoke | `go test ./... -run TestOfflineIsolation -x` (extend Phase 1) | ⚠️ extend existing | ⬜ pending |
| 02-04-06 | 04 | 3 | PLAT-04 | manual/CI | `go list -deps ./... \| grep -Ei 'analytics\|sentry\|s3\|telemetry'` returns nothing | ❌ W0 (CI check) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `server/auth/password_test.go` — argon2id hash/verify (AUTH-01)
- [ ] `server/auth/session_test.go`, `middleware_test.go` — persistence, revocation, role guards (AUTH-01/03/04)
- [ ] `server/users/users_test.go` — user CRUD + role validation (AUTH-02)
- [ ] `server/audit/audit_test.go` — attribution + immutability trigger (AUTH-05)
- [ ] `server/backup/backup_test.go` — snapshot, restore round-trip, rotation, status (PLAT-03)
- [ ] Extend existing Phase 1 offline-isolation smoke test to cover auth+backup paths (PLAT-04)
- [ ] CI/manual dependency-graph check for telemetry/cloud packages (PLAT-04)
- [ ] Verification spike: `scs/sqlite3store` against `modernc.org/sqlite` (open question from research) — resolve before committing to the session store
- [ ] Shared test helper: extend `testutil_test.go` pattern with a seeded-user + logged-in-request fixture
- [ ] Framework install: none needed — Go stdlib `testing` already in use

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification (PLAT-04's dependency-graph check runs as a CI/manual grep, but is scripted, not judgment-based).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
