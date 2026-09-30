# T19 E0 Recorder Phase Command Fix — Task 1 Report

Status: implementation and scoped verification complete; no E0 release claim.

## Changes

- `server_v2.py`: `begin_phase`, `barrier`, and `seal` require host `operation_id` and a strictly increasing `controller_ordinal`, independently of source-local `ordinal`. IDs are unique across phase controls and `helper_start`. Successful boundary events fsync these fields and acknowledgments echo source/phase/identity plus the exact event `stream_cursor`. Recorder phase and identity state is committed only after append/fsync succeeds.
- `test_source_owned_v2.py`: covers valid boundary joins, missing/duplicate/stale host identity rejection without append, fsync failure without ack or identity consumption, seal cursor/identity echo; helper-start and early-TCP coverage remains.

Runner owner confirmed the schema: phase commands are `{op, ordinal, phase_id, operation_id, controller_ordinal}` plus `mode` for begin; helper_start retains its existing schema.

## Verification

- RED focused phase tests failed before implementation because the recorder rejected host identity fields and lacked identity acknowledgments.
- `python3 -m py_compile server_v2.py test_source_owned_v2.py` — PASS.
- `python3 -m unittest test_source_owned_v2` — PASS, 23 tests.
- `python3 -m unittest test_assert_v2` — PASS; synthetic verifier mutation cases rejected, complete synthetic live schema accepted only as excluded from measured E0.
- `patch -p1 -i task1-incremental.patch` against the archived preimage — PASS; reconstructed SHA256 values match both recorded postimages.

## Scope and limits

Only the two assigned recorder files were changed. `run_v2.py`, verifier, and production files were not modified. Task 1 proves source recorder schema behavior only; runner integration and the approved live raw evidence/provenance matrix remain separate gates. This is not an E0 PASS.

The archived exact preimage was reconstructed by replaying the reviewed source-helper-boundary Task 1 patch onto its saved preimage. The resulting hashes match the task-start preimage hashes. The checkpoint directory contains the exact preimage bytes, current postimage copies, replayable incremental patch, and SHA256 manifest.
