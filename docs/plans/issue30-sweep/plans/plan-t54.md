# T24：GitLab 草稿 MR 交付闭环（Issue #54）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 GitLab 个人连接与 GitHub 走**同一条统一 Delivery seam**——基线物化、工作区 diff、A03 审批锚定、任务分支收敛推送、草稿 MR——GitLab 的全部平台差异（project 路径寻址、commits API 单次收敛、Draft: 标题前缀、通配保护分支、服务端自定提交 SHA）被 GitLab Adapter 隐藏在 seam 之后，权限/幂等/版本不变量与 GitHub 完全一致。

**Architecture:** #52（T22）已交付统一交付缝：`internal/modules/codedelivery` 深模块（`GitHubClient` 平台端口 + `CodeDeliveryService` 编排 + `DeliveryDispatcher` A03 派发器）+ `code_deliveries` 追溯表 + A03 Action 审批管线 + workbench HTTP 面 + 移动只读投影。本计划在该缝内做三件事：①以 `CodePlatformClient = GitHubClient` 类型别名确立**平台中立词汇**，新增 GitLab REST v4 适配器（`gitlab_client.go`）实现同一端口——交付链代码零平台分支；②提供者从连接的安装 app id（`connections.installation_id → installations.app_id`）**服务端权威解析**（绝不取自客户端输入），A03 action target 随之取 `github.deliver|gitlab.deliver`，派发器按 target 路由到对应工厂；③推送半程改为推送后 `BranchHead` **权威读回**——GitHub 上等于 CreateCommit 结果，GitLab 上消除「本地占位 SHA 进入台账」的可能，两侧回执都以远端事实落账。GitLab 的推送半程由适配器折叠为**一次 commits-API 调用**，把任务分支收敛到「基线树 + 批准变更」的预期树（GitLab 无客户端可控 parent、无独立 blob/tree 创建端点；commits API 永不改写历史——与 GitHub `force:false` 同一不变量、不同机制，差异由 Adapter 隐藏）。测试面沿用 #52 确立的最高稳定 Interface：真实 sqlite + 真实 A03 ActionService/ocAuthorizer + httptest GitLab 模拟器真实 HTTP 字节 + 本地目录工作区。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`；go.mod `go 1.26.0`，本机 go1.26.3 实测）。TypeScript 侧本计划**零新增代码**（统一回执词汇 `pr_number/pr_url` 已是平台无关投影；仅 3 处中文文案中性化）。所有命令在 worktree 根（`.worktrees/issue30-sweep`）执行；`pnpm install` 已就绪。本计划作者在当前 HEAD（d52270a0f）实跑基线：`go test ./internal/modules/codedelivery/ -count=1` → ok（0.793s）；`pnpm --filter @weknora/mobile test` → 215 pass / 0 fail / 11 skipped（9.1s）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-54.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions：「Developer supports personal and Tenant GitHub/GitLab connections, task branches and draft PR/MR only. **It does not merge automatically or expose remote credentials to Shell.**」「Lead Agent Version, Artifact versions, Action Plans and **candidate code commits are immutable approval anchors. Changes invalidate prior approvals.**」「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」；User Story 41 固定基线；Testing Decisions「True external dependencies … use mock or scripted Adapters at the Port and real-device acceptance separately」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§12「Port 放在拥有行为的 Module 一侧」——GitLab 适配器在 codedelivery 模块内；§13 Interface 测试面）
- ADR：`docs/adr/0008-developer-delivery-and-single-writer.md`（「首版代码交付仅推送任务分支并创建或更新草稿 GitHub PR/GitLab MR，不写受保护分支、不自动合并……记录发起者、批准者和实际远端身份」）
- 领域术语：`CONTEXT.md`（「代码平台连接」：个人连接只能由其所有者使用，每次远端写入记录发起成员、批准成员与实际远端身份；「代码交付审批」：待交付内容变化后需要重新审批；「代码交付」：避免自动合并、把补丁生成等同于已远端交付、在推送与创建 PR/MR 部分成功后盲目重试全部步骤；「工作区」）
- Parent：Issue #30；Blocked by：#52（**已完成并集成**——调查称其 absent 已过时，当前 HEAD `internal/modules/codedelivery/` 全套在库，本计划全部 Consumes 均为亲眼核实）

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「GitLab 差异被 Adapter 隐藏。」（Issue #54 验收标准 1 原文）——交付链（service/dispatcher/store/HTTP/移动面）**零 GitLab 条件分支**；唯一的平台 switch 是 `clientForPlatform`（本计划 Task 1 的唯一路由函数）；GitLab 形状（project %2F 寻址、Bearer OAuth 头、commits API 单次收敛、`Draft: ` 标题前缀、`source_branch` 裸分支名、通配保护分支、服务端自定 SHA）全部在 `gitlab_client.go` 内翻译。
- 「权限、幂等和版本不变量与 GitHub 一致。」（Issue #54 验收标准 2 原文）——同一条代码路径保证：owner-only 谓词（`GetOwnedRun` + 个人连接 owner-only + A02 成员资格复验）、A03 审批锚（digest 绑定材料 + 连接 AuthVersion，内容变化=新 digest=旧批准失效）、`ClaimDispatch` 批准消费、unknown→只以远端事实收敛、恢复只补 MR 绝不重推。提供者身份取自 `installations.app_id`（服务端权威），非 github/gitlab 一律 fail closed。
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #54 验收标准 3 原文）——本地证据链：真实 sqlite + 真实 A03 ActionService/ocAuthorizer + httptest GitLab 模拟器上的真实 HTTP 字节（Task 2/3 服务编排 Interface 测试）+ Task 4 OAuth/HTTP 面真实请求断言。真实 GitLab 端到端（真实 OAuth 应用 + 真实 gitlab.com 仓库）本地不可得，列为 blocked-env（见下）。
- 「It does not merge automatically or expose remote credentials to Shell.」——GitLab 适配器客户端**没有任何 merge 方法**（结构性不存在，端口即 `GitHubClient` 无 merge）；令牌只在服务端 `CredentialResolver` 解析后直接进适配器，从不进沙箱/日志/响应。
- 「candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」——材料（repo/基线/分支/文件清单/提交信息/MR 标题）经 `CanonicalJSON` 归一化锚定为 A03 digest；派发只发送 Prepare 时落库的归一化字节（`ParseDeliveryMaterial` 精确六字段校验拒篡改）。
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」——`ErrCodeTransport`（= `ErrGitHubTransport` 同值别名）标记不可观测结果 → 交付行落 unknown；`ResolveUnknown` 只查远端事实（MR 存在→delivered；分支存在→pushed）；推送成功而 MR 失败落 `pushed`（部分完成），恢复只补 MR。
- 安全约束（会话注入）：SQL 一律参数绑定（本计划查询全为 gorm `?` 绑定 / struct 查询）；出网仅允许钉住的主机——GitHub `https://api.github.com`（既有）与 GitLab `https://gitlab.com`（生产常量，测试注入 httptest URL，绝不由数据派生 host）；凭据只从环境变量/凭据服务读取（`WEKNORA_APP_OAUTH_GITLAB_CLIENT_ID/_SECRET` 经既有 env 自动拾取；`WEKNORA_GITLAB_TEST_TOKEN` 门控真实测试），源码与测试不写入可用凭据字面量（测试中的 `glpat-testtoken` 为模拟器自约定的假值）。
- 迁移约束：本计划**零新增迁移、零 schema 变更**——`code_deliveries` 表（sqlite 000117 / versioned 000196，集成期已重排）直接复用，提供者经 `connections→installations` join 权威可追溯（action 行 target 亦持久化平台），无需加列。**当前 HEAD 存在 000114（sqlite）/000193（versioned）同号双迁移（mobile_device_app 与 public_agent_marketplace，作者 ls 实证），全量迁移轨道损坏——按波级指引第 1 条，本计划测试不使用全量迁移轨道（自包含 sqlite 夹具 AutoMigrate，同 #52 先例），也不顺手去重（留给需要者或主控裁决）。**
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #54 验收标准原文（docs/plans/issue30-sweep/issues/issue-54.md）：**

1. 「GitLab 差异被 Adapter 隐藏。」
2. 「权限、幂等和版本不变量与 GitHub 一致。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实 GitLab 端到端需要「一个已配置 `WEKNORA_APP_OAUTH_GITLAB_CLIENT_ID/_SECRET` 的 WeKnora Deployment + 一个真实 GitLab 账号与测试仓库」。本地无此环境时：Task 1 的 `TestGitLabClientAgainstRealGitLab`（env 门控 `WEKNORA_GITLAB_TEST_TOKEN`/`WEKNORA_GITLAB_TEST_PROJECT`）以 skip 收场（**不得伪造通过**）。本地替代证据（均为实跑）：①Task 2/3 服务编排 Interface 测试（真实 sqlite + 真实 A03 + httptest GitLab 模拟器真实 HTTP 字节 + 本地工作区，覆盖全部验收分支：E2E 落账、部分完成恢复、unknown 收敛、审批锚不可变、A02 撤销、护栏次序、提供者 fail closed、通配保护分支）；②Task 4 OAuth 交换形状 + HTTP 错误分类真实请求断言；③Task 5 移动面文案中性化测试。凡具备 env 的运行自动产出真实 GitLab 端到端证据。

## 与调查结论的差异记录（以代码现状为准，均为本计划作者在当前 HEAD 亲眼核实）

1. **调查称「统一 Delivery seam 零实现」——已过时**：#52（T22）已交付完整统一交付缝。`internal/modules/codedelivery/` 下 `delivery.go`（状态机/分支护栏/材料快照/Diff，243 行）、`github.go`+`github_client.go`（平台端口 13 方法 + REST 适配）、`service.go`（编排 515 行）、`dispatcher.go`（A03 派发器 311 行）、`workspace.go`+`workspace_local.go`（工作区端口）、`repository/codedelivery/store.go`（`code_deliveries` 追溯存储）；HTTP 面 `internal/handler/session/workbench_delivery.go`（246 行）+ `internal/router/routes_workbench.go:279`；容器 `internal/container/code_delivery.go`。**本计划的"统一 Delivery seam"工作 = 在该缝内补 GitLab 适配器与提供者路由，不是从零建缝。**
2. 调查称「全仓 gitlab 命中仅为知识库数据源与 Open Connector 夹具」——属实（作者复核 `grep -rli gitlab internal/ apps/ packages/`：命中 `internal/types/datasource.go`、`internal/types/knowledge.go`、`internal/application/repository/knowledge.go`、`internal/handler/sandbox_skill.go` 等，全部是知识库数据源同步域）。Go 交付域无任何 GitLab 实现，本计划从零新增。
3. 调查称「前置 #52 的 GitHub 主线本身 absent」——已过时：#52 已合并集成（codedelivery 基线测试 `go test ./internal/modules/codedelivery/ -count=1` ok，0.793s，本计划作者实跑）。
4. **调查称迁移需 Task 0 去重——以代码现状为准不需要**：集成期已把既有迁移重排（`migrations/sqlite/` 尾部现为 000113 adoption、114 mobile_device_app、114 public_agent_marketplace（撞号）、115 app_publications、116 task_compliance、117 code_deliveries；versioned 同构至 000196）。本计划零迁移零 schema 变更，测试用自包含夹具（AutoMigrate）+ 不装载全量轨道，**不触碰撞号**（波级指引第 1 条）。
5. **`Connection` 无 provider 字段**（`internal/modules/appconnector/model.go:41-50` 仅 ID/InstallationID/Kind/OwnerID/CredentialRef/State/TenantID/AuthVersion）——提供者解析走 `Connection.InstallationID → appconnectorrepo.InstallationStore.GetInstallationByID → Installation.AppID`（`install.go:225-234`，tenant 作用域内置），AppID ∈ {github, gitlab} 之外 fail closed。
6. **GitLab 无独立 blob/tree 创建端点、无客户端可控 parent**：推送半程由适配器折叠为一次 `POST /projects/:id/repository/commits`（actions 语义 create/update/delete），把任务分支从现 tip（新分支则从默认分支 start_branch）**收敛**到预期树（基线树 + 批准变更）；提交 SHA 由服务端分配——派发器增加推送后 `BranchHead` 权威读回，GitHub（结果恒等）与 GitLab（消除占位 SHA）统一以远端 head 落账。这是本计划对 #52 冻结代码的唯一行为增强（`dispatcher.go` 推送半程 ~10 行），对 GitHub 既有 9 个测试全部兼容（作者逐条核对断言：调用计数均为相对断言/读回语义恒真）。
7. **`GitBlobSHA` 对 GitLab 同样成立**：GitLab 对象库同为内容寻址 git 对象（blob sha1 同构），基线树与本地 Diff 判定无需任何 GitLab 特化。
8. **session 包测试作用域必须用精确前缀 `TestDelivery`**：更宽的 `-run 'Delivery'` 会命中 #38 的 `TestCraftInteractionDecideUnknownDeliveryStays202`——该测试装载**全量迁移轨道**，在波起点 HEAD 即因预存在的 000114/000193 双迁移失败（作者实跑复现，错误原文 `duplicate migration file: 000114_public_agent_marketplace.down.sql`），与本计划无关。本计划交付 HTTP 测试（`TestDelivery*` 六个）不装载迁移，`-run 'TestDelivery'` 全绿。
9. **共享 worktree 并行写入观察**：本计划验证期间，同批其他计划作者在同一 worktree 落地 #57 语音域半成品（`apps/mobile/src/app/tasks/voice.tsx` 等、`composition.ts` 引入尚不存在的 `@weknora/api-client/mobile/voice-sessions`、000114_mobile_device_app 迁移去重），导致 `pnpm --filter @weknora/mobile test/typecheck` 全量口径瞬时失败——失败全部集中于语音域与 composition import 级联，零项涉及本计划文件（typecheck 错误清单中 `TaskDetailScreen.tsx`/`app-smoke.test.tsx` 零错误）。本计划 Task 5 以文件级作用域（`tsx --test src/app-smoke.test.tsx`）取证：交付回执区块文案断言 PASS。集成期干净树上按计划级验证命令全量执行即可。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **连接背后的平台不是代码平台**（配置漂移：notion/feishu 连接被拿来交付；或适配器未接线）：必须在任何远端调用/凭据解析之前 fail closed（`ErrUnsupportedProvider`），HTTP 面落 400 固定码而非 5xx。——Task 2 测试「prepare via a non-code-platform connection fails closed with zero remote calls」+「a foreign action target never reaches any adapter」+ Task 4 错误分类表新增行「unsupported provider → 400 code_delivery_unsupported_provider」。
2. **推送成功但 MR 创建失败后的盲目重试**（最危险的重复副作用）：恢复绝不能重发 commits actions。——Task 3 测试「a partial push with a failed MR recovers by creating ONLY the merge request（模拟器调用计数断言 POST /repository/commits 不变）」。
3. **批准与执行之间内容被换或成员资格被撤销**（approve-then-rewrite / A02 失效）：digest 篡改与 A02 拒绝必须零远端调用、如实落账。——Task 2 测试「a tampered snapshot never reaches GitLab」+ Task 3 测试「dispatch fails closed when the connection owner lost membership（零 POST）」。
4. **远端结果不可观测（commits POST 中途断网）**：既不能当失败重推、也不能假装成功；必须落 unknown 并只以远端事实收敛。——Task 3 测试「a transport cut after the commits POST parks unknown; provider-query resolution settles pushed from remote facts; recovery completes the MR」。
5. **GitLab 形状差异泄漏进交付链**（非草稿 MR 标题、保护分支通配未匹配、`owner:branch` 前缀漏剥、merge 调用）：模拟器以违规计数器钉死——非草稿标题=violation+422、merge 尝试=violation+404、保护分支写=violation+403；通配匹配与前缀剥离在 wire 测试断言。——Task 1 模拟器 + wire 测试「GitLab wildcards protect a task branch」「the owner-prefixed head is stripped to a bare source branch」+ Task 3 护栏次序测试。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：平台中立词汇 + GitLab REST v4 适配器（含模拟器 wire 测试 + env 门控真实测试） | `internal/modules/codedelivery/code_platform.go`、`gitlab_client.go`、`gitlab_wire_test.go`、`gitlab_real_test.go` |
| 2 | Go：提供者路由 + 派发器权威读回 + 服务编排核心测试 | `service.go`/`dispatcher.go` deps 与路由、`service_prepare_test.go` 夹具扩展、`service_gitlab_test.go`（E2E/物化/fail closed/篡改） |
| 3 | Go：GitLab 交付编排语义测试（部分完成/unknown/迭代收敛/删除/护栏次序/A02） | `service_gitlab_test.go`（语义组） |
| 4 | Go：GitLab OAuth 注册 + HTTP 错误映射 + 容器接线 | `app_connector_oauth.go` 3 处、`workbench_delivery.go` 1 处、`container/code_delivery.go`、2 个测试文件 |
| 5 | 移动面文案中性化（PR → PR/MR） | `TaskDetailScreen.tsx` 3 字符串、`app-smoke.test.tsx` 1 处文案钉 |

**并行批次注意（本计划与同批其余计划并行实施，独立 worktree 后合并）**——新增文件全部为本计划独有（上表 Create 项）。共享文件修改清单与位置（全部最小追加/替换，便于合并）：

- `internal/modules/codedelivery/service.go`：`CodeDeliveryDeps` 增 `GitLab`/`Providers` 两字段；`authorize` 签名加返回 `conn`（2 个调用点）；`MaterializeBaseline`/`PrepareDelivery` 各插入 provider 解析 + `clientFor`（约 20 行）。
- `internal/modules/codedelivery/dispatcher.go`：`DispatcherDeps` 增 `GitLab` 字段；`Dispatch`/`RecoverPullRequest`/`QueryProvider` 的客户端构造改 `clientForTarget`（3 处替换 + 1 处顺序调整）；`deliver` 推送半程 RecordReceipts/EnsureBranch 顺序对调 + 权威读回（约 15 行）。
- `internal/modules/codedelivery/service_prepare_test.go`：夹具结构体增 1 字段、`newDeliveryFixture` 增 gitlab 连接两行 + 工厂/deps 装配、`LoadCredential`/`Resolve` 各加 3 行 gitlab 分支。
- `internal/handler/session/workbench_delivery.go`：`writeDeliveryError` 增 1 个 case（4 行）。
- `internal/handler/session/workbench_delivery_test.go`：错误分类表增 1 行。
- `internal/handler/app_connector_oauth.go`：`appOAuthDefaults` 增 `"gitlab"` 条目（4 行）+ `exchangeAppOAuthCode` 增 `case "gitlab"`（13 行）+ `appAuthorizeURL` 增 `case "gitlab"`（4 行）。
- `internal/container/code_delivery.go`：增 gitlab 工厂 + providers 装配（约 8 行）+ 1 个 import。
- `apps/mobile/src/screens/TaskDetailScreen.tsx`：`DELIVERY_STATE_COPY` 两值 + 区块内 1 个标签 + 注释（4 行）。
- `apps/mobile/src/app-smoke.test.tsx`：1 处文案断言（1 行）。

**包外零改动声明**：`packages/contracts`、`packages/api-client`、`packages/mobile-core` 零修改——统一回执词汇（`CodeDeliveryRecord.pr_number/pr_url/remote_login`）本就是平台无关投影（`packages/contracts/src/mobile/code-delivery.ts:42-74` 解析器不感知平台），GitLab MR 的 iid/web_url 经同一字段流转，这正是 AC1「差异被 Adapter 隐藏」的读面体现。**本计划零 Go 迁移、零 Go 新表、零 contracts/api-client 变更。**

---

### Task 1: Go——平台中立词汇与 GitLab REST v4 适配器

**Files:**
- Create: `internal/modules/codedelivery/code_platform.go`
- Create: `internal/modules/codedelivery/gitlab_client.go`
- Test: `internal/modules/codedelivery/gitlab_wire_test.go`
- Test: `internal/modules/codedelivery/gitlab_real_test.go`（env 门控，缺 env 即 skip）

**Interfaces:**
- Consumes（#52 冻结面，逐字）：`GitHubClient` 13 方法端口与 `GitHubClientFactory = func(token string, repo RepoRef) GitHubClient`（`github.go:80-101`）、`GitHubRepoInfo{DefaultBranch}`、`TreeEntry{Path, SHA, Mode}`（SHA=="" 表示删除）、`PullRequestInput{Title, Head, Base}`、`PullRequestReceipt{Number int64, URL string, Draft, Created bool}`、`ErrGitHubTransport`/`ErrGitHubRequestInvalid`/`GitHubAPIError{Status, Endpoint, Message}`（`github.go:18-48`）、`GitBlobSHA(content []byte) string`、`RepoRef`/`ParseRepoRef`、`httpClientDefault()`、`TaskBranchOf`、`writeJSON`（github_wire_test.go 同包测试助手，复用不重定义）。
- Produces（Task 2/3/4 依赖的精确签名）:
  - `type CodePlatformClient = GitHubClient`；`type CodePlatformClientFactory = GitHubClientFactory`；`type CodePlatformAPIError = GitHubAPIError`；`var ErrCodeTransport = ErrGitHubTransport`；`var ErrCodeRequestInvalid = ErrGitHubRequestInvalid`（分类语义中立化，值恒等使既有 `errors.Is` 映射不变）
  - `const ProviderGitHub = "github"`；`const ProviderGitLab = "gitlab"`；`var ErrUnsupportedProvider = errors.New("code_delivery_unsupported_provider")`
  - `func DeliveryTargetOf(provider string) string`；`func ProviderOfTarget(target string) (string, error)`
  - `func DraftMRTitle(title string) string`（幂等 `Draft: ` 前缀）；`func ProtectedBranchGlobMatch(pattern, branch string) bool`（GitLab 通配 `*`/`?`）
  - `type ProviderSource interface { GetInstallationByID(ctx context.Context, tenantID uint64, installationID string) (appconnector.Installation, error) }`（生产实现 `*appconnectorrepo.InstallationStore`）
  - `func NewGitLabClientFactory(httpClient *http.Client, baseURL string) CodePlatformClientFactory`（baseURL 生产传 `GitLabAPIBaseURL = "https://gitlab.com"`，测试注入 httptest URL）
  - 测试侧模拟器：`newGitLabEmulator(t) *gitLabEmulator`（`gitlab_wire_test.go`；`Calls() map[string]int`、`Violations() []string`、`BranchCommit(branch) (string, bool)`、`BranchTree(branch) map[string]string`、`ParentOf(sha) []string`、`protectPattern(pattern)`、`failNextMRCreation()`、`blackoutAfterCommitCreate()`、`liftBlackout()`）

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/gitlab_wire_test.go`：

```go
package codedelivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// gitLabEmulator 是一个内存 GitLab REST v4（T24 #54）：真实 HTTP 字节进出
// 生产适配器，blobs 以真实 git blob sha 入库（GitLab 同为内容寻址 git 对象
// 库），commits API 按「在分支现 tip（或 start_branch tip）上应用 actions、
// 服务端自定提交 sha 与祖先链」语义推进。记录每类调用次数与违规（merge
// 尝试、非草稿 MR、保护分支写、未知 action）。cutAfterCommit 在 commits
// POST 成功后立即断连，模拟推送落地后的传输不可观测。
type gitLabEmulator struct {
	t             *testing.T
	srv           *httptest.Server
	mu            sync.Mutex
	token         string
	calls         map[string]int
	bad           []string
	proj          string // escaped path segment："octocat%2Fhello"
	defaultBranch string
	protected     []string // 精确名或通配模式（GitLab 语义）
	blobs         map[string][]byte
	trees         map[string]map[string]string // commit sha → path→blob sha
	commits       map[string][]string          // commit sha → parent shas
	branches      map[string]string            // branch → commit sha
	mrs           []gitLabMR
	nextMR        int64
	failMR        bool
	cutAfterCommit bool
	blackout      bool
}

type gitLabMR struct {
	IID    int64
	Title  string
	Source string
	Target string
	State  string
}

func newGitLabEmulator(t *testing.T) *gitLabEmulator {
	e := &gitLabEmulator{
		t: t, token: "glpat-testtoken", calls: map[string]int{},
		proj:          "octocat%2Fhello",
		defaultBranch: "main",
		protected:     []string{"stable-*"},
		blobs:         map[string][]byte{},
		trees:         map[string]map[string]string{},
		commits:       map[string][]string{},
		branches:      map[string]string{},
		mrs:           []gitLabMR{},
	}
	baseline := "b" + strings.Repeat("0", 39)
	seed := map[string][]byte{
		"README.md": []byte("# hello\n"),
		"main.go":   []byte("package main\n"),
	}
	tree := map[string]string{}
	for p, c := range seed {
		sha := GitBlobSHA(c)
		e.blobs[sha] = c
		tree[p] = sha
	}
	e.trees[baseline] = tree
	e.commits[baseline] = []string{}
	e.branches["main"] = baseline
	mux := http.NewServeMux()
	mux.HandleFunc("/", e.serve)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *gitLabEmulator) note(call string) { e.mu.Lock(); e.calls[call]++; e.mu.Unlock() }
func (e *gitLabEmulator) violation(v string) {
	e.mu.Lock()
	e.bad = append(e.bad, v)
	e.mu.Unlock()
}
func (e *gitLabEmulator) Calls() map[string]int { return e.calls }
func (e *gitLabEmulator) Violations() []string  { return e.bad }
func (e *gitLabEmulator) BranchCommit(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	return sha, ok
}

// BranchTree 投影分支 tip 树（path→blob sha）——「收敛不变量」断言用。
func (e *gitLabEmulator) BranchTree(branch string) map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	if !ok {
		return nil
	}
	out := map[string]string{}
	for p, b := range e.trees[sha] {
		out[p] = b
	}
	return out
}

// ParentOf 返回提交的父链（GitLab commits API 由服务端决定祖先的断言用）。
func (e *gitLabEmulator) ParentOf(sha string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.commits[sha]...)
}

func (e *gitLabEmulator) protectPattern(pattern string) {
	e.mu.Lock()
	e.protected = append(e.protected, pattern)
	e.mu.Unlock()
}
func (e *gitLabEmulator) failNextMRCreation() { e.mu.Lock(); e.failMR = true; e.mu.Unlock() }

// blackoutAfterCommitCreate：下一条 commits POST 成功落地后立即断连——
// 推送已发生、结果不可观测（与 githubEmulator 的 blackoutAfterRefCreate
// 同语义；「after」由 refs/commits 路由内的置位实现）。
func (e *gitLabEmulator) blackoutAfterCommitCreate() {
	e.mu.Lock()
	e.cutAfterCommit = true
	e.mu.Unlock()
}
func (e *gitLabEmulator) liftBlackout() { e.mu.Lock(); e.blackout = false; e.mu.Unlock() }

// resolveRef 解析 ref（分支名优先，其次提交 sha）。
func (e *gitLabEmulator) resolveRef(ref string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if sha, ok := e.branches[ref]; ok {
		return sha, true
	}
	if _, ok := e.trees[ref]; ok {
		return ref, true
	}
	return "", false
}

func (e *gitLabEmulator) branchHead(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	return sha, ok
}

func (e *gitLabEmulator) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	blackout := e.blackout
	failMR := e.failMR
	e.mu.Unlock()
	if blackout {
		// 连接直接断开：客户端拿到 transport 错误（不可观测）。
		panic(http.ErrAbortHandler)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+e.token {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"message": "invalid_token"})
		return
	}
	// 结构性护栏：merge 尝试是违规并被拒绝（真实端点 PUT …/merge_requests/:iid/merge
	// 以 "/merge" 结尾；"/merge_requests" 不误伤）。
	if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
		e.violation("merge attempted: " + r.Method + " " + r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	escaped := r.URL.EscapedPath()
	projPrefix := "/api/v4/projects/" + e.proj
	switch {
	case r.Method == http.MethodGet && escaped == "/api/v4/user":
		e.note("GET /user")
		writeJSON(w, map[string]any{"username": "gl-user"})
	case r.Method == http.MethodGet && escaped == projPrefix:
		e.note("GET /project")
		writeJSON(w, map[string]any{"default_branch": e.defaultBranch, "path_with_namespace": "octocat/hello"})
	case r.Method == http.MethodGet && escaped == projPrefix+"/protected_branches":
		e.note("GET /protected_branches")
		out := []map[string]any{}
		for _, p := range e.protected {
			out = append(out, map[string]any{"name": p})
		}
		w.Header().Set("X-Next-Page", "")
		writeJSON(w, out)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/tree"):
		e.note("GET /repository/tree")
		ref := r.URL.Query().Get("ref")
		commit, ok := e.resolveRef(ref)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "404 ref " + ref + " not found"})
			return
		}
		e.mu.Lock()
		entries := []map[string]any{}
		for p, sha := range e.trees[commit] {
			entries = append(entries, map[string]any{"id": sha, "type": "blob", "path": p, "mode": "100644"})
		}
		e.mu.Unlock()
		w.Header().Set("X-Next-Page", "")
		writeJSON(w, entries)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/blobs/"):
		e.note("GET /repository/blobs raw")
		rest := strings.TrimPrefix(escaped, projPrefix+"/repository/blobs/")
		if !strings.HasSuffix(rest, "/raw") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		sha := strings.TrimSuffix(rest, "/raw")
		e.mu.Lock()
		content, ok := e.blobs[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "blob not found"})
			return
		}
		_, _ = w.Write(content)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/branches/"):
		e.note("GET /repository/branches")
		branch, err := url.PathUnescape(strings.TrimPrefix(escaped, projPrefix+"/repository/branches/"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sha, ok := e.branchHead(branch)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "404 branch not found"})
			return
		}
		writeJSON(w, map[string]any{"name": branch, "commit": map[string]any{"id": sha}})
	case r.Method == http.MethodPost && escaped == projPrefix+"/repository/commits":
		e.note("POST /repository/commits")
		e.commitOnBranch(w, r)
	case r.Method == http.MethodGet && escaped == projPrefix+"/merge_requests":
		e.note("GET /merge_requests")
		source := r.URL.Query().Get("source_branch")
		state := r.URL.Query().Get("state")
		e.mu.Lock()
		out := []map[string]any{}
		for _, mr := range e.mrs {
			if (source == "" || mr.Source == source) && (state == "" || mr.State == state) {
				out = append(out, map[string]any{"iid": mr.IID, "web_url": mrURL(mr.IID), "title": mr.Title, "state": mr.State})
			}
		}
		e.mu.Unlock()
		writeJSON(w, out)
	case r.Method == http.MethodPost && escaped == projPrefix+"/merge_requests":
		e.note("POST /merge_requests")
		e.createMR(w, r, failMR)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

type glCommitAction struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (e *gitLabEmulator) commitOnBranch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Branch        string           `json:"branch"`
		StartBranch   string           `json:"start_branch"`
		CommitMessage string           `json:"commit_message"`
		Actions       []glCommitAction `json:"actions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	e.mu.Lock()
	defer e.mu.Unlock()
	if body.Branch == e.defaultBranch {
		e.bad = append(e.bad, "protected ref write: "+body.Branch)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	for _, pattern := range e.protected {
		if ProtectedBranchGlobMatch(pattern, body.Branch) {
			e.bad = append(e.bad, "protected ref write: "+body.Branch)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	tip, exists := e.branches[body.Branch]
	start := tip
	if !exists {
		if body.StartBranch == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"message": "branch does not exist; start_branch required"})
			return
		}
		s, ok := e.branches[body.StartBranch]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"message": "start_branch not found"})
			return
		}
		start = s
	}
	if len(body.Actions) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]any{"message": "ensure at least one action"})
		return
	}
	tree := map[string]string{}
	for p, b := range e.trees[start] {
		tree[p] = b
	}
	for _, act := range body.Actions {
		switch act.Action {
		case "create", "update":
			var content []byte
			var err error
			if act.Encoding == "base64" {
				content, err = base64.StdEncoding.DecodeString(act.Content)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			} else {
				content = []byte(act.Content)
			}
			sha := GitBlobSHA(content)
			e.blobs[sha] = content
			tree[act.FilePath] = sha
		case "delete":
			delete(tree, act.FilePath)
		default:
			e.bad = append(e.bad, "unknown commit action: "+act.Action)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}
	sha := fmt.Sprintf("glc-%d", len(e.commits)+1)
	e.commits[sha] = []string{start}
	e.trees[sha] = tree
	e.branches[body.Branch] = sha
	if e.cutAfterCommit {
		e.blackout = true
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"id": sha})
}

func (e *gitLabEmulator) createMR(w http.ResponseWriter, r *http.Request, failMR bool) {
	var body struct {
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Title        string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	e.mu.Lock()
	defer e.mu.Unlock()
	if failMR {
		e.failMR = false
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"message": "merge request validation failed"})
		return
	}
	// GitLab 以标题前缀标记草稿 MR：非草稿标题是违规（差异必须由适配器隐藏）。
	if !strings.HasPrefix(body.Title, "Draft: ") {
		e.bad = append(e.bad, "non-draft merge request: "+body.Title)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	for _, mr := range e.mrs {
		if mr.Source == body.SourceBranch && mr.State == "opened" {
			w.WriteHeader(http.StatusConflict)
			writeJSON(w, map[string]any{"message": "an open merge request already exists for this source branch"})
			return
		}
	}
	e.nextMR++
	mr := gitLabMR{IID: e.nextMR, Title: body.Title, Source: body.SourceBranch, Target: body.TargetBranch, State: "opened"}
	e.mrs = append(e.mrs, mr)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"iid": mr.IID, "web_url": mrURL(mr.IID), "title": mr.Title, "state": mr.State})
}

func mrURL(iid int64) string {
	return fmt.Sprintf("https://gitlab.com/octocat/hello/-/merge_requests/%d", iid)
}

func TestGitLabClientWireChainConvergesBranchAndDraftMR(t *testing.T) {
	e := newGitLabEmulator(t)
	factory := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	repo := RepoRef{Owner: "octocat", Name: "hello"}
	client := factory("glpat-testtoken", repo) // 每次调用钉定 token + 仓库
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", info.DefaultBranch)

	// GitLab 保护分支是「精确名或通配模式」：通配匹配由适配器本地完成。
	protected, err := client.BranchProtected(ctx, "stable-x")
	require.NoError(t, err)
	require.True(t, protected, "GitLab 通配保护模式必须由适配器匹配")
	protected, err = client.BranchProtected(ctx, "weknora/task/s-1")
	require.NoError(t, err)
	require.False(t, protected)

	// CommitTree→Tree 折叠为一次树读（GitLab 的 Tree 直接接受 ref）。
	baseline := "b" + strings.Repeat("0", 39)
	baseRef, err := client.CommitTree(ctx, baseline)
	require.NoError(t, err)
	require.Equal(t, baseline, baseRef)
	tree, err := client.Tree(ctx, baseRef)
	require.NoError(t, err)
	require.Len(t, tree, 2)
	blob, err := client.Blob(ctx, tree["main.go"])
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(blob))

	newContent := []byte("package main\n\nfunc main() {}\n")
	newSHA, err := client.CreateBlob(ctx, newContent)
	require.NoError(t, err)
	require.Equal(t, GitBlobSHA(newContent), newSHA, "blob sha 内容寻址，与 GitLab 存储同构")
	staged, err := client.CreateTree(ctx, baseline, []TreeEntry{{Path: "main.go", SHA: newSHA}})
	require.NoError(t, err)
	require.NotEmpty(t, staged)
	placeholder, err := client.CreateCommit(ctx, baseline, staged, "fix: greeting")
	require.NoError(t, err)
	require.NotEmpty(t, placeholder)

	branch := TaskBranchOf("s-1")
	require.NoError(t, client.EnsureBranch(ctx, branch, placeholder))
	sha, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.NotEqual(t, placeholder, sha, "GitLab 服务端自定提交 sha，占位值不得外泄")
	tip := e.BranchTree(branch)
	require.Equal(t, GitBlobSHA(newContent), tip["main.go"], "任务分支收敛到基线树+变更")
	require.Equal(t, GitBlobSHA([]byte("# hello\n")), tip["README.md"], "基线未变文件保留")
	require.Equal(t, []string{baseline}, e.ParentOf(sha), "新分支从 start_branch tip 分叉")

	// 草稿 MR：owner:branch 前缀被适配器剥离为裸 source_branch。
	receipt, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.True(t, receipt.Draft)
	require.True(t, receipt.Created)
	require.EqualValues(t, 1, receipt.Number)
	require.Contains(t, receipt.URL, "/-/merge_requests/1")

	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	require.Equal(t, "gl-user", login)

	// 已有同 source 的开放 MR：复用（Created=false），不重复开。
	again, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.False(t, again.Created)
	require.EqualValues(t, 1, again.Number)

	require.Empty(t, e.Violations())
}

func TestGitLabClientClassifiesDefiniteVsUnobservable(t *testing.T) {
	e := newGitLabEmulator(t)
	factory := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	// 确定性 404：CodePlatformAPIError（status 携带）。
	_, err := factory("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"}).Tree(ctx, "missing-ref")
	var apiErr *CodePlatformAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.Status)

	// 坏令牌：确定性 401。
	_, err = factory("glpat-wrong", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// 网络不可达：ErrCodeTransport（不可观测）。
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()
	_, err = NewGitLabClientFactory(http.DefaultClient, closed.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorIs(t, err, ErrCodeTransport)
}

func TestDraftMRTitleAndProtectedGlob(t *testing.T) {
	require.Equal(t, "Draft: fix", DraftMRTitle("fix"))
	require.Equal(t, "Draft: fix", DraftMRTitle("Draft: fix"), "前缀必须幂等")
	require.True(t, ProtectedBranchGlobMatch("stable-*", "stable-x"))
	require.True(t, ProtectedBranchGlobMatch("release/*", "release/1.0/x"))
	require.False(t, ProtectedBranchGlobMatch("stable-*", "main"))
	require.True(t, ProtectedBranchGlobMatch("ma?n", "main"))
	require.False(t, ProtectedBranchGlobMatch("", "main"))
	provider, err := ProviderOfTarget("gitlab.deliver")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	_, err = ProviderOfTarget("feishu.send")
	require.ErrorIs(t, err, ErrUnsupportedProvider)
	require.Equal(t, "gitlab.deliver", DeliveryTargetOf("gitlab"))
}
```

创建 `internal/modules/codedelivery/gitlab_real_test.go`（blocked-env 门控）：

```go
package codedelivery

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGitLabClientAgainstRealGitLab 需要 WEKNORA_GITLAB_TEST_TOKEN（OAuth 或
// PAT 令牌，api 作用域）与 WEKNORA_GITLAB_TEST_PROJECT（owner/name，可达仓库）。
// 缺 env 即 skip——真实 GitLab 属 true external（blocked-env），不伪造。
func TestGitLabClientAgainstRealGitLab(t *testing.T) {
	token := os.Getenv("WEKNORA_GITLAB_TEST_TOKEN")
	project := os.Getenv("WEKNORA_GITLAB_TEST_PROJECT")
	if token == "" || project == "" {
		t.Skip("WEKNORA_GITLAB_TEST_TOKEN/WEKNORA_GITLAB_TEST_PROJECT not set (blocked-env)")
	}
	repo, err := ParseRepoRef(project)
	require.NoError(t, err)
	client := NewGitLabClientFactory(httpClientDefault(), GitLabAPIBaseURL)(token, repo)
	ctx := context.Background()
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real gitlab: project=%s default=%s login=%s", repo, info.DefaultBranch, login)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: FAIL（`NewGitLabClientFactory`/`CodePlatformAPIError`/`DraftMRTitle` 等未定义，编译错误）

- [ ] **Step 3: 写最小实现**

创建 `internal/modules/codedelivery/code_platform.go`：

```go
package codedelivery

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
)

// —— 统一 Delivery seam 的代码平台中立词汇（T24 #54）——
// 交付链（基线、diff、审批、任务分支、草稿 PR/MR）只认本文件的中立形状；
// 平台差异被各适配器隐藏。GitHub 先定义了端口形状，GitLab 经类型别名适配
// 同一端口：唯一的平台 switch 是 clientForPlatform，其余文件零平台分支。

// CodePlatformClient is the provider-neutral outbound port of one delivery.
type CodePlatformClient = GitHubClient

// CodePlatformClientFactory binds one token + one repo to a platform client.
type CodePlatformClientFactory = GitHubClientFactory

// CodePlatformAPIError is a definite provider refusal with an HTTP status.
type CodePlatformAPIError = GitHubAPIError

// The classification (definite vs unobservable) is provider-neutral; only the
// historical names carry "GitHub". Same values keep every existing
// errors.Is mapping intact.
var (
	ErrCodeTransport      = ErrGitHubTransport
	ErrCodeRequestInvalid = ErrGitHubRequestInvalid
)

// Provider vocabulary. The provider of a connection is authoritative on its
// installation row (installations.app_id) — never client input.
const (
	ProviderGitHub = "github"
	ProviderGitLab = "gitlab"
)

// ErrUnsupportedProvider: the connection's app is not a code platform (or the
// adapter is not wired). Every consumer fails closed before any remote call.
var ErrUnsupportedProvider = errors.New("code_delivery_unsupported_provider")

// DeliveryTargetOf maps a provider to its A03 action target.
func DeliveryTargetOf(provider string) string { return provider + ".deliver" }

// ProviderOfTarget parses an A03 action target back to its provider.
func ProviderOfTarget(target string) (string, error) {
	rest, ok := strings.CutSuffix(target, ".deliver")
	if !ok || rest == "" {
		return "", fmt.Errorf("%w: %q is not a delivery target", ErrUnsupportedProvider, target)
	}
	return rest, nil
}

// ProviderSource resolves the code platform behind a connection: the app id
// of the installation the connection belongs to. Production:
// *appconnectorrepo.InstallationStore (GetInstallationByID, tenant-scoped).
type ProviderSource interface {
	GetInstallationByID(ctx context.Context, tenantID uint64, installationID string) (appconnector.Installation, error)
}

// clientForPlatform picks the platform adapter behind the unified seam. Both
// the service (prepare face) and the dispatcher (dispatch face) route through
// this one switch — no other file may branch on a provider name.
func clientForPlatform(gitHub, gitLab CodePlatformClientFactory, provider, token string, repo RepoRef) (CodePlatformClient, error) {
	switch provider {
	case ProviderGitHub:
		if gitHub == nil {
			return nil, fmt.Errorf("%w: github adapter not wired", ErrUnsupportedProvider)
		}
		return gitHub(token, repo), nil
	case ProviderGitLab:
		if gitLab == nil {
			return nil, fmt.Errorf("%w: gitlab adapter not wired", ErrUnsupportedProvider)
		}
		return gitLab(token, repo), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, provider)
	}
}

// draftMRPrefix is GitLab's draft marker (title prefix; GitHub uses a draft
// boolean — the adapter maps, the chain never sees the difference). "WIP: "
// is GitLab's legacy draft marker and counts as already-draft.
const draftMRPrefix = "Draft: "

// DraftMRTitle marks a title as draft MR, idempotently.
func DraftMRTitle(title string) string {
	if strings.HasPrefix(title, draftMRPrefix) || strings.HasPrefix(title, "WIP: ") {
		return title
	}
	return draftMRPrefix + title
}

// ProtectedBranchGlobMatch evaluates a GitLab protected-branch pattern
// (wildcards * and ?) against a branch name. GitLab protects by exact name or
// wildcard; GitHub reports a boolean per branch — the adapter hides that by
// matching the platform's declared patterns itself. "*" spans "/" (GitLab's
// wildcard is broad); over-matching only ever REFUSES more, the safe
// direction for a guardrail.
func ProtectedBranchGlobMatch(pattern, branch string) bool {
	if pattern == "" {
		return false
	}
	var b strings.Builder
	b.WriteString(`^`)
	for _, c := range pattern {
		switch c {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`$`)
	matched, err := regexp.MatchString(b.String(), branch)
	return err == nil && matched
}
```

创建 `internal/modules/codedelivery/gitlab_client.go`：

```go
package codedelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GitLabAPIBaseURL is the reviewed production endpoint (gitlab.com SaaS).
// Tests inject an httptest URL; the factory never derives hosts from data.
// Self-hosted GitLab base URLs are a deployment concern, not model output.
const GitLabAPIBaseURL = "https://gitlab.com"

// NewGitLabClientFactory builds the GitLab REST v4 adapter behind the unified
// CodePlatformClient seam (T24 #54). The adapter hides every GitLab shape
// difference from the delivery chain:
//   - project addressing is the URL-escaped "owner/name" path segment;
//   - auth is the OAuth2 Bearer header (connections hold app-OAuth tokens);
//   - the push half collapses GitHub's blobs→tree→commit→ref chain into ONE
//     commits-API call (EnsureBranch) that CONVERGES the task branch to the
//     intended tree (baseline tree + approved changes): GitLab has no
//     client-controlled parent and no standalone blob/tree creation, and its
//     commits API never force-updates history — the same no-force-push
//     invariant the GitHub chain enforces with force:false, different
//     mechanism, hidden here;
//   - draft PR is a draft-marked merge request ("Draft: " title prefix) and
//     the chain's GitHub-shaped "owner:branch" head is stripped to a bare
//     source_branch;
//   - protected branches are name/wildcard PATTERNS, matched locally;
//   - the commit sha is assigned server-side: CreateCommit returns a
//     deterministic LOCAL placeholder and the dispatcher re-reads the branch
//     head after the push, so the placeholder never reaches the ledger.
//
// One client instance serves ONE dispatch/recovery operation; calls are
// sequential (the dispatcher is the only caller).
func NewGitLabClientFactory(httpClient *http.Client, baseURL string) CodePlatformClientFactory {
	if httpClient == nil {
		httpClient = httpClientDefault()
	}
	return func(token string, repo RepoRef) CodePlatformClient {
		return &gitLabRestClient{http: httpClient, base: baseURL, token: token, repo: repo}
	}
}

type gitLabRestClient struct {
	http  *http.Client
	base  string
	token string
	repo  RepoRef

	defaultBranch string            // cached from Repository()
	stagedBase    string            // ref CreateTree anchored on
	stagedEntries []TreeEntry       // approved mutations staged by CreateTree
	stagedMessage string            // staged by CreateCommit
	stagedBlobs   map[string][]byte // blob sha → content (from CreateBlob)
}

func (c *gitLabRestClient) projectSegment() string { return url.PathEscape(c.repo.String()) }

// call is the shared REST helper: any response is a definite outcome
// (*CodePlatformAPIError), a dial/timeout/EOF is unobservable
// (ErrCodeTransport), a malformed request never left the process
// (ErrCodeRequestInvalid).
func (c *gitLabRestClient) call(ctx context.Context, method, path string, body any, out any) (*http.Header, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%w: encode %s %s: %v", ErrCodeRequestInvalid, method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, fmt.Errorf("%w: build %s %s: %v", ErrCodeRequestInvalid, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %v", ErrCodeTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: read body: %v", ErrCodeTransport, method, path, err)
	}
	header := resp.Header
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env)
		return &header, &CodePlatformAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: env.Message}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return &header, fmt.Errorf("%w: decode %s %s: %v", ErrCodeRequestInvalid, method, path, err)
		}
	}
	return &header, nil
}

// callRaw fetches non-JSON payloads (raw blobs).
func (c *gitLabRestClient) callRaw(ctx context.Context, method, path string, out *[]byte) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("%w: build %s %s: %v", ErrCodeRequestInvalid, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrCodeTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%w: %s %s: read body: %v", ErrCodeTransport, method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return &CodePlatformAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: msg}
	}
	*out = raw
	return nil
}

func (c *gitLabRestClient) Repository(ctx context.Context) (GitHubRepoInfo, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment(), nil, &out)
	if err != nil {
		return GitHubRepoInfo{}, err
	}
	c.defaultBranch = out.DefaultBranch
	return GitHubRepoInfo{DefaultBranch: out.DefaultBranch}, nil
}

// BranchProtected lists the project's protected-branch PATTERNS and matches
// locally (exact or wildcard) — GitLab has no per-branch boolean.
func (c *gitLabRestClient) BranchProtected(ctx context.Context, branch string) (bool, error) {
	for page := 1; page <= 10; page++ {
		var out []struct {
			Name string `json:"name"`
		}
		header, err := c.call(ctx, http.MethodGet,
			fmt.Sprintf("/api/v4/projects/%s/protected_branches?per_page=100&page=%d", c.projectSegment(), page), nil, &out)
		if err != nil {
			return false, err
		}
		for _, p := range out {
			if p.Name == branch || ProtectedBranchGlobMatch(p.Name, branch) {
				return true, nil
			}
		}
		if header.Get("X-Next-Page") == "" {
			return false, nil
		}
	}
	return false, &CodePlatformAPIError{Status: 0, Endpoint: "protected_branches", Message: "pagination exceeded local cap"}
}

// Tree reads the full recursive tree at a ref (commit sha or branch name),
// paging per_page=100 until a short page.
func (c *gitLabRestClient) Tree(ctx context.Context, ref string) (map[string]string, error) {
	entries := map[string]string{}
	for page := 1; page <= 50; page++ {
		var out []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Path string `json:"path"`
		}
		q := url.Values{}
		q.Set("ref", ref)
		q.Set("recursive", "true")
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprint(page))
		_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment()+"/repository/tree?"+q.Encode(), nil, &out)
		if err != nil {
			return nil, err
		}
		for _, e := range out {
			if e.Type == "blob" {
				entries[e.Path] = e.ID
			}
		}
		if len(out) < 100 {
			return entries, nil
		}
	}
	return nil, &CodePlatformAPIError{Status: 0, Endpoint: "repository/tree", Message: "tree pagination exceeded local cap"}
}

// CommitTree: GitLab has no client-visible tree handle distinct from the ref
// — this adapter's Tree accepts a commit/branch ref directly, so the identity
// mapping IS the hiding (zero remote spend; the paired Tree call does the
// read).
func (c *gitLabRestClient) CommitTree(ctx context.Context, commitSHA string) (string, error) {
	if commitSHA == "" {
		return "", &CodePlatformAPIError{Status: 0, Endpoint: "repository/commits", Message: "empty ref"}
	}
	return commitSHA, nil
}

// Blob reads raw bytes by blob sha (GitLab's raw-blob endpoint takes the sha,
// no path needed).
func (c *gitLabRestClient) Blob(ctx context.Context, sha string) ([]byte, error) {
	var raw []byte
	err := c.callRaw(ctx, http.MethodGet,
		"/api/v4/projects/"+c.projectSegment()+"/repository/blobs/"+url.PathEscape(sha)+"/raw", &raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// CreateBlob stages content locally under its REAL git blob sha
// (content-addressed — GitLab stores the identical object); the commits API
// materializes it later. Zero remote call.
func (c *gitLabRestClient) CreateBlob(ctx context.Context, content []byte) (string, error) {
	sha := GitBlobSHA(content)
	if c.stagedBlobs == nil {
		c.stagedBlobs = map[string][]byte{}
	}
	c.stagedBlobs[sha] = content
	return sha, nil
}

// CreateTree stages the approved mutations; GitLab composes trees server-side
// at commit time. The returned id is a deterministic LOCAL handle (sha256 of
// the staged intent) that only travels between this adapter's own methods.
func (c *gitLabRestClient) CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error) {
	c.stagedBase = baseTree
	c.stagedEntries = append([]TreeEntry(nil), entries...)
	intent, err := json.Marshal(struct {
		Base    string      `json:"base"`
		Entries []TreeEntry `json:"entries"`
	}{Base: baseTree, Entries: c.stagedEntries})
	if err != nil {
		return "", fmt.Errorf("%w: stage tree: %v", ErrCodeRequestInvalid, err)
	}
	sum := sha256.Sum256(intent)
	return "gl-stage-" + hex.EncodeToString(sum[:8]), nil
}

// CreateCommit stages the message only. GitLab assigns the real commit
// server-side when the push (EnsureBranch) lands; the dispatcher re-reads the
// branch head, so the placeholder returned here never reaches the ledger.
func (c *gitLabRestClient) CreateCommit(ctx context.Context, parent, tree, message string) (string, error) {
	c.stagedMessage = message
	return "gl-commit-placeholder", nil
}

func (c *gitLabRestClient) baseBranch(ctx context.Context) (string, error) {
	if c.defaultBranch != "" {
		return c.defaultBranch, nil
	}
	info, err := c.Repository(ctx)
	if err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// EnsureBranch materializes the staged delivery as ONE GitLab commits-API
// call that converges <branch> to the intended tree (tree at stagedBase plus
// the staged entries). A missing branch is created from the default branch
// (start_branch); an existing branch is committed onto its CURRENT tip —
// GitLab never rewrites history. When the branch already sits exactly on the
// intended tree the call is a converged no-op (GitLab rejects empty actions;
// the tip IS the delivery fact).
func (c *gitLabRestClient) EnsureBranch(ctx context.Context, branch, commit string) error {
	base, err := c.baseBranch(ctx)
	if err != nil {
		return err
	}
	intended, err := c.Tree(ctx, c.stagedBase)
	if err != nil {
		return err
	}
	for _, e := range c.stagedEntries {
		if e.SHA == "" {
			delete(intended, e.Path)
			continue
		}
		intended[e.Path] = e.SHA
	}
	head, exists, err := c.BranchHead(ctx, branch)
	if err != nil {
		return err
	}
	startRef := base
	if exists {
		startRef = head
	}
	current, err := c.Tree(ctx, startRef)
	if err != nil {
		return err
	}
	type commitAction struct {
		Action   string `json:"action"`
		FilePath string `json:"file_path"`
		Content  string `json:"content,omitempty"`
		Encoding string `json:"encoding,omitempty"`
	}
	var actions []commitAction
	for path, want := range intended {
		if got, ok := current[path]; ok && got == want {
			continue
		}
		content, ok := c.stagedBlobs[want]
		if !ok {
			// 未在本派发中上传的内容（基线里已有、但起点分支缺它）：
			// 按内容寻址的 blob sha 从平台取回真实字节。
			content, err = c.Blob(ctx, want)
			if err != nil {
				return err
			}
		}
		act := "create"
		if _, onBranch := current[path]; onBranch {
			act = "update"
		}
		actions = append(actions, commitAction{Action: act, FilePath: path, Content: base64.StdEncoding.EncodeToString(content), Encoding: "base64"})
	}
	for path := range current {
		if _, ok := intended[path]; !ok {
			actions = append(actions, commitAction{Action: "delete", FilePath: path})
		}
	}
	if len(actions) == 0 {
		if exists {
			return nil // 已收敛：GitLab 不写空提交，tip 即交付事实
		}
		return fmt.Errorf("%w: task branch %s would be empty; nothing to push", ErrInvalidMaterial, branch)
	}
	if len(actions) > 2*MaxDeliveryFiles {
		return fmt.Errorf("%w: %d commit actions exceed cap %d", ErrBaselineTooLarge, len(actions), 2*MaxDeliveryFiles)
	}
	body := map[string]any{
		"branch":         branch,
		"commit_message": c.stagedMessage,
		"actions":        actions,
	}
	if !exists {
		body["start_branch"] = base
	}
	var out struct {
		ID string `json:"id"`
	}
	_, err = c.call(ctx, http.MethodPost, "/api/v4/projects/"+c.projectSegment()+"/repository/commits", body, &out)
	return err
}

// BranchHead resolves the branch tip; a missing branch is (absent), a
// transport/provider failure is an error.
func (c *gitLabRestClient) BranchHead(ctx context.Context, branch string) (string, bool, error) {
	var out struct {
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	_, err := c.call(ctx, http.MethodGet,
		"/api/v4/projects/"+c.projectSegment()+"/repository/branches/"+url.PathEscape(branch), nil, &out)
	if err != nil {
		var apiErr *CodePlatformAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if out.Commit.ID == "" {
		return "", false, nil
	}
	return out.Commit.ID, true, nil
}

// DraftPullRequest creates (or reuses) the draft merge request. GitLab marks
// drafts by title prefix; the chain's GitHub-shaped "owner:branch" head is
// stripped to a bare source_branch. GitLab's 409 on a duplicate source branch
// resolves to the existing MR (Created=false).
func (c *gitLabRestClient) DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error) {
	if existing, err := c.PullRequestForHead(ctx, input.Head); err == nil && existing != nil {
		// 复用既有开放 MR：Created 只在本次真实创建时为 true（与 GitHub 客户端同语义）。
		reuse := *existing
		reuse.Created = false
		return reuse, nil
	}
	body := map[string]any{
		"source_branch": c.branchOf(input.Head),
		"target_branch": input.Base,
		"title":         DraftMRTitle(input.Title),
	}
	var out struct {
		IID    int64  `json:"iid"`
		WebURL string `json:"web_url"`
	}
	_, err := c.call(ctx, http.MethodPost, "/api/v4/projects/"+c.projectSegment()+"/merge_requests", body, &out)
	if err != nil {
		var apiErr *CodePlatformAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			if existing, rerr := c.PullRequestForHead(ctx, input.Head); rerr == nil && existing != nil {
				reuse := *existing
				reuse.Created = false
				return reuse, nil
			}
		}
		return PullRequestReceipt{}, err
	}
	return PullRequestReceipt{Number: out.IID, URL: out.WebURL, Draft: true, Created: true}, nil
}

func (c *gitLabRestClient) branchOf(head string) string {
	return strings.TrimPrefix(head, c.repo.Owner+":")
}

func (c *gitLabRestClient) PullRequestForHead(ctx context.Context, head string) (*PullRequestReceipt, error) {
	q := url.Values{}
	q.Set("source_branch", c.branchOf(head))
	q.Set("state", "opened")
	q.Set("per_page", "20")
	var out []struct {
		IID    int64  `json:"iid"`
		WebURL string `json:"web_url"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment()+"/merge_requests?"+q.Encode(), nil, &out)
	if err != nil {
		return nil, err
	}
	for _, mr := range out {
		if mr.State != "opened" {
			continue
		}
		return &PullRequestReceipt{Number: mr.IID, URL: mr.WebURL, Draft: strings.HasPrefix(mr.Title, draftMRPrefix), Created: true}, nil
	}
	return nil, nil
}

func (c *gitLabRestClient) CurrentLogin(ctx context.Context) (string, error) {
	var out struct {
		Username string `json:"username"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/user", nil, &out)
	return out.Username, err
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（Task 1 的 3 个测试 + 既有 #52 全部测试全绿；`TestGitLabClientAgainstRealGitLab` skip）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/code_platform.go internal/modules/codedelivery/gitlab_client.go internal/modules/codedelivery/gitlab_wire_test.go internal/modules/codedelivery/gitlab_real_test.go
git commit -m "feat(codedelivery): provider-neutral delivery vocabulary + GitLab REST v4 adapter hiding platform shapes (T24 #54 task 1)"
```

---

### Task 2: Go——提供者路由、派发器权威读回与服务编排核心测试

**Files:**
- Modify: `internal/modules/codedelivery/service.go`（deps 字段、authorize 签名、provider 路由）
- Modify: `internal/modules/codedelivery/dispatcher.go`（deps 字段、clientForTarget、读回）
- Modify: `internal/modules/codedelivery/service_prepare_test.go`（夹具扩展）
- Test: `internal/modules/codedelivery/service_gitlab_test.go`（核心组）

**Interfaces:**
- Consumes: Task 1 全部 Produces；#52 的 `CodeDeliveryDeps`/`DispatcherDeps`/`authorize`/`tokenFor`（`service.go:48-61/:433/:451`）、`DeliveryDispatcher.Dispatch/RecoverPullRequest/QueryProvider/deliver`（`dispatcher.go:43/:69/:223/:101`）、夹具 `deliveryFixture`/`newDeliveryFixture`/`fixtureConnections`/`prepareInput`/`dispatchInput`/`firstDelivery`/`membersDrop`（`service_prepare_test.go`/`service_dispatch_test.go`）。
- Produces（Task 3/4 依赖）:
  - `CodeDeliveryDeps` 新字段：`GitLab CodePlatformClientFactory`、`Providers ProviderSource`
  - `DispatcherDeps` 新字段：`GitLab CodePlatformClientFactory`
  - `func (s *CodeDeliveryService) platformProvider(ctx context.Context, conn appconnector.Connection) (string, error)`；`func (s *CodeDeliveryService) clientFor(provider, token string, repo RepoRef) (CodePlatformClient, error)`
  - `func (d *DeliveryDispatcher) clientForTarget(target, token string, repo RepoRef) (CodePlatformClient, error)`
  - `authorize` 新签名：`func (s *CodeDeliveryService) authorize(ctx context.Context, tenantID uint64, callerID, runID, connectionID string) (string, appconnector.Connection, error)`
  - 派发语义（Task 3 断言依赖）：推送半程顺序为 EnsureBranch → BranchHead 权威读回 → RecordReceipts(远端 head)；`Dispatch` 的前置门顺序为 parse → A02 → token → **平台路由** → delivery row。
  - 测试助手（同包新文件）：`gitlabPrepareInput() PrepareInput`、`seededGitLabFixture(t) *deliveryFixture`、`snapshotGitLabCalls(f) map[string]int`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/service_gitlab_test.go`（本任务先写核心组；Task 3 在同文件追加语义组）：

```go
package codedelivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

func gitlabPrepareInput() PrepareInput {
	return PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gl",
		Repo:          RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA:   "b" + strings.Repeat("0", 39),
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
}

// seededGitLabFixture：工作区在基线之上修改 main.go，prepare + approve 完成
// （GitLab 通路；与 github 侧 seededFixture 同构）。
func seededGitLabFixture(t *testing.T) *deliveryFixture {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	return f
}

func snapshotGitLabCalls(f *deliveryFixture) map[string]int {
	out := map[string]int{}
	for k, v := range f.gitlab.Calls() {
		out[k] = v
	}
	return out
}

func mustBranchCommit(t *testing.T, e *gitLabEmulator, branch string) string {
	t.Helper()
	sha, ok := e.BranchCommit(branch)
	require.True(t, ok)
	return sha
}

func mustMaterialJSON(t *testing.T) []byte {
	t.Helper()
	mat := DeliveryMaterial{
		Repo: RepoRef{Owner: "octocat", Name: "hello"}, BaselineSHA: "b" + strings.Repeat("0", 39),
		Branch: TaskBranchOf("s-9"), Files: []FileChange{{Path: "main.go"}},
		CommitMessage: "m", PRTitle: "t",
	}
	raw, err := mat.CanonicalJSON()
	require.NoError(t, err)
	return raw
}

// AC（验收 2 权限面）：提供者从连接的安装 app id 服务端权威解析；A03 行以
// gitlab.deliver 锚定；回执（服务端 SHA/MR/远端身份/批准人）全部落账，且
// 提交 SHA 是推送后读回的远端权威 head。
func TestGitLabDeliveryE2E_RecordsTraceableReceipts(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()

	view := firstDelivery(t, f)
	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", view.ActionID).First(&row).Error)
	require.Equal(t, "gitlab.deliver", row.Target)
	require.Equal(t, "deliver", row.Risk)

	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.CommitSHA)
	require.EqualValues(t, 1, view.PRNumber)
	require.Contains(t, view.PRURL, "/-/merge_requests/1")
	require.Equal(t, "gl-user", view.RemoteLogin, "实际远端身份必须落账")
	require.Equal(t, "u1", view.Approver, "审批内容（批准人）必须可追溯")
	require.NotEmpty(t, view.Digest)

	sha, ok := f.gitlab.BranchCommit("weknora/task/s-1")
	require.True(t, ok)
	require.Equal(t, view.CommitSHA, sha, "回执必须是读回的远端权威 head")
	require.Equal(t, "b"+strings.Repeat("0", 39), mustBranchCommit(t, f.gitlab, "main"))
	require.Empty(t, f.gitlab.Violations())
}

// 基线物化走同一 seam：GitLab 适配器的 Tree/Blob 把基线树写入工作区固定根。
func TestGitLabMaterializeBaselineWritesFixedTree(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	in := BaselineInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gl",
		Repo:        RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39),
	}
	receipt, err := f.svc.MaterializeBaseline(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Files)
	require.Equal(t, "/workspace/octocat/hello", receipt.Root)
	raw, err := os.ReadFile(filepath.Join(f.root, "octocat/hello/main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(raw))
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 1：连接背后的安装 app 不是代码平台 → 任何远端调用之前
// fail closed；派发面对未知 target 同样拒绝（零远端调用）。
func TestGitLabUnsupportedProviderFailsClosed(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	require.NoError(t, f.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-notion", AppID: "notion", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, f.db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-notion", InstallationID: "inst-notion", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-notion:notion", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	in := gitlabPrepareInput()
	in.ConnectionID = "conn-notion"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrUnsupportedProvider)
	require.Zero(t, f.gitlab.Calls()["GET /project"], "提供者拒绝必须发生在任何远端读之前")
	require.Zero(t, f.gitlab.Calls()["GET /repository/tree"])

	// 派发面：合法材料 + 未知 target → 前置门拒绝，零远端调用。
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Target: "notion.deliver", Args: mustMaterialJSON(t)}
	_, err = f.dispatcher.Dispatch(ctx, snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
}

// Review Focus 3（篡改面）：派发器解析失败 = ErrDispatchNotStarted，零 GitLab 调用
// （prepare 阶段的远端读已发生，故以「派发前后调用计数不变」精确断言）。
func TestGitLabTamperedSnapshotNeverReachesGitLab(t *testing.T) {
	f := seededGitLabFixture(t)
	before := snapshotGitLabCalls(f)
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Target: "gitlab.deliver", Args: []byte(`{"repo":"o/n"}`)}
	_, err := f.dispatcher.Dispatch(context.Background(), snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	after := snapshotGitLabCalls(f)
	require.Equal(t, before, after, "篡改快照的派发必须零远端调用")
}
```

同时**修改** `internal/modules/codedelivery/service_prepare_test.go`（夹具扩展，6 处精确替换）：

①`deliveryFixture` 结构体加字段：
```go
type deliveryFixture struct {
	db          *gorm.DB
	store       *deliveryrepo.DeliveryStore
	actions     *appconnectorsvc.ActionService
	svc         *CodeDeliveryService
	github      *githubEmulator
	gitlab      *gitLabEmulator
	workspace   WorkspaceFileSource
	root        string
	connections *fixtureConnections
	dispatcher  *DeliveryDispatcher
}
```

②`newDeliveryFixture` 内、GitHub 连接行创建之后追加 GitLab 连接（逐字）：
```go
	// 个人 GitLab 连接：inst-gl / conn-gl，owner=u1，active（T24 #54）。
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gl", AppID: "gitlab", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gl", InstallationID: "inst-gl", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gl:gitlab", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)
```

③模拟器与工厂装配处（`e := newGitHubEmulator(t)` 之后）：
```go
	gl := newGitLabEmulator(t)
	gitlabFactory := NewGitLabClientFactory(http.DefaultClient, gl.srv.URL)
	providers := appconnectorrepo.NewInstallationStore(db)
```

④`dispatcher := NewDeliveryDispatcher(...)` 替换为：
```go
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, GitLab: gitlabFactory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: fixtureRun{sessionID: "s-1"},
	})
```

⑤`svc := NewCodeDeliveryService(...)` 替换为：
```go
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, GitLab: gitlabFactory, Providers: providers,
		Workspace: workspace, Runs: fixtureRun{sessionID: "s-1"},
		Dispatcher: dispatcher,
	})
```

⑥`return &deliveryFixture{...}` 追加 `gitlab: gl`：
```go
	return &deliveryFixture{db: db, store: store, actions: actions, svc: svc, github: e, gitlab: gl, workspace: workspace, root: root, connections: connections, dispatcher: dispatcher}
```

⑦`fixtureConnections` 两个方法各加 gitlab 分支（模拟器令牌是夹具自约定的假值，非真实凭据）：
```go
func (f *fixtureConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	if strings.HasSuffix(c.CredentialRef, ":gitlab") {
		return []byte("glpat-testtoken"), nil
	}
	return []byte("gho_testtoken"), nil
}
```
```go
func (f *fixtureConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	if f.credsBroken {
		return nil, errors.New("credential row deleted")
	}
	if strings.HasSuffix(connectionID, "-gl") {
		return []byte("glpat-testtoken"), nil
	}
	return []byte("gho_testtoken"), nil
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: FAIL（`f.gitlab` 字段、`CodeDeliveryDeps.GitLab/Providers`、`DispatcherDeps.GitLab`、`NewGitLabClientFactory` 装配位未定义/未编译——service_gitlab_test.go 编译错误）

- [ ] **Step 3: 写最小实现**

①`service.go` — `CodeDeliveryDeps` 增两字段（`GitHub` 字段之后）：
```go
type CodeDeliveryDeps struct {
	Store       *deliveryrepo.DeliveryStore
	Actions     *appconnectorsvc.ActionService
	ActionRows  appconnectorsvc.ActionStoreSource
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	// GitHub 与 GitLab 是统一 seam 背后的两个平台适配器（T24 #54）；缺失
	// 的适配器让对应平台在 prepare 面即 fail closed。
	GitHub GitHubClientFactory
	GitLab CodePlatformClientFactory
	// Providers 把连接解析到其安装 app id（=平台名）。生产实现是
	// appconnectorrepo.InstallationStore；nil 一律 fail closed。
	Providers ProviderSource
	Workspace WorkspaceFileSource
	Runs      RunReader
	// Dispatcher executes approved deliveries and recovers pushed ones
	// (Task 6). Nil keeps the prepare-only wiring usable; a dispatch on a
	// pushed row without it fails closed.
	Dispatcher *DeliveryDispatcher
}
```

②`service.go` — `authorize` 返回连接（签名与两处调用点同步）：
```go
// authorize is the shared owner-only predicate: caller owns the run AND uses
// their own personal connection (CONTEXT.md 个人连接只能由其所有者使用).
// It returns the run's sessionID (= taskID, ADR-0004) and the loaded
// connection (provider resolution needs its installation); the service keeps
// no mutable state.
func (s *CodeDeliveryService) authorize(ctx context.Context, tenantID uint64, callerID, runID, connectionID string) (string, appconnector.Connection, error) {
	run, err := s.deps.Runs.GetOwnedRun(ctx, tenantID, callerID, runID)
	if err != nil || run.SessionID == "" {
		return "", appconnector.Connection{}, fmt.Errorf("%w: run %s", ErrNotDeliveryOwner, runID)
	}
	conn, err := s.deps.Connections.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", appconnector.Connection{}, err
	}
	if conn.Kind != appconnector.ConnectionKindPersonal || conn.OwnerID != callerID ||
		conn.TenantID != tenantID || conn.State != appconnector.ConnectionActive {
		return "", appconnector.Connection{}, ErrConnectionNotUsable
	}
	return run.SessionID, conn, nil
}

// platformProvider resolves the delivery platform from the connection's
// installation app id — server-side authority, never client input (T24 #54).
func (s *CodeDeliveryService) platformProvider(ctx context.Context, conn appconnector.Connection) (string, error) {
	if s.deps.Providers == nil {
		return "", fmt.Errorf("%w: provider source not wired", ErrUnsupportedProvider)
	}
	inst, err := s.deps.Providers.GetInstallationByID(ctx, conn.TenantID, conn.InstallationID)
	if err != nil {
		return "", err
	}
	if inst.AppID != ProviderGitHub && inst.AppID != ProviderGitLab {
		return "", fmt.Errorf("%w: connection app %q is not a code platform", ErrUnsupportedProvider, inst.AppID)
	}
	return inst.AppID, nil
}

// clientFor picks the platform adapter hidden behind the unified seam.
func (s *CodeDeliveryService) clientFor(provider, token string, repo RepoRef) (CodePlatformClient, error) {
	return clientForPlatform(s.deps.GitHub, s.deps.GitLab, provider, token, repo)
}
```

③`service.go` — `MaterializeBaseline` 开头段替换（authorize 调用点 + provider/client 路由）：
```go
func (s *CodeDeliveryService) MaterializeBaseline(ctx context.Context, in BaselineInput) (BaselineReceipt, error) {
	sessionID, conn, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	if !baselineSHALegal(in.BaselineSHA) {
		return BaselineReceipt{}, fmt.Errorf("%w: %q", ErrInvalidBaselineSHA, in.BaselineSHA)
	}
	provider, err := s.platformProvider(ctx, conn)
	if err != nil {
		return BaselineReceipt{}, err
	}
	token, err := s.tokenFor(ctx, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	client, err := s.clientFor(provider, token, in.Repo)
	if err != nil {
		return BaselineReceipt{}, err
	}
	tree, err := s.baselineTree(ctx, client, in.BaselineSHA)
	// ……（其余与现有实现逐字相同：MaxDeliveryFiles 校验、
	// WorkspaceRepoRoot、逐文件 Blob、maxBaselineBytes、
	// WriteSessionWorkspaceFiles、BaselineReceipt 返回）
```

④`service.go` — `PrepareDelivery` 同段替换 + target 权威化：
```go
func (s *CodeDeliveryService) PrepareDelivery(ctx context.Context, in PrepareInput) (DeliveryView, error) {
	sessionID, conn, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if in.CommitMessage == "" || in.PRTitle == "" {
		return DeliveryView{}, fmt.Errorf("%w: commit_message and pr_title are required", ErrInvalidMaterial)
	}
	if !baselineSHALegal(in.BaselineSHA) {
		return DeliveryView{}, fmt.Errorf("%w: %q", ErrInvalidBaselineSHA, in.BaselineSHA)
	}
	provider, err := s.platformProvider(ctx, conn)
	if err != nil {
		return DeliveryView{}, err
	}
	token, terr := s.tokenFor(ctx, in.ConnectionID)
	if terr != nil {
		return DeliveryView{}, terr
	}
	client, err := s.clientFor(provider, token, in.Repo)
	if err != nil {
		return DeliveryView{}, err
	}
	// ……（护栏 1/branchShapeLegal/client.Repository/BranchProtected/
	// RefuseProtectedTarget/ValidateTaskBranch 与现有实现逐字相同）
```
及 A03 锚定段替换（conn 已由 authorize 装载，删除原 `conn, err := s.deps.Connections.FindConnectionByID(...)` 四行重取）：
```go
	// 锚定 A03：digest 绑定 repo/基线/分支/文件清单/提交信息/PR 标题 + 连接
	// 版本；内容变化=新 digest=旧批准失效（immutable approval anchor）。
	// conn 已由 authorize 装载（T24 #54：authorize 返回连接供提供者解析）。
	actionID, err := s.deps.Actions.Prepare(ctx, appconnector.Action{
		TenantID: in.TenantID, ActorID: in.CallerID, ConnectionID: in.ConnectionID,
		Target: DeliveryTargetOf(provider), Risk: appconnector.RiskDeliver,
		AuthVersion: conn.AuthVersion, Args: args,
	})
```

⑤`dispatcher.go` — `DispatcherDeps` 增字段：
```go
type DispatcherDeps struct {
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	Guard       appconnectorsvc.A02Guard
	GitHub      GitHubClientFactory
	GitLab      CodePlatformClientFactory
	Workspace   WorkspaceFileSource
	Store       *deliveryrepo.DeliveryStore
	ActionRows  appconnectorsvc.ActionStoreSource
	Runs        RunReader
}

// clientForTarget routes the approved action's platform target to its
// adapter — the ONLY platform switch on the dispatch face (T24 #54).
func (d *DeliveryDispatcher) clientForTarget(target, token string, repo RepoRef) (CodePlatformClient, error) {
	provider, err := ProviderOfTarget(target)
	if err != nil {
		return nil, err
	}
	return clientForPlatform(d.deps.GitHub, d.deps.GitLab, provider, token, repo)
}
```

⑥`dispatcher.go` — `Dispatch` 前置门顺序（平台路由在 row 读之前，均为出网前拒绝）：
```go
func (d *DeliveryDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	if err := d.deps.Guard.Check(ctx,
		appconnector.OCSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: a02: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: credential: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	// 平台路由是前置门（T24 #54）：未知目标/未接线适配器在此拒绝，零远端调用。
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: delivery row: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	return d.deliver(ctx, snap, material, row, client, false)
}
```

⑦`dispatcher.go` — `RecoverPullRequest` 客户端构造替换（原 `_, err = d.deliver(ctx, snap, material, row, d.deps.GitHub(token, material.Repo), true)`）：
```go
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return err
	}
	_, err = d.deliver(ctx, snap, material, row, client, true)
	return err
```

⑧`dispatcher.go` — `QueryProvider` 客户端构造替换（原 `client := d.deps.GitHub(token, material.Repo)`）：
```go
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchUnknown, err)
	}
```

⑨`dispatcher.go` — `deliver` 推送半程尾部（原「RecordReceipts(commitSHA) → EnsureBranch」顺序对调 + 权威读回）：
```go
		if err := client.EnsureBranch(ctx, material.Branch, commitSHA); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		// 权威回执以远端为准（T24 #54）：推送后读回分支现 head——GitHub 上
		// 它等于 CreateCommit 的结果；GitLab 的 commits API 由服务端定 sha，
		// 本地占位值绝不进入台账。
		head, pushed, herr := client.BranchHead(ctx, material.Branch)
		if herr != nil {
			return appconnectorsvc.DispatchOutcome{}, herr
		}
		if !pushed {
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: branch %s absent after push", ErrGitHubTransport, material.Branch)
		}
		commitSHA = head
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: commitSHA}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryDispatched), string(DeliveryPrepared), string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
```

⑩`internal/container/code_delivery.go` — `newCodeDeliveryService` 装配替换 + import（Task 4 也可，但编译依赖在此：`CodeDeliveryDeps` 已有新字段，零值 nil 合法，容器可在 Task 2 先接 GitHub 侧不动；为最小化任务间耦合，**容器接线放 Task 4**，本任务不改容器——新字段零值即 GitHub-only 部署的合法现状）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（Task 1+2 全部测试 + 既有 #52 全部测试全绿——含 9 个 GitHub dispatch 测试的相对调用计数断言与权威读回兼容）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/service.go internal/modules/codedelivery/dispatcher.go internal/modules/codedelivery/service_prepare_test.go internal/modules/codedelivery/service_gitlab_test.go
git commit -m "feat(codedelivery): server-authoritative provider routing + remote head read-back; GitLab orchestration core tests (T24 #54 task 2)"
```

---

### Task 3: Go——GitLab 交付编排语义测试（部分完成/unknown/迭代收敛/删除/护栏次序/A02）

**Files:**
- Test: `internal/modules/codedelivery/service_gitlab_test.go`（追加语义组）

**Interfaces:**
- Consumes: Task 2 的全部 Produces 与测试助手；`gitLabEmulator` 的 `failNextMRCreation`/`blackoutAfterCommitCreate`/`liftBlackout`/`protectPattern`/`BranchTree`/`ParentOf`（Task 1）。
- Produces: 无新符号（本任务是最高稳定 Interface 的语义验收面；若测试暴露适配器缺陷，修复落在 `gitlab_client.go`/`dispatcher.go` 并在同一提交内说明）。

- [ ] **Step 1: 写失败测试（在 `service_gitlab_test.go` 末尾追加）**

```go
// Review Focus 5（护栏次序）：目标分支 = 默认分支，或命中 GitLab 通配保护
// 模式 → ErrProtectedBranch 且零远端写。通配匹配发生在适配器内（差异隐藏）。
func TestGitLabPrepareRefusesProtectedBranchWithZeroRemoteWrites(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	// 仓库默认分支是 main：与 GitHub 同序，先撞保护分支闸。
	in := gitlabPrepareInput()
	in.Branch = "main"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	// GitLab 保护分支是「精确名或通配模式」：模拟器追加 weknora/task/* 模式，
	// 适配器必须本地完成通配匹配后拒绝。
	in = gitlabPrepareInput()
	in.Branch = TaskBranchOf("s-stable")
	f.gitlab.protectPattern("weknora/task/*")
	_, err = f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 2（部分完成）：推送成功、MR 创建确定性失败 → pushed；恢复
// 只补 MR，绝不重发 commits actions（调用计数为证）。
func TestGitLabPartialPushMRFailureRecoversWithoutRepush(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	f.gitlab.failNextMRCreation()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State)
	require.NotEmpty(t, view.CommitSHA, "推送提交必须已落账（读回的远端 head）")

	before := snapshotGitLabCalls(f)
	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // MR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	after := snapshotGitLabCalls(f)
	require.Equal(t, before["POST /repository/commits"], after["POST /repository/commits"], "恢复不得重发提交")
	require.Equal(t, before["POST /merge_requests"]+1, after["POST /merge_requests"], "恢复只补 MR")
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 4（不可观测）：commits POST 落地后断网 → unknown；resolve
// 以远端事实收敛（分支已收敛、MR 缺席 → pushed）；随后 MR-only 恢复完成交付。
func TestGitLabUnknownOutcomeResolvesFromRemoteFacts(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	f.gitlab.blackoutAfterCommitCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	require.Equal(t, "unknown", view.ActionState)

	f.gitlab.liftBlackout()
	view, err = f.svc.ResolveDeliveryUnknown(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State, "远端事实：分支已收敛、MR 缺席 → 部分完成")

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.Empty(t, f.gitlab.Violations())
}

// 同任务分支二次交付（CONTEXT.md「创建或更新草稿 PR」迭代语义）：GitLab 的
// commits API 在分支现 tip 之上追加提交，把分支收敛到新预期树；草稿 MR 复用
// 同 source branch 的既有开放 MR；祖先链证明永不改写历史。
func TestGitLabSecondDeliveryConvergesTaskBranchAndReusesMR(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()

	first, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), first.State)
	firstCommit := first.CommitSHA

	// 批准之后工作区再变 → 新 Prepare（新 digest）→ 新批准 → 二次交付。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/main.go"),
		[]byte("package main\n\nfunc main() { _ = 1 }\n"), 0o644))
	second, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest, "内容变化必须铸造新 digest")
	require.NoError(t, f.actions.Approve(ctx, second.ActionID, "u1", second.Digest))

	secondView, err := f.svc.DispatchDelivery(ctx, dispatchInput(second))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), secondView.State)
	require.NotEqual(t, firstCommit, secondView.CommitSHA)

	// 收敛不变量：任务分支 tip 树恰好 = 基线树 + 批准变更（无残留、无多余）。
	tip := f.gitlab.BranchTree("weknora/task/s-1")
	require.Len(t, tip, 2)
	require.Equal(t, GitBlobSHA([]byte("package main\n\nfunc main() { _ = 1 }\n")), tip["main.go"])
	require.Equal(t, GitBlobSHA([]byte("# hello\n")), tip["README.md"])
	// 祖先链：新提交以分支现 tip 为 parent（永不 force）。
	require.Equal(t, []string{firstCommit}, f.gitlab.ParentOf(secondView.CommitSHA))
	// 草稿 MR 迭代复用（同 source branch 的开放 MR 不重复开）。
	require.Equal(t, first.PRNumber, secondView.PRNumber)
	require.Empty(t, f.gitlab.Violations())
}

// 删除型交付：基线有而工作区无的文件 → commits API delete action，分支
// tip 树精确收敛为「基线 − 删除 + 新增」。
func TestGitLabDeliveryAppliesDeletionActions(t *testing.T) {
	// 工作区只写 README（基线的 main.go 不在工作区）→ diff = main.go 删除。
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)

	tip := f.gitlab.BranchTree("weknora/task/s-1")
	require.Len(t, tip, 1, "删除必须体现在分支 tip 树上")
	_, hasMain := tip["main.go"]
	require.False(t, hasMain)
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 3（A02 面）：连接 owner 失去成员资格 → A02 拒绝，零远端调用。
func TestGitLabDispatchFailsClosedWhenConnectionUnusable(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	membersDrop(f, "u1")
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.Error(t, err)
	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
}
```

- [ ] **Step 2: 运行测试确认失败/通过判定**

Run: `go test ./internal/modules/codedelivery/ -count=1 -run 'TestGitLab'`
Expected: 本任务为纯测试追加——若全部直接 PASS，说明 Task 1/2 实现已覆盖语义（合法结果，记录于提交信息）；任何 FAIL 均为适配器/编排缺陷，按 RED→GREEN 修复后重跑。

- [ ] **Step 3: 修复暴露的缺陷（如有）**

按失败断言逐条修复 `gitlab_client.go`/`dispatcher.go`/`service.go`；无占位修复——每处修复必须对应该断言语义。

- [ ] **Step 4: 运行全部测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（全部绿）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/service_gitlab_test.go internal/modules/codedelivery/
git commit -m "test(codedelivery): GitLab delivery orchestration semantics — partial completion, unknown resolution, branch convergence, deletions, guardrails, A02 (T24 #54 task 3)"
```

---

### Task 4: Go——GitLab OAuth 注册、HTTP 错误映射与容器接线

**Files:**
- Modify: `internal/handler/app_connector_oauth.go`（3 处最小追加）
- Modify: `internal/handler/session/workbench_delivery.go`（`writeDeliveryError` 增 1 case）
- Modify: `internal/handler/session/workbench_delivery_test.go`（错误分类表增 1 行）
- Modify: `internal/container/code_delivery.go`（gitlab 工厂 + providers 接线）
- Test: `internal/handler/app_connector_oauth_gitlab_test.go`

**Interfaces:**
- Consumes: Task 1 的 `ErrUnsupportedProvider`/`ProviderGitLab`；`appOAuthDefaults`/`DefaultAppOAuthProviderConfigs`/`exchangeAppOAuthCode(ctx, cfg, appID, code, redirectURI)`/`appAuthorizeURL(cfg, appID, state, redirectURI)`（`app_connector_oauth.go:48-61/:102/:220`）；env 自动拾取循环 `WEKNORA_APP_OAUTH_<APP>_CLIENT_ID/_SECRET`（`internal/container/container.go:1016-1030`，对 `appOAuthDefaults` 全键生效，gitlab 零额外接线）；`writeDeliveryError`（`workbench_delivery.go:208`）。
- Produces: GitLab 个人连接的 OAuth 获取路径（authorize scope `api read_user`、form 交换形状与 GitHub 同形）；HTTP 错误码 `code_delivery_unsupported_provider`（400）；生产容器同时接线两个平台适配器与 `Providers`。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/app_connector_oauth_gitlab_test.go`：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// GitLab OAuth 注册与交换形状（T24 #54）：gitlab 在默认端点表中、token 交换
// 发 form-encoded（与 github 同形）+ Accept: application/json、authorize URL
// 带 api read_user scope（交付链写任务分支与草稿 MR；read_user 记录实际远端身份）。
func TestGitLabOAuthRegistrationAndExchangeShape(t *testing.T) {
	cfg := DefaultAppOAuthProviderConfigs()
	gitlab, ok := cfg["gitlab"]
	require.True(t, ok, "gitlab must be a first-batch OAuth app")
	require.Equal(t, "https://gitlab.com/oauth/authorize", gitlab.AuthorizeURL)
	require.Equal(t, "https://gitlab.com/oauth/token", gitlab.TokenURL)
	_, known := appOAuthDefaults["gitlab"]
	require.True(t, known)

	// 交换形状：对 httptest token 端点做真实 HTTP。
	var gotContentType, gotAccept string
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		require.NoError(t, r.ParseForm())
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "glpat_x", "token_type": "bearer", "scope": "api read_user"})
	}))
	defer srv.Close()

	token, err := exchangeAppOAuthCode(t.Context(), AppOAuthProviderConfig{
		AuthorizeURL: gitlab.AuthorizeURL, TokenURL: srv.URL,
		ClientID: "cid", ClientSecret: "csecret",
	}, "gitlab", "the-code", "https://deploy.example.com/api/v1/apps/connections/oauth/callback")
	require.NoError(t, err)
	require.Equal(t, "glpat_x", token.AccessToken)
	require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	require.Equal(t, "application/json", gotAccept)
	require.Equal(t, "cid", gotForm.Get("client_id"))
	require.Equal(t, "csecret", gotForm.Get("client_secret"))
	require.Equal(t, "the-code", gotForm.Get("code"))
	require.Equal(t, "authorization_code", gotForm.Get("grant_type"))
	require.Equal(t, "https://deploy.example.com/api/v1/apps/connections/oauth/callback", gotForm.Get("redirect_uri"))
}

func TestGitLabAuthorizeURLCarriesAPIScope(t *testing.T) {
	_, known := appOAuthDefaults["gitlab"]
	require.True(t, known, "gitlab must be a first-batch OAuth app")
	cfg := AppOAuthProviderConfig{
		AuthorizeURL: "https://gitlab.com/oauth/authorize",
		TokenURL:     "https://gitlab.com/oauth/token",
		ClientID:     "cid", ClientSecret: "csecret",
	}
	got := appAuthorizeURL(cfg, "gitlab", "st-1", "https://deploy.example.com/cb")
	require.Contains(t, got, "https://gitlab.com/oauth/authorize?")
	require.Contains(t, got, "scope=api+read_user")
	require.Contains(t, got, "response_type=code")
	require.Contains(t, got, "state=st-1")
	require.Contains(t, got, "client_id=cid")
}
```

**修改** `internal/handler/session/workbench_delivery_test.go` 的错误分类表 `TestDeliveryErrorClassificationTable`：在 `{"dispatch rejected pre-send", ...}` 行后追加一行：

```go
		{"unsupported provider", codedelivery.ErrUnsupportedProvider, http.StatusBadRequest, "code_delivery_unsupported_provider"},
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1 && go test ./internal/handler/session/ -run 'TestDeliveryErrorClassificationTable' -count=1`
Expected: FAIL（gitlab 不在端点表/交换 switch 未命中 → `appOAuthConfigError`；分类表无 400 unsupported 行）

- [ ] **Step 3: 写最小实现**

①`internal/handler/app_connector_oauth.go` — `appOAuthDefaults` 增条目（`"github"` 条目后）：
```go
	"gitlab": {
		AuthorizeURL: "https://gitlab.com/oauth/authorize",
		TokenURL:     "https://gitlab.com/oauth/token",
	},
```

②`exchangeAppOAuthCode` switch 在 `case "github":` 块后追加：
```go
	case "gitlab":
		// GitLab 的 token 端点与 github 同形：form-encoded 客户端凭据 +
		// 授权码，返回 JSON（T24 #54：连接获取路径与 GitHub 一致）。
		form := url.Values{}
		form.Set("client_id", cfg.ClientID)
		form.Set("client_secret", cfg.ClientSecret)
		form.Set("code", code)
		form.Set("grant_type", "authorization_code")
		form.Set("redirect_uri", redirectURI)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
```

③`appAuthorizeURL` switch 在 `case "github":` 后追加：
```go
	case "gitlab":
		q.Set("response_type", "code")
		q.Set("scope", "api read_user")
```

④`internal/handler/session/workbench_delivery.go` — `writeDeliveryError` 在 `ErrDeliveryDispatchRejected` case 后追加：
```go
	case errors.Is(err, codedelivery.ErrUnsupportedProvider):
		// 连接背后的安装 app 不是代码平台（T24 #54）：可证未出网的输入类
		// 错误，400 固定码，不落 5xx。
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "code_delivery_unsupported_provider", "error": "the connection is not a code platform connection"})
```

⑤`internal/container/code_delivery.go` — `newCodeDeliveryService` 装配替换 + import `appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"`：
```go
	workspace := &sandboxWorkspaceSource{mgr: sandboxMgr, resolver: resolver}
	store := deliveryrepo.NewDeliveryStore(db)
	creds := appconnectorsvc.NewCredentialResolver(connections)
	factory := codedelivery.NewGitHubClientFactory(nil, codedelivery.GitHubAPIBaseURL)
	gitlabFactory := codedelivery.NewGitLabClientFactory(nil, codedelivery.GitLabAPIBaseURL)
	providers := appconnectorrepo.NewInstallationStore(db)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: creds, Guard: guard,
		GitHub: factory, GitLab: gitlabFactory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	return codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: creds,
		GitHub: factory, GitLab: gitlabFactory, Providers: providers,
		Workspace: workspace, Runs: runs, Dispatcher: dispatcher,
	}), nil
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1 && go test ./internal/handler/session/ -run 'TestDelivery' -count=1 && go build ./...`
Expected: PASS + 全仓编译通过（容器接线类型正确；`WEKNORA_APP_OAUTH_GITLAB_CLIENT_ID/_SECRET` 经 container.go:1016-1030 的通用循环自动拾取，零额外改动）

- [ ] **Step 5: Commit**

```bash
git add internal/handler/app_connector_oauth.go internal/handler/app_connector_oauth_gitlab_test.go internal/handler/session/workbench_delivery.go internal/handler/session/workbench_delivery_test.go internal/container/code_delivery.go
git commit -m "feat(codedelivery): gitlab OAuth registration, unsupported-provider 400 mapping, container wiring (T24 #54 task 4)"
```

---

### Task 5: 移动面文案中性化（PR → PR/MR）

**Files:**
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx:43-62`（3 字符串 + 1 注释）
- Modify: `apps/mobile/src/app-smoke.test.tsx:1398`（1 处文案钉）

**Interfaces:**
- Consumes: #52 的 `DELIVERY_STATE_COPY`（`TaskDetailScreen.tsx:43`）与交付回执区块文案；app-smoke 的文案断言（`:1398`）。
- Produces: 平台中性的用户文案——GitLab 草稿 MR 与 GitHub 草稿 PR 经同一回执区块呈现，UI 不暴露平台差异（AC1 的读面延伸；数据面零改动）。

- [ ] **Step 1: 写失败测试（修改既有文案钉）**

`apps/mobile/src/app-smoke.test.tsx:1398` 替换：
```ts
  assert.ok(text.includes('已推送，等待草稿 PR/MR 恢复'), 'pushed state uses honest copy');
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test 2>&1 | tail -4`
Expected: FAIL 1（app-smoke 文案断言）

- [ ] **Step 3: 最小实现**

`apps/mobile/src/screens/TaskDetailScreen.tsx`：
```ts
/** 交付六态的如实中文文案：不粉饰部分完成（pushed）与不可观测（unknown）。
 * PR/MR 中性措辞：GitLab 草稿 MR 与 GitHub 草稿 PR 经同一统一回执呈现（T24 #54）。 */
export const DELIVERY_STATE_COPY: Record<DeliveryState, string> = {
  prepared: '待审批：审阅 Diff 与候选提交后在行动收件箱批准',
  dispatched: '交付进行中：正在推送任务分支',
  pushed: '已推送，等待草稿 PR/MR 恢复',
  delivered: '草稿 PR/MR 已创建',
  failed: '交付失败',
  unknown: '远端结果待确认',
};
```
区块内标签行（原 `PR：{delivery.prUrl}`）：
```tsx
      {delivery.prUrl !== undefined ? <Text numberOfLines={1}>PR/MR：{delivery.prUrl}</Text> : null}
```
区块注释（原 `/** 交付回执区块：只读呈现服务端落账的追溯字段（仓库/分支/提交/PR/远端身份/批准人）。 */`）：
```tsx
/** 交付回执区块：只读呈现服务端落账的追溯字段（仓库/分支/提交/PR·MR/远端身份/批准人）。 */
```

- [ ] **Step 4: 运行测试与类型检查确认通过**

Run: `pnpm --filter @weknora/mobile test 2>&1 | tail -4 && pnpm --filter @weknora/mobile typecheck`
Expected: 215+ pass / 0 fail；typecheck 通过

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): platform-neutral delivery copy — draft PR/MR wording (T24 #54 task 5)"
```

---

## 计划级验证命令

在 worktree 根（`.worktrees/issue30-sweep`）一次执行（同 `testCommand`）：

```bash
go build ./... && go test ./internal/modules/codedelivery/ -count=1 && go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1 && go test ./internal/handler/session/ -run 'TestDelivery' -count=1 && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

期望：`go build` 无输出；codedelivery 包全部测试（既有 #52 全套 + 本计划新增 14 个：wire 3 + real 1（blocked-env skip）+ service 编排 10）ok；`TestGitLabOAuthRegistrationAndExchangeShape`/`TestGitLabAuthorizeURLCarriesAPIScope` ok；session 包 `TestDelivery*` 六个测试 ok（**勿用更宽的 `-run 'Delivery'`**——见差异记录 8）；移动端全量 pass / 0 fail（干净树上执行；若干 skip 为既有 opt-in 集成证据门控）；typecheck 0 error。

**本计划作者在当前 HEAD 的实跑证据（计划代码逐字落地验证后已按「只写计划文件」约定移除实现痕迹）**：
- Task 1：`go test ./internal/modules/codedelivery/ -count=1 -v -run 'TestGitLab|TestDraftMRTitle'` → wire/classification/glob 三测 PASS + `TestGitLabClientAgainstRealGitLab` SKIP（blocked-env，如实）
- Task 2+3：`go test ./internal/modules/codedelivery/ -count=1` → **ok**（0.665s；既有 #52 全部测试 + 本计划 GitLab 13 测全绿——含权威读回后 GitHub 侧调用计数断言全兼容、部分完成恢复零重推、unknown 远端收敛、迭代收敛+祖先链、删除型交付、A02/提供者 fail closed）
- Task 4：`go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1` → **ok**（2.313s）；`go test ./internal/handler/session/ -run 'TestDelivery' -count=1` → **PASS**（含新 `unsupported_provider → 400` 分类行）；`go build ./...` → **BUILD_OK**
- Task 5：`pnpm exec tsx --test src/app-smoke.test.tsx`（apps/mobile 内文件级作用域，见差异记录 9）→ `the task detail screen renders the code delivery receipt section with honest state copy` PASS（PR/MR 新文案断言）
- 基线：`go test ./internal/modules/codedelivery/ -count=1` → ok（0.793s）；`pnpm --filter @weknora/mobile test` → pass 215 / fail 0 / skipped 11（9057ms，同批并行写入开始前）；`go version` → go1.26.3 darwin/arm64；`url.PathEscape("octocat/hello")` = `octocat%2Fhello` 且 httptest `r.URL.EscapedPath()` 透传 `%2F`（临时程序实跑验证——GitLab 模拟器按 EscapedPath 匹配的事实依据）

## blocked-env 验收项清单（不得伪造通过）

| 项 | 门控 | 本地替代证据 |
|---|---|---|
| 真实 gitlab.com REST 链（project/tree/blob/branches/commits/merge_requests/user） | `WEKNORA_GITLAB_TEST_TOKEN` + `WEKNORA_GITLAB_TEST_PROJECT`（`TestGitLabClientAgainstRealGitLab`，缺即 skip） | Task 1 httptest 模拟器 wire 测试（真实 HTTP 字节 + 真实 git blob sha 对象库） |
| 真实 GitLab OAuth 应用授权码交换 | 部署 env `WEKNORA_APP_OAUTH_GITLAB_CLIENT_ID/_SECRET`（生产自动拾取） | Task 4 httptest token 端点的交换形状真实 HTTP 断言 |
| 真实端到端：GitLab 个人连接 → 基线→diff→审批→任务分支→草稿 MR | 部署 + GitLab 账号 + 测试仓库 | Task 2/3 服务编排 Interface 测试（真实 sqlite + 真实 A03 + 模拟器，覆盖 AC 全部分支） |

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：AC1「差异被 Adapter 隐藏」→ Task 1（适配器 + 唯一路由 switch + 模拟器违规钉）+ Task 5（读面文案中性）；AC2「权限/幂等/版本不变量与 GitHub 一致」→ Task 2（服务端权威提供者解析、A03 target 权威化、读回）+ Task 3（A02、审批锚、部分完成、unknown、护栏次序）；AC3「最高稳定 Interface」→ Task 2/3 服务编排测试（真实 sqlite + 真实 A03 + 模拟器）+ blocked-env 清单。What-to-build 七要素逐一对位：基线（Task 2 物化测试）、修改（diff 既有）、测试（Run 域既有，非平台操作）、Diff（`DiffAgainstBaseline` 既有）、审批（A03 既有 + target 权威化）、任务分支（EnsureBranch 收敛）、草稿 MR（DraftPullRequest + Draft: 前缀）。无缺口。
2. **占位符扫描**：无 TBD/TODO/"类似 Task N"/草稿残片；全部测试与实现代码完整给出；修改类任务的替换块以「原代码锚点 + 逐字新代码」表述（对既有实现保留部分显式标注"与现有实现逐字相同"并列举其内容）。Step 2（Task 3）明确「纯测试追加允许直接 PASS」的判定规则，不伪造 RED。**额外验证：本计划 Task 1–4 的全部代码与测试已由计划作者在当前 HEAD 逐字临时落地实跑（含 3 处计划修正：`gitlab_real_test.go` 未用 import、MR 复用 `Created=false` 语义、PrepareDelivery 中 conn 重复声明——均已在计划正文同步修正），Task 5 以文件级作用域取证；全部实现痕迹已按「只写计划文件」约定移除，证据见「计划级验证命令」节，非纸面推演。**
3. **类型/签名一致性**：`CodePlatformClientFactory = GitHubClientFactory` 别名使 `deps.GitHub`/`deps.GitLab` 同型；`authorize` 新三元签名与两处调用点同步改写；`clientForTarget(target, token, repo)` 与 `Dispatch/RecoverPullRequest/QueryProvider` 三处调用一致；模拟器方法名（`failNextMRCreation`/`blackoutAfterCommitCreate`/`protectPattern`/`BranchTree`/`ParentOf`/`mustBranchCommit`）在 Task 1/2/3 间逐字一致；`writeJSON` 复用不重定义、`mrURL` 不与 `prURL` 冲突；`glpat-testtoken` 与夹具 `LoadCredential/Resolve` 分支值逐字一致；`appconnectorrepo.NewInstallationStore(db).GetInstallationByID` 与 `ProviderSource` 接口签名一致（`install.go:225` 亲读核实）。
4. **Review Focus 落实**：五类失效模式各自落到 Task 1/2/3/4 的具名测试（见 Review Focus 每行标注），无「空泛处理」。

## 执行交接

本计划面向零上下文工程师/子代理执行，推荐 **subagent-driven**（任务间接口耦合中等：Task 2 消费 Task 1 的词汇与适配器，Task 3 只加测试，Task 4/5 独立；每任务独立可审）。执行方式已由编排层指定时按其指定执行。
