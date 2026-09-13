"""Structured model-conclusion validation (Q03).

A model conclusion is ONLY accepted when its cited premises exist in the
AUTHORIZED assertion set for this request. conclusion_kind is always
"model" - a model explanation is NEVER a rule proof. Citation existence
means traceability, not semantic correctness.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

ALLOWED_STATUSES = frozenset({"supported", "insufficient_evidence",
                              "conflicting_evidence", "budget_exhausted"})

MAX_CONCLUSION_CHARS = 2000
MAX_PREMISES = 64


class InvalidConclusion(Exception):
    """The model output violates the versioned output schema."""


@dataclass(frozen=True)
class ValidatedConclusion:
    status: str
    conclusion: str
    premise_ids: tuple[str, ...] = ()
    conclusion_kind: str = "model"
    limitations: str = ""


def validate_conclusion(payload: dict, authorized_assertion_ids: set[str]) -> ValidatedConclusion:
    """Validate a model payload against the versioned output schema.

    Raises InvalidConclusion for: unknown status, supported without
    evidence, premises outside the authorized set, oversized conclusions,
    premise-count overflow, or a payload claiming rule proof.
    """
    if not isinstance(payload, dict):
        raise InvalidConclusion("payload must be an object")
    status = payload.get("status")
    if status not in ALLOWED_STATUSES:
        raise InvalidConclusion(f"invalid status {status!r}")
    conclusion = payload.get("conclusion", "")
    if not isinstance(conclusion, str):
        raise InvalidConclusion("conclusion must be a string")
    if not conclusion.strip():
        raise InvalidConclusion("conclusion is empty or whitespace")
    if len(conclusion) > MAX_CONCLUSION_CHARS:
        raise InvalidConclusion("conclusion exceeds length limit")
    limitations = payload.get("limitations", "")
    if not isinstance(limitations, str):
        raise InvalidConclusion("limitations must be a string")
    if len(limitations) > MAX_CONCLUSION_CHARS:
        raise InvalidConclusion("limitations exceeds length limit")
    premise_ids = payload.get("premise_ids", [])
    if not isinstance(premise_ids, list) or not all(isinstance(p, str) for p in premise_ids):
        raise InvalidConclusion("premise_ids must be a list of strings")
    if len(premise_ids) > MAX_PREMISES:
        raise InvalidConclusion("premise count exceeds limit")
    # Order-preserving dedupe: duplicates carry no evidential weight.
    premise_ids = list(dict.fromkeys(premise_ids))
    if status == "supported" and not premise_ids:
        raise InvalidConclusion("missing evidence")
    if not set(premise_ids) <= authorized_assertion_ids:
        raise InvalidConclusion("unknown evidence")
    kind = payload.get("conclusion_kind", "model")
    if kind != "model":
        # A model output NEVER claims rule proof.
        raise InvalidConclusion("model conclusion must carry conclusion_kind=model")
    return ValidatedConclusion(
        status=status,
        conclusion=conclusion,
        premise_ids=tuple(premise_ids),
        conclusion_kind="model",
        limitations=limitations,
    )
