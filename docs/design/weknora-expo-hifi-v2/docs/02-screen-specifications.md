# 18页详细交互规格与组件映射

日期：2026-09-17。页面编号M01–M18沿用输入线框。路由为建议的Expo Router映射，不代表当前仓库路径已经存在。所有页面使用[设计系统](03-design-system.md)，业务模块规范见[详细设计](01-detailed-design.md)。

## 1. 路由与导航

根Tab：M03工作台、M04会话、M11资源、M17我的。二级详情隐藏底部Tab并显示返回；保留用户回到原列表位置。M05是创建流程，Agent/目标/知识选择后回到原草稿，不再重新创建整个表单。iOS手势返回/Android物理返回遵循同一未保存/录音中确认规则。

参考映射：`(product)/(tabs)/index.tsx`、`sessions.tsx`、`resources.tsx`、`profile.tsx`；`(product)/new-task.tsx`；`(product)/sessions/[sessionId].tsx`；`executions/[runId].tsx`；`interactions/[interactionId].tsx`。实施时复用现有路由组织，MX-009登记所有适用路径，不强制迁移已有根目录。

## 2. 通用页面状态

| 状态 | 规则 | 固定文案 |
|---|---|---|
| loading | 首次加载骨架，无假业务计数；保留导航 | 正在同步当前空间 |
| stale | 缓存只读摘要，显示更新时间 | 当前显示上次同步结果 |
| offline | 允许编辑草稿；禁用危险mutation | 当前离线，草稿已保留 |
| empty | 只显示当前scope空态，不借用另一空间数据 | 这里还没有任务 |
| error | 保留可恢复草稿；显式读取重试 | 暂时无法连接，稍后重试 |
| forbidden | 不展示敏感正文/目标名称 | 无法访问这项资源 |
| conflict | 只读当前冲突提示，不重新发旧决定 | 内容已在另一端改变，请刷新 |
| uncertain | 保留request身份；提供lookup | 正在核实原请求，请勿重复提交 |

原型顶部“正常/离线/冲突/无权限/启动未知/加载/空/错误”用于验收说明。不是生产功能开关；原生App需要由真实网络、权限和状态驱动。

## M01 · 登录

**目的：** 以 WeKnora 产品身份开始  
**实现责任：** MX-010；原型文件：[`M01-login.html`](../pages/M01-login.html)。

### 布局与字段
邮箱/密码输入、产品说明、主登录、企业SSO、可信服务器入口。不要在用户首次进入时显示整个运维配置。

字段清单：邮箱、密码、企业 SSO、可信服务器。

### 数据契约与校验
产品 auth；OIDC SDK 路径沿原方案复验。邮箱去空白后校验格式；密码不写任何普通缓存；提交中按钮忙碌并保持表单；401显示通用登录失败，不暴露账号是否存在。SSO拒绝/用户取消回到M01而非空白页。

### 用户操作与反馈
输入邮箱密码 → 产品auth → bootstrap → M02或M03；SSO → 系统浏览器PKCE → 一次性交换 → 同一bootstrap。原型按钮只进入演示。

### 需要独立验收的状态
无凭证 / 提交中 / 登录失败 / SSO取消 / 服务器不可信。

### 组件与接线
ProductLoginScreen、FormField、Button、TrustNotice、AuthController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证邮箱、密码仅存在临时表单；登录后拉 memberships；不保存到原型缓存。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M02 · 选择空间

**目的：** 将工作与个人空间清晰分开  
**实现责任：** MX-011；原型文件：[`M02-spaces.html`](../pages/M02-spaces.html)。

### 布局与字段
当前用户身份、个人与团队空间卡、角色标签、当前选择。空间切换是明确操作，不将不同空间余额合并。

字段清单：空间名称、角色、当前标识、独立计费说明。

### 数据契约与校验
bootstrap / memberships（聚合为提议）。列表来自memberships；无空间展示创建/申请入口能力，不默认给owner。成员被移除时去掉选择项，旧缓存不可见。

### 用户操作与反馈
选择空间 → 推进generation → 关闭旧请求和流 → 恢复新scope草稿 → M03；业务Run不被取消。

### 需要独立验收的状态
空memberships / 已移除成员 / 切换中 / 离线缓存仅可读。

### 组件与接线
SpacePicker、SpaceCard、RoleBadge、ScopeController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证先禁写、推进 generation、清可见态再切空间；已有任务不取消。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M03 · 工作台

**目的：** 今天，交给 Agent 做点什么  
**实现责任：** MX-013；原型文件：[`M03-home.html`](../pages/M03-home.html)。

### 布局与字段
顶部空间和收件箱；浅绿任务入口Hero；运行/待处理/完成摘要；需你处理；进行中；最近成果。重要审批可在首屏看到，次要内容纵向滚动。

字段清单：当前空间、任务计数、待处理、进行中、最近成果。

### 数据契约与校验
GET /workbench/overview（提议）。计数含as_of；首次无数据用骨架，后台刷新保留旧内容并显示更新时间。只展示当前用户有权看到的摘要。

### 用户操作与反馈
新建→M05；待处理→M09；执行卡→M08；成果→M14；通知→M10；空间→M02。

### 需要独立验收的状态
无任务 / 加载 / 后台刷新失败 / 离线只读 / 资源被撤权。

### 组件与接线
WorkbenchScreen、TaskHero、MetricStrip、ApprovalCard、RunCard、ArtifactCard。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证我的任务，不等于全空间所有任务；聚合由后端授权后生成。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M04 · 会话

**目的：** 每一次协作，都可以继续  
**实现责任：** MX-014；原型文件：[`M04-sessions.html`](../pages/M04-sessions.html)。

### 布局与字段
搜索框、状态筛选、按时间分组的会话行、Agent标识与最新执行摘要。

字段清单：搜索、状态筛选、会话名称、Agent、最近更新。

### 数据契约与校验
复用会话列表；缺聚合时新增薄读模型。关键词防抖约300ms；query key含scope和filter；分页20项max50；稳定排序与去重。空搜索展示“没有匹配的会话”，不提示用户删除筛选外记录。

### 用户操作与反馈
点击会话→M07；新建→M05；切筛选重新取首页；下拉刷新保持草稿不变。

### 需要独立验收的状态
列表为空 / 搜索无结果 / 下一页失败可重试 / 离线缓存 / 长标题。

### 组件与接线
SessionListScreen、SearchField、FilterChips、VirtualSessionList、SessionRow。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证查询键带 origin/user/tenant；过滤按服务端分页，不逐会话 N+1。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M05 · 新建任务

**目的：** 说清目标，其他交给 Agent  
**实现责任：** MX-015；原型文件：[`M05-new.html`](../pages/M05-new.html)。

### 布局与字段
Agent卡、16号正文多行任务描述、附件与可选知识、执行目标、预算上限；底部固定提交操作。键盘弹起时输入区可滚动，操作区避开键盘。

字段清单：Agent、任务描述、附件、知识范围、执行目标、预算上限。

### 数据契约与校验
POST /workbench/executions（已读）；资源配置另接口。描述非空且长度来自limits；预算整数范围取部署能力；附件未ready不提交。一次意图生成一次request_id，预提交落盘后才发网络。当前StartInput不包含attachment/model字段，先做已核验会话准备。

### 用户操作与反馈
换Agent→M06；知识→选择Sheet；目标→M15；提交→原请求受理→M07；ACK未知→M08查询原请求。

### 需要独立验收的状态
草稿 / 上传未完成 / 超限 / 权益不足 / 离线保存 / 启动未知 / 被拒绝。

### 组件与接线
NewTaskScreen、TaskComposer、ResourcePills、BudgetField、TargetRow、SubmitController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证request_id 在网络前持久化；knowledge/attachments 不私自添加到已有 start body。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M06 · 选择 Agent

**目的：** 为这项工作找一个合适的助手  
**实现责任：** MX-016；原型文件：[`M06-agents.html`](../pages/M06-agents.html)。

### 布局与字段
通用、研究、写作与编码分类，Agent说明、可用资源摘要和能力不可用原因。

字段清单：Agent 名称、类别、能力标签、可用状态。

### 数据契约与校验
空间可见 Agent 目录；能力取服务端。Agent来自当前空间发布目录；不可用不构造成disabled但无解释。选择不是授权证明，提交再校验版本。

### 用户操作与反馈
选择→保存当前草稿Agent→返回M05；未就绪Coding展示说明，不自动切任意节点。

### 需要独立验收的状态
加载 / 空目录 / 搜索无结果 / forbidden / unavailable / Agent版本改变。

### 组件与接线
AgentPicker、AgentCard、SearchField、CapabilityReason。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证不可用 coding 能力保留原因；无权 Agent 不暴露敏感配置。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M07 · 对话

**目的：** 消息、证据与执行放在一起  
**实现责任：** MX-017；原型文件：[`M07-conversation.html`](../pages/M07-conversation.html)。

### 布局与字段
上部标题与执行状态；用户消息、Agent段落、引用、工具卡、待审批卡、成果卡；底部附件/语音/编辑器/发送。

字段清单：角色消息、引用、工具活动、审批卡、成果卡、输入框。

### 数据契约与校验
snapshot / events；SSE envelope 先修复 G01。消息按稳定ID投影，重复seq不追加；未知事件安全展示；用户上翻时不强制自动到底。无Happy登录也能展示产品数据。

### 用户操作与反馈
发送补充→steer或下一Run由已发布能力决定；详情→M08；审批→M09；引用→证据Sheet/M12；成果→M14；语音→M16。

### 需要独立验收的状态
流式 / 等待用户 / 已离线 / 正在重连 / 同步已完成 / 发送失败保留草稿 / 能力不可用。

### 组件与接线
ProductConversationScreen、MessageList、ToolCard、ReferenceChip、Composer、ConversationController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证渲染不依赖 Happy 全局会话；用户上滑后不强制滚到底。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M08 · 执行详情

**目的：** 任务、进程与费用，各自说明  
**实现责任：** MX-018；原型文件：[`M08-run.html`](../pages/M08-run.html)。

### 布局与字段
三状态清单、目标、预算与已用说明、事件时间线、request_id、最近同步、成果与取消。

字段清单：产品状态、执行观察、结算状态、request_id、预算、时间线。

### 数据契约与校验
get / snapshot / lookup / commands（已读）。保留run/execution/settlement三个字段；无最新revision禁用cancel/steer。未知进程状态不可被UI推断成停止。

### 用户操作与反馈
返回会话→M07；查看审批→M09；结果→M14；取消→明确后果确认Sheet→等待停止；查询原请求不重发start。

### 需要独立验收的状态
排队 / 运行 / waiting_user / unknown / 取消请求已受理 / 停止确认 / 结算延迟。

### 组件与接线
ExecutionScreen、StateTriplet、Timeline、BudgetSummary、CancelSheet。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证申请取消不等于停止已确认，unknown 不允许新请求重发。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M09 · 操作审批

**目的：** 先看清影响，再决定一次  
**实现责任：** MX-019；原型文件：[`M09-approval.html`](../pages/M09-approval.html)。

### 布局与字段
风险提示、明确动作、账号别名、目标、参数/内容、有效期、版本；底部拒绝/批准这一次。

字段清单：动作、连接账号、目标、内容摘要、风险、有效期、revision。

### 数据契约与校验
interactions / decisions（路径已读，body 为提议）。数据来自当前interaction详情，不取通知正文。确认Sheet捕获revision/digest/generation；变更即关闭确认或要求刷新。403遮蔽内容。

### 用户操作与反馈
批准→确认当前目标/内容→decision幂等提交→刷新Run；拒绝同样记录当前对象；409刷新详情后重新决定。

### 需要独立验收的状态
pending / 提交中 / 已在另一端处理 / expired / forbidden / 离线 / 已批准但外部执行未完成。

### 组件与接线
InteractionScreen、RiskNotice、FrozenActionDetails、DecisionSheet、InteractionController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证工具批准、预算增加、连接授权分开；确认时重新校验 revision。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M10 · 收件箱

**目的：** 只把需要关注的事情带给你  
**实现责任：** MX-021；原型文件：[`M10-inbox.html`](../pages/M10-inbox.html)。

### 布局与字段
未读提示、按待处理/全部等过滤的消息行，类型图标和时间。

字段清单：类别、未读、事项摘要、空间、更新时间。

### 数据契约与校验
GET /workbench/inbox（提议）。badge来自服务端读模型；已读与审批分离；列表摘要最小化，撤权时不展示敏感标题。

### 用户操作与反馈
点击→恢复认证→必要跨空间确认→重新授权→M09/M08/M14；标记已读不触发任务。

### 需要独立验收的状态
拒绝通知权限但收件箱可用 / 空 / 过期链接 / 跨空间 / 已删除资源 / 弱网。

### 组件与接线
InboxScreen、NotificationRow、DeepLinkController、UnreadBadge。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证通知只是提示；回到详情重新授权，不能从通知直接批准。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M11 · 资源

**目的：** 为你的 Agent 准备好工具  
**实现责任：** MX-022；原型文件：[`M11-resources.html`](../pages/M11-resources.html)。

### 布局与字段
Agent、知识、连接器、成果、执行目标按业务分类进入；搜索和轻量筛选。

字段清单：搜索、分类、资源计数、知识、连接、成果。

### 数据契约与校验
沿用知识 / Agent / 连接 / 产物 API。不将Web管理菜单全部缩小搬入；每类资源权限独立；个人空间为空时不能显示团队示例。

### 用户操作与反馈
知识→M12；连接→M13；成果→M14；Agent→M06；远程→M15。

### 需要独立验收的状态
分类无资源 / 无权限 / 能力未开启 / 搜索无结果。

### 组件与接线
ResourcesScreen、ResourceCategory、ResourceRow、ResourceSelector。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证只读消费与轻管理；能力未验收不开放外部写入。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M12 · 知识详情

**目的：** 答案要有依据  
**实现责任：** MX-022；原型文件：[`M12-knowledge.html`](../pages/M12-knowledge.html)。

### 布局与字段
知识库摘要、文档数、索引状态、来源与更新、可读引用片段、用此知识提问。

字段清单：知识库、可用文档、更新时间、文档行、引用片段。

### 数据契约与校验
沿用知识库、文档、引用 API。关联知识仅表达本次任务使用；索引未就绪提示而非假定可检索；证据片段重新授权。

### 用户操作与反馈
用此知识→保留M05草稿并添加resource引用；文档→安全预览；引用Sheet返回原会话。

### 需要独立验收的状态
索引中 / 部分失败 / 无文档 / 访问撤销 / 文档过期。

### 组件与接线
KnowledgeScreen、DocumentRow、IndexStatus、CitationPreview。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证知识不是必选；引用访问重新授权；索引未就绪不参与检索。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M13 · 连接详情

**目的：** 明确连接到哪里、允许做什么  
**实现责任：** MX-023；原型文件：[`M13-connector.html`](../pages/M13-connector.html)。

### 布局与字段
Provider、个人/空间归属、连接状态、允许动作、风险说明、重新授权/撤销。

字段清单：Provider、个人/空间连接、账号、授权范围、动作能力、状态。

### 数据契约与校验
产品 Connection / ActionService，非 OC 管理接口。只显示产品Connection，不展示token/client secret。授权URL来源受信后端；撤销前明确外部已执行操作不会回滚。

### 用户操作与反馈
重新授权→系统浏览器→查询产品连接状态；撤销→二次确认→等待版本更新。原型全模拟。

### 需要独立验收的状态
可用 / 授权过期 / 重连中 / 权限撤销 / provider不可用 / 外写门禁未过。

### 组件与接线
ConnectionScreen、ScopeBadge、ActionAllowlist、RevokeSheet。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证一项连接一个凭据管理方；撤销不回滚外部副作用。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M14 · 任务成果

**目的：** 把结果变成可带走的工作  
**实现责任：** MX-024；原型文件：[`M14-artifact.html`](../pages/M14-artifact.html)。

### 布局与字段
文件类型、名称、不可变版本、来源Run、预览、大小、更新时间；底部下载/分享。

字段清单：文件名、版本、来源 Run、正文预览、大小、访问范围。

### 数据契约与校验
受授权 artifact ID + version / 短期下载。下载与分享分别授权；缓存只读并标记过期，复杂HTML不得带产品登录态执行。示例Markdown下载明确标注模拟。

### 用户操作与反馈
预览→安全renderer；下载→授权短链/native file；系统分享→再授权→native share；返回Run。

### 需要独立验收的状态
预览加载 / 格式不支持 / 签名过期 / 撤权 / 远程导入未完成 / 离线。

### 组件与接线
ArtifactScreen、ArtifactHeader、SafePreview、DownloadController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证模拟 Markdown 可本地下载；正式版签名 URL 不进持久缓存。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M15 · 执行目标

**目的：** 让任务在合适的地方执行  
**实现责任：** MX-026；原型文件：[`M15-targets.html`](../pages/M15-targets.html)。

### 布局与字段
平台默认、托管目标、个人节点、在线观察、能力与不可用原因；工作目录为服务端引用。

字段清单：平台目标、托管节点、个人节点、状态、支持能力、工作目录引用。

### 数据契约与校验
execution-targets / execution-workspaces（已读）。可见不等于可执行；提交前重新校验target、workspace、driver和成员权限；用户不可输入任意daemon URL。

### 用户操作与反馈
选择支持目标→返回M05；不支持→原因Sheet；个人节点入口留待对应profile。

### 需要独立验收的状态
离线节点 / driver不兼容 / 权限不足 / 工作区被占用 / 未验收。

### 组件与接线
TargetPicker、TargetCard、CapabilityList、TargetSelector。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证手机只选受权引用；无任意 daemon URL 或 shell RPC。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M16 · 语音输入

**目的：** 先说出来，再确认发送  
**实现责任：** MX-028；原型文件：[`M16-voice.html`](../pages/M16-voice.html)。

### 布局与字段
按住/点击录音控制、状态提示、转写结果可编辑、确认放入草稿。

字段清单：录音/转写状态、音频指示、可编辑转写、隐私说明。

### 数据契约与校验
按 W29 转写端口；实时语音另门禁。原生录音拒权不阻塞文字；取消录音不取消任务；最终采用当前文本而非旧转写回调。原型音波只装饰，无真实音频。

### 用户操作与反馈
录音→停止→转写→编辑→放入M07草稿；用户再次点发送才产生业务请求。

### 需要独立验收的状态
麦克风拒权 / 空转写 / 用户取消 / 离线 / 转写失败 / 编辑后确认。

### 组件与接线
DictationScreen、RecordControl、TranscriptEditor、DictationController。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证当前不请求麦克风；演示转写可编辑，发送使用最新文本。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M17 · 我的

**目的：** 工作方式，由你决定  
**实现责任：** MX-030；原型文件：[`M17-profile.html`](../pages/M17-profile.html)。

### 布局与字段
用户/空间/角色卡、空间用量、通知开关、外观、隐私与退出。

字段清单：用户、空间、角色、主题、通知、隐私、退出。

### 数据契约与校验
产品用户、scope、偏好和设备绑定。账单角色独立校验；偏好不覆盖服务器权限。退出清除本地凭证和当前可见数据，撤销设备绑定。

### 用户操作与反馈
空间→M02；用量→M18；外观切主题；退出确认→M01。

### 需要独立验收的状态
未登录 / 只读成员 / 通知拒权 / 离线退出后待撤销绑定。

### 组件与接线
ProfileScreen、PreferenceRow、ThemeSwitch、LogoutSheet。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证账单管理员单独授权；注销清凭证与本地敏感缓存。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## M18 · 空间用量

**目的：** 知道用在哪里，也知道边界  
**实现责任：** MX-031；原型文件：[`M18-usage.html`](../pages/M18-usage.html)。

### 布局与字段
当前空间、可用额度/预占/待结算/已结算、分项消耗、统计区间、最终性解释和权限提示。

字段清单：周期、可用额度、预占、已结算、用量构成、最近任务。

### 数据契约与校验
商业域读取适配；不创建新钱包。只读商业域返回值；不在手机计算钱包。数字带as_of、单位；无账单权限隐藏敏感明细。示例数字不能当定价。

### 用户操作与反馈
返回M17；账单权限说明；不包含付款/充值/商店购买按钮。

### 需要独立验收的状态
加载 / 截止时间提示 / 结算未完成 / 账单权限不足 / 空消费。

### 组件与接线
UsageScreen、CreditSummary、UsageBreakdown、FinalityNotice。Screen只接收VM与commands；Host在composition root注入，禁止组件直接导入后端token。

### 该页完成标准
进入页面时得到当前scope的数据；执行上述每个按钮的正例及失败路径；验证余额、预占、最终消耗分别显示；本原型不含购买与支付。。至少在390与320宽度、明暗模式和字体放大下检查正文可读、底部操作可达，不能只提交静态截图。

## 3. 复用交互覆盖

### 3.1 Sheet与确认层

确认层必须包含动作标题、具体影响、当前冻结对象和两侧按钮；危险动作不使用默认回车立即确认。背景不可交互，焦点移入对话框；Esc/返回关闭且不提交，关闭后返回触发控件。内容超长时对话框内部滚动，操作始终可达。原生使用可访问性modal语义，Web用dialog角色与aria-modal；二者分别测试。

### 3.2 四类交互共用布局而非共用请求body

M09默认展示tool审批。MX-020补budget、question、connection分支：共享标题/状态/错误/操作布局，但分别使用金额表单、回答表单、系统授权浏览器。它们不扩充18页顶层导航，也不把所有分支压成approve布尔值。当前HTML仅完整演示tool审批，其他分支在详细设计和任务中定义，不能声称原型已经实现全部专业审批。

### 3.3 交互状态与跨端可见性

一份决定发生后，M03卡片、M07待处理、M08状态、M09详情与M10提示统一从同一交互ID更新。以服务端回执及重读为准，不乐观把外部动作标为成功。多设备并发由revision/CAS仲裁。原型状态仅用于演示这层联动，不模拟真实消息投递。

## 4. 原生专项验收

iOS/Android键盘避让、交互返回、系统字体200%、VoiceOver/TalkBack、Safe Area、旋转、平板、通知冷启动、照片/文件权限、低存储与后台恢复由MX-034/MX-035执行。浏览器125%字体或截图不能替代这些结果。终端/Diff/full_happy高级交互仍沿原清单补齐，不在18页原型覆盖之外暗中宣称完成。
