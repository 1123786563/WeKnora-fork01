"""Semantic gRPC servicers (Q01+).

The Search RPC pins a generation, expands the authorized subgraph and
returns evidence with the ACTUAL mode; unsupported modes and missing
dependencies fail with explicit statuses - never fake success.
"""

from __future__ import annotations

import grpc

from .contracts import ScopeKey
from .proto import semantic_pb2, semantic_pb2_grpc
from .query.search import (AccessScope, NoActiveGeneration, SearchLimits,
                             SearchRequest, SearchService, UnsupportedMode)


class SemanticServicer(semantic_pb2_grpc.SemanticServicer):
    def __init__(self, search_factory):
        """search_factory(scope) -> SearchService: builds a PER-REQUEST
        search stack bound to the request's own scope -
        the access graph SQL is always tenant/kb-scoped to the caller."""
        self._search_factory = search_factory

    def Search(self, request, context):
        wire_scope = request.access_scope
        scope = ScopeKey(tenant_id=wire_scope.scope.tenant_id,
                         kb_id=wire_scope.scope.kb_id)
        limits = request.limits
        search_request = SearchRequest(
            access_scope=AccessScope(
                scope=scope,
                allowed_document_ids=list(wire_scope.allowed_document_ids),
            ),
            query=request.query,
            mode=request.mode or "graphrag",
            limits=SearchLimits(
                max_nodes=limits.max_nodes or 64,
                max_edges=limits.max_edges or 256,
                max_hops=limits.max_hops or 3,
                max_evidence=limits.max_evidence or 32,
            ),
        )
        search = self._search_factory(scope)
        try:
            result = search.search(search_request)
        except UnsupportedMode as exc:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(exc))
        except NoActiveGeneration as exc:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(exc))
        response = semantic_pb2.SearchResponse(
            query_id=request.query_id,
            generation=result.generation,
            mode=result.mode,
            assertion_ids=result.assertion_ids,
            stale=result.stale,
            partial=result.partial,
            truncated=result.truncated,
        )
        for evidence in result.evidence:
            response.evidence.append(semantic_pb2.Evidence(
                evidence_id=evidence.evidence_id,
                document_id=evidence.document_id,
                chunk_id=evidence.chunk_id,
                content_hash=evidence.content_hash,
                assertion_ids=evidence.assertion_ids,
            ))
        for path in result.paths:
            response.paths.append(semantic_pb2.SearchResponse.Path(nodes=path))
        return response