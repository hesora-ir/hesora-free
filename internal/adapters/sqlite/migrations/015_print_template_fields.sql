ALTER TABLE document_templates ADD COLUMN prefix TEXT NOT NULL DEFAULT 'FACT-' CHECK(length(prefix) <= 100);
ALTER TABLE document_templates ADD COLUMN thank_you TEXT NOT NULL DEFAULT '' CHECK(length(thank_you) <= 2000);
ALTER TABLE document_templates ADD COLUMN show_seal INTEGER NOT NULL DEFAULT 0 CHECK(show_seal IN (0,1));
ALTER TABLE document_templates ADD COLUMN show_signature INTEGER NOT NULL DEFAULT 0 CHECK(show_signature IN (0,1));
