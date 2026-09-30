# WeKnora 求职专业 Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在同一 WeKnora 身份和个人求职空间中，交付 #140 的 33 个正式子 Issue 与 Web、Expo iOS/Android、鸿蒙原生闸口及微信小程序的完整求职闭环。

**Architecture:** Career Office 拥有求职事实并通过版本化 open/list/act/changes 合同暴露；Career Desk 拥有跨客户端待处理意图、revision 冲突、未知结果恢复及空间切换失效。现有 Identity/Tenant、Task/Run、预算、通知及 Artifact 服务继续拥有各自权威。每个纵向 Ticket 交付可验收行为，平台适配器分别处理文件、分享、通知和受控存储。

**Tech Stack:** Go/Gin/GORM 后端；React/TDesign Web；Expo 55/React Native 0.83.10 iOS/Android；Taro 4.2.1 微信小程序；pnpm 10；Go 与 TypeScript 合同测试。

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`；Issue 快照 `docs/plans/issue-140/issues/`；DAG `docs/plans/issue-140/2026-09-24-issue-140-dag.md`；领域语言 `CONTEXT.md`；ADR 0015–0018。

## Global Constraints

- BASE 是 `f7753fa160927195e388c65e1dbbdff7e288506c`，其来自 `.worktrees/issue30-sweep` 的 `codex/issue30-mobile-office` 已提交 HEAD；该工作区的现场不参与 #140 改动。
- Career Office 仅接受服务端认证所得 User/Tenant 作用域；不从客户端输入读取所有者。
- 上传简历只生成待确认事实；JD 为不可信数据；三值资格判断与技能匹配分离，硬性冲突在显式继续后仍可见。
- 一个岗位和招聘批次只有一个申请与 Task；未知 Task 创建结果必须用同一 request ID 恢复。
- 材料由同一结构化正文生成 PDF/DOCX，真实核验后发布不可变版本；用户本人完成外部投递。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；空间切换后旧响应失效。
- 搜索规则由用户显式启停并经预算准入；额度耗尽仍可读取既有档案和申请。
- 鸿蒙原生验收必须有真实原生构建、设备、受认证 Task 读取和文件/分享/通知/存储探针。Android 兼容包或 WebView 不计。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色；移动端映射原生视觉，不能直接导入移动 Web 组件。
- 不自动投递、发送邮件、跨站填表、绕过 CAPTCHA/登录或读取邮箱推断进展。
- 所有实现者只修改任务所有权内的文件；独立 Worktree 从已记录的集成 HEAD 创建。只在审查并集成前置提交后派发下游。

## Review Focus

1. 同一 request ID 携不同正文重放：拒绝并保持原收据；T03、T14 的合同测试。
2. 空间切换发生在异步读取之后、响应之前：旧响应不回填；T03、T23、T24 的可观察测试。
3. 招聘链接仅返回登录页或摘要：显示不完整并请求 JD，绝不补造条件；T09 测试。
4. 硬条件不符合却匹配技能：冲突仍占主要位置，明确继续后也可追踪；T10、T25、T26 测试。
5. 材料生成单格式成功、另一格式失败：不发布可投递版本；T16 测试。

## Frozen interface and ownership preflight

`packages/career-core/src/contracts.ts` 由 T03 单独创建并拥有。客户端使用以下 TypeScript 合同，后端 JSON 字段与其一致：

```ts
type CareerScope = { tenantId: string; userId: string; epoch: number };
type CareerRef = { kind: "profile" | "opportunity" | "application" | "material" | "rule"; id: string };
type CareerIntent = { kind: string; payload: unknown };
type CareerReceipt = { requestId: string; revision: number; resultRef: CareerRef; status: "applied" | "pending" };
interface CareerRemote {
  open(ref: CareerRef, signal?: AbortSignal): Promise<unknown>;
  list(kind: CareerRef["kind"], cursor?: string, signal?: AbortSignal): Promise<{ items: unknown[]; nextCursor?: string }>;
  act(intent: CareerIntent, requestId: string, expectedRevision: number): Promise<CareerReceipt>;
  changes(cursor?: string, signal?: AbortSignal): Promise<{ events: unknown[]; nextCursor?: string }>;
  receipt(requestId: string): Promise<CareerReceipt | null>;
}
```

T03 的实现必须把 `unknown` 收窄成版本化判别联合类型并在契约测试中固定 wire format；此处的封闭业务 intent 集是 `confirm_fact, import_jd, import_url, assess, search_once, set_rule, create_application, edit_material, publish_material, record_submission, append_progress, correct_progress, set_reminder, export_career, delete_career`。不接受未识别 intent。Go 服务端 scope 从认证中生成，不能信任 `CareerScope` 的客户端字段。

| 共享边界 | 唯一写者 | 消费任务 | 检查 |
| --- | --- | --- | --- |
| Career wire 合同与 DB 迁移 | T03 | T07–T33 | Go/TS fixture 合同测试，revision/request ID 一致 |
| Artifact 固定版本 grant | T04 | T16、T22、T06 | 授权与撤销接口测试 |
| Mobile Runtime 公共合同 | T02 | T05、T23、T25、T27、T29、T31 | T01 只写鸿蒙试验和证据；若必须动共享文件，先串行集成 |
| Mini Task 下载 | T06 | T24、T26、T28、T30、T32 | 小程序服务与真实下载测试 |
| Career Office 各子域文件 | 每个后端 Task 依次持有对应子域 | Web/移动/小程序 | 合同稳定后并行；共享 router/module 注册由主控串行集成 |

预检：33 个 Task 均引用同一 `CareerReceipt` 的 request ID/revision 语义；T03 先冻结合同。T01/T02 的共享 Runtime 风险已列出并禁止同时写。T03/T04 的后端域文件不同，T04 grant 与 T16 的消费方向一致。所有各端业务页面消费同一服务端事实。每个 Task 的失败测试要先呈 RED，再作最小实现与 GREEN；具体命令见各 Task。修复只在本 Task 作用域内进行，提交后由独立 reviewer 给出 Spec 与质量双结论。

## DAG 波次与集成次序

| 波次 | 可独立验收 Task | 集成次序及冲突处理 |
| --- | --- | --- |
| G0 | T01、T02、T03、T04、T06 | T03 合同先合；T04 grant 次之；T02/T06 独立；T01 的真实原生门槛未通过则只记录 blocked，不伪造依赖完成 |
| G1 | T05、T07、T08 | T05 需 T01 与 T02 均 verified；T07/T08 按 Career 迁移和 Office 接口串行集成 |
| G2 | T09、T10 | 来源适配器和资格评估目录不重叠，可独立 Worktree 并行 |
| G3 | T11、T14 | 搜索 Task 与申请 Task 归不同业务目录；同一 Workbench Task 服务测试环境分离 |
| G4 | T12、T13、T15、T17、T23、T24 | 后端各子域可并行；客户端从已集成合同起步，移动受 T05 阻塞 |
| G5 | T16、T20、T21 | PDF/DOCX、提醒、预算各有唯一写者；共享 DB 迁移编号由主控分配 |
| G6 | T18、T22、T29、T30 | 投递、删除与两端规则界面分开；删除测试使用隔离数据库 |
| G7 | T19、T25、T26、T31、T32 | 后端准备与各端不同文件目录；集成后重跑跨端合同 |
| G8 | T27、T28 | Expo 与 Taro 页面独立 |
| G9 | T33 | 统一验收，鸿蒙真实设备缺失则保持 blocked |

并发工作区从当前集成 HEAD 建立，任务提交使用本地 commit。每个流报告 BASE/HEAD、测试、Review Package；主控只 cherry-pick 已通过审查的提交，发生冲突由原任务实现者修复并复审。测试数据库、端口、构建目录各自隔离。不能通过的任务不合入并不解锁下游。

---
### Task 1: T01/#143 鸿蒙原生兼容性闸口

**Depends:** 无。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** 创建 `docs/plans/issue-140/verification/harmony-native-gate.md` 与 `apps/mobile/harmony/` 下的独立探针；不写 `packages/mobile-core/src/`。**Consumes:** 现有 Expo 55、RN 0.83.10 App、受认证 Task read API。**Produces:** 真实 .hap 构建与设备能力矩阵，或可复现 blocked 裁定。**Parallel:** 可与 T02/T03/T04/T06 同波，鸿蒙设备和构建目录独占。

- [ ] 记录 `apps/mobile/package.json`、`app.json`、本机 DevEco/OH SDK/hdc/hvigor、设备系统版本；检索 React Native OpenHarmony 与 Expo native module 兼容矩阵，写来源 URL 和版本。
- [ ] 在独立试验目录创建最小受认证 Task read 原生探针；尝试 `ohpm --version`、`hvigorw assembleHap`、`hdc list targets`。预期：若本机工具缺失，命令失败且报告 blocked，不能视为通过。
- [ ] 若有真实 .hap 与设备，验证登录、跨空间失效、文件、分享、通知、受控存储和越权 Task 拒绝；记录命令、日志、设备标识匿名摘要和结果。没有原生运行证据时，T01 保持 blocked。
- [ ] 独立 validator 检查原生日志与包格式，确认不是 Android APK/WebView；审查后本地提交证据。失败处理：把 T05 及依赖移动 Ticket 标 blocked，保留 iOS/Android 的独立 T02 进展。

### Task 2: T02/#145 Expo iOS/Android 受认证 Task 薄切片

**Depends:** 无。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/task-office-integration-smoke.ts`、`apps/mobile/src/task-office-integration-smoke.test.ts`、`apps/mobile/src/composition.ts`、`packages/mobile-core/src/task-office/`；不写鸿蒙目录。**Consumes:** 现有 OIDC/ScopeLease、Task Office read API。**Produces:** iOS/Android Task 登录、读取、空间切换与错误恢复合同；T05 只消费已集成合同。**Parallel:** 与 T01 并行但不共享 Runtime 文件；模拟器和 Metro 端口单独分配。

- [ ] 在现有 Task Office 集成测试增加未认证、另一 Tenant、会话过期和切换空间后延迟响应场景；运行 `pnpm --filter @weknora/mobile test`，先记录 RED。
- [ ] 修复最小 Runtime/页面接线，使受认证 Task 读取和失败恢复可观察；同一请求返回时检查 scope epoch。
- [ ] 运行 `pnpm --filter @weknora/mobile test` 与 `pnpm --filter @weknora/mobile typecheck`，预期通过；在 iOS 和 Android 开发构建上各取一条真实受保护 Task 读取证据。设备不可用时明确留下环境门槛，不将单测作为设备验收。
- [ ] 自查两端实际版本和跨 Tenant 拒绝，提交本任务范围，由 reviewer 分别给出 Spec/质量结论。

### Task 3: T03/#141 个人求职空间与已确认基础档案

**Depends:** 无。**Owner/validator:** backend_implementer 实现服务端与合同，frontend_implementer 消费稳定合同实现 Web；backend_validator + frontend_validator 验证。**Files:** 创建 `internal/modules/career/`、`packages/career-core/src/contracts.ts`、`packages/career-core/src/desk.ts`、`apps/web/src/career/`；Tenant 开通策略只通过既有 `internal/handler/tenant.go` API 调用。**Consumes:** 认证 User/Tenant、现有 Tenant 自助开通策略。**Produces:** `CareerRemote.open/list/act/changes/receipt`、确认事实的 revision 与 receipt、Web 建档入口。**Parallel:** 与 T04/T06 独立；本任务独占 career wire、迁移、router 注册。

- [ ] 先写 Career Office 公共 seam 测试：未确认提案不能进入评估视图；重复 request ID 只确认一次；同 ID 不同内容拒绝；expected revision 冲突返回当前 revision；跨 User/Tenant 拒绝；延迟响应在 scope epoch 改变后不回填。运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] 创建按 User/Tenant 隔离的事实、提案、请求收据与修订持久模型；服务端从认证上下文派生 scope。Office 返回封闭状态与 typed error；同一事务内写事实和收据，跨服务副作用不伪装为单事务。
- [ ] 在 `packages/career-core/src/contracts.ts` 收窄上述 wire 类型，写 Go/TS 固定 fixture 合同；Desk 的 `act` 在结果未知时先 `receipt(requestId)`，切空间时 abort 并清除缓存。运行 `pnpm exec tsx --test packages/career-core/src/*.test.ts`，预期 GREEN。
- [ ] Web 增加单成员求职空间入口、字段提案/确认/冲突界面；首次开通经既有 Tenant 策略，拒绝/未知结果可见。浏览器验证登录、建档、刷新、跨空间阻断。运行 `pnpm typecheck:web` 与 `pnpm test:web`。
- [ ] 运行 `go test ./internal/modules/career/... ./internal/handler/...`；提交服务端合同后才派发 Web 子任务，合并后做独立双结论 Review。失败处理：任何授权或幂等缺陷阻塞后继 T07/T08。

### Task 4: T04/#142 固定版本 Artifact 授权下载

**Depends:** 无。**Owner/validator:** backend_implementer / backend_validator。**Files:** `internal/modules/workbench/artifact_signing.go`、`internal/handler/session/workbench_artifacts.go` 及相邻测试。**Consumes:** Workbench Artifact 权威与 Tenant/User/Task 授权。**Produces:** 固定资源版本的短时 grant，下载时重新鉴权及撤销拒绝。**Parallel:** 与 T03/T06 独立；T06 暂消费现有下载路径，T16 才消费新 grant。

- [ ] 在公共下载接口测试成功、跨 Tenant、非所有者、过期、篡改、撤销、已删除和旧 Task 产物回归；运行 `go test ./internal/modules/workbench/... ./internal/handler/session/...`，先见 RED。
- [ ] 给 grant 增加确定版本、资源和所有者绑定，签发与每次下载均查询当前授权/撤销态；拒绝时不泄露资源存在性。
- [ ] 以一个真实产物从 Web 下载并比对 SHA-256；运行上述 Go 测试，预期 GREEN。保存证据并提交，reviewer 检查签名规范化、撤销竞态和兼容旧路径。
- [ ] 失败处理：新 grant 不可用于 T16；现有 Task 下载回归必须修复后合入。

### Task 5: T06/#148 微信小程序 Task 产物下载

**Depends:** 无。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/subpackages/execution/artifact/index.tsx`、`apps/miniprogram/src/services/workbench.ts`、`apps/miniprogram/tests/` 内相应测试。**Consumes:** 已有 Task artifact 获取/下载接口；T04 的固定版本 grant 合入后追加兼容合同。**Produces:** Taro 4 真机可认证下载与失败/过期可恢复 UI。**Parallel:** 与后端 T03/T04 独立；独占小程序构建目录。

- [ ] 增加产物列表→下载→本地打开的合同测试，覆盖无权限、过期链接、网络未知、空间切换；运行 `pnpm --filter @weknora/miniprogram test`，先见 RED。
- [ ] 接线 Taro 下载与文件打开适配器，按用户动作取授权，不长期缓存 URL；下载前后检查 scope epoch，受控删除临时文件。
- [ ] 运行 `pnpm --filter @weknora/miniprogram test`、`typecheck`、`build:weapp`；用真实微信开发者工具/设备下载一个受保护 Task Artifact，比对内容摘要和权限拒绝。缺真机时该验收单列 blocked。
- [ ] 提交独立分支，reviewer 检查错误提示、权限、临时文件；集成 T04 后重跑下载合同。

### Task 6: T05/#144 Expo 移动端 Task Office 可进入与恢复

**Depends:** #143、#145。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/screens/TasksScreen.tsx`、`apps/mobile/src/composition.ts`、`apps/mobile/src/task-office-integration-smoke.test.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** Career Desk 之外的现有 Task Office open/list。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** Expo 移动运行面替换占位首页，提供可用 Task 入口；刷新或重启后不创建新 Task，只恢复原 Task；活动空间切换时旧 Task 数据和迟到响应不显示；离线显示受控缓存或明确不可用状态，不暗示任务成功

- [ ] **Step 1:** 给进入、退出、恢复现有 Task 页面写失败测试，覆盖空间失效和未知读取；先运行上述命令确认 RED。
- [ ] **Step 2:** 接入 T02 已冻结的认证读取合同，以原生页面展示 loading/denied/retry；不改 Career Office 业务合同。
- [ ] **Step 3:** 在 iOS/Android 实机构建各读一条受保护 Task；鸿蒙必须使用 T01 原生验收路径。运行上述测试命令和平台构建，预期全部通过。
- [ ] **Step 4:** 审查导航可达性与 scope 失效；没有 T01 原生证据时保持 blocked，不解锁移动求职 UI。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Mobile Runtime/Task Office 公共 seam 测试覆盖恢复和 scope 失效。 - iOS、Android 开发构建与鸿蒙可用构建各演示一条现有 Task；鸿蒙不支持时按 #143 判定阻塞。。**失败处理：** 网络超时保持未知并允许重新同步，不自动重复执行。

### Task 7: T07/#147 简历上传、逐步建档与事实确认

**Depends:** #141；Web 子任务还需 T03 Career Web 路由/API 经审查并集成。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/profile_intake.go`、`profile_intake_test.go`、`model_input.go`、相邻 Career handler/test；共享解析/存储的窄 Adapter 由独立前置子任务拥有；`packages/career-core/src/contracts.ts` 与 fixture；`apps/web/src/career/profile_intake.tsx` 和测试。**Consumes:** T03 已确认事实、提案、修订、收据与 owner-only scope；现有文件校验、存储和解析能力。**Produces:** Career 持久来源版本、整批提案收据、单项确认/拒绝及目的限定的已确认事实模型输入。**Parallel:** T07 后端与 T08 后端共享 Career 表/合同/迁移，串行；Web 工作在 T03 路由稳定后进行。

**验收：** 教育、经历、项目、技能、成果数字与证书均有来源和确认状态；拒绝或未确认的提案不能用于评估或材料生成；上传失败不覆盖已确认档案，修改保留版本和来源；送模型前遮蔽与当前目的无关的证件号码等字段

- [ ] **Step 1:** 以 `docs/plans/issue-140/task-7-research.md` 与 `task-7-architecture.md` 为接口依据，先冻结 Career 来源记录、字段 key/类别、batch receipt、解析失败状态及私有资源句柄的 Go/TS wire fixture。抽取/存储 Adapter 仅暴露经校验的 scoped 存储结果与文本或 typed 失败，不把 24 小时 Session 临时文档 ID 当权威来源；预检共享 `container.go`、合同及迁移版本归属。
- [ ] **Step 2:** RED：用教育、经历、项目、技能、数字成果、证书、缺失毕业时间、冲突任职日期和证件号 fixture，写来源不可变、整批提案原子性、相同 request ID 重放/不同正文冲突、expected revision、跨 Tenant/owner 拒绝、解析失败不改变已确认事实和历史的 Career 公共 seam 测试。运行 `go test ./internal/modules/career/...`，预期这些新测试先失败。
- [ ] **Step 3:** GREEN：实现 Career 自有来源版本和 `processing|ready|failed` 状态；Blob/解析与 DB 不共享事务时用 sourceID 对账，完成批次与收据同事务写入，逐项复用 T03 `confirm_proposal`/`dismiss`。原始简历和未脱敏抽取文本只在私有来源路径读取；普通档案视图只暴露提案、来源和已确认事实。失败时保留此前事实；绝不把模型抽取直接确认为权威事实。
- [ ] **Step 4:** 在服务端模型输入构造 seam 从已确认事实按用途白名单选取字段，并在最终序列化前遮蔽无关证件号码；测试最终载荷确实不含证件号、未确认/已拒提案或原始简历。固定原始文件保留时长不在本 Task 凭空制定，删除能力留给 T22 的保留策略。
- [ ] **Step 5:** T03 Web 路由与 API 经审查集成后，RED/GREEN 实现上传进度、处理中/失败/重试、缺失和冲突提案逐项确认、旧事实与来源版本展示；可观察测试覆盖失败替换不清空档案、scope 切换清理、修订冲突和收据恢复。运行 `pnpm typecheck:web`、相关测试、`pnpm test:web`、`pnpm build:web`。
- [ ] **Step 6:** 在隔离的本地服务/浏览器用上述 fixture 走上传、审阅、确认、刷新，并保存原始命令/状态/摘要与跨 Tenant 拒绝；运行 Career、handler、数据库迁移测试。后端/前端验证者分别核验后由独立 reviewer 给出 Spec 与质量双结论，再放行 T08/T15 消费该事实合同。

**验证命令：** `go test ./internal/modules/career/... ./internal/handler/... ./internal/database/...`；`pnpm typecheck:web && pnpm test:web && pnpm build:web`；隔离服务上的浏览器上传/审阅/刷新/越权记录。**原始证据：** Career Office 来源版本、批次幂等、失败不覆盖、模型输入脱敏测试及样例简历浏览器链路。**失败处理：** 上传/抽取失败保留旧事实和可见失败状态，允许手动建档；Blob 结果未知时以 sourceID 对账，不能重复产生提案。

### Task 8: T08/#146 粘贴 JD 形成岗位机会与快照

**Depends:** #141 完整验证并集成，且 T03 SQLite 启动修复通过。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/opportunity.go` 与测试、Career handler/read 路由、机会迁移、`packages/career-core/src/contracts.ts` 与 fixture、`apps/web/src/career/opportunity.tsx` 与测试；共享 Chat host 路由由主控协调。**Consumes:** T03 owner-only scope、request-ID 收据及已集成 Web Career 路由。**Produces:** 独立于 profile revision 的 `import_jd` 类型化命令、`opportunity_imported` 收据与按 opportunityId/snapshotId 读取的证据视图。**Parallel:** 与 T07 后端共享 Career 合同/迁移而串行；Web 在后端类型合同经审查集成后实现。

**验收：** 原文与抽取字段分开保存，缺字段标为未知；同一请求 ID 重放返回同一机会或回执；岗位原文中的指令不能获得 Agent 工具权限；页面可从对话结果进入岗位证据详情

- [ ] **Step 1:** 按 `docs/plans/issue-140/task-8-research.md` 和 `task-8-architecture.md` 冻结 Go/TS wire：精确 `rawText`、`manual_paste` 来源、服务器取得时间、显式 known/unknown 字段、稳定 opportunity/observation/snapshot ID、`stored|needs_review` 状态。`import_jd` 不是 T03 profile `act(expectedRevision)`，不能传虚构的档案 revision。
- [ ] **Step 2:** RED：在 Office/HTTP 公共 seam 测试同 ID 同正文仅一份快照与原回执、同 ID 改正文冲突、跨 User/Tenant 拒绝、缺字段为 unknown、提取失败仍能按固定 snapshotId 重新打开原文、后来观察不改旧快照、恶意 JD 不触发工具/网络/授权；运行 `go test ./internal/modules/career/...`，预期新测试失败。
- [ ] **Step 3:** GREEN：在同一数据库事务持久化机会、来源观察、不可变 JD 快照和收据；先按 scoped request ID 查重放，再做保守抽取。原文不作为系统提示、工具参数或 URL 抓取目标；抽取不确定时返回 `needs_review` 且保留原文。独立机会读接口按 ID 与 snapshotId 鉴权，profile revision 和 `/changes` 不因 JD 导入推进。
- [ ] **Step 4:** 写 Go/TS 固定 fixture 与解码测试；异常网络/提交结果以原 request ID 查收据。Web RED/GREEN：C 对话结果仅携类型化 opportunityId/snapshotId 的显式动作，证据页面转义展示原文、来源、取得时间和未知标记；复制/刷新/重新打开仍指向原快照。不要从助理文本或 JD 解析路由。
- [ ] **Step 5:** 跑 Career/handler/数据库/路由测试及 `pnpm typecheck:web && pnpm test:web && pnpm build:web`；在隔离服务与浏览器走粘贴→对话结果→证据→刷新、恶意 JD 与跨 Tenant 拒绝。后端/前端验证者分别核验，独立 reviewer 给出 Spec/质量双结论后集成。

**验证命令：** `go test ./internal/modules/career/... ./internal/handler/... ./internal/database/... ./internal/router/...`；`pnpm typecheck:web && pnpm test:web && pnpm build:web`；浏览器证据链。**原始证据：** 固定 ID/摘要、原文与抽取分离、同 ID 重放、恶意 JD 无副作用、Web 粘贴到再次打开完全一致。**失败处理：** 提取失败保留原文和 `needs_review`，不生成虚构岗位条件；提交未知时按 request ID 对账。

### Task 9: T09/#149 链接导入与不完整来源回退

**Depends:** #146。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/source_import.go`、`internal/modules/career/service/source_import_test.go`、`apps/web/src/career/source_import.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "import_url", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 只接入经核验允许读取的来源，不绕过登录或反爬限制；保留原链接、取得时间、完整性及失败原因；摘要不足以推断届别、学历等硬条件；用户补充 JD 后产生新的固定快照，原始来源仍可追溯

- [ ] **Step 1:** 先在 `source_import_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `source_import.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `source_import.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - 固定来源响应契约覆盖完整、登录阻断、摘要、不存在和超时。 - Web 演示链接失败后粘贴 JD 成功。。**失败处理：** 来源未知时不标为已核验；重试不覆写历史快照。

### Task 10: T10/#150 三值资格与证据化匹配

**Depends:** #146。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/assessment.go`、`internal/modules/career/service/assessment_test.go`、`apps/web/src/career/assessment.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "assess", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 2026 届对仅限 2027 届为不符合，缺毕业日期为待确认；每项资格指向岗位快照与已确认档案版本；总分不表述为录用概率，不符合不能被其他高分掩盖；用户改档案后可生成新评估，旧评估仍可读取

- [ ] **Step 1:** 先在 `assessment_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `assessment.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `assessment.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 公共 seam 场景测试覆盖符合、不符合、未知和版本变化。 - Web E2E 检查警示、证据与来源在窄布局可见。。**失败处理：** 模型无法结构化解释时显示评估待复核，不假装符合。

### Task 11: T11/#152 C 对话入口的一次性真实找岗

**Depends:** #149、#150。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/search_once.go`、`internal/modules/career/service/search_once_test.go`、`apps/web/src/career/search_once.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "search_once", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 找岗默认一次性 Task，指令不自动创建持续规则；首批来源明确实际可用方式与城市覆盖，不宣称全国完整；结果列出检查时间、资格状态、原始链接及不确定性；执行失败、未知回执和额度拒绝均可恢复，不复制搜索 Task

- [ ] **Step 1:** 先在 `search_once_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `search_once.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `search_once.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - 固定来源与真实允许来源各跑一次合同场景。 - Web E2E 从指令到岗位详情再到恢复历史结果。。**失败处理：** 外部来源不可用时说明范围与失败，不以演示数据替代。

### Task 12: T12/#151 多来源去重、岗位更新与覆盖说明

**Depends:** #152。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/reconciliation.go`、`internal/modules/career/service/reconciliation_test.go`、`apps/web/src/career/reconciliation.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "import_url", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 仅有充分岗位编号、企业、地点与批次证据时合并；不确定重复并列，所有原始链接和检查时间保留；过期、下架和要求变化显式标注，旧申请仍展示旧快照；可查看已接入来源和实际覆盖城市

- [ ] **Step 1:** 先在 `reconciliation_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `reconciliation.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `reconciliation.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 合同测试覆盖同岗双来源、同名不同批次和 JD 更新。 - Web 展示变化前后差异且历史可访问。。**失败处理：** 来源检查失败保留最后成功观察并显示陈旧时间。

### Task 13: T13/#154 可控的持续找岗规则

**Depends:** #152。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/search_rule.go`、`internal/modules/career/service/search_rule_test.go`、`apps/web/src/career/search_rule.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "set_rule", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 未开启不后台运行，暂停后下一次触发被取消或不再入队；启用前展示条件、频率与预计消耗；同一新岗位只产生一个发现待办；重复触发和结果未知经同一请求身份对账

- [ ] **Step 1:** 先在 `search_rule_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `search_rule.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `search_rule.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 定时触发测试覆盖开关、重复与未知状态。 - Web E2E 修改规则后显示下次运行计划。。**失败处理：** 预算不足或来源不完整形成可见状态，不静默跳过。

### Task 14: T14/#155 每份求职申请关联独立 Task

**Depends:** #150。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/application.go`、`internal/modules/career/service/application_test.go`、`apps/web/src/career/application.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "create_application", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 申请固定所用岗位快照、档案版本与资格评估；硬条件不符可显式继续，但警示常驻且不计合格申请指标；跨模块 Task 建立未知时保留 linking 状态并用原请求 ID 对账；同岗不同批次可分别申请，重复请求不创建第二个 Task

- [ ] **Step 1:** 先在 `application_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `application.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `application.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 公共 seam 测试覆盖正常、超时、重复、跨批次和警示。 - Web E2E 从岗位卡打开独立申请。。**失败处理：** Task 创建失败保留可恢复申请，不将未关联状态显示为就绪。

### Task 15: T15/#153 可信结构化材料与不可变版本

**Depends:** #147、#155。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/material.go`、`internal/modules/career/service/material_test.go`、`apps/web/src/career/material.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "edit_material", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 生成前冻结岗位与档案版本，主张链接到确认事实；缺失实习、证书、数字不得由模型补造，审阅指出风险；用户确认正文后形成新不可变版本，旧版本可比较；三端共同编辑的是结构化正文，修改后不得覆盖旧投递版

- [ ] **Step 1:** 先在 `material_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `material.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `material.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 公共 seam 测试覆盖未确认事实、伪造成果、版本冲突。 - Web E2E 修改正文并比较 V1/V2。。**失败处理：** 生成或审阅失败保留草稿与原因，不发布可投递版本。

### Task 16: T16/#158 同版 PDF/DOCX 生成与验证

**Depends:** #142、#153。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/rendering.go`、`internal/modules/career/service/rendering_test.go`、`apps/web/src/career/rendering.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "publish_material", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 两种文件绑定同一正文摘要与材料版本；PDF 校验可提取文本与页面版面，DOCX 校验完整与可编辑性；两种验证均通过后版本才可标记可用于投递；旧版本保持可下载，删除或撤销后旧授权立即失效

- [ ] **Step 1:** 先在 `rendering_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `rendering.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `rendering.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - 真实渲染器验收检查 PDF 文本与页面、DOCX 内容与编辑。 - Workbench 授权测试和 Web 下载 E2E。。**失败处理：** 单一格式失败保留 staged 状态和错误，不只发布成功的一半为可投递。

### Task 17: T17/#157 申请进展事件与阶段投影

**Depends:** #155。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/progress.go`、`internal/modules/career/service/progress_test.go`、`apps/web/src/career/progress.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "append_progress", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 事件追加保存，纠错追加更正事件而非覆盖原记录；当前阶段由已确认事件投影，重新打开结果一致；同一请求重放不出现第二个事件；事件来源和确认者可追溯，跨申请记录不串联

- [ ] **Step 1:** 先在 `progress_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `progress.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `progress.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 合同测试覆盖事件顺序、更正、幂等、阶段投影。 - Web E2E 录入面试后筛选并查看历史。。**失败处理：** 写入结果未知先查回执，不用新 ID 盲目重试。

### Task 18: T18/#159 本人投递确认与实际材料绑定

**Depends:** #158、#157。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/submission.go`、`internal/modules/career/service/submission_test.go`、`apps/web/src/career/submission.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "record_submission", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 系统不点击外部提交或自动发信；实际版本可选择已验证版，未知时明确记录未确认；投递事件绑定渠道、时间、版本或未知标记；重复确认不生成第二次投递记录，后续准备不引用错误版本

- [ ] **Step 1:** 先在 `submission_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `submission.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `submission.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office seam 测试已知/未知版本与重复回执。 - Web E2E 记录投递并回看版本及时间线。。**失败处理：** 外部提交是否成功由用户确认；产品不得从点击下载推断投递。

### Task 19: T19/#156 求职信与基于投递版的面试准备

**Depends:** #159。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/preparation.go`、`internal/modules/career/service/preparation_test.go`、`apps/web/src/career/preparation.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "edit_material", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 求职信与回答只引用已确认事实和岗位快照；面试准备优先固定实际投递版本，版本未知必须提示；生成结果可审阅、修订且有来源；不自动发送或替用户承诺事实

- [ ] **Step 1:** 先在 `preparation_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `preparation.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `preparation.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office 测试证明 V2 已投递时引用 V2、未知时不猜 V3。 - Web E2E 查看来源并修订草稿。。**失败处理：** 模型失败保留请求和可恢复状态，不展示空白成功产物。

### Task 20: T20/#160 站内待办与隐私通知

**Depends:** #154、#157。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/reminder.go`、`internal/modules/career/service/reminder_test.go`、`apps/web/src/career/reminder.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "set_reminder", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 同一机会和事件只产生一条待办；站内记录是权威，推送只提醒重新同步；通知正文无公司、岗位、面试细节；取消订阅后不再发送，已存在待办仍可读取

- [ ] **Step 1:** 先在 `reminder_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `reminder.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `reminder.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Inbox/通知契约测试去重、隐私文案和取消订阅。 - Web E2E 从待办进入权威申请。。**失败处理：** 通知送达失败不丢站内事实，也不将推送视为状态更新。

### Task 21: T21/#161 搜索与生成的额度预估及阻断

**Depends:** #154、#153。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/usage.go`、`internal/modules/career/service/usage_test.go`、`apps/web/src/career/usage.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "search_once", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 执行前展示将消耗的额度与触发条件；超额阻止新的收费 Run 而非删除既有档案或申请；重复请求不会重复预占或收费；付费状态不改变岗位排序或资格判定

- [ ] **Step 1:** 先在 `usage_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `usage.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `usage.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - admission 合同测试额度边界、重复请求和只读访问。 - Web E2E 模拟超额后仍能打开旧申请。。**失败处理：** 预估不可得时不得先执行后补报，显示可理解的不可用原因。

### Task 22: T22/#162 求职数据导出与完整删除

**Depends:** #158、#157。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/career_export.go`、`internal/modules/career/service/career_export_test.go`、`apps/web/src/career/career_export.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "export_career", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 导出包含档案、原岗位快照、申请事件和材料版本；删除前说明空间内与外部平台资料的边界；删除使旧 Task、Artifact 授权和客户端缓存不可再访问；若保留策略要求延迟或例外，显示范围与状态

- [ ] **Step 1:** 先在 `career_export_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- [ ] **Step 2:** 在 `career_export.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- [ ] **Step 3:** 为 Web `career_export.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- [ ] **Step 4:** 运行 `go test ./internal/modules/career/...` 与 Web 检查；保存 API fixture、数据库迁移及浏览器证据；后端和前端分别验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令：** `go test ./internal/modules/career/...`；`pnpm typecheck:web && pnpm test:web`。**原始证据：** - Career Office/Identity/Workbench 联合合同测试导出完整性和旧链接失效。 - Web E2E 导出后发起删除并检查不可读取。。**失败处理：** 局部删除失败保留可恢复状态与审计，不声称已完全删除。

### Task 23: T23/#163 Expo 移动端 C 找岗、建档与分享导入

**Depends:** #144、#152。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/career/discovery.tsx`、`apps/mobile/src/career/discovery.test.tsx`、`apps/mobile/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 复用同一 WeKnora 身份与 Tenant，不建本地权威副本；可上传已有简历或逐步建档，并逐项确认抽取事实；系统分享进入的链接/JD 可核对后导入；窄屏持续展示硬条件、依据、来源和待核实项；跨端修改档案后读到同一确认版本

- [ ] **Step 1:** 先在 `discovery.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Career Desk 合同测试 scope 切换与跨端版本。 - iOS、Android、鸿蒙各跑分享导入到岗位评估的设备流程。。**失败处理：** 离线输入只保存草稿，联网后需用户确认提交。

### Task 24: T24/#164 微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入

**Depends:** #148、#152。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/discovery.tsx`、`apps/miniprogram/src/career/discovery.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；微信身份只作为关联方式，不生成第二份求职档案；可上传已有简历或逐步建档，并逐项确认抽取事实；分享导入先展示可核对内容再提交；资格冲突、来源和待核实项在窄屏可见；同一用户 Web 的档案变更可在微信小程序（Taro 4 + TDesign Miniprogram）同步看到

- [ ] **Step 1:** 先在 `discovery.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** - Career Desk 合同测试身份绑定与版本变化。 - 微信小程序（Taro 4 + TDesign Miniprogram）真机或官方环境跑完整找岗流程。。**失败处理：** 分享负载缺失或授权失效时展示恢复入口，不用演示数据冒充结果。

### Task 25: T25/#165 Expo 移动端申请、材料与本人投递

**Depends:** #159、#163。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/career/application-material.tsx`、`apps/mobile/src/career/application-material.test.tsx`、`apps/mobile/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 同岗不同批次申请分别展示；材料修改产生新版本且两种导出可下载；记录实际投递版或未知，警示硬条件冲突；离线草稿不在重联网后静默确认投递

- [ ] **Step 1:** 先在 `application-material.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Career Desk 跨端版本与未知回执测试。 - iOS、Android、鸿蒙各完成岗位到投递确认设备流程。。**失败处理：** 下载失败或提交结果未知保留原版本和待核对状态。

### Task 26: T26/#166 微信小程序（Taro 4 + TDesign Miniprogram）申请、材料与本人投递

**Depends:** #148、#159、#164。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/application-material.tsx`、`apps/miniprogram/src/career/application-material.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；与 Web、移动 App 使用同一申请和结构化正文版本；PDF/DOCX 实际下载且内容对应确定版本；硬条件不符时警示常驻，用户显式继续；投递确认不触发招聘平台自动提交

- [ ] **Step 1:** 先在 `application-material.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** - Career Desk 版本冲突测试。 - 微信小程序（Taro 4 + TDesign Miniprogram）真实环境完成岗位到文件下载和投递确认。。**失败处理：** 下载授权过期可重取；结果未知先对账。

### Task 27: T27/#167 Expo 移动端申请时间线与按需准备

**Depends:** #156、#165。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/career/progress-preparation.tsx`、`apps/mobile/src/career/progress-preparation.test.tsx`、`apps/mobile/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 事件历史不可覆盖，阶段与 Web 同源；跨端新增事件后 Expo 移动端重新同步显示新阶段；面试准备显示所引用的实际投递版；未知版本不推断招聘方看到最新材料

- [ ] **Step 1:** 先在 `progress-preparation.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Career Desk 事件投影与版本引用测试。 - iOS、Android、鸿蒙各演示投递后面试与更正。。**失败处理：** 离线事件草稿需联网后确认，未知回执不重复提交。

### Task 28: T28/#169 微信小程序（Taro 4 + TDesign Miniprogram）申请时间线与按需准备

**Depends:** #156、#166。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/progress-preparation.tsx`、`apps/miniprogram/src/career/progress-preparation.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；事件列表与其他端共享权威顺序；跨端更正后旧事件仍可追溯；准备内容引用确定投递版，未知时提示；微信小程序（Taro 4 + TDesign Miniprogram）切换账号不显示前一用户缓存

- [ ] **Step 1:** 先在 `progress-preparation.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** - Career Desk 事件与 scope 测试。 - 微信小程序（Taro 4 + TDesign Miniprogram）真实环境录入、修改并查看面试准备。。**失败处理：** 网络断开只保留本地草稿，不静默改申请状态。

### Task 29: T29/#168 Expo 移动端持续规则、额度与提醒

**Depends:** #154、#160、#161、#163。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/career/rules-usage-reminders.tsx`、`apps/mobile/src/career/rules-usage-reminders.test.tsx`、`apps/mobile/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 规则暂停后不再产生新搜索运行；收费执行前显示额度与预估，超额仍可读历史；系统推送不暴露公司、岗位或面试详情；空间切换和登出清理受控缓存

- [ ] **Step 1:** 先在 `rules-usage-reminders.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Career Desk 规则与额度测试。 - iOS、Android、鸿蒙各验证暂停、超额、通知权限和锁屏文案。。**失败处理：** 推送权限拒绝不妨碍站内待办。

### Task 30: T30/#170 微信小程序（Taro 4 + TDesign Miniprogram）持续规则、额度与提醒

**Depends:** #154、#160、#161、#164。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx`、`apps/miniprogram/src/career/rules-usage-reminders.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；规则与 Web、移动 App读写同一版本；超额不阻断历史访问；订阅消息只提醒同步，不携带敏感岗位详情；拒绝订阅后站内待办仍可用

- [ ] **Step 1:** 先在 `rules-usage-reminders.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** - Career Desk 规则冲突测试。 - 微信小程序（Taro 4 + TDesign Miniprogram）真实环境检查订阅授权、提醒与隐私文案。。**失败处理：** 订阅消息不可用时显示站内待办，不伪造已送达。

### Task 31: T31/#171 Expo 移动端导出与删除

**Depends:** #162、#163。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/mobile/src/career/export-deletion.tsx`、`apps/mobile/src/career/export-deletion.test.tsx`、`apps/mobile/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 导出包包含档案、岗位快照、申请事件和材料；删除说明外部平台内容不受影响；完成删除后缓存不可读，旧材料下载授权失效；权限或保留策略阻断时显示确切状态

- [ ] **Step 1:** 先在 `export-deletion.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`。**原始证据：** - Career Desk scope/删除测试。 - iOS、Android、鸿蒙各演示导出、删除和旧链接拒绝。。**失败处理：** 下载中断不显示完整导出；删除未知先对账。

### Task 32: T32/#173 微信小程序（Taro 4 + TDesign Miniprogram）导出与删除

**Depends:** #162、#164。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/export-deletion.tsx`、`apps/miniprogram/src/career/export-deletion.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；导出内容与其他端一致；删除说明保留规则和外部平台边界；删除后微信账号重进不恢复旧私有资料；旧材料链接和本地缓存不可访问

- [ ] **Step 1:** 先在 `export-deletion.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp` 确认 RED。
- [ ] **Step 2:** 接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，不以原型静态数据代替。
- [ ] **Step 3:** 运行 `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`，预期 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- [ ] **Step 4:** 自查版本与缓存失效，独立 frontend_validator 验证，reviewer 做 Spec/质量双结论；未通过真实设备门槛则保留 blocked。

**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** - Career Desk 删除与重新登录测试。 - 微信小程序（Taro 4 + TDesign Miniprogram）真实环境检查导出、删除、重进和旧授权。。**失败处理：** 导出能力受平台限制时提供等效可取得的文件流程，不静默截断。

### Task 33: T33/#172 五环境真实闭环与发布门槛

**Depends:** #151、#165、#166、#167、#169、#168、#170、#171、#173。**Owner/validator:** implementer / backend_validator + frontend_validator。**Files:** `docs/plans/issue-140/verification/launch-matrix.md`、`docs/plans/issue-140/verification/source-coverage.md`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** 所有 CareerRemote 合同与平台能力适配器。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 五个实际环境均跑完整链路和失败恢复；分享导入、PDF/DOCX 下载、通知权限、跨端同步与越权检查有可复现记录；公布真实岗位来源、覆盖城市、使用条件与数据处理说明；来源、模型、微信能力、个人信息和收费流程的运营核验完成

- [ ] **Step 1:** 建立 Web、Expo iOS、Expo Android、HarmonyOS 原生、微信小程序五环境的同一档案→岗位→评估→申请→材料→本人投递→进展验收矩阵，每格包含版本、设备、命令与证据。
- [ ] **Step 2:** 逐环境运行真实服务端闭环与文件、分享、通知、scope 切换、未知结果恢复；岗位来源列原始链接、获取条件、城市覆盖和失败态。
- [ ] **Step 3:** 运行完整 Go/TS/Lint/构建及设备检查；预期全通过。缺鸿蒙原生或微信真机等环境时将对应格记 blocked，不宣称公开门槛完成。
- [ ] **Step 4:** 独立最终 Review 和 OCR 覆盖原始 BASE..HEAD；保存合规/来源公开门槛与全部 blocker，只有全部通过才标 verified。

**验证命令：** `go test ./internal/modules/career/... && pnpm typecheck:web && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/miniprogram typecheck`。**原始证据：** - 保存各端操作记录、服务端合同结果与产物检查摘要。 - 最终差异审查与父规格 43 条故事覆盖核对。。**失败处理：** 任何真实阻塞缺口保留未完成，不以 Web 通过代表其他目标通过。
