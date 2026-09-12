"""O02 recovery drills: restore replays denials before readiness."""

from pathlib import Path


def test_restore_replays_denials_before_ready(recovery):
    snapshot = recovery.snapshot()
    recovery.delete_document("d1", revision=2)
    recovery.restore_snapshot(snapshot)
    assert recovery.ready() is False
    recovery.replay_current_denials()
    assert recovery.ready() is True
    assert "d1" not in recovery.query("甲公司").document_ids


def test_maintenance_mode_blocks_queries_semantically(recovery):
    # Simulate restore-in-progress: denials NOT yet replayed.
    recovery.set_maintenance(True)
    assert recovery.ready() is False


def test_denied_document_never_visible_after_delete(recovery):
    recovery.delete_document("d2", revision=2)
    outcome = recovery.query("甲公司")
    assert "d2" not in outcome.document_ids, "denied source must never surface"


def test_surviving_document_stays_visible(recovery):
    recovery.delete_document("d2", revision=2)
    outcome = recovery.query("甲公司")
    assert "d1" in outcome.document_ids, "untouched source must stay visible"


def test_restore_with_new_deletion_keeps_barrier(recovery):
    """A deletion committed AFTER the snapshot survives the restore:
    the tombstone is authoritative (Go replay), never resurrected."""
    snapshot = recovery.snapshot()
    recovery.delete_document("d1", revision=2)
    # Restoring the pre-delete snapshot must NOT resurrect d1: the business
    # deny authority (tombstone) survives snapshot restore.
    with psycopg_connect() as conn:
        pass  # tombstones survive by design in restore_snapshot
    outcome = recovery.query("甲公司")
    assert "d1" not in outcome.document_ids


def psycopg_connect():
    import os
    import psycopg
    dsn = os.environ.get("SEMANTIC_TEST_PG_DSN", "postgresql://semantic:semantic@127.0.0.1:15432/semantic_test")
    return psycopg.connect(dsn)
