"""Fact and entity identity model (C03).

Rules enforced here (spec section 4):
- entity identity is a server-generated stable UUID; names and aliases are
  SOURCED assertions, never free-floating strings;
- source facts cite evidence (validated by evidence.validate_evidence at the
  loading layer); rule/model conclusions cite same-scope premises and cite
  NO evidence directly (their support is transitive through premises);
- nothing dangles; exactly one non-empty object_id/value; derivations form
  an acyclic graph;
- conflicting values for the same subject/predicate COEXIST - last write
  never overwrites - and multi-source support is NEVER collapsed.

Deletion support: every assertion keeps its full evidence/premise support
set, so removing a supporting document (I04) revokes exactly the support it
provided.

Bulk-ingestion performance is NOT this module's concern (I03 owns the store);
upsert_preserving_conflicts is a validation-time helper.
"""

from __future__ import annotations

import uuid
from typing import Mapping

from .contracts import ASSERTION_KINDS, Assertion, Evidence, ScopeKey

VALID_KINDS = ASSERTION_KINDS


def new_entity_id() -> str:
    """Server-generated stable entity UUID (never a name-derived id)."""
    return str(uuid.uuid4())


def _present(value: str | None) -> bool:
    return value is not None and value.strip() != ""


def validate_assertion(
    assertion: Assertion,
    evidence_by_id: Mapping[str, Evidence],
    premises_by_id: Mapping[str, Assertion],
) -> None:
    """Validate one assertion against its resolvable support.

    evidence_by_id maps evidence ids the caller has loaded FOR THE SAME
    SCOPE (the loading layer owns scope filtering and runs
    evidence.validate_evidence on each object); premises are assertions and
    are explicitly checked for scope equality here.
    """
    if assertion.kind not in VALID_KINDS:
        raise ValueError(f"assertion {assertion.assertion_id} has unknown kind {assertion.kind!r}")
    if _present(assertion.object_id) == _present(assertion.value):
        raise ValueError(
            f"assertion {assertion.assertion_id} must have exactly one non-empty object_id/value")
    if not assertion.subject_id.strip() or not assertion.predicate.strip():
        raise ValueError(f"assertion {assertion.assertion_id} needs subject and predicate")
    _validate_scope(assertion.scope, f"assertion {assertion.assertion_id}")

    if assertion.kind == "source":
        if assertion.premise_ids:
            raise ValueError(
                f"source fact {assertion.assertion_id} must not cite premises (premises are for derivations)")
        if not assertion.evidence_ids:
            raise ValueError(f"source fact {assertion.assertion_id} requires at least one evidence id")
    else:
        if not assertion.premise_ids:
            raise ValueError(
                f"{assertion.kind} derivation {assertion.assertion_id} requires at least one premise")
        if assertion.evidence_ids:
            raise ValueError(
                f"{assertion.kind} derivation {assertion.assertion_id} must not cite evidence directly "
                f"(derived support flows through premises)")

    for evidence_id in assertion.evidence_ids:
        if evidence_id not in evidence_by_id:
            raise ValueError(f"assertion {assertion.assertion_id} cites dangling evidence {evidence_id!r}")

    for premise_id in assertion.premise_ids:
        premise = premises_by_id.get(premise_id)
        if premise is None:
            raise ValueError(f"assertion {assertion.assertion_id} cites dangling premise {premise_id!r}")
        if premise.scope != assertion.scope:
            raise ValueError(
                f"assertion {assertion.assertion_id} cites premise {premise_id!r} from a different scope "
                f"(cross-tenant/cross-KB references are forbidden)")


def _validate_scope(scope: ScopeKey, what: str) -> None:
    if scope.tenant_id < 1:
        raise ValueError(f"{what} has invalid tenant_id {scope.tenant_id}")
    if not scope.kb_id.strip():
        raise ValueError(f"{what} has blank kb_id")


def validate_derivation_dag(assertions: list[Assertion]) -> None:
    """Reject duplicate ids and cycles in the premise graph.

    Iterative tri-color DFS: deep chains are judged without recursion
    limits, and duplicate assertion_ids fail loudly instead of silently
    collapsing the graph (which could hide a real cycle).
    """
    by_id: dict[str, Assertion] = {}
    for assertion in assertions:
        if assertion.assertion_id in by_id:
            raise ValueError(f"duplicate assertion_id {assertion.assertion_id!r} in derivation graph")
        by_id[assertion.assertion_id] = assertion

    WHITE, GRAY, BLACK = 0, 1, 2
    color = {assertion_id: WHITE for assertion_id in by_id}

    for root in by_id:
        if color[root] != WHITE:
            continue
        stack: list[tuple[str, int]] = [(root, 0)]
        while stack:
            node, premise_index = stack.pop()
            if premise_index == 0:
                color[node] = GRAY
            premises = by_id[node].premise_ids if node in by_id else ()
            if premise_index < len(premises):
                stack.append((node, premise_index + 1))
                nxt = premises[premise_index]
                if color.get(nxt, WHITE) == WHITE:
                    stack.append((nxt, 0))
                elif color.get(nxt, WHITE) == GRAY:
                    raise ValueError(f"derivation cycle detected through {nxt!r}")
            else:
                color[node] = BLACK


def _same_support(left: Assertion, right: Assertion) -> bool:
    return (
        tuple(left.evidence_ids) == tuple(right.evidence_ids)
        and tuple(left.premise_ids) == tuple(right.premise_ids)
    )


def _same_content(left: Assertion, right: Assertion) -> bool:
    return (
        left.subject_id == right.subject_id
        and left.predicate == right.predicate
        and left.object_id == right.object_id
        and left.value == right.value
        and left.kind == right.kind
        and left.scope == right.scope
    )


def upsert_preserving_conflicts(
    existing: list[Assertion], incoming: Assertion
) -> list[Assertion]:
    """Append a fact without ever dropping support or overwriting a value.

    - same assertion_id with DIFFERENT content raises (upstream id bug);
    - same assertion_id with the same content but a different support set is
      a re-index: both entries coexist (caller reconciles generations);
    - the same claim with the same support set is deduplicated;
    - conflicting values for the same subject/predicate coexist.
    """
    for item in existing:
        if item.assertion_id == incoming.assertion_id:
            if _same_support(item, incoming) and _same_content(item, incoming):
                return existing
            if not _same_content(item, incoming):
                raise ValueError(
                    f"assertion_id {incoming.assertion_id!r} reused with different content "
                    f"(upstream id bug: {item!r} vs {incoming!r})")
            return existing + [incoming]
        if _same_content(item, incoming) and _same_support(item, incoming):
            return existing
    return existing + [incoming]
