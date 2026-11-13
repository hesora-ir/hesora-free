CREATE TABLE payments (
    id           INTEGER PRIMARY KEY,
    external_ref TEXT NOT NULL DEFAULT '',
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    currency     TEXT NOT NULL CHECK (currency IN ('IRR', 'IRT')),
    paid_at      TEXT NOT NULL,
    method       TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);

CREATE UNIQUE INDEX ux_payments_external_ref
    ON payments(external_ref)
    WHERE external_ref <> '';

CREATE TABLE payment_allocations (
    id         INTEGER PRIMARY KEY,
    payment_id INTEGER NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    invoice_id INTEGER NOT NULL REFERENCES invoices(id) ON DELETE RESTRICT,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    created_at TEXT NOT NULL,
    UNIQUE(payment_id, invoice_id)
);

CREATE INDEX ix_invoices_customer_issue_date ON invoices(customer_id, issue_date DESC);
CREATE INDEX ix_invoices_status_issue_date ON invoices(status, issue_date DESC);
CREATE INDEX ix_invoice_items_invoice_position ON invoice_items(invoice_id, position);
CREATE INDEX ix_payment_allocations_invoice ON payment_allocations(invoice_id);
CREATE INDEX ix_payments_paid_at ON payments(paid_at DESC);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY CHECK (length(trim(key)) > 0),
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE audit_events (
    id          INTEGER PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    action      TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    details     TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX ix_audit_events_entity ON audit_events(entity_type, entity_id, occurred_at DESC);
