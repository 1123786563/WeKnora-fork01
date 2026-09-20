"""Docker-free measurement helpers: percentiles, throughput, latency summary.

Every metric here is derived from recorded timestamps and counts -- never
from after-the-fact guesses.  Percentiles use the nearest-rank method so a
reported p50/p95 is always an actually observed sample.  Unresolved tasks
are excluded from latency percentiles but counted separately (an honest
finding, never silently dropped).
"""

from __future__ import annotations

import math
from datetime import datetime

P95_TARGET_SECONDS = 60.0  # spec objective 11: event-to-reconcilable-batch p95 <= 60 s


def percentiles(samples) -> dict:
    """Nearest-rank p50/p95/max over ``samples`` (any numeric type)."""
    if not samples:
        raise ValueError("at least one sample is required")
    ordered = sorted(samples)
    count = len(ordered)

    def nearest_rank(p):
        rank = max(1, math.ceil(p / 100 * count))
        return ordered[rank - 1]

    return {"p50": nearest_rank(50), "p95": nearest_rank(95), "max": ordered[-1], "count": count}


def throughput(accepted_events: int, window_seconds: float) -> float:
    """Accepted events per second over the sending window."""
    if window_seconds <= 0:
        raise ValueError("window must be positive")
    return accepted_events / window_seconds


def peak_events_per_second(timestamps_seconds) -> int:
    """Largest number of accepted events inside any single wall-clock second."""
    if not timestamps_seconds:
        return 0
    buckets: dict[int, int] = {}
    for stamp in timestamps_seconds:
        bucket = int(stamp)
        buckets[bucket] = buckets.get(bucket, 0) + 1
    return max(buckets.values())


def latency_summary(per_task) -> dict:
    """Percentile stats over resolved tasks; unresolved counted separately.

    ``per_task`` is a list of ``{"key": ..., "latency_seconds": float | None}``;
    ``None`` marks an unresolved task (poll timeout) which must not enter the
    latency denominator but must be reported.
    """
    if not per_task:
        raise ValueError("at least one task is required")
    resolved = [entry for entry in per_task if entry["latency_seconds"] is not None]
    unresolved_keys = [entry["key"] for entry in per_task if entry["latency_seconds"] is None]
    if resolved:
        stats = percentiles([entry["latency_seconds"] for entry in resolved])
    else:
        stats = {"p50": None, "p95": None, "max": None, "count": 0}
    return {
        "resolved": stats,
        "resolved_count": len(resolved),
        "unresolved_count": len(unresolved_keys),
        "unresolved_keys": unresolved_keys,
    }


def p95_verdict(p95, target: float = P95_TARGET_SECONDS) -> dict:
    """Compare a measured p95 against the spec target (<= target passes)."""
    if p95 is None:
        return {
            "p95_seconds": None,
            "target_seconds": target,
            "verdict": "fail",
            "reason": "no resolved measurements",
        }
    return {
        "p95_seconds": p95,
        "target_seconds": target,
        "verdict": "pass" if p95 <= target else "fail",
    }


def iso_delta_seconds(later_iso: str, earlier_iso: str) -> float:
    """Seconds between two ISO-8601 timestamps (later minus earlier)."""
    later = datetime.fromisoformat(later_iso)
    earlier = datetime.fromisoformat(earlier_iso)
    return (later - earlier).total_seconds()
