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


class ReasonStatus(str, Enum):
    DERIVED = "derived"
    INSUFFICIENT_EVIDENCE = "insufficient_evidence"
    CONFLICT = "conflict"
    UNAVAILABLE = "unavailable"


@dataclass(frozen=True)
class ReasonResponse:
    status: ReasonStatus
    conclusion: str | None
