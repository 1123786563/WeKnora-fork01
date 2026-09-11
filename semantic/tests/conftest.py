"""Shared fixtures for the semantic service test suite (C02+).

rpc_client starts a REAL gRPC server on a random local port with a test-only
trustme certificate and internal service token, plus authenticated /
unauthenticated client stubs. Nothing here is mocked at the transport layer.
"""

import os
import socket
from types import SimpleNamespace

import grpc
import pytest
import trustme
from grpc_health.v1 import health_pb2_grpc

from semantic_service.config import SemanticServiceConfig
from semantic_service.proto import semantic_pb2_grpc as pb_grpc
from semantic_service.server import create_server

INTERNAL_TOKEN = "test-internal-token-7f3a"

# I01: isolated PostgreSQL for the semantic service operations store. The
# fixture FAILS (never skips) when the environment is missing.
PG_DSN_ENV = "SEMANTIC_TEST_PG_DSN"
DEFAULT_PG_DSN = "postgresql://semantic:semantic@127.0.0.1:15432/semantic_test"


@pytest.fixture(scope="session")
def pg_dsn():
    dsn = os.environ.get(PG_DSN_ENV, DEFAULT_PG_DSN)
    try:
        from semantic_service.operations import OperationStore, apply_migrations

        apply_migrations(dsn)
        probe = OperationStore(dsn)
        probe.close()
    except Exception as exc:  # noqa: BLE001 - explicit environment failure
        pytest.fail(
            f"operations PostgreSQL unavailable at {dsn} ({exc}); start the isolated "
            "test container (see docs/superpowers/plans/semantica/progress.md I01 record)"
        )
    return dsn


@pytest.fixture()
def operation_store(pg_dsn):
    """A real OperationStore over the isolated PG, with per-test cleanup."""
    from semantic_service.operations import OperationStore

    store = OperationStore(pg_dsn)
    store.clear_for_test()
    yield store
    store.close()


@pytest.fixture()
def apply_request():
    from semantic_service.contracts import (
        ApplyRequest,
        ChunkSnapshot,
        DocumentRevision,
        IndexConfig,
        ScopeKey,
    )

    return ApplyRequest(
        document=DocumentRevision(
            scope=ScopeKey(tenant_id=1, kb_id="kb-ops"), document_id="doc-1",
            revision=1, content_hash="content-hash", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="ch1"),),
        config=IndexConfig(
            config_digest="digest", engine_version="semantica-0.6.8",
            model_profile_ref="profile", prompt_version="p1",
            rule_set_version="r1", schema_version="s1"),
        idempotency_key="idem-1",
        payload_hash="hash-1",
    )


@pytest.fixture()
def claimed_operation(operation_store, apply_request):
    operation_store.accept(apply_request)
    operation = operation_store.claim(worker_id="worker-test", lease_seconds=30)
    assert operation is not None
    return operation


def _free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


@pytest.fixture()
def rpc_client(tmp_path):
    ca = trustme.CA()
    server_cert = ca.issue_cert("127.0.0.1", "localhost")
    cert_path = tmp_path / "server.pem"
    key_path = tmp_path / "server.key"
    ca_path = tmp_path / "ca.pem"
    server_cert.private_key_pem.write_to_path(str(key_path))
    server_cert.cert_chain_pems[0].write_to_path(str(cert_path))
    ca.cert_pem.write_to_path(str(ca_path))

    port = _free_port()
    address = f"127.0.0.1:{port}"
    config = SemanticServiceConfig(
        address=address,
        internal_token=INTERNAL_TOKEN,
        tls_cert_path=str(cert_path),
        tls_key_path=str(key_path),
    )
    server = create_server(config)
    channels: list[grpc.Channel] = []
    try:
        server.start()

        roots = grpc.ssl_channel_credentials(root_certificates=ca_path.read_bytes())
        token_call_creds = grpc.metadata_call_credentials(
            lambda context, callback: callback((("x-semantic-token", INTERNAL_TOKEN),), None)
        )
        authenticated_creds = grpc.composite_channel_credentials(roots, token_call_creds)

        authenticated_channel = grpc.secure_channel(address, authenticated_creds)
        unauthenticated_channel = grpc.secure_channel(address, roots)
        health_channel = grpc.secure_channel(address, authenticated_creds)
        channels.extend([authenticated_channel, unauthenticated_channel, health_channel])

        grpc.channel_ready_future(authenticated_channel).result(timeout=10)

        def open_channel(credentials):
            """Open an extra tracked channel (closed with the fixture)."""
            channel = grpc.secure_channel(address, credentials)
            channels.append(channel)
            return channel

        yield SimpleNamespace(
            server=server,
            address=address,
            token=INTERNAL_TOKEN,
            ca_cert_bytes=ca_path.read_bytes(),
            open_channel=open_channel,
            authenticated=pb_grpc.SemanticStub(authenticated_channel),
            unauthenticated=pb_grpc.SemanticStub(unauthenticated_channel),
            health=health_pb2_grpc.HealthStub(health_channel),
        )
    finally:
        server.stop(grace=None).wait()
        for channel in channels:
            channel.close()
