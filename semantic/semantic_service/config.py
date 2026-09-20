"""Configuration for the opt-in internal Semantica gRPC service."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class SemanticServiceConfig:
    address: str
    tls_certificate_path: Path
    tls_private_key_path: Path
    service_token: str
    audience: str
    max_workers: int = 8

    def __post_init__(self) -> None:
        if not self.address or not self.service_token or not self.audience:
            raise ValueError("semantic address, service identity token, and audience are required")
        if self.max_workers < 1:
            raise ValueError("max_workers must be positive")
        if not self.tls_certificate_path.is_file() or not self.tls_private_key_path.is_file():
            raise ValueError("semantic service TLS certificate and key must exist")

    @classmethod
    def from_env(cls) -> "SemanticServiceConfig":
        cert = os.environ.get("SEMANTIC_TLS_CERTIFICATE")
        key = os.environ.get("SEMANTIC_TLS_PRIVATE_KEY")
        token = os.environ.get("SEMANTIC_SERVICE_TOKEN")
        audience = os.environ.get("SEMANTIC_SERVICE_AUDIENCE")
        address = os.environ.get("SEMANTIC_LISTEN_ADDRESS")
        if not all((cert, key, token, audience, address)):
            raise ValueError("semantic service requires explicit address, TLS files, service token, and audience")
        return cls(address, Path(cert), Path(key), token, audience)
