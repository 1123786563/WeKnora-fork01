"""Rule reasoner adapter over authorized assertions (Q02).

Forward-chaining derivation over the RESTRICTED registered rule grammar.
Facts come from the authorized store as (subject, predicate, object,
assertion_id) tuples; conclusions carry premise assertion ids and the
rule id, forming a verifiable DAG. Missing premises are NEVER treated as
negation; cycles and limits stop derivation with explicit statuses.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from .rules import RuleRegistry, RuleSet


class InvalidProof(Exception):
    """The proof references a cycle or an unregistered rule."""


@dataclass
class ReasonResult:
    # supported | insufficient_evidence | budget_exhausted.
    # conflicting_evidence is RESERVED: the first-version grammar has no
    # negation (facts cannot contradict), so no code path can produce it
    # yet; it activates when the assertion layer grows explicit conflict
    # predicates (Q03+). Never emitted as a placeholder.
    status: str
    premise_ids: list[str] = field(default_factory=list)
    rule_ids: list[str] = field(default_factory=list)
    conclusion: Optional[tuple] = None
    explanation: str = ""


@dataclass(frozen=True)
class ReasonRequest:
    rule_set_version: str
    facts: tuple  # (subject, predicate, object, assertion_id)
    goal: tuple    # (subject, predicate, object)
    max_rounds: int = 16
    max_facts: int = 4096


class RuleReasoner:
    def __init__(self, registry: RuleRegistry):
        self._registry = registry

    def reason(self, request: ReasonRequest) -> ReasonResult:
        ruleset = self._registry.load(request.rule_set_version)
        return self._derive(ruleset, request)

    def reason_fixture(self, *, facts, goal, rule_set_version: str = "test-control-v1") -> ReasonResult:
        """Test/fixture entry: the SAME formal derivation path (never a
        second reasoner implementation)."""
        request = ReasonRequest(rule_set_version=rule_set_version,
                                facts=tuple(facts), goal=tuple(goal))
        return self.reason(request)

    def _derive(self, ruleset: RuleSet, request: ReasonRequest) -> ReasonResult:
        # goal matching must consider the FULL derivation closure.
        facts: dict[tuple, str] = {}   # (s,p,o) -> assertion id (source or derived)
        derived_by: dict[tuple, tuple[str, str]] = {}  # fact tuple -> (rule_id, derived id)
        # premise edges keyed by FACT TUPLE (never by id string - caller
        # ids may collide with derived ids; tuples cannot).
        premise_edges: dict[tuple, list[tuple]] = {}    # derived fact tuple -> premise fact tuples
        for subject, predicate, obj, assertion_id in request.facts:
            facts[(subject, predicate, obj)] = assertion_id

        def fact_ids(key: tuple) -> list[str]:
            """Recursively collect SOURCE assertion ids under a fact:
            derived premises expand into their own premise TUPLES."""
            if key in premise_edges:
                out: list[str] = []
                for premise_key in premise_edges[key]:
                    out.extend(fact_ids(premise_key))
                return out
            return [facts[key]]

        goal_key = tuple(request.goal)
        rounds = 0
        while goal_key not in facts:
            if rounds >= request.max_rounds:
                return ReasonResult(status="budget_exhausted", conclusion=request.goal,
                                    explanation=f"derivation exceeded {request.max_rounds} rounds")
            if len(facts) >= request.max_facts:
                return ReasonResult(status="budget_exhausted", conclusion=request.goal,
                                    explanation=f"fact budget {request.max_facts} exhausted")
            new_facts = 0
            snapshot = dict(facts)  # iterate a snapshot; derived facts join next round
            for rule in ruleset.rules:
                for binding in self._bindings(rule, snapshot):
                    conclusion = self._instantiate(rule.conclusion, binding)
                    premise_keys = [self._instantiate(c, binding) for c in rule.conditions]
                    # Cycle check FIRST (before the already-known skip): a
                    # rule whose conclusion equals one of its own premises
                    # is an invalid proof DAG, whether or not the fact is
                    # already known.
                    if conclusion in premise_keys:
                        raise InvalidProof(f"rule {rule.rule_id!r} derives its own premise")
                    if conclusion in facts:
                        continue
                    if not all(pk in facts for pk in premise_keys):
                        continue
                    derived_id = f"{rule.rule_id}:{conclusion[0]}:{conclusion[2]}"
                    facts[conclusion] = derived_id
                    derived_by[conclusion] = (rule.rule_id, derived_id)
                    premise_edges[conclusion] = list(premise_keys)
                    new_facts += 1
            if new_facts == 0:
                return ReasonResult(
                    status="insufficient_evidence", conclusion=request.goal,
                    explanation="no rule can derive the goal from the given facts")
            rounds += 1

        premise_ids = sorted({pid for pid in fact_ids(goal_key)})
        rule_ids = sorted({derived_by[k][0] for k in [goal_key] if k in derived_by})
        return ReasonResult(
            status="supported",
            premise_ids=premise_ids,
            rule_ids=rule_ids,
            conclusion=request.goal,
            explanation=self._explain(goal_key, derived_by, premise_edges, facts),
        )

    def _bindings(self, rule, facts: dict):
        """Yield variable bindings satisfying ALL conditions against the
        GIVEN fact snapshot (restricted grammar: variables are ?names)."""
        def recurse(conditions, binding):
            if not conditions:
                yield dict(binding)
                return
            condition, *rest = conditions
            for fact_key in facts:
                trial = dict(binding)
                ok = True
                for term, value in ((condition.subject, fact_key[0]),
                                    (condition.object, fact_key[2])):
                    if term.startswith("?"):
                        if term in trial and trial[term] != value:
                            ok = False
                            break
                        trial[term] = value
                if not ok or fact_key[1] != condition.predicate:
                    continue
                if not condition.subject.startswith("?") and condition.subject != fact_key[0]:
                    continue
                if not condition.object.startswith("?") and condition.object != fact_key[2]:
                    continue
                yield from recurse(rest, trial)

        yield from recurse(rule.conditions, {})

    def _instantiate(self, condition, binding) -> tuple:
        def resolve(term):
            return binding.get(term, term) if term.startswith("?") else term
        return (resolve(condition.subject), condition.predicate, resolve(condition.object))

    def _explain(self, goal_key, derived_by, premise_edges, facts) -> str:
        """Text explanation from the PROOF STRUCTURE only - no invented
        premises, model inference never presented as rule proof."""
        if goal_key not in derived_by:
            return f"source fact {goal_key[0]} {goal_key[1]} {goal_key[2]}"
        rule_id, _ = derived_by[goal_key]
        premise_ids = sorted({pid for pid in
                              [i for pk in premise_edges.get(goal_key, []) for i in [facts[pk]]]})
        return (f"by rule {rule_id} over premises {premise_ids}: "
                f"{goal_key[0]} {goal_key[1]} {goal_key[2]}")
