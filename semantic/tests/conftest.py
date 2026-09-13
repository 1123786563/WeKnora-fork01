"""Shared fixtures for the semantic service test suite (C02+).

rpc_client starts a REAL gRPC server on a random local port with a test-only
trustme certificate and internal service token, plus authenticated /
unauthenticated client stubs. Nothing here is mocked at the transport layer.
"""

import os
import socket
import tempfile
from pathlib import Path
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


@pytest.fixture()
def deletion_service(pg_dsn, operation_store_factory):
    """Shared A02/Q01 fact-store fixture over the real PG (per-test clean)."""
    import psycopg

    from semantic_service.indexing.deletion import DeletionService
    from semantic_service.operations import apply_migrations

    apply_migrations(pg_dsn)
    migration = Path(__file__).resolve().parents[1] / "migrations" / "003_deletion_receipts.sql"
    with psycopg.connect(pg_dsn) as conn:
        conn.execute(migration.read_text(encoding="utf-8"))
        conn.execute("DELETE FROM semantic.deletion_receipts")
        conn.execute("DELETE FROM semantic.assertions")
        conn.execute("DELETE FROM semantic.tombstones")
        conn.commit()
    operations = operation_store_factory()
    operations.clear_for_test()
    service = DeletionService(pg_dsn, operations)
    yield service
    service.close()


@pytest.fixture()
def published_generation(generation_index_store, pg_dsn):
    """Q01: publish a complete generation for the search scope."""
    import psycopg

    from semantic_service.contracts import ScopeKey
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    scope = ScopeKey(tenant_id=1, kb_id="kb-q01")
    manifest = IndexManifest(
        scope=scope, generation="gen-q01", base_generation=None,
        documents={"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h1", artifact_ids=("a1",)),
                   "d2": DocumentEntry(document_id="d2", revision=1, content_hash="h2", artifact_ids=("a2",))},
        config_digest="d", artifacts=(ArtifactRef(kind="graph", artifact_id="a1", hash="h1"),),
        complete=True)
    with psycopg.connect(pg_dsn) as conn:
        conn.execute("DELETE FROM semantic.read_leases WHERE kb_id = %s", ("kb-q01",))
        conn.execute("DELETE FROM semantic.active_generations WHERE kb_id = %s", ("kb-q01",))
        conn.execute("DELETE FROM semantic.generations WHERE kb_id = %s", ("kb-q01",))
        conn.commit()
    generation_index_store.save_manifest(manifest)
    generation_index_store.publish(scope, None, manifest, lease_token=1)
    return manifest


@pytest.fixture()
def search_stack(pg_dsn, deletion_service, published_generation):
    """Seed Q01 facts so searches find evidence (imported by the RPC tests)."""
    from semantic_service.contracts import ScopeKey

    scope = ScopeKey(tenant_id=1, kb_id="kb-q01")
    deletion_service.add_assertion_support(scope, "f1", "ent-a", "controls",
        object_id="ent-b", support_document_id="d1", support_revision=1)
    deletion_service.add_assertion_support(scope, "f2", "ent-b", "owns",
        object_id="ent-c", support_document_id="d2", support_revision=1)
    return deletion_service


@pytest.fixture(scope="session")
def generation_index_store(pg_dsn):
    """The I03 IndexStore under a name that never collides with the
    function-scoped deletion index_store fixture in test_delete_races."""
    import psycopg

    from semantic_service.indexing.store import IndexStore
    from semantic_service.operations import apply_migrations

    apply_migrations(pg_dsn)
    migration = Path(__file__).resolve().parents[1] / "migrations" / "002_generations.sql"
    with psycopg.connect(pg_dsn) as conn:
        conn.execute(migration.read_text(encoding="utf-8"))
        conn.execute("INSERT INTO semantic.schema_migrations (version) VALUES ('002_generations') ON CONFLICT DO NOTHING")
        conn.commit()
    return IndexStore(pg_dsn)


@pytest.fixture(scope="session")
def service_pg_dsn(pg_dsn):
    """The SERVICE database (semantic schema) - same isolated container."""
    return pg_dsn


@pytest.fixture(scope="session")
def index_store(pg_dsn):
    """A real IndexStore over the isolated service PG; schema applied once."""
    import psycopg

    from semantic_service.indexing.store import IndexStore
    from semantic_service.operations import apply_migrations

    apply_migrations(pg_dsn)
    migration = Path(__file__).resolve().parents[1] / "migrations" / "002_generations.sql"
    with psycopg.connect(pg_dsn) as conn:
        conn.execute(migration.read_text(encoding="utf-8"))
        conn.execute(
            "INSERT INTO semantic.schema_migrations (version) VALUES ('002_generations') ON CONFLICT DO NOTHING")
        conn.commit()
    store = IndexStore(pg_dsn)
    yield store
    store.close()


@pytest.fixture(scope="session")
def operation_store_factory(pg_dsn):
    """Session-scoped factory producing per-test OperationStores."""
    from semantic_service.operations import OperationStore

    def _make():
        return OperationStore(pg_dsn)
    return _make


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
def operation_store(operation_store_factory):
    """A real OperationStore over the isolated PG, with per-test cleanup."""
    store = operation_store_factory()
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


# A03: the model gateway fixture compiles and runs the REAL Go internal
# handler server (go test -c binary) on a random local port, backed by a
# real migrated SQLite database and a controlled provider.
@pytest.fixture(scope="session")
def go_model_server():
    import socket
    import subprocess
    import time
    import urllib.request

    repo_root = Path(__file__).resolve().parents[2]
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    addr = f"127.0.0.1:{port}"
    binary = Path(tempfile.mkdtemp(prefix="semantic-model-server-")) / "server.test"
    build = subprocess.run(
        ["go", "test", "-c", "-o", str(binary), "./internal/handler"],
        cwd=repo_root, capture_output=True, text=True, timeout=600,
    )
    if build.returncode != 0:
        pytest.fail(f"go test -c failed: {build.stderr[-2000:]}")
    env = dict(os.environ, SEMANTIC_TEST_MODEL_ADDR=addr)
    proc = subprocess.Popen(
        [str(binary), "-test.run", "TestSemanticModelGatewayServer", "-test.v"],
        cwd=repo_root, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    base = f"http://{addr}"
    for _ in range(120):
        try:
            urllib.request.urlopen(base + "/internal/semantic/model/count?invocation_id=probe", timeout=1)
            break
        except Exception:
            if proc.poll() is not None:
                pytest.fail("Go model server exited during startup")
            time.sleep(0.5)
    else:
        proc.kill()
        pytest.fail("Go model server did not become ready")
    yield SimpleNamespace(url=base, token="model-test-token")
    proc.kill()
    proc.wait()


@pytest.fixture()
def model_gateway(go_model_server):
    from semantic_service.model_gateway import ModelGateway

    return ModelGateway(
        go_model_server.url,
        go_model_server.token,
        invocation_counter_url=go_model_server.url + "/internal/semantic/model/count",
    )


@pytest.fixture(scope="session")
def complete_manifest():
    from semantic_service.contracts import ScopeKey
    from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest

    return IndexManifest(
        scope=ScopeKey(tenant_id=1, kb_id="kb-gen"),
        generation="gen-1",
        base_generation=None,
        documents={"d1": DocumentEntry(document_id="d1", revision=1, content_hash="h1", artifact_ids=("art-1",))},
        config_digest="digest-1",
        artifacts=(ArtifactRef(kind="graph", artifact_id="art-1", hash="hash-1"),),
        complete=True,
    )


@pytest.fixture(autouse=True)
def clean_generations(request):
    """Per-test cleanup of generation tables so tests are independent."""
    module_name = getattr(getattr(request, "module", None), "__name__", "") or ""
    if "test_generation" not in module_name and "test_delete_races" not in module_name:
        yield
        return
    import psycopg

    def _clean():
        import contextlib

        with psycopg.connect(os.environ.get(PG_DSN_ENV, DEFAULT_PG_DSN)) as conn:
            # Setup may run before the deletion fixtures (re)create I04
            # tables - tolerate their absence on the first pass.
            for table in ("semantic.read_leases", "semantic.active_generations",
                          "semantic.generations", "semantic.deletion_receipts",
                          "semantic.tombstones", "semantic.assertions"):
                with contextlib.suppress(psycopg.errors.UndefinedTable):
                    conn.execute("DELETE FROM " + table)
            conn.commit()

    _clean()
    yield
    _clean()


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
