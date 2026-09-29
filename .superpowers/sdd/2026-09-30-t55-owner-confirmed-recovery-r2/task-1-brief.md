# Task 1 Brief — Complete T55 Confirmed Dispatched Recovery

**Plan:** `docs/plans/issue30-sweep/plans/2026-09-30-t55-owner-confirmed-recovery-r2.md`
**Base:** `30adf8b1afcd12fb0c175bbf16267bc6432a252d`
**Findings:** R1 independent review High F1 (confirmation dropped on ActionSucceeded path), High F2 (dispatched→pushed CAS source omitted), Low F3 (JSON null accepted).
**Worktree:** `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-t55-repair-backend`.
**Owner:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** `reviewer`.

## Objective

Repair only those three findings. Preserve existing owner/run checks, explicit confirmation semantics, no-POST resolve behavior, and generic A03 callers fail-closed.

## Owned paths

- `internal/modules/codedelivery/service.go`
- `internal/modules/codedelivery/dispatcher.go`
- `internal/modules/codedelivery/service_dispatch_test.go`
- `internal/modules/codedelivery/service_gitlab_test.go`
- `internal/handler/session/workbench_delivery.go`
- `internal/handler/session/workbench_delivery_test.go`
- `internal/modules/codedelivery/repository/codedelivery/store.go` (minimal transaction seam required: atomically write branch receipt and CAS state so failed source-state CAS cannot leave a misleading receipt)
- `internal/modules/codedelivery/repository/codedelivery/store_test.go` (transaction behavior regression: rejected CAS leaves state and receipt unchanged; successful CAS updates both)

Do not edit other files unless a minimal atomic store seam is proven necessary; if so, update this brief before editing. Preserve all unrelated artifacts.

## Required checks

Test first. Cover both A03 action states for a persisted dispatched delivery: no attestation remains blocked; confirmation plus absent PR and present branch settles to `pushed`; Action and Delivery remain consistent. Add a JSON null rejection case. Run package checks from the plan, commit only owned paths, and write `task-1-report.md` with commands, RED/GREEN evidence, payload hash and remaining late-provider risk. Do not spawn agents.
