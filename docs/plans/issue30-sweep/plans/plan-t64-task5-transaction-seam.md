# T64 Task5 Plan Supplement — atomic revocation and audit persistence

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Supplement to `plan-t64.md` Task5; architecture ruling 2026-09-28.

**Goal:** Satisfy Task5's same-transaction ledger+audit requirement without exposing transaction handles to the service.

**Evidence:** `AgentSecurityStore.AppendReleaseRevocation` and dependency peer each write independently and hold a private `*gorm.DB`; `NewAuditLogRepository(tx)` accepts a transaction-scoped GORM handle. `AgentSecurityService` owns store/run-store only. Therefore the plan's two-file ownership is insufficient for required atomicity.

## Ownership adjustment
- Add/modify `internal/application/repository/agent_security.go` and its test only for typed transactional append+audit operations.
- Continue Task5 service changes in `internal/application/service/agent_security.go` and its test.
- No raw DB/transaction callback exposed to service. Keep existing standalone append methods for compatibility.

## Interface and behavior
Add typed store methods for release and dependency revocation, each accepting the ledger entity plus sanitized `*types.AuditLog`; internally run `s.db.WithContext(ctx).Transaction`, create ledger row, then `NewAuditLogRepository(tx).Create(ctx, audit)`. Return any error so both writes roll back. The service invokes this method, then performs T64 Task3 run cancellation after commit and updates canceled count through existing typed store methods. Do not put run cancellation into ledger/audit transaction.

## Required tests
- Audit insert failure leaves zero ledger rows and zero audit rows.
- Successful revoke persists exactly one tenant-scoped ledger row and matching audit action/scope.
- Existing in-flight cancel/allow and canceled-count semantics remain unchanged.

Role remains backend_implementer. Local commit authorized; independent reviewer/backend validator follow.
