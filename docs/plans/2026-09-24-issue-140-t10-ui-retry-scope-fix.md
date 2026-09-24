# T10 UI retry and result identity repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep one evaluation intent tied to one request ID across timeouts and prevent stale result state when navigating between fixed snapshots.

**Architecture:** Treat POST transport timeout and uncertain write as the same pending intent. Retain its request ID, query the receipt before any retry, and retry with that ID when necessary. Mint a new ID only for a deliberate new evaluation after the prior intent is resolved. Bind local action state to `(opportunityId,snapshotId,initialEvaluationId)` so navigation remounts or resets result/retry state. Apply the retry rule to both the stable action and transient import card.

**Sources:** #150, approved Spec recovery/immutability requirements, T10 UI plan and first repair; independent re-review at `d2ab0698` high timeout duplicate and medium stale state findings.

## Global Constraints

- Isolated T10 UI worktree. Own `apps/web/src/career/OpportunityPage.tsx`, its tests and narrow affected route tests only. No Go, migration, shared API client/contract or guard changes. Preserve others' edits.
- Local commit authorized; no push, merge, deploy. A timeout is not evidence that a POST failed. An unresolved intent cannot be silently replaced with a fresh ID.
- Keep hard verdict first and fixed snapshot/fact evidence. JD text stays inert. Scope/identity change clears status and history before showing the next result.

## Review Focus

- After POST returns `TIMEOUT` or `outcome_unknown`, UI retains original request ID, looks up its receipt, and any POST retry uses that same ID. Exactly one evaluation exists for a committed-but-timed-out intent.
- Deliberate re-evaluation after prior intent resolves uses a fresh ID. Repeated click while pending does not create another intent. Transient import and stable action both follow this rule.
- Same-route navigation between snapshot/evaluation A and B never shows A's warning, link, or history under B. New requests use B's pinned IDs. Logout/403 clears local state.

---

### Task 1: Preserve uncertain intent and remount by fixed identity

**Depends:** T10 UI review-fix code `d2ab0698`. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** OpportunityPage component/tests and narrow route test only. **Consumes:** existing typed `evaluateOpportunity` and `evaluationReceipt`. **Produces:** recoverable single intent and identity-scoped UI state.

- [ ] RED: Add tests for committed POST followed by `ApiError{code:'TIMEOUT'}`, receipt recovery, retry with the same ID when receipt absent, no duplicate new ID on repeated click. Cover transient import panel and stable evidence/detail action. Add same-component navigation A→B test with distinct IDs/history/status.
- [ ] GREEN: Centralize uncertain-error classification for `TIMEOUT`, `outcome_unknown` and transport failures; keep pending request ID and reconcile before retry. Remount/reset stable action when fixed identity changes. Do not add timer loops or duplicate network calls.
- [ ] VERIFY: Focused tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`; independent Spec/quality review and frontend validation before integration. Controller then validates live browser flow on integrated code.

## Shared-file and interface preflight

One frontend implementer owns the same UI files serially. The reviewed Go and TypeScript API interfaces remain fixed. Reviewer and validator are read-only. No concurrent writers or shared Web build directory.
