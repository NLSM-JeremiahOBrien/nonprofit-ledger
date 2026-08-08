CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('asset','liability','equity','revenue','expense')),
    parent_id INTEGER REFERENCES accounts(id),
    is_active INTEGER NOT NULL DEFAULT 1
);
