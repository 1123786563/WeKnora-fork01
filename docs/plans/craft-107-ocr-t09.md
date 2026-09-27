Review complete: 5 finding(s) across 4 selected item(s).

─── packages/views/src/craft/workbench-edit.tsx:48-49 ───
[bug · high] 投影契约与线上响应无法对接：实测 PostCraftRun 响应为 {"data": runView(run), "writer_acquisition":
...}，runView（internal/handler/session/agent_run.go:124）只输出 snake_case 的 run_id/session_id/status
等字段，且不含任何发起成员身份（本仓库契约层刻意不上线 actor/owner 身份，见 packages/contracts/src/appconnector.ts 注释）。而本函数强制要求
camelCase 的 runId/actorUserId/writerAcquisition 且 actorUserId 非空，否则整体丢弃。T09 服务端只把发起成员写入审计时间线与
agent_runs.actor_user_id，并未放入 HTTP 响应；面板头注释又明确禁止客户端推导身份（“never ... invents an initiator”）。因此 T20
装配层接线后，任何真实响应都会被判为“服务器响应无效”，面板核心功能（展示实际发起成员）不可满足。建议：① 键名对齐冻结线上命名（run_id / writer_acquisition，与嵌套
workspace_id 一致），消除 camel/snake 混排；② actorUserId
在现有线上契约中无来源，需要么由服务端响应增补（需协同变更），要么将该字段改为非必需/由装配层从真实存在的成员视图单独喂入，并在注释中写明其权威来源。



─── packages/views/src/craft/workbench-edit.tsx:18-20 ───
[maintainability · medium] 本地镜像已过时：本 PR 正是在 @weknora/contracts
根索引新增了该冻结词汇表的再导出（CraftWriterAcquireStatus/CraftWriterAcquireOutcome/CRAFT_WRITER_ACQUIRE_OUTCOMES，取值
['acquired','conflict','unknown'] 与镜像完全一致），views 的 package.json 也已声明 @weknora/contracts
依赖。文件头“central package top-level index does not re-export them yet, so this panel mirrors”的注释在本 PR
内即失真，本地 CRAFT_WRITER_ACQUIRE_STATUSES/CraftWriterAcquireStatus/CraftWriterAcquireOutcome
与包导出同名构成遮蔽，词汇演进时必然漂移。建议直接从包导入并删除镜像块，同步更新头注释。

- const CRAFT_WRITER_ACQUIRE_STATUSES = ['acquired', 'conflict', 'unknown'] as const;
- export type CraftWriterAcquireStatus = (typeof CRAFT_WRITER_ACQUIRE_STATUSES)[number];
- export interface CraftWriterAcquireOutcome { workspace_id: string; status: CraftWriterAcquireStatus }
+ import { CRAFT_WRITER_ACQUIRE_OUTCOMES, type CraftWriterAcquireOutcome, type CraftWriterAcquireStatus } from '@weknora/contracts';
+ // 删除本地 CRAFT_WRITER_ACQUIRE_STATUSES、CraftWriterAcquireStatus、CraftWriterAcquireOutcome 镜像，parseWriterAcquisition 改用 CRAFT_WRITER_ACQUIRE_OUTCOMES 校验。


─── packages/views/src/craft/workbench-edit.tsx:114-114 ───
[bug · medium] writerAcquisition.status === 'unknown'（T00 冻结三态之一，租约状态未知）时无任何提示，UI 呈现与 'acquired'
完全相同（展示 runId+发起成员），用户会误以为写入租约已取得；同时 conflict 分支 setDraft('') 无条件清空草稿，但 conflict
文案明确引导“等待其完成后再试”，用户被迫整段重输 prompt。建议为 'unknown' 增加中性提示（与 conflict 的 alert 区分），并考虑 conflict
时保留草稿以便显式重试（与“a refused request keeps the draft”的体验对齐）。



─── packages/views/src/craft/workbench-edit.tsx:126-126 ───
[style · low] “服务器响应无效/Invalid server response”以内联三元写在 submit 中，是面板内唯一未纳入 EDIT_STRINGS
双语表的用户可见文案，组织方式与其余文案不一致。建议在 EDIT_STRINGS.zh/en 中增加 invalidResponse 键并引用。

-         throw new Error(props.locale === 'zh' ? '服务器响应无效' : 'Invalid server response');
+     invalidResponse: '服务器响应无效',
+     // en: invalidResponse: 'Invalid server response',
+     // submit 内改为： throw new Error(strings.invalidResponse);


─── packages/views/src/craft/workbench-edit.tsx:64-65 ───
[maintainability · medium] 回调类型与面板职责自相矛盾：prop 声明返回已投影的 CraftEditRequestOutcome，但 submit 中又将其作为
unknown 交给 projectEditOutcome 重新校验。若 assembly 遵守该类型，就必须自己完成投影（面板内的投影/丢弃逻辑成为重复死码，且 TS 不会暴露漏校验）；若
assembly 按注释本意返回原始 wire 载荷，则类型注解失真，只能靠 as 强转满足。与文件头"本面板只投影、不推导"的职责声明一致的做法是声明为
Promise<unknown>，由面板持有唯一一次投影与拒绝点。

    /** Sends one serialized-edit request; the assembly owns the API call and the request id. */
-   onRequestEdit(prompt: string): Promise<CraftEditRequestOutcome>;
+   onRequestEdit(prompt: string): Promise<unknown>;

