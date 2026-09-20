"""Transport-neutral C01 Semantica domain records."""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


UINT64_MAX = 2**64 - 1


def _uint(value: object, maximum: int, name: str, *, nonzero: bool = False) -> None:
    if type(value) is not int or value < (1 if nonzero else 0) or value > maximum:
        raise ValueError(f"{name} must be an unsigned integer")


@dataclass(frozen=True)
class ScopeKey:
    tenant_id: int
    kb_id: str

    def __post_init__(self) -> None:
        _uint(self.tenant_id, UINT64_MAX, "tenant_id")
        if not self.kb_id:
            raise ValueError("tenant_id must be uint64 and kb_id is required")


@dataclass(frozen=True)
class DocumentRevision:
    scope: ScopeKey
    document_id: str
    revision: int
    content_hash: str
    deleted: bool

    def __post_init__(self) -> None:
        _uint(self.revision, UINT64_MAX, "revision", nonzero=True)
        if not self.document_id:
            raise ValueError("revision must be nonzero uint64 and document_id is required")

    def require_delete(self) -> None:
        if not self.deleted:
            raise ValueError("semantic delete requires deleted=true")


@dataclass(frozen=True)
class DeleteDocumentRequest:
    revision: DocumentRevision

    @classmethod
    def from_revision(cls, revision: DocumentRevision) -> "DeleteDocumentRequest":
        revision.require_delete()
        return cls(revision=revision)


@dataclass(frozen=True)
class Evidence:
    evidence_id: str
    document_id: str
    revision: int
    chunk_id: str
    content_hash: str
    quote: str
    start_char: int | None
    end_char: int | None

    def __post_init__(self) -> None:
        _uint(self.revision, UINT64_MAX, "revision", nonzero=True)
        if not self.quote:
            raise ValueError("evidence quote and uint64 revision are required")
        if (self.start_char is None) != (self.end_char is None):
            raise ValueError("evidence span must be fully absent or present")
        if self.start_char is not None:
            _uint(self.start_char, 2**32 - 1, "start_char")
            _uint(self.end_char, 2**32 - 1, "end_char")
            if self.end_char <= self.start_char:
                raise ValueError("evidence span must be a positive half-open range")


@dataclass(frozen=True)
class AccessScope:
    scope: ScopeKey
    subject_id: str
    scope_ref: str
    scope_hash: str
    permission_epoch: int
    expires_at: str
    audience: str
    purpose: str
    budget_ref: str

    def __post_init__(self) -> None:
        _uint(self.permission_epoch, UINT64_MAX, "permission_epoch")
        if not self.subject_id or not self.scope_ref or not self.scope_hash or not self.expires_at or not self.audience or not self.purpose:
            raise ValueError("access scope identity fields are required")


@dataclass(frozen=True)
class OperationRef:
    scope: ScopeKey
    operation_id: str

    def __post_init__(self) -> None:
        if not self.operation_id:
            raise ValueError("operation_id is required with scope")


@dataclass(frozen=True)
class Operation:
    operation_id: str
    scope: ScopeKey
    document_id: str
    revision: int
    state: str
    stage: str
    lease_token: int = 0
    result_generation: str | None = None
    error_code: str | None = None

    def __post_init__(self) -> None:
        _uint(self.revision, UINT64_MAX, "revision", nonzero=True)
        _uint(self.lease_token, UINT64_MAX, "lease_token")
        if not self.operation_id or not self.document_id or not self.state or not self.stage:
            raise ValueError("operation fields are invalid")


@dataclass(frozen=True)
class QueryLimits:
    max_hops: int
    max_nodes: int
    max_edges: int
    top_k: int
    max_tokens: int
    deadline_ms: int

    def __post_init__(self) -> None:
        for name, value in self.__dict__.items():
            _uint(value, 2**32 - 1, name)


@dataclass(frozen=True)
class SearchRequest:
    query_id: str
    query: str
    access_scope: AccessScope
    limits: QueryLimits
    requested_mode: str

    def __post_init__(self) -> None:
        if not self.query_id or not self.query or self.requested_mode not in {"graph_rag", "reason"}:
            raise ValueError("search request is invalid")


@dataclass(frozen=True)
class ReasonRequest:
    search: SearchRequest
    reasoning_mode: str
    rule_set_version: str

    def __post_init__(self) -> None:
        if self.reasoning_mode not in {"rules", "model"} or not self.rule_set_version:
            raise ValueError("reason request is invalid")


@dataclass(frozen=True)
class SearchResponse:
    query_id: str
    generation: str
    requested_mode: str
    actual_mode: str
    evidence: tuple[Evidence, ...]
    assertion_ids: tuple[str, ...]
    paths: tuple[tuple[str, ...], ...] = ()
    stale: bool = False
    partial: bool = False
    truncated: bool = False

    def __post_init__(self) -> None:
        if not self.query_id or not self.generation or self.requested_mode not in {"graph_rag", "reason"} or self.actual_mode not in {"graph_rag", "reason"}:
            raise ValueError("search response is invalid")
        if self.requested_mode == "graph_rag" and self.actual_mode == "reason":
            raise ValueError("search cannot implicitly upgrade graph_rag to reason")


def evidence_to_wire(value: Evidence):
    from semantic_service.proto import semantic_pb2

    wire = semantic_pb2.Evidence(evidence_id=value.evidence_id, document_id=value.document_id, revision=value.revision, chunk_id=value.chunk_id, content_hash=value.content_hash, quote=value.quote)
    if value.start_char is not None:
        wire.start_char = value.start_char
        wire.end_char = value.end_char
    return wire


def evidence_from_wire(wire) -> Evidence:
    start_char = wire.start_char if wire.HasField("start_char") else None
    end_char = wire.end_char if wire.HasField("end_char") else None
    return Evidence(wire.evidence_id, wire.document_id, wire.revision, wire.chunk_id, wire.content_hash, wire.quote, start_char, end_char)


def access_scope_to_wire(value: AccessScope):
    from semantic_service.proto import semantic_pb2

    purposes = {"search": semantic_pb2.PURPOSE_SEARCH, "reason": semantic_pb2.PURPOSE_REASON, "index": semantic_pb2.PURPOSE_INDEX}
    return semantic_pb2.AccessScope(scope=semantic_pb2.ScopeKey(tenant_id=value.scope.tenant_id, kb_id=value.scope.kb_id), subject_id=value.subject_id, scope_ref=value.scope_ref, scope_hash=value.scope_hash, permission_epoch=value.permission_epoch, expires_at=value.expires_at, audience=value.audience, purpose=purposes[value.purpose], budget_ref=value.budget_ref)


def access_scope_from_wire(wire) -> AccessScope:
    from semantic_service.proto import semantic_pb2

    purposes = {semantic_pb2.PURPOSE_SEARCH: "search", semantic_pb2.PURPOSE_REASON: "reason", semantic_pb2.PURPOSE_INDEX: "index"}
    if wire.purpose not in purposes:
        raise ValueError("unknown access scope purpose")
    return AccessScope(ScopeKey(wire.scope.tenant_id, wire.scope.kb_id), wire.subject_id, wire.scope_ref, wire.scope_hash, wire.permission_epoch, wire.expires_at, wire.audience, purposes[wire.purpose], wire.budget_ref)


def operation_to_wire(value: Operation):
    from semantic_service.proto import semantic_pb2
    states = {"pending": semantic_pb2.OPERATION_STATE_PENDING, "running": semantic_pb2.OPERATION_STATE_RUNNING, "succeeded": semantic_pb2.OPERATION_STATE_SUCCEEDED, "failed": semantic_pb2.OPERATION_STATE_FAILED, "cancelled": semantic_pb2.OPERATION_STATE_CANCELLED}
    wire = semantic_pb2.Operation(operation_id=value.operation_id, scope=semantic_pb2.ScopeKey(tenant_id=value.scope.tenant_id, kb_id=value.scope.kb_id), document_id=value.document_id, revision=value.revision, state=states[value.state], stage=value.stage, lease_token=value.lease_token)
    if value.result_generation is not None: wire.result_generation = value.result_generation
    if value.error_code is not None: wire.error_code = value.error_code
    return wire


def operation_from_wire(wire) -> Operation:
    from semantic_service.proto import semantic_pb2
    states = {semantic_pb2.OPERATION_STATE_PENDING: "pending", semantic_pb2.OPERATION_STATE_RUNNING: "running", semantic_pb2.OPERATION_STATE_SUCCEEDED: "succeeded", semantic_pb2.OPERATION_STATE_FAILED: "failed", semantic_pb2.OPERATION_STATE_CANCELLED: "cancelled"}
    if wire.state not in states: raise ValueError("unknown operation state")
    return Operation(wire.operation_id, ScopeKey(wire.scope.tenant_id, wire.scope.kb_id), wire.document_id, wire.revision, states[wire.state], wire.stage, wire.lease_token, wire.result_generation if wire.HasField("result_generation") else None, wire.error_code if wire.HasField("error_code") else None)


def search_response_to_wire(value: SearchResponse):
    from semantic_service.proto import semantic_pb2
    modes = {"graph_rag": semantic_pb2.RETRIEVAL_MODE_GRAPH_RAG, "reason": semantic_pb2.RETRIEVAL_MODE_REASON}
    return semantic_pb2.SearchResponse(query_id=value.query_id, generation=value.generation, requested_mode=modes[value.requested_mode], actual_mode=modes[value.actual_mode], evidence=[evidence_to_wire(item) for item in value.evidence], assertion_ids=list(value.assertion_ids), paths=[semantic_pb2.StringPath(ids=list(path)) for path in value.paths], stale=value.stale, partial=value.partial, truncated=value.truncated)


def search_response_from_wire(wire) -> SearchResponse:
    from semantic_service.proto import semantic_pb2
    modes = {semantic_pb2.RETRIEVAL_MODE_GRAPH_RAG: "graph_rag", semantic_pb2.RETRIEVAL_MODE_REASON: "reason"}
    if wire.requested_mode not in modes or wire.actual_mode not in modes:
        raise ValueError("unknown retrieval mode")
    return SearchResponse(wire.query_id, wire.generation, modes[wire.requested_mode], modes[wire.actual_mode], tuple(evidence_from_wire(item) for item in wire.evidence), tuple(wire.assertion_ids), tuple(tuple(path.ids) for path in wire.paths), wire.stale, wire.partial, wire.truncated)


class ReasonStatus(str, Enum):
    DERIVED = "derived"
    INSUFFICIENT_EVIDENCE = "insufficient_evidence"
    CONFLICT = "conflict"
    UNAVAILABLE = "unavailable"


@dataclass(frozen=True)
class ReasonResponse:
    status: ReasonStatus
    conclusion: str | None
    retrieval: SearchResponse | None = None
    conclusion_kind: str | None = None
    premise_ids: tuple[str, ...] = ()
    rule_ids: tuple[str, ...] = ()
    model_version: str | None = None
    prompt_version: str | None = None
    limitations: tuple[str, ...] = ()
