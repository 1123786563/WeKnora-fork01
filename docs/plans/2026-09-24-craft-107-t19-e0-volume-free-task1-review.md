# Craft #107 T19 E0 volume-free image Task 1 — independent review

Date: 2026-09-24. Scope: Task 1 checkpoint only. Read the approved Craft web-artifact Spec, `CONTEXT.md`, relevant E0 evidence architecture, RI-1 review, volume-free plan/report/checkpoint, Dockerfile, focused tests, unchanged Fix6 verifier and retained raw evidence. No Docker or OCR run, production edit, or remote issue change.

## Findings

### VF-1 — Medium: provisioning and cleanup are not reconstructable from retained commands

**Evidence / affected files:** The plan requires separate bounded pre-chown provisioning and proof that the provisioner was removed. `volume-free-task1-report.md` says the raw provisioner commands are retained, including `--rm --network none --cap-drop ALL --cap-add CHOWN`. `volume-free/topology-commands.txt` contains four volume names and six bare container IDs, without argv, exit codes, ownership/chown output, or a provisioner name. `volume-free/topology-writer-checks.txt` contains only two successful UID/path strings. `volume-free/topology-cleanup.txt` proves the four named participant containers, four volumes and two networks were absent, but has no provisioner removal or absence observation. The focused test counts the `[]` strings in that cleanup file and does not validate the preceding commands or provisioner lifecycle.

**Impact:** The observed UID 10001 writes show the ledger volumes were writable in this probe, but the retained evidence cannot establish how ownership was set, whether the claimed capability and network bounds applied, or whether the temporary privileged provisioner was removed. This leaves an explicit Task 1 acceptance condition unproved.

**Smallest correction:** Retain the exact provisioner create/run/remove argv, exit status, target volume names, and a specific not-found inspect for each provisioner. Bind the focused test or a small read-only evidence check to those records. Keep the existing participant inspect and writer evidence.

### VF-2 — Medium: the focused topology oracle omits the fixed verifier's source-isolation predicate

**Evidence / affected symbol:** `test_volume_free_image.py:topology_errors` requires different D/A volume `Name`s, but does not compare their `Source` paths or ancestor aliases. The unchanged `assert_v2.py` topology check compares the D/A `/ledger` mount `Source`s and rejects equality or parent/child overlap. A mutation with different names and identical sources passes `topology_errors`; none of the malformed tests mutates `Source`. The retained real D/A inspect sources are distinct, so this is a test coverage defect rather than an observed unsafe mount.

**Impact:** A future image/runner change could satisfy this task's focused test while exposing D's recorder evidence to A and failing the unchanged verifier. The test's claimed verifier compatibility is broader than it proves.

**Smallest correction:** Reuse the unchanged verifier's mount/UID predicate through a properly shaped test artifact, or mirror its complete D/A source equality and ancestor checks in the focused test. Add a mutation with distinct names sharing one `Source`.

## Positive evidence and scope limit

The checkpoint lists 31 files. Every listed SHA-256 matches current bytes; the patch SHA-256 is `91684377c87cc218fb6e84548200d310c9077864a1230c299cb2e4a5d71608d7`, and reconstructing its 31 added files reproduces current bytes. The Dockerfile pins base `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, copies its root filesystem through `scratch`, bakes `server_v2.py` as mode 0444, and restores the inspected environment, UID, workdir, entrypoint, command and port. Base and derivative raw image inspect show Linux/arm64, the same config fields, and derivative ID `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615` with empty `Config.Volumes`. Retained isolated commands report OpenCode 1.18.4, binary SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`, and UID 10001-readable baked recorder SHA-256 `d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463`, matching source bytes. The final build log resolves the pinned base and emits the inspected derivative digest.

Raw C/D/A/helper container inspect all use that derivative ID, UID `10001:10001`, no privilege or added capabilities, and `CapDrop=[ALL]`. C has exactly the two named writable XDG config/data volumes; D and A each have one distinct writable `/ledger` source; helper has no mounts. These mount/UID facts satisfy the corresponding unchanged Fix6 predicates. The disposable probe does not satisfy the full Fix6 topology: its networks were deliberately internal-only and D had no published 8083 control port. It therefore cannot be passed as a complete verifier artifact. The cleanup file gives specific absence diagnostics for the named participants, volumes and networks, subject to VF-1's provisioner gap. The base inspect and four base participant inspects show the two inherited XDG volumes that caused RI-1.

## Verdict

**Spec compliance: FAIL for Task 1 completion**, limited to VF-1's missing provisioning/removal proof; image identity and the scoped mount/UID layout are supported. **Code quality: FAIL**, because VF-2 leaves a security-relevant mismatch between the focused test and the fixed verifier. Task 2 should wait for the corrected evidence and focused test review. The derivative is a local fixture only; full network matrix, source joins, OpenCode physical-attempt provenance and E0 acceptance remain **BLOCKED**.
