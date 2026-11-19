CREATE TABLE cloud_payment_reversals (
    cloud_id       TEXT PRIMARY KEY,
    local_payment_id INTEGER NOT NULL UNIQUE REFERENCES payments(id) ON DELETE RESTRICT,
    reversed_at    INTEGER NOT NULL,
    reason         TEXT NOT NULL
);

CREATE INDEX ix_cloud_payment_reversals_payment ON cloud_payment_reversals(local_payment_id);
