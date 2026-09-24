# Craft #107 T19 E0 volume-free image Task 2 — independent review

Date: 2026-09-24. Scope: Task 2 runner adoption in the integration worktree. Read the approved Craft web-artifact Spec, `CONTEXT.md`, relevant ADRs, volume-free plan, Task 1 and VF Fix1 reviews, RI-2 cleanup review, Task 2 report/checkpoint, runner/focused tests, unchanged Fix6 verifier, and retained raw smoke. No Docker, OCR, source/test, or remote issue operation was run.

## Findings

### VI-1 — Medium: the unchanged full verifier rejects the derivative image ID

**Evidence / affected symbols:** `run_v2.py:18` pins derivative `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615` and puts it in the full-run manifest. `assert_v2.py:966` still requires base `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` and raises `wrong or missing pinned image ID` for the derivative. The Task 2 checkpoint explicitly acknowledges this. The volume-free plan's architecture says the unchanged verifier checks the new manifest-pinned ID, which the current verifier does not do.

**Impact:** Any ordinary full-run artifact from this runner is deterministically rejected, even if its source, network and attempt evidence later pass. The scoped one-phase boundary smoke does not invoke or claim the full verifier and is unaffected. It cannot close full E0 or establish unchanged-verifier compatibility.

**Smallest correction:** In a separately owned verifier alignment task, update only the approved image identity check to the independently reviewed derivative ID while retaining raw participant-ID, platform, binary/source, and fixed topology checks. Add a mutation rejecting the old base ID and an integration assertion against a derivative manifest. Keep full E0 blocked until that review and the remaining physical-attempt matrix pass. Do not weaken the image pin to accept arbitrary manifest IDs.

### VI-2 — Medium: full-run manifest omits the names of C's fixed XDG volumes

**Evidence / affected symbols:** `run_v2.py:39` initializes `manifest["names"]` without `config` or `data`, although `run_v2.py:1059-1060,1083` creates and mounts those two named volumes. `assert_v2.py:632-635` obtains the expected C mount names from `manifest["names"]["config"]` and `["data"]`; both resolve to `None`, while raw Docker inspect contains the actual volume names. This omission predates Task 2, but Task 2's newly adopted fixed named C mounts make it a definite verifier mismatch. The focused tests assert argv shape and the runner's own topology predicate, not the unchanged verifier's manifest-name join.

**Impact:** Aligning the verifier's image ID alone cannot make an otherwise valid full-run C topology pass. The retained smoke's own `topology_mismatches` succeeds because it compares against the module-level `NAMES`, and its `verdict=BLOCKED` is appropriate.

**Smallest correction:** Add the exact run-owned `config` and `data` names to the full-run manifest and add a focused test that passes a representative manifest plus raw C inspect through the unchanged verifier's topology check. Preserve the exact two-volume C restriction.

## Positive evidence and limits

The previous reviewed RI-2 checkpoint reconstructs pre-Task-2 `run_v2.py` SHA-256 `78264db22e919a32511d8344d35a963fdfa49044efdc77f4283d0af41e5d5c94` and focused test SHA-256 `c787d5fb2f8085043eea6dd67e948997dca050181fc2166330171d462ea83c01`. Applying its retained patch to its retained preimages in a temporary directory reproduced those hashes. Diffing those exact bytes against current Task 2 files isolates the Task 2 changes; current hashes match checkpoint: runner `773c80f80fec877a210a1878e1b7eaf44af0682ae1729cb5069c191e85596cf1`, focused test `39301920954a34cf0268af0bd7b5d5d1cd78ba64b25b0479680caf3384b9d37d`. The Task 2 checkpoint does not retain a separate incremental patch or preimage index; this reconstruction depends on the independently reviewed RI-2 checkpoint.

The retained smoke has 124 `file-hashes.json` entries, all matching current artifact bytes; all eight artifact hashes named in the Task 2 checkpoint also match. Its pinned image inspect shows Linux/arm64, no declared volumes, UID `10001:10001`, and the expected environment/entrypoint/command/workdir. Retained preflight commands report OpenCode 1.18.4, binary SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`, and baked/local recorder SHA-256 `d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463`. The runner verifies those before participant launch. Task 1/VF Fix1 independently reviewed the derivative construction and its source isolation.

Raw C/D/A/helper inspect in `boundary-source-mount-inspects.json` shows all four use the derivative ID and UID `10001:10001`, with capability drop and no privilege. C has exactly two named writable XDG volumes, D and A each have one distinct writable `/ledger` volume with nonoverlapping source paths, and helper has zero mounts. The runner no longer binds the recorder script; D/A execute its baked path. Each root chown provisioner has network mode `none`, `CapDrop=[ALL]`, `CapAdd=[CAP_CHOWN]`, one `/ledger` mount, exited status/code 0, successful removal, and a same-name not-found inspect before its source participant is launched. Nine run-owned resources have successful removal and exact absence records. The manifest reports `source_boundary_smoke=PASS`, `cleanup_absent=true`, `verdict=BLOCKED`; the smoke lasted 62 seconds. The worker reports 48 passing focused fixture tests and Python compilation; I did not repeat them or rerun Docker.

The one-phase internal-network smoke establishes the RI-1 mount/UID correction for that scoped source boundary. It does not execute the full pinned OpenCode attempt matrix, prove provider selection or physical network attempts, or pass the unchanged full verifier. RI-2's previously reviewed low-severity exceptional CLI cleanup weakness remains unchanged and cannot yield a false PASS.

## Verdict

**Spec compliance: PASS for Task 2's bounded source-boundary smoke and volume-free topology; FAIL for the plan's unchanged-full-verifier integration claim (VI-1 and VI-2). Code quality: FAIL for full-run integration because both deterministic verifier mismatches lack a behavioral integration test.** The runner may be treated as a scoped RI-1 smoke result only. Full E0 remains **BLOCKED**, with no full PASS claim.
