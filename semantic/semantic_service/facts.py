"""Transport-neutral scoped semantic facts and their support validation."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from enum import Enum
import uuid
from collections.abc import Mapping

from semantic_service.contracts import Evidence, ScopeKey


def _required_text(value: object, name: str) -> None:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")


def _canonical_uuid(value: object, name: str) -> None:
    if not isinstance(value, str):
        raise ValueError(f"{name} must be a canonical UUID")
    try:
        canonical = str(uuid.UUID(value))
    except (ValueError, AttributeError) as exc:
        raise ValueError(f"{name} must be a canonical UUID") from exc
    if canonical != value:
        raise ValueError(f"{name} must be a canonical UUID")


def _scope_key(value: object, name: str = "scope") -> None:
    if not isinstance(value, ScopeKey):
        raise ValueError(f"{name} must be a ScopeKey")


def new_entity_id() -> str:
    return str(uuid.uuid4())


@dataclass(frozen=True)
class Entity:
    entity_id: str
    scope: ScopeKey
    entity_type: str

    def __post_init__(self) -> None:
        _scope_key(self.scope)
        _canonical_uuid(self.entity_id, "entity_id")
        _required_text(self.entity_type, "entity_type")


@dataclass(frozen=True)
class EntityRef:
    entity_id: str
    scope: ScopeKey

    def __post_init__(self) -> None:
        _scope_key(self.scope)
        _canonical_uuid(self.entity_id, "entity_id")


@dataclass(frozen=True)
class ScopedEvidence:
    scope: ScopeKey
    evidence: Evidence

    def __post_init__(self) -> None:
        _scope_key(self.scope)
        if not isinstance(self.evidence, Evidence):
            raise ValueError("evidence must be an Evidence record")


class AssertionKind(str, Enum):
    SOURCE = "source"
    RULE_DERIVED = "rule_derived"
    MODEL_INFERRED = "model_inferred"


@dataclass(frozen=True)
class Derivation:
    scope: ScopeKey
    conclusion_id: str
    premise_ids: tuple[str, ...]
    rule_id: str | None = None
    rule_version: str | None = None
    model_version: str | None = None
    prompt_version: str | None = None

    def __post_init__(self) -> None:
        _scope_key(self.scope)
        _required_text(self.conclusion_id, "conclusion_id")
        if not isinstance(self.premise_ids, tuple) or not self.premise_ids:
            raise ValueError("derivation requires premise IDs")
        for premise_id in self.premise_ids:
            _required_text(premise_id, "premise_id")
        if len(set(self.premise_ids)) != len(self.premise_ids):
            raise ValueError("premise IDs must be unique")

        rule_pair = (self.rule_id is not None, self.rule_version is not None)
        model_pair = (self.model_version is not None, self.prompt_version is not None)
        complete_rule = all(rule_pair)
        complete_model = all(model_pair)
        if rule_pair[0] != rule_pair[1] or model_pair[0] != model_pair[1]:
            raise ValueError("derivation version pairs must be complete")
        if complete_rule == complete_model:
            raise ValueError("derivation requires exactly one version pair")
        if complete_rule:
            _required_text(self.rule_id, "rule_id")
            _required_text(self.rule_version, "rule_version")
        if complete_model:
            _required_text(self.model_version, "model_version")
            _required_text(self.prompt_version, "prompt_version")


@dataclass(frozen=True)
class Assertion:
    assertion_id: str
    scope: ScopeKey
    subject: EntityRef
    predicate: str
    object_id: EntityRef | None
    value: str | None
    kind: AssertionKind
    evidence_ids: tuple[str, ...]
    derivation: Derivation | None
    valid_from: datetime | None
    valid_until: datetime | None

    def __post_init__(self) -> None:
        _scope_key(self.scope)
        if not isinstance(self.subject, EntityRef):
            raise ValueError("subject must be an EntityRef")
        if self.object_id is not None and not isinstance(self.object_id, EntityRef):
            raise ValueError("object_id must be an EntityRef")
        _required_text(self.assertion_id, "assertion_id")
        _required_text(self.predicate, "predicate")
        if not isinstance(self.kind, AssertionKind):
            raise ValueError("assertion kind is unsupported")
        if self.subject.scope != self.scope:
            raise ValueError("subject scope must match assertion scope")
        if self.object_id is not None and self.object_id.scope != self.scope:
            raise ValueError("object scope must match assertion scope")
        if (self.object_id is None) == (self.value is None):
            raise ValueError("assertion requires exactly one object or value")
        if self.value is not None and not isinstance(self.value, str):
            raise ValueError("assertion value must be a string")
        if not isinstance(self.evidence_ids, tuple):
            raise ValueError("evidence_ids must be a tuple")
        for evidence_id in self.evidence_ids:
            _required_text(evidence_id, "evidence_id")
        if len(set(self.evidence_ids)) != len(self.evidence_ids):
            raise ValueError("evidence IDs must be unique")

        if self.kind == AssertionKind.SOURCE:
            if not self.evidence_ids or self.derivation is not None:
                raise ValueError("source assertions require evidence and no derivation")
        else:
            if self.evidence_ids or self.derivation is None:
                raise ValueError("derived assertions require derivation and no direct evidence")
            if not isinstance(self.derivation, Derivation):
                raise ValueError("derivation must be a Derivation record")
            if self.derivation.conclusion_id != self.assertion_id:
                raise ValueError("derivation conclusion must match assertion identity")
            if self.derivation.scope != self.scope:
                raise ValueError("derivation scope must match assertion scope")
            expects_rule = self.kind == AssertionKind.RULE_DERIVED
            if expects_rule != (self.derivation.rule_id is not None):
                raise ValueError("derivation version pair must match assertion kind")

        for name, value in (("valid_from", self.valid_from), ("valid_until", self.valid_until)):
            if value is not None and (not isinstance(value, datetime) or value.tzinfo is None or value.utcoffset() is None):
                raise ValueError(f"{name} must be timezone-aware")
        if self.valid_from is not None and self.valid_until is not None and self.valid_from >= self.valid_until:
            raise ValueError("validity window must be increasing")


def validate_assertion(
    assertion: Assertion,
    evidence_by_id: Mapping[str, ScopedEvidence],
    premises_by_id: Mapping[str, Assertion],
) -> None:
    """Validate all reachable support records and reject dangling or cyclic support."""
    if not isinstance(assertion, Assertion):
        raise ValueError("assertion must be an Assertion record")
    if not isinstance(evidence_by_id, Mapping) or not isinstance(premises_by_id, Mapping):
        raise ValueError("evidence and premise indexes must be mappings")
    _scope_key(assertion.scope)
    if assertion.subject.scope != assertion.scope or (
        assertion.object_id is not None and assertion.object_id.scope != assertion.scope
    ):
        raise ValueError("entity reference scope must match assertion scope")

    visiting: set[str] = set()
    validated: set[str] = set()

    def visit(current: Assertion) -> None:
        if not isinstance(current, Assertion):
            raise ValueError("premise must be an Assertion record")
        _scope_key(current.scope)
        if current.assertion_id in visiting:
            raise ValueError("derivation support contains a cycle")
        if current.assertion_id in validated:
            return
        if current.subject.scope != current.scope or (
            current.object_id is not None and current.object_id.scope != current.scope
        ):
            raise ValueError("entity reference scope must match assertion scope")
        visiting.add(current.assertion_id)
        if current.kind == AssertionKind.SOURCE:
            for evidence_id in current.evidence_ids:
                scoped = evidence_by_id.get(evidence_id)
                if scoped is None:
                    raise ValueError("source assertion references missing evidence")
                if not isinstance(scoped, ScopedEvidence) or scoped.evidence.evidence_id != evidence_id:
                    raise ValueError("evidence map key must match evidence identity")
                if scoped.scope != current.scope:
                    raise ValueError("evidence scope must match assertion scope")
        else:
            derivation = current.derivation
            if derivation is None or derivation.conclusion_id != current.assertion_id:
                raise ValueError("derivation conclusion must match assertion identity")
            for premise_id in derivation.premise_ids:
                premise = premises_by_id.get(premise_id)
                if premise is None:
                    raise ValueError("derivation references missing premise")
                if premise.assertion_id != premise_id:
                    raise ValueError("premise map key must match assertion identity")
                if premise.scope != current.scope:
                    raise ValueError("premise scope must match assertion scope")
                visit(premise)
        visiting.remove(current.assertion_id)
        validated.add(current.assertion_id)

    visit(assertion)
