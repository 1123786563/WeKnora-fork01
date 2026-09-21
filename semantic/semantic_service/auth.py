"""Internal service identity interceptor for the Semantica gRPC boundary."""

from __future__ import annotations

import hmac

import grpc

from semantic_service.config import SemanticServiceConfig


class ServiceIdentityInterceptor(grpc.ServerInterceptor):
    def __init__(self, config: SemanticServiceConfig) -> None:
        self._token = config.service_token
        self._audience = config.audience

    def intercept_service(self, continuation, handler_call_details):
        handler = continuation(handler_call_details)
        if handler is None:
            return None

        def authorized(context: grpc.ServicerContext) -> bool:
            metadata = {item.key.lower(): item.value for item in context.invocation_metadata()}
            authorization = metadata.get("authorization", "")
            scheme, _, token = authorization.partition(" ")
            return (
                scheme.lower() == "bearer"
                and hmac.compare_digest(token, self._token)
                and metadata.get("x-weknora-audience") == self._audience
            )

        def deny(request, context):
            if not authorized(context):
                context.abort(grpc.StatusCode.UNAUTHENTICATED, "service identity required")
            return handler.unary_unary(request, context)

        if handler.unary_unary:
            return grpc.unary_unary_rpc_method_handler(
                deny,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )
        if handler.unary_stream:
            def deny_stream(request, context):
                if not authorized(context):
                    context.abort(grpc.StatusCode.UNAUTHENTICATED, "service identity required")
                yield from handler.unary_stream(request, context)
            return grpc.unary_stream_rpc_method_handler(
                deny_stream,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )
        return handler
