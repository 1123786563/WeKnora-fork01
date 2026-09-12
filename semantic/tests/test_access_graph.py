"""A02 authorized-subgraph tests against the REAL PG fact store.

Hidden documents must not seed entities (D4), hidden premises must kill
derived edges, truncation is marked, cache keys partition by scope+epoch,
and same-named entities across tenants never associate.
"""

from pathlib import Path

import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.indexing.deletion import DeletionService
from semantic_service.pg_access import PgAccessGraph
from semantic_service.query.cache import ScopedCache
from semantic_service.query.subgraph import QueryLimits, build_authorized_subgraph

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-a02")
OTHER_TENANT = ScopeKey(tenant_id=99, kb_id="kb-a02")


@pytest.fixture()
def access_graph(pg_dsn, deletion_service):
    return PgAccessGraph(pg_dsn, SCOPE, deletion_service)


def seed(access_graph: PgAccessGraph, deletion: DeletionService):
    """ent-cedar has a DIFFERENT visible alias; the only 松柏 alias sits on
    the hidden document dH - searching 松柏 must yield no seeds (D4)."""
    deletion.add_assertion_support(SCOPE, "alias-cedar", "ent-cedar", "alias",
        value="雪松", support_document_id="d1", support_revision=1)
    deletion.add_assertion_support(SCOPE, "alias-hidden", "ent-pine", "alias",
        value="松柏", support_document_id="dH", support_revision=1)
    # dH is tombstoned: its alias must never seed.
    deletion.delete_revision(SCOPE, "dH", 1)
    # Derived edge premised on a hidden premise.
    deletion.add_assertion_support(SCOPE, "src-a-c", "ent-a", "controls",
        object_id="ent-c", support_document_id="d1", support_revision=1)
    deletion.add_assertion_support(SCOPE, "prem-hidden", "ent-h", "is", value="h",
        support_document_id="dH2", support_revision=1)
    deletion.delete_revision(SCOPE, "dH2", 1)
    deletion.add_assertion_support(SCOPE, "derived-a-c", "ent-a", "linked",
        object_id="ent-c", kind="rule", support_document_id="d1", support_revision=1,
        premise_ids=["prem-hidden"])


def test_hidden_alias_cannot_seed_visible_entity(access_graph, deletion_service):
    seed(access_graph, deletion_service)
    result = access_graph.search_seeds(query="松柏", allowed_documents={"d1", "d2"})
    assert result == []


def test_hidden_premise_invalidates_derived_edge(access_graph, deletion_service):
    seed(access_graph, deletion_service)
    assert not access_graph.visible("derived-a-c", allowed_documents={"d1"})


def test_visible_assertion_via_allowed_document(access_graph, deletion_service):
    seed(access_graph, deletion_service)
    assert access_graph.visible("alias-cedar", allowed_documents={"d1"})
    # Excluding d1 hides it too (scope discipline, not just tombstones).
    assert not access_graph.visible("alias-cedar", allowed_documents={"d2"})


def test_same_name_cross_tenant_never_associates(access_graph, deletion_service):
    # Same entity name in another tenant.
    deletion_service.add_assertion_support(OTHER_TENANT, "alias-cedar", "ent-cedar", "alias",
        value="松柏", support_document_id="dx", support_revision=1)
    result = access_graph.search_seeds(query="松柏", allowed_documents={"dx"})
    assert result == [], "tenant 1 scope must never see tenant 99 assertions"


def test_subgraph_truncation_marks_and_respects_limits(access_graph, deletion_service):
    deletion_service.add_assertion_support(SCOPE, "f1", "n1", "p", object_id="n2",
        support_document_id="d1", support_revision=1)

    class StubGraph:
        def visible(self, assertion_id, allowed_documents):
            return assertion_id in {"f1"}

        def neighborhood(self, assertion_id, allowed_documents):
            return ["n1", "n2"] if assertion_id == "f1" else []

        def assertions_on(self, node):
            return ["f1"] if node in {"n1", "n2"} else []

    graph = build_authorized_subgraph(StubGraph(), SCOPE, None, ["f1"], {"d1"},
                                      QueryLimits(max_nodes=1, max_edges=2, max_hops=3))
    assert graph.truncated is True
    assert graph.assertions == ["f1"]


def test_premise_on_live_but_disallowed_doc_kills_derived(access_graph, deletion_service):
    """DB-visible premise on d2, derived on d1, allowed={d1}: the Python
    premise recursion (not the DB fixpoint) must reject."""
    deletion_service.add_assertion_support(SCOPE, "src-live", "ent-l", "is", value="v",
        support_document_id="d2", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "derived-live", "ent-d", "derived", kind="rule",
        support_document_id="d1", support_revision=1, premise_ids=["src-live"])
    assert not access_graph.visible("derived-live", allowed_documents={"d1"})


def test_positive_seeds_find_visible_alias(access_graph, deletion_service):
    seed(access_graph, deletion_service)
    result = access_graph.search_seeds(query="雪松", allowed_documents={"d1"})
    assert result == ["alias-cedar"]


def test_real_store_subgraph_walk_excludes_hidden(access_graph, deletion_service):
    seed(access_graph, deletion_service)
    # A visible assertion chain the walk can expand through.
    deletion_service.add_assertion_support(SCOPE, "link-a-b", "ent-a", "controls",
        object_id="ent-b", support_document_id="d1", support_revision=1)
    deletion_service.add_assertion_support(SCOPE, "hidden-link", "ent-b", "controls",
        object_id="ent-hx", support_document_id="dH", support_revision=1)
    from semantic_service.query.subgraph import QueryLimits, build_authorized_subgraph

    graph = build_authorized_subgraph(access_graph, SCOPE, None, ["link-a-b"],
                                      allowed_documents={"d1"},
                                      limits=QueryLimits(max_nodes=10, max_edges=10, max_hops=2))
    assert "link-a-b" in graph.assertions
    assert "hidden-link" not in graph.assertions, "hidden assertions must never enter the walk"
    assert "alias-hidden" not in graph.assertions


def test_node_limit_enforced_within_single_hop(access_graph, deletion_service):
    """A star assertion's neighborhood must not blow past max_nodes."""
    deletion_service.add_assertion_support(SCOPE, "star", "ent-s", "p", object_id="ent-c",
        support_document_id="d1", support_revision=1)
    for i in range(30):
        deletion_service.add_assertion_support(SCOPE, f"star-{i}", "ent-s", "p",
            object_id=f"ent-{i}", support_document_id="d1", support_revision=1)
    from semantic_service.query.subgraph import QueryLimits, build_authorized_subgraph

    graph = build_authorized_subgraph(access_graph, SCOPE, None, ["star"],
                                      allowed_documents={"d1"},
                                      limits=QueryLimits(max_nodes=5, max_edges=64, max_hops=2))
    assert len(graph.nodes) <= 5, f"node limit blown: {len(graph.nodes)}"
    assert graph.truncated is True


def test_shared_premise_diamond_terminates_quickly(access_graph, deletion_service):
    """Depth-20 shared-premise DAG must NOT explode exponentially."""
    import time

    # Chain of depth 20, each derived from the previous.
    deletion_service.add_assertion_support(SCOPE, "base", "n0", "is", value="v",
        support_document_id="d1", support_revision=1)
    # Fibonacci-style DAG: node i premised on [i-1, i-2] - without
    # memoization this is ~2^depth visits.
    deletion_service.add_assertion_support(SCOPE, "chain-1", "n1", "derived", kind="rule",
        support_document_id="d1", support_revision=1, premise_ids=["base"])
    prev1, prev2 = "chain-1", "base"
    for i in range(2, 21):
        aid = f"chain-{i}"
        deletion_service.add_assertion_support(SCOPE, aid, f"n{i}", "derived", kind="rule",
            support_document_id="d1", support_revision=1, premise_ids=[prev1, prev2])
        prev2, prev1 = prev1, aid
    start = time.monotonic()
    assert access_graph.visible(prev1, allowed_documents={"d1"}) is True
    elapsed = time.monotonic() - start
    assert elapsed < 5.0, f"memoization missing: depth-20 chain took {elapsed:.1f}s"


def test_authorize_assertion_fail_closed_and_delegates(access_graph, deletion_service):
    from semantic_service.access import authorize_assertion

    class Snapshot:
        allowed_document_ids = ["d1"]

    # Fail closed without a concrete graph.
    with pytest.raises(ValueError, match="fail closed"):
        authorize_assertion("alias-cedar", Snapshot(), None, access_graph=None)
    # Delegates to visible() with the snapshot's allowed documents.
    deletion_service.add_assertion_support(SCOPE, "auth-seam", "x", "p",
        object_id="y", support_document_id="d1", support_revision=1)
    assert authorize_assertion("auth-seam", Snapshot(), None, access_graph=access_graph) is True
    assert authorize_assertion("alias-hidden", Snapshot(), None, access_graph=access_graph) is False


def test_memoized_false_not_poisoned_by_cycle_guard():
    """Reviewer repro: a1<->a2 cycle with a premise-free fallback row.
    visible('a0') must be True (reference AND least-fixpoint agree);
    a context-tainted False memo for a1 must not survive."""
    from semantic_service.access import AccessGraph

    class Row:
        def __init__(self, doc, visible, premises):
            self.doc, self.visible, self.premises = doc, visible, premises

    class NullDeletion:
        def denied(self, scope, document_id, revision):
            return False

    class RowsGraph(AccessGraph):
        def __init__(self, rows):
            super().__init__(deletion=NullDeletion())
            self.rows = rows

        def _support_rows(self, assertion_id):
            out = []
            for r in self.rows.get(assertion_id, []):
                out.append({
                    "support_document_id": r.doc, "support_revision": 1,
                    "visible": r.visible, "_premises": r.premises,
                    "_scope": None,
                })
            return out

        def _alias_index(self):
            return {}

    graph = RowsGraph({
        # row ORDER matters (reviewer seed 20260204 trial 94)
        "a2": [Row("d3", True, []), Row("d2", True, ["a1"]), Row("d1", True, [])],
        "a1": [Row("d2", True, ["a2"])],
        "a3": [Row("d2", False, ["a3", "a2"])],
        "a0": [Row("d2", True, ["a3"]), Row("d2", True, ["a3", "a2"]),
               Row("d1", True, ["a2", "a1"])],
    })
    allowed = {"d1", "d2", "d3"}
    assert graph.visible("a2", allowed) is True
    assert graph.visible("a1", allowed) is True
    assert graph.visible("a0", allowed) is True, "tainted memo poisoned a1's True"


def test_guard_refire_does_not_untaint_false():
    """Reviewer trial-7857 repro: a2 is evaluated twice; the second walk's
    guard RE-FIRE on an already-recorded id must still taint a2's False
    (a2 is genuinely True via another row)."""
    from semantic_service.access import AccessGraph

    class NullDeletion:
        def denied(self, scope, document_id, revision):
            return False

    class Row:
        def __init__(self, doc, visible, premises):
            self.doc, self.visible, self.premises = doc, visible, premises

    class RowsGraph(AccessGraph):
        def __init__(self, rows):
            super().__init__(deletion=NullDeletion())
            self.rows = rows

        def _support_rows(self, assertion_id):
            return [{"support_document_id": r.doc, "support_revision": 1,
                     "visible": r.visible, "_premises": r.premises, "_scope": None}
                    for r in self.rows.get(assertion_id, [])]

        def _alias_index(self):
            return {}

    graph = RowsGraph({
        "a0": [Row("d1", False, ["a1"]), Row("d1", True, ["a2"]),
               Row("d1", True, ["a3", "a2", "a0"])],
        "a1": [Row("d1", False, [])],
        "a2": [Row("d1", True, ["a1", "a4"]), Row("d2", True, ["a4"])],
        "a3": [Row("d1", True, ["a4", "a2"])],
        "a4": [Row("d2", True, ["a2", "a4", "a0"]), Row("d2", True, ["a2"]),
               Row("d1", True, [])],
    })
    allowed = {"d1", "d2"}
    assert graph.visible("a3", allowed) is True, "idempotent guard re-fire untainted a2's False"


def test_cache_epoch_and_scope_partition(pg_dsn):
    cache = ScopedCache()
    from semantic_service.access import cache_key

    class Request:
        tenant_id = 1
        kb_id = "kb-x"
        query = "q"
        model_version = "m1"
        rule_set_version = "r1"

    key_a = cache_key(Request(), None, scope_hash="scopeA", permission_epoch=7)
    key_b = cache_key(Request(), None, scope_hash="scopeB", permission_epoch=7)
    key_a2 = cache_key(Request(), None, scope_hash="scopeA", permission_epoch=8)
    assert len({key_a, key_b, key_a2}) == 3, "scope hash AND epoch must both partition"

    cache.put(key_a, 7, {"answer": 1})
    assert cache.get(key_a, 7) == {"answer": 1}
    # Epoch bump invalidates structurally.
    assert cache.get(key_a, 8) is None
    # Hit re-verification rejects on deny change.
    cache.put(key_a, 7, {"answer": 1})
    assert cache.get(key_a, 7, verify=lambda v: False) is None
    assert cache.get(key_a, 7) is None, "rejected entry must be dropped"
