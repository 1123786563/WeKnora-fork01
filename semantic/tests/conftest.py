from __future__ import annotations

import shutil
import socket
import subprocess
import os
import secrets
from dataclasses import dataclass
from pathlib import Path

import grpc
import pytest
import psycopg
from psycopg import sql
from grpc_health.v1 import health_pb2_grpc

from semantic_service.config import SemanticServiceConfig
from semantic_service.server import create_server
from semantic_service.proto import semantic_pb2_grpc
from semantic_service.contracts import ApplyRequest, ChunkSnapshot, DocumentRevision, IndexConfig, ScopeKey


@dataclass
class RPCClient:
    unauthenticated: object
    authenticated: object
    health: object
    channel: grpc.Channel


@pytest.fixture
def apply_request() -> ApplyRequest:
    return ApplyRequest(
        document=DocumentRevision(ScopeKey(tenant_id=42, kb_id="knowledge-base"), "document", 7, "document-hash", False),
        chunks=(ChunkSnapshot("chunk-1", "test content", "chunk-hash"),),
        config=IndexConfig("config-hash", "engine-v1", "model-profile", "prompt-v1", "rules-v1", "schema-v1"),
        idempotency_key="idempotency-key",
        payload_hash="payload-hash",
    )


@pytest.fixture
def operation_store_factory():
    dsn = os.environ.get("SEMANTIC_TEST_POSTGRES_DSN")
    if not dsn:
        pytest.fail("SEMANTIC_TEST_POSTGRES_DSN is required")
    schema = f"semantic_test_{secrets.token_hex(8)}"
    from semantic_service.operations import PostgresOperationStore

    store = PostgresOperationStore(dsn, schema)
    store.migrate()
    try:
        yield lambda: PostgresOperationStore(dsn, schema)
    finally:
        with psycopg.connect(dsn) as connection:
            with connection.cursor() as cursor:
                cursor.execute(sql.SQL("DROP SCHEMA {} CASCADE").format(sql.Identifier(schema)))


@pytest.fixture
def operation_store(operation_store_factory):
    return operation_store_factory()


@pytest.fixture
def rpc_client(tmp_path: Path):
    if shutil.which("openssl") is None:
        pytest.fail("openssl is required to create an ephemeral C02 test certificate")
    key_path = tmp_path / "server.key"
    cert_path = tmp_path / "server.crt"
    subprocess.run([
        "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
        "-keyout", str(key_path), "-out", str(cert_path), "-subj", "/CN=localhost",
        "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1",
    ], check=True, capture_output=True)
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    address = f"127.0.0.1:{port}"
    config = SemanticServiceConfig(address=address, tls_certificate_path=cert_path, tls_private_key_path=key_path, service_token="test-service-token", audience="weknora-semantic")
    server = create_server(config)
    server.start()
    channel = grpc.secure_channel(address, grpc.ssl_channel_credentials(root_certificates=cert_path.read_bytes()))
    stub = semantic_pb2_grpc.SemanticServiceStub(channel)
    health_stub = health_pb2_grpc.HealthStub(channel)
    try:
        yield RPCClient(unauthenticated=stub, authenticated=stub, health=health_stub, channel=channel)
    finally:
        channel.close()
        server.stop(grace=0).wait()
