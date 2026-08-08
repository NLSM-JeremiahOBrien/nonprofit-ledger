CREATE TABLE journal_entries (
    id INTEGER PRIMARY KEY,
    entry_date TEXT NOT NULL,
    memo TEXT,
    posted_by INTEGER NOT NULL,
    posted_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    reverses_entry_id INTEGER REFERENCES journal_entries(id)
);

CREATE TABLE journal_lines (
    id INTEGER PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES journal_entries(id),
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    fund_id INTEGER NOT NULL REFERENCES funds(id),
    functional_category TEXT
        CHECK (functional_category IN ('program','management_general','fundraising')),
    debit_amount INTEGER NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount INTEGER NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    CHECK (NOT (debit_amount > 0 AND credit_amount > 0))
);

-- Immutability: SQLite CREATE TRIGGER accepts exactly one DML event per
-- trigger, so UPDATE and DELETE each need their own trigger per table.
CREATE TRIGGER journal_lines_no_update
BEFORE UPDATE ON journal_lines
BEGIN
    SELECT RAISE(ABORT, 'journal_lines is append-only: posted lines cannot be updated');
END;

CREATE TRIGGER journal_lines_no_delete
BEFORE DELETE ON journal_lines
BEGIN
    SELECT RAISE(ABORT, 'journal_lines is append-only: posted lines cannot be deleted');
END;

CREATE TRIGGER journal_entries_no_update
BEFORE UPDATE ON journal_entries
BEGIN
    SELECT RAISE(ABORT, 'journal_entries is append-only: posted entries cannot be updated');
END;

CREATE TRIGGER journal_entries_no_delete
BEFORE DELETE ON journal_entries
BEGIN
    SELECT RAISE(ABORT, 'journal_entries is append-only: posted entries cannot be deleted');
END;
