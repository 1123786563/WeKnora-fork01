# Vue / React 全量页面与入口盘点台账

盘点日期：2026-09-15  
工作目录：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`  
范围：Vue 权威源、React Web/Desktop/Mobile/Embed 入口、旧 URL、设置 section、全局/页面弹层与侧栏、运行前置条件。  
约束：本轮未修改业务代码；保留工作区已有修改。

## 证据口径

| 标记 | 含义 |
|---|---|
| `static` | 源码、路由、矩阵、配置或测试静态证据；不等于浏览器验收 |
| `browser` | 已有或本轮可复现的浏览器/AX/截图证据；需注意是否同账号同条件 |
| `backend` | 真实后端 HTTP/API/业务链路证据 |
| `blocked-env` | 受环境、凭证、服务、设备或外部 provider 限制，当前不能声称已验收 |
| `review` | 矩阵已有实现/局部证据，但仍缺完整状态、视觉、后端或平台闭环 |

## 1. 输入文档与当前基线

- `AGENTS.md`：Issue 使用 GitHub；领域文档先看 `CONTEXT.md` 与相关 ADR（本 worktree 未发现需要额外纳入的 `CONTEXT.md`/ADR）。`static`
- `docs/agents/issue-tracker.md`、`docs/agents/domain.md`：已读取。`static`
- `docs/superpowers/plans/2026-09-10-react-multiclient-migration.md`：方案 B 仍是计划建议，不是已批准完成；要求保留旧 URL、Embed、Lite/Wails、六种语言和 Vue 语义。`static`
- `docs/superpowers/plans/2026-09-10-react-migration-inventory.md`：既有库存称 Vue 组件/API/Swagger 操作等静态库存；补充调查明确要求双向路由、`/files`、SSE、WS、Embed、无独立路由设置和弹层。`static`
- `docs/migrations/react/route-parity.csv`：历史基线曾记录 53 行；2026-09-16 当前快照为 59 行入口/表面清单，含特殊文件、WebSocket、Embed 和设置项。当前行数与 Apps authority 缺口见 `artifacts/route-inventory-parity-ledger-audit-20260916.md`。`static`
- `docs/migrations/react/vue-react-parity-matrix.md`：当前矩阵；头部记录 Vue baseline source commit `5cf093706ebecdfe8bc4eca80886e80c01805289`、task base `9b79558b6229d79d0ceebe22e1de4a439982c615`。矩阵仍以 `review` 为主，不能批量视为 accepted。`static`
- `docs/migrations/react/vue-react-parity-progress.md`：截至 R326 的逐轮证据；明确大量行仍缺同条件 Vue/React、真实后端、Wails/iOS/Android 或负路径证据。`static`

既有矩阵的验收原则：路由可达、构建、组件测试或单张截图都不足以证明 parity；每行还需覆盖 loading/empty/error/forbidden/edit/submitting/success/failure、角色/能力、语言、主题、响应式和适用平台。

## 2. Vue 路由与入口逐项盘点

权威源：`frontend/src/router/index.ts`；现有 53 行入口明细见 `docs/migrations/react/route-parity.csv`。下表为按语义重新归组的完整入口集合；`direct` 是 Vue router 直接注册，`compat` 是兼容重定向，`special` 是非 SPA 路由。

| Vue 入口/模式 | 类型 | 行为与关键状态 | React 对应 | 证据/当前结论 |
|---|---|---|---|---|
| `/` | compat | 重定向 `/platform/knowledge-bases`；Lite 可能恢复上次安全子路径 | `routes.tsx`/`main.tsx` | `static`; route tests 覆盖，最终浏览器 reload 仍属 `review` |
| `/login` | direct | 登录、OIDC、语言切换、自动初始化、失败/过期会话 | `LoginPage` + shared auth/mobile auth | `static` + 历史 `browser`; 外部 IdP 为 `blocked-env` |
| `/register` | direct | 复用 Login.vue 的邀请/自助注册模式 | `LoginPage`/`JoinPage` | `static` + 已有 live 注册证据；OIDC 分支 `blocked-env` |
| `/onboarding/workspace` | direct | 已认证无租户引导；有租户则回知识库 | `WorkspaceOnboardingPage`/mobile | `static` + 局部 browser/backend；完整角色/原生仍 `review` |
| `/join` | compat | `code` 转 `/platform/organizations?invite_code=...`；邀请 token 走注册/加入语义 | `JoinPage`/organizations | `static`; route tests 与历史 live invite；完整 token 变体仍 `review` |
| `/knowledgeBase` | direct legacy | 旧知识库入口，当前 Vue `KnowledgeBase.vue` | React legacy dispatch → KB list | `static`; 深链测试有，保护页后端/视觉 `review` |
| `/platform` | layout+compat | Platform shell，重定向知识库列表；全局菜单、设置、命令面板、邀请铃、引导、拖拽上传 | `PlatformShell`/`App` | `static` + 局部 authenticated browser；响应式/Wails/native/权限降级 `review` |
| `/platform/settings` | direct modal | 查询 `section` 选择设置；Teleport modal、分组侧栏、角色/能力隐藏 | `SettingsPage`/registry | `static` + 局部 browser；全状态/多端 `review` |
| `/platform/knowledge-bases` | direct | KB 列表、搜索/命令面板、创建、空间 rail、分享、上传 | `KnowledgeBaseList`/App | `static` + 局部 browser；真实后端/Wails/native `review` |
| `/platform/knowledge-bases/:kbId` | direct | 文档列表、文件夹、标签、批量、上传、预览、processing、Wiki/FAQ/图谱/KB 设置入口 | React documents/knowledge subpages + mobile | `static` + 局部 browser；上传/预览/后端/原生 `review` |
| `/platform/knowledge-search?q=` | compat | 转 KB 列表并一次性打开 GlobalCommandPalette，保留 q | `routeRedirect`/command palette | `static` + 局部 browser；语义搜索 MVP 限制已登记 |
| `/platform/agents` | direct | Agent 列表、创建/编辑 modal、分享/嵌入、能力门禁 | React `AgentsPage`/Configuration | `static` + 既有 browser 局部；DB 收藏/完整引导/平台 `review` |
| `/platform/integrations` | compat | `tab`/`section` 归一化到 `/platform/settings?section=integration-*` | `IntegrationsRoutePage` | `static`; provider/受保护 mutation/同条件 Vue 对照 `review` |
| `/platform/creatChat` | direct legacy spelling | 新建全局对话，必须保留 `creatChat` 拼写 | React `ChatRoutePage` | `static` + 局部 browser/backend；完整 chat 状态 `review` |
| `/platform/knowledge-bases/:kbId/creatChat` | direct | 指定 KB 新建对话 | React chat route | `static`；完整 stream/附件/审批/平台 `review` |
| `/platform/chat/:chatid` | direct | 历史会话恢复、流式消息、附件、引用、工具/审批/产物/终端 | React chat route | `static` + 局部 runtime；provider/完整状态/原生 `review` |
| `/platform/organizations` | direct | 组织列表、邀请预览/加入、共享空间 | React `OrganizationsPage`/mobile management | `static` + 局部 live invite；完整 CRUD/角色/原生 `review` |
| `/platform/tenant` | compat | 转 settings（当前 React `/platform/tenant` → `/platform/settings`） | route redirect | `static`; 深链已测，设置后端/权限 `review` |
| `/platform/system`, `/platform/system/settings`, `/platform/system/admins` | compat | system-admin 门禁后转 `section=system-global` | route redirect | `static`; 非管理员门禁测试，真实系统管理员 `review` |
| `/platform/system/queues` | compat | system-admin 门禁后转 `section=runtime-queues` | route redirect | `static`; 后端队列状态 `blocked-env`/`review` |
| `/platform/dev/markdown` | dev-only | 仅 DEV 的 Markdown fixture | `DevMarkdownPage` | `static`; test-only，不计产品页面 |
| `frontend/embed.html` / `/embed/*` | independent | 独立 Embed entry、token exchange、postMessage、iframe chat、附件/文件 proxy | `apps/embed` | `static` + 局部 browser；preview token 与 `/embed/sessions` 语义问题、CORS/provider 为 `blocked-env` |
| `/files`、`/r/:token`、presigned、KB/message/embed file proxy | special | 保护文件、签名资源、HMAC presigned、消息/Embed 文件访问 | API client/file surfaces | `static`; 真实签名/权限/过期/下载仍 `review` |
| `/api/v1/sessions/:id/sandbox/terminal` | special WebSocket | ticket 鉴权、terminal 输入/resize/取消 | Web terminal + API client | `static` + 局部 runtime；真实 sandbox/provider 为 `blocked-env` |

React-only compatibility forms also found in `apps/web/src/routes.tsx`: `/knowledgeBase/:id/documents`, `/wiki`, `/faq`, `/settings` subpaths, `/platform/chat` and `/creatChat`. They are explicit React aliases/dispatch targets, not additional Vue router rows; do not count them as missing Vue routes or silently drop them.

## 3. 设置项与能力/角色门禁

Vue `Settings.vue` 当前静态识别 23 个 section；Settings modal 由 `Teleport` 挂载，查询参数是主要深链契约。`settingsRoute.ts` 还保留 `integrations`、裸 `api`/tab 名称及旧 `tab` 参数兼容。

| 分组 | section keys（Vue） | 典型操作/门禁 | 证据 |
|---|---|---|---|
| 账户 | `general`, `userprofile`, `mymemory`, `envvars` | 本地偏好/账户资料；个人记忆与用户 env/密钥的读写删除 | `static`; env/memory 局部 browser，完整角色/失败状态 `review` |
| 空间 | `tenant`, `members`, `chathistory`, `memory` | 空间资料、成员/邀请/审计入口、消息索引、长期记忆；按 viewer/admin/owner | `static`; 成员/邀请局部 backend，完整 protected CRUD `review` |
| 模型运行时 | `models`, `ollama`, `weknoracloud` | 模型 CRUD/测试；Ollama 连接/下载；Cloud 凭证/模型状态；多为 admin 写 | `static` + authenticated AX 局部；Ollama 服务不可达，成功路径 `blocked-env` |
| 集成 | `integration-im`, `integration-embed`, `integration-api`, `integration-cli`, `integration-chrome`, `integration-claw` | IM/Embed/API 管理；CLI/Chrome/Claw 外链 landing；API owner-only | `static`; IM/Embed/API provider 与 callback、API key `review`/`blocked-env` |
| 数据与扩展 | `vectorstore`, `parser`, `storage`, `sandbox`, `skills`, `websearch`, `mcp` | provider CRUD/test、解析器、存储、沙箱、Skill 安装、MCP OAuth/tools；能力缺失应隐藏/不可用 | `static`; 图数据库、sandbox、MCP provider 缺失项 `blocked-env` |
| 系统管理 | `system-global`, `runtime-queues`, `platform-api-keys`, `system-audit-log` | system-admin；平台设置、队列、平台 key、审计 | `static`; 需 system-admin + backend，当前未闭环 |
| 平台 | `system` | 版本/系统信息只读 | `static`; 局部 browser，完整多端 `review` |

React registry (`packages/views/src/settings/registry.ts`) 已显式标出 `ported: true/false`；其中 `models`、`members`、`mcp`、`sandbox`、`skills` 及系统管理 section 仍不能因有 registry 或静态组件而视为 parity 完成。每个 section 必须另测：正常、loading、empty、error、no-permission、disabled/unavailable、editing、submitting、success、failure。

## 4. 弹窗、抽屉、菜单、预览与全局 surface 清单

静态扫描 Vue `frontend/src` 得到 123 个含 Dialog/Drawer/Modal/ContextMenu/Preview/Popover/CommandPalette/Dropdown 语义的候选文件；下表按功能面汇总（文件是可追溯入口，不把每个子组件误算成独立路由）。

| Surface | Vue 关键文件/组件 | 必须覆盖的行为 | 当前证据 |
|---|---|---|---|
| 平台全局 | `menu.vue`, `UserMenu.vue`, `TenantSelector.vue`, `KBSwitcherDropdown.vue`, `GlobalCommandPalette.vue`, `GlobalInvitationBell.vue`, `NewUserGuide.vue`, `SpotlightGuide.vue`, `CreateTenantDialog.vue`, `MyInvitationsDialog.vue` | 用户菜单、空间切换、⌘K、邀请、创建空间、引导重开、键盘/Esc/焦点、能力/角色隐藏 | `static` + 局部 browser AX；全 tour、跨租户迟到响应、responsive/Wails/native `review` |
| 设置 | `Settings.vue`, `SettingDrawer.vue`, `SettingCard.vue`, `McpServiceDialog.vue`, `McpMetadataPanel.vue`, `McpTestResultBody.vue`, `McpToolsList.vue` | modal Teleport、分组/子菜单、drawer resize、表单校验、确认/取消、MCP OAuth/test/tools/删除 | `static` + settings 局部 browser；真实 provider/完整 role/平台 `review` |
| 知识库列表/创建/分享 | `KnowledgeBaseEditorModal.vue`, `ShareKnowledgeBaseDialog.vue`, `KBInfoPopover.vue`, `KbCreateContextualGuide.vue`, `SourceSwitcherDropdown.vue` | 创建/编辑、组织共享、权限反馈、空/错误/加载、外链/引导 | `static` + share dialog focused/browser 局部；真实 share/unshare 与 paired screenshot `review` |
| 文档上传/操作 | `UploadConfirmDialog.vue`, `UploadConfirmHost.vue`, `KbUploadSourceDropdown.vue`, `DocumentActionMenu.vue`, `DocumentBatchBar.vue`, `BatchTagDialog.vue`, `TagEditDialog.vue`, `KbTagManageDrawer.vue`, `FolderPickerMenu.vue` | 多来源上传、文件列表、目的地/配置分段、批量/Shift/框选、标签/文件夹、取消重试、提交去重 | `static` + focused tests；真实上传/解析、浏览器 computed-style、Wails/native `review` |
| 文档预览/处理 | `document-preview.vue`, `ProtectedResourcePreview.vue`, `knowledge-processing-timeline.vue`, `ChatAttachmentPreviewDrawer.vue` | 保护文件、过期/权限、下载、媒体/Office/Markdown、processing trace、失败重解析/取消 | `static` + 局部 browser/domain tests；真实 file proxy、完成态与 provider `review` |
| Wiki/FAQ/图谱 | `WikiFolderActions.vue`, `WikiRevisionDrawer.vue`, `FAQEntryManager.vue`, `KBChunkingDebug.vue`, `GraphSettings.vue` 等 | 文件夹/版本抽屉、编辑/回退、FAQ 导入导出、chunk/图谱调试、空/冲突/错误 | `static` + focused tests/局部 browser；真实写入、图谱 backend、原生 `review`/`blocked-env` |
| Chat | `ChatArtifactsDrawer.vue`, `ChatArtifactsPanel.vue`, `ChatReferencesDrawer.vue`, `SandboxSidePanel.vue`, `ToolApprovalCard.vue`, `McpOAuthCard.vue`, `ContentPopup.vue`, `ChatQuestionMinimap.vue` | 流式/停止/重连、引用、附件、产物预览下载、审批 args、OAuth、终端 WS、错误重试 | `static` + 局部 Web/iOS/Android runtime 历史证据；完整 provider/状态矩阵 `review` |
| Agent/集成/组织 | `AgentEditorModal.vue`, `AgentShareSettings.vue`, `AgentEmbedChannelPanel.vue`, `IMChannelPanel.vue`, `OrganizationEditorModal.vue`, `OrganizationSettingsModal.vue` | 分区编辑器、分享、Embed preview、IM wizard/QR/callback、组织 CRUD/邀请 | `static` + Agent focused/browser 局部；provider/callback/完整 CRUD/原生 `review` |
| 沙箱/技能 | `SandboxConfigEditorDrawer.vue`, `SandboxSkillsPanel.vue`, `SkillFilesDrawer.vue`, `SkillInstallTimeline.vue` | 配置 drawer、连接测试、规则/headers、技能安装进度、文件读取 | `static`; Docker/sandbox backend/真实工具目录 `blocked-env` 或 `review` |
| Embed | `EmbedChannelPreview.vue`, `EmbedPage.vue`, `EmbedChatCore.vue`, `EmbedInputField.vue`, `EmbedBotMessage.vue` | visitor token、CORS/allowed origins、postMessage、流式回答、附件禁用/失败、文件 proxy | `static` + 局部 browser；token 语义/provider `blocked-env` |

## 5. React/多端入口盘点

| 端 | 入口/源文件 | 当前范围 |
|---|---|---|
| Web | `apps/web/src/main.tsx`, `App.tsx`, `routes.tsx` | 认证、平台 shell、KB/文档/Wiki/FAQ/图谱/设置/组织/Agent/Chat/Embed compatibility；route tests 11/11 |
| Desktop | `apps/desktop/src/main.tsx` + renderer | 与 Web 共享页面，Wails platform adapter/Lite reload/data path；仅静态及既有局部 build/runtime 证据，完整 Wails `review` |
| Mobile | Expo `apps/mobile/app/**` 与 `apps/mobile/src/**` | auth/onboarding、KB、文档/详情、Wiki/FAQ/graph、chat、management；移动补齐不等于 Web 路由删除；设备/权限/成功后端链路仍按行验收 |
| Embed | `apps/embed/src/main.tsx` | 独立 visitor/iframe 入口，不共享 bearer 主账号 refresh |
| Shared | `packages/contracts`, `api-client`, `domain`, `views`, `i18n`, `ui`, `design-tokens` | 无 DOM 业务规则、DTO、transport、状态机、视图 registry；静态边界检查与 focused tests 不能替代端运行 |

## 6. 可运行条件与本轮验证

| 检查 | 结果 | 证据层 |
|---|---|---|
| Node / pnpm | Node `v26.7.0`; pnpm `10.28.2` | `static` |
| 依赖目录 | root、`frontend/node_modules`、`apps/web/node_modules` 均存在 | `static` |
| React route tests | 11 passed, 0 failed | `static` test |
| `pnpm typecheck:web` | exit 0 | `static` |
| `pnpm --dir frontend type-check` | exit 0 | `static` |
| Vue dev `:5180` | 未监听（curl connection refused） | `blocked-env` |
| React dev `:5181` | 未监听（curl connection refused） | `blocked-env` |
| 旧/当前后端 `:8091` | 未监听（curl connection refused） | `blocked-env` |
| 后端 `:8080` | HTTP 401；服务可达但当前无认证 | `backend`（仅健康/未认证层） |
| Ollama `:11434` | 未监听 | `blocked-env` |
| 运行进程 | 未发现 vite/weknora/go/expo/metro/wails 进程 | `blocked-env` |

因此本轮没有声称新的 authenticated same-session Vue/React browser parity，也没有声称真实后端业务成功、Wails 交互、iOS/Android 完整验收。已有矩阵中的历史证据仍按其原 evidence 文件和状态使用，不因本轮静态检查自动晋级。

## 7. 未闭环项与下一次执行入口

1. 启动 Vue `:5180`、React `:5181` 与目标后端（或记录实际端口/代理），使用同一测试账号、租户、viewport `1440x900`、语言/主题，逐行补 browser AX/computed-style/interaction evidence。
2. 首先处理矩阵仍为 `implementing` 的深水 surface：N005 分享、N007 上传确认/解析、N016 MCP、R013 integrations/API playground、R027 settings 等；每项独立回归与 review，不批量改状态。
3. 真实 provider/fixture：文件签名与过期、上传/解析队列、MCP/沙箱/图谱、IM QR/callback、Embed preview token、模型 stream/SSE、权限降级和错误反馈。
4. 补 Web → Wails renderer，再补 iOS/Android 设备/模拟器；分别记录 typecheck、bundle/export、native compile、launch、真实交互，不把 Expo export 当 native acceptance。
5. 只有当 route row、设置 section、弹层/抽屉/菜单/预览及其状态 profile 均有对应证据，且无 `blocked-env` 未裁定项，才可考虑从 `review` 晋级 `accepted`；Vue 在全矩阵闭环前继续保留。

## 8. 可追溯文件

- 路由/特殊入口：`docs/migrations/react/route-parity.csv`
- 行级状态与证据：`docs/migrations/react/vue-react-parity-matrix.md`
- 逐轮进度：`docs/migrations/react/vue-react-parity-progress.md`
- 迁移设计与任务图：`docs/superpowers/plans/2026-09-10-react-multiclient-migration.md`
- 静态库存与补充调查：`docs/superpowers/plans/2026-09-10-react-migration-inventory.md`
- Vue router：`frontend/src/router/index.ts`
- Vue settings modal：`frontend/src/views/settings/Settings.vue`
- React route resolver：`apps/web/src/routes.tsx`
- React settings registry：`packages/views/src/settings/registry.ts`
- Integration registry：`packages/views/src/integrations/registry.ts`
