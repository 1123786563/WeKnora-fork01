# Craft #107 T19 E0 volume-free runner Fix1 Task 2 — independent review

Date: 2026-09-24. Scope: VI-2 runner manifest C volume names in the integration worktree. Read the Fix1 plan, original VI-2 review, Task 2 report/checkpoint, source/test, unchanged verifier C topology predicate, reviewed Task 1 derivative-pin result, and retained raw smoke inspect. No Docker, OCR, source/test edit, or remote issue operation.

## Findings

No blocking finding in the VI-2 scope.

## Evidence

The current `run_v2.py` SHA-256 is `55e992c561f07aa4994b7fe581c6c4544014630679c8fe277e4505b7312cc385`; the focused test is `d6aa6edb7852b8e2fd6d5e74f0738af652e772f357419993f4007df075e9fa1f`, both matching the Fix1 checkpoint. This checkpoint has no separately retained patch/preimage files. Removing only the manifest-name addition and new focused test/imports in memory reproduces the parent Task 2 checkpoint hashes exactly: runner `773c80f80fec877a210a1878e1b7eaf44af0682ae1729cb5069c191e85596cf1`, test `39301920954a34cf0268af0bd7b5d5d1cd78ba64b25b0479680caf3384b9d37d`. The incremental production change is confined to the manifest initializer.

`run_v2.py:37-39` creates the unique run-owned `NAMES` and immediately puts `NAMES["config"]` and `NAMES["data"]` into `manifest["names"]`. This happens at module initialization, before `run_boundary_smoke` or the full runner's initial `write_manifest`, volume creation, and C launch. Both runner paths create those same two volumes and pass the same names to `client_xdg_mounts`; C gets exactly `/home/craft/.config/opencode` and `/home/craft/.local/share/opencode`. The unchanged `assert_v2.py:632-635` requires precisely these manifest names to match the two raw C volume mount names.

The focused test invokes `assert_v2.verify()` against a representative pre/post C/D/A/helper inspect structure. With the runner manifest names, its C XDG mismatch is absent. Removing both names or changing the config name produces `raw pre client mount set is not the fixed XDG config/data volume pair`. The test intentionally checks this predicate within an incomplete synthetic artifact; it does not assert a full-verifier PASS. I independently ran the 16 focused runner tests; all passed. The worker also reports 49 passing fixture tests, compilation, and `git diff --check`. Task 1's independent review found the exact derivative pin PASS and confirmed old-base/arbitrary-ID rejection.

The retained pre-fix smoke's raw inspect records C with exactly the run-owned `craft-e0-20260924T040250Z-70089-config` and `...-data` volumes at the same XDG destinations, while D/A each have their separate `/ledger` volume and helper has zero mounts. Those observed mount facts support the representative test shape; the old smoke manifest itself predates this addition, so it is not a post-fix full-verifier artifact.

## Verdict

**Spec compliance: PASS for VI-2's exact C volume-name manifest join. Code quality: PASS for the narrow runner change and positive/missing/tampered-name verifier-predicate coverage.** Together with the reviewed Task 1 derivative pin, the two deterministic full-verifier identity/name mismatches identified in the prior review are corrected in source. Full E0 remains **BLOCKED** pending the separate scoped integration replay and measured physical provider/network-attempt matrix; no new Docker evidence or full E0 PASS is claimed.
