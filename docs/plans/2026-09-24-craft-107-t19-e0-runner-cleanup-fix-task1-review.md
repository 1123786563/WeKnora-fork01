# Craft #107 T19 E0 runner cleanup Fix Task 1 — independent review

Date: 2026-09-24. Scope: RI-2 only, using the cleanup Fix plan, approved Craft web-artifact Spec, `CONTEXT.md`, the preceding runner review, exact incremental checkpoint and retained two bounded smokes. No Docker or OCR run was repeated.

## Verdict

- **Spec compliance: PASS for RI-2's scoped cleanup classification.** A named owned resource is resolved only after successful removal or its type-specific, same-name already-absent response, followed by a separate exact not-found inspect. The smoke remains `BLOCKED` when any such check fails. This does not resolve RI-1 or the full E0 acceptance contract.
- **Code quality: PASS with one low-severity residual below.** The returned Docker failures and timeouts requested by RI-2 fail closed, preserve command records, and are covered by focused tests and raw smoke evidence.
- **Overall E0: BLOCKED.** The previous RI-1 image mount/UID/verifier contradiction remains. Neither retained smoke is the full pinned OpenCode matrix, provider/attempt provenance, or E0 release proof.

## Evidence checked

The patch SHA-256 is `4ee07ea9b78ecfc400942845d1a64f1db9de3f534dca9fdca351ebb0e5502f2f`. The preimage index hash is `3920b0fce77f2f0ef6821a6384c48ed83a8896ed1329e0e103dcb43661033d56`; both captured preimage file hashes and sizes match it. Applying the patch to those copies reconstructed the current `run_v2.py` (`78264db2...`, 107054 bytes) and test file (`c787d5fb...`, 12260 bytes) byte for byte. The report hash matches the checkpoint. I independently ran the 12 focused unit tests; all passed.

`cleanup_owned_resource` at `run_v2.py:325-382` uses separate container, network and volume removal/inspect argv and exact same-name errors. A success requires non-timeout removal exit 0, or non-timeout exit 1 plus exact already-absent stderr and empty stdout; then non-timeout inspect exit 1, stdout `[]`, and exact same-name not-found stderr. Daemon, permission, timeout, wrong name, and still-present responses cannot set `ok=true`. `run_boundary_smoke` at `:280-304` records each raw remove/inspect pair, sets `cleanup_absent` only if every resource resolves, and keeps the overall manifest `verdict=BLOCKED`.

The first smoke's `cleanup-evidence.jsonl` SHA-256 is `95c2a4f15d963517e2b7f12517c18ce4f81bde96766f86f6334dda6bb4d4fd34`. It records all seven same-name removals at exit 0 and follow-up inspect exit 1/`[]`; the initial classifier marked the three network/volume records unresolved because it expected empty stdout. Its `source_boundary_smoke=BLOCKED` shows fail-closed behavior. The final smoke's cleanup SHA-256 is `6e38ab7d3e2a218660812b834bf98c5324d9ca84d59082d161bc41103912eb8b`. Its four containers, two named volumes and one internal network each have exit-0 removal with the exact resource name, exit-1 type-specific same-name not-found inspect with stdout `[]`, and `ok=true`. The final manifest records `cleanup_absent=true`, `source_boundary_smoke=PASS`, `verdict=BLOCKED`. All 89 retained file-hash entries in each smoke package match current bytes; manifest cleanup hashes match the JSONL files.

## Residual finding

**Low — Local CLI invocation exceptions can interrupt the cleanup loop.** `run_v2.py:92-122, 282-290, 354-355` records normal exit and `TimeoutExpired`, but other `subprocess.run` exceptions (for example a missing Docker executable) propagate from `cleanup_owned_resource`. The `finally` loop then stops before later owned resources receive removal attempts, and the per-resource raw result/manifest is not finalized. This cannot produce a false PASS; it weakens cleanup recovery and evidence under an exceptional host failure. Smallest correction: catch invocation exceptions per removal/inspect, retain the exception and resource name as unresolved, and continue bounded attempts for the remaining owned resources before writing a `BLOCKED` manifest.

## Limit

The live evidence proves absence of the seven named resources at the immediate post-removal inspect. It does not prove the RI-1 topology contract or the complete E0 denial matrix. The previous RI-1 finding must be resolved and independently reviewed before E0 can advance.
