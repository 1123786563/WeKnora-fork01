Review complete: 46 finding(s) across 29 selected item(s).

─── apps/miniprogram/tests/assembly.test.mjs:132-132 ───
[maintainability · low] 两处 snapshot stub 载荷重复（仅 events 不同）：本文件顶部已有 execDto/execEvent 的 helper
风格，且本次补 wire 字段就需要同步修改两处，恰好体现了该重复的维护成本——后续 snapshot 契约再演进时容易遗漏其中一处。建议提取 snapshotDto helper，例如：`const
snapshotDto = events => ({ execution: execDto(5), watermark: 5, incomplete: false,
confirmed_watermark: 5, events });`，两处分别传 `[execEvent(5)]` 与 `[]`。

-     'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } }),
+     'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: snapshotDto([execEvent(5)]) } }),


─── apps/mobile/src/screens/DeploymentLoginScreen.tsx:40-47 ───
[maintainability · low] “Registered deployments” 区块只判断了 `deployments` 非空，未判断 `onSwitchDeployment`
是否存在。该 prop 是可选的，缺失时按钮仍会渲染，但点击是静默 no-op（可选链直接返回 undefined），形成无反馈的死交互。建议将 handler 是否存在纳入渲染条件，与
`officialCloudOrigin` / `onOpenTask`（TasksScreen）处“prop 存在才渲染对应入口”的既有约定保持一致。

-       {deployments && deployments.length > 0 ? (
+       {deployments && deployments.length > 0 && onSwitchDeployment ? (
          <View>
            <Text>Registered deployments</Text>
            {deployments.map((deployment) => (
-             <Button key={deployment.origin} title={deployment.label} onPress={() => { void onSwitchDeployment?.(deployment.origin); }} />
+             <Button key={deployment.origin} title={deployment.label} onPress={() => { void onSwitchDeployment(deployment.origin); }} />
            ))}
          </View>
        ) : null}


─── apps/mobile/src/screens/HomeScreen.tsx:40-42 ───
[maintainability · low] 与 DeploymentLoginScreen 相同的模式：`onSwitchDeployment` 为可选 prop，但 “Switch to …”
按钮的渲染只取决于 `otherDeployments`。handler 缺失时按钮仍渲染且点击无任何效果（可选链短路），用户会误以为切换失败。建议以 `onSwitchDeployment`
是否存在作为渲染前提，缺失时不渲染这组入口。

-       {(otherDeployments ?? []).map((deployment) => (
-         <Button key={deployment.origin} title={`Switch to ${deployment.label}`} onPress={() => { void onSwitchDeployment?.(deployment.origin); }} />
-       ))}
+       {onSwitchDeployment
+         ? (otherDeployments ?? []).map((deployment) => (
+             <Button key={deployment.origin} title={`Switch to ${deployment.label}`} onPress={() => { void onSwitchDeployment(deployment.origin); }} />
+           ))
+         : null}


─── apps/mobile/src/screens/ReadOnlyScreen.tsx:19-33 ───
[maintainability · low] 本组件的 controller 生命周期代码（useRef + useState 初始投影 + effect 内创建/订阅/清理）与
`app/resources.tsx` 的 `ResourcesRouteLifecycle`
逐行重复，连“初始投影是纯计算”的注释都几乎相同。这段逻辑恰恰是仓库里纪律性最强、最怕漂移的部分（controller 必须在 effect 内创建以避免 concurrent
渲染遗留订阅），两处手工同步风险高。建议把该宿主提取为共享组件（如 `screens/ResourcesRouteLifecycleHost`，接收 `handle?:
ResourceShelfHandle` 并渲染 ResourcesScreen），ReadOnlyScreen 与 `/resources` 路由共同复用；顺带可统一 ref 类型写法（此处用
`ReturnType<typeof createResourceShelfController>`，resources.tsx 用 `ResourceShelfController`）。

-   const controllerRef = useRef<ReturnType<typeof createResourceShelfController> | undefined>(undefined);
-   // 初始投影是纯计算：与 app/resources.tsx 的 ResourcesRouteLifecycle 相同的 commit 后副作用纪律。
-   const [state, setState] = useState<ResourceShelfViewState>(handle ? { loading: true } : { loading: false });
-   useEffect(() => {
-     if (!handle) return;
-     const controller = createResourceShelfController(handle);
-     controllerRef.current = controller;
-     setState(controller.state());
-     const unsubscribe = controller.subscribe(setState);
-     return () => {
-       unsubscribe();
-       controller.dispose();
-       controllerRef.current = undefined;
-     };
-   }, [handle]);
+   // 复用共享的 Resources 生命周期宿主，避免与 /resources 路由各自维护一份 controller 纪律
+   // e.g. import { ResourcesRouteLifecycleHost } from './ResourcesRouteLifecycleHost.tsx';
+   // {handle ? <ResourcesRouteLifecycleHost handle={handle} /> : <Text>Read-only browsing is unavailable.</Text>}


─── apps/mobile/src/screens/TasksScreen.tsx:81-81 ───
[style · low] 新增按钮文案使用中文“详情”，而本屏及其余 UI 的可见文案均为英文（“Restore”/“Archive”/“Load more”/“View all
tasks”/“Sign out”/“Switch to …”），用户可见文案语言不一致。建议统一为英文 “Details”（或整体接入文案表后统一本地化）。

-           {onOpenTask !== undefined && <Button title="详情" onPress={() => onOpenTask(card)} />}
+           {onOpenTask !== undefined && <Button title="Details" onPress={() => onOpenTask(card)} />}


─── apps/mobile/src/screens/ReadOnlyScreen.tsx:22-33 ───
[maintainability · low] 这里的状态类型与 `app/resources.tsx` 的 `ResourcesRouteLifecycle` 语义一致但初始化来源不同：当
`handle` 在组件存活期间从有值变为 undefined（Runtime 撤销 shelf）时，cleanup 会 dispose controller，但 `state` 仍保留上一个
deployment/tenant 的 page 投影。当前渲染分支 `handle ? … : …` 恰好兜住了这个陈旧投影不外泄，但这依赖两处代码的隐式配合；若后续有人在 handle
缺失分支渲染 `state.page`，会泄露已切换 scope 的资源数据。建议在 effect 中显式处理 handle→undefined 的迁移（如 `if (!handle) {
setState({ loading: false }); return; }`），把“陈旧投影不可见”从隐式约定变为显式保证。

    useEffect(() => {
-     if (!handle) return;
+     if (!handle) { setState({ loading: false }); return; }
      const controller = createResourceShelfController(handle);
      controllerRef.current = controller;
      setState(controller.state());
      const unsubscribe = controller.subscribe(setState);
      return () => {
        unsubscribe();
        controller.dispose();
        controllerRef.current = undefined;
      };
    }, [handle]);


─── packages/api-client/src/mobile/task-office.ts:133-133 ───
[maintainability · low] `stream` 内的局部变量 `request`（ClientRequest 描述对象）遮蔽了外层第 57 行的 `const request =
options.request`（授权传输函数）。本适配器其余方法（overview/list/archive/restore）中 `request` 均指授权传输通道，此处同名不同义；后续若在
`stream` 内新增普通 REST 调用，很容易误把请求描述对象当函数调用。建议改名为 `eventRequest`（或 `sseRequest`）以消除歧义。

-       const request = executionEventsRequest(input.runId, String(input.cursor));
+       const eventRequest = executionEventsRequest(input.runId, String(input.cursor));
+       // 后续引用同步改为 eventRequest.method / eventRequest.path / eventRequest.headers


─── packages/api-client/src/mobile/task-office.ts:135-137 ───
[maintainability · low] control 帧的 payload 通过 `JSON.parse(frame.data) as {...}` 直接断言，未经结构校验。本文件与
@weknora/contracts 的其余 wire 数据均走严格 parse（如 parseExecutionEvent 会拒绝非对象），唯独 control 帧不一致：若远端发送非对象
payload（数组/字符串/数字），会静默退化为默认 `stream_error`，掩盖契约漂移。建议与既有风格对齐，先校验为非空对象再读取字段。

          if (frame.event === 'control') {
-           const payload = JSON.parse(frame.data) as { code?: string; message?: string };
+           const value: unknown = JSON.parse(frame.data);
+           if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('control frame data must be an object');
+           const payload = value as { code?: string; message?: string };
            input.onControl({ code: payload.code ?? 'stream_error', message: payload.message ?? '' });


─── packages/contracts/src/mobile/execution.ts:63-64 ───
[maintainability · low] `archived_at` 的校验执行了两次：此处 `isoTime(archivedAt, 'task.archived_at')`
的返回值被丢弃，而下方 return 的 spread 中 `archived_at: isoTime(archivedAt, 'task.archived_at')`
会再次执行完全相同的校验。首次调用是冗余代码，建议删除或先归一化为局部变量，仅校验一次（与 `title` 分支先校验后复用的写法保持一致）。

-   const archivedAt = row.archived_at;
-   if (archivedAt !== undefined && archivedAt !== null) isoTime(archivedAt, 'task.archived_at');
+   const archivedAt = row.archived_at === undefined || row.archived_at === null ? undefined : isoTime(row.archived_at, 'task.archived_at');
+   // return 中改为：...(archivedAt === undefined ? {} : { archived_at: archivedAt })


─── apps/mobile/src/screens/DeploymentLoginScreen.tsx:43-45 ───
[bug · medium] 点击注册 deployment 切换后，若目标 deployment 无已存凭据，Runtime 会发布 surface 仍为 'deployment-login'
的新快照（mobile-runtime.ts switchDeployment 的无凭据分支），但 RuntimeSurface 渲染本组件时未加 key，组件实例被复用：本地
origin/email/password/error 状态全部保留，且组件未接收 Runtime 当前活动 deployment 的任何信息。结果是表单仍显示上一个 deployment 的
origin（残留已输入的 email/密码也跨 scope 保留），用户随后点 "Sign in" 会登录到表单中的旧 origin 而非刚切换的目标。建议在发起切换时同步重置表单到目标
deployment（或由上层传入当前活动 origin 作为 prop 同步）。

            {deployments.map((deployment) => (
-             <Button key={deployment.origin} title={deployment.label} onPress={() => { void onSwitchDeployment?.(deployment.origin); }} />
+             <Button key={deployment.origin} title={deployment.label} onPress={() => { setOrigin(deployment.origin); setEmail(''); setPassword(''); setError(undefined); void onSwitchDeployment?.(deployment.origin); }} />
            ))}


─── apps/mobile/src/screens/ReadOnlyScreen.tsx:38-38 ───
[bug · low] 降级引导文案让用户 "switch to another deployment"，但本屏只提供 Sign out，未接收/渲染任何 deployment
切换入口（composition 的 read-only 分支也未传入 deployments/onSwitchDeployment）。用户唯一路径是先登出再回登录屏选择，与 #66
兼容性降级面上直达切换的预期不符，引导与可用操作不一致。建议参照 HomeScreen 接收可选 otherDeployments/onSwitchDeployment props
并渲染切换入口，或将文案改为引导先登出后切换。



─── internal/handler/session/workbench_read.go:181-181 ───
[bug · critical] run.Owner 传错了字段:这里是租约属主而非业务属主。证据链:(1) agentRunRow.view()(agent_run.go:64)的映射是
`Owner: r.LeaseOwner`、`UserID: r.OwnerID`,即 run.Owner 来自 agent_runs.lease_owner(VARCHAR(128),由 Claim
写入 worker 标识,Settle 时清空为 ""),而 owner_id(业务属主,即认证用户)映射到 run.UserID——同包
handler/session/agent_run.go:85 的属主比较用的正是 run.UserID;(2) ReadTaskFactsForRun 的 WHERE 绑定
`agent_runs.owner_id = ?`,且 ownerID=="" 时守卫直接返回 agentruntime.ErrNotFound。因此:已 settle 的 run 的
lease_owner 为空串 → 守卫返回 ErrNotFound → 404;运行中 run 的 lease_owner 是 worker id(如 "worker-xxx")→ owner_id
永不匹配 → ErrNotFound → 404。容器已在 NewWorkbenchReadHandler 中无条件 .WithTaskFacts(lists),上线后
GetWorkbenchSnapshot 对所有合法请求统一 404,端点完全不可用。单测未暴露此问题是因为 workbenchRunReaderStub 手工构造了 Owner:"u1"(与真实
view() 映射相悖),且 stubTaskFactsReader.ReadTaskFactsForRun 完全忽略入参。应改为传 run.UserID(经 GetOwnedRun
解析后恒等于认证用户)。

- 		facts, factsErr := h.taskFacts.ReadTaskFactsForRun(c.Request.Context(), run.Key.TenantID, run.Owner, run.Key.RunID)
+ 		facts, factsErr := h.taskFacts.ReadTaskFactsForRun(c.Request.Context(), run.Key.TenantID, run.UserID, run.Key.RunID)


─── apps/mobile/src/app/tasks/detail.tsx:16-20 ───
[bug · medium] catch 分支把 open() 抛出的一切异常统一替换为「请先登录并激活空间」，与实际失败原因不符：mobile-core 的 open() 会抛带错误码的
TaskOfficeError——路由缺参（String(params.taskId ?? '') 降级为空串）抛 TASK_OFFICE_INVALID_INPUT、detail 端口缺失抛
TASK_OFFICE_DETAIL_UNAVAILABLE，这些场景下用户已登录且空间已激活，提示会误导排障。建议区分「无 office（未登录）」与「open 异常」两类失败，并在路由层对空
taskId/runId 前置校验给出明确提示。

-     } catch {
-       controller = undefined;
+     let openFailure: unknown;
+     let controller: TaskDetailController | undefined;
+     try {
+       const handle = activeTaskOffice()?.open({ taskId, runId });
+       controller = handle === undefined ? undefined : createTaskDetailController(handle);
+     } catch (failure) {
+       openFailure = failure;
      }
      if (controller === undefined) {
-       setState({ loading: false, error: '请先登录并激活空间，再打开任务详情。' });
+       setState({
+         loading: false,
+         error: openFailure === undefined
+           ? '请先登录并激活空间，再打开任务详情。'
+           : `无法打开任务详情：${openFailure instanceof Error ? openFailure.message : String(openFailure)}`,
+       });


─── apps/mobile/src/task-detail-integration-smoke.ts:69-70 ───
[test · medium] 异常路径绕过证据契约且资源清理不一致：(1) 函数注释与文件内注释均声称失败结果会被如实记录（"including failed live outcomes"、"将以
opened:'failed' 如实暴露"），但 signIn 之外 office.tasks()/open()/hydrate() 任一抛错都会让整函数 reject——调用方
task-office-detail.integration.test.ts 的 `await runTaskDetailIntegration(config)` 无 catch，未处理
rejection 直接打断测试且不留任何 evidence，opened:'failed' 实际只在「已登录但 surface 未授权」这一条路径产生；(2) no-tasks
早退与未授权早退均直接 return，不执行 runtime.dispose()，与 happy path 不一致。建议用 try/catch（记 opened:'failed' 后 return
evidence）+ finally（handle?.close + runtime.dispose()）收口。

+   let handle: ReturnType<TaskOffice['open']> | undefined;
+   try {
-   const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
+     const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
-   if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;
+     if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;
+     // ... tasks()/open()/hydrate() 逐段 catch：记录 opened: 'failed' 后 return evidence
+   } finally {
+     handle?.close('integration-complete');
+     runtime.dispose();
+   }


─── apps/mobile/src/task-detail-integration-smoke.ts:38-40 ───
[test · low] 相比其「自包含复制」来源 runtime-integration-smoke.ts 的 env
校验语义，这里缺少主机防线（disallowedDeploymentHost：拒绝 localhost/环回/私网/链路本地/保留地址）。两者都是凭 WEKNORA_MOBILE_TEST_*
变量发起真实 HTTP/SSE 请求的 opt-in 通道，本文件却允许指向内网地址，防线不一致；建议复制同款主机校验后再放行 enabled: true。



─── apps/mobile/src/task-detail-view.ts:26-26 ───
[maintainability · low] messageOf 对 TaskOfficeError 取 message 只能得到裸错误码（其构造为 super(code)，如
'TASK_OFFICE_BACKEND'），该字符串会经 hydrate/refresh 失败分支直达 TaskDetailScreen 的错误文案，最终用户看到的是内部码而非可读信息；非
Error 对象 String() 还会得到 '[object Object]'。建议按 TaskOfficeError.code 映射用户友好文案（可携带 cause
信息），未识别类型时兜底为通用错误描述。

-   const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));
+   const DETAIL_ERROR_LABELS: Record<string, string> = {
+     TASK_OFFICE_BACKEND: '服务暂时不可用，请稍后重试',
+     TASK_OFFICE_SCOPE_CHANGED: '登录状态或空间已变化，请重新进入',
+     TASK_OFFICE_DETAIL_CLOSED: '任务详情已关闭',
+   };
+   const messageOf = (failure: unknown): string => {
+     if (failure instanceof TaskOfficeError) return DETAIL_ERROR_LABELS[failure.code] ?? '任务详情同步失败';
+     return failure instanceof Error ? failure.message : '任务详情同步失败';
+   };


─── packages/mobile-core/src/runtime/mobile-runtime.ts:422-422 ───
[bug · medium] switchDeployment 中 `await ports.deploymentRegistry.list()` 位于内层 try/catch 之外（外层 try
只有 finally 没有 catch）：SecureStore 读取失败时异常会直接 reject。Runtime
其余全部状态迁移（signIn/boot/completeOidc/activateTenant）都保证内部全捕获、从不 reject，而生产装配中 HomeScreen.tsx:41 与
DeploymentLoginScreen.tsx:44 均以 `void onSwitchDeployment?.(...)` fire-and-forget 调用，该 rejection
将成为未处理的 Promise rejection。forgetDeployment 中 signOut()/registry.remove() 的持久化失败同理会向调用方 reject。建议为外层
try 补充 catch 返回当前 state，与兄弟方法的不 reject 约定对齐。

-         const record = (await ports.deploymentRegistry.list()).find((entry) => entry.origin === target!.origin);
+       } catch {
+         return state;
+       } finally {
+         await vaultTail;
+       }
+     },
+     // forgetDeployment 同理：signOut()/registry.remove() 的持久化异常应捕获而非向调用方 reject


─── packages/mobile-core/src/runtime/mobile-runtime.ts:419-421 ───
[maintainability · low] switchDeployment 与 forgetDeployment 中存在两段几乎相同的「尝试归一化 origin、失败置
undefined、为空则静默返回」逻辑（此处与第 440 行）。两处校验规则必须保持同步（空串、畸形 URL 均静默 no-op），后续若调整其一极易漏改另一处，建议抽取共享 helper（如
`const tryNormalizeOrigin = (origin: unknown): Deployment | undefined => { try { return typeof
origin === 'string' && origin.trim() !== '' ? normalizeDeployment({ origin }) : undefined; } catch {
return undefined; } }`）。



─── packages/mobile-core/src/runtime/mobile-runtime.ts:379-380 ───
[bug · low] `ports.authorizedStream?.(deployment.origin)` 返回 undefined（ports.ts 注释明确该情形为「平台无流式
fetch，fail closed」）时，与 surface 非 authorized 混用同一个 RUNTIME_UNAUTHORIZED
错误码，把「能力缺失」误报为「未授权」。task-detail 的 streamFailed 会把它作为 stream-error
中断展示，排障与用户文案都会指向重新登录方向。建议为能力缺失区分错误码（如 RUNTIME_STREAM_UNSUPPORTED），便于上层区分「不支持流式」与「未授权」两种失败。

        const transport = deployment && state.surface === 'authorized' ? ports.authorizedStream?.(deployment.origin) : undefined;
-       if (!deployment || !transport) throw new Error('RUNTIME_UNAUTHORIZED');
+       if (!deployment) throw new Error('RUNTIME_UNAUTHORIZED');
+       if (!transport) throw new Error('RUNTIME_STREAM_UNSUPPORTED');


─── packages/mobile-core/src/runtime/ports.ts:57-58 ───
[documentation · low] AuthorizedStreamTransport 的契约注释只记载了 pre-stream 401 与正常结束两种情形，但 mobile-runtime
的 guardedChunk 依赖第三条隐含义务：onChunk 回调抛错（如 scope 变更时的 RUNTIME_SCOPE_CHANGED）必须沿 transport
传播并中止底层连接——scope 撤销后完全依靠该异常截断流。当前 sse-stream 适配器会传播回调异常（onChunk 在 read 循环内同步调用），但若未来某适配器吞掉回调异常，已撤销
scope 的连接不会被终止、流也不会结束。建议把「onChunk 抛错必须 reject 并释放底层连接」写入契约注释，固化这一跨文件依赖。

- /** Authorized SSE read channel. Contract: a pre-stream 401 rejects (ApiError, status 401) with no chunks emitted; normal end resolves. */
- export type AuthorizedStreamTransport = (
+ /** Authorized SSE read channel. Contract: a pre-stream 401 rejects (ApiError, status 401) with no chunks emitted; normal end resolves; a throw from onChunk must propagate as a rejection and abort the underlying connection (the Runtime relies on this to cut streams on scope revocation). */


─── packages/mobile-core/src/runtime/in-memory-adapters.ts:20-20 ───
[maintainability · low] 内存版 upsert 原样保存 label，而同端口的
createSecureDeploymentRegistry（apps/mobile/src/adapters/deployment-registry.ts:30）会 trim label
且为空时回退 origin。同一 DeploymentRegistry 端口的两个 Adapter 数据规范化行为不一致：initial 传入或调用方直接 upsert 未清洗的 Deployment
时，list() 会返回带空白甚至空字符串的 label。代码注释声称与安全版「一致」仅覆盖前移顺序契约，建议同时把 trim/fallback 语义对齐，避免以内存版为参照编写的新 Adapter
复制了弱化的规范化行为。

-       const next = { origin: deployment.origin, label: deployment.label };
+       const label = deployment.label.trim() !== '' ? deployment.label.trim() : deployment.origin;
+       const next = { origin: deployment.origin, label };


─── packages/mobile-core/src/runtime/mobile-runtime.ts:237-240 ───
[bug · low] deploymentStore.write 与新增的 deploymentRegistry.upsert 串行执行且无回滚：upsert 持久化失败时活动实例已写入
deploymentStore 但不在登记清单（listDeployments 缺项、switchDeployment 永远找不到它），两份持久化状态 diverge。更关键的是外层 catch
会把这类持久化失败误分类为 'authentication-required'（surface 变为 upgrade-required），把一个实际已通过 me/capabilities
验证的用户误导到登录/升级路径。建议将登记类持久化失败与认证失败区分处理——例如 registry.upsert 失败时容忍并继续授权流程（登记清单本就是 presentation-safe
的辅助数据），或在 catch 中识别持久化异常单独上报。



─── packages/mobile-core/src/task-office/task-office.ts:158-158 ───
[maintainability · medium] 缺省 defaultDetailStore 是进程内 Map 且按 runId
无界增长：生产组合根（apps/mobile/src/composition.ts 的 taskOfficeFor）只传了 detail 未传 store，导致「App
重启合并恢复」在生产静默失效，也违背本批「移动缓存按 Deployment/user/Tenant 加密隔离」的约束——缺省兜底把接线遗漏变成了无提示的功能降级。建议与 detail 端口一样
fail closed：未显式提供 store 时 open() 抛 TASK_OFFICE_DETAIL_UNAVAILABLE（或至少给缺省 store 加容量上限并在注释标明仅限测试场景）。

-   const defaultDetailStore = createInMemoryTaskProjectionStore();
+     open(taskOpen: { taskId: string; runId: string }): TaskHandle {
+       const taskId = taskOpen.taskId.trim();
+       const runId = taskOpen.runId.trim();
+       if (taskId === '' || runId === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
+       if (ports.detail === undefined) throw new TaskOfficeError('TASK_OFFICE_DETAIL_UNAVAILABLE');
+       if (ports.store === undefined) throw new TaskOfficeError('TASK_OFFICE_DETAIL_UNAVAILABLE'); // 生产必须显式接线持久化投影存储，禁止静默降级为进程内缓存
+       return createTaskDetail({ taskId, runId }, { backend: ports.detail, store: ports.store, lease: ports.lease });
+     },


─── packages/mobile-core/src/task-office/task-detail.ts:156-158 ───
[bug · medium] hydrate 对 detail/events/committedCursor 的赋值缺少 epoch/串行化守卫：interrupt() 触发的有界自动重同步与用户显式
resync() 并发时，两个 hydrate 交错，较慢的旧 detail 响应后到达会覆盖新状态——committedCursor 从较大水位回退、并以裁剪后的事件集覆写
store（持久投影短暂回退）。此外 hydrate 未挂到 chain 上，已入队的旧流 processEvent 可在 hydrate 的 await 挂起点之间插入执行，造成内存与 store
短暂不一致。虽然后续靠缺口检测/重放可自愈，但会引发虚假中断与重复重放。建议进入 hydrate 时捕获 streamEpoch（或专用 hydrateEpoch），在每个 await
之后检查变化即放弃本次结果，与 startStream 的 epoch 守卫对齐；或将 hydrate 也经 chain 串行化。

      const fetched = await wrap(() => ports.backend.detail(input.runId));
-     if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
+     if (!leaseActive(lease) || epoch !== streamEpoch) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      detail = fetched;


─── packages/mobile-core/src/task-office/task-detail.ts:281-286 ───
[maintainability · low] close(reason) 的 reason 形参仅以 void
消解，属死参数：调用方（apps/mobile/src/task-detail-view.ts:53 传入
'controller-disposed'）会误以为关闭原因被记录消费。模块当前无日志端口，建议要么删除该参数并同步调整调用方，要么将其透传给可观测端口（如后续接入 telemetry 时），避免
API 意图与实现不符。

-     close(reason?: string) {
+     close() {
        closed = true;
        abortStream();
        listeners.clear();
-       void reason;
      },


─── packages/mobile-core/src/task-office/task-detail.ts:88-88 ───
[maintainability · low] createTaskDetail 的 input.taskId 从未被读取：persist 写入的是服务端权威的 detail!.taskId，视图
taskId 也取自 detail，调用方传入并经 open() 校验的 taskId 实际被丢弃。若这是有意以服务端为准，建议删除该参数（open() 相应只收
runId）或在注释中说明「taskId 仅用于 open() 输入校验，投影一律以服务端返回为准」，避免读者误以为句柄绑定调用方提供的 taskId。

- export function createTaskDetail(input: { taskId: string; runId: string }, ports: TaskDetailPorts): TaskHandle {
+ export function createTaskDetail(input: { runId: string }, ports: TaskDetailPorts): TaskHandle {
+   // taskId 一律以服务端 detail 返回为准；open() 的入参校验仅为输入卫生。


─── packages/mobile-core/src/task-office/task-detail.ts:219-219 ───
[performance · low] duplicateSeqs 只增不减且无上限：若服务端异常反复重放旧 seq（长连接不触发 hydrate 重置），该数组无界增长，且每次 notify 都以
[...duplicateSeqs] 全量拷贝进视图并推给所有订阅者引发重渲染。既然它仅承担可观测职责，建议保留滑动窗口（如最近 N 条）并在视图标注省略，与
TASK_DETAIL_HISTORY_LIMIT 的有界思路保持一致。

-       duplicateSeqs = [...duplicateSeqs, event.seq]; // 重复事件幂等跳过，但可观测
+       duplicateSeqs = [...duplicateSeqs, event.seq].slice(-TASK_DETAIL_DUPLICATE_LIMIT); // 重复事件幂等跳过，但可观测（有界）


─── packages/mobile-core/src/task-office/task-timeline.ts:67-68 ───
[maintainability · low] 同一批事件类型字面量在三处平行维护：terminalRunStatusOf 的终态判定、KIND_OF_TYPE
的分类表、EVENT_SUMMARIES 的摘要表。新增事件类型（或服务端扩展 wire 枚举）需要同步修改三处，漏改任一处会导致终态推进、Timeline
分类与摘要互相矛盾——其中终态表还必须逐字镜像 agent_run_snapshot.go。建议提取单一事件类型注册表（每项声明 terminal 效果 + kind +
摘要），三处投影都从注册表派生，把「镜像服务端」收敛到一个位置。

- const KIND_OF_TYPE: Record<string, TaskTimelineKind> = {
-   'run.started': 'run_status', 'run.status': 'run_status', 'run.failed': 'run_status', 'run.canceled': 'run_status',
+ const EVENT_TYPE_REGISTRY: Record<string, { kind: TaskTimelineKind; summary?: string; terminal?: 'succeeded' | 'failed' | 'canceled' }> = {
+   'run.completed': { kind: 'conclusion', summary: '任务已完成', terminal: 'succeeded' },
+   // ... KIND_OF_TYPE / EVENT_SUMMARIES / terminalRunStatusOf 均由此派生
+ };


─── apps/mobile/src/adapters/sse-stream.ts:17-20 ───
[maintainability · medium] 手工伪造 ApiError 形态脆弱：当前 mobile-runtime 的 unauthorizedStatus 与 api-client
task-office 的 409 映射确实按 name/status 属性判断、功能兼容，但 '@weknora/api-client' 包根已导出真正的 ApiError 类（errors.ts
定义、index.ts re-export），伪造错误缺少 code 字段且与类定义存在漂移风险——后续任何消费方改用 instanceof ApiError 或读取 error.code
判定时将静默失效。建议直接抛出 ApiError 实例。

-     const error = new Error(`workbench event stream failed with HTTP ${response.status}`);
-     error.name = 'ApiError';
-     (error as unknown as { status?: number }).status = response.status;
-     throw error;
+ import { ApiError } from '@weknora/api-client';
+ // ...
+     throw new ApiError({ status: response.status, code: `HTTP_${response.status}`, message: `workbench event stream failed with HTTP ${response.status}` });


─── apps/mobile/src/adapters/sse-stream.ts:22-29 ───
[bug · medium] 读取循环缺少 finally 清理：onChunk 同步抛出时（runtime authorizedEventStream 的 guardedChunk 在 scope
变化时抛 RUNTIME_SCOPE_CHANGED；api-client task-office 的 SSE 回调对 frame.data 直接 JSON.parse
无保护，畸形数据即抛），reader 既不 cancel 也不释放，底层 SSE 连接保持打开持续接收服务端推送；而 task-detail 的
interrupt→hydrate→startStream 会另建新流，旧连接泄漏叠加（移动端流量/电量）。AbortSignal 仅覆盖 fetch 发起与 read 等待阶段，无法覆盖
onChunk 抛出路径。建议以 try/finally 对 reader.cancel() 兜底（正常 done 后 cancel 无害）。

    const reader = (response.body as ReadableStream<Uint8Array>).getReader();
    const decoder = new TextDecoder();
+   try {
-   for (;;) {
+     for (;;) {
-     const { done, value } = await reader.read();
+       const { done, value } = await reader.read();
-     if (done) break;
+       if (done) break;
-     if (value !== undefined && value.length > 0) onChunk(decoder.decode(value, { stream: true }));
+       if (value !== undefined && value.length > 0) onChunk(decoder.decode(value, { stream: true }));
-   }
+     }
-   onChunk(decoder.decode());
+     onChunk(decoder.decode());
+   } finally {
+     void reader.cancel().catch(() => undefined);
+   }


─── apps/mobile/src/adapters/deployment-registry.ts:12-14 ───
[bug · medium] parseRecords 对任一畸形条目整体返回 undefined 后，list() 显示为空、upsert/remove 以空列表整体覆写
SecureStore：单条脏数据或未来 schema 演进即静默清空全部已注册 Deployment；且 switchDeployment 仅依赖 registry.list()
查找记录（mobile-runtime.ts），多部署切换入口随之消失（credentialStore 中的凭据仍在但无 UI 入口可达）。多记录聚合存储应跳过畸形条目（filter
语义）而非整体弃用。

-       if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return undefined;
+       if (!entry || typeof entry !== 'object' || Array.isArray(entry)) continue;
        const record = entry as { origin?: unknown; label?: unknown };
-       if (typeof record.origin !== 'string' || record.origin.trim() === '') return undefined;
+       if (typeof record.origin !== 'string' || record.origin.trim() === '') continue;


─── apps/mobile/src/adapters/deployment-registry.ts:31-32 ───
[bug · medium] 注册表无条目数/大小上限：Android 上 Expo SecureStore 单值约 2KB，多条目累积（origin+label 的 JSON）超限时
setItemAsync 抛错，经 runtime.authenticate 的 catch 将整个登录裁决为 authentication-required——credential
已持久化却呈现未授权，且每次 boot 复现，用户被锁在降级面。注册表是 presentation 辅助数据（后端为唯一权威），写入失败不应有能力阻塞授权主流程；建议限制保留条数并隔离写失败。

-       const next = [{ origin: deployment.origin, label }, ...records.filter((record) => record.origin !== deployment.origin)];
+       const MAX_REGISTRY_ENTRIES = 16;
+       const next = [{ origin: deployment.origin, label }, ...records.filter((record) => record.origin !== deployment.origin)].slice(0, MAX_REGISTRY_ENTRIES);
+       try {
-       await store.setItemAsync(REGISTRY_KEY, JSON.stringify(next));
+         await store.setItemAsync(REGISTRY_KEY, JSON.stringify(next));
+       } catch {
+         /* 注册表写失败不阻塞授权主流程；下次成功写入时自然重试 */
+       }


─── apps/mobile/src/composition.ts:193-195 ───
[bug · medium] 该 effect 无 catch：listDeployments → registry.list() → SecureStore getItemAsync
reject（系统 Keychain/Keystore 错误）时形成未处理 rejection 且 deployments 不更新；依赖整个 snapshot 对象（每次 publish
都是新对象）导致任意快照变化都重复读一次安全存储；两次读取乱序完成时还可能以过期列表覆盖新列表。建议 .catch 兜底 + cancelled 标志丢弃过期结果，并将依赖收敛到真正需要的字段。

    useEffect(() => {
-     void activeRuntime.listDeployments().then(setDeployments);
-   }, [activeRuntime, snapshot]);
+     let cancelled = false;
+     activeRuntime.listDeployments()
+       .then((next) => { if (!cancelled) setDeployments(next); })
+       .catch(() => { if (!cancelled) setDeployments([]); });
+     return () => { cancelled = true; };
+   }, [activeRuntime, snapshot.surface, snapshot.deployment?.origin]);


─── apps/mobile/src/composition.ts:106-106 ───
[bug · medium] authorizedStream 解析 expo/fetch 失败返回 undefined（fail closed）时，此处仍无条件把
authorizedEventStream 接为 office 的 stream 通道：runtime 对 undefined 传输抛 'RUNTIME_UNAUTHORIZED'，详情页流层将以
stream-error 收场——错误语义误导（用户实际已授权，仅流通道不可用），且 task-detail 会按该错误触发 AUTO_RESYNC_LIMIT(2) 次徒劳的自动
resync（每次都重走 REST 详情+流）。建议组合层显式区分"流通道不可用"（例如仅在 streamFetch 可用时传入 stream，或引入专用错误码让 task-detail
跳过自动重连并给出准确状态），落实 #66 的兼容性降级。



─── apps/mobile/src/composition.ts:108-109 ───
[bug · low] taskOffices 按 origin 缓存，但 remote 的 request/stream 闭包经
runtime.authorizedRequest/authorizedEventStream 按"当前 activeDeployment"解析
baseURL（authorizedTransport/authorizedStream 均以 activeDeployment.origin 调用），与缓存键 origin
脱钩：详情路由挂载期间若授权目标变化（如 OIDC 深链回调完成另一部署登录），旧 handle 会持新 lease 向新 deployment 发起旧 runId 的请求（lease
迟到拒绝只覆盖降级窗口，不覆盖"切换后再授权"）。同时该 Map 在 signOut/forgetDeployment 后不回收。建议 office 绑定的通道显式携带/校验 origin。



─── apps/mobile/src/composition.ts:203-203 ───
[bug · low] onSwitchDeployment 仅 await 无 catch：runtime.switchDeployment 外层是 try/finally 而无
catch，registry 的 SecureStore list() 失败会以 rejection 逃逸；HomeScreen/DeploymentLoginScreen 均以 void
onSwitchDeployment?.(...) 调用，结果是未处理 rejection 且用户点击切换后无任何反馈。建议在此处捕获（或由 runtime 层兜底为返回快照），错误呈现交由
Screen 层。

-     onSwitchDeployment: async (origin) => { await activeRuntime.switchDeployment(origin); },
+     onSwitchDeployment: async (origin) => { await activeRuntime.switchDeployment(origin).catch(() => undefined); },


─── packages/api-client/src/mobile/task-office.ts:146-147 ───
[maintainability · low] `'TASK_STREAM_CURSOR_EXPIRED'` 是跨包契约码：此处（api-client）负责写入 error.code，而
packages/mobile-core/src/task-office/task-detail.ts 第 260 行以独立的字符串字面量 `code ===
'TASK_STREAM_CURSOR_EXPIRED'` 匹配。两个包各自硬编码同一魔法字符串，任一侧改动/拼写漂移都会让 409 开流即过期的恢复路径静默退化为通用
stream-error（自动快照重载失效），且编译期无法发现。建议提取为共享常量导出（如从 api-client 导出该常量供消费方导入，或放入 @weknora/domain/mobile
等两侧共同依赖的常量模块，参照 CLIENT_PROTOCOL_VERSION 的做法）。

-           const expired = new Error('workbench event stream cursor expired');
-           (expired as unknown as { code?: string }).code = 'TASK_STREAM_CURSOR_EXPIRED';
+ // 共享常量（例如 api-client 导出，mobile-core 导入）
+ export const TASK_STREAM_CURSOR_EXPIRED = 'TASK_STREAM_CURSOR_EXPIRED';
+ 
+ // task-office.ts
+ (expired as unknown as { code?: string }).code = TASK_STREAM_CURSOR_EXPIRED;


─── apps/mobile/src/screens/TaskDetailScreen.tsx:19-26 ───
[bug · medium] 错误分支成为死路：首帧 hydrate 失败时 controller 发布 { view: undefined, loading: false, error
}，该早退分支只渲染文本、完全丢弃 onRefresh——而 mobile-core 的设计是「保持 interrupted 已通知状态；显式 resync() 可再试」，控制器 refresh()
也支持从无 view 状态恢复。瞬时网络故障后用户没有任何就地重试入口，只能退出路由再重进。建议在失败态渲染重试按钮（复用传入的 onRefresh）。

- if (view === undefined) {
+   if (view === undefined) {
      return (
        <View>
          <Text>{loading ? '正在读取服务端快照…' : '无法读取该任务'}</Text>
          {error !== undefined && <Text>{error}</Text>}
+         {!loading && <Button title="重试" onPress={onRefresh} />}
        </View>
      );
    }


─── apps/mobile/src/screens/TaskDetailScreen.tsx:60-60 ───
[bug · low] 刷新按钮未随 loading 禁用：有 view 的分支从未消费 loading prop，controller.refresh() 置 loading: true
后界面无任何反馈，快速连点会并发触发多次 handle.resync()（每次各自 abortStream + hydrate，交错写 detail/committedCursor 并
notify，可能用旧快照短暂覆盖新快照）。建议按钮加 disabled={loading}，同时让 loading 在该分支有可见语义。

-       <Button title="重新同步快照" onPress={onRefresh} />
+       <Button title="重新同步快照" onPress={onRefresh} disabled={loading} />


─── apps/mobile/src/screens/TaskDetailScreen.tsx:39-39 ───
[maintainability · low] 内部中断码直出 UI：interruption.reason 的取值是
'gap'/'cursor-expired'/'stream-error'/'stream-ended-nonterminal'/'persist-failed' 这类内部标识符，直接拼在中文
CONNECTION_LABELS 之后展示给最终用户，而更具诊断价值的 interruption.message 被丢弃。建议与 CONNECTION_LABELS/LIFECYCLE_LABELS
一致地建立 reason → 中文标签映射，并在存在 message 时附加展示。

-         <Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${view.interruption.reason}` : ''}</Text>
+ const INTERRUPTION_REASON_LABELS: Record<TaskDetailView['interruption'] extends { reason: infer R } ? R : never, string> = { gap: '事件缺口', 'cursor-expired': '游标过期', 'stream-error': '流错误', 'stream-ended-nonterminal': '流提前结束', 'persist-failed': '本地持久化失败' };
+ // …渲染处：
+ <Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${INTERRUPTION_REASON_LABELS[view.interruption.reason]}${view.interruption.message === undefined ? '' : `：${view.interruption.message}`}` : ''}</Text>


─── apps/mobile/src/app/tasks/detail.tsx:32-32 ───
[bug · low] TaskDetailScreen 内部的 expanded（以 seq 为键）未按任务隔离：seq 在每个 run 内从 1 重新计数（processEvent 校验 seq
>= 1 且逐 +1），当 /tasks/detail 的路由参数原位变化（深链、setParams）复用同一组件实例时，effect 会重建 controller 并重置 state，但
Screen 的 expanded 会原样保留，旧任务已展开的 seq 会命中新任务时间线的同名条目、错误地自动展开其证据。建议给 Screen 加 key 强制按任务重挂载。

-   return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} />;
+   return <TaskDetailScreen key={`${taskId}:${runId}`} view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} />;


─── packages/mobile-core/src/runtime/mobile-runtime.ts:412-414 ───
[bug · medium] listDeployments 缺少失败包含：registry 读取失败（生产装配中为 SecureStore）会直接 reject。与 switchDeployment
的 fail-closed 契约不同（同 PR 的已确认发现只覆盖了 switchDeployment 内部的 list 调用），这是新的查询 API 本身。实际后果：生产装配
composition.ts:194 以 `void activeRuntime.listDeployments().then(setDeployments)` 调用且无 catch，而该
useEffect 依赖 [activeRuntime, snapshot]，在每次 snapshot 变化时都会执行——持久失败的存储会反复产生 unhandled
rejection，且实例列表永远停留在空/旧值。建议与「未提供 registry 端口时返回空数组」的 fail-closed 语义对齐，捕获后返回 []。

      async listDeployments(): Promise<Deployment[]> {
-       return ports.deploymentRegistry ? await ports.deploymentRegistry.list() : [];
+       if (!ports.deploymentRegistry) return [];
+       try {
+         return await ports.deploymentRegistry.list();
+       } catch {
+         return [];
+       }
      },


─── packages/mobile-core/src/runtime/mobile-runtime.ts:449-449 ───
[bug · low] forgetDeployment 的持久化失败未被包含：mutateCredential(clear) 或 mutateDeployment(remove) 抛错时（try
只有 finally 无 catch）方法直接 reject。当前清理顺序（先清凭据、后移除登记）在安全上是对的，但 remove 失败会留下「已登记但无凭据」的分歧条目，且调用方一旦按
HomeScreen 对 switch/signOut 的惯例以 `void onForget(...)` 触发，就会变成 unhandled rejection。建议与
switchDeployment 的「内部全捕获、从不 reject」语义对齐（顺序保持先清凭据，残留 registry 条目可由下次 forget 重试修复）。

          await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); });
+       } catch {
+         // fail closed：清理失败不上抛（调用方多以 void 触发）；凭据已尽力清除，
+         // 残留的登记条目可通过再次 forget 重试移除。
+       } finally {


─── packages/mobile-core/src/runtime/mobile-runtime.ts:424-425 ───
[bug · low] switchDeployment 缺少「同 origin 且当前会话有效」的短路：切换目标就是当前活动实例时，begin() 仍会撤销一个完全有效的 scope（关闭
shelf、revoke vault、清空 activeCredential），随后完全依赖重新走 me/capabilities 验证；若此时网络瞬时失败，catch 会把本来有效的会话降级为
upgrade-required/authentication-required，用户只是点了一下当前实例的切换按钮（HomeScreen 对每个登记项都渲染 Switch 按钮）就被登出。建议在
begin 之前对「目标 === 当前活动实例且 surface 为 authorized/read-only 且 lease 存在」短路返回 state。

          const deployment = normalizeDeployment({ origin: record.origin, label: record.label });
+         if (activeDeployment?.origin === deployment.origin && lease && (state.surface === 'authorized' || state.surface === 'read-only')) return state;
          const requestEpoch = begin(deployment);


─── packages/mobile-core/src/runtime/types.ts:59-60 ───
[documentation · low] resourceShelf() 的接口契约注释已过期：authenticate 中 read-only
surface（server_upgrade_required 降级）同样会签发 lease 并打开 shelf（mobile-runtime.ts:242-247），而注释仍写「undefined
unless authorized」。实现者按此注释会误判 read-only 下 shelf 不可用。建议同步为「authorized 或 read-only surface 且提供
ports.resourceShelf 时可用」。

-   /** Resource Shelf for the active scope; undefined unless authorized with ports.resourceShelf provided. */
+   /** Resource Shelf for the active scope; undefined unless the surface is 'authorized' or 'read-only' with ports.resourceShelf provided. */
    resourceShelf(): ResourceShelfHandle | undefined;


─── apps/mobile/src/runtime-integration-smoke.ts:165-169 ───
[maintainability · low] alt 部署 URL 的校验逻辑（new URL 解析 + 协议/凭据/pathname/search/hash 检查 + 主机防线）与上方主 URL
的校验（第 138-148 行）除变量名外逐行重复，约 14 行。本次改动已将 disallowedDeploymentHost 的错误消息参数化以复用主机防线，但更外层的整段 origin
校验仍是复制粘贴：后续若调整校验规则（如放行自定义 scheme、增加端口限制）或新增第三个环境变量，需要多处同步修改，容易漂移。建议提取一个以变量名为参数的辅助函数（与
disallowedDeploymentHost 的参数化方向一致），两个调用点各自传入 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL' /
'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL'。

-     if (parsedAlt.protocol !== 'https:' || parsedAlt.username || parsedAlt.password || parsedAlt.pathname !== '/' || parsedAlt.search || parsedAlt.hash) {
-       return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
+ function parseCredentialFreeHttpsOrigin(raw: string, variable: string): { origin: string } | { reason: string } {
+   let parsed: URL;
+   try {
+     parsed = new URL(raw);
+   } catch {
+     return { reason: `${variable} is not an absolute URL` };
-     }
+   }
-     const altHostRejection = disallowedDeploymentHost(parsedAlt.hostname, 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL');
-     if (altHostRejection) return { enabled: false, disposition: 'invalid', reason: altHostRejection };
+   if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
+     return { reason: `${variable} must be a credential-free HTTPS origin` };
+   }
+   const hostRejection = disallowedDeploymentHost(parsed.hostname, variable);
+   if (hostRejection) return { reason: hostRejection };
+   return { origin: parsed.origin };
+ }
+ 
+ // 主/alt 两处调用：
+ const mainOrigin = parseCredentialFreeHttpsOrigin(deploymentOrigin, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
+ if ('reason' in mainOrigin) return { enabled: false, disposition: 'invalid', reason: mainOrigin.reason };
+ const altOrigin = parseCredentialFreeHttpsOrigin(altOriginRaw, 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL');
+ if ('reason' in altOrigin) return { enabled: false, disposition: 'invalid', reason: altOrigin.reason };


LLM retry report summary: 8 of 213 requests affected -- 2 requests failed, 6 requests recovered after retry

Review planning (2 requests):
- packages/api-client/src/mobile/task-office.ts,packages/contracts/src/mobile/execution.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/mobile-core/src/task-office/in-memory-task-detail.ts,packages/mobile-core/src/task-office/task-detail.ts,packages/mobile-core/src/task-office/task-office.ts,packages/mobile-core/src/task-office/task-timeline.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Core review (6 requests):
- packages/mobile-core/src/task-office/in-memory-task-detail.ts,packages/mobile-core/src/task-office/task-detail.ts,packages/mobile-core/src/task-office/task-office.ts,packages/mobile-core/src/task-office/task-timeline.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks.tsx,apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx,apps/mobile/src/task-detail-integration-smoke.ts,apps/mobile/src/task-detail-view.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- packages/api-client/src/mobile/task-office.ts,packages/contracts/src/mobile/execution.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- packages/mobile-core/src/index.ts,packages/mobile-core/src/runtime/in-memory-adapters.ts,packages/mobile-core/src/runtime/mobile-runtime.ts,packages/mobile-core/src/runtime/ports.ts,packages/mobile-core/src/runtime/types.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- packages/mobile-core/src/index.ts,packages/mobile-core/src/runtime/in-memory-adapters.ts,packages/mobile-core/src/runtime/mobile-runtime.ts,packages/mobile-core/src/runtime/ports.ts,packages/mobile-core/src/runtime/types.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 1 more

Per-attempt detail: --format json (retry_report).
