CREATE TABLE issuer_letterheads (
    id INTEGER PRIMARY KEY,
    issuer_profile_id INTEGER REFERENCES issuer_profiles(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 200),
    logo_path TEXT NOT NULL DEFAULT '' CHECK(length(logo_path) <= 2048),
    title TEXT NOT NULL DEFAULT '' CHECK(length(title) <= 300),
    subtitle TEXT NOT NULL DEFAULT '' CHECK(length(subtitle) <= 500),
    contact_line TEXT NOT NULL DEFAULT '' CHECK(length(contact_line) <= 1000),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE bank_accounts (
    id INTEGER PRIMARY KEY,
    issuer_profile_id INTEGER REFERENCES issuer_profiles(id) ON DELETE RESTRICT,
    bank_name TEXT NOT NULL CHECK(length(trim(bank_name)) BETWEEN 1 AND 200),
    account_holder TEXT NOT NULL CHECK(length(trim(account_holder)) BETWEEN 1 AND 200),
    account_number TEXT NOT NULL DEFAULT '' CHECK(length(account_number) <= 100),
    card_number TEXT NOT NULL DEFAULT '' CHECK(length(card_number) <= 100),
    iban TEXT NOT NULL DEFAULT '' CHECK(length(iban) <= 100),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE document_templates (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE CHECK(length(trim(name)) BETWEEN 1 AND 200),
    page_size TEXT NOT NULL CHECK(page_size IN ('A4','Letter')),
    orientation TEXT NOT NULL CHECK(orientation IN ('portrait','landscape')),
    font_family TEXT NOT NULL CHECK(length(font_family) BETWEEN 1 AND 80),
    font_size INTEGER NOT NULL CHECK(font_size BETWEEN 8 AND 32),
    primary_color TEXT NOT NULL CHECK(length(primary_color)=7),
    accent_color TEXT NOT NULL CHECK(length(accent_color)=7),
    letterhead_id INTEGER REFERENCES issuer_letterheads(id) ON DELETE SET NULL,
    show_logo INTEGER NOT NULL DEFAULT 1 CHECK(show_logo IN (0,1)),
    show_tax INTEGER NOT NULL DEFAULT 1 CHECK(show_tax IN (0,1)),
    show_bank_accounts INTEGER NOT NULL DEFAULT 1 CHECK(show_bank_accounts IN (0,1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE template_bank_accounts (
    template_id INTEGER NOT NULL REFERENCES document_templates(id) ON DELETE CASCADE,
    bank_account_id INTEGER NOT NULL REFERENCES bank_accounts(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL DEFAULT 0 CHECK(position >= 0),
    PRIMARY KEY(template_id, bank_account_id),
    UNIQUE(template_id, position)
);

CREATE INDEX ix_letterheads_issuer ON issuer_letterheads(issuer_profile_id);
CREATE INDEX ix_bank_accounts_issuer ON bank_accounts(issuer_profile_id);
CREATE INDEX ix_templates_letterhead ON document_templates(letterhead_id);
