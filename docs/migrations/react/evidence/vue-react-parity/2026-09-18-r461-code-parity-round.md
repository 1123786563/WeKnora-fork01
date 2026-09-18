# 2026-09-18 Round R461 — wiki edit-permission contract, AuthPages dead-stack removal, creator_id fix (4 parallel agents + orchestrator)

Round type: TDD round with 4 parallel agents (A1 post-recovery browser evidence, A2 AuthPages dead-stack
deletion, A3 wiki edit-permission contract, A4 verifier) plus an orchestrator-executed cross-domain fix from
the A1+A3 findings. Verdict: **all PASS**; final gates: test:web 1673/1673 (A4's 1672 pre-closure + the
creator_id test), test:shared 751/751, typecheck 0, build ✓, integrity 0 P0 after staging.

## A1 — Post-recovery browser evidence (PASS, 12 screenshots)

- **Wiki edit-button gap CONFIRMED with a live root cause**: React shows only 历史 where Vue shows
  编辑/新建页面/新建目录/历史/删除. Browser-level API capture: the live KB payload carries ONLY `creator_id`
  (no user_id/created_by/my_permission; /auth/me role=null, memberships=[]) → permissions.ts isCreator never
  matched → canContribute=false. Vue's isOwner matches creator_id → true. (canUploadKnowledgeDocuments in the
  same file already includes creator_id — an omission, not a contract.)
- Settings panel flicker regression (external 59c9dd88): PASS — 5-section fast switching, 40ms sampling, zero
  blank frames. New queued finding: nested `<form>` inside the share section (hydration console warning).
- agent-chat crash fix regression (5e382d0b): PASS — existing session renders, new session works, zero
  console errors, zero data pollution.
- embed-channels still empty (mermaid e2e precondition unchanged).

## A3 — Wiki edit-permission contract (PASS)

Vue canEdit (KnowledgeBase.vue:288-293) mirrored branch-for-branch in a new `wiki/edit-permission.ts`:
org-share grant ∈ {admin, editor} (authoritative source: the org shared-KB list, which React never queried),
else creator (creator_id match), else tenant role ≥ admin with contributor EXPLICITLY excluded (React used to
accept contributor). WikiPage's mount probe now fans into three inputs (KB detail + /auth/me +
listShared-with-tolerance). 11 unit tests + wiring test; wiki 59/59. Left: the router.tsx initial prop
(`role() !== 'viewer'`) flips one frame before the probe lands (router domain); the permissions.ts creator_id
bug affects FAQ/documents too (fixed by the orchestrator below).

## A2 — AuthPages dead-stack removal (PASS, net −203 lines)

Reachability re-verified: the new router lazy-loads standalone LoginPage/JoinPage/WorkspaceOnboardingPage
files, NOT AuthPages exports; zero external references remained (R460's edits were mechanical typecheck
convergence, not reachability). Deleted AuthPages.tsx + auth-state.ts + its test; migrated 2 live
auth/api.ts contract tests into weknora-user-session.test.ts (coverage must not die with the dead stack).
scoped 78/78; integrity 0 P0. Left: createAuthApi now test-only (persistLogin still active) — minimal-delete
principle kept it; next-round adjudication.

## Orchestrator — permissions.ts creator_id fix (A1+A3's cross-domain finding)

`isCreator` now matches `kb.creator_id` FIRST (the only creator field the live backend sends; user_id/created_by
kept as legacy fallbacks), mirroring Vue isOwner — fixing the same gap for every computeKBPermissions consumer
(FAQ batch bar, documents surfaces). TDD: new test pins creator_id-match grants editing and mismatch stays
plain-member. This plus A3's contract makes the creator's wiki controls appear under the real backend.

## Gates (final)

`pnpm test:web` 1673/1673, `pnpm test:shared` 751/751, `pnpm typecheck:web` 0, `pnpm build:web` ✓,
`pnpm check:integrity` 0 P0 after staging (the interim P0 was the unstaged new file — the script's first live
catch of its own target shape). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r461/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
