"""Q03 Reason RPC dispatch tests: rules vs model modes."""

import grpc
import pytest

from semantic_service.proto import semantic_pb2
from semantic_service.servicer import SemanticServicer


class FakeContext:
    def __init__(self):
        self.code = None
        self.details = None

    def abort(self, code, details):
        self.code = code
        self.details = details
        raise RuntimeError(f"aborted: {code}")


@pytest.fixture()
def reason_servicer():
    """Servicer with rules + model factories wired (Q03)."""
    def rules_factory(scope):
        from semantic_service.query.reason_rules import RuleReasoner
        from semantic_service.query.rules import RuleRegistry
        return RuleReasoner(RuleRegistry())

    def model_factory(scope):
        from semantic_service.query.reason_model import ModelReasoner
        import json as _json
        # Empty-evidence-safe payload: valid against ANY authorized set
        # (the supported-with-citations path is covered by unit tests).
        payload = {"status": "insufficient_evidence", "conclusion": "证据不足",
                   "premise_ids": [], "conclusion_kind": "model"}

        class Gateway:
            def invoke(self, invocation_id, operation_id, model_profile_ref, messages, budget_ref):
                return {"text": _json.dumps(payload, ensure_ascii=False)}

        return ModelReasoner(Gateway())

    return SemanticServicer(rules_factory=rules_factory, model_factory=model_factory,
                            search_factory=None)


def make_reason_request(mode):
    return semantic_pb2.ReasonRequest(
        reasoning_mode=mode,
        search=semantic_pb2.SearchRequest(
            query_id="q-reason-1",
            query="甲控制丙?",
            access_scope=semantic_pb2.AccessScope(
                scope=semantic_pb2.ScopeKey(tenant_id=1, kb_id="kb-q03"),
                allowed_document_ids=["d1", "d2"],
            ),
        ),
        rule_set_version="test-control-v1",
    )


def test_reason_rules_mode_dispatches_rule_reasoner(reason_servicer):
    context = FakeContext()
    response = reason_servicer.Reason(make_reason_request(semantic_pb2.ReasonRequest.REASONING_MODE_RULES), context)
    assert context.code is None
    assert response.conclusion_kind == semantic_pb2.ReasonResponse.CONCLUSION_KIND_RULE


def test_reason_model_mode_dispatches_model_reasoner(reason_servicer):
    context = FakeContext()
    response = reason_servicer.Reason(make_reason_request(semantic_pb2.ReasonRequest.REASONING_MODE_MODEL), context)
    assert context.code is None
    assert response.conclusion_kind == semantic_pb2.ReasonResponse.CONCLUSION_KIND_MODEL
    assert response.retrieval.mode == "model"


def test_reason_unspecified_mode_invalid_argument(reason_servicer):
    context = FakeContext()
    with pytest.raises(RuntimeError):
        reason_servicer.Reason(make_reason_request(semantic_pb2.ReasonRequest.REASONING_MODE_UNSPECIFIED), context)
    assert context.code == grpc.StatusCode.INVALID_ARGUMENT


def test_reason_unconfigured_mode_unimplemented():
    from semantic_service.servicer import SemanticServicer as Srv
    servicer = Srv(search_factory=None, rules_factory=None, model_factory=None)
    context = FakeContext()
    with pytest.raises(RuntimeError):
        servicer.Reason(make_reason_request(semantic_pb2.ReasonRequest.REASONING_MODE_RULES), context)
    assert context.code == grpc.StatusCode.UNIMPLEMENTED
