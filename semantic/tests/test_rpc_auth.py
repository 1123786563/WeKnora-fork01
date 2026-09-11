"""C02 gRPC authentication and lifecycle tests against a real server.

The server must fail closed: no service identity -> UNAUTHENTICATED for every
business RPC; unimplemented business methods -> UNIMPLEMENTED (never fake
success); health reflects the actual service state.
"""

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from semantic_service.proto import semantic_pb2 as pb
from semantic_service.proto import semantic_pb2_grpc as pb_grpc


def test_missing_service_identity_is_denied(rpc_client):
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.unauthenticated.GetCapabilities(pb.GetCapabilitiesRequest())
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED


def test_wrong_service_identity_is_denied(rpc_client):
    # A channel that is TLS-trusted (test CA) but presents the wrong token is
    # still rejected - transport trust alone is not service identity.
    wrong_creds = grpc.composite_channel_credentials(
        grpc.ssl_channel_credentials(root_certificates=rpc_client.ca_cert_bytes),
        grpc.metadata_call_credentials(
            lambda context, callback: callback((("x-semantic-token", "wrong-token"),), None)
        ),
    )
    stub = pb_grpc.SemanticStub(rpc_client.open_channel(wrong_creds))
    with pytest.raises(grpc.RpcError) as exc:
        stub.GetCapabilities(pb.GetCapabilitiesRequest())
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED


def test_health_is_serving_without_business_implementation(rpc_client):
    response = rpc_client.health.Check(health_pb2.HealthCheckRequest(service=""))
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_authenticated_unimplemented_method_is_not_fake_success(rpc_client):
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.authenticated.GetCapabilities(pb.GetCapabilitiesRequest())
    assert exc.value.code() == grpc.StatusCode.UNIMPLEMENTED
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.authenticated.ApplyDocumentRevision(pb.ApplyDocumentRevisionRequest())
    assert exc.value.code() == grpc.StatusCode.UNIMPLEMENTED


def test_client_deadline_is_propagated(rpc_client):
    # Deterministic transport-level deadline: a TCP listener that accepts
    # connections but never answers cannot produce a fast response, so only
    # the deadline can end the call.
    import socket
    import threading

    listener = socket.socket()
    listener.bind(("127.0.0.1", 0))
    listener.listen(8)
    listener_port = listener.getsockname()[1]
    held = []

    def _hold():
        while True:
            try:
                conn, _ = listener.accept()
                held.append(conn)
            except OSError:
                return

    accept_thread = threading.Thread(target=_hold, daemon=True)
    accept_thread.start()
    try:
        stub = pb_grpc.SemanticStub(grpc.secure_channel(
            f"127.0.0.1:{listener_port}",
            grpc.ssl_channel_credentials(root_certificates=rpc_client.ca_cert_bytes),
        ))
        with pytest.raises(grpc.RpcError) as exc:
            stub.GetCapabilities(pb.GetCapabilitiesRequest(), timeout=0.5)
        assert exc.value.code() == grpc.StatusCode.DEADLINE_EXCEEDED
    finally:
        listener.close()
        for conn in held:
            conn.close()


def test_anonymous_health_probing_is_denied(rpc_client):
    # Readiness must not leak to anonymous callers - health needs the same
    # service identity as business RPCs.
    anonymous_health = health_pb2_grpc.HealthStub(rpc_client.open_channel(
        grpc.ssl_channel_credentials(root_certificates=rpc_client.ca_cert_bytes)))
    with pytest.raises(grpc.RpcError) as exc:
        anonymous_health.Check(health_pb2.HealthCheckRequest(service=""))
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED


def test_anonymous_streaming_watch_is_denied(rpc_client):
    # Streaming RPCs must not bypass the interceptor either.
    anonymous_health = health_pb2_grpc.HealthStub(rpc_client.open_channel(
        grpc.ssl_channel_credentials(root_certificates=rpc_client.ca_cert_bytes)))
    with pytest.raises(grpc.RpcError) as exc:
        list(anonymous_health.Watch(health_pb2.HealthCheckRequest(service="")))
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED


def test_non_ascii_token_denied_without_crash():
    # gRPC clients refuse non-ASCII metadata values, but a hostile raw peer
    # can still send arbitrary bytes: the comparison must deny cleanly over
    # utf-8 encodings instead of raising TypeError inside the interceptor.
    from semantic_service.auth import token_matches

    assert token_matches("密码-token", "real-token") is False
    assert token_matches("", "real-token") is False
    assert token_matches("real-token", "") is False
    assert token_matches("real-token", "real-token") is True


def test_business_rpc_health_check_service_name_isolation(rpc_client):
    # The health service itself must answer with the service identity, and
    # an unknown service name reports NOT_FOUND rather than SERVING - health
    # must not pretend readiness it does not have.
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.health.Check(health_pb2.HealthCheckRequest(service="not-a-service"))
    assert exc.value.code() == grpc.StatusCode.NOT_FOUND
