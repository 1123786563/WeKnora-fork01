"""Deterministic Pricing Group fixture with exact integer-cents expectations.

Every amount is computed with int/Decimal arithmetic only (never float),
mirroring the pinned Lago Community v1.53.0 charge models
(getlago/lago-api @ 591ae90):

* ``app/services/charge_models/standard_service.rb``
  amount = units * BigDecimal(properties["amount"])
* ``app/services/charge_models/package_service.rb``
  amount = ceil((units - free_units) / package_size) * BigDecimal(properties["amount"])
* ``app/validators/decimal_amount_service.rb``
  properties["amount"] MUST be a decimal *string* on the wire.

Field plan deviation (recorded in the T04 ledger): the interface sketch in
the plan labels the ``*-tool-calls`` metric as COUNT-aggregated, but the
binding reference math (5 events totalling 250 units -> ceil(250/100) = 3
bundles -> 4500 cents; per-charge events_count 10 and 5) requires 250 units
from 5 events.  COUNT yields exactly 5 units, so ``*-tool-calls`` is a SUM
metric over the integer ``calls`` property instead.  The exact expectation
values from the plan (7000 / 4500 / 11500 cents) are preserved.

Event timestamps are not baked into the fixture: each event stores a
monotonic ``offset_seconds`` and is materialized against a send-time base,
so every timestamp falls after subscription creation and inside the
current monthly billing period (offsets never exceed 60 s).
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from decimal import Decimal

RUN_ID_PREFIX = "weknora-t04-"
MODEL_METRIC_KEY = "model-units"
TOOL_METRIC_KEY = "tool-calls"
CURRENCY = "CNY"
PLAN_INTERVAL = "monthly"

# Single source of truth for both the decimal string sent to Lago and the
# integer expectation compared locally -- derived, never duplicated.
STANDARD_AMOUNT_CENTS = 7
STANDARD_AMOUNT_DECIMAL = str(STANDARD_AMOUNT_CENTS)
PACKAGE_AMOUNT_CENTS = 1500
PACKAGE_AMOUNT_DECIMAL = str(PACKAGE_AMOUNT_CENTS)
PACKAGE_SIZE = 100
FREE_UNITS = 0

# metric key -> (aggregation field, charge model)
METRIC_FIELDS = {
    MODEL_METRIC_KEY: ("units", "standard"),
    TOOL_METRIC_KEY: ("calls", "package"),
}

# Lago v1.53.0 aggregation enum value for SUM metrics (live-verified 2026-09-20:
# aggregation_type "sum" is rejected 422 value_is_invalid; "sum_agg" is accepted)
AGGREGATION_TYPE_SUM = "sum_agg"

CHARGE_ORDER = (MODEL_METRIC_KEY, TOOL_METRIC_KEY)

# Reference task (index 0): 10 model events x 100 units = 1000 units
# -> 1000 x 7 = 7000 cents; 5 tool events x 50 calls = 250 calls
# -> ceil(250/100) = 3 bundles x 1500 = 4500 cents; total 11500 cents.
REFERENCE_MODEL_EVENTS = 10
REFERENCE_MODEL_UNITS_PER_EVENT = 100
REFERENCE_TOOL_EVENTS = 5
REFERENCE_TOOL_CALLS_PER_EVENT = 50

# Load tasks (index k >= 1): 10 model events x k units + 10 tool events
# x k calls; k = 10 and k = 11 sit on / cross a package bundle boundary.
LOAD_EVENTS_PER_METRIC = 10


def _ceil_div(numerator: int, denominator: int) -> int:
    """Integer ceil division without float arithmetic."""
    return -(-numerator // denominator)


@dataclass(frozen=True)
class BillableMetric:
    code: str
    name: str
    aggregation_type: str
    field_name: str
    key: str  # short identity: "model-units" / "tool-calls"


@dataclass(frozen=True)
class Charge:
    metric_code: str
    metric_key: str
    charge_model: str
    amount_decimal: str
    amount_cents: int
    package_size: int | None = None
    free_units: int = 0


@dataclass(frozen=True)
class PlanSpec:
    code: str
    name: str
    interval: str
    amount_cents: int
    currency: str
    charges: tuple[Charge, ...]


@dataclass(frozen=True)
class UsageEvent:
    transaction_id: str
    code: str
    offset_seconds: int
    properties: dict  # integer values only

    def to_dict(self) -> dict:
        return {
            "transaction_id": self.transaction_id,
            "code": self.code,
            "offset_seconds": self.offset_seconds,
            "properties": dict(self.properties),
        }

    @classmethod
    def from_dict(cls, data: dict) -> "UsageEvent":
        return cls(
            transaction_id=data["transaction_id"],
            code=data["code"],
            offset_seconds=int(data["offset_seconds"]),
            properties={key: int(value) for key, value in data["properties"].items()},
        )


@dataclass(frozen=True)
class Task:
    index: int
    subscription_external_id: str
    events: tuple[UsageEvent, ...]

    @property
    def reference(self) -> bool:
        return self.index == 0

    def to_dict(self) -> dict:
        return {
            "index": self.index,
            "subscription_external_id": self.subscription_external_id,
            "events": [event.to_dict() for event in self.events],
        }

    @classmethod
    def from_dict(cls, data: dict) -> "Task":
        return cls(
            index=int(data["index"]),
            subscription_external_id=data["subscription_external_id"],
            events=tuple(UsageEvent.from_dict(item) for item in data["events"]),
        )


@dataclass(frozen=True)
class RunSpec:
    run_id: str
    customer_name: str
    metrics: tuple[BillableMetric, ...]
    plan: PlanSpec
    tasks: tuple[Task, ...]

    def to_dict(self) -> dict:
        return {
            "run_id": self.run_id,
            "customer_name": self.customer_name,
            "metrics": [
                {
                    "code": metric.code,
                    "name": metric.name,
                    "aggregation_type": metric.aggregation_type,
                    "field_name": metric.field_name,
                    "key": metric.key,
                }
                for metric in self.metrics
            ],
            "plan": {
                "code": self.plan.code,
                "name": self.plan.name,
                "interval": self.plan.interval,
                "amount_cents": self.plan.amount_cents,
                "currency": self.plan.currency,
                "charges": [
                    {
                        "metric_code": charge.metric_code,
                        "metric_key": charge.metric_key,
                        "charge_model": charge.charge_model,
                        "amount_decimal": charge.amount_decimal,
                        "amount_cents": charge.amount_cents,
                        "package_size": charge.package_size,
                        "free_units": charge.free_units,
                    }
                    for charge in self.plan.charges
                ],
            },
            "tasks": [task.to_dict() for task in self.tasks],
        }

    @classmethod
    def from_dict(cls, data: dict) -> "RunSpec":
        return cls(
            run_id=data["run_id"],
            customer_name=data["customer_name"],
            metrics=tuple(
                BillableMetric(
                    code=item["code"],
                    name=item["name"],
                    aggregation_type=item["aggregation_type"],
                    field_name=item["field_name"],
                    key=item["key"],
                )
                for item in data["metrics"]
            ),
            plan=PlanSpec(
                code=data["plan"]["code"],
                name=data["plan"]["name"],
                interval=data["plan"]["interval"],
                amount_cents=int(data["plan"]["amount_cents"]),
                currency=data["plan"]["currency"],
                charges=tuple(
                    Charge(
                        metric_code=item["metric_code"],
                        metric_key=item["metric_key"],
                        charge_model=item["charge_model"],
                        amount_decimal=item["amount_decimal"],
                        amount_cents=int(item["amount_cents"]),
                        package_size=item["package_size"],
                        free_units=int(item["free_units"]),
                    )
                    for item in data["plan"]["charges"]
                ),
            ),
            tasks=tuple(Task.from_dict(item) for item in data["tasks"]),
        )


@dataclass(frozen=True)
class ChargeExpectation:
    metric_key: str
    units: Decimal
    events_count: int
    amount_cents: int


@dataclass(frozen=True)
class ExpectedUsage:
    charges: tuple[ChargeExpectation, ...]
    total_amount_cents: int

    def charge(self, key: str) -> ChargeExpectation:
        for expectation in self.charges:
            if expectation.metric_key == key:
                return expectation
        raise KeyError(key)


def _metric_code(run_id: str, key: str) -> str:
    return f"{run_id}-{key}"


def _reference_task(run_id: str) -> Task:
    events = []
    offset = 0
    for _ in range(REFERENCE_MODEL_EVENTS):
        events.append(
            UsageEvent(
                transaction_id=f"{run_id}-task-0-model-{offset}",
                code=_metric_code(run_id, MODEL_METRIC_KEY),
                offset_seconds=offset,
                properties={"units": REFERENCE_MODEL_UNITS_PER_EVENT},
            )
        )
        offset += 1
    for _ in range(REFERENCE_TOOL_EVENTS):
        events.append(
            UsageEvent(
                transaction_id=f"{run_id}-task-0-tool-{offset}",
                code=_metric_code(run_id, TOOL_METRIC_KEY),
                offset_seconds=offset,
                properties={"calls": REFERENCE_TOOL_CALLS_PER_EVENT},
            )
        )
        offset += 1
    return Task(
        index=0,
        subscription_external_id=f"{run_id}-task-0",
        events=tuple(events),
    )


def _load_task(run_id: str, k: int) -> Task:
    events = []
    offset = 0
    for _ in range(LOAD_EVENTS_PER_METRIC):
        events.append(
            UsageEvent(
                transaction_id=f"{run_id}-task-{k}-model-{offset}",
                code=_metric_code(run_id, MODEL_METRIC_KEY),
                offset_seconds=offset,
                properties={"units": k},
            )
        )
        offset += 1
    for _ in range(LOAD_EVENTS_PER_METRIC):
        events.append(
            UsageEvent(
                transaction_id=f"{run_id}-task-{k}-tool-{offset}",
                code=_metric_code(run_id, TOOL_METRIC_KEY),
                offset_seconds=offset,
                properties={"calls": k},
            )
        )
        offset += 1
    return Task(
        index=k,
        subscription_external_id=f"{run_id}-task-{k}",
        events=tuple(events),
    )


def build_run(run_id: str, task_count: int) -> RunSpec:
    """Build one isolated run: customer, 2 SUM metrics, plan, task_count tasks.

    ``task_count`` includes the reference task (index 0); tasks 1..N-1 are
    load tasks whose amounts depend deterministically on their index.
    """
    if not run_id.startswith(RUN_ID_PREFIX):
        raise ValueError(
            f"run_id must start with {RUN_ID_PREFIX!r} (got {run_id!r})"
        )
    if task_count < 1:
        raise ValueError("task_count must include the reference task (>= 1)")

    metrics = tuple(
        BillableMetric(
            code=_metric_code(run_id, key),
            name=f"WeKnora T04 {key} ({run_id})",
            aggregation_type=AGGREGATION_TYPE_SUM,
            field_name=field,
            key=key,
        )
        for key, (field, _model) in METRIC_FIELDS.items()
    )
    standard_charge = Charge(
        metric_code=_metric_code(run_id, MODEL_METRIC_KEY),
        metric_key=MODEL_METRIC_KEY,
        charge_model="standard",
        amount_decimal=STANDARD_AMOUNT_DECIMAL,
        amount_cents=STANDARD_AMOUNT_CENTS,
    )
    package_charge = Charge(
        metric_code=_metric_code(run_id, TOOL_METRIC_KEY),
        metric_key=TOOL_METRIC_KEY,
        charge_model="package",
        amount_decimal=PACKAGE_AMOUNT_DECIMAL,
        amount_cents=PACKAGE_AMOUNT_CENTS,
        package_size=PACKAGE_SIZE,
        free_units=FREE_UNITS,
    )
    plan = PlanSpec(
        code=f"{run_id}-plan",
        name=f"WeKnora T04 Pricing Group plan ({run_id})",
        interval=PLAN_INTERVAL,
        amount_cents=0,
        currency=CURRENCY,
        charges=(standard_charge, package_charge),
    )
    tasks = tuple(
        _reference_task(run_id) if index == 0 else _load_task(run_id, index)
        for index in range(task_count)
    )
    return RunSpec(
        run_id=run_id,
        customer_name=f"WeKnora T04 Billing Account ({run_id})",
        metrics=metrics,
        plan=plan,
        tasks=tasks,
    )


def expected_usage(task: Task) -> ExpectedUsage:
    """Exact per-charge and total expectation for one task (int/Decimal only)."""
    sums: dict[str, int] = {}
    counts: dict[str, int] = {}
    for event in task.events:
        key = key_of(event.code)
        field = METRIC_FIELDS[key][0]
        sums[key] = sums.get(key, 0) + int(event.properties[field])
        counts[key] = counts.get(key, 0) + 1

    charges = []
    for key in CHARGE_ORDER:
        if key not in sums:
            continue
        units = sums[key]
        field, model = METRIC_FIELDS[key]
        if model == "standard":
            amount = units * STANDARD_AMOUNT_CENTS
        elif model == "package":
            billable_units = max(0, units - FREE_UNITS)
            amount = _ceil_div(billable_units, PACKAGE_SIZE) * PACKAGE_AMOUNT_CENTS
        else:  # pragma: no cover - fixed catalog
            raise ValueError(model)
        charges.append(
            ChargeExpectation(
                metric_key=key,
                units=Decimal(units),
                events_count=counts[key],
                amount_cents=amount,
            )
        )
    return ExpectedUsage(
        charges=tuple(charges),
        total_amount_cents=sum(charge.amount_cents for charge in charges),
    )


def key_of(metric_code: str) -> str:
    """Short metric key ("model-units"/"tool-calls") of a full metric code."""
    for key in METRIC_FIELDS:
        if metric_code.endswith(f"-{key}"):
            return key
    raise ValueError(f"unknown metric code: {metric_code}")


def event_payload(
    event: UsageEvent, external_subscription_id: str, base: datetime
) -> dict:
    """Materialize one event into a v1.53.0 POST /api/v1/events body member."""
    timestamp = base + timedelta(seconds=event.offset_seconds)
    return {
        "transaction_id": event.transaction_id,
        "external_subscription_id": external_subscription_id,
        "code": event.code,
        "timestamp": timestamp.isoformat(),
        "properties": dict(event.properties),
    }


@dataclass(frozen=True)
class NegativeControlEvent:
    """Accepted-and-queryable-but-never-rated event: legal metric code,
    external_subscription_id pointing at a subscription that does not exist."""

    transaction_id: str
    external_subscription_id: str
    code: str
    properties: dict


def negative_control_event(run_id: str) -> NegativeControlEvent:
    return NegativeControlEvent(
        transaction_id=f"{run_id}-negative-control",
        external_subscription_id=f"{run_id}-task-nonexistent",
        code=_metric_code(run_id, MODEL_METRIC_KEY),
        properties={"units": 1},
    )


def conflicting_variant(event: UsageEvent) -> UsageEvent:
    """Same transaction identity, different integer content."""
    return UsageEvent(
        transaction_id=event.transaction_id,
        code=event.code,
        offset_seconds=event.offset_seconds,
        properties={key: value * 3 + 1 for key, value in event.properties.items()},
    )
