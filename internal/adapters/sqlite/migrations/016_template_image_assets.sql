ALTER TABLE document_templates ADD COLUMN logo_data TEXT NOT NULL DEFAULT '' CHECK(length(logo_data) <= 700000);
ALTER TABLE document_templates ADD COLUMN seal_data TEXT NOT NULL DEFAULT '' CHECK(length(seal_data) <= 700000);
ALTER TABLE document_templates ADD COLUMN signature_data TEXT NOT NULL DEFAULT '' CHECK(length(signature_data) <= 700000);
