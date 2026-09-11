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

    # Business RPCs stay UNIMPLEMENTED until their owning tasks land
    # (the bare generated SemanticServicer returns UNIMPLEMENTED for every
    # method - never fake success).
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
