"""Q01 GraphRAG search tests against the REAL store (generation pin,
read lease, authorized subgraph, truncation, source validation)."""

from dataclasses import replace
from pathlib import Path

import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.indexing.deletion import DeletionService
from semantic_service.indexing.manifest import IndexManifest  # noqa: F401 (fixture parity)
from semantic_service.pg_access import PgAccessGraph
from semantic_service.query.adapter import FrozenGraphRAGAdapter
from semantic_service.query.search import (AccessScope, SearchLimits, SearchRequest,
                                           SearchService, UnsupportedMode)

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-q01")


@pytest.fixture()
def search_service(pg_dsn, deletion_service, generation_index_store, published_generation, search_stack):
    graph = PgAccessGraph(pg_dsn, SCOPE, deletion_service)
    return SearchService(generation_index_store, graph, FrozenGraphRAGAdapter())


@pytest.fixture()
def scoped_search_request():
    return SearchRequest(
        access_scope=AccessScope(scope=SCOPE, allowed_document_ids=["d1", "d2"]),
        query="ent-a",
    )


def test_search_reports_truncation(search_service, scoped_search_request):
    request = replace(scoped_search_request,
        limits=replace(scoped_search_request.limits, max_nodes=1, max_evidence=1))
    result = search_service.search(request)
    assert result.truncated is True
    assert result.generation
    assert result.evidence, "evidence must trace to authorized documents"
    assert all(e.document_id in {"d1", "d2"} for e in result.evidence)


def test_search_pins_generation_and_returns_evidence(search_service, scoped_search_request):
    result = search_service.search(scoped_search_request)
    assert result.generation == "gen-q01"
    assert result.mode == "graphrag"
    assert result.truncated is False
    assert set(result.assertion_ids) >= {"f1", "f2"}


def test_search_releases_lease_afterwards(search_service, scoped_search_request, generation_index_store):
    search_service.search(scoped_search_request)
    # No lingering read leases for this scope.
    assert generation_index_store.lease_holds(SCOPE, "gen-q01") is False


def test_unsupported_mode_fails_precondition(search_service, scoped_search_request):
    request = replace(scoped_search_request, mode="magic-full-store")
    with pytest.raises(UnsupportedMode):
        search_service.search(request)


def test_source_outside_graph_rejected(search_service, scoped_search_request):
    from semantic_service.query.search import RankedEvidence

    class LeakyAdapter:
        def retrieve(self, graph, query, limits):
            return RankedEvidence(evidence_ids=[], assertion_ids=["not-in-graph"], paths=[])

    search_service._adapter = LeakyAdapter()
    with pytest.raises(ValueError, match="outside the authorized graph"):
        search_service.search(scoped_search_request)


def test_max_evidence_boundary_exact_limit_not_truncated(search_service, scoped_search_request):
    """Reaching exactly max_evidence with nothing left over is NOT truncation."""
    result_all = search_service.search(scoped_search_request)
    request = replace(scoped_search_request,
        limits=replace(scoped_search_request.limits,
                       max_evidence=len(result_all.evidence) or 1))
    result = search_service.search(request)
    assert len(result.evidence) <= request.limits.max_evidence
    if len(result.evidence) == request.limits.max_evidence:
        assert result.truncated is False or result_all.truncated is True, \
            "exact-fit evidence must not be flagged truncated by the cap"


def test_evidence_dedup_and_stable_order(search_service, scoped_search_request):
    """Evidence is deduped by (document, chunk, hash) and sorted stably."""
    result = search_service.search(scoped_search_request)
    keys = [(e.document_id, e.chunk_id, e.content_hash) for e in result.evidence]
    assert len(keys) == len(set(keys)), "duplicate evidence keys leaked through"
    assert keys == sorted(keys), "evidence order is not the stable sort"


def test_disallowed_evidence_rejected(search_service, scoped_search_request):
    """Adapter evidence for a document outside the allowed set is REJECTED."""
    from semantic_service.query.search import RankedEvidence

    class SmugglerAdapter:
        def retrieve(self, graph, query, limits):
            return RankedEvidence(
                evidence_ids=["dX:chunk-smuggled:1"],
                assertion_ids=list(graph.assertions)[:1] or [],
                paths=[],
            )

    service = search_service
    service._adapter = SmugglerAdapter()
    import pytest as _pytest

    with _pytest.raises(ValueError, match="outside the allowed documents"):
        service.search(scoped_search_request)


def test_hidden_documents_never_in_evidence(search_service, scoped_search_request, deletion_service):
    deletion_service.add_assertion_support(SCOPE, "f-hidden", "ent-x", "p",
        object_id="ent-y", support_document_id="dH", support_revision=1)
    deletion_service.delete_revision(SCOPE, "dH", 1)
    result = search_service.search(scoped_search_request)
    assert "f-hidden" not in result.assertion_ids
