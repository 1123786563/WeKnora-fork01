# R8 Task 2 Fulfilled Receipt Customer Scope Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make fulfilled-order exception recovery query the expected top-up receipt in the tenant customer scope required by the production gateway.

**Architecture:** Keep the existing recovery checks and transaction boundary. Derive the customer from the loaded order's tenant using the established `OrderCustomerID` helper, attach it with `domain.WithBenefitCustomer`, and call `FindBenefit` with that scoped context. Strengthen the service fake so the success assertion fails unless the gateway receives the expected customer.

**Tech Stack:** Go, commercial fulfillment domain context, service unit tests.

**Spec:** R8 Task 2 fix round 1 plan `docs/plans/issue-72-r8-task2-fix-r1.md`; Task 2 review finding in `.superpowers/sdd/issue-72-ocr-findings-r8/task-2-fix1-review.md`; approved Lago payment/fulfillment Spec AC 6–7 and 22; ADR-0012; `CONTEXT.md`.

## Global Constraints

- Derive the customer scope from `order.TenantID` loaded from the database, never from the outbox payload or request caller.
- Preserve invalid-winner, missing/empty receipt, Pending/open, and atomic resolution behavior from fix round 1.
- Do not call `ApplyBenefit` in already-fulfilled exception recovery.
- Modify only `internal/modules/commercial/service/commercial/fulfillment.go` and `fulfillment_test.go`; preserve existing reports and unrelated changes.
- Commit owned files locally only as `fix(commercial): scope fulfilled receipt recovery`; do not push.

## Review Focus

- Corrected valid winner plus existing receipt succeeds only with `tenant_<order.TenantID>` scope.
- A missing/empty receipt still remains Pending/open.
- Invalid winner remains Pending/open and no benefit is applied.
- No-open-exception fulfilled event preserves the existing fast path.
- Cancellation and transient gateway errors do not acknowledge the event.

## Task 1 — Require and pass tenant-scoped receipt lookup

**Dependency:** Fix round 1 commit `bbf79a784ace4dad4c470d61fe44a1c70a4bfd60`; its independent validation passed, but review found the production gateway contract unmet.  
**Owner:** backend_implementer. **Validator:** backend_validator.  
**Owned files:** the two Go files listed above.  
**Consumes:** `domain.WithBenefitCustomer`, `OrderCustomerID`, `domain.BenefitCustomerFrom`, `FulfillmentService.fulfillEvent`, the focused fix-round-1 regression.

- [ ] **Step 1 — RED:** Extend the test gateway stub with an optional required customer ID. When set, make `FindBenefit` fail unless `domain.BenefitCustomerFrom(ctx)` exactly matches it. In `TestFulfillmentTopUpFulfilledOpenExceptionRequiresWinnerAndReceipt`, after correcting the winner and storing the nonempty deterministic receipt, set required customer to `OrderCustomerID(7)` before the final recovery. Run:
  `go test ./internal/modules/commercial/service/commercial -run '^TestFulfillmentTopUpFulfilledOpenExceptionRequiresWinnerAndReceipt$' -count=1`
  Expected before production edit: the final recovery fails to resolve because fix round 1 passes an unscoped context; earlier invalid/missing/empty receipt assertions continue to pass.
- [ ] **Step 2 — GREEN:** In the open-exception fulfilled branch in `fulfillEvent`, call `FindBenefit(domain.WithBenefitCustomer(ctx, OrderCustomerID(order.TenantID)), domain.FulfillmentKey(order.ID, "credits"))`. Keep all other behavior unchanged.
- [ ] **Step 3 — Verification:** Re-run the focused regression, `go test ./internal/modules/commercial/service/commercial -count=1`, and `git diff --check`. Expected: all commands pass; the fake confirms exact tenant scope.
- [ ] **Step 4 — Report and commit:** Write `.superpowers/sdd/issue-72-ocr-findings-r8/task-2-fix2-report.md` with before/after RED, exact tests, hashes and HEAD. Commit only the two owned files with the specified message.

## Self-review

- **Coverage:** The review finding is directly tested at the concrete gateway context seam. Prior fix-round-1 acceptance remains covered by the same regression.
- **Step clarity:** RED asserts the old branch fails because customer scope is absent; GREEN specifies the exact context helper and tenant source.
- **Interface consistency:** Helper names and lookup key match existing production callers and fix-round-1 code.
- **Review focus:** The required customer assertion is in the test fake; prior tests continue to cover invalid, missing/empty receipt, and no-Apply behavior.
- **Scope:** One missing context value only; no migration, API or policy changes.
