# T19 E0 source helper boundary Task1 independent review

## Scope and reconstruction

Reviewed the approved Craft #107 T19 evidence requirements, E0 evidence architecture, Fix6 verifier contract and report, Task1 plan/checkpoint/report, and only the `server_v2.py` / `test_source_owned_v2.py` increment. The saved patch SHA-256 is `116abeb8c8ccdfd60a925de9eeee4811a2ef857149ffdb1256705a68a64f7334`. Both captured preimages matched the checkpoint hashes; applying the patch in a temporary directory produced byte-identical current files with postimage hashes `87750f01e677784ee12e87965159907ef0f1d9882b39c95eaa80434475feea22` and `d0d13d875e69149bb7f91db505a9ec8a6c240fa30747b49ab98f1b9369e703d3`. No unrelated source was included in this review.

I independently ran `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q`: **21 tests passed**. The checkpoint also records passing synthetic verifier tests and Python compilation. I did not run Docker or a live E0 measurement.

## Review findings

No blocking finding in this two-file increment.

- `server_v2.py:139-171` accepts an exact `helper_start` schema only for the active control phase, a drained source, a previously unused operation ID, and a strictly increasing positive integer controller ordinal. A duplicate or wrong-phase request cannot append another event. The event carries the Fix6 fields and the recorder's `append` supplies source/run/boot identity, contiguous sequence, timestamps, flush and `fsync` before returning the sequence as `stream_cursor`.
- `server_v2.py:208-218,248-262,352-360` refuses a control barrier without the boundary. A `tcp_accept` that takes the recorder lock first is written before poisoning the source; a later helper command cannot acknowledge or seal it. If the helper command takes the lock first, its durable event has the lower source sequence. Append or `fsync` failure poisons the recorder and returns no acknowledgment.
- D and A use independent `SourceRecorder` instances and separate evidence files/control sockets. The focused test exercises both identities. HTTP headers and bodies have no route into the private control operation.

## Verdicts and limit

- **Scoped Spec compliance: PASS.** The producer emits the Fix6 source-owned helper-start boundary with the required local ordering and fatal-failure behavior. The recorder necessarily treats the private host command's Docker operation ID and ordinal as claims; the Fix6 verifier and later runner must bind them to raw successful host operations.
- **Scoped code quality: PASS.** The lock and append sequence is coherent, the patch is exact, and the focused tests cover normal, duplicate, wrong-phase, early-TCP, active, seal and append-failure cases. No source correction is required for this Task1 scope.

`run_v2.py` is unchanged and currently sends no paired helper-start commands. No real host Docker operation or live D/A E0 artifact was measured here. **Live E0 and release remain BLOCKED** until the runner supplies the real operation log, both acknowledged source cursors, and the other established evidence gates pass.
