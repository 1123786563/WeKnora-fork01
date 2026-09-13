"""Internal service authentication for the semantic service (C02).

Fail closed: every RPC - including the health service - must present the
approved internal service identity (constant-time byte comparison).
Transport-level TLS alone is NOT service identity. The plan's illustrative
verified_service_identity(context) helper was folded into the interceptor:
interception happens before any handler runs, so there is no
servicer-context-level check to duplicate.
"""

from __future__ import annotations

import hmac

import grpc

TOKEN_METADATA_KEY = "x-semantic-token"


def encode_token(value: str) -> bytes:
    """Encode a token for comparison; lossy on invalid sequences so a
    misconfigured surrogate-bearing token denies instead of raising."""
    return value.encode("utf-8", "replace")


def token_matches(presented: str, internal_token: str) -> bool:
    """Constant-time identity check over utf-8 bytes.

    Encoding first keeps compare_digest happy for non-ASCII metadata values
    (anonymous clients can send arbitrary bytes); the result is always a
    denial on mismatch, never a crash.
    """
    return bool(internal_token) and hmac.compare_digest(
        encode_token(presented), encode_token(internal_token))


class AuthInterceptor(grpc.ServerInterceptor):  # type: ignore[misc]
    """Denies every RPC without the approved service identity.

    Denial happens before any handler runs. The deny handler is
    unary-unary; all current business RPCs are unary. When the first
    streaming business RPC lands, the deny path must gain a
    stream-flag-matched handler (health.Watch today is still denied
    correctly because the framework rejects the type mismatch - see
    test_rpc_auth.py's Watch regression test).
    """

    def __init__(self, internal_token: str) -> None:
        if not internal_token:
            raise ValueError("internal_token is required for the auth interceptor")
        self._internal_token = internal_token
        self._internal_token_bytes = encode_token(internal_token)

    def intercept_service(self, continuation, handler_call_details):
        def _deny(_request, context):
            context.abort(grpc.StatusCode.UNAUTHENTICATED, "service identity required")

        metadata = dict(handler_call_details.invocation_metadata or ())
        presented = metadata.get(TOKEN_METADATA_KEY, "")
        if not hmac.compare_digest(encode_token(presented), self._internal_token_bytes):
            return grpc.unary_unary_rpc_method_handler(_deny)
        return continuation(handler_call_details)
