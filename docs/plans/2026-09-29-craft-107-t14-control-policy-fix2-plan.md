# T14 Control Policy Fix 2 Plan

Source: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix1-review.md`. Scope is only the generated canary syntax defect and USR1 marker read race. T14 runtime acceptance remains blocked by the previously recorded Docker Buildx timeout.

## Task 1 — Make canary executable and synchronize reuse proof

- **Depends on:** Fix1 implementation and review.
- **Owner:** implementer; **validator:** backend_validator; independent reviewer.
- **Owned files:** `deploy/craft/render-boundary/policy-helper/controller.py`, `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, this plan, task brief, report.
- **Consumes:** Fix1 review finding evidence and existing bounded policy/canary behavior.
- **Produces:** Generated IPv4/IPv6 canary scripts compile; reuse marker is polled to a fixed deadline, renderer exit is detected, and exact bytes are checked.
- **Verification:** RED test for both generated families and marker success/timeout/exit; focused non-Docker unit tests; py_compile; diff check. Docker and immutable probe are prohibited for this task.
- **Failure handling:** Preserve runtime-blocked acceptance status; do not weaken canary, policy, socket identity, or resource limits.
