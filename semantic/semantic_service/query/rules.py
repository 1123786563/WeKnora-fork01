"""Versioned restricted rule sets (Q02).

Rules are deployed artifacts ONLY: a JSON grammar with a predicate
whitelist, a content digest, and explicit version ids. Clients may only
name an already-authorized version - never upload or evaluate code, and
never eval/SPARQL/anything executable.
"""

from __future__ import annotations

import hashlib
import json
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

RULES_DIR = Path(__file__).resolve().parents[2] / "rules"

# Registered version names are strict identifiers - no path separators,
# no traversal, no absolute paths (the registry contract: clients may
# only NAME an on-disk registered version).
_VERSION_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")

# Predicates the restricted grammar may mention (first version:
# control-relationship reasoning only).
PREDICATE_WHITELIST = frozenset({"controls"})


class RuleGrammarError(ValueError):
    """The rule file violates the restricted grammar."""


@dataclass(frozen=True)
class RuleCondition:
    subject: str  # variable "?x" or literal entity id
    predicate: str
    object: str

    @classmethod
    def from_json(cls, data: dict) -> "RuleCondition":
        predicate = data["predicate"]
        if predicate not in PREDICATE_WHITELIST:
            raise RuleGrammarError(f"predicate {predicate!r} not in whitelist")
        for key in ("subject", "object"):
            value = data[key]
            if not isinstance(value, str) or not value:
                raise RuleGrammarError(f"{key} must be a non-empty string")
        return cls(subject=data["subject"], predicate=predicate, object=data["object"])


@dataclass(frozen=True)
class Rule:
    rule_id: str
    description: str
    conditions: tuple[RuleCondition, ...]
    conclusion: RuleCondition

    @classmethod
    def from_json(cls, data: dict) -> "Rule":
        rule_id = data.get("rule_id", "")
        if not rule_id or not isinstance(rule_id, str):
            raise RuleGrammarError("rule_id must be a non-empty string")
        conditions = tuple(RuleCondition.from_json(c) for c in data.get("if", []))
        if not conditions:
            raise RuleGrammarError(f"rule {rule_id!r} has no conditions")
        conclusion = RuleCondition.from_json(data["then"])
        # Conclusion variables must be bound by some condition - an
        # unbound variable would derive nonsense facts like ("a", "controls", "?w").
        condition_vars = {t for c in conditions for t in (c.subject, c.object) if t.startswith("?")}
        conclusion_vars = {t for t in (conclusion.subject, conclusion.object) if t.startswith("?")}
        if not conclusion_vars <= condition_vars:
            raise RuleGrammarError(
                f"rule {rule_id!r} conclusion has unbound variables {sorted(conclusion_vars - condition_vars)}")
        return cls(rule_id=rule_id, description=data.get("description", ""),
                   conditions=conditions, conclusion=conclusion)


@dataclass(frozen=True)
class RuleSet:
    version: str
    digest: str
    rules: tuple[Rule, ...] = ()

    def rule(self, rule_id: str) -> Optional[Rule]:
        for rule in self.rules:
            if rule.rule_id == rule_id:
                return rule
        return None

    def contains(self, rule_id: str, version: str) -> bool:
        return version == self.version and self.rule(rule_id) is not None


class RuleRegistry:
    """Deployment-registered rule versions. Clients may only NAME a
    registered version - load() refuses anything not on disk."""

    def __init__(self, rules_dir: Path = RULES_DIR):
        self._rules_dir = rules_dir
        self._cache: dict[str, RuleSet] = {}

    def load(self, version: str) -> RuleSet:
        if version in self._cache:
            return self._cache[version]
        if not isinstance(version, str) or not _VERSION_PATTERN.match(version):
            # Traversal-shaped names are unregistered by definition; no
            # file is ever probed for them (no existence oracle).
            raise FileNotFoundError(f"rule version {version!r} is not registered")
        path = self._rules_dir / f"{version}.json"
        if not path.is_file():
            raise FileNotFoundError(f"rule version {version!r} is not registered")
        payload = path.read_text(encoding="utf-8")
        data = json.loads(payload)
        # Digest over canonicalized CONTENT (version + rules) - never over
        # the raw file, which would make the declared digest self-referential.
        canonical = json.dumps({"version": data["version"], "rules": data.get("rules", [])},
                               sort_keys=True, ensure_ascii=False)
        digest = "sha256:" + hashlib.sha256(canonical.encode("utf-8")).hexdigest()
        rules = tuple(Rule.from_json(r) for r in data.get("rules", []))
        rule_ids = [r.rule_id for r in rules]
        if len(rule_ids) != len(set(rule_ids)):
            raise RuleGrammarError(f"rule file {version!r} has duplicate rule ids")
        ruleset = RuleSet(
            version=data["version"],
            digest=digest,
            rules=rules,
        )
        if ruleset.version != version:
            raise RuleGrammarError(f"rule file version {ruleset.version!r} != requested {version!r}")
        declared = data.get("digest")
        if declared is None:
            raise RuleGrammarError(f"rule file {version!r} declares no digest")
        if declared != digest:
            raise RuleGrammarError(f"rule file digest mismatch for {version!r}")
        self._cache[version] = ruleset
        return ruleset
