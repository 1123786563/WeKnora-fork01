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


class ReasonStatus(str, Enum):
    DERIVED = "derived"
    INSUFFICIENT_EVIDENCE = "insufficient_evidence"
    CONFLICT = "conflict"
    UNAVAILABLE = "unavailable"


@dataclass(frozen=True)
class ReasonResponse:
    status: ReasonStatus
    conclusion: str | None
