Review partially complete: 50 finding(s); 28 of 62 selected item(s) failed.

─── apps/miniprogram/tests/assembly.test.mjs:132-132 ───
[maintainability · low] 两处 snapshot stub 重复了完整载荷结构（仅 events 不同）。本次 diff 本身体现了该重复的维护成本：契约新增
incomplete/confirmed_watermark 字段时需要同步修改两处，未来契约再演进（如 task 段变为必填）仍需多点修改且易漂移。建议提取一个 helper（如
snapshotBody(events)），让 stub 载荷与契约字段保持单点对齐。

-     'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } }),
+ // 与 execDto/execEvent 并列的小工厂，保证快照载荷结构与契约单点对齐：
+ const snapshotBody = (events = []) => ({ execution: execDto(5), watermark: 5, incomplete: false, confirmed_watermark: 5, events });
+ // 用法：
+ // 'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: snapshotBody([execEvent(5)]) } })


─── packages/mobile-core/src/shelf/resource-shelf.ts:73-74 ───
[bug · medium] 并发 browse() 交错问题：browse() 内部的 agents/knowledge/connections/verdicts
是共享可变快照，且以"完成顺序"覆盖而非"开始顺序"。上层 controller（resources-view.ts）在 authorization-revoked 事件触发的 `void
load()` 与用户 refresh 并发时不等待 tail，两个在途 browse 中后完成者（可能是先启动、网络较慢的旧请求）会用旧数据覆盖新快照：UI 侧有 generation
守卫只保护页面展示，但 shelf 内部快照无代次保护——此后 selection() 将基于过期快照裁决（例如放行一个新快照已 403 清空的 agent），且 `previous`
边缘检测可能基于过期值漏触发/重复触发 authorization-revoked。建议在 handle 闭包内引入单调递增序号，Promise.all 完成后若已被更新的 browse
超越则丢弃本次写入。

-           guard(); // 在途期间 scope 已关闭/撤销 → 丢弃迟到结果（spec §5.3 不变量同源）
+           // open() 闭包内：let browseSeq = 0;
+           const seq = ++browseSeq;
+           const [agentResult, knowledgeResult, connectionResult] = await Promise.all([...]);
+           guard();
+           if (seq !== browseSeq) {
+             // 已被更新的 browse 超越：丢弃过期写入，避免快照回退与 selection 误判
+             return pageOfCurrentSnapshot();
+           }
            const previous = verdicts;


─── packages/domain/src/mobile/resource-presentation.ts:52-53 ───
[maintainability · low] kind 字段的 fail-closed 不一致：同函数中 state 未知值被映射为 'unknown' 且 capability 落为
unavailable（无事实不放行），但 kind 未知值被静默回退为 'personal'，row.id 缺失时落为空串仍会生成一行投影（ResourcesScreen 会渲染 `personal
connection — ...`）。wire 适配层（api-client resources.ts）对 kind/state
是透传，服务端未来出现新枚举值或脏行时会被误分类为个人连接展示，与文件头及 connectionCapability 自述的"没有能力事实不得放行、不猜测"原则相悖。建议对 kind
做显式白名单并对齐 state 的处理策略（至少过滤无 id 的行）。

-     id: String(row.id ?? ''),
-     kind: row.kind === 'space' ? 'space' : 'personal',
+     id: row.id === undefined || row.id === null || row.id === '' ? undefined : String(row.id),
+     kind: row.kind === 'personal' || row.kind === 'space' ? row.kind : 'personal', // TODO: 未知 kind 与 state 的 'unknown' 对齐，脏行（无 id）应整行拒绝而非展示


─── packages/mobile-core/src/shelf/resource-shelf.ts:96-97 ───
[maintainability · low] 同一 browse() 内三类资源的 keyword 过滤行为不一致：agents 经 filterAgents(keyword)、knowledge
手工 keyword 过滤，connections 原样返回（当前模型确实已无 name/title 文本字段，但这一例外没有任何注释说明，后续维护者接入搜索时容易误以为 connections
也被过滤）。另外每次 browse 都全量拉取三路远端、无 TTL/节流，若上层未来接"输入即搜"将产生成倍请求开销（当前 controller 未传 query、UI
无搜索框，属潜在问题而非现症）。建议至少以注释固化"connections 不参与 keyword 过滤"的契约，或在 ResourceQuery 文档中说明。



─── apps/mobile/plugins/ios-xcode27.js:123-127 ───
[bug · medium] 冷启动场景只转发了 connectionOptions.urlContexts（自定义 scheme），未转发
connectionOptions.userActivities（通用链接）与 notificationResponses。文件头注释声称"custom-scheme/universal-link
URLs ... forwards them back"，但通用链接冷启动时 UIKit 是通过 willConnectTo 的 userActivities
投递的，此路径会被静默丢弃。已核实仓库当前未配置 associatedDomains/applinks、OIDC 回调走 weknora
scheme，故现网影响有限；但一旦后续接入通用链接（如分享/深链场景），冷启动初始路由将静默失效，且与代码注释的承诺不符。建议在 willConnectTo 中把 userActivities
也转发回 AppDelegate.application(_:continue:restorationHandler:)（复用 scene(_:continue:) 的转发逻辑）。

      (UIApplication.shared.delegate as? AppDelegate)?.startReactNativeOnce(in: window)
      // Cold launch by URL: UIKit delivers it here (not through
      // scene(_:openURLContexts:)), so forward it before the JS bundle runs and
-     // expo-router reads Linking.getLinkingURL() as the initial route.
+     // expo-router reads Linking.getLinkingURL() as the initial route. Custom
+     // schemes arrive via urlContexts; universal links arrive via userActivities.
      forwardURLContexts(connectionOptions.urlContexts)
+     for activity in connectionOptions.userActivities {
+       guard let appDelegate = UIApplication.shared.delegate as? AppDelegate else { continue }
+       _ = appDelegate.application(
+         UIApplication.shared,
+         continue: activity,
+         restorationHandler: { (_: [UIUserActivityRestoring]?) in })
+     }


─── apps/mobile/plugins/ios-xcode27.js:239-251 ───
[maintainability · medium] 插件在 withInfoPlist（RCTNewArchEnabled = false）与
withPodfileProperties（'expo.newArchEnabled': 'false'）两处硬编码关闭新架构，完全不读取 app.json 的 newArchEnabled
声明；配合 app.json 中该开关为跨端字段（Android 一并被降为旧架构），未来重新启用新架构时 app.json 的翻转在 iOS
侧会被插件静默覆盖，造成双端渲染架构与配置声明漂移且无任何告警。建议至少在声明值与强制值冲突时 fail loudly（或复用 prebuild 已写入 mod.modResults 的
'expo.newArchEnabled' 值作为判断依据），把"iOS 27 下 Fabric 不可用"这一约束显式化而不是静默改写。

-   config = withInfoPlist(config, (mod) => {
-     mod.modResults.UIApplicationSceneManifest = SCENE_MANIFEST;
-     mod.modResults.RCTNewArchEnabled = false;
-     return mod;
-   });
    config = withPodfileProperties(config, (mod) => {
+     // app.json 的 newArchEnabled 已由 prebuild 写入 podfile properties；iOS 27 兼容层
+     // 只在声明为 true（Fabric 在 iOS 27 模拟器渲染为空）时强制改写并显式告警，避免静默覆盖声明。
+     if (mod.modResults['expo.newArchEnabled'] === 'true') {
+       console.warn('ios-xcode27 plugin: newArchEnabled=true is not supported on iOS 27; forcing legacy arch on iOS.');
+     }
      mod.modResults = {
        ...mod.modResults,
        'ios.deploymentTarget': DEPLOYMENT_TARGET,
        'expo.newArchEnabled': 'false',
      };
      return mod;
    });


─── apps/mobile/plugins/ios-xcode27.js:207-210 ───
[maintainability · low] applyPodfileClamp 以子串 'Xcodeproj::Plist.read_from_path' 作为幂等标记，若未来 RN
模板/其它插件的 post_install 自身包含相同调用，会被误判为"已注入"而跳过钳制，Xcode 27 构建问题将悄然复发。建议在 PODFILE_CLAMP 首行注入唯一标记注释（如 '#
ios-xcode27:deployment-clamp'）并以该标记做幂等判断。

+ const CLAMP_MARKER = '# ios-xcode27:deployment-clamp';
+ 
  function applyPodfileClamp(contents) {
-   if (contents.includes('Xcodeproj::Plist.read_from_path')) {
+   if (contents.includes(CLAMP_MARKER)) {
      return contents;
    }


─── apps/mobile/plugins/ios-xcode27.js:225-228 ───
[bug · low] raiseDeploymentTargets 直接对 IPHONEOS_DEPLOYMENT_TARGET 做 parseFloat：pbxproj 中该值可能带引号（如
'"15.1"'）或为变量（'$(DEPLOYMENT_TARGET)'），parseFloat 得到 NaN 后被静默跳过，低版本目标不会被抬升。当前 Expo
模板主工程值为不带引号的数字所以能工作（测试也只覆盖了裸字符串），但这是一个静默失败的启发式。建议先剥离引号再解析，解析失败时至少告警。

-       const current = parseFloat(settings.IPHONEOS_DEPLOYMENT_TARGET);
+       const current = parseFloat(String(settings.IPHONEOS_DEPLOYMENT_TARGET).replace(/['"]/g, ''));
        if (!Number.isNaN(current) && current < parseFloat(DEPLOYMENT_TARGET)) {
          settings.IPHONEOS_DEPLOYMENT_TARGET = DEPLOYMENT_TARGET;
        }


─── .gitignore:97-98 ───
[style · low] 新增的 apps/mobile/ios/ 规则实际冗余：本文件上方已存在不带前导斜杠的 `ios/` 规则（匹配任意层级同名目录），apps/mobile/ios/
已被其忽略。保留仅具自文档化价值（注释说明 CNG 产物）；若不希望依赖全局规则，建议把既有 `ios/` 改为锚定根路径 `/ios/` 后再保留本条，避免两套语义并存。



─── packages/mobile-core/src/vault/scoped-vault.ts:83-84 ───
[bug · high] 索引键与行键命名空间冲突：`indexKey(scopeKey)` 生成 `${scopeKey}.drafts.index`，而 `rowKey(scopeKey,
"index")` 生成完全相同的键，且 `DRAFT_ID_PATTERN` 允许 id="index"。`put({id:"index"})` 会用 base64 密文覆写明文索引，此后
`readIndex` 的 `JSON.parse(raw)`（无 try/catch）抛裸 SyntaxError，list/put/remove 全部依赖 readIndex，导致该 scope
的整个 drafts 仓储被砖化、草稿列表永久不可达。`remove("index")` 同样会误删索引。建议显式保留字拒绝（如 id==='index' 时抛 VAULT_ID），或将索引键移入
id 不可能碰撞的独立命名空间（如 `${scopeKey}.drafts#index`）。

-   const indexKey = (scopeKey: string): string => `${scopeKey}.drafts.index`;
+   const indexKey = (scopeKey: string): string => `${scopeKey}.drafts#index`;
    const rowKey = (scopeKey: string, id: string): string => `${scopeKey}.drafts.${id}`;
+   // put/remove 入口处：if (input.id === 'index') throw new Error('VAULT_ID');


─── packages/mobile-core/src/vault/scoped-vault.ts:170-175 ───
[bug · medium] rotate 并非注释所称的原子操作：两阶段只保证「解密失败先于任何写入」，不保证重封写入中途失败的一致性。若循环中某行 `storage.write`
抛错，已重封的行用 newKey 加密但 `writeWrappedKey(scopeKey, newKey)` 未执行，keyStore 仍是旧 key——这些行永久 VAULT_DECRYPT
不可恢复（newKey 已丢失）。同样，模块内无互斥：并发的 `put` 在密钥交换窗口内用旧 session.key 写行，交换后同样不可解。建议至少用模块内 promise 队列串行化
rotate 与 put/remove，并如实修正「两阶段原子」的注释边界；更彻底的方案是引入行级密钥版本元数据以支持中断后恢复。



─── packages/mobile-core/src/vault/scoped-vault.ts:85-90 ───
[bug · medium] 索引维护存在健壮性与竞态缺陷：(1) `JSON.parse(raw)` 无 try/catch，索引行损坏（如被覆写为非 JSON）时抛裸 SyntaxError
而非受控 VAULT_* 错误；(2) 值为合法 JSON 但非 string 数组时静默返回 []，随后首次 put 会以单元素索引覆写，使既有全部行永久孤儿化（get 可达但 list
永不显示、无法经 remove 清理）；(3) 并发 put 各自 readIndex 后回写会互相覆盖造成孤儿行（JS 单线程下 await 交错即可触发）。建议：JSON.parse 包
try/catch 抛受控错误（或返回 [] 并告警），并以模块内 promise 链串行化索引的读-改-写。

    const readIndex = async (scopeKey: string): Promise<string[]> => {
      const raw = await ports.storage.read(indexKey(scopeKey));
      if (raw === null) return [];
+     try {
-     const value: unknown = JSON.parse(raw);
+       const value: unknown = JSON.parse(raw);
-     return Array.isArray(value) && value.every((id) => typeof id === 'string') ? value : [];
+       return Array.isArray(value) && value.every((id) => typeof id === 'string') ? value : [];
+     } catch {
+       throw new Error('VAULT_INDEX');
+     }
    };


─── packages/mobile-core/src/vault/scoped-vault.ts:71-72 ───
[bug · medium] requireSession 对长度 ≠ 32 的既有 wrapped key 静默走「生成新 key 并覆写
keyStore」分支：一旦持久化值损坏（或未来密钥长度/版本变更），该 scope 的全部旧密文即刻永久不可解，且没有任何显式错误或告警，属于静默数据丢失路径。建议对长度异常的 wrapped
key 抛受控错误（如 VAULT_KEY_LENGTH）或至少输出告警，把「重新生成」显式化，避免破坏性覆盖。

      const wrapped = await ports.keyStore.readWrappedKey(scopeKey);
-     if (wrapped && wrapped.length === KEY_LENGTH) {
+     if (wrapped) {
+       if (wrapped.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
+       const session: ScopeSession = { key: wrapped, destroyed: false };
+       sessions.set(scopeKey, session);
+       return session;
+     }


─── packages/mobile-core/src/vault/scoped-vault.ts:190-192 ───
[documentation · low] inspectPolicy 宣告 drafts 保留 30 天（RETENTION_DAYS=30），但整个模块（含 open/list/put）没有任何
TTL 检查或过期清理实现，策略与实际行为不符，草稿存储会随时间无界增长。建议在 open 或 list 时按 entry.updatedAt 清理超期条目落地该策略，或在策略中移除未实现的
retentionDays 宣告以免误导调用方。



─── packages/mobile-core/src/vault/scoped-vault.ts:33-35 ───
[security · low] scopeKeyOf 以 '.' 作为段分隔符，但 encodeURIComponent 不转义 '.'（也不转义 ! ~ * ' ( ) - _）：不同
(userId, tenantId) 组合（如 userId="a"+tenantId="b.c" 与 userId="a.b"+tenantId="c"）会拼出相同 scopeKey，理论上导致跨
scope 共享同一数据密钥与行前缀，Deployment×User×Tenant 隔离退化。实际风险取决于服务端 ID 字符集，但隔离边界不应依赖此假设。建议改用
encodeURIComponent 必然转义的分隔符（如 ':' → %3A），或对每段做 base64url/hex 编码。

  export function scopeKeyOf(scope: LeaseScope): string {
-   return `weknora.vault.v1.${encodeURIComponent(scope.deploymentOrigin)}.${encodeURIComponent(scope.userId)}.${encodeURIComponent(scope.tenantId)}`;
+   return `weknora.vault.v1:${encodeURIComponent(scope.deploymentOrigin)}:${encodeURIComponent(scope.userId)}:${encodeURIComponent(scope.tenantId)}`;
  }


─── packages/mobile-core/src/vault/scoped-vault.ts:37-41 ───
[maintainability · low] bytesToBase64/base64ToBytes 与 apps/mobile/src/adapters/vault-adapters.ts
中的实现完全重复，应提取为共享 util 统一维护（并顺带优化大草稿下的逐字节字符串拼接）。另外 list() 逐行串行 await、revoke()
逐行串行删除均为相互独立的存储操作，可按异步规范用 Promise.all 并行化（revoke 的密码学擦除语义不受删除顺序影响）。



─── apps/mobile/src/adapters/vault-adapters.ts:27-33 ───
[bug · medium] createSecureVaultStorage 将 base64 密文行直接写入 SecureStorePort，而 composition.ts
的原生路径（createNativeScopedVaultIfAvailable）后端是 expo-secure-store：Android 上单值上限约 2048 字节。base64 膨胀 4/3
加上 IV(12)+GCM tag(16)，草稿 body 超过约 1.4KB 即超限——写入失败或（旧版 SDK）仅警告不持久，造成草稿静默丢失。ports.ts 注释也表明行存储的预期后端是
SQLite/SecureStore，SecureStore 并不适合承载任意大小的密文行。建议行存储改走 SQLite（如 expo-sqlite）并在超限/失败时抛受控 VAULT_*
错误，或在写入前校验大小显式失败。



─── apps/mobile/src/adapters/vault-adapters.ts:10-11 ───
[bug · low] base64ToBytes 的 `atob(value)` 对损坏/非法 base64 会抛裸 InvalidCharacterError（DOMException），而
readWrappedKey 路径的其余失败均为受控 VAULT_* 错误——OS 存储异常时错误语义泄漏到上层，破坏模块的错误契约。建议包 try/catch 转为受控错误（如
VAULT_KEY_CORRUPT），与 createScopedVault 中 openRow 的处理方式保持一致。

  function base64ToBytes(value: string): Uint8Array {
-   const binary = atob(value);
+   let binary: string;
+   try {
+     binary = atob(value);
+   } catch {
+     throw new Error('VAULT_KEY_CORRUPT');
+   }


─── packages/mobile-core/src/task-office/task-office.ts:197-199 ───
[bug · medium] 归档/恢复写成功后仅置 accumulated = undefined 而不递增 listEpoch:settle() 只比对 epoch 与 lease,不感知
mutate。若 tasks() 的 backend.list 在途,其结果仍会通过 settle 校验并用归档前快照重建 accumulated,后续 moreTasks()
基于归档前数据继续分页,用户可见"已归档任务仍在列表"的过期视图。建议 mutate 成功后同步递增 listEpoch,使在途 tasks()/moreTasks() 按 SUPERSEDED
拒绝。

      await callBackend(() => action(trimmed));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
+     listEpoch += 1; // 列表内容已变更：使在途 tasks()/moreTasks() 按 SUPERSEDED 拒绝，避免归档前快照回流
      accumulated = undefined;


─── packages/mobile-core/src/task-office/task-office.ts:158-158 ───
[maintainability · medium] 已核实 apps/mobile 组合根（composition.ts 的 taskOfficeFor）只注入 { backend, detail,
lease }，未传 store：生产实际落入此 in-memory 默认回退。结果是 #35 的 App 重启事件历史恢复与 #32 的"缓存按 Deployment×user×Tenant
加密隔离"在生产装配中均不成立，且回退完全静默、无任何观测点（TaskProjectionStore 目前也不存在生产实现）。建议：生产缺注入时 open() fail fast（如抛
TASK_OFFICE_DETAIL_UNAVAILABLE 类错误）或至少一次性告警，in-memory 默认仅限测试装配使用。

-   const defaultDetailStore = createInMemoryTaskProjectionStore();
+   const defaultDetailStore = createInMemoryTaskProjectionStore(); // TODO(#32/#35)：生产组合根必须注入 Scoped Vault 支撑的 store；漏注入时 open() 应 fail closed 而非静默内存回退


─── packages/mobile-core/src/task-office/task-detail.ts:2-2 ───
[maintainability · medium] task-office.ts（值）导入 createTaskDetail，本文件又（值）导入 task-office.ts 的
TaskOfficeError，构成运行时循环依赖。当前双方均在函数体内延迟使用而未触发初始化错误，但任何一方在模块顶层直接引用（或打包器对循环的处理差异）即引入
TDZ/初始化顺序脆弱性——与本次更新中已修复的"循环导入警示"属同类问题。建议将 TaskOfficeError 及错误码下沉到独立的
task-office-errors.ts，task-office.ts 与 task-detail.ts 均只导入该错误模块，消除环。

- import { TaskOfficeError } from './task-office.ts';
+ import { TaskOfficeError } from './task-office-errors.ts'; // TaskOfficeError 下沉独立模块，消除 task-office.ts ↔ task-detail.ts 循环值导入


─── packages/mobile-core/src/task-office/task-detail.ts:269-270 ───
[maintainability · medium] streamFailed 以 message 全等字符串判定"流通道缺失"降级分支，而同函数内游标裁剪已基于 error.code ===
'TASK_STREAM_CURSOR_EXPIRED' 判定（api-client 侧注释明示这是"跨包契约码，不得改名或改用 ApiError
形态"），两处契约形态不一致。已核实当前链路可命中：mobile-runtime.ts:386 抛裸 Error('RUNTIME_STREAM_UNAVAILABLE')，经 api-client
stream 的 catch 透传（仅 ApiError 409 被改写）。但该契约仅靠 message 文本维系，一旦
runtime/适配器对该错误做包装、加前缀或本地化，降级分支即静默失效，退化为 stream-error 并触发针对不可用通道的有界自动 resync 重试（每次重连都要失败一次）。建议与
TASK_STREAM_CURSOR_EXPIRED 统一为 code 属性契约：runtime 抛错时挂 code，此处把 code 读取上移并优先按 code 判定。

      const message = error instanceof Error ? error.message : String(error);
-     if (message === 'RUNTIME_STREAM_UNAVAILABLE') {
+     const code = (error as { code?: unknown } | null)?.code; // 与 TASK_STREAM_CURSOR_EXPIRED 一致：跨包契约统一走 code 属性
+     if (message === 'RUNTIME_STREAM_UNAVAILABLE' || code === 'RUNTIME_STREAM_UNAVAILABLE') {


─── packages/mobile-core/src/task-office/task-detail.ts:238-241 ───
[performance · medium] 每条 SSE 事件都全量重写至多 200 条历史（persist →
store.save(history.slice(-TASK_DETAIL_HISTORY_LIMIT))），长生命周期任务产生约 O(limit²) 写放大；若未来接入 Scoped Vault
支撑的 store，每次 put 都要 sealRow 加密 + 写行（scoped-vault.ts put 路径），低端移动设备可能卡顿。同时 startStream 的 chain =
chain.then(...) 串行排队且无背压上限，事件洪峰时待处理队列内存无界。建议：前沿（events/committedCursor）立即推进、落盘按 N 条或空闲窗口合并（崩溃窗口最多重放
N 条，服务端按 seq 幂等去重），并评估 chain 队列长度上限。

      const nextEvents = [...events, event].slice(-TASK_DETAIL_HISTORY_LIMIT);
      try {
+       // 节流/批量持久化示意：前沿先推进，落盘按 N 条或空闲窗口合并（崩溃重放窗口由服务端 seq 幂等去重兜底）
        await persist(event.seq, nextEvents); // 先持久化，后推进已提交游标
      } catch (cause) {


─── packages/mobile-core/src/task-office/task-detail.ts:143-144 ───
[bug · medium] 广播循环无隔离：任一 listener 抛错会截断后续订阅者的通知，且异常泄入调用链（processEvent/streamEnded 路径会被 chain 的
.catch(() => undefined) 吞掉，导致部分订阅者永久丢失该视图且无任何痕迹）。多订阅场景（如详情屏 + 调试探针）下通知完整性无保障。建议逐个 try/catch 隔离。

      current = buildView(connection);
-     for (const listener of [...listeners]) listener(current);
+     for (const listener of [...listeners]) {
+       try { listener(current); } catch { /* 单个订阅者异常不得截断广播 */ }
+     }


─── packages/mobile-core/src/task-office/task-detail.ts:208-210 ───
[maintainability · low] void ports.backend.stream({...}) 仅覆盖 Promise rejection：若适配器在开流参数解析阶段同步
throw（已核实当前 api-client 实现为 async 函数不会，但 TaskDetailBackendPort 契约未禁止同步抛错），异常将以裸 Error 同步穿透 hydrate
而非走 streamFailed/TASK_OFFICE_BACKEND 路径。建议用 async 包装把同步抛错与 rejection 归一到同一处理路径。

-     void ports.backend.stream({
+     const opened = (async () => ports.backend.stream({
        runId: input.runId,
        cursor: committedCursor,


─── packages/mobile-core/src/task-office/task-timeline.ts:31-33 ───
[other · medium] runStatus === 'failed' 的任务被映射为 'active'（进行中）：CONTEXT.md:81
定义生命周期四态（进行中/已完成/已取消/已归档）未明确 failed Run 的归属，且 CONTEXT.md:209 明言"运行状态不直接决定任务生命周期"。当前实现下，Run
已终态（isTerminalRunStatus('failed') = true → 详情 drain、停止重连）的失败任务在详情页仍显示"进行中"，与 timeline
中"任务未能完成"并存，语义矛盾，也可能与首页 recentlyCompleted 段的归类不一致。succeeded/canceled 各有专属映射而 failed 落 active
属于隐性决策，建议与产品确认归属并在注释中固化（或引入显式生命周期态）。

    if (runStatus === 'succeeded') return 'completed';
    if (runStatus === 'canceled') return 'canceled';
+   if (runStatus === 'failed') return 'completed'; // 待产品确认：失败 Run 已终态，归入 completed 或引入显式生命周期态
    return 'active';


─── packages/mobile-core/src/task-office/task-timeline.ts:40-41 ───
[other · low] 镜像核对结论：事件类型集合（九种）与终态优先级 canceled > failed > succeeded 与 agent_run_snapshot.go:104-134
完全一致。但服务端在事件推进终态时还会同步设置 SettlementStatus = "settled"（agent_run_snapshot.go:129-131，以及
ExecutionStatus），本投影仅推进 runStatus——buildView 的 settlementStatus/executionStatus 在 SSE 推进终态后仍保持快照旧值，与
runStatus 短暂不一致，直至下次 resync。与注释"逐字镜像"的声明有偏差，建议补全镜像或在注释中说明取舍。

- /** 镜像 internal/application/repository/agent_run_snapshot.go:104-134 的推进与优先级。 */
+ /**
+  * 镜像 internal/application/repository/agent_run_snapshot.go:104-134 的推进与优先级。
+  * 已知取舍：服务端推进终态时还会同步 SettlementStatus='settled' 与 ExecutionStatus（agent_run_snapshot.go:121-131），
+  * 本投影仅推进 runStatus；settlementStatus/executionStatus 以快照为准，直至下次 resync。
+  */
  export function terminalRunStatusOf(base: string, events: ReadonlyArray<{ type: string }>): string {


─── packages/mobile-core/src/task-office/task-timeline.ts:77-77 ───
[maintainability · low] 中文 UI 文案（EVENT_SUMMARIES、KIND_LABELS、timelineKindLabel 及进入
TaskDetailView.timeline 的 summary）内嵌于 mobile-core 深模块：已核实消费面仅 apps/mobile 的
TaskDetailScreen.tsx，core 被绑定单一语言，也违背核心域/展示层 seam
分离（packages/domain/src/mobile/resource-presentation.ts 已有展示投影上置先例）。另外 KIND_OF_TYPE 与 EVENT_SUMMARIES
键集高度重复，可合并为单一 type → { kind, summary } 投影表。建议将文案表上移至 presentation 层或参数化注入。

- const EVENT_SUMMARIES: Record<string, string> = {
+ const TYPE_PROJECTION: Record<string, { kind: TaskTimelineKind; summary: string }> = {
+   'run.started': { kind: 'run_status', summary: '任务已开始' },
+   // 合并 KIND_OF_TYPE 与 EVENT_SUMMARIES，消除重复键集
+ };


─── packages/mobile-core/src/task-office/task-office.ts:147-147 ───
[style · low] 接口名 listAccumulation 小写开头，违反 TS 接口 PascalCase 命名约定（同文件其余接口如
TaskOfficeQuery、TaskListPage 均符合）。

- interface listAccumulation {
+ interface ListAccumulation {


─── packages/mobile-core/src/task-office/task-detail.ts:164-164 ───
[maintainability · low] 若干可观测性/魔数问题集中在此：1) store.load(...).catch(() => undefined) 静默吞掉持久化读取的真实 IO
故障，按损坏缓存降级本身合理（snapshot 会重建），但零观测信号使缓存失效原因无从排查，建议留 debug 钩子或注释说明；2) processEvent 中 duplicateSeqs 的
.slice(-50) 为裸魔数，建议提为具名常量（对照 TASK_DETAIL_HISTORY_LIMIT 的做法）；3) in-memory-task-backend.ts 的
emptyOverview 默认 asOf '2026-09-23T00:00:00Z' 是硬编码日期魔串，建议改为 new Date().toISOString() 或参数必填。

-     const persisted = await ports.store.load(input.runId).catch(() => undefined);
+     // 缓存读取失败按损坏缓存降级（由 snapshot 重建），但保留可观测信号便于排查
+     const persisted = await ports.store.load(input.runId).catch((error) => { debugLog('task-detail store load failed', error); return undefined; });


─── packages/mobile-core/src/runtime/mobile-runtime.ts:206-209 ───
[bug · low] `accessTokenFor` 的 refresh 路径在 `await refreshedCredential(...)` 之后未复核
scope（`current(epoch, deployment)`）就直接回写 `activeCredential` 并返回 token。若 refresh
期间发生租户/部署切换（`begin()` 会将 `activeCredential` 清空），此处会把已撤销 scope 的凭据重新写回 `activeCredential`，并向调用方返回旧
scope 的 access token。虽然实际影响被 shelf 的 `guard()` 与 surface 检查兜底，但这与本模块自身声明的「guards scope at every
step」及 `sendWithCredential` 在每个 await 后复核 `current` 的纪律不一致。建议在赋值前补一次 epoch 复核。

      const refreshed = await refreshedCredential(epoch, deployment, activeCredential);
      if (!refreshed) throw new Error('SHELF_AUTH');
+     if (!current(epoch, deployment)) throw new Error('SHELF_SCOPE');
      activeCredential = refreshed;
      return refreshed.token;


─── packages/mobile-core/src/runtime/mobile-runtime.ts:387-392 ───
[performance · low] `authorizedEventStream` 在 scope 变化（epoch 前移）后只能等到下一个 chunk 到达时经 `guardedChunk`
抛错才会终止读取；若服务端此后不再推送数据，底层 SSE 连接会一直挂起（`sse-stream.ts` 的 `reader.cancel()` 在 finally 中，只有 read
返回/抛错后才会执行），造成连接泄漏。`RuntimeAuthorizedRequest` 已带 `signal` 字段但 Runtime 未主动接线。建议为每个在途流创建
AbortController 并在 `revoke()` 时统一 abort（与调用方 signal 合并），确保切租户/切部署/登出时立即释放连接。

        const requestEpoch = epoch;
+       const controller = new AbortController();
        const guardedChunk = (chunk: string): void => {
-         if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
+         if (!current(requestEpoch, deployment)) { controller.abort(); throw new Error('RUNTIME_SCOPE_CHANGED'); }
          onChunk(chunk);
        };
-       await sendWithCredential(requestEpoch, deployment, (token) => transport(input, token, guardedChunk));
+       // revoke() 亦应统一 abort 在途 controller，scope 变化即刻断流而非等待下一个 chunk
+       await sendWithCredential(requestEpoch, deployment, (token) => transport({ ...input, signal: controller.signal }, token, guardedChunk));


─── packages/mobile-core/src/runtime/mobile-runtime.ts:35-39 ───
[maintainability · low] `tenantId()`（读 `me.tenant.id`，作为 activeTenantId）与新增的
`membershipTenantId()`（读 `membership.tenant_id`）是同概念的近重复实现，但归一化行为不一致：前者字符串不 trim，后者 trim。若两侧 wire
值出现空白差异，`tenantOptions` 的去重/`unshift` 会生成近似重复的租户项，且与 `activateTenant` 输入可能对不上。建议抽取共享的归一化函数供两者使用（统一
trim + 数字安全整数转换）。

- function membershipTenantId(value: unknown): string | undefined {
-   const id = typeof value === 'object' && value !== null ? (value as { tenant_id?: unknown }).tenant_id : undefined;
+ function normalizeTenantId(id: unknown): string | undefined {
    if (typeof id === 'string' && id.trim() !== '') return id.trim();
    return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? String(id) : undefined;
+ }
+ 
+ function membershipTenantId(value: unknown): string | undefined {
+   const id = typeof value === 'object' && value !== null ? (value as { tenant_id?: unknown }).tenant_id : undefined;
+   return normalizeTenantId(id);
  }
+ // tenantId() 同样改为 return normalizeTenantId(raw .id)


─── packages/mobile-core/src/index.ts:3-3 ───
[maintainability · low] `MobileRuntimePorts` 已从包根导出，但其引用的 `AuthorizedTransport` /
`AuthorizedStreamTransport`（ports.ts 中定义）未一并再导出。组合根（如 apps/mobile 的
composition.ts）当前靠内联函数结构匹配才能编译，无法对这两个 seam 类型命名（例如为共享 helper 或测试替身标注类型），公共 API 面不完整。建议补入 ports
类型导出列表。

- export type { AppLifecyclePort, CredentialStore, DeploymentRegistry, DeploymentStore, MobileRuntimePorts, OidcBrowserPort, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './runtime/ports.ts';
+ export type { AppLifecyclePort, AuthorizedStreamTransport, AuthorizedTransport, CredentialStore, DeploymentRegistry, DeploymentStore, MobileRuntimePorts, OidcBrowserPort, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './runtime/ports.ts';


─── packages/mobile-core/src/shelf/resource-shelf.ts:80-80 ───
[bug · low] knowledge 行缺少空 id 过滤，selection 存在 fail-open 缺口：agent 路径有双重防线（toAgentOptions 内部
`.filter((option) => option.id !== '')` + disabled 过滤），但 knowledge 路径直接 `map(toKnowledgeResource)`
不过滤空 id（该投影函数对缺失 id 落为 `''`，见 resource-presentation.ts `String(row.id ?? '')`）。此后 `selection({
knowledgeId: '' })` 中 `typeof input.knowledgeId === 'string'` 对空串放行，`knowledge.find((item) =>
item.id === '')` 会命中脏行，且 `knowledgeRefForPrompt(resource, { revoked: false })` 恒非 null，最终返回 `{
allowed: true, selection: { kind: 'knowledge_ref', knowledgeId: '' }
}`——空引用进入新任务表单。与本模块处处强调的"没有事实不得放行、不猜测"不变量相悖（agent 路径同样输入会得到 agent_not_found）。建议对齐 agent 路径过滤空 id，或在
selection 入口拒绝空串。

-           knowledge = knowledgeResult.ok ? knowledgeResult.value.map((row) => toKnowledgeResource(row)) : [];
+           knowledge = knowledgeResult.ok ? knowledgeResult.value.map((row) => toKnowledgeResource(row)).filter((resource) => resource.id !== '') : [];


─── packages/mobile-core/src/shelf/resource-shelf.ts:16-16 ───
[maintainability · low] 模块级共享的可变 `SUPPORTED` 对象被所有 shelf 实例复用（初始 verdicts 三类同引用、每次成功 browse 也复用），而
`ResourcePage.classVerdicts` 只是浅 Readonly（`Readonly<Record<...>>` 不冻结 verdict
对象本身），属公开导出接口。任何消费方一处误写（如 `page.classVerdicts.agent.state = 'forbidden'`）会同时污染：本 shelf 内部快照与 403
迁移检测（`previous[resourceClass].state !== 'forbidden'`）、以及进程内所有经 createResourceShelf 打开的
shelf（常量为模块级）。建议改为工厂函数每次返回新对象，或将 ResourceClassVerdict 字段声明为 readonly 配合 Object.freeze。

- const SUPPORTED: ResourceClassVerdict = { state: 'supported', reason: '' };
+ const supportedVerdict = (): ResourceClassVerdict => ({ state: 'supported', reason: '' });
+ // 使用处改为：agent: agentResult.ok ? supportedVerdict() : agentResult.verdict, …


─── apps/mobile/src/runtime-integration-smoke.ts:45-47 ───
[security · low] IPv4 防线遗漏了多个注释中宣称覆盖的保留段：100.64.0.0/10（CGNAT shared，实际可路由且常被
overlay/内网使用）、198.18.0.0/15（benchmarking）、192.0.0.0/24、192.0.2.0/24（TEST-NET-1）、198.51.100.0/24（TEST
-NET-2）、203.0.113.0/24（TEST-NET-3）。该函数同时被导出复用于 task-detail smoke 的 URL
防线（B2-F15），且函数注释承诺「拒绝……保留地址」，建议补齐上述段位。

    if (a === 192 && b === 168) return `${variable} must not target private addresses`;
    if (a === 169 && b === 254) return `${variable} must not target link-local addresses`;
+   if (a === 100 && b >= 64 && b <= 127) return `${variable} must not target shared (CGNAT) addresses`;
+   if (a === 198 && (b === 18 || b === 19)) return `${variable} must not target benchmarking addresses`;
+   if ((a === 192 && (b === 0 || b === 2)) || (a === 198 && b === 51 && octets[2] === 100) || (a === 203 && b === 0 && octets[2] === 113)) {
+     return `${variable} must not target reserved addresses`;
+   }
    return undefined;


─── apps/mobile/src/runtime-integration-smoke.ts:62-62 ───
[bug · low] IPv6 内嵌 dotted-quad 展开时把 16 位组写成「十进制字符串」（如 (192<<8)|168 → '49320'），而后续统一用
/^\[0-9a-f\]\{1,4\}$/ 按「十六进制组形态」校验：任何 ≥10000 的组（首 octet ≥ 40 即触发）会生成 5 字符字符串被判为非法 IPv6。后果是
::ffff:192.168.1.1 会以「invalid IPv6 host」拒绝而非命中「私网」分类（IPv4 规则被绕过），且合法公网 mapped 地址（如
::ffff:40.1.1.1）会被误拒。虽是 fail-closed 方向的误拒（且 WHATWG URL 通常已把 dotted-quad
归一化为十六进制组，此分支多为防御性路径），建议改为数值范围校验或以十六进制写回，保证校验语义与分类规则一致。

-       groups.splice(groups.length - 1, 1, `${(octets[0]! << 8) | octets[1]!}`, `${(octets[2]! << 8) | octets[3]!}`);
+       const firstWord = (octets[0]! << 8) | octets[1]!;
+       const secondWord = (octets[2]! << 8) | octets[3]!;
+       if (firstWord > 0xffff || secondWord > 0xffff) return undefined;
+       groups.splice(groups.length - 1, 1, firstWord.toString(16), secondWord.toString(16));


─── apps/mobile/src/adapters/vault-adapters.ts:21-21 ───
[bug · high] SecureStore 键字符集违规：`scopeKeyOf` 产生的 scope key（如
`weknora.vault.v1.https%3A%2F%2Fcloud.weknora.io...`）包含 `encodeURIComponent` 产出的 `%`（以及 `!~*'()`
等不被转义的字符），而 expo-secure-store 在 Android 上强制校验 key 只允许字母数字与 `.`、`-`、`_`（"Keys must contain only
alphanumeric characters, ., -, and _"，set/get/delete 均抛异常）。composition.ts 的
createNativeScopedVaultIfAvailable 把 expo-secure-store 直接接到本适配器，构造期 try/catch 拦不住运行期调用：任何真实部署
origin（都含 `://`）在 Android 上首次 writeWrappedKey 即抛错，原生 Vault 路径整体不可用。createSecureVaultStorage 的行键同样携带
`%`，问题相同。建议在适配器内先把 key 归一化到合法字符集（如对不允许字符做 `_xx.` 十六进制转义，或摘要后十六进制化），再传入 SecureStore。

-     async writeWrappedKey(scopeKey, key) { await store.setItemAsync(scopeKey, bytesToBase64(key)); },
+ /** expo-secure-store（Android）key 仅允许字母数字与 . - _；encodeURIComponent 产物中的 '%'、'~' 等需先映射（'_' 也转义以保证无歧义）。 */
+ function toSecureStoreKey(key: string): string {
+   let out = '';
+   for (const ch of key) out += /[A-Za-z0-9.-]/.test(ch) ? ch : `_${ch.codePointAt(0)!.toString(16)}.`;
+   return out;
+ }


─── packages/mobile-core/src/vault/scoped-vault.ts:134-139 ───
[bug · medium] id 校验不对称：`put` 强制 DRAFT_ID_PATTERN，但 `get`/`remove` 完全不校验，任意 id
直接拼进存储键透传到底层存储。具体危害：`remove('index')` 的 rowKey 与 indexKey 完全相同，会直接删除明文索引行——全部草稿行立即孤儿化（list
永远返回空、且这些行再也无法经仓储 remove 清理，只能等 revoke），是静默数据丢失路径；`get('index')` 则会对明文 JSON 索引走解密路径抛
VAULT_DECRYPT，造成错误语义混乱；含控制字符/超长的 id 也会原样打到存储层。建议在 get/remove 入口同样执行 DRAFT_ID_PATTERN 校验。

          async get(id) {
            assertAccessible(scopeLease, session);
+           if (!DRAFT_ID_PATTERN.test(id)) throw new Error('VAULT_ID');
            const raw = await ports.storage.read(rowKey(scopeKey, id));
            if (raw === null) return undefined;
            return openRow(session.key, raw);
          },
+         // remove(id) 同样在入口加 if (!DRAFT_ID_PATTERN.test(id)) throw new Error('VAULT_ID');


─── packages/mobile-core/src/vault/scoped-vault.ts:68-71 ───
[bug · medium] requireSession 无并发防护（single-flight 缺失）：同一 scope 首次并发调用两次 open() 时，两者都在缓存未命中后 `await
readWrappedKey`，各自生成不同密钥并先后 writeWrappedKey/sessions.set。keyStore 最终只保留一个 key（如 K2），但先返回的
ScopedStore 闭包持有另一个 session（K1）：用它 put 写入的行全部以 K1 加密，而 K1 从未持久化——进程重启后这些行永久
VAULT_DECRYPT，草稿静默丢失。这与此前确认的 rotate 窗口竞态是不同入口（open 引导阶段），建议对 requireSession 做在途 Promise 缓存（同 scope
复用同一次引导），并与 rotate/put 的串行化队列配合。

-   const requireSession = async (scopeKey: string): Promise<ScopeSession> => {
+   const inflight = new Map<string, Promise<ScopeSession>>();
+   const requireSession = (scopeKey: string): Promise<ScopeSession> => {
      const existing = sessions.get(scopeKey);
-     if (existing && !existing.destroyed) return existing;
-     const wrapped = await ports.keyStore.readWrappedKey(scopeKey);
+     if (existing && !existing.destroyed) return Promise.resolve(existing);
+     const pending = inflight.get(scopeKey);
+     if (pending) return pending;
+     const created = bootstrapSession(scopeKey).finally(() => inflight.delete(scopeKey));
+     inflight.set(scopeKey, created);
+     return created;
+   };
+   // bootstrapSession(scopeKey) 封装现有 readWrappedKey/生成新 key/writeWrappedKey 逻辑


─── packages/mobile-core/src/vault/scoped-vault.ts:92-95 ───
[security · low] 密文未与行标识绑定：sealRow/openRow 的 AES-GCM 未使用 AAD，且 get(id) 不校验 `entry.id === id`、list()
不校验 entry.id 与索引 id 一致。同一 scope 内对底层存储有写权限的攻击者可以把 A 行的密文复制到 B 行键下（cut-and-paste），解密照常通过，get("b")
会静默返回 id 为 "a" 的草稿内容——GCM 只保证密文完整性，不保证行归属。最低成本修复：get/list 解密后校验 entry.id 与请求 id（不一致抛
VAULT_DECRYPT）；更强的做法是扩展 CipherPort 支持 additionalData，以 rowKey 作为 AAD 参与认证。

-   const sealRow = async (key: Uint8Array, entry: DraftEntry): Promise<string> => {
-     const sealed = await ports.cipher.seal(key, text.encode(JSON.stringify(entry)));
-     return bytesToBase64(sealed);
-   };
+ // get(id) 中：
+ const entry = await openRow(session.key, raw);
+ if (entry.id !== id) throw new Error('VAULT_DECRYPT');
+ return entry;
+ // 或扩展 CipherPort：seal(key, plaintext, additionalData = rowKey) 作为 AES-GCM additionalData


─── packages/mobile-core/src/task-office/task-detail.ts:245-248 ───
[bug · medium] processEvent 在 `await persist` 挂起点之后直接回写 events/committedCursor/autoResyncs，缺少 epoch
复验：resync() 是外部调用（不在 chain 上），可在 persist 挂起期间完整执行一次 hydrate（abortStream 递增 streamEpoch、按权威 watermark
重置 events/committedCursor）；persist 恢复后这些陈旧赋值会把内存态游标回退到旧 seq、用旧事件集覆写新事件集，并错误地重置 autoResyncs，随后新流事件因
seq 错位触发 gap→interrupt→hydrate 的连锁抖动，视图时间线短暂回滚。hydrate 对同类挂起危害已有显式守卫（“persist
挂起点期间被取代”分支），此处应同构处理：函数入口捕获 epoch，await persist 返回后若 epoch 已变则丢弃本次推进。

+   const processEvent = async (event: TaskBackendEvent): Promise<void> => {
+     const epoch = streamEpoch;
+     if (closed || detail === undefined) return;
+     // ...
+     try {
+       await persist(event.seq, nextEvents);
+     } catch (cause) {
+       await interrupt('persist-failed', cause instanceof Error ? cause.message : String(cause));
+       return;
+     }
+     if (closed || epoch !== streamEpoch) return; // persist 挂起期间被 resync/interrupt 取代：丢弃陈旧推进
      events = nextEvents;
      committedCursor = event.seq;
      autoResyncs = 0;
      notify('live');


─── packages/mobile-core/src/task-office/task-detail.ts:219-220 ───
[bug · medium] 流式路径（processEvent/processControl/streamEnded → notify/persist）完全不校验 lease：lease
撤销（切租户/换部署/退出）后，SSE 事件仍会继续合并进内存态、写入投影 store 并通知订阅者，直到调用方显式 close()。模块自述契约是“scope lease 撤销后的一切结果按
TASK_OFFICE_SCOPE_CHANGED 拒绝”，且测试 406（“a revoked lease after hydration stops
notifications”）的断言目前只因撤销后没有事件投放才偶然成立。当前生产组合根靠 runtime authorizedEventStream 的 guardedChunk 在传输层切断跨
scope 分片兜底，但模块级（in-memory/scenario 后端、其它 runtime 装配）不成立。建议在 processEvent 推进/持久化前校验 leaseActive（或由
office 在 lease 撤销时主动 close 打开的句柄）。

    const processEvent = async (event: TaskBackendEvent): Promise<void> => {
-     if (closed || detail === undefined) return;
+     if (closed || detail === undefined || !leaseActive(ports.lease())) return; // 撤销后停止合并/持久化/通知


─── packages/mobile-core/src/task-office/task-detail.ts:252-256 ───
[maintainability · low] 除 cursor_expired 外的一切控制帧都被按致命 stream-error 处理（interrupt + 有界自动重连）。但服务端 v2
字节合同的注释与测试（workbench_read.go writeWorkbenchControlSSE “clients classify them separately from
business events”；workbench_stream_mx004_test.go 已内插 heartbeat_ack 良性控制帧）表明控制帧会承载非致命的 keepalive/未知
code。一旦服务端启用这类帧，每帧都会触发 interrupt 并烧掉 AUTO_RESYNC_LIMIT 预算，详情句柄将永久停在 interrupted。建议按已知致命码白名单（如
stream_error）分类，未知 code 仅观测不打断流。

+ const FATAL_CONTROL_CODES: ReadonlySet<string> = new Set(['stream_error']);
+ // ...
      if (frame.code === 'cursor_expired') {
        await interrupt('cursor-expired', frame.message);
        return;
      }
+     if (!FATAL_CONTROL_CODES.has(frame.code)) return; // 心跳/未知控制帧：按字节合同仅分类观测，不打断流
      await interrupt('stream-error', `${frame.code}: ${frame.message}`);


─── packages/mobile-core/src/runtime/mobile-runtime.ts:430-431 ───
[bug · medium] 同 origin 短路的表面集合过宽，会封锁失败后的重试路径。场景：已授权于 A 的用户切换到已登记的 B，`begin(B)` 已撤销 A 的 scope；随后
`authenticate` 因瞬时故障失败（`me`/`deploymentCapabilities` 网络错误，或 `credentialStore.read` 抛错）→ `safe()` 发布
`{ surface: 'upgrade-required', deployment: B }`。此时用户再点 B：`state.surface !== 'deployment-login'`（是
upgrade-required）且 `state.deployment.origin === B` 成立 → 直接 `return state`，重试被静默短路。`activateTenant`
失败同理（deployment 未变，同样落入 upgrade-required）。而 upgrade-required 在 UI 上是受限说明面（UpgradeRequiredScreen
无授权控件），会话内唯一恢复路径是 signOut（会清除该实例已存凭据，破坏性）或重启 App。建议参照 B2-F44 的意图（不打断「已验证的活跃会话」），把短路收窄到
authorized/read-only 两个已验证面，让失败面保留重试能力。

-       // 同 origin 短路（B2-F44）：目标即当前活动实例且不在登录面时不重走 begin/authenticate。
-       if (state.surface !== 'deployment-login' && state.deployment?.origin === target.origin) return state;
+       // 同 origin 短路（B2-F44）：仅对已验证会话面短路；upgrade-required 是失败面，须保留重试入口。
+       if ((state.surface === 'authorized' || state.surface === 'read-only') && state.deployment?.origin === target.origin) return state;


─── packages/mobile-core/src/runtime/mobile-runtime.ts:457-462 ───
[bug · low] 两步清理存在顺序依赖：前一步失败会整体跳过 `registry.remove`。`signOut()` 内部是
`Promise.all`，任一清理（credentialStore / pendingOidcStore / deploymentStore）reject 都会让 `await signOut()`
抛出；非活动实例分支的 `credentialStore.clear` 同理。控制流随即跳入外层 catch，`registry.remove`
永不执行——用户明确要求「移除」的实例仍留在登记列表（listDeployments 继续返回它，switchDeployment 仍会尝试恢复其凭据）。两步彼此独立（凭据清理 vs
登记移除），应各自包含失败，与 `authenticate` 中对 registry.upsert 的单独包含（B2-F32）保持同一纪律。

-         if (activeDeployment?.origin === deployment.origin) {
-           await signOut();
-         } else {
-           await mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); });
-         }
+         try {
+           if (activeDeployment?.origin === deployment.origin) await signOut();
+           else await mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); });
+         } catch { /* 凭据清理失败不阻塞登记移除（B2-F43 包含语义） */ }
          await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); });


─── apps/mobile/src/composition.ts:140-141 ───
[bug · high] HomeScreen 的 remount key 仅使用 activeTenantId，未包含 deployment origin。switchDeployment
是原子切换（authorized→authorized，不经过中间 surface），当两个部署的租户 id 相同（自增整数 id 下非常常见，如都是 "1"）时 key 不变，HomeScreen
实例被复用：`useEffect(load, [])` 不会重跑，屏幕继续展示旧部署 A 的 home 数据（跨部署数据串显，违反"无跨租户/跨部署泄露"约束）。更严重的是 load 闭包捕获的是
mount 时的 taskOffice（A 的实例），切换后用户点 "Load home"/重试会持续命中已撤销 lease 的 TASK_OFFICE_SCOPE_CHANGED，无法看到部署 B
的数据。建议 key 融合完整 scope 身份（origin+tenantId）。

      return createElement(HomeScreen, {
-       key: snapshot.identity.activeTenantId,
+       key: `${snapshot.deployment.origin}|${snapshot.identity.activeTenantId}`,


─── apps/mobile/src/composition.ts:122-123 ───
[bug · high] 与 RuntimeSurface 中的 HomeScreen 同一缺陷：TasksScreen 的 key 仅含 activeTenantId。跨部署切换且两部署
activeTenantId 相同时组件不 remount，`useEffect(reload, [])` 不重跑，items/duplicateRunIds/hasMore 以及
search/status/archived 表单状态全部残留旧部署的数据；后续 loadMore/archive 会以新 taskOffice 打旧 UI
状态（旧部署列表行归档到新部署），行为错乱。key 应包含 origin 以保证 scope 变化必然 remount。

    return createElement(TasksScreen, {
-     key: snapshot.identity.activeTenantId,
+     key: `${snapshot.deployment.origin}|${snapshot.identity.activeTenantId}`,


─── apps/mobile/src/adapters/sse-stream.ts:21-22 ───
[performance · medium] 非 2xx 且响应带 body 时（401/403/409 等错误响应通常携带 JSON body）直接 throw，未消费也未 cancel
`response.body`，底层连接会保持占用直到 GC。Runtime 的 401 刷新重试路径（sendWithCredential 捕获 401 后换 token
重试）会反复制造这类失败响应，移动端连接池与电量会受影响。建议 throw 前显式取消 body。

+     void response.body?.cancel().catch(() => undefined);
      throw new ApiError({ status: response.status, code: `HTTP_${response.status}`, message: `workbench event stream failed with HTTP ${response.status}` });
    }


LLM retry report summary: 25 of 230 requests affected -- 8 requests failed, 17 requests recovered after retry

Review planning (5 requests):
- apps/mobile/src/adapters/deployment-registry.ts,apps/mobile/src/adapters/sse-stream.ts,apps/mobile/src/composition.ts,apps/mobile/src/runtime-integration-smoke.ts,apps/mobile/src/screens/DeploymentLoginScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks.tsx,apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx,apps/mobile/src/screens/TasksScreen.tsx,apps/mobile/src/task-detail-integration-smoke.ts,apps/mobile/src/task-detail-view.ts,apps/mobile/src/task-office-integration-smoke.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/mobile-core/src/index.ts,packages/mobile-core/src/runtime/in-memory-adapters.ts,packages/mobile-core/src/runtime/mobile-runtime.ts,packages/mobile-core/src/runtime/ports.ts,packages/mobile-core/src/runtime/scope-lease.ts,packages/mobile-core/src/runtime/types.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/resources.tsx,apps/mobile/src/resources-view.ts,apps/mobile/src/screens/HomeScreen.tsx,apps/mobile/src/screens/ReadOnlyScreen.tsx,apps/mobile/src/screens/ResourcesScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- packages/domain/src/mobile/resource-presentation.ts,packages/mobile-core/src/shelf/in-memory-resource-remote.ts,packages/mobile-core/src/shelf/ports.ts,packages/mobile-core/src/shelf/resource-shelf.ts,packages/mobile-core/src/shelf/types.ts: rate limited (HTTP 429) -> succeeded

Core review (20 requests):
- .gitignore,apps/mobile/app.json,apps/mobile/package.json,apps/mobile/plugins/ios-xcode27.js: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/resources.tsx,apps/mobile/src/resources-view.ts,apps/mobile/src/screens/HomeScreen.tsx,apps/mobile/src/screens/ReadOnlyScreen.tsx,apps/mobile/src/screens/ResourcesScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks.tsx,apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx,apps/mobile/src/screens/TasksScreen.tsx,apps/mobile/src/task-detail-integration-smoke.ts,apps/mobile/src/task-detail-view.ts,apps/mobile/src/task-office-integration-smoke.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/application/repository/workbench_list.go,internal/application/repository/workbench_task_facts.go,internal/application/repository/workbench_task_state.go,internal/modules/workbench/contracts.go,internal/modules/workbench/service/workbench/overview.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/api-client/package.json,packages/api-client/src/mobile/executions.ts,packages/api-client/src/mobile/resources.ts,packages/api-client/src/mobile/runtime.ts,packages/api-client/src/mobile/task-office.ts,packages/contracts/src/mobile/execution.ts,packages/contracts/src/mobile/read-models.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 15 more

Per-attempt detail: --format json (retry_report).
