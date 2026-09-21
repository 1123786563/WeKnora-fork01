# Semantica 0.6.8 experiments

This is an experiment only. It does not create a service, call a model, use
credentials, connect Neo4j, or validate Go authorization/deletion behavior.

Download the exact public wheel and verify its hash before probing:

```sh
curl -fsSL https://files.pythonhosted.org/packages/f5/4d/edf005139044fca3f86711d2bff96de93ae3e10ed1a6d02bc05252b610e2/semantica-0.6.8-py3-none-any.whl -o /tmp/semantica-0.6.8-py3-none-any.whl
shasum -a 256 /tmp/semantica-0.6.8-py3-none-any.whl
uv venv --python 3.12.12 semantic/experiments/.venv312
uv pip install --python semantic/experiments/.venv312/bin/python -r semantic/experiments/requirements-probe.txt
semantic/experiments/.venv312/bin/python semantic/experiments/runtime_probe.py --wheel /tmp/semantica-0.6.8-py3-none-any.whl --output docs/plans/semantica/evidence/2026-09-20/runtime-312.json
semantic/experiments/.venv312/bin/python -m pytest semantic/experiments/test_capability_probe.py -q
```

Expected SHA-256: `0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`.
An explicit missing or mismatched `--wheel` fails before extraction or import;
omitting `--wheel` produces only `not-tested` evidence.
The probe records public-import failures as `actual-runtime` and release-wheel
source inspection as `source-only`; neither is a production acceptance result.
The current runtime evidence uses CPython 3.12.12 and the pinned probe dependencies above. CPython 3.9.6 import failure is historical diagnostic evidence only.

`capability_probe.py` can be run separately to reproduce the historical
source-only/CPython 3.9 diagnostic; it does not replace `runtime-312.json`.

## V01 installed-distribution contract

`pyproject.toml`, `.python-version`, and `uv.lock` define a separate V01
environment. It installs the frozen `semantica==0.6.8` distribution instead
of extracting a wheel onto `PYTHONPATH`. The verifier checks the installed
distribution version, its location below the active environment's `sys.prefix`,
the committed lock SHA-256, and runtime signatures for the public imports used
by downstream work. It does not construct providers or connect to storage.

```sh
uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q
uv run --locked --isolated --project semantic/experiments python semantic/experiments/verify_version.py --output docs/plans/semantica/evidence/2026-09-20/v01-installed-contract.json
```

`--isolated` deliberately replays the lock in a fresh uv environment. The
result is `actual-runtime` evidence only for installation, imports, metadata,
and signatures. It is not evidence for Semantica providers, models, persistent
graph/vector storage, Go authorization, deletion/revocation, gRPC, quality,
performance, budget control, or production readiness.
