# Craft #107 T19 E0 Fix3 Task 1 — independent review

Date: 2026-09-24. Scope: the uncommitted Fix3 Task 1 checkpoint in the integration Worktree, its plan and report, the Fix2 review, E0 evidence architecture, approved Craft Spec, CONTEXT.md and relevant ADRs. This review did not edit source or tests, run OCR or Docker, or change the measured E0 verdict.

## Checkpoint and verification

All four captured preimage SHA256 values and all six checkpoint postimage values match the current files. Applying `2026-09-24-craft-107-t19-e0-fix3-task1.patch` to the four captured preimages with `patch -p1` exited 0; every reconstructed source/test SHA256 matches the checkpoint. The patch SHA256 is `ba4a82a46e67146be52e545c2887c2e688a5ed724bfa4e2e165537e615c9c97e`. `run_v2.py` is unchanged and remains incompatible with the new schema.

Independent focused runs passed: `python3 docs/testing/craft/egress-probe/test_assert_v2.py` exited 0, and `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` ran 14 tests and exited 0. These are synthetic contract checks, not a live E0 result.

## Finding

### F1 — High — malformed second request on an expected direct control connection passes

**Evidence:** `server_v2.py:354-361,498-501` records a parse error when a later HTTP request cannot be parsed. In `assert_v2.py:274-276`, `parse_error` is checked only for an open connection. `_verify_direct_phase` at `assert_v2.py:428-452` compares completed requests to expected events but never checks parse errors on those connections. The existing malformed mutation in `test_assert_v2.py:254-294` creates a *new* connection with no valid request; its rejection comes from the one-request-per-connection rule and does not cover malformed bytes after a valid control on the same connection.

**Reproduction:** Starting with the passing complete synthetic fixture, I inserted `parse_error(connection_id="direct-c1", detail="400 malformed second request")` immediately before that connection's close, after its expected GET and response. I recomputed contiguous sequence/timestamps, final seal count, phase acknowledgment cursors and raw evidence hashes. `assert_v2.verify(root)` returned `(True, [])`.

**Impact:** A direct sink can observe malformed HTTP traffic during a control window and the artifact can still claim PASS. The Task 1 acceptance requires malformed/partial HTTP to remain visible and fail closed; the E0 architecture requires every raw event to be classified and unmatched D traffic to fail globally. This leaves a false-PASS path despite the completed keep-alive request fix.

**Smallest correction:** Track `parse_error` on each accepted connection and reject it in a direct control phase, including after a successful request on the same connection. Add a full-fixture mutation for valid expected request followed by a malformed second request, with refreshed hashes and acknowledgments, and assert rejection for the parse-error reason. Apply the same fail-closed treatment to any other unexpected D parse/error event that may currently be ignored.

## Verdict and remaining gate

**Spec compliance: FAIL for Fix3 Task 1.** F1 from Fix2 is closed for completed multi-request HTTP/1.1 traffic: every ordered request is retained and direct controls reject bypass-then-expected keep-alive requests. F4 is closed by shutdown/join/drain before the fsynced final seal, including a tested accept-versus-seal race. F5 is closed for generated IDs/ordinals, one request terminal and one connection close. The malformed second-request false PASS above remains a Task 1 blocker.

**Code quality: changes required.** The state transitions and append-failure poison are implemented and focused tests pass. The missing malformed-after-valid mutation explains why the remaining false PASS escaped the suite. Fix2 F2 (fixed control schedule and independently proved C stop/helper identity) and F3 (raw argv/config/topology/restart/cleanup reconstruction) remain unimplemented for Fix3 Task 2. E0 remains BLOCKED; this checkpoint does not release Task 2, a live Docker matrix, E1, or model egress.
