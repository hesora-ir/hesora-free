CREATE TABLE customers (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 300),
    phone       TEXT NOT NULL DEFAULT '' CHECK (length(phone) <= 80),
    email       TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 254),
    address     TEXT NOT NULL DEFAULT '' CHECK (length(address) <= 2000),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE issuer_profiles (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
    legal_name  TEXT NOT NULL DEFAULT '',
    tax_id      TEXT NOT NULL DEFAULT '',
    phone       TEXT NOT NULL DEFAULT '',
    email       TEXT NOT NULL DEFAULT '',
    address     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE invoices (
    id                INTEGER PRIMARY KEY,
    number            TEXT NOT NULL UNIQUE CHECK (length(trim(number)) > 0),
    kind              TEXT NOT NULL CHECK (kind IN ('proforma', 'invoice', 'settlement')),
    status            TEXT NOT NULL CHECK (status IN ('draft', 'issued', 'void')),
    currency          TEXT NOT NULL CHECK (currency IN ('IRR', 'IRT')),
    customer_id       INTEGER REFERENCES customers(id) ON DELETE RESTRICT,
    issuer_profile_id INTEGER REFERENCES issuer_profiles(id) ON DELETE RESTRICT,
    customer_snapshot TEXT NOT NULL DEFAULT '{}',
    issuer_snapshot   TEXT NOT NULL DEFAULT '{}',
    issue_date        TEXT NOT NULL,
    due_date          TEXT NOT NULL DEFAULT '',
    subtotal_minor    INTEGER NOT NULL DEFAULT 0 CHECK (subtotal_minor >= 0),
    discount_minor    INTEGER NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    tax_minor         INTEGER NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
    total_minor       INTEGER NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
    note              TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    CHECK (discount_minor <= subtotal_minor),
    CHECK (total_minor = subtotal_minor - discount_minor + tax_minor)
);

CREATE TABLE invoice_items (
    id             INTEGER PRIMARY KEY,
    invoice_id     INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    position       INTEGER NOT NULL CHECK (position >= 0),
    description    TEXT NOT NULL CHECK (length(trim(description)) > 0),
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    unit           TEXT NOT NULL DEFAULT '',
    unit_price     INTEGER NOT NULL CHECK (unit_price >= 0),
    discount_minor INTEGER NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    line_total     INTEGER NOT NULL CHECK (line_total >= 0),
    UNIQUE(invoice_id, position),
    CHECK (discount_minor <= quantity * unit_price)
);
