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
from semantic_service.contracts import apply_request_to_wire


@dataclass(frozen=True)
class LegacyOperationSchema:
    dsn: str
    schema: str

    def new_store(self):
        from semantic_service.operations import PostgresOperationStore
        return PostgresOperationStore(self.dsn, self.schema)

    def seed_from_unversioned_operations_table(self, request: ApplyRequest) -> str:
        from uuid import uuid4
        migration = (Path(__file__).parent.parent / "migrations" / "001_operations.sql").read_text()
        operation_id = str(uuid4())
        with psycopg.connect(self.dsn) as connection:
            with connection.cursor() as cursor:
                cursor.execute(sql.SQL("CREATE SCHEMA {}").format(sql.Identifier(self.schema)))
                cursor.execute(migration.replace("{{schema}}", sql.Identifier(self.schema).as_string(connection)))
                cursor.execute(sql.SQL("""
                    INSERT INTO {}.operations
                      (operation_id, tenant_id, kb_id, document_id, revision, config_digest, idempotency_key, payload_hash,
                       phase, request_bytes, lease_owner, lease_until, lease_token)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'running', %s, 'legacy-worker', clock_timestamp() + interval '30 seconds', 1)
                """).format(sql.Identifier(self.schema)),
                               (operation_id, request.document.scope.tenant_id, request.document.scope.kb_id,
                                request.document.document_id, request.document.revision, request.config.config_digest,
                                request.idempotency_key, request.payload_hash,
                                apply_request_to_wire(request).SerializeToString(deterministic=True)))
        return operation_id


@dataclass(frozen=True)
class UnmigratedOperationSchema:
    dsn: str
    schema: str

    def new_store(self):
        from semantic_service.operations import PostgresOperationStore
        return PostgresOperationStore(self.dsn, self.schema)

    def schema_version(self) -> int:
        with psycopg.connect(self.dsn) as connection:
            return connection.execute(sql.SQL("SELECT max(version) FROM {}.schema_migrations").format(sql.Identifier(self.schema))).fetchone()[0]


@dataclass(frozen=True)
class TransitionalV1OperationSchema(UnmigratedOperationSchema):
    def seed_versioned_v1_with_stage_and_fence(self, request: ApplyRequest) -> str:
        from uuid import uuid4
        migration = (Path(__file__).parent.parent / "migrations" / "001_operations.sql").read_text()
        operation_id = str(uuid4())
        identifier = sql.Identifier(self.schema)
        with psycopg.connect(self.dsn) as connection:
            with connection.cursor() as cursor:
                cursor.execute(sql.SQL("CREATE SCHEMA {}").format(identifier))
                cursor.execute(migration.replace("{{schema}}", identifier.as_string(connection)))
                cursor.execute(sql.SQL("ALTER TABLE {}.operations ADD COLUMN stage TEXT NOT NULL DEFAULT 'running'").format(identifier))
                cursor.execute(sql.SQL("ALTER TABLE {}.operations ADD CONSTRAINT operations_lease_token_uint64_check CHECK (lease_token >= 0 AND lease_token <= 18446744073709551615)").format(identifier))
                cursor.execute(sql.SQL("CREATE TABLE {}.schema_migrations (version INTEGER PRIMARY KEY)").format(identifier))
                cursor.execute(sql.SQL("INSERT INTO {}.schema_migrations (version) VALUES (1)").format(identifier))
                cursor.execute(sql.SQL("""
                    INSERT INTO {}.operations
                      (operation_id, tenant_id, kb_id, document_id, revision, config_digest, idempotency_key, payload_hash,
                       phase, stage, request_bytes, lease_owner, lease_until, lease_token)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'running', 'running', %s, 'legacy-worker', clock_timestamp() + interval '30 seconds', 1)
                """).format(identifier),
                               (operation_id, request.document.scope.tenant_id, request.document.scope.kb_id,
                                request.document.document_id, request.document.revision, request.config.config_digest,
                                request.idempotency_key, request.payload_hash,
                                apply_request_to_wire(request).SerializeToString(deterministic=True)))
        return operation_id


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
def legacy_operation_schema():
    dsn = os.environ.get("SEMANTIC_TEST_POSTGRES_DSN")
    if not dsn:
        pytest.fail("SEMANTIC_TEST_POSTGRES_DSN is required")
    fixture = LegacyOperationSchema(dsn, f"semantic_legacy_{secrets.token_hex(8)}")
    try:
        yield fixture
    finally:
        with psycopg.connect(dsn) as connection:
            connection.execute(sql.SQL("DROP SCHEMA IF EXISTS {} CASCADE").format(sql.Identifier(fixture.schema)))


@pytest.fixture
def unmigrated_operation_schema():
    dsn = os.environ.get("SEMANTIC_TEST_POSTGRES_DSN")
    if not dsn:
        pytest.fail("SEMANTIC_TEST_POSTGRES_DSN is required")
    fixture = UnmigratedOperationSchema(dsn, f"semantic_unmigrated_{secrets.token_hex(8)}")
    try:
        yield fixture
    finally:
        with psycopg.connect(dsn) as connection:
            connection.execute(sql.SQL("DROP SCHEMA IF EXISTS {} CASCADE").format(sql.Identifier(fixture.schema)))


@pytest.fixture
def transitional_v1_operation_schema():
    dsn = os.environ.get("SEMANTIC_TEST_POSTGRES_DSN")
    if not dsn:
        pytest.fail("SEMANTIC_TEST_POSTGRES_DSN is required")
    fixture = TransitionalV1OperationSchema(dsn, f"semantic_transitional_{secrets.token_hex(8)}")
    try:
        yield fixture
    finally:
        with psycopg.connect(dsn) as connection:
            connection.execute(sql.SQL("DROP SCHEMA IF EXISTS {} CASCADE").format(sql.Identifier(fixture.schema)))


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
