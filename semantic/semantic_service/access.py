"""Authorized-fact visibility (A02).

Visibility is computed BEFORE any graph expansion: an assertion is
visible iff (a) at least one support row survives the tombstones,
(b) every premise of a derived assertion is itself visible (conjunctive,
cycle-safe), and (c) the asserting document is in the scope snapshot's
allowed set. Hidden documents never seed entities, never contribute
equivalence edges, and never influence ranking or summaries.
"""

from __future__ import annotations

from typing import Optional

from .indexing.deletion import DeletionService
from .indexing.manifest import IndexManifest


class AccessGraph:
    """Authorization-first visibility over the fact store."""

    def __init__(self, deletion: DeletionService):
        self._deletion = deletion

    def close(self) -> None:
        return None

    def visible(self, assertion_id: str, allowed_documents: set[str]) -> bool:
        """True iff the assertion has a support row that (a) is marked
        visible, (b) survives tombstones, (c) sits on an allowed document,
        and (d) if derived, has all premises visible (conjunctive).

        Memoized per call: shared premises in DAGs are evaluated once
        (cycle revisits still fail closed via the visiting set)."""
        memo: dict[str, bool] = {}
        return self._visible(assertion_id, allowed_documents, frozenset(), memo)

    def _visible(self, assertion_id: str, allowed_documents: set[str], visiting: frozenset,
                 memo: dict[str, bool]) -> bool:
        if assertion_id in memo:
            return memo[assertion_id]
        if assertion_id in visiting:  # cycle guard: no finite derivation
            return False
        rows = self._support_rows(assertion_id)
        for row in rows:
            doc = row["support_document_id"]
            if doc not in allowed_documents:
                continue
            if not row["visible"]:
                continue
            if self._deletion.denied(row["_scope"], doc, row["support_revision"]):
                continue
            premises = row["_premises"]
            if premises and not all(
                self._visible(p, allowed_documents, visiting | {assertion_id}, memo) for p in premises
            ):
                continue
            memo[assertion_id] = True
            return True
        memo[assertion_id] = False
        return False

    def _support_rows(self, assertion_id: str) -> list[dict]:
        """All support rows across scopes are fetched by the deletion
        service; scope discipline is enforced by allowed_documents."""
        raise NotImplementedError("wired by the fixture-backed subclass")

    def search_seeds(self, query: str, allowed_documents: set[str]) -> list[str]:
        """Seed entities from ALIASES: only aliases whose assertion is
        visible under the allowed documents. A hidden document's alias can
        never seed an entity even if the entity itself is visible via
        other documents (D4: hidden words stay out of seeds)."""
        seeds = []
        for alias, assertion_ids in self._alias_index().items():
            if query in alias:
                for assertion_id in assertion_ids:
                    if self.visible(assertion_id, allowed_documents):
                        seeds.append(assertion_id)
                        break
        return seeds

    def _alias_index(self) -> dict[str, list[str]]:
        raise NotImplementedError("wired by the fixture-backed subclass")


def authorize_assertion(assertion_id: str, scope_snapshot, manifest: Optional[IndexManifest],
                        access_graph: Optional[AccessGraph] = None) -> bool:
    """Plan-facing seam delegating to AccessGraph.visible with the scope
    snapshot's allowed documents. Without a concrete graph this REFUSES
    (fail closed) - authorization never defaults to True."""
    if access_graph is None:
        raise ValueError("authorize_assertion requires a concrete AccessGraph (fail closed)")
    allowed = set(scope_snapshot.allowed_document_ids)
    return access_graph.visible(assertion_id, allowed)


def cache_key(request, manifest: Optional[IndexManifest], *, scope_hash: str, permission_epoch: int) -> str:
    """Cache identity: scope hash + epoch + generation + query + limits +
    model/rule/config digests. Cross-scope sharing is structurally
    impossible because the scope hash and epoch are in the key."""
    parts = [
        "owner-tenant", str(getattr(request, "tenant_id", "")),
        "kb", str(getattr(request, "kb_id", "")),
        "gen", manifest.generation if manifest is not None else "",
        "scope", scope_hash,
        "epoch", str(permission_epoch),
        "query", str(getattr(request, "query_digest", getattr(request, "query", ""))),
        "limits", str(getattr(request, "limits_digest", "")),
        "model", str(getattr(request, "model_version", "")),
        "rule", str(getattr(request, "rule_set_version", "")),
        "config", manifest.config_digest if manifest is not None else "",
    ]
    return "|".join(parts)
