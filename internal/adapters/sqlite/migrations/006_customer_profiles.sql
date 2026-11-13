ALTER TABLE customers ADD COLUMN customer_type TEXT NOT NULL DEFAULT 'individual'
    CHECK (customer_type IN ('individual', 'company'));
ALTER TABLE customers ADD COLUMN first_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN last_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN legal_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN tax_id TEXT NOT NULL DEFAULT '';

CREATE TABLE customer_phones (
    id INTEGER PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    phone TEXT NOT NULL CHECK (length(trim(phone)) BETWEEN 1 AND 80),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    UNIQUE(customer_id, position),
    UNIQUE(customer_id, phone)
);

CREATE TABLE customer_addresses (
    id INTEGER PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    address TEXT NOT NULL CHECK (length(trim(address)) BETWEEN 1 AND 2000),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    UNIQUE(customer_id, position),
    UNIQUE(customer_id, address)
);

INSERT INTO customer_phones(customer_id, phone, position)
SELECT id, trim(phone), 0 FROM customers WHERE length(trim(phone)) > 0;

INSERT INTO customer_addresses(customer_id, address, position)
SELECT id, trim(address), 0 FROM customers WHERE length(trim(address)) > 0;

CREATE INDEX ix_customer_phones_customer ON customer_phones(customer_id);
CREATE INDEX ix_customer_addresses_customer ON customer_addresses(customer_id);
