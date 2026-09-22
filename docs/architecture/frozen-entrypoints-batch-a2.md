# 冻结入口契约 — Batch A2（execution / airesource / agentcatalog / policy）

> 读者：Pass A worker（A9 knowledge、A10 conversation、A11 agentruntime、A12 workbench、A13 craft、A14 insights）。
> 本文冻结 batch A2 四个 capability 模块在 HEAD（`0bb9b354f`）的导入路径与跨模块消费面。
> 事实来源：`go list ./internal/modules/{execution,airesource,agentcatalog,policy}/...`、
> `tools/architectureguard/check.go`、`internal/container/*.go` 装配与全部跨模块调用点；下列符号均在 HEAD 验证存在。

## 1. 模块根与包地图（HEAD，`go list` 原样）

| 模块 | 包路径（省略前缀 `github.com/Tencent/WeKnora/`） |
|---|---|
| execution | `internal/modules/execution`、`internal/modules/execution/browserskill`、`internal/modules/execution/sandbox` |
| airesource | `internal/modules/airesource`、`internal/modules/airesource/mcp`、`internal/modules/airesource/storageurl`、`internal/modules/airesource/web_search`、`internal/modules/airesource/models/asr`、`internal/modules/airesource/models/chat`、`internal/modules/airesource/models/embedding`、`internal/modules/airesource/models/limiter`、`internal/modules/airesource/models/provider`、`internal/modules/airesource/models/rerank`、`internal/modules/airesource/models/utils`、`internal/modules/airesource/models/utils/ollama`、`internal/modules/airesource/models/vlm` |
| agentcatalog | `internal/modules/agentcatalog`（仅 `module.go` 零逻辑骨架 + `legacy/README.md`，无子包） |
| policy | `internal/modules/policy`、`internal/modules/policy/access`、`internal/modules/policy/embedpolicy`、`internal/modules/policy/ipclass`、`internal/modules/policy/ratelimit`、`internal/modules/policy/storageallowlist` |

四个模块根包（`module.go`）均为零逻辑骨架，仅以注释声明预期 façade（`NewModule` / `RegisterRoutes` / `RegisterWorkers` / `Start` / `Stop`），**当前无任何导出符号**；agentcatalog 亦无外部导入方（A11 为其首个消费者）。

## 2. 跨模块实际消费的公共面（HEAD 验证）

### execution 根包（`internal/modules/execution`）

| 符号 | 用途 |
|---|---|
| `NewRegistrationService(db, TargetProvisioner) *RegistrationService` | 节点注册服务（container/handler 装配） |
| `RegistrationService`、`NodeRegistrationRequest`、`NodeChallengeRequest`、`TargetProvisioner`、`TargetIdentityProvider` | 注册契约类型 |
| `ErrRegistration{Challenge,Invalid,NotFound,Replay,Revoked,Unauthorized,Unavailable}` | 注册错误哨兵 |
| `Target`、`Workspace`、`AuthorizeTarget`、`ErrTargetUntrusted` | 执行目标与租户/actor 授权 |
| `NewBridgeClient(BridgeConfig) *BridgeClient`、`BridgeClient`、`BridgeConfig`、`StartCommand`、`AdmissionContext`、`AdmissionVerifier`、`CommandHash`、`CommandSignature` | execution bridge 启动命令与准入 |
| `CleanupFacts`、`CanPurge` | 执行清理判定（repository 层消费） |
| `AllowModelSettlement`、`CountExecutionStopUnconfirmed` | 用量结算策略与部署策略计数 |

### execution/sandbox（`internal/modules/execution/sandbox`）

| 符号族（load-bearing 成员） | 用途 |
|---|---|
| `NewManager(*Config)`、`Manager`、`NewDisabledManager`、`NewTenantSandboxResolver`、`TenantSandboxResolver(+Deps)`、`ResolvedTenantSandboxConfig`、`TenantSandboxConfigLoader` | 沙箱管理器与租户沙箱解析 |
| `Config`、`DefaultConfig`、`ResolveEffectiveConfig`、`SandboxType(+Cube,Disabled,Docker,E2B)`、`ParseSandboxType`、`IsNamedSandboxBackendType`、`EffectiveTemplateID`、`StandardTemplateName`、`DefaultDockerImage`、`DefaultE2BTemplateTag`、`DefaultSandboxExecUser`、`DefaultTerminalIdleDisconnect`、`NormalizeCubeDNSServers` | 配置与后端类型 |
| `DockerBackendEnabled(+Env,SettingKey)`、`SetDockerBackendEnabled`、`EnsureDockerBackendAllowed`、`ErrDockerBackendDisabled`、`ConfigureDockerResourceProtection`、`ProtectionLookupFunc` | Docker 后端开关与容器资源保护 |
| `ValidateOutboundURLWithPolicy`、`OutboundURLPolicy`、`ErrUnsafeOutboundURL` | 沙箱出站 URL 策略 |
| `SessionSandboxBinding(+Store)`、`NewMemorySessionSandboxBindingStore`、`NewRedisSessionSandboxBindingStore`、`SessionSandboxKey`、`MetadataSessionIDKey`、`HasActiveTurn`、`SessionTurnHolder`、`ErrNoLiveSessionSandbox` | 会话-沙箱绑定与 turn 持有 |
| `Session{CapabilityProvider,Destroyer,ExistenceChecker,FileReader,FileStore,InputRoot,OutputRoot,TerminalManager,TerminalProvider,ShellExecutor,InstallShellExecutor,InstallCapabilityProvider,CreateFailureHandler,WorkspaceFile,WorkspaceRoot}`、`PermissiveSessionExistenceChecker`、`SessionBoundManager`、`SessionBootstrapper(+WithClient)` | 会话侧端口接口（A10/A12/A13 主要消费面） |
| `Remote{SandboxClient,SandboxHandle,CreateRequest,ExecRequest,DirEntry,StatEntry,SnapshotManager,SnapshotRef,Template,TemplateCatalog,TerminalSession,TerminalOptions,TimeoutPolicy,NetworkPolicy}`、`RemoteEntry{Dir,File,Other}`、`RemoteError(+Diagnostics,Kind*)`、`RemoteTimeout{Explicit,OnTimeoutKill}`、`IsRemote{Conflict,InvalidRequest,NotFound}`、`IsTemplate{Ready,BuildFailed}`、`NewRemoteClientForCheck`、`NewGuardedTransport`、`SnapshotManagerFrom` | 远端沙箱驱动契约 |
| `CreateForkSnapshot`、`DeleteForkSnapshot`、`ExecutionRecovery`、`ExecutionObservation`、`ExecutionRef`、`CanImportObservation` | fork 快照与执行恢复 |
| `SkillsImageRoot`、`SkillsManifestPath`、`SkillRequirementsPath`、`SkillCommandPath`、`SkillDirFor`、`SkillImageActive`、`SkillNameFromImagePath`、`SkillOwnerFingerprint`、`IsValidSkillName`、`ValidatedImageSkillDir`、`ValidatedSessionOutputDir` | Skill 镜像/路径约定 |
| `ShellExecOptions`、`ShellOutputSnapshot`、`ExecuteResult`、`ShellQuote`、`WithCommandOutput`、`WithSessionFileOperation`、`ResolveWorkspacePath`、`IdentityOf`、`SandboxIdentity`、`BoundSandboxID`、`ConfigSandboxClient`、`ListConfigSandboxes`、`ReleaseConfigSandboxes` | shell 执行与文件操作辅助 |
| `ErrNotFound`、`ErrTimeout`、`ErrSandboxPaused`、`ErrSandboxConfigIncomplete`、`ErrTerminalUnsupported`、`ErrUnsupportedSandboxType` | 错误哨兵 |

### execution/browserskill（`internal/modules/execution/browserskill`）

| 符号 | 用途 |
|---|---|
| `NewManager(...*Store) *Manager`、`Manager`、`NewStore`、`Store` | 浏览器技能会话管理（A11/A12 消费） |
| `Scope`、`Status`、`AccountStatus`、`RPCError`、`NavigationIncomplete` | 作用域/状态/错误类型 |

### airesource

| 符号 | 包 | 用途 |
|---|---|---|
| `Chat`、`NewChat`、`NewOllamaChat`、`ChatConfig`、`ChatOptions`、`Message`、`MessageContentPart`、`MessageKindCompactionSummary`、`Tool`、`ToolCall`、`FunctionDef`、`FunctionCall`、`ImageURL`、`ChatStream`、`GetModelName`、`ConfigFromModel`、`EffectiveThinkingControl`、`BuildPromptCacheKey`、`PromptPrefixFingerprint`、`FingerprintPromptPrefix`、`CacheRetentionNone`、`UsageFactFromTokenUsage`、`UsageFactInput` | `models/chat` | 模型对话门面（A10/A11 最大消费面；platform `types/interfaces` 直接引用其类型） |
| `LocalImageResolver`（**可变 var**） | `models/chat` | 本地图片解析回调：仅允许应用装配（启动）时赋值，请求路径只读 |
| `Embedder`、`NewEmbedder`、`NewBatchEmbedder`、`EmbedderPooler`、`ConfigFromModel` | `models/embedding` | 向量化门面（A9 主要消费面） |
| `Reranker`、`NewReranker`、`RankResult`、`ConfigFromModel` | `models/rerank` | 重排门面 |
| `ASR`、`NewASR`、`ConfigFromModel` | `models/asr` | 语音识别门面 |
| `VLM`、`NewVLM`、`NewVLMFromLegacyConfig`、`ConfigFromModel` | `models/vlm` | 视觉模型门面 |
| `NewLocalLimiter`、`NewRedisLimiter`、`SetGlobalLimit`、`SetGovernor`、`RuntimeStat(+s)` | `models/limiter` | 模型调用限流 |
| `List`、`ListByModelType`、`ProviderInfo`、`ProviderName`、`ProviderLKEAP`、`ProviderVolcengine`、`ProviderWeKnoraCloud`、`WeKnoraCloudBaseURL` | `models/provider` | Provider 目录常量与查询 |
| `ChunkSlice`、`Sign` | `models/utils` | 切片/签名工具 |
| `OllamaService`、`GetOllamaService` | `models/utils/ollama` | Ollama 服务单例 |
| `MCPManager`、`NewMCPManager`、`MCPClient`、`NewMCPClient`、`ClientConfig`、`CallToolResult`、`ContentItem`、`OAuthManager`、`NewOAuthManager`、`OAuthRequiredError`、`OAuthReauthorizationRequiredError`、`ValidateServiceOutboundURLs` | `mcp` | MCP 客户端/OAuth（A11/A12 消费） |
| `NewFileServiceResolver`、`FileServiceResolver`、`BuildFileServiceForProvider`、`NewRequestRewriter`、`NewStreamRewriter`、`Rewriter`、`StreamRewriter`、`Rewrite`、`ResolveMode`、`WithForcedHandleMode`、`QueryParam`、`HoldbackCutoff`、`LocalStorageBaseDir`、`ErrPublicModeForbidden` | `storageurl` | 存储URL改写/文件服务（A9/A10/A13 消费） |
| `Registry`、`NewRegistry`、`(*Registry).Register`、`NewDuckDuckGoProvider`、`NewGoogleProvider`、`NewBingProvider` 等 `New*Provider` 构造器（共 11 个） | `web_search` | Web 搜索 provider 注册表（container 装配注册 8+ 引擎） |

### policy

| 符号 | 包 | 用途 |
|---|---|---|
| `ResolveKB`、`ResolveKBFile`、`ResolveMessageArtifact`、`ResolveMessageFile`、`MessageKBShareAuthorizer`、`SharedAgentLookup`、`WithSharedAgent`、`KBAccess`、`KBRequest`、`KBPermissions`、`NewKBPermissions`、`NewKBSharePermissions`、`HasKBGrant`、`FileAccess`、`CheckOwnershipOrRole`、`OwnershipRequest`、`OwnershipDecision`、`ErrOwnershipForbidden`、`ErrForbidden`、`ErrUnauthorized`、`ErrNotFound`、`ErrResourceNotFound`、`ErrInvalidAgentSource` | `access` | KB/文件/消息产物/共享 Agent 访问判定（A9/A12/A14 主要消费面） |
| `RequireKBWrite`、`WithKBTaskWrite`、`RequireKBTransfer`、`WithKBTransfer(+Task)`、`KBTransfer{Operation,Clone,Move}`、`CloneDestination`、`TransferTaskID`、`ValidateKBTransferCompatibility`、`RejectMovingKnowledge`、`KBShareLookup`、`MessageFileLookup`、`SharedAgentFileLookup` | `access` | KB 写入/迁移/克隆策略 |
| `Allows`、`FrameAncestors`、`NormalizePattern` | `embedpolicy` | embed 嵌入策略（A14 消费） |
| `Classify`、`IsPublic`、`Public` | `ipclass` | IP 分类（A14 及沙箱 URL 守卫消费） |
| `New`、`Limiter` | `ratelimit` | 通用限流器（A14 消费） |
| `Supported`、`IsAllowed`、`FirstAllowed`、`AllowedList` | `storageallowlist` | 存储后端白名单（A9/A13 消费） |

## 3. 冻结规则（A9–A14 必须遵守）

1. **只经 §1 路径引用**：对这四个模块的 import 必须使用 §1 精确路径；旧路径别名已删除（`0bb9b354f`），不得使用或自创路径。
2. **模块间禁互导**：`tools/architectureguard` 的 `forbidden-import` 检查拒绝跨模块 import；唯一例外是 §4 登记的精确 file→package 对（exact-path，无通配）。新增豁免由 Integrator 裁量，worker 不得自行添加。
3. **签名冻结**：§2 符号只消费、不修改；任何签名/语义变更（含 `chat.LocalImageResolver` 赋值协议）经 Integrator 处理，不在 worker 任务内进行。
4. 未列入 §2 的导出符号同样只读消费；如需新依赖面，先报 Integrator。

## 4. 预存横向耦合（守卫 `importExceptions` 登记的 10 条，Pass B 删除）

| # | 导入方文件 | 被导入包 | Pass B 任务 |
|---|---|---|---|
| 1 | `internal/modules/appconnector/service/appconnector/oc_recovery.go` | `modules/commercial/service/commercial` | B-appconnector |
| 2 | `internal/modules/appconnector/adapter.go` | `modules/commercial` | B-appconnector |
| 3 | `internal/modules/appconnector/service/appconnector/action.go` | `modules/commercial` | B-appconnector |
| 4 | `internal/modules/appconnector/service/appconnector/oc_recovery.go` | `modules/commercial` | B-appconnector |
| 5 | `internal/modules/airesource/models/chat/usage.go` | `modules/commercial` | B-airesource |
| 6 | `internal/modules/channels/im/service.go` | `modules/airesource/mcp` | B-channels |
| 7 | `internal/modules/channels/im/service.go` | `modules/airesource/storageurl` | B-channels |
| 8 | `internal/modules/channels/im/service.go` | `modules/policy/ratelimit` | B-channels |
| 9 | `internal/modules/channels/im/yunzhijia/url.go` | `modules/policy/ipclass` | B-channels |
| 10 | `internal/modules/execution/sandbox/url_guard.go` | `modules/policy/ipclass` | B-execution |

第 1–4 条为 IA1 batch-a1 登记（appconnector→commercial）；第 5–10 条为 batch-a2 搬迁显形的预存横向互引。全部为**临时豁免**：对应 Pass B 任务落地时逐条删除，worker 不应复用或扩大这些耦合。
