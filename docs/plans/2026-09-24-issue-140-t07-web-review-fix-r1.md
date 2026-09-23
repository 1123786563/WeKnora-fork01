# T07 Web review fix round 1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close two independent high-severity T07 Web review findings before integrating the upload flow.

**Architecture:** Keep the existing backend identity contract. A public source has no request ID, so the Web must not derive upload identity from file name or source history; it can resolve an ambiguous outcome only by exact replay with the retained File, request ID and expected revision. Authorization loss invalidates the page's async generation and clears private state; every awaited continuation checks that generation before writing it.

**Tech Stack:** React, CareerDesk, Node test runner, TypeScript.

**Sources:** T07 brief `docs/plans/issue-140/task-7-web-brief.md`; independent review of `85f950529..a9e4c6984`; validator report in the T07 Web worktree. Fix in the same isolated Web worktree. Code BASE `a9e4c69844adfc109cbc62d84f808102ca5a852e`.

## Global Constraints

- Own only `apps/web/src/career/CareerPage.tsx` and focused adjacent tests; narrow contract/API edits only if evidence proves required. No backend, route, migration, architecture-manifest or unrelated Web edits.
- Same request ID may be replayed only with the identical retained file and expected revision; a new attempt after terminal failure uses a fresh ID. Do not use file name, file size or digest as a public claim identity.
- A 403 or equivalent forbidden result clears private Career view, sources and upload/recovery state; a late request must not repopulate any of them. Scope switch has the same guarantee.
- Local task commits authorized, no push, merge or deploy. Independent validator and reviewer follow the fix; T07 remains running meanwhile.

## Review Focus

- Same filename in old ready/failed/processing sources cannot resolve the current unknown attempt.
- Exact replay of an ambiguous attempt gets 200 ready, 202 processing or terminal failure for that request and retains/clears state accordingly.
- Authorization loss while upload/source/refresh promises are outstanding fences every late continuation, including notice/error writes, and allows an intentional fresh read after reauthorization without stale private state.
- Tests assert public behavior, not just helper internals.

---

### Task 1: Correct ambiguous upload identity and forbidden generation fencing

**Depends:** initial T07 Web code and review. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `apps/web/src/career/CareerPage.tsx`, `CareerPage.test.tsx`. **Consumes:** backend exact replay contract and current scope generation. **Produces:** safe unknown upload recovery and permission revocation behavior.

- [ ] RED: Test an old ready/failed/processing source with the same filename as a timed-out new upload. Polling `/sources` may update visible history but must leave the retained File/request ID/revision unresolved. Exact replay must reuse that same tuple; its returned source/receipt determines outcome. Also test forbidden `/sources` while an upload promise is outstanding, then resolve that upload and assert no private source, proposal or notice reappears. Test a forbidden recovery read with a later promise continuation.
- [ ] GREEN: Remove filename-based reconciliation. Keep source refresh informational; exact upload replay is the authoritative recovery. Fence async work with a monotonically increasing page epoch on scope switch and forbidden cleanup. Recheck it after every await before setting view, sources, notices, errors or retained upload identity; cancel where existing client APIs permit. Ensure busy state is cleared without resurrecting forbidden UI.
- [ ] REFACTOR: Keep T03 action recovery behavior intact and avoid duplicating request identity state. Run focused T07 page/API/core tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`. Record RED/GREEN and command output in a round-1 report; commit only owned code/tests. Independent validation and separate Spec/quality review then decide integration.

## Shared-file and interface preflight

The backend source response intentionally omits request ID and private resource ref. This fix does not expand backend wire or TS contract. The Web task is the only active writer of `CareerPage.tsx`; T08 backend is preparation only and cannot modify it. The fix must not rely on a guessed server identity or on treating `/sources` ordering as proof of claim ownership.
