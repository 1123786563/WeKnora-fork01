Review complete: 27 finding(s) across 48 selected item(s).

─── apps/web/src/integrations/PluginsPanel.tsx:187-190 ───
[maintainability · medium] OAuth 回调路径 "/api/v1/mcp-oauth/callback" 为硬编码字面量，且是 apps/web 内第 4
处逐字副本（ChatRoutePage.tsx、ConfigurationEditor.tsx、McpSettingsPanel.tsx:838、此处）。后端路由（internal/router
注册的 mcp-oauth callback）一旦调整没有任何编译期保护，四处需人工同步，遗漏任意一处该处 OAuth 授权即静默失效。建议与后端路由对齐的路径提取为共享常量（可放
apps/web/src/plugins/ui.ts 或独立共享模块），四处统一引用。

-       const result = await client.configuration.mcp.oauth.authorizeUrl(connection.serviceId, {
-         redirectURI: window.location.origin + "/api/v1/mcp-oauth/callback",
-         frontendRedirect: window.location.href,
-       });
+ // apps/web/src/plugins/ui.ts
+ export const MCP_OAUTH_CALLBACK_PATH = "/api/v1/mcp-oauth/callback";
+ 
+ // PluginsPanel / McpSettingsPanel / ChatRoutePage / ConfigurationEditor 统一引用：
+ redirectURI: window.location.origin + MCP_OAUTH_CALLBACK_PATH,


─── apps/web/src/integrations/PluginsPanel.tsx:49-51 ───
[maintainability · medium] 授权弹窗+轮询骨架与 McpSettingsPanel.startAuthorize（831-863
行）构成跨文件重复实现：弹窗名/特性串（"weknora_mcp_oauth"/"width=600,height=720"）、1500ms×40 轮询节奏（先例为内联魔数
40/1500，此处独立常量化）、循环骨架完全同款。且两份实现已经开始分叉——本面板多了 aliveRef
卸载终止与兜底刷新，先例没有，后续轮询口径或防御逻辑调整需双处同步、极易漂移（aliveRef
分叉正是复制后各自演化的证据）。建议至少将弹窗参数与轮询常量并入共享模块，进一步可将「authorizeUrl→弹窗→轮询至
authorized」骨架抽为参数化的公共工具（状态源经回调注入：oauth.status 或 getMyConnection）。

- /** 弹窗授权后的状态轮询节奏（McpSettingsPanel startAuthorize 同款常量）。 */
- const AUTH_POLL_INTERVAL_MS = 1500;
- const AUTH_POLL_ATTEMPTS = 40;
+ // apps/web/src/plugins/ui.ts（或独立 mcp-oauth 共享模块）
+ export const MCP_OAUTH_POLL_INTERVAL_MS = 1500;
+ export const MCP_OAUTH_POLL_ATTEMPTS = 40;
+ export const MCP_OAUTH_POPUP_NAME = "weknora_mcp_oauth";
+ export const MCP_OAUTH_POPUP_FEATURES = "width=600,height=720";


─── apps/web/src/integrations/PluginsPanel.tsx:210-210 ───
[bug · low] 卸载后兜底刷新仍发出一次后台请求：循环内 `if (!aliveRef.current) break;` 只终止轮询，但退出后 `if (!authorized)` 无条件执行
loadConnection，面板已随 tab 切换卸载时仍会发出一次 getMyConnection 网络请求（setState 虽 no-op，请求照发），与上方 OCR R2 F08
注释「卸载后轮询不再发后台请求」的意图相悖。兜底刷新应同样受 aliveRef 守卫。

-       if (!authorized) await loadConnection(connection.installationId);
+       if (!authorized && aliveRef.current) await loadConnection(connection.installationId);


─── apps/web/src/integrations/PluginsPanel.tsx:319-324 ───
[style · low] 嵌套三元表达式违反检查单「禁止嵌套三元」：此处徽标三态映射为双层链式三元；正文 241-245 行的列表三态（!listLoaded ? … : length===0 &&
error===null ? … : …）为同款链式。状态枚举固定且封闭（'authorized' | 'expired' | 'unauthorized'），用映射表 +
索引访问即可消除嵌套且天然穷尽；列表三态可提为早返回的 render 辅助函数，后续扩展第四态（如 loading 与 error 并存）时不易出错。

-   const badge =
-     connection.state === "authorized"
-       ? { className: pluginBadgeOk, label: "已授权" }
-       : connection.state === "expired"
-         ? { className: pluginBadgeWarn, label: "已过期" }
-         : { className: pluginBadgeMuted, label: "未授权" };
+ const CONNECTION_BADGES: Record<PluginMyConnection["state"], { className: string; label: string }> = {
+   authorized: { className: pluginBadgeOk, label: "已授权" },
+   expired: { className: pluginBadgeWarn, label: "已过期" },
+   unauthorized: { className: pluginBadgeMuted, label: "未授权" },
+ };
+ 
+ // 组件内：
+ const badge = CONNECTION_BADGES[connection.state];


─── apps/web/src/integrations/PluginsPanel.tsx:255-259 ───
[maintainability · low] 面板正文文案全部直书中文（「可用/不可用/检测到漂移/需个人授权/N
个工具」，以及「已授权/已过期/未授权/撤销/去授权/重试/加载中…/暂无已安装插件」和 actionError 兜底串），而宿主 plugins tab 的标题/描述已消费
integrations.plugins.title/subtitle 的五 locale 词条（messages.ts 中 zh-CN/en-US/ja-JP/ko-KR/ru-RU
均已就位）——非中文 locale 下同一 tab 内将出现中英（中日/韩/俄）混排。注释已声明有意随 T20 统一迁移（先例 PluginsSettingsPanel），属已知债务；建议确保 T20
迁移清单覆盖本面板全部字面量（含三处 actionError 兜底串），避免遗漏导致部分文案永久残留中文。



─── apps/web/src/integrations/PluginsPanel.tsx:141-142 ───
[bug · medium] client 切换失效只清了 connections/requestedRef，installations
未清，叠加渲染分支的问题会残留旧空间数据：列表三态中两个空态守卫都带 `error === null`，当新 client 的 listInstallations 失败（catch 仅
setError）时直接落到第三分支渲染旧 <ul>——新 principal 会长期看到旧空间的插件名/pluginId/版本/工具数/漂移徽标，与本处注释「旧 principal
的…原样残留展示在新 client 视图下」要防御的意图不一致（OCR R1 F42 只修了 connections 一半）。同理 initialInstallations 直出后刷新失败也会残留
SSR 数据。另外面板自管数据，页面级 onReload 不会刷新它，error Status 也没有重试入口，失败后只能靠切 tab 重挂载。建议：client 切换 effect
一并重置列表状态（或渲染分支对 error!==null 时不渲染旧列表），并为列表失败补一个重试按钮。

      requestedRef.current = new Set();
      setConnections({});
+     setInstallations([]);
+     setListLoaded(false);


─── apps/web/src/integrations/PluginsPanel.tsx:202-203 ───
[bug · low] client 切换时在途授权轮询会拖死全面板约 60s：epoch 自增后 loadConnection 恒返回 null，`next?.authorized`
永不为真，循环只能等 popup.closed 或跑满 40×1.5s；期间 pendingId 非空使所有行的按钮因 anyPending 冻结（且旧 principal 的 actionError
一并残留）。client 切换的失效 effect 应同时 setPendingId(null)/setActionError(null)，轮询条件也应检测代数变化提前退出（参照 aliveRef
同款守卫位置）。

-         if (!aliveRef.current) break;
+         const startEpoch = connectionEpochRef.current;
+         for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
+           await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
+           if (!aliveRef.current || startEpoch !== connectionEpochRef.current) break;
-         const next = await loadConnection(connection.installationId);
+           const next = await loadConnection(connection.installationId);


─── apps/web/src/integrations/PluginsPanel.tsx:77-78 ───
[bug · low] aliveRef 的 mounted-flag 模式只在 cleanup 置 false、setup 不复位 true：一旦启用 React
StrictMode（官方推荐启用，且仓库其他模块已把 StrictMode 挂载/卸载安全作为测试断言先例），开发态模拟卸载/重挂载会执行 cleanup 但保留同一 ref，重挂载后
aliveRef.current 恒为 false——授权轮询首循即 break，授权完成状态永远检测不到（退化为一次兜底刷新）。应在 setup 中复位标志。当前 main.tsx 未包
StrictMode 故为潜伏问题，但这是该守卫模式的已知陷阱，顺手修复成本极低。

    const aliveRef = useRef(true);
-   useEffect(() => () => { aliveRef.current = false; }, []);
+   useEffect(() => {
+     aliveRef.current = true;
+     return () => { aliveRef.current = false; };
+   }, []);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:267-269 ───
[bug · low] submit 提交预览成功后未清理 upgradeError，与 toggleState 成功路径的行为不一致：toggleState
的注释自述「陈旧的升级失败提示不在数次无关操作后悬挂」（T15-OCR1-F4），而提交新清单预览正是这样的无关操作——管理员对插件 A 检查升级失败后粘贴新 URL
核验预览，旧横幅「升级预览失败（A）」会跨整个新预览会话持续悬挂在安装列表区。建议在此处一并清理，保持两类横幅清理口径一致。

      setBusy(true);
      setError(null);
      setConfirmError(null);
+     // 与 toggleState 成功路径同款口径：清掉陈旧的升级失败横幅，不在新的预览会话上悬挂。
+     setUpgradeError(null);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:55-55 ───
[style · low] 嵌套三元表达式违反代码规范（Ternary Expressions: Nested ternary expressions are not allowed）。此处为
tone 三元链，下方漂移复审徽标处还有 report 存在性叠加 driftState 的双层三元（driftView.report ? (driftView.report.driftState
=== "detected" ? ... : ...) : null）。建议用映射对象或提前求值变量消解，提高可读性。

-   const badgeClass = tone === "warn" ? pluginBadgeWarn : tone === "info" ? pluginBadgeInfo : pluginBadgeMuted;
+   const badgeClass = { warn: pluginBadgeWarn, info: pluginBadgeInfo, muted: pluginBadgeMuted }[tone];


─── apps/web/src/settings/PluginsSettingsPanel.tsx:99-99 ───
[maintainability · low] DiffToolTable（五列：工具/说明/分类/授权/Scopes）与预览卡中的工具表（同五列 + Schema
列）的列结构与单元格渲染逻辑（readOnly/authorization 徽标、scopes join/「—」兜底、overflow-wrap
处理）高度重复——两处任一列口径调整都需同步修改，易漂移。建议将本组件扩展为共享的工具目录表（如增加可选 schema 列或 renderExtraColumn 回调），预览卡复用之。

- function DiffToolTable({ caption, rows }: { caption: string; rows: readonly PluginUpgradeToolSnapshot[] }) {
+ function DiffToolTable({ caption, rows, extraColumn }: {
+   caption: string;
+   rows: readonly PluginUpgradeToolSnapshot[];
+   extraColumn?: { title: string; render: (tool: PluginUpgradeToolSnapshot) => React.ReactNode };
+ }) {


─── apps/web/src/settings/PluginsSettingsPanel.tsx:336-338 ───
[style · low] toggleState 函数结尾的 finally 块闭合括号与函数闭合括号写在同一行且中间夹两个空格（`}  }`），与文件内其余 async
函数（confirmInstall/checkUpgrade 等）的换行收尾风格不一致，疑似合并残留。建议按其余函数同款格式分行闭合。

      } finally {
        setActionBusyId(null);
-     }  }
+     }
+   }


─── examples/plugins/jira-todo-mcp/main.go:189-191 ───
[bug · low] validateServiceBaseURL 放行尾斜杠形态（u.Path == "/" 通过校验）。runMain 的环境变量路径已由 TrimRight("/")
保护，但编程构造 Options{BaseURL: "https://host/"} 直接调 NewHandler
时（测试/复用方入口），后续所有拼接产物会带双斜杠：Transport.Endpoint = "https://host//mcp"、WWW-Authenticate 与 RFC 8414 元数据指向
"https://host//.well-known/..."。Go 的 ServeMux 不会把 "//x" 归一到 "/x"，OAuth 发现链路将
404、插件端点不可用，服务却静默启动——与本函数注释「path/query/fragment 产物全部不可用却静默启动」要防的失败模式完全同型，也与本文件其余 fail-closed
校验（拒绝非具体 host、userinfo、sub-path）的纪律不一致。建议直接拒绝 u.Path == "/"，或在 NewHandler 入口统一 TrimRight。

- 	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
+ 	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
  		return fmt.Errorf("%s must be a bare origin (no path/query/fragment), got %q", where, raw)
  	}


─── internal/application/service/plugin_install_service.go:2215-2221 ───
[bug · high] SetInstallationToolPolicy 是唯一缺少 per-installation 串行锁的策略写路径：同文件中
ConfirmInstallation(:569)、completeCrashedConfirm(:686)、SetInstallationState(:905)、UninstallInstallat
ion(:1007)、AcceptUpgrade(:1441)、CheckDrift(:1830)、ResolveDrift(:1994) 均以 lockUpgradeAccept
串行化快照与策略行写入，而本方法在 GetInstallation 快照成员校验（读）与
toolApprovalService.SetPolicy（Upsert，仅校验服务存在，不校验快照成员）之间不持锁。经 PUT
/installations/:id/tools/:tool_name/policy 暴露，与并发升级/漂移重定基/卸载交错时：(a)
读旧快照判定工具在册后，升级/重定基已移除该工具，补丁仍为其落策略行——该残留行会使后续候选重加同名写工具时被 7c/5b 增量循环跳过，绕过 B5「新增写工具默认
Enabled=ReadOnly」，正是 DeleteInstallationToolPolicies 补偿注释明确防范的写工具 fail-open 暴露链；(b)
与卸载级联交错可向已删服务写孤儿策略行（生产 SQLite foreign_keys 关闭，无 FK 兜底）。建议在方法入口与其他写路径同款持锁。

  func (s *pluginService) SetInstallationToolPolicy(
  	ctx context.Context,
  	tenantID uint64,
  	installationID, toolName string,
  	enabled, requireApproval *bool,
  ) ([]interfaces.PluginInstallationToolPolicy, error) {
+ 	// 与 AcceptUpgrade/ResolveDrift/UninstallInstallation 同款 per-installation
+ 	// 串行化：快照成员校验（读）与 SetPolicy（写）之间不得与升级/重定基/
+ 	// 卸载交错，否则可为快照外工具落残留策略行（B5 fail-open 链）。
+ 	defer lockUpgradeAccept(installationID)()
  	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)


─── internal/application/service/plugin_install_service.go:1636-1642 ───
[bug · low] AcceptUpgrade 的补偿未还原旧漂移判决，与 ResolveDrift 补偿（T17-OCR1-F1，本文件 :2119-2136 显式 UpdateDrift
还原）不对称。UpdateInstallationAccepted 无条件把 drift_state 重置为 'none'、drift_detail 清空；当 drift=detected
的安装接受升级、7b/7c 中途失败补偿回滚到旧快照/旧端点后，旧端点对旧快照的偏差依旧存在，但判决与审计 detail 已丢失——GetDrift 报 none 而运行时目录加载仍按
ErrPluginDrift fail-close，正是 ResolveDrift 补偿注释（:2124-2129）描述的「user-visible
contradiction」同型。虽然运行时标记器/CheckDrift 可重建 detected，但在重建前管理面状态失真。建议补偿成功恢复安装行后同样经 UpdateDrift 还原捕获的
oldDriftState/oldDriftDetail（需在 7a 前捕获，并解析 driftWriter 能力，与 ResolveDrift 一致）。

  		if err := writer.UpdateInstallationAccepted(compCtx, tenantID, installationID,
  			oldVersion, oldEndpoint, oldSnapshot, oldDigest); err != nil {
  			logger.GetLogger(ctx).Errorf(
  				"plugin upgrade accept compensation: failed to restore installation %s to %s: %v",
  				installationID, oldVersion, err)
+ 		} else {
+ 			// 与 ResolveDrift 补偿同款（T17-OCR1-F1）：UpdateInstallationAccepted
+ 			// 无条件重置 drift——补偿回滚旧快照后旧端点的偏差仍在，须还原判决。
+ 			if err := driftWriter.UpdateDrift(compCtx, tenantID, installationID,
+ 				oldDriftState, oldDriftDetail); err != nil {
+ 				logger.GetLogger(ctx).Errorf(
+ 					"plugin upgrade accept compensation: failed to restore drift verdict %q for installation %s: %v",
+ 					oldDriftState, installationID, err)
+ 			}
  		}
  		return cause


─── internal/application/service/plugin_install_service.go:1237-1240 ───
[performance · low] 空 service_id 的自愈解析只改内存不回写：中断窗口存续期间（直到某次 confirm-heal/uninstall
处理），每次成员连接视图调用都重复执行 serviceIDByInstallation 的租户级 List 全量反查并重复刷一条 Info 日志（completeCrashedConfirm 的
anchor heal 有 UpdateInstallationServiceID 回写先例）。行为语义正确（无孤儿时确为死端
fail-closed），但反查与日志会随成员访问频率放大。建议解析成功时回写 service_id 终结窗口（best-effort，失败仅日志），或将重复 Info 降噪。

  		if resolved != "" {
  			logger.GetLogger(ctx).Infof("connection status healing empty service_id anchor: installation %s resolves to orphan service %s", installationID, resolved)
  			inst.ServiceID = resolved
+ 			// 回写安装行终结中断窗口（completeCrashedConfirm 的 anchor heal 同款），
+ 			// 避免每次连接视图重复反查与刷日志；失败仅日志，不影响本次视图。
+ 			if err := s.pluginRepo.UpdateInstallationServiceID(ctx, tenantID, installationID, resolved); err != nil {
+ 				logger.GetLogger(ctx).Errorf("failed to persist healed service binding for installation %s: %v", installationID, err)
+ 			}
  		}


─── internal/application/service/plugin_install_service.go:727-732 ───
[bug · low] completeCrashedConfirm 对「悬空 service_id 锚」（中断卸载形态：级联已删物化服务行+策略行、安装行留存，即本文件 T18-OCR1-F4
明确定义的形态）缺少服务行存在性判定：当 inst.ServiceID 非空（或经反查回填）而服务行已不存在时，ListByService 因 mcpToolApprovalService 内部的
GetByID 判空而返回 "mcp service not found"，本路径将其包装为
ErrInstallationMaterializeFailed（5xx）。同文件对同一形态的处理均不同：AcceptUpgrade 的 7c 与 ResolveDrift 的 5b 以 svc !=
nil 守卫跳过，resolveInstallationPolicyStore 以确定性「无显式行」读法降级，SetInstallationToolPolicy 以
ErrInstallationServiceMissing 给确定性 4xx。结果是该中断窗口内同内容 confirm 重试会持续返回 500 直到预览过期，把确定性状态误报为服务端故障。建议先
GetByID 加载 svc，svc == nil 时返回 alreadyInstalled()（或与 ErrInstallationServiceMissing
同族的确定性判决），与文件自身纪律对齐。

+ 	svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
+ 	if err != nil {
+ 		logger.GetLogger(ctx).Errorf(
+ 			"confirm heal: failed to load materialized service for installation %s: %v", inst.ID, err)
+ 		return nil, ErrInstallationPersistFailed
+ 	}
+ 	if svc == nil {
+ 		// Dangling anchor (interrupted uninstall): no policy store to
+ 		// complete against — deterministic verdict, same reading as
+ 		// resolveInstallationPolicyStore / AcceptUpgrade 7c.
+ 		return nil, alreadyInstalled()
+ 	}
  	rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID)
  	if err != nil {
  		logger.GetLogger(ctx).Errorf(
  			"confirm heal: failed to load tool policies for installation %s: %v", inst.ID, err)
  		return nil, ErrInstallationMaterializeFailed
  	}


─── internal/application/service/plugin_install_service.go:2305-2309 ───
[maintainability · low] 空 service_id 自愈只改局部变量不回写：与 completeCrashedConfirm 的 anchor heal（成功后调用
UpdateInstallationServiceID 回写并更新 inst.ServiceID）不同，本方法在窗口存续期间每次被 ListInstallationTools /
SetInstallationToolPolicy（经 installationToolPolicies）调用都会重复执行 serviceIDByInstallation 的租户级
mcpServiceRepo.List 全量反查并重复刷一条 Info 日志，窗口不会自行收敛。这是已确认发现 #3（GetMyConnectionStatus
同款不回写）的姊妹调用点，建议修复时一并覆盖：解析成功后 best-effort 回写 service_id（失败仅日志），并同步 inst.ServiceID 供调用方后续使用。

  		if resolved != "" {
  			logger.GetLogger(ctx).Infof(
  				"tool policy store healing empty service_id anchor: installation %s resolves to orphan service %s", inst.ID, resolved)
  			serviceID = resolved
+ 			// best-effort 回写，终结中断窗口（completeCrashedConfirm 的 anchor heal 先例）
+ 			if err := s.pluginRepo.UpdateInstallationServiceID(ctx, tenantID, inst.ID, resolved); err != nil {
+ 				logger.GetLogger(ctx).Warnf(
+ 					"tool policy store: failed to persist healed service_id for installation %s: %v", inst.ID, err)
+ 			} else {
+ 				inst.ServiceID = resolved
+ 			}
  		}


─── internal/modules/plugins/plugintest/server.go:60-62 ───
[maintainability · low] Tool.Call 签名丢弃了 SDK 工具 handler 已持有的 ctx：addToolLocked 的闭包有 (ctx, req) 但只透传
member，searchFakeJiraWeek 随后以 context.Background() 发起出站 HTTP——MCP 调用被客户端取消时，替身对 fake Jira 的出站仍会持续到
client Timeout 为止。替身的定位是复刻示例服务的行为契约（examples 的 handleSearchMyWeek 用请求 ctx + maxToolCallTimeout 整体
deadline，取消即确定性切断出站），取消传播上的偏差会让未来针对「调用中断/超时切断」语义的集成测试在替身上失真。当前有 WithJiraToolTimeout 兜底、影响有限，建议扩展签名为
Call func(ctx context.Context, member string) (string, error) 并在 searchFakeJiraWeek 中把 ctx 传入
http.NewRequestWithContext。

  	// Call 返回该工具的结果文本；member 是 Bearer 归属的成员名（未启用
  	// OAuth 或未认证调用时为 ""）。返回 error 时以 MCP 工具错误面呈现。
- 	Call func(member string) (string, error)
+ 	// ctx 是 MCP 调用的取消信号，出站请求应继承它（与示例服务的
+ 	// handleSearchMyWeek 语义一致）。
+ 	Call func(ctx context.Context, member string) (string, error)


─── packages/api-client/src/plugins.ts:193-195 ───
[maintainability · low] 重复样板：全文件 13 处方法重复「trim → 空串判断 →
固定报错」三行（previewId/installationId/manifestUrl/candidateFingerprint/toolName 各自复制），且
parsePluginInstallations 与 parsePluginToolPolicyRows 的「信封解包 + success 断言 + data 数组断言 +
行映射」结构几乎相同。建议提取 requireNonEmpty(value, label) 之类的小 helper（可顺带统一报错文案），数组信封解包也可收敛为一个
parseArrayEnvelope(path, value, mapRow)，降低后续新增端点时的复制漂移风险。

-     async confirmInstallation(previewId: string, signal?: AbortSignal): Promise<PluginInstallation> {
-       const id = previewId.trim();
-       if (id === '') throw new Error('previewId must not be empty');
+ function requireNonEmpty(value: string, label: string): string {
+   const trimmed = value.trim();
+   if (trimmed === '') throw new Error(`${label} must not be empty`);
+   return trimmed;
+ }
+ 
+ // 各方法内：
+ const id = requireNonEmpty(previewId, 'previewId');


─── packages/api-client/src/plugins.ts:760-764 ───
[maintainability · low] 错误路径前缀不一致：治理面工具列表的真实端点是 .../installations/:id/tools，此处却复用 INSTALLATIONS_PATH
作为报错前缀——报错会显示为 /api/v1/plugins/installations.data[0].name，与 summary
列表的报错无法区分，排障时定位有歧义。CONNECTION_PATH/DRIFT_PATH 已按 `:id` 占位约定独立建常量并注释说明动机，治理面应对齐同一做法。

-   const envelope = record(value, INSTALLATIONS_PATH);
-   if (envelope.success !== true) throw new Error(`${INSTALLATIONS_PATH}.success must be true`);
+ const TOOLS_PATH = `${INSTALLATIONS_PATH}/:id/tools`;
+ 
+ export function parsePluginToolPolicyRows(value: unknown): PluginToolPolicyRow[] {
+   const envelope = record(value, TOOLS_PATH);
+   if (envelope.success !== true) throw new Error(`${TOOLS_PATH}.success must be true`);
    const data = envelope.data;
-   if (!Array.isArray(data)) throw new Error(`${INSTALLATIONS_PATH}.data must be an array`);
-   return data.map((item: unknown, index: number) => {
+   if (!Array.isArray(data)) throw new Error(`${TOOLS_PATH}.data must be an array`);
+   // 后续行路径同步改用 TOOLS_PATH 前缀


─── packages/api-client/src/plugins.ts:396-404 ───
[maintainability · low] nameList 与上文 scopeList 的函数体逐字相同（null/undefined 归一为
[]，非字符串数组抛同文案错误），仅注释和命名不同——这是一对完全同体的解析 helper。后续若调整 string[] 的归一或校验规则需要同步改两处，容易漂移。建议删除
nameList，drift/myConnection 的工具名清单直接复用 scopeList（或提取一个语义中性的 stringList 供两者共享）。

- /** Tool-name list normalized to [] (never null) — mirrors the [] wire shape
-  * the drift handler guarantees (OCR R1 F09) so consumers can .map/.length. */
- function nameList(value: unknown, path: string): string[] {
-   if (value === null || value === undefined) return [];
-   if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) {
-     throw new Error(`${path} must be a string array`);
-   }
-   return value;
- }
+ // 统一复用 scopeList（或将二者合并为语义中性的 stringList）：
+ // requiresAuthTools: stringList(data.requires_auth_tools, `${CONNECTION_PATH}.data.requires_auth_tools`),
+ // snapshotToolNames: stringList(data.snapshot_tool_names, `${DRIFT_PATH}.data.snapshot_tool_names`),


─── scripts/lib/node-gte26.mjs:158-159 ───
[maintainability · low] shimEnv 的回退分支 `baseEnv.PATH` 是死代码：当 `pathKeys.length === 0` 时，说明 baseEnv
中不存在任何大小写等于 "PATH" 的键（包括 "PATH" 本身），因此 `baseEnv.PATH` 恒为 `undefined`，永远不会产生有效值——实际全靠 joinShimPath 的
falsy 过滤兜住。建议直接索引（越界索引本身返回 undefined），消除这个看似有语义、实则不可达的分支：

-   const existing = pathKeys.length > 0 ? baseEnv[pathKeys[0]] : baseEnv.PATH;
+   const existing = baseEnv[pathKeys[0]];
    env.PATH = joinShimPath(shimDir, existing);


─── scripts/lib/node-gte26.mjs:50-52 ───
[performance · low] findNodeGte26 的非 darwin 分支无条件推送 "/usr/local/bin/node"、"/usr/bin/node" 两个 POSIX
路径：在 win32 下它们必然不存在，每次发现过程都要执行两次注定 ENOENT 失败并被静默吞掉的 execFileSync 进程派生。该模块在 shimEntryName 与 $PATH
扫描处都细致区分了 win32（node.exe/PATHEXT），唯独此处遗漏。无功能危害，但属无效候选与平台分支缺口，建议排除 win32：

-   } else {
+   } else if (process.platform !== "win32") {
      candidates.push("/usr/local/bin/node", "/usr/bin/node");
    }


─── apps/web/src/settings/SettingsPage.tsx:707-707 ───
[maintainability · low] 此处 `locale === 'zh-CN' ? '插件' : 'Plugins'` 与第 702 行 integration-plugins
兜底的表达式逐字重复。两处注释分别标注由不同任务（T08-OCR1-F1 迁回、T03 迁入 SECTION_LABEL_KEYS）独立迁移 i18n
词条——相同的直译字面量散落两处，后续任一任务落位时漏改另一处就会留下永久直译、造成两个入口文案不一致。建议提取一个共享的兜底函数/常量，两处共同引用，迁移时只需删除一处定义。

-   if (key === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';
+ // 两处 plugins 直译兜底（integration-plugins tab 与 settings plugins 分区）共用单一来源，
+ // i18n 词条落位后仅需删除此函数与两处引用。
+ const pluginsFallbackLabel = (locale: Locale) => (locale === 'zh-CN' ? '插件' : 'Plugins');


─── apps/web/src/settings/SettingsPage.tsx:475-475 ───
[maintainability · low] portedPanel 的嵌套三元链随 plugins 加入已达 10+
层，每新增一个已迁面板就要再嵌一节，违反项目「嵌套三元表达式不允许」的规范，且可读性随面板数量持续劣化。此处各 panel 变量已是「命中为元素、否则 null」的形态，天然适合改为 Record
映射表：`const PANEL_BY_KEY: Record<string, ReactNode> = { plugins: pluginsPanel, mcp: mcpPanel, ...
}`，`portedPanel` 退化为一次查表 + PARTIALLY_PORTED_SECTIONS 兜底。建议在后续切片顺手收敛，避免链继续膨胀。

-     const portedPanel = key === 'plugins' ? pluginsPanel : key === 'mcp' ? mcpPanel : key === 'models' ? modelPanel : key === 'sandbox' ? sandboxPanel : key === 'skills' ? skillPanel : key === 'members' ? membersPanel : key === 'runtime-queues' ? runtimeQueuesPanel : key === 'system-global' ? systemGlobalPanel : key === 'platform-api-keys' ? platformApiKeysPanel : key === 'system-audit-log' ? systemAuditPanel : PARTIALLY_PORTED_SECTIONS.has(key)
+     const panelByKey: Record<string, React.ReactNode> = {
+       plugins: pluginsPanel, mcp: mcpPanel, models: modelPanel, sandbox: sandboxPanel,
+       skills: skillPanel, members: membersPanel, 'runtime-queues': runtimeQueuesPanel,
+       'system-global': systemGlobalPanel, 'platform-api-keys': platformApiKeysPanel,
+       'system-audit-log': systemAuditPanel,
+     };
+     const portedPanel = panelByKey[key] ?? (PARTIALLY_PORTED_SECTIONS.has(key)


─── packages/views/src/settings/registry.ts:43-43 ───
[bug · high] 新增设置分区 key `'plugins'` 与本 PR 在 integrations
registry（packages/views/src/integrations/registry.ts）新增的 tab key `'plugins'`（即 `integration-plugins`
分区）撞名，导致设置面板深链失效：

- `normalizeIntegrationSettingsSection`（packages/views/src/integrations/settings-route.ts:13）会把
INTEGRATION_SECTIONS 中的裸 key 一律加 `integration-` 前缀；而 SettingsPage 的 `requestedSection`（初始挂载与
popstate 都走它）正是经 `integrationSettingsQuery` 解析 URL 的。
- 侧边栏点击「插件」时 `select('plugins')` 经 `selectSettingsQuery` 把 URL 原样写为
`?section=plugins`；但刷新/直接打开/popstate 回到该 URL 时会被归一化为 `integration-plugins`，落到成员发现
tab（IntegrationsRoutePage）而非本管理员面板——URL 与实际内容不一致，且该设置面板不存在任何可用的深链
URL（integrations/registry.test.ts:31 还显式断言 `?section=plugins` 归属 integration tab，双方各自声称同一别名）。

建议：在 `normalizeIntegrationSettingsSection` 中仅当裸 key 不是已注册的 SETTINGS_SECTIONS key 时才加 `integration-`
前缀（settings 分区 key 优先），或为本设置分区改用不冲突的 key（如 `plugin-install`）。

-   { key: 'plugins', viewId: 'PluginSettings', apiDomain: 'plugins', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'], ported: true },
+ // packages/views/src/integrations/settings-route.ts —— settings 分区 key 优先，避免裸 key 被劫持到 integration tab：
+ export function normalizeIntegrationSettingsSection(section: string, tab?: string | null): string {
+   if (section === 'integrations') { /* 原逻辑 */ }
+   if (settingsSection(section)) return section; // 已注册的 settings 分区不再改写（plugins 等）
+   return INTEGRATION_SECTIONS.some((item) => item.key === section) ? `integration-${section}` : section;
+ }


LLM retry report summary: 1 of 64 requests affected -- 1 request recovered after retry

Core review (1 request):
- apps/web/src/settings/McpSettingsPanel.tsx,apps/web/src/settings/SettingsPage.tsx,packages/views/src/settings/registry.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
