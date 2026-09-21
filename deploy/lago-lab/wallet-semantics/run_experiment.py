#!/usr/bin/env python3
"""CLI entry for the T03 wallet-semantics experiments.

Usage: python3 run_experiment.py <e1|e2|e3|e4|all>

The lab API key comes only from the caller's LAGO_API_KEY environment
variable; the lab origin is http://127.0.0.1:$LAGO_API_PORT (default 48893).
Exit codes mirror the report status: 0 pass, 1 fail, 2 blocked-env.
"""

import os
import sys
from pathlib import Path

if __package__ in (None, ""):
    sys.path.insert(0, str(Path(__file__).resolve().parent))

import harness

# experiment key -> (module under experiments/, evidence slug)
REGISTRY = {
    "e1": ("experiments.e1_expiry", "e1-expiry"),
    "e2": ("experiments.e2_order", "e2-order"),
    "e3": ("experiments.e3_concurrency", "e3-concurrency"),
    "e4": ("experiments.e4_withdraw", "e4-withdraw"),
}
ALL_KEYS = ("e1", "e2", "e3", "e4")
EXIT_CODES = {"pass": 0, "fail": 1, "blocked-env": 2}


def load_experiment(key):
    module_name, _ = REGISTRY[key]
    import importlib

    module = importlib.import_module(module_name)
    return module.RUNNER, module.SLUG


def run_one(key):
    runner, slug = load_experiment(key)
    report = harness.run_experiment(
        key, slug, harness.default_base_url(), os.environ.get("LAGO_API_KEY", ""),
        experiment_main=runner,
    )
    print(report.serialized())
    return EXIT_CODES[report.status]


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    if len(argv) != 1 or argv[0] not in (*ALL_KEYS, "all"):
        print(__doc__.strip(), file=sys.stderr)
        return 2
    keys = list(ALL_KEYS) if argv[0] == "all" else [argv[0]]
    worst = 0
    for key in keys:
        worst = max(worst, run_one(key))
    return worst


if __name__ == "__main__":
    sys.exit(main())
