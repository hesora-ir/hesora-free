INSERT INTO local_cloud_changes(kind,local_id,operation,created_at)
SELECT 'template',id,'upsert',updated_at FROM document_templates;
