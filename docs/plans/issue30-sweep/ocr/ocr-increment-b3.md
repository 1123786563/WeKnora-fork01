Review complete: 88 finding(s) across 95 selected item(s).

─── packages/mobile-core/src/inbox/notification-inbox.ts:154-156 ───
[bug · medium] markRead 未参与 sequence 票据机制，存在丢失更新竞态：若一次 page()/more() 在途（ticket=N，sequence 仍为 N）时
markRead 完成并发布本地已读投影，随后在途的 GET 快照（可能取自 POST 落库之前）settle 时 ticket===sequence 仍通过，merge 会用旧快照覆盖视图，把已置
read=true 的行回退为未读、unreadCount 回升，直到下一次刷新才自愈。模块其余路径（fetchPage/settle）都按票据拒绝迟到结果，markRead 是唯一旁路。建议
markRead 同样 `++sequence` 领取票据，使在途旧读按 SUPERSEDED 拒绝，且发布前校验票据未被更新。

+       const ticket = ++sequence;
        await callRemote(() => ports.remote.markRead(trimmed));
        if (!leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
+       // markRead 也领取票据：在途 page()/more() 的旧快照按 SUPERSEDED 拒绝，不得回写覆盖已读投影。
+       if (ticket !== sequence) return;
        if (view !== undefined) {


─── packages/mobile-core/src/inbox/notification-inbox.ts:129-133 ───
[bug · low] merge 未把空字符串 nextCursor 归一为 undefined：view.nextCursor 与模块 cursor 会变成 ''，而 more() 仅判断
`cursor === undefined`，'' 会被当作有效游标传给 remote。api-client 与 in-memory 两个 remote 都会把 '' 归一掉（丢弃 cursor →
重新拉第一页），导致分页在第一页循环、永不终止。目前两个内置 remote 的归一掩盖了这一点，但 InboxRemote 是对外 seam，任意第三方实现返回
next_cursor:''（语义为"没有下一页"）即触发。建议在模块入口统一归一：'' 等价 undefined。

-     cursor = page.nextCursor;
+     const next = page.nextCursor === '' ? undefined : page.nextCursor;
+     cursor = next;
      return publish({
        items,
        unreadCount: page.unreadCount,
-       ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
+       ...(next === undefined ? {} : { nextCursor: next }),


─── packages/api-client/src/mobile/inbox.ts:37-38 ───
[maintainability · low] requireDeploymentOrigin 已是 packages/api-client/src/mobile/ 下第 7
份逐字拷贝（devices / legacy-tasks / materials / resources / runtime / task-office / 本文件），且
resources.test.ts 的防漂移回归只覆盖其中部分文件。任一处规则演进（如新增对端口的限制）都要手工同步 7 份。建议抽取为共享 helper（例如
packages/api-client/src/mobile/origin.ts 导出 requireDeploymentOrigin，返回校验后的 origin
供需要返回值的调用方使用），各适配器统一引用；若维持逐字拷贝是有意的自包含模式，至少应为新增副本补充同款防漂移断言。

- function requireDeploymentOrigin(origin: string): void {
-   let parsed: URL;
+ // origin.ts（共享）：
+ // export function requireDeploymentOrigin(origin: string): string {
+ //   ...原校验逻辑，返回 origin...
+ // }
+ // inbox.ts / devices.ts / legacy-tasks.ts / materials.ts / resources.ts / runtime.ts / task-office.ts 统一 import 复用。


─── packages/mobile-core/src/task-office/attention-inbox.ts:120-120 ───
[maintainability · low] inbox() 中 limit: 50 为裸业务数字，且默认值 50 与上限 200 目前在三层各自硬编码：mobile-core
此处、api-client interactions.ts 的默认 50/上限 200 校验、Go handler 与 ListPending 的
clamp。任何一层调整都会静默失配。建议导出命名常量（如 ATTENTION_INBOX_DEFAULT_LIMIT = 50）并由 api-client 层透传，消除魔法数字。

-         items = await backend.inbox({ limit: 50 });
+ export const ATTENTION_INBOX_DEFAULT_LIMIT = 50;
+ // ...
+         items = await backend.inbox({ limit: ATTENTION_INBOX_DEFAULT_LIMIT });


─── packages/mobile-core/src/device/device-registry.ts:129-133 ───
[bug · medium] revoke 的 409（revision 冲突）被归入 DEVICE_BACKEND 而非 DEVICE_CONFLICT，与 register 路径不对称：服务端
RevokeForTenant 的 revision 不匹配返回 409（internal/handler/mobile_device.go:404-405
ErrMobileDeviceRevision → StatusConflict），本仓库的 in-memory 替身（in-memory-device-remote.ts revoke 的
'mobile device revision conflict'）也复刻了该语义。当前映射使带 revision 的乐观撤销在并发冲突时与真实后端故障不可区分——调用方无法对
DEVICE_CONFLICT 做重取 revision 重试或提示，而 DEVICE_BACKEND 按约定不可重试。且 device-registry.test.ts 的 revoke 用例仅覆盖
404 与 lease 撤销，未覆盖 409。建议补 409 → DEVICE_CONFLICT 映射并补测试。

          } catch (error) {
            if (error instanceof DeviceError) throw error;
            if (wireStatus(error) === 404) throw new DeviceError('DEVICE_NOT_FOUND', { cause: error });
+           if (wireStatus(error) === 409) throw new DeviceError('DEVICE_CONFLICT', { cause: error });
            throw new DeviceError('DEVICE_BACKEND', { cause: error });
          }


─── packages/api-client/src/mobile/devices.ts:103-104 ───
[bug · low] register 响应中 revision/scope_generation 用 Number() 强转且无类型与整数校验，与同文件另两处解析风格不一致：issueIntent
严格校验 Number.isSafeInteger，list() 用 typeof === 'number' 检查（异常类型回落 0），而 register 处 Number("12")
会静默转换、非数字字符串（如脏值 "1e999"、"abc"）会产生 NaN/Infinity 悄悄进入 MobileDeviceRegistration 并向 UI 层传播。建议统一为 typeof
+ Number.isSafeInteger 防御式校验。

-         revision: Number(data.revision ?? 0),
-         scopeGeneration: Number(data.scope_generation ?? 0),
+         revision: typeof data.revision === 'number' && Number.isSafeInteger(data.revision) ? data.revision : 0,
+         scopeGeneration: typeof data.scope_generation === 'number' && Number.isSafeInteger(data.scope_generation) ? data.scope_generation : 0,


─── apps/mobile/src/adapters/device-identity.ts:44-47 ───
[other · low] nativeDevicePlatform 在 require('react-native') 失败或 Platform.OS 非 ios 时一律回落
'android'（fail open），与同文件 deviceId 的 fail-closed 策略（无安全存储即返回 undefined、不注册）不一致：非 RN
运行时/意外环境下仍会完成注册，并在服务端注册记录中把实际平台误标为 android（服务端 platform 列入库持久化）。若注册记录的平台准确性有意义，建议解析失败时返回 undefined
并由调用方跳过注册，与 deviceId 的围栏对齐；若确属有意（真机仅有 ios/android 两个值），建议在注释中补充误标影响说明。



─── packages/mobile-core/src/device/in-memory-device-remote.ts:42-44 ───
[test · low] intent 校验依赖单一全局 epoch：为设备 B issueIntent 会使设备 A 尚未使用的 intent
立即失效（`intent:${deviceId}:${epoch}` 中 epoch 已被 B 推进）。而服务端的 CurrentScopeGeneration 按 (tenant, owner,
device) 逐设备隔离（mobile_device.go:144/203 将 device 作为查询键），A 的 intent 在 B 签发后依然有效。经 registry
单飞串行调用的测试不受影响，但直接驱动 scenario remote 的多设备交错用例会得到与服务端相悖的结果，替身保真度失真。建议按 deviceId 维护各自的
epoch（Map<string, number>），与 per-device 隔离语义对齐。



─── packages/api-client/src/mobile/devices.ts:123-126 ───
[maintainability · low] list() 内联的 success envelope 头部校验与 unwrap() 前半段逐行重复（response 必须是对象、success
必须为 true），仅 data 形状（array vs object）不同。两处独立维护易漂移（例如后续给 envelope 增加统一字段时只改一边）。建议提取
requireSuccessEnvelope(value, label): unknown 供 unwrap 与 list 共用，list 再对返回值做 Array.isArray 检查。



─── apps/mobile/src/adapters/material-adapters.ts:9-10 ───
[security · low] 免凭据抓取适配器放行 http: 且 fetch 无超时。核实：mobile-core 的 mintGrant 已把 grant URL 钉死在部署
origin（部署面恒为 https），http 链接会被 MATERIAL_GRANT_ORIGIN 拒绝，故此处 http 分支在组合应用中实际不可达——但作为 BlobFetchPort
的通用实现，保留 http 削弱纵深防御（任何新调用方都可能传入明文链接，而签名 URL 内含 HMAC 令牌）；同时无 AbortSignal 超时，CDN/存储挂起会让材料页 loading
永久停留且无法取消。下载大小已由 mobile-core PREVIEW_MAX_BYTES(2MB) 预览判定兜底，无需在此重复限制。建议收紧为 https-only 并加超时。

-       if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('blob url must use http(s)');
-       const response = await fetch(url);
+       if (parsed.protocol !== 'https:') throw new Error('blob url must use https');
+       const response = await fetch(url, { signal: AbortSignal.timeout(30_000) });


─── packages/api-client/src/mobile/materials.ts:30-32 ───
[maintainability · medium] requireDeploymentOrigin 是 mobile 目录下第 7 份逐字相同的实现（devices.ts / inbox.ts /
legacy-tasks.ts / materials.ts 为本批新增，resources.ts / runtime.ts / task-office.ts
为既有）。校验规则一旦演进（例如需禁用特定 host 后缀、放行 localhost 调试面），需要 7 处同步修改，漏改任意一处即产生不一致的 origin 防线。建议提取到共享模块（如
packages/api-client/src/mobile/origin.ts）统一导出，各 Remote 复用同一实现。

- function requireDeploymentOrigin(origin: string): string {
-   let parsed: URL;
-   if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
+ import { requireDeploymentOrigin } from './origin.ts';
+ // origin.ts: 单一权威实现，devices/inbox/legacy-tasks/resources/runtime/task-office 等统一复用


─── apps/mobile/src/screens/MaterialsScreen.tsx:97-104 ───
[style · low] diff 视图渲染存在三处规范问题：(1) 行前缀使用嵌套三元表达式（团队规范禁止）；(2) `key={hunk.header}` —— hunk 头文本在畸形 diff
中可能重复（parseUnifiedDiff 对 malformed 输入仍会产出 hunk 列表），React key 冲突会导致渲染告警与复用异常，内层 lines 已用位置下标做
key，外层应保持一致；(3) Image 使用静态 inline style（width: 320, height: 240 非动态值），应移入 StyleSheet 或常量。

-         {view.hunks.map((hunk) => (
-           <View key={hunk.header}>
+ const DIFF_LINE_PREFIX: Record<DiffLine['origin'], string> = { add: '+', remove: '-', context: ' ' };
+ // ...
+         {view.hunks.map((hunk, hunkIndex) => (
+           <View key={hunkIndex}>
              <Text>{hunk.header}</Text>
              {hunk.lines.map((line, position) => (
-               <Text key={position}>{line.origin === 'add' ? '+' : line.origin === 'remove' ? '-' : ' '}{line.text}</Text>
+               <Text key={position}>{DIFF_LINE_PREFIX[line.origin]}{line.text}</Text>
              ))}
            </View>
          ))}


─── apps/mobile/src/material-integration-smoke.ts:71-72 ───
[test · low] signIn 返回非 authorized 面时直接 return，evidence 维持 listed:'failed' 但 failure
字段为空——真实环境排查时无法区分"登录被拒（凭据错误/协议降级/维护面）"与其他步骤失败，与该冒烟"失败必留摘要"的自述语义不符。建议在该分支补一条不含凭据字段的 failure 摘要（可附
snapshot.surface 便于定位）。另已核实 disallowedDeploymentHost(hostname, variable) 符号存在且签名兼容，跨文件引用无问题。

-     const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
-     if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;
+     if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
+       evidence.failure = `sign-in rejected (surface: ${snapshot.surface})`;
+       return evidence;
+     }


─── apps/mobile/src/attention-inbox-view.ts:12-17 ───
[bug · low] 错误文案映射缺少 TASK_OFFICE_SUPERSEDED。createAttentionDecider.inbox() 在两次读取竞态时（如 decide 成功后自动
load() 期间用户点击 Refresh，后者递增 inbox epoch）会以 TASK_OFFICE_SUPERSEDED
拒绝较早的一次（packages/mobile-core/src/task-office/attention-inbox.ts:125），经 messageOf 的 `?? failure.code`
兜底后原始错误码 "TASK_OFFICE_SUPERSEDED" 会直接显示给用户。该映射是 Record<string, string>，编译器无法穷举校验，请补齐可达错误码的文案。

  export const ATTENTION_INBOX_ERROR_COPY: Record<string, string> = {
    TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
    TASK_OFFICE_INTERACTIONS_UNAVAILABLE: '当前部署未提供交互决定通道。',
    TASK_OFFICE_INVALID_INPUT: '该动作与请求类型不匹配。',
+   TASK_OFFICE_SUPERSEDED: '已有更新的收件箱读取，正在以最新结果为准。',
    TASK_OFFICE_BACKEND: '服务端暂时不可用，请稍后重试。',
  };


─── apps/mobile/src/attention-inbox-view.ts:38-41 ───
[style · low] messageOf 使用两层嵌套三元（TaskOfficeError / Error / String
三分支混在一条链上），违反「禁止嵌套三元表达式」的代码规范，可读性差且后续追加分支时易引入优先级错误。建议改为独立的 if 语句或函数。

- const messageOf = (failure: unknown): string =>
-   failure instanceof TaskOfficeError ? (ATTENTION_INBOX_ERROR_COPY[failure.code] ?? failure.code)
-     : failure instanceof Error ? failure.message
-       : String(failure);
+ function messageOf(failure: unknown): string {
+   if (failure instanceof TaskOfficeError) return ATTENTION_INBOX_ERROR_COPY[failure.code] ?? failure.code;
+   if (failure instanceof Error) return failure.message;
+   return String(failure);
+ }


─── apps/mobile/src/screens/AttentionInboxScreen.tsx:39-41 ───
[bug · low] decide 动作按钮在请求在途期间与列表 loading 期间均未禁用（Refresh 按钮有
disabled={state.loading}，动作按钮没有）。快速连点同一动作时，controller 侧 frozen decisionId（mobile-core
attention-inbox.ts:135-137）保证服务端幂等重放同一 durable 身份，不会造成重复决定，但每次成功都会向 state.receipts 追加一行，产生多条相同
receipt 文案；弱网下还会对不同动作（如 approve+reject 连点）制造本可避免的竞态。建议至少加 disabled={state.loading}，更稳妥的做法是在
controller 内按 interactionId 维护在途锁并在 decide 期间置位。

            {INTERACTION_ACTIONS[item.kind].map((action) => (
-             <Button key={action} title={ACTION_LABELS[action]} onPress={() => onDecide(item, action)} />
-           ))}
+             <Button key={action} title={ACTION_LABELS[action]} disabled={state.loading} onPress={() => onDecide(item, action)} />
+           ))


─── apps/mobile/src/app/inbox.tsx:57-64 ───
[style · low] notice 文案使用两层嵌套三元（invalid-link / blocked-unauthorized /
undefined），违反「禁止嵌套三元表达式」的代码规范。outcome 是三值枚举，建议改为查表映射。

        setState((current) => ({
          ...current,
-         notice: outcome === 'invalid-link'
-           ? '该通知的链接无法安全打开。'
-           : outcome === 'blocked-unauthorized'
-             ? '请先登录并激活空间，再打开该任务。'
-             : undefined,
+         notice: outcome === 'navigated' ? undefined : OPEN_OUTCOME_COPY[outcome],
        }));
+ // 文件顶部：const OPEN_OUTCOME_COPY: Record<'invalid-link' | 'blocked-unauthorized', string> = {
+ //   'invalid-link': '该通知的链接无法安全打开。',
+ //   'blocked-unauthorized': '请先登录并激活空间，再打开该任务。',
+ // };


─── apps/mobile/src/app/inbox.tsx:37-43 ───
[bug · low] refresh() 重置了 error 但未重置 notice：用户打开一条 invalid-link 通知后（notice
显示「该通知的链接无法安全打开。」），再点刷新，旧提示会残留在新数据之上，造成提示与当前列表状态不符。另外其失败回调没有 effect 内的 alive 卫护（卸载后仍 setState），与同文件
useEffect 中的处理不一致，建议一并对齐。

    const refresh = (): void => {
-     setState((current) => ({ ...current, loading: true, error: undefined }));
+     setState((current) => ({ ...current, loading: true, error: undefined, notice: undefined }));
      void inbox.page().then(
        () => undefined,
        (failure) => { setState((current) => ({ ...current, loading: false, error: errorMessage(failure) })); },
      );
    };


─── apps/mobile/src/app/inbox.tsx:15-17 ───
[maintainability · low] 此处是 errorMessage 帮助函数在 apps/mobile 内的第 4
处副本（screens/HomeScreen.tsx:17、screens/TasksScreen.tsx:19、legacy-tasks-view.ts:26
均有同一定义），本批新增后重复继续扩散。建议上提到共享模块（如 composition.ts 或独立 view 工具文件）统一导出。



─── apps/mobile/src/app/attention.tsx:8-8 ───
[maintainability · low] 本文件导出的宿主组件与 apps/mobile/src/app/inbox.tsx 导出的 InboxRouteLifecycle
同名但语义不同（本文件是审批收件箱 /attention，inbox.tsx 是行动通知 /inbox），且 app-smoke.test.tsx:1163 按 inbox.tsx
的同名组件做行为测试。两个同名导出并存于相邻路由文件，易在 import 时错拿、增加维护混淆。建议按域重命名为 AttentionInboxRouteLifecycle（与文件内其余
Attention 前缀命名一致）。

- export function InboxRouteLifecycle() {
+ export function AttentionInboxRouteLifecycle() {


─── apps/mobile/src/screens/LegacyTasksScreen.tsx:51-52 ───
[bug · medium] 跨卡片状态串扰：`question` 是整屏唯一的一份 state，却在 `view.items.map` 内为每张卡片渲染受控
TextInput/发送按钮。在任一卡片输入会同步显示到所有卡片输入框；`setQuestion('')` 清空全部卡片；任一追问
in-flight（followUpState==='sending'）会禁用全部发送按钮；"Follow-up sent" 与 followUpError 也会渲染在所有无关卡片下方。回执类状态应按
taskId 维度组织（history 已按 taskId 归属，question/followUp 应对齐同一模式）。

-           <TextInput value={question} onChangeText={setQuestion} placeholder="Continue with an ordinary follow-up" />
-           <Button title="Send follow-up" disabled={view.followUpState === 'sending' || question.trim() === ''} onPress={() => { void controller?.submitFollowUp(card.taskId, question); setQuestion(''); }} />
+ // 草稿按 taskId 维护，避免跨卡片串扰：
+ const [drafts, setDrafts] = useState<Record<string, string>>({});
+ // ...
+ <TextInput value={drafts[card.taskId] ?? ''} onChangeText={(text) => setDrafts((prev) => ({ ...prev, [card.taskId]: text }))} placeholder="Continue with an ordinary follow-up" />
+ <Button title="Send follow-up" disabled={view.followUpState === 'sending' || (drafts[card.taskId] ?? '').trim() === ''} onPress={() => { void controller?.submitFollowUp(card.taskId, drafts[card.taskId] ?? ''); setDrafts((prev) => ({ ...prev, [card.taskId]: '' })); }} />


─── apps/mobile/src/screens/LegacyTasksScreen.tsx:21-22 ───
[bug · medium] 无授权面时屏幕永久停留在 "Loading"：useEffect 依赖为空数组，`activeTaskOffice()` 返回 undefined 即静默
return，state 保持初始 `loading: true`，且后续授权面就绪也不会重建 controller。这与 legacy.tsx 路由注释宣称的"无授权面时屏内如实空态"直接矛盾；同批
attention.tsx:13-15 的既有模式是在 `!office` 时显式 `setState({ loading: false, error: '请先登录…' })` 呈现如实空态，应对齐。

      const office = activeTaskOffice();
-     if (!office) return;
+     if (!office) {
+       setView((prev) => ({ ...prev, loading: false, error: '请先登录并激活空间，再打开 Legacy 任务列表。' }));
+       return;
+     }


─── apps/mobile/src/legacy-tasks-view.ts:79-80 ───
[bug · medium] 追问成功后的列表刷新失败会把 followUpState 永久卡死在 'sending'：`office.followUp` 自身的失败在 action 内被捕获并复位为
idle，但随后 `office.legacyTasks({})` 刷新若抛错（网络失败、TASK_OFFICE_SCOPE_CHANGED 等），会落入 `run` 的统一 catch，只发布 `{
loading: false, error }` 而不复位 followUpState——此时 followUp 实际已提交成功，但界面停在
sending、发送按钮持续禁用且无任何恢复路径（reload 也不复位该字段）。

+       try {
-       const page = await office.legacyTasks({});
+         const page = await office.legacyTasks({});
-       return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
+         return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
+       } catch {
+         // 追问已提交成功；列表刷新失败不应把回执卡死在 'sending'
+         return { followUpState: 'sent' as const, followUpError: undefined, loading: false };
+       }


─── packages/api-client/src/mobile/legacy-tasks.ts:80-80 ───
[bug · medium] history 硬编码 `limit=20` 且无翻页通道：服务端 `/messages/:id/load` 按 `limit + before_time`
分页（internal/handler/message.go:76），同包既有聊天客户端 `chat/sessions.ts:111-112` 将两者都参数化；本适配器把 limit 固定为 20
且不透传 before_time，超过 20 条消息的旧会话历史被静默截断，与域层 `legacyHistory(taskId): Promise<LegacyMessage[]>`
的"历史"语义不符（仓内惯例是显式暴露截断，如 query-history 的 truncated:true）。建议至少参数化 limit/beforeTime，或在端口契约上显式声明截断上限。

-         path: `/api/v1/messages/${encodeURIComponent(trimmed)}/load?limit=20`,
+     async history(taskId: string, options: { limit?: number; beforeTime?: string } = {}): Promise<RemoteLegacyMessage[]> {
+       // ...
+       const query = new URLSearchParams({ limit: String(options.limit ?? 20) });
+       if (options.beforeTime) query.set('before_time', options.beforeTime);
+       const messages = parseChatMessageListResponse(await request({
+         method: 'GET',
+         path: `/api/v1/messages/${encodeURIComponent(trimmed)}/load?${query.toString()}`,
+       }));


─── packages/api-client/src/mobile/legacy-tasks.ts:21-23 ───
[maintainability · low] 重复实现：`requireDeploymentOrigin` 这份 origin 强校验在
`packages/api-client/src/mobile/` 下已是第 7
份拷贝（runtime/resources/task-office/devices/inbox/materials/legacy-tasks 逐字重复，含相同的
HTTPS/user-info/path/query/fragment 检查）。本批新增 5 份后继续手写拷贝会放大将来校验规则变更（如需收紧 host
防线）时的遗漏面，建议在包内提取共享校验助手（如 `mobile/origin.ts`）供各适配器复用。

- function requireDeploymentOrigin(origin: string): string {
-   let parsed: URL;
-   if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
+ // packages/api-client/src/mobile/origin.ts
+ export function requireDeploymentOrigin(origin: string): string { /* 统一实现（现 legacy-tasks.ts:22-33 的逻辑） */ }
+ 
+ // legacy-tasks.ts 等适配器改为：
+ import { requireDeploymentOrigin } from './origin.ts';


─── apps/mobile/src/composition.ts:164-164 ───
[bug · medium] 四个模块级缓存(taskOffices/deviceRegistries/notificationInboxes/taskMaterials)仅以 deployment
origin 为键,且全文件无任何 delete/clear 失效路径。已核实:office/inbox 内层的 requireLease() 只校验 lease
活性——activateTenant/switchDeployment/重登录会撤销旧 lease 并签发新 lease,同一实例随即以新 scope 继续工作,实例内的
submissionStore、defaultDetailStore 内存投影、accumulated/legacyAccumulated、inbox 的 view/seen/cursor 全部跨
user/tenant 残留。同 origin 下登出用户 A 再登录用户 B 时同样复用含 A 残留状态的实例。当前隔离完全依赖组件层 deploymentScopeKey(origin,
tenantId) 重挂载键与「跨 scope 同 ID 碰撞概率低」,而非缓存键本身,违背本批「缓存按 Deployment/user/Tenant 隔离」的显式约束。建议将 Map 键升级为
origin+userId+activeTenantId(复用 deploymentScopeKey 思路),或在 Runtime scope 撤销发布时清除对应 origin 的缓存条目。

+ // 以 scope 为缓存键(与组件层 deploymentScopeKey 同粒度),登出/切租户自然换实例
+ const scopeKey = `${origin}\n${snapshot.identity.userId}\n${snapshot.identity.activeTenantId ?? ''}`;
  const deviceRegistries = new Map<string, DeviceRegistry>();


─── apps/mobile/src/adapters/request-id.ts:11-15 ───
[bug · medium] CSPRNG 路径依赖 globalThis.crypto,但已搜索确认 apps/mobile 与 packages 全仓库均未安装/注册任何 crypto
polyfill(expo-crypto、react-native-get-random-values 均无),Hermes 默认不暴露 Web Crypto 全局。若无
polyfill,注释宣称的「优先平台 CSPRNG」永远走不通,Math.random 兜底将成为唯一实际路径,幂等关联键强度静默退化。且这与内核纪律不一致:mobile-core 默认的
nextRequestId(task-office.ts:293-294)在同场景选择 fail closed 抛错而非降级。建议注册保证可用的 polyfill(如 expo-crypto 的
randomUUID),或与内核一致 fail closed,或至少在兜底分支留下可观测告警。



─── apps/mobile/src/composition.ts:355-357 ───
[bug · medium] 设备注册以 origin 为粒度「先标记、后尝试、永不重试」:registeredFor.add 在调用前执行,'no-token' 与 'failed'
结果同样永久占用该 origin 的注册机会。iOS 首次安装时通知权限未授予,getDevicePushTokenAsync 必然失败返回 no-token(push-token.ts catch
→ undefined),此后整个 App 会话不再有任何注册路径;已确认 registerActiveDeviceIfPossible 全仓库仅此一处调用,既无通知权限授予后的补偿注册,也无
push token 轮换的刷新入口——iOS 上设备大概率永远注册不上,行动通知收不到,与「注册设备与 App 生命周期」的目标相悖。建议仅在结果为 'registered' 时标记
origin,并对 no-token/failed 保留后续重试时机(如通知权限状态变化、App 前台恢复事件);另注意默认参数
createNativePushTokenIfAvailable()/createNativeDeviceIdentity() 在每次调用时都会重新求值,可提升为模块级常量。

        if (registeredFor.current.has(next.deployment.origin)) return;
-       registeredFor.current.add(next.deployment.origin);
-       void registerActiveDeviceIfPossible(activeRuntime).catch(() => undefined);
+       void registerActiveDeviceIfPossible(activeRuntime)
+         .then((outcome) => {
+           if (outcome === 'registered') registeredFor.current.add(next.deployment.origin);
+         })
+         .catch(() => undefined);


─── apps/mobile/src/composition.ts:204-204 ───
[maintainability · medium] 参数类型 Pick<InboxItem,'notificationId'> 与真实实现不符:已核实 notification-inbox.ts 的
resolveTarget 读取 item.deepLink(缺失时按空串解析返回 undefined)。若调用方按声明类型只传 notificationId,此处 as InboxItem 强转后
resolveTarget 会静默落入 'invalid-link' 分支——既不导航也不报错,类型系统无法拦截;app-smoke 测试里的 stub inbox(不读
deepLink)恰好掩盖了这一缺口。当前生产调用方 inbox.tsx 传全量对象才未触发。作为「通知点击唯一安全入口」的函数,建议把参数类型收紧为完整 InboxItem(或 Pick 中包含
deepLink),删除 as 断言,让编译器守住该契约。

-   const target = inbox.resolveTarget(item as InboxItem);
+   item: Pick<InboxItem, 'notificationId' | 'deepLink'>,
+   // ...
+   const target = inbox.resolveTarget(item as InboxItem); // 或直接改为接收完整 InboxItem,消除断言


─── apps/mobile/src/composition.ts:207-207 ───
[maintainability · low] 路由字符串 '/tasks/detail' 硬编码:该路径需要与 expo-router
文件路由(app/tasks/detail.tsx)、deep-link.ts 的 weknora://tasks/detail 契约、app/tasks.tsx 的 router.push
字面量保持同步,目前全靠注释约定且散落多处,路由文件移动或重命名时这里会静默断链。建议提取集中路由常量(如 export const TASK_DETAIL_ROUTE =
'/tasks/detail')供 composition 与各路由调用方复用。

-   push('/tasks/detail', { taskId: target.taskId, runId: target.runId });
+   push(TASK_DETAIL_ROUTE, { taskId: target.taskId, runId: target.runId }); // 常量集中定义并与 deep-link 契约注释互引


─── packages/mobile-core/src/material/task-material.ts:67-67 ───
[performance · medium] blobs 字节缓存按 materialId@version 无上限累积（单条目最高 2MB），仅在 close()
时清理。长会话内连续打开多个材料会在移动端造成持续内存增长（20 个预览即可驻留 ~40MB），且句柄存活期与任务会话同长。建议增加淘汰策略：按条目数或总字节数设上限（简单 LRU：超限逐出最旧
key），与 close() 的整表清理并存。

-       const blobs = new Map<string, Uint8Array>(); // key: `${materialId}@${version}`
+       const blobs = new Map<string, Uint8Array>(); // key: `${materialId}@${version}`；LRU 淘汰：超限（如 8 条 / 16MB）时逐出最旧 key
+       const BLOB_CACHE_MAX_ENTRIES = 8;
+       const cacheBlobs = (key: string, bytes: Uint8Array): void => {
+         blobs.delete(key);
+         blobs.set(key, bytes);
+         while (blobs.size > BLOB_CACHE_MAX_ENTRIES) {
+           const oldest = blobs.keys().next().value;
+           if (oldest === undefined) break;
+           blobs.delete(oldest);
+         }
+       };


─── packages/mobile-core/src/material/task-material.ts:121-122 ───
[bug · low] 预览大小上限仅校验后端声明的 entry.size，fetchBytes 返回的真实字节长度在入缓存与 decodeUtf8 之前未复检
PREVIEW_MAX_BYTES。后端口径错误或 grant
指向内容被替换时，超限字节仍会被缓存、解码并整串进入视图，绕过内联预览的大小约束（也放大了上一条缓存问题的最坏情况）。建议在缓存前复检实际长度，超限即按 size 不支持降级或抛
MATERIAL_BACKEND（含声明值与实际值），且不缓存。

+         if (fetched.bytes.length > PREVIEW_MAX_BYTES) {
+           throw new MaterialError('MATERIAL_BACKEND', { cause: new Error(`artifact size mismatch: declared ${entry.size}, actual ${fetched.bytes.length}`) });
+         }
          blobs.set(cacheKey, fetched.bytes);
          return fetched.bytes;


─── packages/mobile-core/src/material/task-material.ts:201-202 ───
[performance · low] act 在 mintGrant（网络往返 + 铸造短时效签名 URL）之后才检查 ports.share 是否存在：当系统未接分享通道时，注定失败的 share
也会先铸造一枚有效凭据，浪费往返且多暴露一个短时效 URL。分享可用性是静态事实，应在进入任何远程调用前判定。

+           if (intent.kind === 'share' && ports.share === undefined) throw new MaterialError('MATERIAL_SHARE_UNAVAILABLE');
            const entry = await entryFor(runId, materialId);
            const grant = await mintGrant(runId, entry); // 每次 act 新鲜铸造，不复用任何旧 URL（AC1）


─── packages/mobile-core/src/material/task-material.ts:31-32 ───
[bug · low] decodeUtf8 的回退分支用 String.fromCharCode 逐字节拼接，这是 Latin-1 解码而非 UTF-8：在缺少 TextDecoder
的运行时上，非 ASCII 的 diff/文本内容会静默渲染为乱码，无任何降级提示。而 apps/mobile 的 sse-stream.ts 已无条件使用 new
TextDecoder()，说明移动端运行时假定该 API 存在——回退分支与全局假设不一致且行为错误。建议要么删除回退、缺失时大声失败（与「宁可回退原文也不产出错位结果」的纪律一致），要么移植正确的
UTF-8 解码（仓库内 apps/miniprogram/src/core/utf8.ts 已有严格 UTF-8 实现可参照）。

-   let out = '';
-   const CHUNK = 0x8000;
+ function decodeUtf8(bytes: Uint8Array): string {
+   if (typeof TextDecoder !== 'undefined') return new TextDecoder('utf-8').decode(bytes);
+   throw new Error('TextDecoder unavailable: cannot decode artifact bytes as UTF-8');
+   // 或移植 apps/miniprogram/src/core/utf8.ts 的严格 UTF-8 解码作为回退
+ }


─── packages/mobile-core/src/material/evidence.ts:25-30 ───
[performance · low] 注释称 detail 携带「原始载荷摘要」，实现是对 payload 整包 JSON.stringify 且无截断：大载荷（如含 base64 的 tool
输出）会完整进入 detail，随 citations 数组进入视图状态，带来内存与列表渲染压力，注释与行为也不一致。建议对 detail 设长度上限（如 8KB）并追加截断标记，同步修正注释。

      let detail: string;
      try {
        detail = JSON.stringify(event.payload ?? {});
+       if (detail.length > DETAIL_MAX_CHARS) detail = `${detail.slice(0, DETAIL_MAX_CHARS)}…[truncated]`;
      } catch {
        detail = '[unserializable]';
      }
+ // const DETAIL_MAX_CHARS = 8 * 1024;


─── packages/mobile-core/src/material/task-material.ts:168-168 ───
[maintainability · low] 运行时防御以 page.nextCursor === undefined 判缺省，但 ports.ts 的
MaterialBackendTerminalPage.nextCursor 声明为非可选 number（真实 api-client 适配器也恒产出
number）——类型合同与防御口径不一致，undefined 分支按合同是死代码，而适配器若返回 null 会绕过判断被原样写入视图。建议统一合同：要么 ports.ts 改为
nextCursor?: number 与此处防御对齐，要么此处改为 typeof page.nextCursor === 'number' 的窄化校验（同时过滤 null/NaN）。

-               ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
+               ...(typeof page.nextCursor === 'number' && Number.isSafeInteger(page.nextCursor) ? { nextCursor: page.nextCursor } : {}),
+ // 并在 ports.ts 将 nextCursor 声明为可选（nextCursor?: number）以匹配运行时防御口径


─── apps/mobile/src/new-task-view.ts:86-87 ───
[bug · high] 跨重启恢复的未决意图未回填 intentRequestId，重试将违反 D5「绝不换 ID 重建任务」。已核实 task-office.ts:472 的 start() 在
options.requestId 缺省时直接 nextRequestId() 生成新 ID，且无按 goal 摘要（goalKeyOf 仅用于同 ID
重入校验）自动复用逻辑：intentLog.load(新ID)=undefined 即走新意图路径——新建 session 并重新派发。而本处只把 unresolved receipt 写入
state.inFlight，intentRequestId 仍为 undefined；用户在恢复出的未决意图上按 Submit 时，submit() 走 `intentRequestId ===
undefined ? {} : ...` 分支，同一意图将以新 request_id 重建任务并产生第二次派发，旧 intent log 记录成为孤儿。

        const unresolved = pending.find((receipt) => receipt.phase !== 'bound' && receipt.phase !== 'rejected');
+       if (unresolved !== undefined) intentRequestId = unresolved.requestId; // 重试沿用原 request_id（D5），否则 office.start 会以新 ID 重建任务
        publish({


─── apps/mobile/src/screens/NewTaskScreen.tsx:13-13 ───
[bug · high] maxLength=500 的安全论证漏算 knowledgeIds 与 attachments。intent log 落盘的是完整
goal（task-office.ts:481 intentLog.save 含 startGoal 全字段），而注释只按「500 中文 1500B +
requestId/sessionId/scope 固定开销」估 1.8KB。实际叠加 agentId/tenantID/userID 等 UUID 与字段名后固定开销约 350B，再加每条知识约
45B、每条附件约 70B（name 为中文文件名时更多），合法组合即可逼近甚至越过 SecureStore 2048B 硬限；而 serializeWithinBudget 在仅剩 1
条时（intent-log.ts:50 条件 records.size > 1）不裁剪也不拦截，直接 setItemAsync 超限值——一旦抛错，office.start 在 Start POST
前上抛，恰好复现本注释声称要避免的「提交永久失败」（用户已输满 500 字不会自动截断，无自愈路径）。建议把 knowledgeIds/attachments 纳入预算折算收紧
maxLength，或让 intent log 单条超限时抛出带明确 code 的错误供 UI 引导。



─── apps/mobile/src/new-task-view.ts:70-73 ───
[bug · medium] project() 用旧的 state.agents 计算推荐结果，refreshAgents() 成功后 recommendation 不刷新。publish
的参数在求值时 state 尚未更新：当 refreshAgents 调用 project({ agents }) 时，`...state, ...patch` 中 agents
字段已被新列表覆盖，但 `recommendLeadAgent(state.agents)` 读的仍是刷新前的旧列表。该函数名与注释承诺的「目录刷新」效果实际失效（仅列表按钮变化，Lead Agent
推荐保持旧目录的计算结果）。

    const project = (patch: Partial<NewTaskViewState> & { draft?: NewTaskDraft }): void => {
      const draft = patch.draft ?? state.draft;
-     publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(state.agents) });
+     const agents = patch.agents ?? state.agents;
+     publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(agents) });
    };


─── apps/mobile/src/new-task-view.ts:84-85 ───
[bug · medium] 初始化竞态导致用户输入静默丢失。NewTaskScreen 在 state.loading 为 true 时仅切换标题文案（'Loading'/'New
task'），TextInput 与知识按钮全部可交互；而 initialized() 需等待 drafts.load + agents() + knowledge() +
reconcilePending() 多个网络调用（秒级窗口常见）。窗口期内用户输入经 update() 写入 state.draft 后，本处 publish 用 `draft: stored ??
emptyDraft()` 无条件覆盖（`...state` 在前、draft 字段在后），用户已输入内容被存储草稿或空草稿冲掉。建议 loading
期间禁用输入控件，或仅当当前草稿仍为初始空草稿时才应用恢复值。



─── apps/mobile/src/new-task-view.ts:150-153 ───
[bug · medium] rejected 终态收据被当作未决意图保留同 ID 重试，形成永不成功的重试死路径。initialized() 的 find 明确排除 rejected，但本处
else 分支不排除：receipt.phase === 'rejected' 时同样写入 inFlight 并保留 intentRequestId，UI 将显示「Unresolved
submission ... (rejected) — retrying keeps the same request id」。而按 domain 语义（submission.ts
retryEntry：新意图=新 request_id），rejected 是终态——同 ID 重试经 office→resume→reconcile，服务端 lookup 返回 rejected
只会维持 rejected，用户被引导进无法成功的循环。建议 rejected 单独呈现失败并允许以新 ID 重新提交。

+         } else if (receipt.phase === 'rejected') {
+           intentRequestId = undefined; // rejected 是终态：同 ID 重试只会维持 rejected，重试需新意图新 ID
+           publish({ ...state, submitting: false, error: `submission rejected (${receipt.requestId})` });
          } else {
            intentRequestId = receipt.requestId;
            publish({ ...state, submitting: false, inFlight: receipt });
          }


─── apps/mobile/src/new-task-drafts.ts:23-25 ───
[bug · medium] load() 的类型断言不设防，注释声称的兜底不成立。「字段级校验由 evaluateSubmitReadiness
在投影时兜底」与事实不符：task-form.ts:39-48 中该函数直接调用 draft.text.trim() 与
draft.attachments.filter()，无任何类型防御，且完全不触及 knowledgeIds。后果分两类：(1) text/attachments 字段缺失或类型不符的脏数据（旧版本
schema、部分损坏）会让 evaluateSubmitReadiness 抛 TypeError，被 initialized 的 catch 捕获后草稿被静默丢弃并显示技术性错误消息；(2)
knowledgeIds 缺失时 readiness 不拦截，脏 draft 直接进入 state，NewTaskScreen 渲染
state.draft.knowledgeIds.includes(...) 抛 TypeError 导致整屏崩溃，且因 catch(()=>undefined) 无法自愈。建议按字段做 shape
校验，不通过返回 undefined 走空草稿路径。

          const value: unknown = JSON.parse(entry.body);
          if (typeof value !== 'object' || value === null) return undefined;
-         return value as NewTaskDraft; // 字段级校验由 evaluateSubmitReadiness 在投影时兜底
+         const row = value as Partial<NewTaskDraft>;
+         const valid = typeof row.text === 'string'
+           && (row.agentId === null || typeof row.agentId === 'string')
+           && typeof row.budgetUpper === 'number'
+           && Array.isArray(row.attachments)
+           && Array.isArray(row.knowledgeIds);
+         return valid ? (row as NewTaskDraft) : undefined;


─── packages/api-client/src/mobile/task-office.ts:243-243 ───
[bug · medium] decide() 把所有 400 与 409 一并翻译为 INTERACTION_SUPERSEDED，语义过宽。后端契约（workbench_commands.go
writeWorkbenchCommandError）明确区分：409 = ErrAlreadyResolved（已被决定，即 superseded），400 =
ErrInteractionActionMismatch 或请求体绑定失败等确定性客户端错误。本处翻译后 mobile-core（attention-inbox.ts:154）会把 400 一律归类为
status:'superseded' 终态——action 与 kind 不匹配等真实失败被冒充为「已被他人处理」，原始 ApiError 的 code/message 全部丢失，掩盖缺陷且 UI
停止重试/上报。建议 400 不翻译（保留原错误走 TASK_OFFICE_BACKEND），若后端确有以 400 表达 superseded 的路径应按 error.code 判别而非仅凭
status。

-           if (error.status === 409 || error.status === 400) coded('INTERACTION_SUPERSEDED');
+           if (error.status === 409) coded('INTERACTION_SUPERSEDED');


─── packages/domain/src/mobile/submission.ts:202-207 ───
[bug · medium] resume() 的网络段存在两个问题。(1) catch 全量吞错把确定性 4xx 拒绝（如预算不足）也归为 awaiting_reconciliation：此后
lookup 恒为 unknown（服务端从未持久化），用户每次显式重入 resume 都会再次 transport.start——对恒定失败的意图形成「重入即重发」的循环，且永远无法进入
rejected 终态，与函数注释「仅当服务端明确 unknown 才重发」的守卫意图相悖；建议 catch 区分确定性客户端错误并直接落 rejected。(2) 本段 try/catch（约 15
行）与 submit() 中 129-144 行逐字重复，两处行为已出现同步漂移风险，应提取共享的 dispatchAndRecord(input, digest, scope) helper。



─── apps/mobile/src/task-start-integration-smoke.ts:100-102 ───
[test · medium] 第二次同 ID 重入调用未捕获异常，evidence 链在最关键断言点必断。按当前 office 语义推演：第一次 start bound 后
intentLog.remove 已清除该记录（task-office.ts:497），第二次 start(goal, {requestId: first.requestId}) 时
intentLog.load=undefined，走「新意图」路径新建 session2（真实产生一个多余的空 session），随后 submissions.resume 中 store 里
bound entry 的 input_digest 基于 session1，session_id 不同导致 digest 必不一致，抛 SubmissionConflictError 并被
office 转为 TASK_OFFICE_SUBMISSION_CONFLICT 上抛——此处无 try/catch，runTaskStartIntegration 将整体异常终止而非产出证据，违背
emitTaskStartIntegrationEvidence 注释「including failed live outcomes」的契约（对比下方 office.tasks({}) 就有
try/catch 保护）。建议对 second 调用包异常捕获并映射为可断言的 repeatSubmitSameRequest 值（如 'failed' 或 'conflict'）。

    // 同一意图重入（真服务端）：不得产生第二次派发
+   try {
-   const second = await office.start(goal, { requestId: first.requestId });
+     const second = await office.start(goal, { requestId: first.requestId });
-   evidence.repeatSubmitSameRequest = second.dispatched === false && second.runId === first.runId ? 'no-second-dispatch' : 'second-dispatch';
+     evidence.repeatSubmitSameRequest = second.dispatched === false && second.runId === first.runId ? 'no-second-dispatch' : 'second-dispatch';
+   } catch {
+     evidence.repeatSubmitSameRequest = 'failed'; // 含 TASK_OFFICE_SUBMISSION_CONFLICT：evidence 必须产出而非函数崩溃
+   }


─── apps/mobile/src/screens/NewTaskScreen.tsx:43-43 ───
[bug · low] 预算输入的 0 与「未设置」语义混同，且非法输入被静默清空。onChangeText 对任何非纯数字输入（含用户输入中间态、粘贴带分隔符）一律置 0，而显示层
`budgetUpper === 0 ? ''` 又把 0 渲染为空——用户无法显式表达 0 预算，输入过程中内容会被瞬间清空，体验突兀。建议在组件内保留原始字符串作为受控值，仅在通过 ^\d+$
校验时投影 number（或为 draft 引入 null 语义区分未设置）。



─── packages/api-client/src/mobile/task-office.ts:202-203 ───
[maintainability · low] inbox() 将分页上限 50 硬编码在 remote 层且签名不收 limit 参数——mobile-core
InteractionBackendPort.inbox(query) 传入的 limit 被静默忽略，未来调大调用方参数不会有任何效果；同时 createdAt
缺失回退为空字符串（InboxItem.createdAt 为必填 string），空串时间戳会直接进入下游展示层。建议透传 limit，并为 createdAt 缺失给出显式策略（保持
optional 或以加载时刻替代），避免空字符串污染展示与潜在的时间序逻辑。



─── packages/api-client/src/mobile/devices.ts:108-109 ───
[maintainability · low] 死代码:`registration.deviceId` 在此处不可能为空串——对象字面量构造时 `deviceId:
requireDeviceId(String(data.device_id ?? ''))` 已对空/缺失/纯空白的 device_id 抛出 'device id is
required'(`requireDeviceId` 内 trim 后为空即 throw),构造能完成就必然非空。因此这行 `if` 分支永不执行,特意准备的更具体错误消息 'device
register device_id is required' 也永远不会被抛出。建议改为在构造前显式校验 device_id 并抛出该具体消息,或直接删除此死分支。

-       if (registration.deviceId === '') throw new Error('device register device_id is required');
+       const deviceId = typeof data.device_id === 'string' ? data.device_id.trim() : '';
+       if (deviceId === '') throw new Error('device register device_id is required');
+       const registration: MobileDeviceRegistration = {
+         deviceId,
+         // ...
+       };
        return registration;


─── apps/mobile/src/adapters/device-identity.ts:10-15 ───
[bug · medium] `deviceId()` 无 in-flight 去重,存在并发首次调用竞态:两个并发的首次调用都会读到 `existing` 为空、各自生成不同 uuid,后一次
`setItemAsync` 覆盖先一次;先返回的调用方持有的 id 与最终持久化 id 不一致,破坏注释承诺的「稳定设备身份……首次生成后复用」契约。后果是同一物理设备以两个不同 deviceId
在服务端完成注册((tenant, owner, device) 唯一键下产生一条无法再收到推送的幽灵记录)。当前唯一生产调用方(composition.ts
`registerActiveDeviceIfPossible`)有 per-origin once guard
不会并发,但这是导出的公共适配器,任何并发首次调用(未来的设置页手动重注册、测试并发驱动)都会触发。建议在闭包内缓存 in-flight promise 做单飞(失败时清除缓存以保留重试语义)。

  export function createSecureDeviceIdentity(store: SecureStorePort): NativeDeviceIdentity {
-   return {
-     async deviceId(): Promise<string | undefined> {
+   let inflight: Promise<string | undefined> | undefined;
+   const attempt = async (): Promise<string | undefined> => {
-       try {
+     try {
-         const existing = await store.getItemAsync(DEVICE_ID_KEY);
+       const existing = await store.getItemAsync(DEVICE_ID_KEY);
-         if (typeof existing === 'string' && existing.trim() !== '') return existing.trim();
+       if (typeof existing === 'string' && existing.trim() !== '') return existing.trim();
+       const generated = /* ... */ '';
+       await store.setItemAsync(DEVICE_ID_KEY, generated);
+       return generated;
+     } catch {
+       return undefined;
+     }
+   };
+   return {
+     deviceId(): Promise<string | undefined> {
+       inflight ??= attempt().then((id) => {
+         if (id === undefined) inflight = undefined; // fail closed 失败不缓存，允许下次重试
+         return id;
+       });
+       return inflight;
+     },
+   };
+ }


─── packages/mobile-core/src/device/device-registry.ts:88-90 ───
[maintainability · low] `attemptRegister` 在 `remote.register` 成功返回后未再复检 lease:若 scope 在第二步 register
进行中切换(切租户/换部署/登出),迟到的成功记录仍会原样返回给调用方,与文件头注释宣称的「迟到结果不落地」不完全一致,也与本 PR
其他模块的围栏模式不对称(task-office.ts、notification-inbox.ts 均在远端调用完成后复检 `leaseActive` 再落地结果)。建议 register 成功后复检
lease,失效时抛 `DEVICE_SCOPE_CHANGED`,使调用方不会把旧 scope 的注册结果当作当前会话状态消费。

      const intent = await ports.remote.issueIntent(deviceId);
      if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
-     return ports.remote.register({ deviceId, token, platform, registrationIntent: intent.registrationIntent });
+     const record = await ports.remote.register({ deviceId, token, platform, registrationIntent: intent.registrationIntent });
+     if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
+     return record;


─── apps/mobile/src/adapters/intent-log.ts:68-72 ───
[bug · medium] save/remove 均为「readAll → 修改 → setItemAsync」的读-改-写序列，彼此无互斥，存在丢失更新竞态：两个并发 save（不同
requestId，如用户快速触发两次 start——各自在 createSession 网络往返后到达
save，读快照可能重叠）都会读到同一旧快照，后写者整包覆盖前写者，前一意图记录被静默丢弃。丢失的记录可能已紧随其后完成 Start POST，崩溃/重启后 reconcilePending
将无法发现该意图，恰好击穿 intentLog 承诺的「崩溃后幂等恢复」核心保证。该实例经 composition.ts 的 intentLogOf() 是跨全部 deployment origin
共享的全局单例，并发面更大；remove 与 save 交错同样可能丢记录。建议在实现内用 promise 链（串行队列）互斥所有写操作，使 read-modify-write 原子化。

+ export function createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog {
+   // 串行化写操作：read-modify-write 必须互斥，否则并发 save/remove 整包覆盖丢意图记录
+   let writeQueue: Promise<unknown> = Promise.resolve();
+   const runWrite = <T>(task: () => Promise<T>): Promise<T> => {
+     const next = writeQueue.then(task, task);
+     writeQueue = next.catch(() => undefined);
+     return next;
+   };
+   const readAll = async (): Promise<Map<string, SubmissionIntentRecord>> => parseRecords(await store.getItemAsync(INTENTS_KEY));
+   return {
      async save(record) {
+       return runWrite(async () => {
-       const records = await readAll();
+         const records = await readAll();
-       records.set(record.requestId, record);
+         records.set(record.requestId, record);
-       await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
+         await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
+       });
      },
+     // remove 同样经 runWrite 串行化
+     // ...
+   };
+ }


─── packages/api-client/src/mobile/materials.ts:109-109 ───
[bug · medium] terminalLog 的末页信号在转译时丢失：Go 端（workbench_terminal_log.go）恒发数字
next_cursor，末页的信号是「游标未推进」即 next_cursor === after；本适配器恒返回数字 nextCursor（缺字段还静默回退 Number(... ?? 0) →
0），而消费方 task-material.ts:168 以 page.nextCursor === undefined 判「无下一页」、MaterialsScreen 以 nextCursor
!== undefined 显示「可继续加载」。后果：(1) 生产面末页仍返回数字 → 提示永不消失；(2) 若字段缺失/非数字，回退 0 会把续页游标重置回起点（重复拉取），Number('x')
产生 NaN 则下轮请求 after=NaN 被 400。建议：非 number 大声失败；next_cursor === after（未推进）时省略 nextCursor（需同步把
MaterialBackendTerminalPage.nextCursor 放宽为可选，与 task-material 既有 undefined 消费对齐）。

-         nextCursor: typeof data.next_cursor === 'number' ? data.next_cursor : Number(data.next_cursor ?? 0),
+       const nextCursor = data.next_cursor;
+       if (typeof nextCursor !== 'number') throw new Error('terminal-log next_cursor must be a number');
+       return {
+         lines: lines.map((row) => {
+           /* 原有行映射保持不变 */
+         }),
+         // 游标未推进（=== after）即末页：省略 nextCursor 表示无下一页，与 task-material.ts 的 undefined 消费对齐。
+         ...(nextCursor > after ? { nextCursor } : {}),
+       };


─── apps/mobile/src/screens/MaterialsScreen.tsx:67-67 ───
[bug · low] 「可继续加载（游标 X）」提示无交互承载且在现行合同下永不消失：MaterialsController.openTerminal() 不接收
cursor、屏上也没有「加载更多」按钮，用户看到提示却无法操作；同时 api-client 的 terminalLog 恒返回数字 nextCursor（Go 端末页 next_cursor ===
after 仍为数字），view.nextCursor 生产环境永远存在，末页照样显示该提示，构成误导。建议：控制器透传 cursor（openTerminal(cursor?:
number)），屏上补加载更多按钮；末页省略提示（配合 materials.ts 适配器的末页转译）。



─── apps/mobile/src/materials-view.ts:53-56 ───
[bug · medium] run() 缺少防陈旧发布保护，晚到的旧响应会覆盖新状态：例如用户先点「打开」材料 A（慢）再点 B（快），B 先发布、A 后到又把视图覆盖回
A，用户看到与最后操作不符的材料且无任何提示。同批 resources-view.ts 已采用 generation 令牌（并有 'a stale in-flight projection never
overwrites a newer one' 测试）作为既有范式，本控制器应对齐；dispose() 中也应递增 generation 使在途结果全部失效。

+   let generation = 0; // 与 resources-view.ts 同一防陈旧模式
    const run = async (action: () => Promise<MaterialsViewState>): Promise<void> => {
+     const ticket = ++generation;
      publish({ ...state, loading: true, error: undefined });
      try {
-       if (!disposed) publish(await action());
+       const next = await action();
+       if (!disposed && ticket === generation) publish(next);
+     } catch (failure) {
+       if (!disposed && ticket === generation) publish({ ...state, loading: false, error: messageOf(failure) });
+     }
+   };
+   // dispose() 中追加：generation += 1;


─── apps/mobile/src/materials-view.ts:13-13 ───
[maintainability · low] MATERIAL_ERROR_COPY 键类型放宽为 Record<string, string> 丧失穷尽性检查：mobile-core 的
MaterialErrorCode 新增错误码时，编译器不会提示此处缺文案，用户将直接看到裸错误码（messageOf 的 ?? failure.code 兜底）。改用
Record<MaterialErrorCode, string> 可让缺项在编译期暴露。

- export const MATERIAL_ERROR_COPY: Record<string, string> = {
+ import { MaterialError, type MaterialErrorCode } from '@weknora/mobile-core';
+ 
+ export const MATERIAL_ERROR_COPY: Record<MaterialErrorCode, string> = {


─── apps/mobile/src/app/tasks/materials.tsx:22-25 ───
[maintainability · low] 该错误文案与 materials-view.ts 中 MATERIAL_ERROR_COPY.MATERIAL_SCOPE_CHANGED
逐字重复：两处独立维护，后续改文案极易漂移。建议复用同一来源（import { MATERIAL_ERROR_COPY } from '../../materials-view.ts'）。

      } catch {
-       setState({ loading: false, error: '登录状态或活动空间已变化，请重新进入。' });
+       setState({ loading: false, error: MATERIAL_ERROR_COPY.MATERIAL_SCOPE_CHANGED });
        return;
      }


─── apps/mobile/src/adapters/material-adapters.ts:31-33 ───
[bug · medium] react-native Share.share 的 url 参数仅 iOS 生效，Android 实现只取 message 构建 ACTION_SEND：在
Android 上该适配器会静默丢弃签名链接，分享内容退化为裸文件名，分享功能形同虚设且无任何报错。建议按平台分支——iOS 用 { url, message }，Android 将链接并入
message（或经 expo-sharing 分享已下载文件）。

        async share({ url, name }) {
-         await shareLike.share({ url, message: name });
+         // RN Share 的 url 仅 iOS 生效；Android 面把链接并入 message，避免只分享出裸文件名。
+         const reactNative = require('react-native') as { Platform?: { OS?: string } };
+         const onIOS = reactNative.Platform?.OS !== 'android';
+         await shareLike.share(onIOS ? { url, message: name } : { message: `${name}\n${url}` });
        },


─── packages/mobile-core/src/material/task-material.ts:103-106 ───
[bug · medium] mintGrant 未回验 grant.artifact 的身份。ports.ts 的 MaterialBackendGrant 明确携带
artifact（id/version 等完整身份），此处只校验了 URL origin 后直接放行。而调用方 entryFor 依赖句柄内缓存的 lastIndex（同 runId
下无失效机制，仅显式 index() 或失败才刷新），signedUrl 又按位置键 index 铸造：当 run 仍在产出工件导致后端列表重排、或后端返回异常时，entry.index 与
materialId 已错位，后端会按漂移后的 index 铸造指向另一工件的签名 URL——act(download/share) 会把错材料的签名凭据外发给用户/系统分享面板，open 预览的
fetchBytes 同样会缓存并展示错材料字节，fail open 而非 fail closed。建议在 origin 校验之后追加身份回验：grant.artifact.id ===
entry.materialId（必要时含 version），不一致抛 MATERIAL_BACKEND（错误 cause 附期望与实际身份），且不进入 blobs
缓存、不返回上层。校验数据已在手，零额外往返。

          if (origin !== scope.deploymentOrigin) {
            throw new MaterialError('MATERIAL_GRANT_ORIGIN', { cause: new Error(`grant origin ${origin} is not the active deployment`) });
+         }
+         // 身份回验：index 是易变位置键，铸造结果必须钉回 materialId，防列表重排后错位外发。
+         if (grant.artifact?.id !== entry.materialId || (grant.artifact?.version !== undefined && grant.artifact.version !== entry.version)) {
+           throw new MaterialError('MATERIAL_BACKEND', {
+             cause: new Error(`grant artifact mismatch: expected ${entry.materialId}@${entry.version}, got ${grant.artifact?.id}@${grant.artifact?.version}`),
+           });
          }
          return grant;


─── apps/mobile/src/new-task-view.ts:76-80 ───
[bug · high] 初始化 Promise.all 中唯独 ports.agents() 没有 .catch 兜底(drafts.load、knowledge 均有,refreshAgents
也有 catch「失败保持现状」),不对称且后果最重:agents() 是网络调用(NewTaskRouteLifecycle 里
resourceShelf().browse()),离线/服务端故障时 reject 会让 Promise.all 整体失败进入外层 catch——此时
stored(已保存的离线草稿)被直接丢弃,draft 停留在 emptyDraft(),loading 置 false 后表单照常可交互。用户随后的任何输入经
update()→persistDraft() 会以 emptyDraft+新字段的完整 JSON 覆盖
NEW_TASK_DRAFT_ID,旧草稿永久丢失。离线场景恰恰是离线草稿功能的核心场景。建议给 agents() 同样兜底(失败返回空目录、recommendation 降级为
no_supported_agent),让本地草稿恢复不依赖 agent 目录网络调用。

        const [stored, agents, knowledge] = await Promise.all([
          ports.drafts?.load().catch(() => undefined),
-         ports.agents(),
+         ports.agents().catch(() => [] as readonly AgentOption[]),
          ports.knowledge?.().catch(() => [] as const) ?? ([] as const),
        ]);


─── apps/mobile/src/task-start-integration-smoke.ts:68-72 ───
[bug · medium] evidence 生产链路的裸 await 会让「失败也返回 evidence」的契约失效(与已确认的第二次重入调用未捕获问题互补但独立):evidence.start
初始值 'failed' 与 errorReason 字段的设计意图是失败时仍产出证据(emitTaskStartIntegrationEvidence 注释 'including failed
live outcomes'),但 runtime.signIn、runtime.authorizedRequest 以及第一次 office.start(goal) 均无
try/catch——凭据错误、网络故障、授权过期时 runTaskStartIntegration 直接抛异常,evidence 对象被丢弃,调用方拿不到
TaskStartIntegrationEvidence,只能捕获异常。建议各失败点统一收敛为 evidence.errorReason 赋值 + return evidence(可抽一个
fail(cause) 局部 helper)。

-   const snapshot = await runtime.signIn({
+   let snapshot: Awaited<ReturnType<typeof runtime.signIn>>;
+   try {
+     snapshot = await runtime.signIn({
-     deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
+       deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
-     email: config.email,
+       email: config.email,
-     password: config.password,
+       password: config.password,
-   });
+     });
+   } catch (cause) {
+     evidence.errorReason = cause instanceof Error ? cause.message : String(cause);
+     return evidence;
+   }


─── apps/mobile/src/task-start-integration-smoke.ts:95-95 ───
[style · low] 嵌套三元表达式(checklist 明确禁止):first.phase === 'bound' ? 'admitted' : first.phase ===
'rejected' ? 'rejected' : 'pending' 是两层三元嵌套,可读性差。建议改为 if/else 链或提取具名映射函数。

-   evidence.start = first.phase === 'bound' ? 'admitted' : first.phase === 'rejected' ? 'rejected' : 'pending';
+   if (first.phase === 'bound') evidence.start = 'admitted';
+   else if (first.phase === 'rejected') evidence.start = 'rejected';
+   else evidence.start = 'pending';


─── packages/domain/src/mobile/submission.ts:188-191 ───
[performance · low] resume() 在 lookup.state !== 'unknown' 时丢弃刚拿到的 lookup 结果,转而调用 this.reconcile()——而
reconcile 内部(第 153 行)会再次 transport.lookup(requestId),导致每次显式重入对同一 request_id 连发两次相同的 GET
对账请求;两次查询间服务端状态还可能不一致(reconcile 以第二次结果为准)。建议让 reconcile 接受可选的预取结果复用本次 lookup,或将「lookup →
entry」的状态映射提取为共享纯函数供 submit/reconcile/resume 三处复用(顺带消解与 submit 重复的那段落盘逻辑)。

        const lookup = await transport.lookup(input.request_id);
        if (lookup.state !== 'unknown') {
-         return { entry: await this.reconcile(input.request_id, scope), dispatched: false };
+         return { entry: await this.reconcile(input.request_id, scope, lookup), dispatched: false };
        }


─── internal/handler/session/workbench_terminal_log.go:53-57 ───
[bug · medium] 该端点使用了 owner-only 的 `resolveOwnedRun`,而同一变更已把同路由组的只读端点
`GetWorkbenchExecution`/`GetWorkbenchSnapshot` 迁移到 `h.resolveReadableRun`(T12:owner 未中后回退到
task_grants 的 Viewer/Collaborator 授权读取)。`WithGrantedRuns` 的注释明确枚举 owner-only 面为"run submission, SSE
recovery and source-event ingestion",terminal-log 这个纯 GET 只读面不在其列。后果:被授权的 Viewer/Collaborator 在 `GET
/:run_id/snapshot` 能拿到 200,却在 terminal-log 上得到 404——而 `ReadRunSnapshot` 返回全部事件类型(含每条 tool.terminal
的完整 stream/text payload,`ExecutionEvent.Validate` 无类型白名单),即同样的终端内容已通过 snapshot 暴露给被授权读者,owner-only
谓词在此无任何保密收益,只造成功能缺口(协作者视角终端页 404)。建议改用 `h.resolveReadableRun(c)`;若终端输出确需对 Viewer 保密,则应在
snapshot/events 的授权读取中过滤 tool.terminal,而非仅在此端点收紧。

  func (h *WorkbenchReadHandler) GetWorkbenchTerminalLog(c *gin.Context) {
- 	run, ok := resolveOwnedRun(c, h.runs)
+ 	run, ok := h.resolveReadableRun(c)
  	if !ok {
  		return
  	}


─── packages/mobile-core/src/task-office/task-office.ts:497-497 ───
[bug · high] start()：`intentLog.remove` 失败会把已 bound 的成功提交伪装成失败。任务已在服务端创建并绑定 run_id，但 remove
抛出的原始错误（非 TaskOfficeError）经 catch 原样上抛——上层（new-task-view 的 submit catch）不会记住
intentRequestId，用户看到报错后自然重试将以新 requestId 走 createSession→Start
全链路，造成重复创建任务与重复预算占用。这与端口契约自相矛盾：注释明确「不实现则记录留存（reconcile 幂等无害）」，即记录留存本就无害，remove 应为
best-effort。建议吞掉清理失败，保证已 bound 的 receipt 如实返回。

-         if (outcome.entry.phase === 'bound') await intentLog.remove?.(requestId);
+         if (outcome.entry.phase === 'bound') {
+           // 记录留存幂等无害（端口契约）：清理失败不得把已 bound 的成功伪装成失败，
+           // 否则上层以新 requestId 重试会重复创建任务/重复扣预算。
+           try { await intentLog.remove?.(requestId); } catch { /* best-effort 清理 */ }
+         }


─── packages/mobile-core/src/task-office/task-office.ts:517-519 ───
[bug · medium] reconcilePending()：循环内无逐记录错误隔离。任一记录的 lookup 网络异常或 remove 拒绝都会中断整批对账——已收集的 receipts
全部丢弃（函数抛出），其余 pending 意图无法恢复；且同一条「毒记录」会让后续每次 reconcilePending 都在同一位置失败，重启恢复能力被单点卡死。建议对每条记录 try/catch
后继续处理其余记录（可在返回值或日志中带出部分失败），恢复流程是本特性的核心场景，应具备部分成功语义。

+         try {
-         const entry = await submissions.reconcile(record.requestId, scope);
+           const entry = await submissions.reconcile(record.requestId, scope);
-         if (entry.phase === 'bound') await intentLog.remove?.(record.requestId);
+           if (entry.phase === 'bound') { try { await intentLog.remove?.(record.requestId); } catch { /* best-effort */ } }
-         receipts.push(receiptOf(entry, false));
+           receipts.push(receiptOf(entry, false));
+         } catch {
+           // 单条对账失败不阻塞其余 pending 意图的恢复（该记录下次 reconcilePending 再试）。
+           continue;
+         }


─── packages/mobile-core/src/task-office/task-office.ts:547-549 ───
[bug · low] legacyHistory() 未走 settle/epoch 失效保护，与同模块 legacyTasks/moreLegacyTasks 的 SUPERSEDED
语义不一致：followUp 成功后虽然递增了 legacyListEpoch 失效在途读（558-561 行注释「一切在途读失效」），但 legacyHistory 从未捕获/比对
epoch，同一任务在追问在途期间加载的旧历史仍会成功返回。建议与 list 系列一致：进入时取 epoch、settle 校验后再返回。

-       const messages = await callBackend(() => requireLegacy().history(trimmed));
-       if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
+       const epoch = ++legacyListEpoch;
+       const lease = requireLease();
+       const trimmed = taskId.trim();
+       if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
+       const messages = settle(epoch, 'legacy', lease, await callBackend(() => requireLegacy().history(trimmed)));
        return messages;


─── packages/mobile-core/src/task-office/task-office.ts:346-358 ───
[maintainability · low] accumulateLegacy 与同文件 385-397 行的 accumulate（tasks
列表）是近乎逐行相同的去重/游标累积逻辑（seen/duplicates/cursor/nextCursor 展开），legacyListAccumulation 亦与
listAccumulation 同构。两处并行维护易漂移：例如一侧修去重键或 duplicates 上报规则时另一侧易遗漏。建议提取以「id 提取器 +
行映射器」参数化的公共分页累积工具，两个入口共用。



─── packages/mobile-core/src/task-office/task-office.ts:493-493 ───
[maintainability · low] Start wire 的 targetId 'platform' / workspaceRef '' 在 start()（493 行）与
reconcilePending()（514 行）各硬编码一次。取值与 api-client 的 wire 约定一致，但作为七字段冻结契约的业务字面量散落两处，后续若约定变更（如引入真实
target）易只改一处导致两条路径 digest 不一致（inputDigest 覆盖 target_id/workspace_ref，不一致会伪报
SUBMISSION_CONFLICT）。建议提取模块级具名常量（如 `const START_TARGET_ID = 'platform'`）统一引用。

-       const input = toStartInput(draft, { requestID: requestId, sessionId, targetId: 'platform', workspaceRef: '' });
+       const input = toStartInput(draft, { requestID: requestId, sessionId, targetId: START_TARGET_ID, workspaceRef: START_WORKSPACE_REF });


─── internal/router/routes_agent_adoption.go:29-29 ───
[bug · medium] available-agents 是注释中声明的"移动只读面 Viewer+"端点，但 API-key 地板用了裸
apiKeyFullAccess()。作为对照，它所镜像的 GET /api/v1/agents（routes_agent.go:27/51）声明为
apiKeyReadAgents(apiKeyManageAgents(apiKeyChat(apiKeyFullAccess())))——OR 语义下持 read_agents/chat 等受限能力
key 的集成可以读 /agents，却会在这个同字段 wire 的端点上被 403。建议至少叠加 read_agents 能力：apiKeyReadAgents(admin)。

- 	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/available-agents", admin, g.Viewer(), adoptionHandler.ListAvailableAgents)
+ 	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/available-agents", apiKeyReadAgents(admin), g.Viewer(), adoptionHandler.ListAvailableAgents)


─── internal/application/service/agent_adoption.go:188-194 ───
[bug · medium] 能力映射仅校验 capability 名属于 Manifest 必需项，未校验 ModelID/KnowledgeBaseIDs/ConnectionIDs
指向的本地资源是否存在（已确认 customAgentService.CreateAgent 同样不校验，仅检查名称非空）。任意不存在的 model/kb id
可直接入库；PublishVariant 据此 buildLocalAgent 后产出的 Agent 虽标记为
tested+published（即"可用"），实际运行时才失败，违背该治理流程"映射即绑定本地资源"的语义。建议在入库前（或至少 PublishVariant 前）校验 ModelID
可解析、KB/Connection id 存在。

+ 		if trimmed := strings.TrimSpace(mapping.ModelID); trimmed != "" {
+ 			if _, err := s.models.GetModelByID(ctx, tenantID, trimmed); err != nil {
+ 				return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: model %q does not exist", ErrAgentAdoptionInvalidInput, trimmed)
+ 			}
+ 		}
+ 		// KnowledgeBaseIDs/ConnectionIDs 同样需要存在性校验后再入库
  		rows = append(rows, types.AgentVariantCapabilityMappingEntity{
  			TenantID: tenantID, VariantID: variant.ID, Capability: capability,
  			ModelID:          strings.TrimSpace(mapping.ModelID),
  			KnowledgeBaseIDs: encodeIDList(mapping.KnowledgeBaseIDs),
  			ConnectionIDs:    encodeIDList(mapping.ConnectionIDs),
  			UpdatedBy:        actorID,
  		})


─── internal/application/service/agent_adoption.go:346-346 ───
[maintainability · medium] capability 恒为 {"supported", ""}，使契约（packages/contracts 的
CAPABILITY_STATES）与移动领域模型（AgentCapabilityState 三态）中 unavailable/forbidden 从该端点永远不可达，Reason
字段成为死字段。发布后本地模型被删除、KB 失联的 Agent 仍被标为 supported，下游基于该状态过滤的策略（lead-agent.ts 的 recommendLeadAgent 只信任
supported）无法拒绝实际不可用的 Agent。建议至少结合映射资源的当前可用性计算判定，或明确注释此端点仅表示"治理意义上已发布"并让移动端另行裁决。



─── internal/application/service/agent_adoption.go:104-111 ───
[performance · low] ListAdoptions 存在 N+1：每个 adoption 一次 ListVariantsByAdoption，而
adoptionView→variantView→missingCapabilitiesOf 又对每个 variant 各触发一次 GetRelease（重复解码同一 Manifest JSON）和
ListCapabilityMappings。多变体租户的管理端列表查询数线性放大。建议一次性批量取回该租户全部 variants（已有 tenant 维度查询条件），并按
ReleaseID/VariantID 分组内存关联。



─── internal/application/service/agent_adoption.go:456-460 ───
[maintainability · low] decodeIDList 用 _ = json.Unmarshal 忽略解析错误：映射行中的 JSON
损坏（手工改库、迁移事故）会被静默当作空列表，capabilityBound 随之判为未绑定、missingCapabilities 失真，且无任何日志线索可循。建议至少在解析失败时记一条 warn
日志，或返回错误让调用方决策。



─── internal/application/repository/task_grant_store.go:102-105 ───
[bug · low] TaskOwnerID 对 NULL user_id 会破坏自身声明的"统一 ErrTaskGrantTaskNotFound"契约：sessions.user_id 在
DDL 中可空（migration 000039 `ADD COLUMN IF NOT EXISTS user_id VARCHAR(36)` 无 NOT NULL，且
sessionRepository.applySessionUserScope 明确处理 `user_id IS NULL` 的 legacy/API
租户级会话——这类行按设计存在）。`Select("user_id").Scan(&owner)` 把 SQL NULL 扫进普通 string 会触发 driver Scan
错误（"converting NULL to string is unsupported"），方法返回原始错误而非哨兵，调用方将得到 500 并在边界暴露内部错误。建议改扫
sql.NullString，把 NULL/空串一并归入 ErrTaskGrantTaskNotFound（与 loadTaskForGrantManagement 的 "task has no
owner" 立场一致）。

- 	var owner string
+ 	var owner sql.NullString
  	err := s.db.WithContext(ctx).Table("sessions").
  		Where("tenant_id = ? AND id = ?", tenantID, taskID).
  		Select("user_id").Scan(&owner).Error
+ 	if err != nil {
+ 		return "", err
+ 	}
+ 	if !owner.Valid || owner.String == "" {
+ 		return "", ErrTaskGrantTaskNotFound
+ 	}
+ 	return owner.String, nil


─── internal/application/repository/task_grant_store.go:44-53 ───
[bug · low] UpsertGrant 在"重新共享轮换角色"（冲突走 DoUpdates 分支）时，数据库行的 created_at 保留原值，但返回的 grant 结构体携带的是本次构造的
now——POST 响应中的 data.grant.created_at 会随后一次 re-grant 漂移，且与随后 GET /grants 列表读回的持久化真值不一致。建议冲突更新后回读该行（或用
RETURNING）再返回，保证 API 载荷与持久化状态一致。

  		DoUpdates: clause.Assignments(map[string]interface{}{
  			"role":       role,
  			"granted_by": grantedBy,
  			"updated_at": now,
  		}),
  	}).Create(&grant).Error
  	if err != nil {
  		return types.TaskGrant{}, err
+ 	}
+ 	// 冲突更新路径下 DB 保留原 created_at；回读以返回持久化真值。
+ 	var stored types.TaskGrant
+ 	if err := s.db.WithContext(ctx).
+ 		Where("tenant_id = ? AND task_id = ? AND grantee_id = ?", tenantID, taskID, granteeID).
+ 		Take(&stored).Error; err == nil {
+ 		return stored, nil
  	}
  	return grant, nil


─── internal/handler/session/workbench_artifacts.go:128-130 ───
[bug · medium] terminal.available 被硬编码为 true，但该声明并不总为真：(1) terminal-log 路由仅在 WorkbenchReadHandler
非空时挂载（RouterParams 中两者均为 optional，artifact handler 可在 read handler 未装配时独立挂载，此时端点根本不存在却仍声明
available:true）；(2) 即使路由挂载，GetWorkbenchTerminalLog 还会对 snapshots seam 做 TerminalLogReader
类型断言，未实现该接口的装配会以 501 terminal_log_unavailable 失败关闭。按本注释自身的语义（"declares whether this server
mounts..."），该标志在上述可达状态下都会误导客户端展示一个必然失败的终端入口。建议将可用性作为构造期注入的布尔（由装配处根据 read handler/快照能力推导），而非在此硬编码。

- 	// terminal declares whether this server mounts the read-only terminal
- 	// log endpoint; older deployments omit the flag and clients degrade.
- 	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items, "terminal": gin.H{"available": true}}})
+ // 在 WorkbenchArtifactHandler 增加构造期注入的能力位，由装配处根据
+ // read handler 与 TerminalLogReader 断言结果推导，而非硬编码 true：
+ func NewWorkbenchArtifactHandler(runs OwnedRunReader, refs ArtifactRefReader, terminalAvailable bool) *WorkbenchArtifactHandler {
+ 	return &WorkbenchArtifactHandler{runs: runs, refs: refs, terminalAvailable: terminalAvailable}
+ }
+ 
+ c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items, "terminal": gin.H{"available": h.terminalAvailable}})


─── internal/handler/app_connector_action.go:229-233 ───
[bug · low] ApproveAction 中连接查找把"任何 DB 错误"与"记录不存在"合并为一个布尔（.Error == nil）。瞬时存储故障（连接闪断、锁超时）会被折叠成
personalOwner=false，使个人连接所有者在基础设施降级期间收到 403 ACTION_APPROVAL_FORBIDDEN 而非
500——失败方向是关闭的（无越权），但把基础设施错误伪装成了授权拒绝，既误导客户端也污染排障信号。建议区分 gorm.ErrRecordNotFound 与其他错误：非 NotFound 错误应直接
500（errors 与 gorm 均已导入）。

  	var conn appconnectorrepo.ConnectionRow
- 	connFound := h.db.WithContext(c.Request.Context()).
+ 	connErr := h.db.WithContext(c.Request.Context()).
  		Where("tenant_id = ? AND id = ?", tenantID, row.ConnectionID).
- 		First(&conn).Error == nil
- 	personalOwner := connFound && conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID == userID
+ 		First(&conn).Error
+ 	if connErr != nil && !errors.Is(connErr, gorm.ErrRecordNotFound) {
+ 		appFail(c, http.StatusInternalServerError, "ACTION_LOOKUP_FAILED", "failed to load the action's connection")
+ 		return
+ 	}
+ 	personalOwner := connErr == nil && conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID == userID


─── packages/mobile-core/src/task-office/task-office.ts:473-475 ───
[bug · high] start() 幂等重放契约被破坏：bound 成功后 `intentLog.remove?.(requestId)` 会删除意图记录（缺省 in-memory log
即实现了 remove），此后调用方以同一 requestId 重放（task-start-integration-smoke.ts 的
repeatSubmitSameRequest「no-second-dispatch」正是此场景）时，`record === undefined` 分支会无条件 createSession 生成新
session——而 inputDigest 覆盖 session_id，新输入摘要与 submissionStore 中 bound entry 的旧摘要必然不一致 →
SubmissionConflictError → TASK_OFFICE_SUBMISSION_CONFLICT 抛出而非返回 `{phase:'bound',
dispatched:false}`。协调器「bound 直接返回、零网络」的幂等语义（submission.ts:107）在 office 接缝处被击穿，且该 requestId
被永久毒化（intentLog 记下新 session、store 仍是旧摘要，后续每次重入都冲突，只能靠 reconcilePending 治愈）。store 只保存摘要、sessionId 只能从
intentLog 重建，因此重入时记录缺失绝不能换 session。建议二选一：① record 缺失时先查 `submissionStore.load(requestId)`，存在
entry（尤其 bound）则不得 createSession，直接 `receiptOf(entry, false)` 回执或走 lookup 对账；② bound
后不删除意图记录（端口注释本就声明「记录留存、reconcile 幂等无害」，此方案同时消解已确认的 remove 失败掩蔽成功问题）。

        const record = await intentLog.load(requestId);
+       const existingEntry = submissionStore.load(requestId);
        let sessionId: string;
        if (record === undefined) {
+         if (existingEntry !== undefined) {
+           // 该 requestId 已有持久提交（store 只存摘要，sessionId 已不可重建）：
+           // bound 直接回执；awaiting_* 走 lookup 对账——绝不能 createSession 换 session 重放 digest。
+           if (existingEntry.phase === 'bound') return receiptOf(existingEntry, false);
+           return receiptOf(await submissions.reconcile(requestId, scope), false);
+         }
+         const session = await callBackend(() => ports.backend.createSession({ title: startGoal.text.trim().slice(0, 60) }));


─── packages/mobile-core/src/task-office/task-office.ts:498-500 ───
[bug · medium] start() 成功 bound（及 reconcilePending() 绑定 pending
意图）后未作废在途读与列表累积缓存，与同模块既有写失效语义不一致：mutate()（archive/restore）、followUp()、decide()（onDecided）都在写成功后递增
homeEpoch/listEpoch 并清空 accumulated（R1-F19「写成功作废一切在途读」），而 start 创建新 Task 是影响面最大的写。后果：① start 前发起的
tasks()/home() 仍会以旧快照 settle 成功（epoch 未变），UI 拿到不含新任务的列表；② accumulated 保留 pre-start 累积，moreTasks()
沿旧游标/seen 继续分页。TasksScreen 只在 mount 时 reload（useEffect(reload, [])），从 /new 创建任务返回后（stack 下 /tasks
未重挂载）用户看不到刚创建的任务，需手动刷新。建议在 `outcome.entry.phase === 'bound'` 时与 mutate 同步执行 `listEpoch += 1;
homeEpoch += 1; accumulated = undefined;`（reconcilePending 绑定成功处同理）。

+         if (outcome.entry.phase === 'bound') {
+           await intentLog.remove?.(requestId);
+           // 新 Task 落地：与 mutate()/followUp() 同一 R1-F19 失效语义，在途读不得以旧快照 settle。
+           listEpoch += 1;
+           homeEpoch += 1;
+           accumulated = undefined;
+         }
          return receiptOf(outcome.entry, outcome.dispatched);
        } catch (error) {
          if (error instanceof TaskOfficeError) throw error;


─── packages/mobile-core/src/task-office/task-office.ts:553-555 ───
[maintainability · low] followUp 的追问长度上限 8000 与 createSession 标题截断 slice(0, 60)
均为内联魔数（核对服务端：KnowledgeQA/steer 仅校验非空或另有 maxSteerQueryLength，均无 8000/60 契约），取值依据无处可查，且与本文件
searchMaxLen 命名常量的既有风格不一致。建议提取为模块级具名常量（如 legacyQuestionMaxLen = 8000、sessionTitleMaxLen =
60）并注明取值来源，避免后续两处口径漂移。

-       const taskId = (input?.taskId ?? '').trim();
-       const question = (input?.question ?? '').trim();
-       if (taskId === '' || question === '' || question.length > 8000) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
+ const legacyQuestionMaxLen = 8000;
+ // ...
+       if (taskId === '' || question === '' || question.length > legacyQuestionMaxLen) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');


─── packages/mobile-core/src/task-office/task-office.ts:378-379 ───
[style · low] settle() 的 epoch 选择改写成了嵌套三元（`kind === 'home' ? homeEpoch : kind === 'list' ? listEpoch
: legacyListEpoch`），属于检查清单明确禁止的嵌套三元表达式；分支再增多时可读性会持续下降。建议改为 switch 或查表（如 `const epochOf = { home: ()
=> homeEpoch, list: () => listEpoch, legacy: () => legacyListEpoch }; epochOf[kind]()`）。

    const settle = <T>(epoch: number, kind: 'home' | 'list' | 'legacy', lease: ScopeLease, value: T): T => {
-     const currentEpoch = kind === 'home' ? homeEpoch : kind === 'list' ? listEpoch : legacyListEpoch;
+     const currentEpoch = kind === 'home' ? homeEpoch : kind === 'list' ? listEpoch : legacyListEpoch; // → 改为 switch (kind) { case 'home': ... } 或查表


─── internal/application/service/task_grant.go:60-63 ───
[bug · medium] 任意 GetByID 错误都被折叠成 404：sessionRepository.GetByID 对未命中返回
apperrors.ErrSessionNotFound，对基础设施故障（连接断开、超时等）返回原始 DB 错误；这里 `if err != nil` 一律转
NewNotFoundError，使数据库故障在 Grant/Revoke/List 三个端点上都伪装成 "task not found"（ResolveTaskAccess
同模式）。这既掩盖了失败操作（客户端/监控看到 404 而非 5xx，移动端可能据此丢弃本地任务状态），也偏离了仓库惯例（同 PR 的 GetRunForGrantedReader 对
ErrRecordNotFound→404、其余错误透传 500）。建议仅把 miss（ErrSessionNotFound / gorm.ErrRecordNotFound）映射为
404，其余错误原样返回，由 taskGrantWriteError 走 500 分支。

  	session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
  	if err != nil {
+ 		if !errors.Is(err, apperrors.ErrSessionNotFound) && !errors.Is(err, gorm.ErrRecordNotFound) {
+ 			return nil, err
+ 		}
  		return nil, apperrors.NewNotFoundError("task not found")
  	}


─── internal/handler/commercial_task_budget.go:103-106 ───
[security · medium] 扩额门禁的"任务所有者"谓词取 agent_runs.owner_id，与本 PR 自己确立的权威（ADR-0004 / types.TaskGrant /
TaskGrantStore 注释：owner 权威是 sessions.user_id，且 TaskGrantStore.TaskOwnerID
就是为此提供的解析器）不一致。今天二者恰好重合（AgentRunStore.Admit 要求 sessions.user_id == 提交者，SetOwnerID 无生产调用方），但同一特性集已交付
TaskRoleCanRun(collaborator)=true 并计划由 #36/#37 让协作者请求新 run；届时协作者创建的 run 会令协作者（非 owner、无 billing
授权）通过本门禁扩额，而真正的任务 owner 反被 403 —— 与注释引用的 CONTEXT 规则（"只有任务所有者或获授权的账单管理员可以增加上限"）倒挂，且撤销 grant
也不会收敛其扩额权。建议经 run 的 session 解析 owner（join sessions on agent_runs.session_id 取 sessions.user_id，或复用
TaskOwnerID 语义）。

  	var ownerID string
  	err := h.db.WithContext(ctx).Table("agent_runs").
- 		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
- 		Select("owner_id").Scan(&ownerID).Error
+ 		Joins("JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
+ 		Where("agent_runs.tenant_id = ? AND agent_runs.run_id = ?", tenantID, runID).
+ 		Select("sessions.user_id").Scan(&ownerID).Error


─── internal/application/service/task_grant.go:173-174 ───
[bug · medium] ResolveTaskAccess 只凭 grant 行即返回 viewer/collaborator，不做成员 active 校验；而同一特性里
AgentRunStore.GetRunForGrantedReader 在查询时实时 JOIN tenant_members（status='active' AND deleted_at IS
NULL），使被停用/移除成员的读权限即时收敛。两者不一致的后果：成员被停用后其 grant 行仍在，本方法仍解析出
Collaborator（TaskRoleCanRun→true），而按计划文档本方法正是 #36/#37 运行入口与 #46 Material 面的授权数据源——消费方按此放行会绕过停用收敛（读面
fail closed、运行面却放行）。建议在非 owner 的 grant 命中分支先经 s.members 校验 active 成员身份（端口已注入），非 active 则收敛为
TaskAccessNone。

  	} else if found {
+ 		member, mErr := s.members.Get(ctx, caller.UserID, caller.TenantID)
+ 		if mErr != nil {
+ 			return types.TaskAccess{}, mErr
+ 		}
+ 		if member == nil || member.Status != types.TenantMemberStatusActive {
+ 			return access, nil
+ 		}
  		access.GrantRole = role


─── packages/contracts/src/marketplace/agent-adoption.ts:41-42 ───
[bug · high] 两个状态常量缺少 `as const`（仓库惯例见 craft/index.ts 的 `as const satisfies readonly ...`），类型被拓宽为
`string[]`：`enumValue` 的类型参数 T 从实参推断为 `string` 而非字面量联合，于是 `parseVariantRow` 中 `state: enumValue(row,
'state', VARIANT_STATES, path)` 的返回类型是 `string`，无法赋给 `AgentAdoptionVariant.state` 的 `'draft' |
'mapped' | 'tested' | 'published'`，是一条确定的 TS2322。注意：`packages/contracts/src/index.ts:729-730` 已把本模块
re-export 进契约根入口，而根 package.json 的 `typecheck:shared` tsc 入口包含
`packages/contracts/src/index.ts`，合并后整个工作区类型检查门禁会失败；t59 计划自带的验证只跑 `tsx
--test`（剥类型不检查），发现不了该错误。`parseAvailableAgentListResponse` 里的 CAPABILITY_STATES 目前靠末尾 `as
AvailableAgent` 断言侥幸通过，也应一并修复。

- const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published'];
- const CAPABILITY_STATES = ['supported', 'unavailable', 'forbidden'];
+ const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published'] as const;
+ const CAPABILITY_STATES = ['supported', 'unavailable', 'forbidden'] as const;


─── internal/handler/agent_adoption.go:231-231 ───
[bug · medium] 发布响应的 `data` 是 `{"variant": {...}}`，而同文件另外三个返回 Variant
的端点（CreateVariant/UpdateCapabilityMapping/TestVariant）的 `data` 都是 variant 本体；契约侧
packages/contracts/src/marketplace/agent-adoption.ts 只提供了 `parseVariantResponse`（把 `data` 直接按
variant 解析），并没有与发布信封匹配的解析器。TS 调用方对 publish 响应套用 parseVariantResponse 必然抛
ContractError('data.id')，除非手写特判。建议与其余端点对齐直接返回 variant（result.Variant 本来就是唯一的载荷），或在契约里补一个
parsePublishVariantResponse。

- 	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"variant": adoptionVariantDTO(result.Variant)}})
+ 	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(result.Variant)})


─── internal/application/repository/agent_adoption.go:184-186 ───
[bug · medium] 状态回写缺少 state 守卫（TOCTOU）：服务层在事务外先读 variant.State 做前置检查（仅 draft/mapped/tested
允许重映射），但事务内这条 UPDATE 只按 tenant+id 匹配、无条件覆盖 state。并发 TestVariant/PublishVariant 的 CAS
若在前置检查之后、本事务提交之前落地，会被静默覆盖——最坏情形是 publish 已 CAS 到 published，本 UPDATE 把它打回 draft/mapped 并清换映射，而
local_agent_id/local_agent_version_id 与 tested/published 戳残留在行上，available-agents 读模型随即丢失该 Agent。这正违背
UpdateVariantState 注释声明的『fails loudly instead of double-applying』语义。建议给 UPDATE 加 `AND state IN ?`
守卫（接口增加 expectedFrom 参数，由服务层传 {draft,mapped,tested}），RowsAffected != 1 时回滚并返回
ErrAgentAdoptionVariantTransition；回退到 draft/mapped 时顺带清空 tested_by/tested_at，避免陈旧的已测试元数据。

  		updated := tx.Model(&types.AgentAdoptionVariantEntity{}).
- 			Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).
+ 			Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(variantID), expectedFrom).
  			Updates(map[string]any{"state": nextState, "updated_at": now})
+ 		if updated.Error != nil {
+ 			return updated.Error
+ 		}
+ 		if updated.RowsAffected != 1 {
+ 			return ErrAgentAdoptionVariantTransition
+ 		}


─── internal/modules/workbench/service/workbench/interaction.go:212-219 ───
[bug · low] ListPending 对已过期 pending 行做了 SQL 谓词 + Go 侧双重剪除（且过期行无任何清理路径、status 永远停留
'pending'），但注释中声称对齐的 overview 待处理投影（overview.go 的 pending 查询）既没有 expires_at 谓词、也没有 Go
侧跳过：同一条已过期交互会永久计入首页 PendingInteractions 计数/列表，却永远不会出现在 Attention Inbox
中——两个投影实际会在过期维度上分歧（首页有待办计数、收件箱为空），与注释目标 'inbox and the home projection never disagree' 相悖。建议
overview 的 pending 查询补上相同的 `(expires_at IS NULL OR expires_at > ?)` UTC 谓词，或在注释中明确记录这一刻意差异。

- // ListPending returns the owner's open interactions across runs: the Attention
- // Inbox read (T08). It reuses the overview's archived-task LEFT JOIN so the
- // inbox and the home projection never disagree, and expired prompts are pruned
- // by a bound SQL predicate so they can never fill the LIMIT window and starve
- // live prompts; a Go-side skip remains as a clock-skew guard between the
- // predicate and the scan. Interactions whose run row is dangling stay visible:
- // the LEFT JOIN keeps sessions.archived_at NULL, matching the overview
- // semantics exactly.
+ // ... (建议同步为 overview.go 的 pending 查询追加相同的过期谓词：)
+ // s.db.WithContext(ctx).
+ // 	Table("workbench_interactions").
+ // 	Select("workbench_interactions.id, workbench_interactions.kind, workbench_interactions.created_at").
+ // 	Joins("LEFT JOIN agent_runs ar ON ar.tenant_id = workbench_interactions.tenant_id AND ar.run_id = workbench_interactions.run_id").
+ // 	Joins("LEFT JOIN sessions ON sessions.tenant_id = ar.tenant_id AND sessions.id = ar.session_id").
+ // 	Where("workbench_interactions.tenant_id = ? AND workbench_interactions.owner_id = ? AND workbench_interactions.status = ? AND sessions.archived_at IS NULL AND (workbench_interactions.expires_at IS NULL OR workbench_interactions.expires_at > ?)", tenantID, ownerID, "pending", time.Now().UTC()).


LLM retry report summary: 16 of 440 requests affected -- 5 requests failed, 11 requests recovered after retry

Review planning (6 requests):
- docs/architecture/moves/workbench.yaml,internal/handler/app_connector_action.go,internal/handler/session/workbench_artifacts.go,internal/handler/session/workbench_commands.go,internal/handler/session/workbench_read.go,internal/modules/workbench/interaction.go,internal/modules/workbench/service/workbench/interaction.go,internal/router/router.go,internal/router/routes_app_connectors.go,internal/router/routes_workbench.go: timed out -> failed
- packages/api-client/src/mobile/inbox.ts,packages/api-client/src/mobile/interactions.ts,packages/contracts/src/mobile/interaction-inbox.ts,packages/mobile-core/src/inbox/deep-link.ts,packages/mobile-core/src/inbox/in-memory-inbox-remote.ts,packages/mobile-core/src/inbox/notification-inbox.ts,packages/mobile-core/src/task-office/attention-inbox.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/package.json,apps/mobile/src/adapters/intent-log.ts,apps/mobile/src/adapters/request-id.ts,apps/mobile/src/adapters/sse-stream.ts,apps/mobile/src/composition.ts,packages/api-client/package.json,packages/contracts/src/index.ts,packages/domain/src/mobile/index.ts,packages/mobile-core/src/index.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- apps/mobile/src/app/tasks.tsx,apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx,apps/mobile/src/screens/TasksScreen.tsx,packages/mobile-core/src/task-office/in-memory-task-backend.ts,packages/mobile-core/src/task-office/task-office-errors.ts,packages/mobile-core/src/task-office/task-office.ts: rate limited (HTTP 429) -> succeeded
- internal/application/repository/agent_adoption.go,internal/application/service/agent_adoption.go,internal/container/container.go,internal/handler/agent_adoption.go,internal/router/routes_agent_adoption.go,internal/types/agent_adoption_persistence.go,internal/types/interfaces/agent_adoption.go,packages/contracts/src/marketplace/agent-adoption.ts,packages/domain/src/mobile/lead-agent.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 1 more

Core review (10 requests):
- apps/mobile/src/app/attention.tsx,apps/mobile/src/app/inbox.tsx,apps/mobile/src/attention-inbox-integration-smoke.ts,apps/mobile/src/attention-inbox-view.ts,apps/mobile/src/screens/AttentionInboxScreen.tsx,apps/mobile/src/screens/HomeScreen.tsx,apps/mobile/src/screens/InboxScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks/legacy.tsx,apps/mobile/src/legacy-tasks-integration-smoke.ts,apps/mobile/src/legacy-tasks-view.ts,apps/mobile/src/screens/LegacyTasksScreen.tsx,internal/application/repository/workbench_legacy_list.go,internal/handler/session/workbench_legacy_list.go,packages/api-client/src/mobile/legacy-tasks.ts,packages/mobile-core/src/task-office/legacy-tasks.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/api-client/src/mobile/inbox.ts,packages/api-client/src/mobile/interactions.ts,packages/contracts/src/mobile/interaction-inbox.ts,packages/mobile-core/src/inbox/deep-link.ts,packages/mobile-core/src/inbox/in-memory-inbox-remote.ts,packages/mobile-core/src/inbox/notification-inbox.ts,packages/mobile-core/src/task-office/attention-inbox.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/device-identity.ts,apps/mobile/src/adapters/push-token.ts,apps/mobile/src/device-inbox-integration-smoke.ts,packages/api-client/src/mobile/devices.ts,packages/mobile-core/src/device/device-registry.ts,packages/mobile-core/src/device/in-memory-device-remote.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- apps/mobile/src/adapters/material-adapters.ts,apps/mobile/src/app/tasks/materials.tsx,apps/mobile/src/material-integration-smoke.ts,apps/mobile/src/materials-view.ts,apps/mobile/src/screens/MaterialsScreen.tsx,packages/api-client/src/mobile/materials.ts: rate limited (HTTP 429) -> succeeded
- ... and 5 more

Per-attempt detail: --format json (retry_report).
