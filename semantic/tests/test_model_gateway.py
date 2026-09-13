"""A03 model gateway tests against the REAL Go internal HTTP endpoint.

The fixture starts the actual Go server (compiled from the handler test
binary) backed by a real migrated SQLite database and a controlled
provider - no mocks at the transport boundary.
"""

import pytest

from semantic_service.model_gateway import MESSAGES, ModelGateway


def test_retry_same_invocation_does_not_double_finalize(model_gateway):
    first = model_gateway.invoke("inv-1", "op-1", "model-a", MESSAGES, "budget-a")
    second = model_gateway.invoke("inv-1", "op-1", "model-a", MESSAGES, "budget-a")
    assert first == second
    assert model_gateway.recorded_invocations("inv-1") == 1
    # model_gateway fixture：真实Go内部HTTP+受控provider，计数来自持久调用表。


def test_missing_identity_is_denied(go_model_server):
    gateway = ModelGateway(go_model_server.url, "wrong-token")
    with pytest.raises(RuntimeError, match="401"):
        gateway.invoke("inv-x", "op-1", "model-a", MESSAGES, "budget-a")


def test_budget_exhaustion_is_payment_required(model_gateway):
    # The controlled Go server starts budget-tight with 40 units; an
    # estimate of 100 must be refused with 402 BEFORE any provider call.
    import json
    import urllib.request
    payload = json.dumps({
        "invocation_id": "inv-budget",
        "operation_id": "op-1",
        "model_profile_ref": "model-a",
        "messages": MESSAGES,
        "budget_ref": "budget-tight",
        "estimated_tokens": 100,
    }).encode()
    request = urllib.request.Request(
        model_gateway._base_url + "/internal/semantic/model/invoke",
        data=payload,
        headers={"Content-Type": "application/json",
                 "X-Semantic-Internal-Token": model_gateway._token},
        method="POST",
    )
    with pytest.raises(Exception) as excinfo:
        urllib.request.urlopen(request, timeout=10)
    assert excinfo.value.code == 402