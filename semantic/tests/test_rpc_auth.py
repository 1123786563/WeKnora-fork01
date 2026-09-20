from __future__ import annotations

import grpc
import pytest

from semantic_service.proto import semantic_pb2
from grpc_health.v1 import health_pb2


def test_missing_service_identity_is_denied(rpc_client) -> None:
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.unauthenticated.GetCapabilities(semantic_pb2.Empty(), timeout=2)
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED


def test_valid_identity_reaches_unimplemented_method(rpc_client) -> None:
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.authenticated.GetCapabilities(
            semantic_pb2.Empty(),
            timeout=2,
            metadata=(("authorization", "Bearer test-service-token"), ("x-weknora-audience", "weknora-semantic")),
        )
    assert exc.value.code() == grpc.StatusCode.UNIMPLEMENTED


def test_process_health_does_not_claim_semantic_readiness(rpc_client) -> None:
    response = rpc_client.health.Check(
        health_pb2.HealthCheckRequest(service="weknora.semantic.v1.SemanticService"),
        timeout=2,
        metadata=(("authorization", "Bearer test-service-token"), ("x-weknora-audience", "weknora-semantic")),
    )
    assert response.status == health_pb2.HealthCheckResponse.NOT_SERVING
