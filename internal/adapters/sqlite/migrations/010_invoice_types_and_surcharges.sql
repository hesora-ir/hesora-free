ALTER TABLE invoices ADD COLUMN document_type TEXT NOT NULL DEFAULT 'sales';
ALTER TABLE invoices ADD COLUMN additional_charges_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE invoices ADD COLUMN additional_charge_minor INTEGER NOT NULL DEFAULT 0 CHECK (additional_charge_minor >= 0);
UPDATE invoices SET document_type='proforma' WHERE kind='proforma';
