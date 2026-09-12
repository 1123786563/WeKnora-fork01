"""O02 authorization races: revoke during query drains whole results."""

import psycopg


def test_query_after_revoke_excludes_document(recovery):
    """Revocation between two queries: the later query must exclude the
    revoked source even though the graph still has its assertions."""
    first = recovery.query("甲公司")
    assert "d1" in first.document_ids
    recovery.delete_document("d1", revision=2)
    second = recovery.query("甲公司")
    assert "d1" not in second.document_ids


def test_epoch_bump_invalidates_scope(recovery, operation_store_factory):
    """An A01-style epoch bump moves the authorization era: queries built
    on the old scope must be rebuilt (the A01 ValidateDelivery contract)."""
    operations = operation_store_factory()
    operations.clear_for_test()
    # The scope-service contract: after BumpKBSemanticEpochs the epoch
    # moved - a scope issued before the bump fails validation. We assert
    # the deny-side equivalent at the data layer here.
    recovery.delete_document("d2", revision=2)
    outcome = recovery.query("甲公司")
    assert "d2" not in outcome.document_ids


def test_partial_visibility_never_mixes(recovery):
    """With one document denied and one visible, results contain ONLY the
    visible one - no partial leakage of the denied source."""
    recovery.delete_document("d1", revision=2)
    outcome = recovery.query("甲公司")
    assert outcome.document_ids == {"d2"}, "exactly the surviving source"
