"""Q01 RPC-level tests: the Search RPC through the REAL gRPC servicer."""

import grpc
import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.pg_access import PgAccessGraph
from semantic_service.proto import semantic_pb2
from semantic_service.query.adapter import FrozenGraphRAGAdapter
from semantic_service.query.search import SearchService
from semantic_service.servicer import SemanticServicer

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-q01")


class FakeContext:
    def __init__(self):
        self.code = None
        self.details = None

    def abort(self, code, details):
        self.code = code
        self.details = details
        raise RuntimeError(f"aborted: {code}")


@pytest.fixture()
def servicer(pg_dsn, deletion_service, generation_index_store, published_generation, search_stack):
    """Per-request factory mirroring the server wiring."""
    def factory(scope):
        graph = PgAccessGraph(pg_dsn, scope, deletion_service)
        return SearchService(generation_index_store, graph, FrozenGraphRAGAdapter())
    return SemanticServicer(factory)


def make_request(mode="graphrag", allowed=("d1", "d2")):
    return semantic_pb2.SearchRequest(
        query_id="q-1",
        query="ent-a",
        mode=mode,
        access_scope=semantic_pb2.AccessScope(
            scope=semantic_pb2.ScopeKey(tenant_id=SCOPE.tenant_id, kb_id=SCOPE.kb_id),
            allowed_document_ids=list(allowed),
        ),
    )


def test_rpc_search_serves_real_request(servicer, search_stack):
    context = FakeContext()
    response = servicer.Search(make_request(), context)
    assert context.code is None
    assert response.generation == "gen-q01"
    assert response.mode == "graphrag"
    assert "f1" in response.assertion_ids
    assert response.paths, "paths must surface on the wire"


def test_rpc_unsupported_mode_failed_precondition(servicer, search_stack):
    context = FakeContext()
    with pytest.raises(RuntimeError):
        servicer.Search(make_request(mode="magic"), context)
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION


def test_rpc_no_generation_failed_precondition(servicer, pg_dsn, deletion_service, generation_index_store):
    """A scope with NO published generation fails with a clear status."""
    from semantic_service.contracts import ScopeKey as SK
    from semantic_service.pg_access import PgAccessGraph as PAG
    import semantic_service.query.search as search_mod
    from semantic_service.servicer import SemanticServicer as Srv

    empty_scope = SK(tenant_id=424242, kb_id="kb-none")

    def factory(scope):
        graph = PAG(pg_dsn, scope, deletion_service)
        return search_mod.SearchService(generation_index_store, graph, FrozenGraphRAGAdapter())

    isolated = Srv(factory)
    request = semantic_pb2.SearchRequest(
        query_id="q-2", query="x",
        access_scope=semantic_pb2.AccessScope(
            scope=semantic_pb2.ScopeKey(tenant_id=empty_scope.tenant_id, kb_id=empty_scope.kb_id),
            allowed_document_ids=["d1"],
        ),
    )
    context = FakeContext()
    with pytest.raises(RuntimeError):
        isolated.Search(request, context)
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION
    assert "generation" in (context.details or "")
