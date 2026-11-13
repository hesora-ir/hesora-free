ALTER TABLE invoices ADD COLUMN bank_accounts_snapshot TEXT NOT NULL DEFAULT '[]';
CREATE INDEX IF NOT EXISTS ix_invoices_kind_status ON invoices(kind, status);
CREATE INDEX IF NOT EXISTS ix_invoices_customer_date ON invoices(customer_id, issue_date);
