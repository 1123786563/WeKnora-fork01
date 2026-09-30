# Craft #107 T19 E0 recorder restart mode Task 1 — independent review

Date: 2026-09-24. Scope: the two-file recorder restart checkpoint, its plan, the approved Craft web-artifact Spec and the existing Fix6 verifier contract. No Docker, OCR, or live E0 run was performed.

## Verdict

- **Spec compliance: FAIL for the requested source-owned operation-identity uniqueness.** The recorder correctly emits a verifier-compatible `phase_begin` with `phase_class=restart`, echoes the two private restart references, rejects malformed initial refs, and poisons early TCP. It nevertheless accepts a later phase command whose `operation_id` reuses an already accepted restart host reference (finding RR-1).
- **Code quality: FAIL for RR-1.** The new uniqueness set is checked only while validating a restart begin, leaving the ordinary controller-identity path open. Focused tests miss this cross-command collision. The independent verifier would reject the resulting global timeline, so this is a fail-closed E0 boundary, but the recorder's own acknowledgment is incorrect.
- **E0: BLOCKED.** This checkpoint does not provide raw Docker restart/inspect provenance or the fixed live schedule. The existing runner RI-1 topology conflict and full E0 evidence gates remain separate.

## Finding

### RR-1 — Medium: later source command can reuse a restart host operation ID

**Affected symbols:** `server_v2.py:117-130` (`_controller_identity`), `:191-208,230-251` (restart ref validation/commit), and `test_source_owned_v2.py:129-170` (reused-ref test).

**Evidence:** On restart begin, the source adds `restart_command_id` and `post_restart_inspect_id` to `used_restart_reference_ids`. `_controller_identity` checks only `used_controller_operation_ids`, so the subsequent `barrier` may use either host reference as its own `operation_id`. An independent private-socket reproduction sent restart begin with `restart_command_id=docker-restart-client`, then barrier with `operation_id=docker-restart-client`; both returned `ok: true`, and the barrier was appended. The added reused-ref test tries a second begin with `phase_id=restart-again`, which is rejected by the fixed phase-ID guard before the reused-reference branch is reached; it does not test the collision that remains possible.

**Impact:** The source fsyncs and acknowledges a timeline containing duplicate controller/host operation identity, contrary to the plan's fresh unique operation IDs and the Fix6 verifier's global uniqueness rule. The verifier prevents an E0 PASS, but the source no longer rejects malformed identity before state mutation.

**Smallest correction:** Have `_controller_identity` also reject IDs in `used_restart_reference_ids`, or maintain one shared used-operation-ID set for source commands and accepted host references. Add a socket test that a barrier (and later phase command) using either accepted restart reference fails without appending or advancing the cursor; retain the verifier rejection test.

## Positive evidence and limits

The archived preimage hashes match the reviewed phase/seal checkpoint postimages (`045849d1...` and `07578377...`). The 11,175-byte incremental patch hashes to `7c96d6e627ca66134069574938a84cc56534863f747ff9bb95d2ae82c4c32a42`; applying it to the captured preimage reproduces both current files and checkpoint postimage hashes (`1df05356...` and `4d05fd6c...`) byte for byte. The top-level duplicate files in the checkpoint directory are preimage copies; `postimage/` contains the recorded postimages.

The restart begin schema requires the fixed `restart` phase ID, two nonempty distinct refs, and a separate begin operation ID. The source append is fsynced before refs are committed and echoed in the private ack. Its event keeps the verifier's exact source schema without adding reference fields; the host phase command carries those refs. Helper start is control-only, and restart TCP accept is recorded and poisons the recorder. I independently ran `python3 -m unittest test_source_owned_v2 test_assert_v2`: 26 recorder tests passed and the synthetic verifier mutation suite completed as reported. These tests do not cover RR-1 or constitute measured live E0 proof.
