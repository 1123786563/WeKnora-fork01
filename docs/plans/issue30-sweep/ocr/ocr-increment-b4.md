Review partially complete: 9 finding(s); 145 of 150 selected item(s) failed.

─── packages/mobile-core/src/task-office/task-detail.ts:355-355 ───
[bug · high] 流式终态路径不触发 flush：`flushQueuedIntents` 目前仅有两个触发点——本行（hydrate 终态分支）与显式调用。但 Run 正常完成的主路径是
SSE 事件先行（`processEvent` 只 persist+notify），随后服务端关流，`streamEnded` 的终态分支只做 `flushPersisted +
notify('drained')`，不再进入 hydrate；apps/mobile 的控制器（task-detail-view.ts）也不在任何时机调用
flushQueuedIntents。结果：Run 运行中 park 的 queue-next 在最常见的自然完成路径上永远不会发出，回执承诺的"等待当前 Run 结束后发出"被静默违背（句柄
close 即丢失）；且此期间新的 act(queue-next) 会因已终态直接派发，插队越过更早的 parked 项。建议在 streamEnded 终态分支补上与 hydrate
相同的一次性放行。

-       void flushQueuedIntents().catch(() => undefined); // 终态观察即放行 parked queue-next（一次性，失败不重试）
+     // streamEnded 终态分支对齐：
+     if (isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events))) {
+       await flushPersisted();
+       interruption = undefined;
+       notify('drained');
+       void flushQueuedIntents().catch(() => undefined); // 与 hydrate 终态分支同规则放行 parked queue-next
+       return;
+     }


─── packages/mobile-core/src/task-office/task-detail.ts:344-345 ───
[bug · medium] unknown 门核对使用原始 runStatus，与本模块终态判断约定不一致：文件内其余 5
处终态判断（buildView:187、currentRunStatus:143、hydrate:353、processEvent:431、streamEnded:454）均用
`terminalRunStatusOf(runStatus, events)` 合并事件流，模块注释明确假设"快照未刷新前流内终态事件也要推进视图（R1-F27）"。若取消以
run.canceled/cancellation_requested 事件先行到达而 execution.runStatus 尚未刷新，此处会把已落地的取消判为
not-landed：stopState 被清除、门解除，随后同一 hydrate 的 :353 又按合并状态判为终态——视图 runStatus=canceled 但停止卡消失，用户重试 stop
只会得到 conflict。建议与同函数下方终态判断使用同一合并口径。

      if (unknownGate !== undefined) {
-       const resolved = resolveUnknownStop(fetched.execution.runStatus);
+       const resolved = resolveUnknownStop(terminalRunStatusOf(fetched.execution.runStatus, events));


─── packages/mobile-core/src/task-office/task-detail.ts:549-550 ───
[bug · medium] unknown 分支置门后无任何核对安排，与 AC2 意图存在落差：act() 的 accepted 分支（:536）与 conflict 分支（:545）都触发
`void hydrate()`，唯独 unknown 分支只置 unknownGate/stopState 并 notify。而门只在
hydrate（:344-352）中解除；若此时连接已中断（离线降级分支 :321 还把 autoResyncs 置满、不自动重连），或 SSE 流仍存活但随后只经 streamEnded
观察到终态（该路径不进 hydrate），unknownGate 将一直滞留：后续一切写意图被 TASK_OFFICE_COMMAND_UNKNOWN 阻塞、stopProjection 恒返
'unknown' 相，仅剩外部显式 resync() 一条恢复路径——apps/mobile 的停止卡文案却承诺"正在与服务端核对"。建议在置门后安排一次（可稍延迟的）hydrate 核对，并考虑在
streamEnded 终态分支同样执行核对。

          if (intent.kind === 'stop') stopState = { phase: 'unknown', since: at, note: messageOf(error) };
          unknownGate = { revision };
+         void hydrate().catch(() => undefined); // 安排一次核对：门必须可被事实解除（AC2），不能只依赖外部 resync


─── packages/mobile-core/src/task-office/task-detail.ts:13-13 ───
[bug · medium] 新增 'offline' 枚举未同步消费方，且精心构造的 message
不会被展示：apps/miniprogram/src/services/office-views.ts 的 `interruptionNotice` 是
`Record<TaskInterruptionReason, string>` 穷举映射，缺 'offline' 键（对该类型是编译错误；若跳过类型检查则运行时返回 undefined →
空白提示）；apps/mobile/src/screens/TaskDetailScreen.tsx 的 INTERRUPTION_COPY 同样缺该键，`??
view.interruption.reason` 回退会把内部码 'offline' 直出给用户（恰是该屏 smoke 测试"不得直出内部码"要防的情形）；且两处 UI 均只渲染 reason
映射、不渲染 interruption.message，:320 的中文文案实际不可见。建议同步补齐两处消费方映射（如 'offline': '当前离线，展示的是最近一次同步的缓存内容'），并考虑让
UI 优先渲染 message。



─── packages/mobile-core/src/task-office/task-detail.ts:517-521 ───
[style · low] dispatch 构造使用嵌套三元（stop/steer/queue_next 三路），违反团队"禁止嵌套三元"规范，三路分支的类型标注也使可读性下降。建议改为 IIFE
switch 或提前分派的辅助函数。

-       const dispatch = intent.kind === 'stop'
-         ? { runId: input.runId, action: 'cancel' as const, expectedRevision: revision }
-         : intent.kind === 'steer'
-           ? { runId: input.runId, action: 'steer' as const, text, expectedRevision: revision }
-           : { runId: input.runId, action: 'queue_next' as const, text, expectedRevision: revision, ...(intentId === undefined ? {} : { intentId }) };
+       const dispatch = (() => {
+         switch (intent.kind) {
+           case 'stop': return { runId: input.runId, action: 'cancel' as const, expectedRevision: revision };
+           case 'steer': return { runId: input.runId, action: 'steer' as const, text, expectedRevision: revision };
+           default: return { runId: input.runId, action: 'queue_next' as const, text, expectedRevision: revision, ...(intentId === undefined ? {} : { intentId }) };
+         }
+       })();


─── packages/mobile-core/src/task-office/task-detail.ts:241-244 ───
[maintainability · low] snapshotOf 手工逐字段拷贝：OfflineTaskSnapshot = Omit<TaskBackendDetail,
'events'>，未来 TaskBackendDetail 新增字段时类型检查不报错，但新字段会被静默排除出离线快照（离线视图字段缺失且难以察觉）。建议改为解构剔除 events，让快照形态与权威
detail 自动保持同步。

-   const snapshotOf = (source: TaskBackendDetail): OfflineTaskSnapshot => ({
-     taskId: source.taskId,
-     runId: source.runId,
-     title: source.title,
+   const snapshotOf = (source: TaskBackendDetail): OfflineTaskSnapshot => {
+     const { events: _events, ...snapshot } = source;
+     return snapshot;
+   };


─── packages/mobile-core/src/task-office/task-office.ts:661-664 ───
[bug · medium] askKnowledge 未接入 Offline Gate，与 T10 纵深防御不一致：start() 已在首笔网络派发（createSession）前
`ports.gate.assertOnline('run')`（:544），且 guardLegacyTaskBackend 对 knowledge-chat 追问同样按 'run'
语义拒绝（guarded-ports.ts:32）——知识问答一样触发服务端执行。但 guardTaskBackend 只拦截 start，本函数的 createSession 与
knowledgeQA.ask 均为未拦截的首笔派发：离线发起提问会以 transport 错误伪装成 TASK_OFFICE_BACKEND，而非
OFFLINE_ACTION_BLOCKED:run 结构化拒绝。建议在首笔派发前（覆盖新建会话与追问两条路径）接入与 start() 相同的门禁。

+       if (ports.gate !== undefined) await ports.gate.assertOnline('run'); // 与 start()/guardLegacyTaskBackend 的 knowledge-chat 'run' 语义一致
        let sessionId = (askInput?.sessionId ?? '').trim();
        if (sessionId === '') {
          // 快速知识问题也是 Task（spec 故事 12/24）：一次初始提问创建目标会话。
-         const session = await callBackend(() => ports.backend.createSession({ title: question.slice(0, 60) }));
+         const session = await callBackend(() => ports.backend.createSession({ title: question.slice(0, SESSION_TITLE_MAX_CHARS) }));


─── packages/mobile-core/src/task-office/task-office.ts:658-659 ───
[maintainability · low] 魔法数与重复逻辑：8000 字提问上限与 60 字标题截断均为内联硬编码，且 `slice(0, 60)` 与 start()
中的标题截断逻辑重复（两处独立维护易漂移）。建议提取具名常量（如 KNOWLEDGE_QUESTION_MAX_CHARS = 8000、SESSION_TITLE_MAX_CHARS =
60）并复用同一截断辅助。

        const question = (askInput?.question ?? '').trim();
-       if (question === '' || question.length > 8000) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
+       if (question === '' || question.length > KNOWLEDGE_QUESTION_MAX_CHARS) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');


─── packages/mobile-core/src/task-office/task-office-errors.ts:25-25 ───
[maintainability · low] 疑似死码：'TASK_OFFICE_COMMAND_CONFLICT' 在 mobile-core 内没有任何抛出点——task-detail.ts
的冲突场景走跨包契约码 'TASK_COMMAND_CONFLICT' 透传并以 outcome: 'conflict' 回执呈现（wrapCommand:155 与
act():541），TaskOfficeError 仅在 act():502 抛 TASK_OFFICE_COMMAND_UNKNOWN。全库引用只有本处定义与 apps/mobile
的文案映射（该映射同样永远不会命中）。留着会误导调用方以为模块会抛此码；建议删除，或在确实需要结构化抛出的路径上启用。



LLM retry report summary: 53 of 84 requests affected -- 36 requests failed, 17 requests recovered after retry

Review planning (19 requests):
- apps/miniprogram/package.json,apps/miniprogram/src/core/auth.ts,apps/miniprogram/src/core/intent.ts,apps/miniprogram/src/platform/authorized-channels.ts,apps/miniprogram/src/platform/credential-store.ts,apps/miniprogram/src/platform/files.ts,apps/miniprogram/src/platform/intent-log.ts,apps/miniprogram/src/platform/transport.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/features/execution/pages.tsx,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/src/services/mobile-office.ts,apps/miniprogram/src/services/office-views.ts,apps/miniprogram/src/services/runtime.ts,apps/miniprogram/src/services/session.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/app-state.ts,apps/mobile/src/adapters/network-status.ts,apps/mobile/src/app-id.ts,apps/mobile/src/foreground-sync.ts,internal/application/repository/mobile_device.go,packages/api-client/src/mobile/devices.ts,packages/mobile-core/src/device/device-registry.ts,packages/mobile-core/src/device/in-memory-device-remote.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/ask.tsx,apps/mobile/src/app/new.tsx,apps/mobile/src/composition.ts,apps/mobile/src/new-task-view.ts,apps/mobile/src/screens/HomeScreen.tsx,apps/mobile/src/screens/NewTaskScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks/budget.tsx,apps/mobile/src/screens/TaskBudgetScreen.tsx,apps/mobile/src/task-budget-integration-smoke.ts,apps/mobile/src/task-budget-view.ts,apps/web/src/commercial/TaskBudget.tsx,internal/handler/commercial_task_budget.go,packages/api-client/src/mobile/task-budget.ts,packages/contracts/src/mobile/task-budget.ts,packages/mobile-core/src/task-office/in-memory-task-budget.ts,packages/mobile-core/src/task-office/task-budget.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 14 more

Core review (33 requests):
- apps/miniprogram/package.json,apps/miniprogram/src/core/auth.ts,apps/miniprogram/src/core/intent.ts,apps/miniprogram/src/platform/authorized-channels.ts,apps/miniprogram/src/platform/credential-store.ts,apps/miniprogram/src/platform/files.ts,apps/miniprogram/src/platform/intent-log.ts,apps/miniprogram/src/platform/transport.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/features/execution/pages.tsx,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/src/services/mobile-office.ts,apps/miniprogram/src/services/office-views.ts,apps/miniprogram/src/services/runtime.ts,apps/miniprogram/src/services/session.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/tests/assembly.test.mjs,apps/miniprogram/tests/core.test.mjs,apps/miniprogram/tests/helpers/assembly-harness.mjs,apps/miniprogram/tests/helpers/node-taro.mjs,apps/miniprogram/tests/helpers/taro-stub.mjs,apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs,apps/miniprogram/tests/office-assembly.test.mjs,apps/miniprogram/tests/office-views.test.mjs,apps/miniprogram/tests/orchestrator.test.mjs,apps/miniprogram/tests/platform-adapters.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/app-state.ts,apps/mobile/src/adapters/network-status.ts,apps/mobile/src/app-id.ts,apps/mobile/src/foreground-sync.ts,internal/application/repository/mobile_device.go,packages/api-client/src/mobile/devices.ts,packages/mobile-core/src/device/device-registry.ts,packages/mobile-core/src/device/in-memory-device-remote.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/dictation-capture.ts,apps/mobile/src/dictation-view.ts,apps/mobile/src/voice-dictation-integration-smoke.ts,packages/api-client/src/mobile/voice.ts,packages/mobile-core/src/voice/dictation.ts,packages/mobile-core/src/voice/in-memory-dictation.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 28 more

Comment filtering (1 request):
- packages/mobile-core/src/task-office/task-detail.ts,packages/mobile-core/src/task-office/task-intent.ts,packages/mobile-core/src/task-office/task-office-errors.ts,packages/mobile-core/src/task-office/task-office.ts,packages/mobile-core/src/task-office/task-timeline.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
