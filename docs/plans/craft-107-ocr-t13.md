Review complete: 3 finding(s) across 7 selected item(s).

─── internal/modules/craft/export_consent.go:143-145 ───
[bug · medium] ExportConsentStateOf 的 rejected 分支未校验 currentOwner，与函数文档契约矛盾：文档明确承诺 "a decision of a
former owner is history, so after an ownership move the state awaits the new owner's decision
again"，且同函数 approved 路径（经 GrantsExportAuthority 的 d.OwnerID != currentOwner 检查）确实兑现了该承诺——前 owner
的批准正确落到 Awaiting。但 rejected 决策在前 owner 手中时直接返回 Declined：所有权转移后，新 owner 将看到 "declined/已拒绝导出" 而非
"awaiting/等待新 owner 决定"。同时 projectCraftExportConsentView 因 owner 不匹配不会投影这条 decision，前端面板将呈现 declined
状态却无 decidedBy，状态误归因于当前所有权。无安全影响（declined 与 awaiting 在 ConsentGatedExportService
门控下均发安全回退包），属用户可见的状态机语义不一致。建议在 rejected 分支同样校验 owner，使其与前 owner 批准决策的处理对称。

- 	if d.Decision == DecisionRejected {
+ 	if d.Decision == DecisionRejected && currentOwner != "" && d.OwnerID == currentOwner {
  		return ExportConsentDeclined
  	}


─── packages/views/src/craft/export.tsx:187-187 ───
[bug · medium] consented 状态下决策控件被隐藏，导致已生效的同意无法通过该面板撤回。紧邻注释声称 "the owner can decide again at any
time"，服务端 DecideExport 的 upsert 注释也明确 "an approval can be withdrawn by a fresh rejection"，且迁移注释写明
"No TTL by design ... a rejection can overwrite an approval anytime"——即拒绝是终止长期有效同意的唯一手段。但
`canDecide` 仅在 awaiting/declined 时为 true，consented
后按钮消失，且本面板是唯一的决策入口，所有者实际无法在前端撤回同意，受限派生数据将持续可导出，与三处文档化的设计能力相矛盾。建议将 'consented' 也纳入可决策状态（例如 canDecide
= isOwner && view.state !== 'none'），并在该状态下将 decline 按钮呈现为撤回语义。

-   const canDecide = isOwner && (view.state === 'awaiting' || view.state === 'declined');
+   // A live approval stays revocable: the server keeps one decision row
+   // per task+version and a fresh decision upserts over it, so the owner
+   // can decide (and re-decide) at any time — consent included.
+   const canDecide = isOwner && view.state !== 'none';


─── packages/views/src/craft/export.tsx:42-42 ───
[maintainability · low] 本地硬编码了冻结契约的封闭词汇表：originKinds 重复了 @weknora/contracts 已导出的
CRAFT_EXPORT_ORIGIN_KINDS（['knowledge','input','artifact']），exportDecisions 重复了
CRAFT_DECISIONS（['approved','rejected','unknown']），而本文件本就从 '@weknora/contracts' 导入类型。一旦契约/服务端新增
origin kind 或 decision 枚举（T 系列后续票务的常见演进），此处不会同步，fail-closed 投影会直接返回
null，整个同意面板对所有成员静默消失。建议直接复用契约导出的常量（如 `(CRAFT_EXPORT_ORIGIN_KINDS as readonly
string[]).includes(kind)`），消除漂移风险。

- const originKinds: readonly string[] = ['knowledge', 'input', 'artifact'];
+ import { CRAFT_DECISIONS, CRAFT_EXPORT_ORIGIN_KINDS } from '@weknora/contracts';
+ 
+ // (删除本地 originKinds/exportDecisions，校验处改用契约常量)
+ // typeof kind !== 'string' || !(CRAFT_EXPORT_ORIGIN_KINDS as readonly string[]).includes(kind)
+ // exportDecisions.includes(dKind as CraftExportDecisionKind) → (CRAFT_DECISIONS as readonly string[]).includes(dKind)

