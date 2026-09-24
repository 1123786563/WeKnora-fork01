# Craft #107 T19 E0 Source Helper Boundary Task 1 Report

Date: 2026-09-24. Owned files: `server_v2.py` and `test_source_owned_v2.py` only. No runner, verifier, production code, Docker run, or commit was performed. E0 remains default-off and **BLOCKED** until the later runner task emits the real operation log and paired source acknowledgments.

## Implementation

The private Unix control socket accepts a new exact-shape command:

```json
{"op":"helper_start","phase_id":"<active control phase>","operation_id":"<successful docker-start operation>","controller_ordinal":123}
```

The recorder requires the active phase to be a control phase, no existing source connections/requests, no prior acknowledgment in that phase, a non-empty unused operation ID, and a positive strictly increasing integer controller ordinal. On success it appends the Fix6 `helper_start` event under the recorder's shared RLock; the existing append path flushes and `fsync`s it before the socket replies with its source cursor. D and A are separate `SourceRecorder` instances and tests exercise both source identities.

A control phase cannot cross its barrier without the helper-start acknowledgment. If a TCP accept reaches the recorder first, it appends the source-owned accept row, poisons the recorder, and refuses to produce a helper-start acknowledgment or successful seal. The HTTP server closes that accepted socket while keeping its listener loop alive; later traffic remains rejected by the poisoned recorder. Lock ordering defines the race: either the fsynced boundary gets the lower source sequence, or the accepted socket is recorded first and the stream is poisoned.

The source recorder validates that the supplied operation ID is fresh and its controller ordinal increases among helper-start boundaries. It cannot itself prove that Docker executed the operation successfully or know that a well-formed caller-supplied ID refers to a real host command. The later runner must issue the private command only after observing the successful Docker start, record the same operation ID/ordinal in its raw host log, and send the exact same pair to both D and A. The Fix6 verifier binds each event to that raw host row. The current runner is unchanged and does not yet issue this command.

## RED/GREEN verification

- Added socket tests for successful event shape/cursor/fsync on both D and A; wrong phase, empty/reused operation ID, invalid/non-increasing ordinal, duplicate boundary; active recorder state; control barrier/seal bypass; early TCP through a real listener; and fsync failure with no acknowledgment/state advancement.
- The new tests failed against the preimage because `helper_start` was an unknown command, control accept was not gated, and control barrier skipped the boundary. They now pass.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — 21 tests passed.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed; the synthetic verifier suite continues to reject 66 evidence mutations, including Fix6 absent-boundary and pre-boundary-traffic cases. This synthetic parser result is not measured E0 evidence.
- `python3 -m py_compile docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.
- Owned-file trailing-whitespace scan — passed.
- Patch replay against the captured preimage — passed; exact per-file postimage hashes are in the checkpoint.

On injected `fsync` failure, append may already have written bytes before the filesystem reports failure. The recorder does not return an acknowledgment, does not advance its helper identity state, and is poisoned, so no subsequent successful seal can certify that stream.

## Remaining blockers

1. `run_v2.py` has not been migrated to send `helper_start` after a successful raw Docker start and before allowing control traffic; therefore current live artifacts lack the required D/A source boundaries.
2. The host log must bind the actual successful Docker start row to the same operation ID/controller ordinal and preserve each recorder's returned source cursor. The recorder trusts the private host socket caller for that operation fact.
3. Earlier E0 gates remain: compatible unified timeline and isolated recorder volumes, independent pinned OpenCode selected-provider/actual-network-attempt provenance, and complete fixed runtime/route evidence. No evidence here releases E0 or enables production networking.
