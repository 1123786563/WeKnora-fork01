"""I03 generation publish tests against the REAL service PostgreSQL.

Incomplete manifests never become active; CAS publishes swap atomically;
concurrent same-base publishes yield exactly one winner; read leases pin
generations; persisted manifests survive restart (new store instance).
"""

from dataclasses import replace

import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest
from semantic_service.indexing.publisher import GenerationConflict, Publisher
from semantic_service.indexing.store import IndexStore

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-gen")


def make_manifest(generation="gen-1", complete=True):
    return IndexManifest(
        scope=SCOPE,
        generation=generation,
        base_generation=None,
        documents={"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h1", artifact_ids=("art-1",))},
        config_digest="digest-1",
        artifacts=(ArtifactRef(kind="graph", artifact_id="art-1", hash="hash-1"),),
        complete=complete,
    )


def make_publish_request(idem="idem-pub", payload="hash-pub"):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig
    return ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="d1", revision=1, content_hash="h1", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="text", content_hash="ch1"),),
        config=IndexConfig(config_digest="digest-1", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key=idem, payload_hash=payload,
    )


def test_publishing_operation_with_expired_lease_is_reclaimable(operation_store):
    """A worker crash mid-publishing must not strand the operation forever."""
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig

    request = ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="d9", revision=1, content_hash="h9", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c9", text="t", content_hash="ch9"),),
        config=IndexConfig(config_digest="d", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-stuck", payload_hash="hash-stuck",
    )
    operation_store.accept(request)
    claimed = operation_store.claim(worker_id="w1", lease_seconds=1)
    operation_store.transition(claimed.operation_id, claimed.lease_token, "running", "staged")
    operation_store.transition(claimed.operation_id, claimed.lease_token, "staged", "publishing")
    import time as _time

    _time.sleep(1.5)
    operation_store.expire_lease_for_test(claimed.operation_id)
    recovered = operation_store.claim(worker_id="w2", lease_seconds=30)
    assert recovered is not None, "a publishing operation with an expired lease MUST be reclaimable"
    assert recovered.operation_id == claimed.operation_id
    assert recovered.lease_token == claimed.lease_token + 1


def test_staged_operation_with_expired_lease_is_reclaimable(operation_store):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig

    request = ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="d8", revision=1, content_hash="h8", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c8", text="t", content_hash="ch8"),),
        config=IndexConfig(config_digest="d", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-staged", payload_hash="hash-staged",
    )
    operation_store.accept(request)
    claimed = operation_store.claim(worker_id="w1", lease_seconds=30)
    operation_store.transition(claimed.operation_id, claimed.lease_token, "running", "staged")
    operation_store.expire_lease_for_test(claimed.operation_id)
    recovered = operation_store.claim(worker_id="w2", lease_seconds=30)
    assert recovered is not None, "staged + expired lease must be reclaimable"
    assert recovered.lease_token == claimed.lease_token + 1


def test_publisher_conflict_marks_operation_superseded(index_store, operation_store, complete_manifest):
    request = make_publish_request(idem="idem-sup", payload="hash-sup")
    operation_store.accept(request)
    claimed = operation_store.claim(worker_id="w", lease_seconds=60)
    operation_store.transition(claimed.operation_id, claimed.lease_token, "running", "staged")
    publisher = Publisher(index_store, operation_store)
    other = make_manifest(generation="gen-sup-winner")
    index_store.save_manifest(other)
    index_store.publish(SCOPE, None, other, lease_token=1)
    import pytest as _pytest

    with _pytest.raises(GenerationConflict):
        publisher.publish(SCOPE, None, complete_manifest, claimed.lease_token, operation_id=claimed.operation_id)
    final = operation_store.get(SCOPE, claimed.operation_id)
    assert final.state == "superseded", "CAS loss means someone else published; superseded carries that signal"


def test_save_manifest_conflict_on_divergent_payload(index_store, complete_manifest):
    from dataclasses import replace as _replace

    index_store.save_manifest(complete_manifest)
    divergent = _replace(complete_manifest, config_digest="tampered")
    import pytest as _pytest

    with _pytest.raises(ValueError, match="divergent"):
        index_store.save_manifest(divergent)


def test_incomplete_generation_never_becomes_active(index_store, complete_manifest):
    before = index_store.active(complete_manifest.scope)
    incomplete = replace(complete_manifest, complete=False)
    with pytest.raises(ValueError):
        index_store.publish(incomplete.scope, before, incomplete, lease_token=1)
    assert index_store.active(incomplete.scope) == before


def test_complete_publish_swaps_active(index_store, complete_manifest):
    index_store.save_manifest(complete_manifest)
    assert index_store.publish(SCOPE, None, complete_manifest, lease_token=1) is True
    assert index_store.active(SCOPE) == complete_manifest.generation


def test_cas_conflict_when_base_moved(index_store, complete_manifest):
    index_store.save_manifest(complete_manifest)
    index_store.publish(SCOPE, None, complete_manifest, lease_token=1)
    # A different publisher moves the active pointer away first.
    winner = make_manifest(generation="gen-winner")
    index_store.save_manifest(winner)
    assert index_store.publish(SCOPE, complete_manifest.generation, winner, lease_token=1) is True
    # Now the stale publish with the outdated base must lose the CAS.
    stale = make_manifest(generation="gen-stale")
    index_store.save_manifest(stale)
    assert index_store.publish(SCOPE, complete_manifest.generation, stale, lease_token=1) is False
    assert index_store.active(SCOPE) == "gen-winner"


def test_first_publish_wins_when_no_base(index_store):
    """base=None semantics: INSERT ON CONFLICT - exactly one winner."""
    a = make_manifest(generation="gen-a")
    b = make_manifest(generation="gen-b")
    index_store.save_manifest(a)
    index_store.save_manifest(b)
    results = [index_store.publish(SCOPE, None, m, lease_token=1) for m in (a, b)]
    assert sorted(results) == [False, True]
    assert index_store.active(SCOPE) in {"gen-a", "gen-b"}


def test_real_thread_same_base_cas_single_winner(index_store, complete_manifest):
    """Two THREADS racing the same-base UPDATE CAS: exactly one winner."""
    import threading

    index_store.save_manifest(complete_manifest)
    index_store.publish(SCOPE, None, complete_manifest, lease_token=1)
    a = make_manifest(generation="gen-ta")
    b = make_manifest(generation="gen-tb")
    index_store.save_manifest(a)
    index_store.save_manifest(b)
    barrier = threading.Barrier(2)
    winners = []

    def contender(manifest):
        barrier.wait()
        if index_store.publish(SCOPE, complete_manifest.generation, manifest, lease_token=1):
            winners.append(manifest.generation)

    threads = [threading.Thread(target=contender, args=(m,)) for m in (a, b)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert len(winners) == 1, f"exactly one same-base CAS winner, got {winners}"
    assert index_store.active(SCOPE) == winners[0]


def test_pin_during_swap_keeps_old_generation_readable(index_store, complete_manifest):
    """A lease taken before the swap still pins the OLD generation."""
    index_store.save_manifest(complete_manifest)
    index_store.publish(SCOPE, None, complete_manifest, lease_token=1)
    lease = index_store.pin(SCOPE)
    assert lease.generation == complete_manifest.generation
    nxt = make_manifest(generation="gen-next")
    index_store.save_manifest(nxt)
    assert index_store.publish(SCOPE, complete_manifest.generation, nxt, lease_token=1) is True
    assert index_store.active(SCOPE) == "gen-next"
    assert index_store.lease_holds(SCOPE, complete_manifest.generation) is True
    assert index_store.load_manifest(SCOPE, lease.generation) == complete_manifest


def test_read_lease_pins_generation(index_store, complete_manifest):
    index_store.save_manifest(complete_manifest)
    index_store.publish(SCOPE, None, complete_manifest, lease_token=1)
    lease = index_store.pin(SCOPE)
    assert lease.generation == complete_manifest.generation
    assert index_store.lease_holds(SCOPE, complete_manifest.generation) is True
    index_store.release(lease.lease_id)
    assert index_store.lease_holds(SCOPE, complete_manifest.generation) is False


def test_manifest_survives_restart(index_store, complete_manifest, service_pg_dsn):
    index_store.save_manifest(complete_manifest)
    restarted = IndexStore(service_pg_dsn)
    loaded = restarted.load_manifest(SCOPE, complete_manifest.generation)
    assert loaded == complete_manifest


def test_builder_stage_rejects_scope_mismatch(operation_store):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig, Operation, ScopeKey
    from semantic_service.indexing.builder import IndexBuilder

    other_scope = ScopeKey(tenant_id=99, kb_id="kb-other")
    request = ApplyRequest(
        document=DocumentRevision(scope=other_scope, document_id="d1", revision=1, content_hash="h1", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="t", content_hash="ch1"),),
        config=IndexConfig(config_digest="d", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-b1", payload_hash="hash-b1",
    )
    op = Operation(operation_id="op-b1", scope=SCOPE, document_id="d1", revision=1)
    builder = IndexBuilder(None, extractor=None)
    import pytest as _pytest

    with _pytest.raises(ValueError, match="scope"):
        builder.stage(op, request)


def test_builder_stage_inherits_base_and_mints_new_artifacts(operation_store):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig, Operation
    from semantic_service.indexing.builder import IndexBuilder
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    class StubExtractor:
        def extract(self, scope, request):
            return (ArtifactRef(kind="graph", artifact_id="art-new", hash="hash-new"),)

    base = IndexManifest(
        scope=SCOPE, generation="gen-base", base_generation=None,
        documents={"d-old": DocumentEntry(document_id="d-old", revision=1, content_hash="h-old", artifact_ids=("art-old",))},
        complete=True,
    )
    request = ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="d-new", revision=2, content_hash="h-new", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="t", content_hash="ch1"),),
        config=IndexConfig(config_digest="d2", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-b2", payload_hash="hash-b2",
    )
    op = Operation(operation_id="op-b2", scope=SCOPE, document_id="d-new", revision=2)
    builder = IndexBuilder(None, extractor=StubExtractor())
    staged = builder.stage(op, request, base_manifest=base)
    assert staged.complete is False
    assert staged.base_generation == "gen-base"
    assert staged.documents["d-old"].artifact_ids == ("art-old",), "unchanged docs inherit existing artifacts"
    assert staged.documents["d-new"].artifact_ids == ("art-new",), "changed doc gets NEW artifacts"
    assert staged.documents["d-new"].revision == 2


def test_builder_stage_rejects_empty_artifacts_for_live_document(operation_store):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig, Operation
    from semantic_service.indexing.builder import IndexBuilder

    class EmptyExtractor:
        def extract(self, scope, request):
            return ()

    request = ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="d-e", revision=1, content_hash="h", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="t", content_hash="ch1"),),
        config=IndexConfig(config_digest="d", engine_version="e", model_profile_ref="m", prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-b3", payload_hash="hash-b3",
    )
    op = Operation(operation_id="op-b3", scope=SCOPE, document_id="d-e", revision=1)
    builder = IndexBuilder(None, extractor=EmptyExtractor())
    import pytest as _pytest

    with _pytest.raises(ValueError, match="artifact"):
        builder.stage(op, request)


def test_publisher_success_marks_operation_succeeded(index_store, operation_store, complete_manifest):
    request = make_publish_request(idem="idem-ok", payload="hash-ok")
    operation_store.accept(request)
    claimed = operation_store.claim(worker_id="w", lease_seconds=60)
    operation_store.transition(claimed.operation_id, claimed.lease_token, "running", "staged")
    publisher = Publisher(index_store, operation_store)
    assert publisher.publish(SCOPE, None, complete_manifest, claimed.lease_token, operation_id=claimed.operation_id) is True
    final = operation_store.get(SCOPE, claimed.operation_id)
    assert final.state == "succeeded"
    assert final.result_generation == complete_manifest.generation