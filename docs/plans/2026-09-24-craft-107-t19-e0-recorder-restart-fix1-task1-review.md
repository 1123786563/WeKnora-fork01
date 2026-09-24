# Craft #107 T19 E0 recorder restart Fix1 Task 1 — independent re-review

Date: 2026-09-24. Scope: RR-1 in the actual recorder files `docs/testing/craft/egress-probe/server_v2.py` and `test_source_owned_v2.py`, against the Fix1 plan, previous independent review, approved Craft Spec and existing verifier contract. The plan's `deploy/craft/model-egress/` ownership path is a brief typo; the checkpoint and reviewed bytes are under `docs/testing/craft/egress-probe/`. No Docker or OCR run was performed.

## Verdict

- **Spec compliance: PASS for RR-1.** A private-socket command cannot reuse either accepted restart host reference as a later controller operation ID. Rejection happens under the recorder lock before append, ordinal advancement or positive acknowledgment. Distinct IDs still allow the restart barrier.
- **Code quality: PASS for the scoped fix.** The shared membership check is minimal, the new socket regression checks unchanged timeline/state on rejection, and the prior phase/helper-start rules remain untouched. No new blocking finding was identified in this two-file patch.
- **E0 remains BLOCKED.** This source contract is not raw Docker restart/inspect provenance, the fixed live E0 matrix, or a release verdict. Runner topology and remaining E0 gates need their own evidence.

## Evidence

The incremental patch SHA-256 is `87e074f4e7d29b6d813bd1d4b4702b2fc90c66c21334eaea6bec9e53af99a3e0`, matching the checkpoint. The captured preimage hashes match the prior restart Task1 postimages (`1df05356...` and `4d05fd6c...`). Applying the patch to those preimages reproduced the checkpoint postimages and current integration source byte for byte: `server_v2.py` `d1e7b870...`, test file `95961b9a...`.

At `server_v2.py:111-123`, `_controller_identity` now checks `used_controller_operation_ids | used_restart_reference_ids`. The restart begin commits both accepted host references after its source append/fsync; every later barrier, begin and seal passes through `_controller_identity` before mutation. The new test at `test_source_owned_v2.py:160-184` covers the previously accepted restart-command-ID collision and asserts negative socket acknowledgment, no new event, unchanged source sequence/ordinals and still-active restart phase.

I independently exercised the private socket twice: once with `operation_id=docker-restart-client`, once with `operation_id=inspect-client-after-restart` after a valid restart begin. Both returned `ok=false` with `controller operation ID was already used`, and the event list remained unchanged. In each instance, a following barrier with a distinct ID returned `ok=true` and appended sequence 3. I also ran `python3 -m unittest test_source_owned_v2`: all 27 tests passed. The checkpoint report records `test_assert_v2` synthetic verifier, `py_compile` and diff checks passing; those were not duplicated here.
