CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    actor_user_id INTEGER NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER,
    detail TEXT,
    occurred_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX audit_log_entity_idx ON audit_log(entity_type, entity_id);

-- Immutability: mirrors journal_entries/journal_lines in 0003_journal.
-- SQLite CREATE TRIGGER accepts exactly one DML event per trigger, so
-- UPDATE and DELETE each need their own trigger.
CREATE TRIGGER audit_log_no_update
BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;

CREATE TRIGGER audit_log_no_delete
BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;
