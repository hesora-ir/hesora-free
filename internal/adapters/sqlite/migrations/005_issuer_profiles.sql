ALTER TABLE issuer_profiles ADD COLUMN profile_type TEXT NOT NULL DEFAULT 'individual'
    CHECK (profile_type IN ('individual', 'company'));
ALTER TABLE issuer_profiles ADD COLUMN first_name TEXT NOT NULL DEFAULT '';
ALTER TABLE issuer_profiles ADD COLUMN last_name TEXT NOT NULL DEFAULT '';

CREATE TABLE issuer_profile_phones (
    id INTEGER PRIMARY KEY,
    issuer_profile_id INTEGER NOT NULL REFERENCES issuer_profiles(id) ON DELETE CASCADE,
    phone TEXT NOT NULL CHECK(length(trim(phone)) BETWEEN 1 AND 80),
    position INTEGER NOT NULL DEFAULT 0 CHECK(position >= 0),
    UNIQUE(issuer_profile_id, position),
    UNIQUE(issuer_profile_id, phone)
);

CREATE TABLE issuer_profile_bank_details (
    id INTEGER PRIMARY KEY,
    issuer_profile_id INTEGER NOT NULL REFERENCES issuer_profiles(id) ON DELETE CASCADE,
    card_number TEXT NOT NULL DEFAULT '' CHECK(length(card_number) <= 100),
    account_number TEXT NOT NULL DEFAULT '' CHECK(length(account_number) <= 100),
    iban TEXT NOT NULL DEFAULT '' CHECK(length(iban) <= 100),
    position INTEGER NOT NULL DEFAULT 0 CHECK(position >= 0),
    UNIQUE(issuer_profile_id, position),
    CHECK(length(trim(card_number)) + length(trim(account_number)) + length(trim(iban)) > 0)
);

INSERT INTO issuer_profile_phones(issuer_profile_id, phone, position)
SELECT id, trim(phone), 0 FROM issuer_profiles WHERE length(trim(phone)) > 0;

CREATE INDEX ix_issuer_profile_phones_profile ON issuer_profile_phones(issuer_profile_id);
CREATE INDEX ix_issuer_profile_bank_details_profile ON issuer_profile_bank_details(issuer_profile_id);
