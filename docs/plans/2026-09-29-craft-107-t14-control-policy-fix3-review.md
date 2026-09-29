# T14 Control Policy Fix 3 — Independent Review

Date: 2026-09-29 (Asia/Shanghai)  
Reviewer: `/root/t14_fix3_reviewer`  
Scope: Fix 3 plan, brief, report, frozen `fix3.patch`, prior Fix 2 Medium finding, current test source, approved Craft Spec, and T14 control policy plan. Read-only source review; no Docker or OCR run.

## Verdict

- **Fix 3 Spec compliance: PASS for the assigned deadline repair.** Both Docker callbacks in the disposable proof receive the remaining time from one monotonic deadline, and a late marker cannot be accepted. The exact `b"reuse-ok"` comparison and renderer-exit check remain in the active integration path.
- **Fix 3 code quality: PASS.** The Fix 2 Medium finding is closed for the active disposable integration path. No new blocking finding was found in the Fix 3 delta.
- **Overall T14 Spec acceptance: UNVERIFIED.** The Docker Buildx setup timeout recorded in earlier reports prevented the live established-socket reuse, fresh same-listener denial/counter, conntrack behavior, and full T14 attempt matrix from running. Source and unit evidence cannot establish those behaviors.

## Evidence

- `fix3.patch` SHA-256 is `2f6d1d3f5d6c2afd1efac795201ed2a1e892d6345568bf8d75bf231deaf4358d`, matching the Fix 3 report and frozen manifest. The current `tests/test_controller_integration.py` SHA-256 is `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`, matching the report. The baseline manifest records Fix 2's reviewed file hash `bc9affe43234b5da09654e3df9f9b7a5867be4662ea4d663736f31988fd36ab0`.
- `tests/test_controller_integration.py:30–37` computes `deadline - time.monotonic()`, refuses to start after expiry, passes that exact remainder to `subprocess.run(timeout=...)`, and rejects callbacks that finish at or after expiry. Both `docker exec ... cat /tmp/reuse-proof` and `docker inspect` call this wrapper using the same deadline at lines 565–575.
- The active polling path at lines 40–62 checks expiry before and after liveness/read callbacks and before accepting marker bytes. Lines 577–580 still require exact `b"reuse-ok"`; a newline or partial marker does not pass. The fixture at lines 387–390 writes the browser response bytes to the marker file.
- The new fake-clock regression at lines 251–270 verifies a `.25` second subprocess timeout argument, rejection of an over-deadline callback result, refusal to start after expiry, and rejection of a correct marker returned late. It does not substitute for a live Docker run.
- Independently ran the focused non-Docker command from the helper directory: **17 tests passed, 1 Docker-gated test skipped**. `py_compile` for controller/helper/adapter/integration test and scoped `git diff --check` both passed.

## Residual note

`controller.py:173–187` retains the older `wait_for_reuse_marker` helper with no callback deadline enforcement, and the older unit test still exercises it. Repository search found no active call outside that old unit test: the disposable integration now uses `wait_for_reuse_marker_bounded`. This is a low-priority cleanup opportunity if that helper is retained or reused; it does not reopen the Fix 3 finding on the active proof path.
