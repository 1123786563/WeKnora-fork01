# WeKnora 微信小程序｜详细设计合并版

版本1.0 · 2026-09-17。原型与任务设计交付，不是生产实现完成证明。

入口及源码边界见README。


---

# 01｜Taro 微信小程序前端详细设计

版本：1.0 · 2026-09-17。设计基线沿用提交 `9cc91c31ef7853fb50073f92edd92807817c9259` 的上版选读报告，不表示本次重新取得最新全仓源码。E=上版确认存在，A=适配，N=本方案新增，V=实机/部署核验。

## 1. 目标与不可变边界

新增 `apps/miniprogram`，作为同一WeKnora平台的微信入口；保留现有React Web、Expo Mobile、Go服务及运行引擎。复用共享契约、API客户端、平台无关domain规则，不复制账号、租户、钱包、计费或Agent Loop。复杂模型配置、MCP安装、Agent编排、沙箱终端、完整编辑器与管理后台留在Web。

信息架构固定四Tab：工作台、任务、知识、我的。20页面映射见04；高保真页面是视觉和交互规格，不是可直接提交审核的Taro生产代码。

## 2. 工程组织与责任

```text
apps/miniprogram/
  config/index.ts                    # 独立兼容版本、designWidth=750
  src/app.tsx app.config.ts app.scss # 生命周期与分包注册
  src/platform/                     # 唯一允许直接调用Taro平台API的基础层
    http.ts stream.ts utf8.ts files.ts
    credentials.ts refresh.ts scope.ts storage.ts
    lifecycle.ts metrics.ts navigation.ts deep-link.ts
    payment.ts notifications.ts
  src/components/
    primitives/ layout/ chat/ execution/ knowledge/ commercial/
  src/features/                     # 端用例与ViewModel，不复制后端业务规则
    auth/ agents/ chat/ execution/ knowledge/ commercial/ invitations/
  src/pages/home/ tasks/ knowledge/ me/
  src/subpackages/auth/ agents/ chat/ execution/ documents/ account/ system/
  src/custom-tab-bar/
  src/styles/_tokens.scss
  tests/platform/ tasks/ chat/ execution/ knowledge/ commercial/ visual/ device/
```

依赖方向：页面→feature/controller→共享client/domain→port→platform。页面只接受ViewModel和动作函数；domain不得反向依赖Taro；UI组件不直接发HTTP。临时demo数据限制在原型和测试fixture中，不进入生产页面。

主包只放四Tab及首屏必须的代码。聊天Markdown、文档、安全预览、付款与任务详情按业务分包。深路径导出明确，避免导入整个编辑器、语言包、图标集或Web/RN组件。版本采用官方Taro React模板验证过的组合，所有@tarojs/*锁同一补丁；不要为了小程序整体降级其他端。

## 3. 路由与进入流程

路由统一使用内部页面键，platform/navigation映射为Taro路径。主Tab走switchTab，业务子页走navigateTo，登录完成按合法目标redirect；不得把任意用户URL传给navigateTo/WebView。

`AppLaunch → 读取非敏感部署配置 → 恢复凭证 → auth/me → 选择合法空间 → 检查目标能力 → 打开目标`。

无token时进入登录；无member关系时显示邀请/创建入口，不能强调租户必需API。存在待处理深链时只保存白名单业务类型与资源ID，登录后重新鉴权。分享/通知链接不是授权凭证。

返回规则：详情优先返回发起页；孤立深链从详情返回对应Tab。切空间必须清除旧业务导航上下文。登录失效保留合法、无机密的返回意图，不自动重新提交旧表单。

## 4. ViewModel与数据责任

服务端数据、界面状态、持久恢复数据分开：

| 层 | 内容 | 唯一真值/约束 |
|---|---|---|
| ServerState | session、执行、知识状态、用量订单、成员权限 | Go API；缓存不能授权 |
| UIState | 选中的Tab/筛选、输入草稿、展开、弹窗、滚动锚点 | 当前scope下端状态 |
| RecoveryState | 未确认request_id、已提交投影/游标、schemaVersion | 已成功存储且与scope绑定 |
| CapabilityState | 部署/租户/用户/终端的可用性与原因 | 服务端声明；写入仍重新鉴权 |

建议统一读状态为 `idle | loading | ready | refreshing | empty | error`，业务错误额外携带 `reasonCode/retryable/requestId/asOf`。不让error被空数组掩盖；刷新失败可以保留旧数据，但显著说明时点。

页面投影必须明确单位、状态和来源。TaskCardView来自新的列表读模型，不假装旧ExecutionDTO已有title/updated_at；预算显示来自授权启动选项，不在前端猜测credits/token换算。

## 5. 平台端口详细约定

### 5.1 普通请求

`send`使用Taro.request；对象JSON与字符串body保持区分，响应header统一小写，保留真实statusCode。ApiError映射网络、超时、401、403、404、409、429与服务不可用，不打印请求凭证或用户正文。

取消机制将AbortSignal桥接到RequestTask.abort并解除监听。signal是本地请求生命周期，不是业务取消命令。只对明确幂等读取进行有界重试；聊天POST、发起任务、审批、支付下单都不能无条件重放。

### 5.2 流式传输

Taro.request启用chunked；ArrayBuffer→增量UTF8→SSE分帧→对应聊天或执行协议验证→ViewModel。必须在字节解码层保留半个UTF8字符，在SSE层保留半个CRLF；不能逐chunk调用无状态替换。

测试样本覆盖：中文/emoji逐字节、多事件粘包、多行data、注释心跳、空id、CRLF任意拆分、EOF不完整帧、异常Content-Type、401和取消。心跳不推进业务seq，不显示为回答；未知schema不能当普通JSON继续处理。

所选微信基础库是否在流未结束时提供可信HTTP状态，是MP00门禁。若无法满足既有HttpStreamResult：新增可选、显式版本的stream扩展port，区分“头已到/状态未确认”和“状态已确认”；旧Web/Expo端口不改为假200。只允许经过协议校验的临时显示，不能在未确认状态时声称鉴权/提交成功。无法安全实现的执行流降级为获授权快照轮询；普通聊天不得通过重复POST冒充恢复。

缓冲和渲染使用有界队列。推荐初始工程预算：文本刷新不超过每50ms一次、单SSE帧最大1MiB、未消费队列最大2MiB；这是项目初始保护值，不是微信平台限额，MP00/真机测试后按真实事件分布调整。触限必须中止并给可恢复原因，不能静默截断JSON。

### 5.3 文件与二进制

先判断NativeFileSource和sendMultipartFile，再构造浏览器FormData。Taro.uploadFile使用平台multipart封装，固定字段名与formData，不自造boundary。禁止把Base64大文件常驻页面状态。

临时聊天附件归属session；知识导入归属指定knowledge base；不共享“上传成功”状态机。大文件下载先到微信临时路径，复核授权和类型再openDocument/previewImage。HTML/Craft仅受限源码摘要或服务端静态图，不在小程序执行。

## 6. 凭证、刷新与作用域

CredentialAdapter异步封装唯一入口。小程序存储不能被宣传为硬件安全保管；采用短期access token、可撤销刷新、退出清理、日志脱敏和机密最小化。

刷新按origin+user+tenant+generation单飞。相同scope并发401只调用一次refresh；等待者使用相同结果；新scope安装后旧刷新结果必须丢弃。首帧已输出的POST不得在刷新后自动重发。

切换顺序：

```text
freeze writes
→ abort old requests / subscriptions
→ 使用原合法身份请求switch-tenant
→ 成功：原子安装token+scope，generation递增
→ 清除/隔离旧缓存、草稿、导航、临时文件
→ 读取新空间权限与数据
失败：保留原空间与凭证，解除冻结；不展示目标空间伪数据
```

每个异步任务捕获发起时scope，在提交UI/store前再比对generation。不能只取消网络而遗漏定时器、上传进度、Promise回调和刷新回包。

## 7. 本地存储与恢复记录

统一键前缀 `mini:v1:{originHash}:{userId}:{tenantId}:{kind}:{resourceId}`；generation不作为永久资源身份，但用来拒绝过时异步写。不得只按session_id/run_id建缓存键。

记录包含 `schemaVersion, scope, savedAt, expiresAt, payload`。初始项目策略：最近10个执行快照、普通列表缓存TTL24h；草稿最多7天；退出、切账号、撤权立即清理。数量/期限是可配置产品策略，不是微信存储额度承诺。不得把临时文件、签名下载URL、session_key、完整敏感工具参数放入持久记录。

最小意图记录在POST前成功写入：request_id、scope、session_id、agent_id、创建时间和本地phase；不必持久化完整敏感文本。存储失败须提示不可恢复并阻止声称“已可靠提交”。安全需求禁止保存正文时，未知请求只能按原request_id查询，不靠本地重构新请求。

快照与游标可作为一条不可分割记录写入；不能先写游标再写投影。平台存储若不能保证跨key事务，就不拆为互相独立的key。写失败保留最后有效记录并从后端快照重建。

## 8. 普通聊天详细流程

选择授权session或新建→加载历史分页→用户明确发送→绑定临时附件→调用既有knowledge-chat/agent-chat→增量渲染→完成后完整受限格式化。工具调用只显示用户可理解的动作摘要，不显示私有推理、系统提示或秘密参数。

输入保持焦点和草稿；发送按钮禁用空白内容和并发重复发送。用户向上看历史时不强行滚到底，提供“有新消息”；只有接近底部时跟随新增内容。流途中切页/隐藏可停止本地读取，重入先读取服务端实际消息状态；旧聊天能否继续流由现有API和实测决定。

引用保存内部文档/片段标识。点击重新授权读取真实原文；缺少位置不编造页码，文档已更新则说明版本差异。错误或未完成回答保留可理解状态，不伪造“回答完毕”。

## 9. 持久执行详细流程

### 9.1 发起

`editing → validating → persisting_intent → submitting → admitted / uncertain / rejected`。

读取授权launch options，解析session/agent/target/workspace及预算单位；现有StartExecutionInput原样使用，不添加任意attachment_ids。确认后只生成一个request_id。响应丢失进入uncertain，查询原`executions/requests/:request_id`；unknown不是新建业务意图的许可。用户明确新建另一任务才生成新ID。

### 9.2 读取与恢复

```text
load snapshot(scope, run)
→ 校验schema/run/权限和watermark
→ installSnapshot原子提交投影与水位
→ events从已提交游标继续
→ seq相同/更旧：去重；seq连续：应用后提交
→ seq缺口/失效游标/未知schema：停流，重取快照
```

本地连接态 `connecting | live | offline | resyncing | polling` 不映射为后端run_status。退出小程序不取消后台任务。任务列表使用聚合读取而不是每卡一个流；当前可见执行最多一个主订阅。

取消与追加指令均带expected_revision，冲突先刷新，危险操作不能自动拿新revision重试。取消不撤销已经完成的外部副作用，界面需明示。

## 10. 审批与风险控制

P09展示当前空间、任务、动作、目标、必要脱敏参数、风险、服务端有效期/版本及状态。用户选择“仅本次同意”后弹二次确认；只使用真实interaction支持的decision类型，不虚构approve字段。

UI状态 `pending → confirming → submitting → accepted/rejected/conflict/expired`；离开确认层不提交。提交期间禁用重复；他端已处理、版本变化、授权撤销进入只读并刷新。预算扩容与外部写操作审批分离，无全局永久放行项。

## 11. 知识导入、订单与通知

导入：`idle → selected → uploading → parsing → ready/failed`；解析中允许离开。前台列表按退避轮询后端状态，后台停止轮询；返回后按knowledge_id读取，不再次上传。

付款：报价和资格检查→业务订单→获准渠道收银→查询原订单。UI付款状态与履约状态单独显示；客户端SDK成功不加余额，paid未fulfilled明确说明正在处理。报价到期重新从服务端取值，不能以前端计时器决定实际有效性。

通知仅在用户主动事件后请求订阅；拒绝不阻断任务。深链返回后重新验证账号、空间和资源。后端通知使用权威事件和可靠投递，不由页面进度假造“任务完成”。

## 12. 测试与交付出口

适配层单测（UTF8/CRLF、HTTP状态、FormData分流、刷新与scope）；domain状态测试；契约fixture；页面行为；真机网络/生命周期/键盘；真实后端越权与幂等；Web/Expo兼容；20页视觉与无障碍检查。

HTML检查只证明离线原型行为，不能证明Taro编译、微信登录支付、数据库迁移或后端安全。每项实机/业务验收需记录设备、基础库、网络、后端commit、用例、命令及实际结果。

## 技术依据

上版源码证据和固定URL索引见baseline/00-源码核查.md。Taro设计尺寸参考：https://docs.taro.zone/docs/size 。分块请求参考：https://docs.taro.zone/docs/apis/network/request/RequestTask 。本次新增API、尺寸转换策略与保护阈值为设计建议，不宣称已在原仓库实现。


---

# 02｜Go 后端、共享契约与关键数据详细设计

版本：1.0 · 2026-09-17。此文以随包baseline为事实边界；所有N端点、DTO、表结构和文件名均为拟议增量。实施前先完成MP00固定当前源码和差异。

## 1. 变更面与既有事实

| 模块 | E/A既有能力 | N拟议变更 |
|---|---|---|
| Auth/Tenant | 平台login/refresh/me/switch-tenant、成员邀请 | 微信code桥接、双侧绑定 |
| 普通Chat | session、message、knowledge/agent-chat、临时附件 | Taro适配，无新Agent循环 |
| Workbench | start、request_id对账、单run、snapshot/events、decision/command | 本人列表、本人待办、授权启动选项 |
| Knowledge | 库、文档、上传导入、引用和受控下载 | 轻量投影与端错误映射 |
| Commercial | summary/plans/usage/quotes/orders/refunds、支付/履约 | 合规小程序渠道及可用性 |
| Capabilities | 部署能力和单执行操作能力 | 完整mini/workbench/商业原因码 |
| Notifications | 上版未确认存在完整mini服务 | 授权记录、可靠投递与深链 |

不用Node BFF。新的Go handler只处理校验/鉴权/序列化，应用服务复用现有成员、订单、任务、资源服务。不得以“聚合接口”为由绕过原资源权限。

## 2. 通用接口约定

所有路径前缀 `/api/v1`。既有错误和响应封装优先复用，新增DTO放在共享contracts的mini分区。以下schema为领域负载，不自动替代原API envelope；接入时用adapter映射。

请求身份来自服务端验证过的user/tenant/membership，不接收任意owner_user_id决定查询范围。HTTP拒绝语义：401身份失效；403已知资源无权；404按当前反枚举策略隐藏不可见资源；409版本/绑定/幂等冲突；422合法结构但业务值非法；429限流；503功能或上游暂不可用。实际项目若已有错误码，保留并建立显式映射。

新增读取应支持request_id/trace_id回传但不泄露堆栈。敏感写入审计记录actor、tenant、resource、action、result、revision、trace，不记录access token、session_key、原始机密工具参数。

## 3. 微信身份桥接

### 3.1 新增端点

| N端点 | 输入 | 输出/副作用 |
|---|---|---|
| POST auth/wechat/mini/login | code、预定义客户端环境标识（非任意appid） | 已绑定：复用平台登录结果；未绑定：短期challenge |
| POST auth/wechat/mini/bind | challenge_id + 已验证平台账号登录态/一次性近期证明 | 消费challenge并绑定，再复用原登录模型 |
| 绑定解除入口 | 本轮不默认公开 | 需另审替代登录方式、近期认证及风险；不能解绑后锁死账号 |

登录的服务端顺序：限流→服务端配置选择appid→code2Session→识别provider/appid/openid→查询既有平台映射→签发原token/refresh。未绑定时默认不自动按邮箱手机号合并，也不隐式创建新的用户体系；是否允许平台注册沿用平台配置，第一版可要求已有账号绑定。

code短期且一次性，不写明文日志。客户端不能通过传openid或session_key声明身份；unionid可缺失，不作为必须条件。

### 3.2 记录结构与并发

优先扩展现有外部身份provider仓库；只有没有合适结构时才新建以下逻辑记录。字段物理类型沿用已有user主键，不擅自改成UUID或bigint。

```text
ExternalIdentity
  id, user_id
  provider = wechat_mini
  app_id, subject = openid
  union_subject?                  # optional; not automatic merge authority
  created_at, last_login_at, revoked_at?
  UNIQUE(provider, app_id, subject)

BindingChallenge
  challenge_id (unpredictable)
  provider, app_id, verified_subject
  expires_at, consumed_at?, attempt_count
  proof_context_hash
```

绑定事务：重新验证challenge有效性与平台账号证明→锁定/唯一约束写identity→标记challenge consumed→审计。冲突返回既有身份不可被覆盖的错误，不将并发失败转成成功合并。

session_key仅在获准流程确有需要时服务端受控加密短期保管，跟随微信会话更新；不与可查询的身份元信息共用明文存储，不下发小程序。

### 3.3 租户选择与邀请

登录返回原平台身份和member信息；tenantless调用原auth/me、创建/接受邀请流程。原auth/switch-tenant仍是切换入口。接受邀请必须验证受邀身份和有效状态，返回刷新后的成员关系；邀请token不直接作为资源授权。

## 4. 本人任务集合读模型

### 4.1 N接口

`GET workbench/executions?status=running&cursor=...&limit=20`

limit默认20，最大50为本项目建议；越界拒绝或规范化策略应在契约中固定。状态采用已确认RunStatus集合，空值为全部。不能由前端指定“任意用户”。团队管理全量任务属于另外的产品权限需求，不复用此入口暗中开放。

```ts
type RunStatus = 'queued'|'running'|'waiting_user'|'reconciling'|'succeeded'|'failed'|'canceled';
interface TaskCardDTO {                 // N read projection, NOT existing ExecutionDTO
  run_id: string;
  session_id: string;
  title: string;
  agent: { id: string; name: string; icon_key?: string };
  run_status: RunStatus;
  created_at: string;
  updated_at: string;
  pending_action_count: number;
}
interface TaskPage { items: TaskCardDTO[]; next_cursor: string|null; as_of: string; }
```

title与时间来自权威任务/请求资料的投影；缺标题才使用明确兜底“未命名任务”，不由客户端伪造最近更新时间。pending_action_count仅统计当前用户有权处理的交互。

### 4.2 查询一致性

首屏建立as_of；以(created_at DESC,run_id DESC)稳定keyset分页。后续cursor包含最后排序键、scope指纹、筛选指纹、首屏as_of、版本及签名/服务端绑定。篡改、跨租户复用、改变筛选、过期版本均拒绝并要求重读首页。

不要用持续变化的updated_at作为静态分页排序键。更新状态可在原卡片原位刷新；下拉刷新再取得新的列表快照。同一created_at必须有run_id作为唯一第二排序键。

查询必须在数据库/存储层过滤tenant和owner后分页，不能拉出跨租户记录再前端过滤。建议组合索引(tenant_id, owner_user_id, created_at, run_id)，是否加status索引由实际查询计划决定。可以从现有执行投影读取，不要求为了小程序建立第二份任务事实表。

## 5. 待办、能力与启动选项

### 5.1 N待办接口

`GET workbench/inbox?cursor=...&limit=20`。只返回本人当前可处理的interaction投影：interaction_id/run_id、title、action_type、脱敏target_summary、current_revision、state、created_at、expires_at（服务端有才返回）。不得直接返回所有工具入参或机密连接器配置。

进入详情再次使用既有单run interactions；提交仍调用既有decisions。列表和统计都遵循相同权限条件；他端处理后的待办应移除或标为已处理。

### 5.2 能力统一投影（N）

建议新增机器原因码与用户可理解消息：

```ts
type Availability =
 | { state:'available' }
 | { state:'forbidden'; reason_code:string; message:string }
 | { state:'unavailable'; reason_code:string; message:string };
```

因子分别为部署装配、租户开通、用户权限、终端支持、商品资格。可以并入现有system/capabilities而不是新建替代端点。不能仅根据菜单或角色名称构造权限；现有ExecutionDTO.capabilities继续决定单任务操作，写入时后端再校验。

### 5.3 N授权启动选项

`GET workbench/launch-options?agent_id=...`。只提供当前用户可用的target_id/workspace_ref组合、展示label、选项版本/时点、budget显示单位、上限/步长与确定性换算。

预算显示与现有budget_upper关系必须先审实际单位。建议用十进制字符串和明确分子/分母描述转换，确保合法步长内精确；不支持的超范围数直接拒绝，不能截断后提交。服务器POST再次校验授权和预算，不能信任过去读取的选项。

现有StartExecutionInput保持：request_id/session_id/agent_id/target_id/workspace_ref/text/budget_upper。**没有证据支持attachment_ids字段，不私自加入。**

## 6. 幂等、快照与执行事件

已有request_id对账服务继续做唯一真值。幂等作用域应以实际平台合同为准，至少隔离当前授权主体与tenant；同ID不同意图返回冲突，不覆盖历史。已接纳请求返回同run，unknown由现有对账语义处理。

snapshot返回既有投影与watermark语义，不能另造不兼容事件schema。MP03必须确认：水位覆盖哪些事件、是否支持压缩快照、游标保留期限、gap与超期错误码、run/schema是否强校验。

共享ExecutionCache增加显式installSnapshot能力：先验证snapshot属于相同scope/run和支持schema，然后原子安装投影与水位；再处理大于水位的事件。未知版本停流重同步，不能把事件JSON任意展开到UI。

服务器事件日志是只读来源，小程序不能写source-events制造“已完成”。执行完成与结算完成是两条事实；结算状态沿用已有enum与服务，本文不杜撰一套新后端枚举。

## 7. 审批、命令与资源控制

| 操作 | 使用方式 | 冲突处理 |
|---|---|---|
| 读取交互 | 既有单run interactions | 授权后返回真实schema |
| 审批决定 | 既有interactions/:id/decisions | 原decision类型、版本、幂等约束 |
| cancel / steer | 既有:run_id/commands | expected_revision；409后让用户重确认 |
| 产物下载 | 既有session/message资源接口 | 复核资源归属与授权，不只检查文件URL |

审批对象包含用户可理解的外部影响。服务端检查actor、tenant、run、interaction状态与版本，而不仅靠请求里传tenant_id。与金额相关的预算扩容不能自动授权外部写动作。

历史引用撤权、文档删除、产物过期，应按现有策略返回清晰不可访问结果；不得以小程序缓存绕过。URL导入继续由服务端SSRF防护与访问策略执行。

## 8. 商业化：重用订单而非重建钱包

summary、plans、usage、quotes、orders继续走现有商业服务。available/held/refund_locked分别显示，as_of/stale不可丢。金额字符串在边界只作确定性格式化，不用浮点做账。

QuoteInput保留plan_key/plan_version/subscription_version；下单保留quote_id/provider/idempotency_key。不允许小程序直接提交自己算出的价格。通道枚举新增要兼容旧Web Native和已有订单读取。

### 8.1 可用性和参数边界

当前已读WeChat Provider为Native code_url；它不是小程序收银参数。主体、商品、终端、协议和后台开通状态需由MP00-T02-03核验，默认购买不可用。

可以设计通道可区分的checkout envelope：kind、order_id、channel_version、对应**严格校验后的**参数。只有真实获准通道被注册后才对小程序开放；不要使用`Record<string,any>`把未知载荷强转给支付SDK。资格未知时只返回unavailable和原因，不能外跳绕过规则。

符合普通小程序支付的商品才注册JSAPI参数校验与requestPayment；适用虚拟支付的商品走其获准版本。本文不提供未经主体验证的费率、iOS通用可用性或审核保证。原参数字段/签名协议在接入时对照官方文档和渠道版本锁定，不混用两种通道验签。

### 8.2 支付事实与履约事实

```text
quote有效 + 可购买
→ 创建/找到同一业务订单
→ 获准渠道下单与签名
→ 客户端操作结果（不修改余额）
→ 服务端验签回调/主动查单确认付款
→ 既有商业服务幂等履约
→ 订单payment与fulfillment分别返回
```

回调校验签名、渠道身份、订单、金额、币种、必要appid/mchid/商品标识；签名正确也不代表金额一定匹配。支付事件记录与权益发放应沿现有账务事务/幂等机制；通知重复或乱序不能多发权益。paid但履约失败必须可重试，不要求用户再付钱。

精确公开回调路径由provider验签，不放开整个commercial。关闭购买开关时仍保留既有订单回调、查单、履约与退款处理；否则灰度/回滚可能导致已付款用户无法履约。

## 9. 订阅消息与可靠投递

授权记录关联平台user+appid+模板+授权结果/时点，微信实际可发送额度和有效性不能由本地偏好替代。用户拒绝不影响执行。

利用已有可靠队列；没有适合机制才增加outbox。事件键建议包含tenant/user/resource/event_id/template_version；投递前再次确认用户仍可访问资源。通知正文仅给脱敏任务标题和状态，不包含私人文档、工具入参或token。失败分限流、临时网络、授权失效、永久无效并分别处理；投递失败不能回滚已完成任务。

## 10. 增量数据与迁移门禁

| 数据 | 是否一定新增表 | 约束 |
|---|---|---|
| 微信身份映射 | 优先复用provider仓库 | provider+appid+subject唯一 |
| 绑定挑战 | 可用既有短期存储 | 有效期、尝试限额、一次消费 |
| 本人任务读模型 | 可直接查既有投影 | 权限先过滤、keyset与索引 |
| 通知授权/outbox | 优先复用队列与通知模型 | 去重、最小内容、权限重查 |
| 商业订单/余额 | 不新建第二份事实 | 使用原订单、账本与履约 |

迁移需先验证已有主键/枚举/索引命名并采用原迁移框架。添加索引评估锁表和查询计划；扩展字段先兼容读，再启用写，回滚保留对既有记录的解释能力。不能执行本交付中的逻辑示意来覆盖真实生产schema。

## 11. 安全验收矩阵

同用户跨租户、同租户不同用户、Owner读他人私有任务、伪造run/session/order/file、无空间登录、撤销member、绑定挑战重放、回调伪签名、正确签名错误金额、支付重复/乱序、报价过期、下载撤权、旧scope刷新回写，都必须在真实Go router与存储上验证。HTML原型中的同名状态只验证交互，不能证明以上安全属性。

## 依据与证据链

固定源码路径及R01–R24见baseline/00-源码核查.md；本方案没有重新克隆或启动该仓库。新增DTO的参考定义位于contracts/mini-read-models.ts；只有N读取/能力模型，不替换现有执行、订单或认证契约。


---

# 03｜Quiet Work 视觉风格、设计令牌与组件规范

设计版本1.0.0。品牌展示名保持WeKnora，Quiet Work是本次小程序主题名称，不是另一个产品。选择浅色首发；本轮不提供未经验证的自动深色反转。

## 1. 视觉方向

定位是可靠、温润、轻量的工作工具：米白画布、深墨正文、森林绿主操作、淡薄荷辅助面板、少量暖黄风险提示。首页聚焦“把想法，变成下一步”，任务页聚焦状态与行动，文档页提供安静阅读，商业页明确事实而非制造促销压力。

不要使用大面积紫蓝霓虹、玻璃拟态、无意义发光、随机渐变和密集徽章。渐变仅用于少量浅色介绍卡，不作正文背景。图标优先20/24逻辑像素的统一线性风格；不混用Emoji替代业务状态图标。原型SVG为本地代码生成，不需要第三方图标CDN。

所有人名、团队、价格、积分、时间、文件和统计均为虚构示例，不代表真实账号或后端能力。

## 2. 令牌源与输出

`design/tokens.json`是106个token的唯一源，采用本项目自有扁平schema；使用`$type/$value`与`{token.name}`别名，但不声明完全符合任何第三方token标准。`build-tokens.py`解析别名、拒绝循环并生成：

- `tokens.css`：用于HTML的逻辑尺寸CSS变量；
- `_tokens.taro.scss`：用于Taro750设计输入的SCSS变量；
- `tokens.ts`：供TS使用的原始逻辑尺寸与类型；
- `resolved-tokens.json`：便于审查与差异对比。

修改源后执行 `python design/build-tokens.py`，再重建原型。禁止分别手改CSS和SCSS制造三份真值。Taro环境中优先编译期SCSS变量，不将浏览器CSS变量支持情况当成所有基础库都已验证。

## 3. 核心色彩

| 语义 | 值 | 使用 |
|---|---|---|
| 页面背景 | #F5F7F2 | 全局画布 |
| 卡片背景 | #FFFFFF | 内容容器、表单 |
| 主文字 | #172B27 | 标题、正文、关键数值 |
| 次文字 | #566B63 | 辅助描述、可读元信息 |
| 第三级文字 | #61746C | 时间、说明；不可再降低透明度 |
| 主操作 | #176B52 | 确认、发起、主要链接 |
| 主操作按下 | #154E40 | active状态 |
| 深色重点卡 | #103D32 | 执行状态和用量摘要 |
| 品牌浅底 | #EDF6EE | 正向提示、选择状态 |
| 选择/图标浅底 | #DCEEE3 | 选中Tab、头像、轻卡 |
| 青柠点缀 | #D6EDA5 | 深色卡上少量非正文装饰 |
| 警告文字/底色 | #84520C / #FBF0DC | 待确认、未核验、风险 |
| 错误文字/底色 | #AF4039 / #FFF0EE | 错误与破坏性操作 |
| 处理中蓝/底色 | #315F91 / #EAF1FA | 连接/解析等信息状态 |
| 普通分隔线 | #DBE4DD | 装饰性分隔 |
| 表单交互边界 | #8EABA0 | 输入框轮廓与控件识别 |

文字使用实色配对，不随意叠加透明度。装饰分隔线不承担唯一控件识别；表单边界、焦点和状态指示需独立检验。检查普通文本对比度目标4.5:1、大文本3:1；这是WCAG参照目标，并不等于本次已做完整可访问性认证。

## 4. 尺寸、字体与栅格

逻辑基准375，HTML以CSS像素表达；Taro designWidth=750。逻辑16px→SCSS输入32px→构建换算32rpx。在375宽设备上视觉约16px，在390宽上按设计比例变化。不要将SCSS输入再乘2，也不要将真实菜单胶囊、安全区、窗口坐标再翻倍。

| 项目 | 逻辑尺寸 | 规则 |
|---|---:|---|
| 页面水平边距 | 20 | 窄屏可用16；微信实现以同一token策略为准 |
| 卡片padding | 16 | 普通卡；重点卡20/24 |
| 卡片间距 | 12–16 | 同组紧凑，分区更疏 |
| 分区间距 | 24–32 | 不依赖空标签堆高度 |
| 主按钮/输入 | 48 | 关键操作满行或明确主次 |
| 最小常用触点 | 44×44 | 小图标可有透明扩大命中区 |
| 顶部导航内容高 | 44 | 顶部状态栏和胶囊由真实度量驱动 |
| Tab内容高 | 60 | 另加安全区；不能用模拟home indicator替代 |
| 图标 | 20 / 24 | 线宽约1.5–1.8 |
| 圆角 | 6/10/14/20/24/28 | pill=999；按钮14、普通卡20、重点卡24/28 |

字体使用系统中文无衬线，不分发字体文件。标题24/32，分区18，主文14，输入15，辅助13，标签12，caption11。常规400、中等500、半粗600、粗体700；正文行高1.65，标题1.24，紧凑说明1.4。

长文本不把关键动作和状态截断。卡片标题最多两行；任务正文摘要最多三行，点击看完整。订单号与run_id允许断行，文件名保留扩展名；金额使用等宽数字或tabular-nums，但不靠人为空格对齐。

## 5. 组件合同

| 组件 | 输入 | 尺寸/状态 | 行为 |
|---|---|---|---|
| Button | tone,size,disabled,loading,onPress | primary/secondary/danger；48/44 | disabled/loading不重复触发 |
| FormField | label,value,error,hint,required | 明确label，错误在字段邻近 | 错误不只靠红边 |
| StatusBadge | state,label,icon | 小面积底色+文字 | 不用颜色独自表示成功/失败 |
| MiniNav | title,workspace,canBack | 实测胶囊避让 | 返回和切空间是不同操作 |
| WorkspaceCard | name,role,selected | 单选勾选+边界变化 | 选择后需明确进入，不后台切换 |
| AgentCard | title,description,capabilities,availability | 图标44、文案两层 | 不可用有原因，不暴露密钥 |
| TaskCard | title,agent,status,updatedAt,action | 状态标签+说明+唯一主行动 | 运行中看进度；待确认进审批 |
| Timeline | safeSteps,currentStep | 连线与步骤摘要 | 无后端百分比就不造百分比 |
| ApprovalCard | action,target,impact,revision | 暖黄提示；目标醒目 | 只同意本次，二次确认 |
| AssistantMessage | blocks,citations,streamState | 无额外大气泡，阅读优先 | 增量轻渲染，完成再格式化 |
| ChatComposer | draft,attachments,sending | 底部固定，输入与发送明确 | 键盘弹起不盖输入/内容 |
| CitationCard | documentId,label,locator | 细边框、编号与来源名 | 只跳内部授权资源 |
| UploadCard | target,file,status,progress | 选择/上传/解析/就绪各自可见 | 上传100%不表示解析完成 |
| UsageCard | available,held,refundLocked,asOf,stale | 主数值突出，分项有标签 | 不把预占当可用 |
| OrderStatus | payment,fulfillment | 两步并排/纵排均可 | 已付款未发放不显示“全部完成” |
| BottomSheet | title,body,primary,secondary | 上圆角26、安全区padding | 危险确认不提供默认自动执行 |
| EmptyState | kind,message,action,asOf | 空/错/无权/未开通/失效/恢复 | 恢复不重建业务意图 |

组件slot仅承载轻量Taro节点；复杂页面业务请求留在feature。props具体schema在MP01冻结，以共享domain投影为输入；不要把整份含秘密原始工具响应直接传给组件。

## 6. 关键交互与文案

主CTA使用动词加对象：发起任务、审阅并确认、确认导入、刷新原订单。避免“立即成功”“自动批准”“一键永久放行”等越过真实状态的承诺。

审批：先展示摘要→查看详情→仅本次同意→二次确认→提交中→服务端结果；拒绝允许说明原因。发起任务超时提示“正在确认是否已接收”，不是直接“失败，请重试创建”。

支付：通道未核验显示暂不可购买；示例开关使用虚线与“演示”前缀，不能混入生产。已付款但权益处理中使用蓝/暖黄，只有服务端确认fulfilled才使用绿色完成。

上传：本地选择→确认目标→上传进度→解析中；允许离开并返回查看。解析失败说明原因，重新解析/上传必须是服务器允许的显式动作。

## 7. 动效与可访问性

120ms按压、180ms状态切换、240ms轻量弹层。优先transform/opacity，不做整屏反复闪动；prefers-reduced-motion下关闭非必要动画。进度装饰不得模拟真实执行百分比。

HTML保留语义button、label、输入类型、可见焦点和弹窗角色；移动微信端使用可用的可访问性属性，配合真机读屏。正文不小于14作为主要阅读基准，小字号仅用于辅助元信息。原型桌面工具栏不属于小程序正式界面。

## 8. 还原验收

对照390×844的原型截图检查信息层级、留白、颜色和组件；同时验收320/375/430宽度、长用户名/任务名、无权状态、键盘、安全区和系统大字体。不能只在一个截图分辨率上靠绝对定位还原。

Taro截图与HTML可存在字体渲染和系统导航差异；应以token、组件间距和可操作区域为准，不要求把HTML模拟状态栏像素硬编码到微信。

参照资料：Taro尺寸换算 https://docs.taro.zone/docs/size ；WCAG文字对比度 https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html 。设计决策与本次测量结果分别见tokens源与tests/报告。


---

# 04｜20页详细设计与开发映射

本文件沿用上版20页信息架构。每页对应原型、Taro路由、ViewModel、组件、API、校验、状态和任务编号。

## 全局页面合同

统一遵循03的tokens与触点；主Tab显示底栏，聊天/审批使用独立底部操作区；真实胶囊与键盘尺寸由平台层提供。服务端数据、UI状态和本地恢复记录分层；所有写动作先校验当前scope、权限与capability。

| 页面 | Taro路由 | 主要任务 |
|---|---|---|
| P01 登录与账号绑定 | `subpackages/auth/pages/login/index` | MP01-T03-02 MP01-T03-03 MP01-T03-04 |
| P02 选择工作空间 | `subpackages/auth/pages/workspace/index` | MP01-T03-04 MP01-T02-06 |
| P03 工作台 | `pages/home/index` | MP01-T01-05 MP03-T01-03 |
| P04 Agent 列表 | `subpackages/agents/pages/list/index` | MP01-T01-05 MP03-T01-04 |
| P05 Agent 详情 / 发起任务 | `subpackages/agents/pages/detail/index` | MP03-T02-01 MP03-T01-04 MP01-T02-07 |
| P06 知识 / Agent 对话 | `subpackages/chat/pages/thread/index` | MP02-T01-01 MP02-T01-02 MP02-T01-03 |
| P07 任务中心 | `pages/tasks/index` | MP03-T01-02 MP03-T01-03 MP03-T02-04 |
| P08 执行详情 | `subpackages/execution/pages/detail/index` | MP03-T02-03 MP03-T02-04 MP03-T02-06 |
| P09 审批与风险确认 | `subpackages/execution/pages/approval/index` | MP03-T02-05 |
| P10 任务产物 | `subpackages/execution/pages/artifact/index` | MP03-T02-07 MP01-T02-04 |
| P11 知识库 | `pages/knowledge/index` | MP02-T02-01 |
| P12 知识库详情 | `subpackages/documents/pages/kb/index` | MP02-T02-01 MP02-T02-03 |
| P13 添加知识 | `subpackages/documents/pages/import/index` | MP02-T02-02 MP02-T02-03 MP01-T02-04 |
| P14 文档 / 引用详情 | `subpackages/documents/pages/detail/index` | MP02-T02-04 |
| P15 我的 | `pages/me/index` | MP04-T01-01 MP01-T03-05 |
| P16 用量与套餐 | `subpackages/account/pages/usage/index` | MP04-T01-01 MP04-T01-02 |
| P17 报价与支付 | `subpackages/account/pages/checkout/index` | MP04-T01-02 MP04-T01-03 MP04-T01-04 |
| P18 订单与履约结果 | `subpackages/account/pages/order/index` | MP04-T01-04 MP04-T01-05 |
| P19 空间邀请 | `subpackages/account/pages/invitations/index` | MP01-T03-05 MP04-T02-03 |
| P20 异常与能力状态 | `subpackages/system/pages/state/index` | MP01-T01-04 MP04-T03-03 |

## P01｜登录与账号绑定

**原型：**`prototype/pages/01-login.html`；**Taro路由：**`subpackages/auth/pages/login/index`。

**目标：**用微信身份或已有账号进入同一 SaaS，不暴露租户 API Key。

**布局：**上部品牌与定位；中部微信入口/账号表单；底部隐私说明与绑定入口。

**组件：**BrandIntro / FormField / ConsentRow。

**状态模型：**AuthStateMachine。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**账号非空；密码不入日志；协议需主动阅读并确认；不要求手机号。

**接口合同：**E auth/login、auth/refresh、auth/me；N auth/wechat/mini/login/bind。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**匿名→微信认证→已绑定/待绑定→平台登录态；绑定证明两侧身份。

**异常与边界：**code过期重新认证；账号冲突不自动合并；注册关闭说明原因。

**权限：**匿名可查看协议；平台 token 仅来自后端；绑定不能以昵称或用户自报手机号推断。

**任务映射：**MP01-T03-02 MP01-T03-03 MP01-T03-04。

**独立验收：**

1. 按“匿名→微信认证→已绑定/待绑定→平台登录态；绑定证明两侧身份”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“code过期重新认证；账号冲突不自动合并；注册关闭说明原因”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P02｜选择工作空间

**原型：**`prototype/pages/02-workspace.html`；**Taro路由：**`subpackages/auth/pages/workspace/index`。

**目标：**明确当前租户与用户角色，处理尚无空间的用户。

**布局：**标题说明；纵向空间卡片；选中空间说明；底部进入按钮；加入入口。

**组件：**WorkspaceCard / RadioIndicator / Notice。

**状态模型：**WorkspaceSelection。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**选项仅来自真实membership；role按服务端返回展示。

**接口合同：**E auth/me、auth/switch-tenant、tenants、me/invitations。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**单选→点击进入→冻结写入→切换成功→新scope首页。

**异常与边界：**无空间引导创建/邀请；失败保留原空间；旧回包不能覆盖新空间。

**权限：**tenant_id 不是授权凭证；服务端验证 membership；组织分享不等于切换租户。

**任务映射：**MP01-T03-04 MP01-T02-06。

**独立验收：**

1. 按“单选→点击进入→冻结写入→切换成功→新scope首页”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“无空间引导创建/邀请；失败保留原空间；旧回包不能覆盖新空间”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P03｜工作台

**原型：**`prototype/pages/03-home.html`；**Taro路由：**`pages/home/index`。

**目标：**快速使用 Agent、继续对话并处理需要关注的执行。

**布局：**顶部空间切换；意图输入；2列 Agent 捷径；待处理卡；最近活动；底部4Tab。

**组件：**IntentComposer / AgentCard / AttentionCard / RecentConversation。

**状态模型：**HomeViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**任务意图先保存在当前scope草稿；点击后到助手确认，不直接发起。

**接口合同：**E 会话/Agent/商业API；N inbox；home聚合可选，不强制新增。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**输入想法→选择助手→确认任务；待办→审批；会话→聊天。

**异常与边界：**单卡请求失败独立降级；无运行能力仍可使用已开放知识问答。

**权限：**只聚合本人可见数据；余额与权限由服务端返回，不能根据角色名猜测。

**任务映射：**MP01-T01-05 MP03-T01-03。

**独立验收：**

1. 按“输入想法→选择助手→确认任务；待办→审批；会话→聊天”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“单卡请求失败独立降级；无运行能力仍可使用已开放知识问答”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P04｜Agent 列表

**原型：**`prototype/pages/04-agents.html`；**Taro路由：**`subpackages/agents/pages/list/index`。

**目标：**选择有权限使用的已配置 Agent，而不是在手机重做配置后台。

**布局：**搜索框；分段筛选；纵向Agent卡片；卡内用途与能力标签。

**组件：**SearchField / SegmentedControl / AgentCard。

**状态模型：**AgentCatalogViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**关键词trim且长度有界；筛选与scope写入查询键。

**接口合同：**E 共享Agent配置API；N launch-options；具体既有URL由MP00固定。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**搜索/筛选→打开可用助手；不可用→说明原因。

**异常与边界：**空结果与未安装/无权/目标离线分开；不让用户填写内部target。

**权限：**后台返回可见 Agent 集；不可用不能通过输入 agent_id 绕过。

**任务映射：**MP01-T01-05 MP03-T01-04。

**独立验收：**

1. 按“搜索/筛选→打开可用助手；不可用→说明原因”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“空结果与未安装/无权/目标离线分开；不让用户填写内部target”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P05｜Agent 详情 / 发起任务

**原型：**`prototype/pages/05-agent.html`；**Taro路由：**`subpackages/agents/pages/detail/index`。

**目标：**在已配置、获授权的执行范围内提交一次任务。

**布局：**Agent信息卡；任务文本；预算与运行环境；边界说明；底部提交。

**组件：**AgentHero / TaskForm / ScopeSummary / BudgetInput。

**状态模型：**LaunchStateMachine。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**text非空；预算按服务器单位/步长/范围；target/workspace来自授权选项。

**接口合同：**E POST workbench/executions、GET executions/requests/:request_id；N launch-options。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**编辑→校验→持久化意图→提交→接纳或原request_id对账。

**异常与边界：**双击不重复；未知状态不换ID；选项失效重读并重新确认。

**权限：**target/workspace为服务端授权选择；不向现有StartExecutionInput私塞attachment_ids。

**任务映射：**MP03-T02-01 MP03-T01-04 MP01-T02-07。

**独立验收：**

1. 按“编辑→校验→持久化意图→提交→接纳或原request_id对账”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“双击不重复；未知状态不换ID；选项失效重读并重新确认”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P06｜知识 / Agent 对话

**原型：**`prototype/pages/06-chat.html`；**Taro路由：**`subpackages/chat/pages/thread/index`。

**目标：**自然语言问答，展示引用、工具摘要和临时附件。

**布局：**导航与知识范围；可滚动消息区；引用卡片；固定底部输入条，不与Tab重叠。

**组件：**AssistantMessage / CitationCard / ChatComposer / AttachmentItem。

**状态模型：**ChatViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**输入非空；附件限定当前session；Markdown转受控节点。

**接口合同：**E sessions、messages、knowledge-chat/agent-chat、attachments。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**用户发送→分片输出→完结格式化；引用→授权原文；停止→结束本地输出。

**异常与边界：**断流显示未完成；权限撤销不读缓存；不当成durable任务重启。

**权限：**仅本人会话；引用文档需再次鉴权；只展示执行摘要、不显示私有推理。

**任务映射：**MP02-T01-01 MP02-T01-02 MP02-T01-03。

**独立验收：**

1. 按“用户发送→分片输出→完结格式化；引用→授权原文；停止→结束本地输出”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“断流显示未完成；权限撤销不读缓存；不当成durable任务重启”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P07｜任务中心

**原型：**`prototype/pages/07-tasks.html`；**Taro路由：**`pages/tasks/index`。

**目标：**跨设备查看本人任务，按状态定位运行和待处理事项。

**布局：**标题与新建；状态筛选；任务卡列表；恢复说明；底部4Tab。

**组件：**TaskStats / StatusFilter / TaskCard。

**状态模型：**TaskPageViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**状态筛选采用已确认enum；cursor与filter/scope匹配。

**接口合同：**N GET workbench/executions；N inbox；E 单run详情。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**首屏→游标分页→筛选重置；待确认→审批；新建→Agent。

**异常与边界：**空/无权/未装配/离线缓存区分；不以本机记录当全量任务。

**权限：**分页和统计基于owner+tenant，不是全租户管理员任务墙。

**任务映射：**MP03-T01-02 MP03-T01-03 MP03-T02-04。

**独立验收：**

1. 按“首屏→游标分页→筛选重置；待确认→审批；新建→Agent”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“空/无权/未装配/离线缓存区分；不以本机记录当全量任务”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P08｜执行详情

**原型：**`prototype/pages/08-execution.html`；**Taro路由：**`subpackages/execution/pages/detail/index`。

**目标：**看进度、恢复状态、追加指令、取消或进入审批。

**布局：**状态大卡；任务信息；步骤时间线；待处理区域；底部操作。

**组件：**ExecutionHero / Timeline / ApprovalCard / CommandSheet。

**状态模型：**ExecutionViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**run来自授权资源；命令带expected_revision；步骤只包含安全摘要。

**接口合同：**E run/get、snapshot、events、interactions、commands。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**进入先快照后事件；断线→同run恢复；steer/cancel二次确认。

**异常与边界：**gap/过期游标/未知schema重同步；已终态不再执行危险命令。

**权限：**单run归属与capabilities检查；操作带expected_revision；不能向source-events写客户端伪事件。

**任务映射：**MP03-T02-03 MP03-T02-04 MP03-T02-06。

**独立验收：**

1. 按“进入先快照后事件；断线→同run恢复；steer/cancel二次确认”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“gap/过期游标/未知schema重同步；已终态不再执行危险命令”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P09｜审批与风险确认

**原型：**`prototype/pages/09-approval.html`；**Taro路由：**`subpackages/execution/pages/approval/index`。

**目标：**用户理解动作对象与影响后，只批准当前这一次操作。

**布局：**风险提示；动作摘要；影响/对象；参数折叠区；固定拒绝/仅此次同意。

**组件：**RiskIntro / ActionSummary / ParameterPreview / ConfirmSheet。

**状态模型：**DecisionViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**目标、影响、参数脱敏；真实decision类型及当前版本；拒绝理由有界。

**接口合同：**E 单run interactions；POST interactions/:id/decisions。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**待处理→二次确认→提交中→通过/拒绝；关闭弹窗不写入。

**异常与边界：**过期/他端已处理/版本冲突刷新；无永久放行；预算不等于外部授权。

**权限：**来自服务端的可处理interaction；预算扩展不等于外部写操作授权。

**任务映射：**MP03-T02-05。

**独立验收：**

1. 按“待处理→二次确认→提交中→通过/拒绝；关闭弹窗不写入”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“过期/他端已处理/版本冲突刷新；无永久放行；预算不等于外部授权”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P10｜任务产物

**原型：**`prototype/pages/10-artifact.html`；**Taro路由：**`subpackages/execution/pages/artifact/index`。

**目标：**获取后台真实保存的产物，不在小程序运行生成代码。

**布局：**文件信息；安全标记；文本预览；下载/返回操作。

**组件：**FileCover / SafeTextPreview / DownloadAction。

**状态模型：**ArtifactViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**文件类型MIME和扩展联合校验；不运行HTML/JS。

**接口合同：**E sessions/message artifacts及授权download，关联按实际schema。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**从任务→重新授权读产物→安全预览或系统下载。

**异常与边界：**过期删除撤权显示原因；无长期公开地址；跨run映射不猜测。

**权限：**通过受鉴权资源接口下载，不保存永久公开地址。

**任务映射：**MP03-T02-07 MP01-T02-04。

**独立验收：**

1. 按“从任务→重新授权读产物→安全预览或系统下载”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“过期删除撤权显示原因；无长期公开地址；跨run映射不猜测”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P11｜知识库

**原型：**`prototype/pages/11-knowledge.html`；**Taro路由：**`pages/knowledge/index`。

**目标：**找可访问的知识库并进入检索或问答。

**布局：**空间/搜索；筛选；知识库卡；最近文档；底部4Tab。

**组件：**KnowledgeSearch / KnowledgeCard / RecentDocument。

**状态模型：**KnowledgeListViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**筛选我的/空间/共享按真实权限；关键词加入scope查询键。

**接口合同：**E 知识库共享API与domain筛选。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**选择库→文档列表；搜索→同范围结果；问答→指定授权库。

**异常与边界：**共享只读库无导入；不存在/无权/空库区分。

**权限：**知识共享范围和tenant范围按后端组合验证。

**任务映射：**MP02-T02-01。

**独立验收：**

1. 按“选择库→文档列表；搜索→同范围结果；问答→指定授权库”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“共享只读库无导入；不存在/无权/空库区分”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P12｜知识库详情

**原型：**`prototype/pages/12-kb.html`；**Taro路由：**`subpackages/documents/pages/kb/index`。

**目标：**查看文档与处理状态，选择基于此库提问。

**布局：**知识库头部；双操作；文档搜索；状态列表。

**组件：**KnowledgeHeader / DocumentRow / ProcessingBadge。

**状态模型：**KnowledgeDetailViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**文档状态真实映射；上传按钮依赖该库写权限。

**接口合同：**E 库内文档与处理状态API。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**基于此库提问→聊天；导入→目标已选；文档→授权详情。

**异常与边界：**upload成功仍parsing；失败保留原因；权限变化刷新。

**权限：**上传权限以实际知识库授权为准，不能仅从菜单判断。

**任务映射：**MP02-T02-01 MP02-T02-03。

**独立验收：**

1. 按“基于此库提问→聊天；导入→目标已选；文档→授权详情”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“upload成功仍parsing；失败保留原因；权限变化刷新”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P13｜添加知识

**原型：**`prototype/pages/13-upload.html`；**Taro路由：**`subpackages/documents/pages/import/index`。

**目标：**通过文件或URL将内容写入当前有权限的知识库。

**布局：**目标说明；导入方式；文件占位/URL；进度；确认与返回。

**组件：**TargetCard / FilePicker / URLField / ProcessingCard。

**状态模型：**UploadStateMachine。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**文件类型大小由服务端配置；URL只做初筛，后端最终防SSRF。

**接口合同：**E 知识upload/import；A Taro.uploadFile/native port。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**选择→确认目标→上传→解析→ready；解析中允许离开。

**异常与边界：**未知上传结果不重复创建；解析失败不显示成功；临时附件不自动入库。

**权限：**明确目标库与权限；URL安全策略由后端执行。

**任务映射：**MP02-T02-02 MP02-T02-03 MP01-T02-04。

**独立验收：**

1. 按“选择→确认目标→上传→解析→ready；解析中允许离开”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“未知上传结果不重复创建；解析失败不显示成功；临时附件不自动入库”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P14｜文档 / 引用详情

**原型：**`prototype/pages/14-document.html`；**Taro路由：**`subpackages/documents/pages/detail/index`。

**目标：**从回答引用追溯到获授权的原文片段。

**布局：**文件元信息；引用高亮块；正文片段；提问/下载。

**组件：**DocumentMeta / HighlightQuote / SafePreview。

**状态模型：**DocumentReferenceViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**来源位置仅用真实locator；显示更新和权限差异。

**接口合同：**E 授权文档/preview与引用领域映射。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**从引用打开→定位片段→看上下文→基于文档提问。

**异常与边界：**删除/撤权/版本变化说明；无位置不伪造页码；外部链接不直接跳。

**权限：**文档级鉴权；模型生成URL不直接当可信来源。

**任务映射：**MP02-T02-04。

**独立验收：**

1. 按“从引用打开→定位片段→看上下文→基于文档提问”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“删除/撤权/版本变化说明；无位置不伪造页码；外部链接不直接跳”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P15｜我的

**原型：**`prototype/pages/15-me.html`；**Taro路由：**`pages/me/index`。

**目标：**查看账号、当前空间、用量订单、邀请和隐私入口。

**布局：**身份卡；空间；用量摘要；功能列表；底部4Tab。

**组件：**ProfileHeader / WorkspaceRow / UsageSummary / AccountMenu。

**状态模型：**ProfileViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**用户和空间职责分开；用量字符串格式化；不从角色猜计费权。

**接口合同：**E auth/me、commercial/summary、orders、me/invitations。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**切空间→选择页；用量/订单/邀请→对应页；退出需确认。

**异常与边界：**stale标统计时点；退出清凭证缓存和请求；不假称撤销服务端已完成。

**权限：**账号与空间职责分开；退出清理本端数据并撤销可撤销登录态。

**任务映射：**MP04-T01-01 MP01-T03-05。

**独立验收：**

1. 按“切空间→选择页；用量/订单/邀请→对应页；退出需确认”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“stale标统计时点；退出清凭证缓存和请求；不假称撤销服务端已完成”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P16｜用量与套餐

**原型：**`prototype/pages/16-usage.html`；**Taro路由：**`subpackages/account/pages/usage/index`。

**目标：**解释可用/预占/退款冻结，并在权限允许时选择套餐报价。

**布局：**余额卡；用量条；套餐卡；政策待核验提示；订单入口。

**组件：**UsageCard / UsageTrend / PlanCard / AvailabilityNotice。

**状态模型：**UsageAndPlansViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**available/held/refund_locked独立；真实价格/单位不可硬编码。

**接口合同：**E summary/plans/usage/quotes；N 通道可用性。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**查看明细→选计划→服务端报价；无权仅只读。

**异常与边界：**未核验通道说明原因；过期价格不下单；历史数据有as_of。

**权限：**由后端billing权限与通道资格控制；金额/credits使用字符串真值。

**任务映射：**MP04-T01-01 MP04-T01-02。

**独立验收：**

1. 按“查看明细→选计划→服务端报价；无权仅只读”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“未核验通道说明原因；过期价格不下单；历史数据有as_of”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P17｜报价与支付

**原型：**`prototype/pages/17-checkout.html`；**Taro路由：**`subpackages/account/pages/checkout/index`。

**目标：**在已核验的购买能力下确认服务端报价并调起正确支付通道。

**布局：**订单信息；金额与权益；合规通道状态；底部确认；清晰模拟按钮。

**组件：**PlanSummary / QuoteDetails / PurchaseAvailability / PayCTA。

**状态模型：**CheckoutViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**quote_id有效且版本匹配；空间商品金额来自服务端。

**接口合同：**E quotes/orders；N 严格typed channel checkout。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**确认→幂等订单→获准收银→原订单查询；默认不可购买。

**异常与边界：**报价过期重取；取消/未知不新建单；客户端不算价格和权益。

**权限：**AI次数与会员先评估虚拟支付；合规普通服务才使用JSAPI；签名与openid从后端得到。

**任务映射：**MP04-T01-02 MP04-T01-03 MP04-T01-04。

**独立验收：**

1. 按“确认→幂等订单→获准收银→原订单查询；默认不可购买”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“报价过期重取；取消/未知不新建单；客户端不算价格和权益”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P18｜订单与履约结果

**原型：**`prototype/pages/18-order.html`；**Taro路由：**`subpackages/account/pages/order/index`。

**目标：**分别显示支付与权益发放状态，避免重复购买。

**布局：**两阶段状态；订单详情；状态说明；查询和返回。

**组件：**PaymentStep / FulfillmentStep / OrderDetails。

**状态模型：**OrderViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**order_id当前主体有权；付款和履约分别映射真实enum。

**接口合同：**E commercial/orders/:id；OrderView.payment/fulfillment。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**待付款确认→已付款未履约→服务端fulfilled→返回用量。

**异常与边界：**延迟/attention/退款按服务器结果；重查原订单不重复购买。

**权限：**仅当前空间可见且授权的订单；示例模拟不代表真实支付。

**任务映射：**MP04-T01-04 MP04-T01-05。

**独立验收：**

1. 按“待付款确认→已付款未履约→服务端fulfilled→返回用量”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“延迟/attention/退款按服务器结果；重查原订单不重复购买”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P19｜空间邀请

**原型：**`prototype/pages/19-invitations.html`；**Taro路由：**`subpackages/account/pages/invitations/index`。

**目标：**让微信深链进入已存在的租户邀请确认，而不是直接授权。

**布局：**邀请说明；卡片；权限范围；接受/拒绝。

**组件：**InviteCard / RoleSummary / AcceptDecline。

**状态模型：**InvitationsViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**token只定位并验证邀请；真实受邀用户、角色、状态。

**接口合同：**E me/invitations、accept/decline、accept-by-token。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**匿名先登录；接受后刷新membership再选择空间；拒绝回列表。

**异常与边界：**过期撤销不能接受；他人邀请不操作；接受不是全资源授权。

**权限：**仅受邀人可操作；分享链接token只用于服务端验证。

**任务映射：**MP01-T03-05 MP04-T02-03。

**独立验收：**

1. 按“匿名先登录；接受后刷新membership再选择空间；拒绝回列表”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“过期撤销不能接受；他人邀请不操作；接受不是全资源授权”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。

## P20｜异常与能力状态

**原型：**`prototype/pages/20-states.html`；**Taro路由：**`subpackages/system/pages/state/index`。

**目标：**展示加载失败、服务未开通、权限拒绝与断流恢复的不同表现。

**布局：**状态筛选；图形占位；原因与恢复；设计验收说明。

**组件：**StateIllustration / ReasonText / RecoveryAction。

**状态模型：**AppStateViewModel。读取支持loading/ready/refreshing/empty/error；业务状态不能压缩成一个loading布尔值。

**字段与校验：**错误不暴露堆栈或凭证；request_id可复制且不含机密。

**接口合同：**E ApiError和capabilities；N完整原因码映射。N为新增；E/A按固定源码适配，不代表本轮后端已实现。

**交互顺序：**六类状态分别提供正确动作；重试仅对应幂等读取。

**异常与边界：**不可把503/403当空；断流恢复不创建新run；登录回跳需白名单。

**权限：**不展示后端堆栈、凭证或敏感参数。

**任务映射：**MP01-T01-04 MP04-T03-03。

**独立验收：**

1. 按“六类状态分别提供正确动作；重试仅对应幂等读取”完成正常路径，确认导航、文案和状态对应实际接口。
2. 注入“不可把503/403当空；断流恢复不创建新run；登录回跳需白名单”，不发生隐式重复写、越权展示或误报成功。
3. 验证320/375/390/430宽度、长文案、底部安全区、焦点/触点，以及切空间后的旧回包隔离。

**原型与生产差异：**原型以本地状态示范流程；没有真实请求、权限、队列、计费和微信SDK。开发时替换feature数据源，不把demo开关/固定账号/价格/任务号带入生产。


---

# 06｜测试矩阵与生产发布验收

本文件定义**后续生产开发**必须提供的证据，不把HTML演示当作真实微信、后端、权限或支付测试。实际完成的原型验证另见07。

## 1. 测试层级

契约与纯逻辑测试 → Taro平台适配测试 → Go Router/应用服务集成 → 三端构建回归 → 微信开发者工具与Android/iOS真机 → 小范围灰度。每一层记录执行命令、环境、数据集、退出码、日志和对应提交；只有截图不算接口安全证据。

## 2. 核心验收矩阵

| 编号 | 场景与输入 | 必须观察的结果 | 关联任务 |
|---|---|---|---|
| QA-01 | 微信首次登录、已绑定登录、过期/重用code | 复用平台身份；失败不生成伪token；不泄露服务端session_key | MP01-T03-02 |
| QA-02 | 微信与平台账号绑定：并发、重复challenge、错用户 | 双侧身份均验证；唯一约束生效；冲突不覆盖原绑定 | MP01-T03-01 / MP01-T03-03 |
| QA-03 | 未加入任何空间的合法用户 | me/创建/邀请路径可用；不伪造tenant；不假定所有列表tenantless可访问 | MP01-T03-04 |
| QA-04 | 切空间时延迟旧请求、流、刷新与文件回包 | 新scope不接收旧generation结果；失败时保留原合法空间 | MP01-T02-05 / MP01-T02-06 |
| QA-05 | 替换tenant、run、session、document、order ID | 服务端逐资源拒绝越权；空间Owner不默认读取所有私有任务 | MP04-T03-02 |
| QA-06 | UTF-8中文/emoji任意字节分片、CRLF跨块、多data、心跳 | 与不分片输入产生同一业务事件序列；无乱码/提前dispatch | MP01-T02-02 / MP01-T02-03 |
| QA-07 | headers先到但HTTP状态未确认、401 JSON、非SSE错误体 | 不伪造HTTP200；已收到正文后不盲目重放POST | MP00-T01-03 / MP01-T02-03 |
| QA-08 | 关闭页面、onHide、用户明确取消任务 | 前两者仅终止订阅；只有明确命令调用cancel且带当前版本 | MP03-T02-03 / MP03-T02-06 |
| QA-09 | 点击两次、POST返回丢失、杀进程再进入 | 同一意图保存同一request_id；对账原run；不重复启动与计量 | MP03-T02-01 / MP03-T02-08 |
| QA-10 | 快照watermark、重复seq、seq缺口、游标过期 | 快照原子安装；先提交投影再推进游标；缺口重新同步 | MP03-T02-02 / MP03-T02-03 |
| QA-11 | 他端审批、revision变化、重复点击、已过期 | 旧决定不可提交；用户重新审阅；无永久放行与充值即批准 | MP03-T02-05 |
| QA-12 | 运行中取消、终态取消、steer遇到409 | 冲突后刷新并重确认；不自动以新版本重发危险动作 | MP03-T02-06 |
| QA-13 | 无FormData环境下原生上传与Web multipart | native分支不构造FormData；已有Web上传回归正常 | MP01-T02-04 |
| QA-14 | 文件上传完成但解析未完成/失败、页面重入 | uploading/parsing/ready/failed分开；不重复上传同一条知识 | MP02-T02-02 / MP02-T02-03 |
| QA-15 | URL导入私网、重定向到私网、无权目标库 | 服务端抓取保护与库写权限生效；客户端校验不是安全边界 | MP02-T02-02 / MP04-T03-02 |
| QA-16 | 聊天临时附件与知识导入 | 生命周期与授权分开；没有用户确认就不自动入库 | MP02-T01-03 |
| QA-17 | 撤权引用、过期产物、模型生成HTML/脚本 | 再次鉴权；无页码不编造；不运行生成代码、不保存永久链接 | MP02-T02-04 / MP03-T02-07 |
| QA-18 | 无计费权限、资格未知、商品或终端不支持 | 前后端均不可购买且有原因；不提供绕过限制的外部支付跳转 | MP04-T01-02 |
| QA-19 | 报价过期、金额变化、收银台取消或未知结果 | 重取合法报价；查询原订单；不以前端金额下单或重复买单 | MP04-T01-04 |
| QA-20 | 客户端支付成功，但回调延迟与履约失败 | payment与fulfillment分开；不自行增加余额；支持对账恢复 | MP04-T01-04 / MP04-T01-05 |
| QA-21 | 重复/乱序回调、错误金额、伪签名、无JWT合法签名 | 精确公开回调入口；验证渠道与业务要素；一次履约 | MP00-T02-02 / MP04-T01-05 |
| QA-22 | 拒绝通知授权、重复事件、消息投递失败 | 不阻断任务；去重投递；通知不泄露敏感内容 | MP04-T02-01 / MP04-T02-02 |
| QA-23 | 邀请/通知深链含任意URL、他租户run、过期token | 白名单目标；先登录/选空间/鉴权；链接不自带授权 | MP04-T02-03 |
| QA-24 | 320/375/390/430宽度、大字体、真实胶囊和键盘 | 无水平溢出；关键按钮可达；输入不被遮挡；触点和状态可区分 | MP04-T03-03 / MP04-T03-04 |
| QA-25 | 长会话、长任务、频繁切页、后台恢复、低网速 | 流缓冲有界；监听释放；列表分页；不因重入再次执行 | MP04-T03-04 |
| QA-26 | 新增共享port/schema/parser后旧React Web与Expo | 类型、构建、聊天和上传回归通过；无被动依赖降级 | MP04-T03-01 |
| QA-27 | 撤销登录、退出、设备缓存满、存储损坏 | 清除scope秘密；恢复记录显式失效；不把写失败声称为可靠恢复 | MP01-T02-07 / MP04-T03-02 |
| QA-28 | 灰度关闭购买、关闭执行写入、UI回滚 | 保留原订单查单与已启动任务读取；数据库兼容旧客户端 | MP04-T03-06 |

## 3. 发布门禁与证据归档

测试环境至少使用两个租户、三个用户、一个只读共享知识库；覆盖成员、Owner和具有实际billing权限的账号。不能仅因角色名称相同就视为权限等价。

代码证据包括修改提交、测试日志、接口请求/响应脱敏样例和数据库/订单/计量核对；终端证据包括基础库、系统版本、设备、网络条件与录屏。不得保存真实密码、openid全量、支付签名、模型密钥或原始敏感文档。

性能阈值必须先由MP00实测确定并写入CI。此设计不承诺未经测量的首屏秒数、包体限额或并发上限。HTML在浏览器的速度不能替代Taro真机性能。

上线开关应分开：知识问答、任务启动、任务操作、购买、通知。关闭购买不停止已付款订单的对账履约；关闭任务启动不阻止原任务恢复读取。任何尚未获准的支付渠道保持unavailable。
