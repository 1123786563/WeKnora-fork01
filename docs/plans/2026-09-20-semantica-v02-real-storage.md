# Semantica V02 Real-Storage Capability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce reproducible `real-storage` evidence for Semantica’s scope-bounded persistence bridge across a dedicated local Neo4j topology and an explicitly provisioned shared-isolated topology, without claiming model, ACL, or production acceptance.

**Architecture:** A V02-only adapter owns stable semantic keys and parameterized Cypher; it does not expose Neo4j internal IDs or Semantica’s process-local ID map. Every storage read constrains tenant, knowledge base, document/revision and generation before an in-memory bridge is assembled. Each topology run is a writer process followed by a separate reader process; no graph object or cache crosses the process boundary.

**Tech Stack:** Python 3.12.12, uv, pytest, neo4j Python driver, Docker/Compose Neo4j 2025.10.1, Semantica 0.6.8.

**Spec:** [ADR-0002](../adr/0002-semantica-independent-service.md), [2026-09-20 rebaseline](2026-09-20-semantica-rebaseline.md), [V01/V02 historical detail](2026-09-11-semantica-00-verification.md), and [architecture specification](../specs/2026-09-11-semantica-graphrag-reasoning-design.md).

## Global Constraints

- V01 is the verified prerequisite; V02 evidence must record its lock SHA-256 and Semantica version.
- Real storage means a physical backend with durable data, writer process exit, and independently launched reader process. An in-memory fake, static source inspection, or mock cannot be `verified`.
- Evaluate `dedicated` and `shared-isolated` separately. Passing a dedicated local Neo4j instance does not select a production topology or replace the shared-isolated comparison.
- The historical V02 model probe is deferred: Python must not invoke a provider or hold long-lived model credentials. Deterministic registered rule evidence only belongs in V02.
- Query tenant/KB/document/revision/generation constraints at the backend before constructing a graph. A foreign/restricted distractor must not be returned from storage.
- Persist public identities and provenance as explicit properties: semantic/assertion/evidence IDs, tenant, KB, document, revision, chunk, content hash, quote, spans, rule version, and generation. Neo4j IDs are backend-only.
- Write through adapter-owned parameterized `MERGE`, with a compound logical key. Do not rely on `GraphStore._app_node_id_map` across process restart.
- Record endpoint, version, volume/namespace/account boundary and process PIDs without recording credentials. The shared-isolated run requires a real account/namespace boundary supplied by an authorized operator.

## Review Focus

- A target query must demonstrate the foreign/restricted distractor was absent from the database result, not merely omitted from output.
- Restart proof must use a separate process that receives only topology configuration, never serialized graph state or an in-process ID cache.
- The adapter must reject an unknown topology or missing scope/generation constraint before opening a query.
- The rule must derive only registered `depends_on` transitivity and return no conclusion when a premise is absent.
- A dedicated local pass must remain `implemented` until an independently provisioned shared-isolated comparison is evidenced and a topology decision is recorded.

### Task 1: Scope-first bridge fixture and adapter

**Files:**
- Create: `semantic/experiments/fixtures/v02-controlled-graph.json`
- Create: `semantic/experiments/bridge_probe.py`
- Create: `semantic/experiments/test_graph_bridge.py`
- Create: `semantic/experiments/test_reasoning_bridge.py`
- Modify: `semantic/experiments/pyproject.toml`
- Modify: `semantic/experiments/uv.lock`

**Interfaces:**
- Produces `TopologyConfig`, `write_fixture(config, fixture)`, `read_scope(config, scope)`, `probe_roundtrip(config, fixture_path)`, and `probe_rule(assertions, rule_id)`.
- `read_scope` returns only persisted records matching `tenant_id`, `kb_id`, `document_id`, `revision`, and `generation`; it returns `storage_returned_ids` for evidence.

- [ ] **Step 1: Write failing scope and rule tests**

```python
def test_scope_query_never_returns_foreign_distractor(dedicated_config):
    result = probe_roundtrip(dedicated_config, FIXTURE)
    assert result["storage_returned_ids"] == {"a-d1", "a-d2"}
    assert result["evidence_ids"] == {"e-d1", "e-d2"}
    assert result["foreign_ids"] == set()

def test_registered_rule_requires_both_premises():
    result = probe_rule(["a-d1"], "technical-dependency-transitivity@v1")
    assert result["status"] == "insufficient_evidence"
    assert result["conclusions"] == []
```

- [ ] **Step 2: Run RED**

Run: `uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q`

Expected: import/behavior failure because the fixture and V02 adapter do not exist. A missing Docker service is not this RED and must be separately recorded.

- [ ] **Step 3: Define the fixture and narrow adapter**

The fixture contains target `T1/K1` facts `a-d1`/`a-d2` with Chinese quotes and evidence `e-d1`/`e-d2`, plus a persisted T2/K2 distractor. Add the Neo4j driver dependency and locked version. `write_fixture` uses parameterized `MERGE` keyed on `(tenant_id, kb_id, generation, record_kind, semantic_id)`. `read_scope` uses `WHERE` constraints for the full scope and returns logical IDs/provenance, never `id(n)` as public output.

- [ ] **Step 4: Implement deterministic rule behavior**

`probe_rule` accepts only `technical-dependency-transitivity@v1`; it derives `A indirectly_depends_on C` only with exact premises `a-d1`, `a-d2`, rule ID/version, and evidence `e-d1`, `e-d2`. A missing premise returns `insufficient_evidence`; it never invokes a model.

- [ ] **Step 5: Run GREEN**

Run: the Step 2 command with a dedicated test configuration connected to an actual Neo4j service.

Expected: all tests pass, and failure to supply real topology configuration exits nonzero rather than skipping.

### Task 2: Dedicated local real-storage run

**Files:**
- Create: `docker/compose.semantic-v02.yml`
- Create: `semantic/experiments/run_v02.py`
- Create: `docs/plans/semantica/evidence/<date>/v02-dedicated.json`
- Create: `docs/plans/semantica/bridge-evidence.md`

- [ ] **Step 1: Write failing restart test**

```python
def test_fresh_reader_process_recovers_persisted_provenance(dedicated_config):
    result = run_writer_then_reader(dedicated_config, FIXTURE)
    assert result["writer_pid"] != result["reader_pid"]
    assert result["restart_verified"] is True
    assert result["assertion_ids"] == {"a-d1", "a-d2"}
    assert result["evidence_ids"] == {"e-d1", "e-d2"}
```

- [ ] **Step 2: Run RED and start the dedicated service**

Run the test first and record its missing-adapter failure. Then start only `docker/compose.semantic-v02.yml` with a test-only, non-default credential supplied through an ignored local environment file. The compose service must use cached `neo4j:2025.10.1`, a named V02-only volume, and host port allocation that does not affect project services.

- [ ] **Step 3: Implement writer/reader CLI**

The writer connects, creates the fixture, verifies durable commit, closes, and exits. The reader is a separately spawned Python process receiving only a topology manifest path; it reconnects, calls `read_scope`, runs the deterministic rule over the returned scope-bounded rows, and writes JSON with PIDs, backend/version, logical expected/actual IDs, provenance, storage-returned IDs, rule output, commands, exit codes, limits, and `evidence_layer: real-storage`.

- [ ] **Step 4: Run dedicated GREEN and capture evidence**

Run writer/reader once after the service starts and once after the service container restart while retaining its named volume. Both runs must retain the same logical IDs/provenance and exclude the distractor.

- [ ] **Step 5: Commit dedicated evidence without promoting V02**

Update `bridge-evidence.md` and `progress.md`: dedicated evidence may be `implemented`, but V02 remains unverified until Task 3. Commit only V02 files.

### Task 3: Shared-isolated comparison and topology decision

**Files:**
- Create: `docs/plans/semantica/evidence/<date>/v02-shared-isolated.json`
- Modify: `docs/plans/semantica/bridge-evidence.md`
- Modify: `docs/plans/semantica/progress.md`

- [ ] **Step 1: Obtain an operator-provisioned shared boundary**

Require a non-secret topology manifest naming the provider/version, endpoint alias, dedicated account/role identity, and database/schema/collection/prefix boundary. The credentials themselves stay in an ignored local file or operator environment. If unavailable, record V02 as `implemented`/`blocked` with this exact missing boundary; do not synthesize a shared result.

- [ ] **Step 2: Run the identical writer/reader matrix**

Use the Task 2 fixture and tests without changing expectations. Evidence must show that the shared account cannot read/write the foreign boundary and that the target query never returns the stored distractor.

- [ ] **Step 3: Record comparison and selection**

Record both topology evidence records, the selected candidate, and why its isolation is sufficient. Only after both rows have `real-storage` evidence may V02 be marked `verified`; this is not a production-promotion decision.

## Self-Review

- V02 no longer treats source inspection, mocks, or model invocation as storage acceptance.
- The test boundary is storage-first scope filtering, not a future Go ACL claim.
- Dedicated and shared-isolated runs have distinct evidence and neither can silently substitute for the other.
- V03 models/quality and all C01+ production work remain outside this plan.
