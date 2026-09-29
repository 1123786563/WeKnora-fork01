# T55 Task 8 review repair — round 1

Source: independent review of `.superpowers/sdd/plan-t55/task-8-implementation.patch`, SHA-256 `eb0eec0c1ecbfb405c591ca9437d7be56d4312316587e318f76ea9439762bcb5`. Spec behavior was aligned; quality changes requested for missing observable coverage of `runDeliveryIntegration` and stale read-only wording.

## Findings

- **Medium:** current tests exercise config and a standalone evidence mapper but do not call `runDeliveryIntegration`. Add deterministic tests covering opt-out/no-read makes zero recovery calls; opt-in after read invokes recovery with exact run/delivery IDs and records success state; conflict/input errors record `not-needed`; other errors record `failed` and append failure. Use existing injectable transport/runtime seams; no live deployment or credentials.
- **Low:** update the flow comment to say the default flow is read-only and opt-in recovery can issue authorized dispatch/resolve requests.

## Task

- Owner: frontend_implementer. Validator: frontend_validator. Independent reviewer: reviewer.
- Workspace `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-task8`, branch `codex/issue30-b6-t55-task8`.
- Repair base: implementation `987494fb1`; current branch includes docs-only report/package commit `46844898a`.
- Owned files remain `apps/mobile/src/delivery-integration-smoke.ts` and `.test.ts` only.
- Preserve default-off/no behavior change. Tests must exercise `runDeliveryIntegration` outputs and call counts; avoid sleeps, network, and secret reads.
- RED→GREEN, run focused test, mobile typecheck, `git diff --check`; commit code/tests, report/package exact repair diff and SHA. No live deployment claim.
- Review focus: exact IDs, no recovery without read or opt-in, state/error mapping, evidence failure append, no unexpected requests, accurate comment.
