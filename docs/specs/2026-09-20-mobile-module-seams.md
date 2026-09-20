# 移动端深 Module 与 seam 设计

日期：2026-09-20
状态：推荐设计，等待架构审阅；只定义 Module、Interface、seam、Adapter 与依赖方向，不授权开始实现。

相关事实源：

- [统一领域语言](../../CONTEXT.md)
- [移动 AI Office 设计规格](./2026-09-20-mobile-ai-office-design.md)
- [Agent Marketplace 领域模型](./2026-09-20-agent-marketplace-domain-model.md)
- [ADR-0005：WeKnora 原生移动客户端](../adr/0005-weknora-native-mobile-client.md)
- [ADR-0006：移动传输按语义分层](../adr/0006-mobile-transport-by-semantics.md)
- [ADR-0007：设备身份与加密缓存](../adr/0007-registered-devices-and-encrypted-cache.md)
- [ADR-0012：移动业务逻辑位于深 Module 后](../adr/0012-mobile-business-logic-lives-behind-deep-modules.md)

## 1. 当前事实

当前 checkout 没有 apps/mobile。Git 历史曾包含一套大型 Flutter/Conduit 移动树，后续整体删除；ADR-0005 已决定新客户端使用 WeKnora 领域模型重新建设，不恢复该旧树。

当前可复用实现集中在：

- packages/contracts/src/mobile：执行、交互和聚合读模型的 wire parser；
- packages/api-client/src/mobile：Workbench REST 请求和 SSE 请求构造；
- packages/domain/src/mobile：scope、兼容性、提交对账、事件缓存、任务表单、列表与呈现规则；
- apps/miniprogram：WeChat transport、文件和本地存储 Adapter，以及当前真实接线；
- Go Workbench：overview、inbox、admission、interaction、snapshot、SSE、artifact、device 与 voice seam。

现有纯函数和约束有复用价值，但 packages/domain/src/mobile 当前公开了十多个小入口。调用方需要知道：

- scope generation 何时失效；
- request_id 何时落盘、何时可重试；
- Snapshot 和 SSE cursor 如何补洞；
- capability 和 protocol window 如何组合；
- cancel ACK 与真实停止、结算如何区分；
- draft、附件、知识和 session preparation 的调用顺序。

这是一组浅 Module：Interface 接近实现复杂度。若 Screen 直接组合这些入口，删除任一小文件只会把逻辑散回多个 Screen，缺少 leverage 和 locality。

## 2. 候选切法

### 2.1 按技术层切

Auth、Network、Storage、State、Navigation、UI 分层容易建立目录，但每个业务流程仍跨越全部层。Task scope 或恢复规则变化时会同时修改多层，Interface 只是在传 DTO，深度不足。

结论：技术层可以作为 Module 内部组织方式，不能成为外部 seam。

### 2.2 按页面切

Home、Tasks、New、Resources、Me 各自拥有 loader、store 和 mutation，默认调用最简单，但会复制 Active Tenant、缓存 scope、capability、错误和重连逻辑。跨页面 Task 状态容易不一致。

结论：页面是 presentation Adapter，不是业务 Module。

### 2.3 按产品能力切

Deployment/Tenant session、Task office、Resource shelf、Task material、Voice room 和 Scoped vault 各自拥有一组高耦合不变量，并通过小 Interface 对外提供完整行为。

结论：采用此方案。它让业务变化集中，并允许 Expo Screen、Taro 小程序和测试通过同一 Interface 使用行为。

## 3. 推荐拓扑

推荐六个深 Module：

1. Mobile Runtime
2. Task Office
3. Resource Shelf
4. Task Material
5. Voice Room
6. Scoped Vault

App Shell 不是第七个业务 Module。它是 composition root 和 presentation Adapter 的集合，负责实例化 Module、连接导航、主题、国际化与原生生命周期。

依赖方向：

    apps/mobile
      -> mobile-core Interfaces
      -> mobile-core implementations
      -> domain mobile policies

    apps/mobile native adapters
      -> mobile-core ports

    api-client mobile adapters
      -> mobile-core ports
      -> contracts wire parsers

contracts 不依赖 domain、mobile-core 或 App。domain 不依赖 I/O、React、Expo 或 api-client。mobile-core 不依赖 React Native、DOM 或具体 transport。Screen 不直接导入 contracts 或 api-client。

## 4. Mobile Runtime Module

### 4.1 所有权

Mobile Runtime 唯一拥有：

- Active Deployment、用户身份和 Active Tenant；
- capability handshake 与 protocol gate；
- scope generation 和 Scope Lease；
-登录、退出、Deployment/Tenant 切换的顺序；
-注册设备与 App 前后台生命周期；
-其他 Module 的启动、失效和关闭。

它不拥有 Task、Artifact、资源目录或语音内容。

### 4.2 Interface

调用方只需要：

- boot(deploymentHint)：恢复或开始登录，返回 RuntimeSnapshot；
- activateTenant(tenantID)：原子切换 scope，返回新的 Scope Lease；
- snapshot()：读取当前身份、capability 和可用 surface；
- subscribe(listener)：观察 RuntimeSnapshot；
- signOut() / dispose()：撤销当前 scope 并关闭子 Module。

Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。

### 4.3 隐藏的实现

OIDC return state、credential refresh single-flight、capability negotiation、设备注册、迟到响应拒绝、缓存 scope 切换、push route 绑定和启动恢复全部隐藏在此 Module。

### 4.4 seam 与 Adapter

- Identity Backend Port：WeKnora REST Adapter、in-memory Adapter；
- Credential Vault Port：Expo SecureStore Adapter、in-memory Adapter；
- Device Port：iOS/Android registration Adapter、test Adapter；
- App Lifecycle Port：Expo lifecycle Adapter、deterministic test Adapter。

## 5. Task Office Module

### 5.1 所有权

Task Office 是移动产品的主 Module，唯一拥有：

- Home、Task list、Attention inbox 的一致读投影；
-一个初始目标创建 Task，追问继续原 Task；
- request_id 持久化、提交、unknown reconciliation；
- Task / Run / Attention 三层状态；
- Snapshot 水合、SSE 顺序、cursor 和重连；
-运行干预、Interaction decision、预算扩展；
-单写者准入和 capability gate；
-Task share、archive 与 notification read state。

它不拥有 Artifact 内容解码、可用 Agent 目录、音频 transport 或凭据存储。

### 5.2 Interface

推荐外部 Interface 保持四个入口：

- home(query)：返回 HomeView；
- tasks(query)：返回 TaskPage；
- start(goal)：持久化意图并返回 TaskStartReceipt；
- open(taskID)：返回 TaskHandle。

TaskHandle 只暴露：

- snapshot()：返回 TaskView；
- act(TaskIntent)：执行 steer、queue-next、stop、decision、share、archive 等受控意图；
- updates(listener)：订阅规范 TaskDelta；
- close()：释放订阅。

Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。

### 5.3 不变量

- 网络前先持久化 request_id 与输入摘要；
- unknown 不自动重发或换 request_id；
- Snapshot 不完整或 cursor 缺口时重新同步；
- cancel ACK 不等于停止完成或退款；
- capability 缺失和未知 schema 一律 fail closed；
- Scope Lease 失效后丢弃迟到结果；
-同一 Task 不产生第二个写 Run；
-推送只触发重新同步，不直接修改 Task 状态。

### 5.4 seam 与 Adapter

- Task Backend Port：REST/SSE Adapter、in-memory scenario Adapter；
- Task Store Port：Scoped Vault Adapter、in-memory Adapter；
- Notification Port：push hint Adapter、test Adapter；
- Clock / ID Port：native Adapter、deterministic Adapter。

现有 submission.ts、execution-cache.ts、session-list.ts、task-form.ts、execution-presentation.ts 和 compatibility.ts 应成为 Module 内部实现或纯策略，不再各自成为 Screen 可见 Interface。

## 6. Resource Shelf Module

### 6.1 所有权

Resource Shelf 统一呈现和选择当前 Tenant 的：

- Available Agent；
-可发现知识资源；
-成员可使用的 Connection 能力摘要；
-附件准备状态；
-Task 创建所需的资源兼容性和不可用原因。

Marketplace Listing、Agent Adoption 和 Agent Mapping 属于 Web 治理，不进入移动 Interface。移动端只读取 Available Agent。

### 6.2 Interface

- browse(ResourceQuery)：返回 ResourcePage；
- selection(ResourceSelection)：返回 SelectionVerdict；
- prepare(TaskResourceDraft)：上传/扫描附件并返回 PreparedTaskContext；
- subscribe(listener)：观察授权撤销和准备状态。

Interface 返回领域状态，不返回原始 Agent/KB/Connection DTO。SelectionVerdict 明确 allowed、unavailable 或 forbidden 及原因；没有能力事实时不猜测。

### 6.3 seam 与 Adapter

- Resource Backend Port：WeKnora Adapter、in-memory Adapter；
- Upload Port：native multipart Adapter、test Adapter；
- Local Favorites Port：Scoped Vault Adapter。

现有 agent-options.ts、resource-presentation.ts 和 target-options.ts 收入此 Module，不再让 Screen 拼接授权目标。

## 7. Task Material Module

### 7.1 所有权

Task Material 统一处理 Task 结果与证据：

- Evidence Citation 和来源；
-不可变 Task Artifact 版本；
-文件树、Diff、测试报告和只读 Terminal；
-预览能力、signed grant、下载、系统分享和批注；
-请求 Agent 基于确定 Artifact 版本生成新版本。

它不拥有 Task 生命周期或外部发布 Action Plan；这些意图由 Task Office 执行。

### 7.2 Interface

- list(taskID)：返回 MaterialIndex；
- open(materialRef)：返回可呈现的 MaterialView 或 ExternalOpenIntent；
- act(MaterialIntent)：处理下载、分享、批注和基于版本请求修改；
- subscribe(listener)：观察新版本与 grant 失效。

MIME、大小限制、signed URL、Diff parser、终端日志分页和 native share 都隐藏在 Module 后。

### 7.3 seam 与 Adapter

- Material Backend Port：WeKnora artifact/files/diff/log Adapter；
- Blob Store Port：encrypted file cache Adapter、in-memory Adapter；
- Preview / Share Port：iOS/Android Adapter、test Adapter。

## 8. Voice Room Module

### 8.1 所有权

Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线；确认后的文字通过 TaskHandle.act 提交。

### 8.2 Interface

- join(taskID, scopeLease)：返回 VoiceHandle；
- VoiceHandle.state() / subscribe(listener)；
- VoiceHandle.confirmTranscript(text)：产生 TaskIntent，不直接发送高风险动作；
- VoiceHandle.leave()。

### 8.3 seam 与 Adapter

- Realtime Voice Port：WebRTC Adapter、WebSocket Adapter、scripted test Adapter；
- Audio Device Port：iOS/Android native Adapter、test Adapter；
- Audio Retention Port：Scoped Vault Adapter。

WebRTC 和 WebSocket 是两个真实 Adapter，因此 seam 有实际价值。原始音频默认在处理后删除。

## 9. Scoped Vault Module

### 9.1 所有权

Scoped Vault 是内部支持 Module，唯一拥有：

- deployment/user/tenant scope 下的加密缓存；
- draft、submission journal、event projection、small artifact 和 preferences；
-key wrapping、retention、eviction、logout/撤权擦除；
-离线可读/可写草稿与禁止离线副作用的规则。

它不拥有 token refresh、Task 业务状态或远端事实。

### 9.2 Interface

- open(scopeLease)：返回 ScopedStore；
- rotate(scopeLease)：处理身份或策略更新；
- revoke(scopeLease, reason)：擦除或使 key 不可用；
- inspectPolicy()：返回可缓存类别和保留期。

ScopedStore 只提供按领域仓储分组的读写，不提供任意全局 key/value。任何没有有效 Scope Lease 的访问失败。

### 9.3 seam 与 Adapter

- Encrypted Database Port：Expo SQLite + encryption Adapter、in-memory Adapter；
- Key Store Port：SecureStore/Keychain/Keystore Adapter、test Adapter；
- File Cache Port：native filesystem Adapter、temporary filesystem Adapter。

## 10. App Shell 与 presentation Adapter

apps/mobile 只包含：

- composition：创建 Port Adapter，实例化六个 Module；
- navigation：Home、Tasks、New、Resources、Me 和 deep link；
- screens：把 Module View 映射为 React Native UI；
- native adapters：lifecycle、push、audio、database、key store、file、share；
- design tokens、i18n 和 accessibility 接线。

禁止：

- Screen 直接导入 packages/contracts 或 packages/api-client；
- Screen 自己维护 request_id、cursor、revision、scope generation；
-每个 Screen 建独立 query cache 或 token refresh；
-从 packages/ui 或 packages/views 引入 DOM/Radix 实现；
-把 Adapter DTO 直接作为长期 UI state。

UI primitives 可以位于 apps/mobile/ui，但只有出现第二个非移动 Adapter 后才考虑抽成共享 native UI package。

## 11. Package 落点

推荐新增 packages/mobile-core，集中六个 Module 的 Interface、实现和 Port。选择新 package 而不是继续扩张 packages/domain/src/mobile，原因是 Module 包含协调、生命周期和 side effect ordering，不是纯领域计算。

| 位置 | 允许内容 |
| --- | --- |
| packages/contracts/src/mobile | wire DTO、parser、schema version；不含产品流程 |
| packages/domain/src/mobile | 纯状态、值对象、policy、projection；不创建 I/O |
| packages/mobile-core | 深 Module Interface、实现、Port 和 Interface-level tests |
| packages/api-client/src/mobile | WeKnora remote Adapter；DTO 在此转换为 mobile-core/domain 类型 |
| apps/mobile | composition root、Screen、navigation、native Adapter |
| packages/design-tokens / i18n | 可移植 token 与文案 |

不复用 packages/ui 和 packages/views 的 DOM 实现。若暂不新增 package，可临时把 mobile-core 放到 packages/core/src/mobile，但必须先移除其对 api-client 的依赖，避免 Module 反向依赖 Adapter。

## 12. 依赖分类

| 依赖 | 分类 | 测试方式 |
| --- | --- | --- |
| domain policy / projection | in-process | 直接通过 Module Interface 测试 |
| SQLite / filesystem / secure key store | local-substitutable | 本地临时 Adapter 或 in-memory Adapter |
| WeKnora REST / SSE | remote but owned | Port + WeKnora Adapter + in-memory scenario Adapter |
| APNs / FCM | true external | Push Port + mock Adapter |
| WebRTC / OS audio / system share | true external | Port + native Adapter + scripted Adapter |

Port 放在拥有行为的 Module 一侧。Adapter 不拥有重试、scope、幂等或业务状态；这些属于深 Module。

## 13. Interface 测试面

测试只从六个 Module Interface 观察行为：

- Mobile Runtime：登录恢复、capability 降级、切 scope、迟到响应、撤销；
- Task Office：提交 ACK 丢失、lookup unknown、SSE 缺口、stop pending、决策 CAS、单写者；
- Resource Shelf：权限撤销、附件扫描、Agent 不可用、SelectionVerdict；
- Task Material：版本固定、grant 过期、下载/分享、Diff 与只读 Terminal；
- Voice Room：permission、断线、转写确认、原音频删除、高风险审批拒绝；
- Scoped Vault：scope 隔离、key 失效、retention、离线草稿和禁止离线副作用。

现有小函数测试在对应 Interface-level tests 建立后删除或降为少量纯 policy tests。不要在新 Interface 测试外再叠一层 Screen 对同一内部行为的白盒测试。

## 14. 迁移顺序

1. 冻结当前 contracts 与 Go Workbench wire 证据；
2. 创建 mobile-core Interface 和 in-memory Adapter，以既有 domain 测试行为建立 Interface tests；
3. 将 scope、compatibility、submission、cache 和 list controller 收入 Mobile Runtime / Task Office；
4. 在 api-client 增加 remote Adapter，不新增第二套 HTTP client；
5. 用 Taro Adapter 跑同一 Task Office scenario，证明 Interface 不依赖 Expo；
6. 创建最小 apps/mobile composition root 与 Runtime/Task smoke；
7. 增加 Resource Shelf、Task Material、Voice Room 和 native Adapter；
8. 达到原生验收门槛后再决定小程序维护和旧 shallow exports 删除。

迁移采用 replace-don't-layer：当 Screen 全部经过新 Interface 且 scenario tests 覆盖不变量后，删除旧公开 controller，不保留新旧两套编排器。

## 15. Deletion test

- 删除 Mobile Runtime：scope、capability、device 和生命周期复杂度会散回所有 Module，因此它有深度；
- 删除 Task Office：提交、恢复、SSE、状态和审批复杂度会散回五个页面，因此它有深度；
- 删除 Resource Shelf：Agent/知识/Connection 的权限与兼容性会散回 New 和 Resources，因此它有深度；
- 删除 Task Material：signed grant、MIME、预览和 native share 会散回 Task 详情各标签，因此它有深度；
- 删除 Voice Room：实时 transport 与 OS audio 会污染 Task Office，因此它有深度；
- 删除 Scoped Vault：scope key、加密、retention 和离线规则会散回所有持久化调用，因此它有深度。

相反，HomeModule、TasksModule、NewTaskModule、SettingsModule 只会把现有实现按页面搬家，删除后复杂度不会增加，不应建立。

## 16. 尚待实现前验证

- 当前 Go overview 仍以 owner 过滤，而已确认的 Task Collaborator/Viewer 需要共享读投影；
-当前 contracts 的 RunStatus 与新 Task/Run/Attention 三层模型尚未完全对齐；
-当前 StartInput 固定七字段，附件/知识通过 session preparation 的顺序尚需后端冻结；
-当前 api-client mobile exports 未在 package exports 中形成独立 seam；
-当前 packages/core 依赖 api-client，不适合作为 inverted mobile-core；
-当前没有 apps/mobile，历史 Flutter 树已经删除；Expo/React Native package、native storage 和 voice Adapter 均是新增工作；
-packages/ui 和 packages/views 是 DOM 实现，不能作为原生 UI 依赖；
-Agent Marketplace 移动端只需要 Available Agent read model，后端尚需提供符合新领域模型的投影。

这些验证可能改变 Adapter 或内部实现，但不得把 scope、幂等、恢复、权限和版本不变量重新推给 Screen。
