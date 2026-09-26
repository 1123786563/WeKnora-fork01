Review complete: 30 finding(s) across 52 selected item(s).

─── apps/web/src/integrations/PluginsPanel.tsx:210-218 ───
[performance · medium] authorizeConnection 的轮询终止条件不完整，有两处与注释宣称（R2 F08「卸载后轮询不再发后台请求」）不符：
1) 循环因 `!aliveRef.current` break 后，`if (!authorized) await loadConnection(...)` 兜底刷新无条件执行——面板已随 tab
切换卸载，仍会补发一次 getMyConnection 请求（setState 虽 no-op，请求照发）。
2) client 切换场景（页面不重挂载、仅 pluginsApi 重建）：aliveRef 仍为 true，循环没有任何终止条件，闭包里的旧 pluginsApi 会继续每 1.5s 发一次
getMyConnection，最长 40 轮约 60s，回包虽被 epoch 丢弃但请求以旧 principal 身份持续发出（结果弃、请求不停）。
建议：在函数开头捕获 `const epoch = connectionEpochRef.current;`，每轮在发请求前检查 epoch 变化即 break；兜底刷新前加
`aliveRef.current` 条件。

          if (!aliveRef.current) break;
+         // client 切换（epoch 变化）同样终止轮询：不再以旧 pluginsApi 发后台请求
+         if (epoch !== connectionEpochRef.current) break;
          const next = await loadConnection(connection.installationId);
          if (next?.authorized) {
            authorized = true;
            break;
          }
          if (popup.closed) break;
        }
-       if (!authorized) await loadConnection(connection.installationId);
+       if (!authorized && aliveRef.current) await loadConnection(connection.installationId);


─── apps/web/src/integrations/PluginsPanel.tsx:327-332 ───
[style · low] 三态徽标使用嵌套三元表达式（检查清单明确禁止）；同文件主渲染的「加载中/空态/列表」链式三元也是同款嵌套。建议改为模块层
state→徽标映射表，既消除嵌套三元也让三态口径单点维护。

-   const badge =
-     connection.state === "authorized"
-       ? { className: pluginBadgeOk, label: "已授权" }
-       : connection.state === "expired"
-         ? { className: pluginBadgeWarn, label: "已过期" }
-         : { className: pluginBadgeMuted, label: "未授权" };
+ // 模块层常量映射（state 与徽标一一对应，消除嵌套三元）
+ const CONNECTION_BADGES: Record<PluginMyConnection["state"], { className: string; label: string }> = {
+   authorized: { className: pluginBadgeOk, label: "已授权" },
+   expired: { className: pluginBadgeWarn, label: "已过期" },
+   unauthorized: { className: pluginBadgeMuted, label: "未授权" },
+ };
+ // 组件内：
+ const badge = CONNECTION_BADGES[connection.state];


─── apps/web/src/integrations/PluginsPanel.tsx:260-267 ───
[maintainability · low] 安装行的徽标序列 JSX（name/pluginId/version/状态/漂移/需个人授权/工具数）与 PluginsSettingsPanel
的对应行近乎逐字重复，仅状态文案不同（此处「可用/不可用」vs 管理侧「启用中/已停用」）、漂移徽标管理侧多一个「无漂移」分支。徽标 class 已收敛到 plugins/ui.ts
单一来源，但行结构仍在两处维护，后续任何行级调整需双处同步、极易漂移。建议提取共享行徽标组件（如 PluginInstallBadges，状态文案经 props 传入），两面板复用。



─── apps/web/src/integrations/PluginsPanel.tsx:219-223 ───
[maintainability · low] authorizeConnection 与 revokeConnection 的 catch 直接透传任意 Error 的 message：网络层
TypeError（如 "Failed to fetch"）等非 ApiError 的英文原文会直接进入界面，且无 console.warn 留痕——与本文件列表 effect 及
plugins/ui.ts 共享错误口径（apiErrorMessage：仅 ApiError 原文透传，其余中文兜底 + warn 留痕）不一致。注意 window.open 失败抛出的本地中文
Error（"未能打开授权窗口…"）依赖此透传路径，建议改用 apiErrorMessage 判定 ApiError，本地已知错误单独识别透传，其余兜底中文并 warn 留痕。

      } catch (cause) {
-       setActionError(cause instanceof Error ? cause.message : "发起授权失败");
+       const message = apiErrorMessage(cause);
+       if (message !== null) {
+         setActionError(message);
+       } else {
+         console.warn("plugin connection authorize failed:", cause);
+         setActionError(cause instanceof Error && LOCAL_KNOWN_MESSAGES.has(cause.message) ? cause.message : "发起授权失败");
+       }
      } finally {
        setPendingId(null);
      }


─── apps/web/src/integrations/PluginsPanel.tsx:132-134 ───
[maintainability · low] 第二个 effect 调用了组件内非 memoized 的 loadConnection，但依赖数组只列 [pluginsApi,
installations]，缺失 loadConnection——react-hooks/exhaustive-deps 会告警；当前行为正确仅因 loadConnection 闭包捕获的
pluginsApi 恰与依赖一致，属脆弱的隐式约定（同批 PluginsSettingsPanel.refreshInstallations 已用 useCallback
解决同款问题并在注释中说明先例）。建议 loadConnection 用 useCallback([pluginsApi]) 包裹并列入依赖。

-       void loadConnection(row.installationId);
-     }
-   }, [pluginsApi, installations]);
+   const loadConnection = React.useCallback(
+     (installationId: string): Promise<PluginMyConnection | null> => { /* … */ },
+     [pluginsApi],
+   );
+   // …
+   }, [pluginsApi, installations, loadConnection]);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:661-663 ───
[bug · medium] 首挂载假空态：面板挂载后 installations 初始为 []、listError 为 null，列表 effect
首拉在途期间即命中本分支，闪现「暂无已安装插件」——弱网下管理员会误以为空间没有任何插件。同批成员面板 PluginsPanel 已用 listLoaded 标记修复完全相同的问题（其 OCR R2
F06 注释自证先例），管理面板口径未同步。建议增加 listLoaded 标记（无 SSR 预载，初始 false），在 refreshInstallations 的 finally 中置
true，首拉完成前呈现「加载中…」。

-         {installations.length === 0 ? (
+         {!listLoaded && listError === null ? (
+           <Status>加载中…</Status>
+         ) : installations.length === 0 ? (
            listError === null ? <Status>暂无已安装插件</Status> : null
          ) : (


─── apps/web/src/settings/PluginsSettingsPanel.tsx:336-338 ───
[style · low] toggleState 函数尾部收括号 `}  }` 格式错乱（函数结束与 try/finally 结束挤在同一行且双空格），疑为手工合并残留，影响可读性；同文件混用
`React.useCallback` 与已具名导入的 hooks（useCallback 未入 import 列表），建议一并统一为具名导入。

      } finally {
        setActionBusyId(null);
-     }  }
+     }
+   }


─── apps/web/src/settings/PluginsSettingsPanel.tsx:441-445 ───
[bug · medium] runDriftResolve 把「重定基写操作」与「重读报告/刷新列表」放在同一 try/catch：当 resolveDrift 已成功、后续 getDrift（或
refreshInstallations）失败时，会落入同一 catch 显示「漂移重定基失败，已回旧快照」——但服务端实际已按新目录重定基成功，文案与真实状态相反；同时
setToolPolicy(null) 与列表刷新被跳过，治理面与漂移徽标停留在重定基前的旧快照。建议将 resolveDrift 单独
try（仅其本身失败才宣称「已回旧快照」），成功后立即失效治理面并刷新列表，报告加载失败单独表述。

      try {
        await pluginsApi.resolveDrift(driftView.installationId);
+     } catch (cause) {
+       // 仅重定基本身失败才宣称「已回旧快照」
+       const message = apiErrorMessage(cause);
+       setDriftView((prev) => prev && prev.installationId === driftView.installationId ? { ...prev, busy: null, error: message ?? "漂移重定基失败，已回旧快照" } : prev);
+       if (message === null) console.warn("plugin drift resolve failed:", cause);
+       return;
+     }
+     setToolPolicy(null);
+     await refreshInstallations();
+     try {
        const report = await pluginsApi.getDrift(driftView.installationId);
-       setDriftView((prev) =>
-         prev && prev.installationId === driftView.installationId ? { ...prev, busy: null, report, error: null } : prev);
+       setDriftView((prev) => prev && prev.installationId === driftView.installationId ? { ...prev, busy: null, report, error: null } : prev);
+     } catch (cause) {
+       setDriftView((prev) => prev && prev.installationId === driftView.installationId ? { ...prev, busy: null, error: "重定基已生效，报告加载失败，请稍后重试" } : prev);
+     }


─── apps/web/src/settings/PluginsSettingsPanel.tsx:661-663 ───
[style · low] 空态分支使用了嵌套三元（外层 installations.length === 0、内层 listError ===
null），检查清单禁止嵌套三元；且与首拉空态问题叠加（installations 初始为 []、listError 为 null 时闪现「暂无已安装插件」，成员面板 PluginsPanel 已用
listLoaded 修复同款问题）。建议拆成两段平铺条件并结合 listLoaded 标记一并修复。

-         {installations.length === 0 ? (
-           listError === null ? <Status>暂无已安装插件</Status> : null
-         ) : (
+         {!listLoaded && listError === null ? <Status>加载中…</Status> : null}
+         {listLoaded && listError === null && installations.length === 0 ? (
+           <Status>暂无已安装插件</Status>
+         ) : null}
+         {installations.length > 0 ? (
+           <ul className="m-0 grid list-none gap-2 p-0">


─── apps/web/src/settings/PluginsSettingsPanel.tsx:717-720 ───
[maintainability · low] 「接受升级」在途（upgradeAcceptBusy）期间，列表行四个操作按钮（工具治理/检查升级/漂移复审/停用启用）的 disabled 均未包含
upgradeAcceptBusy——与本面板其它互斥冻结口径不一致（漂移复审按钮冻结 actionBusyId/upgradeBusyId，acceptUpgrade 成功后还会
setToolPolicy(null) 关闭治理面并整体换版）。在途时展开治理面或点击开关会被随后的面板失效/旧工具名 404 打断。建议各列表行按钮 disabled 补上
upgradeAcceptBusy。

-                       loading={actionBusyId === item.installationId}
-                       // OCR R1 F40：与同组两按钮一致的跨行禁用——toggleState
-                       // 首行守卫静默吞掉其他行在途时的点击，UI 必须同步冻结。
-                       disabled={actionBusyId !== null && actionBusyId !== item.installationId}
+                       disabled={(actionBusyId !== null && actionBusyId !== item.installationId)
+                         || upgradeAcceptBusy}


─── apps/web/src/settings/SettingsPage.tsx:702-702 ───
[maintainability · low] 此直译兜底只覆盖 zh-CN/en 两种语言：packages/views/src/integrations/messages.ts 的
FALLBACK_STRINGS 已为 `integrations.tabs.plugins` 提供五语言词条（ja-JP/ko-KR/ru-RU 均有），integrations 页 tab 按钮经
integrationsT 呈现本地语言，而设置页侧边栏经此硬编码短路会对日/韩/俄用户显示英文 "Plugins"，同一入口两处文案分叉。下方 settings 分区的 `if (key ===
'plugins')` 同款问题。建议复用 integrationsT(locale, 'integrations.tabs.plugins')（fallback 表兜底，@weknora/i18n
主表词条落位后自动优先命中），替代手写双语直译。

-   if (integrationTab === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';
+   if (integrationTab === 'plugins') return integrationsT(locale, 'integrations.tabs.plugins');


─── docs/architecture/moves/appconnector.yaml:109-109 ───
[documentation · low] 集成点行号不准确：RegisterPluginRoutes 的函数签名实际位于 internal/router/routes_plugins.go:23（第
16 行是函数 doc comment 中段）。同文件既有条目「RegisterAppConnectorRoutes —
internal/router/routes_app_connectors.go:20」精确指向签名行（routes_app_connectors.go:20 即 func
声明行），新增条目打破该行号约定，会使迁移导航与审计链路定位偏差 7 行。

-   - RegisterPluginRoutes — internal/router/routes_plugins.go:16
+   - RegisterPluginRoutes — internal/router/routes_plugins.go:23


─── examples/plugins/jira-todo-mcp/main.go:232-234 ───
[bug · low] validateAllowedRedirectHosts 漏掉 port=="" 的条目形态：net.SplitHostPort("example.com:") 成功返回
("example.com", "", nil)，net.SplitHostPort("[::1]") 同样成功返回 ("::1", "", nil)——host 非空、port 为空串，既不命中
host=="" 分支也不命中 80/443 分支，校验放行。但 handleRegister 的匹配是 entry==parsed.Host ||
entry==bareHost，"example.com:" 永不等于 "example.com" 或 "example.com:8443"，"[::1]" 永不等于 "[::1]:8443" 或
"::1"——该条目对任何真实 redirect_uri 恒不匹配，运营者把唯一条目写成此形态时整个 allowlist 静默失效（全部注册被拒）。这与本函数明确要 fail-closed
拒绝的两类「永不命中的静默失效条目」（":port" 空 host、钉默认端口）完全同类，建议补 port=="" 拒绝（裸 IPv6 应提示用无括号的 "::1" 形态，后者经 Hostname()
比对可命中）。

+ 		if port == "" {
+ 			return fmt.Errorf("AllowedRedirectHosts entry %q ends with a colon but has no port (never matches a real redirect_uri host); use host:port, or the bare host without brackets for IPv6", entry)
+ 		}
  		if port == "80" || port == "443" {
  			return fmt.Errorf("AllowedRedirectHosts entry %q pins a default port that URIs normally elide (url.Parse does not materialize it); use the bare host instead", entry)
  		}


─── examples/plugins/jira-todo-mcp/oauth.go:586-586 ───
[documentation · low] 同意页文案「仅用于本次授权验证，服务不保存凭据」与实现不符：凭据经验证后存入 oauthSession（Email+APIToken），随
refreshTokens 在内存驻留最长 refreshTokenTTL=30 天，且此后每次工具调用（handleSearchMyWeek）都以该凭据请求
Jira——并非「仅用于本次授权验证」。「不保存」实际仅为不落盘，内存长期驻留未披露。README（第 119 行）已如实写明 refresh 有效期 30
天，但同意页是成员决定是否交出凭据的唯一知情界面，文案低估凭据用途与存留期，构成知情同意透明度偏差。建议改为如实披露用途与驻留时长。

- <p>输入你的 Jira 邮箱与 API token（仅用于本次授权验证，服务不保存凭据）。</p>
+ <p>输入你的 Jira 邮箱与 API token（凭据仅驻留内存、不落盘不写日志；用于本次授权及后续待办查询，最长随会话保留 30 天）。</p>


─── internal/application/service/plugin_install_service.go:257-261 ───
[maintainability · low] NewManagerEndpointLister 是 container 侧 NewPluginMCPEndpointLister 的 service
包孪生，但少了后者本轮刚补的关键防线：client.ListTools(ctx) 本身不设截止时间。plugin_lister.go 的注释明确指出请求 ctx 本身没有
deadline（server 无 ReadTimeout/WriteTimeout、无路由超时中间件），挂起/慢滴的第三方端点会无限期占用调用 goroutine（T01-R2-F8
的原始动机）。当前唯一生产调用方 markDriftBestEffort 会先以 10s 的 driftMarkListTimeout 包裹
ctx，所以暂时安全；但该函数是导出的，任何未来调用方传入裸 ctx 就会原样复现这一隐患。建议与 container 孪生对齐，在适配器内部自带 30s 兜底
deadline（context.WithTimeout 会取二者更近者，不影响现有 10s 收紧）。

  		defer func() {
  			_ = client.Disconnect()
  			_ = manager.CloseClient(verify.ID)
  		}()
- 		return client.ListTools(ctx)
+ 		// 与 container 侧 NewPluginMCPEndpointLister 对齐：适配器内部兜底
+ 		// deadline，防止未来调用方传入无 deadline 的 ctx（T01-R2-F8）。
+ 		listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
+ 		defer cancel()
+ 		return client.ListTools(listCtx)


─── internal/application/service/plugin_install_service.go:2092-2094 ───
[bug · low] oldSnapshot 的捕获方式在空快照下退化为 nil：`append(types.PluginPreviewTools(nil),
inst.ToolsSnapshot...)` 当 inst.ToolsSnapshot 为空（先前 ResolveDrift 重定基到空 live
目录——ValidateLiveDirectoryForRebase 明文允许）时返回 nil。此后 5a 成功、5b 失败触发的补偿调用 UpdateInstallationAccepted
时，PluginPreviewTools(nil).Value() 产出 SQL NULL，写入 NOT NULL 的 tools_snapshot
列失败——快照回滚失败仅记日志，行停留在新快照，且下方的 drift 判决恢复（else 分支）被跳过，破坏「失败不改变已接受基线」的补偿契约。AcceptUpgrade 的 oldSnapshot
捕获是同一模式。建议修复点放在 types.PluginPreviewTools.Value()（nil → "[]"），或在此处对空快照归一化为
make(types.PluginPreviewTools, 0)。



─── internal/modules/agentruntime/agent/tools/mcp_tool.go:591-599 ───
[maintainability · low] loadPluginDirectory 的 metadata.Put 分支不可达:该函数唯一调用点位于 RegisterMCPTools 闭包的
snap != nil 分支(mcp_tool.go:741),函数内 snap 恒非 nil,因此 `snap == nil && metadata != nil && metadata.Put
!= nil` 永假,metadata 参数实为死参数。这段死代码会误导后续维护者以为插件行仍可能走持久化通道(与注释宣称的"插件行元数据缓存已退役"语义相悖),建议删除该分支并从签名中移除
metadata 参数。

- 	if snap == nil && metadata != nil && metadata.Put != nil {
- 		tenant, _ := types.TenantIDFromContext(loadCtx)
- 		if persistErr := metadata.Put(loadCtx, tenant, service.ID, filtered, instructions); persistErr != nil {
- 			logger.GetLogger(loadCtx).Warnf(
- 				"Failed to persist MCP directory for service %s: %v", service.Name, persistErr,
- 			)
- 		}
- 	}
+ 	// snap 在唯一调用点(RegisterMCPTools 的 snap != nil 分支)恒非 nil,
+ 	// 插件行不走 metadata.Put —— 目录与 instructions 的权威是插件域 API
+ 	// 服务的已接受快照(OCR 终局 F09)。
  	return filtered, instructions, nil
+ }
+ 
+ // 同时从签名移除死参数并同步调用点:
+ // func loadPluginDirectory(
+ // 	loadCtx context.Context,
+ // 	service *types.MCPService,
+ // 	mcpManager *mcp.MCPManager,
+ // 	gate approval.MCPApproval,
+ // 	oauthSess *MCPOAuthSession,
+ // 	snap *PluginRuntimeSnapshot,
+ // ) ([]*types.MCPTool, string, error) {


─── internal/modules/plugins/fetcher.go:105-107 ───
[security · low] 此 url.Parse 失败分支以 %w 透传 *url.Error，其 Error() 文本逐字内嵌完整原始 URL（如 `parse
"https://ci-bot:s3cr3t@host:badport/": invalid port ... after host`）。携带 userinfo 的畸形 URL
恰在此分支泄露凭据——这正是同 PR 中 manifest.go 对 transport endpoint 同场景（ValidateManifest 的 parse 失败路径）专门引入
maskEndpointCredentials/userinfoOf 掩码的原因，且紧邻的注释块声称 "The message does not echo the
URL"，与本分支实际行为矛盾。当前生产路径上 service 层 validateManifestURLInput（plugin_service.go:90 "no details
echoed"）先行拦截使该分支不可达，但 FetchAndVerify 是导出函数、此预检是新增的纵深防御层——未来任何直接以用户输入调用它的入口（drift/新
handler）都会立即暴露。建议与本文件/manifest.go 的掩码纪律对齐：返回固定消息（与 service 层一致），或复用 maskEndpointCredentials
掩码后经截断回显。

- 	if u, err := url.Parse(manifestURL); err != nil {
- 		return nil, fmt.Errorf("invalid manifest URL: %w", err)
+ 	if _, err := url.Parse(manifestURL); err != nil {
+ 		// *url.Error embeds the full URL verbatim — a malformed URL that
+ 		// also carries userinfo would leak credentials (same hygiene as
+ 		// service-layer validateManifestURLInput / manifest.go's masked
+ 		// endpoint path).
+ 		return nil, fmt.Errorf("invalid manifest URL")
  	} else if u.User != nil {


─── internal/types/mcp.go:46-46 ───
[bug · low] 跨文件一致性缺口：新增列在 `dto.MCPServiceResponse` 中没有对应字段，`GET /mcp-services` / `GET
/mcp-services/{id}` 会原样返回插件物化行（`ListMCPServices` 不过滤 `plugin_installation_id != nil`
的行），且响应无法与手工行区分。而本次变更让插件行上的 Update/Delete/Test/Tools/Resources/credentials/metadata 全部确定性 409 —— 管理端
MCP 设置面板（本 PR 同步更新，仅改了 badge 常量，无任何插件感知）没有任何标记可用来隐藏或禁用这些操作入口，管理员在普通编辑流里会直接撞 409 英文报错。建议：在
`dto.MCPServiceResponse` 暴露 `plugin_installation_id`（并在 `NewMCPServiceResponse(s)` 中拷贝），或从通用 MCP
列表过滤插件行，让前端能把插件行渲染为只读并指向插件面板管理。



─── internal/types/plugin.go:71-77 ───
[bug · low] Value() 对 nil 切片返回 SQL NULL，但该类型背书的唯一列
plugin_previews/plugin_installations.tools_snapshot 是 NOT NULL（migrations 000189/000190 均为
JSONB/TEXT NOT NULL）。nil 只能经补偿路径进入：AcceptUpgrade/ResolveDrift 以 `oldSnapshot :=
append(types.PluginPreviewTools(nil), inst.ToolsSnapshot...)` 捕获旧值，当接受快照为空（ResolveDrift 允许重定基到空 live
目录——ValidateLiveDirectoryForRebase 明文放行 nil/empty）时 append 得到 nil，此后任一 7a/5a
之后的失败触发补偿回写，UpdateInstallationAccepted 会把 tools_snapshot 置为 NULL，在 PG 与 SQLite 上都触发 NOT NULL
违约——补偿回写失败（仅记日志），行停留在新快照上而调用方看到错误，违背「补偿恢复旧值」契约（ResolveDrift 的 drift 判决恢复也在其 else 分支被一并跳过）。建议在
Value() 把 nil 序列化为 "[]"（Scan 已兼容空/NULL 两种读回形态），一处修复两个补偿点。

  // Value implements driver.Valuer for PluginPreviewTools (JSON column).
+ // nil 序列化为 "[]" 而非 SQL NULL：该类型背书的 tools_snapshot 列为
+ // NOT NULL，而补偿路径（accept/resolve 的旧快照回写）在旧快照为空时
+ // 会持有一个 nil 切片（append(PluginPreviewTools(nil), ...) 零元素）。
  func (t PluginPreviewTools) Value() (driver.Value, error) {
  	if t == nil {
- 		return nil, nil
+ 		return []byte("[]"), nil
  	}
  	return json.Marshal(t)
  }


─── packages/api-client/src/plugins.ts:427-430 ───
[maintainability · low] transport_type 的「required + 白名单 http-streamable|sse」校验在 parsePluginPreview 与
parsePluginInstallation 两处完整重复（含错误消息模板）。模块内已有 installationState/driftState/connectionState 这类枚举字符串校验
helper 的既定先例，transportType 反而是唯一内联写两处的枚举校验——后续若新增合法传输类型需多点同步修改、易漏改一处造成两解析器行为分叉。建议按同款模式抽取 helper。

-   const transportType = required(data.transport_type, `${INSTALLATIONS_PATH}.data.transport_type`);
-   if (transportType !== 'http-streamable' && transportType !== 'sse') {
-     throw new Error(`${INSTALLATIONS_PATH}.data.transport_type is invalid`);
+ const TRANSPORT_TYPES = ['http-streamable', 'sse'] as const;
+ 
+ function transportType(value: unknown, path: string): 'http-streamable' | 'sse' {
+   const raw = required(value, path);
+   if (!TRANSPORT_TYPES.includes(raw as 'http-streamable' | 'sse')) {
+     throw new Error(`${path} is invalid`);
+   }
+   return raw as 'http-streamable' | 'sse';
-   }
+ }
+ 
+ // 两个解析器内改为：
+ // const type = transportType(data.transport_type, `${PREVIEW_PATH}.data.transport_type`);


─── packages/api-client/src/plugins.ts:759-761 ───
[maintainability · low] 该解析器实际服务于 `${INSTALLATIONS_PATH}/:id/tools` 端点（GET .../tools 与 PUT
.../tools/:tool_name/policy），却复用 INSTALLATIONS_PATH 作为全部错误消息的路径前缀。模块自身在
CONNECTION_PATH/DRIFT_PATH/UPGRADE_PREVIEW_PATH 处已明确建立「前缀对齐真实请求路径（含 :id 段）以便排障定位」的诊断约定（T15-OCR2
注释），此处失真的前缀会在解析失败时误导定位到列表端点。建议比照 CONNECTION_PATH 增加独立常量。

+ // 与 CONNECTION_PATH/DRIFT_PATH 同段声明：
+ const TOOLS_PATH = `${INSTALLATIONS_PATH}/:id/tools`;
+ 
  export function parsePluginToolPolicyRows(value: unknown): PluginToolPolicyRow[] {
-   const envelope = record(value, INSTALLATIONS_PATH);
-   if (envelope.success !== true) throw new Error(`${INSTALLATIONS_PATH}.success must be true`);
+   const envelope = record(value, TOOLS_PATH);
+   if (envelope.success !== true) throw new Error(`${TOOLS_PATH}.success must be true`);
+   // 行路径前缀同步改为 `${TOOLS_PATH}.data[${index}]` ...


─── packages/api-client/src/plugins.ts:184-185 ───
[security · low] manifestUrl 仅做 trim 非空校验即发请求；已核实调用侧（PluginsSettingsPanel.submit）同样只有空串判断、无 scheme
预检。spec 的 SSRF 防线在服务端固然完备，但明显非法输入（file://、javascript: 等）仍会发起一次网络往返后才收到后端拒绝，且错误文案不如本地校验直观。建议客户端
fail-fast 预检 http/https 作为防御纵深（安全强制点仍保留在服务端）。

        const url = manifestUrl.trim();
        if (url === '') throw new Error('manifestUrl must not be empty');
+       let parsed: URL;
+       try {
+         parsed = new URL(url);
+       } catch {
+         throw new Error('manifestUrl must be a valid URL');
+       }
+       if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
+         throw new Error('manifestUrl must use http or https');
+       }


─── packages/api-client/src/plugins.ts:211-215 ───
[maintainability · low] 「trim + 非空抛错」的参数校验样板在 createPluginsApi 的 10 个方法（getInstallation /
setInstallationState / previewUpgrade / acceptUpgrade / getDrift / checkDrift / resolveDrift /
getMyConnection / listInstallationTools / setInstallationToolPolicy）中逐字重复，仅参数名不同；previewId /
candidateFingerprint / toolName 又是同一模式的变体。模块内已有 installationState / driftState / connectionState
这类字段校验 helper 的既定先例，建议同款抽取一个 requireNonEmpty(value, label) helper 收敛约 20
行样板——后续若要统一加长度上限或改错误文案，只改一处。

-       const id = installationId.trim();
-       if (id === '') throw new Error('installationId must not be empty');
-       return parsePluginInstallation(await request({
-         method: 'GET',
-         path: `${INSTALLATIONS_PATH}/${encodeURIComponent(id)}`,
+ function requireNonEmpty(value: string, label: string): string {
+   const trimmed = value.trim();
+   if (trimmed === '') throw new Error(`${label} must not be empty`);
+   return trimmed;
+ }
+ 
+ // 各方法内：
+ const id = requireNonEmpty(installationId, 'installationId');


─── scripts/lib/node-gte26.mjs:110-112 ───
[performance · medium] win32 无开发者模式时 symlinkSync 必然 EPERM，该回退会把整个 node 二进制（约 80MB+）完整复制到新建的 mkdtemp
目录、退出时再删除——而 run-with-node-gte26.mjs 在当前 node 已 ≥26 时也无条件走 createNodeShim，没有跳过路径。结果是 Windows
本地开发者每次执行 pnpm test:shared / test:craft:shared / test:mobile-v2 都要付一次大文件复制
I/O（叠加杀软扫描）的代价。建议二选一：对复制副本按 targetBin 指纹（路径+版本）缓存复用；或在包装器里当 shim 目标与 PATH 当前解析到的 node（realpath
一致）相同时直接跳过 shim 创建。



─── scripts/lib/node-gte26.mjs:27-27 ───
[bug · low] 发现流程会对 $PATH 的每个条目逐一 execFileSync 探测版本，但未设 timeout——一旦某个候选是挂起或等待输入的损坏包装器（nvm/volta
shim、交互式 wrapper），整个 gates / test:shared 将无限阻塞：CI 侧尚有 job 的 timeout-minutes
兜底，本地运行则无上限。建议给探测加超时；超时抛出的错误会被现有 catch 吞掉、该候选被跳过，语义完全不变，是一行式加固。

-     const v = execFileSync(nodeBin, ["-v"], { encoding: "utf8" });
+     const v = execFileSync(nodeBin, ["-v"], { encoding: "utf8", timeout: 10_000, killSignal: "SIGKILL" });


─── scripts/lib/node-gte26.mjs:189-190 ───
[maintainability · low] 作为共享库 API，直接 process.on + process.exit 会静默覆盖调用方已注册的 SIGINT/SIGTERM 处理器，且未覆盖
SIGHUP（终端断开时同样会跳过清理、泄漏 shim 目录）。当前仅两个一次性入口脚本使用、影响有限，但一旦该模块被长驻工具或其它脚本 import
即成隐患。建议：注册前保存既有监听器并链式调用，或至少补上 SIGHUP=129，并在 JSDoc 中注明“独占信号处理”的使用约束。

-   process.on("SIGINT", () => onSignal(130));
-   process.on("SIGTERM", () => onSignal(143));
+   for (const [sig, code] of [["SIGHUP", 129], ["SIGINT", 130], ["SIGTERM", 143]]) {
+     process.on(sig, () => onSignal(code));
+   }


─── scripts/run-gates.mjs:81-81 ───
[maintainability · low] 这条失败文案与 scripts/run-with-node-gte26.mjs
中完全相同的一份逐字重复——这正是本次抽取共享库要消除的复制漂移（F11）形态：后续再调整发现顺序（增删候选来源）时，两处提示极易再次失同步。建议在
scripts/lib/node-gte26.mjs 导出统一的提示常量/函数，两个入口共用一份。

-           "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)",
+ // scripts/lib/node-gte26.mjs 中导出:
+ export const NODE_NOT_FOUND_HINT =
+   `no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — ` +
+   "install node 26 (e.g. brew install node@26 / nvm install 26)";


─── scripts/run-gates.mjs:46-48 ───
[documentation · low] 头部用法文档与本次抽库后的实际行为失同步:文件头(第 14 行)仍写 "Node >=26 discovery order:
$WEKNORA_NODE_BIN, homebrew node, nvm versions",而本 PR 已将发现逻辑扩展为 WEKNORA_NODE_BIN → 平台前缀(darwin
homebrew / Linux /usr/local/bin+/usr/bin)→ ~/.nvm → 全量 $PATH 扫描(F09/F11)——"homebrew node" 现仅是 darwin
分支,Linux 前缀与 $PATH 扫描在头文档中缺失。`pnpm gates`
是面向开发者的入口,头注释是唯一契约说明,这种文档漂移恰是本次抽库(F11)想消除的复制失同步形态。建议在移动注释处同步刷新头部发现顺序一句话,或直接指向共享库的权威文档。

  // majorOf/candidateBin/findNodeGte26 moved to scripts/lib/node-gte26.mjs
  // (OCR round-1 F11: the duplicated copies had drifted into a copy-paste
  // contract — the CI Linux discovery gap, F09, lived in both).
+ // NOTE: keep the header's "discovery order" sentence in sync with
+ // findNodeGte26() — now $WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, $PATH.


─── scripts/run-with-node-gte26.mjs:81-85 ───
[security · low] win32 下 shell:true 会把 command+args 拼成一行交给 cmd.exe 重新解释，且 shell 模式下 Node
不再做参数引号转义——任何含空格、&、|、^、% 等元字符的参数都会被分裂或注入。当前全仓库的调用点确实只有 package.json 的固定命令（已核实），但该文件定位是通用包装入口（--
<command>），注释里“surface is fixed”的假设只存在于文档。建议把约束落到代码：win32 下检测到 cmd 元字符即拒绝执行并给出明确报错，避免未来新增调用点时踩雷。

+   const needsShell = process.platform === "win32";
+   if (needsShell && [command, ...args].some((a) => /[&|^<>%!"']/u.test(a))) {
+     console.error("[with-node] refusing to pass cmd.exe metacharacters through shell:true");
+     process.exit(2);
+   }
    const result = spawnSync(command, args, {
      stdio: "inherit",
      env,
-     shell: process.platform === "win32",
+     shell: needsShell,
    });

