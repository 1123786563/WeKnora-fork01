Review complete: 4 finding(s) across 6 selected item(s).

─── apps/web/src/features/craft/routes.tsx:517-520 ───
[maintainability · low] activeRun 投影表达式（`{ id: run_id, status, waitReason }`）与初始加载 effect（本文件
457-459 行）完全重复：本次改动将该投影从 extendBudget 内联迁移到 refreshWorkbench，但两处合并点仍各自手写同一形状。若未来 workspace 视图的
active_run 增加字段或调整投影（例如新增 waitReason 细分），两处需要同步修改且无编译期约束，易发生字段漂移。建议提取一个共享的小投影函数供初始加载与
refreshWorkbench 共用；同时可将注释中“the same merge the initial load performs”收紧为“仅 active-run
事实与初始加载一致”（初始加载是整体替换，此处是部分合并，字段集不同）。

-       resumed: view.active_run !== null,
-       activeRun: view.active_run === null
+ // 组件外或组件内共享投影，初始加载 effect 与 refreshWorkbench 复用：
+ const toActiveRun = (view: CraftWorkspaceView): { id: string; status: string; waitReason: string } | null =>
+   view.active_run === null
-         ? null
+     ? null
-         : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason },
+     : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason };
+ 
+ // 两处调用点统一为：
+ //   resumed: view.active_run !== null,
+ //   activeRun: toActiveRun(view),


─── apps/web/src/features/craft/routes.tsx:896-898 ───
[documentation · low] 注释中“the same pair enrichedSend and the budget extension perform after they
change run state”表述不准确：预算续期（requestBudgetExtension，538/543 行）确实执行了“重读投影 + 重载控制器”这一对，但 enrichedSend
在提交接纳后仅调用 `controller.load`（本文件 735、804 行），并未重读 workspace 投影（无
craftApi.get/setWorkbenchInfo）。该注释是运行状态刷新路径的维护地图，表述偏差会误导后续维护者对哪条路径维护 workbenchInfo.activeRun
的判断，建议修正措辞为仅预算续期执行完整一对（或说明 enrichedSend 依赖控制器事件流补齐投影）。

-             // workbench subscribes to the new run's event stream — the same
-             // pair enrichedSend and the budget extension perform after they
-             // change run state (best-effort: the panel already answered
+             // workbench subscribes to the new run's event stream — the
+             // refresh+reload pair the budget extension performs after it
+             // changes run state (best-effort: the panel already answered


─── apps/web/src/features/craft/routes.tsx:884-884 ───
[maintainability · low] 此处内层 `canWrite` 遮蔽了组件作用域（第 475 行）的外层 `canWrite`，且二者语义不同：外层是
owner-only（`workbenchInfo.ownerId === currentMeId`，供 993/1051/1059 行的 workbench、输入决策、预算续期使用），此处是
owner 或 collaborator（access 视图的角色）。同名异义的变量遮蔽容易让后续维护者在该闭包内误用/误改错误的语义。建议改用区分性命名，如 `canRequestEdit`。

-         const canWrite = currentMember.role === 'owner' || currentMember.role === 'collaborator';
+         const canRequestEdit = currentMember.role === 'owner' || currentMember.role === 'collaborator';
+         // ... 并同步将 canWrite={canWrite} 改为 canWrite={canRequestEdit}


─── apps/web/src/features/craft/routes.tsx:880-883 ───
[maintainability · low] `currentMember` 的查找 + undefined 兜底逻辑与 'task-access' feature（851-853
行）完全重复：两处各自手写 `currentMeId === null ? undefined : accessState.members.find(...)`。若后续成员行结构或匹配键变化（例如
user_id 改为复合键），两处需同步修改且无编译期约束。建议在组件层提取共享派生（如 `useMemo` 基于 accessState/sessionId/currentMeId 计算
currentMember），两个 feature 的 render 直接消费同一来源。

-         const currentMember = currentMeId === null
-           ? undefined
-           : accessState.members.find((member) => member.user_id === currentMeId);
-         if (currentMember === undefined) return null;
+       // 组件层：const currentMember = useMemo(
+       //   () => accessState.sessionId !== sessionId || accessState.status !== 'ready' || currentMeId === null
+       //     ? undefined
+       //     : accessState.members.find((member) => member.user_id === currentMeId),
+       //   [accessState, sessionId, currentMeId]);
+       // 本 feature 与 task-access feature 的 render 均直接复用该派生值。

