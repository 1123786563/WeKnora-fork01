# T10 evaluation draft attribution race repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Never show an evaluation for JD A beneath JD B after a user starts a new draft while A's evaluation is pending.

**Architecture:** Bind each transient evaluation attempt to the fixed imported receipt `(opportunityId,snapshotId)` and a local draft generation. Increment the generation when starting a new draft or clearing private state. A late POST or receipt lookup may update the visible card only when scope, generation, and fixed receipt identity still match. Keep the original request ID for uncertain-write recovery; avoid mutating the old immutable result.

**Sources:** #150, approved Spec evidence/scope rules, T10 UI plans and independent review at `59fdec92` medium late-result attribution finding. Three preceding UI repair rounds passed targeted tests but exposed separate state transitions; per AGENTS.md round-4 upgrade, use a fresh Sol default agent with frontend ownership constraints for this bounded UI fix.

## Global Constraints

- Isolated T10 UI worktree. Own only `apps/web/src/career/OpportunityPage.tsx` and `OpportunityPage.test.tsx`. You are not alone in the codebase; preserve others' edits. No Go, API client/contract, route, migration or guard changes.
- Local commit authorized; no push, merge, deploy. A pending evaluation remains a recoverable intent with its original request ID, but old results cannot appear under a new JD. JD text is inert.
- The prior 403 private-state clearing and hard-first warning remain intact.

## Review Focus

- Start evaluation for saved JD A; before POST resolves, start new draft and import JD B; late A success/unknown/receipt response does not put A status/link under B or restore A text.
- B evaluation uses B's fixed IDs and a fresh request ID. A's persisted result remains accessible via its own stable URL if it committed.
- Scope/logout/403 invalidates pending visible attempts; repeated clicks still create one intent; normal A success without draft change still renders.

---

### Task 1: Fence transient results by draft generation and fixed receipt

**Depends:** T10 UI 403 code `59fdec92`. **Owner/validator:** upgraded frontend implementer / frontend_validator. **Files:** OpportunityPage component/test. **Consumes:** current import receipt and evaluation APIs. **Produces:** local result attribution guard.

- [ ] RED: Add deferred-promise tests for A→new draft→B with late A POST and receipt lookup outcomes; assert no A result under B. Confirm current failure. Add normal same-draft success control.
- [ ] GREEN: Capture a draft generation and fixed receipt identity at attempt start; invalidate on new draft and private-state clear. Before every result/history/state mutation after await, require the captured identity still matches. Avoid unnecessary network cancellation or new request IDs for uncertain A intent.
- [ ] VERIFY: Focused tests, `pnpm typecheck:web`, `pnpm test:web` once, `pnpm build:web`, `git diff --check`; document any unrelated full-suite failure with focused affected checks. Independent Spec/quality review and frontend validator before integration.

## Shared-file and interface preflight

One upgraded frontend implementation agent writes two UI files. Reviewer and validator remain read-only. The backend and typed Web interface are stable; no parallel Web build in this isolated worktree.
