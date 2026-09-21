# V01 report — frozen minimal installation contract

Status: **DONE** for the controller-approved minimal import-contract profile.
It is not a Semantica default/full-install, model, provider, or real-storage
acceptance.

## Delivered contract

- `semantic/experiments/pyproject.toml` pins CPython `3.12.12` and the
  lightweight dependencies required by the public import contract. Its uv
  dependency-metadata override intentionally suppresses Semantica's default
  dependency expansion only in this experiment.
- `semantic/experiments/uv.lock` locks Semantica `0.6.8`; its wheel record has
  SHA-256 `0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`.
- `verify_version.py` verifies an explicitly supplied local wheel (`--wheel` or
  `SEMANTICA_WHEEL`), checks the matching lock hash, installed distribution
  metadata, all required public imports, and callable signatures. It never
  downloads or source-loads a wheel. Every expected failure writes JSON and
  exits non-zero.
- `capability-evidence.json` records schema version 1, Python/version/source
  provenance, lock and wheel hashes, and seven public signatures.

## Evidence

Baseline: `3939fbdb`; worktree: `codex/semantica-capability-probe`.

RED evidence:

```text
UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py::test_verify_version_accepts_an_explicit_wheel_environment_variable -q
exit 1: verify_version.py required --wheel before the explicit environment-variable contract existed.
```

GREEN and clean replay:

```text
uv lock --project semantic/experiments --check
exit 0

UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv sync --project semantic/experiments --frozen
exit 0; new isolated environment, CPython 3.12.12, 28 packages installed.

UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q
exit 0; 5 passed in 10.15s.

UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python -m pytest semantic/experiments/test_capability_probe.py -q
exit 0; 4 passed in 1.13s.

SEMANTICA_WHEEL=/tmp/semantica-0.6.8-py3-none-any.whl UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python semantic/experiments/verify_version.py --output docs/superpowers/plans/semantica/capability-evidence.json
exit 0.
```

The successful evidence is
`docs/superpowers/plans/semantica/capability-evidence.json`; it was produced
from `/tmp/semantica-v01-clean-final/bin/python3`, records Semantica `0.6.8`,
the seven required public callable signatures, source tag/commit URL and a
matching lock/wheel hash. The retained negative result is
`docs/superpowers/plans/semantica/evidence/2026-09-20/v01/bad-wheel-hash.json`:
the command exited 1 with structured `wheel-sha256-mismatch` evidence.

Relevant output hashes:

```text
pyproject.toml                 12f5f15d08d5c35915ae3e0657cf0aa54c2072a2bedaa88af59f795bed9095c5
uv.lock                        04d1f9a64ef7a17e43480c8613dbe91bf60b3ad912da70eac723ba4bb68377a1
capability-evidence.json       bcb228e27d693c4635eeaa1b4ec1094384b16e9807ba45789ec14b425e2af113
bad-wheel-hash.json            9e85630b1b4df1564031078ea8bc5beebec8c4c21512ebeb404427090e3e5ab2
```

## Limits

This profile omits Semantica's default ML/NLP/vector/document stacks and all
provider, storage, cloud, infrastructure and other extras. It downloads no
model weights and calls no paid or remote model. Neo4j `5.28.2` is only an
import-ready driver pin for the next isolated V02 work; no connection,
persistence, ACL or restart behavior is asserted here.

The controller owns `docs/superpowers/plans/semantica/progress.md`; it was not
edited or staged by this task.
