package ledger

import "testing"

func TestPeriodLock(t *testing.T) {
	db := newTestDB(t)

	t.Run("no periods row returns nil", func(t *testing.T) {
		got, err := GetLockedThroughDate(db)
		if err != nil {
			t.Fatalf("GetLockedThroughDate: unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("GetLockedThroughDate: expected nil, got %q", *got)
		}
	})

	t.Run("LockPeriod then GetLockedThroughDate round-trips", func(t *testing.T) {
		if err := LockPeriod(db, "2026-06-30", 1); err != nil {
			t.Fatalf("LockPeriod: unexpected error: %v", err)
		}

		got, err := GetLockedThroughDate(db)
		if err != nil {
			t.Fatalf("GetLockedThroughDate: unexpected error: %v", err)
		}
		if got == nil || *got != "2026-06-30" {
			t.Fatalf("GetLockedThroughDate: expected 2026-06-30, got %v", got)
		}
	})

	t.Run("re-locking to a later date updates the boundary", func(t *testing.T) {
		if err := LockPeriod(db, "2026-06-30", 1); err != nil {
			t.Fatalf("LockPeriod: unexpected error: %v", err)
		}
		if err := LockPeriod(db, "2026-07-31", 1); err != nil {
			t.Fatalf("LockPeriod (extend): unexpected error: %v", err)
		}

		got, err := GetLockedThroughDate(db)
		if err != nil {
			t.Fatalf("GetLockedThroughDate: unexpected error: %v", err)
		}
		if got == nil || *got != "2026-07-31" {
			t.Fatalf("GetLockedThroughDate: expected 2026-07-31, got %v", got)
		}
	})
}
