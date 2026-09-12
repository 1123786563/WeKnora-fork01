"""gRPC entry point for the semantic service (C02 skeleton).

The C02 skeleton serves: TLS + internal-token authentication, the standard
health service, and UNIMPLEMENTED business RPCs - unimplemented methods
must never fake success. Later tasks (I01+, Q01+) implement the servicers.
"""

from __future__ import annotations

import grpc
from concurrent import futures
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from .auth import AuthInterceptor
from .config import SemanticServiceConfig
from .proto import semantic_pb2_grpc


def create_server(config: SemanticServiceConfig) -> grpc.Server:
    config.validate()

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=8),
        interceptors=[AuthInterceptor(config.internal_token)],
    )

    # Health readiness reflects the process state truthfully; this skeleton
    # has no external dependencies yet, so SERVING is honest. Tasks that add
    # dependencies must gate this status on them.
    health_servicer = health.HealthServicer()
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    # Business RPCs: Search is live when the query stack is provided
    # (Q01); every other method stays UNIMPLEMENTED (never fake success).
    if getattr(config, "query_dsn", None):
        from .indexing.deletion import DeletionService
        from .operations import OperationStore
        from .servicer import SemanticServicer

        operations = OperationStore(config.query_dsn)
        index_store = _build_index_store(config)
        deletion = DeletionService(config.query_dsn, operations)
        search_factory = _build_search_factory(config, index_store, deletion)
        semantic_pb2_grpc.add_SemanticServicer_to_server(SemanticServicer(search_factory), server)
    else:
        semantic_pb2_grpc.add_SemanticServicer_to_server(
            semantic_pb2_grpc.SemanticServicer(), server)

    if config.tls_cert_path and config.tls_key_path:
        with open(config.tls_cert_path, "rb") as cert_file:
            certificate_chain = cert_file.read()
        with open(config.tls_key_path, "rb") as key_file:
            private_key = key_file.read()
        credentials = grpc.ssl_server_credentials([(private_key, certificate_chain)])
        bound = server.add_secure_port(config.address, credentials)
    else:
        if not config.allow_plaintext:
            raise ValueError("refusing to start a plaintext server outside local tests")
        bound = server.add_insecure_port(config.address)
    if bound == 0:
        raise ValueError(f"failed to bind {config.address}")
    return server


def _build_index_store(config):
    from .indexing.store import IndexStore

    return IndexStore(config.query_dsn)


def _build_search_factory(config, index_store, deletion):
    """PER-REQUEST search stack: the access graph binds to the request's
    own scope (the servicer passes it), so tenant/kb SQL scoping is always
    the caller's - never a server-wide default."""
    from .pg_access import PgAccessGraph
    from .query.adapter import FrozenGraphRAGAdapter
    from .query.search import SearchService

    def factory(scope):
        graph = PgAccessGraph(config.query_dsn, scope, deletion)
        return SearchService(index_store, graph, FrozenGraphRAGAdapter())
    return factory
