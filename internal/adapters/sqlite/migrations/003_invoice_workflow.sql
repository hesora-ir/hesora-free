ALTER TABLE invoices ADD COLUMN item_discount_minor INTEGER NOT NULL DEFAULT 0 CHECK (item_discount_minor >= 0);
ALTER TABLE invoices ADD COLUMN invoice_discount_mode TEXT NOT NULL DEFAULT 'fixed' CHECK (invoice_discount_mode IN ('fixed', 'percent'));
ALTER TABLE invoices ADD COLUMN invoice_discount_value INTEGER NOT NULL DEFAULT 0 CHECK (invoice_discount_value >= 0);
ALTER TABLE invoices ADD COLUMN tax_rate_bps INTEGER NOT NULL DEFAULT 0 CHECK (tax_rate_bps BETWEEN 0 AND 10000);
ALTER TABLE invoices ADD COLUMN numbering_series TEXT NOT NULL DEFAULT 'default';

CREATE TABLE numbering_sequences (
    series      TEXT PRIMARY KEY CHECK (length(trim(series)) BETWEEN 1 AND 100),
    next_value  INTEGER NOT NULL CHECK (next_value > 0),
    updated_at  TEXT NOT NULL
);

CREATE INDEX ix_invoices_numbering_series ON invoices(numbering_series, number);
