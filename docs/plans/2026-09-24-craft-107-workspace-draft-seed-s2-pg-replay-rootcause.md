# S2 PostgreSQL same-key replay failure — root-cause investigation

## Scope and environment

Read-only diagnosis of `TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres` at line 155. No production fix was attempted. The test was reproduced against a new disposable `paradedb/paradedb:v0.22.2-pg17` container (PostgreSQL 17.9) on localhost port 55441 using the repository's PostgreSQL migration/test setup. The disposable container was removed after the investigation; no shared development database was used.

The test-only instrumentation around the replay assertion logged the existing row identity, the prepared incoming snapshot, the Craft marker, actor membership, claim count, and canonical intent bytes. I restored `agent_run_craft_seed_test.go` from its pre-instrumentation copy afterward; its SHA-256 is again `feca684053c195e10ccbb73fb9bfe99c0a696f758a8478e8b5c8624b47906f70`, matching the Fix1 checkpoint. No production file changed during diagnosis.

## Reproduction

```sh
TRPC_TEST_POSTGRES_DSN="$DISPOSABLE_PG_DSN" go test ./internal/application/repository \
  -run '^TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres$' \
  -count=1 -v
```

Exit 1, consistently at the same-key replay after advancing D2:

```text
agent_run_craft_seed_test.go:155: Received unexpected error: agent runtime conflict
```

## Evidence at replay

Temporary test diagnostics read the existing Run row and prepared the incoming snapshot through the same `persistUsageBinding` boundary as `Admit`:

```text
owner="u1" actor="u1" existing_actor="u1" session="s1"
driver="platform"/"" target=""/"" budget=""/""
assistant="assistant-seed-b"/"assistant-seed-b"
craft=true actor_exists=true claims=0
marked=true marker_err=<nil> supplied_seed=false
same_intent=false
```

The stored and incoming request hashes differed (`281fd9…` vs `c0fff4…` in this run), as expected: the stored hash covers the server-selected seed while the incoming service hash covers the original unseeded intent. The Craft replay branch intentionally does not compare these hashes.

After removing `craft_workspace_seed`, the canonical outer snapshot JSON was equal in length (174 bytes) but differed in the nested `craft_knowledge_selection` key order:

```text
stored:   ..."craft_knowledge_selection":{"query":"Build B","knowledge_base_ids":["kb-b"]}...
incoming: ..."craft_knowledge_selection":{"knowledge_base_ids":["kb-b"],"query":"Build B"}...
```

All other outer fields and values matched. This makes `sameCraftAdmissionIntent` return false, causing the generic `agent runtime conflict` at the replay branch before any claim transition. The PostgreSQL schema confirms `agent_runs.snapshot` is JSONB (`migrations/versioned/000093_agent_runs.up.sql:24`); PostgreSQL's JSONB rendering normalizes nested object key order on read. SQLite keeps the snapshot as text, so the original Go-encoded nested order survives and its test passes.

## Root cause

**Confirmed:** replay comparison canonicalizes only the top-level object. `sameCraftAdmissionIntent` unmarshals to `map[string]json.RawMessage`, deletes the seed, then marshals the map. Top-level keys are sorted, but each nested `json.RawMessage` is emitted with its existing raw key order. PostgreSQL JSONB returns a different nested key order than the incoming Go-marshaled snapshot. The equality is therefore byte-sensitive to JSONB's nested object normalization even though the JSON values are semantically equal.

The failure is not caused by Session identity, actor identity/membership, assistant identity, T01 claims, `RequestHash`, or the current head D2. The diagnostics show those identity/fence values match, and replay reaches the canonical-intent predicate.

## Fix direction (not implemented)

Use recursive semantic canonicalization of the JSON snapshot before equality comparison, or compare a typed canonical Craft request-intent representation. The change must continue to ignore only the server-selected seed, preserve request/actor equality, and must not use the seed-inclusive RequestHash as the replay comparator. Re-run PostgreSQL and SQLite replay/restart/head-advance tests after a separately authorized fix.
