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
    for invalid in (True, 1.5):
        try:
            ScopeKey(tenant_id=invalid, kb_id="kb")
        except ValueError:
            pass
        else:
            raise AssertionError("expected strict integer validation")


def test_delete_request_rejects_non_tombstone_revision() -> None:
    from semantic_service.contracts import DeleteDocumentRequest, DocumentRevision, ScopeKey

    revision = DocumentRevision(scope=ScopeKey(tenant_id=1, kb_id="kb"), document_id="doc", revision=1, content_hash="hash", deleted=False)
    try:
        DeleteDocumentRequest.from_revision(revision)
    except ValueError as error:
        assert "deleted" in str(error)
    else:
        raise AssertionError("expected delete boundary validation")
    try:
        DeleteDocumentRequest(revision)
    except ValueError:
        pass
    else:
        raise AssertionError("expected direct delete boundary validation")


def test_delete_wire_round_trip_preserves_tombstone() -> None:
    from semantic_service.contracts import DeleteDocumentRequest, DocumentRevision, ScopeKey, delete_request_from_wire, delete_request_to_wire
    value = DeleteDocumentRequest(DocumentRevision(ScopeKey(1, "kb"), "doc", 1, "hash", True))
    assert delete_request_from_wire(delete_request_to_wire(value)) == value


def test_evidence_preserves_absent_span_and_access_scope_identity() -> None:
    from semantic_service.contracts import AccessScope, Evidence, ScopeKey

    evidence = Evidence(evidence_id="e", document_id="d", revision=1, chunk_id="c", content_hash="h", quote="文本", start_char=None, end_char=None)
    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, expires_at="2026-09-20T00:00:00Z", audience="semantic", purpose="search", budget_ref="budget")

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

    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, expires_at="2026-09-20T00:00:00Z", audience="semantic", purpose="search", budget_ref="budget")
    request = SearchRequest(query_id="q", query="问题", access_scope=scope, limits=QueryLimits(max_hops=2, max_nodes=10, max_edges=20, top_k=3, max_tokens=100, deadline_ms=1000), requested_mode="graph_rag")

    assert request.requested_mode == "graph_rag" and request.limits.max_hops == 2


def test_search_request_rejects_incompatible_scope_purpose() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ScopeKey, SearchRequest
    scope = AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, "2026-09-20T00:00:00Z", "semantic", "reason", "budget")
    try:
        SearchRequest("q", "question", scope, QueryLimits(1, 1, 1, 1, 1, 1), "graph_rag")
    except ValueError:
        pass
    else:
        raise AssertionError("expected incompatible scope purpose rejection")


def test_reason_request_embeds_search_and_limits_reasoning_mode() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ReasonRequest, ScopeKey, SearchRequest

    scope = AccessScope(scope=ScopeKey(tenant_id=1, kb_id="kb"), subject_id="user", scope_ref="ref", scope_hash="hash", permission_epoch=2, expires_at="2026-09-20T00:00:00Z", audience="semantic", purpose="reason", budget_ref="budget")
    search = SearchRequest(query_id="q", query="问题", access_scope=scope, limits=QueryLimits(2, 10, 20, 3, 100, 1000), requested_mode="reason")
    request = ReasonRequest(search=search, reasoning_mode="rules", rule_set_version="v1")

    assert request.search is search and request.reasoning_mode == "rules"


def test_reason_request_rejects_search_scoped_graph_rag() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ReasonRequest, ScopeKey, SearchRequest
    scope = AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, "2026-09-20T00:00:00Z", "semantic", "search", "budget")
    search = SearchRequest("q", "question", scope, QueryLimits(1, 1, 1, 1, 1, 1), "graph_rag")
    try:
        ReasonRequest(search, "rules", "v1")
    except ValueError:
        pass
    else:
        raise AssertionError("expected reason scope rejection")


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


def test_evidence_wire_round_trip_preserves_absent_span() -> None:
    from semantic_service.contracts import Evidence, evidence_from_wire, evidence_to_wire

    evidence = Evidence("e", "d", 1, "c", "h", "文本", None, None)

    assert evidence_from_wire(evidence_to_wire(evidence)) == evidence


def test_access_scope_wire_round_trip_preserves_expiry() -> None:
    from semantic_service.contracts import AccessScope, ScopeKey, access_scope_from_wire, access_scope_to_wire

    scope = AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, "2026-09-20T00:00:00Z", "semantic", "search", "budget")
    assert access_scope_from_wire(access_scope_to_wire(scope)) == scope


def test_access_scope_rejects_invalid_expiry_and_empty_budget() -> None:
    from semantic_service.contracts import AccessScope, ScopeKey
    for expires_at, budget_ref in (("not-a-time", "budget"), ("2026-09-20T00:00:00Z", "")):
        try:
            AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, expires_at, "semantic", "search", budget_ref)
        except ValueError:
            pass
        else:
            raise AssertionError("expected access scope validation")


def test_operation_wire_round_trip_preserves_state() -> None:
    from semantic_service.contracts import Operation, ScopeKey, operation_from_wire, operation_to_wire

    value = Operation("op", ScopeKey(1, "kb"), "doc", 1, "running", "index", 2, "g", None)
    assert operation_from_wire(operation_to_wire(value)) == value


def test_operation_ref_wire_round_trip_preserves_scope() -> None:
    from semantic_service.contracts import OperationRef, ScopeKey, operation_ref_from_wire, operation_ref_to_wire

    value = OperationRef(ScopeKey(1, "kb"), "op")
    assert operation_ref_from_wire(operation_ref_to_wire(value)) == value


def test_search_response_wire_round_trip_preserves_modes() -> None:
    from semantic_service.contracts import Evidence, SearchResponse, search_response_from_wire, search_response_to_wire

    value = SearchResponse("q", "g", "graph_rag", "graph_rag", (Evidence("e", "d", 1, "c", "h", "文本", None, None),), ("a",))
    assert search_response_from_wire(search_response_to_wire(value)) == value


def test_reason_response_wire_round_trip_preserves_absent_conclusion() -> None:
    from semantic_service.contracts import ReasonResponse, ReasonStatus, reason_response_from_wire, reason_response_to_wire

    value = ReasonResponse(ReasonStatus.INSUFFICIENT_EVIDENCE, None, None, None, (), (), None, None, ("missing",))
    assert reason_response_from_wire(reason_response_to_wire(value)) == value


def test_search_request_wire_round_trip_preserves_scope_and_limits() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ScopeKey, SearchRequest, search_request_from_wire, search_request_to_wire

    scope = AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, "2026-09-20T00:00:00Z", "semantic", "search", "budget")
    value = SearchRequest("q", "问题", scope, QueryLimits(2, 10, 20, 3, 100, 1000), "graph_rag")
    assert search_request_from_wire(search_request_to_wire(value)) == value


def test_reason_request_wire_round_trip_preserves_rule_version() -> None:
    from semantic_service.contracts import AccessScope, QueryLimits, ReasonRequest, ScopeKey, SearchRequest, reason_request_from_wire, reason_request_to_wire

    scope = AccessScope(ScopeKey(1, "kb"), "user", "ref", "hash", 2, "2026-09-20T00:00:00Z", "semantic", "reason", "budget")
    value = ReasonRequest(SearchRequest("q", "问题", scope, QueryLimits(2, 10, 20, 3, 100, 1000), "reason"), "rules", "v1")
    assert reason_request_from_wire(reason_request_to_wire(value)) == value


def test_apply_request_wire_round_trip_preserves_chunks_and_config() -> None:
    from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig, ScopeKey, apply_request_from_wire, apply_request_to_wire

    revision = DocumentRevision(ScopeKey(1, "kb"), "doc", 1, "hash", False)
    config = IndexConfig("digest", "engine", "profile", "prompt", "rules", "schema")
    value = ApplyRequest(revision, (ChunkSnapshot("c", "text", "chunk-hash"),), config, "idem", "payload", None)
    assert apply_request_from_wire(apply_request_to_wire(value)) == value


def test_capabilities_wire_round_trip_preserves_availability() -> None:
    from semantic_service.contracts import Capabilities, QueryLimits, capabilities_from_wire, capabilities_to_wire

    value = Capabilities("v1", "engine", ("graph_rag",), ("rules",), QueryLimits(1, 2, 3, 4, 5, 6), ("limit",), None)
    assert capabilities_from_wire(capabilities_to_wire(value)) == value
