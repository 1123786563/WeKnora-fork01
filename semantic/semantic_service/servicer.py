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
    def __init__(self, search_factory, rules_factory=None, model_factory=None):
        """search_factory(scope) -> SearchService: builds a PER-REQUEST
        search stack bound to the request's own scope -
        the access graph SQL is always tenant/kb-scoped to the caller.
        rules_factory/model_factory (Q03) build per-request reasoners;
        None keeps that Reason mode UNIMPLEMENTED."""
        self._search_factory = search_factory
        self._rules_factory = rules_factory
        self._model_factory = model_factory

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

    def Reason(self, request, context):
        """Q03: RULES vs MODEL dispatch. A model conclusion NEVER claims
        rule proof; each mode runs its own authorized reasoner."""
        mode = request.reasoning_mode
        if mode == semantic_pb2.ReasonRequest.REASONING_MODE_RULES:
            if self._rules_factory is None:
                context.abort(grpc.StatusCode.UNIMPLEMENTED, "rules mode not configured")
            return self._reason(request, context, "rules")
        if mode == semantic_pb2.ReasonRequest.REASONING_MODE_MODEL:
            if self._model_factory is None:
                context.abort(grpc.StatusCode.UNIMPLEMENTED, "model mode not configured")
            if not request.search.query_id:
                # Every invocation needs a caller-supplied unique query id:
                # the A03 ledger keys on it (empty ids would collide across
                # tenants - permanent conflict / replay).
                context.abort(grpc.StatusCode.INVALID_ARGUMENT,
                              "model mode requires a unique search.query_id")
            return self._reason(request, context, "model")
        context.abort(grpc.StatusCode.INVALID_ARGUMENT, "unspecified reasoning mode")

    def _reason(self, request, context, mode_name):
        wire_scope = request.search.access_scope
        scope = ScopeKey(tenant_id=wire_scope.scope.tenant_id, kb_id=wire_scope.scope.kb_id)
        # Reasoning input closure: run the internal authorized search when
        # available; reasoners see ONLY what the authorized subgraph returned.
        authorized_ids = set()
        evidence_lines = []
        if self._search_factory is not None:
            try:
                search = self._search_factory(scope)
                inner = search.search(SearchRequest(
                    access_scope=AccessScope(
                        scope=scope,
                        allowed_document_ids=list(wire_scope.allowed_document_ids),
                    ),
                    query=request.search.query,
                ))
                for evidence in inner.evidence:
                    for aid in evidence.assertion_ids:
                        authorized_ids.add(aid)
                        evidence_lines.append(f"{aid}: {evidence.document_id}: {evidence.chunk_id}")
            except (NoActiveGeneration, UnsupportedMode) as exc:
                # Precondition failures surface honestly; infrastructure
                # errors propagate as INTERNAL (never fake a verdict).
                context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(exc))
        if mode_name == "rules":
            from .query.reason_rules import ReasonRequest as RulesReasonRequest

            reasoner = self._rules_factory(scope)
            facts = tuple()
            goal = _goal_from_query(request.search.query)
            result = reasoner.reason(RulesReasonRequest(
                rule_set_version=request.rule_set_version, facts=facts, goal=goal))
            kind = semantic_pb2.ReasonResponse.CONCLUSION_KIND_RULE
        else:
            from .query.reason_model import ModelReasonRequest

            reasoner = self._model_factory(scope)
            import uuid as _uuid

            result = reasoner.reason(ModelReasonRequest(
                query=request.search.query,
                authorized_assertion_ids=authorized_ids,
                evidence_text="\n".join(evidence_lines),
                operation_id=request.search.query_id,
                invocation_id=f"reason-{request.search.query_id}-{_uuid.uuid4().hex[:12]}",
                budget_ref=wire_scope.budget_ref or "default"))
            kind = semantic_pb2.ReasonResponse.CONCLUSION_KIND_MODEL
        response = semantic_pb2.ReasonResponse(conclusion_kind=kind)
        response.retrieval.query_id = request.search.query_id
        response.retrieval.mode = mode_name
        response.status = result.status
        conclusion = getattr(result, "conclusion", None)
        if isinstance(conclusion, str):
            response.conclusion = conclusion
        for premise_id in getattr(result, "premise_ids", []):
            response.premise_ids.append(premise_id)
        for rule_id in getattr(result, "rule_ids", []):
            response.rule_ids.append(rule_id)
        limitations = getattr(result, "limitations", "")
        if limitations:
            response.limitations.append(limitations)
        return response


def _goal_from_query(query: str):
    parts = query.replace("?", "").split()
    if len(parts) == 3:
        return (parts[0], parts[1], parts[2])
    return ("", "", "")
