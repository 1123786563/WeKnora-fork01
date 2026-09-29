# T14 Control Policy Fix 3 Plan

Source: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix2-review.md`, MED finding “reuse marker deadline does not bound Docker reads”. Scope is limited to the disposable integration test's marker polling subprocess callbacks and regression tests.

## Task 1 — Enforce marker polling wall-clock budget

- **Depends on:** Fix2 source and independent review.
- **Owner:** implementer; **validator:** backend_validator; independent reviewer.
- **Owned files:** `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, this plan, task brief, report.
- **Produces:** Both Docker callback subprocesses receive the current remaining deadline; each callback rejects late completion before returning data; marker bytes and renderer exit semantics stay intact.
- **Verification:** RED then GREEN test with stubbed slow command verifying timeout forwarding and late-result rejection; focused non-Docker tests; py_compile; diff check. No Docker/live probe.
- **Failure handling:** Preserve exact `reuse-ok` check and runtime-blocked T14 status.
