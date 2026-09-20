# V03 Offline Evaluation Foundation — Implementation Report

## Scope and status

V03 remains `in_progress`. This change delivers only an offline, deterministic scorer and synthetic Chinese fixture set. It has no code path to query native retrieval, Semantica, Neo4j, a provider, a model, the network, or user/production data.

The synthetic report is structural smoke evidence only. It is not evidence for Chinese quality, native-vs-Semantica comparison, latency, cost, a controlled provider, a live model, or promotion. `docs/plans/semantica/acceptance-policy.json` remains `approved: false` and `proposed_thresholds: null`.

## RED → GREEN evidence

Base commit: `654da9cf3047f0651a8b1ea23fa32e17e999ccae`.
Implementation commit was created as `a32b2f44c8caab0a9c674816adc600dea80427ee`; the final amended SHA is recorded by the implementation handoff after this report update.

1. RED: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q` exited 2 because `evaluate` did not exist (`ModuleNotFoundError`).
2. GREEN after minimal per-case scorer: the same command exited 0, `3 passed in 0.01s`.
3. RED after aggregate contract tests: the same command exited 1, `1 failed, 4 passed`, because `aggregate_cases` raised `NotImplementedError`.
4. GREEN after aggregation: the same command exited 0, `5 passed in 0.01s`.
5. RED after loader/run contract tests: the same command exited 2 because `evaluate_run` could not be imported.
6. GREEN after strict JSONL loader, exact join, synthetic fixtures, and CLI: the same command exited 0, `9 passed in 0.07s`.

## Final scoped commands and results

```sh
uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q
# exit 0 — 9 passed in 0.07s

uv run --locked --project semantic/experiments python semantic/experiments/evaluate.py \
  --dataset semantic/experiments/fixtures/questions.jsonl \
  --observations semantic/experiments/fixtures/evaluation-example-observations.jsonl \
  --output /tmp/semantica-v03-synthetic-report.json
# exit 0
```

The generated `/tmp/semantica-v03-synthetic-report.json` has `schema_version: 1`, `evaluator: "semantica-v03-offline"`, `lock_sha256: "7cea23660879d96120a8936b44d9c23d4aaaf036b31c9a4e79812d31a4efe709"`, `evidence_layer: "synthetic"`, and `synthetic_only: true`. It has seven per-case rows. Input/output token totals and p50/p95 latency are all `null`; those values were not coerced to zero.

## Deliverables

- `semantic/experiments/evaluate.py`: standard-library-only scorer, strict case/observation JSONL loaders, exact case-ID join, nearest-rank percentiles, and atomic report CLI.
- `semantic/experiments/fixtures/questions.jsonl`: seven synthetic Chinese cases covering direct source lookup, dependency chain, missing premise, conflict, insufficient evidence, revoked source, and foreign tenant alias.
- `semantic/experiments/fixtures/evaluation-example-observations.jsonl`: one synthetic, non-runtime observation for every case.
- `docs/plans/semantica/evaluation-baseline.md` and `acceptance-policy.json`: documented metric semantics and an intentionally closed approval gate.

## Remaining limitations and required evidence

- No comparable, authorized native/Semantica observations exist, so neither quality nor a backend comparison is verified.
- No controlled-provider evidence exists for Chinese Q14/Q17; mocked seams do not qualify.
- No live-model evidence exists.
- No authorized enterprise corpus, measured latency, measured token/cost data, thresholds, product-owner approval, production integration, or promotion evidence exists.
- A full `semantic/experiments` suite is not used as V03 proof: the preflight ledger records that V02 real-storage tests require an unprovisioned `SEMANTICA_V02_NEO4J_PASSWORD` in this worktree. The scoped V03 suite above is the applicable verification.

## Hard-gate correction round — 2026-09-21

Approved correction plan `84b11b85` was cherry-picked as `e89cdb0ed` before this fix round. The review found three Important defects in the original evaluator: unexpected evidence did not independently fail the permission gate; the case's requested mode was not authoritative; and empty JSONL/run inputs were silently accepted.

### RED → GREEN evidence

After adding the approved assertions, the scoped command below exited 1 with `3 failed, 8 passed`:

```sh
uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q
```

The failures were exactly the intended defects: unsupported `e-hidden` returned `permission_leak=False`; a `graph_rag` case accepted an observation self-reporting `reason`; and empty case JSONL did not raise `ValueError`.

The correction makes `permission_leak` true for any returned evidence outside immutable expected evidence, as well as forbidden evidence or access violations. `mode_match` now requires `case.requested_mode == observation.requested_mode == observation.actual_mode`. The scorer's `correct` result additionally requires complete expected-evidence recall and `answered` status for answerable cases. Loaders reject empty files and `evaluate_run` rejects an empty case/observation set.

Fresh GREEN verification:

```sh
uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q
# exit 0 — 11 passed in 0.02s

uv run --locked --project semantic/experiments python semantic/experiments/evaluate.py \
  --dataset semantic/experiments/fixtures/questions.jsonl \
  --observations semantic/experiments/fixtures/evaluation-example-observations.jsonl \
  --output /tmp/semantica-v03-synthetic-report.json
# exit 0
```

The smoke report remains `synthetic_only: true`, `evidence_layer: "synthetic"`, with unavailable latency/token fields preserved as `null`; `acceptance-policy.json` remains `approved: false` and `proposed_thresholds: null`. This correction changes no V03 evidence layer or promotion status.
