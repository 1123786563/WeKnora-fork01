"""Transport-neutral C01 Semantica domain records."""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


UINT64_MAX = 2**64 - 1


@dataclass(frozen=True)
class ScopeKey:
    tenant_id: int
    kb_id: str

    def __post_init__(self) -> None:
        if not 0 <= self.tenant_id <= UINT64_MAX or not self.kb_id:
            raise ValueError("tenant_id must be uint64 and kb_id is required")


@dataclass(frozen=True)
class DocumentRevision:
    scope: ScopeKey
    document_id: str
    revision: int
    content_hash: str
    deleted: bool

    def __post_init__(self) -> None:
        if not 1 <= self.revision <= UINT64_MAX or not self.document_id:
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
        if not self.quote or not 1 <= self.revision <= UINT64_MAX:
            raise ValueError("evidence quote and uint64 revision are required")
        if (self.start_char is None) != (self.end_char is None):
            raise ValueError("evidence span must be fully absent or present")
        if self.start_char is not None and (self.start_char < 0 or self.end_char <= self.start_char):
            raise ValueError("evidence span must be a positive half-open range")


@dataclass(frozen=True)
class AccessScope:
    scope: ScopeKey
    subject_id: str
    scope_ref: str
    scope_hash: str
    permission_epoch: int
    audience: str
    purpose: str
    budget_ref: str

    def __post_init__(self) -> None:
        if not self.subject_id or not self.scope_ref or not self.scope_hash or not self.audience or not self.purpose or self.permission_epoch < 0:
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
        if not self.operation_id or not self.document_id or not self.state or not self.stage or not 1 <= self.revision <= UINT64_MAX or not 0 <= self.lease_token <= UINT64_MAX:
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
        if any(value < 0 for value in (self.max_hops, self.max_nodes, self.max_edges, self.top_k, self.max_tokens, self.deadline_ms)):
            raise ValueError("query limits must be non-negative")


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
