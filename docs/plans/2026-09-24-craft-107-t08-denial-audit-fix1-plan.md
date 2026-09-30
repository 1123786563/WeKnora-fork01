# T08 denial audit fix1: per-route evidence

> **For Codex:** Correct the one Medium test-evidence finding with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent re-review. No production policy change or commit.

**Sources:** `2026-09-24-craft-107-t08-denial-audit-plan.md`, Task1 report/checkpoint and `2026-09-24-craft-107-t08-denial-audit-task1-review.md`; B5 joined test. T14 production preview remains disabled pending live browser/network proof.

## Global Constraints

Own only `internal/router/craft_b5_joined_test.go` and focused new test-only preview service/handler fixture if necessary; `craft_access.go` and other production source are read-only. Preserve production auth/router/persistent ACL. For each reachable denied direct read/version/download request, assert the exact new durable `craft.access_denied` row for current tenant/actor/Task/action and bounded details, while retaining zero bytes and zero opener calls. Do not assert an ACL denial for preview when the production browser gate short-circuits first. A separately named test-gated preview request may reach the real service ACL with controlled network/browser fakes, but must be labelled non-production evidence and not set production flags.

## Review Focus

Per-request before/after row attribution; hidden 404 no Task-targeted row; no duplicate rows from nested calls; revoked Viewer direct request; preview policy test reaches CheckTaskAccess rather than disabled gate; no raw data in rows; exact checkpoint.

## Task 1

**Depends on:** Task1 independent quality FAIL. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** router joined test and optional new focused preview ACL test file only. **Consumes:** current durable audit log and real persistent ACL. **Produces:** route-level evidence for each reachable refusal.

1. RED: make aggregate audit assertion fail when one denied route omits its own row; add exact delta and row-field assertions for list/direct/version/download/revoked cases that actually reach Task ACL. For pre-ACL 401/hidden 404/gated preview, assert appropriate zero Task-targeted row instead of fabricating an ACL denial.
2. Add a separate explicit test-only preview service/handler configuration that reaches TaskPreview ACL under controlled passed network/browser check, then revoke and assert exactly one bounded audit row plus zero bytes. Keep production constructor/gates unchanged.
3. Run focused router, preview handler/service and Craft access tests, `git diff --check`, save full test pre/post hashes, fix-only patch and report. Obtain independent review. Separate existing broad service ACL fixture failure for its own test-only correction.

**Acceptance:** every claimed denied HTTP ACL decision has its own attributed durable row; non-ACL gates are honestly distinguished. **Failure handling:** if a route cannot reach Task ACL with existing fixtures, report exact seam and narrow claim, never enable production preview or fake a passing row.
