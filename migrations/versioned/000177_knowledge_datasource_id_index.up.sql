DO $$ BEGIN RAISE NOTICE '[Migration 000177] Adding knowledge metadata datasource_id index...'; END $$;

-- Speeds up the delete-source cascade queries (repository
-- FindKnowledgeIDsByDataSourceID / CountKnowledgeByDataSourceID):
--   WHERE tenant_id = $1 AND knowledge_base_id = $2 AND deleted_at IS NULL
--     AND metadata->>'datasource_id' = $3
--
-- Mirrors idx_knowledges_kb_metadata_external_id (migration 000076). The
-- knowledge_base_id equality is served by idx_knowledges_base_id, but the
-- metadata->>'datasource_id' equality would otherwise be a post-filter. This
-- composite expression index lets the datasource_id match be served by the
-- index too. Same two deliberate choices as 000076:
--   * text_pattern_ops — compares raw bytes, so the index is usable regardless
--     of the database collation (which even carries a version mismatch here).
--   * partial on deleted_at IS NULL — matches the queries' own predicate and
--     keeps the index limited to live rows.
-- The queries MUST keep the expression as the SQL literal
-- metadata->>'datasource_id' (see the comment in repository/knowledge.go) —
-- a bound metadata->>$1 cannot be matched by the planner.
-- Non-destructive: index-only, no schema or data change. IF NOT EXISTS makes
-- it idempotent and safe to re-run.
CREATE INDEX IF NOT EXISTS idx_knowledges_kb_metadata_datasource_id
    ON knowledges (knowledge_base_id, (metadata->>'datasource_id') text_pattern_ops)
    WHERE deleted_at IS NULL;
