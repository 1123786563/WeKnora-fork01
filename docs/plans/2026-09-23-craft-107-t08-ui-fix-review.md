# T08 / #126 UI fix independent review

Date: 2026-09-23. Read-only review of the two-file `t08-ui-fix-checkpoint-01` delta in `/Users/wuyongjun/.codex/worktrees/craft-107-t08/WeKnora-fork01`. Manifest SHA-256: `d1211dbdd4790be0356f4ed078e29b2a1c7795683c0c164963bc121e3c480b05`; tracked diff SHA-256: `f5e4e0cfe14580897e7b550c2a6381f69c1fd2f97700803b93f712db8fcb81d3`. Current `access.tsx` and `access.test.tsx` SHA-256 values match the manifest: `3d34c8b03a6c7f1a1ca7e1d9ecf0480ccbbee42a2f6126e9e5de507091398ae6` and `bb6544cdf9117b1cc7c508e3e19a17bf8e6bdcc04cd3b9cabfcaf54fb967df20`. HEAD is `b18de6dd601df7cb155ffd44a86023f835cad1fc`; the worktree also has unrelated backend fix/report changes, which are excluded from this review. Sources: approved Craft web-artifact Spec, `CONTEXT.md`, ADR-0004, #126/T08 brief and plan, prior frontend validation, UI fix plan and implementation report. No OCR was run.

## Verdict

- **Scoped UI Spec compliance: PASS.** The panel covers the UI fix brief's Owner pending/success/error behavior, preserves grant input and role on failure, waits for callback success before clearing input, leaves member rows unchanged until props refresh, and renders Viewer and Collaborator without mutation controls. The source remains a projection of server authority.
- **Code quality: PASS for this two-file checkpoint, with integration evidence pending.** The state and callback handling are small and coherent; native controls retain labels, actions have accessible names, feedback has status/alert semantics, and long IDs wrap. The focused DOM test independently passed: `pnpm exec tsx --test packages/views/src/craft/access.test.tsx` exited 0 with 4/4 tests. The implementation report records `pnpm typecheck:web` and `git diff --check` exit 0; I did not rerun those commands.
- **Vertical #126 Spec compliance: NOT YET VERIFIED.** No product view imports `CraftAccess` (`rg CraftAccess packages/views/src` found only this component and its test). Thus this checkpoint cannot prove an Owner can reach the panel, callback wiring refreshes authoritative members, or backend authorization/audit and revocation apply on product routes. These are integration gates, not defects introduced by this UI delta.

## Findings

No blocking finding in the reviewed two-file UI fix. The following remains a required integration finding for #126:

### 1. High, outside this UI checkpoint — no rendered application path or live mutation adapter

**Evidence / affected symbol:** `CraftAccess` is referenced only in `packages/views/src/craft/access.tsx` and `access.test.tsx`; its `onGrant`/`onRevoke` callbacks and `members` props have no application adapter. The prior frontend validation recorded this same gap. The tests supply fake callbacks and inspect an isolated jsdom component.

**Impact:** The product cannot yet demonstrate Owner membership changes, authoritative member refresh, or read-only Viewer/Collaborator projection on a reachable Task screen. Component success does not establish #126's server-enforced or audited access outcomes.

**Smallest defensible correction:** At the central workbench/T20 integration seam, render the component from current server role/member data; bind callbacks to authenticated grant/revoke endpoints and refresh members after acknowledgement. Verify Owner, Collaborator, and Viewer behavior on the actual route, including failed mutation and revocation. Keep backend endpoint/ACL review separate from this frontend checkpoint.

## Behavioral evidence and limits

`access.tsx:24-53` awaits both mutations, guards non-Owner and pending actions, preserves draft on failure, and leaves rows in `members` unchanged. `access.tsx:55-76` uses labeled native fields, named revoke buttons, disabled pending controls, and feedback with `role=status` or `role=alert` and `aria-live`. The test covers grant rejection and retry readiness, grant success without optimistic addition, revoke rejection and success without optimistic removal, both non-Owner projections, and a long identifier. No browser viewport or screen-reader run was supplied, so actual assistive-technology announcements and layout remain unverified at the integrated route.
