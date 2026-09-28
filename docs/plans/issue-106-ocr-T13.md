Review complete: 4 finding(s) across 4 selected item(s).

─── internal/modules/plugins/plugintest/server.go:526-540 ───
[maintainability · low] 行为差异(非阻塞):重构后错误优先级被调换。旧代码先查 state 再比对 users——未知/已消费 state 无论凭据如何都返回 "unknown
or already-consumed state";新代码在 check==nil 且 username 不在 users 表时先返回 "invalid credentials",即使 state
本身也是未知/已消费的。虽然两条路径都映射为 handleAuthorize 的 401、当前也没有测试断言该组合的错误文案,但这个角落差异未被注释说明,未来若有测试用「未知用户名 + 已消费
state」验证 state 一次性语义,会看到误导性的错误。建议把 state 检查移回锁内 users 查找之前(仍是一次 map 读,零成本),恢复与旧版一致的优先级。

  	_, ok := o.pendingAuths[state]
+ 	if !ok {
+ 		o.mu.Unlock()
+ 		return "", fmt.Errorf("unknown or already-consumed state")
+ 	}
  	check := o.credentialCheck
  	var expected string
  	if check == nil {
  		var known bool
  		expected, known = o.users[username]
  		if !known {
  			o.mu.Unlock()
  			return "", fmt.Errorf("invalid credentials")
  		}
  	}
  	o.mu.Unlock()
- 	if !ok {
- 		return "", fmt.Errorf("unknown or already-consumed state")
- 	}


─── packages/api-client/src/plugins.ts:338-341 ───
[maintainability · low] inputSchemaDigest 字段被解析器完整读入（row.input_schema_digest），但 PluginsSettingsPanel
的 DiffToolTable 与 snapshotSummary 均未渲染该摘要——变更工具行只展示「schema 变更」徽标，管理员无法在预览面板对照两版摘要来核验 schema 差异（验收条件
1/2 的 schema 核验可视化只落到了布尔徽标上）。该字段因此构成「解析后从未消费」的悬空契约。建议二选一：在接口注释中明确标注该字段为后续「接受升级」切片预留（当前 T15 面板刻意只渲染
schemaChanged 徽标），消除死字段疑惑；或在 snapshotSummary 中并列展示 current/candidate 摘要供管理员核验。

  export interface PluginUpgradeToolSnapshot {
    readonly name: string;
    readonly description: string;
+   // Digest only (never schema text). Reserved for the follow-up accept-upgrade
+   // slice — the T15 diff panel intentionally renders only the schemaChanged
+   // badge, not the digest pair.
    readonly inputSchemaDigest: string;


─── packages/api-client/src/plugins.ts:424-426 ───
[maintainability · low] changed_tools 分支中
`${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}]` 路径模板连续重复拼接 7
次（name/schema_changed/scope_changed/read_write_class_changed/personal_auth_changed/current/candidate
），键名与路径前缀分离，后续增删字段需同步多处、易错。建议提取局部 base 变量统一拼接，一次定义、七处复用。

        changedTools: (rawDiff.changed_tools as unknown[]).map((item, index) => {
-         const row = record(item, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}]`);
+         const base = `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}]`;
+         const row = record(item, base);
          return {
+           name: required(row.name, `${base}.name`),
+           schemaChanged: flag(row.schema_changed, `${base}.schema_changed`),
+           // ... 其余字段同样以 `${base}.xxx` 拼接
+         };


─── apps/web/src/settings/PluginsSettingsPanel.tsx:54-58 ───
[maintainability · low] UpgradePreviewState.installationId 是只写不读的死状态：checkUpgrade
成功时写入（setUpgradePreview({ installationId: item.installationId, ... })），但整个组件的渲染与逻辑（面板
JSX、checkUpgrade 守卫）从未读取 upgradePreview.installationId——注释所称「重开/切换互斥渲染」实际由单一可空 state
槽位天然实现，与该字段无关。若是为 T16 接受升级预留（accept 请求需要 installationId），建议在类型注释中明确标注预留意图，否则删除该字段直到消费方落地，避免悬空契约（与
inputSchemaDigest 未消费问题同模式）。


