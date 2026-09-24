# T19 E0 volume-free image Task2 Fix1 report

**Checkpoint:** `t19-e0-volume-free-image-task2-fix1-20260924-01`  
**Status:** scoped runner fix verified; independent review pending.  
**Parent checkpoint:** `t19-e0-volume-free-image-task2-20260924-01`. No commit created.

## Change

The runner's initial `manifest["names"]` now includes exact per-run `config` and `data` volume names from `NAMES`, before the first `write_manifest()` and before either volume is created. This makes the manifest's C mount mapping join the raw named XDG volumes consumed by the unchanged verifier topology predicate.

Added `test_manifest_c_volume_names_join_raw_inspect_in_existing_verifier` in the focused runner test file. It runs the existing `assert_v2.verify()` implementation against representative raw C/D/A/helper inspect. Exact run-owned C volume names clear the C mount predicate; removing both manifest fields or tampering the config name reproduces the expected rejection. It does not claim the partial synthetic evidence is a passing full artifact.

The Task1 verifier worker changed only `assert_v2.py`/its verifier tests under the disjoint ownership. This task changed only `run_v2.py` and `test_runner_source_integration_v2.py`; no Dockerfile or recorder/verifier file was edited here.

## TDD and verification

- RED: before adding `config`/`data`, the focused regression failed because the unchanged verifier reported `raw pre client mount set is not the fixed XDG config/data volume pair` against the representative raw C mounts and missing manifest values.
- GREEN: the same focused regression passes with the runner names; missing and tampered names remain rejected.
- `python3 -m unittest discover -s docs/testing/craft/egress-probe -p 'test_runner_source_integration_v2.py' -v` — PASS, 16 tests.
- `python3 -m unittest discover -s docs/testing/craft/egress-probe -p 'test*.py' -v` — PASS, 49 tests in 16.083 seconds. The mutation suite confirms the reviewed derivative is accepted and old-base/arbitrary image IDs are rejected.
- `python3 -m py_compile docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/test_runner_source_integration_v2.py` — PASS.
- `git diff --check -- docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/test_runner_source_integration_v2.py` — PASS.

No Docker or paid egress was used. Full E0 remains BLOCKED.

## Incremental checkpoint

`run_v2.py`: `773c80f80fec877a210a1878e1b7eaf44af0682ae1729cb5069c191e85596cf1` → `55e992c561f07aa4994b7fe581c6c4544014630679c8fe277e4505b7312cc385`.  
`test_runner_source_integration_v2.py`: `39301920954a34cf0268af0bd7b5d5d1cd78ba64b25b0479680caf3384b9d37d` → `d6aa6edb7852b8e2fd6d5e74f0738af652e772f357419993f4007df075e9fa1f`.

The preimage hashes were reconstructed by removing only this fix's manifest-name addition and focused test/imports; both match the parent checkpoint hashes exactly. The JSON checkpoint records this delta and the test evidence.

## Remaining gates

Independent review of this Fix1 task remains pending. Full E0 still requires the physical attempt/provider-provenance matrix; this runner test is only the C volume-name manifest join.
