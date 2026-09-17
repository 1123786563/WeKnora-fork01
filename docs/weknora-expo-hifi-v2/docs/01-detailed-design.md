# WeKnora Expo 移动工作台 · 详细设计 v2.0

日期：2026-09-17。状态：开发评审交付，未实施仓库业务代码。继承源码基线 `12737238aa7b9d6891e76b2397f6941024f45668`。

## 1. 范围与裁决

本稿将上一版灰度线框升级为可实施的产品、交互、接口与原生工程设计。继续使用已有 `apps/mobile`，保留 Happy 移动体验资产，但产品会话不能依赖 Happy 账户、daemon 或全局同步数据。WeKnora Go 仍是产品权威；不引入第二套任务后端、审批引擎或商业钱包。

交付范围包括18个页面、复用组件、明暗主题、8类场景演示，以及36项MX增量任务。HTML是交互说明工具，不是可直接发布的Expo业务应用。模拟登录、上传、语音、连接授权、审批与费用不会访问真实服务；只有“下载示例”在浏览器生成本地Markdown文件。

本文涉及的A类接口继承已读源码；B类为新增提案；C类为已有计划但仍需在当前checkout核实的接口。正式实现从MX-001对账开始。原W/T/H任务与状态不被替换。输入依据见 [原技术方案](../references/01-expo-mobile-architecture.md)、[原差异清单](../references/02-source-review-and-gap.md)、[原API分类](../references/04-api-contract-proposal.md)。

### 1.1 不变约束

- 空间Tenant负责成员、资源隔离和独立费用；执行工作区workspace_ref只指执行文件目录引用。
- 任何资源操作由服务端重新授权；supported能力不构成永久授权。
- 客户端缓存、推送和界面状态都不是业务权威。
- 工具批准、预算增加和连接授权分开表示；一次批准不构成长期授权。
- 不确定启动先查原request_id；不自动重发危险操作；取消不等于进程停止或退款。
- 旧Happy能力逐项保留追踪；不可用提示不等于完成原交互保留验收。

### 1.2 产品目标与非目标

核心路径：登录与空间 → 发起Agent任务 → 跟踪 → 审批 → 拿到成果 → 离开后恢复。知识库是可选资源，Coding是可选执行能力。首版不做手机长驻worker、全量桌面管理后台、任意shell RPC、自动商店付款、E2EE托管承诺或公共个人节点发布。

## 2. 总体结构与模块责任

```text
Expo Router 路由（只做参数/权限门和页面入口）
  ↓
Screen + 原生组件（props/commands驱动，不直接fetch）
  ↓
Controller / ViewModel（页面编排、状态投影、恢复）
  ↓
contracts / domain / api-client（跨端共享纯TS）
  ↓
NativeHost（网络、凭证、持久化、文件、通知、语音端口）
  ↓
WeKnora产品API → 当前身份/空间/资源授权 → 商业准入 → 既有Run
  ├ 平台Agent worker
  ├ 受控Paseo Adapter → 执行节点
  ├ ActionService → 私网open-connector
  └ 事件/产物/通知Outbox/结算Outbox
```

| 模块 | 责任 | 禁止依赖 |
|---|---|---|
| contracts | wire DTO、运行时校验、版本协商；未知状态保留但禁用控制 | DOM、原生模块、应用状态 |
| domain/mobile | 请求状态机、scope规则、事件投影、交互可执行性 | fetch、SecureStore、Happy全局store |
| api-client/mobile | 真实路径、body、envelope与错误分类 | UI、原生页面、任意动态RPC |
| platform | NativeHost装配、网络与本地事务、前后台生命周期 | 页面具体业务文案 |
| workbench/conversations | 页面controller、聚合选择器、命令编排 | OC管理token、节点管理RPC |
| ui | 令牌、Button、Sheet、Card、StateBanner、Input、Tabs | 真实接口/预算核算 |
| Go facade | ownership入口与安全读模型；复用既有service/repository | 在handler里复制执行循环/商业账本 |

建议新增目录沿现有 `apps/mobile/sources/weknora` 局部扩展，精确任务文件见 [任务索引](../plans/task-index.json)。不能按本稿目录树整体搬迁现有工程。

### 2.1 版本和代码复用策略

首先校准现有Expo55系列与其React/RN/renderer解析结果；以根pnpm版本和锁文件为唯一安装入口。官方SDK55矩阵参见[E01](../references/verified-sources.md)。升级另设分支，验证原生模块、双平台构建与运行，不与产品协议改造混在同一提交。禁止照搬包含删除原生目录的prebuild脚本。

Web复用DTO、parser、错误、数据选择器、Reducer和业务表单校验；不能将 `packages/views` 的HTML、window/localStorage或DOM弹窗导入原生。样式共用语义令牌，Web转CSS、RN转数值主题；不要把CSS px字符串作为原生尺寸。

## 3. 身份、空间和缓存

### 3.1 冷启动顺序

1. 从已信任部署配置获得origin；不得被未认证深链任意替换。
2. 读取SecureStore凭证；没有凭证进入M01。
3. 使用产品auth校验/单飞刷新；读取profile、memberships和当前空间能力。
4. 用户仍有原空间权限则恢复；否则清空旧敏感投影并进入M02。
5. 构造ScopeKey和新的generation，初始化QueryClient/SQLite namespace、controller。
6. 对账未确认提交，读取当前Run快照；界面显示本地缓存时间，再同步服务端事实。

Token恢复成功不等于身份与Tenant恢复成功。OIDC回跳一次性交换后仍执行第3–6步，不直接跳到有旧数据的首页。

### 3.2 Scope隔离

`ScopeKey = canonicalOrigin + userId + tenantId`。URL标准化必须保留协议/端口并拒绝不可信origin。序列化使用确定性元组，不靠无转义的字符串拼接。每次身份或空间切换使generation加一，旧请求signal abort；收到结果时再次比较generation。

切空间执行：先禁用mutation → generation前移 → 停旧stream/语音 → 清当前可见数据 → 选择新QueryClient与存储namespace → 拉最新能力 → 重新允许操作。服务端Run保持存活；旧空间草稿仅在返回且仍获授权时可恢复。退出同时撤销设备绑定并清凭证；失败撤销由服务端会话失效与短期绑定策略补偿，不能让新账号看到旧推送正文。

### 3.3 HTTP认证错误

401只触发一次单飞刷新；所有等待请求共享同一次结果。写回凭证前比较generation。403代表当前权限不允许，不循环刷新。404统一显示不存在或无访问权限，避免泄漏对象标题。用户退后台或断网不是取消Run的理由。

## 4. 会话与任务准入

会话承载用户消息；Run承载一次可追踪执行；request_id标识一次用户提交意图；attempt_id标识一次执行尝试。一个会话可有多次Run，不把最新一条消息当作全部执行状态。

### 4.1 新建任务数据准备

M05包含：Agent、任务文本、可选知识、会话附件、target/workspace、预算上限。当前已读StartInput只有7个字段，不能将界面全部字段直接扩进body。先用已核验的会话配置与附件关联接口完成准备；不存在对应入口时通过B类版本化方案补齐，再调用start。服务端基于冻结的Agent版本、资源与目标进行最终授权，客户端选择只表达意图。

预算输入的UI默认值100是演示，不是产品定价。正式输入范围来自部署limits。零预算的可用语义必须与后端准入对齐；HTML采用正整数以避免误解，不能将其当作当前API新增限制。费用显示包括单位、估算/预占/最终状态，不将Credits写成货币余额。

### 4.2 提交状态机

| 本地状态 | 触发/行为 | 后续 |
|---|---|---|
| draft | 离线编辑，仅保存本地 | 用户在线确认提交 |
| prepared | 校验输入；生成request_id与canonical hash；落盘成功 | sending |
| sending | 发送一次HTTP；当前页面按钮禁用 | accepted / rejected / uncertain |
| uncertain | 连接中断或ACK丢失，可能已被受理 | lookup原request_id |
| accepted | ACK/lookup给出run_id，持久绑定 | snapshot与SSE |
| rejected | 服务端明确拒绝、未准入 | 显示原因；用户修改产生新意图 |

本地Prepared必须在网络前持久化；磁盘满时不发送。相同request_id与不同输入必须冲突；同一请求重放返回原结果。hash材料包含版本化、有序规范化的业务字段；不能把业务数组任意排序。受理时服务端冻结资源引用与权限版本，后续客户端改变选项不改变已受理Run。

ACK不明时禁用“再启动一次”；提供“核实原请求”。lookup的unknown不是失败。自动重试仅对明确无副作用读取或后端证明幂等的原请求，并受次数/退避限制。用户编辑后明确新建是不同意图，不可暗中重用旧request_id。

### 4.3 三类状态展示

RunStatus控制任务进度；ExecutionStatus表示执行节点观察；SettlementStatus表示费用最终性。UI将原始值与显示文案分开，未知枚举显示“等待同步”并禁用控制，而非默认失败。

例：产品已取消、执行停止待确认、费用待对账可以同时存在。M08固定展示三行和最后同步时间；M07只显示主要摘要，详情可展开。任务完成后仍允许查看已授权成果，结算待确认不阻塞纯读取。

## 5. 事件协议与恢复

### 5.1 产品SSE

目标协议使用完整 `ExecutionEvent` 放入data：schema_version/run_id/attempt_id/seq/type/occurred_at/payload；SSE id与seq严格一致，event与type一致。事件seq为正安全整数，快照watermark允许0。旧聊天流保留兼容路径；只有在版本协商后启用新版，不能直接破坏旧客户端。

G01闭环验收：Go writer输出实际bytes → TS真实parser → domain投影 → 原生存储adapter → 重开后还原。逐层独立绿不能替代这一用例。

心跳、认证失效、cursor_expired属于控制帧，不进入业务Reducer、不推进业务cursor；不能伪造attempt_id或时间补齐残缺payload。断流和EOF交给恢复controller，不将它们转为Run失败。UTF8字节和CRLF可以跨chunk；多行data先拼接再JSON解析。

### 5.2 一致快照与重放

服务端快照在同一可证明一致的读取边界给出watermark及对应状态；客户端事务替换投影/cursor后仅消费seq更大的事件。重复seq按稳定ID幂等，缺口停止提交并请求恢复。过期cursor取新快照而不是从0重复附加文本。终态需要drain到终态watermark之后才关闭流，避免丢失最后一个成果事件。

### 5.3 本地数据库（建议结构，不是生产迁移）

```sql
CREATE TABLE IF NOT EXISTS execution_events (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
  envelope_version INTEGER NOT NULL, event_cipher TEXT NOT NULL,
  key_version INTEGER NOT NULL, PRIMARY KEY(scope_key, run_id, seq)
);
CREATE TABLE IF NOT EXISTS execution_cursors (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
  PRIMARY KEY(scope_key, run_id)
);
CREATE TABLE IF NOT EXISTS execution_projections (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, watermark INTEGER NOT NULL,
  projection_cipher TEXT NOT NULL, PRIMARY KEY(scope_key, run_id)
);
CREATE TABLE IF NOT EXISTS pending_submissions (
  scope_key TEXT NOT NULL, request_id TEXT NOT NULL, input_hash TEXT NOT NULL,
  input_cipher TEXT NOT NULL, state TEXT NOT NULL, run_id TEXT,
  created_at TEXT NOT NULL, PRIMARY KEY(scope_key, request_id)
);
```

此库仅是可重建客户端缓存，不增加后端权威mobile_tasks。草稿单独按scope/session保存，退出清理按策略处理。缓存schema有版本，迁移失败可保留尚未确认提交的安全恢复材料，再重建非权威投影，不直接清空所有内容。

### 5.4 事务与加密

Expo的事务API返回Promise<void>，普通异步事务不是独占；真实适配必须使用txn执行语句，显式捕获结果。出处[E02](../references/verified-sources.md)。每个DB写入经串行队列；短事务提交事件、投影和cursor。加密密钥初始化single-flight，在写事务前完成；标准AEAD实现、随机nonce、AAD绑定scope/run/seq/keyVersion，禁止自制算法。

`database locked`可有限退避；磁盘满不能提交cursor；密钥丢失不得将密文当明文展示；AAD失败拒绝读取并重新取已授权快照。加密缓存不代表服务端E2EE。批量读取代替逐seq发起数千次数据库查询，分页与索引按长会话基准验证。

## 6. 审批与安全命令

### 6.1 类型化交互

- tool：动作、连接别名、目标、完整内容/参数摘要、风险、有效期、digest、revision。
- budget：当前授权上限、已用/预占、请求新上限、作用Run预算树、可批准角色。
- question：提示、选项或输入schema、必填/长度限制；不是批准工具操作。
- connection：Provider和scopes、个人/空间归属；通过系统浏览器授权后查询产品连接状态。

详情来自当前服务端读取，不以通知正文作为同意对象。M09先查看目标与内容，再打开一次性确认Sheet；打开时捕获scope generation/revision/digest。点击时再次比较；服务端检查当前授权、截止时间和同一冻结对象，不信任客户端风险标签。

### 6.2 审批事务与外部派发

服务端在同一一致性边界验证owner/tenant、交互pending、revision、digest、有效期与连接授权版本；CAS只允许合法决定落地一次。决策request_id用于重复回执恢复。决定成功不等于外部派发完成：创建或唤醒既有受控工作队列，客户端刷新Run。

并发409展示“已在其他设备处理”，重新获取详情，不静默重发。403清除敏感内容；过期先重新Prepare，不能沿用旧确认。预算增加不创建第二份余额；外部写入成功但本地结算失败只补持久化/结算，不重复Provider请求。

cancel与steer是封闭命令union，使用最新revision；无revision禁用按钮。Cancel按钮文案是“申请取消任务”，接受后显示“停止待确认”；只在可靠节点观察后显示停止。Steer只允许当前driver声明支持，不能用任意shell代替。

## 7. 工作台与资源读模型

首页使用薄overview聚合，按当前actor授权返回counts、in_progress、pending_interactions、recent_artifacts、as_of；不为每张卡逐个会话请求造成N+1。服务端必须过滤每种资源，不只检查相同tenant。卡片摘要不暴露具体审批正文。

会话列表使用稳定分页，查询键包括scope、关键词、筛选、版本。搜索请求防抖约300ms（设计值），取消旧signal；结果比较generation。cursor绑定actor/tenant/filter/order/as_of，不能复用于另一个空间。列表重排保留当前滚动锚点；空态与无权限态分开。

知识、成果和连接复用各自既有权威资源与权限入口；底部“资源”只提供移动轻管理。将Web配置页改成手机新页面前，先抽取验证/分组纯函数，不把整个DOM页面引入原生。

## 8. 附件、预览与分享

上传意图分为本次任务附件与知识入库；默认仅当前会话，用户显式选择才永久入库。nativeFile本地URI由原生transport变为真实multipart bytes；后端不能把file://或content://当可读取URL。

上传状态：selected → uploading → verifying/scanning → ready；失败/取消不丢文字草稿。提交前确保会话附件ready且scope不变。服务端检查真正大小/MIME/摘要/归属与扫描结果；客户端限制只为体验。大文件采用部署支持的分段/流式方案，不能默认整文件base64常驻内存。

产物固定artifact_id/version/mime/size/source_run；远程文件显式导入为不可变版本。读取、预览、下载、外部分享分别授权。复杂HTML不直接在有登录态的WebView运行；优先静态化或严格隔离的无凭证预览。签名链接到期需要重新授权，不能自动延长已撤权权限。

## 9. 通知与深链

Run事务同时写通知intent；投递worker按event/user/device/environment去重、退避和保存回执。锁屏只含最小提示及opaque资源ID，无Prompt、连接密钥和审批正文。设备账号切换使用revision；过期token撤销，拒绝通知权限不阻止使用。

点击深链：受信origin → 认证 → memberships → 跨空间显式确认 → 当前资源授权 → 页面。URI不得携带批准动作；收件箱已读不会变更业务审批。App恢复从产品inbox/snapshot重建，不能依赖推送必达或后台JS常驻。

## 10. 远程、连接器、语音与用量

Paseo先开放经过真实验证的托管目标；客户端只提交服务端target/workspace引用。Bridge使用服务身份和Run绑定/租约/fence；source-event必须在写入前校验，不能先持久化再404。未知启动有命令日志和对账，不自动重新创建进程。个人节点和完整终端为独立profile。

OC保持私网共享运行时，产品ActionService是审批、商业准入和审计唯一入口。手机不能获得管理token、原始凭据或任意Provider Proxy。T18历史passed含blocked-env子项，不能代表当前写入/计费可发布。页面能力由本次部署已验证profile决定。

听写先录音→转写→可编辑文本→放入会话草稿→用户发送。停止播报、结束语音、取消任务是三个动作。实时语音以短期媒体令牌和独立预算/结算绑定Run；不因语音输入而豁免工具确认。原型不调用麦克风。

M18只显示当前空间用量读模型、as_of、单位和结算最终性，不实现充值。账单角色独立于普通Admin；金额/订单采用服务端权威值；BYOK是否免模型用量与平台服务费用分开。HTML数字仅作排版示例。

## 11. 性能、可访问性与观测

以下是验收目标，不是本次真实产品指标。原生发布构建指定中档iOS/Android设备、后台版本和数据集；列表首次可交互、长消息滚动、恢复与内存单独记录，不能只测开发构建。

| 场景 | 设计门槛 |
|---|---|
| 触控 | 普通主操作目标48dp；图标44dp；相邻危险按钮分离 |
| 文本 | 正文16、辅助14，支持系统字体放大；不靠缩小字体挤信息 |
| 长会话 | 1000条消息、连续增量文本，虚拟列表稳定ID；用户上翻不强制到底 |
| 动效 | reduced-motion尊重系统；装饰动效不承担业务进度 |
| 读取 | 20项分页，最大50；合理批量预取，先快照再订阅 |
| 恢复 | 弱网/切网/杀进程后对账同一请求，不能重复外部副作用 |
| 无障碍 | 明暗配色对比；屏幕阅读器标签、动态文字、焦点顺序、表单错误与对话框验证 |

对比门槛参考W3C[E03/E04](../references/verified-sources.md)，单独的令牌检查不等于整App符合全部WCAG条款。原型检查范围另见verification。

观测关联request_id/run_id/attempt_id/tenant_id/trace_id；日志脱敏。指标：准入ACK延迟、unknown待对账年龄、cursor恢复、审批冲突、取消待确认时长、通知回执、结算待对账。记录read/control/provider/settlement失败原因，不用统一“网络错误”掩盖业务冲突。

## 12. 发布与回滚

按core/oidc/resources/connectors/remote/dictation/voice/personal_node/full_happy分别判门禁。MX是增量细化，dictation属于本轮W29切片，不重定义原W发布规则。所有原W条件依赖须取闭包；当前HTML支持的模拟能力不自动解锁部署开关。

升级先关新准入，保留已有Run观察、停止、清理和迟到用量结算；客户端兼容窗口用protocol/capability而非硬编码客户端版本。数据库迁移在PG/SQLite两序列重新分配编号，不覆盖旧预留。原生模块变化需要新build；不当纯JS更新推给不匹配运行时。回滚优先关闭新能力而非破坏性down数据。

完整验收由MX-036聚合明确profile证据：静态/单元、Go与数据库、原生组件、iOS/Android E2E、真实外部服务、安全故障、可访问性及升级。任何skip/blocked-env不记为已通过。
