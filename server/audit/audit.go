// Package audit provides a single append-only write helper for the
// audit_log table. Every consequential ledger action (posting, reversal,
// period lock, and future RBAC-covered mutations) records one row here,
// always inside the caller's own transaction so the audit record and the
// action it describes commit or roll back together.
package audit

import (
	"database/sql"
	"fmt"
)

// Write inserts one row into audit_log using the caller-supplied
// transaction. It never opens its own transaction or commits/rolls back
// tx — atomicity with the operation being recorded is the entire point,
// so the caller must insert this call before its own tx.Commit().
//
// actorUserID must be a real, server-side-resolved user ID (e.g. from an
// authenticated session or an explicit function parameter already
// threaded through by the caller) — never from unvalidated client input.
//
// action is a short verb describing what happened (e.g. "create",
// "reversal", "lock_period"); entityType/entityID identify the affected
// record. detail is free-form context (e.g. a memo or reason).
func Write(tx *sql.Tx, actorUserID int64, action, entityType string, entityID int64, detail string) error {
	if action == "" {
		return fmt.Errorf("audit: action must not be empty")
	}
	if entityType == "" {
		return fmt.Errorf("audit: entityType must not be empty")
	}

	_, err := tx.Exec(
		`INSERT INTO audit_log (actor_user_id, action, entity_type, entity_id, detail)
		 VALUES (?, ?, ?, ?, ?)`,
		actorUserID, action, entityType, entityID, detail,
	)
	if err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}

	return nil
}
