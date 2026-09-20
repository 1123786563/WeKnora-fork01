"""Opt-in Semantica gRPC server; methods stay UNIMPLEMENTED until later tasks."""

from __future__ import annotations

from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from semantic_service.auth import ServiceIdentityInterceptor
from semantic_service.config import SemanticServiceConfig
from semantic_service.proto import semantic_pb2_grpc


class _UnimplementedSemanticService(semantic_pb2_grpc.SemanticServiceServicer):
    """C02 authenticates the transport; later tasks own RPC behavior."""


def create_server(config: SemanticServiceConfig) -> grpc.Server:
    private_key = config.tls_private_key_path.read_bytes()
    certificate = config.tls_certificate_path.read_bytes()
    credentials = grpc.ssl_server_credentials(((private_key, certificate),))
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=config.max_workers),
        interceptors=(ServiceIdentityInterceptor(config),),
    )
    semantic_pb2_grpc.add_SemanticServiceServicer_to_server(_UnimplementedSemanticService(), server)
    health_servicer = health.HealthServicer()
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_servicer.set("weknora.semantic.v1.SemanticService", health_pb2.HealthCheckResponse.NOT_SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    bound_port = server.add_secure_port(config.address, credentials)
    if bound_port == 0:
        raise RuntimeError("semantic gRPC server failed to bind TLS listener")
    return server


def main() -> None:
    config = SemanticServiceConfig.from_env()
    server = create_server(config)
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    main()
