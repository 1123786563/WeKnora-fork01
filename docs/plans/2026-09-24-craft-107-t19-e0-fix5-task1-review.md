# Craft #107 T19 E0 Fix5 Task 1 — independent review

Date: 2026-09-24. Scope: exact three-file Fix5 checkpoint, Fix5 plan/report, Fix3 Task 2 review, approved Craft web artifact Spec, `CONTEXT.md`, applicable ADRs and E0 evidence architecture. This review changed no requirements, production source, tests, remote issues or commits; it did not run OCR or Docker. The shared worktree has other workers, so conclusions bind to the checkpoint hashes below.

## Checkpoint and verification

All preimage/postimage byte counts and SHA256 values, patch and report hashes match `2026-09-24-craft-107-t19-e0-fix5-task1-checkpoint.json`. Independently applying the patch with `patch -p1` to temporary copies of the three preimages exited 0 and reproduced all three postimage hashes. Reviewed `assert_v2.py` SHA256: `92ac76ad8d54d5c9b9f41c8c2ceb081f32c463ec9badf51de67b460c83254b17`; patch SHA256: `5f7d266355e45750b49683d826c694e56db325470ca3114056b8526a9e594f44`.

Independent checks passed: `test_assert_v2.py` (one complete synthetic fixture and 62 `reject_mutation` cases), `test_source_owned_v2.py -q` (15 tests), and `py_compile` for all three owned files. These establish parser behavior only, not measured E0. The tests run in this review created temporary fixture copies; no repository test/source file was edited.

## Findings

### F1 — High — registered helper can write D's evidence volume

**Evidence:** `assert_v2.py:575-598` rejects writable *binds* on the helper but allows arbitrary writable Docker volumes. The private D/A `/ledger` comparison at lines 584-603 compares only those two mounts; it never checks the helper's mounts against either recorder volume. I copied the passing full synthetic artifact, added D's exact writable Docker volume as the helper's `/stolen-evidence` mount in both pre/post raw inspect snapshots, and recomputed the raw inspect hash. `assert_v2.verify()` returned `(True, [])`; the unmodified baseline also returned `True`.

**Impact:** The helper could alter D's source-owned ledger while its network controls run, defeating the recorder ownership boundary even though all manifest summaries and raw hashes agree. This violates the E0 evidence architecture's requirement that A/C/helpers cannot write D evidence.

**Smallest correction:** Require a fixed exact helper mount inventory (normally empty) and reject any source/name equal to, ancestor of, or alias of either recorder-owned volume. Add this self-consistent raw-inspect mutation to the adversarial suite.

### F2 — High — fixed curl attempt can be moved outside its acknowledged case window

**Evidence:** `_validate_controller_timeline` at `assert_v2.py:819-823` checks case-command ordering only when `row.get("phase_id")` is truthy. The `case` row schema at lines 448-459 requires the key but does not require a valid phase ID for scheduled negative cases. I copied the passing artifact, changed `direct_ip_pre`'s raw case row to `phase_id: null`, swapped its ordinal with a pre-phase row to keep ordinals unique and contiguous, then recomputed the host-log hash. `assert_v2.verify()` returned `(True, [])`. The exact curl argv and failed exit remain valid, but their attempt occurs before the control/case window.

**Impact:** A failed transport from another point in the run can be accepted as the direct-IP denial for `direct_ip_pre`. The paired controls and source barriers no longer bound the fixed attempted process. The 62 mutants do not cover null/cross-phase case identifiers with self-consistent ordinals.

**Smallest correction:** Map each required case to its verifier-owned phase and require that phase ID plus `begin < case ordinal < barrier`. Define and enforce explicit bounded positions for preflight, fault and cleanup case records. Add a self-consistent null-phase/early-ordinal mutation.

### F3 — Medium — helper start is ordered against acknowledgments, not source traffic

**Evidence:** `assert_v2.py:797-809` proves `begin < helper_start < barrier < helper_stop` using controller ordinals. Source `tcp_accept`/request/response rows carry source sequence and phase ID but no controller ordinal; `validate_source_streams` only binds phase begin/barrier/seal acknowledgments to the controller (lines 242, 338, 349). A source event may therefore occur after the phase-begin acknowledgment but before the logged helper start, yet appear inside the same valid phase stream. The verifier has no field with which to disprove this interleaving.

**Impact:** The claimed stop → helper start → control traffic order is not reconstructed. A positive control event may be attributed to a helper-active window it did not actually occur within. This is a remaining producer/interface gap, independent of the fixed negative curl argv improvement.

**Smallest correction:** Have the controller issue/acknowledge a helper-start boundary on the same source stream before control traffic, or retain a recorder cursor at helper start and require every control `tcp_accept` after it. Bind the boundary to the controller operation ID and test a pre-start event.

## Provenance and topology disposition

The exact `fixed_invocation()` comparison at `assert_v2.py:365-409, 457` closes the reviewed shell-inert-target path for listed negative curl cases. Verifier-owned hostile origins, provider/model IDs and attempt paths at lines 485-530 reject the self-consistent wrong-origin/provider examples. The new all-participant inspect checks catch several wrong image, UID, privilege, network attachment and A/D shared-volume changes. F1 shows that the mount trust boundary is still incomplete. Raw fixed command/result reconstruction for effective UID/GID, binary version/hash and IPv4/IPv6 routes, plus complete interface/gateway comparison, remains absent as the Task 1 report itself states; manifest/runtime free text cannot satisfy those E0 requirements.

`verify()` at `assert_v2.py:898-899` unconditionally returns BLOCKED for an artifact whose manifest `evidence_type` is not `synthetic`, because independent pinned OpenCode selected-provider and actual-attempt provenance is unavailable. The full synthetic fixture returns `PASS`, and `main()` prints `synthetic: true`. This is a parser test PASS only: `evidence_type` is a manifest label, not an independent proof of origin. A live artifact labelled `synthetic` could still receive the generic `verdict: PASS` output; downstream E0 acceptance must require a separate measured-evidence/provenance gate and must never interpret that synthetic PASS as measured E0. No non-synthetic artifact without provenance can currently pass the verifier.

The unchanged `run_v2.py` cannot emit the unified ordinal/operation IDs or distinct recorder-owned volumes required by this verifier; it still shares a host ledger bind and uses an unregistered disposable helper. No compatible runner, independent pinned OpenCode provenance source, or live Docker matrix was shown. E0 remains **BLOCKED**, and Fix5 Task 2/E1/model egress must stay gated.

## Verdict

**Spec compliance: FAIL for Fix5 Task 1 acceptance.** F1 and F2 are independently reproduced false-PASS paths under self-consistent evidence, and F3 plus the acknowledged runtime/route gaps leave the required causal and topology proof incomplete. The approved offline-egress Spec is not established by this checkpoint.

**Code quality: changes required.** The checkpoint is exact and reproducible, focused tests pass, and several Fix3 gaps were narrowed. Its mutation suite misses the two reproduced forgeries above. Re-review the corrected exact checkpoint before runner migration or measured E0.
