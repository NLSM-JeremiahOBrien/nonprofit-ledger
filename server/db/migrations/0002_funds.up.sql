CREATE TABLE funds (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    net_asset_class TEXT NOT NULL
        CHECK (net_asset_class IN ('without_donor_restrictions','with_donor_restrictions')),
    restriction_detail TEXT,
    is_active INTEGER NOT NULL DEFAULT 1
);
