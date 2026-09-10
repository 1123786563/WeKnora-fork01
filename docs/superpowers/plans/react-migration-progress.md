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

## 任务状态

| 任务 | 状态 | 实现文件/提交 | 测试与退出码 | 证据层级 | 问题/下一步 |
|---|---|---|---|---|---|
| T01 基线、契约和复用来源冻结 | review | `eb0e9a9` + working-tree T01 audit | `python3 -m unittest scripts/test_generate_react_migration_baseline.py -v` 0（4/4）；`python3 scripts/generate_react_migration_baseline.py` 0；`git diff --check` pending | 静态矩阵/确定性/注册路由分类：通过；Swagger 282 paths/361 operations 与注册 Gin 路由对照；真实后端/截图/Wails 包/原生：未完成 | API 矩阵 452 行（361 Swagger + 91 implementation-only），route parity 53 行；handler DTO/权限逐行复核、OpenAPI Generator 试点和真实 smoke 仍待补 |
| T02 无框架 SDK与第一条真实 API 链路 | review | `e3a3a8f` (`feat: add framework-free shared client foundation`) | `pnpm test:shared` 0（10/10）；`pnpm typecheck:shared` 0；TDD nested-error/late-abort RED→GREEN；真实后端 0 | 静态/Node mock：通过；真实后端 Web：未完成 | 已实现 contracts/api-client/domain 与 KB list mock 链路；仍需接入 Web 调试页、共享认证和真实后端 |
| T03 登录、刷新、OIDC与凭证隔离 | review | `118d15d` (`feat: isolate bearer refresh and embed credentials`) | `pnpm test:shared` 0（15/15）；`pnpm typecheck:shared` 0；auth RED→GREEN；`git diff --check` 0；真实后端/OIDC/浏览器 0 | 静态/Node mock：通过；真实后端、OIDC浏览器回调、React登录页：未完成 | 已实现异步 credential adapter、Bearer 单飞 refresh、Embed 隔离、generation/invalidate 和严格 token 校验；需后续接入 auth endpoints、Web adapter 与真实回调 |
| T04 空间上下文、路由和能力守卫 | review | `9b35b5a` (`feat: guard scoped requests across tenant changes`) | `pnpm test:shared` 0（17/17）；`pnpm typecheck:shared` 0；scope RED→GREEN；`git diff --check` 0；真实路由/后端 0 | 静态/Node mock：通过；真实路由、Web 深链、后端权限：未完成 | 已实现切空间 abort、generation stale guard、logout/invalidate；仍需 Web Router/Query 接入与服务端权限负例 |
| T05 UI基础、平台注入和国际化 | review | `a230366` + `ac955da` | `pnpm test:web` 0（3/3）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | React/Vite source and focused tests: passed; package installation: passed; live backend/native/i18n coverage: not completed | Added explicit legacy-session adapter, UI Button/Card/Status primitives and React entry; no broad visual redesign. Internationalization package is still pending as a separate slice. |
| T06 知识库列表与创建编辑闭环 | review | `a230366` + `ac955da` + `de1ce63` + `292fdc4` | `pnpm test:shared` 0（46/46）；`pnpm test:web` 0（12/12）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0 | Static/Node mock and production bundle: passed; live backend list/create/edit/browser: not completed | React list now exposes strict create/update/delete mutation flow with explicit error state; cache invalidation is reload-based. Live backend, permissions, pagination/search, and browser acceptance remain. |
| T07 文档上传、列表、目录标签与预览 | review | `45901be` + `d0af87f` + `93d3099` | `pnpm test:shared` 0（24/24）；`pnpm typecheck:shared` 0；`pnpm test:web` 0（4/4）；`pnpm typecheck:web` 0；`pnpm build:web` 0 | Contracts/API/domain focused tests, Web loader and production bundle: passed; real upload/processing/preview/browser: not completed | Added paginated document DTO validation, processing-state guard, document list API, multipart upload API, no-fallback Web loader and non-browser-safe FormData detection. UI document view, folders/tags/preview, real backend upload and 413/browser proof remain. |
| T08 FAQ与Wiki编辑/版本 | review | `736eca4` + `3d8daa6` | `pnpm test:shared` 0（32/32）；`pnpm typecheck:shared` 0；`git diff --check` 0 | Shared Wiki diff/API client tests: passed; FAQ/Wiki editor/version conflict/browser: not completed | Migrated pure line/revision diff and added typed Wiki page list/get/create/update/delete/revision/revert paths with optimistic version payloads. UI editor conflict handling, FAQ operations and live backend remain. |
| T09 知识库高级配置与数据源 | review | `f2fe50e` + current verification | `pnpm test:shared` 0（35/35）；`pnpm typecheck:shared` 0；`git diff --check` 0 | Data-source CRUD/control endpoint seam and focused tests: passed; settings UI, permission negatives, connector credentials/live sync: not completed | Added typed data-source list/get/create/update/delete, credential validation, resource listing and sync pause/resume paths. Advanced KB settings UI and live connector evidence remain. |
| T10 聊天协议、状态机与恢复底座 | review | `cfb7d29` + `f2c2fd3` + `360ab98` | `pnpm test:shared` 0（59/59）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Existing backend response_type vocabulary, pure reducer, incremental transport reader and SSE framing tests: passed; live SSE/reconnect/restore: not completed | Added response-type contract helper, deduplicating chat reducer, browser ReadableStream transport path and resumable `Last-Event-ID` request builder. Continue-stream recovery, cancellation timeout, and real backend/browser proof remain. |
| T11 会话、消息和问答主界面 | review | `5fcbae4` + `360ab98` | `pnpm test:shared` 0（59/59）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Session/message contracts, scoped draft key, React sidebar/composer/message-list/page, shared SSE reducer binding and Web route compile: passed; real RAG/Agent/browser acceptance: not completed | Added typed session list/create/history APIs and explicit new-chat flow. Web now submits knowledge chat through shared SSE parsing/reducer; title/pin/delete/source filter/suggestion, stream restore, and real backend/browser proof remain. |
| T12 工具审批、MCP OAuth与运行中追加 | review | `93f2e01` | `pnpm test:shared` 0（69/69）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0 | Strict approval/OAuth/steer route contracts and mock endpoint tests: passed; real approval/OAuth/steer browser lifecycle: not completed | Added typed approval and OAuth resolution APIs plus steer enqueue/list/promote/remove with encoded identifiers, status validation, and correlation fields. Chat cards, OAuth external-browser lifecycle, steer race handling, and live backend proof remain. |
| T13 Markdown、引用、工具结果与产物 | review | `addbc48` | `pnpm test:shared` 0（78/78）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Pure reference grouping, artifact metadata sanitization/path construction, tool renderer classification and security policy tests: passed; protected download/preview/browser XSS/performance: not completed | Added UI-independent reference groups, variable-length public resource handles, authenticated artifact download path, expiry handling, and explicit plain-text fallback for unknown tools. DOM markdown renderer, protected live artifact download and browser security evidence remain. |
| T14 沙箱终端与文件面板 | review | `7579143` | `pnpm test:shared` 0（83/83）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Strict ticket DTO, authenticated ticket POST, WS URL without JWT, terminal status/generation pure tests: passed; real WS shell/resize/reconnect/browser: not completed | Added short-lived ticket API and explicit terminal state/URL helpers. Web terminal panel, ticket refresh/reconnect, PTY input/resize, and live shell evidence remain. |
| T15 Agent、模型、MCP与Skill配置 | review | `cbe5710` + `9b253b6` | `pnpm test:shared` 0（89/89）；`pnpm typecheck:shared` 0；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Strict shared configuration list/get/create/update/delete seams for Agent/Model/MCP and Skill listing, secret redaction tests: passed; configuration UI, permission negatives, debug/OAuth/install lifecycle and live calls: not completed | Added shared configuration API with route-specific collections, encoded IDs, success-envelope validation, explicit failure behavior, and secret removal from returned records. Full settings UI and per-feature live acceptance remain. |
| T16 空间与组织管理、系统后台 | review | `8ec7791` (`feat: add identity and administration client seams`) | `pnpm test:shared` 0（98/98）；`pnpm typecheck:shared` 0；`pnpm test:web` 0（15/15）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Shared strict identity/administration API and production Web bundle: passed; real backend role matrix, browser access matrix and Lite runtime: not completed | Added tenant members/invitations/audit, organization CRUD/membership/share routes, system admins/API keys/settings/runtime queues; UI screens, live 403/409 matrix and capability-driven visibility remain |
| T17 其余设置、Memory与运行配置 | review | `798543e` (`feat: add remaining settings client seams`) | `pnpm test:shared` 0（106/106）；`pnpm typecheck:shared` 0；`pnpm test:web` 0（15/15）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | Strict settings client, secret-redaction, encoded-resource, raw-code-probe and section-registry tests: passed; real backend settings permissions, browser E2E and Lite capability visibility: not completed | Added client seams for tenant KV, profile/preferences, tenant, Ollama, parser/retrieval/memory/chat-history, personal env vars, storage/vector/web-search resources, system info and WeKnoraCloud; full React settings screens and live query/save/reset/test/unavailable acceptance remain |
| T18 Embed、IM与外部集成入口 | review | `4ee56e0` (`feat: add isolated embed and integrations surfaces`) | `pnpm test:shared` 0（116/116）；`pnpm typecheck:shared` 0；`pnpm test:web` 0（15/15）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`pnpm test:embed` 0（3/3）；`pnpm typecheck:embed` 0；`pnpm build:embed` 0；`git diff --check` 0；Embed bundle gate 通过 | Isolated Embed entry, exchange/session/chat/history/SSE, origin-guarded bridge, IM/Embed integration API seams and Web integration hub: passed; real backend callback/upload/reference flows, anonymous third-party iframe proxy, API playground SSE, permission negatives and browser E2E: not completed | Added `apps/embed`, lightweight `@weknora/api-client/embed` entry, no-cookie/no-Bearer Embed transport, signed visitor session persistence without token storage, strict parent-origin postMessage handling, Embed/IM management route and external integration guide links. Live backend, iframe proxy, file/reference rendering, API playground and role/capability acceptance remain |
| T19 Wails React renderer与Lite发布 | review | `477793f` (`feat: add React desktop renderer build seam`) | `pnpm typecheck:desktop` 0；`pnpm test:desktop` 0（2/2）；`pnpm build:desktop-renderer` 0；`bash -n scripts/package-lite.sh scripts/package-mac-app.sh` 0；`make -n build-lite REACT_FRONTEND=1` 0；`jq` Wails config 0；Wails frontend install/build phase 0；`git diff --check` 0 | Standalone npm-compatible Wails renderer wrapper, shared React Web entry, injected desktop API root, legacy-path normalization, credential/file/navigation seam and React-opt-in Lite scripts: passed; Wails app/package Go compile failed because the environment could not download several uncached Go modules (network reset), installed package smoke on macOS/Windows/Linux and upgrade/runtime lifecycle: not completed | Changed `cmd/desktop/wails.json` to build `apps/desktop` through root pnpm, kept Vue Lite as default with `REACT_FRONTEND=1` opt-in, and added release workflow pnpm setup. Full Wails package and cross-OS evidence remain required before accepting T19 |
| T20 Expo基础、原生登录和网络生命周期 | pending | — | — | — | 依赖 T02/T03/T04 |
| T21 原生知识库、检索与文件 | pending | — | — | — | 依赖 T07/T20 |
| T22 原生聊天、引用与核心闭环 | pending | — | — | — | 依赖 T11/T12/T20 |
| T23 移动管理功能与能力矩阵补齐 | pending | — | — | — | 依赖 T08/T09/T14/T16/T17 |
| T24 构建、回归矩阵与灰度发布 | pending | — | — | — | 依赖 T07–T23 |
| T25 Vue退役与维护交接 | pending | — | — | — | 依赖 T24 且满足退役门槛 |

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
- 当前环境没有已配置的 OpenAPI Generator 试点结果、真实后端测试身份、核心截图采集和 Wails/Expo 主机证据；这些缺失不能用已有 Vue 构建替代。

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

## T01-T05 completion-plan execution (2026-09-10)

- 详细执行计划：`docs/superpowers/plans/2026-09-10-react-multiclient-t01-t05.md`，提交 `e4134a0`。
- 隔离 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`，分支 `codex/react-multiclient`。
- T01 集成提交：`a356e80`（worker 原提交 `e211610`）；生成器把 Swagger 361 操作、实现专有 91 路由和 53 条入口/特殊路由分开记录，focused unittest 4/4 通过。
- T02 集成提交：`0105fa8`、`19dede6`；新增 Web 注入式 transport，修复 Embed scheme/session/visitor/tenant 隔离，Web focused tests 11/11、shared tests 46/46、Web typecheck/build 通过。
- T03 集成提交：`ef3377a`；新增严格 auth login/refresh/me/logout facade 和 React Login seam，shared tests 46/46、Web typecheck/build 通过；完整 OIDC/真实后端尚未验证。
- T04 集成提交：`cc3556f`；新增 scope runtime、旧 URL 解析/redirect 和跨空间 generation 测试，Web tests 11/11 通过；真实深链/后端权限负例尚未验证。
- T05 集成提交：`a784f2b`；新增无 Vue runtime 的 design-tokens/i18n/platform adapters。当前实际源码只有 5 个 locale（不是计划文字中的 6 个），未凭空新增第六语言；i18n/native/keyboard browser evidence 尚未完成。

### 本轮验证证据

- `pnpm install --frozen-lockfile`：0（隔离 worktree，8 workspace projects）。
- `python3 scripts/generate_react_migration_baseline.py`：0；`python3 -m unittest scripts/test_generate_react_migration_baseline.py -v`：4/4，0。
- `pnpm test:shared`：46/46，0；`pnpm typecheck:shared`：0。
- `pnpm --filter @weknora/web test`：11/11，0；`pnpm typecheck:web`：0；`pnpm build:web`：0；`git diff --check`：0。
- 证据层级：静态/Node mock/Web bundle 已有；真实后端、浏览器真实账号/OIDC、Wails 安装包、移动原生和六语言完整迁移仍缺失，因此 T01-T05 保持 `review`，不得写为 `accepted`。

### T16 review evidence (current worktree)

- Added `packages/api-client/src/identity/` for tenant members, invitations, audit cursor, organization membership, join requests, KB/agent sharing and tenant-independent organization discovery.
- Added `packages/api-client/src/administration/index.ts` for tenant/platform API keys and capability scopes, system-admin operations, system settings, system audit and runtime queue/task pagination.
- Every mutation uses the shared request boundary; encoded identifiers and query parameters are tested. HTTP 403/409 `ApiError` instances are deliberately not converted to successful responses; deletion/leave methods resolve only after the server response is accepted.
- TDD evidence: missing modules produced RED module-resolution failures; focused tests then passed 7/7; aggregate `pnpm test:shared` passed 98/98.
- `pnpm typecheck:shared`, `pnpm typecheck:web`, and `pnpm build:web` exited 0; real backend, browser access matrix, Lite capability visibility and React organization/administration screens remain unverified, so T16 stays `review`.

### T17 review evidence (current worktree)

- Added `packages/api-client/src/settings/index.ts` and wired it under `client.settings`, covering tenant KV settings (parser, retrieval, memory workspace, storage legacy, web-search legacy and chat history), profile/preferences, tenant metadata, Ollama probes/downloads, parser probes/reconnect, personal memory CRUD, caller-owned env vars, storage backends, vector stores, web-search providers/credentials, system info and WeKnoraCloud status/credentials.
- Preserved backend response differences: `success/data` envelopes, raw `code: 0/data` parser/system probes, direct WeKnoraCloud status and logical `success: false` connection-test results are not collapsed into fabricated empty values. Requests continue through the shared transport so HTTP/API failures remain observable.
- Added recursive response secret redaction for API keys, app/client secrets, access tokens, passwords and provider secret variants; resource IDs and Ollama task IDs are encoded. TDD focused coverage includes RED module-resolution failures followed by 15/15 settings/client/registry tests passing.
- Added `packages/views/src/settings/registry.ts` with one entry for every T17 legacy section (`general`, `tenant`, `userprofile`, `ollama`, `parser`, `retrieval`, `memory`, `mymemory`, `envvars`, `storage`, `vectorstore`, `websearch`, `chathistory`, `system`, `weknoracloud`), explicit scope/role/operation metadata, and separate personal versus tenant credential scopes.
- Verification: `pnpm test:shared` 106/106, `pnpm typecheck:shared` 0, `pnpm test:web` 15/15, `pnpm typecheck:web` 0, `pnpm build:web` 0, and `git diff --check` 0. These are static/Node mock and production-bundle proofs; real backend 403/409, browser settings E2E, reset behavior against live state, Lite capability filtering, and React settings screen rendering remain unverified, so T17 stays `review`.

### 本轮问题与裁定

- 计划/设计原文仍写“方案 B 尚未批准”，与用户本轮批准相冲突；本实施分支按用户批准的方案 B 执行，未因旧措辞改变架构。
- Multica 根许可证含 hosted/commercial/branding 附加条件；本轮仅采用分层模式，未复制其产品源码或品牌资产。
- T02 reviewer 发现并已修复 Embed 裸 token、租户头泄露和缺失 session/visitor header；修复提交为 `19dede6`，修复后验证 11/11。
- 后续 reviewer 仍发现：`createRefreshCoordinator` 尚未接入 `client.request` 的 401 重试；真实 OIDC/刷新并发、租户切换 UI 生命周期和 Embed 独立入口仍未闭合，继续保持 `review`。
- `15889e1` 修复 auth logout 的 `success:false`、动态凭证读取、Embed 路由隔离和 `/platform`/`creatChat` 兼容路径；`0fe0e4b` 刷新基线至当前 HEAD。
- 当前 worktree 保留三份用户提供的未跟踪权威输入文件，未修改、未清理、未提交。
