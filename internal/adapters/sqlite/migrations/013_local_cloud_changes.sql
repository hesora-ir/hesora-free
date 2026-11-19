ALTER TABLE cloud_document_map ADD COLUMN remote_version INTEGER NOT NULL DEFAULT 0;

CREATE TABLE local_cloud_changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    local_id INTEGER NOT NULL,
    operation TEXT NOT NULL CHECK(operation IN ('upsert','delete')),
    created_at TEXT NOT NULL
);
CREATE INDEX ix_local_cloud_changes_cursor ON local_cloud_changes(id);

CREATE TABLE cloud_sync_control (
    id INTEGER PRIMARY KEY CHECK(id=1),
    applying_remote INTEGER NOT NULL DEFAULT 0 CHECK(applying_remote IN (0,1))
);
INSERT INTO cloud_sync_control(id,applying_remote) VALUES(1,0);

INSERT INTO local_cloud_changes(kind,local_id,operation,created_at)
 SELECT 'customer',id,'upsert',updated_at FROM customers;
INSERT INTO local_cloud_changes(kind,local_id,operation,created_at)
 SELECT 'issuer',id,'upsert',updated_at FROM issuer_profiles;
INSERT INTO local_cloud_changes(kind,local_id,operation,created_at)
 SELECT 'invoice',id,'upsert',updated_at FROM invoices;
