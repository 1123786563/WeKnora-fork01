# Semantica C01 Versioned Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Define and generate an independently versioned Semantica gRPC contract with matching Go and Python domain DTOs, without coupling it to DocReader or upstream Semantica objects.

**Architecture:** `semantic/proto/semantic.proto` is the sole wire source for package `weknora.semantic.v1`; generated Go/Python code stays transport-only. Go and Python domain DTOs explicitly map to/from that wire contract. The generator runs in a version-recorded, dedicated toolchain and rejects stale generated outputs.

**Tech Stack:** proto3, gRPC, Go protobuf, Python grpcio-tools, Python dataclasses, Go `uint64` tenant identifiers.

**Spec:** [ADR-0002](../adr/0002-semantica-independent-service.md), [rebaseline](2026-09-20-semantica-rebaseline.md), [C01 historical detail](2026-09-11-semantica-01-service.md), [architecture spec](../specs/2026-09-11-semantica-graphrag-reasoning-design.md).

## Global Constraints

- Use a separate Semantica protocol; no RPC, DTO, or generator edit extends DocReader.
- `tenant_id` and revisions are `uint64`; diagnostics/golden JSON serialize 64-bit values as decimal strings.
- Public IDs are stable strings, never Neo4j internal IDs. V02 fixture tenant strings are experiment-only and must not leak into C01.
- RPC set: `GetCapabilities`, `ApplyDocumentRevision`, `DeleteDocument`, `GetOperation`, `CancelOperation`, `Search`, `Reason`.
- Delete requires `deleted=true`; Get/Cancel require `ScopeKey` plus operation ID; Search returns requested and actual modes; Search never upgrades to Reason; Reason allows an absent conclusion.
- Use enums for state/modes/status and preserve proto tags. `Assertion`, derivation, `IndexManifest`, and `ReadLease` are deferred to C03/I03.
- AccessScope/token claims are Go-authoritative; wire payloads do not create authority and no long-lived model credential crosses this boundary.
- Generation must record exact protoc, Go plugin, grpcio-tools, grpcio, and protobuf versions. Host lacks `protoc`/`grpc_tools`; no unpinned host generation.

### Task 1: Contract source and deterministic generator

**Files:**
- Create: `semantic/proto/semantic.proto`
- Create: `semantic/scripts/generate_proto.sh`
- Create: `semantic/proto/semantic.pb.go`, `semantic/proto/semantic_grpc.pb.go`
- Create: `semantic/semantic_service/proto/semantic_pb2.py`, `semantic_pb2_grpc.py`, `semantic_pb2.pyi`
- Create: `semantic/tests/test_proto_contract.py`
- Create: `semantic/proto/GENERATOR_VERSIONS.md`

- [ ] **Step 1: Write failing contract tests**

```python
def test_contract_exposes_only_versioned_semantica_service():
    from semantic_service.proto import semantic_pb2, semantic_pb2_grpc
    assert semantic_pb2.DESCRIPTOR.package == "weknora.semantic.v1"
    assert set(semantic_pb2.DESCRIPTOR.services_by_name["SemanticService"].methods_by_name) == {
        "GetCapabilities", "ApplyDocumentRevision", "DeleteDocument",
        "GetOperation", "CancelOperation", "Search", "Reason",
    }
```

- [ ] **Step 2: Verify RED**

Run: `uv run --project semantic python -m pytest semantic/tests/test_proto_contract.py -q`

Expected: failure because C01 proto, generated package, and production Python project do not exist.

- [ ] **Step 3: Define the proto**

Define ScopeKey, DocumentRevision, ChunkSnapshot, IndexConfig, ApplyRequest, OperationRef/Operation, Evidence with optional spans, AccessScope, QueryLimits, Search/Reason request/response, Capabilities, and only the seven RPCs. Use explicit enums/oneofs for state/modes/status/optional conclusions; preserve tags in comments.

- [ ] **Step 4: Implement dedicated generator**

The script must run a pinned generator environment, verify each tool version before generation, generate both languages from one schema, rewrite only Semantica Python imports, and fail if `git diff --exit-code` finds stale outputs. Record exact versions in `GENERATOR_VERSIONS.md`; do not reuse DocReader’s hard-coded script.

- [ ] **Step 5: Verify GREEN and commit**

Run generated-output tests plus the generator stale-output check. Commit only C01 proto/generator/generated artifacts.

### Task 2: Go/Python domain DTO mapping

**Files:**
- Create: `internal/types/semantic.go`
- Create: `internal/types/semantic_test.go`
- Create: `semantic/semantic_service/contracts.py`
- Create: `semantic/tests/test_contracts.py`

- [ ] **Step 1: Write failing mapping tests**

```go
func TestSemanticDeleteRequiresDeletedRevision(t *testing.T) {
    _, err := types.SemanticDocumentRevisionFromWire(&semanticpb.DocumentRevision{Deleted: false})
    require.Error(t, err)
}
```

```python
def test_reason_without_conclusion_is_representable():
    result = ReasonResponse(status=ReasonStatus.INSUFFICIENT_EVIDENCE, conclusion=None)
    assert result.conclusion is None
```

- [ ] **Step 2: Verify RED**

Run exact Go and Python test paths. Expected: missing DTO/mapping behavior failures, not generator/toolchain errors.

- [ ] **Step 3: Implement transport-neutral DTOs and mapping**

Create `Semantic*` Go types and frozen Python dataclasses. Map protobuf at the adapter edge; validate uint64 IDs, deleted revision, access scope identity fields, decimal-string diagnostic conversion, optional spans, and requested/actual mode echo. No RPC client/server or authority issuance belongs here.

- [ ] **Step 4: Verify GREEN and commit**

Run Go package tests and Python C01 tests. Record generator versions, exact commands, commit SHA, and limitations in `progress.md`; mark C01 `implemented` pending independent review.

## Self-Review

- Contract contains no DocReader extension, provider call, storage topology, assertion model, or production authorization implementation.
- Every RPC parameter/result is owned by C01 or an explicitly deferred later task.
- Generated output reproducibility is a testable contract, not an assumed local setup.
