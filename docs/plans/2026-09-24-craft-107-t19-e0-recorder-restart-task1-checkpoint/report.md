# T19 E0 Recorder Restart Mode — Task 1 Report

Status: scoped implementation and verification complete; no E0 release claim.

## Changes

- `server_v2.py`: added fixed `restart` begin mode requiring nonempty, distinct, unused `restart_command_id` and `post_restart_inspect_id`, distinct from the begin command identity. Restart references are committed only after the source event is fsynced and returned in the private acknowledgment. The source event retains Fix6's exact schema. Restart mode is bound to phase ID `restart` and emits `phase_class=restart`. Helper-start remains control-only; restart TCP accept is retained then poisons the recorder, and no barrier can proceed through the poisoned source.
- `test_source_owned_v2.py`: added restart success, exact event schema/ack refs, missing/empty/duplicate/reused/wrong-phase refs, helper-start rejection, and early-TCP fail-closed coverage.

Runner owner confirmed it will pass and validate the two refs in the private restart request/ack; no refs are added to source events because Fix6's verifier requires the existing exact event schema.

## Verification

- RED: three restart behavior tests failed because recorder rejected restart mode. The fixed-phase guard also had a separate RED: `restart-other` was accepted.
- `python3 -m py_compile server_v2.py test_source_owned_v2.py` — PASS.
- `python3 -m unittest test_source_owned_v2` — PASS, 26 tests.
- `python3 -m unittest test_assert_v2` — PASS; synthetic verifier matrix rejects invalid cases, and the synthetic live schema remains excluded from measured E0.
- `patch -p1 -i task1-incremental.patch` against archived phase/seal postimage — PASS; both output hashes match.

## Scope and limits

Only the two assigned recorder files changed. Runner/verifier/production files were not modified. This recorder contract does not verify the referenced host operation provenance itself; the runner/verifier must bind those IDs to raw Docker restart and post-restart inspect operations. No E0 live run or release claim is included.

The exact preimage was reconstructed by replaying the prior reviewed phase/seal checkpoint patch and verifying its recorded postimage hashes before this task. Preimage, postimage, incremental patch, and manifest are in this checkpoint directory.
