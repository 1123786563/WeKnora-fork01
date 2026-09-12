"""Configuration for the semantic service (C02).

Production servers must fail closed: without TLS material or an internal
service token, create_server refuses to start. allow_plaintext exists only
for local test harnesses that cannot use TLS at all - it gates the
plaintext bind path, NOT self-signed certificates (the test suite uses
trustme certs WITH TLS and therefore does not need this flag).
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class SemanticServiceConfig:
    address: str
    internal_token: str
    tls_cert_path: str = ""
    tls_key_path: str = ""
    allow_plaintext: bool = False
    # Q01+: the query stack's control-DB DSN. Empty keeps the server in
    # skeleton mode (Search RPC stays UNIMPLEMENTED - never fake success).
    query_dsn: str = ""

    def validate(self) -> None:
        errors = []
        if not self.address:
            errors.append("address is required")
        if not self.internal_token:
            errors.append("internal_token is required (service identity)")
        has_tls_paths = bool(self.tls_cert_path or self.tls_key_path)
        if has_tls_paths:
            if not self.tls_cert_path or not self.tls_key_path:
                errors.append("tls_cert_path and tls_key_path must be provided together")
            else:
                if not Path(self.tls_cert_path).is_file():
                    errors.append(f"tls cert not found: {self.tls_cert_path}")
                if not Path(self.tls_key_path).is_file():
                    errors.append(f"tls key not found: {self.tls_key_path}")
        elif not self.allow_plaintext:
            errors.append("TLS cert/key paths are required (allow_plaintext is local-test-only)")
        if errors:
            raise ValueError("invalid semantic service config: " + "; ".join(errors))
