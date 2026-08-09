CREATE TABLE backup_runs (
    id INTEGER PRIMARY KEY,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL,
    file_path TEXT,
    size_bytes INTEGER,
    error TEXT
);
