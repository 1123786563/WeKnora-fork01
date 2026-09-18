# 01｜Taro 微信小程序技术方案

版本：设计稿 v1.0 · 2026-09-17  
代码基线：见《00-源码核查》；所有标为“新增/建议”的类型、目录和 API 都不是当前仓库事实。

## 1. 产品定位与范围

小程序是基于现有 WeKnora SaaS 的随身入口：**使用 Agent、知识问答、发起任务、查看执行、处理本人可处理的审批、查看产物、知识上传、空间切换、用量与订单。**它不是另一套 Agent 平台，也不是压缩版 Web 管理后台。

第一版优先个人及团队成员使用。复杂 Agent 编排、模型密钥、MCP/OAuth 安装、知识解析引擎配置、沙箱终端、完整 Craft 编辑器、系统管理、退款审核保留在 Web。小程序可显示“需要管理员配置”的明确原因，不显示伪可用按钮。

采用四个主 Tab：**工作台 / 任务 / 知识 / 我的**。Agent 和会话从工作台进入，审批从任务进入；不把 Agent 市场、对话历史和通知各占一个主 Tab。

## 2. 技术路线与备选比较

| 路线 | 判断 |
|---|---|
| 在现有原生 miniprogram 直接扩展 | 能少量增功能，但无法自然复用 React 组织方式，且需重做 API Key 身份入口 |
| 新增 Taro React 工程接共享包 | **推荐**：保留 Go 真值、Web/Expo 各端 UI，新增微信端适配与轻量页面 |
| 全站 WebView 包裹 React Web | 不作为主方案：交互与微信能力受容器约束，且主包/原生体验问题没有消失 |

建议基础选型：Taro 4.x 官方 React + TypeScript 模板，所有 @tarojs/* 锁同一经过验证的补丁版本；React 与构建器按该模板兼容组合独立锁定。**不要强行把 Web 的 React 19.3 / Vite 7 / TS 6 版本覆盖到 Taro；也不要为了小程序降级整个 monorepo。**React 18 兼容组合可作为 PoC 起点，具体锁版本由 MP00 编译和真机实验决定，不声称某个补丁是“当前最新”。

样式首版选 SCSS + BEM/局部约定 + 提取的设计变量；基础组件以 Taro View/Text/Button/Input/Textarea/ScrollView 构建。若引入 NutUI Taro，只按需引入并核查当前兼容版本与包体。不要直接打包 Radix/shadcn DOM 组件或现有 @weknora/ui/@weknora/views。

状态选型：先用现有 domain 纯函数与作用域控制器；会话/执行投影与 UI store 分开。需要服务端缓存时可采用 TanStack Query，接入微信前后台及联网状态，关闭不适用的浏览器监听。Zustand 只保存端状态/草稿，不做服务端权限和余额真值。共享包避免直接依赖 React，便于不同端独立版本。

## 3. 总体架构

```text
                       同一账号 / 同一空间 / 同一后端
 React Web                  Expo Mobile                 Taro 微信小程序
 DOM 组件                    Native 组件                 Taro 组件 / 微信 API
     └──────────────────────────┼───────────────────────────┘
                contracts / api-client / domain / i18n
                     HTTP / SSE / 凭证 / 文件 ports
                                │
                         现有 Go Gin API
         ┌──────────────────────┼────────────────────────┐
    Auth / Tenant          Session / Workbench      Knowledge / Commercial
    Membership / RBAC      执行 / 事件 / 交互        文档 / 产物 / 用量 / 订单
         │                      │                        │
    新增微信身份桥接      保持现有 Runtime 装配      新增合规支付渠道适配
         │                      │                        │
    微信 code2Session      现有后台工作/存储         虚拟支付 / 合规 JSAPI
```

不新增一套 Node BFF；必要的移动聚合查询直接在 Go 中增加薄 handler/application 查询服务，调用已有权限和业务服务，不复制规则。小程序不直连模型、MCP、Redis、数据库、支付密钥或运行容器。

## 4. 代码组织与复用规则

以下是建议增量目录，不代表仓库已有这些文件：

```text
apps/miniprogram/
├── config/index.ts
├── project.config.json              # 无密钥；按环境注入 AppID
├── src/app.tsx app.config.ts app.scss
├── src/platform/
│   ├── http.ts stream.ts utf8.ts     # 实现 HttpTransport
│   ├── credentials.ts scope.ts       # CredentialAdapter / Scope 适配
│   ├── files.ts lifecycle.ts         # 上传下载、show/hide、联网
│   ├── payment.ts notifications.ts   # 平台能力封装
│   └── navigation.ts storage.ts
├── src/components/                  # Taro 专用组件
│   ├── WorkspaceSwitcher/ AgentCard/ TaskCard/
│   ├── MessageBubble/ CitationCard/ AttachmentItem/
│   ├── Timeline/ ApprovalCard/ UsageCard/ EmptyState/
├── src/pages/                       # 主包四个 Tab
│   ├── home/ tasks/ knowledge/ me/
├── src/subpackages/
│   ├── auth/ agents/ chat/ execution/ documents/ account/
├── src/features/                    # 小程序用例组装，不复制后端业务
└── tests/                           # 平台适配、契约、页面联调

packages/api-client/src/platform/     # 仅需要共用的端无关适配助手；不要依赖 Taro
packages/contracts/src/miniprogram/   # 新增契约（如有）；保留现有字段语义
packages/domain/src/mobile/          # 补可跨端复用的执行投影/恢复逻辑
internal/handler/                     # 微信登录、聚合查询的薄入口
internal/payment/                     # 渠道扩展，复用现有 commercial 订单
```

| 内容 | 复用策略 | 禁止做法 |
|---|---|---|
| contracts DTO 与 validators | 直接优先；字段差异显式映射 | 每个页面手写一套相似接口 |
| api-client endpoints | 保留 URL、错误模型、解析器 | 页面直接 wx.request 散落调用 |
| domain scope/chat/knowledge/mobile | 逐文件审查后复用纯逻辑 | 把有 window/localStorage 的代码当纯函数 |
| 语言包与 design tokens | 选取词条/颜色/间距，再编译微信样式 | 整包导入巨型图标/语言/编辑器 |
| Web UI / Expo UI | 复用交互规范，不直接复用节点 | 将 div、Radix、RN View 自动视作 Taro View |
| 微信 API 与生命周期 | 限定在 platform 层 | 将 Taro 反向引入共享 domain |

共享包深路径导出需显式声明，避免大型 barrel 入口使小程序被动加载全部功能。主包只装首页/Tab/登录引导所需模块；聊天渲染、文档查看、订单等按分包加载。包体阈值以当前微信规则及 CI 输出为准，不写死未经核验的历史限额。

## 5. HTTP、文件与身份传输

保留已存在接口：HttpTransport.send / sendStream / sendMultipartFile / sendBinary；NativeFileSource 使用 uri/name/type/size；CredentialAdapter 异步读写；RequestScope 含 origin/userId/tenantId/generation。[R15]

实现映射：

| 共享接口 | 微信实现 | 关键要求 |
|---|---|---|
| send | Taro.request | 对象 JSON 与字符串 body 区分、大小写规范化响应 header、非 2xx 错误归一 |
| sendStream | request + enableChunked + onChunkReceived | UTF-8 增量解码 → AsyncIterable<string>；独立连接生命周期 |
| sendMultipartFile | Taro.uploadFile | filePath、固定 form name、formData、进度回调；不手写 multipart boundary |
| sendBinary / 大文件下载 | request arraybuffer / downloadFile | 大文件优先落临时文件，限制内存；重新鉴权下载 |
| signal | AbortSignal → RequestTask.abort | 微信 request.signal 不能当作浏览器原生取消直接依赖 |
| 凭证 | 受限访问的本地存储封装 | 无硬件安全承诺；短有效期、可撤销刷新、退出清理、严禁日志 |

HttpStreamResult.status 与 headers 是一个实际适配难点：微信收到 headers 时未必同时提供可依赖的 HTTP 状态字段。MP00 要验证所选基础库；若不能提前取得真实 statusCode，不应始终伪造200，也不能等整条流结束才开始显示。应对共享流 port 做向后兼容的元信息扩展，显式表示“响应头已到、状态待确认”，由消费层先校验 SSE 类型及应用帧、并在终结回调确认真实 HTTP 状态；非 SSE 返回走缓冲错误路径。现有 Web/Expo 的已知状态路径保留。首事件前可进行有边界的认证刷新，已输出事件后不能盲目重放写入型 POST。这是 MP00 必须跑通的流式适配实验，而非未经验证的零改造承诺。

现有 Client 的默认请求超时与长流分开配置。App 隐藏时应主动中止订阅并清理监听，而不是取消后台任务；用户明确点击“取消任务”才发服务端 command。对普通 POST 问答断开后的生命周期要实测，不能保证所有旧聊天都与 durable execution 一样可后台续跑。

## 6. 登录、绑定和空间隔离

### 6.1 两条登录入口

第一版保留已有账号密码登录，直接复用 auth/login；新增微信快捷登录，调用 Taro.login 拿 code，再由 **Go 服务端**交换微信身份。不得在小程序放 AppSecret、租户 API Key、session_key 或支付渠道密钥。[R08、R09；官方 Taro.login]

```text
用户阅读隐私与服务说明 → 点击微信登录
→ Taro.login 取得短时 code
→ 新增 POST /api/v1/auth/wechat/mini/login
→ Go 使用配置中的 AppID/AppSecret 调 code2Session
→ 查找/绑定已有平台 user_id
→ 复用已有 token/refresh 与会员关系返回模型
→ auth/me + 已有 membership / TENANT_REQUIRED 流程
→ 有空间：选择/进入；无空间：创建或接受邀请
```

账号绑定必须证明两侧身份：微信 code + 已登录平台账号/一次性绑定凭证。不能通过用户自报手机号、昵称、邮箱或 unionid 无验证合并账号。已有 OIDC 身份存储是否能统一 provider 需在实现时审查；若无合适结构，新增 `external_identities`（建议名）记录 provider + appid + subject(openid) + user_id，唯一约束防并发重复绑定。unionid 可选，不能假定一定返回。

session_key 如虚拟支付签名需要，只保存在服务端受控加密存储并按有效性更新；不返回客户端。昵称头像按需采集；手机号授权不是登录的必选前提。

### 6.2 作用域与切换

平台 user_id、tenant_id、membership/role 是业务授权依据；微信 openid 仅为外部身份。URL/请求头的 tenant_id 不是可信授权结果。继续由后端验证 membership 和资源归属，沿用现有 auth/switch-tenant 机制。[R08–R09]

切换顺序：冻结写入 → 终止旧 scope 请求/流 → 请求 switch-tenant → 成功后原子更新 token 和租户 → generation 增加 → 丢弃旧 generation 回包 → 清理旧数据/草稿视图 → 加载新租户。失败保留原空间且解冻。所有缓存键至少含 origin + userId + tenantId + 资源 ID；不只用 session_id。

auth/me 可在 tenantless 状态使用；已读白名单中 GET tenants 不是 tenantless 通用入口，不能先假定“无空间用户一定可以调 GET tenants”。继续使用现有身份返回/邀请/创建空间流程。

## 7. 对话与任务：保留两条协议

### 7.1 普通知识/Agent 对话

复用 sessions、messages、knowledge-chat、agent-chat、attachments、references 与已有聊天 parser/domain。流式输出投影为文本、引用、工具状态和附件卡片，不显示未经筛选的内部 prompt、秘密参数或私有推理。用户看到的是执行摘要，不是内部思维链。[R11、R17、R21]

临时聊天文件属于 session/attachment，只有用户明确执行“保存到知识库”才走知识入库流程；二者权限、生命周期、计费与解析状态分开。工作台 StartExecutionInput 当前没有 attachment_ids 或任意知识库选择字段，第一版启动任务按已配置 Agent 的资源范围执行，不私自向 DTO 填字段；确需任意附件任务输入时另做共享契约扩展。

Markdown 在流式阶段做轻量文本/段落渲染，完结再完整格式化；代码块延后高亮，长对话分页。引用跳到授权文档页，不直接导航模型给出的任意 URL。禁止小程序运行模型生成的 HTML/JS。

### 7.2 持久执行

直接使用当前 StartExecutionInput：[R18]

```ts
interface StartExecutionInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number; // 保持现有后端单位，展示前明确单位转换
}
```

session 由已有 API 创建/选择；agent、target、workspace 由有权限的服务端选项解析。小程序界面不让普通用户输入内部 target_id 或连接执行器。第一版优先平台托管的 target，外部设备/Paseo 支持按部署能力后置。

一次点击生成一个 request_id，发送前记录最小待确认请求。超时不换 ID 重发：先 GET executions/requests/:request_id 查询 pending/dispatching/admitted/rejected/unknown；unknown 也不是允许创建新业务意图的依据，按后端幂等约定重试同一请求。页面离开后回来仍查原请求。

保留现有 RunStatus：queued / running / waiting_user / reconciling / succeeded / failed / canceled。`run_status`、`execution_status`、`settlement_status` 分开展示：执行完毕不必然代表账务已结算。[R19]

### 7.3 恢复协议

```text
进入执行页 / 回到前台
→ 先取 snapshot（同一 run 和 scope）
→ 安装快照投影，记录 watermark
→ GET events，Last-Event-ID = 已持久提交的游标
→ 验证 schema/run/seq
→ 先提交事件/投影，再提交游标
→ 重复 seq 忽略；gap / 游标失效 / schema 不支持 → 停流并重新同步
```

复用 ExecutionCache 的范围隔离、连续序号校验和 commitEvent 顺序。[R23] 但它是**内存参考实现**，没有“从快照安装 watermark”的现成公开方法；应补共享投影安装方法/测试，不能 new Cache 后把大于1的事件直接 commit。快照历史 events 是否完整、watermark 的权威语义、服务端保留窗口要在 handler 联调中确认，不把一次下载所有历史当成无限可扩展方案。

已验证的断流降级为获取快照和带退避的状态轮询，不静默再次触发模型调用。后端快照还未持久可读时显示“正在恢复”，不能显示伪已完成。建议前台一次只维护当前可见执行的主流；任务列表用聚合查询更新，不为每张卡开长连接。

## 8. 审批、追加指令与产物

单执行已有 GET interactions、POST decisions；命令为 cancel/steer + expected_revision。小程序只映射已经支持的决策类型，按实际 interaction schema 构造输入；不要虚构 approve 字段替代既有协议。[R10、R18]

审批页必须展示：空间、任务、动作、目标资源、必要且脱敏的参数摘要、影响/风险、有效期（如服务端提供）、当前状态。决定绑定 interaction、当前用户、tenant、run、revision/版本；重复决定、过期或他端已处理显示明确冲突并刷新。预算扩容和外部写操作审批分开，不能“充值即同意执行”。默认不提供全局永久放行。

产物首版支持纯文本/受限 Markdown、图片和系统支持的文档下载查看。通过已有鉴权下载接口取得文件；短链接也需由服务端签发、绑定范围与有效期。HTML/Craft/可执行脚本仅显示源码摘要/静态截图或提示在 Web 审阅，不在小程序执行。缓存中不保存长期公开下载 URL。

## 9. 知识库与上传

提供知识库列表、知识库内文档、检索结果、引用段落、文档状态、文件上传、URL 导入；高级 chunk、模型、向量库、图谱和解析引擎配置留在 Web。服务端结果决定可见与可操作范围，UI 再做可理解的权限提示。

选择文件/图片 → 验证类型与服务端允许大小 → Taro.uploadFile 到现有 API → 上传完成后进入解析状态 → 用户可离开页面 → 列表刷新或状态查询显示 ready/failed。上传成功不等于可检索。失败保留原因与操作，不在网络重试中重复建知识条目。URL 导入仍使用后端现有抓取保护；不由小程序自己抓取/转发任意内网 URL。

## 10. 商业化与微信支付

### 10.1 复用的真值

沿用 CommercialSummary 的 plan_name、paid_until、available、held、refund_locked、as_of、stale。UI 将 available 与 held 分开展示，并标明统计时点；金额 amount_fen、余额等字符串字段不转成浮点做业务计算。[R20]

沿用 QuoteInput 的 plan_key / plan_version / subscription_version；下单使用 quote_id、provider、idempotency_key。Quote 过期或版本变化要重新报价，不能让前端提交自算金额。只有具备当前计费管理权限的主体能支付/变更，普通成员可按后端规则看用量或请求管理员处理。

### 10.2 支付通道不能混为一谈

当前 wechat provider 是 Native：`/v3/pay/transactions/native → code_url`。[R13] 微信普通小程序支付官方文档为 JSAPI 下单，生成 prepay_id；这与现有 QR checkout 不同。[W04]

**AI 会员、功能解锁、次数包先按虚拟商品方向做主体与商品分类核实。**若适用虚拟支付，接 `requestVirtualPayment` 对应渠道，不能以普通 JSAPI 规避；确属获准使用普通小程序支付的服务商品才使用 JSAPI + requestPayment。官方虚拟支付文档存在 iOS 支持路线，不沿用“iOS 一律无法购买”的旧假设。[W05]

本次没有取得该小程序的主体资质、商品分类或商户后台，因此不保证任何通道已开通，也不提供固定费率或审核通过承诺。默认 `canPurchase=false`（建议字段），后端核验账号、终端、商品、开通状态后开放；不可用时仅说明原因，不给绕过平台规则的外部支付跳转。

建议增加可区分的 checkout 描述（**新增 DTO，不是已有字段**）：

```ts
type MiniCheckout =
  | { kind: 'wechat_virtual'; order_id: string; payload: VerifiedVirtualPayArgs }
  | { kind: 'wechat_jsapi'; order_id: string; payload: VerifiedJsapiArgs }
  | { kind: 'unavailable'; reason: string };
```

`Verified*Args` 由对应渠道版本的服务端 DTO 生成/校验，不是客户端自由对象。保留 Web Native checkout，允许旧客户端忽略新增字段；已有限定 provider 的枚举需向后兼容扩展和版本化测试。渠道不同可以复用业务订单与权益履约，**不能复用不适用的回调验签格式**。

JSAPI 场景：服务端从已验证身份取小程序 AppID 对应 openid，下单/签发 timeStamp、nonceStr、package、signType、paySign。客户端不可自报任意付款人 openid。虚拟支付场景：平台 AppKey 与有效 session_key 的签名在服务端生成，签名字符串与真正发送字符串保持一致；用户态失效重新微信认证但仍使用原业务订单对账。[W04–W05]

### 10.3 付款与履约双状态

```text
后端报价 → 业务订单 → 合规渠道参数 → 微信/系统支付界面
→ 小程序返回（只说明客户端结果）
→ Go 验签回调 / 主动查单 → 付款 paid
→ 幂等履约 → fulfillment=fulfilled → 刷新权益/余额
```

`requestPayment/requestVirtualPayment` 成功回调不能自行加余额。客户端展示“付款待确认 / 已付款权益处理中 / 已开通 / 需要处理”，保持现有 OrderView.payment 与 fulfillment 两条状态。[R20] 用户取消收银台不自动代表渠道订单已关闭；超时/未知支付不能马上再建新单。每个回调校验渠道身份、订单、金额、货币、商品及必要 AppID/MchID，重放不能重复发放；退款后权益与积分回收沿用后端规则。

第一版以明确周期的一次购买为主，自动续费需单独的签约协议、通道开通与状态设计，不把普通“月度套餐”按钮自动实现成代扣。

## 11. API 增量清单

下表路径省略 /api/v1。N 路径均为建议命名，实施时需要更新共享契约与测试。

| 业务 | E：可复用 | N/A：增量 |
|---|---|---|
| 登录 | POST auth/login、auth/refresh；GET auth/me | N：POST auth/wechat/mini/login；绑定/解绑端点按身份设计 |
| 空间 | POST auth/switch-tenant、tenants；me/invitations | A：微信深链回到邀请流程；不重复建设成员库 |
| 普通对话 | sessions；messages/:session_id/load；knowledge-chat/:id、agent-chat/:id | A：流与页面；不改已有会话归属 |
| 上传/产物 | sessions/:id/attachments；sessions/:id/artifacts 等 | A：微信文件接口与安全预览 |
| 启动执行 | POST workbench/executions；GET .../requests/:request_id | A：持久提交意图、恢复、不重复计费 |
| 执行详情 | GET .../:run_id、snapshot、events | A：断流恢复；能力校验 |
| 任务列表 | 没有在已读 workbench 路由看到集合 GET | N：GET workbench/executions?cursor=&status=，严格 owner/tenant 范围 |
| 待办聚合 | 单执行 interactions 已有 | N：GET workbench/inbox?cursor=，只汇总本人可处理条目 |
| 工作台首页 | 会话/知识/商业信息分别已有 | N：可选 GET workbench/home，聚合失败单卡降级，不复制权限规则 |
| 操作 | interactions/:id/decisions；:run_id/commands | A：现有决策类型、版本冲突、取消/steer |
| 用量订单 | commercial/summary、plans、usage、quotes、orders | N：checkout 多形态与微信渠道；原订单真值不重建 |
| 能力 | GET system/capabilities + ExecutionDTO.capabilities | N：补 workbench、mini登录/支付/通知的部署及终端可用性 |
| 消息 | 本次未核实小程序订阅消息服务已存在 | N：用户授权记录、模板映射、投递与失败追踪（后置） |

新增任务列表建议返回只读卡片 DTO：run_id/session_id、title、agent 展示摘要、run_status、created_at/updated_at、可见的待办数、分页 cursor。title/timestamps 等**不在已读 ExecutionDTO 里**，需要来自真实查询投影，而不是假装旧 get 接口已经返回。总任务数只能来自权限范围内服务端统计。

## 12. 安全、隐私和发布门禁

安全底线：身份与资源授权全部在服务端；绑定二次证明；scope隔离；JWT/刷新单飞；不上传密钥；下载授权；执行指令参数最小化；回调验签与幂等；日志脱敏。手机丢失/退出后可撤销刷新凭证，缓存与文件按用户/租户清理。

内容与隐私：登录前可查看隐私协议、服务协议；文件/录音/手机号等按需请求权限，不在启动时一次申请全部；说明数据上传、存储及模型处理范围。实现举报/删除入口与人工处理流程；对用户内容、上传及模型输出接现有或新增审核环节，日志限制访问。中国大陆公开运营需由运营/合规负责人核实当前小程序备案、服务类目、生成式 AI/深度合成相关要求，设计稿不替代合规意见。

发布采用 dev/staging/prod 分离的固定域名和配置。按当前微信规则配置 HTTPS 请求、上传、下载及必要 socket 域名；不能让终端用户随意填写 API origin。Nginx/网关对 SSE 关闭缓冲、合理设置空闲超时并实测 flush；非浏览器无需套 CORS，但共享 Web 回放头仍需核对网关允许头。AppSecret/渠道密钥/CI 上传私钥只在服务端或安全 CI，禁止在项目配置与包中出现。

## 13. 实施阶段、依赖与并行边界

任务编号前缀 MP，避免覆盖仓库其他阶段编号。每个任务都应配代码位置、测试证据和 PR 链接。此处不给未经实测的工期承诺。

| 任务 | 目标/主要步骤 | 依赖 | 可并行 | 文件锁/验收 |
|---|---|---|---|---|
| MP00-T01 | 固定源码、验证 Taro/React/TS workspace组合，最小真机页 | 无 | 与T02 | apps/miniprogram 基础配置；微信编译和运行证据 |
| MP00-T02 | 核查 Auth/RBAC、工作台装配、回调公开路径与支付分类 | 无 | 与T01 | router/auth/payment；无JWT合法回调可达且伪签名拒绝 |
| MP01-T01 | 新工程、导航、基础组件、空错态、分包 | MP00 | 与后端/契约工作 | mini shell；四Tab和页面跳转 |
| MP01-T02 | request/stream/upload/storage/scope 适配；修FormData、CRLF | MP00 | 与T01/T03 | api-client ports/client/stream 由单负责人合并；兼容测试 |
| MP01-T03 | 微信身份桥接、绑定、邀请和tenantless流程 | MP00-T02 | 与T01/T02 | auth/migrations；重复code/错绑定/无空间/跨租户测试 |
| MP02-T01 | 普通聊天、引用、临时附件与历史 | MP01 | 与知识模块 | mini chat + shared chat；中文流/断流/附件完整链路 |
| MP02-T02 | 知识列表、文档、导入与解析状态 | MP01 | 与聊天模块 | mini knowledge；上传成功与解析成功分开 |
| MP03-T01 | 任务列表/待办服务端读模型、能力声明 | MP01 | 与前端mock页 | workbench handlers/contracts；分页及owner隔离 |
| MP03-T02 | 启动/快照/回放/取消/steer/审批/产物 | MP02、MP03-T01 | 按页面分配 | shared mobile projection 单锁；后台切回不重启任务 |
| MP04-T01 | 商业页、合规渠道、checkout契约、查单履约 | MP00-T02、MP01 | 与通知/回归 | commercial/payment/contracts 单锁；重复回调不加两次权益 |
| MP04-T02 | 按授权通知、深链与邀请 | MP03 | 与支付 | mini platform/Go通知；拒绝订阅不阻断任务 |
| MP04-T03 | 真机、负向、安全、性能、灰度发布 | 所有上线能力 | 可并行测试 | 发布门禁；不宣称仅浏览器HTML测试等于微信验收 |

并行顺序：先锁定 contracts/ports/路由命名，再按 UI 页面、Go auth、Go read-model、payment 分轨开发。共享 client.ts、contracts根导出、router.go、锁文件必须单负责人串行合并，避免多代理同时覆盖。仅在接口和验收条目确定后并行，不靠页面假数据掩盖接口缺口。

## 14. 必须通过的验收用例

1. 微信首次登录/已绑定登录/已有账号绑定/无空间加入邀请/账号退出与刷新失效。
2. 切换空间时旧请求、旧流、旧草稿、旧余额不会写回新空间；伪造 tenant、run、attachment ID 均被拒绝。
3. UTF-8中文和emoji按任意字节分片；CRLF跨chunk；粘包、多行data、评论心跳、错误Content-Type；无重复消息或乱码。
4. 弱网启动超时后按 request_id 找回同一run；杀进程后恢复同一run；取消订阅不误取消任务。
5. snapshot + watermark 后续流不重放已提交事件；seq gap、游标过期、schema不兼容进入同步而不是继续乱序显示。
6. 审批重复点击、过期、异端已处理、revision变化；预算扩容不自动批准外部写操作。
7. 临时附件不自动成为知识；上传中断可处理；解析失败可诊断；受限文件不能通过深链越权下载。
8. 报价过期、无购买权限、通道未开通、支付取消、回调延迟、已付款未履约、重复回调、乱序回调、查单恢复、退款异常。
9. Android/iOS实机、长文本、后台恢复、低网速、缓存清理、基础库最低版本；包体与首屏性能通过项目设定门槛。
10. 所有 capability=forbidden/unavailable 场景显示明确原因；404/503不伪装为空数据。

## 15. 外部技术参考与核实边界

- W01 Taro 4.x request：https://docs.taro.zone/docs/apis/network/request/
- W02 Taro RequestTask（chunk、headers、abort）：https://docs.taro.zone/docs/apis/network/request/RequestTask
- W03 Taro.login：https://docs.taro.zone/docs/apis/open-api/login/
- W04 微信支付官方 JSAPI/小程序下单：https://pay.wechatpay.cn/doc/v3/merchant/4012791897
- W05 微信官方虚拟支付文档及 iOS 接入页：本次原站直连失败，使用 Context7 检索到的官方来源文档副本；原地址为 https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment 及其 /ios.html 页面。涉及主体、商品分类、费率、支付可用性须上线前在实际公众号/小程序后台复核，本方案不使用第三方博客的费率/政策日期作确定依据。

本次交付是方案和离线交互线框，不是可提交微信审核的 Taro 生产代码。源码存在性核查与 HTML 原型验证，分别见其他交付文件。
