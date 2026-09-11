"""V01 import contract for the frozen Semantica release.

The pinned upstream release must expose the graph, extraction, context and
reasoning APIs the WeKnora semantic service depends on. Downstream tasks may
only import what this contract proves importable; it is the seam between the
frozen upstream capability and our production code.

Test environment problems (python/pytest) must be fixed before RED is
recorded; a missing semantica capability is the failure under test.
"""


def test_required_import_contract():
    from semantica.semantic_extract import NERExtractor, RelationExtractor
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.reasoning import Reasoner, GraphReasoner
    from semantica.graph_store import GraphStore
    assert all(callable(c) for c in (
        NERExtractor, RelationExtractor, ContextGraph,
        ContextRetriever, Reasoner, GraphReasoner, GraphStore))
