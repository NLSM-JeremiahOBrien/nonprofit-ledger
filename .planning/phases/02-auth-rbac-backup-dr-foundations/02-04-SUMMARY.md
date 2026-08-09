---
phase: 02-auth-rbac-backup-dr-foundations
plan: 04
subsystem: backup
tags: [sqlite, vacuum-into, backup, restore, dr, lsof, go-list-deps, rbac]

# Dependency graph
requires:
  - phase: 02-auth-rbac-backup-dr-foundations (02-02)
    provides: RequireRole middleware, RegisterRoutes single-wiring-point, admin-only route group
  - phase: 02-auth-rbac-backup-dr-foundations (02-03)
    provides: audit_log append-only pattern this plan's backup_runs mutable-table pattern is contrasted against
provides:
  - Automated, scheduled local backups (VACUUM INTO) with rotation and health tracking (PLAT-03)
  - Proven restore path (integrity_check + trial-balance round-trip), not just documented
  - Admin-only GET /api/backup/status health endpoint
  - Automated PLAT-04 smoke tests: offline-isolation (lsof-based) and no-telemetry-dependency (go list -deps)
affects: [03-remote-access, all-future-phases-touching-cmd-server-main-or-api-routes]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "VACUUM INTO for WAL-consistent snapshots (never hand-copy .db/.db-wal files)"
    - "backup_runs is a mutable health-status table (like accounting_periods), explicitly not append-only like audit_log/journal_entries"
    - "RegisterRoutes takes backupDir as an explicit param rather than a global, keeping server/api stateless and test-isolated (mirrors server/auth's RequireAuth(sm, db, next) pattern from 02-02)"
    - "lsof-based socket isolation test skips gracefully (t.Skip) when lsof is unavailable, rather than failing"

key-files:
  created:
    - server/db/migrations/0008_backup_runs.up.sql
    - server/db/migrations/0008_backup_runs.down.sql
    - server/backup/backup.go
    - server/backup/restore.go
    - server/backup/backup_test.go
    - server/backup/testutil_test.go
    - server/api/backup.go
    - server/api/backup_test.go
    - server/api/offline_isolation_test.go
    - server/api/dependency_check_test.go
  modified:
    - server/api/routes.go
    - cmd/server/main.go
    - server/auth/external_accountant_scope_test.go

key-decisions:
  - "RegisterRoutes now takes an explicit backupDir string param (used by the new backup-status route) rather than a global/config struct, keeping the single-wiring-point pattern from 02-02 intact"
  - "Backup scheduler wiring (env vars + ticker goroutine) landed in the Task 2 commit rather than Task 3, since changing RegisterRoutes's signature required updating its only production caller (cmd/server/main.go) to compile at all; Task 3 then only added the two new smoke tests"
  - "TestOfflineIsolation disables HTTP keep-alives and closes idle connections before inspecting sockets, to avoid a lingering client-side idle connection producing a false ESTABLISHED-socket failure"

patterns-established:
  - "Snapshot/Rotate/RunScheduled cleanly separated: Snapshot does the VACUUM INTO, Rotate does file pruning by sorted filename, RunScheduled wraps both and always records a backup_runs row (success or failure) so backup health is never silently unobserved"
  - "Restore returns an opened *sql.DB after integrity_check rather than duplicating ledger-domain verification inside server/backup — callers (tests, future admin tooling) run ledger.TrialBalance themselves"

requirements-completed: [PLAT-03, PLAT-04]

# Metrics
duration: 35min
completed: 2026-08-09
---

# Phase 2 Plan 4: Backup, Restore Verification & Offline-Isolation Smoke Tests Summary

**Automated SQLite backups via VACUUM INTO with rotation and health tracking, a proven (not just documented) restore path verified against ledger.TrialBalance, an admin-only backup-status endpoint, and lsof/go-list-deps smoke tests that make PLAT-04's zero-outbound-network guarantee a permanently enforced automated test rather than a one-time manual check.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3
- **Files modified:** 13 (10 created, 3 modified)

## Accomplishments
- `backup_runs` mutable health table + `server/backup` package: `Snapshot` (VACUUM INTO, WAL-consistent), `Rotate` (retain-N pruning by sorted timestamp filename), `RunScheduled` (wraps both, always records success/failure)
- `Restore` copies a snapshot, runs `PRAGMA integrity_check`, and returns the opened handle for callers to independently verify (test proves restored `ledger.TrialBalance` matches the source database row-for-row and nets to zero)
- `GET /api/backup/status` (admin-only, wired into the existing `adminOnlyGroup`) reports last success time, last status, last error, and retained snapshot count
- Backup scheduler wired into `cmd/server/main.go`: runs once immediately at startup and then on a configurable interval (`LEDGER_BACKUP_DIR`/`LEDGER_BACKUP_INTERVAL`/`LEDGER_BACKUP_RETAIN` env vars), always writing to a local path
- `TestOfflineIsolation`: starts the full auth+RBAC+audit+backup stack on an ephemeral port, performs a real login and health check, then uses `lsof` to assert exactly one LISTEN socket and zero ESTABLISHED sockets on the running test process (skips gracefully if `lsof` is absent)
- `TestNoTelemetryDependencies`: asserts `go list -deps ./...` contains no analytics/sentry/s3/telemetry dependency anywhere in the module graph

## Task Commits

Each task was committed atomically:

1. **Task 1: Backup snapshot, rotation, and health tracking** - `27048e9` (feat)
2. **Task 2: Restore round-trip verification and admin-only status endpoint** - `f53385f` (feat)
3. **Task 3: Scheduler wiring + automated PLAT-04 isolation and dependency-graph checks** - `5c98c39` (test)

_Note: main.go's scheduler-wiring and env-var reading landed in the Task 2 commit — see Decisions._

## Files Created/Modified
- `server/db/migrations/0008_backup_runs.{up,down}.sql` - mutable `backup_runs` health table
- `server/backup/backup.go` - `Snapshot`, `Rotate`, `RunScheduled`
- `server/backup/restore.go` - `Restore` (copy + integrity_check + open)
- `server/backup/backup_test.go` - `TestSnapshot`, `TestRotation`, `TestRunScheduledRecordsHealth`, `TestRunScheduledRecordsFailure`, `TestRestoreRoundTrip`
- `server/backup/testutil_test.go` - shared `newTestDB` helper (seeds user id=1 for FK-constrained posting fixtures)
- `server/api/backup.go` - `BackupHandlers.BackupStatusHandler`
- `server/api/backup_test.go` - `TestBackupStatus` (admin 200, staff/external 403)
- `server/api/routes.go` - `RegisterRoutes` gains a `backupDir` param, wires `GET /api/backup/status` into `adminOnlyGroup`
- `server/api/offline_isolation_test.go` - `TestOfflineIsolation`
- `server/api/dependency_check_test.go` - `TestNoTelemetryDependencies`
- `cmd/server/main.go` - reads backup env vars, starts ticker-driven scheduler goroutine, passes `backupDir` to `RegisterRoutes`
- `server/auth/external_accountant_scope_test.go` - updated `RegisterRoutes` call site for the new signature

## Decisions Made
- `RegisterRoutes(mux, sm, db, backupDir)` keeps the single-wiring-point pattern from 02-02 rather than introducing a config struct — one more explicit param was simpler than a breaking abstraction change this late in the phase.
- Scheduler wiring in `cmd/server/main.go` was written alongside Task 2 (not deferred to Task 3) because changing `RegisterRoutes`'s signature required its only production caller to compile; Task 3's commit then contains only the two new smoke tests, matching the plan's stated task boundaries in spirit even though the file was touched one commit earlier than the plan's file-per-task list implies.
- `TestOfflineIsolation` explicitly disables HTTP keep-alives and calls `CloseIdleConnections()` before socket inspection to avoid a false-positive ESTABLISHED count from a lingering client-side pooled connection.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated `server/auth/external_accountant_scope_test.go` call site**
- **Found during:** Task 2
- **Issue:** Changing `RegisterRoutes`'s signature to add `backupDir` broke this pre-existing test file (from an earlier plan) at compile time.
- **Fix:** Updated its `api.RegisterRoutes(mux, sm, conn)` call to `api.RegisterRoutes(mux, sm, conn, t.TempDir())`.
- **Files modified:** server/auth/external_accountant_scope_test.go
- **Verification:** `go build ./...` and `go test ./...` both pass.
- **Committed in:** f53385f (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to keep the build green after the (plan-specified) `RegisterRoutes` signature change; no scope creep beyond fixing the compile break it caused.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. Backup behavior is configurable via `LEDGER_BACKUP_DIR` (default `./backups`), `LEDGER_BACKUP_INTERVAL` (default `24h`), and `LEDGER_BACKUP_RETAIN` (default `7`) environment variables, all optional.

## Next Phase Readiness
Phase 2 is now fully complete: login+session (02-01), RBAC/user management (02-02), audit log (02-03), and backup/DR + automated offline-isolation proof (02-04) are all implemented and test-covered. `go build ./...` succeeds and `go test ./...` is green across every package. Manual sanity check confirmed: a fresh `cmd/server` run bootstraps an admin, produces a `backups/` snapshot within seconds of startup, and the bootstrap admin can query `/api/backup/status` and see the successful run. Ready to proceed to Phase 3 (remote access) on top of a finalized RBAC and a verified-restorable backup story.

---
*Phase: 02-auth-rbac-backup-dr-foundations*
*Completed: 2026-08-09*

## Self-Check: PASSED

All created files and all three task commit hashes (27048e9, f53385f, 5c98c39) verified present.
