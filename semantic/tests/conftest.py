from __future__ import annotations

import shutil
import socket
import subprocess
from dataclasses import dataclass
from pathlib import Path

import grpc
import pytest
from grpc_health.v1 import health_pb2_grpc

from semantic_service.config import SemanticServiceConfig
from semantic_service.server import create_server
from semantic_service.proto import semantic_pb2_grpc


@dataclass
class RPCClient:
    unauthenticated: object
    authenticated: object
    health: object
    channel: grpc.Channel


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
