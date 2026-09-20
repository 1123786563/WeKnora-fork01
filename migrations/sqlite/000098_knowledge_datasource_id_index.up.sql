-- SP2-a connectors governance: SQLite twin of versioned 000177. Expression
-- index for the delete-source cascade queries on knowledges
-- (repository FindKnowledgeIDsByDataSourceID / CountKnowledgeByDataSourceID):
--   WHERE tenant_id = ? AND knowledge_base_id = ? AND deleted_at IS NULL
--     AND metadata->>'datasource_id' = ?
--
-- SQLite supports expression and partial indexes but has no text_pattern_ops
-- opclass, so the equivalent expression is spelled with json_extract (the
-- desugared form of ->>, available far longer than the operator itself).
-- The repository predicate uses the ->> operator, which SQLite 3.38+ parses
-- to the same json_extract call, so the planner can still match this index.
-- Even if a given SQLite build did not match it, the cascade sweep scans at
-- most one knowledge base's rows — acceptable at KB scale — so this index is
-- best-effort acceleration, never a correctness dependency.
CREATE INDEX IF NOT EXISTS idx_knowledges_kb_metadata_datasource_id
    ON knowledges (knowledge_base_id, json_extract(metadata, '$.datasource_id'))
    WHERE deleted_at IS NULL;
