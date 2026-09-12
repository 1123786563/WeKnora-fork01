"""Bounded authorized subgraph expansion (A02).

Expansion admits ONLY assertions visible under the scope snapshot; the
walk stops at node/edge/hop limits and marks truncation. No global
degree statistics, no hidden equivalence edges and no hidden summaries
are read anywhere in this walk. Ranking/metric recomputation from the
authorized subgraph is NOT implemented here (Q01 scope); nothing in
this module computes or consumes ranking inputs.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from ..contracts import ScopeKey  # query is a subpackage: two-level relative OK


@dataclass(frozen=True)
class QueryLimits:
    max_nodes: int = 64
    max_edges: int = 256
    max_hops: int = 3


@dataclass
class AuthorizedGraph:
    nodes: list[str] = field(default_factory=list)
    assertions: list[str] = field(default_factory=list)
    evidence: list[str] = field(default_factory=list)
    truncated: bool = False
    generation: Optional[str] = None


def build_authorized_subgraph(access_graph, scope: ScopeKey, manifest, seeds: list[str],
                              allowed_documents: set[str], limits: QueryLimits) -> AuthorizedGraph:
    """Breadth-first expansion admitting only visible assertions."""
    graph = AuthorizedGraph(generation=manifest.generation if manifest is not None else None)
    frontier = list(seeds)
    visited: set[str] = set()
    hops = 0
    halted = False
    while frontier and not halted:
        if hops >= limits.max_hops:
            graph.truncated = True
            break
        next_frontier: list[str] = []
        for assertion_id in frontier:
            if assertion_id in visited:
                continue
            visited.add(assertion_id)
            if len(graph.assertions) >= limits.max_edges:
                graph.truncated = True
                halted = True
                break
            if not access_graph.visible(assertion_id, allowed_documents):
                continue  # hidden facts never enter the walk
            graph.assertions.append(assertion_id)
            for node in access_graph.neighborhood(assertion_id, allowed_documents):
                if len(graph.nodes) >= limits.max_nodes:
                    # Node cap enforced INSIDE the expansion loop - a single
                    # assertion's neighborhood can never blow past the limit.
                    graph.truncated = True
                    halted = True
                    break
                if node not in graph.nodes:
                    graph.nodes.append(node)
                for neighbor in access_graph.assertions_on(node):
                    if neighbor not in visited and neighbor not in next_frontier:
                        next_frontier.append(neighbor)
            if halted:
                break
        frontier = next_frontier
        hops += 1
    return graph
