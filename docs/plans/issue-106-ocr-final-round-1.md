Review complete: 19 finding(s) across 44 selected item(s).

─── apps/web/src/integrations/PluginsPanel.tsx:198-206 ───
[bug · medium] 授权轮询循环存在 client 切换下的陈旧性缺口，与文件自身宣称的代数防护（OCR R2 F02「迟到回包凭旧代数被丢弃」）不符：

1. authorizeConnection 及其调用的 loadConnection 是**点击时刻渲染帧的闭包**，捕获的是旧 pluginsApi。client 切换（页面不重挂载）后
connectionEpochRef 已自增，但 loadConnection 内部 `const epoch = connectionEpochRef.current`
是**调用时**读取——轮询的后续调用发起时代数已等于新代数，校验通过，旧 principal 的 getMyConnection 回包（旧凭据视角的 authorized/expired 状态）经
setConnections 写入新视图，并在最长约 60s 内每 1.5s 反复覆盖新 principal 刚刷新的徽标。aliveRef 只覆盖卸载，不覆盖 client 切换（对照
PluginsSettingsPanel 的 installationsEpoch 对动作直调的默认代数绑定，本面板恰好缺这块）。

2. 另外 `if (!aliveRef.current) break;` 退出后，`if (!authorized) await loadConnection(...)`
兜底刷新仍会执行——卸载后仍多发一次后台请求，与 F08 注释「卸载后轮询不再发后台请求」不完全一致。

建议在循环内同时校验代数，并在因陈旧/卸载退出时跳过兜底刷新。

-         if (!aliveRef.current) break;
+       const startEpoch = connectionEpochRef.current;
+       let abandoned = false;
+       for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
+         await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
+         if (!aliveRef.current || startEpoch !== connectionEpochRef.current) {
+           abandoned = true;
+           break;
+         }
          const next = await loadConnection(connection.installationId);
          if (next?.authorized) {
            authorized = true;
            break;
          }
          if (popup.closed) break;
        }
-       if (!authorized) await loadConnection(connection.installationId);
+       if (!authorized && !abandoned) await loadConnection(connection.installationId);


─── apps/web/src/integrations/PluginsPanel.tsx:40-44 ───
[maintainability · medium] pluginBadge* 四组样式常量如今在仓库中已有三份同串值拷贝：本文件、PluginsSettingsPanel.tsx（各
pluginBadge*），加上 McpSettingsPanel.tsx 的 mcpBadge*（完全相同的 class 串）。同时「ApiError 判定 +
中文兜底文案」的错误分类逻辑也在两个插件面板重复维护（PluginsSettingsPanel 有 apiErrorMessage 工具函数，本文件在列表加载 catch 里内联 `cause
instanceof Error && cause.name === "ApiError"` 同款判定）。后续任何视觉或错误口径调整需同步三处，极易漂移。建议抽取共享模块（如
apps/web/src/plugins/ui.ts 导出 badge 常量与 apiErrorMessage），两个面板共同复用。



─── apps/web/src/integrations/PluginsPanel.tsx:315-320 ───
[style · medium] badge 为三层嵌套三元表达式，违反检查清单「嵌套三元表达式不允许」。三态枚举到样式的映射用查表更清晰且 O(1) 可读。

-   const badge =
-     connection.state === "authorized"
-       ? { className: pluginBadgeOk, label: "已授权" }
-       : connection.state === "expired"
-         ? { className: pluginBadgeWarn, label: "已过期" }
-         : { className: pluginBadgeMuted, label: "未授权" };
+ const CONNECTION_BADGES = {
+   authorized: { className: pluginBadgeOk, label: "已授权" },
+   expired: { className: pluginBadgeWarn, label: "已过期" },
+   unauthorized: { className: pluginBadgeMuted, label: "未授权" },
+ } as const;
+ // 组件内：
+ const badge = CONNECTION_BADGES[connection.state];


─── apps/web/src/integrations/PluginsPanel.tsx:237-242 ───
[style · low] 主渲染为链式嵌套三元（加载中/空态/列表三层），违反检查清单「嵌套三元不允许」；且存在口径缺口：error 非空且 installations 为空（或
listLoaded 为 false 的首次加载失败）时两个条件分支均不命中，落入列表分支渲染一个空 <ul>——与本页「失败时只留错误横幅」的注释口径及 PluginsSettingsPanel 的
null 分支处理不一致。建议提取 renderBody 辅助函数以早返回消除嵌套，同时让 error 且空列表时返回 null。

-       {!listLoaded && error === null ? (
-         <Status>加载中…</Status>
-       ) : installations.length === 0 && error === null ? (
-         <Status>暂无已安装插件</Status>
-       ) : (
-         <ul className="m-0 grid list-none gap-2 p-0">
+   function renderBody() {
+     if (error !== null && installations.length === 0) return null;
+     if (!listLoaded && error === null) return <Status>加载中…</Status>;
+     if (installations.length === 0) return <Status>暂无已安装插件</Status>;
+     return <ul className="m-0 grid list-none gap-2 p-0">{installations.map(/* ... */)}</ul>;
+   }


─── apps/web/src/integrations/PluginsPanel.tsx:206-206 ───
[performance · low] 卸载后的兜底刷新缺少 aliveRef 检查：轮询循环内已按 OCR R2 F08 用 `if (!aliveRef.current) break;`
终止后台请求，但 break（tab 切换卸载）或耗尽退出后的这次兜底 loadConnection 仍会发出一次多余的 getMyConnection 请求——setState 虽为
no-op，但与 R2 F08「卸载后轮询不再发后台请求」的宣称不符。建议在兜底调用前同样校验 aliveRef。

-       if (!authorized) await loadConnection(connection.installationId);
+       if (!authorized && aliveRef.current) await loadConnection(connection.installationId);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:663-666 ───
[style · low] 此处为两层嵌套三元（外层判空列表、内层判 listError），文件后部 driftView 区块还有 `driftView.report ? (...) :
!driftView.error ? ... : null` 同款嵌套。违反检查清单「嵌套三元表达式不允许」，建议改写为提取变量或提前分支。

-         {installations.length === 0 ? (
-           listError === null ? <Status>暂无已安装插件</Status> : null
-         ) : (
+         {installations.length === 0 && listError === null ? (
+           <Status>暂无已安装插件</Status>
+         ) : installations.length === 0 ? null : (
            <ul className="m-0 grid list-none gap-2 p-0">


─── apps/web/src/settings/PluginsSettingsPanel.tsx:338-340 ───
[style · low] toggleState 函数收尾 `}  }` 两个右花括号挤在同一行（`} finally`
块结束与函数体结束），疑似编辑残留，虽语法有效但影响可读性，建议格式化为常规收尾。

      } finally {
        setActionBusyId(null);
-     }  }
+     }
+   }


─── apps/web/src/settings/SettingsPage.tsx:702-702 ───
[maintainability · low] settingsSectionLabel 中 plugins 的直译兜底出现两处逐字相同的 `locale === 'zh-CN' ? '插件' :
'Plugins'`，可合并为同一条件（置于 `if (integrationTab)` 通用分支之前，行为不变）。另外该兜底只区分 zh-CN 与英文：ja-JP/ko-KR/ru-RU 下
settings 侧边栏会回退显示英文 'Plugins'，而 integrations 侧 messages.ts 的 FALLBACK_STRINGS 已为
`integrations.tabs.plugins` 提供五语词条——两侧 locale 覆盖不同步，建议兜底复用 integrationsT 词条或至少合并条件，待 i18n 主表 key
落位后统一迁回。

-   if (integrationTab === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';
+   if (integrationTab === 'plugins' || key === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';


─── internal/application/service/mcp_service.go:501-508 ───
[security · high] 新守卫边界存在遗漏面：POST /mcp-services/:id/metadata/refresh（routes_infra.go:178，Viewer+
可达）对插件物化行未设同款守卫。该面 handler（mcp_metadata.go mcpMetadata）的门槛是 `!service.AuthConfig.IsOAuth() &&
!mayWriteSharedMCPMetadata(ctx)`——个人 OAuth 插件行（正是 Jira 纵向案例）IsOAuth() 为 true 直接跳过 Admin 门槛，随后
mcpServiceService.RefreshMCPMetadata（internal/application/service/mcp_metadata.go:190）直连实时远端
Initialize+ListTools，经 commitMCPMetadata 在 200
响应中向调用者原样返回完整目录。已对自己账号完成授权的成员（成员授权即本特性的正常主路径）即可借此看到未接受能力与漂移后 schema——与本守卫注释所述泄露（"expose unaccepted
capabilities and post-drift schemas to any Viewer"）完全同型，且该面权限比 Test 面（Admin+，本次也守卫了）更低。建议在
loadServiceForMetadata 或 RefreshMCPMetadata 入口对 `service.PluginInstallationID != nil` 返回
ErrPluginManagedService，并在 handler 的 mcpMetadataAppError 映射中将其映射为 409（GET /:id/metadata 只读持久化快照，修复
refresh 后污染源即断，可一并评估）。

- 	// OCR R1 F07: plugin-materialized rows serve their directory through the
- 	// plugin install APIs (accepted snapshot, filtered). The live ListTools
- 	// call below would expose unaccepted capabilities and post-drift schemas
- 	// to any Viewer — the very leak the write faces and the agent runtime
- 	// (FilterToolsBySnapshot) close. Deterministic 409 via the handler.
+ // internal/application/service/mcp_metadata.go — loadServiceForMetadata
+ // (RefreshMCPMetadata/PersistMCPMetadata 共用入口) 补齐同款守卫：
+ 
+ 	if service == nil || tenant == 0 {
+ 		return nil, "", nil, types.ErrMCPServiceNotFound
+ 	}
+ 	// Plugin-materialized rows serve their directory through the plugin
+ 	// install APIs (accepted snapshot, filtered); a live refresh would
+ 	// expose unaccepted capabilities to the caller (same leak class as
+ 	// GetMCPServiceTools/GetMCPServiceResources, OCR R1 F07).
  	if service.PluginInstallationID != nil {
- 		return nil, ErrPluginManagedService
+ 		return nil, "", nil, ErrPluginManagedService
  	}


─── internal/application/service/plugin_install_service.go:598-603 ───
[bug · medium] 确认/补齐流程游离在 lockUpgradeAccept
串行化之外：SetInstallationState/UninstallInstallation/AcceptUpgrade/CheckDrift/ResolveDrift 五处对同一
installation 的读-改-写都持 per-installation 锁（B+A 裁决 #2 的同族写序），但 ConfirmInstallation 的 step
5→7（安装行创建后即对外可见：物化、绑定 service_id、逐工具 SetPolicy）与 completeCrashedConfirm（回绑
service_id、增量补策略行）均不持锁。具体交错：并发 UninstallInstallation 读到安装行后在补齐/确认的策略写序中途级联删除物化 service——PostgreSQL 上
SetPolicy 触发 FK 违约，管理员看到伪 500（ErrInstallationMaterializeFailed）；生产 SQLite（foreign_keys off）上后续
SetPolicy 落为指向已死 serviceID 的孤儿策略行；更糟的时序下确认流程的 MarkPreviewConsumed 先于卸载的 DeleteInstallation
完成，确认方收到成功响应而安装行已删除。建议：completeCrashedConfirm 入口（inst.ID 已知）与正常确认路径 CreateInstallation
之后（installation.ID 已知）同样取 lockUpgradeAccept，与既有五处保持同一串行化纪律（SetInstallationToolPolicy
的单工具策略写也属同族，可一并评估）。

  func (s *pluginService) completeCrashedConfirm(
  	ctx context.Context,
  	tenantID uint64,
  	preview *types.PluginPreview,
  	inst *types.PluginInstallation,
  ) (*types.PluginInstallationResult, error) {
+ 	// 与 UninstallInstallation/AcceptUpgrade 同款 per-installation 串行化：
+ 	// 补齐路径回绑 service_id、增量写策略行，与并发卸载的级联删除交错
+ 	// 会在 PG 上以 FK 违约伪 500、在 SQLite 上留下死 serviceID 孤儿行。
+ 	defer lockUpgradeAccept(inst.ID)()


─── internal/application/service/plugin_install_service.go:101-103 ───
[performance · medium] 健康安装永远退不出标记循环：注释称 "never again once the state sticks"，但该性质只对
drift_state=detected 成立——端点与快照一致时状态永远停留在 none，markDriftBestEffort 在每个成员会话的每次目录加载（catalog
每次会话装配重建，加载闭包按需触发）都执行一次全新 nonce 排他 MCP 客户端完整握手 + live ListTools（上限 10s）。对多成员高频使用的空间，这使第三方端点的连接数与
ListTools 调用量对每个健康插件永久翻倍，且最坏为成员会话组装串行增加 10s 延迟（标记在 loadPluginDirectory 自身的 live
加载之前串行执行）。建议至少加时间门（如记录上次检查时间，间隔内跳过标记——可复用 drift_detail 或新增 last_checked_at 列），或将标记改为复用
loadPluginDirectory 已经抓取的 live 目录（后置钩子）而非第二次握手——目录加载路径本身已有该数据，仅因 provider 在加载前调用而被丢弃。

- 		if inst.DriftState != types.PluginDriftDetected {
+ 		if inst.DriftState != types.PluginDriftDetected && driftMarkDue(inst) {
  			markDriftBestEffort(ctx, repo, lister, tenantID, inst)
  		}
+ // driftMarkDue：距上次漂移检查不足 minDriftMarkInterval（如 5m）时跳过，
+ // 避免健康安装在每次成员目录加载上永久支付第二次端点握手+ListTools。


─── internal/application/service/plugin_install_service.go:215-219 ───
[maintainability · low] service 孪生 lister 缺少内建超时，与其 container
孪生（NewPluginMCPEndpointLister，plugin_lister.go 已内建 min(caller, 30s) 的
pluginListerListToolsTimeout）发生行为分叉：本函数完全依赖调用方派生 deadline，当前唯一消费点 markDriftBestEffort 的 10s
上限是唯一防线。注释自述两者为 "twin"（container 同构），而 http.Server 无 ReadTimeout/WriteTimeout、无路由超时中间件——任何未来未自带
deadline 的复用点都会让慢滴/挂起端点无限占用 goroutine 与连接（container 侧正是为此在 OCR T01-R2-F8 补上了 30s）。建议镜像孪生的内建上限（对
GetOrCreateClient 握手与 ListTools 一并 min(caller, 30s)），消除对调用方纪律的隐式依赖。

  		defer func() {
  			_ = client.Disconnect()
  			_ = manager.CloseClient(verify.ID)
  		}()
- 		return client.ListTools(ctx)
+ 		// 与 container.NewPluginMCPEndpointLister 同款内建上限：
+ 		// min(caller deadline, 30s)，不依赖调用方自带 deadline。
+ 		listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
+ 		defer cancel()
+ 		return client.ListTools(listCtx)


─── internal/application/service/plugin_install_service.go:1536-1538 ───
[bug · medium] AcceptUpgrade 静默丢弃候选版本的 transport 类型变化:候选指纹 = (pluginID, version, endpoint,
toolsDigest) 不含 transport.type,PluginVersionDiff 也没有 transport 维度,而 7a 的 UpdateInstallationAccepted
只写 version/endpoint/snapshot/digest,7b 只切 svc.URL 与 AuthConfig——svc.TransportType 与安装行
transport_type 始终是安装时的旧值。插件从 sse 升级到 http-streamable(常见迁移)并通过指纹校验后:manager
按旧协议拨新端点导致运行时连接失败;CheckDrift/liveDirectoryFromAcceptedEndpoint(lister(ctx, inst.TransportType,
inst.EndpointURL))同样拨错协议、永远报 ErrDriftEndpointUnreachable;管理员在预览差异面上看不到任何原因,只能卸载重装。建议:在 identity
guard 同级增加 transport 一致性 guard(transport 变化走卸载重装,与 plugin_id 整体替换同款确定性 4xx),或者在 diff 中显式暴露并由 7a/7b
持久化 candidate transport。

+ 	// Step 4.5: transport 类型跨版本不可静默切换——指纹与五维差异都不含它,
+ 	// 而运行时/漂移检查都以持久化的 transport_type 拨号;变化走卸载重装。
+ 	if string(result.Manifest.Transport.Type) != inst.TransportType {
+ 		return nil, fmt.Errorf("%w: manifest now declares transport %q, not the installed %q; uninstall and re-install instead",
+ 			ErrPluginVerifyFailed, result.Manifest.Transport.Type, inst.TransportType)
+ 	}
+ 
  	if svc != nil {
  		newEndpoint := candidateEndpoint
  		svc.URL = &newEndpoint


─── internal/application/service/plugin_install_service.go:636-641 ───
[bug · low] completeCrashedConfirm 对悬挂 service_id 锚(中断卸载残留:级联已删服务行与策略行、DeleteInstallation
失败留下安装行)返回确定性 500:该形态下 ServiceID != "" 会跳过锚自愈,随后 toolApprovalService.ListByService 内部的 GetByID 返回
nil 即报 "mcp service not found",被原样映射为 ErrInstallationMaterializeFailed(5xx)——同内容确认的每次重试都
500,直到管理员恰好执行卸载。这与本文件 resolveInstallationPolicyStore(T18-OCR1-F4)对同一形态的既有纪律矛盾:「服务行已删 = 无策略存储可写 →
确定性状态拒绝(ErrInstallationServiceMissing),绝不是 5xx」。建议在读取策略行前先解析策略存储,把 svc==nil 映射为确定性判决。

+ 	svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
+ 	if err != nil {
+ 		logger.GetLogger(ctx).Errorf(
+ 			"confirm heal: failed to load materialized service for installation %s: %v", inst.ID, err)
+ 		return nil, ErrInstallationPersistFailed
+ 	}
+ 	if svc == nil {
+ 		// 悬挂锚(中断卸载残留):服务行与策略行已同删,无策略存储可写——
+ 		// 确定性状态判决,而非 5xx(T18-OCR1-F4 同款纪律)。
+ 		return nil, fmt.Errorf("%w: %q (uninstall and re-install to recover)", ErrInstallationServiceMissing, inst.PluginID)
+ 	}
  	rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID)
  	if err != nil {
  		logger.GetLogger(ctx).Errorf(
  			"confirm heal: failed to load tool policies for installation %s: %v", inst.ID, err)
  		return nil, ErrInstallationMaterializeFailed
  	}


─── internal/modules/plugins/snapshot.go:60-65 ───
[performance · low] maxLiveTools 只限制了工具「数量」，未限制单个工具 input_schema 的「字节大小」：live 目录无传输级大小上限（仅 30s
超时），生产 lister（container/plugin_lister.go）也未对 tools/list 响应体设字节上限。BuildVerifiedSnapshot 与
DiffLiveAgainstSnapshot 会对每个 live 工具的 schema 执行 decode→re-encode→SHA256（内存瞬时放大约 3
倍），恶意公共端点可在管理预览/漂移检查路径用 1024 个超大 schema 制造瞬时 CPU/内存尖峰（请求路径 DoS 向量）。这与本模块对其它所有不可信维度的有界姿态不一致（manifest
1MiB、live 1024 个、echo 512/64 runes、problems 32 条）。建议在 vet 阶段同步增加 per-tool（或聚合）原始 schema
字节预算，超限按确定性拒绝处理，例如拒绝 input_schema > 1MiB 的 live 工具。

- // The live directory has no transport-level size bound (only the manifest
- 	// download is capped at 1MiB) — refuse to process an oversized directory
- 	// before any O(n) map/digest/snapshot work.
- 	if len(live) > maxLiveTools {
- 		return nil, "", fmt.Errorf("live endpoint returned %d tools, exceeding the maximum of %d", len(live), maxLiveTools)
+ if err := validateToolName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
+ 			if !reportedUnvetted[tool.Name] {
+ 				reportedUnvetted[tool.Name] = true
+ 				problems = append(problems, err.Error())
+ 			}
+ 			unvettedName[i] = true
+ 			continue
- 	}
+ 		}


─── internal/modules/plugins/snapshot.go:81-91 ───
[performance · low] 性能/纵深防御（低）：maxLiveTools 只限制了工具「数量」，未限制单个工具 input_schema 的「字节大小」——live
目录无传输级大小上限（仅 30s 超时），生产 lister（container/plugin_lister.go）也未对 tools/list
响应体设字节上限。BuildVerifiedSnapshot 与 DiffLiveAgainstSnapshot 会对每个 live 工具的 schema 执行
decode→re-encode→SHA256（瞬时内存约为原始大小的 2~3 倍），恶意公共端点可在管理预览/漂移检查路径用最多 1024 个超大 schema 制造瞬时 CPU/内存尖峰（请求路径
DoS 向量）。这与本模块对其它不可信维度的有界姿态不一致（manifest 1MiB、echo 512/64 runes、problems 32 条）。建议在此 vet 阶段增加 per-tool
原始 schema 字节预算（如 1MiB），超限按确定性拒绝处理，并让 DiffLiveAgainstSnapshot/ValidateLiveDirectoryForRebase 复用同一常量。

- 		// Vet the name up front, keyed by position (never by the untrusted
- 		// name itself): everything that reaches an error message below has
- 		// passed the same hygiene the manifest validator enforces.
- 		if err := validateToolName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
+ 		// maxLiveSchemaBytes bounds one live tool's raw input_schema (1 MiB,
+ 		// mirroring maxManifestBytes): the live directory has no transport-level
+ 		// size bound, so without a per-tool cap a hostile endpoint can balloon
+ 		// the canonicalize+digest work on the preview/drift path.
+ 		if err := validateToolName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil ||
+ 			len(tool.InputSchema) > maxLiveSchemaBytes {
  			if !reportedUnvetted[tool.Name] {
  				reportedUnvetted[tool.Name] = true
  				problems = append(problems, err.Error())
  			}
  			unvettedName[i] = true
  			continue
  		}


─── scripts/run-gates.mjs:33-37 ───
[documentation · low] 新增第 7 个 gate(node-gte26-selftest)后,文件头部注释已过时:第 9 行仍写 "2. four gates:
test:shared -> typecheck:shared -> test:web -> typecheck:web"(未列出新首项与 integrity/build 的完整计数),第 14 行
"discovery order: $WEKNORA_NODE_BIN, homebrew node, nvm versions" 也与共享模块扩展后的顺序(平台前缀 + $PATH
扫描)不符。头部是使用者理解 gates 行为的第一入口,建议同步更新,避免误导后续维护者。

- const GATES = [
-   // R1-F17: the shim/discovery plumbing this runner itself depends on must be
-   // self-tested before any gate rides on it (previously the suite only ran
-   // when invoked by hand — node --test itself needs node >= 18, not 26).
-   ["node-gte26-selftest", "pnpm run test:node-gte26"],
+ /* 头部注释同步,例如:
+  *   1. node-gte26-selftest (gate plumbing 自测, R1-F17)
+  *   2. test:shared -> typecheck:shared -> test:web -> typecheck:web
+  *   3. check:integrity
+  *   4. build:web
+  * Node >=26 discovery order 见 scripts/lib/node-gte26.mjs:
+  * $WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, $PATH scan.
+  */


─── scripts/run-with-node-gte26.mjs:81-85 ───
[security · low] win32 分支以 shell:true 分发时,args 会被拼接交由 cmd.exe 解释:当前调用方虽限于仓库内 package.json
脚本(注释也以此为由),但该包装器接口是通用的 `-- <command> [args...]`,没有任何机制约束未来参数——含空格的路径会静默断裂,含 & | < > 等元字符的参数会被 shell
解释,形成潜在命令注入面。且 shell 模式下 cmd.exe 不透传子进程信号,result.signal 恒为 null,spawnExitCode 的信号分支在 win32
实际不可达,诊断语义与非 shell 分支不一致。建议在 shell 分发前对参数做元字符校验(或显式加引号转义),将隐性约定变为显式防护。

+   const useShell = process.platform === "win32";
+   if (useShell && [command, ...args].some((a) => /[\r\n&|<>^"]/.test(a))) {
+     console.error("[with-node] refusing win32 shell dispatch: command/args contain cmd.exe metacharacters");
+     process.exit(2);
+   }
    const result = spawnSync(command, args, {
      stdio: "inherit",
      env,
-     shell: process.platform === "win32",
+     shell: useShell,
    });


─── scripts/run-with-node-gte26.mjs:59-62 ───
[maintainability · low] 重复代码:这段"发现失败"诊断信息与 scripts/run-gates.mjs 第 79-82 行几乎逐字相同(仅
[with-node]/[gates] 前缀不同)。消息中枚举的发现来源(WEKNORA_NODE_BIN、platform prefixes、~/.nvm、$PATH)必须与
scripts/lib/node-gte26.mjs 中 findNodeGte26 的实际发现顺序逐字同步——这正是本次抽取共享模块要消除的 F11
漂移模式(发现顺序改动时注释/消息双双过时)。建议在 node-gte26.mjs 中导出一个带 label 参数的消息构造器(如 noNodeFoundMessage(label,
currentVersion)),两个消费者复用,使提示文本无法与实际发现逻辑脱节。

-       console.error(
-         `[with-node] node >= ${MIN_MAJOR} required (current ${process.version}); ` +
-           "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)",
+ // scripts/lib/node-gte26.mjs 中新增:
+ export function noNodeFoundMessage(label, currentVersion) {
+   return (
+     `[${label}] node >= ${MIN_MAJOR} required (current ${currentVersion}); ` +
+     "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)"
-       );
+   );
+ }
+ 
+ // run-with-node-gte26.mjs / run-gates.mjs 中:
+ console.error(noNodeFoundMessage("with-node", process.version));


LLM retry report summary: 1 of 17 requests affected -- 1 request recovered after retry

Core review (1 request):
- .github/workflows/frontend.yml,package.json,scripts/lib/node-gte26.mjs,scripts/lib/node-gte26.test.mjs,scripts/run-gates.mjs,scripts/run-with-node-gte26.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
