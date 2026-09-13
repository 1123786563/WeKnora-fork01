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
    # Restore-in-progress: denials NOT yet replayed - queries REFUSED.
    recovery.set_maintenance(True)
    assert recovery.ready() is False
    import pytest
    with pytest.raises(RuntimeError, match="maintenance"):
        recovery.query("甲公司")


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
    the tombstone is authoritative, never resurrected."""
    snapshot = recovery.snapshot()
    recovery.delete_document("d1", revision=2)
    # ACTUALLY restore the pre-delete assertion snapshot: the tombstone
    # (deny authority) survives, so the restore must NOT resurrect d1.
    recovery.restore_snapshot(snapshot)
    recovery.replay_current_denials()
    assert recovery.ready() is True
    outcome = recovery.query("甲公司")
    assert "d1" not in outcome.document_ids, "post-snapshot deletion survives restore"
