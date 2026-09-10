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
| T01 基线、契约和复用来源冻结 | review | `eb0e9a9` (`docs: freeze React migration baseline and contracts`) | `python3 scripts/generate_react_migration_baseline.py` 0；矩阵自检 0；`go test ./docs` 0；`cd frontend && npm test` 0（804/804）；`npm run type-check` 0；`npm run build` 0；`git diff --check` 0 | 静态：通过；现有 Vue mock/unit：通过；现有 Vue build：通过；真实后端/截图/Wails 包/原生：未完成 | API 行已全量覆盖但 handler DTO/权限逐行仍需审阅；真实后端 smoke 与生成器试点待补；首轮 reviewer 超时并关闭，tester 复核通过 |
| T02 无框架 SDK与第一条真实 API 链路 | review | `e3a3a8f` (`feat: add framework-free shared client foundation`) | `pnpm test:shared` 0（10/10）；`pnpm typecheck:shared` 0；TDD nested-error/late-abort RED→GREEN；真实后端 0 | 静态/Node mock：通过；真实后端 Web：未完成 | 已实现 contracts/api-client/domain 与 KB list mock 链路；仍需接入 Web 调试页、共享认证和真实后端 |
| T03 登录、刷新、OIDC与凭证隔离 | review | `118d15d` (`feat: isolate bearer refresh and embed credentials`) | `pnpm test:shared` 0（15/15）；`pnpm typecheck:shared` 0；auth RED→GREEN；`git diff --check` 0；真实后端/OIDC/浏览器 0 | 静态/Node mock：通过；真实后端、OIDC浏览器回调、React登录页：未完成 | 已实现异步 credential adapter、Bearer 单飞 refresh、Embed 隔离、generation/invalidate 和严格 token 校验；需后续接入 auth endpoints、Web adapter 与真实回调 |
| T04 空间上下文、路由和能力守卫 | review | `9b35b5a` (`feat: guard scoped requests across tenant changes`) | `pnpm test:shared` 0（17/17）；`pnpm typecheck:shared` 0；scope RED→GREEN；`git diff --check` 0；真实路由/后端 0 | 静态/Node mock：通过；真实路由、Web 深链、后端权限：未完成 | 已实现切空间 abort、generation stale guard、logout/invalidate；仍需 Web Router/Query 接入与服务端权限负例 |
| T05 UI基础、平台注入和国际化 | review | `a230366` + workspace integration (uncommitted) | `pnpm test:web` 0（3/3）；`pnpm typecheck:web` 0；`pnpm build:web` 0；`git diff --check` 0 | React/Vite source and focused tests: passed; package installation: passed; live backend/native/i18n coverage: not completed | Added explicit legacy-session adapter, UI Button/Card/Status primitives and React entry; no broad visual redesign. Internationalization package is still pending as a separate slice. |
| T06 知识库列表与创建编辑闭环 | review | `a230366` + workspace integration (uncommitted) | `pnpm test:shared` 0（17/17）；`pnpm test:web` 0（3/3）；`pnpm typecheck:web` 0；`pnpm build:web` 0 | Static/Node mock and production bundle: passed; live backend list/create/edit/browser: not completed | Real `GET /api/v1/knowledge-bases` client path is wired with loading/error/empty states and no fake data. Create/edit and live backend proof remain next work. |
| T07 文档上传、列表、目录标签与预览 | pending | — | — | — | 依赖 T06 |
| T08 FAQ与Wiki编辑/版本 | pending | — | — | — | 依赖 T07 |
| T09 知识库高级配置与数据源 | pending | — | — | — | 依赖 T06 |
| T10 聊天协议、状态机与恢复底座 | pending | — | — | — | 依赖 T04 |
| T11 会话、消息和问答主界面 | pending | — | — | — | 依赖 T05/T10 |
| T12 工具审批、MCP OAuth与运行中追加 | pending | — | — | — | 依赖 T11 |
| T13 Markdown、引用、工具结果与产物 | pending | — | — | — | 依赖 T07/T12 |
| T14 沙箱终端与文件面板 | pending | — | — | — | 依赖 T13 |
| T15 Agent、模型、MCP与Skill配置 | pending | — | — | — | 依赖 T04/T05 |
| T16 空间与组织管理、系统后台 | pending | — | — | — | 依赖 T04/T05 |
| T17 其余设置、Memory与运行配置 | pending | — | — | — | 依赖 T09/T15 |
| T18 Embed、IM与外部集成入口 | pending | — | — | — | 依赖 T12/T13/T15 |
| T19 Wails React renderer与Lite发布 | pending | — | — | — | 依赖 T06；最终依赖 T07–T18 |
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
- 生成 `docs/migrations/react/route-parity.csv`（46 行入口/设置清单）、`api-contract-matrix.csv`（361 行 Swagger 操作）、`reuse-manifest.csv`、`version-matrix.md` 和 `runtime-baseline.md`。
- 运行当前 Vue 基线：`npm ci --ignore-scripts`、804 个前端测试、`vue-tsc --build` 和 Vite 构建均退出码 0；`go test ./docs` 退出码 0。
- 记录 Multica 仅作为架构模式参考；其源码、UI、品牌和业务模型不复制，因根许可证带附加条件而采用 clean-room 路线。
- T01 范围提交：`eb0e9a9`；提交未包含用户提供的三份未跟踪权威输入文档。
- 独立 tester 复核矩阵确定性、361 个 API 行、46 个入口行、6 个复用项和 `go test ./docs`；首轮 reviewer 因超时关闭，未产生可采纳 findings。

### 待完成

- route parity：逐项连接 Vue 入口/旧 URL、Go 注册路由和能力/角色。
- API contract：以 Go 路由、handler DTO/注解和测试为行为权威，分类 Swagger 独有/实现独有/客户端未使用。
- reuse manifest：记录目标/参考源码、固定提交、许可证与复用策略；未核实授权的实现独立编写。
- version matrix：验证 Swagger 2.0 生成路线、Node/pnpm/TypeScript/Vite/React/Expo/Wails 版本和原生兼容性。
- runtime baseline：六种语言、旧 URL、Lite 数据路径、核心截图及真实后端 smoke；无法运行的条件单列为缺失证据。

### T01 review findings

- 计划库存与当前 HEAD 存在 SFC 数量差异（199 → 200），已在运行基线和矩阵中记录。
- Swagger 2.0 行数与当前文档一致（282 paths/361 operations），但这不是运行路由证明；所有 API 行保留 `requires-handler-dto-permission-review` 状态。
- 当前环境没有已配置的 OpenAPI Generator 试点结果、真实后端测试身份、核心截图采集和 Wails/Expo 主机证据；这些缺失不能用已有 Vue 构建替代。

## 变更与提交记录

| 时间 | 变更 | 提交 |
|---|---|---|
| 2026-09-10 | 创建账本；保留用户已有权威文档未提交状态 | 待 T01 范围提交 |
| 2026-09-10 | T05/T06 React Web slice + workspace integration, package install and verification | `a230366` + pending integration commit |
