# R033 — Skill settings install timeline (SSE), progress, transcript preview, drawer resize

Date: 2026-09-13 · Branch: codex/react-multiclient · Slice: React SkillSettingsPanel parity for
Vue frontend/src/views/settings/SkillSettings.vue + frontend/src/components/Skill*.vue,
closing the remaining R033 matrix gaps (install timeline, upload progress, markdown preview,
drawer widths/card CSS).

## Backend route facts (cited from internal/ source — none invented)

All skill routes are Admin-gated (internal/router/routes_infra.go:70-77):

- GET  /api/v1/sandbox-configs/:id/skills/:skillId/install-events → skills.InstallEvents (routes_infra.go:76)
- GET  .../skills/:skillId/transcript → skills.InstallTranscript (routes_infra.go:77)
- GET  .../skills/:skillId/guidance → skills.InstallGuidance (routes_infra.go:70)
- POST .../skills/:skillId/guidance → skills.SteerInstall (routes_infra.go:71)

SSE protocol (internal/handler/sandbox_skill.go):

- install-events frame = gin SSE event `message` with JSON `{percent:int, stage:string, log?, status?, done:bool}`
  (skillInstallEvent, sandbox_skill.go:554-563); keep-alive comment `: keep-alive` (sandbox_skill.go:886-892).
  Terminal stages `done`/`failed`, handler-only `detached` — all carried with `done:true` so clients terminate
  on the flag (sandbox_skill.go:565-586). The stream ALWAYS terminates: it replays LastProgress first
  (sandbox_skill.go:657-664), synthesizes a terminal frame from the durable status (sandbox_skill.go:592-612,
  714-717), publishes a single status frame when Redis is absent (sandbox_skill.go:669-679), and emits
  `detached` at the follow deadline (sandbox_skill.go:723-731).
- transcript frames are chat-shaped StreamResponse records `{id, response_type, content, done, session_id,
  assistant_message_id, data}` (emitTranscript, sandbox_skill.go:839-863); the stream ends after a
  `complete` frame. 204 while the run has no locators yet; 404 once the event log expired — the durable
  message history is the fallback (sandbox_skill.go:771-781, 794-799).
- guidance GET returns `{success, data:{accepting, messages:[{id,content,status}]}}`
  (sandbox_skill.go:903-916); steer POST binds `{expected_message_id, steer_id, content(max=10000)}` → 202
  `{success:true}` (sandbox_skill.go:918-937).

## Vue references

- SkillInstallTimeline.vue — the whole component: transcript SSE follow with reconnect/abort semantics
  (open()/follow()/stop(), lines 152-330), durable-history fallback via getMessageList (182-190), 1s re-open
  loop while live (302-313), install_prompt as the single user turn (176-180), guidance poll every 1.5s while
  live (85-96, 134-144), steer with pendingSend dedupe so Retry cannot enqueue twice (98-132), reinstall with
  guidance on the canRetry path (119-123), unavailable/failed copy (35-44), compact styling (333-456).
- SandboxSkillsPanel.vue — progressOf percent fallbacks (1017-1024), progressStageText (1026-1042),
  forgetProgress on retry/stop/remove (1051-1060), hasTranscript (846-849), followBusySkills/followProgress
  install-events wiring with done → loadSkills + refreshImage (1089-1174, 1391-1408, 1429-1458), focus drawer
  transcript section with progress ring (236-261), remove section replacing content while removing (118-135),
  transcriptEpoch bump on popup open (871-885).
- SkillSettings.vue — drawer specs: add 680px min 560 max 920 key `setting-drawer:width:skill-catalog-add`
  (132-134), install 560/480/760 key `...-install` (267-269), manage 680/560/920 key `...-manage`
  (316-318); SettingDrawer.vue drag-resize + persisted width (150-248, 499-540).
- SkillFilesPanel.vue — markdown rendered as HTML in preview mode with frontmatter table and source toggle
  (44-93, 284, 476-478, 808-810).

## Changes

New files:

- packages/domain/src/sandbox/skill-install.ts (+ .test.ts) — pure progress-event parsing
  (parseSkillInstallProgressFrame mirrors the backend frame), terminal flag (done covers done/failed/detached),
  installProgressPercent = Vue progressOf, and reduceSkillTimelineFrame: a pure reducer folding transcript
  frames (install_prompt/thinking/answer/tool_call/tool_result/complete/error, flat chat fields tolerated)
  into the timeline view state. Export registered as `@weknora/domain/sandbox/skill-install` in
  packages/domain/package.json.
- packages/api-client/src/sandbox/skill-install.ts (+ .test.ts) — createSandboxSkillInstallApi({request,
  sendStream}): followInstallEvents / followTranscript (SSE via the shared createServerSentEventParser and the
  client's sendStream transport, buffered request fallback, auth/tenant/refresh handled by the existing
  transport chain — no competing request layer), guidance GET + steer POST with snake_case payloads and
  content ≤ 10000, path builders, parse helpers. Exports registered in packages/api-client/src/index.ts
  (sandbox module was already exported there).
- apps/web/src/settings/skill-settings.css — Vue-ported styles: timeline + guidance composer
  (SkillInstallTimeline.vue:333-456), manage sections + progress ring (SandboxSkillsPanel.vue:1695-1789),
  catalog cards/chips/install-panel/pick rows/upload area (SkillSettings.vue:1314-2060), drawer resize handle
  (SettingDrawer.vue:499-540). TDesign vars mapped to the React app palette; geometry kept.

Modified files:

- packages/api-client/src/client.ts — wires createSandboxSkillInstallApi into the client as
  `client.sandbox.skills` (alongside issueTicket), reusing options.transport.sendStream. Minimal additive
  change to a shared file — flagged for the integrating agent; packages/api-client/src/client.test.ts still
  passes 16/16.
- apps/web/src/settings/SkillSettingsPanel.tsx — SkillInstallTimeline component (transcript SSE + durable
  fallback + guidance/steer/retry composer + terminal states); ManageSkillDialog: transcript section
  (skillTranscriptTitle + ProgressRing with live %), install-events SSE follow per busy skill with
  terminal → silent reload + onChanged, forgetProgress equivalents on retry/stop/uninstall, transcriptEpoch
  bump per open, remove section now mirrors Vue (ring + skillRemoveStage text); DrawerShell drag-resize with
  Vue's clamps + localStorage keys on add/install/manage dialogs; catalog files preview renders markdown HTML
  via @weknora/views renderChatMarkdown (sanitized; raw HTML escaped, links allow-listed) with the existing
  source toggle.
- apps/web/src/settings/SkillSettingsPanel.test.tsx — 4 new jsdom interaction tests (harness follows
  McpSettingsPanel.test.tsx): timeline + progress mount, steer payload/dedupe id/append + composer clear,
  failed-run durable replay + reinstall-with-guidance, markdown preview + frontmatter.

Note: the worktree's pnpm `file:` snapshot for @weknora/domain predates the new module; the snapshot under
node_modules was refreshed in place (two copied files + one exports entry) so tests/tsc resolve. A
`pnpm install` at integration reproduces this from the real package directory.

## Test evidence (node:test; TDD)

Red → Green:

- packages/domain/src/sandbox/skill-install.test.ts — RED (module missing, 1 failing file) → GREEN 7/7.
- packages/api-client/src/sandbox/skill-install.test.ts — RED (module missing) → GREEN 7/7. Assertions cover
  URL/path encoding, GET+accept headers, SSE frame parsing incl. comments, terminal flag, 204-vs-404
  transcript semantics, and the steer wire payload captured at the request-layer boundary (ClientRequest body
  is snake_case `{expected_message_id, steer_id, content}`).
- apps/web/src/settings/SkillSettingsPanel.test.tsx — 23 pre-existing tests stayed green; 4 new tests RED
  (timeline/section/composer absent) → GREEN. Final: 27/27.
- Regression: packages/api-client/src/client.test.ts 16/16 after the client.ts wiring.

Focused commands (all green): tsx --test packages/api-client/src/sandbox/skill-install.test.ts
packages/domain/src/sandbox/skill-install.test.ts; pnpm --filter @weknora/web exec tsx --test
src/settings/SkillSettingsPanel.test.tsx.

Typecheck: pnpm run typecheck:shared — no errors from this slice's files (current failures are another
agent's WIP in packages/views/src/guides/* and chat/message-list.tsx); tsc over the two new package files in
isolation exits 0; pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit reports no errors from this
slice (the only error is apps/web/src/agents/list.ts, another agent's file).

## i18n

No new keys needed and packages/i18n untouched. Used existing shared keys (verified present in all 5
locales): settings.sandbox.skillGuidance.{placeholder,send,retry,pending,injected,unprocessed,unavailable,
failed}, skillTranscript{Title,Waiting,Empty}, skillStatusInstalling, skillRemoveStage.*, skillRemoveWaiting,
skillRemoveInProgress, skillRemoveDone, skillFiles*. Note for the matrix: there are NO
`settings.sandbox.skillTimeline*` keys anywhere (Vue locales or packages/i18n) — the timeline copy lives
under skillTranscript*/skillGuidance*, which this port uses.

## Remaining gaps (open)

1. Browser/native parity evidence (live SSE against backend :8080, drawer drag interaction, visual diff of
   card CSS) is explicitly NOT captured this round — listed as open for the browser-evidence pass. jsdom
   tests use mocked SSE sources at the client-method boundary.
2. Upload progress% — NOT ported. Vue has it (SkillSettings.vue:208 + 993-997 percent from
   registerSkillCatalogFromFile; SandboxSkillsPanel.vue:64-72 + 1324-1326 uploadConfigSkill onProgress; copy
   key settings.sandbox.skillUploading exists in packages/i18n). The React transport has no upload-progress
   surface: packages/api-client/src/ports.ts HttpRequest/NativeMultipartFileRequest carry no onProgress and
   transport/json.ts + apps/web/src/platform/http.ts are fetch-based (XHR would be needed). Those files are
   outside this slice's ownership — needs a transport-level change (sendMultipartFile onProgress via XHR)
   plus rendering skillUploading + a progress bar in AddSkillWizard register.
3. Catalog-level pick-row percentages (sandboxPickPercent circles in Vue add/install drawers,
   SkillSettings.vue:244-246, 295-297) — install-events SSE is wired at the manage-drawer level only;
   fanning it into the pick lists is follow-up.
4. Transcript answer/thinking rendering is a compact port (prompt block, thinking block, tool-call steps with
   status dots, sanitized markdown answer) — not a full AgentStreamDisplay tree port; hljs highlighting for
   non-markdown file previews (SkillFilesPanel.vue:93) is likewise not ported.
5. files browser: Vue keeps markdown source/preview toggle state per file and enhances code blocks; React
   resets to preview per selection (pre-existing behaviour, kept).
