"""Frozen retrieval adapter (Q01).

The adapter is the ONLY upstream seam: it retrieves from the authorized
graph (never the full store), ranks internally, and returns evidence
with paths. Scores stay adapter-internal per the source protocol.
"""

from __future__ import annotations

from .search import RankedEvidence
from .subgraph import AuthorizedGraph


class FrozenGraphRAGAdapter:
    """Deterministic first-version adapter: paths from the authorized
    graph, ranked by path length (shorter first) - a stable, explainable
    order that never reads global statistics.

    Evidence binding: evidence_id encodes "doc:chunk:hash"; the adapter
    derives them from the access graph's support rows so every evidence
    item traces to an authorized document."""

    def __init__(self, access_graph=None):
        self._access_graph = access_graph

    def bind(self, access_graph) -> None:
        self._access_graph = access_graph

    def retrieve(self, graph: AuthorizedGraph, query: str, limits) -> RankedEvidence:
        paths: list[list[str]] = []
        evidence_ids: list[str] = []
        evidence_assertions: dict[str, tuple[str, ...]] = {}
        for assertion_id in graph.assertions:
            paths.append([assertion_id])
            for evidence_id in self._evidence_for(assertion_id):
                if evidence_id not in evidence_ids:
                    evidence_ids.append(evidence_id)
                evidence_assertions.setdefault(evidence_id, ())
                evidence_assertions[evidence_id] += (assertion_id,)
            if len(paths) >= limits.max_nodes:
                break
        truncated = len(paths) >= limits.max_nodes and len(graph.assertions) > len(paths)
        return RankedEvidence(
            evidence_ids=evidence_ids,
            assertion_ids=list(graph.assertions),
            paths=paths,
            truncated=graph.truncated or truncated,
            evidence_assertions=evidence_assertions,
        )

    def _evidence_for(self, assertion_id: str) -> list[str]:
        """Evidence ids from the assertion's support rows (authorized docs)."""
        if self._access_graph is None:
            return []
        out = []
        for row in self._access_graph._support_rows(assertion_id):
            if row["visible"]:
                out.append(f"{row['support_document_id']}:chunk-{assertion_id}:{row['support_revision']}")
        return out
