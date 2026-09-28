Review complete: 22 finding(s) across 44 selected item(s).

─── packages/views/src/settings/registry.ts:42-43 ───
[bug · high] 新增的 settings 分区 key `'plugins'` 与本次同时新增的 integration key
`'plugins'`（packages/views/src/integrations/registry.ts）发生裸 key
冲突：`normalizeIntegrationSettingsSection`（packages/views/src/integrations/settings-route.ts:13）会把任何命中
INTEGRATION_SECTIONS 的裸 section 改写为 `integration-*`。实际后果：管理员点击侧边栏「插件」时 `select('plugins')` 将 URL
replace 为 `/platform/settings?section=plugins`（SettingsPage.tsx:378），此后一旦刷新页面或分享该链接，抽屉重挂载时
`requestedSection` → `integrationSettingsQuery` 会把 `section=plugins` 归一化为
`integration-plugins`，落到成员只读发现 tab（integrations
分组的「插件」）而不是本管理面板——管理面板深链被劫持，且侧边栏出现两个同名「插件」项指向不同分区。建议在 normalizer 中对裸 key 改写前先排除已注册的 settings
分区（settings-route.ts 引入 settingsSection 判定不构成循环依赖：settings/registry → integrations/registry
单向），或重命名其中一个 key 消除歧义。

-   // Issue #108 T03 — 插件清单预览分区（管理员粘贴清单 URL 核验预览；安装确认 T08）。
-   { key: 'plugins', viewId: 'PluginSettings', apiDomain: 'plugins', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'], ported: true },
+ // settings-route.ts 修正方向（裸 key 同时是 settings 分区时不改写为 integration-*）：
+ // import { settingsSection } from '../settings/registry.ts';
+ // export function normalizeIntegrationSettingsSection(section: string, tab?: string | null): string {
+ //   if (section === 'integrations') { /* 原逻辑 */ }
+ //   return !settingsSection(section) && INTEGRATION_SECTIONS.some((item) => item.key === section)
+ //     ? `integration-${section}` : section;
+ // }


─── packages/views/src/integrations/registry.ts:27-27 ───
[maintainability · low] 声明 `operations: ['manage']`
与紧邻注释自相矛盾：注释明确「治理动作只留在管理员设置面板，绝不在本面」，本面是成员只读发现面（无任何渠道/资源管理动作）。operations 当前虽无运行时消费方，但它是 appconnector
的契约元数据（im/embed 的 ['manage'] 均对应真实的管理面板动作）；未来若有按 operations 渲染动作按钮或做操作门控的实现，plugins tab
会被据此挂上不该有的管理入口。建议让声明与实际能力一致（或在注释中说明为何必须声明 manage）。



─── internal/modules/plugins/fetcher.go:105-107 ───
[security · medium] 新增分支 `invalid manifest URL: %w` 直接透传 *url.Error——其 Error() 文本为 `parse
"<完整原始URL>": ...`,会原样内嵌管理员提交的 URL。当提交形如 https://user:pass@host:badport/manifest.json(带凭据且畸形,parse 在
userinfo 检查之前失败)时,凭据明文进入错误串,与紧邻注释声明的纪律("The message does not echo the URL, which contains the
credentials (OCR T01-R3-F2)")直接矛盾。service 层 validateManifestURLInput 目前以固定消息前置拦截了此路径,但其注释也明确承认这是在补
fetcher.go 该分支的洞;FetchAndVerify 是导出函数且已被 plugin_install_service 的 confirm/upgrade/drift
路径直接调用,任何新调用点绕过 service 前置校验即触发凭据泄露到管理员可见的 400 响应体(handler default 分支回显 err.Error())与服务端日志。同一 PR 在
manifest.go 中对 transport endpoint 的完全相同失败模式已建立处理范式(剥离 *url.Error wrapper + maskEndpointCredentials +
echoQuoted 有界回显),建议此处对齐,最简修复为不回显细节的固定消息。

  	if u, err := url.Parse(manifestURL); err != nil {
- 		return nil, fmt.Errorf("invalid manifest URL: %w", err)
+ 		// Never pass *url.Error through: its message embeds the FULL
+ 		// original URL verbatim, so a malformed URL that also carries
+ 		// userinfo would leak the credentials (see manifest.go's handling
+ 		// of the same failure shape on the transport endpoint).
+ 		return nil, fmt.Errorf("invalid manifest URL (no details echoed)")
  	} else if u.User != nil {


─── scripts/lib/node-gte26.mjs:25-28 ───
[bug · low] candidateBin 的版本探测没有任何超时保护：findNodeGte26 会按序对 $PATH 的每个条目逐一 execFileSync 探测（本 diff 新增的
PATH 扫描），若 PATH 中存在一个同名但挂起/交互式的 node（损坏的安装、包装脚本等），包装器与 gates 将无限阻塞且无任何诊断输出——本地 node<26 触发 discovery
时 test:shared 直接卡死。CI 侧因 node 26 短路 discovery 且 job 有 timeout-minutes 兜底不受影响，故降级为 low，但建议给探测加
timeout（超时抛出的异常会被现有 catch 吞掉并继续探测下一个候选，行为安全）。

  function candidateBin(nodeBin) {
    try {
-     const v = execFileSync(nodeBin, ["-v"], { encoding: "utf8" });
+     const v = execFileSync(nodeBin, ["-v"], { encoding: "utf8", timeout: 5000 });
      return majorOf(v) >= MIN_MAJOR ? { bin: nodeBin, version: v.trim() } : null;


─── scripts/run-with-node-gte26.mjs:70-71 ───
[performance · low] 当前 node 已 >=26 时仍无条件 createNodeShim(process.execPath)：常见调用路径（pnpm test:shared →
node scripts/run-with-node-gte26.mjs）下 process.execPath 本身就是 PATH 解析出的 node，子进程会解析到同一二进制，shim
只是冗余保险；但在 win32 无开发者模式/非管理员下 symlinkSync 抛 EPERM，回退为整份复制 node.exe（数十 MB）到 tmp——Windows 开发者每执行一次
test:shared / test:craft:shared / test:mobile-v2 都触发一次大文件复制+删除，纯属持续开销（该回退是 T01-OCR1-F12
为修复崩溃有意加的，正确性没问题，问题在于本可避免）。建议短路：当 PATH 解析出的 `node` 主版本已 >=26 时跳过 shim，直接以原 env 运行。

-   const { shimDir } = createNodeShim(target.bin);
+   // Short-circuit: when the PATH-resolved `node` is already >=26, children
+   // resolve it identically — skipping the shim avoids the win32 EPERM
+   // fallback copying the entire node.exe to tmp on every run.
+   const probed = spawnSync(shimEntryName(), ["-v"], { encoding: "utf8", timeout: 5000 });
+   const pathNodeOk = probed.status === 0 && majorOf(probed.stdout.trim()) >= MIN_MAJOR;
+   let shimDir = null;
+   let env = process.env;
+   if (!pathNodeOk) {
+     ({ shimDir } = createNodeShim(target.bin));
-   installShimSignalCleanup(shimDir);
+     installShimSignalCleanup(shimDir);
+     env = shimEnv(shimDir);
+   }


─── scripts/run-with-node-gte26.mjs:81-85 ───
[security · low] win32 分支以 shell:true 运行且未对 command/args 做任何引号/转义处理：spawnSync 在 shell 模式下将参数按空格拼接交给
cmd.exe，含空格路径或 cmd 元字符（^ & | < > " 等）的参数会被拆分或执行，形成潜在命令拼接错误/注入面。当前调用方仅限 package.json 中的固定
glob（注释也声明了这一约束），故仅为 low；但脚本自我定位为通用包装器（usage 即 `-- <command>`），一旦被其他脚本复用即踩坑，建议对 win32
拼接路径做最基本的引号包裹，或至少在 usage/注释中把"参数不得含 shell 元字符"列为硬性契约。

-   const result = spawnSync(command, args, {
-     stdio: "inherit",
-     env,
-     shell: process.platform === "win32",
-   });
+   const useShell = process.platform === "win32";
+   const quoted = (a) => (/[ \t"^&|<>]/.test(a) ? `"${a.replace(/"/g, '\"')}"` : a);
+   const result = useShell
+     ? spawnSync([command, ...args].map(quoted).join(" "), { stdio: "inherit", env, shell: true })
+     : spawnSync(command, args, { stdio: "inherit", env });


─── scripts/run-with-node-gte26.mjs:10-12 ───
[documentation · low] 注释引用了未提交到仓库的本地工作区文件
.superpowers/sdd/2026-09-23-issue-106/environment.md（注释自身也说明 NOT committed），对其他读者是不可达的死引用，无法据此追溯
createRoot 问题的复现细节。建议删除该本地路径，改为内联一句话概述复现条件（v22 + tsx CJS 下 react-dom createRoot 失败）或指向仓库内的
issue/文档编号。

- * (known v22 tsx/CJS issue; the reproduction notes live in the local
-  * workspace file .superpowers/sdd/2026-09-23-issue-106/environment.md, which
-  * is NOT committed to the repository — OCR round-1 F13). Prefixing this
+  * (known v22 tsx/CJS issue: under node 22 the tsx CJS interop breaks and
+  * unrelated .tsx tests false-red with `createRoot is not a function`).


─── apps/web/src/settings/PluginsSettingsPanel.tsx:209-210 ───
[bug · medium] 陈旧闭包竞态残留窗口：epoch
在函数调用时才读取（`installationsEpoch.current`），而非动作发起时捕获。序列复现：管理员点击停用（toggleState 闭包持有
refreshInstallations_v1）→ setInstallationState 在途时 client 切换 → effect 自增 epoch 并发出新列表请求 →
setInstallationState 返回后调用 `await refreshInstallations()`（v1，旧 pluginsApi）→ 此刻读取的 epoch 已是自增后的新值，与
installationsEpoch.current 相等 → stale() 恒 false → 旧
client/旧空间的列表响应落地并覆盖新视图。checkUpgrade/acceptUpgradeInstall/runDriftCheck/runDriftResolve/confirmInsta
ll 的直调 refreshInstallations 同样暴露。注释声称「直调与 effect 调用同一套陈旧性防护」，但该防护对『切换后才发起的刷新』无效——同文件
toggleToolPolicyView 在 await 前捕获 `const epoch = toolPolicyEpoch.current` 才是正确范式。建议：各动作函数在发起首个请求前捕获
epoch，显式传入 `refreshInstallations(() => epoch !== installationsEpoch.current)`。

-       const epoch = installationsEpoch.current;
-       const stale = isStale ?? (() => epoch !== installationsEpoch.current);
+ // 动作函数内（以 toggleState 为例）：
+ async function toggleState(item: PluginInstallationSummary) {
+   if (!canEdit || actionBusyId !== null) return;
+   setActionBusyId(item.installationId);
+   const epoch = installationsEpoch.current; // 发起时捕获，await 前的确定值
+   try {
+     await pluginsApi.setInstallationState(
+       item.installationId,
+       item.state === "active" ? "disabled" : "active",
+     );
+     await refreshInstallations(() => epoch !== installationsEpoch.current);
+   } catch (cause) { /* ... */ }
+ }


─── apps/web/src/integrations/PluginsPanel.tsx:144-147 ───
[bug · medium] 与 PluginsSettingsPanel.refreshInstallations 同款竞态：epoch 在 loadConnection
调用时读取，防护只覆盖『已发出的请求』，不覆盖『client 切换后才发起的请求』。复现：authorizeConnection 轮询持续最长约 60s，期间 client
切换（connectionEpoch 自增、connections 清空）→ 循环下一轮调用 loadConnection 时读取的是自增后的新 epoch → 旧闭包 pluginsApi 的
getMyConnection 响应回来时 epoch === connectionEpochRef.current → 旧 principal 的授权状态写入新视图，且 requestedRef
已含该 id、无后续刷新自愈。revokeConnection 的刷新调用同样暴露。建议：authorizeConnection/revokeConnection 进入时捕获 epoch 并透传给
loadConnection（或在每次调用前比对 captured epoch !== connectionEpochRef.current 则提前返回）。

-   function loadConnection(installationId: string): Promise<PluginMyConnection | null> {
-     // OCR R2 F02：捕获发起时代数——client 切换（上面 effect 已自增）后旧
-     // principal 的迟到回包/迟到失败直接丢弃，不写进新视图。
+ function loadConnection(installationId: string, callerEpoch?: number): Promise<PluginMyConnection | null> {
+   const epoch = callerEpoch ?? connectionEpochRef.current;
+   // ...响应落地前比对 epoch !== connectionEpochRef.current 则丢弃（现有逻辑不变）
+ }
+ 
+ // authorizeConnection/revokeConnection 进入时捕获并透传：
-     const epoch = connectionEpochRef.current;
+ const epoch = connectionEpochRef.current;
+ const next = await loadConnection(connection.installationId, epoch);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:698-705 ───
[bug · medium] 操作按钮冻结矩阵不对称，并发治理动作存在竞态：①「检查升级」未受 actionBusyId/toolPolicyBusyId/policyToggleBusy
冻结，「工具治理」「停用/启用」也未受 upgradeBusyId/driftView.busy 冻结（checkUpgrade/toggleState 等函数守卫同样各只看自己维度的
busy），管理员可同时触发多类动作，各自的 refreshInstallations 并发回包互相覆盖；②更关键的是 acceptUpgradeInstall 不受 toolPolicyBusyId
冻结：治理 GET .../tools 在途时点击「接受升级」→ 成功后 setToolPolicy(null) 失效治理面 → 旧 GET 迟到回包因 toolPolicyEpoch
未自增而通过校验 → 基于升级前旧快照的策略行复活写回——这正是 OCR R2 F10 注释声称已闭合的窗口（runDriftResolve 的 setToolPolicy(null)
同理）。toggleState 注释宣称「与同组两按钮一致的跨行禁用」，但矩阵并未对齐 PluginsPanel anyPending 的全锁口径。建议：各按钮 disabled
与函数守卫统一为全维度冻结（任一 busy 在途即冻结），并在 acceptUpgrade/runDriftResolve 成功时 toolPolicyEpoch.current += 1
丢弃在途回包。

-                     <Button
-                       type="button"
-                       loading={upgradeBusyId === item.installationId}
-                       disabled={upgradeBusyId !== null && upgradeBusyId !== item.installationId}
-                       onClick={() => void checkUpgrade(item)}
-                     >
-                       检查升级
-                     </Button>
+ // 统一冻结量（render 内计算）：
+ const anyGovernanceBusy = actionBusyId !== null || upgradeBusyId !== null
+   || toolPolicyBusyId !== null || policyToggleBusy || upgradeAcceptBusy
+   || (driftView?.busy ?? null) !== null;
+ 
+ // 各按钮：
+ disabled={anyGovernanceBusy && <本行 busyId> !== item.installationId}
+ 
+ // acceptUpgradeInstall/runDriftResolve 成功分支补：
+ //   toolPolicyEpoch.current += 1; // 丢弃在途治理 GET 的迟到回包


─── apps/web/src/integrations/PluginsPanel.tsx:50-54 ───
[maintainability · low] 死参数：initialInstallations 无任何生产调用方传值——唯一挂载点 IntegrationsRoutePage 仅传
`<PluginsPanel client={client}
/>`，仅测试文件使用（且测试是自造直出数据）。fix-plans.md（.superpowers/sdd/2026-09-23-issue-106/fix-plans.md:1608）已明确认定「P
luginsPanel Props 的 initialInstallations 去掉」。该参数连同 listLoaded 的直出初始化分支（`initialInstallations !==
undefined`）与 effect3 的跳过首跑补偿（OCR R2 F03 注释所防御的恰是这个参数引入的双重请求）共同构成一条不存在的 SSR 直出通路，误导维护者。建议删除该 prop
并简化三处关联逻辑。

  type Props = {
    client: WeKnoraClient;
-   /** SSR/首屏直出数据；挂载后仍会经 client 刷新（GET /plugins/installations，Viewer+）。 */
-   initialInstallations?: readonly PluginInstallationSummary[];
  };


─── apps/web/src/integrations/PluginsPanel.tsx:237-241 ───
[style · low] 嵌套/链式三元表达式违反代码规约「禁止嵌套三目」：此处加载中/空态/列表三级链式条件，以及 MemberConnectionControl 内
connection.state 的两层嵌套三元（authorized/expired/unauthorized 徽标求值）。条件分支建议提前提取为变量或函数（如 `const body =
!listLoaded && error === null ? 加载态 : listBody;` 的两段式，或提取 renderListBody()/connectionBadge()
函数），提高可读性与可测性。

-       {!listLoaded && error === null ? (
-         <Status>加载中…</Status>
-       ) : installations.length === 0 && error === null ? (
-         <Status>暂无已安装插件</Status>
-       ) : (
+ // 提前求值，渲染处单层条件：
+ const emptyList = listLoaded && installations.length === 0 && error === null;
+ // 渲染：
+ {!listLoaded && error === null ? <Status>加载中…</Status> : emptyList ? <Status>暂无已安装插件</Status> : <ul>…</ul>}
+ // 或提取 listBody 变量消解链式条件


─── apps/web/src/settings/PluginsSettingsPanel.tsx:663-665 ───
[style · low] 嵌套三元表达式违反「禁止嵌套三目」规约：`installations.length === 0 ? (listError === null ? 空态 : null) :
列表` 外层三元的两个分支又各自内嵌条件；下方 drift 徽标处 `driftView.report ? (driftView.report.driftState === "detected" ?
… : …) : null` 同为嵌套。建议提取变量/函数消解（如 `const showEmpty = installations.length === 0 && listError ===
null;`）。另两处细节：① toggleState 函数结尾 `}  }` 双花括号连排排版异常（diff 中 `    }  }`），与文件内其余函数的 `}` 换行收尾不一致；②
formatPreviewExpiry 被复用格式化漂移报告的 checkedAt（「上次检查 …」），函数名与导出语义均为「预览有效期」，命名误导后续复用，建议改名
formatTimestamp/formatDateTime 或拆分。

-         {installations.length === 0 ? (
-           listError === null ? <Status>暂无已安装插件</Status> : null
-         ) : (
+ // 消解嵌套三目：
+ {installations.length === 0 && listError === null ? <Status>暂无已安装插件</Status> : installations.length > 0 ? (
+   <ul className="m-0 grid list-none gap-2 p-0">…</ul>
+ ) : null}
+ 
+ // toggleState 结尾排版修正：
+     } finally {
+       setActionBusyId(null);
+     }
+   }


─── apps/web/src/integrations/PluginsPanel.tsx:41-44 ───
[maintainability · low] 跨面板重复代码：pluginBadge* 四个样式常量在 PluginsPanel 与 PluginsSettingsPanel（第 39-42
行）各复制一份（同为 McpSettingsPanel mcpBadge* 的第三份近似副本）；ApiError 识别逻辑（cause instanceof Error && cause.name
=== "ApiError"）在 PluginsPanel 内联两处、PluginsSettingsPanel 另有 apiErrorMessage
helper——同类判定三种写法。后续任一口径调整（如错误分类细化、徽标配色调整）需多点同步，易产生分叉。建议抽取共享模块（如
apps/web/src/plugins/ui-shared.ts：badge 常量 + isApiError/apiErrorMessage），两面板与后续插件域面板统一消费。

- const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
- const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
- const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
- const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";
+ // 新建共享模块（两面板统一 import）：
+ // apps/web/src/plugins/shared.ts
+ export const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
+ export const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
+ export const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
+ export const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";
+ export function apiErrorMessage(cause: unknown): string | null {
+   return cause instanceof Error && cause.name === "ApiError" ? cause.message : null;
+ }


─── internal/application/service/plugin_install_service.go:1295-1300 ───
[security · high] 崩溃窗口 + 幂等短路导致"快照工具缺策略行"的永久 fail-open：AcceptUpgrade 在 7a（快照落库）之后、7c
增量策略循环完成之前被硬杀（SIGKILL/OOM/部署重启），新工具已进入 accepted 快照但没有 mcp_tool_approvals
行。重试时此处幂等条件全部满足（版本/摘要/端点相等、drift 已被 7a 重置为 none、serviceInSync 已成立）→ 零写入提前返回，7c
永不重跑，缺失的策略行永远不补。而运行时门（approval/tool_policy.go 的 enabledToolsIndividually 与
MCPToolApprovalRepository.IsEnabled）对缺失行默认 enabled=true 且无任何插件感知，FilterToolsBySnapshot
也只校验快照成员资格不校验策略——新增写工具将在管理员从未启用的情况下可被发现并派发，直接违反验收边界"新增写工具默认关闭"。ConfirmInstallation 同理：step 5
落安装行后、策略循环中崩溃，重试被 step 3 的 ErrPluginAlreadyInstalled（409）挡回，同样无法补齐。更隐蔽的是治理视图
installationToolPolicies 对缺行显示 Enabled=ReadOnly（写工具"已禁用"），与运行时 fail-open 实际行为相反，管理员在 UI
上看不出异常。建议：幂等短路前校验候选快照的每个工具都已有显式策略行（svc != nil 时），缺失则落入 7c（其本就是增量、只补缺行）；ResolveDrift step 4
同样处理；ConfirmInstallation 对"existing 的 digest 与 preview 一致"的重复确认改为补齐策略行而非 409，或至少让运行时门对插件物化服务缺行
fail-closed。



─── internal/application/service/plugin_install_service.go:689-693 ───
[bug · medium] SetInstallationState 未纳入 upgradeAcceptMutexes 序列化，与 AcceptUpgrade/ResolveDrift
对同一物化服务行构成并发读-改-写竞争：syncService 与 AcceptUpgrade 7b 都是 GetByID 加载 svc → 内存改字段 →
mcpServiceRepo.Update，而共享 Update 会从内存副本写回 url、auth_config、enabled
全部列（repository/mcp_service.go:100-130）。交错后果：(1) disable 的 Update 用 accept 之前读到的旧副本写回，把 accept 刚切换的新
endpoint 和重推导的 OAuth 基线整体回滚——安装行快照指向新端点而服务行停在旧端点，运行时按快照过滤会因 schema 不符对全部工具
fail-closed（ErrPluginDrift），或 schema 碰巧一致时成员被静默服务旧版本；(2) 反向交错中 accept 的 Update 把 disable 之前读到的
enabled=true 写回，重新启用管理员刚停用的安装（fail-open 方向）。本文件对 UninstallInstallation 的注释（OCR R2
F20）已明确列举需共享该锁的写流程，SetInstallationState 是遗漏的一个。建议：在本方法入口同样 defer
lockUpgradeAccept(installationID)()。



─── apps/web/src/integrations/PluginsPanel.tsx:206-206 ───
[bug · low] 兜底刷新缺少卸载守卫，与 OCR R2 F08 的修复意图相悖：轮询循环因 `!aliveRef.current` break 退出后（tab
切换即卸载），`authorized` 为 false，`if (!authorized)` 的兜底 `loadConnection` 仍会执行——卸载后照样发出一次 GET
/connections/me 后台请求（setState 虽 no-op，请求照发）。建议补上与循环内同款的 aliveRef 守卫。

-       if (!authorized) await loadConnection(connection.installationId);
+       if (!authorized && aliveRef.current) await loadConnection(connection.installationId);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:444-445 ───
[bug · medium] runDriftResolve 的 try 块把写操作与后续读取/失效串联，getDrift 失败时会误报并跳过快照失效：若 resolveDrift
已成功（重定基已持久化）而紧随的 getDrift 失败，catch 会显示「漂移重定基失败，已回旧快照」——文案失实（重定基实际已生效，服务端并无回滚）；同时 setToolPolicy(null)
与 refreshInstallations() 被跳过，工具治理面板停留在重定基前的旧快照上（对已移除工具 PUT 吃 404、新工具不可见——正是 OCR R2 F10/F11
描述的危害），列表漂移徽标也不刷新。建议将 resolve 与报告读取分开处理：resolve 失败才用「已回旧快照」文案；resolve 成功后无条件失效 toolPolicy
并刷新列表，报告加载失败单独提示。

        await pluginsApi.resolveDrift(driftView.installationId);
+       // OCR R2 F10/F11：重定基已持久化——失效旧快照治理面并刷新列表，
+       // 不随后续报告读取失败而跳过。
+       setToolPolicy(null);
+       await refreshInstallations();
        const report = await pluginsApi.getDrift(driftView.installationId);
+       setDriftView((prev) =>
+         prev && prev.installationId === driftView.installationId ? { ...prev, busy: null, report, error: null } : prev);


─── scripts/run-gates.mjs:14-14 ───
[documentation · low] 文件头部注释（第 14 行）仍描述旧的发现顺序 "$WEKNORA_NODE_BIN, homebrew node, nvm
versions"，而本次改动后共享模块 scripts/lib/node-gte26.mjs 的发现顺序已扩展为 "$WEKNORA_NODE_BIN → 平台前缀（darwin:
homebrew；Linux: /usr/local/bin、/usr/bin）→ ~/.nvm → 全量 $PATH 扫描"，且下方 main() 中的错误消息也已同步更新为 "platform
prefixes, ~/.nvm, or $PATH"。头部注释与实现及错误消息自相矛盾，会误导后续维护者（尤其是 F09 的 Linux/PATH
扫描正是本次要修的缺口）。建议同步更新头部注释，或直接指向共享模块的文档。

-  * Node >=26 discovery order: $WEKNORA_NODE_BIN, homebrew node, nvm versions.
+  * Node >=26 discovery order: $WEKNORA_NODE_BIN, platform prefixes
+  * (homebrew / /usr/local/bin / /usr/bin), ~/.nvm versions, then every $PATH
+  * entry — see scripts/lib/node-gte26.mjs (single source of truth).


─── internal/modules/plugins/manifest.go:451-458 ───
[bug · low] userinfoOf 只用 '/' 终止 authority，而 net/url 实际在第一个 '/'、'?' 或 '#' 处终止 authority。对进入此分支的畸形
URL（如 `https://goodhost:badport?redirect=user@evil.example.com`——端口非法导致 url.Parse 失败），query 中的 '@'
会被误当成 userinfo 分隔符：userinfoOf 返回 "goodhost:badport?redirect=user@"，maskEndpointCredentials
据此把错误消息里的回显掩码成 `"https://REDACTED@evil.example.com"`——host 与 query 被完全篡改，违反函数自身声明的 "never a wrong
redaction" 契约（管理员看到的是一个与输入完全不同的 URL，还误以为里面带过凭据）。建议与 net/url 对齐，用 IndexAny("/?#") 终止 authority。

  	authority := rest
- 	if slash := strings.Index(rest, "/"); slash >= 0 {
- 		authority = rest[:slash]
+ 	// net/url 在第一个 '/', '?' 或 '#' 处终止 authority；
+ 	// 只切 '/' 会把 query/fragment 里的 '@' 误判为 userinfo 分隔符。
+ 	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
+ 		authority = rest[:end]
  	}
  	if at := strings.LastIndex(authority, "@"); at > 0 {
  		return authority[:at+1]
  	}
  	return ""


─── internal/modules/plugins/manifest.go:474-477 ───
[bug · low] maskEndpointCredentials 的 authority 切分与 userinfoOf 同款缺陷：只按 '/' 截断。畸形 URL（url.Parse
已失败、进入回显分支）如 `https://goodhost:badport?redirect=user@evil.example.com`，LastIndex("@") 命中 query 里的
'@'，掩码结果变成 `https://REDACTED@evil.example.com`——真实 host（goodhost:badport）与 query
全部丢失，错误回显与管理员输入完全不符。应与 userinfoOf 一并改为在第一个 '/', '?', '#' 处切分 authority。

  	authority, tail := rest, ""
- 	if slash := strings.Index(rest, "/"); slash >= 0 {
- 		authority, tail = rest[:slash], rest[slash:]
+ 	// 与 net/url 对齐：authority 在第一个 '/', '?' 或 '#' 处终止。
+ 	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
+ 		authority, tail = rest[:end], rest[end:]
  	}


─── internal/application/service/mcp_service.go:493-500 ───
[security · medium] 新增的 plugin-managed 守卫系列漏掉了资源读取面：同文件的 GetMCPServiceResources（对应 Viewer+ 路由 GET
/mcp-services/:id/resources，routes_infra.go:181）没有等价的 PluginInstallationID 守卫。插件物化服务行是 mcp_services
的普通行，会出现在 Viewer 可见的服务列表里；对它调用该接口会对插件端点发起实时 ListResources，把未经管理员接受核验的远端内容直接暴露给任何
Viewer——这正是本守卫注释（OCR R1 F07）要封堵的「实时目录绕过已接受快照」泄漏类别（agent 侧 FilterToolsBySnapshot 只覆盖 tools，不覆盖
resources）。同理 TestMCPService（POST /:id/test，Admin+）对插件服务也会实时连端点并返回完整 live tools（含 InputSchema
文本），而插件域的设计口径是 schema 文本即使对管理员也只以 digest 呈现。建议在这两个入口补上与 GetMCPServiceTools 相同的守卫；另请顺带确认
mcp_metadata.go 的 RefreshMCPMetadata/GetMCPMetadata（Viewer+）对插件服务行的处理是否也需要对齐（该文件不在本次评审范围内，仅作关联提示）。

- 	// OCR R1 F07: plugin-materialized rows serve their directory through the
- 	// plugin install APIs (accepted snapshot, filtered). The live ListTools
- 	// call below would expose unaccepted capabilities and post-drift schemas
- 	// to any Viewer — the very leak the write faces and the agent runtime
- 	// (FilterToolsBySnapshot) close. Deterministic 409 via the handler.
+ // 在 GetMCPServiceTools 同款守卫之外，为 GetMCPServiceResources（以及 TestMCPService）补齐：
+ 
+ 	// Plugin-materialized rows serve their directory through the plugin
+ 	// install APIs only — a live ListResources would expose unvetted remote
+ 	// content to any Viewer (same F07 posture as GetMCPServiceTools).
  	if service.PluginInstallationID != nil {
  		return nil, ErrPluginManagedService
  	}

