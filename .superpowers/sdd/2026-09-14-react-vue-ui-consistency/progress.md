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
- Platform shell session pagination is now implemented in `985a9147` (`feat(platform): paginate shell session list`). The shell listens on the actual overflow container, loads subsequent Web-session pages near the scroll end, de-duplicates rows, preserves loaded rows on later-page failure, exposes a localized retry action for initial failure, resets state on client/generation changes, and keeps the listener stable through collapse/expand. Independent review found no P1/P2 after the lifecycle fixes. Focused shell tests pass 8/8, Web full regression passes 864/864, `typecheck:web`, `build:web`, and `git diff --check` pass. Source-bucket filtering, batch management, and protected runtime evidence remain open.
- Chat composer model chip is now an explicit disabled button in `7c1523d5` because the current React stream contract has no `model_id` selection field. This preserves the Vue-shaped display while preventing a misleading interactive affordance and adds `aria-disabled="true"`. Focused chat tests pass 21/21, Web full regression passes 865/865, `typecheck:web`, `build:web`, and `git diff --check` pass. Full model selection, attachment upload, and knowledge-base mention flows remain open pending an end-to-end API/UI contract.
- Chat answer bookmark is now a native disabled button in `8800090c`, retaining the `aria-disabled` hook while preventing focus or accidental activation until the manual-editor/API path exists. The focused chat suite remains 21/21 and `git diff --check` passes; the broader chat attachment/mention and real runtime evidence remain open.
- Chat attachments are now a complete code-level slice across `4b62db40`, `2bc25f16`, `50cdfe0e`, and `883f383b`: the composer opens a multi-file picker, validates Vue-aligned count/size/type limits (including runtime size overrides and dynamic parser extensions), uploads against the active session, preserves uploaded/processing/ready/failed states with polling, sends only real `attachment_ids`, and cancels/cleans up stale operations on removal, session switch, or unmount. Failed deletion remains retryable. Focused coverage passes 32/32, Web full regression passes 871/871, `typecheck:web`, `build:web`, and `git diff --check` pass; independent review found no P1/P2. Browser/backend/platform evidence remains open.
- Knowledge-base `@` mention support is now a code-level slice in `e6861f5e` and `b02e2ce6`: the composer loads accessible tenant KBs, provides search, listbox selection, ArrowUp/ArrowDown/Enter/Escape keyboard behavior, active-option ARIA state, removable chips, localized loading/empty/error copy, and sends `mentioned_items` only when selected. Client/scope generations clear stale data and release loading locks safely. Focused coverage passes 35/35, Web full regression passes 875/875, `typecheck:web`, `build:web`, and `git diff --check` pass; independent review found no P1/P2/P3. Document/file/tag/MCP/skill mentions, steer mention payloads, and protected runtime evidence remain open.
- Steer now carries the same KB mention contract in `fadbada8` and `cae22003`: the running-turn composer supports the accessible mention picker and serializes `mentioned_items`, while explicitly blocking submission when attachments are present so they cannot be silently dropped. Focused coverage passes 32/32, Web full regression passes 879/879, `typecheck:web`, `build:web`, and `git diff --check` pass; independent review found no P1/P2. Full document/file/tag/MCP/skill mention parity and protected runtime evidence remain open.
- Session source filtering is now complete at code level across `205a4224`, `283af9e7`, and `06b1e977`: the shell discovers admin-visible API/Embed/IM buckets, maps display keys to backend sources, probes counts before exposing the filter, reuses scroll pagination and retry behavior, and resets stale sources on client, role, or bucket changes. Focused shell tests pass 12/12, Web full regression passes 883/883, `typecheck:web`, `build:web`, and `git diff --check` pass; independent review found no P1/P2/P3. Protected runtime evidence and remaining Vue inline/batch session actions remain open.
- Shell session renaming now follows the Vue inline editor in `0f95ffb0` and `e4d872ca`: Enter/blur submit once, Escape cancels, titles are normalized and capped at 80 characters, failed updates remain editable, and successful updates synchronize both React state and the pagination ref. Focused shell tests pass 16/16, Web full regression passes 887/887, `typecheck:web`, `build:web`, and `git diff --check` pass; independent review found no P1/P2. Batch management/spinner and protected runtime evidence remain open.
- Batch session management is now code-complete across `b0f39ef9`, `284a0e3b`, and `126be8b9`: batch mode toggles rows without navigation, uses the backend batch-delete API, exposes indeterminate select-all state, hides source filtering and row menus while active, keeps a sticky localized action bar, reloads page one after deletion, and localizes all visible/ARIA controls across five locales. Focused shell/chat-copy/API tests pass 18/18, 8/8, and 8/8; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2/P3. Protected browser/runtime and Wails/native evidence remain open.
- Chat header session renaming now matches Vue inline editing across `8a6cb414`, `b07d84de`, `89e80098`, `7c25d66e`, `e9b10457`, and `b22d79c5`: the header menu closes into an inline title editor, selects the full title, submits on blur/Enter with duplicate-submit protection, supports Escape/cancel and trigger-focus restoration, shows visible localized failures, normalizes and caps titles at 80 characters, and preserves the existing sidebar callback contract. Focused chat coverage passes 26/26; `typecheck:web` and `git diff --check` pass. Independent review found no P1/P2/P3. Protected runtime evidence remains open.
- Mobile knowledge-base uploads now support multi-file picking and ordered queue processing across `a08ef44`, `bfe7aa62`, and `ac69f728`: single-file compatibility remains intact, failures continue through the queue, cancellation stops later files, lifecycle events and refreshes are preserved, and structured duplicate/unknown upload errors are mapped to five-language user copy without losing backend error codes. Focused API tests pass 10/10 and mobile queue tests 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native runtime evidence and byte-level progress remain open.
- Mobile recents and favorites are now isolated by authenticated user and tenant in `6ffeb7a5` and `f645ae5d`: runtime exposes the stable `auth.me().user.id`, uses the Vue-style `WeKnora_{userId}_t{tenantId}_resource_recents` key with an explicit anonymous namespace, clears all identity state on refresh failure, and commits workspace hydration atomically after generation checks. Focused mobile coverage passes 24/24; typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native runtime evidence remains open.
- Mobile knowledge graph copy and states are now localized in `eb853ddd`: depth, node counts, bounded-overview hint, links, familiar marker, load failure and empty graph all use the five-language knowledge surface table; unknown server node types remain data-driven and no unsupported capability gate was added. Focused graph tests pass 3/3; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native graph runtime evidence remains open.
- Mobile document detail and preview copy is now localized across `8bb3d9d1` and `12de9f01`: detail actions, metadata, loading/error states, and download-only preview labels use five-language keys, with optional preview labels preserving ChatScreen compatibility. Focused coverage passes 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native file-preview evidence remains open.
- Mobile Wiki/FAQ editor copy and validation are now localized across `0552f5bd` and `e42ed731`: labels, permission/conflict/load/save states, and required-field validation use typed five-language keys while preserving owner/admin gating and existing callers. Focused editor coverage passes 8/8; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native editor evidence remains open.
- Mobile document-list failure states are now localized in `41de0f18`: document loading, filter loading, and upload failure fallbacks use five-language knowledge keys while preserving queue, pagination, cancellation and refresh behavior. Focused coverage passes 4/4; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native document-list evidence remains open.
- Mobile data-source surfaces now localize list/editor/test/sync-log failures and connector fallbacks in `b0ef4055`, including permission, cleanup, resume, capability, and placeholder states across five locales. Existing sync, pagination, credentials, and editor behavior is preserved. Focused data-source coverage passes 23/23 plus i18n 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native data-source evidence remains open.
- Mobile Wiki/FAQ reference labels now use locale-aware injected copy in `dbb9e7cb`: loading/error states, empty titles, versions, FAQ enabled/disabled/recommended states, and edit labels are localized without changing slug/id routes, permissions, or pagination. Focused reference coverage passes 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native reference evidence remains open.
- Mobile chat surfaces now use five-language copy across `0de1d4ed` and `ceb64491`: session/KB/message/steer/approval/MCP/artifact states, action buttons, and the NativeArtifactPreview drawer labels are localized without changing stream lifecycle or API behavior. Focused chat/i18n coverage passes 2/2 and artifact-label coverage 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native chat runtime evidence remains open.
- Mobile identity capability screen now uses five-language management copy in `5a22da52`: title, explanation, loading/error/empty states, supported/unavailable status and server-provided reasons are localized without inventing capabilities. Focused coverage passes 3/3; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native management evidence remains open.
- Mobile API key management now uses five-language copy across `cada5697`, `7ce141de`, and `f41da865`: loading/errors, owner-only read-only guidance, create/revoke flows, token states, accessibility labels, and Korean destructive-action wording are localized without changing API or permission behavior. Focused coverage passes 2/2; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native management evidence remains open.
- Mobile configuration management now uses five-language copy in `42789cf7`: page/section states, CRUD actions and confirmations, form labels/placeholders, MCP transport/enabled states, loading/errors and partial-result handling are localized without changing API, permission, credential or read-only skill behavior. Focused coverage passes 2/2; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native management evidence remains open.
- Mobile ManagementHub capability catalog now uses five-language labels, reasons, mode statuses and role text in `9bf93b84`; server-supplied reasons remain authoritative, default disabled copy is localized, and fail-closed projection/routing behavior is unchanged. Focused capability coverage passes 4/4; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native management evidence remains open.
- Mobile organization management now uses five-language copy across `9f65d3fc` and `5919e274`: organization/member/share/invitation flows, permissions, confirmations, empty/error states and unnamed-resource fallbacks are localized while server roles and resource values remain authoritative. Focused organization coverage passes 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native organization evidence remains open.
- Mobile administration now localizes member, invitation and audit surfaces across five locales in `57f62a70` and `a3c85066`: owner/admin/contributor/viewer/system-admin roles plus allowlisted audit action/outcome/actor enums use typed keys, unknown server values remain visible, and locale changes invalidate error fallbacks. Focused administration coverage passes 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Native administration evidence remains open.
- Web ModelSettings/ModelDebug residual style families now use utilities in `9fbe4044`; selected tabs, filters, editor/debug dialogs, result panels, responsive sizing and scroll behavior remain Vue-aligned, with shared `.wk-model-tabs` retained only for SandboxSettingsPanel. Focused settings coverage passes 27/27; Web full regression passes 890/890, build/typecheck and `git diff --check` pass. Independent review found no P1/P2/P3. Browser pixel and protected runtime evidence remain open.
- Web WeKnora Cloud settings now localize the description, save hint and model/status fallback labels in `c56a411f`; server-returned status values remain authoritative while missing values use five-language copy. Settings coverage remains 15/15, Web typecheck/build and `git diff --check` pass. Independent review found no P1/P2/P3. Protected runtime evidence remains open.
- React authenticated Chrome AX evidence for the WeKnora Cloud settings route was captured in `222de744` (`evidence/vue-react-parity/2026-09-15-r013-cloud-settings-runtime.md`); the available Vue tab redirected to login, so same-session Vue pixel comparison and backend credential scenarios remain open.
- Web Ollama settings now localize service description, address/model labels, download guidance, progress success feedback and task/model size fallbacks in `68ed4897`; download, refresh, progress and credential behavior remain unchanged. Settings coverage remains 15/15, Web typecheck/build and `git diff --check` pass. Independent review found no P1/P2/P3. Protected runtime evidence remains open.
- Authenticated React Chrome AX evidence reached `/platform/settings?section=ollama`; the page showed localized Chinese title/description and the server-reported Ollama-unavailable state. The local Ollama service was not running, so model/download success paths and same-session Vue comparison remain open.
- Web parser configuration copy now uses existing localized endpoint and MinerU credential keys in `68c59b8f`; retrieval field mutation semantics remain unchanged. Settings coverage remains 15/15 and Web typecheck/diff checks pass; independent review found no P1/P2/P3.
- Web resource settings now localize editor labels, provider placeholders, safe-config copy, security guidance and row fallbacks across five locales in `42d82596`; sensitive config filtering and CRUD/test/default flows remain unchanged. Settings coverage remains 15/15, Web typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Web parser settings now localize the API-key safety hint across five locales in `fc5b2377`; endpoint/API-key fields and parser test/save behavior remain unchanged. Settings coverage remains 15/15, Web typecheck and build pass. Independent review found no P1/P2/P3.
- Web settings section headings now map storage, vector-store and web-search routes to their Vue i18n title/description keys in `f5426f9d`; no English inventory fallback leaks in these sections. Surface tests pass 20/20 and Web typecheck/diff checks pass. Independent review found no P1/P2/P3.
- Authenticated React Chrome AX evidence for `/platform/settings?section=storage` confirms localized section heading, description, editor labels and row fallbacks; same-session Vue pixel comparison remains open.
- Authenticated React Chrome AX evidence for vector-store and web-search settings confirms localized section headings/descriptions and editor labels; same-session Vue pixel comparison remains open.
- Authenticated React Chrome AX evidence for `/platform/settings?section=retrieval` confirms the sidebar label now renders localized `搜索设置`; admin-only visibility and controls remain intact.
- Authenticated React Chrome AX evidence for `/platform/settings?section=parser` was captured in `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r013-parser-settings-runtime.md`; localized fields and safety hint are visible, while successful parser connectivity and same-session Vue comparison remain open.
- Web MCP delete confirmation now uses the shared localized `mcpSettings.deleteConfirmBody` interpolation in `9eb90ba0`; permissions, built-in protection and delete lifecycle remain unchanged. MCP-focused tests pass 18/18 and independent review found no P1/P2/P3.
- Authenticated React Chrome AX evidence for `/platform/settings?section=mcp` confirms localized MCP heading, description and add action; no service row was present to exercise the destructive confirmation dialog, and Vue comparison remains open.
- Chat grep tool results now follow Vue grouping and metadata in `20c99de6` and `27ffb52e`: ordinary documents aggregate title matches, FAQ rows stay distinct and use localized FAQ-entry copy across five locales. Shared tests pass 453/453 and Web tests pass 890/890; independent review found no P1/P2, with two non-blocking P3 coverage notes.
- Shared i18n contract test now separates React Native-owned `mobileChat.*` keys from the Vue web-chat baseline in `05211164`; mobile keys remain required across all five locales, and shared tests pass 453/453.
- WebFetchRenderer now uses the Vue `chat.unknownLink` copy for missing URLs across five locales in `d53998a9`; shared tests pass 454/454 and Web focused coverage/typecheck/build pass. Independent review found no P1/P2, with one non-blocking P3 renderer-locale coverage note.
- WebFetch summary/error precedence now follows Vue in `5ae7365b`: summary error fallback fields stay hidden when a non-empty summary exists, while real fetch errors remain visible. Shared tests pass 457/457 and Web tests pass 890/890; independent review found no P1/P2, with one non-blocking mixed-fixture P3 note.
- ChunkDetailRenderer now uses Vue `chat.fullContentLabel` across five locales in `b72e0946`; focused renderer coverage is 44/44, shared tests 458/458 and Web tests 890/890. Independent review found no P1/P2/P3.
- The shared utility batch `085de06e` adds Vue `chat.chunkIdLabel` copy to ChunkDetailRenderer across five locales; focused renderer/copy tests cover the label and independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note.
- ChunkDetailRenderer now uses Vue `chat.positionLabel` across five locales in `873589a6`; shared tests pass 460/460 and Web tests 890/890. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note.
- ChunkDetailRenderer now uses Vue `chat.documentIdLabel` across five locales in `7561cda4`; shared tests pass 461/461 and Web tests 890/890. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note.
- ChunkDetailRenderer now uses Vue content-length label/unit keys across five locales in `2d616875`; shared tests pass 462/462 and Web tests 890/890. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note.
- ChunkDetail now uses Vue-aligned five-locale labels for chunk ID, document ID, position and content length across the recent N021 commits; shared tests 462/462 and Web tests 890/890 remain green, with no P1/P2 in independent reviews.
- RelatedChunksRenderer now uses Vue `chat.noRelatedChunks` for empty results across five locales in `2c091387`; shared tests pass 463/463 and Web tests 890/890. Independent review found no P1/P2/P3.
- KnowledgeBaseListRenderer now uses Vue `chat.knowledgeBaseCount` interpolation across five locales in `77f4988b`; shared tests pass 464/464 and Web tests 890/890. Independent review found no P1/P2/P3.
- Mobile chat quick prompts now use locale catalog entries in `01452baf`: summarize and related-file suggestions populate localized prompts in all five supported locales, removing the last hardcoded English prompt text from the chat composer. Mobile tests pass 187/187; mobile and shared typechecks pass. Native chat runtime evidence remains limited to the Android login screen; iOS remains at the development-client scheme prompt.
- React Chrome AX 平台路由复核记录于 `2026-09-15-r014-platform-route-ax.md`：知识库与智能体列表的分组筛选、计数、空态/内置条目、创建入口和共享侧栏均可达；Vue `:5173` 当前未认证回 `/login`，因此同条件像素/computed-style 与写路径仍为 `blocked-env`。
- 回归门禁复核（`01452baf` 后）：`pnpm test:shared` 465/465、`pnpm run test:web` 891/891、`pnpm typecheck:shared`、`pnpm typecheck:web`、`pnpm typecheck:mobile`、`pnpm build:web` 全部通过；构建仍报告既有主包约 4.8MB 的 chunk warning，未改变本轮行为。
- 桌面与 Embed 当前门禁复核：`pnpm test:desktop` 2/2、`pnpm typecheck:desktop`、`pnpm test:embed` 7/7、`pnpm typecheck:embed` 均通过；这属于静态/组件层证据，Wails 宿主和真实 Embed 容器运行仍需环境可用后验收。
- `tool-result` 渲染链补齐一批剩余英文泄漏：搜索/数据库/grep 空态、Shell 工作目录/退出码/stdout/stderr/二进制与空输出、Chunk/文档/WebFetch/Plan/Thinking 空态现在通过 `ChatCopyTable` 使用五语言文案，提交 `2cad677a`。共享回归 465/465、Web 回归 891/891、Web 类型检查通过；工具结果标题、计数元数据和服务端原始状态值仍保持数据/协议语义不变。
- `tool-result` 状态文案继续收敛：五语言新增搜索/记录/匹配/空输出、工作目录、退出码、标准输出/错误、二进制隐藏和未命名文档标签，并由各 typed renderer 读取活动 `ChatCopyTable`；提交 `2cad677a`。shared 465/465、Web 891/891、Web typecheck 通过；工具标题与命中计数仍保留下一轮 Vue 文案核对项。
- 工具结果标题已支持活动语言映射，同时保留 `toolResultPresentation()` 无 copy 调用的既有英文兼容行为，修复测试回归；提交 `2cad677a` 后续修正。shared 465/465、Web typecheck 通过。
- 工具结果标题五语言键正式提交为 `fix(chat): add localized tool result titles`，由 `ToolResultView` 按活动 `ChatCopyTable` 渲染；无 copy 的纯函数调用仍保持旧英文兼容。提交后工作树仅保留原有未跟踪用户文件。
- 工具结果标题映射提交后的 Web 全量回归保持 891/891，`git diff --check` 通过；当前提交链为 `5a87f8c1`、`fa31a5a6`、`16271df3`。
- 工具搜索命中元数据已支持活动语言：`toolChunkHits`/`toolKeywordHits` 五语言键接入 Search/Grep view，且无 copy 的旧调用继续输出原英文格式；提交 `fix(chat): localize tool hit metadata`。shared 465/465、Web typecheck 通过。
- Shell 工具结果的 killed/truncated 状态已接入五语言 `ChatCopyTable`，提交 `fix(chat): localize shell status labels`；shared 465/465、Web typecheck 与 `git diff --check` 通过。
- 工具结果详情字段已本地化：评分、片段数、FAQ/文档 ID、答案、描述、来源、文件、URL 和错误码均通过五语言 `ChatCopyTable` 渲染；提交 `fix(chat): localize tool detail fields`。shared 465/465、Web typecheck、`git diff --check` 通过。
- ChatPage 余留英文 accessibility/fallback 文案已收敛：Live response、空态 starter 区、主聊天区、沙箱终端以及 action/steer 异常均使用活动 `ChatCopyTable`；空态条件避免 aria 标签干扰欢迎文案判断，提交 `fix(chat): localize accessibility fallbacks`。Web 891/891、typecheck:web、git diff --check 通过。
- ChatPage 发送异常的非 Error 回退已改用活动语言 `copy.sendFailed`，并将沙箱终端测试断言同步为 zh-CN 文案；Web 回归 891/891、typecheck:web、git diff --check 通过。
- R015 完成 Chat literal audit（`2026-09-15-r015-chat-literal-audit.md`）：确认 tool-result/page 可见文案已覆盖，剩余英文主要是 aria 稳定标识、协议枚举、测试 fixture 和待确认 Clipboard 能力异常；没有发现新的可见英文泄漏。
- 移动端/桌面/Embed 当前回归复核：`pnpm test:mobile` 187/187、`typecheck:mobile`、`test:desktop` 2/2、`typecheck:desktop`、`test:embed` 7/7、`typecheck:embed` 全部通过；Clipboard 底层异常因 UI 层静默捕获，不构成可见文案泄漏。
- MessageList 的滚动到底部 aria 标签与产物预览非 Error 回退已接入五语言 copy，提交 `fix(chat): localize message list fallbacks`；Web 891/891、typecheck:web、git diff --check 通过。
- R016 登录页双端运行态复核已完成：Vue `:5173/login` 与 React `:5181/login` 均 HTTP 200，在同一中文 Chrome 会话中捕获 AX 与视觉截图；品牌区、登录卡片、字段、按钮、注册入口和能力摘要结构一致。公开认证入口的 runtime 阻断解除，受保护路由/角色/Wails/native/写路径仍开放。

## Round N+34az — system global settings copy parity

- Updated `SystemGlobalSettingsPanel` with five-locale copy for risk confirmation, tabs, empty states, metadata and controls.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r021-system-global-settings-copy.md`.
- Remaining: authenticated runtime checks across locales and high-risk save/reset interaction evidence.

## Round N+34ba — cross-package regression after settings copy

- Shared suite: 465/465.
- Mobile suite: 189/189.
- Embed suite: 7/7.
- Desktop renderer suite: 2/2.
- No additional failures observed; React Web remains 891/891 from the preceding slice.

## Round N+34bb — system audit log copy parity

- Added five-locale copy for audit log heading, controls, empty/loading/error states and detail drawer labels.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r022-system-audit-copy.md`.
- Remaining: authenticated runtime evidence for audit pagination/detail and non-Chinese locale rendering.

## Round N+34bc — platform API key copy parity

- Reworked `PlatformApiKeysPanel` copy into five-locale local fallback table while preserving API behavior and permission capability values.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r023-platform-api-keys-copy.md`.
- Remaining: authenticated runtime create/revoke evidence across supported locales.

## Round N+34bd — runtime queue task copy parity

- Added five-locale task drawer and task-state copy in `RuntimeQueuesPanel`.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r024-runtime-queues-task-copy.md`.
- Remaining: environment-dependent authenticated queue task drawer evidence.

## Round N+34be — API key capability label parity

- Added locale-aware display labels for all seven system capability values without changing backend payload identifiers.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r025-platform-api-key-capability-labels.md`.

## Round N+34bf — capability display labels

- Separated API key capability display labels from backend identifiers and localized all seven permissions across five locales.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r026-platform-api-key-capability-display.md`.

## Round N+34bg — runtime fallback locale coverage

- Added four non-Chinese runtime fallback dictionaries so queue and limiter surfaces no longer fall back to Chinese when shared i18n keys are absent.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r027-runtime-fallback-locales.md`.

## Round N+34bh — system global empty locale

- Removed the remaining hardcoded Chinese empty-state fallback from system global settings and covered all five locales.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r028-system-global-empty-locale.md`.

## Round N+34bi — system setting field labels

- Added five-locale labels for known auth, tenant, runtime, SSRF and sandbox settings while preserving unknown-key fallback behavior.
- Validation: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r029-system-setting-labels.md`.

## Round N+34bj — Web production build baseline

- Verified `pnpm run build:web` succeeds on the current branch.
- Confirmed Mermaid renderer already uses dynamic imports; manual vendor split was rejected after producing circular chunks without reducing the initial app chunk.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r030-web-build-baseline.md`.
- Follow-up: route-level code splitting remains an optional performance task requiring a separate measured design.

## Round N+34bk — mobile upload progress accessibility copy

- Replaced the hardcoded English upload-progress accessibility label with the localized upload progress detail message.
- Validation: `pnpm test:mobile` 189/189; `pnpm run typecheck:mobile`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r031-mobile-upload-a11y-label.md`.

## Round N+34bl — mobile graph loading accessibility

- Corrected the graph screen loading indicator accessibility label to use the localized loading message.
- Validation: `pnpm test:mobile` 189/189; `pnpm run typecheck:mobile`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r032-mobile-graph-loading-a11y.md`.

## Round N+34bm — desktop renderer build evidence

- Verified `@weknora/desktop-renderer` production build succeeds.
- Recorded emitted visualization chunks and the remaining bundle-size warning without changing chunk behavior.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r033-desktop-build.md`.
- Remaining: Wails host launch and authenticated parity evidence.

## Round N+34bn — Web route lazy loading

- Converted protected route page imports in `apps/web/src/main.tsx` to `React.lazy` with a shared loading boundary, preserving auth pages and route guards.
- Production build confirms initial index JS reduced from ~4.92MB to ~3.05MB minified, with route-specific chunks emitted.
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r034-web-route-lazy-loading.md`.

## Round N+34bo — settings panel lazy loading

- Converted settings panels and integration route to `React.lazy` under a shared `Suspense` boundary; kept system-global panel eager to preserve its existing synchronous render contract.
- Production build reduced SettingsPage chunk from ~562KB to ~49KB (gzip ~15KB).
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r035-settings-panel-lazy-loading.md`.

## Round N+34bp — lazy-loading boundary localization

- Connected the protected-route Suspense fallback to the shared locale table, removing the hardcoded English loading label.
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r036-lazy-loading-copy.md`.

## Round N+34bq — cross-package regression after lazy loading

- Re-ran shared, mobile, embed and desktop suites after Web route/settings lazy loading changes: 465/465, 189/189, 7/7 and 2/2.
- Web remained green at 891/891; all package typechecks and diff check passed.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r037-cross-package-after-lazy-loading.md`.

## Round N+34br — settings lazy-loading runtime verification

- Fixed import-order regression discovered in the browser (`ReferenceError: Cannot access 'lazy' before initialization`).
- Authenticated browser re-check now renders the settings dialog and expected role-denied state for a non-system-admin account.
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r038-settings-lazy-runtime.md`.

## Round N+34bs — entry error copy and label regression

- Localized the Embed isolated-entrypoint error and aligned the system settings test with the new localized field-label behavior.
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r039-entry-error-and-label-test.md`.

## Round N+34bt — mobile export toolchain gate

- Attempted Expo Web export; toolchain reports missing `react-dom` and `react-native-web` dependencies.
- No dependency changes made because this package targets native mobile acceptance; export failure is recorded as `blocked-env`, not as a code regression.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r040-mobile-export-toolchain.md`.

## Round N+34bu — platform shell deep imports

- Switched PlatformShell session-sidebar and guide primitives from the views barrel to direct module imports to reduce unnecessary coupling.
- Validation: `pnpm run build:web`; `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-15-r041-shell-deep-imports.md`.
- Web eager-entry import coupling was reduced in `a043d127` (`refactor(web): use direct platform shell imports`) and the follow-up direct imports in `apps/web/src/App.tsx`/`routes.tsx`: contextual-guide and integration-route consumers bypass the `@weknora/views` barrel. Web typecheck passed, full Web regression remains 891/891, and build passed; the eager `index` chunk measured about 2.74 MB versus the earlier 3.05 MB baseline. Evidence: `evidence/vue-react-parity/2026-09-15-r042-web-direct-view-imports.md`. Remaining bundle work is intentionally scoped for a later import-by-import review.
- Settings lazy chunks now import `renderChatMarkdown` and `roleAtLeast` from their owning modules in `ac2ce3ce`, removing two remaining barrel edges. Web typecheck, 891/891 regression tests, and build passed. Evidence: `evidence/vue-react-parity/2026-09-15-r043-settings-direct-view-imports.md`.
- Protected React knowledge settings runtime re-verified on the shared backend at `/knowledgeBase/14ea2229-67bf-4f27-b759-a08b0dc23563/settings` after lazy-route and direct-import changes: the full settings surface renders, parser capability reasons remain truthful/disabled, and no mutation was performed. Evidence: `evidence/vue-react-parity/2026-09-15-r044-knowledge-settings-runtime.md`. Save-success and native evidence remain open.
- Authenticated chat runtime re-verified at `/platform/creatChat`: the localized KB mention control opens the searchable listbox and exposes the tenant KB option with active selection semantics. Evidence: `evidence/vue-react-parity/2026-09-15-r045-chat-mention-runtime.md`. The evidence explicitly records the remaining React gap for file/tag/MCP/skill mention loaders.
- React chat mention contracts expanded in the current slice: `ChatMentionView`, stream payloads, and steer payloads now preserve Vue resource types (`kb`, `file`, `tag`, `mcp`, `skill`) and resource identifiers. The host loads recent documents, MCP services, and skills alongside KBs with partial-failure tolerance; existing mocks remain compatible. Web tests pass 892/892, typecheck and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r046-chat-resource-mentions.md`. Non-KB browser fixtures and tag discovery remain open.
- Chat mention discovery now includes per-KB tag loading in `client.knowledge.documents.tags` with bounded page size, partial-failure tolerance, and post-await scope-generation guards. Tag items carry KB context and are serialized through the existing five-type mention contract. Web tests remain 892/892; typecheck and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r047-chat-tag-mentions.md`.
- Chat mention aggregate failure handling was tightened after the `allSettled` expansion: total primary-resource failure now surfaces localized error copy and releases the load guard for retry, while partial success remains usable. Web tests 892/892, typecheck, and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r048-chat-mention-error-state.md`.
- Cross-package regression after the five-type chat mention expansion passed: shared 465/465, mobile 189/189, embed 7/7, desktop 2/2. Evidence: `evidence/vue-react-parity/2026-09-15-r049-cross-package-regression.md`. Native/browser acceptance remains separate.
- Chat mention UI now distinguishes the five resource types with stable `data-mention-type` hooks and compact markers in options/chips, while preserving selection, ARIA semantics, and payloads. Shared tests, Web 892/892, typecheck, and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r050-chat-mention-markers.md`.
- Mention failure detection now accounts for optional client capabilities: fallback promises used by reduced clients cannot mask a real aggregate failure. Web tests 892/892, typecheck, and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r051-chat-mention-capability-failure.md`.
- Authenticated MCP settings runtime re-verified at `/platform/settings?section=mcp`: lazy settings shell resolves from loading to localized MCP management empty state with add-service entry. No service mutation performed; synced/stale directory evidence still needs a configured MCP fixture. Evidence: `evidence/vue-react-parity/2026-09-15-r052-mcp-empty-runtime.md`.
- Added component regression coverage for the five-type mention markers and `data-mention-type` hooks. Web tests now pass 893/893; typecheck and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r053-chat-mention-marker-tests.md`.
- Protected settings close path re-verified in browser: `/platform/settings?section=mcp` closes through the localized button and returns to `/platform/knowledge-bases` with the shell/list intact. No mutation performed. Evidence: `evidence/vue-react-parity/2026-09-15-r054-settings-close-runtime.md`.
- Settings Escape close path re-verified in authenticated browser: `/platform/settings?section=mcp` returns to `/platform/knowledge-bases` and restores the protected list after the loading transition. No mutation performed. Evidence: `evidence/vue-react-parity/2026-09-15-r055-settings-escape-runtime.md`.
- Settings deep-link history behavior re-verified in browser: navigating from `/platform/knowledge-bases` to `/platform/settings?section=mcp` and pressing Back returns to the list with shell state intact. No mutation performed. Evidence: `evidence/vue-react-parity/2026-09-15-r056-settings-popstate-runtime.md`.
- Captured authenticated React chat mention computed-style baseline: trigger 28x28/6px radius and listbox 280px/1px border/8px radius/0 8px 24px shadow. Evidence: `evidence/vue-react-parity/2026-09-15-r057-chat-mention-computed-style.md`; Vue-side computed-style comparison remains open.
- Vue `/platform/creatChat` was checked on the authenticated parity tenant; its current fixture does not mount the mention control (only the disabled send control is exposed), so cross-end mention computed-style comparison is gated on a tenant with mention capability enabled. Evidence: `evidence/vue-react-parity/2026-09-15-r058-vue-mention-fixture-gate.md`.
- Repository regression sweep completed after the latest slices: shared 465/465, Web 893/893, mobile 189/189, embed 7/7, desktop 2/2; all failures 0. Evidence: `evidence/vue-react-parity/2026-09-15-r059-repository-regression.md`.
- Chat model selection is now wired end-to-end in the React composer: loaded model options render as an accessible selector, the selected id is preserved in `ChatSubmission`, and Web knowledge/agent stream bodies include `summary_model_id` when selected. Web tests pass 894/894, typecheck and build pass. Evidence: `evidence/vue-react-parity/2026-09-15-r060-chat-model-selection.md`. Multi-model browser interaction remains gated on a tenant fixture with at least two enabled models.
- Chat model option sanitization now trims and filters incomplete backend rows, while the model chip falls back to localized unconfigured copy for unnamed records. Web tests pass 895/895, typecheck/build and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r061-chat-model-option-sanitization.md`. Multi-model browser interaction remains gated on a tenant fixture with at least two valid enabled models.
- Steer mention chips and options now share the main composer’s five resource markers and stable `data-mention-type` hooks. Focused shared chat tests pass 63/63, Web tests 895/895, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r062-steer-mention-markers.md`. Live streaming and non-KB resource fixtures remain unavailable.
- Chat model selection now persists per origin/user/tenant and restores only valid catalog ids, matching the Vue last-pick behavior while preventing cross-tenant leakage. Web tests pass 895/895, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r063-chat-model-persistence.md`. Live multi-model persistence evidence remains gated by the available tenant fixture.
- Browser recheck at `/platform/creatChat` confirms malformed model metadata no longer creates a blank model option; the visible chip resolves to `mock-stream-model200K`. Evidence: `evidence/vue-react-parity/2026-09-15-r064-chat-model-runtime-sanitization.md`. A valid multi-model fixture is still required for selection interaction evidence.
- MCP tool detail popup dismissal now restores focus to its trigger for Escape and outside-pointer close, with a non-browser scheduling fallback. MCP tests pass 16/16, Web typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r065-mcp-detail-focus-return.md`.
- Web chat resource mentions now emit Vue-compatible `knowledge_ids`, `tag_ids`, `mcp_service_ids`, and `skill_names` alongside `mentioned_items`, with MCP/Skill fields gated to Agent mode. Agent selection tests pass 12/12, Web tests 895/895, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r066-chat-resource-compat-payload.md`.
- Mobile knowledge graph node navigation no longer issues a duplicate graph request after updating the ego center; the effect responds only to actual graph filters and mode changes. Mobile tests pass 189/189, typecheck passes, and diff check passes. Evidence: `evidence/vue-react-parity/2026-09-15-r067-mobile-graph-request-dedup.md`. Successful native graph payload evidence remains fixture-gated.
- Mobile graph type filters and depth controls now expose localized labels, button roles, and selected state for accessibility while preserving their Vue-aligned visual states. Mobile tests pass 189/189 and typecheck passes. Evidence: `evidence/vue-react-parity/2026-09-15-r068-mobile-graph-accessibility.md`.
- Mobile graph loading now ignores stale success/error responses from superseded filter or node requests; only the newest request controls graph, error, and loading state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r069-mobile-graph-race-guard.md`.
- Mobile document folder and tag filters now expose selected state and accessible labels through native button semantics. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r070-mobile-doc-filter-accessibility.md`.
- Mobile KB list refreshes now use a generation guard so older responses cannot overwrite newer list, error, or loading state during resume/upload/create-triggered reloads. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r071-mobile-kb-list-race-guard.md`.
- Mobile document folder/tag auxiliary loading now ignores superseded responses and errors, preventing stale filters from overwriting current state during refresh or resume. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r072-mobile-doc-auxiliary-race-guard.md`.
- Mobile data-source inventory, resource, and child-resource loading now use generation guards so superseded responses cannot overwrite current rows, errors, or loading state during refresh and source switching. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r073-mobile-datasource-race-guard.md`.
- Cross-package regression after mobile document/data-source request guards passed: shared 465/465, Web 895/895, mobile 189/189, embed 7/7, desktop 2/2; mobile/shared typechecks and diff check passed. Evidence: `evidence/vue-react-parity/2026-09-15-r074-cross-package-regression.md`.
- Mobile document detail and Wiki/FAQ reference reloads now use generation guards, preventing superseded responses from replacing current rows or loading/error state during refresh and foreground resume. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r075-mobile-detail-reference-race-guard.md`.
- Mobile Wiki/FAQ editor reloads now use a generation guard, preventing late responses from overwriting the active draft, conflict state, or loading/error state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r076-mobile-editor-race-guard.md`.
- Mobile organization capability/list refreshes and organization detail selection now use generation guards, preventing stale organization/member/share responses from replacing current state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r077-mobile-organizations-race-guard.md`.
- Mobile API-key list refreshes now use a generation guard, preventing stale responses from replacing current keys or loading/error state after create, revoke, or manual refresh. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r078-mobile-apikey-race-guard.md`.
- Mobile tenant administration reloads now guard concurrent members, invitation, and audit-log responses with a generation token, preventing stale refreshes from overwriting current management state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r079-mobile-administration-race-guard.md`.
- Mobile configuration catalog and identity capability loads now use generation guards, preventing stale agent/model/MCP/skill or capability responses from overwriting newer state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r080-mobile-management-race-guards.md`.
- Mobile onboarding invitation loads now use a generation guard, preventing repeated panel opens from allowing stale invitation responses or errors to replace current state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r081-mobile-onboarding-race-guard.md`.
- Mobile chat session, knowledge-base, message-history, attachment, and steer-queue loads now use generation guards plus active-session checks, preventing stale responses from replacing the current conversation state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r082-mobile-chat-load-race-guards.md`.
- Mobile document preview downloads now use a generation guard and close invalidation, preventing stale downloads from reopening or replacing the active preview. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r083-mobile-preview-race-guard.md`.
- Mobile chat attachment uploads now use an upload generation and active-session check, preventing late uploads from previous sessions or picker actions from mutating the current attachment list. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r084-mobile-attachment-upload-race-guard.md`.
- Mobile management hub capability/edition loads now use a generation guard, preventing stale responses from replacing the current capability catalog or error/loading state. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r085-mobile-management-hub-race-guard.md`.
- Mobile configuration catalog refreshes now expose a consistent loading state while retaining generation protection, so manual refreshes do not appear idle over stale rows. Mobile tests pass 189/189, typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r086-mobile-config-refresh-state.md`.
- Release-oriented static gates rechecked after the mobile slices: Web production build passed (2,446 modules), desktop renderer typecheck passed, Embed typecheck passed; the existing Vite large-chunk advisory remains non-failing. Evidence: `evidence/vue-react-parity/2026-09-15-r087-release-build-gates.md`.
- Added explicit `@weknora/views` subpath exports and Web/Vite aliases for selected chat, guide, integration, and settings modules; updated the ChatRoutePage esbuild stub to cover subpath imports. Focused send test, Web 895/895, typecheck, build, and diff check passed. Evidence: `evidence/vue-react-parity/2026-09-15-r088-views-subpath-exports.md`. Eager index bundle remains approximately 2.74 MB; further reduction requires additional import-by-import migration. IntegrationsRoutePage and AgentsPage were added to this direct-import slice.
- Remaining Web/Embed runtime consumers in IntegrationsRoutePage, AgentsPage, EmbedApp, and settings surface helpers now use explicit `@weknora/views` subpaths with synchronized package exports, TypeScript paths, and Vite aliases. Web 895/895, Embed 7/7, typechecks, production build, and diff check passed. Evidence: `evidence/vue-react-parity/2026-09-15-r089-views-subpath-runtime-migration.md`.
- Runtime barrel audit now finds no remaining value imports from aggregate `@weknora/views` in Web/Embed/Desktop; remaining matches are test-only or type-only. Web 895/895, Embed 7/7, typechecks, build, and diff check remain green. Evidence: `evidence/vue-react-parity/2026-09-15-r090-views-runtime-barrel-audit.md`.
- Web entry now lazy-loads PlatformShell, LoginPage, JoinPage, and WorkspaceOnboardingPage through a shared auth Suspense boundary. Web tests 895/895, typecheck/build and diff check passed; eager index reduced from about 2,735.9 kB to 2,531.25 kB (gzip 698.2 kB to 641.23 kB). Evidence: `evidence/vue-react-parity/2026-09-15-r091-auth-shell-lazy-chunks.md`.
- Added lightweight `@weknora/i18n/runtime` for bootstrap locale validation/loading copy and moved Web entry off the full locale catalog. Web 895/895, typecheck/build, and diff check passed; eager index remained about 2,531.5 kB (gzip 641.4 kB), confirming i18n was not the dominant remaining source. Evidence: `evidence/vue-react-parity/2026-09-15-r092-i18n-runtime-boundary.md`.
- Inspected the post-lazy-load eager chunk for heavyweight runtime markers: Mermaid remains dynamically imported and no views barrel value import remains. The index is about 2,531.5 kB (gzip 641.4 kB), so further reduction requires route-boundary/shared dependency graph work. Evidence: `evidence/vue-react-parity/2026-09-15-r093-eager-chunk-source-audit.md`.
- Added contract tests for the lightweight i18n bootstrap runtime: five-locale completeness, invalid-locale rejection, and localized loading copy. Shared tests pass 467/467, Web typecheck and diff check pass. Evidence: `evidence/vue-react-parity/2026-09-15-r094-i18n-runtime-contract.md`.
- Browser gate reached both local services (Vue :5173 and React :5181) and verified `/platform/knowledge-bases` with the parity fixture. Both render the expected navigation, filters, title, initialization notice, “我创建的” group, and `Parity KB Demo`; React exposes richer ARIA roles while Vue still presents several icon/container controls without equivalent roles. Evidence: `evidence/vue-react-parity/2026-09-15-r095-browser-knowledge-base-route.md`. This is browser fixture evidence only; backend/native acceptance remains open.
- Browser gate also verified `/platform/settings?section=mcp` on Vue :5173 and React :5181. Both expose the settings drawer and MCP management surface; React has one settings tree with button semantics, while Vue currently exposes duplicated settings trees and mostly container/icon semantics. Evidence: `evidence/vue-react-parity/2026-09-15-r096-browser-settings-mcp-route.md`; treat the duplicate Vue tree as an open parity finding.
- 修复 Vue 平台层重复挂载 Settings：`/platform/settings` 子路由不再同时渲染全局 Settings。浏览器复验 `.settings-overlay/.settings-modal` 均为 1，MCP 深链接和设置导航仍可见；Vue `pnpm type-check` 通过。证据：`evidence/vue-react-parity/2026-09-15-r097-browser-settings-duplicate-fix.md`。
- 浏览器复验 `/platform/creatChat` 双端首屏：导航、空会话列表、用户区、欢迎语和输入区均可达。React 暴露侧栏/导航/批量管理/输入/智能体/附件/知识库/发送等 ARIA 语义；Vue 多数仍为容器/图标语义。两端模型状态分别为 `mock-stream-model 200K` 与“未配置”，记录为夹具配置差异。证据：`evidence/vue-react-parity/2026-09-15-r098-browser-platform-chat-route.md`。
- 完成知识库列表双端 computed-style 采样：标题字号/行高一致但字体族和纵向起点不同；React 卡片渐变、边框、圆角和阴影已在浏览器中生效；Vue 卡片节点结构不同，不能直接比较同一节点盒模型；React `main` 的 `max-w-[960px]` computed 为 `none`，列为待确认宽度样式。证据：`evidence/vue-react-parity/2026-09-15-r099-browser-knowledge-computed-style.md`。
- Added a regression contract test for the Vue platform shell's single Settings mount (`frontend/src/views/platform/settingsMount.test.ts`); targeted `tsx --test` passes 1/1. This protects the browser-verified duplicate-mount fix.
- iOS 原生门禁取得新证据：iPhone 17 Pro 模拟器上 `xcodebuild` 成功，`com.weknora.mobile` 安装并由 `simctl launch` 启动成功；启动页为 Expo Development Build，但提示没有 development server，尚未进入认证业务页面。证据：`evidence/vue-react-parity/2026-09-15-r100-ios-native-build-launch.md`，截图 `/tmp/weknora-react-multiclient-ios-launch.png`。因此仅关闭 native compile/install/launch 层，业务交互仍开放。
- iOS Metro 连接尝试取得更强边界证据：Expo Router 在宿主机成功打包 1376 modules，但模拟器连接 8081/8082/8083 被拒绝，当前 worktree 仍停在 Development Build 选择页，模拟器到 8081/8082/8083 连接被拒绝；同一模拟器中另一个 `happy-mobile-verify` worktree 的 `Unistyles` 日志不归因于本分支。证据：`evidence/vue-react-parity/2026-09-15-r101-ios-metro-connectivity.md`。因此 native compile/install/launch 已关闭，业务运行时连接仍开放。
- iOS Release 业务启动门禁通过：Release Simulator build succeeded，安装启动后直接进入 WeKnora 登录页（品牌、Email/Password、SSO、注册、邀请、切换服务器入口均可见），无需 Metro。证据：`evidence/vue-react-parity/2026-09-15-r102-ios-release-business-launch.md`，截图 `/tmp/weknora-react-multiclient-ios-release.png`。认证后业务交互和 Android 仍开放。
- 跨包回归门禁通过：Shared 467/467、Web 895/895、Mobile 189/189、Embed 7/7、Desktop 2/2，所有进程退出码 0。证据：`evidence/vue-react-parity/2026-09-15-r103-cross-package-regression.md`。
- Android 原生 Debug 构建通过：使用本机 Android SDK 执行 `assembleDebug`，Gradle `BUILD SUCCESSFUL`，生成 `apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk`。当前无 `adb` 可用设备，未做安装/启动/认证交互。证据：`evidence/vue-react-parity/2026-09-15-r104-android-native-build.md`。
- Android Release 业务启动门禁通过：`assembleRelease` 在 Android SDK 36 环境成功完成 JS bundle/native targets/APK 打包；`test36-small` 模拟器安装并启动 `com.weknora.mobile/.MainActivity` 成功，直接渲染 WeKnora 知识库首屏。截图 `/tmp/weknora-react-multiclient-android-release.png`，证据 `evidence/vue-react-parity/2026-09-15-r105-android-release-business-launch.md`。模拟器未连接后端，认证后业务链路与错误恢复仍开放。
- Android 认证后知识库列表门禁通过：Release APK 在 `test36-small` 上设置 `http://10.0.2.2:8080` 后使用 `parity-test@local.dev` 登录成功，进入 `knowledge/index` 并加载真实 `Parity KB Demo` 数据（文档 0 项）；截图 `/tmp/android-authenticated4.png`，证据 `evidence/vue-react-parity/2026-09-15-r106-android-authenticated-kb.md`。详情、上传、编辑、Wiki/FAQ、图谱、聊天和写操作仍开放。
- Android 知识库壳层缺陷已修复并复验：关闭 Expo Router 默认 pathname header，避免 `knowledge/index`/`knowledge/[id]` 泄漏；文档列表计数传入 `total` 插值，设备页由 `{count} 项` 修正为 `0 项`。移动端定向测试 5/5、typecheck、Release 增量构建和 `test36-small` 设备复验通过。证据 `evidence/vue-react-parity/2026-09-15-r107-android-kb-shell-fix.md`。
- Android Wiki 禁用能力错误已本地化：识别后端 `feature is not enabled` 错误并按 Wiki/FAQ 输出五语言文案；`test36-small` 真实后端复验显示“此知识库未启用 Wiki 功能”，不再泄漏原始英文错误串。移动端全量测试 190/190、typecheck、Release 增量构建通过。证据 `evidence/vue-react-parity/2026-09-15-r108-android-disabled-feature-localization.md`。
- Android 图谱禁用能力错误已本地化：识别后端 `feature is not enabled` 并显示“此知识库未启用知识图谱功能”，保留重试操作；`test36-small` 真实后端复验通过。移动端 190/190、typecheck、Release 增量构建通过。证据 `evidence/vue-react-parity/2026-09-15-r109-android-graph-disabled-localization.md`。
- Android 认证后聊天空态门禁通过：`test36-small` 从真实知识库列表进入“新对话”，ChatScreen 渲染标题、导航、空会话、知识库 chip、总结/相关文件、输入和发送控件；后端无可用聊天模型，发送保持禁用，未伪造请求。证据 `evidence/vue-react-parity/2026-09-15-r110-android-authenticated-chat-empty.md`。
- 本轮移动端与共享层回归通过：Shared 467/467、Mobile 190/190、Mobile typecheck 全部退出码 0，覆盖知识库壳层、Wiki/图谱禁用错误本地化和五语言消息注册。证据 `evidence/vue-react-parity/2026-09-15-r111-shared-mobile-regression.md`。
- Android 聊天发送失败态门禁通过：`test36-small` 真实认证会话发送 `hello` 后，用户消息落地并渲染 assistant 失败结果，输入区恢复可发送状态，补充队列与重试控件可见。证据 `evidence/vue-react-parity/2026-09-15-r112-android-chat-send-failure.md`。成功模型流、停止、审批、附件和引用仍开放。
- Android 知识库创建写路径通过：真实认证态空表单校验阻止空名称；提交测试知识库成功并刷新列表显示卡片与 0 文档计数，随后删除临时数据保持租户整洁。证据 `evidence/vue-react-parity/2026-09-15-r113-android-kb-create.md`。
