# T19 E0 runner cleanup Fix Task 1 report

Date: 2026-09-24. Scope: `run_v2.py` and `test_runner_source_integration_v2.py` only. No verifier, source-recorder, production code, or commit was changed. Two bounded no-egress smokes were run only after the T14 UX validator and then the T14 policy helper each released the Docker slot. E0 remains BLOCKED independently due RI-1 and missing full pinned OpenCode provenance.

## Change

Added `cleanup_owned_resource(kind, name, invoke)` to classify remove and inspect outcomes per Docker resource type. A cleanup is accepted only when removal succeeded, or returned the exact resource-specific already-absent error for the same name, and a separate inspect returns exit 1, stdout `[]`, and the exact not-found stderr for that resource and name. Container, network, and volume commands are classified separately. Timeout, daemon/permission/unrelated errors, inspect success, and mismatched resource names remain unresolved. Both complete command records (argv, exit status, stdout, stderr, timeout and runner metadata) are embedded in each cleanup evidence row. The one-phase source-boundary smoke sets `cleanup_absent` only when every resource is resolved and reports `source_boundary_smoke=BLOCKED` on any cleanup failure.

## RED / GREEN and smoke evidence

Initial RED before the classifier existed:

`python3 -m unittest docs.testing.craft.egress-probe.test_runner_source_integration_v2`

Result: 11 tests ran with 8 error reports because `cleanup_owned_resource` did not exist; the existing helper-boundary tests passed.

After first implementation, 12 tests passed. The bounded smoke then exposed a real response-shape mismatch: OrbStack's exact network/volume inspect not-found responses include stdout `[]` as well as the exact stderr. The first strict classifier expected empty stdout, therefore correctly failed closed and yielded `source_boundary_smoke=BLOCKED` (not PASS). Captured raw cleanup evidence is at `docs/testing/craft/egress-probe/e0-fix2/boundary-smoke-craft-e0-20260924T023547Z-75847/cleanup-evidence.jsonl` (SHA-256: `95c2a4f15d963517e2b7f12517c18ce4f81bde96766f86f6334dda6bb4d4fd34`). It shows all four containers, two volumes and one internal network were removed with exit 0, each follow-up inspect exited 1 with stdout `[]`, and exact name-matching not-found stderr. No resource was left behind. This evidence motivated the correction to require `[]` for all inspect types.

Final targeted run after correction:

- `python3 -m unittest docs.testing.craft.egress-probe.test_runner_source_integration_v2` — **12 tests passed**, including exact network/volume inspect response shapes and wrong-name rejection.
- `python3 -m py_compile docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/test_runner_source_integration_v2.py` — passed.
- `git diff --no-index --check /dev/null <each owned file>` — both untracked owned files passed whitespace checks (the command exits 1 for ordinary differences; no check diagnostics were emitted).

After the parent confirmed the Docker slot was free, the bounded smoke was rerun with the corrected `[]` inspect shape. It completed with `source_boundary_smoke=PASS`, `cleanup_absent=true`, and all seven per-resource cleanup records resolved. The overall manifest still says `verdict=BLOCKED`; this is one recorder/runner boundary phase, not E0. The first smoke's `source_boundary_smoke=BLOCKED` was the observed response-shape mismatch and did not leave resources behind.

## Incremental checkpoint

Patch: `docs/plans/2026-09-24-craft-107-t19-e0-runner-cleanup-fix-task1.patch` (SHA-256: `4ee07ea9b78ecfc400942845d1a64f1db9de3f534dca9fdca351ebb0e5502f2f`). Exact pre/post file hashes and byte counts are in `docs/plans/2026-09-24-craft-107-t19-e0-runner-cleanup-fix-task1-checkpoint.json`. Applying the unified patch to the captured preimage copies in a temporary directory reconstructed both postimage files byte for byte.

## Remaining risks

- RI-1 image, mount and UID incompatibility identified by the prior independent review remains unresolved and out of scope.
- The exact Docker message forms intentionally fail closed; an engine variant not matching the observed exact messages will be marked unresolved rather than accepted.
- The bounded live rerun covers the exact observed Docker response shape for the seven cleanup resources. It does not resolve the separate RI-1 runtime mount/UID conflict or supply the fixed E0 matrix/pinned OpenCode provenance.
