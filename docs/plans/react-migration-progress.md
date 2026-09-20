# React 多端迁移进度账本

> 账本只记录本仓库实际执行的证据。`accepted` 必须同时有实现、相关测试和与任务类型匹配的运行证据；静态检查或 mock 不得冒充真实后端、原生或安装包验收。

## 执行上下文

- 目标仓库：`/Users/wuyongjun/trea/WeKnora-fork01`
- 参考仓库（只读）：`/Users/wuyongjun/trea/multica`
- 方案：B（React + Vite Web、Wails + React 桌面、Expo + React Native 移动）
- 目标基线：`main@5db13a1`（2026-09-10）
- 参考基线：`main@85b1fdbb4`（2026-09-10；工作区干净，领先远端 1 个提交）
- 基线脏文件：权威规格/计划/库存为用户提供的未提交文件；不得覆盖或清理。
- 初始静态事实：200 个 Vue SFC、33 个 `frontend/src/api` TS 文件、Swagger 2.0/282 paths/361 operations；Wails `cmd/desktop/wails.json` 的 `frontend:dir` 为 `../../frontend`。
- 适用规范：目标仓库根目录未发现 `AGENTS.md`/`CLAUDE.md`；`cli/AGENTS.md` 仅适用于 CLI 子模块。

## 状态定义

`pending` 未开始；`implementing` 正在实现/取证；`review` 已有实现但等待规格与质量评审；`accepted` 证据满足任务验收；`blocked` 有明确外部/授权/环境阻塞并记录下一步。

## 移动方向增补（2026-09-10）

详细执行入口：[Happy 实施总计划](2026-09-10-happy-agent-mobile.md)，H01–H35细分T20–T23；管理域按子任务单独评审。当前只有计划，原任务状态不变。

用户已确认以 Happy 保留现有移动交互，承载知识问答、通用 Agent 与专业 Agent；设计见 [Happy 统一 Agent 移动客户端](../specs/2026-09-10-happy-agent-mobile-design.md)。本次仅完成静态盘点与文档同步，T20–T23 继续 pending，没有运行或原生验收。任务范围与依赖以实施计划对应增补为准；Happy 服务依赖缺口必须逐项保留，旧移动工作量估算需重估。

> 注：T20/T24 表格行保留了本次 follow-up 前的汇总措辞；Android logout/layout follow-up 的最新结论与证据见下方 2026-09-11 追加记录，logout 已通过，但 T20/T24 仍因其他开放门槛保持 `review`。
> 注：T21 表格行也保留了 Android completed-fixture follow-up 前的汇总措辞；最新 Android 上传完成、检索、下载和 Sharesheet 证据见下方 2026-09-12 追加记录，T21 仍因负例、真实设备和生产存储门槛保持 `review`。

## 任务状态

| 任务 | 状态 | 实现文件/提交 | 测试与退出码 | 证据层级 | 问题/下一步 |
|---|---|---|---|---|---|
| T01 基线、契约和复用来源冻结 | review | `eb0e9a9` + working-tree T01 audit | `python3 -m unittest scripts/test_generate_react_migration_baseline.py -v` 0（4/4）；`python3 scripts/generate_react_migration_baseline.py` 0；`go test ./internal/router -count=1` 0；OpenAPI Generator strict validation exit 1 / skip-validation generation exit 0；`git diff --check` 0 | 静态矩阵/确定性/注册路由/RBAC 自检：通过；Swagger 282 paths/361 operations 与注册 Gin 路由对照；Generator 7.14.0/typescript-fetch compatibility trial recorded；真实后端/截图/Wails 包/原生：未完成 | API 矩阵 452 行（361 Swagger + 91 implementation-only）；Swagger strict validation still has 9 errors/1 warning，generated client intentionally not adopted；76 Swagger-only、91 implementation-only 行的 handler DTO/权限/响应和真实 smoke 仍待补。证据：`docs/migrations/react/evidence/t01-openapi-generator-2026-09-11.md`、`docs/migrations/react/evidence/t01-router-rbac-audit-2026-09-11.md` |
| T02 无框架 SDK与第一条真实 API 链路 | review | `e3a3a8f` (`feat: add framework-free shared client foundation`) | `pnpm test:shared` 0（10/10）；`pnpm typecheck:shared` 0；TDD nested-error/late-abort RED→GREEN；2026-09-12 隔离 Lite + Chrome Web 注册/登录/auth.me/KB list 通过 | 静态/Node mock：通过；隔离 Lite 真实后端 Web 登录与 KB list：通过；完整 API 链路仍未完成 | 已实现 contracts/api-client/domain 与 KB list mock 链路；真实浏览器已覆盖注册、登录、auth.me 和 KB list。仍需补齐完整 API/权限矩阵与可复现的端到端契约证据。证据：`docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md` |
| T03 登录、刷新、OIDC与凭证隔离 | review | `118d15d` + `2f8697a` + `6118321` + `8fdbac0` + `f0babf5` + `9e5f970` + `f3a4f38` | `pnpm test:shared` 0（128/128）；`pnpm test:web` 0（64/64）；`pnpm typecheck:web` 0；`pnpm build:web` 0；boundary check 0；`git diff --check` 0；2026-09-12 隔离 Lite Web 注册/登录/auth.me 通过；真实 OIDC/刷新仍未完成 | 静态/Node mock 与 Web 生产构建：通过；一次性旧会话导入、损坏回退与读取异常降级：通过；隔离 Lite 浏览器注册/登录/auth.me：通过；OIDC provider、刷新/浏览器邀请回调：未完成 | 新增版本化 React session、一次性 `weknora_*` 凭证/已知偏好导入、durable fallback 与完成标记；旧键保留给 Vue 回退 artifact；Embed 仍不会触发主账号 refresh。真实刷新/OIDC/浏览器回调、写入异常与跨版本升级仍待补。证据：`docs/migrations/react/evidence/t03-legacy-browser-import-2026-09-11.md`、`docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md` |
| T04 空间上下文、路由和能力守卫 | review | `9b35b5a` + `f5517e7` + `55fca3f` (`feat: wire React scoped session gate`, `fix: persist active React tenant scope`) | `pnpm test:shared` 0（132/132）；`pnpm typecheck:shared` 0；`pnpm test:web` 0（33/33）；`pnpm typecheck:web` 0；`pnpm build:web` 0；boundary/diff checks 0 | Static/Node mock and React production bundle: through current patch; real deep-link browser, backend role/capability negatives and native routing: not completed | Added auth.me identity/capability hydrate, dynamic X-Tenant-ID, switchTenant seam with tenant persistence callback, no-tenant onboarding, protected-route guard, `/join?code` organization redirect, `/register?token` invite compatibility, logout invalidation and strict auth.me IDs; retain `review` until live evidence |
| T05 UI基础、平台注入和国际化 | review | `a230366` + `ac955da` + `a784f2b` + `544e739` (`feat: complete React UI platform ports`) | `pnpm test:shared` 0（135/135）；`pnpm test:web` 0（34/34）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；boundary/diff checks 0 | React/Vite source, Node tests, and production bundle: passed; live backend, browser keyboard/focus, native platform, and full legacy-catalog migration: not completed | Added semantic design tokens, five-locale foundation key/placeholder parity, loading/disabled/status UI states, DOM-only Dialog with Escape/focus restore behavior, and replaceable Web navigation/storage/file/clipboard ports; actual source inventory has five locales, so no sixth locale was invented |
| T06 知识库列表与创建编辑闭环 | review | `a230366` + `ac955da` + `de1ce63` + `292fdc4` + `f441475` (`feat: add React knowledge base list filters`) | `pnpm test:shared` 0（138/138）；`pnpm test:web` 0（35/35）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；boundary/diff checks 0；2026-09-12 两次隔离 Lite Web create 201/update 200/list reload 通过 | Static/Node mock and production bundle: passed; isolated Lite real browser list/create/edit: passed twice, with independent authenticated list 200; permission/detail/document and full browser acceptance: not completed | Added pure query/type/creator/favorite filtering with clamped client pagination, server-compatible creator query forwarding, FAQ/document mutation selector, empty/filter states, and source/permission metadata display. Live browser now covers server-confirmed create/edit; full detail navigation, document processing, permission negatives and server pagination remain. Evidence: `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md`, `docs/migrations/react/evidence/t06-web-crud-rerun-2026-09-12.md` |
| T07 文档上传、列表、目录标签与预览 | review | `4ecd4c1` + `dcff3fa` + `29c6d5d` + `369ddb8` + `f701dee` (`feat: add React document migration slice`, `fix: keep unknown document statuses unavailable`, `fix: satisfy shared React boundary guard`, `feat(web): add authenticated document preview transport`, `fix(web): wire authenticated binary transport`) | `pnpm test:shared` 0（184/184）；`pnpm test:web` 0（97/97）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0；2026-09-12 isolated Lite + React Web document route showed authenticated KB navigation, embedding-backed multipart upload/readback, `parse_status=completed`, and rendered `Root · txt Completed`; protected preview API/browser success state and browser download bytes also passed | Contracts/API/domain focused tests, Web route tests, boundary gate and production bundle: passed; isolated Lite document parse, authenticated 401/200 preview bytes, React Web completed preview renderer/download action, and Chrome downloaded file with matching SHA-256/filename: passed; native chooser submission, progress telemetry, search→reference/download, and full preview renderer acceptance: not completed | Added strict document mutation seams for file/URL/manual sources, reparse/cancel/delete/batch-delete, folder move/rename, batch tag updates and authenticated binary preview/download; React list/detail routes expose folder/status/tag filters, selection-preserving bulk actions, upload cancel/413 feedback, and authoritative processing-state display. Unknown backend status values remain unavailable instead of being treated as complete. Live native browser upload, search→reference/download, and full preview renderer inventory remain. Evidence: `docs/migrations/react/evidence/t07-web-upload-live-2026-09-12.md`, `docs/migrations/react/evidence/t07-web-authenticated-preview-2026-09-12.md`, `docs/migrations/react/evidence/t07-web-authenticated-preview-transport-2026-09-12.md`, `docs/migrations/react/evidence/t07-web-preview-live-2026-09-12.md` |
| T08 FAQ与Wiki编辑/版本 | review | `736eca4` + `3d8daa6` + `1d08229` + `e1070b1` + `6d1e668` | `pnpm test:shared` 0（146/146）；`pnpm test:web` 0（42/42）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0 | React FAQ/Wiki source and API seams/build: passed; live backend and browser conflict/import acceptance: not completed | Added `/knowledgeBase/:knowledgeBaseId/faq` with strict list/search/create/update, CSV/JSON import with append/replace, export, field/tag batch actions, selection and explicit empty/error states. Wiki now has revision list/detail diff and explicit confirmed revert while preserving optimistic-save conflict/reload behavior. Live backend and browser proof remain. |
| T09 知识库高级配置与数据源 | review | `f2fe50e` + `c94ee50` + `6da05c6` | `pnpm test:shared` 0（149/149）；`pnpm test:web` 0（46/46）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0 | React data-source and advanced-settings source/API seams plus production bundle: passed; live connector credentials/sync, permission negatives and browser acceptance: not completed | Added `/knowledgeBase/:knowledgeBaseId/settings` advanced sections for general fields, parser registry overrides, explicit chunking controls with real `/chunker/preview`, indexing strategy, read-only model/storage/vector bindings, feature flags and KB activity; data sources remain a separate tab. The current backend update contract is respected: immutable bindings are displayed read-only and question generation is not presented as a fake writable field. Live connector credentials/sync, advanced endpoint permissions, share mutation UX and browser proof remain. |
| T10 聊天协议、状态机与恢复底座 | review | `cfb7d29` + `f2c2fd3` + `360ab98` + `c63529c` + `53ee20e` | `pnpm test:shared` 0（179/179）；`pnpm test:web` 0（91/91）；`pnpm typecheck:web` 0；`pnpm typecheck:shared` 0；`pnpm build:web` 0（126 modules）；`git diff --check` 0；2026-09-12 Web chat deep-link/error-state 通过；隔离 Lite + 本地流式模型的 React Web SSE 通过 | Existing backend response_type vocabulary, pure reducer, incremental transport reader, SSE framing, and Web live-response projection: passed; authenticated Web route mounted; real backend SSE emitted `agent_query`/incremental `answer`/`complete` and the reducer completed the response | Added response-type contract helper, deduplicating chat reducer, browser ReadableStream transport path, resumable `Last-Event-ID` request builder, and visible Web thinking/tool/reference/phase state. Continue-stream recovery, cancellation timeout, real third-party/RAG/Agent generation remain unverified. Evidence: `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md`, `docs/migrations/react/evidence/t10-t11-web-sse-live-2026-09-12.md`, `docs/migrations/react/evidence/t10-t13-web-live-state-2026-09-12.md` |
| T11 会话、消息和问答主界面 | review | `56393e4` + `beb7333` + `5fcbae4` + `360ab98` | `pnpm test:shared` 0（203/203）；`pnpm test:web` 0（107/107）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0（135 modules）；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0；2026-09-12 isolated Lite + local model-stub React Web SSE/session/history run passed | Strict session/history/suggestion contracts, scoped draft key, React sidebar/composer/message-list/page, older-history scroll preservation, source/search/date grouping and server-total pagination, role-gated channel source options, server-confirmed clear and mutations, suggestion polling/telemetry, concurrent-send guard, stale-session/run guards, and authoritative post-stream refresh: static/build evidence passed; prior isolated Lite browser run created a session, rendered a streamed assistant answer and read authenticated history | Added typed list/create/update/pin/unpin/delete/clear/history and suggestion APIs; first-send session creation; de-duplicated message append and pending-vs-server reconciliation; role-aware channel-session visibility and per-session async/send guards. New sidebar mutations, source/search/filter, clear/history/suggestion live provider behavior were not run in this increment. Real RAG/Agent/third-party generation, Last-Event-ID recovery, cancellation timeout, permission negatives and native chat remain. Evidence: `docs/migrations/react/evidence/t11-web-chat-main-2026-09-12.md`, `docs/migrations/react/evidence/t10-t11-web-sse-live-2026-09-12.md` |
| T12 工具审批、MCP OAuth与运行中追加 | review | `93f2e01` + `2c440f0` + `236f003` | `pnpm test:shared` 0（179/179）；`pnpm test:web` 0（91/91）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0（126 modules）；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0 | Strict approval/OAuth/steer route contracts, shared reducer, Web approval/OAuth cards, typed action wiring and session follow-up form: passed; real provider approval, OAuth callback/popup completion, steer race and live backend proof: not completed | Added typed approval and OAuth resolution APIs plus steer enqueue/list/promote/remove with encoded identifiers, status validation, and correlation fields. Web now renders pending/resolved tool approval and MCP OAuth cards; authorize starts the attempt-bound popup/status flow and cancel uses the dedicated route. A session chat entry now queues follow-up text through the steer API and preserves failed drafts. Live approval/OAuth/provider lifecycle and race handling remain. Evidence: `docs/migrations/react/evidence/t12-web-approval-actions-2026-09-12.md` |
| T13 Markdown、引用、工具结果与产物 | review | `addbc48` + `04707b3` + `4d364f7` + `4c53589` + `0dba0c3` + `c5c2fa9` + `466ed8d` + `ad735a4` + `c0d3471` + `141ba7d` + `4e85b53` + `82cac20` + `c05d289` + `d58b4a6` + `19e1c52` | `pnpm test:shared` 0（223/223）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm test:web` 0（112/112）；focused artifact-preview/artifact-name tests 10/10；`pnpm typecheck:mobile` 0；`git diff --check` 0 | Pure reference/artifact/tool contracts, safe Markdown/Mermaid hydration, protected Web artifact preview model (text/Markdown/image/PDF), Web action wiring, and cross-client safe filenames pass; browser fixture still proves sanitized Mermaid DOM; protected live artifact preview/download, broad browser XSS/performance, live citation navigation and native acceptance remain unverified | Added authenticated artifact preview callback and a bounded React preview surface. HTML/SVG/Office and unknown types remain explicit download-only; parameterized SVG MIME values are also rejected; Web/native sinks sanitize path separators/control characters while preserving the protected message artifact route and no storage path. Evidence: `docs/migrations/react/evidence/t13-artifact-preview-follow-up-2026-09-12.md`; T13 remains `review` pending live protected preview and remaining runtime/native evidence. |
| T14 沙箱终端与文件面板 | review | `7579143` | `pnpm test:shared` 0（83/83）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Strict ticket DTO, authenticated ticket POST, WS URL without JWT, terminal status/generation pure tests: passed; real WS shell/resize/reconnect/browser: not completed | Added short-lived ticket API and explicit terminal state/URL helpers. Web terminal panel, ticket refresh/reconnect, PTY input/resize, and live shell evidence remain. |
| T15 Agent、模型、MCP与Skill配置 | review | `cbe5710` + `9b253b6` + `194aee4` + `77d431a` + `b75e452` + `63ce00d` + `3b70657` + `ce24c7b` + `2102a70` + `533e86e` + `bc2b44c` + `ae47816` + `dc15b8f` + `b700141` + `6a70877` | `pnpm test:shared` 0（179/179）；`pnpm test:web` 0（91/91）；T15 focused configuration 34/34；Agent selection/error-code follow-up 4/4；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0（126 modules）；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0；2026-09-12 隔离 Lite + Chrome 配置页加载、Agent hide 操作通过；Lite SQLite migration 14/catalog persistence follow-up passed | Strict shared configuration/credential/MCP/model-debug/Skill catalog/sandbox-skill contracts, Web editors and operation panels: static/Node/build evidence passed; the actual Web chat entry now loads Agent state and sends agent-chat with a selected enabled `agent_id`; numeric lifecycle codes stay structured for model-in-use UI; isolated browser Agent render/hide and empty Model/MCP disabled-state passed; Lite SQLite skill schema/catalog persistence now passed; live writes beyond Agent hide, role negatives, real model/MCP calls, OAuth callback, sandbox install progress/removal and production/provider acceptance remain open. Evidence: `docs/migrations/react/evidence/t15-configuration-live-2026-09-12.md`, `docs/migrations/react/evidence/t15-lite-skill-catalog-migration-2026-09-12.md`, `docs/migrations/react/evidence/t15-agent-selection-error-code-2026-09-12.md` | Added dedicated credential subresources with recursive secret rejection/redaction, strict 204 and credential metadata handling, MCP transport/test/tools/OAuth controls, legacy transport fallback, model provider/debug with multipart-without-file support, Skill catalog/installed-skill APIs, Agent selection/share/hide, model debug rendering, Skill installation status/stop and safe file panel, server-confirmed remove controls, Lite skill storage/catalog migrations, and actual Web chat Agent selection wiring. Model usage/occupancy is not claimed because no such Go route exists. |
| T16 空间与组织管理、系统后台 | review | `8ec7791` + `cf34e94` + `4f68008` (`feat: add identity and administration client seams`, `feat: add React administration workspace surface`, `feat: add React organization workspace surface`) | `pnpm test:shared` 0（149/149）；`pnpm test:web` 0（50/50）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0；isolated live viewer matrix recorded | Shared strict identity/administration API, React administration/organization surfaces, production bundle and real backend viewer read/403 negatives: passed; browser access matrix, organization write/409, system-admin, Lite runtime: not completed | Added `/platform/administration` for members/invitations/audit, `/platform/system` for system-admin inventory, and `/platform/organizations` for organization list/create/detail members/join-request visibility. An isolated viewer accepted an invitation, switched to tenant 1, could read members (`200`) but received `403 Forbidden: insufficient workspace role` for invitation and memory-config writes. Organization share/API-key writes, full 403/409 matrix and capability-driven live acceptance remain; evidence: `docs/migrations/react/evidence/t16-role-matrix-live-2026-09-11.md` |
| T17 其余设置、Memory与运行配置 | review | `798543e` + `e053780` + `48b7527` + `0ec7270` + `95ee864` + working tree resource/config/Ollama/Cloud panels | `pnpm test:shared` 0（150/150）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0（61/61）；`pnpm build:web` 0；`node scripts/check-react-boundaries.mjs` 0；`git diff --check` 0；isolated Lite settings reads 10/10 HTTP 200 and config writes/checks 4/4 HTTP 200；2026-09-12 Web tenant update 200 | Strict settings client, secret-redaction, encoded-resource, raw-code-probe, section-registry, React settings inventory, tenant/profile/memory regressions, resource create/update/delete/test seams, field-level parser/retrieval/chat-history forms, Ollama status/download-progress seams, Cloud credential validation and real backend/browser tenant + memory read-write: passed; remaining settings permissions, external connection mutation, Lite capability visibility and other setting writes: not completed | Added client seams for tenant KV, profile/preferences, tenant, Ollama, parser/retrieval/memory/chat-history, personal env vars, storage/vector/web-search resources, system info and WeKnoraCloud, plus `/platform/settings` covering all 15 registry sections with real read operations, scope/role/operation badges, safe value summaries and panel-local errors. Fixed route drift: active tenant reads `/api/v1/auth/me` `data.tenant` (old `/api/v1/auth/tenant` returned 404`). The current Web run proves tenant settings read/save against isolated Lite; remaining live provider credentials, other settings writes/resets/tests, permission/Lite/native coverage and browser interaction evidence remain. Evidence: `docs/migrations/react/evidence/t17-settings-resource-live-2026-09-11.md`, `docs/migrations/react/evidence/t17-config-live-2026-09-11.md`, `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md` |
| T18 Embed、IM与外部集成入口 | review | `4ee56e0` + working tree T18 management/live slice | `pnpm test:shared` 0（151/151）；`pnpm test:web` 0（61/61）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`pnpm test:embed` 0（3/3）；`pnpm typecheck:embed` 0；`pnpm build:embed` 0；boundary/diff 0；隔离 Lite Embed config 200、origin 拒绝 403、CRUD/IM toggle/Principal 200；2026-09-12 Web/独立 Embed 入口缺 token 失败关闭通过 | Independent Embed entry, no-cookie/no-Bearer session/chat/history/SSE, origin guard, Web management CRUD, IM toggle, API Principal and executable SSE playground: passed; current browser run additionally proves Web `/embed` guard and independent Embed `Missing embed token.` state; Lite session exchange 503 due no Redis, third-party iframe proxy, valid external SSE/provider callback, permission/browser E2E: not accepted | Added concrete Embed/IM create/update/delete/toggle/rotate controls, API Principal config and short-lived test-token client, in-memory API playground SSE, workspace-linked dependencies, and fail-closed full-field Embed update payload. Evidence: `docs/migrations/react/evidence/t18-integrations-live-2026-09-11.md`, `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md`; valid token/session, proxy/provider callback and role/browser matrix remain |
| T19 Wails React renderer与Lite发布 | review | `477793f` + `fb0a3a9` + working tree desktop alias fix | `pnpm typecheck:desktop` 0；`pnpm test:desktop` 0（2/2）；`pnpm build:desktop-renderer` 0（修复 alias 后 121 modules）；`REACT_FRONTEND=1 ./scripts/package-mac-app.sh` 0；Wails `v2.12.0` macOS arm64 package, Resources, React Web/Embed identity and codesign 0；`git diff --check` 0 | Wails React renderer and isolated Lite macOS package/runtime smoke passed; Windows/Linux package, installed upgrade lifecycle and cross-OS behavior remain unverified | `apps/desktop/vite.config.ts` now maps every shared domain subpath consumed by Web views. Evidence: `docs/migrations/react/evidence/t19-t20-rerun-2026-09-11.md`; full cross-OS and installed upgrade evidence remain required before accepting T19 |
| T20 Expo基础、原生登录和网络生命周期 | review | `fcc1069` + `df7d2a9` + `6362bf5` + `8121c51` + `01f346c` + `2057a8a` + `6115cb8` + `841d431` + `3275e78` + current transition guard | `pnpm test:shared` 0（当前 164/164）；`pnpm typecheck:shared` 0；`pnpm test:mobile` 0（当前回归 57/57；T20 原始 19/19）；`pnpm typecheck:mobile` 0；iOS/Android Expo exports 0；Android `:app:bundleRelease`/`:app:assembleRelease` 0；Android release APK install/launch 0；Android release real login/KB list/cold SecureStore restore 0；Android logout 0；`expo run:ios --device "iPhone 17 Pro" --no-bundler` build/install 0；worktree Metro iOS bundle 0；iOS simulator live login/list/server-address/workspace/role/SSO capability validation/cold SecureStore restore 0；boundary/diff 0 | Expo Router/RN foundation, SecureStore credential adapter, HTTP(S)-validated native transport, portable native error boundary, iOS native host build/install, Android native release host install/start, Android real password login/authenticated KB list/cold SecureStore restore/logout, Metro connection, persisted native server-address setting with invalid-scheme rejection, server-backed workspace membership display/switching and current-role presentation, route readiness before bearer restoration, session-epoch protection for late refresh results, transition-time refresh/header suppression, registration/invite routes, and server-owned SSO-disabled error handling: passed; Android SSE/chat is covered by T22; real refresh-token rotation, Android AppState/network recovery, successful provider callback and backend registration/permission matrix remain | Fixed the shared request boundary so React Native without a global `DOMException` preserves cancellation/error classification instead of replacing a network failure with a ReferenceError; the same portable abort error is used by Embed/native stream/file seams. Mobile transport now performs one refresh-coordinated retry only for idempotent reads, never replays POST/upload/chat writes, and refreshes the SecureStore credential on iOS foreground transitions. Added SecureStore-backed server address and selected-workspace adapters, native registration/invite routes, OIDC callback parsing/state check and external-start seam, `auth.me` membership hydration, a server-owned workspace switcher, and route/session guards for cold deep-links and late logout/switch responses. Added a transition guard that blocks new refresh attempts, removes stale auth/tenant headers while transitions are active, and clears the bearer/tenant/workspace persistence before activating a new server address. Commit `3275e78` closes the duplicate-refresh race in the same transport. On iPhone 17 Pro iOS 26.5 the route loaded the persisted isolated Lite address, rejected `file:///tmp/weknora`, showed `reactmobile's Workspace · owner · Current`, rendered `Single sign-on is not enabled on this server` from the real Lite API, and the API-key screen presented `Workspace role: owner` in the current authenticated runtime. Android now permits the documented HTTP(S) server-address contract through an Expo manifest plugin; a release APK authenticated against isolated Lite, rendered the real empty KB state, restored the SecureStore session after force-stop/relaunch, and returned to Sign in after logout. Evidence: `docs/migrations/react/evidence/t20-ios-simulator-auth-live-2026-09-11.md`, `docs/migrations/react/evidence/t20-ios-simulator-server-address-live-2026-09-11.md`, `docs/migrations/react/evidence/t20-ios-simulator-auth-extensions-live-2026-09-11.md`, `docs/migrations/react/evidence/t24-ios-simulator-cold-securestore-live-2026-09-11.md`, `docs/migrations/react/evidence/t20-android-release-live-2026-09-11.md`, `docs/migrations/react/evidence/t20-android-release-logout-live-2026-09-11.md`, `docs/migrations/react/evidence/t24-android-native-release-2026-09-11.md`, `docs/migrations/react/evidence/t24-ios-simulator-workspace-refresh-live-2026-09-11.md`, `docs/migrations/react/evidence/t24-session-race-review-2026-09-11.md` |
| T21 原生知识库、检索与文件 | review | `a60a1c7` + working tree evidence (`apps/mobile`, shared contracts/API client) | `pnpm test:shared` 0（当前 164/164）；`pnpm exec tsx --test packages/contracts/test/knowledge-documents.test.ts` 0（5/5）；`pnpm test:mobile` 0（当前 57/57）；`pnpm typecheck:shared` 0；`pnpm typecheck:mobile` 0；Expo iOS/Android exports 0；iOS simulator KB/detail/list/picker/share/download/search/403 0；boundary/diff 0 | Shared DTO/API tests, native platform URI seam, native FlatList source, production iOS JS bundle, real authenticated KB list, real folder/tag/document list envelopes, native empty-state route, native document detail, iOS Files picker selection, processing→completed lifecycle, embedding-backed hybrid search, protected download bytes/filename and cross-account 403, native share panel: passed; Android completed-fixture parity, cancellation/large-file/413, real-device and production-storage evidence: not completed | Added folder/tag/detail/search/download-path contracts, native `{uri,name,type}` upload source, Expo DocumentPicker/FileSystem/Sharing seam, KB/document/filter/detail routes, pagination and AppState refresh. Live Lite returned a paginated nested tag envelope that was initially rejected; strict parser support was added with RED→GREEN coverage. A current iPhone 17 Pro run selected a 144-byte TXT fixture through the real Files picker, observed `processing→completed`, confirmed embedding and hybrid-search content, verified authenticated download bytes/filename, opened the native share panel, and confirmed an isolated viewer received 403. Earlier Android evidence still covers picker/upload/detail/share while processing; cancellation/large-file/413, same-fixture Android completion, real-device and production-storage acceptance remain. Evidence: `docs/migrations/react/evidence/t21-ios-simulator-list-live-2026-09-11.md`, `docs/migrations/react/evidence/t21-ios-upload-picker-attempt-2026-09-12.md`, `docs/migrations/react/evidence/t21-ios-upload-complete-download-live-2026-09-12.md` |
| T22 原生聊天、引用与核心闭环 | review | `999a5b2` + `8900f41` + working tree chat parity/entry follow-up (`apps/mobile/src/features/{chat,knowledge}`, shared chat contracts/API client) | `pnpm test:shared` 162/162；`pnpm typecheck:mobile` 0；`pnpm --filter @weknora/mobile test` 0（79/79）；`pnpm typecheck:shared` 0；Expo iOS/Android exports 0；Android `:app:assembleRelease` 0；iOS Release build/install 0；`git diff --check` 0；Android release Stop/background-resume/SSE live probes 0；iOS native Chat entry/SSE/Stop probes 0；原生产物预览 focused 2/2 | Shared stop/continue/attachment/SSE/OAuth/steer tests, nested assistant-id parity fixture, native FlatList chat, authenticated artifact download/share seam, MCP OAuth authorize/status client and production iOS/Android JS bundles: passed; Android early-stream Stop, background/resume recovery, and successful FTS5 Lite→OpenAI-compatible SSE token delivery with one completed assistant rendering: passed; iOS release knowledge-base→Chat entry, authenticated native iOS→isolated Lite→OpenAI-compatible SSE with one completed `Hello from iOS` assistant rendering, and bounded native cancellation returning to `Send`: passed; native text/Markdown preview projection and download-only safety classification: passed; approval/attachment/device recovery, server continuation after interruption, remote stop-after-id acceptance, protected artifact provider download/click-through, and production-provider behavior: not completed | Added native chat route with sessions/history, selected-KB request scope, shared reducer streaming, stop/continue after AppState, retry/error state, attachments, tool approval, grouped references, MCP OAuth authorization cards with external callback/status resolution, safe plain-text rendering and artifact metadata. A history reload no longer duplicates the pending user message or completed assistant. The knowledge-base header now exposes a native Chat entry that reaches `chat/index` on iOS. Android release probes cover the real `Stop` button before assistant-id assignment, background cancellation/foreground recovery to `Send`, and a completed `Hello from Android` response from the isolated FTS5 Lite backend. The chat artifact control now offers native Modal preview for text/Markdown/image, while HTML/XHTML/SVG/PDF/Office/unknown files remain download/share-only. Evidence: `docs/migrations/react/evidence/t22-android-stop-live-2026-09-11.md`, `docs/migrations/react/evidence/t22-android-background-resume-live-2026-09-11.md`, `docs/migrations/react/evidence/t22-android-sse-live-2026-09-11.md`, `docs/migrations/react/evidence/t22-ios-chat-entry-live-2026-09-11.md`, `docs/migrations/react/evidence/t22-mobile-artifact-preview-follow-up-2026-09-12.md` |
| T23 移动管理功能与能力矩阵补齐 | review | `aa35f4` + `4becf40` + `d05f657`（server-capability projection/API-key、mobile Wiki/FAQ editor、administration/organization/API-key slices、FAQ/Wiki live contract fixes and evidence） | `pnpm test:shared` 0（当前 164/164）；`pnpm test:mobile` 0（当前 57/57）；FAQ focused 4/4；reference focused 5/5；`pnpm typecheck:mobile` 0；`pnpm typecheck:shared` 0；Expo iOS/Android exports 0；iOS Release build/install 0；Web/boundary/diff 0；isolated Lite organization owner/viewer HTTP matrix and native iOS owner/invitation/audit/FAQ/Wiki writes recorded | Explicit mobile capability matrix, capability-gated management hub with server fail-closed projection, role-gated Wiki/FAQ list/detail/edit/create surface, owner/admin member/invitation/audit management, Owner-only scoped API-key route with one-time token display, organization create/member-role/remove/join-review and shared KB/Agent inventory/removal flows, read-only configuration surface, production iOS/Android JS bundles, and prior real Lite capability projection/identity/configuration screens: passed; remaining full role matrix, live share removal, configuration writes and native non-owner/409/device-provider matrix: not completed | Added native FAQ/Wiki editor route with shared `get/create/update` APIs, server-owned Wiki optimistic `version`, local required-field validation, explicit 403/error handling and 409 reload-latest action. Added native Members/audit and Organizations routes with server-backed mutations; owner/admin controls are shown only for the active role, owner removal is disabled, and failed mutations do not optimistically alter rows. Added organization-scoped shared KB/Agent inventory using existing server list endpoints; admin-only destructive actions preserve the server share/resource IDs, confirm before mutation, call the existing authenticated delete route, and reload only after server confirmation. Exported the shared `OrganizationShare` contract from the SDK. Added native workspace API-key management using the real Owner-only backend route; scoped capability validation is local and returned tokens stay memory-only. Management now projects server-disabled surfaces as unsupported; the Lite `organizations: not_supported_in_lite` response prevents the hub and direct organization route from presenting writable content. An isolated Lite owner created the organization, viewer joined, owner role update returned 200, and viewer role update returned 403. The audit parser now accepts empty optional scope/request metadata and the fixed iOS Release screen rendered the real `rbac.invitation_sent` row. The fixed iOS Release screen also created/edited/refreshed real FAQ and Wiki records; nullable optional FAQ arrays and Wiki slug addressing were repaired with focused RED→GREEN tests. Unsupported sandbox/offline/embed admin surfaces remain visible with reasons. Evidence: `docs/migrations/react/evidence/t23-mobile-reference-editor-2026-09-11.md`, `docs/migrations/react/evidence/t23-mobile-administration-2026-09-11.md`, `docs/migrations/react/evidence/t23-mobile-organizations-2026-09-11.md`, `docs/migrations/react/evidence/t23-mobile-api-keys-2026-09-11.md`, `docs/migrations/react/evidence/t23-mobile-lite-capabilities-live-2026-09-11.md`; T23 remains `review`. |
| T24 构建、回归矩阵与灰度发布 | review | `0e886be` + `32286f0` + `8121c51` + `3d13c15` + `c97ce2d` + `01f346c` + `2057a8a` + `000d828` + `6115cb8` + `841d431` + `cac3e95` + `002b3e2` + `docs/migrations/react/evidence/t24-lite-live-2026-09-11.md` + `docs/migrations/react/evidence/t24-lite-react-cli-package-2026-09-12.md` + `docs/migrations/react/evidence/t24-wails-macos-2026-09-11.md` + `docs/migrations/react/evidence/t24-docker-multiarch-2026-09-11.md` + `docs/migrations/react/evidence/t24-nginx-react-2026-09-11.md` + `docs/migrations/react/evidence/t24-react-bundle-rerun-2026-09-12.md` + `docs/migrations/react/evidence/t24-react-nginx-rerun-2026-09-12.md` + `docs/migrations/react/evidence/t24-desktop-release-resources-2026-09-12.md` + `docs/migrations/react/evidence/t24-rollback-local-2026-09-11.md` + `docs/migrations/react/evidence/t24-static-performance-2026-09-11.md` + `docs/migrations/react/evidence/t24-mobile-android-export-2026-09-11.md` + `docs/migrations/react/evidence/t24-android-native-build-2026-09-11.md` + `docs/migrations/react/evidence/t24-android-native-release-2026-09-11.md` + `docs/migrations/react/evidence/t20-android-release-live-2026-09-11.md` + `docs/migrations/react/evidence/t24-android-native-runtime-attempt-2026-09-11.md` + `docs/migrations/react/evidence/t24-android-network-recovery-live-2026-09-12.md` + `docs/migrations/react/evidence/t24-ios-simulator-workspace-refresh-live-2026-09-11.md` + `docs/migrations/react/evidence/t24-session-race-review-2026-09-11.md` + `docs/migrations/react/evidence/t24-go-regression-2026-09-11.md` + `docs/migrations/react/evidence/t24-go-regression-rerun-2026-09-12.md` + `docs/migrations/react/evidence/t24-go-regression-final-2026-09-12.md` | Focused static/protocol verification, isolated Lite live backend/browser smoke, macOS arm64 Wails package smoke, local React Lite CLI package smoke, local multi-arch Vue Docker build, local React-candidate Nginx proxy smoke, local old-Vue rollback rehearsal, static performance/package sampling, Android/iOS Expo exports, Android debug/release package builds, release install/start/authenticated KB-list/cold-session restore, and iOS Simulator native workspace/current-role/cold SecureStore smoke passed; React Web/Embed candidate bundle, mobile CI workflow, root-context Docker wiring, dedicated Embed fallback test, release runbook, and fail-closed desktop React resource/checksum workflow plus NSIS resource path added; live Lite health, React Web/deep-link/Embed serving, auto-setup, authenticated `auth.me`, capabilities and KB list passed on isolated SQLite; local React Lite CLI tarball now contains Web/Embed entries, BUILD_INFO and checksum; local desktop artifact checksum/extraction smoke passes; macOS arm64 Wails build/sign/runtime, Android/iOS JS exports, Android native debug/release packages, release APK install/native host start, real Android login/KB list/cold SecureStore restore/logout, and `linux/amd64` + `linux/arm64` OCI manifests for the legacy Vue image passed; prior candidate Nginx evidence covers `/api`, `/files`, `/r`, SSE, terminal WS 101, SPA/Embed fallback and long-cache assets, while the current-source rerun covers Web/Embed static fallback/cache; local rollback restores Vue `frontend` image input without DB changes; Android foreground offline→online refresh and AppState background/resume behavior are now evidenced on the emulator, while iOS network recovery, earlier debug cold-dev-client ANR, static HTML/entry timing and artifact sizes are recorded without browser paint/memory/real first-token performance, real registry/deployed Nginx, Windows/Linux Wails and full browser/OS/role matrix remain incomplete; `GOWORK=off go test ./...` now exits 0 after making DNS-independent test fixtures explicit; the linker still emits non-fatal duplicate `-lc++` warnings on macOS | Keep Vue `frontend/` as production release input until remaining T24 gates are evidenced; do not start T25 Vue deletion |
| T25 Vue退役与维护交接 | pending | — | — | — | 依赖 T24 且满足退役门槛 |

### T22 native artifact preview follow-up (2026-09-12)

- The T22 native chat follow-up adds a platform-owned artifact preview Modal.
  Text and Markdown use safe native `Text` projection; non-SVG images use a
  private cache URI; HTML/XHTML/SVG/PDF/Office/unknown files remain
  download/share-only. The authenticated message artifact route is reused and
  text is read only from the downloaded private cache. Focused 2/2 artifact
  preview tests, mobile 79/79, mobile typecheck, iOS/Android Expo exports, and
  the React boundary check pass. Evidence:
  `docs/migrations/react/evidence/t22-mobile-artifact-preview-follow-up-2026-09-12.md`.
  This closes a source-level native interaction gap only; provider-backed
  download and device click-through evidence remain open, so T22/T24 stay
  `review`.

### T23 mobile data-source management follow-up (2026-09-12)

- The native knowledge-base data-source route now provides create/edit,
  explicit credential validation, sync, pause/resume, logs, and
  confirmation-gated delete using the existing authenticated SDK. Editing
  never prefills server credentials; failed mutations leave the list unchanged
  until reload. The focused form suite is 3/3, full mobile tests are 82/82,
  mobile typecheck, iOS/Android Expo exports, boundary checks, and diff checks
  pass. Evidence:
  `docs/migrations/react/evidence/t23-mobile-data-source-management-follow-up-2026-09-12.md`.
  Provider credentials, sync completion, role/tenant negatives, and device
  click-through remain open, so T23/T24 stay `review`.

### T23 mobile configuration management follow-up (2026-09-12)

- The native Configuration screen now writes Agent, Model, and MCP records,
  with dedicated write-only credential fields and recursive secret-key
  rejection for JSON configuration. Skills remain read-only and are recorded
  as a separate capability. Focused capability/configuration tests are 6/6,
  full mobile tests are 85/85, mobile typecheck, iOS/Android Expo exports,
  boundary checks, and diff checks pass. Evidence:
  `docs/migrations/react/evidence/t23-mobile-configuration-management-follow-up-2026-09-12.md`.
  Live provider credentials/connectivity, role/tenant negatives, and device
  click-through remain open, so T23/T24 stay `review`.

### T13 follow-up (2026-09-12)

- `c63529c` adds Web text-only live-response rendering for thinking, tool-call status/results, and references with nested secret redaction and React escaping. Verification and evidence: `docs/migrations/react/evidence/t10-t13-web-live-state-2026-09-12.md`.
- The current React Web increment adds `packages/views/src/chat/markdown.ts`, wires `MessageList` through a safe `marked`/KaTeX renderer, adds strict metadata-only artifact list/download seams in `@weknora/api-client`, connects the Web save-file action, maps normalized tool results into readable structured views with recursive secret redaction, renders grouped stream/history references with safe web links and local chunk activation, opens absolute HTTP(S) citation targets through the Web host adapter, lazily hydrates Mermaid SVG only after strict DOMPurify sanitization, and turns the development Markdown route into a real browser fixture for that same boundary. Fresh verification is `pnpm test:shared` 221/221, `pnpm typecheck:shared` 0, `pnpm typecheck:web` 0, `pnpm test:web` 111/111, Desktop 2/2, Embed 3/3, Mobile 77/77, all five typechecks 0, Web/Desktop/Embed builds 0, both Expo exports 0, and the browser fixture observed one sanitized SVG with zero rendered scripts. Evidence: `docs/migrations/react/evidence/t13-react-markdown-renderer-2026-09-12.md`, `docs/migrations/react/evidence/t13-react-markdown-browser-2026-09-12.md`.
- Commits `04707b3`, `4d364f7`, `4c53589`, `0dba0c3`, `c5c2fa9`, and `466ed8d` add and review the tool-result view, grouped reference panel, Web citation host adapter, bounded snippets, generic-title preservation, and de-duplicated reference rendering. Focused reference/citation checks and the same full shared/Web/typecheck/build/boundary verification remain green; the evidence boundary is unchanged.
- 2026-09-12 规格/质量复核：逐项对照设计与迁移计划中的 T13 renderer、引用、工具结果、受保护产物和安全边界；未发现硬性规范违规。复核修复了泛化引用 `title` 丢失、引用全文无界展示、流式引用重复展示和会话切换残留高亮，随后复跑 Web/shared 及跨端受影响检查；真实后端引用导航、浏览器/原生安全与性能证据仍未具备。
- Mermaid is now a controlled, lazy SVG hydration path with strict configuration, sanitizer injection, and readable code-block fallback on failure. Citation navigation, protected artifact live download/preview, browser XSS/performance and native acceptance remain open; T13 stays `review`.

### T11 follow-up (2026-09-12)

- `fecbbb7` adds server-confirmed Web Rename, Pin/Unpin, and Delete actions backed by typed session mutations; failures do not optimistically alter the sidebar. Evidence: `docs/migrations/react/evidence/t11-web-session-management-2026-09-12.md`.
- Title generation, source filtering, live permission/error matrix, and native acceptance remain open; T11 stays `review`.

### T11 chat main-interface increment (2026-09-12)

- `beb7333` adds typed message-suggestion contracts/client methods, clear and
  bounded older-history loading, source/search/date grouping, scroll-position
  preservation, suggestion telemetry/polling, and authoritative history
  refresh after a completed Web stream. The first-send path now creates a
  server session and reconciles the transient assistant row without a
  duplicate persisted user message.
- Fresh verification passes: `pnpm test:shared` 201/201, `pnpm test:web`
  106/106, shared/Web typechecks, Web build, React boundary check, and
  `git diff --check` all exit 0. The Web build transforms 135 modules and
  retains the existing non-fatal chunk-size warning.
- New live mutation/filter/history/suggestion-provider behavior was not run in
  this increment. Real RAG/Agent/third-party generation, continuation,
  cancellation timeout, permission negatives and native chat remain open;
  T11 stays `review`. Full scope and evidence boundary:
  `docs/migrations/react/evidence/t11-web-chat-main-2026-09-12.md`.

### T11 review follow-up (2026-09-12)

- `56393e4` closes the review findings: session list pagination now uses the
  server total; source options are restricted to Web for Viewer/Contributor
  and expose tenant-wide channel sources only to active-tenant Owner/Admin;
  stale history/stream results are rejected after a session switch; the
  Composer and route reject concurrent sends; MessageList resets its scroll
  baseline per session; and ready suggestion sets emit one impression event.
- TDD regression coverage first failed on the new page-count/session-switch
  exports and the streaming Composer state, then passed after the minimal
  implementation. Fresh full verification is 203/203 shared tests and
  107/107 Web tests, with shared/Web typechecks, Web build, boundary and diff
  checks all exiting 0. T11 remains `review` pending live mutation/filter/
  history/suggestion-provider, real RAG/Agent/third-party, recovery,
  cancellation, permission-negative and native evidence.

## T01 证据记录

### 已完成

- 重新读取并核对设计、实施计划和迁移库存；计划中方案 B 原为建议，现按用户批准决策执行。
- 检查目标/参考仓库 HEAD、分支和工作区；未修改参考仓库。
- 核对目标仓库适用规范与 CLI 子目录边界。
- 采集 Vue/API/Swagger/Wails 的当前实际数量与入口。
- 生成 `docs/migrations/react/route-parity.csv`（53 行入口/设置/特殊路由清单）、`api-contract-matrix.csv`（361 行 Swagger 操作 + 91 条实现专有行），并补充 `source_status`、Gin 注册来源、SSE/Embed/文件/终端 WS 特殊分类。
- 新增 `scripts/test_generate_react_migration_baseline.py`：检查 282 paths/361 operations、Swagger/注册路由覆盖、非空任务/身份/能力、特殊路由分类和双次生成字节确定性。
- 运行当前 Vue 基线：`npm ci --ignore-scripts`、804 个前端测试、`vue-tsc --build` 和 Vite 构建均退出码 0；`go test ./docs` 退出码 0。
- 记录 Multica 仅作为架构模式参考；其源码、UI、品牌和业务模型不复制，因根许可证带附加条件而采用 clean-room 路线。
- T01 范围提交：`eb0e9a9`；提交未包含用户提供的三份未跟踪权威输入文档。
- 独立 tester 复核矩阵确定性、361 个 API 行、46 个入口行、6 个复用项和 `go test ./docs`；首轮 reviewer 因超时关闭，未产生可采纳 findings。

### 待完成

- route parity：逐项连接 Vue 入口/旧 URL、Go 注册路由和能力/角色。
- API contract：已以 Go 路由注册代码对照 Swagger，并分类 Swagger 独有/实现独有/客户端使用/特殊路由；handler DTO/注解/权限/测试仍需逐行复核。
- reuse manifest：记录目标/参考源码、固定提交、许可证与复用策略；未核实授权的实现独立编写。
- version matrix：验证 Swagger 2.0 生成路线、Node/pnpm/TypeScript/Vite/React/Expo/Wails 版本和原生兼容性。
- runtime baseline：六种语言、旧 URL、Lite 数据路径、核心截图及真实后端 smoke；无法运行的条件单列为缺失证据。

### T01 review findings

- 计划库存与当前 HEAD 存在 SFC 数量差异（199 → 200），已在运行基线和矩阵中记录。
- Swagger 2.0 行数与当前文档一致（282 paths/361 operations），但这不是运行路由证明；矩阵用 `source_status` 区分 `swagger`、`registered-route`、`client-used` 和 `special-route`，handler DTO/权限仍未宣称已审结。
- OpenAPI Generator 7.14.0 的兼容性试点已完成，但 Swagger 严格校验仍失败；真实后端测试身份、核心截图采集和 Wails/Expo 主机证据仍缺失，这些缺失不能用已有 Vue 构建替代。

## 变更与提交记录

| 时间 | 变更 | 提交 |
|---|---|---|
| 2026-09-10 | 创建账本；保留用户已有权威文档未提交状态 | 待 T01 范围提交 |
| 2026-09-10 | T05/T06 React Web slice + workspace integration, package install and verification | `a230366`, `ac955da` |
| 2026-09-10 | T07 document contracts, processing state guard and multipart API seam | `45901be` |
| 2026-09-10 | T07 Web document loader seam and client alias integration | `d0af87f` |
| 2026-09-10 | T07 harden multipart transport environment guard | `93d3099` |
| 2026-09-10 | T06 add typed knowledge-base create/update/delete API paths | `de1ce63` |
| 2026-09-10 | T08 add shared Wiki revision diff algorithm and regression tests | `736eca4` |
| 2026-09-10 | T08 add typed Wiki page/revision API client | `3d8daa6` |
| 2026-09-10 | T09 add data-source API client seam and endpoint tests | `f2fe50e` |
| 2026-09-10 | T10 add chat response contracts and pure stream reducer | `cfb7d29` |
| 2026-09-10 | T10 add incremental SSE framing and resumable request builder | `f2c2fd3` |
| 2026-09-10 | T16 shared identity and administration API seam, strict contract tests and client wiring | `8dd391b` |
| 2026-09-10 | T17 remaining settings client seams, secret redaction, section registry and regression coverage | `798543e` |
| 2026-09-10 | T01-T05 completion plan and scoped React fixes | `e4134a0`, `0105fa8`, `ef3377a`, `19dede6`, `cc3556f`, `a784f2b`, `a356e80` |
| 2026-09-10 | T06 React knowledge-base mutation flow | `292fdc4` |
| 2026-09-11 | T20 Expo mobile foundation, native transport and iOS export verification | `fcc1069` |
| 2026-09-11 | T21 native knowledge, retrieval, filters, upload/download/share seams and iOS export verification | `df6148f` |
| 2026-09-11 | T22 native chat, resumable stream, attachments, approvals, citations and iOS export verification | `2006cd8` |
| 2026-09-11 | T23 mobile capability matrix and read-only management/configuration surfaces | `aa35f4a` |
| 2026-09-11 | T04 scoped Web session gate, capability guard, dynamic tenant transport, and strict auth.me identity DTOs | `f5517e7`, `55fca3f` |
| 2026-09-11 | T05 semantic tokens, five-locale key parity, Dialog primitive, and Web platform ports | `544e739` |
| 2026-09-11 | T06 React knowledge-base filters, creator query forwarding, and paginated list surface | `f441475` |
| 2026-09-12 | Make Go verifier/provider unit fixtures deterministic under the sandbox SSRF/Python contracts; full Go regression rerun exits 0 | working tree + `docs/migrations/react/evidence/t24-go-regression-rerun-2026-09-12.md` |

## T01-T05 completion-plan execution (2026-09-10)

- 详细执行计划：`docs/superpowers/plans/2026-09-10-react-multiclient-t01-t05.md`，提交 `e4134a0`。
- 隔离 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`，分支 `codex/react-multiclient`。
- T01 集成提交：`a356e80`（worker 原提交 `e211610`）；生成器把 Swagger 361 操作、实现专有 91 路由和 53 条入口/特殊路由分开记录，focused unittest 4/4 通过。
- T02 集成提交：`0105fa8`、`19dede6`；新增 Web 注入式 transport，修复 Embed scheme/session/visitor/tenant 隔离，Web focused tests 11/11、shared tests 46/46、Web typecheck/build 通过。
- T03 集成提交：`ef3377a`；新增严格 auth login/refresh/me/logout facade 和 React Login seam，shared tests 46/46、Web typecheck/build 通过；完整 OIDC/真实后端尚未验证。
- T04 集成提交：`cc3556f`；新增 scope runtime、旧 URL 解析/redirect 和跨空间 generation 测试，Web tests 11/11 通过；真实深链/后端权限负例尚未验证。
- T04 follow-up（working tree）：新增 `auth.me` → origin/user/tenant/capability hydrate、动态 `X-Tenant-ID`、`auth.switchTenant` 提交 seam、无租户 onboarding、登录/能力/system-admin guard、`/join?code` 组织邀请兼容和 `/register?token` 分享邀请入口；focused T04 tests 通过，真实浏览器深链与后端权限负例仍未验证。
- T05 集成提交：`a784f2b`；新增无 Vue runtime 的 design-tokens/i18n/platform adapters。当前实际源码只有 5 个 locale（不是计划文字中的 6 个），未凭空新增第六语言；i18n/native/keyboard browser evidence 尚未完成。
- T05 follow-up：`544e739` 新增五个现有 locale 的基础 UI key 与 placeholder parity、semantic focus/disabled/typography tokens、loading/disabled/status 状态、DOM-only Dialog（Escape/focus restore）以及可替换的 Web navigation/storage/file/clipboard ports；Node/Vite 证据通过，完整旧目录翻译迁移、浏览器键盘/焦点和原生实现仍待补。

### T23 follow-up: mobile organization shared resources (2026-09-11)

- Added organization-scoped shared knowledge-base and agent inventory to `apps/mobile/src/features/management/OrganizationsScreen.tsx` using the existing `GET /api/v1/organizations/:id/shares` and `GET /api/v1/organizations/:id/agent-shares` contracts.
- Added admin-only, confirmation-gated removal through the existing authenticated KB/agent share delete routes. The screen keeps the server-provided share/resource identifiers, reloads after a confirmed server response, and leaves rows unchanged on failure; missing identifiers fail closed.
- Exported `OrganizationShare` from `packages/api-client/src/index.ts` so the mobile surface consumes the shared SDK contract rather than a local DTO.
- TDD evidence: `pnpm exec tsx --test apps/mobile/src/features/management/organizations.test.ts` first failed because the helper exports were absent, then passed 3/3 after the implementation.
- Verification: `pnpm --filter @weknora/mobile test` 34/34 exit 0; `pnpm --filter @weknora/mobile typecheck` exit 0; `pnpm test:shared` 158/158 exit 0; `pnpm exec expo export --platform ios` exit 0 to `/tmp/weknora-react-mobile-shares-ios.zbCMTC`; Android export exit 0 to `/tmp/weknora-react-mobile-shares-android.eId5YA`; `node scripts/check-react-boundaries.mjs` exit 0; `git diff --check` exit 0.
- Evidence boundary: no live mobile share list/remove or native device interaction was claimed; T23 remains `review`.

### 本轮验证证据

- `pnpm install --frozen-lockfile`：0（隔离 worktree，8 workspace projects）。
- `python3 scripts/generate_react_migration_baseline.py`：0；`python3 -m unittest scripts/test_generate_react_migration_baseline.py -v`：4/4，0。
- `pnpm test:shared`：46/46，0；`pnpm typecheck:shared`：0。
- `pnpm --filter @weknora/web test`：11/11，0；`pnpm typecheck:web`：0；`pnpm build:web`：0；`git diff --check`：0。
- 证据层级：静态/Node mock/Web bundle 已有；真实后端、浏览器真实账号/OIDC、Wails 安装包、移动原生和六语言完整迁移仍缺失，因此 T01-T05 保持 `review`，不得写为 `accepted`。

### T15 configuration inventory evidence (2026-09-11)

- `194aee4` adds `apps/web/src/configuration/ConfigurationPage.tsx` at `/platform/configuration` and an explicit surface registry. It loads Agent/Model/MCP/Skill inventories through the existing shared API, preserves panel-local failures, renders server-disabled agent IDs and Skill catalog availability, and labels current support as planned/read-only instead of inventing write capability.
- The shared configuration client already strips secret fields before records reach React; the new hub only displays returned metadata and explicitly says that presence does not prove health. No generic JSON editor or long-lived credential surface was added.
- Fresh verification: `pnpm test:web` 48/48, `pnpm typecheck:web`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0; the shared suite at the same point remained 149/149. This is static/Node mock and production-bundle evidence only, so T15 remains `review` pending write-flow, permission, debug/OAuth/install, real-call and browser evidence.

### T16 administration surface evidence (2026-09-11)

- `cf34e94` adds `/platform/administration` and `/platform/system`. The workspace surface loads tenant members, pending invitations and cursor audit rows through `client.identity`; the system view is reachable only after the existing `/platform/system` system-admin guard and loads administrators, system settings and runtime queue availability through `client.administration`.
- Member role changes, removals, invitation revocation and invitation creation wait for the server result before reloading. Errors keep the current rows intact; owner removal/role controls are disabled in the UI. System secrets are displayed only as `secret` and no system-setting write form is invented in this inventory slice.
- Fresh verification: `pnpm test:web` 49/49, `pnpm test:shared` 149/149, `pnpm typecheck:web`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0. This remains static/Node mock and bundle evidence; organization CRUD/share UX, real role × tenant 403/409 behavior, browser and Lite capability acceptance remain open, so T16 stays `review`.

### T16 review evidence (current worktree)

- Added `packages/api-client/src/identity/` for tenant members, invitations, audit cursor, organization membership, join requests, KB/agent sharing and tenant-independent organization discovery.
- Added `packages/api-client/src/administration/index.ts` for tenant/platform API keys and capability scopes, system-admin operations, system settings, system audit and runtime queue/task pagination.
- Every mutation uses the shared request boundary; encoded identifiers and query parameters are tested. HTTP 403/409 `ApiError` instances are deliberately not converted to successful responses; deletion/leave methods resolve only after the server response is accepted.
- TDD evidence: missing modules produced RED module-resolution failures; focused tests then passed 7/7; aggregate `pnpm test:shared` passed 98/98.
- `pnpm typecheck:shared`, `pnpm typecheck:web`, and `pnpm build:web` exited 0; real backend, browser access matrix, Lite capability visibility and React organization/administration screens remain unverified, so T16 stays `review`.
- `4f68008` adds `/platform/organizations` with server-backed organization list/create, member role update/removal, and pending join-request visibility. The UI does not fabricate organization shares or API-key writes; those remain explicit follow-up scope.
- Fresh organization-slice verification: `pnpm test:web` 50/50, `pnpm test:shared` 149/149, `pnpm typecheck:web`, `pnpm typecheck:shared`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0. This is static/Node mock and bundle evidence; live organization permissions, 403/409 behavior, browser and Lite acceptance remain open.

### T16 live viewer role matrix evidence (2026-09-11)

- On the isolated SQLite backend, the owner invited a newly registered user as `viewer`. The invitee listed the pending invitation (`200`), accepted it, switched to tenant `1`, and received an active viewer membership.
- The viewer could read `/api/v1/tenants/1/members` (`200`, two members), but `POST /api/v1/tenants/1/invitations` and `PUT /api/v1/tenants/kv/memory-config` both returned HTTP `403` with `Forbidden: insufficient workspace role`.
- A viewer attempt to update tenant metadata also returned HTTP `403`. No production account or data was used; the temporary identities and SQLite file are isolated test state.
- This is a real HTTP role/tenant check, not a mock or browser-page claim. It is partial evidence only: owner/admin/contributor permutations, organization share/API-key writes, 409 conflict paths, system-admin screens, Lite capability filtering, and native access remain open.

### T17 review evidence (current worktree)

- Added `packages/api-client/src/settings/index.ts` and wired it under `client.settings`, covering tenant KV settings (parser, retrieval, memory workspace, storage legacy, web-search legacy and chat history), profile/preferences, tenant metadata, Ollama probes/downloads, parser probes/reconnect, personal memory CRUD, caller-owned env vars, storage backends, vector stores, web-search providers/credentials, system info and WeKnoraCloud status/credentials.
- Preserved backend response differences: `success/data` envelopes, raw `code: 0/data` parser/system probes, direct WeKnoraCloud status and logical `success: false` connection-test results are not collapsed into fabricated empty values. Requests continue through the shared transport so HTTP/API failures remain observable.
- Added recursive response secret redaction for API keys, app/client secrets, access tokens, passwords and provider secret variants; resource IDs and Ollama task IDs are encoded. TDD focused coverage includes RED module-resolution failures followed by 15/15 settings/client/registry tests passing.
- Added `packages/views/src/settings/registry.ts` with one entry for every T17 legacy section (`general`, `tenant`, `userprofile`, `ollama`, `parser`, `retrieval`, `memory`, `mymemory`, `envvars`, `storage`, `vectorstore`, `websearch`, `chathistory`, `system`, `weknoracloud`), explicit scope/role/operation metadata, and separate personal versus tenant credential scopes.
- Verification: `pnpm test:shared` 106/106, `pnpm typecheck:shared` 0, `pnpm test:web` 15/15, `pnpm typecheck:web` 0, `pnpm build:web` 0, and `git diff --check` 0. These are static/Node mock and production-bundle proofs; real backend 403/409, browser settings E2E, reset behavior against live state, Lite capability filtering, and React settings screen rendering remain unverified, so T17 stays `review`.
- `e053780` adds `/platform/settings`: all 15 registered sections have a concrete inventory description and a selected-section read operation mapped to the existing typed client (`preferences`, tenant/profile, Ollama, parser, retrieval, memory, env vars, storage, vector stores, web search, chat history, system info and WeKnora Cloud). The view exposes scope, minimum role and supported operations and omits secret-shaped fields from rendered summaries.
- Fresh T17 verification: `pnpm test:web` 52/52, `pnpm test:shared` 149/149, `pnpm typecheck:web`, `pnpm typecheck:shared`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0. This remains static/Node mock and bundle evidence; live settings writes/resets/tests/deletes, permission negatives, browser and Lite capability acceptance remain open.
- `48b7527` adds the first explicit T17 write path: the Tenant section edits only server-owned `name` and `description`, trims and validates the name, waits for `client.settings.tenant.update`, and keeps the draft/error state when the server rejects the mutation.
- Fresh tenant-edit verification: `pnpm test:web` 53/53, `pnpm test:shared` 149/149, `pnpm typecheck:web`, `pnpm typecheck:shared`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0. Live tenant permissions and browser acceptance remain open, so T17 stays `review`.

### T17 live tenant route-drift evidence (2026-09-11)

- Before the fix, the real React settings page issued `GET /api/v1/auth/tenant` and the isolated Go backend returned `404`; this was recorded in the backend request log rather than hidden as an empty settings state.
- The shared client now issues `GET /api/v1/auth/me` and strictly extracts `data.tenant`; the regression test in `packages/api-client/src/settings/index.test.ts` asserts both the request path and returned tenant identity.
- In Chrome tab `948799442`, an authenticated registration/login session loaded `/platform/settings?section=tenant` against `http://127.0.0.1:18080`. The accessibility tree showed the server-created tenant, status `active`, and editable name/description fields; after save it showed `React live workspace updated` and `Live tenant settings verification`.
- The real backend log records `PUT /api/v1/tenants/1` with the same name and description, status `200`, and `Tenant updated successfully`. This proves a real backend read and write using the shared client; it is not a mock test.
- Fresh post-fix verification: `pnpm test:shared` 150/150 (exit 0), `pnpm test:web` 53/53 (exit 0), `pnpm typecheck:shared` (exit 0), `pnpm typecheck:web` (exit 0), `pnpm build:web` (exit 0), `node scripts/check-react-boundaries.mjs` (exit 0), and `git diff --check` (exit 0). The backend also emitted pre-existing isolated-Lite warnings for missing `tenant_skills` and SQLite FTS5; those are not represented as React acceptance.
- T17 remains `review`: only the tenant read/write slice has real backend/browser evidence. Other settings writes/resets/tests/deletes, role-negative coverage, Lite capability filtering, and native settings acceptance remain open.

### T17 profile and memory controls (working tree, 2026-09-11)

- Added strict password-change form validation (`old_password`, `new_password`, confirmation and same-password rejection) before the existing typed `/api/v1/auth/change-password` mutation. Profile identity remains server-owned/read-only.
- Added concrete personal-memory controls backed by the existing typed SDK: user enable toggle, create, edit, and delete. Added tenant-admin workspace controls for `enabled`, `write_mode`, `max_items`, `vector_recall`, and `retrieval_conditioning`; no generic JSON editor was introduced.
- Live browser/backend sequence: personal-memory create first returned the server error `memory is disabled` while workspace memory was disabled; the tenant-admin workspace toggle then produced `PUT /api/v1/tenants/kv/memory-config` HTTP 200; creating `React migration live memory` then produced `POST /api/v1/memory/items` HTTP 200 and rendered the returned active fact in the React page. This confirms the separate workspace/user enable boundary.
- Focused TDD verification: the new surface tests first failed on missing `memoryItemPatch`, `memoryEnabledPatch`, and `memoryWorkspacePatch` exports, then passed 7/7 after the minimal implementations. Full post-slice verification: `pnpm test:shared` 150/150, `pnpm test:web` 57/57, shared/Web typechecks, Web build, boundary check, and `git diff --check` all exited 0.

### T20 review evidence (2026-09-11)

- Added `apps/mobile` with Expo Router file routes for auth/login and the initial knowledge surface, plus `app.config.ts`, Babel/Metro/TypeScript configuration and the mobile package entry.
- Added native platform seams: `SecureStore`-backed `CredentialAdapter`, HTTP(S)-only JSON transport with explicit API-base validation, native `ReadableStream` SSE framing, and a React runtime provider that supplies scoped client/auth state to routes.
- `pnpm --filter @weknora/mobile typecheck`: 0; `pnpm --filter @weknora/mobile test`: 1/1, 0; the focused test proves non-HTTP(S) mobile API bases are rejected.
- `pnpm --filter @weknora/mobile exec expo config --type public --json`: 0; resolved app identity is `WeKnora` / `weknora` / `weknora`.
- `pnpm --filter @weknora/mobile exec expo export --platform ios --output-dir <temporary directory>`: 0; Metro bundled 1,069 modules and emitted one Hermes iOS bundle (`2.8 MB`) plus metadata. The output was kept outside the repository.
- Expo SDK 55 compatibility check was run after aligning React Native `0.83.10`, `react-native-screens ~4.23.0`, and `@types/react ~19.2.10`; pnpm/Metro required explicit runtime dependency declarations for the Expo/React Navigation graph.
- This is still `review`: no iOS/Android simulator or device launch, native install/package artifact, live backend login/refresh/logout, AppState/network recovery, or real API acceptance was available in this run. T21–T23 remain blocked on those runtime proofs rather than being marked complete by the bundle export.

### T21 review evidence (2026-09-11)

- Extended `packages/contracts` with strict document detail, folder-tree, tag-list, and file-search envelopes. Extended `packages/api-client/src/knowledge/documents.ts` with folders, tags, detail, search, download-path, and `NativeFileSource` upload support; native `{uri,name,type,size}` is appended only at the multipart platform boundary and is not part of shared document DTOs.
- Added `apps/mobile/src/platform/files.ts` using Expo DocumentPicker (`copyToCacheDirectory: false`), FileSystem native download with Bearer headers, and Expo Sharing. Added pure URI/source tests and an upload cancellation controller; the mobile screen does not buffer picked files into a Blob.
- Replaced the single knowledge shell with Expo Router native routes: knowledge-base list, virtualized document list, folder/tag filters, KB-scoped keyword filtering, pagination, AppState foreground refresh, native upload/cancel, document detail, and download/share action. `src/features/knowledge/parity.ts` provides the shared selector seam and tests count/order/status parity.
- `pnpm test:shared`: 121/121, exit 0; `pnpm typecheck:shared`: exit 0; `pnpm --filter @weknora/mobile test`: 3/3, exit 0; `pnpm --filter @weknora/mobile typecheck`: exit 0; `expo config --type public --json`: exit 0; `expo export --platform ios`: exit 0, 1,092 modules and one Hermes iOS bundle (`2.9 MB`); `git diff --check`: exit 0.
- This remains `review`: no iOS/Android simulator/device test, native picker or large-file upload/download/share run, real backend parity/403/413 evidence, or full background cancellation proof was available. The iOS export proves the JS/native module graph is bundleable, not that platform behavior is accepted.

### T22 review evidence (2026-09-11)

- Added strict temporary-attachment contracts and `chat.attachments` shared API for session-scoped native multipart upload/list/get/delete. Added `chat.stop` and `chat.continueStream`; SSE stream options now carry the caller abort signal through the native transport.
- Added `apps/mobile/src/features/chat/ChatScreen.tsx` and the `/chat` Expo route. It uses the shared session/history APIs and T10 reducer for incremental answer/thinking/tool approval/reference state, uses a native `FlatList`, sends mobile channel metadata, supports retry/stop, uploads native URI attachments, and resumes an incomplete assistant message after AppState returns to foreground. React Native `Text` is the only markdown surface, so untrusted HTML has no executable sink.
- Added `apps/mobile/src/features/chat/parity.ts` and `parity.test.ts`: a fixture verifies mobile replay produces the same answer, approval, and grouped-reference state as the shared reducer; incomplete assistant selection is explicit for recovery.
- `pnpm test:shared`: 125/125, exit 0; `pnpm typecheck:shared`: exit 0; `pnpm --filter @weknora/mobile test`: 5/5, exit 0; `pnpm --filter @weknora/mobile typecheck`: exit 0; Expo iOS export: exit 0, 1,100 modules and one Hermes iOS bundle (`2.9 MB`); `git diff --check`: exit 0.
- This remains `review`: no real iOS/Android SSE/stop/resume, tool approval/OAuth/steer, attachment processing, artifact download, or background-disconnect device evidence was available. Bundle success does not replace native/backend acceptance.

### T22 OAuth/artifact follow-up (working tree, 2026-09-11)

- Extended the shared chat reducer with a separate `oauthApprovals` state so
  `mcp_oauth_required`/`mcp_oauth_resolved` cannot be mistaken for ordinary
  tool approval. The reducer covers same-service parallel prompt resolution
  and preserves authorization outcome/reason fields.
- Added typed MCP OAuth authorize-url/status/revoke methods under the shared
  configuration client. The authorize URL requires the backend callback URI;
  status requires the server-owned `authorized`, state, and refresh metadata.
- Native Chat now renders an MCP authorization card, opens the returned URL
  through the platform Linking adapter, receives the `weknora://mcp-oauth`
  callback, polls the attempt-scoped status, and resolves the paused agent
  run only after the server reports authorization. Cancel uses the existing
  authenticated cancellation endpoint. History artifacts retain the protected
  download path and native share action.
- Focused RED→GREEN reducer/API coverage completed. Full verification after
  the slice: `pnpm test:shared` 157/157, `pnpm test:mobile` 19/19,
  mobile typecheck 0, iOS/Android Expo exports 0, and `git diff --check` 0.
- This remains source/contract/bundle evidence: no real MCP OAuth provider,
  real SSE tool event, attachment processing, or device callback acceptance
  has been claimed.

### T22 steer queue follow-up (working tree, 2026-09-11)

- Native Chat now loads the session-scoped steer queue, renders `inject` versus
  `after` delivery, and exposes refresh, promote-to-inject, and remove actions
  through the shared typed client.
- While a run is active, `Inject` and `After` enqueue the draft with the active
  assistant message correlation and mobile channel. A single in-flight ref
  prevents double taps from producing duplicate mutations; if the server
  reports `new_run` because the run ended during the action, the same draft is
  sent once through the normal stream path.
- Stream cleanup is controller-bound: an older run cannot clear `sending` or
  replace the controller after the fallback run has started. Queue refreshes
  after session load, mutation, and run completion.
- `pnpm test:shared`: 157/157; `pnpm test:mobile`: 19/19;
  `pnpm typecheck:mobile`: exit 0; iOS/Android Expo exports: exit 0;
  `git diff --check`: exit 0.
- This is still source/contract/bundle evidence: no live backend steer
  mutation, real concurrent SSE run, or device-level queue interaction has
  been claimed.

### T22 error-event follow-up (working tree, 2026-09-11)

- The isolated Lite backend returned a real SSE `error` event for a chat with
  no configured knowledge base/model. The payload's `assistant_message_id` was
  nested under `data`; a mobile parity regression now extracts that id so Stop
  and steer correlation do not silently no-op.
- The same error path leaves the server assistant row `is_completed:false`.
  Native Chat now marks that assistant as failed locally, renders a retryable
  failure label, and excludes it from foreground auto-resume. This keeps the
  server's resumable state intact without presenting an endless spinner.
- `pnpm test:mobile`: 20/20; `pnpm typecheck:mobile`: exit 0;
  `pnpm test:shared`: 157/157; iOS/Android Expo exports: exit 0;
  `git diff --check`: exit 0.
- The live probe is protocol/error evidence only; no model-backed answer or
  successful real-device stop/resume is claimed.

### T23 review evidence (2026-09-11)

- Added `MOBILE_CAPABILITIES` as the explicit mobile scope matrix. Core knowledge/chat/attachments/approvals are linked; identity and configuration are read-only; Wiki/FAQ is explicitly read-only; sandbox terminal, offline writes and Embed/IM administration are explicitly unsupported with reasons.
- Added a capability-gated `/management` hub that queries `/api/v1/system/capabilities`, plus read-only Agents/Models/MCP/Skills and identity capability screens. No mobile screen invents tenant membership or bypasses server role/capability checks; write forms are intentionally not presented in this slice.
- `pnpm --filter @weknora/mobile test`: 6/6, exit 0; `pnpm --filter @weknora/mobile typecheck`: exit 0; Expo iOS export: exit 0, 1,107 modules and one Hermes iOS bundle (`2.9 MB`); `git diff --check`: exit 0.
- This remains `review`: real role × tenant backend capability matrix, member/audit live reads, configuration permission negatives, Wiki/FAQ read screens and every required management write/sync flow still need live acceptance or a subsequent scope decision; no untracked “待完善” row is treated as accepted.
- The isolated Lite smoke additionally logged `no such table: tenant_skills` from the background skill reaper; the Lite SQLite migration set is not evidence for live Skills/sandbox management, so that capability remains review/needs a backend schema decision.

### T23 read-only Wiki/FAQ slice evidence (2026-09-11)

- Added KB-scoped native `/knowledge/:id/wiki` and `/knowledge/:id/faq` routes from the mobile Files screen.
- Wiki and FAQ lists use the shared API client, preserve server-owned read-only state, and render explicit empty/error states without adding mobile write controls.
- `reference-parity.test.ts`: 3/3; `pnpm test:mobile`: 9/9; `pnpm typecheck:mobile`: exit 0.
- This is source/typecheck evidence only; simulator/device, real backend, and role/tenant negative evidence remain open.

### T23 mobile Wiki/FAQ editor increment evidence (2026-09-11)

- Added `apps/mobile/src/features/knowledge/KnowledgeEditorScreen.tsx` and the scoped route `/knowledge/:id/editor`. Existing Wiki rows use server slugs and versioned updates; new Wiki pages use the typed create endpoint. FAQ rows use typed create/update with the existing enabled/recommended fields.
- The native list shows New/Edit only for the active owner/admin workspace role. Viewer and unknown roles remain read-only; an owner/admin label does not bypass server authorization. No offline queue or generic JSON editor was introduced.
- Required title/content and question/answer fields are validated before a network write. HTTP 409 is classified as a stale Wiki conflict and offers reload of the server version; other API failures preserve the draft and display the server error for explicit retry.
- TDD focused coverage: `apps/mobile/src/features/knowledge/editor.test.ts` and `reference-parity.test.ts` pass 8/8. Full `pnpm test:mobile` passes 24/24; `pnpm typecheck:mobile` exits 0; `pnpm test:shared` passes 157/157; Web tests pass 61/61; shared/Web typechecks, Web build, React boundary check and `git diff --check` exit 0.
- Expo iOS export exits 0 with a 3 MB Hermes bundle; Expo Android export exits 0 with a 3.1 MB Hermes bundle. Outputs were written outside the repository under `/tmp/weknora-react-mobile-editor-ios.3zSoPa` and `/tmp/weknora-react-mobile-editor-android.KidKhc`.
- This remains source/bundle evidence: no live backend FAQ/Wiki write, 403/409 response, iOS/Android native editor interaction, or complete T23 management matrix is claimed.

### T24 review evidence (2026-09-11)

- Changed the UI Docker build invocation to use repository root context while keeping the legacy Vue Dockerfile inputs explicit; updated `.dockerignore`, Makefile, and `scripts/build_images.sh` so shared React packages remain visible to root-context checks.
- Added `scripts/check-react-boundaries.mjs` for shared-runtime/legacy-source leakage and Docker context guards. Added `scripts/build_react_web_bundle.sh`, which builds Web and the isolated Embed entry, writes `BUILD_INFO.json`, and emits `web/index.html`, `web/embed.html`, `web/assets/`, and `web/embed/assets/` for a candidate Lite artifact.
- Added a Go Lite static-server test proving `/embed/<channelId>` serves the dedicated `embed.html` fallback, and set the React Embed Vite base to `/embed/` so its assets do not resolve against the main SPA.
- Added explicit React Web/Embed/Desktop checks to `.github/workflows/frontend.yml`, a mobile workflow at `.github/workflows/mobile.yml`, and a separate React candidate artifact job in `release-lite.yml`. The current Vue Lite artifact remains the production/recovery input until T25.
- Added `docs/migrations/react/release-runbook.md` covering API/files exclusion, WS upgrade, SSE buffering, SPA/deep-link/embed fallback, Lite artifact identity, role × tenant × deployment × locale × browser/OS matrix, performance thresholds, staged rollout and stop/rollback conditions.
- Verification: `node scripts/check-react-boundaries.mjs` 0; `git diff --check` 0; `bash -n scripts/build_react_web_bundle.sh scripts/package-lite.sh scripts/package-mac-app.sh scripts/build_images.sh` 0; workflow YAML parsed by Ruby; `pnpm test:shared` 125/125 and `pnpm typecheck:shared` 0; Web 15/15 + typecheck/build 0; Embed 3/3 + typecheck/build 0; Desktop 2/2 + typecheck/build 0; Mobile 6/6 + typecheck 0; Expo iOS export 0 with 1,107 modules and a 2.9 MB Hermes bundle; `go test ./internal/router -run TestFrontendStatic -count=1` 0; `docker build --check -f frontend/Dockerfile .` 0; real `docker build` 0 for `weknora-ui:t24-local`; container static smoke returned 200 for `/`, `/platform/chat/session-1`, `/embed/channel-1`, and both Web/Embed JS assets.
- Additional live evidence: an `EDITION=lite` arm64 binary was built in isolation and served React assets from `dist/react-web/web` on `127.0.0.1:18081`; `/health`, Web root, Web deep-link, Embed deep-link, Lite `auto-setup`, authenticated `/auth/me`, `/system/capabilities`, and `/knowledge-bases` passed, while unauthenticated `/auth/me` returned 401. Evidence: `docs/migrations/react/evidence/t24-lite-live-2026-09-11.md`.
- Chrome rendered the live React deep-link and exposed Sessions/Conversation/message input in the accessibility tree; the expected 401 state was visible for the unauthenticated browser. This is browser runtime evidence only, not Wails or native-device acceptance.
- `PATH="/Users/wuyongjun/go/bin:$PATH" REACT_FRONTEND=1 ./scripts/package-mac-app.sh` exited 0; Wails `v2.12.0` produced a macOS arm64 `.app`, React Web/Embed resources were present, `BUILD_INFO.json` identified the React renderer, and the final assembled app passed `codesign --verify --deep --strict`. Evidence: `docs/migrations/react/evidence/t24-wails-macos-2026-09-11.md`.
- `docker buildx build --platform linux/amd64,linux/arm64 ... -f frontend/Dockerfile .` exited 0 and exported an OCI index with both platform manifests. Evidence: `docs/migrations/react/evidence/t24-docker-multiarch-2026-09-11.md`.
- A red→green Nginx candidate smoke found that the broad `/embed/` location gave hashed Embed assets `no-cache`; `frontend/nginx.conf` now has a dedicated `/embed/assets/` long-cache location. Rebuilt image `weknora-ui:t24-nginx-fix` passed root/deep-link/Embed, hashed Web/Embed asset cache headers, `/api`, `/files`, `/r`, SSE event forwarding, and terminal WS `101` against a deterministic mock backend. Evidence: `docs/migrations/react/evidence/t24-nginx-react-2026-09-11.md`.
- A local rollback rehearsal started the same Nginx image with the React candidate, stopped it, then started it without the candidate mount and verified the shipped Vue root/hashed main asset returned `200`; no backend or database process was started. Evidence: `docs/migrations/react/evidence/t24-rollback-local-2026-09-11.md`.
- A five-sample static comparison recorded React/Vue HTML and primary-entry TTFB, bytes, and artifact-tree sizes under the same local Nginx path. It is explicitly partial: no browser paint, memory, real first-token, or long-message result is claimed. Evidence: `docs/migrations/react/evidence/t24-static-performance-2026-09-11.md`.
- Android Expo export initially failed on the missing direct Material Symbols dependency; after adding the compatible package and refreshing the lockfile, the export passed with 1,130 modules and a 3 MB Hermes bundle. Evidence: `docs/migrations/react/evidence/t24-mobile-android-export-2026-09-11.md`.
- The initial Lite SQLite smoke found that migration version `000013` had no skill tables while the tenant-skill reaper was wired unconditionally. T15 follow-up commit `bc2b44c` adds SQLite migration `000014` for skill/catalog/snapshot/env storage; fresh startup and v4 upgrade now reach version 14 cleanly, and authenticated catalog registration/file listing pass. Sandbox provider installation, reaper behavior, and production/provider acceptance remain open; the original T24 note is retained as historical pre-fix evidence.
- T24 remains `review`: local static/typecheck/test/build, single-host Docker static smoke, candidate Nginx proxy behavior, isolated Lite backend and browser evidence are collected, but real registry/deployed Nginx, Wails installed package, complete browser/OS and role matrix, native package/runtime, performance baseline, and rollback rehearsal still need to be collected before T24 can be accepted.

### T24 root-context Compose follow-up (2026-09-11)

- Fixed `docker-compose.yml` to build `frontend/Dockerfile` with the repository root as context, matching the Dockerfile's `COPY frontend/...` and shared-package inputs.
- Extended `scripts/check-react-boundaries.mjs` to fail if Compose drifts back to `./frontend` context. Boundary check passed; `docker compose config --quiet` passed with a temporary empty `.env` (the repository did not contain one).
- This is static/configuration evidence only; it does not claim a registry push or deployed Compose runtime.

### T23 API-key live follow-up (2026-09-11)

- A fresh isolated Lite process on `127.0.0.1:18082` accepted owner registration/login, API-key create (`201`), list (`200`, one row), and revoke (`200`) for a scoped `retrieve` key. The temporary SQLite data and process were discarded/stopped after the probe; no token was recorded.
- This upgrades the API-key slice from source/bundle-only to isolated real-backend evidence, but does not complete the full server role/tenant 403 matrix or native device interaction. Evidence: `docs/migrations/react/evidence/t23-mobile-api-keys-2026-09-11.md`.

### T23 native iOS API-key follow-up (2026-09-12)

- The installed iPhone 17 Pro Release app opened the owner-only API-key screen,
  created `ios-live-probe` with scoped `retrieve`, displayed the returned
  token once, then revoked the key through the native confirmation dialog.
  The final server-backed list was empty.
- Authenticated `GET /api/v1/tenants/1/api-keys` returned `200` with an empty
  data array after revoke. Screenshot:
  `/tmp/weknora-ios-api-key-latest.png`, SHA-256
  `3daed49db47610653b638f6dec41ca53615bb7bc7df4a257b941c94a894e9b31`.
- This adds native iOS owner CRUD evidence only; non-owner 403/tenant
  isolation and production secret-management evidence remain open. Evidence:
  `docs/migrations/react/evidence/t23-mobile-api-keys-2026-09-11.md`.

### T23 native iOS administration follow-up (2026-09-12)

- The first live administration load exposed a real response-shape mismatch:
  the Lite audit row returned empty optional scope/request metadata, while the
  shared parser required every string to be non-empty. A focused test was
  observed RED (3/4 passing) against that exact row before the fix.
- Added `stringValueAllowEmpty` for string-typed optional metadata and covered
  the complete `rbac.invitation_sent` response. The focused identity suite
  then passed 4/4; the full shared suite passed 163/163.
- Rebuilt the iOS Release host and refreshed Manage → Members and audit. The
  screen rendered the owner, pending invitation, and `rbac.invitation_sent ·
  success` without an error. Screenshot:
  `/tmp/weknora-ios-admin-latest.png`, SHA-256
  `9da69973e3c35d9ae11311bd6236b6125447306749ffe11b50a8a942aabb6961`.
- This is bounded isolated Lite owner/invitation/audit read evidence; it does
  not close non-owner 403/409 permutations, physical-device behavior, or the
  remaining T23 management matrix. Evidence:
  `docs/migrations/react/evidence/t23-mobile-administration-2026-09-11.md`.

### T23 native iOS Wiki/FAQ editor follow-up (2026-09-12)

- The isolated FTS5 Lite owner probe used a local embedding mock so FAQ create
  could execute. A temporary embedding model, FAQ KB, and Wiki KB were deleted
  after the probe and the temporary server/mock were stopped.
- The iPhone 17 Pro iOS 26.5 Release app created/loaded the real FAQ and Wiki
  records, edited them natively, saved, refreshed, and retained the updated
  server values. Wiki editing now addresses the server page by `slug` rather
  than the list UUID; FAQ editing keeps its numeric server id. Evidence:
  `docs/migrations/react/evidence/t23-mobile-reference-editor-2026-09-11.md`.
- The live FAQ response exposed nullable `similar_questions` and
  `negative_questions`; the parser regression was captured RED (3/4) then
  GREEN (4/4). The reference edit-key regression was captured RED (3/5) then
  GREEN (5/5). These are contract repairs, not loosened validation for the
  required `answers` array.
- Screenshots: `/tmp/weknora-ios-faq-latest.png`, SHA-256
  `80b2a5ba2202a5eb35c0f5a3cd10318520ed2e5f5634be6c4315057d0c08e2f8`; and
  `/tmp/weknora-ios-wiki-latest.png`, SHA-256
  `92dc9a90f64f7af749041eba44e5c80288691fe0b260f0ec53840c450c994704`.
- A post-fix affected-surface rerun passed Web 64/64, Web typecheck/build
  (120 modules), iOS Expo export (1138 modules, 3.1 MB Hermes), and Android
  Expo export (1161 modules, 3.2 MB Hermes).
- Desktop 2/2 plus typecheck/build (121 modules), Embed 3/3 plus
  typecheck/build (68 modules), and `GOWORK=off go test ./internal/router`
  also passed against the shared-client change.
- A fresh isolated Lite owner/member probe created a KB share, verified owner
  and joined-member list visibility (`1`), rejected the joined member's
  source-share deletion with HTTP `403`, then accepted owner removal and
  verified the list returned `0`; temporary KB/organization data was deleted.
- This supersedes the earlier T23 table wording that listed live share removal
  as open; agent-share runtime, native organization interaction, full 403/409
  permutations, and the remaining configuration/integration/sandbox matrix
  are the outstanding boundaries for this slice.
- On the iPhone 17 Pro iOS 26.5 Release app, Manage → Organizations rendered
  the real organization and shared KB. The native Remove confirmation removed
  the share; a direct authenticated GET then returned `total: 0`. Screenshots:
  `/tmp/weknora-ios-organization-latest.png` (SHA-256
  `9653fe219413440793628a9d93046a2dde113d472f963153ffbba91ff0de708a`) and
  `/tmp/weknora-ios-organization-removed.png` (SHA-256
  `cafb3d87f8b1f4fe28071521e93112a473972930f1cc5c56fd4255f710706fea`).
- This supersedes the earlier wording that listed native organization
  interaction as open; native Android organization interaction, agent-share
  runtime, full 403/409 permutations, and the remaining configuration/
  integration/sandbox matrix remain open.
- This is bounded isolated Lite owner/native-device evidence only; non-owner
  403/409 permutations, production-provider acceptance, physical-device
  behavior, and the remaining T23 management matrix remain open.

### T24 mobile CI follow-up (2026-09-11)

- `.github/workflows/mobile.yml` now explicitly exports both iOS and Android JavaScript bundles after mobile tests/typecheck. Workflow YAML parsing, `node scripts/check-react-boundaries.mjs`, and `git diff --check` passed locally.
- This proves CI coverage is declared; it does not claim a hosted GitHub runner execution or native package/runtime acceptance.

### T24 iOS workspace refresh follow-up (2026-09-11)

- Fixed a real native runtime loop in `apps/mobile/app/(app)/workspace.tsx` and `apps/mobile/src/runtime.tsx`: the route no longer depends on the unstable Provider object, does not load before bearer restoration, workspace refreshes are single-flight, and session epochs reject late refresh results after logout or workspace switch. A bearer session with no cached memberships can hydrate them after restoration. Cold SecureStore-only restoration was not re-run in this follow-up.
- Added RED→GREEN regression coverage for route readiness, session invalidation, hydration condition and concurrent refresh behavior. `node --import tsx --test 'apps/mobile/src/**/*.test.ts'` passed 38/38; `pnpm --filter @weknora/mobile typecheck` and `git diff --check` exited 0.
- On the iPhone 17 Pro Simulator with the current worktree Metro and isolated Lite server, Workspace settled on the real `mobilet24's Workspace · owner · Current` row after 5 seconds; the native API keys screen then displayed `Workspace role: owner` and the server-backed empty state in the current authenticated runtime. This does not prove cold SecureStore-only restoration. Evidence: `docs/migrations/react/evidence/t24-ios-simulator-workspace-refresh-live-2026-09-11.md`.
- Android native runtime, complete T24 release matrix and deployment evidence remain open; T24 stays `review` and T25 remains gated.

### T24 session-race review follow-up (2026-09-11)

- Independent review of the previous Workspace patch found four concrete gaps: a refresh write could finish after invalidation, an invalidated `inFlight` promise could be reused, session adoption/workspace switching could write back after a newer session, and a session without an active tenant could retain the previous tenant. The follow-up now invalidates and detaches old refresh work, serializes/reconciles credential persistence, guards session adoption and workspace switching with their own epochs, serializes workspace-selection writes, and explicitly clears the tenant selection for no-tenant sessions.
- TDD evidence: focused auth/workspace tests passed 18/18, including an already-started credential write, invalidated in-flight refresh replacement, stale session-write restoration, and newest workspace-selection ordering. Full shared tests passed 162/162; mobile tests passed 39/39; shared/mobile typecheck and `git diff --check` exited 0.
- This addresses the identified source-level race findings, but does not change release status: no fresh independent whole-branch review has been recorded after this patch, Android native business acceptance is still incomplete, and T24 remains `review`. Evidence: `docs/migrations/react/evidence/t24-session-race-review-2026-09-11.md`.

### T24 Android native runtime attempt (2026-09-11)

- The explicit SDK path exposed `/Users/wuyongjun/Library/Android/sdk/platform-tools/adb`; AVD `test36-small` booted as `emulator-5554`, and the recorded debug APK installed and launched as `com.weknora.mobile`.
- The current worktree Metro served the Android bundle (`1327 modules`, cold bundle time about `77.8s`) after opening the Expo dev-client URL through `10.0.2.2:8082`; the native host reached the React login UI. Android displayed a system `WeKnora isn't responding` dialog during this cold dev-client load, and no authenticated login, Workspace, upload, refresh, or logout sequence was accepted.
- This is a documented runtime attempt and blocker, not Android acceptance. Evidence: `docs/migrations/react/evidence/t24-android-native-runtime-attempt-2026-09-11.md`. T24 stays `review`; T25 remains gated.

### T24 Android native release artifact follow-up (2026-09-11)

- After adding the missing direct `babel-preset-expo` development dependency, the production Android bundle completed with `:app:bundleRelease` (605 actionable tasks, exit 0). `:app:assembleRelease` then completed in 35 seconds (603 actionable tasks, exit 0), producing a 75 MB signed APK and a 52 MB AAB.
- The release APK installed on `test36-small` (`emulator-5554`) with `adb install -r` returning `Success`; `com.weknora.mobile/.MainActivity` was visible/resumed and the `com.weknora.mobile` process remained alive after launch. A bounded logcat check found no fatal exception or ANR. This is package/install/native-host startup evidence, not authenticated Android business-flow acceptance.
- The missing AndroidX artifacts were fetched through the configured online repositories after the offline attempt reported no cached versions. Evidence: `docs/migrations/react/evidence/t24-android-native-release-2026-09-11.md`. T24 stays `review`; T25 remains gated.

### T20 Android release live follow-up (2026-09-11)

- The first release login attempt consistently failed as `Network request failed` before reaching the isolated Lite backend. Root-cause evidence showed the Android merged manifest lacked cleartext HTTP permission even though the mobile server-address contract accepts HTTP(S). A red-to-green test and local Expo config plugin now add `android:usesCleartextTraffic="true"` during prebuild.
- The rebuilt release APK authenticated a temporary owner against a fresh isolated Lite SQLite backend, rendered `Knowledge bases`, `Workspace`, and the real `No knowledge bases available.` state, then restored the SecureStore session after force-stop/relaunch without re-entering credentials.
- This closes Android package, HTTP transport, password login, authenticated list, and Android cold-session evidence. It does not close Android SSE/chat, file operations, AppState/network recovery, logout, provider callback, or full role/tenant acceptance. Evidence: `docs/migrations/react/evidence/t20-android-release-live-2026-09-11.md`. T20/T24 stay `review`; T25 remains gated.

### T20 Android logout/layout follow-up (2026-09-11)

- The Android release header overflow was reproduced: `Sign out` was clipped outside the 720px viewport and could not be activated by a real tap. The title/actions layout now uses a column with a wrapping action row; `header-layout.test.ts` locks that reachability contract.
- `signOutAndRedirect` now waits for session cleanup before replacing the route with `/(auth)/login`, and does not navigate when cleanup fails. Mobile tests pass 45/45 and mobile typecheck exits 0.
- The rebuilt release APK (`sha256 06139c246cc68cd6a8504543687fe16fe5ed337c612dc58c4c7fd4a3db7aeb00`) installed successfully. A temporary owner logged in against isolated Lite, saw the real empty KB state, tapped the now-visible action, and returned to `Sign in to your workspace` / `Sign in` within three seconds. Evidence: `docs/migrations/react/evidence/t20-android-release-logout-live-2026-09-11.md`.
- Android SSE/chat, refresh/AppState/network recovery, provider callback, and complete role/tenant acceptance remain open; T20/T24 remain `review` and T25 remains gated.

### T21 Android upload/share follow-up (2026-09-11)

- Android release opened the real `DocumentsUI`, selected a pushed 65-byte text fixture, uploaded it through the authenticated multipart route, and rendered the resulting `processing` row and `File details` screen. The document detail displayed the server id, type, and size.
- `Download and share` invoked the Android Sharesheet, which showed `Sharing 1 file` and `weknora-native-upload.txt`. The Lite parser remained in `processing`, so indexing/search/content completion is not claimed.
- Evidence: `docs/migrations/react/evidence/t21-android-upload-share-live-2026-09-11.md`. T21 remains `review` pending processing completion, cancellation/large-file/permission negatives, and iOS counterpart evidence.

### T24 iOS cold SecureStore follow-up (2026-09-11)

- Built and installed the current iOS native host with `expo run:ios --device 'iPhone 17 Pro' --no-bundler --configuration Release`; the build completed with 0 errors and the app contained an embedded Hermes bundle.
- Created a temporary owner on the isolated Lite server, logged in through the native iOS form, and observed the real empty knowledge-base state.
- After `simctl terminate` plus `simctl launch` and a six-second wait, the app returned directly to `knowledge/index` without the login form or re-entering credentials. The route again rendered `No knowledge bases available.`; opening Workspace showed `ioscold_20260911's Workspace, owner · Current`.
- This closes the previously open iOS cold SecureStore-only restoration proof. T20/T24 remain `review` because Android SSE/chat and other release/role/deployment gates remain open. Evidence: `docs/migrations/react/evidence/t24-ios-simulator-cold-securestore-live-2026-09-11.md`.

### T22 Android Stop follow-up (2026-09-11)

- Added a portable stop-run seam and regression coverage for the early-stream case where no assistant message id has arrived yet. Mobile tests pass 47/47 and mobile typecheck exits 0.
- The rebuilt Android release opened the real chat route, sent `hello`, rendered `Stop`, and after a real tap returned to `Send` within 300ms; a one-second follow-up still showed `Send` and no error text. The isolated Lite stream was pending, so successful assistant token delivery is not claimed.
- Evidence: `docs/migrations/react/evidence/t22-android-stop-live-2026-09-11.md`. T22 remains `review` pending successful SSE/token delivery, remote-stop-after-id, approval/OAuth, attachment processing, steering, and iOS/device recovery evidence.

### T22 Android background/resume follow-up (2026-09-11)

- Added a pure lifecycle decision seam: an active stream is locally cancelled on background, and foreground recovery is requested only when a session exists without an owned stream. Mobile tests pass 50/50 and mobile typecheck exits 0.
- The rebuilt release APK sent `hello`, was backgrounded with Android Home while the Lite stream was pending, and returned to the chat route with the message retained and `Send` restored; no duplicate local send, `Stop`, or error text appeared during the bounded observation.
- Evidence: `docs/migrations/react/evidence/t22-android-background-resume-live-2026-09-11.md`. T22 remains `review` pending server continuation after interruption, successful SSE/token delivery, remote-stop-after-id, approval/OAuth, attachment processing, steering, and iOS/device recovery evidence.

### T22 Android successful SSE follow-up (2026-09-11)

- Added the selected knowledge-base scope to the native chat request and
  prevented a completed assistant answer from being rendered twice after the
  history reload. Mobile tests pass 53/53 and mobile typecheck exits 0.
- The current Android release APK (`sha256
  6aad5e918fe0bd1c646afe6f0956680ef38547d20b863e3f08abbacbe04218c5`) ran on
  `test36-small` API 36, authenticated against isolated FTS5 Lite on port
  `18084`, selected the real `Probe KB`, and completed `hello` through the
  native UI. The final accessibility dump contained one user `hello`, one
  assistant `Hello from Android`, no `Stop`, and no error alert.
- A direct authenticated probe also recorded the full Lite SSE sequence from
  `agent_query` through incremental answer tokens to `complete`, using a
  local OpenAI-compatible mock model. This is bounded native/isolated-runtime
  evidence; production-provider behavior, iOS SSE, server continuation,
  remote stop after assistant-id, approval/OAuth, attachment processing,
  steering, and network/device recovery remain open.
- Evidence: `docs/migrations/react/evidence/t22-android-sse-live-2026-09-11.md`.
  T22 and T24 remain `review`; T25 remains gated.

### T22 iOS native chat entry follow-up (2026-09-11)

- Added a native `Chat` action to the authenticated knowledge-base header and
  locked its `/chat` target with a focused regression test.
- The iPhone 17 Pro iOS 26.5 Release build completed with 0 errors, installed
  successfully, and a real tap on `Chat` transitioned from `knowledge/index`
  to `chat/index`, rendering the native composer and `Send` control.
- The isolated server restart invalidated the temporary persisted bearer, so
  the resulting 401 was recorded rather than treated as an iOS SSE pass.
- Evidence: `docs/migrations/react/evidence/t22-ios-chat-entry-live-2026-09-11.md`.

### T22 iOS authenticated SSE follow-up (2026-09-11)

- With the same iPhone 17 Pro iOS 26.5 Release app and isolated FTS5 Lite
  server, restarted a local OpenAI-compatible SSE mock and logged in through
  the native form. The real `Probe KB` was selected, `hello from ios` was
  sent, and the screen rendered one completed `Hello from iOS` assistant
  answer with `Send` restored and no duplicate/error.
- SQLite rows for session `d1f41b92-b616-43d0-896e-d69ce7c89f99` contain the
  completed user and assistant messages; authenticated session listing
  returned the same selected KB scope. Screenshot:
  `/tmp/weknora-ios-chat-latest.png`, SHA-256
  `9e6cf4455037a98a0cc81c0ce9af6ea0949cba7ce4569bfcd844a74a4c94ec88`.
- This closes the bounded iOS native SSE/token path only. It remains isolated
  mock-provider evidence; approval/OAuth, attachments/artifacts, steering,
  background continuation, remote stop-after-id, physical device and
  production-provider acceptance remain open. Evidence:
  `docs/migrations/react/evidence/t22-ios-chat-entry-live-2026-09-11.md`.

### T22 iOS native Stop follow-up (2026-09-11)

- A delayed local OpenAI-compatible stream exposed the native `Stop` control
  during an active request. Tapping it returned the composer to `Send`, kept
  the user message, and left the server-backed assistant row explicitly
  incomplete; the UI showed `Resuming…` for that row.
- Screenshot: `/tmp/weknora-ios-chat-stop-latest.png`, SHA-256
  `6b6e07c836c2197e1c1c8f7590bc4838673e8044744423968ab3f78a6cfa59dc`.
- This is bounded cancellation evidence only; the delayed fixture did not
  persist an assistant id before stopping, so remote stop-after-id and
  continuation remain open. Evidence:
  `docs/migrations/react/evidence/t22-ios-chat-entry-live-2026-09-11.md`.

### T24 full Go regression (2026-09-11)

- `GOWORK=off go test ./...` exited `1`. Migration-relevant backend packages, including `internal/router`, passed; the complete run has two environment-sensitive groups: the Python skill verifier fixture sees host-installed packages that the test expects to be absent, and Notion/Azure/OpenAI fixture hostnames resolve to the SSRF-reserved `198.18.0.0/15` range. No test was skipped or weakened. Evidence: `docs/migrations/react/evidence/t24-go-regression-2026-09-11.md`.

### T23 mobile data-source follow-up (2026-09-11)

- Added the native knowledge-base data-source inventory route. It uses shared list/type seams, keeps connector `config` and credentials out of the UI, and exposes no mobile write/sync controls.
- Mobile tests are 33/33, typecheck passes, and both iOS/Android Expo exports pass. This remains source/bundle evidence; live connector and native device acceptance are not claimed. Evidence: `docs/migrations/react/evidence/t23-mobile-data-sources-2026-09-11.md`.
- The capability matrix now also explicitly discloses system runtime queue/task controls as unsupported on mobile instead of leaving that administration row implicit.

### T25 retirement preflight (2026-09-11)

- Read-only scan recorded in `docs/migrations/react/evidence/t25-retirement-preflight-2026-09-11.md`.
- `frontend/src`, the Vue dependency graph, legacy Docker/Nginx workflows, Lite/desktop fallback branches, and legacy verification scripts still have active references. `cmd/desktop/wails.json` targets the React desktop renderer, but that does not make the old Lite release input disposable.
- T25 remains `pending`; no Vue source or fallback artifact was deleted. T24 acceptance, independent old-artifact retention, installed Wails rollback, complete browser/role/tenant/deployment matrix, and iOS/Android runtime gates remain prerequisites.

### T20 mobile refresh race follow-up (2026-09-12)

- The mobile JSON transport now captures the credential used for the initial
  request. If a concurrent request has already rotated the bearer after a
  `401`, the retry reuses that newer credential and does not invoke the refresh
  coordinator a second time. The existing rules remain unchanged: only
  idempotent reads retry, write requests are not replayed, and transition-time
  requests do not refresh.
- TDD RED reproduced the duplicate-refresh path by rotating the credential
  between the initial `401` and retry. GREEN is `node --import tsx --test
  apps/mobile/src/platform/transport.test.ts` 6/6; the full mobile suite is
  57/57, mobile typecheck and `git diff --check` exit 0. Commit: `3275e78`.
- This is source-level concurrency evidence only; real refresh-token rotation,
  provider/OIDC, and device network-recovery evidence remain open. T20/T24
  stay `review`; T25 remains gated.

### T21 iOS native picker follow-up (2026-09-12)

- The current iPhone 17 Pro / iOS 26.5 Release host logged into an isolated
  Lite account, opened a real knowledge base, reached the native `Upload`
  control, and opened the system Files picker. After exposing a 26-byte TXT
  fixture through the Simulator Files provider, selection returned to WeKnora;
  the list showed `processing`, and the detail screen showed the server id,
  `Type: txt`, and `Size: 26`.
- `Download and share` opened the native iOS share panel with the document name
  and size plus system actions. The isolated Lite parser left the document in
  `processing` and the UI remained `Preparing...`, so completed parsing and
  protected download bytes are not claimed.
- Evidence: `/tmp/weknora-ios-upload-picker.png`, SHA-256
  `d08bafa8bf82bcda5849963089aeabc3a39c8756c20c0b549e13a9ae397f111a`, and
  `/tmp/weknora-ios-upload-share.png`, SHA-256
  `6d629b903414d45351fcc517f38020598b7d84b9ff58351cacfaef7117f2794c`; full
  record: `docs/migrations/react/evidence/t21-ios-upload-picker-attempt-2026-09-12.md`.
  T21 remains `review` pending parser completion, download-byte/filename
  verification, cancellation/large-file negatives, and real-device evidence.

### T21 iOS completed upload/download/search follow-up (2026-09-12)

- A fresh isolated Lite run with a local OpenAI-compatible embedding stub used
  the current iPhone 17 Pro / iOS 26.5 Release host to select a 144-byte TXT
  fixture through the real Files picker. The document transitioned from
  `processing` to `completed`; the authenticated list/detail responses showed
  `pending_subtasks_count=0`, and the embedding stub recorded HTTP 200.
- Hybrid search returned the uploaded fixture content. Authenticated download
  returned HTTP 200 with the expected filename and bytes/hash; a separate
  isolated viewer received HTTP 403 without file bytes. The native download
  action opened the iOS Share Sheet with the document name and size.
- Full record: `docs/migrations/react/evidence/t21-ios-upload-complete-download-live-2026-09-12.md`.
  This closes the iOS completion/download/search slice, but T21 remains
  `review` pending same-fixture Android completion, cancellation/large-file/
  413 negatives, real-device and production-storage evidence.

### T03/T06/T17 React Web live follow-up (2026-09-12)

- In a fresh isolated SQLite Lite run on `127.0.0.1:18082`, Chrome tab
  `948799908` used the React Web dev server at `127.0.0.1:5175`. Registration
  returned `201`, login returned `200`, and the React route navigated to
  `/platform/knowledge-bases` after the shared `auth.me` hydration.
- The authenticated React knowledge-base surface created `React live
  knowledge base` (`POST /api/v1/knowledge-bases`, `201`), rendered the server
  row, edited it to `React live knowledge base updated` (`PUT
  /api/v1/knowledge-bases/<server-id>`, `200`), and reloaded the list from the
  backend. The rendered scope key included origin, user, tenant, and resource.
- Direct navigation to `/platform/settings?section=tenant` rendered tenant
  `1`, status `active`, and the owner capability labels. Saving
  `React Web live settings current` produced `PUT /api/v1/tenants/1` HTTP
  `200`; the page rendered the server-returned description and timestamp.
- Evidence: `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md`.
  Fresh `GET /health` and React root probes returned HTTP `200`.
- This is real browser plus isolated backend evidence, not mock evidence. It
  strengthens T03/T06/T17 but does not close their remaining refresh/OIDC,
  document/full role matrix, or other-settings gates; all three remain
  `review`. The temporary database and account are outside the repository.

- The same authenticated browser deep-linked to `/platform/chat/session-1`;
  React mounted the chat route and rendered the server-backed `session not
  found` error for the intentionally nonexistent ID. The Web bundle's
  `/embed/channel-1` path rendered its explicit isolated-entry guard, while a
  separate Embed dev entry at `127.0.0.1:5176/embed/channel-1` rendered
  `Missing embed token.` without entering the Web login flow.
- This adds real browser route/error-state and independent Embed-entry
  evidence, but does not claim a valid third-party Embed session or a real
  chat generation. T10/T11/T18 remain `review`. The expanded record is in
  `docs/migrations/react/evidence/t06-t17-web-live-2026-09-12.md`.

### T24 React bundle and route regression rerun (2026-09-12)

- Fresh baseline generation and tests passed: baseline unittest 4/4, shared
  164/164, Web 64/64, Embed 3/3, desktop 2/2, and mobile 57/57; all five
  typechecks, the React boundary check, and `git diff --check` exited 0.
- Fresh production builds passed for Web, Embed, the combined React Web/Embed
  candidate, and the desktop renderer. `go test ./internal/router -run
  'TestFrontendStatic' -count=1` also exited 0. Candidate artifact sizes and
  module counts are recorded in
  `docs/migrations/react/evidence/t24-react-bundle-rerun-2026-09-12.md`.
- This strengthens static/release-input evidence only. T24 stays `review`
  for the remaining deployed/cross-OS/performance/full-matrix gates; T25
  stays gated and no Vue source was removed.

### T24 current React candidate Nginx rerun (2026-09-12)

- Rebuilt the current React Web/Embed candidate (`BUILD_INFO.json` reports
  `renderer: react`, commit `f231d1c`) and ran `docker build --check` plus a
  current-source Docker image build successfully.
- Mounted that candidate into the local Nginx image and asserted HTTP 200 for
  the Web root, Web chat deep-link, and dedicated Embed fallback. Hashed Web
  and Embed assets both returned the required immutable cache header. The
  test container was removed afterward and had no backend/data dependency.
- Evidence: `docs/migrations/react/evidence/t24-react-nginx-rerun-2026-09-12.md`.
  This is static/proxy evidence only; T24 remains `review` and T25 remains
  gated.

### T24 release artifact fail-closed follow-up (2026-09-12)

- Added `scripts/validate-release-lite-artifacts.sh` and wired it before
  `gh release create` in `.github/workflows/release-lite.yml`. It requires
  every Web CLI and desktop matrix artifact plus the React candidate globs,
  requires exactly one React candidate archive, extracts it, and validates React
  `BUILD_INFO.json`, Web/Embed entries, both entry asset directories, and all
  SHA-256 sidecars.
- Removed the release command's `|| true`, so an upload or artifact failure
  now fails the release job before `update-homebrew` can run. The unrelated
  Windows pre-compile tolerance remains outside the publish step.
- `scripts/test_validate_release_lite_artifacts.sh` passes its valid fixture
  plus missing-checksum and non-React-`BUILD_INFO.json` negative fixtures;
  `bash -n`, workflow YAML parsing, and `git diff --check` also pass. No
  release, registry push, Homebrew push, or production service was run.
- This hardens publication safety but does not make the React candidate the
  production Lite artifact; T24 remains `review` and T25 remains gated.

### T24 React Lite CLI package follow-up (2026-09-12)

- Fixed `REACT_FRONTEND=1` in `scripts/package-lite.sh` and `Makefile` to use
  the combined React Web/Embed bundle rather than copying Web-only `dist`.
- A real local macOS arm64 `REACT_FRONTEND=1 ./scripts/package-lite.sh`
  verification exited 0 and produced an 81 MB tarball. Its `web/` contains
  `index.html`, `embed.html`, `BUILD_INFO.json`, `assets/`, and
  `embed/assets/`; `BUILD_INFO.renderer` is `react`, and the checksum check
  returned `OK`.
- Evidence: `docs/migrations/react/evidence/t24-lite-react-cli-package-2026-09-12.md`.
  This closes the local CLI packaging copy gap only; T24 remains `review`.

### T24 React Docker candidate follow-up (2026-09-12)

- Added the separate root-context `apps/web/Dockerfile` for the React
  Web/Embed candidate and tightened `.dockerignore` to exclude native/build
  outputs. `docker build --check` passed with no warnings; the actual image
  build passed after the context was reduced from about 4.8 GB to about 24 KB.
- The local container returned Web root, Web chat deep-link, and Embed route
  HTTP 200 responses, and served React `BUILD_INFO.json` with commit `f231d1c`.
  The container was removed afterward and no backend, registry, or production
  service was used.
- Evidence: `docs/migrations/react/evidence/t24-react-docker-candidate-2026-09-12.md`.
  This is candidate static/runtime evidence; the Vue Docker image remains the
  production/recovery input and T24 remains `review`.

### T20/T24 Android NetInfo network-recovery follow-up (2026-09-12)

- Added the platform-neutral offline→online edge detector and wired the
  foreground authenticated-session recovery hook to the native
  `@react-native-community/netinfo` subscription. Unknown reachability is now
  held pending until NetInfo confirms `true`, with a regression test for the
  transient `null` state. The focused test passed 4/4, the full mobile suite
  passed 61/61, mobile typecheck and Android Expo export passed, and the
  Android release build completed successfully with the native NetInfo module
  linked.
- On the `test36-small` Android API 36 emulator, the current release APK
  authenticated against an isolated Lite server, then survived disabling and
  re-enabling Wi-Fi and mobile data while foregrounded. Connectivity changed
  from no active network to a connected/validated network; the final
  current-HEAD Lite log recorded a real HTTP 200 `POST /api/v1/auth/refresh` at
  `03:32:11.982`, and the authenticated `Knowledge bases` route remained
  visible with no login or error state.
- Full record: `docs/migrations/react/evidence/t24-android-network-recovery-live-2026-09-12.md`.
  This closes the Android emulator foreground network-recovery slice, but T20
  and T24 remain `review` for the broader iOS/network hardware, deployed,
  cross-OS, provider, role/tenant, performance, and browser acceptance matrix.

### T21 Android completed upload/download/search follow-up (2026-09-12)

- A current React Android release APK on `test36-small` used the real
  DocumentsUI to select a 144-byte TXT fixture. The authenticated native list
  rendered `android-t21-complete.txt, completed`; the backend returned
  `parse_status=completed` and `pending_subtasks_count=0`, and the detail page
  rendered the server document ID, `Type: txt`, and `Size: 144`.
- The isolated Lite run used `go run -tags sqlite_fts5` plus a local
  OpenAI-compatible embedding stub. The embedding request returned HTTP 200;
  hybrid search for `completed upload fixture` returned one result whose
  content matched the uploaded fixture. Authenticated download returned HTTP
  200 with the expected filename and 144 bytes; the downloaded SHA-256 matched
  the fixture exactly.
- Native `Download and share` opened Android's Sharesheet, which showed
  `Sharing 1 file` and `android-t21-complete.txt`. The current Android and
  iOS completed-fixture runs therefore have matching 144-byte upload,
  processing, search, and download-byte evidence. Full record:
  `docs/migrations/react/evidence/t21-android-upload-complete-download-live-2026-09-12.md`.
- T21 remains `review`: cancellation, large-file/413 negatives, Android
  cross-account 403, real-device, production-storage, and full Web/mobile
  same-account comparison evidence remain open.

### T21 Android native transport and negative follow-up (2026-09-12)

- The current Android release now routes picked native files through the
  cancellable Expo FileSystem multipart task. A real DocumentsUI selection of
  the 144-byte TXT fixture returned HTTP 200 from the isolated Lite handler and
  produced a server-backed 144-byte document row.
- A real 10 MB DocumentsUI selection exposed `Cancel`; tapping it returned the
  page to `Upload` without surfacing a network error. The controlled relay
  showed that bytes already queued in Android/relay may still reach the server,
  so this is client-side cancellation evidence only and does not claim server
  rollback. A 1.10 MB fixture showed `文件大小不能超过1MB`; the isolated
  handler returned HTTP 400, not the separate ingress-level HTTP 413 contract.
- The same 1,100,000-byte native multipart shape was replayed through a
  controlled local ingress: it returned HTTP 413 `Payload Too Large` at
  `Content-Length=1,100,249`, and the backend row count stayed at 2. The
  Android selection attempt was also logged as 413, but its follow-up list
  refresh hit an expired-session HTTP 401, so this is transport/ingress
  evidence rather than complete native 413 UI acceptance. This is not
  production reverse-proxy evidence.
- A separate tenant-2 account attempted the owner document download and
  received HTTP 403 with `Permission denied to access this knowledge base`
  and no file bytes. This is isolated backend permission evidence, not a
  second-account native UI run.
- Full record: `docs/migrations/react/evidence/t21-android-native-upload-cancel-live-2026-09-12.md`.
  T21 remains `review` pending server-side cancellation semantics, production
  413 ingress, Android second-account UI 403, real-device and
  production-storage evidence, and the complete Web/mobile comparison matrix.

### 本轮问题与裁定

- 计划/设计原文仍写“方案 B 尚未批准”，与用户本轮批准相冲突；本实施分支按用户批准的方案 B 执行，未因旧措辞改变架构。
- Multica 根许可证含 hosted/commercial/branding 附加条件；本轮仅采用分层模式，未复制其产品源码或品牌资产。
- T02 reviewer 发现并已修复 Embed 裸 token、租户头泄露和缺失 session/visitor header；修复提交为 `19dede6`，修复后验证 11/11。
- 后续 reviewer 仍发现：`createRefreshCoordinator` 尚未接入 `client.request` 的 401 重试；真实 OIDC/刷新并发、租户切换 UI 生命周期和 Embed 独立入口仍未闭合，继续保持 `review`。
- `15889e1` 修复 auth logout 的 `success:false`、动态凭证读取、Embed 路由隔离和 `/platform`/`creatChat` 兼容路径；`0fe0e4b` 刷新基线至当前 HEAD。
- 当前 worktree 保留三份用户提供的未跟踪权威输入文件，未修改、未清理、未提交。

### T07 review evidence (2026-09-11)

- `4ecd4c1` + `dcff3fa` + `29c6d5d` add the React Web document list/detail slice, the unknown-status safety fix, and the shared boundary-guard fix. The shared client now exposes strict file multipart, URL/manual source, folder move/rename, batch tag update, reparse/cancel, single/batch delete, detail, protected preview and download paths. Native file references remain platform-owned `NativeFileSource` values and are only appended at the multipart boundary.
- The Web list is reachable from a knowledge-base card at `/knowledgeBase/:knowledgeBaseId`; document deep links use `/knowledgeBase/:knowledgeBaseId/documents/:documentId`. It loads folder trees and tags, forwards keyword/status/tag/folder pagination filters, preserves multi-page selection, exposes move/tag/delete bulk actions, supports file/URL/manual creation, aborts file uploads, and surfaces a typed 413 message. Detail state refuses to call a document preview ready unless the authoritative parse status is `completed`.
- TDD RED evidence included missing-module/absent-export failures for folders, preview, route detail and mutation methods; GREEN focused tests then passed. Full verification passed: `pnpm test:shared` 142/142, `pnpm test:web` 37/37, `pnpm typecheck:shared`, `pnpm typecheck:web`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0.
- Evidence remains `review`: no live backend upload→processing→search→reference/download chain, browser interaction capture, protected binary response/filename verification, actual upload progress telemetry, or per-format PDF/image/Markdown/CSV/XLSX/DOCX/PPTX renderer acceptance was available. The UI intentionally does not claim those runtime gates.

### T08 review evidence (2026-09-11)

- `1d08229` adds a React Wiki route at `/knowledgeBase/:knowledgeBaseId/wiki` with server-backed search, page creation, page editing, preserved optimistic `version`, and a reload-latest action after HTTP 409 conflicts. `e1070b1` adds a strict FAQ client for the nested list envelope and create/update/upsert/field/tag/batch-delete routes. The existing shared Wiki API and pure line/revision diff remain the source of truth; no Vue component is imported.
- TDD RED evidence covered the missing editor and FAQ modules; GREEN focused tests passed 3/3 for Wiki and 2/2 for FAQ, and Web/shared typecheck/build passed. The full shared/Web verification recorded above remained green after this slice.
- The follow-on UI slice adds `/knowledgeBase/:knowledgeBaseId/faq`, FAQ create/edit, CSV/JSON append-or-replace import, CSV/JSON export, enable/recommend/tag batch actions and batch delete. The Wiki route now loads historical revisions, fetches a full snapshot, renders title/summary/content diffs through the shared pure algorithm, and requires explicit confirmation before server-side revert.
- Focused GREEN tests pass for CSV/JSON normalization and round-trip parsing, the FAQ search/export client, Wiki revision retrieval, and the FAQ route. `pnpm test:web` passes 42/42, `pnpm test:shared` passes 146/146, Web typecheck/build pass, and `git diff --check` passes. Shared typecheck and boundary checks remain to be rerun after this working-tree slice.
- T08 remains `review`: live backend version conflict/revert, import task progress/failure display, real export/download response headers, permission negatives, and browser acceptance are not available. The UI does not infer those gates from mock tests or production bundling.

### T09 review evidence (2026-09-11)

- `c94ee50` adds the React data-source route and extends the shared client with strict connector metadata parsing, credential subresource writes, and sync-log reads. Connector type metadata follows the actual backend `ConnectorMetadata` DTO rather than assuming a string list; list/logs preserve array-versus-raw response shapes while rejecting malformed rows.
- The settings route separates credentials from connector settings, validates entered credentials before mutating a data source, preserves configured credentials when an edit leaves the secret field blank, and keeps sync requested versus sync completed distinct through the logs panel. Form parsing is tested first with key-value credentials/settings, duplicate/malformed lines and explicit payload assertions.
- Full static verification after the slice passed: `pnpm test:web` 44/44, `pnpm test:shared` 147/147, shared/Web typechecks, Web production build, React boundary check and `git diff --check` all exited 0.
- T09 remains `review`: no real connector credential, permission-negative, sync completion/failure, resource picker, advanced KB settings endpoint, or browser acceptance evidence is available. The route does not claim those unsupported sections are migrated.

### T09 advanced-settings increment evidence (2026-09-11)

- `6da05c6` adds `packages/api-client/src/knowledge/settings.ts` and exposes it under `client.knowledge.settings` / `client.knowledgeBases.settings`. The seam strictly parses KB get/update envelopes, raw `code: 0` parser-engine responses, chunk-preview diagnostics, storage/vector inventories and cursor activity; malformed rows reject instead of becoming empty settings.
- `apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx` adds concrete General, Parser engines, Chunking/debug preview, Indexing strategy, Storage/vector binding, Feature flags and Activity sections. The route keeps data-source management available in a sibling tab. It uses field-level controls and does not replace the legacy settings with a generic JSON editor.
- The update payload contains only fields accepted by the current `KnowledgeBaseConfig` contract. Vector-store and storage bindings are visibly read-only after creation; question generation is shown as read-only because the current update DTO does not accept it. Save errors leave the edited form intact; parser availability and chunk preview results remain distinct from connection/sync health.
- Fresh verification after this increment: `pnpm test:shared` 149/149, `pnpm test:web` 46/46, `pnpm typecheck:shared`, `pnpm typecheck:web`, `pnpm build:web`, `node scripts/check-react-boundaries.mjs`, and `git diff --check` all exited 0. This is static/Node mock and production-bundle evidence only; live endpoint, role/403, connector sync, share mutation and browser acceptance remain open, so T09 stays `review`.

### T17 settings resource/config increment (working tree, 2026-09-11)

- Added concrete React Web controls for Storage, Vector Store, Web Search, Retrieval, Chat History, Parser Engine, Ollama, and WeKnora Cloud. Resource rows support create/update/delete/test and supported default-storage selection; KV forms use field-level controls; Ollama exposes status, model inventory, download initiation and progress refresh; Cloud credentials are write-only in the UI.
- Secret-shaped values are not prefetched into editable fields. Blank parser API keys are omitted from update payloads, and Cloud/Ollama forms clear sensitive input after successful writes.
- TDD RED→GREEN covered resource row/input validation, retrieval/chat-history/parser field payloads, Ollama model names, and Cloud credential requirements. The current Web test suite passes 61/61; Web typecheck and production build pass.
- Isolated Lite owner probes returned HTTP 200 for ten settings reads and HTTP 200 for retrieval, chat-history, parser writes plus parser-engine check. Evidence: `docs/migrations/react/evidence/t17-settings-resource-live-2026-09-11.md` and `docs/migrations/react/evidence/t17-config-live-2026-09-11.md`.
- T17 remains `review`: no real external provider credentials or model download was used, browser interaction/permission-negative coverage is incomplete, and native/Lite capability filtering is not yet accepted.

### T18 integrations management/live increment (working tree, 2026-09-11)

- The integrations route now has concrete Embed and IM forms/actions, including create, update, delete, rotate/toggle, and preview wiring. API Principal configuration is Owner-only in the UI, HMAC secrets are write-only, short-lived external-user test tokens are generated through the server, and the SSE playground keeps API keys only in memory.
- Embed `PUT` uses full-row update semantics in the current Go handler. The React update helper now preserves all non-secret list fields and rejects an unavailable origin allowlist; regression tests cover both preservation and fail-closed behavior.
- Real isolated Lite evidence: Embed public config `200` without Bearer/Cookie; allowed-origin exchange reached the server but returned explicit `503 session tokens unavailable` because Redis is absent; forbidden origin returned `403 origin not allowed`; owner Embed CRUD, IM toggle, Principal signed-token config, and 900-second test-token generation returned success.
- `pnpm test:shared` passes 151/151, `pnpm test:web` 61/61, shared/Web typechecks and builds pass, Embed test/typecheck/build pass, boundary check and diff check pass. T18 remains `review` because no real third-party iframe proxy, provider-backed SSE, IM callback, browser E2E, or full role-negative matrix was available.

### T19/T20 renderer/mobile rerun (working tree, 2026-09-11)

- Desktop typecheck/tests passed. Desktop production build initially failed because its Vite alias table did not map the shared Wiki/knowledge/chat/sandbox subpaths; the alias table was corrected and the build then passed with 121 transformed modules.
- Mobile typecheck passed, mobile tests passed 6/6, and Expo iOS/Android Hermes exports plus public config passed. These remain export evidence only; no native installation, device runtime, live auth, or network recovery is accepted.

### T10/T11 React Web live SSE follow-up (2026-09-12)

- In a fresh isolated Lite run, a temporary owner account logged in through
  the React Web form, created a session through `/platform/creatChat`, and
  sent `hello from React Web SSE`. The page rendered the streamed assistant
  answer `React live SSE answer`, cleared `Sending…`, and returned the send
  control to its enabled state.
- The backend emitted `agent_query`, incremental `answer`, and `complete`
  SSE events. A local OpenAI-compatible model stub recorded the streaming
  `/v1/chat/completions` request and four content chunks. A fresh authenticated
  history read returned four completed rows in `user, assistant, user,
  assistant` order, including the React turn.
- This is real backend plus browser evidence against a local model stub; it
  is not third-party provider, RAG, Agent-tool, recovery, or native evidence.
  The browser-use Node runtime could not load its host-native dependency, so
  the visible browser steps used the connected browser control surface; the
  distinction is recorded in `docs/migrations/react/evidence/t10-t11-web-sse-live-2026-09-12.md`.
- T10/T11 remain `review`: `Last-Event-ID` continuation, cancellation
  timeout, real RAG/Agent generation, tool approval/OAuth/steer, and native
  chat lifecycle are still open.

### T07 React Web upload follow-up (2026-09-12)

- An isolated account created a tenant-local knowledge base through the visible
  React Web flow and reached its document route. The route rendered File/URL/
  Manual source controls, folder root, status and tag filters, and the upload
  action.
- The connected in-app browser could expose the file input but could not drive
  its native chooser with a local path. The UI consequently showed the
  authoritative `Choose a file before uploading.` validation state. The same
  fixture was then submitted through the documented multipart API so the
  backend lifecycle could be observed without overstating chooser coverage.
- An unbound KB correctly stayed processing and logged the missing embedding
  model. A second isolated KB with a local OpenAI-compatible embedding model
  accepted the upload; the authenticated list returned
  `parse_status=completed`, `pending_subtasks_count=0`, and no document error.
  The visible React Web list rendered `Root · txt Completed`; the embedding stub
  recorded a successful `/v1/embeddings` request.
- T07 remains `review`: native chooser submission, progress telemetry,
  protected preview/download, search/reference rendering, and per-format
  renderer acceptance remain open. Full evidence is in
  `docs/migrations/react/evidence/t07-web-upload-live-2026-09-12.md`.

### T14 Web terminal and route/runtime follow-up (2026-09-12)

- `5431f30` adds the Web sandbox terminal panel and ticket-backed WebSocket controller. It uses binary PTY input/output, handles JSON terminal control frames, validates resize bounds, caps retained output, observes panel size for bounded PTY resize frames, and ignores stale events after close or session changes. The controller derives the WebSocket origin and deployment sub-path from the configured API base and never adds a bearer token to the URL.
- The same slice makes knowledge-base detail/chat deep links, `/platform/agents`, root/search redirects, `/platform/system/*`, and explicit 404 handling visible in route dispatch. React Web and Embed now load the runtime `config.js` contract and a bundled favicon in the candidate artifact.
- Verification: Web tests 94/94, Web/shared typechecks, Web build (127 modules), Embed build (68 modules), React boundary check, and diff check passed. Evidence: `docs/migrations/react/evidence/t14-web-terminal-route-runtime-2026-09-12.md`.
- T14 remains `review`: deterministic fake-socket tests do not prove real shell I/O/resize, provider paused/no-sandbox behavior, cross-tenant live switching, or browser E2E.

### T16 Web organization actions follow-up (2026-09-12)

- The organization detail now loads typed member, pending join-request, and shared knowledge-base data. It exposes confirmation-gated approve/reject, leave, and delete actions and only clears/reloads local state after a successful server response; failures leave the server-backed view intact.
- Web tests remain 94/94, Web typecheck and diff check pass. Evidence: `docs/migrations/react/evidence/t16-web-organization-actions-2026-09-12.md`.
- T16 remains `review`: role-specific 403/409, browser, live share mutation, and full cross-tenant/system-admin matrix evidence remain open.

### T24 React cross-client rerun (2026-09-12)

- Source commit `3823ad6` rebuilt a React Web/Embed candidate whose `BUILD_INFO.json` identifies the React renderer and whose artifact contains the runtime config and favicon assets; later route assertions are in `46bc313`. Shared 180/180, Web 94/94, Mobile 61/61, Desktop 2/2, and Embed 3/3 tests passed; all corresponding typechecks, Web/Embed builds, boundary/diff checks, and release preflight scripts passed.
- This remains local static/Node/bundle/package evidence. Installed Wails Windows/Linux behavior, complete device and browser/role matrix, registry/deployed proxy behavior, provider-backed flows, and production rollback remain open, so T24 stays `review` and T25 remains gated.

### T07 Web authenticated preview follow-up (2026-09-12)

- `369ddb8` adds authenticated binary preview/download transport and removes
  the unauthenticated anchor download path. Safe text/Markdown, image, and PDF
  preview rendering is backed by cleaned-up Blob URLs; unsupported formats are
  explicit download-only. Shared 182/182 and Web 96/96 tests plus both
  typechecks passed. Evidence: `docs/migrations/react/evidence/t07-web-authenticated-preview-2026-09-12.md`.
- T07 remains `review` pending live protected-byte/browser download and full
  format-renderer evidence.

### T07 Web authenticated preview transport review fix (2026-09-12)

- During review, the Web browser transport was found not to expose the shared
  client's optional `sendBinary`, so the authenticated preview implementation
  could not run through the real Web client. The transport now delegates
  binary requests through the same scoped headers and bearer refresh/retry
  boundary as JSON and SSE requests.
- TDD red/green and fresh verification are recorded in
  `docs/migrations/react/evidence/t07-web-authenticated-preview-transport-2026-09-12.md`:
  focused 9/9, Web 97/97, shared 183/183, Web/shared typechecks, Web build,
  boundary check, and diff check exited 0.
- Live protected bytes/download filename and full browser/format coverage are
  still missing, so T07 remains `review`.

### T20 mobile OIDC code-exchange follow-up (2026-09-12)

- `708132a` adds the allowlisted `weknora://oidc` callback target, signed
  frontend binding, short-lived provider-code handoff, native exchange route,
  and mobile fail-closed callback parsing. OIDC/API-client focused tests,
  handler tests, and mobile typecheck passed. Evidence:
  `docs/migrations/react/evidence/t20-mobile-oidc-code-exchange-2026-09-12.md`.
- T20 remains `review` pending external IdP/provider callback and physical
  device deep-link evidence.

### T22 mobile run-lifecycle follow-up (2026-09-12)

- `2d72d0f` adds per-session SecureStore-backed run lifecycle state and wires
  ChatScreen/AppState recovery so user Stop, failure, and completion cannot be
  silently resumed while background interruption remains resumable. Mobile
  71/71 and typecheck passed. Evidence:
  `docs/migrations/react/evidence/t22-mobile-run-lifecycle-2026-09-12.md`.
- `d884340` adds a lifecycle revision guard so a late SecureStore hydration
  result cannot overwrite a newer local transition. Current mobile 75/75 and
  typecheck passed; existing user-stopped/failed/completed states remain
  non-resumable.
- T22 remains `review` pending provider/device continuation, approval/OAuth,
  and remote stop-after-id runtime evidence.

### T23 mobile capability matrix follow-up (2026-09-12)

- `a56ab45` makes each management capability mode/reason/required roles
  explicit and keeps non-native entries non-actionable in the Hub. Management
  11/11, mobile 71/71, and mobile typecheck passed. Evidence:
  `docs/migrations/react/evidence/t23-mobile-capability-matrix-2026-09-12.md`.
- T23 remains `review` pending full role/tenant/device/provider acceptance.

### T07 Web protected download follow-up (2026-09-12)

- Using the connected Chrome session against the isolated Lite/Web fixture, the
  protected document detail route showed the completed Markdown preview and
  `Download Private preview fixture.md` action. Clicking it created
  `/Users/wuyongjun/Downloads/Private preview fixture.md` with 35 bytes and
  SHA-256 `7e0692730b4e720619c720841333d1a742a38cf2bb7ccc5036d673107671ea0c`,
  matching the authenticated preview response.
- This closes the browser download byte/filename check for the Markdown
  fixture. Native chooser upload, search-to-reference/download, and all
  legacy format renderer variants remain open; T07 stays `review`.

### Web canonical list route follow-up (2026-09-12)

- Browser reproduction showed `/platform/knowledge-bases` passed the auth and
  tenant guards but fell through to `NotFoundPage` because the protected
  renderer omitted the list-page branch. `549bed8` adds the explicit route
  mapping and mounts the existing `KnowledgeBasesPage`.
- TDD evidence: the new route-to-page regression first failed with the missing
  export, then passed after the minimal mapping and renderer wiring. Fresh Web
  tests passed 98/98; Web typecheck/build, React boundary check, and diff
  check all exited 0. Chrome then rendered the live Knowledge bases page and
  loaded the fixture row from the isolated Lite backend.

### Web knowledge-base deep-link follow-up (2026-09-12)

- The live FAQ fixture exposed a second route bug: `/knowledgeBase/:id` was
  classified as a knowledge-base route but dropped its identifier, so the
  renderer could not mount the document page. `c3b9abb` preserves the decoded
  `knowledgeBaseId` and keeps `/knowledgeBase` as the list route.
- The route regression failed before the parser change and passed afterward;
  the connected Chrome session then rendered the FAQ fixture's document page
  and its FAQ route. The FAQ write was intentionally not claimed as passed:
  the isolated Lite server returned HTTP 500 because the fixture had no
  configured embedding model, which is recorded as a backend prerequisite
  rather than hidden as a UI success.

### Web route dispatch audit follow-up (2026-09-12)

- A read-only audit of `resolveRoute`, `guardRoute`, and `renderProtected`
  found three additional inconsistencies: `/knowledgeBase` passed the guard
  but had no-ID renderer branch, the public `/platform/dev/markdown` fixture
  was treated as protected and had no React page, and the broad `/creatChat/*`
  compatibility match normalized unknown suffixes before falling through to
  `NotFoundPage`.
- The route mapping now sends `/knowledgeBase` to the list page, mounts a safe
  React `DevMarkdownPage` for the development fixture, and limits the legacy
  alias to its registered exact path. RED→GREEN route tests cover all three;
  the Web test suite passed 99/99, typecheck/build passed, and the full audit is
  recorded in `docs/migrations/react/evidence/t04-route-dispatch-audit-2026-09-12.md`.
- This closes only the static dispatch defects found in this audit. T04/T13
  remain `review` for the broader live, native, and legacy renderer parity
  gates; the pre-existing `>500 kB` build warning remains non-fatal.

### Web route compatibility follow-up (2026-09-12)

- The second route audit closes the remaining static compatibility gaps in the
  current React candidate: safe URI decoding, explicit tenant/system mappings,
  knowledge-base tab/slug dispatch, legacy `q` to `cmdk` search mapping, and a
  fail-closed default for the development-only Markdown route.
- Knowledge-base chat now forwards its route id as `knowledge_base_ids`; Wiki
  selects a requested slug after loading; and organization invite preview,
  join, and request actions consume `invite_code` from the URL only after a
  successful server mutation.
- Verification: Web 100/100, shared 184/184, Embed 3/3, Desktop 2/2;
  Web/shared typechecks, Web/Embed/Desktop builds, React boundary check, and
  `git diff --check` all exited 0. Web build transformed 131 modules and kept
  the existing non-fatal `>500 kB` warning. Full browser/native/live invite
  acceptance is not claimed; T04/T10/T11/T16/T24 remain `review`.
- Evidence: `docs/migrations/react/evidence/t04-route-compatibility-follow-up-2026-09-12.md`.
- A live graph deep-link initially exposed a dispatcher gap: the initial
  entry passed only `pathname`, so the resolver's tested `tab`/`slug` handling
  was bypassed. `main.tsx` now passes pathname plus search; a fresh reload
  rendered `Knowledge graph` with `selected slug: docs/start`. The follow-up
  Web test/typecheck/build remained 100/100, 0, and 0.

### T08 Web knowledge graph follow-up (2026-09-12)

- The React Wiki graph placeholder is now a bounded server-backed graph view.
  The shared client strictly parses graph nodes/edges/meta and supports
  overview and ego queries; Web adds type/query filters, keyboard-accessible
  SVG nodes, readable list fallback, explicit loading/error/empty states, and
  neighbor expansion.
- TDD focused graph tests passed 2/2 and Wiki API tests passed 6/6. Fresh Web
  102/102, shared 186/186, Web/shared typechecks, Web build (132 modules),
  React boundary check, and diff check all exited 0. The existing non-fatal
  `>500 kB` bundle warning remains.
- This does not claim full browser graph interaction, large-KB performance,
  role/tenant negative coverage, or production backend acceptance. T08 stays
  `review`; evidence: `docs/migrations/react/evidence/t08-web-knowledge-graph-2026-09-12.md`.
- The shared renderer's adjacent Embed 3/3 and Desktop 2/2 tests, both
  typechecks, and both production builds also passed after the graph slice;
  this is cross-client bundle evidence, not native/installed acceptance.
- `7cdece5` closes a follow-up interaction defect: changing the graph type
  filter after ego expansion now preserves the active mode and center. The
  focused graph suite is 3/3; Web regression is 103/103.

### T04 Web integrations query follow-up (2026-09-12)

- The React integrations route now consumes legacy `section`/`tab` aliases
  and passes the resolved IM, Embed, API, CLI, Chrome, or Claw tab to the
  shared page. Unknown values fail closed to the existing Embed tab.
- TDD focused integration registry tests passed 3/3; Web 103/103 and shared
  187/187, typechecks, Web build, boundary check, and diff check passed.
  Evidence: `docs/migrations/react/evidence/t04-integrations-query-2026-09-12.md`.
- This closes the static query-dispatch gap only; browser tab interaction,
  provider callbacks, and integration mutation/permission evidence remain
  open, so T04/T18 stay `review`.

### T24 full Go regression final rerun (2026-09-12)

- `GOWORK=off go test ./...` exited 0 across the complete backend package
  graph. The two prior environment-sensitive groups now pass with the current
  explicit fixtures; the macOS linker emitted only non-fatal duplicate
  `-lc++` warnings for test binaries.
- Evidence: `docs/migrations/react/evidence/t24-go-regression-final-2026-09-12.md`.
  This closes the full Go regression slice, but T24 remains `review` for the
  deployed registry, installed-app cross-OS, provider, performance, and full
  browser/native acceptance matrix. T25 remains gated.

### T03 Web write replay guard follow-up (2026-09-12)

- Review found that the browser transport could refresh and replay a
  non-idempotent JSON request after HTTP 401. `http.ts` now limits automatic
  JSON/binary refresh retries to `GET`, `HEAD`, and `OPTIONS`; the existing
  streaming handshake path remains separate. A new regression proves a 401
  `POST` makes one request and does not invoke refresh.
- Focused transport tests passed 11/11. JSON and streaming POST requests both
  return the original 401 without invoking refresh or replaying their body.
  Evidence:
  `docs/migrations/react/evidence/t03-web-write-replay-guard-2026-09-12.md`.
  This closes the client-side replay defect; provider, browser role/tenant,
  and deployed acceptance evidence remains open.

### T04 integrations scope tenant follow-up (2026-09-12)

- The Web integrations route no longer reads the legacy selected-tenant key
  for API Principal operations. It consumes the current authenticated scope
  tenant and fails closed for malformed/non-positive identifiers. Focused
  tenant parsing passed 1/1; Web passed 105/105, with typecheck, build,
  boundary, and diff checks passing.
- Evidence: `docs/migrations/react/evidence/t04-integrations-scope-tenant-2026-09-12.md`.
  This closes the stale local-storage scope defect; full browser role/tenant
  and provider callback evidence remains open.

### T08 Wiki revision contract follow-up (2026-09-12)

- The shared Wiki revision parser now rejects unsafe pagination/version values
  and validates optional revision fields in both list and detail responses.
  Focused tests passed 7/7; shared passed 188/188 and Web 106/106, with both
  typechecks, boundary, and diff checks passing.
- Evidence: `docs/migrations/react/evidence/t08-wiki-revision-contract-2026-09-12.md`.
  Live conflict/revert/import/export and provider/browser acceptance remain
  open.

### T09 settings contract follow-up (2026-09-12)

- The shared knowledge-settings parser now rejects unsafe Activity numbers,
  malformed optional parser fields, non-finite chunk statistics, invalid tier
  entries, and invalid default storage identifiers. Focused tests passed 4/4
  and shared typecheck passed.
- Evidence: `docs/migrations/react/evidence/t09-settings-contract-2026-09-12.md`.
  Live provider, permission, and browser acceptance remain open.

### T22/T24 mobile chat race and Desktop renderer follow-up (2026-09-12)

- Added session/run tokens to native chat stream callbacks. Events from a
  superseded run are ignored, and the first stream created after automatic
  session creation now records assistant IDs and failure state against the
  new session rather than the previous render closure.
- Lifecycle persistence now serializes writes per session. A RED test
  reproduced concurrent SecureStore writes; the GREEN regression confirms a
  late completion cannot restore stale lifecycle state.
- The cross-client build caught a missing Desktop Vite alias for
  `@weknora/domain/chat/session-state`; the alias was added and the Desktop
  production build passed.
- Focused parity/lifecycle tests passed 7/7 and 8/8. Full mobile passed
  77/77; shared 203/203; Web 107/107; Embed 3/3; Desktop 2/2. All five
  typechecks, Web/Embed/Desktop builds, boundary check and diff check passed;
  iOS and Android Expo exports also passed.
- Evidence: `docs/migrations/react/evidence/t22-mobile-chat-race-follow-up-2026-09-12.md`.
  This closes the source/build defects only. Native device, provider-backed
  SSE, installed-app cross-OS and deployed acceptance remain open, so T22 and
  T24 stay `review` and T25 remains gated.

### T24 React Wails macOS rerun (2026-09-12)

- Rebuilt the React Wails macOS arm64 artifact after the Desktop alias fix
  with `REACT_FRONTEND=1 ./scripts/package-mac-app.sh`; Wails `v2.12.0`
  exited 0 and the assembled app was re-signed.
- Resource, commit identity, and `codesign --verify --deep --strict` smoke
  all passed. Evidence:
  `docs/migrations/react/evidence/t24-wails-macos-react-rerun-2026-09-12.md`.
- This strengthens only the macOS arm64 installed-artifact slice. Windows,
  Linux, runtime interaction, rollback rehearsal, and the complete release
  matrix remain open; T24 stays `review` and T25 remains gated.

### T15 MCP settings and N007 upload confirmation follow-up (2026-09-12)

- MCP settings now has a concrete Web service panel for viewer/admin states,
  built-in restrictions, CRUD/toggle/delete flows, credential subresource
  writes, metadata refresh/stale handling, tool-policy operations, usage
  generation, connection-test results, and OAuth status/authorize/revoke
  controls. Shared metadata and policy routes are backed by the existing Go
  MCP endpoints. Commits: `3cb1def3`, `4120b926`, `f24070ac`.
- The upload confirmation slice now supports per-file status, cancellation,
  retry-after-error, tag selection, drag/drop staging, removal of one staged
  file, and URL confirmation with HTTP(S) validation. Focused upload tests
  passed 7/7. Commits: `04f18ead` and `7d42afb2`.
- The current Web suite passed 263/263 and shared/Web typechecks passed after
  a compatibility/type-narrowing repair in the existing dirty command-palette
  slice; its files remain uncommitted and no unrelated files were staged by
  the MCP/upload commits.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-12-mcp-settings-base.md`
  and `docs/migrations/react/evidence/vue-react-parity/2026-09-12-upload-confirmation.md`.
  MCP rows R024/R043-R046/N016 and upload row N007 remain `implementing`;
  browser, real-backend, Wails, iOS, and Android acceptance is still absent
  for these slices.
- A fixed 1355×720 browser recording reached the React login page but could
  not enter protected routes without an authenticated SSO session. Recording:
  `/Users/wuyongjun/.config/browser-harness/agent-workspace/recordings/react-parity-20260912`.

### T15 MCP review repair and R027 model settings follow-up (2026-09-12)

- Independent MCP review found 7 Important and 3 Minor findings. The
  Important behavior defects were repaired: missing default tool-policy rows
  now appear immediately after a successful write; generated usage text is
  persisted through the existing MCP update route; stale metadata blocks
  generation and policy writes; metadata/policy/OAuth loads use a generation
  guard; MCP writes no longer appear for system-admin-only UI sessions when
  the routes require tenant Admin; bearer and stdio records are preserved for
  safe display/edit handling; and a post-create credential failure retains
  the returned service ID for recovery instead of creating a duplicate.
- R027 now has a typed Web model panel and pure model contract helper. It
  maps UI types to the backend `KnowledgeQA|Embedding|Rerank|VLLM|ASR`
  vocabulary, validates remote URLs and embedding dimensions, loads provider
  metadata, renders type tabs/cards, gates built-in operations by role, and
  writes credentials through the dedicated model subresource. It remains
  `implementing` because Ollama download/progress, provider-specific fields,
  custom headers, debug/usage flows, localization, visual and runtime/native
  evidence remain.
- Focused MCP/model/upload tests passed 32/32; Web passed 263/263; shared and
  Web typechecks plus diff check passed. Evidence:
  `docs/migrations/react/evidence/vue-react-parity/2026-09-12-mcp-settings-base.md`
  and `docs/migrations/react/evidence/vue-react-parity/2026-09-12-model-settings.md`.
- MCP test-result rendering now includes Vue-equivalent success/failure details,
  expandable tool schemas, resource URI/MIME rows, and success empty state;
  focused tests and the full Web suite pass (276/276).
- MCP metadata tools now have Vue-equivalent search, 20-item pagination,
  description/parameters/schema detail tabs, policy controls, and fail-closed
  retry state; Web typecheck and the full suite pass (278/278).
- MCP editor now accepts standard `mcpServers` JSON import without auto-save,
  preserves custom headers and OAuth scopes, and sends bounded timeout/retry
  configuration through the existing update contract; focused editor tests and
  full Web regression pass (279/279).

### R031 Sandbox settings and review repairs (2026-09-12)

- Added the shared sandbox configuration API client for the existing list,
  get, create, update, delete, inventory, and workspace-policy routes. Strict
  envelope parsing and conflict-code normalization are covered by 3/3 focused
  tests.
- Added the Web Sandbox settings slice with loading/empty states, viewer/admin
  gating, backend tabs/cards, script-policy confirmation, basic create/edit
  entry, and server-backed deletion. Component tests passed 5/5 and the full
  Web suite passed 269/269; shared and Web typechecks passed.
- Independent review also found model type/parameter regressions and an MCP
  policy fail-open. The model payload now emits exact `VLLM`/`ASR` values and
  preserves unknown parameters; MCP policy controls fail closed when policy
  loading fails. These repairs remain implementing-level evidence only.
- Evidence: `docs/migrations/react/evidence/vue-react-parity/2026-09-12-sandbox-settings.md`.
  R031 remains `implementing`; Vue wizard/template/deep-check/inventory
  states, localization, browser, real-backend, Wails, iOS, and Android
  evidence are still open.

- Follow-up review aligned Sandbox role checks with the shared `roleAtLeast`
  helper and made the settings route load its initial state from the shared
  `sandboxConfigurations` client instead of coercing an unrelated payload.
  Web typecheck and the 5/5 Sandbox component tests passed again.
- Sandbox delete now parses the shared structured conflict error and shows the
  backend-reported live sandbox count; focused Sandbox tests, Web typecheck,
  and full Web regression passed (274/274).

### R033 Skill settings entry follow-up (2026-09-12)

- Connected the Settings `skills` entry to the existing typed catalog
  operations. Administrators now reach registration, installation polling,
  safe file listing/content reads, and stop-install controls; viewers receive
  a read-only inventory. No second skill API was introduced.
- Focused Skill component tests passed 2/2, the full Web suite passed 271/271,
  and the Web typecheck passed. Evidence:
  `docs/migrations/react/evidence/vue-react-parity/2026-09-12-skill-settings.md`.
- R033 remains `implementing`: exact Vue child-surface behavior, localization,
  visual comparison, real backend, browser, Wails, iOS, and Android evidence
  are still required.

### R038 Tenant members settings entry follow-up (2026-09-12)

- Connected the Settings `members` entry to the existing tenant identity
  client. Viewer state is read-only; admin/owner state supports member search,
  pagination, invitation, role updates, and removal with server-backed
  requests and explicit loading/error/empty states.
- Focused TenantMembers tests passed 2/2 and the Web typecheck passed. Evidence:
  `docs/migrations/react/evidence/vue-react-parity/2026-09-12-tenant-members.md`.
- R038 remains `implementing`: invitation table/revoke/copy, audit and
  permission surfaces, localization, visual comparison, real backend,
  browser, Wails, iOS, and Android evidence remain open.

- Follow-up added pending-invitation loading/revoke and a lazy audit-log
  surface using the existing typed tenant APIs. Focused tests and Web
  typecheck remain green; invitation copy/link variants and the Vue permission
  popover are still open.

### N005 Knowledge-base share dialog follow-up (2026-09-12)

- Added a Web share dialog to manageable knowledge-base cards. It reuses the
  existing organization/share endpoints for eligible-organization loading,
  viewer/editor permission selection, current-share listing, confirmed
  unshare, and list refresh after mutation.
- Focused share-dialog test passed 1/1, the full Web suite passed 274/274, and
  the Web typecheck passed. Evidence:
  `docs/migrations/react/evidence/vue-react-parity/2026-09-12-kb-share-dialog.md`.
- N005 remains `implementing`: upload-mask/progress linkage, exact Vue visual
  and localized copy, browser, real-backend, Wails, iOS, and Android evidence
  remain open.
