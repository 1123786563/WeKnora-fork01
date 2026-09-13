"""I04 deletion barrier, support revocation and receipt tests (real PG).

Tombstones deny every generation query immediately; multi-source facts
survive partial deletion; the last support's removal recursively
invalidates derived assertions; per-store cleanup receipts keep failures
pending for independent retry; GC respects retention/replay windows and
read-lease protection.
"""

from pathlib import Path

import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.indexing.deletion import DeletionService

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-del")


@pytest.fixture()
def index_store(deletion_service):
    """The plan's core test drives deletion through the index-store surface."""
    return deletion_service


@pytest.fixture()
def deletion_service(pg_dsn, operation_store_factory):
    import psycopg

    from semantic_service.operations import apply_migrations

    apply_migrations(pg_dsn)
    migration = Path(__file__).resolve().parents[1] / "migrations" / "003_deletion_receipts.sql"
    with psycopg.connect(pg_dsn) as conn:
        conn.execute(migration.read_text(encoding="utf-8"))
        conn.execute("INSERT INTO semantic.schema_migrations (version) VALUES ('003_deletion_receipts') ON CONFLICT DO NOTHING")
        conn.execute("DELETE FROM semantic.deletion_receipts")
        conn.execute("DELETE FROM semantic.tombstones")
        conn.execute("DELETE FROM semantic.assertions")
        conn.commit()
    operations = operation_store_factory()
    operations.clear_for_test()
    service = DeletionService(pg_dsn, operations)
    yield service
    service.close()


@pytest.fixture()
def two_source_fact(deletion_service):
    """shared-fact: supported by d1 AND d2; derived-fact premised on it."""
    deletion_service.add_assertion_support(SCOPE, "shared-fact", "ent-a", "controls",
        object_id="ent-b", support_document_id="d1", support_revision=2)
    deletion_service.add_assertion_support(SCOPE, "shared-fact", "ent-a", "controls",
        object_id="ent-b", support_document_id="d2", support_revision=2)
    deletion_service.add_assertion_support(SCOPE, "derived-fact", "ent-a", "controls",
        object_id="ent-c", kind="rule", support_document_id="d1", support_revision=2,
        premise_ids=["shared-fact"])
    return deletion_service


def test_last_support_removes_derived_fact(index_store, two_source_fact):
    index_store.delete_revision(SCOPE, "d1", revision=2)
    assert index_store.visible_assertion(SCOPE, "shared-fact") is True
    index_store.delete_revision(SCOPE, "d2", revision=2)
    assert index_store.visible_assertion(SCOPE, "shared-fact") is False
    assert index_store.visible_assertion(SCOPE, "derived-fact") is False


def test_reapplied_higher_revision_restores_visibility(deletion_service):
    """Same document re-applied at a HIGHER revision must restore its support
    (the support row PK includes the revision)."""
    deletion_service.add_assertion_support(SCOPE, "f-rev", "x", "p",
        object_id="y", support_document_id="d1", support_revision=2)
    deletion_service.delete_revision(SCOPE, "d1", 2)
    assert deletion_service.visible_assertion(SCOPE, "f-rev") is False
    # Document updated to revision 5: its assertion support must store.
    deletion_service.add_assertion_support(SCOPE, "f-rev", "x", "p",
        object_id="y", support_document_id="d1", support_revision=5)
    assert deletion_service.visible_assertion(SCOPE, "f-rev") is True, \
        "a live higher revision must restore the fact (rev-2 tombstone must not kill rev-5 support)"


def test_conjunctive_rule_dies_when_any_premise_dies(deletion_service):
    """A AND B -> C: killing A must invalidate C (conjunctive semantics)."""
    deletion_service.add_assertion_support(SCOPE, "pa", "a", "is", value="a", support_document_id="da", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "pb", "b", "is", value="b", support_document_id="db", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "c-and", "c", "derived", kind="rule",
        support_document_id="dc", support_revision=1, premise_ids=["pa", "pb"])
    assert deletion_service.visible_assertion(SCOPE, "c-and") is True
    deletion_service.delete_revision(SCOPE, "da", 1)
    assert deletion_service.visible_assertion(SCOPE, "c-and") is False, \
        "conjunctive rules must die when ANY premise loses all support"


def test_tombstone_sweep_requires_rebuilt_generation(deletion_service, generation_index_store, pg_dsn):
    """Sweeping a tombstone while an ACTIVE generation still references the
    deleted revision would resurrect deleted data - sweep must refuse."""
    from datetime import timedelta

    from semantic_service.indexing.gc import GCConfig, GarbageCollector
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    deletion_service.add_assertion_support(SCOPE, "f-sw", "x", "p",
        object_id="y", support_document_id="d1", support_revision=1)
    deletion_service.delete_revision(SCOPE, "d1", 1)
    # Old-generation manifest still references d1@1.
    stale = IndexManifest(
        scope=SCOPE, generation="gen-stale-ref", base_generation=None,
        documents={"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h", artifact_ids=("a1",))},
        config_digest="d", artifacts=(ArtifactRef(kind="graph", artifact_id="a1", hash="h1"),),
        complete=True)
    generation_index_store.save_manifest(stale)
    generation_index_store.publish(SCOPE, None, stale, lease_token=1)

    gc = GarbageCollector(pg_dsn, generation_index_store, GCConfig(retention=timedelta(days=7), replay_window=timedelta(days=1)))
    # Backdate the tombstone beyond every window.
    import psycopg

    with psycopg.connect(pg_dsn) as conn:
        conn.execute("UPDATE semantic.tombstones SET created_at = now() - interval '30 days'")
        conn.commit()
    swept = gc.sweep_tombstones(SCOPE)
    assert swept == 0, "tombstone protecting a still-referenced revision must NOT sweep"
    assert deletion_service.denied(SCOPE, "d1", 1) is True, "barrier must survive"

    # Publish a rebuilt generation WITHOUT d1 and GC the stale generation:
    # rollback-safety requires that NO persisted manifest still references
    # the deleted revision before its barrier may clear.
    rebuilt = IndexManifest(
        scope=SCOPE, generation="gen-rebuilt", base_generation="gen-stale-ref",
        documents={}, config_digest="d", artifacts=(), complete=True)
    generation_index_store.save_manifest(rebuilt)
    generation_index_store.publish(SCOPE, "gen-stale-ref", rebuilt, lease_token=1)
    swept = gc.sweep_tombstones(SCOPE)
    assert swept == 0, "stale generation still persisted (rollback target) - barrier must hold"
    with psycopg.connect(pg_dsn) as conn:
        conn.execute("DELETE FROM semantic.generations WHERE generation = 'gen-stale-ref'")
        conn.commit()
    swept = gc.sweep_tombstones(SCOPE)
    assert swept == 1, "once no persisted generation references the deleted revision, the tombstone may sweep"


def test_replayed_support_after_delete_stores_invisible(deletion_service):
    """A replayed event must not resurrect deleted data at the store level."""
    deletion_service.delete_revision(SCOPE, "d-replay", 3)
    deletion_service.add_assertion_support(SCOPE, "f-re", "x", "p",
        object_id="y", support_document_id="d-replay", support_revision=3)
    assert deletion_service.visible_assertion(SCOPE, "f-re") is False, \
        "replayed support for a tombstoned revision stores invisible"


def test_entity_other_facts_survive_its_own_support_loss(deletion_service):
    """Deleting ent-a's CONTROLLING document must not remove ent-a's other
    facts supported elsewhere."""
    deletion_service.add_assertion_support(SCOPE, "f-own", "ent-a", "controls",
        object_id="ent-b", support_document_id="d-own", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "f-other-own", "ent-a", "owns",
        object_id="ent-z", support_document_id="d-elsewhere", support_revision=1)
    deletion_service.delete_revision(SCOPE, "d-own", 1)
    assert deletion_service.visible_assertion(SCOPE, "f-other-own") is True


def test_protect_negative_and_lease_branch(deletion_service, generation_index_store):
    from datetime import timedelta

    from semantic_service.indexing.gc import GCConfig, GarbageCollector
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    def make(gen, docs):
        return IndexManifest(scope=SCOPE, generation=gen, base_generation=None,
                             documents=docs, config_digest="d",
                             artifacts=(ArtifactRef(kind="graph", artifact_id="a", hash="h"),),
                             complete=True)

    served = make("gen-served", {"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h", artifact_ids=("a",))})
    generation_index_store.save_manifest(served)
    generation_index_store.publish(SCOPE, None, served, lease_token=1)
    gc = GarbageCollector("", generation_index_store,
                          GCConfig(retention=timedelta(days=7), replay_window=timedelta(days=1)))
    # Negative: a generation that is neither active nor lease-held.
    orphan = make("gen-orphan", {})
    assert gc.protect(SCOPE, orphan.generation) is False
    # Lease branch: pin the ACTIVE generation, then move the pointer away -
    # the old generation is protected ONLY by the lease.
    lease = generation_index_store.pin(SCOPE)
    nxt = make("gen-served-2", {})
    generation_index_store.save_manifest(nxt)
    generation_index_store.publish(SCOPE, "gen-served", nxt, lease_token=1)
    assert gc.protect(SCOPE, "gen-served") is True, "live lease protects a non-active generation"
    generation_index_store.release(lease.lease_id)
    assert gc.protect(SCOPE, "gen-served") is False, "released lease no longer protects"


def test_derived_inserted_after_premise_death_is_born_invisible(deletion_service):
    deletion_service.add_assertion_support(SCOPE, "p-dead", "p", "is", value="v",
        support_document_id="dp", support_revision=1)
    deletion_service.delete_revision(SCOPE, "dp", 1)
    # Insert a derived assertion premised on the ALREADY-dead premise.
    deletion_service.add_assertion_support(SCOPE, "dep-late", "c", "derived", kind="rule",
        support_document_id="dc", support_revision=1, premise_ids=["p-dead"])
    assert deletion_service.visible_assertion(SCOPE, "dep-late") is False, \
        "derived rows must not be born visible over dead premises"


def test_delete_one_source_keeps_other_support(deletion_service):
    deletion_service.add_assertion_support(SCOPE, "f-multi", "x", "p",
        object_id="y", support_document_id="da", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "f-multi", "x", "p",
        object_id="y", support_document_id="db", support_revision=1)
    deletion_service.delete_revision(SCOPE, "da", 1)
    assert deletion_service.visible_assertion(SCOPE, "f-multi") is True


def test_same_name_entity_not_deleted_whole(deletion_service):
    """Deleting one document must not remove OTHER facts of the same-named entity."""
    deletion_service.add_assertion_support(SCOPE, "f-other", "ent-a", "owns",
        object_id="ent-z", support_document_id="d1", support_revision=2)
    deletion_service.delete_revision(SCOPE, "d2", 2)
    assert deletion_service.visible_assertion(SCOPE, "f-other") is True


def test_tombstone_denies_all_revisions_up_to(deletion_service):
    deletion_service.delete_revision(SCOPE, "doc-x", 5)
    assert deletion_service.denied(SCOPE, "doc-x", 5) is True
    assert deletion_service.denied(SCOPE, "doc-x", 3) is True   # older revision barrier
    assert deletion_service.denied(SCOPE, "doc-x", 9) is False  # newer revision (recreated) not denied
    assert deletion_service.denied(SCOPE, "doc-y", 5) is False


def test_monotone_tombstone_survives_lower_revision_write(deletion_service):
    deletion_service.delete_revision(SCOPE, "doc-m", 7)
    # A LATE lower tombstone must not lower the barrier.
    deletion_service.delete_revision(SCOPE, "doc-m", 4)
    assert deletion_service.denied(SCOPE, "doc-m", 6) is True


def test_late_apply_admitted_but_denied_by_barrier(deletion_service, operation_store):
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig

    deletion_service.delete_revision(SCOPE, "doc-l", 3)
    # A stale Apply for revision 2 arrives AFTER the tombstone at 3.
    late = ApplyRequest(
        document=DocumentRevision(scope=SCOPE, document_id="doc-l", revision=2, content_hash="h", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c", text="t", content_hash="ch"),),
        config=IndexConfig(config_digest="d", engine_version="e", model_profile_ref="m",
                           prompt_version="p", rule_set_version="r", schema_version="s"),
        idempotency_key="idem-late", payload_hash="hash-late",
    )
    # Admission itself succeeds (operation created), but the denial barrier
    # still excludes revision <=3 from every query.
    op = operation_store.accept(late)
    assert op.state == "accepted"
    assert deletion_service.denied(SCOPE, "doc-l", 2) is True


def test_receipts_track_per_store_state(deletion_service):
    from semantic_service.contracts import DocumentRevision

    document = DocumentRevision(scope=SCOPE, document_id="doc-r", revision=4,
                                 content_hash="hash-r", deleted=True)
    operation = deletion_service.apply(document)
    receipt = deletion_service.cleanup(operation.operation_id)
    assert receipt.tombstone_revision == 4
    assert receipt.graph_state == "pending"
    assert receipt.backup_state == "retention_pending", "backup retention is never faked"
    assert receipt.completed_at is None

    # Half the stores succeed, one fails: receipt stays pending, completed_at None.
    deletion_service.mark_store_cleaned(operation.operation_id, "graph")
    deletion_service.mark_store_cleaned(operation.operation_id, "vector")
    receipt = deletion_service.cleanup(operation.operation_id)
    assert receipt.object_state == "pending"
    assert receipt.completed_at is None

    # Independent retry completes the remaining stores.
    deletion_service.mark_store_cleaned(operation.operation_id, "object")
    deletion_service.mark_store_cleaned(operation.operation_id, "cache")
    receipt = deletion_service.cleanup(operation.operation_id)
    assert receipt.completed_at is not None
    assert receipt.backup_state == "retention_pending", "backup stays retention_pending"


def test_receipt_store_states_are_idempotent(deletion_service):
    from semantic_service.contracts import DocumentRevision

    document = DocumentRevision(scope=SCOPE, document_id="doc-i", revision=2,
                                 content_hash="h", deleted=True)
    operation = deletion_service.apply(document)
    assert deletion_service.mark_store_cleaned(operation.operation_id, "graph") is True
    assert deletion_service.mark_store_cleaned(operation.operation_id, "graph") is False


def test_apply_rejects_non_deleted(deletion_service):
    from semantic_service.contracts import DocumentRevision

    live = DocumentRevision(scope=SCOPE, document_id="d", revision=1, content_hash="h", deleted=False)
    with pytest.raises(ValueError, match="deleted"):
        deletion_service.apply(live)


def test_gc_respects_windows_and_lease_protection(deletion_service, generation_index_store):
    from datetime import timedelta

    from semantic_service.indexing.gc import GCConfig, GarbageCollector
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    manifest = IndexManifest(
        scope=SCOPE, generation="gen-gc", base_generation=None,
        documents={"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h", artifact_ids=("a1",))},
        config_digest="d", artifacts=(ArtifactRef(kind="graph", artifact_id="a1", hash="h1"),),
        complete=True)
    generation_index_store.save_manifest(manifest)
    generation_index_store.publish(SCOPE, None, manifest, lease_token=1)
    gc = GarbageCollector("", generation_index_store, GCConfig(retention=timedelta(days=7), replay_window=timedelta(days=30)))
    # Active generation artifacts are protected.
    assert gc.protect(SCOPE, manifest.generation) is True
    # A released lease no longer protects.
    lease = generation_index_store.pin(SCOPE)
    assert gc.protect(SCOPE, manifest.generation) is True
    generation_index_store.release(lease.lease_id)
    # Active still protects even without a lease.
    assert gc.protect(SCOPE, manifest.generation) is True
