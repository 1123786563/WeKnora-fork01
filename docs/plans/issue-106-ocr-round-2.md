Review complete: 35 finding(s) across 44 selected item(s).

─── .github/workflows/frontend.yml:116-117 ───
[test · medium] 该自测步骤的既定目标是让 node>=26
管线(scripts/lib/node-gte26.mjs、scripts/lib/node-gte26.test.mjs、scripts/run-with-node-gte26.mjs、script
s/run-gates.mjs)在 CI 中受到回归保护,但本 workflow 的 on.push / on.pull_request paths
过滤器中没有任何指向这些文件的条目(现有条目仅显式列出了
scripts/build_frontend_dist.sh、scripts/build_react_web_bundle.sh、scripts/check-react-boundaries.mjs
三个脚本)。一个只修改管线本身的 PR(例如修复 findNodeGte26 或 shimEnv 的缺陷,通常不会动 package.json)会因 paths 不匹配而整体跳过本
workflow,这个新步骤根本不会执行——自测恰好在其最需要运行的变更上失效,回归照旧只在开发者手工运行时才暴露。建议在两处 paths
过滤器中补充这些脚本的路径(与既有按脚本逐条列出的约定保持一致,例如
"scripts/lib/node-gte26.mjs"、"scripts/lib/node-gte26.test.mjs"、"scripts/run-with-node-gte26.mjs"、"sc
ripts/run-gates.mjs",或统一用 "scripts/lib/**" + 两个入口脚本)。

-       - name: Test node-gte26 gate plumbing
-         run: pnpm test:node-gte26
+ on:
+   push:
+     paths:
+       # ... existing entries ...
+       - "scripts/lib/**"
+       - "scripts/run-with-node-gte26.mjs"
+       - "scripts/run-gates.mjs"
+   pull_request:
+     paths:
+       # ... existing entries ...
+       - "scripts/lib/**"
+       - "scripts/run-with-node-gte26.mjs"
+       - "scripts/run-gates.mjs"


─── apps/web/src/integrations/PluginsPanel.tsx:127-127 ───
[bug · medium] loadConnection 缺少 client 切换时的代际/陈旧守卫：第三个 useEffect（[pluginsApi]）只清空 connections 与
requestedRef，但旧 principal 已在途的 getMyConnection 响应仍会经此函数式 setConnections 落键——若晚于新 principal
的重新拉取返回，旧账号的「已授权/已过期」徽标与 serviceId 将覆盖新视图，且 authorizeConnection/revokeConnection 会基于旧 serviceId
发起操作。注释中引用的先例 PluginsSettingsPanel.refreshInstallations（其 199-218 行）正是以 isStale 回调 + toolPolicyEpoch
代数丢弃陈旧回包来防这一竞态；现有 F42 测试两个 client 均同步返回，未覆盖该时序。建议仿照先例加 epoch 比对（也可顺带在轮询循环退出条件中检查代数，终止旧 client
上的持续轮询）。

+ // 组件顶层：
+ const connectionEpoch = useRef(0);
+ 
+ useEffect(() => {
+   connectionEpoch.current += 1; // 在途旧回包凭旧代数被丢弃
+   requestedRef.current = new Set();
+   setConnections({});
+ }, [pluginsApi]);
+ 
+ function loadConnection(installationId: string): Promise<PluginMyConnection | null> {
+   const epoch = connectionEpoch.current;
+   return pluginsApi
+     .getMyConnection(installationId)
+     .then((next) => {
+       if (epoch === connectionEpoch.current) {
          setConnections((prev) => ({ ...prev, [next.installationId]: next }));
+       }
+       return next;
+     })
+     .catch((cause: unknown) => {
+       if (epoch !== connectionEpoch.current) return null;
+       console.warn("plugin connection load failed:", cause);
+       setConnections((prev) => ({ ...prev, [installationId]: "load-failed" }));
+       return null;
+     });
+ }


─── apps/web/src/integrations/PluginsPanel.tsx:118-121 ───
[bug · low] 该重置 effect 在首挂时也会无条件执行：effects 按声明序运行，上一 effect 刚基于 initialInstallations 标记 requestedRef
并发起 loadConnection，本 effect 随即清空去重集；待 listInstallations 返回触发 setInstallations（新数组引用）后，上一个 effect
重跑时对同一 installationId 会再次发起 getMyConnection——Props 注释声明的 SSR/首屏直出路径（测试 PluginsPanel.test.tsx:43-73
即在用）下每个需授权行连接状态被请求两次。建议跳过首跑，仅在 pluginsApi 真正变化（client 切换）时重置。

+ const clientChangedRef = useRef(false);
    useEffect(() => {
+     if (!clientChangedRef.current) {
+       clientChangedRef.current = true;
+       return; // 首挂不重置：initialInstallations 路径的去重标记需保留
+     }
      requestedRef.current = new Set();
      setConnections({});
    }, [pluginsApi]);


─── apps/web/src/integrations/PluginsPanel.tsx:203-203 ───
[maintainability · medium] plugins tab 出现标题/描述双重渲染：page.tsx 对非 external section（plugins 属之，其 517-523
行）已按 HEADING_KEYS/DESCRIPTION_KEYS 渲染「空间插件」标题与五语言副标题（messages.ts 本次已补齐），而面板又自带同名 h2
与描述段，用户在同屏看到两份标题，且两份副标题文案不一致（面板版多「需个人授权……」分句）。先例 im/embed/api 面板（ChannelListPanel 等）均不重复 section
标题。建议删除面板内置标题块；若需保留更完整的文案，合并进 integrations.plugins.subtitle 词条。

-         <h2 className="m-0 mb-1 text-[18px] font-semibold leading-[normal] text-[rgb(0_0_0_/_90%)]">空间插件</h2>
+ // 删除面板内 <div><h2>空间插件</h2><p>…</p></div> 标题块——
+ // page.tsx 的 section heading（integrations.plugins.title/subtitle）已渲染；
+ // 需保留「需个人授权的工具按成员本人授权调用」分句时，将其并入 messages.ts
+ // 的 integrations.plugins.subtitle 词条（五个 locale 同步）。


─── apps/web/src/integrations/PluginsPanel.tsx:286-291 ───
[style · low] badge 取值使用嵌套三元（authorized ? … : expired ? … : …），违反代码规范「禁止嵌套三元表达式」。connection.state
是封闭联合类型，改为查表映射更清晰。

-   const badge =
-     connection.state === "authorized"
-       ? { className: pluginBadgeOk, label: "已授权" }
-       : connection.state === "expired"
-         ? { className: pluginBadgeWarn, label: "已过期" }
-         : { className: pluginBadgeMuted, label: "未授权" };
+ const CONNECTION_BADGES = {
+     authorized: { className: pluginBadgeOk, label: "已授权" },
+     expired: { className: pluginBadgeWarn, label: "已过期" },
+     unauthorized: { className: pluginBadgeMuted, label: "未授权" },
+   } as const;
+   const badge = CONNECTION_BADGES[connection.state];


─── apps/web/src/integrations/PluginsPanel.tsx:210-211 ───
[bug · low] 未区分「加载中」与「无数据」：生产路径挂载后首拉期间 installations 为空且 error 为
null，条件立即命中，先闪现「暂无已安装插件」空态误报，列表返回后才纠正；另外当 installations 为空且 error 非空时会落入 else 分支渲染一个无子项的空 <ul>，产生冗余
DOM。建议引入列表加载完成标记（initialInstallations 直出路径可直接视为已加载）。

-       {installations.length === 0 && error === null ? (
+ // state 增加：const [listLoaded, setListLoaded] = useState(initialInstallations.length > 0);
+ // listInstallations().then 中：setListLoaded(true);
+ 
+ {!listLoaded ? (
+         <Status>空间插件目录加载中…</Status>
+       ) : installations.length === 0 ? (
          <Status>暂无已安装插件</Status>
+       ) : (
+         <ul className="m-0 grid list-none gap-2 p-0">…</ul>
+       )}


─── apps/web/src/integrations/PluginsPanel.tsx:1-2 ───
[other · low] `import * as React from "react"` 未被使用：全文件无任何 `React.` 命名空间引用（仅用了下方具名 hooks 导入），且
tsconfig 为 jsx: "react-jsx" 自动运行时，JSX 不依赖该导入。属死导入，建议删除。

- import * as React from "react";
  import { useEffect, useMemo, useRef, useState } from "react";


─── apps/web/src/integrations/PluginsPanel.tsx:168-176 ───
[performance · low] authorizeConnection 的弹窗轮询缺少组件卸载终止条件：本面板挂载于 integrations 页 plugins tab
的内容插槽（page.tsx 仅在 tab === 'plugins' 时渲染 pluginsSlot），用户点「去授权」后切到其他 tab 面板即卸载，而轮询循环会继续以 1.5s 间隔发起
getMyConnection 请求直至 40 次耗尽（约 60s）——期间 setState 虽为 no-op，但后台网络请求照发。所引用先例 McpSettingsPanel
是常驻设置面板，卸载罕见；本面板卸载（切 tab）是高频路径，建议引入卸载标志（或复用 getMyConnection 已支持的 AbortSignal 参数）在循环内提前退出。

+   // 组件顶层新增卸载标志
+   const aliveRef = useRef(true);
+   useEffect(() => () => { aliveRef.current = false; }, []);
+ 
+   // authorizeConnection 轮询内：
-       for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
+   for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
-         await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
+     await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
+     if (!aliveRef.current) return; // 已随 tab 切换卸载：停止轮询（finally 仍会执行，setState 为 no-op）
-         const next = await loadConnection(connection.installationId);
+     const next = await loadConnection(connection.installationId);
-         if (next?.authorized) {
+     if (next?.authorized) {
-           authorized = true;
+       authorized = true;
-           break;
+       break;
-         }
+     }
-         if (popup.closed) break;
+     if (popup.closed) break;
-       }
+   }


─── apps/web/src/integrations/PluginsPanel.tsx:157-161 ───
[maintainability · low] 弹窗授权流程与 McpSettingsPanel.startAuthorize（833-864 行）大面积同构复制：authorizeUrl
入参（redirectURI/frontendRedirect 拼接）、弹窗名与尺寸特征串（"weknora_mcp_oauth"/"width=600,height=720"）、1.5s×40
的轮询节奏均重复维护（ConfigurationEditor.tsx:68 亦是同款第三处）。后续调整轮询策略、弹窗参数或 OAuth 回调路径需多点同步，易漂移。建议将「authorizeUrl →
开弹窗 → 轮询至授权完成」提取为共享 helper，由调用方注入轮询探针与错误文案。

-       const result = await client.configuration.mcp.oauth.authorizeUrl(connection.serviceId, {
+ // 提取共享 helper（如 apps/web/src/mcpOAuthPopup.ts），面板与设置面板共用：
+ export async function openMcpOAuthPopupAndPoll(
+   client: WeKnoraClient,
+   serviceId: string,
+   pollOnce: () => Promise<boolean>,
+   attempts = 40,
+   intervalMs = 1500,
+ ): Promise<boolean> {
+   const result = await client.configuration.mcp.oauth.authorizeUrl(serviceId, {
-         redirectURI: window.location.origin + "/api/v1/mcp-oauth/callback",
+     redirectURI: window.location.origin + "/api/v1/mcp-oauth/callback",
-         frontendRedirect: window.location.href,
+     frontendRedirect: window.location.href,
-       });
+   });
-       const popup = window.open(result.authorizationUrl, "weknora_mcp_oauth", "width=600,height=720");
+   const popup = window.open(result.authorizationUrl, "weknora_mcp_oauth", "width=600,height=720");
+   if (!popup) throw new Error("未能打开授权窗口，请检查浏览器弹窗设置");
+   for (let attempt = 0; attempt < attempts; attempt += 1) {
+     await new Promise((resolve) => window.setTimeout(resolve, intervalMs));
+     if (await pollOnce()) return true;
+     if (popup.closed) break;
+   }
+   return pollOnce();
+ }


─── apps/web/src/settings/PluginsSettingsPanel.tsx:360-363 ───
[bug · medium] 接受升级成功后未失效同安装的工具治理面：upgrade-accept 切换了该安装的已接受快照（工具集、digest、默认启停都可能变化），但 toolPolicy
状态未清——若管理员已展开该安装的「工具治理」，行内仍显示旧快照的策略行，toggleToolPolicyFlag 还会基于旧行的 enabled/requireApproval 取反发起
PUT（对已移除的工具将 404，对新工具不可见）。建议与 setUpgradePreview(null) 一并失效。

          upgradePreview.result.candidateFingerprint,
        );
        setUpgradePreview(null);
+       // 接受升级已切换该安装的已接受快照：同安装展开的工具治理行停留在
+       // 旧快照（toggleToolPolicyFlag 会基于旧行 enabled 取反发 PUT），一并失效。
+       setToolPolicy((prev) => (prev?.installationId === upgradePreview.installationId ? null : prev));
        await refreshInstallations();


─── apps/web/src/settings/PluginsSettingsPanel.tsx:424-425 ───
[bug · medium] 漂移重定基后同样未失效工具治理面：resolve 以当前目录 REBASE 快照（新增工具保守默认停用、可能移除工具），但 toolPolicy
未清——展开中的治理行停留在重定基前的旧快照，与上一条 acceptUpgrade 同款问题。建议在 resolve 成功后一并失效。

        await pluginsApi.resolveDrift(driftView.installationId);
+       // 重定基改写了已接受快照：同安装展开的治理行基于旧快照，一并失效。
+       setToolPolicy((prev) => (prev?.installationId === driftView.installationId ? null : prev));
        const report = await pluginsApi.getDrift(driftView.installationId);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:283-288 ───
[bug · medium] 动作驱动的 refreshInstallations()
直调（confirmInstall/toggleState/acceptUpgradeInstall/runDriftCheck/runDriftResolve 共五处）均未传
isStale：client 切换（跨账号/工作区）后，旧 client 闭包里的动作完成时会用旧 pluginsApi 发起列表 GET 且无陈旧丢弃，迟到响应可在新 effect
重载落地之后覆盖新空间的已安装列表——与 effect 的 cancelled、tools GET 的
toolPolicyEpoch（T19-OCR2-F1）防护不一致，属同一威胁模型未收敛。建议直调处也传入代数比对（或让 refreshInstallations 内部默认绑定 epoch）。

-       // 预览已消费：撤卡并刷新已安装列表（列表将出现该插件与已接受版本）。
        setPreview(null);
-       // OCR R1 F34：安装成功即当前已接受版本变更——残留的升级差异面板针对
-       // 的是旧版本基线，一并撤下。
        setUpgradePreview(null);
-       await refreshInstallations();
+       // 与 effect 的 cancelled / tools GET 的 epoch 同款陈旧防护：动作发起时
+       // 捕获代数，client 已切换（effect 失效过）则丢弃本次刷新。
+       const epoch = toolPolicyEpoch.current;
+       await refreshInstallations(() => epoch === toolPolicyEpoch.current);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:336-338 ───
[bug · low] checkUpgrade 失败分支无条件 setUpgradePreview(null)：当 A 安装的升级差异面板已展开、再对 B 安装点「检查升级」失败时，A
的面板被误撤——错误条已定位到来源安装行（pluginName），面板撤除也应只作用于来源安装行，与注释「错误条定位来源安装行，面板不残留」的意图不符。

-       // 候选不可达/核验失败：错误条定位来源安装行，面板不残留，安装列表不动。
-       // message 存纯文案（渲染层统一「升级预览失败（插件名）：」前缀，不再双拼）。
-       setUpgradePreview(null);
+       // 候选不可达/核验失败：错误条定位来源安装行；仅撤下来源安装行自己的
+       // 差异面板，其他安装已展开的面板不受本次失败影响。
+       setUpgradePreview((prev) => (prev?.installationId === item.installationId ? null : prev));


─── apps/web/src/settings/PluginsSettingsPanel.tsx:231-236 ───
[bug · low] client 切换失效块漏清 error（预览失败横幅）：注释宣称「ALL client-bound 临时态一并失效」，清空了
toolPolicy/preview/confirmError/upgradePreview/upgradeError/driftView，但 error（旧 client
的预览失败文案）未被清除，会悬挂到新账号/新工作区视图直到下一次预览提交。listError 虽会被随后的列表加载覆盖，但一并显式清空更稳妥且与注释宣称一致。

      setToolPolicy(null);
      setPreview(null);
+     setError(null);
      setConfirmError(null);
      setUpgradePreview(null);
      setUpgradeError(null);
      setDriftView(null);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:639-641 ───
[style · low] 嵌套三元表达式（检查清单明确禁止）：installations.length === 0 的分支内又套 listError === null
的三元。可拆为两个并列条件渲染消除嵌套。

-         {installations.length === 0 ? (
-           listError === null ? <Status>暂无已安装插件</Status> : null
-         ) : (
+         {listError === null && installations.length === 0 ? <Status>暂无已安装插件</Status> : null}
+         {installations.length > 0 ? (


─── apps/web/src/settings/PluginsSettingsPanel.tsx:323-325 ───
[style · low] toggleState 末尾 "}  }" 双闭合挤压在一行（函数结束与函数体结束未换行），与文件内其余 async 函数的格式不一致，建议拆行。

      } finally {
        setActionBusyId(null);
-     }  }
+     }
+   }


─── apps/web/src/settings/SettingsPage.tsx:702-707 ───
[maintainability · low] settingsSectionLabel 中 plugins 直译兜底出现两次且返回相同字面量，各自带独立的「待 i18n
迁移」注释——迁移时容易只改一处漏另一处。建议收敛为一个共享常量/局部变量，两处判定共用，迁移点唯一。

-   if (integrationTab === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';
+   const pluginsLabel = locale === 'zh-CN' ? '插件' : 'Plugins';
+   if (integrationTab === 'plugins') return pluginsLabel;
    if (integrationTab) return formatMessage(locale, `integrations.tabs.${integrationTab}`);
-   // Issue #108 T03 — plugins 分区标签直译：packages/i18n 不在本任务文件所有权
-   // 内（无 settings.plugins* key，formatMessage 缺 key 会回显 key 本身），
-   // 待 i18n key 落位后迁入 SECTION_LABEL_KEYS。
-   if (key === 'plugins') return locale === 'zh-CN' ? '插件' : 'Plugins';
+   if (key === 'plugins') return pluginsLabel;


─── examples/plugins/jira-todo-mcp/jira.go:195-197 ───
[bug · high] 分页请求参数键名与 Atlassian 官方契约不符：POST /rest/api/3/search/jql 的请求体分页参数与响应字段同名，均为
`nextPageToken`（该 API 家族不存在 `pageToken` 请求参数；已废弃的 /rest/api/3/search 用的是 startAt）。真实 Jira 会静默忽略未知的
`pageToken` 字段并始终返回第一页 + 新的 nextPageToken——本周事项超过 100 条（maxResults=100）时，循环会把第一页重复抓取至多 10
页，输出大量重复条目并误报 truncated=true，且永远取不到第 2 页及以后；≤100 条时单页即返回空
token，行为正常，因此小数据测试无法暴露。注意两处替身均掩盖了此问题：examples 的 ocr_fix_test.go 假 Jira 按同样错误的 `pageToken`
键断言（钉死了错误契约），internal/modules/plugins/plugintest/fakejira.go 则完全不读分页参数、单页全量返回。建议改为
`nextPageToken`，并同步修正 ocr_fix_test.go 替身的 JSON 键。

  		if pageToken != "" {
- 			payload["pageToken"] = pageToken
+ 			payload["nextPageToken"] = pageToken
  		}


─── examples/plugins/jira-todo-mcp/oauth.go:427-429 ───
[bug · low] handleRegister 对 redirect_uri 仅校验 scheme/host，未拒绝 fragment 与 userinfo（RFC 6749
§3.1.2：redirect endpoint URI MUST NOT include a fragment component；RFC 7591 注册校验同理应拒绝）。携带 fragment 的
URI 会原样入库并在 submitAuthorizeForm 构造 302 时保留（url.URL.String() 在 RawQuery 后追加 #fragment，Location 变为
...?code=...&state=...#orig-fragment）。由于注册→授权→换码三处均为字符串精确匹配，实际影响仅限自注册客户端自身的流程，不构成跨方劫持路径——但本文件自述以 RFC
合规为契约（validateAuthorizationRequest 即引用 §3.1.2.3 精确匹配），示例服务作为行为契约标的应补齐该拒收。建议在注册期单条 URI 校验循环中追加
fragment/userinfo 拒绝。

  		parsed, err := url.Parse(uri)
  		if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
- 			(parsed.Scheme != "http" && parsed.Scheme != "https") {
+ 			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
+ 			parsed.Fragment != "" || parsed.User != nil {
+ 			writeJSON(w, http.StatusBadRequest, map[string]any{
+ 				"error":         "invalid_redirect_uri",
+ 				"error_description": "redirect_uri must be an absolute http(s) URL without fragment or userinfo",
+ 			})
+ 			return
+ 		}


─── internal/application/service/plugin_install_service.go:773-778 ───
[bug · medium] UninstallInstallation 未纳入 upgradeAcceptMutexes 串行化，与
AcceptUpgrade/ResolveDrift/CheckDrift 并发时存在竞态窗口：例如 AcceptUpgrade 在 7a
写入新快照后，并发的卸载级联删除服务行与策略行，AcceptUpgrade 的 7c SetPolicy 随后仍向已删除的 serviceID 写入策略行——SQLite 生产 DSN
未启用外键时（代码注释自认）这些行成为永久孤儿；PostgreSQL 上则因 FK violation 走补偿，补偿回写已删安装行 0 行后向管理员误报 500。CheckDrift 与卸载并发时
UpdateDrift 0 行更新同样被 ErrDriftPersistFailed 误报。建议在函数入口同样 `defer
lockUpgradeAccept(installationID)()`，与其他三个写路径共享同一把 per-installation 互斥锁。

  func (s *pluginService) UninstallInstallation(
  	ctx context.Context,
  	tenantID uint64,
  	installationID string,
  ) error {
+ 	// Serialize against AcceptUpgrade/ResolveDrift/CheckDrift (the shared
+ 	// per-installation mutex): a concurrent accept's 7c SetPolicy must not
+ 	// land policy rows onto a service row this uninstall just cascaded away.
+ 	defer lockUpgradeAccept(installationID)()
  	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)


─── internal/application/service/plugin_install_service.go:736-740 ───
[bug · low] UpdateInstallationState 在 0 行更新时返回 gorm.ErrRecordNotFound（repository 契约），此处被一律包装为
ErrInstallationPersistFailed——handler 将其映射为 500（plugin.go:888-897）。但入口 GetInstallation 已确认行存在，0
行更新实际上只可能来自并发卸载删除了安装行，语义应为 404（ErrInstallationNotFound）。建议像 ConfirmInstallation 的 Step 7 那样区分
ErrRecordNotFound，避免把「行已不存在」误报为服务端故障。

  	flipInstallation := func() error {
  		if err := s.pluginRepo.UpdateInstallationState(ctx, tenantID, installationID, state); err != nil {
+ 			if errors.Is(err, gorm.ErrRecordNotFound) {
+ 				// A zero-row update after GetInstallation confirmed existence
+ 				// means a concurrent uninstall removed the row — 404, not 5xx.
+ 				return ErrInstallationNotFound
+ 			}
  			logger.GetLogger(ctx).Errorf("failed to update plugin installation state: %v", err)
  			return ErrInstallationPersistFailed
  		}


─── internal/application/service/plugin_install_service.go:899-901 ───
[maintainability · low] pluginService 对 mcpServiceRepo 的 nil 防护不一致：serviceIDByInstallation 在
repo==nil 时显式报错（并注释援引 fail-loudly 惯例），GetMyConnectionStatus 对 oauthRepo 同样有 nil 检查；但
installationResult（此处）、SetInstallationState 的 syncService、AcceptUpgrade/ResolveDrift 的
GetByID、resolveInstallationPolicyStore 在 repo==nil 且 ServiceID != "" 时会直接 panic。生产经 dig
注入无虞，但一旦装配遗漏（注释自认存在 wiring-less 单测切片），将以 nil 解引用崩溃而非哨兵错误暴露，与代码自身声明的一致性惯例相悖。建议在这些入口统一 nil 防护（或明确记录
panic 即约定的 fail-loudly 语义并删除 serviceIDByInstallation 的特殊分支）。

  	enabledByTool := map[string]*bool{}
  	if inst.ServiceID != "" {
+ 		if s.mcpServiceRepo == nil {
+ 			logger.GetLogger(ctx).Errorf("plugin service store is not wired")
+ 			return nil, ErrInstallationPersistFailed
+ 		}
  		if svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID); err == nil && svc != nil {


─── internal/application/service/plugin_install_service.go:1032-1035 ───
[maintainability · low] AuthorizeURLPath/RevokePath 以字符串拼接硬编码
"/api/v1/mcp-services/{id}/oauth/authorize-url" 与 "/oauth/token"，与路由注册（routes_infra.go 的 mcpServices
group：POST /:id/oauth/authorize-url、DELETE /:id/oauth/token）无单一事实来源。当前前缀恰好一致，但路由前缀或 OAuth
路径一旦调整，成员连接视图将静默给出失效的授权/撤销入口（前端按此拼接发请求得到 404）。建议提取为共享常量（路由注册处与插件域共同引用），并注意 AuthorizeURL 对应
POST、Revoke 对应 DELETE——若消费端不知道方法，路径本身不足以发起正确请求。

  	if inst.ServiceID != "" {
- 		conn.AuthorizeURLPath = "/api/v1/mcp-services/" + inst.ServiceID + "/oauth/authorize-url"
- 		conn.RevokePath = "/api/v1/mcp-services/" + inst.ServiceID + "/oauth/token"
+ 		// Keep in sync with routes_infra.go's mcpServices group — ideally
+ 		// hoist these prefixes into a shared constant both sides reference.
+ 		conn.AuthorizeURLPath = mcpOAuthAuthorizeURLPathPrefix + inst.ServiceID + "/oauth/authorize-url"
+ 		conn.RevokePath = mcpOAuthAuthorizeURLPathPrefix + inst.ServiceID + "/oauth/token"
  	}


─── internal/types/interfaces/plugin.go:70-75 ───
[documentation · low] 接口契约与生产实现不一致：接口文档只承诺 HardDeleteServiceCascade 硬删物化 mcp_services
行「及其派生的逐工具策略行（mcp_tool_approvals）」，但生产实现（internal/application/repository/plugin.go）在同一事务中还会硬删
mcp_oauth_tokens（成员个人令牌，AES-256-GCM 静态加密）、mcp_oauth_clients 以及 PostgreSQL 上的
mcp_metadata——实现侧注释明确指出缺少这轮清扫会让敏感凭据行永久滞留在已死服务上。该接口是被独立实现/模拟的契约（本文件注释即强调存在 in-tree test fakes）：仅按文档实现
HardDeleteServiceCascade 的 fake 会保留凭据行，导致针对 fake 的卸载/补偿测试验证到与生产语义相悖的行为（生产删凭据、fake
不删），测试通过但真实泄漏面未被覆盖。建议在接口文档中完整枚举清扫范围（approvals + oauth tokens/clients +
mcp_metadata），使替代实现与测试夹具与生产语义对齐。

- // HardDeleteServiceCascade HARD-deletes a plugin-materialized MCP
+ 	// HardDeleteServiceCascade HARD-deletes a plugin-materialized MCP
- 	// service row AND its derived per-tool approval rows (OCR round-1 R12
- 	// F21): the approvals FK only cascades on hard DELETE, so the shared
- 	// MCPServiceRepository.Delete (soft) would orphan policy rows keyed to
- 	// a dead serviceID. Used by the confirm compensation and by uninstall.
+ 	// service row AND EVERY service_id-keyed derived row (OCR round-1 R12
+ 	// F21): mcp_tool_approvals (the approvals FK only cascades on hard
+ 	// DELETE, so the shared MCPServiceRepository.Delete (soft) would orphan
+ 	// policy rows keyed to a dead serviceID), mcp_oauth_tokens and
+ 	// mcp_oauth_clients (member personal OAuth credentials must not outlive
+ 	// their service), plus mcp_metadata where the dialect provides it.
+ 	// Implementations MUST sweep all of these — residual credential rows on
+ 	// a dead service are unrecoverable orphans. Used by the confirm
+ 	// compensation and by uninstall.
  	HardDeleteServiceCascade(ctx context.Context, tenantID uint64, serviceID string) error


─── packages/api-client/src/plugins.ts:210-212 ───
[maintainability · low] installationId/previewId/toolName/fingerprint 的「trim + 空串抛错」校验在本文件重复了 12
次，文案与逻辑完全一致——建议提取 requireNonEmpty(value, label) 辅助函数，避免后续调整校验文案时需要改 12 处。

      async getInstallation(installationId: string, signal?: AbortSignal): Promise<PluginInstallation> {
-       const id = installationId.trim();
-       if (id === '') throw new Error('installationId must not be empty');
+       const id = requireNonEmpty(installationId, 'installationId');
+ // ---- 文件顶部辅助：----
+ // function requireNonEmpty(value: string, label: string): string {
+ //   const trimmed = value.trim();
+ //   if (trimmed === '') throw new Error(`${label} must not be empty`);
+ //   return trimmed;
+ // }


─── packages/views/src/integrations/registry.ts:27-27 ───
[documentation · low] operations: ['manage'] 与紧邻注释「成员只读发现面、治理动作留在管理侧设置面板，never
here」语义相悖。已核实该字段当前无运行时消费者（page.tsx tab 条只读 item.key 渲染文案；settings registry 映射 integration-* 时将
operations 重写为 ['read']），故无功能性影响；但作为对外导出的元数据（IntegrationSection 经 packages/views/src/index.ts
导出），未来任何按 operations 渲染动作或鉴权的消费方都会据此认为 viewer 可在该入口执行 manage 级操作。registry.test.ts 有意断言
includes('manage')，若取值有明确理由（如把个人授权/撤销视为 per-user manage），建议在注释中说明，否则与只读口径对齐。



─── scripts/lib/node-gte26.mjs:136-141 ───
[bug · low] shimEnv 只覆写 find() 命中的第一个大小写变体键：当环境中同时存在 `PATH` 与 `Path`（POSIX
上父进程显式注入双键时可能出现，测试注释也承认该形态可在大小写敏感平台共存）时，未命中的那个旧值键会原样传给子进程。POSIX 子进程按大小写敏感读 `PATH`，若 find() 先命中
`Path`，子进程拿到的仍是旧 PATH，shim 静默失效 —— 与 R1-F30 要消除的故障完全同型。建议先删除全部大小写变体键，再写入唯一键；顺带增加 `baseEnv`
形参让测试无需改动全局 process.env（与测试文件的两条 shimEnv 用例配套）。

- export function shimEnv(shimDir) {
-   const env = { ...process.env };
+ export function shimEnv(shimDir, baseEnv = process.env) {
+   const env = { ...baseEnv };
    const pathKey = Object.keys(env).find((key) => key.toUpperCase() === "PATH") ?? "PATH";
-   env[pathKey] = joinShimPath(shimDir, env[pathKey]);
+   const oldValue = env[pathKey];
+   for (const key of Object.keys(env)) {
+     if (key.toUpperCase() === "PATH") delete env[key];
+   }
+   env[pathKey] = joinShimPath(shimDir, oldValue);
    return env;
  }


─── scripts/lib/node-gte26.mjs:107-110 ───
[bug · low] 复制回退的错误路径会泄漏 mkdtempSync 创建的私有目录：若 copyFileSync 抛错（磁盘满、杀软临时锁定等），异常在 createNodeShim
返回前上抛，此时调用方尚未拿到 shimDir，removeNodeShim / installShimSignalCleanup 都还没注册，目录残留。建议 copy 失败时先
removeNodeShim(shimDir) 再原样上抛、快速失败且不留垃圾。另注（可选项）：win32 无开发者模式时，三个被包裹的测试入口每次运行都要整份复制 node.exe（数十
MB），可考虑按目标二进制（path+size+mtime）键控的稳定缓存目录摊销，或至少在注释中把成本写明为已知取舍。

      console.warn(
        `[node-gte26] symlink denied (${cause && cause.code ? cause.code : cause}) — falling back to copying ${targetBin}`,
      );
+     try {
-     fs.copyFileSync(targetBin, shimNode);
+       fs.copyFileSync(targetBin, shimNode);
+     } catch (copyError) {
+       removeNodeShim(shimDir);
+       throw copyError;
+     }


─── scripts/lib/node-gte26.test.mjs:66-67 ───
[test · medium] 该用例在 win32 上必然假红：Windows 的 process.env 赋值与 delete 均为大小写不敏感 —— `process.env.Path =
"/legacy/value"` 会覆写既有的 PATH（保留原键名），随后的 `delete process.env.PATH` 直接把该变量整个删除。于是 shimEnv 看不到任何已存在的
PATH 变体，joinShimPath 过滤 undefined 后只剩 shimDir 本身，`startsWith(shimDir + path.delimiter)` 与
`includes("/legacy/value")` 两条断言都会失败。讽刺的是这个用例正是要钉住 R1-F30/F33 的 win32 回归，却在 win32 上无法运行（CI 是
ubuntu，发现不了）。建议二选一：给该用例加 `skip: process.platform === "win32"`；更优做法是给 shimEnv 增加 `baseEnv =
process.env` 参数，用普通对象字面量 `{ Path: "/legacy/value" }` 模拟重复键 —— 平台无关、确定性强，还顺带免除对全局 process.env 的改动/恢复。

-   process.env.Path = "/legacy/value";
-   delete process.env.PATH;
+   // process.env 在 win32 上大小写不敏感，无法真实模拟双键共存：
+   // 改用注入 baseEnv 的普通对象（配合 shimEnv(shimDir, baseEnv) 签名）
+   const env = shimEnv(shimDir, { Path: "/legacy/value" });


─── scripts/lib/node-gte26.test.mjs:87-88 ───
[test · medium] 断言直接读 `env.PATH` 在 win32 上是机器相关的：Windows 环境块中的键名常见为 `Path`，对 `process.env.PATH =
...` 的大小写不敏感写入会保留原键名 `Path`，`{...process.env}` 拷贝出的对象里没有 `PATH` 键，`env.PATH` 为
undefined，严格相等断言失败（若机器键名恰为 `PATH` 则通过 —— 不稳定）。建议沿用上一用例的大小写无关键查找来取值断言。

    const env = shimEnv(shimDir);
-   assert.equal(env.PATH, [shimDir, "/a", "/b"].join(path.delimiter));
+   const pathKeys = Object.keys(env).filter((key) => key.toUpperCase() === "PATH");
+   assert.equal(pathKeys.length, 1);
+   assert.equal(env[pathKeys[0]], [shimDir, "/a", "/b"].join(path.delimiter));


─── scripts/lib/node-gte26.test.mjs:96-96 ───
[test · low] 该发现路径用例整体在 win32 skip（fake node 是 sh 脚本，win32 无法直接 execFileSync），而 F27 修复的
node.exe/$PATH 扫描恰恰是 win32 分支；CI 只有 ubuntu，Windows 相关回归永远没有自动兜底，只能靠开发者手工运行 —— 与本 PR「win32
修复轮」的目标相悖。建议：当运行测试的解释器本身已 >=26（CI 即如此）时，将 override 指向 process.execPath
而非跳过，这样同一用例在所有平台都能实跑；解释器低于下限时才退回 skip。

- test("findNodeGte26 honors $WEKNORA_NODE_BIN pointing at a >=26 binary", { skip: process.platform === "win32" }, (t) => {
+ test("findNodeGte26 honors $WEKNORA_NODE_BIN pointing at a >=26 binary", {
+   // win32 cannot exec a shell-script fake binary; when this suite already
+   // runs under node >= 26, point the override at process.execPath instead.
+   skip: process.platform === "win32" && majorOf(process.version) < MIN_MAJOR,
+ }, (t) => {


─── scripts/run-with-node-gte26.mjs:81-85 ───
[maintainability · low] shell: true 分支下 spawnSync 不会为参数加引号，args 按空格原样拼接进 cmd.exe 命令行。当前参数全部来自
package.json 固定的无空格 glob，是安全的，但这一约束只存在于注释里：将来任何含空格或 cmd 元字符（& ^ | %
!）的参数都会被拆断甚至改变命令语义（例如新增带空格的测试路径），且失败形态隐蔽。建议把隐式约束变成显式契约：win32
分支对含空白/元字符的参数直接拒绝并报错退出，或拼接时做显式双引号包裹（转义内嵌引号）。

+   if (process.platform === "win32" && args.some((arg) => /[\s&^|()%!"]/u.test(arg))) {
+     console.error("[with-node] refusing shell dispatch: argument contains whitespace/cmd metacharacters");
+     process.exit(2);
+   }
    const result = spawnSync(command, args, {
      stdio: "inherit",
      env,
      shell: process.platform === "win32",
    });


─── internal/modules/plugins/manifest.go:446-451 ───
[security · medium] 凭据掩码对协议相对（无 scheme）URL 失效：maskEndpointCredentials 与 userinfoOf 都以 "://" 定位
authority，对 "//user:pass@host:badport/" 这类以 "//" 开头的畸形 URL，rest 首字符即 '/'，authority
被切为空串，LastIndex("", "@") = -1，两函数分别原样返回 rawURL 与 ""。而该输入恰好会令 url.Parse 失败（invalid
port），走的就是本掩码专门防御的路径（parse 失败先于 userinfo 拒绝触发，OCR round-1 R12 F03）：随后 echoQuoted(masked) 把
"//user:pass@host:badport/" 连同凭据明文回显进 admin 可见的 400 错误体（echoQuoted 仅截到 64 rune，凭据仍完整可见）。建议：无 "://"
时识别前导 "//" 作为 authority 起点（userinfoOf 与 maskEndpointCredentials 需同步修改），或当原文含 "@" 而掩码判定为无 userinfo
时降级为完全不回显 endpoint。

  func maskEndpointCredentials(rawURL string) string {
- 	schemeEnd := strings.Index(rawURL, "://")
  	prefix, rest := "", rawURL
- 	if schemeEnd >= 0 {
+ 	if schemeEnd := strings.Index(rawURL, "://"); schemeEnd >= 0 {
  		prefix, rest = rawURL[:schemeEnd+3], rawURL[schemeEnd+3:]
+ 	} else if strings.HasPrefix(rawURL, "//") {
+ 		// 协议相对形式同样承载 authority——否则 "//user:pass@host:bad/"
+ 		// 在 url.Parse 失败路径上绕过掩码，凭据被原样回显。
+ 		prefix, rest = "//", rawURL[2:]
  	}


─── internal/modules/plugins/manifest.go:263-265 ───
[maintainability · low] '/' 与 '%' 拒绝规则（OCR R1 F16）的依据——/tools/:tool_name/policy 单个 URL
路径段可达性——只适用于工具名，但该检查加在 ValidateManifest 对 name、author 也共用的 validateName 里："CI/CD
Helper"、"Jira/Confluence 桥接"、作者 "100% OSS" 这类合法展示字段将被确定性拒绝，且错误文案仍在解释 policy endpoint
寻址，对被拒的插件作者有误导性。建议将 '/'/'%' 检查限定在工具名校验路径（拆出 validateToolName 或给 validateName 加仅工具名使用的参数），插件
name/author 只保留 Unicode 卫生检查。



─── internal/modules/plugins/plugintest/fakejira.go:144-153 ───
[test · low] SetMyselfDelay 违反本文件自述的 OCR R1 F13 约定：DenySearch/InvalidateAccount 均接受 testing.TB 且
email 未命中即 Fatal（防止注入静默失效导致下游断言难定位），而 SetMyselfDelay 既不接收 testing.TB 也对未命中静默
no-op——凭据/邮箱拼写不一致时延迟注入失效，"慢验证不得串行化替身其他链路"（T13-OCR1-F4）类回归测试失败时难定位。建议与同组注入方法对齐（当前唯一调用方
jira_e2e_integration_test.go:499 传入的是已登记 email，可同步更新签名）。

- func (j *FakeJira) SetMyselfDelay(email string, d time.Duration) {
+ func (j *FakeJira) SetMyselfDelay(t testing.TB, email string, d time.Duration) {
+ 	t.Helper()
  	j.mu.Lock()
  	defer j.mu.Unlock()
  	for _, account := range j.accounts {
  		if account.Email == email {
  			account.MyselfDelay = d
  			return
  		}
  	}
+ 	t.Fatalf("fakejira: SetMyselfDelay: no account registered for %s — the injection would silently no-op", email)
  }

