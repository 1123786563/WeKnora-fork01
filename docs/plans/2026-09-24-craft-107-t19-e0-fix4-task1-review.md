# Craft #107 T19 E0 Fix4 Task 1 — independent review

Date: 2026-09-24. Scope: the Fix4 Task 1 uncommitted checkpoint and patch, its assigned plan and report, the Fix3 review, approved Craft Spec, CONTEXT.md, T19 E0 design and applicable ADR constraints. Source, tests, measured evidence and remote issues were not edited. OCR and Docker were not run.

## Checkpoint and independent verification

All three preimage SHA256 values match the captured copies; all three postimage values match the current owned files. The report, patch, unchanged `server_v2.py` and unchanged `run_v2.py` also match the checkpoint SHA256 values. Applying the patch with `patch -p1` to only the captured three-file preimage exited 0 and reproduced all three postimage hashes. The patch changes only `assert_v2.py`, `test_assert_v2.py` and `test_source_owned_v2.py`.

Independent runs: `python3 docs/testing/craft/egress-probe/test_assert_v2.py` exited 0; its complete synthetic fixture passed and all six Fix4 mutations were rejected for the new direct-error reason. `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` exited 0 with 15 tests. `git diff --check` for the owned files exited 0. These are synthetic and local recorder checks, not a live E0 measurement.

The direct stream parser at `assert_v2.py:256-280` appends an error for **every** direct `request_error` and `parse_error`, before ownership and terminal checks. Therefore a valid expected GET or CONNECT cannot authorize a later same-connection error, and a later valid request or close cannot remove the error. The full-fixture mutations refresh stream sequence, timestamps, source cursors, final seal and raw hashes. They cover GET and CONNECT followed by same-connection `parse_error`, GET and CONNECT followed by a partial second request and `request_error`, a negative-window parse error and an outside-control parse error. The unmodified clean control fixture still passes. The real local HTTP/1.1 recorder test at `test_source_owned_v2.py:86-124` observes one accepted connection's expected GET, response, malformed next request, `parse_error` and close in order.

## Finding

### F1 — Low — later-valid-request regression is absent from the fixture suite

**Evidence:** Fix4 plan Task 1 asks for a later-valid-request variant, but `test_assert_v2.py:390-423` inserts each error immediately before or after close and does not append a valid request after an error. The test named `fix4-connect-then-partial-http-request` calls the GET mutation helper, while the next test exercises CONNECT. The unconditional error append in `assert_v2.py:256-280` makes this a coverage and naming gap, not an observed false PASS.

**Impact:** A future refactor could accidentally clear or overlook an earlier error after a later successful request without this regression being exercised. The mislabeled GET case makes test coverage harder to audit.

**Smallest correction:** Add one full-fixture mutation with an error followed by a valid request/terminal/close, refreshing sequence, cursors, seal and hashes; assert rejection for the direct-error reason. Rename the GET partial-request case to match the connection it uses. This can be done with the next focused verifier test change.

## Verdict and remaining gate

**Spec compliance: PASS for Fix4 Task 1's narrow malformed/partial direct-stream acceptance.** The Fix3 same-connection malformed-second-request false PASS is closed, including GET and CONNECT controls. Direct parse/request errors fail globally, and the clean expected control fixture remains valid. No approved requirement or ADR was changed.

**Code quality: PASS with the low test gap above.** The correction is localized, fail-closed, and the focused tests pass. This review does not establish full E0 evidence or production network isolation. Fix2 F2 remains open for the fixed paired pre/post control schedule and independently proved client stop/helper identity; F3 remains open for raw argv/config/topology/restart/cleanup reconstruction. Fix3 Task 2 can now address those findings, but the unchanged runner remains incompatible with the strict evidence schema. E0 stays **BLOCKED**; the live Docker matrix, E1 and model egress remain gated on further review and evidence.
