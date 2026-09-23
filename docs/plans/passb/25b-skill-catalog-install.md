# Pass B 子计划 25b — B2-AC-Skills 租户 Skill 目录/安装/运行时验证/reaper 边界（20 legacy 文件）

> 实施方式：superpowers:executing-plans / subagent-driven-development，按任务逐个执行，一个任务一个 commit。
> 节点：`b2-ac-skills`（DAG `docs/plans/passb/execution-dag.json`，execution_mode=parallel，role=work，depends_on=ib1）。
> 本计划由「计划撰写-b2-ac-skills」于 2026-09-23 在 worktree `codex/passb-b2-ac-skills`（起点 `8c45a88153d0b20088252dbb29d2fb3815b253c2`，= 当前集成头）撰写；文中全部代码坐标在该 SHA 实测（grep/Read 取证，关键输出摘录见 §2、§12）。

## 0. Spec 与事实源指针

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.4（Agent Catalog 职责：Skill、安装治理，不拥有运行状态）、§5.8（Execution 拥有沙箱配置与用户环境变量）、§4.2（依赖方向、接口定义在使用方模块）、§11（B2 并行 + IB2 串行）、§12（Pass B 迁移循环）、§13（提交隔离 M1–M5 与回滚）、§14.1–14.3（测试梯度与差分）、§15（模块 API 最小化）、§16（禁止以包级可变状态绕过依赖注入） | 边界语义、端口方向、差分义务、提交纪律 |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` :131-139（Agent Catalog 55 文件、25a/25b 并行、25c 跟随、25b=tenant skill catalog/install/runtime verification/reaper）、:29（`_test.go` 随迁、host 不留转发性业务声明）、:32（计数基线）、:38（禁止复制实现/塞 common）、:152（IB2 按 25a→25b→25c 集成） | 面切分、并行安全、门禁口径 |
| 冻结计划 | `docs/plans/passb/00-contract-and-ownership-freeze.md`（B0 产物口径） | 契约冻结流程 |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（禁改跨 owner 文件）、§2 门禁、§3 禁改清单、§4 提交、§5 升级、§6 差分证据、§7 package-private 耦合、§8 计数基线 |
| ownership-matrix | `.worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml`（module=agentcatalog 55 行 `plan=25-agentcatalog-program`、destination、integration_owner=ib2、delete_barrier=ib2；execution 相关行 `plan=13-execution`） | 文件归属与目标包；子面（25a/25b/25c）切分由本计划 §1 分配表表达（passbguard `KnownPlans` 只登记 `25-agentcatalog-program`，tools/passbguard/check.go:64，本节点不改矩阵） |
| contracts.yaml | `.worktrees/passb-int/docs/architecture/passb/contracts.yaml`（owner: agentcatalog 9 条 :9–:149） | 冻结契约：`agentcatalog.facade`（NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）、`agentcatalog.routes`（11 项，含 RegisterSkillRoutes — routes_agent.go:76）、`agentcatalog.lifecycle`（startTenantSkillReaper — container.go:702/2406）、`agentcatalog.workers`（items 空，无 worker）、`agentcatalog.skill-market-service`、`agentcatalog.tenant-skill-market-service`（25c 消费面）；`agentruntime.agent-engine/.agent-service` 的 characterization_tests 含 `tenant_skill_install_test.go`（本面测试锚定 agentruntime 冻结契约）。本节点只读，回写归 IB2 |
| exception-ledger | `.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml`（105 条） | 无 agentcatalog 行（python 遍历实测），本节点零例外删除/新增义务（§9） |
| event-catalog | `.worktrees/passb-int/docs/architecture/passb/event-catalog.yaml` :103-113（`conversation.turn.appended` v1，consumer 含 `internal/application/service/tenant_skill_transcript.go`） | 本面是会话完成事件的消费方，搬迁不改事件契约 |
| MOVE-MANIFEST | `docs/architecture/moves/agentcatalog.yaml`（55 条 legacy_files、integration_points、test_commands） | 25b 行清单与集成点 |
| DAG 裁定 | `execution-dag.json` 节点 `b2-ac-skills`（owned_files/required_contracts/notes）+ `package_private_couplings.pairs`（execution→agentcatalog 4 符号 5 sites；conversation→agentcatalog 2 符号；agentcatalog→conversation 7 符号 10 sites 中 4 个落本面文件） | §2 全量断链面 |
| 上游 Brief | `docs/architecture/passb/`（Knowledge/Agent Runtime/Conversation/Craft/Insights/Workbench）、`docs/architecture/moves/*.yaml`（16 manifest） | 无 agentcatalog 专卷 brief（framework:20 未列），以 manifest + B0 产物为准 |

### 前置条件（开工前逐条核验）

1. **b0 已 done**：`execution-dag.json` b0 节点 `status=done`、`head_sha=d57a2fa708c3fecf5f0510553ea26db3de51d3c9`、`review=approved`（本会话 python 读 DAG 实测）。B0 四产物（ownership-matrix/contracts/event-catalog/exception-ledger.yaml）已固化在集成头。
   - 节点 `b2-ac-skills.notes` 残留「BLOCKED（2026-09-23）：前置 b0 阻塞」为当日早间历史登记文本；b0 已于当日修复并收口（commit `0be903ef2` 合并、`d57a2fa70` 台账覆盖，git log 实测）。实施者以 DAG `status`/`base_sha`/`depends_on` 字段与台账登记为准，不因残留文本停工（与 12-commercial §前置条件 1 同口径）。
2. **depends_on=ib1**：ib1 已执行并登记（commit `8c45a8815`：四支计划合入、三门禁绿色；因 B1 零实施，装配切换未发生——本计划 §2 断链面按「B1 未搬迁」现状编写，非按 13-execution 计划的假想状态），`status=in_progress` 未收口。**派发前置**：协调者将 ib1 置 done 或显式裁定放行 b2 派发；25a/25b 并行的契约前提（framework:139 共享 immutable-version 契约）已满足——`agentcatalog.agent-version-service`（FreezeAgentVersion/GetAgentVersion/ListAgentVersions + AgentVersionSnapshot）`stability: frozen`（contracts.yaml :9-22）。
3. worktree `codex/passb-b2-ac-skills` 起点等于集成头 `8c45a8815`（`git rev-parse HEAD` 实测一致），工作树干净（`git status --short` 空）。
4. **基线门禁绿（撰写者在 8c45a8815 未改动工作树实跑记录，实施者开工前须复跑同命令）**：
   - `go build ./...` → 退出码 0；
   - `go test ./internal/application/service -count=1` → `ok ... 154.512s`；
   - `go test ./internal/application/service -run 'Skill' -count=1` → `ok ... 4.710s`；
   - `go test ./internal/handler -run 'Skill' -count=1` → `ok ... 1.182s`；
   - `go test ./internal/router -run 'ApiKey|Skill' -count=1` → `ok ... 1.363s`；
   - `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16; OK (0 violations)`；
   - `make verify-module-moves` → `modulemove: OK (16 manifests verified)`。
5. **与 25a/25c 文件零交集**：本计划 §1 的 20 个生产文件 + 19 个测试文件与 25a（30 文件：custom agent/versions/persona/expert/subagent/favorites 面）、25c（5 文件：`skill_market_service.go`、`tenant_skill_market_service.go`、`published_skill.go`、`skill_market.go`、`tenant_skill_market.go` 及各自测试）不相交（framework:139 面切分 + manifest 逐行归属）。
6. 本节点 **不改** `internal/modules/agentcatalog/module.go`（conventions §3：门面仅 b0 与集成节点可写）。

## 1. 目标与范围

**目标**：把困在三个水平宿主包的 20 个租户 Skill 面 legacy 文件（manifest `agentcatalog.yaml` 55 行中归属 25b 的行，含 `tenant_skill_reaper.go`——DAG notes 裁定「若归本面：b1-execution 遗留的 4 条 execution→agentcatalog 断链在此导出化收口」，§2.1 实证其归属本面）迁入 `internal/modules/agentcatalog/{repository,service,handler}`，导出化 4 条 execution→agentcatalog 未导出符号与 conversation 依赖的 skillsForRun，并以注入端口解除 4 个搬迁文件对 agentruntime 内部包的 import（§4.4，guard `forbidden-import` 实证），全程 `go build ./...` 绿、既有测试经过渡 shim 全绿，差分证据落盘，向 IB2 交付 Integration Brief。

**范围内（owned_files 展开：生产 20 + 随迁测试 19 + 节点自身产出）**：

| # | legacy 文件（旧路径） | destination（ownership-matrix 冻结） |
|---|---|---|
| 1 | `internal/application/repository/tenant_skill.go` | `internal/modules/agentcatalog/repository/tenant_skill.go`（新包 `repository`） |
| 2 | `internal/application/service/tenant_skill_service.go` | `internal/modules/agentcatalog/service/tenant_skill_service.go`（新包 `service`） |
| 3 | `internal/application/service/tenant_skill_admin.go` | `internal/modules/agentcatalog/service/tenant_skill_admin.go` |
| 4 | `internal/application/service/tenant_skill_bundle.go` | `internal/modules/agentcatalog/service/tenant_skill_bundle.go` |
| 5 | `internal/application/service/tenant_skill_catalog.go` | `internal/modules/agentcatalog/service/tenant_skill_catalog.go` |
| 6 | `internal/application/service/tenant_skill_effective.go` | `internal/modules/agentcatalog/service/tenant_skill_effective.go` |
| 7 | `internal/application/service/tenant_skill_env_declare.go` | `internal/modules/agentcatalog/service/tenant_skill_env_declare.go` |
| 8 | `internal/application/service/tenant_skill_files.go` | `internal/modules/agentcatalog/service/tenant_skill_files.go` |
| 9 | `internal/application/service/tenant_skill_install.go` | `internal/modules/agentcatalog/service/tenant_skill_install.go` |
| 10 | `internal/application/service/tenant_skill_progress.go` | `internal/modules/agentcatalog/service/tenant_skill_progress.go` |
| 11 | `internal/application/service/tenant_skill_reaper.go` | `internal/modules/agentcatalog/service/tenant_skill_reaper.go` |
| 12 | `internal/application/service/tenant_skill_remove.go` | `internal/modules/agentcatalog/service/tenant_skill_remove.go` |
| 13 | `internal/application/service/tenant_skill_runtime_verify.go` | `internal/modules/agentcatalog/service/tenant_skill_runtime_verify.go` |
| 14 | `internal/application/service/tenant_skill_source.go` | `internal/modules/agentcatalog/service/tenant_skill_source.go` |
| 15 | `internal/application/service/tenant_skill_steer.go` | `internal/modules/agentcatalog/service/tenant_skill_steer.go` |
| 16 | `internal/application/service/tenant_skill_stop.go` | `internal/modules/agentcatalog/service/tenant_skill_stop.go` |
| 17 | `internal/application/service/tenant_skill_transcript.go` | `internal/modules/agentcatalog/service/tenant_skill_transcript.go` |
| 18 | `internal/application/service/tenant_skill_verify.go` | `internal/modules/agentcatalog/service/tenant_skill_verify.go` |
| 19 | `internal/handler/skill_handler.go` | `internal/modules/agentcatalog/handler/skill_handler.go`（新包 `handler`） |
| 20 | `internal/handler/skill_catalog.go` | `internal/modules/agentcatalog/handler/skill_catalog.go` |

随迁 `_test.go`（framework:29，19 个，同名同目录随迁）：service 17 个——`tenant_skill_{admin,bundle,catalog,effective,env_declare,files,install,reaper,remove,runtime_verify,service,source,stale_sandbox,steer,stop,transcript,verify_python}_test.go`（`ls internal/application/service/tenant_skill*_test.go` 实测 17 个，全部归属本面；其中 `env_declare_test.go`、`install_test.go` 现引用 agentruntime 测试辅助——guard 扫描跳过 `_test.go`（tools/architectureguard/check.go:1182），随迁后可保留）；handler 2 个——`skill_catalog_test.go`、`skill_handler_test.go`。

**范围外（一律不动）**：
- `internal/router/router.go`、`internal/router/routes_agent.go`、`internal/container/container.go`、`internal/bootstrap/**`、`go.mod`、`go.sum`、`migrations/`、`tools/**`（conventions §3；architectureguard 豁免表属集成/后续契约任务）。
- `internal/modules/agentcatalog/module.go`（conventions §3；B0 骨架，IB2 按 Brief 装配）。
- 跨 owner 消费方文件（shim 维持其零改动编译，§2.4/§5）：`internal/application/service/{tenant_sandbox_config.go,user_env.go,session.go,session_agent_qa.go,session_sandbox_pin.go,session_attachment_staging.go,session_knowledge_qa.go,expert_skills.go,agent_service.go,skill_market_service.go,tenant_skill_market_service.go}`、`internal/application/repository/tenant_sandbox_config.go`、`internal/handler/{sandbox_skill.go,sandbox_config.go,me_env_var.go,skill_market.go,tenant_skill_market.go}` 及全部 `_test.go`（除随迁的 19 个）。
- 25a/25c 全部文件（§前置条件 5）。
- `internal/types/**`（`TenantSkillEntity`/`TenantSkillCatalogEntity`/`TenantUserEnvVar`/`SkillEnvVars`/`TenantIDContextKey` 等为现役公共类型；contracts.yaml 冻结符号引用 `internal/types/interfaces/*` 不变）。
- `docs/architecture/passb/{ownership-matrix,contracts,event-catalog,exception-ledger}.yaml`（conventions §3：b0 建、barrier 回写）。

**节点自身产出（新文件白名单）**：`internal/modules/agentcatalog/{repository,service,handler}/**`（含随迁测试）、旧路径过渡残差 `internal/application/repository/tenant_skill.go`（重写）、`internal/application/service/tenant_skill_residual.go`（新）、`internal/application/service/tenant_skill_effective.go`（重写为残差包装）、`internal/application/service/tenant_skill_export_parity_test.go`（新，差分装置）、`internal/handler/skill_handler.go` 与 `internal/handler/skill_catalog.go`（重写为残差）、`docs/architecture/evidence/passb/b2-ac-skills.md`、`docs/plans/passb/reports/b2-ac-skills.md`、`docs/plans/passb/reviews/b2-ac-skills.md`（审查者写）、`docs/architecture/passb/briefs/b2-ac-skills.md`。

## 2. 现状断链面（真实代码证据，全部在 8c45a8815 实测）

### 2.1 execution→agentcatalog 4 符号（DAG required_contracts：本面导出化收口）

定义方均为本面文件；调用方均为 execution 属主文件（ownership-matrix：`tenant_sandbox_config.go`/`user_env.go` → plan `13-execution`）。当前同宿主包 `internal/application/service` 所以可编译；本节点搬走定义方后由**残差转发 + 导出名**双轨收口（§4.1/§5.1）：

| 符号 | 定义处（grep 实测） | 调用点（非测试） |
|---|---|---|
| `matchSnapshotByName` | `tenant_skill_reaper.go:672` `func matchSnapshotByName(listed []sandbox.RemoteSnapshotRef, plannedName string) string` | `tenant_skill_reaper.go:572`（同面）；`tenant_sandbox_config.go:1137`（execution） |
| `skillSnapshotNamePrefix` | `tenant_skill_install.go:1633` `func skillSnapshotNamePrefix(tenantID uint64, configID string) string`（格式 `weknora-sk-t%d-%s` + `compactConfigID`） | `tenant_skill_install.go:1673`、`tenant_skill_reaper.go:316,567`（同面）；`tenant_sandbox_config.go:1134`（execution） |
| `snapshotsNotFromOtherConfig` | `tenant_skill_install.go:1703` `func snapshotsNotFromOtherConfig(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef` | `tenant_skill_reaper.go:316,567`（同面）；`tenant_sandbox_config.go:1135`（execution） |
| `validateUserEnvName` | `tenant_skill_env_declare.go:158` `func validateUserEnvName(name string) error` | `user_env.go:275,381`（execution，2 处） |

`sandbox.RemoteSnapshotRef` 定义于 `internal/modules/execution/sandbox/remote_client.go:529`（`ID string; Names []string`，Pass A 已在模块包，新包继续 import）。

### 2.2 conversation→agentcatalog：skillsForRun（DAG pair「def_plan: 25a/25b」落本面）

- 定义：`tenant_skill_effective.go:35` `func skillsForRun(ctx context.Context, pinner *SessionSandboxPinner, configs skillImageConfigReader, skills installedSkillLister, tenantID uint64, sessionID string, agentConfigID string) (string, []*types.TenantSkillEntity)`；内部 :45 调 conversation 属主 `sandboxConfigForExistingSandbox`（`session_sandbox_pin.go:205`，`pinner == nil || TrimSpace(sessionID)==""` 时返回 `"", nil`，:210-213）。
- 调用方：`session_agent_qa.go:357`（conversation，plan 35-conversation-program）。
- `*SessionSandboxPinner.Read(ctx, sessionID)` 存在（session_sandbox_pin.go:213），收敛为使用方窄接口（§4.2）。
- 同文件 `effectiveTenantSkills`（:74）为纯过滤逻辑；其参数接口 `skillImageConfigReader`（:14）/`installedSkillLister`（:19）中 `installedSkillLister` 被 25a 属主 `expert_skills.go:36,43` 消费（`bundledSkillResolver.lister` 字段、`NewBundledSkillResolver` 参数）。

### 2.3 agentcatalog→conversation/execution 出向未导出耦合（本面文件内调用点，grep 实测）

| 调用点 | 被调符号（属主、定义处） | 处理（§4.3/§4.4） |
|---|---|---|
| `tenant_skill_install.go:1123` | `sessionSandboxInstallShellExecutor(mgr sandbox.Manager) sandbox.SessionInstallShellExecutor`（conversation，`session_attachment_staging.go:49`） | `HostAdapters.InstallShellExecutor` 注入 |
| `tenant_skill_install.go:1461` | `resolveTenantSandboxForConfig(ctx, resolver, _ sandbox.Manager, tenantID, configID string, policy WorkspaceSandboxPolicy)`（execution，`tenant_sandbox_resolve.go:70`；不静默回退默认 Manager） | `HostAdapters.ResolveConfigManager` 注入（闭包捕获 sandboxes + policy + nil Manager 位） |
| `tenant_skill_install.go:1473` | `sessionUserIDFromContext(ctx)`（conversation，`session.go:26`，1:1 委托公共 `types.SessionOwnerIDFromContext`） | `HostAdapters.SessionUserID`，缺省实现直接调公共 API |
| `tenant_skill_catalog.go:213` | `uniqueNonEmptyStrings(values []string) []string`（conversation，`session_knowledge_qa.go:638`，去空去重） | 改用本面同语义 `uniqueStrings`（`tenant_skill_bundle.go:502`），奇偶断言进差分测试 |
| `tenant_skill_effective.go:45` | `sandboxConfigForExistingSandbox`（见 §2.2） | `PinnedConfigReader` 窄接口参数化 |

`s.sandboxPolicy`（execution 属主类型 `WorkspaceSandboxPolicy`，`tenant_sandbox_resolve.go:60` 定义）在本面文件的唯一消费点即 :1461——新包构造器去除该参数，由残差闭包吸收。

### 2.4 agentruntime 内部包 import（guard `forbidden-import` 实证，须在搬迁时解除）

tools/architectureguard/check.go:1175-1220：`internal/modules/<owner>/**` 下非测试文件 import 其他模块任何包即 `forbidden-import` 违规，仅 `importExceptions` 精确 file→package 豁免可放行；豁免表属 `tools/**`，不在本节点 owned_files，且新增豁免须走契约任务。因此下列 4 个搬迁文件的 5 条 import 必须改为注入端口（§4.4），新包生产文件零 agentruntime import：

| 文件（搬迁后新包路径） | import（现状实测） | 消费符号（定义处实测） | 调用点 |
|---|---|---|---|
| `tenant_skill_bundle.go` | `agentruntime/agent/skills`（:17） | `skills.ParseSkillFile(content string) (*skills.Skill, error)`（skills/skill.go:161） | bundle.go:309，消费 `skill.Name/.Description/.Instructions/.FrontmatterRepaired` 4 字段（:319-325） |
| 同上 | 同上 | `skills.UnmarshalSkillFrontmatter(frontmatter string, dest any) (repaired bool, err error)`（skills/skill_frontmatter.go:24） | bundle.go:425（版本探测 helper，dest 为本地 `struct{Version string}`） |
| `tenant_skill_env_declare.go` | `agentruntime/agent/skills`（:10） | `skills.InjectedSandboxEnvVars() []string`（skills/manager.go:51） | env_declare.go:61 **init()**（:56-64，把注入名并入包级 `reservedEnvNames`，:44-54 字面 9 名 + `reservedEnvPrefix="WEKNORA_SKILL_"` :69；注入名含不带该前缀的 `SESSION_INPUT_DIR` 等，见 :58-59 注释） |
| `tenant_skill_install.go` | `agentruntime/agent/skills`（:22）+ `agentruntime/agent/tools`（:23） | `tools.ToolShellExec="shell_exec"`/`ToolWriteSkillFile="write_skill_file"`/`ToolEditSkillFile="edit_skill_file"`（tools/definitions.go:52-62） | install.go:1976（自由函数 `installerAgentConfig` :1970 的 AllowedTools） |
| `tenant_skill_transcript.go` | `agentruntime/agent/tools`（:15，别名 agenttools） | `agenttools.StreamContentForToolResult(toolName string, success bool, errMsg string, data map[string]interface{}) string`（tools/persist.go:99）、`agenttools.SanitizeToolResultForClient(toolName string, result *types.ToolResult) map[string]interface{}`（persist.go:82）、`agenttools.SanitizeAgentStepsForStorage(steps []types.AgentStep) []types.AgentStep`（persist.go:113） | transcript.go:316、:331、:406（均在 `installTranscript` 方法内） |

调用链锚点：`skillBundleFromFiles` 自由函数（bundle.go:304，caller `:91`、`tenant_skill_source.go:864`）；`newInstallTranscript`（transcript.go:81，caller `install.go:708` 在 `s.beginInstallTranscript` 内）；`installerAgentConfig`（install.go:1970，caller `:851`）。

### 2.5 禁改文件消费面（残差 shim 必须覆盖，grep 实测）

- `repository.TenantSkillRepository` / `repository.NewTenantSkillRepository`（旧包路径引用）：`container.go:293,398,780,828`（集成工程师独占）；`session.go:141,169`（conversation）；`agent_service.go:671,730,744`（agentruntime，直接 `repository.NewTenantSkillRepository(s.db)` 构造）；`user_env.go:67,72`、`tenant_sandbox_config.go(svc):313-317`（execution 构造器参数）。
- `service.TenantSkillService` 具体类型：`container.go:566,743,779,801,811,827,2406`（含 `startTenantSkillReaper`，集成工程师独占）。
- `service.Skill*` 导出类型：`handler/sandbox_skill.go:44`（`SkillFileEntry`）、`:47`（`SkillFileContent`）、`:50`（`SkillAdminUpdate`）、`:61`（`SkillInstallGuidanceState`）、`:69,72`（`SkillProgress`）；`handler/sandbox_config.go:175`（`*service.SkillSnapshotReleaseFailedError`——该类型定义在 execution 属主 `tenant_sandbox_config.go:193`，**不随迁**，无需 shim）。
- `service.CatalogInstallResult`：`skill_market_service.go:33,216`、`tenant_skill_market_service.go:41`（25c，其自有窄接口引用）。
- `installedSkillLister`（未导出接口名）：`expert_skills.go:36,43`（25a，零改动约束 → 残差必须保留同名别名，§5.1）。
- `handler.SkillHandler`：`routes_agent.go:76`（RegisterSkillRoutes 参数）、`container.go:801-803`（`handler.NewSkillHandler(s, s)`）、`router_api_key_capabilities_test.go`、`routes_skill_market_test.go`。
- 移动的 handler 文件对宿主包 helper 的引用：`sandboxConfigTenantID(c)`（`sandbox_config.go:92`，execution 属主，= `c.GetUint64(types.TenantIDContextKey.String())` 一行公共键取值）。

### 2.6 集成点（contracts.yaml 冻结，本节点零改动）

- 路由：`RegisterSkillRoutes — routes_agent.go:76`（`/skills` Viewer+ 读、`/skills/catalog` 写走 `apiKeyFullAccess` 组 + `Admin()` 守卫，routes_agent.go:76-97 实测）；残差别名保其编译，IB2 决定切参或改走 `module.go RegisterRoutes`。
- 生命周期：`startTenantSkillReaper`（`container.go:2406`，`svc.Start(ctx)` + `cleaner.RegisterWithName("TenantSkillReaper", ...)`）；reaper cron `skillReaperCronSpec = "0 */5 * * * *"`（`tenant_skill_reaper.go:16`）、`Start` 幂等（:771）/`Stop` 等待在飞（:794）。残差类型别名保 `container.go` 零改动；清理注册名 `TenantSkillReaper` 不变。
- Worker：`agentcatalog.workers` items 为空——本面无 worker 注册义务。
- 事件：`tenant_skill_transcript.go` 消费 `conversation.turn.appended`（event-catalog :103-113）并经 `interfaces.StreamManager` 写安装转录流——依赖走 `internal/types/interfaces` 公共契约，搬迁不改。

## 3. 目标包结构与写入所有权

```text
internal/modules/agentcatalog/
  repository/               # 新包 package repository（import 别名 acrepo）
    tenant_skill.go         # TenantSkillRepository 接口 :15-99 + NewTenantSkillRepository :104 全量随迁，签名零变更
  service/                  # 新包 package service（import 别名 acatsvc）
    host_adapters.go        # 新增：HostAdapters + SkillManifestView（§4.3/§4.4）
    tenant_skill_service.go # TenantSkillService 结构 + NewTenantSkillService + keyedMutex/锁族（§1 #2）
    tenant_skill_{admin,bundle,catalog,effective,env_declare,files,install,progress,reaper,remove,runtime_verify,source,steer,stop,transcript,verify}.go  # §1 #3-18
  handler/                  # 新包 package handler（import 别名 acathandler）
    skill_handler.go        # SkillHandler + 形状接口（保持未导出）
    skill_catalog.go        # 目录 CRUD/文件浏览端点
```

写入所有权：上述新包内文件 + §1 白名单残差/证据文件，仅本节点（b2-ac-skills）可写；宿主包其余文件只读；`internal/container/**`、`internal/router/**`、`module.go`、`tools/**` 禁改（conventions §1.2/§3）。

**并行安全（与 25a）**：25a 的 30 个文件与本面 20 个不相交；双方残差落在宿主包的**不同文件**（25a 尚未派发，其残差文件名由其计划自定）；双方都不写 `module.go`/`container.go`/`router`。唯一共享点是宿主包 `internal/application/service` 的包级编译状态——约定：残差文件命名不得占用 `agent_residual.go`/`market_residual.go` 等 25a/25c 语义名；IB2 按 25a→25b→25c 序合并（framework:152），先合分支的残差在后合分支重放时以同名文件冲突显形、由集成工程师按两份 Brief 收口。

## 4. 导出端口（真实签名，不发明；更名导出加粗）

### 4.1 `internal/modules/agentcatalog/service`（package service，别名 acatsvc）

随搬迁导出（原签名 1:1，仅可见性变更；宿主旧名由 §5.1 残差委托）：

```go
func MatchSnapshotByName(listed []sandbox.RemoteSnapshotRef, plannedName string) string          // 原 matchSnapshotByName，reaper:672
func SkillSnapshotNamePrefix(tenantID uint64, configID string) string                            // 原 skillSnapshotNamePrefix，install:1633
func SnapshotsNotFromOtherConfig(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef  // 原 snapshotsNotFromOtherConfig，install:1703
func ValidateUserEnvName(name string) error                                                      // 原 validateUserEnvName，env_declare:158
func SkillsForRun(ctx context.Context, pinned PinnedConfigReader, configs SkillImageConfigReader, skills InstalledSkillLister, tenantID uint64, sessionID, agentConfigID string) (string, []*types.TenantSkillEntity)   // 原 skillsForRun，effective:35（首参改造，§4.2）
func EffectiveTenantSkills(ctx context.Context, configs SkillImageConfigReader, skills InstalledSkillLister, tenantID uint64, configID string) []*types.TenantSkillEntity  // 原 effectiveTenantSkills，effective:74
func RegisterReservedEnvNames(names []string)                                                    // 新增过渡接缝（§4.4）：把注入名并进 reservedEnvNames，幂等，remove_at: ib2
```

随迁的既有导出类型/函数（名字不变、包路径变更）：`TenantSkillService`、`SkillBundle`、`SkillBundleParseOptions`、`ParseSkillBundle`、`ParseSkillBundleWithOptions`、`SkillFileEntry`、`SkillFileContent`、`SkillCatalogView`、`SkillCatalogInstallView`、`CatalogInstallResult`、`SkillAdminUpdate`、`SkillProgress`、`SkillInstallGuidance`、`SkillInstallGuidanceState`；方法面（各文件顶层声明 grep 清单，坐标见 §12）：`InstallSkill/ReinstallSkill/RemoveSkill/StopSkill/InstallSkillFromSource/InstallGuidance/SteerInstall/SubscribeProgress/LastProgress/ListSkills/ListUsableSkills/GetSkill/UpdateSkillAdmin/SetSkillEnabled/SetSkillEnvValues/ListCatalog/RegisterCatalogFromArchive/RegisterCatalogFromSource/InstallCatalogToConfigs/DeleteCatalog/ListCatalogFiles/ReadCatalogFile/ListSkillFiles/ReadSkillFile/ReapStuckRuns/ReconcileSnapshots/PruneSupersededSnapshots/Start/Stop`。

### 4.2 SkillsForRun 首参改造（行为锚点）

`pinner *SessionSandboxPinner` → `pinned PinnedConfigReader`：

```go
type PinnedConfigReader interface{ Read(ctx context.Context, sessionID string) (string, error) }   // *SessionSandboxPinner 结构满足（session_sandbox_pin.go:213）
type SkillImageConfigReader interface{ GetByID(ctx context.Context, tenantID uint64, id string) (*types.TenantSandboxConfigEntity, error) }  // 原 skillImageConfigReader，effective:14
type InstalledSkillLister interface{ ListSkillsByConfig(ctx context.Context, tenantID uint64, configID string) ([]*types.TenantSkillEntity, error) }          // 原 installedSkillLister，effective:19
```

行为锚点（原实现 session_sandbox_pin.go:210-213 语义）：`pinned == nil || strings.TrimSpace(sessionID) == ""` → 视为无 pin，`configID` 取 `agentConfigID`。旧调用形状由残差保持（§5.2）。

### 4.3 HostAdapters（新文件 `internal/modules/agentcatalog/service/host_adapters.go`）——conversation/execution 出向耦合

```go
// HostAdapters 承接旧宿主包内仍留驻的 conversation/execution 未导出能力与
// agentruntime 工具面（§4.4），由旧路径残差构造器装配（IB2 起、35b/13-execution
// 搬迁后收口为对端导出端口）。字段语义与被替换符号 1:1；nil 字段行为见注释。
type HostAdapters struct {
    // ResolveConfigManager 替代 resolveTenantSandboxForConfig（execution，
    // tenant_sandbox_resolve.go:70）：显式 (tenantID, configID) 的 Manager，
    // 不得静默回退默认 Manager；旧调用第 3 参恒为 nil、policy 吸收进闭包。
    ResolveConfigManager func(ctx context.Context, tenantID uint64, configID string) (sandbox.Manager, error)
    // InstallShellExecutor 替代 sessionSandboxInstallShellExecutor（conversation，
    // session_attachment_staging.go:49）：类型断言 SessionInstallCapabilityProvider。
    InstallShellExecutor func(mgr sandbox.Manager) sandbox.SessionInstallShellExecutor
    // SessionUserID 替代 sessionUserIDFromContext（conversation，session.go:26）。
    // nil 缺省为 types.SessionOwnerIDFromContext(ctx)（公共 API，无逻辑复制）。
    SessionUserID func(ctx context.Context) string
    // —— §4.4 agentruntime 解耦位（均必填；残差构造器绑真源）——
    // SkillManifestParser 替代 skills.ParseSkillFile（skill.go:161）+ 4 字段拷贝。
    SkillManifestParser func(content string) (SkillManifestView, error)
    // FrontmatterVersionParser 替代 skills.UnmarshalSkillFrontmatter（skill_frontmatter.go:24）。
    FrontmatterVersionParser func(frontmatter string, dest any) (bool, error)
    // InstallerToolNames 替代 tools.ToolShellExec/ToolWriteSkillFile/ToolEditSkillFile
    // （definitions.go:52-62）；返回 [shellExec, writeSkillFile, editSkillFile]。
    InstallerToolNames func() [3]string
    // StreamContentForToolResult / SanitizeToolResultForClient / SanitizeAgentStepsForStorage
    // 替代 tools/persist.go:99/:82/:113 三个自由函数。
    StreamContentForToolResult  func(toolName string, success bool, errMsg string, data map[string]interface{}) string
    SanitizeToolResultForClient func(toolName string, result *types.ToolResult) map[string]interface{}
    SanitizeAgentStepsForStorage func(steps []types.AgentStep) []types.AgentStep
}

// SkillManifestView 是 bundle.go:319-325 实际消费的 4 字段数据视图（数据形状，非逻辑复制）。
type SkillManifestView struct {
    Name                string
    Description         string
    Instructions        string
    FrontmatterRepaired bool
}
```

`TenantSkillService` 增未导出字段 `adapters HostAdapters`；构造器定稿：

```go
func NewTenantSkillService(
    skillsRepo acrepo.TenantSkillRepository,
    configsRepo repository.TenantSandboxConfigRepository,   // execution 属主接口，仍留驻 internal/application/repository（模块包 import 宿主 repository 有 workbench/agentruntime 既有先例，guard 实测放行）
    resolver interfaces.StorageBackendResolver,
    sandboxes sandbox.TenantSandboxResolver,                // internal/modules/execution/sandbox（Pass A 已在模块包）
    agents interfaces.AgentService,
    customAgents interfaces.CustomAgentService,
    sessions interfaces.SessionService,
    models interfaces.ModelService,
    redisClient *redis.Client,
    streams interfaces.StreamManager,
    messages interfaces.MessageRepository,
    adapters HostAdapters,                                  // 新增尾参；7 个 §4.4 字段由残差绑真源，nil 即 panic-fast（构造器内显式校验非 nil 并 fail-fast）
) *TenantSkillService
```

相对旧 12 参签名：**去除 `sandboxPolicy WorkspaceSandboxPolicy`**（唯一消费点 install:1461 被 `ResolveConfigManager` 闭包吸收），**追加 `adapters HostAdapters`**。`internal/application/service` 宿主包**禁止被新包 import**（13-execution.md:71 同律防环；`WorkspaceSandboxPolicy` 不出现在新包签名）。

调用点改写（搬迁文件内，函数体最小变更；自由函数改收 `adapters` 参数或升为方法，逻辑不变）：
- install:1123 → `executor := s.adapters.installShellExecutor(mgr)`；
- install:1461 → `mgr, err := s.adapters.resolveConfigManager(ctx, tenantID, configID)`；
- install:1473 → `UserID: s.adapters.sessionUserID(ctx)`；
- install:1976（`installerAgentConfig` 增加 `installTools []string` 参数，caller :851 传 `s.adapters.installerToolNames()` 展开）；
- install:708（`newInstallTranscript` 增加 sanitize 三函数参数或收 `adapters`，caller 传 `s.adapters`）；
- catalog:213 → `ids := uniqueStrings(configIDs)`（本面 helper，bundle.go:502）；
- bundle:309 → `skill, err := adapters.skillManifestParser(string(manifest))`（`skillBundleFromFiles` 加 `adapters` 参数，caller `:91`、`source.go:864` 透传）；
- bundle:425 → `adapters.frontmatterVersionParser(frontmatter, &metadata)`。

### 4.4 agentruntime 解耦细则（§2.4 五条 import 的落地）

1. **reservedEnvNames 与 init()**：`init()`（env_declare.go:56-64）随搬迁删除；包级 `reservedEnvNames`（:44-54 字面 9 名）与 `reservedEnvPrefix`（:69）保留为既有状态。注入名经双通道装载、行为与现状等价：
   - `RegisterReservedEnvNames(names []string)`（§4.1 新增导出，map 并集、幂等、init 时期调用）——由残差文件的 `init()` 调 `acatsvc.RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())`（宿主包 import agentruntime 合法，见 §2.4 现状）。凡链接旧包的二进制（含 `user_env.go`/`user_env_test.go` 所在测试二进制）行为与今日逐字节一致；
   - 备用通道：`HostAdapters` 不再设该字段，运行期无其他消费点（:61 是唯一使用处）。
   - 该接缝 `remove_at: ib2`（IB2 后由 33/35 面按契约任务决定保留名单归属）。诚实记录：包级表为 Pass A 现状（:44-64 init 副作用），本设计只把隐式 init 副作用显式化为导出注册接缝，不新增第二份名单。
2. **测试文件豁免**：随迁的 `env_declare_test.go`/`install_test.go` 对 agentruntime 的引用可保留（guard 跳过 `_test.go`，check.go:1182 实测），不改测试语义。
3. **新包纪律**：`internal/modules/agentcatalog/{service,repository,handler}` 生产文件 import 集合 = `internal/types`、`internal/types/interfaces`、`internal/application/repository`（仅 TenantSandboxConfigRepository 等留驻接口）、`internal/modules/execution/sandbox`、`internal/modules/agentcatalog/repository`、公共库（redis/cron/singleflight/gorm/gin）——**零 `internal/modules/agentruntime/**`、零 `internal/application/service`、零 `internal/handler`**（验收 §10.10）。

### 4.5 `internal/modules/agentcatalog/repository`（package repository，别名 acrepo）

`TenantSkillRepository` 接口（tenant_skill.go:15-99）与 `NewTenantSkillRepository(db *gorm.DB) TenantSkillRepository`（:104）1:1 随迁；imports 仅 `context/errors/time + internal/types + gorm`。

### 4.6 `internal/modules/agentcatalog/handler`（package handler，别名 acathandler）

- `SkillHandler` 结构、`NewSkillHandler(usableSkills usableSkillLister, catalog skillCatalogService)`、`SkillInfoResponse`、`ListSkills/ListCatalog/RegisterCatalog/registerCatalogFromSource/InstallCatalog/DeleteCatalog/ListCatalogFiles/GetCatalogFile` 1:1 随迁；构造器参数接口保持**未导出**（spec §15 API 最小化；结构满足即注入）。
- `skillCatalogService` 接口内 `service.SkillCatalogView` 等引用改 `acatsvc.` 前缀。
- `sandboxConfigTenantID(c)` → 包内新 helper `func skillTenantID(c *gin.Context) uint64 { return c.GetUint64(types.TenantIDContextKey.String()) }`（与 sandbox_config.go:92 同一公共键表达式，无业务逻辑；报告注明）。
- `catalogInstallRequest` 等未导出类型随迁。

## 5. 过渡 shim 精确设计（宿主旧路径，全部 no-logic forwarding；remove_at: ib2）

### 5.1 `internal/application/service/tenant_skill_residual.go`（新文件）

```go
package service

import (
    "context"

    "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
    "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills"
    agenttools "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
    "github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
)

// —— Pass B 25b 过渡残差（b2-ac-skills 产出；IB2 按 brief 删除；禁止新增业务逻辑）——

func init() { // 替代随迁走的 env_declare.go init()（§4.4-1），行为等价
    service.RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())
}

// 类型别名：保 container.go / session.go / agent_service.go / user_env.go /
// tenant_sandbox_config.go(svc) / skill_market_service.go / tenant_skill_market_service.go /
// handler/sandbox_skill.go 编译（§2.5 清单）。installedSkillLister 保留旧未导出名，
// expert_skills.go:36,43 零改动（§2.5）。
type (
    TenantSkillService        = service.TenantSkillService
    SkillBundle               = service.SkillBundle
    SkillBundleParseOptions   = service.SkillBundleParseOptions
    SkillFileEntry            = service.SkillFileEntry
    SkillFileContent          = service.SkillFileContent
    SkillCatalogView          = service.SkillCatalogView
    SkillCatalogInstallView   = service.SkillCatalogInstallView
    CatalogInstallResult      = service.CatalogInstallResult
    SkillAdminUpdate          = service.SkillAdminUpdate
    SkillProgress             = service.SkillProgress
    SkillInstallGuidance      = service.SkillInstallGuidance
    SkillInstallGuidanceState = service.SkillInstallGuidanceState
    installedSkillLister      = service.InstalledSkillLister
)

func ParseSkillBundle(archive []byte) (*SkillBundle, error) { return service.ParseSkillBundle(archive) }
func ParseSkillBundleWithOptions(archive []byte, opts SkillBundleParseOptions) (*SkillBundle, error) {
    return service.ParseSkillBundleWithOptions(archive, opts)
}

// NewTenantSkillService：旧 12 参签名原样保留（container.go:566 零改动）；
// sandboxPolicy 吸收进 ResolveConfigManager 闭包；§4.4 七个 agentruntime 能力位在此绑真源。
func NewTenantSkillService(
    skillsRepo repository.TenantSkillRepository,
    configsRepo repository.TenantSandboxConfigRepository,
    resolver interfaces.StorageBackendResolver,
    sandboxes sandbox.TenantSandboxResolver,
    sandboxPolicy WorkspaceSandboxPolicy,
    agents interfaces.AgentService,
    customAgents interfaces.CustomAgentService,
    sessions interfaces.SessionService,
    models interfaces.ModelService,
    redisClient *redis.Client,
    streams interfaces.StreamManager,
    messages interfaces.MessageRepository,
) *TenantSkillService {
    return service.NewTenantSkillService(skillsRepo, configsRepo, resolver, sandboxes,
        agents, customAgents, sessions, models, redisClient, streams, messages,
        service.HostAdapters{
            ResolveConfigManager: func(ctx context.Context, tenantID uint64, configID string) (sandbox.Manager, error) {
                return resolveTenantSandboxForConfig(ctx, sandboxes, nil, tenantID, configID, sandboxPolicy)
            },
            InstallShellExecutor: sessionSandboxInstallShellExecutor,
            SessionUserID:        nil, // nil → 新包缺省 types.SessionOwnerIDFromContext（与 session.go:26 等价）
            SkillManifestParser: func(content string) (service.SkillManifestView, error) {
                skill, err := skills.ParseSkillFile(content)
                if err != nil {
                    return service.SkillManifestView{}, err
                }
                return service.SkillManifestView{Name: skill.Name, Description: skill.Description,
                    Instructions: skill.Instructions, FrontmatterRepaired: skill.FrontmatterRepaired}, nil
            },
            FrontmatterVersionParser: skills.UnmarshalSkillFrontmatter,
            InstallerToolNames:       func() [3]string { return [3]string{agenttools.ToolShellExec, agenttools.ToolWriteSkillFile, agenttools.ToolEditSkillFile} },
            StreamContentForToolResult:  agenttools.StreamContentForToolResult,      // 与工具名常量同包（agent/tools），单一 import
            SanitizeToolResultForClient: agenttools.SanitizeToolResultForClient,
            SanitizeAgentStepsForStorage: agenttools.SanitizeAgentStepsForStorage,
        })
}

// —— execution→agentcatalog 4 符号收口（DAG required_contracts）：旧名 1:1 委托导出名 ——
// 调用方 tenant_sandbox_config.go:1134,1135,1137 / user_env.go:275,381 本节点零改动；IB2 改指 acatsvc 导出名后删除。
func skillSnapshotNamePrefix(tenantID uint64, configID string) string {
    return service.SkillSnapshotNamePrefix(tenantID, configID)
}
func snapshotsNotFromOtherConfig(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef {
    return service.SnapshotsNotFromOtherConfig(listed, prefix)
}
func matchSnapshotByName(listed []sandbox.RemoteSnapshotRef, plannedName string) string {
    return service.MatchSnapshotByName(listed, plannedName)
}
func validateUserEnvName(name string) error { return service.ValidateUserEnvName(name) }
```

（`tools`/`agenttools` 两个 import 在残差文件内；同名同面方法集经 `type TenantSkillService = …` 别名整体可达：`InstallSkill` 等全部方法、`Start/Stop`、`SubscribeProgress` 继续满足 `container.go:743/779/801/811/827/2406` 编译。）

### 5.2 `internal/application/service/tenant_skill_effective.go`（重写为残差包装）

```go
package service

// —— Pass B 25b 过渡残差（IB2 删除）：conversation 调用点 session_agent_qa.go:357 零改动 ——
func skillsForRun(
    ctx context.Context,
    pinner *SessionSandboxPinner,
    configs service.SkillImageConfigReader,
    skills installedSkillLister,
    tenantID uint64,
    sessionID string,
    agentConfigID string,
) (string, []*types.TenantSkillEntity) {
    var pinned service.PinnedConfigReader
    if pinner != nil {
        pinned = pinner
    }
    return service.SkillsForRun(ctx, pinned, configs, skills, tenantID, sessionID, agentConfigID)
}
```

（`skillImageConfigReader` 旧名随迁退场——其实测消费方仅 effective.go 自身与随迁测试，宿主包零残留；`installedSkillLister` 别名已在 §5.1（expert_skills.go 零改动约束）；`effectiveTenantSkills` 主体随迁导出，本文件不保留副本。）

### 5.3 `internal/application/repository/tenant_skill.go`（重写为残差）

```go
package repository

import "gorm.io/gorm"

// —— Pass B 25b 过渡残差（IB2 删除）：保 container.go:293/398/780/828、
// session.go:141/169、agent_service.go:671/730/744、user_env.go:67/72、
// tenant_sandbox_config.go(svc):313-317 编译 ——
type TenantSkillRepository = acrepo.TenantSkillRepository

func NewTenantSkillRepository(db *gorm.DB) TenantSkillRepository {
    return acrepo.NewTenantSkillRepository(db)
}
```

### 5.4 `internal/handler/skill_handler.go` 与 `internal/handler/skill_catalog.go`（重写为残差）

`skill_handler.go`：本地保留同形未导出接口 `usableSkillLister`（:15-17）与 `skillCatalogService`（:25-33，返回类型改 `acatsvc.` 前缀）+ `type SkillHandler = acathandler.SkillHandler` + `func NewSkillHandler(usableSkills usableSkillLister, catalog skillCatalogService) *SkillHandler { return acathandler.NewSkillHandler(usableSkills, catalog) }`；`skill_catalog.go`：8 个方法 1:1 委托 `acathandler.SkillHandler` 同名方法（参数/返回/HTTP 状态零变更）。保 `routes_agent.go:76`、`container.go:801-803`、`router_api_key_capabilities_test.go`、`routes_skill_market_test.go` 零改动编译。

### 5.5 残差纪律

残差仅允许：类型别名、1:1 转发函数/构造器、同形接口再声明、init 时期注册调用（§4.4-1，13-execution.md:69 先例类推）；禁止携带业务逻辑；全部标注 `remove_at: ib2`；每任务末 `go build ./...` 必须绿。

## 6. 任务分解（业务完整、可独立审阅；严格按序执行；一任务一 commit）

> 任务粒度依据：`func (s *TenantSkillService)` 方法接收者分布在全部 17 个 service 文件（grep 实测，§12），Go 方法集要求类型与方法同包——**service 层必须原子搬迁**，不可按文件批次拆分；repository、handler 各为独立可审单元。

### T1（25b.1）— 特征化基线与证据骨架

既有 19 个测试文件即特征化装置（`tenant_skill_install_test.go` 3500 行锚定 install/remove 状态机与 agentruntime 冻结契约；`tenant_skill_reaper_test.go` 1067 行锚定 reaper 幂等/保留窗），不新写行为测试。

- [ ] 复跑并记录基线（§前置条件 4 全部命令 + `go test ./internal/application/service -run 'TestReap|TestPrune|TestReconcile' -count=1 -v` 输出摘录），写入 `docs/architecture/evidence/passb/b2-ac-skills.md` §基线。
- [ ] 建 evidence 骨架 + `docs/plans/passb/reports/b2-ac-skills.md` 骨架。
- [ ] 预期：与 §前置条件 4 数值一致；任何红即停（conventions §5 上报，不开工）。
- commit：`test(passb): passb b2-ac-skills 特征化基线`

### T2（25b.2）— repository 层搬迁 + 残差

- [ ] `git mv internal/application/repository/tenant_skill.go internal/modules/agentcatalog/repository/tenant_skill.go`，package 行改 `repository`，imports 修至编译（仅 types/gorm）。
- [ ] §5.3 残差重写旧路径。
- [ ] GREEN：`go build ./...` 0；`go test ./internal/application/service -count=1` 全绿（session/agent_service/user_env/tenant_sandbox_config 消费方零改动）；`go test ./internal/modules/agentcatalog/... -count=1` ok。
- [ ] `git diff --summary` 确认 rename 识别。
- commit：`refactor(agentcatalog): move tenant skill repository into module`

### T3（25b.3）— service 层原子搬迁（17 文件 + 17 测试）+ HostAdapters + 5 符号导出化 + 全量残差

- [ ] 建 `host_adapters.go`（§4.3/§4.4：HostAdapters + SkillManifestView + 构造器 nil 校验）。
- [ ] `git mv` §1 #2-18 的 17 个文件 + 17 个测试到 `internal/modules/agentcatalog/service`，package 改 `service`。
- [ ] 导出化更名：`matchSnapshotByName`/`skillSnapshotNamePrefix`/`snapshotsNotFromOtherConfig`/`validateUserEnvName`/`skillsForRun`/`effectiveTenantSkills` → §4.1 导出名（包内调用点 reaper:316/567/572、install:1673 同步改引导出名）；`SkillsForRun` 按 §4.2 改造；`effective.go` 参数接口导出（§4.2）。
- [ ] §2.3/§2.4 调用点按 §4.3 末表改写（含 `installerAgentConfig`/`newInstallTranscript`/`skillBundleFromFiles` 增参或升方法）；`uniqueNonEmptyStrings`→`uniqueStrings`；env_declare.go 删 `init()`、增 `RegisterReservedEnvNames`（§4.4-1）。
- [ ] §5.1 残差文件落位（类型别名族 + 构造器残差 + 4 符号转发 + init 注册）；§5.2 effective 残差重写。
- [ ] 新增 `internal/application/service/tenant_skill_export_parity_test.go`（差分装置，表驱动，`remove_at: ib2`）：`skillSnapshotNamePrefix(t,c) == acatsvc.SkillSnapshotNamePrefix(t,c)`（含空 configID/大写/含连字符长 ID 用例）、`snapshotsNotFromOtherConfig`/`matchSnapshotByName` 同表等价、`validateUserEnvName` 合法/字面保留名/`WEKNORA_SKILL_` 前缀/注入名（如 `SESSION_INPUT_DIR`，验证 §4.4-1 init 通道）/非法格式逐例等价、`uniqueStrings == uniqueNonEmptyStrings` 序列等价。
- [ ] GREEN：`go build ./...` 0；`go test ./internal/modules/agentcatalog/... -count=1` ok；`go test ./internal/application/service -count=1` ok（execution/conversation/agentruntime/25a/25c 消费方零回归）；`grep -r "agentruntime" internal/modules/agentcatalog/service --include="*.go" -l` 仅命中 `_test.go`（§4.4-3）。
- commit：`refactor(agentcatalog): move tenant skill service into module with host adapters`

### T4（25b.4）— handler 层搬迁（2 文件 + 2 测试）+ 残差

- [ ] `git mv` `skill_handler.go`/`skill_catalog.go` + 测试到 `internal/modules/agentcatalog/handler`；`sandboxConfigTenantID` → `skillTenantID`（§4.6）；`service.` 前缀改 `acatsvc.`。
- [ ] §5.4 两残差文件落位。
- [ ] GREEN：`go build ./...` 0；`go test ./internal/modules/agentcatalog/... -count=1`、`go test ./internal/handler -run 'Skill' -count=1`、`go test ./internal/router -run 'ApiKey|Skill' -count=1` 全绿（RBAC/路由面零回归）。
- commit：`refactor(agentcatalog): move skill http handlers into module`

### T5（25b.5）— 差分证据收口 + Integration Brief + 实施报告 + 节点门禁

- [ ] 双跑差分：T1 基线输出 vs 终态同命令输出，逐包比对结论写入 evidence §差分（spec §14.2 同一测试集合结果一致；conventions §6 格式：用例清单/双跑输出/比对结论/命令与退出码）。
- [ ] §1.2 公约命令全跑：`PASSB_BASE_SHA=$(git merge-base origin/main HEAD)`；`git diff --stat "$PASSB_BASE_SHA"...HEAD`；`go build ./...`；`git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort` 对照 §1 白名单求差集（非空即失败）。
- [ ] 节点 gates 全跑：`go test ./internal/modules/agentcatalog/... -count=1`、`make check-backend-architecture`、`make verify-module-moves`；计数断言 633/23+23/58 不变。
- [ ] 定稿 `docs/architecture/passb/briefs/b2-ac-skills.md`（§8）与实施报告（每条命令原文+退出码+输出摘录；owned_files 逐条核对；未完成项如实列出）。
- commit：`docs(passb): passb b2-ac-skills 差分证据与集成简报`

## 7. 高风险差分证据要求（conventions §6；spec §14.2/14.3）

本面不在 framework:40 点名六面内，按 §14.3「Worker 状态机、重试和幂等」「HTTP 响应/错误码」执行：

1. **搬迁等价差分（必做）**：19 个既有测试文件搬迁前后同集合双跑（T1 基线 vs T5 终态），逐包 `ok/FAIL` 比对；重点用例组：install/remove 状态机与补偿（install_test/remove_test）、reaper 三任务幂等与保留窗（reaper_test：`ReapStuckRuns`/`PruneSupersededSnapshots`/`ReconcileSnapshots`、`skillSnapshotRetention`、Conflict 重试）、环境变量声明校验（env_declare_test）、steer/转录（steer_test/transcript_test 锚定 `conversation.turn.appended` 消费与 StreamManager 面）。
2. **导出化差分（必做）**：`tenant_skill_export_parity_test.go`（T3）——4 符号 + `uniqueStrings` + 注入保留名通道逐例等价。
3. **消费方面差分（必做）**：`user_env_test.go`/`tenant_sandbox_config_test.go`（execution 面，validateUserEnvName/快照名匹配经残差）、session_agent_qa 相关 service 测试（skillsForRun 经残差）、`router_api_key_capabilities_test.go`（RBAC 面）零回归。
4. **快照命名外部契约锚点**：`weknora-sk-t<tenant>-<config>` 命名与 `compactConfigID` 规格进 parity 用例（Cube/E2B/Docker 跨账号 listing 依赖该前缀，install.go:1699-1705 注释为锚）。
5. 差分失败只修新实现，不改期望（spec §14.3）；证据落 `docs/architecture/evidence/passb/b2-ac-skills.md`；legacy 残差删除前差分必须已通过。

## 8. Integration Brief（交付 IB2 的装配变更申请，T5 定稿；barrier 只从已评审 Brief 切换，framework:103）

(a) **残差全删清单**：`tenant_skill_residual.go`、`tenant_skill_effective.go`（残差版）、`internal/application/repository/tenant_skill.go`（残差版）、`internal/handler/skill_handler.go`/`skill_catalog.go`（残差版）、`tenant_skill_export_parity_test.go`——前置：(b)(c)(d) 切换完成且差分绿。
(b) **调用点改指导出名**：`tenant_sandbox_config.go(svc):1134,1135,1137` → `acatsvc.SkillSnapshotNamePrefix/SnapshotsNotFromOtherConfig/MatchSnapshotByName`（届时若 13-execution 搬迁已落地则改注入端口，按其 Brief）；`user_env.go:275,381` → `acatsvc.ValidateUserEnvName`（`RegisterReservedEnvNames` 的 init 接缝随残差删除，由 execution 面按契约任务决定保留名单归属）；`session_agent_qa.go:357` → `acatsvc.SkillsForRun`（`*SessionSandboxPinner` 满足 `PinnedConfigReader`）；`expert_skills.go:36,43` → `acatsvc.InstalledSkillLister`。
(c) **container.go 接线**：`Provide(repository.NewTenantSkillRepository)` → `acarepo.NewTenantSkillRepository`；`Provide(service.NewTenantSkillService)` → `acatsvc.NewTenantSkillService` + `HostAdapters` 装配（§4.3 七个 agentruntime 位在 IB2 时改绑对端导出真源或维持残差闭包，按 33/35 面 Brief）；SkillHandler provider（container.go:801）→ `acathandler.NewSkillHandler`；`startTenantSkillReaper` 类型引用改 `*acatsvc.TenantSkillService`（cleanup 注册名 `TenantSkillReaper` 与 `agentcatalog.lifecycle` 契约不变）。
(d) **routes_agent.go RegisterSkillRoutes** 参数 `*handler.SkillHandler` → `*acathandler.SkillHandler`（或改走 `module.go RegisterRoutes`，IB2 按 25a→25b→25c 汇总裁定，framework:152）。
(e) **contracts.yaml 回写**（barrier 义务）：`agentcatalog.facade/routes/lifecycle` 符号路径；`agentruntime.agent-engine/.agent-service` consumers 中 `tenant_skill_install_test.go` 新路径。
(f) **DAG 回写确认**：`b2-ac-skills.required_contracts` 第 2 条（4 条 execution→agentcatalog 导出化落地）核验记录；`conversation→agentcatalog skillsForRun` pair 收口记录。
(g) **例外台账**：本节点零新增零删除（§9）；无 exc 提案；architectureguard `importExceptions` 无本面新条目（§4.4-3 注入替代豁免）。

## 9. 必须删除的 legacy/alias/例外（本节点口径）

- **legacy**：§1 表 20 行自本节点起标记「已搬迁（destination + 本分支 SHA）」，宿主旧路径仅存 §5 残差；ownership-matrix `delete_barrier=ib2`，正式台账回写归 IB2（conventions §3）。396 台账中 agentcatalog 55 行的 25a/25b/25c 子面映射以本计划 §1 + 25a/25c 计划为准。
- **alias/接缝（本节点产出、IB2 删除）**：§5.1/5.2/5.3/5.4 全部别名、转发、init 注册；`RegisterReservedEnvNames` 导出接缝；无新增长期别名。
- **例外**：exception-ledger 105 条中无本面行（python 遍历实证 §0）——零删除、零新增、零通配（framework:38）；本面以注入端口替代豁免（§4.4-3）。
- **禁止**：B5 前残留「unused but harmless」兼容包（framework:41）；IB2 后本面不得仍有宿主包内 Skill 符号转发。

## 10. 独立验收标准（全部满足才算完成；审查者逐条核验）

1. `git diff --name-only "$PASSB_BASE_SHA"...HEAD | sort` 与 §1 白名单差集为空（20 迁移 + 19 测试随迁 rename、残差/parity/证据新文件；无白名单外文件）。
2. `go build ./...` 退出码 0。
3. `go test ./internal/modules/agentcatalog/... -count=1` ok（新包 19 个随迁测试全绿，逐文件列出）。
4. `go test ./internal/application/service ./internal/handler ./internal/router -count=1` 全 ok（消费方零回归；与 T1 基线同集合同结果）。
5. `make check-backend-architecture`（633/23+23/58/16 断言）与 `make verify-module-moves`（16 manifests）退出码 0。
6. parity 测试（T3）通过且覆盖 §7.2 全部符号与边界用例。
7. 差分证据（§7）双跑输出与比对结论落盘 evidence 文件；报告含全部命令原文+退出码（conventions §1.2）。
8. `internal/container/**`、`internal/router/**`、`internal/modules/agentcatalog/module.go`、`go.mod`、`go.sum`、`migrations/`、`tools/**`、25a/25c 文件、execution/conversation/agentruntime 属主文件在 diff 中零出现（`git diff --name-only` 核验）。
9. 4 条 execution→agentcatalog 符号在新包为导出名且残差 1:1：`rg -n "func (MatchSnapshotByName|SkillSnapshotNamePrefix|SnapshotsNotFromOtherConfig|ValidateUserEnvName)" internal/modules/agentcatalog/service/` 命中 4；旧名仅在残差文件出现。
10. 新包生产文件零 `internal/modules/agentruntime/**`、零 `internal/application/service`、零 `internal/handler` import（`grep -rn --include="*.go" -v _test` + `go list -deps` 核验；§4.4-3）；architectureguard `forbidden-import` 0（含在 §10.5 的 make 目标内）。
11. env_declare 注入名行为等价：parity 用例覆盖字面保留名、`WEKNORA_SKILL_` 前缀名、`SESSION_INPUT_DIR` 注入名三类拒绝（§7.2）。

## 11. 升级路径（conventions §5）

- 门禁不可能通过（如宿主包隐藏消费点未列入 §2.5）→ 停在当前任务，报告记录未过项/根因/复现命令/建议裁定，DAG `status=blocked` + notes 上报；禁止删测试/扩例外/塞 common/复制实现。
- 需改 execution/conversation/agentruntime 冻结签名或需新增 architectureguard 豁免 → ADR/Spec 修订 + 串行契约任务（framework:26），不在本节点现场改判。
- 上游门面未落地类阻塞（ib1 未收口）→ 引用 DAG 边 + F2 裁定，不自行实现上游门面。

## 12. 计划自检记录（撰写者已执行于 8c45a8815）

- **Spec 覆盖**：§5.4/§5.8 边界（§1 表 + §2.1 归属）、§4.2 端口方向（§4.3/§4.4 使用方端口/注入）、§11/§12/§13（§6 任务/提交、§5 残差删除批次）、§14（§7 差分）、§15（§4.6 API 最小化）、§16（§4.4-1 无新增包级可变 DI 状态：reservedEnvNames 为 Pass A 既有表，注入时点由 init 显式化）、§17.2 对应 IB2/b5 义务（§8/§9）。
- **无占位符**：全部接口签名取自源码实测——tenant_skill_reaper.go:16/88/287/432/672/771/794、tenant_skill_install.go:1970-1977/1633/1703、tenant_skill_env_declare.go:44-69/154-180/56-64、tenant_skill_effective.go:14-83、tenant_skill_bundle.go:304-327/502、tenant_skill_catalog.go:213、tenant_skill_transcript.go:316/331/406、tenant_skill_service.go:59-169、tenant_skill_source.go:864、repository/tenant_skill.go:15-106、skill_handler.go:15-42、routes_agent.go:76-97、container.go:293/566/702/801/2406、tenant_sandbox_resolve.go:60/70、tenant_sandbox_config.go:193/313-317/1134-1137、session.go:26、session_sandbox_pin.go:205-213、session_attachment_staging.go:49、session_knowledge_qa.go:638、user_env.go:67/275/381、execution/sandbox/remote_client.go:529、tenant_resolver.go:69、agentruntime skills/skill.go:161、skills/skill_frontmatter.go:24、skills/manager.go:51、tools/definitions.go:52-62、tools/persist.go:82/99/113、tools/architectureguard/check.go:1175-1220、tools/passbguard/check.go:64——无 TODO/TBD/待定接口。
- **类型一致**：`RemoteSnapshotRef`/`TenantSandboxResolver`/`SessionInstallShellExecutor`/`Manager` 均为 `internal/modules/execution/sandbox` 既有导出；`WorkspaceSandboxPolicy` 不进新包签名；`skills.Skill` 4 消费字段有 `SkillManifestView` 数据视图承接（bundle.go:319-325 实测）；`[3]string` 工具名元组对应 definitions.go:52-62 三个常量。
- **跨任务接口一致**：T2 残差→T3 消费（repository 别名）；T3 HostAdapters 七位↔§5.1 残差绑真源一一对应；T3 导出名↔§5.1 转发/§8(b) 切换清单一致；T4 `acatsvc.` 前缀↔T3 导出面；T5 Brief↔§5 删除清单一致。
- **原子性论证**：service 层单任务搬迁因 `func (s *TenantSkillService)` 方法接收者分布全部 17 文件（`grep -c "func (s \*TenantSkillService)"` 实测非零于 admin/install/catalog/reaper/effective 等各文件），拆批必断方法集编译。
- **基线实跑**：§前置条件 4 全部命令撰写者已跑，输出如实记录（含全量 service 包 154.512s）；`go test ./internal/modules/agentcatalog/...` 当前为骨架包（无测试文件）`ok` 预期，T2 起转为真实门禁。
