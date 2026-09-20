DO $$ BEGIN RAISE NOTICE '[Migration 000177] Dropping knowledge metadata datasource_id index...'; END $$;

DROP INDEX IF EXISTS idx_knowledges_kb_metadata_datasource_id;
