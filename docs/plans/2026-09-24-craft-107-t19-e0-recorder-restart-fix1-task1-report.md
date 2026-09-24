# Craft #107 T19 E0 Recorder Restart Fix1 Task 1 Report

Date: 2026-09-24. Scope: RR-1 only, recorder source and focused private-socket recorder test. No Docker/live E0 run, runner change, verifier change, or commit.

## Change

`_controller_identity` now rejects an operation ID found in either the controller-operation registry or accepted restart-reference registry. This shared pre-append validation means a later barrier or phase operation cannot reuse a restart command or post-restart inspect ID. The existing controller lock, append/fsync, state update, and acknowledgment order remains unchanged.

Added a socket regression that begins a valid restart phase, then sends a barrier using `restart_command_id`. It asserts an error acknowledgment, unchanged event timeline and sequence, unchanged phase ordinal/controller ordinal, and an active restart phase after rejection.

## Evidence

- RED: `python3 -m unittest test_source_owned_v2.RecorderTests.test_restart_reference_cannot_be_reused_as_later_barrier_operation_id` failed before implementation because the barrier was acknowledged (`ok: true`) with `stream_cursor: 3`.
- GREEN: `python3 -m unittest test_source_owned_v2` — PASS, 27 tests.
- `python3 -m unittest test_assert_v2` — PASS, synthetic verifier forgery matrix rejected every mutation; the complete synthetic schema case is explicitly excluded from measured E0.
- `python3 -m py_compile server_v2.py test_source_owned_v2.py` — PASS.
- `git diff --check -- docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — PASS. Replayed the incremental patch against the archived preimages; both resulting SHA-256 hashes match the captured postimages.
- No migrations or API/auth changes apply to this private recorder command path. Rejection occurs under the existing recorder command lock before timeline append, state advancement, or positive acknowledgment.

## Checkpoint

Exact preimage/postimage copies, hashes, and incremental patch are recorded under `2026-09-24-craft-107-t19-e0-recorder-restart-fix1-task1-checkpoint/`. Incremental patch SHA-256: `87e074f4e7d29b6d813bd1d4b4702b2fc90c66c21334eaea6bec9e53af99a3e0`.

## Limits

The regression exercises the restart command ID collision. The same shared membership guard covers the accepted inspect reference. No live Docker provenance or E0 PASS is claimed.
