Review complete: 3 finding(s) across 9 selected item(s).

─── packages/contracts/src/index.ts:179-179 ───
[bug · high] 本次将 CraftExportConsentView 与 parseCraftExportConsentView 首次从根索引导出为公共契约面，但该冻结面消费的是嵌套形状
{manifest: {...}, decision: ...}（packages/contracts/src/craft/web-artifact.ts L93-L98，其单测亦喂嵌套 raw），而
T13 服务端 craftExportConsentBody（internal/handler/session/craft_export_consent.go）实际返回扁平 wire：顶层
version_id / manifest_digest / state / restricted_derived / files / decision，并无 manifest
嵌套。下游若按命名直觉用 parseCraftExportConsentView 解析 exportConsent 端点响应，会因 v.manifest 缺失直接抛 'invalid export
manifest'。建议随本次 T13 契约面落地同步修正冻结面为与服务端一致的扁平形状（补
state、restricted_derived），或暂缓导出该解析器，避免下游把"已从根索引导出"当作已对齐真实 wire 的保证。



─── packages/views/src/craft/export.tsx:66-67 ───
[maintainability · medium] projectExportConsentView 与 contracts 侧 parseCraftExportConsentView
是对同一端点响应的两套解析器，且形状已经分叉（契约侧为嵌套 {manifest, decision}，本投影为含 state/restricted_derived
的扁平形状）；契约解析器已随本次变更从 @weknora/contracts 根索引公共导出，两套校验规则会随契约演进各自漂移，出现"契约解析通过但视图丢弃"（或反向）的隐性分歧，且与本文件及
api-client 注释中 "single fail-closed projection" 的说法不符。建议：修正冻结契约面为真实 wire 形状后，本投影复用
parseCraftExportConsentView 完成形状校验，仅叠加本层独有的绑定守卫（version_id + manifest_digest）与 effectiveStatus
降级规则；若刻意保留双轨，请在注释中说明与契约解析器的形状差异及不复用的原因。另：`CraftExportDecision = CraftExportDecisionContract & {
decision: CraftDecisionStatus }` 的交集并未实际收窄（'unknown' 已在冻结词表 CRAFT_DECISIONS 中），注释所称 "adds the
'unknown' parse residual" 未兑现，属冗余声明，可直接引用契约类型。



─── packages/views/src/craft/export.tsx:22-24 ───
[maintainability · low] 这里的类型交叉是无效操作：`CraftExportDecisionContract` 的 `decision` 字段已经是
`CraftDecisionStatus`，而 `CraftExportDecisionKind` 恰好就是 `CraftDecisionStatus` 的别名，`A & { decision: X
}` 在 A 本就含 `decision: X` 时结果仍为 A 本身。注释声称 "the panel only adds the 'unknown' parse residual on top of
it"，但交叉实际上什么都没有添加——'unknown' 已在契约的 CRAFT_DECISIONS
里。这会误导读者以为视图层决定类型与契约类型存在差异（并在下次契约演进时基于此假设做扩展）。建议直接使用契约类型别名，或删掉这段注释。

- // The decision shape is the FROZEN @weknora/contracts export; the panel
- // only adds the 'unknown' parse residual on top of it.
- export type CraftExportDecision = CraftExportDecisionContract & { decision: CraftExportDecisionKind };
+ // The decision shape is the FROZEN @weknora/contracts export verbatim —
+ // 'unknown' is already part of CRAFT_DECISIONS, nothing is widened here.
+ export type CraftExportDecision = CraftExportDecisionContract;

