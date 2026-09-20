# Semantica V02 persistence-bridge evidence

## Dedicated local Neo4j — 2026-09-20

Evidence artifact: [v02-dedicated.json](evidence/2026-09-20/v02-dedicated.json).

- Evidence layer: `real-storage`; Semantica `0.6.8`; V01 lock SHA-256 `c643ce123490c93f56ed95e9d6501bbd90daf8b2cdc74b183067dccd63864c54`; current V02 lock SHA-256 `7cea23660879d96120a8936b44d9c23d4aaaf036b31c9a4e79812d31a4efe709`.
- Boundary: test-only dedicated Neo4j `2025.10.1`, loopback Bolt alias `loopback:17687`, database `neo4j`, account identity `neo4j`, and durable volume `semantica-v02_semantica-v02-neo4j-data`. Credentials are stored only in ignored `docker/.env.semantic-v02.local` and are not recorded here.
- Initial writer/reader PIDs were `84147` / `84252`. After a container restart retaining the named volume, writer/reader PIDs were `84723` / `84725`. Both reader processes were independently launched and received no graph or in-process ID cache from the writer.
- The storage-constrained reads returned exactly assertions `a-d1`, `a-d2` and evidence `e-d1`, `e-d2`, retaining document/revision/chunk/hash/Chinese quote provenance. Stored `T2/K2` assertion `a-foreign` was not returned.
- Registered rule `technical-dependency-transitivity@v1` derived only `A indirectly_depends_on C`, with premises `[a-d1, a-d2]` and evidence `[e-d1, e-d2]`; the missing-premise test returns `insufficient_evidence`.
- Commands: `uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q` exited 0 with 7 tests; `docker compose --env-file docker/.env.semantic-v02.local -f docker/compose.semantic-v02.yml restart neo4j` exited 0; dedicated capture commands exited 0.

This is not a production topology selection, shared-isolated comparison, provider/model result, Go authorization result, deletion/revocation result, or quality/performance acceptance.
