"""Transport-neutral C01 Semantica domain records."""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


@dataclass(frozen=True)
class ScopeKey:
    tenant_id: int
    kb_id: str


@dataclass(frozen=True)
class DocumentRevision:
    scope: ScopeKey
    document_id: str
    revision: int
    content_hash: str
    deleted: bool

    def require_delete(self) -> None:
        if not self.deleted:
            raise ValueError("semantic delete requires deleted=true")


class ReasonStatus(str, Enum):
    DERIVED = "derived"
    INSUFFICIENT_EVIDENCE = "insufficient_evidence"
    CONFLICT = "conflict"
    UNAVAILABLE = "unavailable"


@dataclass(frozen=True)
class ReasonResponse:
    status: ReasonStatus
    conclusion: str | None
