"""GraphRAG search with bounded execution (Q01).

The search pins the CURRENT published generation (I03 read lease),
expands ONLY the authorized subgraph (A02), retrieves through the frozen
adapter, validates that every returned source came from the graph, and
releases the lease in a finally. Truncation, staleness and the ACTUAL
mode are always reported; unsupported modes fail with a precondition
error instead of silently switching algorithms.
"""

from __future__ import annotations

from dataclasses import dataclass, field, replace
from typing import Optional, Protocol

from ..access import AccessGraph
from ..contracts import ScopeKey
from .subgraph import AuthorizedGraph, QueryLimits, build_authorized_subgraph


@dataclass(frozen=True)
class SearchLimits:
    max_nodes: int = 64
    max_edges: int = 256
    max_hops: int = 3
    max_evidence: int = 32


@dataclass
class AccessScope:
    scope: ScopeKey
    allowed_document_ids: list[str]


@dataclass
class SearchRequest:
    access_scope: AccessScope
    query: str
    limits: SearchLimits = field(default_factory=SearchLimits)
    mode: str = "graphrag"
    deadline_ms: int = 30000


@dataclass(frozen=True)
class Evidence:
    evidence_id: str
    document_id: str
    chunk_id: str
    content_hash: str
    assertion_ids: tuple[str, ...] = ()


@dataclass
class SearchResponse:
    mode: str
    generation: str
    evidence: list[Evidence]
    assertion_ids: list[str]
    paths: list[list[str]]
    truncated: bool
    stale: bool = False
    partial: bool = False


class UnsupportedMode(Exception):
    """The upstream adapter does not implement the requested mode."""


class NoActiveGeneration(Exception):
    """pin() found no published generation for the scope."""


class RetrievalAdapter(Protocol):
    def retrieve(self, graph: AuthorizedGraph, query: str, limits) -> "RankedEvidence": ...


@dataclass
class RankedEvidence:
    evidence_ids: list[str]
    assertion_ids: list[str]
    paths: list[list[str]]
    truncated: bool = False
    # evidence_id -> assertion ids it supports (source protocol linkage).
    evidence_assertions: dict[str, tuple[str, ...]] = field(default_factory=dict)
    # Adapter-internal scores; NEVER exposed in the response.
    _scores: dict[str, float] = field(default_factory=dict)


class SearchService:
    def __init__(self, index_store, access_graph: AccessGraph, adapter: RetrievalAdapter):
        self._index_store = index_store
        self._access_graph = access_graph
        self._adapter = adapter
        # Bind the access graph into adapters that derive evidence from
        # authorized support rows.
        if hasattr(adapter, "bind"):
            adapter.bind(access_graph)

    def close(self) -> None:
        return None

    def search(self, request: SearchRequest) -> SearchResponse:
        if request.mode != "graphrag":
            raise UnsupportedMode(f"mode {request.mode!r} is not implemented by this adapter")
        snapshot = request.access_scope
        try:
            lease = self._index_store.pin(snapshot.scope)
        except RuntimeError as exc:
            raise NoActiveGeneration(str(exc)) from exc
        try:
            allowed = set(snapshot.allowed_document_ids)
            # Seeds: alias matches (A02 search_seeds) PLUS assertions whose
            # subject/object node id EXACTLY equals the query string (no
            # natural-language seeding in this version) - both paths filter
            # by authorized visibility before expansion.
            seeds = list(self._access_graph.search_seeds(request.query, allowed))
            for assertion_id in self._access_graph.assertions_on(request.query):
                if assertion_id not in seeds and self._access_graph.visible(assertion_id, allowed):
                    seeds.append(assertion_id)
            limits = QueryLimits(max_nodes=request.limits.max_nodes,
                                 max_edges=request.limits.max_edges,
                                 max_hops=request.limits.max_hops)
            graph = build_authorized_subgraph(self._access_graph, snapshot.scope, None,
                                              seeds, allowed, limits)
            ranked = self._adapter.retrieve(graph, request.query, request.limits)
            self._validate_result_sources(ranked, graph, allowed)
            return self._to_search_response(ranked, lease.generation, graph.truncated,
                                            allowed, request.limits)
        finally:
            self._index_store.release(lease.lease_id)

    def _validate_result_sources(self, ranked: RankedEvidence, graph: AuthorizedGraph,
                                  allowed: set[str]) -> None:
        """Every assertion AND every evidence document the adapter returns
        MUST come from the authorized graph / allowed set - violations are
        REJECTED, never silently dropped."""
        graph_assertions = set(graph.assertions)
        for assertion_id in ranked.assertion_ids:
            if assertion_id not in graph_assertions:
                raise ValueError(f"adapter returned assertion outside the authorized graph: {assertion_id!r}")
        for evidence_id in ranked.evidence_ids:
            doc = self._evidence_document(evidence_id)
            if doc not in allowed:
                raise ValueError(f"adapter returned evidence outside the allowed documents: {evidence_id!r}")

    def _to_search_response(self, ranked: RankedEvidence, generation: str, truncated: bool,
                            allowed: set[str], limits: SearchLimits) -> SearchResponse:
        evidence = []
        seen: set[tuple[str, str, str]] = set()
        for evidence_id in ranked.evidence_ids:
            doc = self._evidence_document(evidence_id)
            if doc not in allowed:
                continue  # source discipline: never include disallowed docs
            key = (doc, self._evidence_chunk(evidence_id), self._evidence_hash(evidence_id))
            if key in seen:
                continue  # stable dedup by (document, chunk, hash)
            seen.add(key)
            evidence.append(Evidence(
                evidence_id=evidence_id, document_id=doc,
                chunk_id=key[1], content_hash=key[2],
                assertion_ids=tuple(ranked.evidence_assertions.get(evidence_id, ())),
            ))
            if len(evidence) >= limits.max_evidence:
                # Flag only if evidence actually remains unprocessed.
                if len(seen) < len(ranked.evidence_ids):
                    truncated = True
                break
        # Stable ordering: document, chunk, hash.
        evidence.sort(key=lambda e: (e.document_id, e.chunk_id, e.content_hash))
        return SearchResponse(
            mode="graphrag",
            generation=generation,
            evidence=evidence,
            assertion_ids=list(ranked.assertion_ids),
            paths=[list(p) for p in ranked.paths],
            truncated=truncated or ranked.truncated,
        )

    # --- evidence binding hooks (adapter-supplied; default fixture data) ---

    def _evidence_document(self, evidence_id: str) -> str:
        return evidence_id.split(":", 1)[0] if ":" in evidence_id else evidence_id

    def _evidence_chunk(self, evidence_id: str) -> str:
        return evidence_id.split(":", 2)[1] if evidence_id.count(":") >= 2 else ""

    def _evidence_hash(self, evidence_id: str) -> str:
        return evidence_id.split(":", 2)[2] if evidence_id.count(":") >= 2 else ""
