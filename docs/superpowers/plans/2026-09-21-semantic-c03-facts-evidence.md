# Semantica C03 Facts and Evidence Implementation Plan

> **For agentic workers:** REQUIRED SKILL: Use superpowers:dispatching-parallel-agents for the two independent implementation tasks; steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add transport-neutral evidence validation and scoped fact/assertion models that preserve source provenance, conflicts, and acyclic derivations.

**Architecture:** Keep C01 protobuf/contracts unchanged. `evidence.py` validates C01 `ChunkSnapshot`/`Evidence` values and Unicode codepoint spans. `facts.py` owns scoped Entity/Assertion/Derivation values and validates explicit scope references and referential closure against evidence and premise maps; callers validate source text with `evidence.py` before admitting the scoped evidence into assertion validation. Neither module persists or writes graphs.

**Tech Stack:** Python 3.12, frozen dataclasses, stdlib `hashlib` only if later justified (this slice does not select a content-hash algorithm), `uuid`, pytest, existing C01 contracts.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §4, with boundaries in `docs/adr/0002-semantica-independent-service.md`, `docs/plans/2026-09-20-semantica-rebaseline.md`, and C03 in `docs/plans/2026-09-11-semantica-01-service.md`.

## Global Constraints

- Tenant, KB, document, standard chunk, revision, and ACL authority remains in Go; these modules only validate transport-neutral snapshots and model semantic facts.
- Every semantic record is scoped to exactly one tenant and KB; facts cannot reference entities, evidence, or premises from another scope.
- Entity names, aliases, and equivalences are assertions with sources, not identity keys; generated entity IDs are UUIDs.
- A source assertion records what a source claims, not objective truth; contradictory values coexist and are never last-write-wins.
- Rule/model derivations retain premise assertion identities and version metadata; cyclic or dangling derivations are rejected.
- Evidence quote spans use Python Unicode codepoint indexing over the exact chunk text, as Go `[]rune` does; no normalization or offset repair is allowed.
- This task does not add protocol fields, persistence, inference rules, model calls, graph storage, operation/generation publication, authorization issuance, or production wiring.

## Review Focus

- Emoji/non-BMP and combining Unicode: verify exact codepoint slicing and reject code-unit/grapheme confusion.
- Span/quote mismatch, absent span, and absent substring: accept only exact source text; never repair offsets.
- Hash identity: compare `Evidence.content_hash` to the supplied standard `ChunkSnapshot.content_hash`; the approved spec does not define a digest algorithm, so do not recompute one.
- Scope escape: reject subject/object/evidence/premise references outside the assertion's exact tenant+KB scope.
- Conflicting facts and derivation DAGs: preserve distinct values and reject only malformed/cyclic support, not semantic conflict itself.

---

### Task 1: Validate evidence against standard chunks

**Files:**
- Create: `semantic/semantic_service/evidence.py`
- Create: `semantic/tests/test_evidence.py`
- Consume unchanged: `semantic/semantic_service/contracts.py` (`ChunkSnapshot`, `Evidence`)

**Interfaces:**
- `extract_quote(text: str, start: int, end: int) -> str` returns `text[start:end]` for a valid half-open Unicode-codepoint span and raises `ValueError` unless `text` is `str`, `type(start) is type(end) is int` (so bool is rejected), and `0 <= start <= end <= len(text)`.
- `validate_span(text: str, start: int, end: int, quote: str) -> None` requires `quote` to be a string and exact extracted quote equality.
- `validate_evidence(chunk: ChunkSnapshot, evidence: Evidence) -> None` requires string chunk text/quote, matching chunk id and opaque content-hash value; an anchored quote must exactly match its span, and an unanchored quote must be an exact substring of chunk text.

- [x] **Step 1: Write failing behavior tests**

```python
def test_span_uses_unicode_codepoints():
    assert extract_quote("甲😀乙", 1, 2) == "😀"
    assert extract_quote("e\u0301", 1, 2) == "\u0301"
    with pytest.raises(ValueError):
        validate_span("甲😀乙", 1, 2, "乙")

def test_span_rejects_invalid_bounds_without_repair():
    for start, end in ((-1, 0), (2, 1), (0, 4), (True, 1)):
        with pytest.raises(ValueError):
            extract_quote("甲😀乙", start, end)

def test_evidence_requires_matching_chunk_hash_and_quote():
    chunk = ChunkSnapshot("c1", "甲😀乙", "opaque-hash")
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e1", "d1", 1, "c1", "other-hash", "😀", 1, 2))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e2", "d1", 1, "c1", "opaque-hash", "乙", 1, 2))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e3", "d1", 1, "wrong-chunk", "opaque-hash", "😀", 1, 2))

def test_unanchored_evidence_must_be_an_exact_substring():
    chunk = ChunkSnapshot("c1", "原文 😀", "h")
    validate_evidence(chunk, Evidence("e1", "d1", 1, "c1", "h", "😀", None, None))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e2", "d1", 1, "c1", "h", "缺失", None, None))
    decomposed = ChunkSnapshot("c2", "e\u0301", "h2")
    validate_evidence(decomposed, Evidence("e3", "d1", 1, "c2", "h2", "e\u0301", None, None))
    with pytest.raises(ValueError):
        validate_evidence(decomposed, Evidence("e4", "d1", 1, "c2", "h2", "\u00e9", None, None))

def test_unanchored_evidence_rejects_non_string_text_and_quote():
    with pytest.raises(ValueError):
        validate_evidence(ChunkSnapshot("c1", 1, "h"), Evidence("e1", "d1", 1, "c1", "h", "q", None, None))
    with pytest.raises(ValueError):
        validate_evidence(ChunkSnapshot("c1", "text", "h"), Evidence("e2", "d1", 1, "c1", "h", 1, None, None))
```

- [x] **Step 2: Run RED**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_evidence.py -q`

Expected: fail because `semantic_service.evidence` does not exist.

- [x] **Step 3: Implement the minimal validators**

Implement the three functions as specified. Do not normalize text; compare hash strings only; do not import the Semantica upstream package.

- [x] **Step 4: Run GREEN and the existing semantic contract suite**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_evidence.py semantic/tests/test_contracts.py -q`

Expected: all tests pass.

- [x] **Step 5: Commit Task 1**

Commit `feat(semantic): validate source evidence spans` with only these files.

---

### Task 2: Model scoped entities, assertions, and derivations

**Files:**
- Create: `semantic/semantic_service/facts.py`
- Create: `semantic/tests/test_facts.py`
- Consume unchanged: `semantic/semantic_service/contracts.py` (`ScopeKey`, `Evidence`)

**Interfaces:**
- `new_entity_id() -> str` returns a canonical UUID string.
- `Entity(entity_id: str, scope: ScopeKey, entity_type: str)` is identity only; it has no name or alias field. It rejects non-`ScopeKey` scope values.
- `EntityRef(entity_id: str, scope: ScopeKey)` makes scope part of every subject/object reference and rejects non-`ScopeKey` values.
- `ScopedEvidence(scope: ScopeKey, evidence: Evidence)` associates a C01 evidence DTO with the scope required by the domain model without changing the C01 wire contract; both fields are type-checked and a non-`ScopeKey` scope is rejected.
- `AssertionKind` has exactly `SOURCE`, `RULE_DERIVED`, and `MODEL_INFERRED` values (`source`, `rule_derived`, `model_inferred`). `Assertion` has non-empty `assertion_id`, a required `ScopeKey` `scope`, `subject: EntityRef`, non-empty `predicate`, exactly one of `object_id: EntityRef | None` and `value: str | None`, `kind`, tuple `evidence_ids`, optional `derivation`, and optional timezone-aware `valid_from`/`valid_until: datetime`; when both times exist, `valid_from < valid_until`.
- A `SOURCE` assertion requires one or more unique evidence IDs and no derivation. `RULE_DERIVED` and `MODEL_INFERRED` assertions require one derivation and no direct evidence IDs.
- `Derivation` explicitly carries a required `ScopeKey` `scope`, `conclusion_id`, non-empty unique `premise_ids`, optional `rule_id`/`rule_version`, and optional `model_version`/`prompt_version`. Exactly one complete version pair is required; the pair and scope must match the parent assertion.
- `validate_assertion(assertion, evidence_by_id: Mapping[str, ScopedEvidence], premises_by_id: Mapping[str, Assertion]) -> None` first requires the expected public record/mapping types and valid `ScopeKey` values; it then validates subject/object ref scopes, all reachable evidence/premise references, exact scope equality, map-key identity, version/kind consistency, and the complete reachable support DAG's acyclicity. It does not claim referential existence for entity IDs because C03's approved signature has no entity lookup map. Missing evidence/premise references and scope mismatches raise `ValueError`.

- [x] **Step 1: Write failing model and validation tests**

```python
def test_new_entity_id_is_uuid_and_entity_has_no_name_identity():
    entity_id = new_entity_id()
    assert str(uuid.UUID(entity_id)) == entity_id
    entity = Entity(entity_id, ScopeKey(1, "kb-a"), "component")
    assert entity.entity_id == entity_id

def test_assertion_requires_exactly_one_object_or_value():
    with pytest.raises(ValueError):
        make_assertion(object_id=None, value=None)
    with pytest.raises(ValueError):
        make_assertion(object_id=EntityRef(ENTITY_2, SCOPE), value="literal")

def test_source_assertion_retains_conflicting_values():
    first = source_assertion(assertion_id="a1", value="1", evidence_id="src-1")
    second = source_assertion(assertion_id="a2", value="2", evidence_id="src-2")
    validate_assertion(first, evidence_map("src-1"), {})
    validate_assertion(second, evidence_map("src-2"), {})
    assert first != second

def test_cross_scope_evidence_and_premises_are_rejected():
    assertion = source_assertion(scope=ScopeKey(1, "kb-a"), evidence_id="src-1")
    with pytest.raises(ValueError):
        validate_assertion(assertion, evidence_map("src-1", scope=ScopeKey(2, "kb-a")), {})
    derived = rule_assertion(scope=ScopeKey(1, "kb-a"), premise_id="p1")
    with pytest.raises(ValueError):
        validate_assertion(derived, {}, {"p1": source_assertion(assertion_id="p1", scope=ScopeKey(1, "kb-b"))})

def test_subject_and_object_refs_must_share_assertion_scope():
    foreign = ScopeKey(2, "kb-a")
    with pytest.raises(ValueError):
        make_assertion(subject=EntityRef(ENTITY_2, foreign))
    with pytest.raises(ValueError):
        make_assertion(object_id=EntityRef(ENTITY_2, foreign))

def test_derivation_carries_the_same_explicit_scope_as_its_assertion():
    derivation = Derivation(scope=OTHER_SCOPE, conclusion_id="r1", premise_ids=("p1",),
                            rule_id="depends_on_transitive", rule_version="v1")
    with pytest.raises(ValueError):
        make_assertion(kind=AssertionKind.RULE_DERIVED, evidence_ids=(),
                       derivation=derivation)

def test_every_fact_record_requires_a_scope_key():
    evidence = Evidence("e1", "d1", 1, "c1", "h", "quote", None, None)
    for invalid_scope in (None, "tenant-only"):
        with pytest.raises(ValueError):
            Entity(ENTITY_1, invalid_scope, "component")
        with pytest.raises(ValueError):
            EntityRef(ENTITY_1, invalid_scope)
        with pytest.raises(ValueError):
            ScopedEvidence(invalid_scope, evidence)
    with pytest.raises(ValueError):
        Derivation(scope=None, conclusion_id="r1", premise_ids=("p1",),
                   rule_id="depends_on_transitive", rule_version="v1")
    with pytest.raises(ValueError):
        make_assertion(scope=None)

def test_source_assertion_requires_resolvable_evidence():
    with pytest.raises(ValueError):
        validate_assertion(source_assertion(evidence_id="missing"), {}, {})

def test_dangling_and_cyclic_premises_are_rejected():
    dangling = rule_assertion(premise_id="missing")
    with pytest.raises(ValueError):
        validate_assertion(dangling, {}, {})
    cyclic = rule_assertion(assertion_id="a1", premise_id="a1")
    with pytest.raises(ValueError):
        validate_assertion(cyclic, {}, {"a1": cyclic})

def test_rule_and_model_derivations_require_matching_version_pairs():
    premise = source_assertion(assertion_id="p1")
    rule = rule_assertion(assertion_id="r1", premise_id="p1")
    validate_assertion(rule, evidence_map("src-1"), {"p1": premise})
    model_derivation = Derivation(scope=SCOPE, conclusion_id="m1", premise_ids=("p1",),
                                 model_version="model-v1", prompt_version="prompt-v1")
    model = make_assertion(assertion_id="m1", kind=AssertionKind.MODEL_INFERRED,
                           evidence_ids=(), derivation=model_derivation)
    validate_assertion(model, evidence_map("src-1"), {"p1": premise})
    with pytest.raises(ValueError):
        make_assertion(kind=AssertionKind.RULE_DERIVED, evidence_ids=(),
                       derivation=model_derivation)

def test_validity_window_requires_aware_increasing_times():
    with pytest.raises(ValueError):
        make_assertion(valid_from=datetime(2026, 1, 2))
    with pytest.raises(ValueError):
        make_assertion(valid_from=datetime(2026, 1, 2, tzinfo=timezone.utc),
                       valid_until=datetime(2026, 1, 1, tzinfo=timezone.utc))
```

Define these helpers in `test_facts.py` (keep them test-only):

```python
from datetime import datetime, timezone
import uuid

import pytest

from semantic_service.contracts import Evidence, ScopeKey
from semantic_service.facts import Assertion, AssertionKind, Derivation, Entity, EntityRef, ScopedEvidence, new_entity_id, validate_assertion

SCOPE = ScopeKey(1, "kb-a")
ENTITY_1 = "00000000-0000-4000-8000-000000000001"
ENTITY_2 = "00000000-0000-4000-8000-000000000002"

def make_assertion(**overrides):
    values = dict(
        assertion_id="a1", scope=SCOPE, subject=EntityRef(ENTITY_1, SCOPE),
        predicate="depends_on", object_id=EntityRef(ENTITY_2, SCOPE), value=None,
        kind=AssertionKind.SOURCE, evidence_ids=("src-1",), derivation=None,
        valid_from=None, valid_until=None,
    )
    values.update(overrides)
    return Assertion(**values)

def source_assertion(assertion_id="a1", scope=SCOPE, evidence_id="src-1", value="claimed"):
    subject = EntityRef(ENTITY_1, scope)
    return make_assertion(assertion_id=assertion_id, scope=scope, subject=subject,
                          object_id=None, value=value, kind=AssertionKind.SOURCE,
                          evidence_ids=(evidence_id,), derivation=None)

def rule_assertion(assertion_id="r1", scope=SCOPE, premise_id="p1"):
    derivation = Derivation(scope=scope, conclusion_id=assertion_id, premise_ids=(premise_id,),
                            rule_id="depends_on_transitive", rule_version="v1")
    return make_assertion(assertion_id=assertion_id, scope=scope,
                          kind=AssertionKind.RULE_DERIVED, evidence_ids=(),
                          derivation=derivation)

def evidence_map(evidence_id, scope=SCOPE):
    evidence = Evidence(evidence_id, "doc-1", 1, "chunk-1", "opaque-hash", "claimed", None, None)
    return {evidence_id: ScopedEvidence(scope=scope, evidence=evidence)}
```

- [x] **Step 2: Run RED**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_facts.py -q`

Expected: fail because `semantic_service.facts` does not exist.

- [x] **Step 3: Implement frozen domain records and validation**

Use immutable dataclasses and enums. Validate canonical UUID entity/entity-ref IDs, non-empty assertion identity/predicate/type, reference scopes, optional timezone-aware time-range ordering, exact object/value cardinality, fact-kind/derivation metadata, evidence and premise closure, and cycles. Do not merge conflicting source assertions or persist anything.

- [x] **Step 4: Run GREEN and the full C03 tests**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_evidence.py semantic/tests/test_facts.py semantic/tests/test_contracts.py -q`

Expected: all tests pass, including Chinese/emoji and invalid spans, bad hash/quote, cross-tenant and cross-KB references for subject/object/evidence/premises, missing source evidence, missing premises, mismatched derivation version pairs, naive or reversed validity windows, cyclic DAG, and coexistence of conflict.

- [x] **Step 5: Commit Task 2**

Commit `feat(semantic): model scoped semantic facts and derivations` with only Task 2 files.

---

## Final C03 Gate

- [x] Run: `uv run --locked --project semantic python -m pytest semantic/tests -q` — expected to pass all C02+C03 semantic service tests.
- [x] Run: `git diff --check` — expected exit 0.
- [ ] Request one fresh read-only review of the complete C03 range; fix Critical/Important findings with RED→GREEN tests and rerun the suite.
- [ ] Record exact base/head SHA, RED/GREEN commands, environment, review result, and limitations in `docs/plans/semantica/progress.md`; mark C03 `verified` only after review and tests pass.
