# Integration Brief — b2-ac-definition（25a Agent 定义/版本/人格/专家/子代理/收藏）

> 交付节点：`b2-ac-definition`（分支 `codex/passb-b2-ac-definition`，代码终点 `bd9e0f8e9`，证据终点 `5d543c49d`，本 Brief 为其后续纯文档提交）。
> 受理 barrier：**IB2**（`ownership-matrix.yaml` agentcatalog 行 `integration_owner: ib2`、`delete_barrier: ib2`）。
> 依据：计划 `docs/plans/passb/25a-agent-definition-version.md`（v3）§2.1/§2.3/§4-②/§4-③/§7；`conventions.md` §7（package-private 耦合处理）、§3（集成工程师独占文件）、framework:103（barrier 只从已评审 Brief 切换共享装配）。
> 性质：本文件是**装配变更申请**——①§ 的消费方切换/shim 删除与②§ 的导出/去方法化/搬迁均由集成工程师在 IB2 串行执行；实施者不写 `internal/router/**`、`internal/container/container.go`、`internal/agentcatalog/module.go`（conventions §3）。
> 所有行号除特别注明外为节点分支 HEAD `5d543c49d` 实测（grep/sed 实跑，见报告 `docs/plans/passb/reports/b2-ac-definition.md`）。

## 0. 交付摘要（本节点已落地，非申请项）

- 批次 1 共 7 个生产文件已物理搬入 `internal/agentcatalog/{repository,service,handler}`，内容 1:1（仅 package 行与 import 路径差异）；原路径重写为薄 shim（7 个，§①表 A 逐符号别名/转发，零业务语句，文件头带过渡注释）。
- 测试：3 份既有测试随迁（`git diff -M --summary` 实证 `rename (100%)`：`subagent_test.go`、`agent_share_source_test.go`、`tenant_subagent_test.go`）+ 4 份特征化新测试（repo/service/handler favorite 面，conventions §1.4 授权）。
- 门禁全绿：DAG 四条 gates + 通用门禁，计数 `total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`modulemove: OK (16 manifests verified)`（evidence `docs/architecture/evidence/passb/b2-ac-definition.md` §1.3/§2.1）。
- 批次 2 的 15 个文件**留宿**（推迟非缩水，物理约束同 10-identity §3.2 / 13-execution:65 先例），②§ 逐文件登记。
- swagger 产物与错误回显两项 OCR low findings 已裁定 IB2 期执行（②§2.6）。

## ① IB2 装配变更申请：7 个 shim 的消费方切换与删除顺序

### ①.0 shim 清单与全量消费方（grep -rwn @5d543c49d 实测，含测试）

| # | shim 文件（宿主原路径） | shim 符号 | 消费方（文件:行，全部实测） |
|---|---|---|---|
| S1 | `internal/application/repository/agent_share.go` | `var NewAgentShareRepository`；`var ErrAgentShareNotFound`、`var ErrAgentShareAlreadyExists`（哨兵转发，保 `errors.Is` 同一性） | `internal/container/container.go:304`（Provide）；`internal/application/service/semantic_scope_mutation_test.go:79`；`internal/application/service/agent_share.go`（批次 2 留宿）：`repository.ErrAgentShareAlreadyExists` :204、`repository.ErrAgentShareNotFound` :229/:471/:486/:526/:538 |
| S2 | `internal/application/repository/tenant_subagent.go` | `type TenantSubagentRepository`（type 别名）；`var NewTenantSubagentRepository` | `container.go:294`（Provide）、`:795`（Provider 形参 `store repository.TenantSubagentRepository`）；`service/subagent_service.go:32/:43`（字段/形参，:26 注释）；`service/subagent_delegate.go:60`（:22 注释）；`service/subagent_service_test.go:13`（注释）；`service/subagent_delegate_test.go:23/:41/:82`；`repository/expert_install.go:42`（仅注释，25c 留宿） |
| S3 | `internal/application/repository/tenant_disabled_shared_agent.go` | `var NewTenantDisabledSharedAgentRepository` | `container.go:306` |
| S4 | `internal/application/repository/user_resource_favorite.go` | `var NewUserResourceFavoriteRepository` | `container.go:307` |
| S5 | `internal/application/service/user_resource_favorite.go` | `var NewUserResourceFavoriteService`；`var ErrFavoriteInvalidType`、`var ErrFavoriteEmptyID`（哨兵转发） | `container.go:430`（宿主哨兵消费方已随 handler 随迁归零，grep 实证仅此一点） |
| S6 | `internal/handler/user_resource_favorite.go` | `type UserResourceFavoriteHandler`；`var NewUserResourceFavoriteHandler` | `internal/router/router.go:104`（字段类型）；`internal/router/routes_agent.go:61`（注册签名）；`container.go:800`（Provide） |
| S7 | `internal/handler/subagent.go` | `type SubagentHandler`；`var NewSubagentHandler` | `router.go:103`（字段）、`:399`（注册调用）；`internal/router/routes_subagent.go:26`（注册签名）；`internal/router/routes_subagent_test.go:29`（`&handler.SubagentHandler{}` 零值字面量）；`container.go:796/:798`（Provider 返回/构造） |

shim 间无相互依赖；模块包文件不 import 任何 shim（`module/handler/subagent.go` 的 `service.ErrSubagentNotFound/ErrAgentNotFound` 来自留宿的 `service/subagent_service.go:22` 与 `service/custom_agent.go:21`，与 shim 文件无关）。

### ①.1 建议执行顺序（三波，均由集成工程师在 IB2 执行；宿主→模块 import 方向合法，12-commercial.md:302 已裁定）

**第一波（无推迟件依赖，切换后即可删 shim）**：S4、S5、S6、S3、S7。
- S4：`container.go:307` 改 `agentcatalogrepository.NewUserResourceFavoriteRepository`（或按容器既有 import 别名风格）→ 删 `internal/application/repository/user_resource_favorite.go`。
- S5：`container.go:430` 改模块 service 构造器 → 删 `internal/application/service/user_resource_favorite.go`（两个哨兵转发随之消失；宿主侧 `errors.Is` 链经模块同名 var 实体保持）。
- S6：`router.go:104`、`routes_agent.go:61`、`container.go:800` 改引 `internal/agentcatalog/handler` → 删 `internal/handler/user_resource_favorite.go`。
- S3：`container.go:306` → 删 `internal/application/repository/tenant_disabled_shared_agent.go`。
- S7：`router.go:103`/`:399`、`routes_subagent.go:26`、`routes_subagent_test.go:29`（零值字面量改 `&agentcataloghandler.SubagentHandler{}`）、`container.go:796`/`:798` → 删 `internal/handler/subagent.go`。

**第二波（Provide/测试先切，shim 删除被留宿文件引用阻塞，随对应推迟件搬迁收口）**：
- S2：先切 `container.go:294`/`:795` 与 `subagent_delegate_test.go:23`/`:41`/`:82`；`service/subagent_service.go:32/:43/:60`、`service/subagent_delegate.go:60` 的 `repository.TenantSubagentRepository`/`repository.NewTenantSubagentRepository` 引用二选一——(a) 机械改指模块包（跨 owner 宿主文件写，本 Brief 授权），shim 即可删；(b) 留待 `subagent_service.go`/`subagent_delegate.go` 搬迁（②§2.1 台账 #10 所在文件，guard 裁定组）时同包化自然消失，shim 删除排其后。**推荐 (b)**（少一次跨 owner 写；shim 为纯别名无行为风险）。
- S1：先切 `container.go:304` 与 `semantic_scope_mutation_test.go:79`；`service/agent_share.go`（台账 #3，IB2 搬迁件）的 6 处 `repository.ErrAgentShare*` 引用随其搬迁同包化，shim 删除排其后。若协调者裁定 #3 提前导出 `applyTenantRoleCap` 而非搬迁，则 S1 删除需按 (a) 机械改引模块包。

**第三波（终态核验）**：7 个 shim 全部删除后，`internal/{application,handler}` 宿主包不保留任何 agentcatalog 转发声明（framework:29 终态）；`go build ./...` + `make verify-module-moves` 复跑（此时 manifest 行处置必须已按 ①.2 落定）。

### ①.2 manifest 行处置建议（协调者职权，本节点禁改 `docs/architecture/moves/*.yaml`）

7 条 `legacy_files` 行（S1–S7 对应 path）目前由「原路径 shim 文件」满足 `tools/modulemove/verify.go:327-334` 的逐条 `os.Stat`。shim 删除后这些行失去磁盘文件；manifest schema LOCKED（`docs/architecture/moves/README.md:11`，`KnownFields(true)`）无「已搬迁」标记字段。**建议**：协调者在 IB2 对该 7 行统一裁定承载形式（framework B5 方向「标记 migrated + destination + SHA」的落法属其职权；选项含 schema 变更协调或 verify 白名单）。**排序约束：①.2 裁定先于/同步于第三波删除，否则 `make verify-module-moves` 必挂**。

## ② 推迟台账（13 行）+ 反向义务清单

### ②.1 批次 2 推迟台账（15 文件 / 13 行，行号 @5d543c49d 实测复核）

| # | 推迟文件 | 硬阻塞（符号 @ 定义:行 ← 调用点，实测） | 约束类别 | 解除条件与建议裁定 | 受益 barrier |
|---|---|---|---|---|---|
| 1 | `service/agent_service.go` | `resolveSandboxForExecution`@`session_sandbox_pin.go:146` ← `agent_service.go:538,:565`；`sessionSandboxFileStore`@`session_attachment_staging.go:28` ← :429；`sessionSandboxShellExecutor`@:38 ← :820；`sessionSandboxInstallShellExecutor`@:49 ← :810；`archiveMatchesSHA`@`tenant_skill_bundle.go:337` ← :685 | 调宿主他 owner 未导出符号 → module/service import 宿主 service 与宿主 shim 反向 import 成 Go import 环 | conversation 4 符号提前导出（②§2.3 统一申请，集成工程师执行）；`archiveMatchesSHA` 无需导出——25b 搬迁后与本项目标包同包，**排序约束：agent_service.go 搬迁排在 25b 合并之后（IB2 内串行）** | IB2（25b 后） |
| 2 | `service/agent_service.go`（附加）、`service/agent_browser_preferences.go` | `agentService` 具体类型@`agent_service.go:98` 被留宿文件以方法接收者/类型断言引用：`browserSearchInstructions`@`agent_browser_preferences.go:12`（宿主调用点 `agent_capabilities.go:185`）、`prepareAgentCapabilities`@`agent_capabilities.go:84`、`registerCraftDelegateTool`@`craft_delegate.go:306`、`sessionSandboxInputStore`@`session_attachment_staging.go:78`、`stageSessionAttachments`@:101、`resolveSessionAttachmentURLs`@:197、`registerSubagentDelegateTool`@`subagent_delegate.go:33`、断言 `s.agentService.(*agentService)`@`agent_run_graph.go:363` | 方法接收者/类型断言跨 owner（conventions §7.4；DAG 符号级分析未覆盖，本计划扫描新增） | §7.4 去方法化裁定（集成工程师执行）或推迟至 B3/B4 对应文件搬迁后随 IB3/IB4 收口；两案均登记，由协调者择一；`agent_browser_preferences.go` 与 `agent_service.go` 同批搬迁 | IB2 起，至 IB3/IB4 |
| 3 | `service/agent_share.go` | `applyTenantRoleCap`@`kbshare.go:70`（22-K2 留宿）← :297/:356/:419 ×3 | 同 #1 import 环 | 提前导出走 Brief；或按 conventions §7.3 上报所有权重裁（org 角色封顶语义是否属 knowledge 由协调者裁） | IB2（导出案）/IB3（K2 搬迁案） |
| 4 | `service/custom_agent.go` | `var _ installerAgentSource = (*customAgentService)(nil)`@`tenant_skill_install.go:2232`（25b 留宿）——对 25a 具体类型的断言 | 具体类型被留宿文件引用 | 排序约束：25b 合并后（两文件同入模块 service 包）即可搬，IB2 串行；其 `repository.ErrCustomAgentNotFound`（:162 等 5 点，定义 `repository/custom_agent.go:13`，同属批次 2）随同文件搬迁自然同包 | IB2（25b 后） |
| 4b | `service/agent_version.go` | `ErrAgentNotFound` 裸引用 @:107（`errors.Is(err, ErrAgentNotFound)`）——定义于留宿 `service/custom_agent.go:21`（全包唯一定义） | 同层哨兵 import 环（§2.3 环检测） | 与 #4 同批（25b 合并后，`custom_agent.go` 已同入模块 service 包，裸标识符恢复同包可见）；**随迁测试义务**：`agent_version_test.go` 同批随迁，其 `repoRoot`（`agent_version_test.go:36`，`../../..`）须改 4 层 `../../../..`（实跑验证：host 三层=根，模块 service 三层=`internal`、四层=根）——IB2 执行单 | IB2（25b 后） |
| 5 | `repository/custom_agent.go` | `customAgentModelUsageBindings`@宿主 `model_usage.go:31`（← :94）、`scopeCustomAgentsByModelID`@:80（← :72,:87）、`scopeCustomAgentsBySandboxConfigID`@:101（← :119,:136）——commercial 属主 | 同 #1 import 环 | **消费 IB1 已导出 model_usage 绑定族**：`internal/modules/commercial/repository/model_usage.go` 的 `CustomAgentModelUsageBindings/ScopeCustomAgentsByModelID/ScopeCustomAgentsBySandboxConfigID`（签名冻结于 12-commercial.md §4.1）。**前置门（当前未解除）**：该导出文件尚未落地（`test -f` 实测 `EXPORT-MISSING`，evidence §1.5/§2.5-2）；缺失即 conventions §5 blocked 上报，**不得自行实现或复制**（B1-CM 实施合入为排序前提） | IB2（B1-CM 后） |
| 6 | `repository/agent_version.go` | `isUniqueViolation`@`internal/application/repository/voice_session.go:289`（40-workbench 留宿）← :89 ×1；同文件族另见 `repository/agent_marketplace.go:179`（25c 文件，同 def） | 同 #1 import 环 | conventions §7.1：isUniqueViolation 族现存三份（`voice_session.go:289`（workbench）、`service/resource.go:358`（11-airesource）、`internal/modules/commercial/repository/commercial/planversion.go:323`（12-commercial，Pass A 已入模块树）——grep 实证）由 IB2 收口为单一实现，收口后本文件改引之；备选：推迟至 IB4 workbench 搬迁 | IB2（收口）/IB4（备选） |
| 7 | `handler/agent_version.go` | `sandboxConfigTenantID`@`sandbox_config.go:92`（13-execution 留宿，B1-EX 未实施）← :45/:64/:89 ×3 | 同 #1 import 环 | execution 模块搬迁落地时其 handler 面导出；或提前导出走 Brief | IB3/IB4（B1-EX 后） |
| 8 | `handler/custom_agent.go` | `pickUserDisplayName`@`knowledgebase.go:663`（22-K2 留宿）← :306 ×1；`rbac_lookups.go:66` 在 `*CustomAgentHandler` 上声明 `AgentCreatorLookup`（10-identity 推迟件留宿） | 同 #1 + 方法接收者跨 owner（§7.4） | identity 侧 §7.4 去方法化裁定；K2 搬迁/导出走 Brief | IB3（K2）/IB4（identity） |
| 9 | `handler/expert.go`、`service/expert_service.go`、`service/expert_skills.go` | import `internal/agentruntime/agent/{experts,skills}`（expert.go:15、expert_service.go:11、expert_skills.go:10-11）；`expert_skills.go:36/:43` 另引用 `installedSkillLister`@`tenant_skill_effective.go:19`（25b） | **guard forbidden-import**：入模块树后成 agentcatalog→agentruntime 内部包 import（architectureguard check.go:1210-1215 diagnostic），实施者禁新增例外 | 13-execution.md:65 同型：agentruntime 根门面 re-export 所需符号，或推迟至 B3 R 面搬迁协同收口；`installedSkillLister` 随 25b 同包化自然解除 | IB3 |
| 10 | `service/subagent_service.go` | import `internal/agentruntime/agent/subagents`（:15，`subagents.LoadBuiltinSubagents`） | 同 #9 | 同 #9 | IB3 |
| 11 | `handler/persona.go` | import `internal/agentruntime/agent/persona`（:11） | 同 #9 | 同 #9 | IB3 |
| 12 | `handler/shared_agent_access.go` | import `internal/agentruntime/agent/tools`（:10）、`internal/policy/access`（:11）；消费 `service.ErrAgentShareNotFound/ErrAgentSharePermission/ErrAgentNotFoundForShare`（留宿 `service/agent_share.go:18/:19/:20`） | 同 #9 + #1 | 同 #9；哨兵消费随 #3 的 `agent_share.go` 搬迁转模块导出后改引 | IB3（#3 后） |

### ②.2 反向义务清单（agentcatalog 属主符号被他 owner 消费；本节点因本体留宿未断链，搬迁时由对应 barrier 执行）

1. **`agentRequiresRerankModel`（调度指定本节点导出义务）**：定义 `service/agent_share.go:40` ← `session_agent_qa.go:133` + `agent_run_graph.go:207`（conversation 35 面，另 :165 同文件内部消费）+ agentruntime 消费（DAG `required_contracts`：conversation→agentcatalog 与 agentruntime→agentcatalog 两对）。因 `service/agent_share.go` 本体推迟至 IB2（台账 #3），**导出 + 宿主一行委托 shim（10-identity §3.1 模式）随其 IB2 搬迁执行**——IB2 执行单，勿漏。
2. `skillsForRun`（`tenant_skill_effective.go:35`）← conversation 消费：归 25b 面，随 25b 搬迁导出。
3. execution→agentcatalog 4 符号（`matchSnapshotByName`/`skillSnapshotNamePrefix`/`snapshotsNotFromOtherConfig`/`validateUserEnvName`，DAG pair 4）：定义 `tenant_skill_reaper.go` 等 25b 文件——归 25b 搬迁导出。
4. `shared_agent_access.go` 三函数 `resolveSharedAgentForRequest`（:18）/`filterKnowledgeByAgentScope`（:51）/`filterKnowledgeBasesForSharedAgent`（:64）← `handler/knowledge.go:1607/:1667/:2185`（24-K4）+ `handler/knowledgebase.go:516/:533`（22-K2）共 6 点：随台账 #12 搬迁时导出/同包化。
5. `handler/expert.go` 三 helper `decodeExpertInstantiateBody`（:178）/`expertServiceError`（:260）/`maxExpertAgentNameLen`（:23）← `skill_market.go:219/:228` + `tenant_expert_market.go:110/:186/:195/:202`（25c 文件）共 6 点：随台账 #9 搬迁时导出/同包化。
6. 高风险差分义务（conventions §6）：共享代理 KB 可见性过滤三函数（义务 4）与 rerank 校验（义务 1）的「旧实现特征化 → 搬迁 → 同用例双跑 → 逐用例比对」随对应推迟件在 IB2/IB3 执行窗口执行，证据写入 evidence 差分章节（presence 核对已留证，evidence §2.3）。

### ②.3 conversation 7 符号统一提前导出申请（25a 4 符号 + 25b 3 符号，一次走 Brief）

DAG pair 2（agentcatalog→conversation，10 调用点 7 符号；定义 `internal/application/service/{session_sandbox_pin,session_attachment_staging,session,session_knowledge_qa}.go`，conversation 属主 → 35b 才搬）：

| 归属 | 符号 @ 定义:行（实测） | 调用点 |
|---|---|---|
| **25a（本台账 #1，IB2 需）** | `resolveSandboxForExecution`@`session_sandbox_pin.go:146`、`sessionSandboxFileStore`@`session_attachment_staging.go:28`、`sessionSandboxShellExecutor`@:38、`sessionSandboxInstallShellExecutor`@:49 | `agent_service.go:538/:565`、`:429`、`:820`、`:810`（5 点） |
| **25b（25b 节点需，建议同批）** | `sandboxConfigForExistingSandbox`@`session_sandbox_pin.go:205`、`sessionUserIDFromContext`@`session.go:26`、`uniqueNonEmptyStrings`@`session_knowledge_qa.go:638` | `tenant_skill_effective.go:45`、`tenant_skill_install.go:1473`、`tenant_skill_catalog.go:213`（3 点） |

申请：集成工程师在 conversation 属主文件上统一提前导出（导出名建议保持原名首字母大写，7 符号一次 PR，跨 owner 写由本 Brief 授权，conventions §1.3）。另：`resolveTenantSandboxForConfig`（execution 属主，`tenant_sandbox_resolve.go:70`，DAG pair 24）在 agentcatalog 面唯一调用点为 `tenant_skill_install.go:1461`（25b scope），归 25b 台账，25b 搬迁前需 B1-EX 落地或提前导出——此处登记防漏。

### ②.4 §7.4 方法扩散裁定请求（DAG 符号级分析盲区，请协调者裁定收口方式）

- `*agentService`（`agent_service.go:98`）方法被 6 个他 owner 留宿文件以接收者/断言消费（台账 #2 全列：agent_capabilities、craft_delegate、session_attachment_staging、subagent_delegate、agent_run_graph、agent_browser_preferences 自身）。裁定选项：(a) 去方法化（集成工程师执行）；(b) 推迟至 B3/B4 对应文件搬迁后随 IB3/IB4 收口。
- `*CustomAgentHandler` 上的 `AgentCreatorLookup`（`rbac_lookups.go:66`，10-identity 推迟件）——同 §7.4 类别，10-identity.md:79 已登记同一边，请与 identity 侧统一裁定。

### ②.5 前置门汇总（缺失即 blocked 上报，禁止自行实现上游门面）

1. B1-CM 导出 `internal/modules/commercial/repository/model_usage.go`——**当前 `EXPORT-MISSING`**（evidence §1.5 实测；旁证 `internal/execution/service` 不存在）。阻塞台账 #5。
2. B1-EX `internal/execution/service`（`ResolveTenantSandboxForConfig`/handler 面导出）。阻塞台账 #7 及 25b 面调用点。

### ②.6 已裁定 IB2 期执行的既有 low findings（OCR R1，登记防漏）

1. swagger 定义名漂移：`AddFavoriteRequest` 已随迁模块包，`docs/docs.go`/`swagger.json`/`swagger.yaml` 仍引用 `internal_handler.AddFavoriteRequest`——IB2 删 shim 时（或文档再生成轮次）运行 swag 再生成刷新。
2. favorite handler 兜底分支错误原文外泄（模块 `handler/user_resource_favorite.go` :84-87/:112/:140 随迁既有行为）：IB2 删 shim 时统一加固为固定文案，原始 err 仅落日志。

## ③ 拆分确认请求（计划 §2.1：55 总盘 → 22/20/13）

本会话以 manifest `docs/architecture/moves/agentcatalog.yaml`（55 条 legacy_files）做集合划分脚本实证：**25a=22、25b=20、25c=13，三集并集恰为 55，两两交集为空，与 manifest 差集双向为空**。请协调者在 `25-agentcatalog-program.md` 中确认本拆分，并将矩阵行级拆分回写 `ownership-matrix.yaml`（矩阵写权归协调者/barrier，conventions §3）。

**25a（22，本节点 scope）**：repository `agent_share, agent_version, custom_agent, tenant_disabled_shared_agent, tenant_subagent, user_resource_favorite`（6）；service `agent_browser_preferences, agent_service, agent_share, agent_version, custom_agent, expert_service, expert_skills, subagent_service, user_resource_favorite`（9）；handler `agent_version, custom_agent, expert, persona, shared_agent_access, subagent, user_resource_favorite`（7）。

**25b（20）——17 件 service 逐文件列示**（防前缀误划：manifest `tenant_skill_*` service 共 **18** 件，`tenant_skill_market_service.go` 按其 Publish/Install 市场面归 25c，**不在**下列 17 件内）：`tenant_skill_admin, tenant_skill_bundle, tenant_skill_catalog, tenant_skill_effective, tenant_skill_env_declare, tenant_skill_files, tenant_skill_install, tenant_skill_progress, tenant_skill_reaper, tenant_skill_remove, tenant_skill_runtime_verify, tenant_skill_service, tenant_skill_source, tenant_skill_steer, tenant_skill_stop, tenant_skill_transcript, tenant_skill_verify`（17）+ repository `tenant_skill`（1）+ handler `skill_catalog, skill_handler`（2）。

**25c（13）**：repository `agent_marketplace, expert_install, published_expert, published_skill`（4）；service `agent_marketplace, expert_market_source, skill_market_service, tenant_expert_market_service, tenant_skill_market_service`（5）；handler `agent_marketplace, skill_market, tenant_expert_market, tenant_skill_market`（4）。

## ④ 计数零漂移声明

- 节点代码终点（`bd9e0f8e9`）T4 实测（evidence §2.1，命令原文与退出码在案）：`make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`OK (0 violations)`；`make verify-module-moves` → `modulemove: OK (16 manifests verified)`；`go test -count=1 ./internal/agentcatalog/...` 三包 ok。
- 计数口径为 conventions §8 三方一致（guard 实测 == 台账 == manifests 发现），基线 `633 / 23+23 / 58 / 537` 零漂移。
- 本 Brief 与实施报告为纯文档提交，零代码改动；T5 会话末在最终 HEAD 复跑四条 gates 复核（见报告 §3）。
