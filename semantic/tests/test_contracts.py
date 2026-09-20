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


def test_operation_ref_requires_scope_and_operation_id() -> None:
    from semantic_service.contracts import OperationRef, ScopeKey

    assert OperationRef(scope=ScopeKey(tenant_id=1, kb_id="kb"), operation_id="op").operation_id == "op"


def test_operation_carries_scope_and_revision() -> None:
    from semantic_service.contracts import Operation, ScopeKey

    operation = Operation(operation_id="op", scope=ScopeKey(tenant_id=1, kb_id="kb"), document_id="doc", revision=1, state="running", stage="index")

    assert operation.scope.kb_id == "kb" and operation.revision == 1


def test_search_request_preserves_requested_mode_and_limits() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ScopeKey, SearchRequest

    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, audience="semantic", purpose="search", budget_ref="budget")
    request = SearchRequest(query_id="q", query="问题", access_scope=scope, limits=QueryLimits(max_hops=2, max_nodes=10, max_edges=20, top_k=3, max_tokens=100, deadline_ms=1000), requested_mode="graph_rag")

    assert request.requested_mode == "graph_rag" and request.limits.max_hops == 2


def test_reason_request_embeds_search_and_limits_reasoning_mode() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ReasonRequest, ScopeKey, SearchRequest

    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, audience="semantic", purpose="reason", budget_ref="budget")
    search = SearchRequest(query_id="q", query="问题", access_scope=scope, limits=QueryLimits(2, 10, 20, 3, 100, 1000), requested_mode="reason")
    request = ReasonRequest(search=search, reasoning_mode="rules", rule_set_version="v1")

    assert request.search is search and request.reasoning_mode == "rules"


def test_search_response_echoes_modes_and_evidence() -> None:
    from semantic_service.contracts import Evidence, SearchResponse

    evidence = Evidence("e", "d", 1, "c", "h", "文本", None, None)
    response = SearchResponse(query_id="q", generation="g", requested_mode="graph_rag", actual_mode="graph_rag", evidence=(evidence,), assertion_ids=("a",))

    assert response.actual_mode == "graph_rag" and response.evidence[0].evidence_id == "e"


def test_reason_response_keeps_retrieval_without_conclusion() -> None:
    from semantic_service.contracts import Evidence, ReasonResponse, ReasonStatus, SearchResponse

    retrieval = SearchResponse("q", "g", "reason", "reason", (Evidence("e", "d", 1, "c", "h", "文本", None, None),), ("a",))
    response = ReasonResponse(status=ReasonStatus.INSUFFICIENT_EVIDENCE, conclusion=None, retrieval=retrieval, premise_ids=(), rule_ids=(), limitations=("missing",))

    assert response.conclusion is None and response.retrieval is retrieval
