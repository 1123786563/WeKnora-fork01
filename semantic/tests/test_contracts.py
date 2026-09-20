from __future__ import annotations


def test_reason_without_conclusion_is_representable() -> None:
    from semantic_service.contracts import ReasonResponse, ReasonStatus

    result = ReasonResponse(status=ReasonStatus.INSUFFICIENT_EVIDENCE, conclusion=None)

    assert result.conclusion is None


def test_delete_revision_requires_deleted_flag() -> None:
    from semantic_service.contracts import DocumentRevision, ScopeKey

    try:
        DocumentRevision(scope=ScopeKey(tenant_id=1, kb_id="kb"), document_id="doc", revision=1, content_hash="hash", deleted=False).require_delete()
    except ValueError as error:
        assert "deleted" in str(error)
    else:
        raise AssertionError("expected deleted validation")


def test_scope_and_revision_reject_uint64_overflow() -> None:
    from semantic_service.contracts import DocumentRevision, ScopeKey

    try:
        ScopeKey(tenant_id=-1, kb_id="kb")
    except ValueError:
        pass
    else:
        raise AssertionError("expected tenant uint64 validation")
    try:
        DocumentRevision(scope=ScopeKey(tenant_id=1, kb_id="kb"), document_id="doc", revision=2**64, content_hash="hash", deleted=True)
    except ValueError:
        pass
    else:
        raise AssertionError("expected revision uint64 validation")


def test_delete_request_rejects_non_tombstone_revision() -> None:
    from semantic_service.contracts import DeleteDocumentRequest, DocumentRevision, ScopeKey

    revision = DocumentRevision(scope=ScopeKey(tenant_id=1, kb_id="kb"), document_id="doc", revision=1, content_hash="hash", deleted=False)
    try:
        DeleteDocumentRequest.from_revision(revision)
    except ValueError as error:
        assert "deleted" in str(error)
    else:
        raise AssertionError("expected delete boundary validation")


def test_evidence_preserves_absent_span_and_access_scope_identity() -> None:
    from semantic_service.contracts import AccessScope, Evidence, ScopeKey

    evidence = Evidence(evidence_id="e", document_id="d", revision=1, chunk_id="c", content_hash="h", quote="文本", start_char=None, end_char=None)
    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, audience="semantic", purpose="search", budget_ref="budget")

    assert evidence.start_char is None and evidence.end_char is None
    assert scope.scope.tenant_id == 1 and scope.permission_epoch == 2
