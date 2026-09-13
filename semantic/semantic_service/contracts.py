"""Domain contracts for the WeKnora semantic service (C01).

Frozen dataclasses mirroring semantic/proto/semantic.proto one field at a
time. Business code uses these DTOs only; protobuf objects stay behind the
to_wire/from_wire mapping layer. JSON helpers implement the public boundary
convention: 64-bit integers are decimal strings and optional spans are
decimal strings or null, so JavaScript/TypeScript consumers never round
uint64 through float64.

Mapping rules:
- unknown wire enum values map to the explicit "unspecified" domain value,
  never to a success value;
- unknown domain enum strings raise ValueError instead of being silently
  encoded as unspecified;
- mutable collections are copied at construction (frozen dataclasses).
"""

from __future__ import annotations

import re
from dataclasses import dataclass, replace
from typing import Optional

from .proto import semantic_pb2 as pb

__all__ = [
    "replace",
    "ScopeKey",
    "DocumentRevision",
    "ChunkSnapshot",
    "IndexConfig",
    "ApplyRequest",
    "Operation",
    "Evidence",
    "Assertion",
    "AccessScope",
    "QueryLimits",
    "SearchRequest",
    "SearchResponse",
    "ReasonRequest",
    "ReasonResponse",
    "Capability",
    "Capabilities",
]


_CANONICAL_DECIMAL = re.compile(r"\A[0-9]+\Z")
_UINT32_MAX = 2 ** 32 - 1


def _as_int(value) -> int:
    """Accept int or a canonical decimal string at the JSON boundary.

    Non-canonical strings (whitespace, signs, hex, underscores) and floats
    are rejected so uint64 values never pass through a lossy representation.
    Plain ints are accepted for Python-internal callers; the Go side (json
    `,string` tags) requires strings - the shared fixture uses strings.
    """
    if isinstance(value, bool):
        raise ValueError("boolean is not a valid integer field")
    if isinstance(value, int):
        return value
    if isinstance(value, str) and _CANONICAL_DECIMAL.match(value):
        return int(value)
    raise ValueError(f"expected int or canonical decimal string, got {value!r}")


def _as_uint(value, maximum: int, what: str) -> int:
    parsed = _as_int(value)
    if not 0 <= parsed <= maximum:
        raise ValueError(f"{what} out of range [0, {maximum}]: {parsed}")
    return parsed


# ---------------------------------------------------------------------------
# Domain DTOs
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ScopeKey:
    tenant_id: int
    kb_id: str

    def __post_init__(self) -> None:
        _as_uint(self.tenant_id, 2 ** 64 - 1, "tenant_id")


@dataclass(frozen=True)
class DocumentRevision:
    scope: ScopeKey
    document_id: str
    revision: int
    content_hash: str
    deleted: bool

    def __post_init__(self) -> None:
        if self.revision < 0:
            raise ValueError(f"revision must be a uint64, got {self.revision}")


@dataclass(frozen=True)
class ChunkSnapshot:
    chunk_id: str
    text: str
    content_hash: str


@dataclass(frozen=True)
class IndexConfig:
    config_digest: str
    engine_version: str
    model_profile_ref: str
    prompt_version: str
    rule_set_version: str
    schema_version: str


@dataclass(frozen=True)
class ApplyRequest:
    document: DocumentRevision
    chunks: tuple[ChunkSnapshot, ...] = ()
    manifest_ref: Optional[str] = None
    config: Optional[IndexConfig] = None
    idempotency_key: str = ""
    payload_hash: str = ""

    def __post_init__(self) -> None:
        object.__setattr__(self, "chunks", tuple(self.chunks))


@dataclass(frozen=True)
class Operation:
    operation_id: str
    scope: ScopeKey
    document_id: str
    revision: int
    state: str = "unspecified"
    stage: str = ""
    lease_token: int = 0
    result_generation: Optional[str] = None
    error_code: Optional[str] = None

    def __post_init__(self) -> None:
        if self.revision < 0:
            raise ValueError(f"revision must be a uint64, got {self.revision}")
        if self.lease_token < 0:
            raise ValueError(f"lease_token must be a uint64, got {self.lease_token}")


@dataclass(frozen=True)
class Evidence:
    evidence_id: str
    document_id: str
    revision: int
    chunk_id: str
    content_hash: str
    quote: str
    start_char: Optional[int] = None
    end_char: Optional[int] = None

    def __post_init__(self) -> None:
        for name in ("start_char", "end_char"):
            value = getattr(self, name)
            if value is not None:
                _as_uint(value, _UINT32_MAX, name)
        _as_uint(self.revision, 2 ** 64 - 1, "revision")


@dataclass(frozen=True)
class Assertion:
    assertion_id: str
    scope: ScopeKey
    subject_id: str
    predicate: str
    object_id: Optional[str] = None
    value: Optional[str] = None
    kind: str = "unspecified"
    evidence_ids: tuple[str, ...] = ()
    premise_ids: tuple[str, ...] = ()
    valid_from: Optional[str] = None
    valid_until: Optional[str] = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "evidence_ids", tuple(self.evidence_ids))
        object.__setattr__(self, "premise_ids", tuple(self.premise_ids))


@dataclass(frozen=True)
class AccessScope:
    scope: ScopeKey
    subject_id: str
    scope_ref: str
    scope_hash: str
    permission_epoch: int
    expires_at: str
    audience: str
    purpose: str = "unspecified"
    budget_ref: str = ""

    def __post_init__(self) -> None:
        if self.permission_epoch < 0:
            raise ValueError(f"permission_epoch must be a uint64, got {self.permission_epoch}")


@dataclass(frozen=True)
class QueryLimits:
    max_hops: int = 0
    max_nodes: int = 0
    max_edges: int = 0
    top_k: int = 0
    max_tokens: int = 0
    deadline_ms: int = 0


@dataclass(frozen=True)
class SearchRequest:
    query_id: str
    query: str
    access_scope: Optional[AccessScope] = None
    limits: Optional[QueryLimits] = None
    mode: str = ""


@dataclass(frozen=True)
class SearchResponse:
    query_id: str
    generation: str
    mode: str
    evidence: tuple[Evidence, ...] = ()
    assertion_ids: tuple[str, ...] = ()
    paths: tuple[tuple[str, ...], ...] = ()
    stale: bool = False
    partial: bool = False
    truncated: bool = False

    def __post_init__(self) -> None:
        object.__setattr__(self, "evidence", tuple(self.evidence))
        object.__setattr__(self, "assertion_ids", tuple(self.assertion_ids))
        object.__setattr__(self, "paths", tuple(tuple(path) for path in self.paths))


@dataclass(frozen=True)
class ReasonRequest:
    search: SearchRequest
    reasoning_mode: str = "unspecified"
    rule_set_version: str = ""


@dataclass(frozen=True)
class ReasonResponse:
    retrieval: SearchResponse
    status: str
    conclusion: str
    conclusion_kind: str = "unspecified"
    premise_ids: tuple[str, ...] = ()
    rule_ids: tuple[str, ...] = ()
    model_version: Optional[str] = None
    prompt_version: Optional[str] = None
    limitations: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "premise_ids", tuple(self.premise_ids))
        object.__setattr__(self, "rule_ids", tuple(self.rule_ids))
        object.__setattr__(self, "limitations", tuple(self.limitations))


@dataclass(frozen=True)
class Capability:
    mode: str
    available: bool
    unavailable_reason: str = ""


@dataclass(frozen=True)
class Capabilities:
    protocol_version: str
    engine_version: str
    capabilities: tuple[Capability, ...] = ()
    default_limits: Optional[QueryLimits] = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "capabilities", tuple(self.capabilities))


# ---------------------------------------------------------------------------
# Enum tables
# ---------------------------------------------------------------------------

_ASSERTION_KIND_TO_WIRE = {
    "unspecified": pb.Assertion.Kind.KIND_UNSPECIFIED,
    "source": pb.Assertion.Kind.KIND_SOURCE,
    "rule": pb.Assertion.Kind.KIND_RULE,
    "model": pb.Assertion.Kind.KIND_MODEL,
}
_ASSERTION_KIND_FROM_WIRE = {value: key for key, value in _ASSERTION_KIND_TO_WIRE.items()}

_PURPOSE_TO_WIRE = {
    "unspecified": pb.AccessScope.Purpose.PURPOSE_UNSPECIFIED,
    "search": pb.AccessScope.Purpose.PURPOSE_SEARCH,
    "reason": pb.AccessScope.Purpose.PURPOSE_REASON,
    "index": pb.AccessScope.Purpose.PURPOSE_INDEX,
}
_PURPOSE_FROM_WIRE = {value: key for key, value in _PURPOSE_TO_WIRE.items()}

_REASONING_MODE_TO_WIRE = {
    "unspecified": pb.ReasonRequest.ReasoningMode.REASONING_MODE_UNSPECIFIED,
    "rules": pb.ReasonRequest.ReasoningMode.REASONING_MODE_RULES,
    "model": pb.ReasonRequest.ReasoningMode.REASONING_MODE_MODEL,
}
_REASONING_MODE_FROM_WIRE = {value: key for key, value in _REASONING_MODE_TO_WIRE.items()}

_CONCLUSION_KIND_TO_WIRE = {
    "unspecified": pb.ReasonResponse.ConclusionKind.CONCLUSION_KIND_UNSPECIFIED,
    "rule": pb.ReasonResponse.ConclusionKind.CONCLUSION_KIND_RULE,
    "model": pb.ReasonResponse.ConclusionKind.CONCLUSION_KIND_MODEL,
}
_CONCLUSION_KIND_FROM_WIRE = {value: key for key, value in _CONCLUSION_KIND_TO_WIRE.items()}

_OPERATION_STATE_TO_WIRE = {
    "unspecified": pb.Operation.State.OPERATION_STATE_UNSPECIFIED,
    "accepted": pb.Operation.State.OPERATION_STATE_ACCEPTED,
    "running": pb.Operation.State.OPERATION_STATE_RUNNING,
    "staged": pb.Operation.State.OPERATION_STATE_STAGED,
    "publishing": pb.Operation.State.OPERATION_STATE_PUBLISHING,
    "succeeded": pb.Operation.State.OPERATION_STATE_SUCCEEDED,
    "failed": pb.Operation.State.OPERATION_STATE_FAILED,
    "cancelled": pb.Operation.State.OPERATION_STATE_CANCELLED,
    "superseded": pb.Operation.State.OPERATION_STATE_SUPERSEDED,
}
_OPERATION_STATE_FROM_WIRE = {value: key for key, value in _OPERATION_STATE_TO_WIRE.items()}


# Domain assertion kinds (derived from the wire enum table so C01 stays
# the single source of truth; "unspecified" is excluded on purpose).
ASSERTION_KINDS = frozenset(
    kind for kind in _ASSERTION_KIND_TO_WIRE if kind != "unspecified"
)


def _enum_to_wire(tables: dict, domain: str, what: str) -> int:
    try:
        return tables[domain]
    except KeyError as exc:
        raise ValueError(f"unknown {what}: {domain!r}") from exc


def _enum_from_wire(tables: dict, wire_value: int) -> str:
    # Unknown wire values deliberately degrade to "unspecified" instead of
    # any success value (spec section 7).
    return tables.get(int(wire_value), "unspecified")


# ---------------------------------------------------------------------------
# Wire mapping
# ---------------------------------------------------------------------------


def scope_to_wire(scope: ScopeKey) -> pb.ScopeKey:
    return pb.ScopeKey(tenant_id=scope.tenant_id, kb_id=scope.kb_id)


def scope_from_wire(message: pb.ScopeKey) -> ScopeKey:
    return ScopeKey(tenant_id=message.tenant_id, kb_id=message.kb_id)


def document_revision_to_wire(revision: DocumentRevision) -> pb.DocumentRevision:
    return pb.DocumentRevision(
        scope=scope_to_wire(revision.scope),
        document_id=revision.document_id,
        revision=revision.revision,
        content_hash=revision.content_hash,
        deleted=revision.deleted,
    )


def document_revision_from_wire(message: pb.DocumentRevision) -> DocumentRevision:
    return DocumentRevision(
        scope=scope_from_wire(message.scope),
        document_id=message.document_id,
        revision=message.revision,
        content_hash=message.content_hash,
        deleted=message.deleted,
    )


def chunk_to_wire(chunk: ChunkSnapshot) -> pb.ChunkSnapshot:
    return pb.ChunkSnapshot(chunk_id=chunk.chunk_id, text=chunk.text, content_hash=chunk.content_hash)


def chunk_from_wire(message: pb.ChunkSnapshot) -> ChunkSnapshot:
    return ChunkSnapshot(chunk_id=message.chunk_id, text=message.text, content_hash=message.content_hash)


def index_config_to_wire(config: IndexConfig) -> pb.IndexConfig:
    return pb.IndexConfig(
        config_digest=config.config_digest,
        engine_version=config.engine_version,
        model_profile_ref=config.model_profile_ref,
        prompt_version=config.prompt_version,
        rule_set_version=config.rule_set_version,
        schema_version=config.schema_version,
    )


def index_config_from_wire(message: pb.IndexConfig) -> IndexConfig:
    return IndexConfig(
        config_digest=message.config_digest,
        engine_version=message.engine_version,
        model_profile_ref=message.model_profile_ref,
        prompt_version=message.prompt_version,
        rule_set_version=message.rule_set_version,
        schema_version=message.schema_version,
    )


def apply_request_to_wire(request: ApplyRequest) -> pb.ApplyRequest:
    message = pb.ApplyRequest(
        document=document_revision_to_wire(request.document),
        chunks=[chunk_to_wire(chunk) for chunk in request.chunks],
        config=index_config_to_wire(request.config) if request.config is not None else None,
        idempotency_key=request.idempotency_key,
        payload_hash=request.payload_hash,
    )
    if request.manifest_ref is not None:
        message.manifest_ref = request.manifest_ref
    return message


def apply_request_from_wire(message: pb.ApplyRequest) -> ApplyRequest:
    return ApplyRequest(
        document=document_revision_from_wire(message.document),
        chunks=tuple(chunk_from_wire(chunk) for chunk in message.chunks),
        manifest_ref=message.manifest_ref if message.HasField("manifest_ref") else None,
        config=index_config_from_wire(message.config) if message.HasField("config") else None,
        idempotency_key=message.idempotency_key,
        payload_hash=message.payload_hash,
    )


def operation_to_wire(operation: Operation) -> pb.Operation:
    message = pb.Operation(
        operation_id=operation.operation_id,
        scope=scope_to_wire(operation.scope),
        document_id=operation.document_id,
        revision=operation.revision,
        state=_enum_to_wire(_OPERATION_STATE_TO_WIRE, operation.state, "operation state"),
        stage=operation.stage,
        lease_token=operation.lease_token,
    )
    if operation.result_generation is not None:
        message.result_generation = operation.result_generation
    if operation.error_code is not None:
        message.error_code = operation.error_code
    return message


def operation_from_wire(message: pb.Operation) -> Operation:
    return Operation(
        operation_id=message.operation_id,
        scope=scope_from_wire(message.scope),
        document_id=message.document_id,
        revision=message.revision,
        state=_enum_from_wire(_OPERATION_STATE_FROM_WIRE, message.state),
        stage=message.stage,
        lease_token=message.lease_token,
        result_generation=message.result_generation if message.HasField("result_generation") else None,
        error_code=message.error_code if message.HasField("error_code") else None,
    )


def evidence_to_wire(evidence: Evidence) -> pb.Evidence:
    message = pb.Evidence(
        evidence_id=evidence.evidence_id,
        document_id=evidence.document_id,
        revision=evidence.revision,
        chunk_id=evidence.chunk_id,
        content_hash=evidence.content_hash,
        quote=evidence.quote,
    )
    if evidence.start_char is not None:
        message.start_char = evidence.start_char
    if evidence.end_char is not None:
        message.end_char = evidence.end_char
    return message


def evidence_from_wire(message: pb.Evidence) -> Evidence:
    return Evidence(
        evidence_id=message.evidence_id,
        document_id=message.document_id,
        revision=message.revision,
        chunk_id=message.chunk_id,
        content_hash=message.content_hash,
        quote=message.quote,
        start_char=message.start_char if message.HasField("start_char") else None,
        end_char=message.end_char if message.HasField("end_char") else None,
    )


def assertion_to_wire(assertion: Assertion) -> pb.Assertion:
    message = pb.Assertion(
        assertion_id=assertion.assertion_id,
        scope=scope_to_wire(assertion.scope),
        subject_id=assertion.subject_id,
        predicate=assertion.predicate,
        kind=_enum_to_wire(_ASSERTION_KIND_TO_WIRE, assertion.kind, "assertion kind"),
        evidence_ids=list(assertion.evidence_ids),
        premise_ids=list(assertion.premise_ids),
    )
    if assertion.object_id is not None:
        message.object_id = assertion.object_id
    if assertion.value is not None:
        message.value = assertion.value
    if assertion.valid_from is not None:
        message.valid_from = assertion.valid_from
    if assertion.valid_until is not None:
        message.valid_until = assertion.valid_until
    return message


def assertion_from_wire(message: pb.Assertion) -> Assertion:
    return Assertion(
        assertion_id=message.assertion_id,
        scope=scope_from_wire(message.scope),
        subject_id=message.subject_id,
        predicate=message.predicate,
        object_id=message.object_id if message.HasField("object_id") else None,
        value=message.value if message.HasField("value") else None,
        kind=_enum_from_wire(_ASSERTION_KIND_FROM_WIRE, message.kind),
        evidence_ids=tuple(message.evidence_ids),
        premise_ids=tuple(message.premise_ids),
        valid_from=message.valid_from if message.HasField("valid_from") else None,
        valid_until=message.valid_until if message.HasField("valid_until") else None,
    )


def access_scope_to_wire(access: AccessScope) -> pb.AccessScope:
    return pb.AccessScope(
        scope=scope_to_wire(access.scope),
        subject_id=access.subject_id,
        scope_ref=access.scope_ref,
        scope_hash=access.scope_hash,
        permission_epoch=access.permission_epoch,
        expires_at=access.expires_at,
        audience=access.audience,
        purpose=_enum_to_wire(_PURPOSE_TO_WIRE, access.purpose, "access purpose"),
        budget_ref=access.budget_ref,
    )


def access_scope_from_wire(message: pb.AccessScope) -> AccessScope:
    return AccessScope(
        scope=scope_from_wire(message.scope),
        subject_id=message.subject_id,
        scope_ref=message.scope_ref,
        scope_hash=message.scope_hash,
        permission_epoch=message.permission_epoch,
        expires_at=message.expires_at,
        audience=message.audience,
        purpose=_enum_from_wire(_PURPOSE_FROM_WIRE, message.purpose),
        budget_ref=message.budget_ref,
    )


def query_limits_to_wire(limits: QueryLimits) -> pb.QueryLimits:
    return pb.QueryLimits(
        max_hops=limits.max_hops,
        max_nodes=limits.max_nodes,
        max_edges=limits.max_edges,
        top_k=limits.top_k,
        max_tokens=limits.max_tokens,
        deadline_ms=limits.deadline_ms,
    )


def query_limits_from_wire(message: pb.QueryLimits) -> QueryLimits:
    return QueryLimits(
        max_hops=message.max_hops,
        max_nodes=message.max_nodes,
        max_edges=message.max_edges,
        top_k=message.top_k,
        max_tokens=message.max_tokens,
        deadline_ms=message.deadline_ms,
    )


def search_request_to_wire(request: SearchRequest, header: Optional[pb.RequestHeader] = None) -> pb.SearchRequest:
    return pb.SearchRequest(
        header=header,
        query_id=request.query_id,
        query=request.query,
        access_scope=access_scope_to_wire(request.access_scope) if request.access_scope is not None else None,
        limits=query_limits_to_wire(request.limits) if request.limits is not None else None,
        mode=request.mode,
    )


def search_request_from_wire(message: pb.SearchRequest) -> SearchRequest:
    return SearchRequest(
        query_id=message.query_id,
        query=message.query,
        access_scope=access_scope_from_wire(message.access_scope) if message.HasField("access_scope") else None,
        limits=query_limits_from_wire(message.limits) if message.HasField("limits") else None,
        mode=message.mode,
    )


def search_response_to_wire(response: SearchResponse) -> pb.SearchResponse:
    return pb.SearchResponse(
        query_id=response.query_id,
        generation=response.generation,
        mode=response.mode,
        evidence=[evidence_to_wire(item) for item in response.evidence],
        assertion_ids=list(response.assertion_ids),
        paths=[pb.SearchResponse.Path(nodes=list(path)) for path in response.paths],
        stale=response.stale,
        partial=response.partial,
        truncated=response.truncated,
    )


def search_response_from_wire(message: pb.SearchResponse) -> SearchResponse:
    return SearchResponse(
        query_id=message.query_id,
        generation=message.generation,
        mode=message.mode,
        evidence=tuple(evidence_from_wire(item) for item in message.evidence),
        assertion_ids=tuple(message.assertion_ids),
        paths=tuple(tuple(path.nodes) for path in message.paths),
        stale=message.stale,
        partial=message.partial,
        truncated=message.truncated,
    )


def reason_request_to_wire(request: ReasonRequest, header: Optional[pb.RequestHeader] = None) -> pb.ReasonRequest:
    return pb.ReasonRequest(
        header=header,
        search=search_request_to_wire(request.search),
        reasoning_mode=_enum_to_wire(_REASONING_MODE_TO_WIRE, request.reasoning_mode, "reasoning mode"),
        rule_set_version=request.rule_set_version,
    )


def reason_request_from_wire(message: pb.ReasonRequest) -> ReasonRequest:
    return ReasonRequest(
        search=search_request_from_wire(message.search),
        reasoning_mode=_enum_from_wire(_REASONING_MODE_FROM_WIRE, message.reasoning_mode),
        rule_set_version=message.rule_set_version,
    )


def reason_response_to_wire(response: ReasonResponse) -> pb.ReasonResponse:
    message = pb.ReasonResponse(
        retrieval=search_response_to_wire(response.retrieval),
        status=response.status,
        conclusion=response.conclusion,
        conclusion_kind=_enum_to_wire(_CONCLUSION_KIND_TO_WIRE, response.conclusion_kind, "conclusion kind"),
        premise_ids=list(response.premise_ids),
        rule_ids=list(response.rule_ids),
        limitations=list(response.limitations),
    )
    if response.model_version is not None:
        message.model_version = response.model_version
    if response.prompt_version is not None:
        message.prompt_version = response.prompt_version
    return message


def reason_response_from_wire(message: pb.ReasonResponse) -> ReasonResponse:
    return ReasonResponse(
        retrieval=search_response_from_wire(message.retrieval),
        status=message.status,
        conclusion=message.conclusion,
        conclusion_kind=_enum_from_wire(_CONCLUSION_KIND_FROM_WIRE, message.conclusion_kind),
        premise_ids=tuple(message.premise_ids),
        rule_ids=tuple(message.rule_ids),
        model_version=message.model_version if message.HasField("model_version") else None,
        prompt_version=message.prompt_version if message.HasField("prompt_version") else None,
        limitations=tuple(message.limitations),
    )


def capabilities_to_wire(capabilities: Capabilities) -> pb.GetCapabilitiesResponse:
    return pb.GetCapabilitiesResponse(
        protocol_version=capabilities.protocol_version,
        engine_version=capabilities.engine_version,
        capabilities=[
            pb.Capability(mode=item.mode, available=item.available, unavailable_reason=item.unavailable_reason)
            for item in capabilities.capabilities
        ],
        default_limits=query_limits_to_wire(capabilities.default_limits) if capabilities.default_limits is not None else None,
    )


def capabilities_from_wire(message: pb.GetCapabilitiesResponse) -> Capabilities:
    return Capabilities(
        protocol_version=message.protocol_version,
        engine_version=message.engine_version,
        capabilities=tuple(
            Capability(mode=item.mode, available=item.available, unavailable_reason=item.unavailable_reason)
            for item in message.capabilities
        ),
        default_limits=query_limits_from_wire(message.default_limits) if message.HasField("default_limits") else None,
    )


# ---------------------------------------------------------------------------
# JSON boundary (64-bit integers as decimal strings)
# ---------------------------------------------------------------------------


def scope_to_json(scope: ScopeKey) -> dict:
    return {"tenant_id": str(scope.tenant_id), "kb_id": scope.kb_id}


def scope_from_json(data: dict) -> ScopeKey:
    return ScopeKey(tenant_id=_as_int(data["tenant_id"]), kb_id=data["kb_id"])


def document_revision_to_json(revision: DocumentRevision) -> dict:
    return {
        "scope": scope_to_json(revision.scope),
        "document_id": revision.document_id,
        "revision": str(revision.revision),
        "content_hash": revision.content_hash,
        "deleted": revision.deleted,
    }


def document_revision_from_json(data: dict) -> DocumentRevision:
    return DocumentRevision(
        scope=scope_from_json(data["scope"]),
        document_id=data["document_id"],
        revision=_as_int(data["revision"]),
        content_hash=data["content_hash"],
        deleted=bool(data["deleted"]),
    )


def evidence_to_json(evidence: Evidence) -> dict:
    return {
        "evidence_id": evidence.evidence_id,
        "document_id": evidence.document_id,
        "revision": str(evidence.revision),
        "chunk_id": evidence.chunk_id,
        "content_hash": evidence.content_hash,
        "quote": evidence.quote,
        "start_char": str(evidence.start_char) if evidence.start_char is not None else None,
        "end_char": str(evidence.end_char) if evidence.end_char is not None else None,
    }


def evidence_from_json(data: dict) -> Evidence:
    start = data.get("start_char")
    end = data.get("end_char")
    return Evidence(
        evidence_id=data["evidence_id"],
        document_id=data["document_id"],
        revision=_as_int(data["revision"]),
        chunk_id=data["chunk_id"],
        content_hash=data["content_hash"],
        quote=data["quote"],
        start_char=_as_int(start) if start is not None else None,
        end_char=_as_int(end) if end is not None else None,
    )


def assertion_from_json(data: dict) -> Assertion:
    return Assertion(
        assertion_id=data["assertion_id"],
        scope=scope_from_json(data["scope"]),
        subject_id=data["subject_id"],
        predicate=data["predicate"],
        object_id=data.get("object_id"),
        value=data.get("value"),
        kind=data.get("kind", "unspecified"),
        evidence_ids=tuple(data.get("evidence_ids", ())),
        premise_ids=tuple(data.get("premise_ids", ())),
        valid_from=data.get("valid_from"),
        valid_until=data.get("valid_until"),
    )


def access_scope_from_json(data: dict) -> AccessScope:
    return AccessScope(
        scope=scope_from_json(data["scope"]),
        subject_id=data["subject_id"],
        scope_ref=data["scope_ref"],
        scope_hash=data["scope_hash"],
        permission_epoch=_as_int(data["permission_epoch"]),
        expires_at=data["expires_at"],
        audience=data["audience"],
        purpose=data.get("purpose", "unspecified"),
        budget_ref=data.get("budget_ref", ""),
    )


def query_limits_from_json(data: dict) -> QueryLimits:
    return QueryLimits(
        max_hops=_as_int(data["max_hops"]),
        max_nodes=_as_int(data["max_nodes"]),
        max_edges=_as_int(data["max_edges"]),
        top_k=_as_int(data["top_k"]),
        max_tokens=_as_int(data["max_tokens"]),
        deadline_ms=_as_int(data["deadline_ms"]),
    )


def apply_request_from_json(data: dict) -> ApplyRequest:
    return ApplyRequest(
        document=document_revision_from_json(data["document"]),
        chunks=tuple(chunk_from_json(item) for item in data.get("chunks", ())),
        manifest_ref=data.get("manifest_ref"),
        config=index_config_from_json(data["config"]) if data.get("config") is not None else None,
        idempotency_key=data.get("idempotency_key", ""),
        payload_hash=data.get("payload_hash", ""),
    )


def chunk_from_json(data: dict) -> ChunkSnapshot:
    return ChunkSnapshot(chunk_id=data["chunk_id"], text=data["text"], content_hash=data["content_hash"])


def index_config_from_json(data: dict) -> IndexConfig:
    return IndexConfig(
        config_digest=data["config_digest"],
        engine_version=data["engine_version"],
        model_profile_ref=data["model_profile_ref"],
        prompt_version=data["prompt_version"],
        rule_set_version=data["rule_set_version"],
        schema_version=data["schema_version"],
    )


def search_response_from_json(data: dict) -> SearchResponse:
    return SearchResponse(
        query_id=data["query_id"],
        generation=data["generation"],
        mode=data["mode"],
        evidence=tuple(evidence_from_json(item) for item in data.get("evidence", ())),
        assertion_ids=tuple(data.get("assertion_ids", ())),
        paths=tuple(tuple(path) for path in data.get("paths", ())),
        stale=data.get("stale", False),
        partial=data.get("partial", False),
        truncated=data.get("truncated", False),
    )


def capabilities_from_json(data: dict) -> Capabilities:
    return Capabilities(
        protocol_version=data["protocol_version"],
        engine_version=data["engine_version"],
        capabilities=tuple(
            Capability(mode=item["mode"], available=item["available"], unavailable_reason=item.get("unavailable_reason", ""))
            for item in data.get("capabilities", ())
        ),
        default_limits=query_limits_from_json(data["default_limits"]) if data.get("default_limits") is not None else None,
    )


def assertion_to_json(assertion: Assertion) -> dict:
    return {
        "assertion_id": assertion.assertion_id,
        "scope": scope_to_json(assertion.scope),
        "subject_id": assertion.subject_id,
        "predicate": assertion.predicate,
        "object_id": assertion.object_id,
        "value": assertion.value,
        "kind": assertion.kind,
        "evidence_ids": list(assertion.evidence_ids),
        "premise_ids": list(assertion.premise_ids),
        "valid_from": assertion.valid_from,
        "valid_until": assertion.valid_until,
    }


def search_response_to_json(response: SearchResponse) -> dict:
    return {
        "query_id": response.query_id,
        "generation": response.generation,
        "mode": response.mode,
        "evidence": [evidence_to_json(item) for item in response.evidence],
        "assertion_ids": list(response.assertion_ids),
        "paths": [list(path) for path in response.paths],
        "stale": response.stale,
        "partial": response.partial,
        "truncated": response.truncated,
    }


def apply_request_to_json(request: ApplyRequest) -> dict:
    return {
        "document": document_revision_to_json(request.document),
        "chunks": [
            {"chunk_id": chunk.chunk_id, "text": chunk.text, "content_hash": chunk.content_hash}
            for chunk in request.chunks
        ],
        "manifest_ref": request.manifest_ref,
        "config": {
            "config_digest": request.config.config_digest,
            "engine_version": request.config.engine_version,
            "model_profile_ref": request.config.model_profile_ref,
            "prompt_version": request.config.prompt_version,
            "rule_set_version": request.config.rule_set_version,
            "schema_version": request.config.schema_version,
        } if request.config is not None else None,
        "idempotency_key": request.idempotency_key,
        "payload_hash": request.payload_hash,
    }


def capabilities_to_json(capabilities: Capabilities) -> dict:
    return {
        "protocol_version": capabilities.protocol_version,
        "engine_version": capabilities.engine_version,
        "capabilities": [
            {"mode": item.mode, "available": item.available, "unavailable_reason": item.unavailable_reason}
            for item in capabilities.capabilities
        ],
        "default_limits": {
            "max_hops": capabilities.default_limits.max_hops,
            "max_nodes": capabilities.default_limits.max_nodes,
            "max_edges": capabilities.default_limits.max_edges,
            "top_k": capabilities.default_limits.top_k,
            "max_tokens": capabilities.default_limits.max_tokens,
            "deadline_ms": capabilities.default_limits.deadline_ms,
        } if capabilities.default_limits is not None else None,
    }


def reason_request_from_json(data: dict) -> ReasonRequest:
    return ReasonRequest(
        search=search_request_from_json(data["search"]),
        reasoning_mode=data.get("reasoning_mode", "unspecified"),
        rule_set_version=data.get("rule_set_version", ""),
    )


def search_request_from_json(data: dict) -> SearchRequest:
    return SearchRequest(
        query_id=data["query_id"],
        query=data["query"],
        access_scope=access_scope_from_json(data["access_scope"]) if data.get("access_scope") is not None else None,
        limits=query_limits_from_json(data["limits"]) if data.get("limits") is not None else None,
        mode=data.get("mode", ""),
    )


def operation_from_json(data: dict) -> Operation:
    return Operation(
        operation_id=data["operation_id"],
        scope=scope_from_json(data["scope"]),
        document_id=data["document_id"],
        revision=_as_int(data["revision"]),
        state=data.get("state", "unspecified"),
        stage=data.get("stage", ""),
        lease_token=_as_int(data.get("lease_token", 0)),
        result_generation=data.get("result_generation"),
        error_code=data.get("error_code"),
    )


def reason_response_from_json(data: dict) -> ReasonResponse:
    retrieval = data["retrieval"]
    return ReasonResponse(
        retrieval=SearchResponse(
            query_id=retrieval["query_id"],
            generation=retrieval["generation"],
            mode=retrieval["mode"],
            evidence=tuple(evidence_from_json(item) for item in retrieval.get("evidence", ())),
            assertion_ids=tuple(retrieval.get("assertion_ids", ())),
            paths=tuple(tuple(path) for path in retrieval.get("paths", ())),
            stale=retrieval.get("stale", False),
            partial=retrieval.get("partial", False),
            truncated=retrieval.get("truncated", False),
        ),
        status=data["status"],
        conclusion=data["conclusion"],
        conclusion_kind=data.get("conclusion_kind", "unspecified"),
        premise_ids=tuple(data.get("premise_ids", ())),
        rule_ids=tuple(data.get("rule_ids", ())),
        model_version=data.get("model_version"),
        prompt_version=data.get("prompt_version"),
        limitations=tuple(data.get("limitations", ())),
    )
