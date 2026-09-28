-- Read-only Task8 rollout preflight. Execute while every old Run/Variant writer is quiesced.
-- Save the complete result set only in the access-controlled deployment evidence store.
WITH variant_map AS (
  SELECT id AS variant_id, tenant_id, local_agent_id, local_agent_version_id, release_id, state, published_at, retired_at,
         COUNT(*) OVER (PARTITION BY tenant_id, local_agent_id) AS mapping_count
  FROM agent_adoption_variants WHERE local_agent_id IS NOT NULL AND local_agent_id<>''
), run_identity AS (
  SELECT tenant_id, run_id, session_id, created_at, json_extract(snapshot, '$.agent_id') AS agent_id
  FROM agent_runs WHERE status NOT IN ('succeeded','failed','canceled')
), candidates AS (
  SELECT r.*, v.local_agent_version_id, v.release_id, v.mapping_count, v.published_at,
         CASE WHEN r.created_at < v.published_at THEN 1 ELSE 0 END AS timestamp_conflict
  FROM run_identity r JOIN variant_map v ON v.tenant_id=r.tenant_id AND v.local_agent_id=r.agent_id
)
SELECT 'duplicate_variant_mapping' AS finding, tenant_id, variant_id AS subject_id, mapping_count AS row_count
FROM variant_map WHERE mapping_count > 1
UNION ALL
SELECT 'missing_or_stale_version_owner', v.tenant_id, v.local_agent_id, COUNT(*)
FROM variant_map v LEFT JOIN agent_versions av ON av.tenant_id=v.tenant_id AND av.agent_id=v.local_agent_id AND av.id=v.local_agent_version_id
WHERE (av.id IS NULL OR v.release_id='') GROUP BY v.tenant_id,v.local_agent_id
UNION ALL
SELECT CASE WHEN timestamp_conflict=1 THEN 'timestamp_conflict' ELSE 'attributable_active_run' END,
       tenant_id, run_id, COUNT(*) FROM candidates GROUP BY tenant_id,run_id,timestamp_conflict
UNION ALL
SELECT 'unmatched_active_run', r.tenant_id, r.run_id, 1
FROM run_identity r LEFT JOIN variant_map v ON v.tenant_id=r.tenant_id AND v.local_agent_id=r.agent_id
WHERE v.local_agent_id IS NULL;
