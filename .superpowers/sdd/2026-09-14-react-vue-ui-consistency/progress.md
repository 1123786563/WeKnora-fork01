# SDD ledger — plan: docs/superpowers/plans/2026-09-14-react-vue-ui-consistency.md

## Preflight

- Workspace: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
- Branch: `codex/react-multiclient`
- Starting HEAD: `30700be9`
- Existing dirty files are preserved user work: `apps/web/src/administration/AdministrationPage.tsx`, `apps/web/src/data-sources/DataSourcesPage.tsx`, `apps/web/src/styles.css`, and evidence screenshots.
- Spec: existing Vue/React parity matrix and progress ledger; no new product design spec.

## Plan consistency scan

| Tasks | Shared boundary | Finding / ruling |
|---|---|---|
| T01/T02 | parity evidence and CSS inventory | T01 records current behavior; T02 may change only confirmed active styles. |
| T02/T03 | `packages/ui`, `apps/web/src/styles.css` | T02 owns variables and cascade; T03 owns component states and must not add page-specific overrides. |
| T03/T04 | Dialog/Sheet/Dropdown consumed by shell and settings | Public props remain compatible; focus and layering fixes require regression checks. |
| T04/T05 | shell and `App.tsx` knowledge-base surface | Sequential; shared shell changes are rechecked before KB work. |
| T05/T06 | page shell and settings overlays | Keep URL, permission and scope behavior unchanged. |
| T06/T07 | settings shell and registry panels | Shell is stable before panel batches begin. |
| T07/T08/T09 | shared UI and page-specific CSS | Page tasks consume the stable UI primitives; no parallel edits to shared files. |
| T10/T11 | all routes and platform entries | Final acceptance reopens any page affected by shared style changes. |

## Rulings

- The user authorized execution of the previously delivered plan; its content is now recorded in the plan file above.
- Existing uncommitted changes are treated as user-owned work and will not be reverted or folded into unrelated refactors.
- Vue remains the authority; if both ends match, preserve the current behavior.

## Task status

- T01: blocked-env — baseline scope and candidate register recorded, but fresh paired browser screenshots/computed styles are blocked by Chrome remote-debugging permission/request-header policy. The baseline also records `frontend/pnpm-lock.yaml` generated while starting Vue dev tooling.
- T02: complete — bounded semantic CSS alias migration in `packages/ui/src/styles.css`; fallback values preserve standalone package behavior. Verified `test:shared` 445/445, `test:web` 856/856, shared/Web typecheck and Web build, plus `git diff --check`. Manual diff review found no selector or computed-value change when theme variables are absent.
- T03: complete (code-level) — `packages/ui/src/dialog.tsx` now traps Tab/Shift+Tab within open dialogs and restores focus after close; added jsdom coverage for disabled and untabbable controls. Focused test 2/2 and `test:shared` 446/446 pass. Full browser focus/stacking verification remains blocked-env with T01.
- T04–T10: implementation already present across the current branch from prior migration slices; do not rework without fresh Vue/React evidence. Matrix still lists unresolved browser/computed-style, real-backend, Wails, Embed, iOS, and Android dimensions for several nested surfaces.
- Verification checkpoint: current `test:web` 856/856 and `typecheck:web` pass after T02/T03 changes.

## 2026-09-14 execution continuation

- Re-ran the required verification after the current worktree changes: `pnpm test:shared` 446/446, `pnpm test:web` 856/856, `pnpm typecheck:shared`, `pnpm typecheck:web`, `pnpm build:web`, and `git diff --check` all pass.
- Web build still reports the existing large-chunk advisory (the main chunk is about 4.66 MB); this is recorded as a performance follow-up and is outside the Vue visual-parity fixes in this plan.
- A concurrent integrations/channel styling slice is present in `packages/views/src/integrations/page.tsx` plus its Web render tests. Focused integration coverage now passes 14/14 (`embedPreviewFallback`, `embedWizardRender`, `imWizardRender`); this is code-level evidence only and still needs independent diff review/live parity evidence before integration.
- Vue `:5173` has been restarted and is reachable, but its browser tab is unauthenticated at `/login` while the React tab is authenticated at `/platform/settings`. Because the protected Vue/React pages are not under the same authenticated condition, T01/T11 remain blocked-env; no current paired browser/computed-style acceptance is claimed.
- After the integrations/channel slice changed, the full Web regression was rerun: 856/856 tests pass, `typecheck:web` passes, and `git diff --check` passes.
- MCP tools-directory review found and repaired one migration regression: the Tailwind conversion had removed the legacy `.wk-mcp-directory-heading` contract used by the mounted-panel parity test. The semantic hook is now retained alongside utilities; focused MCP tests pass 16/16 and the full Web suite is back to 856/856. The same review also confirmed `parametersOf` destructures `Object.entries` correctly.
- Follow-up MCP settings migration review found two additional removed semantic hooks (`.wk-mcp-metadata` and `.wk-mcp-server-docs-trigger`) still required by mounted-panel parity assertions. Both are retained alongside the utility classes; focused MCP tests pass 18/18, Web remains 856/856, `typecheck:web` passes, and `git diff --check` passes.
- Rebuilt Web after the MCP settings fixes; `pnpm build:web` passes. Vite continues to report the pre-existing large-chunk advisory (main chunk about 4.67 MB), tracked separately from parity work.
- Cross-platform regression gate also passes: Desktop tests 2/2 + typecheck, Embed tests 7/7 + typecheck, and Mobile tests 146/146 + typecheck. These are compile/unit gates only; they do not replace Wails/native runtime acceptance.
- MCP utility-style migration plus semantic-hook repairs are committed as `3828ecba` (`refactor(mcp): migrate settings surfaces to utility styles`). Plan/evidence artifacts and unrelated untracked files remain untouched.
- Renderer build gate passes: `pnpm build:desktop-renderer` and `pnpm build:embed`. Both retain existing Vite large-chunk advisories; no new build failure was introduced.
- Baseline evidence was refreshed against current runtime state: React `:5181` is reachable, Vue `:5173` served `/login` during the check but is now stopped, and the Vue tab remains unauthenticated. The baseline now records this accurately; no protected-page parity claim was added.
- Public login-page comparison is now backed by fresh Vue/React screenshots and AX trees. It found the React form card overflowing its 480px Vue container because padding was outside the box sizing, and the registration CTA used a gray/black outline instead of Vue's green outline. Fixed in commit `bf091984` with a regression test for `box-border` and the green CTA classes; focused login test, Web 856/856, typecheck, and build pass.
- Post-fix browser check confirms the React login card computes to `480px` at x≈780 (matching Vue's 480px form panel), and the create-account CTA carries the green brand border/text classes.
- Fresh public registration-page DOM comparison found React omitted Vue's required-field markers. Added the four `*` markers in commit `71f7f73e` and added a regression assertion; Web test count is now 858/858 and `typecheck:web` passes.
- Fresh dual-browser registration check confirms Vue `:5173` and React `:5181` both render a 480px registration card. React's accessibility/DOM labels now expose all four required markers, matching the Vue form contract.
- Computed-style comparison then found two more public registration differences: React used a 28px/700 heading and a plain return button, while Vue uses 24px/600 and a medium green link with hover underline. Fixed in `d9149c5b`; browser now reports 24px/600 heading and green 500-weight return action. Web regression is 859/859 and `typecheck:web` passes.
- Registration return action now also matches Vue's DOM semantics (`<a href="#">`) while preventing navigation and preserving the mode switch. Committed as `7c4628a5`; login-page tests remain green (859 Web tests total) and `git diff --check` passes.
- Current HEAD also contains the completed MCP CSS-family cleanup `7b75b96b`; after that commit, Web/shared tests, Web/shared typechecks, and `git diff --check` were rerun successfully. This closes the static MCP style cleanup portion of T07 while live/platform evidence remains open.
- Updated the Tailwind migration plan with current auth/MCP evidence and commit references (`84dc142a`); the documentation change passes `git diff --check`.
- Recorded the 2026-09-15 public auth dual-browser evidence in the baseline document and committed it as `d277a247`; protected-page and platform evidence remain explicitly separate.
- Removed the React runtime warnings emitted by the public auth page (`stroke-width`/`stroke-linecap`), using React's `strokeWidth`/`strokeLinecap` attributes in commit `6b20bb20`. Login focused tests, Web 859/859, typecheck, and diff check pass.
- Knowledge-base share-dialog utility migration is now committed as `41d125b4` (`refactor(knowledge-base): migrate share dialog to utility styles`). The custom organization picker, permission radio group, share list, responsive actions, and form footer now carry the Vue-shaped utility anatomy while preserving ARIA roles and the native required-field seam. Focused share-dialog tests pass 13/13; full Web regression remains 859/859, `typecheck:web` passes, and `git diff --check` passes. Protected knowledge-base browser parity is still pending under T01/T11 blocked-env conditions.
- Post-share migration verification: `pnpm build:web` passes (2441 modules; existing main-chunk advisory ~4.67 MB remains) and `git diff --check` passes. No new build or formatting failure was introduced.
- Knowledge-base upload progress feedback is now utility-styled in `apps/web/src/App.tsx` while retaining the `wk-upload-progress-*` semantic hooks and Vue copy/state behavior. Commit `991c011c` also closes the Wiki contributor gate: default permission is fail-closed, viewer mode hides create/edit/revert surfaces, write handlers short-circuit without contribution access, and contributor mode remains available. Wiki focused tests pass 4/4, Web full regression passes 861/861, `typecheck:web`, `build:web`, and `git diff --check` pass. The broader Vue knowledge-base permission calculation (owner/admin/editor share versus tenant role) still needs route-level integration and protected runtime evidence.
- Follow-up Wiki permission review completed in `f8f90320` (`fix(wiki): resolve knowledge-base edit permissions`). `WikiPage` now probes the KB record plus `/auth/me` and applies the shared `computeKBPermissions` rule, preserving the caller's conservative role gate if the probe fails; viewer mode also hides and short-circuits revision rollback. Added default read-only and contributor coverage. Wiki focused tests pass 5/5, Web full regression passes 862/862, `typecheck:web`, `build:web`, and `git diff --check` pass. The remaining gap is protected browser/runtime evidence and any backend-specific sharing permission fixture.
