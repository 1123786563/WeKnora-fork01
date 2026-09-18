# 测试矩阵（分支 fix/miniprogram-qa-regression，2026-09-18）

页面共 20：4 主 Tab + 16 分包页（`src/core/routes.ts` ROUTES 与 `dist/app.json` 实测一致）。
验证层级缩写：**UT** 单元测试（tests/*.test.mjs 直测源码）；**ASM** 装配测试
（tests/assembly.test.mjs：真实 runtime/workbench/transport/auth + 契约级 Taro 替身）；
**CT** 契约对照（与 Go 路由/DTO）；**SIM** 微信开发者工具模拟器（既往提交 9e326603 与本轮补验）；
**RT** 真实后端联调；— 表示该层级本轮未执行（不宣称）。

设计参考：`docs/WeKnora-Miniapp-Implementation-Kit/WeKnora-Detailed-Design.md` 与
`docs/WeKnora-Taro-Design/02-页面与交互说明.md`（高保真 HTML 仅作视觉参考）。

## 页面与链路

| # | 页面（ROUTES key） | 真实 API | 状态覆盖（加载/空/错误/无权限/成功/禁用/提交中） | 层级证据 |
| --- | --- | --- | --- | --- |
| 1 | home 工作台 | auth/me、commercial/summary、execution-targets | Screen 骨架按 session.phase 分支（anonymous/loading/error/ready）；useData 覆盖加载/错误/空 | ASM（登录链/401/403）；SIM（未登录态渲染，9e326603） |
| 2 | tasks 任务 | `GET /workbench/executions`（无 Go 路由，404 诚实降级）；requests lookup；本机 recent-runs | 错误→降级提示；pending 意图卡（查询原请求）；空列表；按 ID 恢复 | ASM（D5 修复 + duplicate-taps + CT 路由表） |
| 3 | knowledge 知识 | knowledge-bases 列表 | 加载/错误/空/成功 | ASM（envelope 解析路径）；UT（views.rows/data） |
| 4 | me 我的 | commercial/summary；logout | 加载/错误/退出确认/提交中 | ASM（logout 清缓存+远端失败仍本地成功） |
| 5 | login 登录 | auth/login → me | 表单校验禁用、错误、提交中、stale 响应丢弃 | UT（auth 8 项 + D4 两项）；ASM 登录链；SIM（渲染+键盘输入，9e326603） |
| 6 | workspace 选择空间 | auth/switch-tenant | 切换中/失败回滚/幂等凭据轮换 | UT（switch 3 项）；ASM（SCOPE_CHANGED 丢弃） |
| 7 | agents Agent 列表 | execution-targets / agents | 加载/错误/空 | ASM（targets envelope） |
| 8 | agent 发起任务 | startTask（request_id 幂等） | 预算/目标能力禁用态、提交中、重复点击 | ASM（D5、duplicate-taps） |
| 9 | chat 对话 | knowledge-chat/agent-chat POST 流、sessions、attachments | 停止接收、中断提示、thinking 过滤、附件临时性、引用跳转 | ASM（chatStream SSE 逐字节）；UT（SSE 解析 mini-regression 7 项）；RT（—） |
| 10 | execution 执行详情 | snapshot + events GET 流、commands | waiting_user、追加指令（能力禁用）、取消确认、断流"已暂停" | ASM（watchExecution watermark 接续）；UT（execution 4 项） |
| 11 | approval 审批 | interactions + decisions | 缺动作详情→批准禁用；重复/跨类型拒绝；已处理态 | ASM（decisions 路径 + CT）；UT（intent decision id 复用） |
| 12 | artifact 产物 | chat artifacts（会话级） | 空态、无定位→不猜下载链接 | ASM（CT 路由） |
| 13 | kb 知识库详情 | knowledge-bases/:id | 加载/错误 | ASM（envelope） |
| 14 | upload 添加知识 | documents.upload（native multipart） | 20MiB 上限、进度、失败；不建重复条目 | mini-regression（FormData 不实例化 4 项）；UT（transport.sendMultipartFile） |
| 15 | document 文档与引用 | documents + openProtectedDocument | 扩展名白名单、授权下载、scope 校验 | UT（transport）；源码审查（files.ts 路径防穿越） |
| 16 | usage 用量与套餐 | commercial/summary | stale 提示、available/held/refund_locked 分列 | UT（formatCredits/formatMoney 大整数）；ASM（envelope） |
| 17 | checkout 确认订单 | —（渠道未接入） | 入口关闭 + 原因说明，不生成虚构报价 | 源码审查（CheckoutPage 静态关闭态） |
| 18 | order 订单与履约 | commercial/getOrder | 付款/履约双状态、按号查询、刷新 | UT（formatMoney fen 字符串不过浮点） |
| 19 | invitations 空间邀请 | accept-by-token | 凭证校验、确认、错误 | ASM（CT 路由） |
| 20 | states 状态说明 | —（publicPage） | 静态说明 | —（纯静态） |

## 非页面链路（平台层）

| 链路 | 覆盖 | 层级 |
| --- | --- | --- |
| Utf8 增量解码（中文/emoji 每字节拆分、代理对、截断/非法拒绝） | core.test.mjs | UT |
| SSE 解析（CRLF 跨 chunk、心跳、多行 data、逐字符拆分、不完整帧不派发） | core.test + mini-regression | UT |
| 原生传输（状态保真、头归一、abort 单次、流元数据未知、非 SSE 不透传、非 2xx 头先拒绝、chunk 协议缺失拒绝） | transport.test.mjs | UT |
| Scope 生命周期（切换中止旧请求、拒绝 stale commit、scopeKey 隔离） | core.test + assembly | UT/ASM |
| 401 单飞刷新 + GET 重放、写请求不自动重试、403 不进刷新、scope 变更丢弃 | assembly.test.mjs | ASM |
| 任务恢复（快照 watermark、seq 缺口拒绝、游标不因持久化失败前移、unknown 同 ID 重提交） | core.test + assembly | UT/ASM |
| 契约对照（发线路径 ⊆ Go 路由表；requests 列表端点缺席被钉住） | assembly.test.mjs | CT |

## 明确未验证（blocked-env / not-implemented）

- Android/iOS 真机：未执行（复验步骤见 report.md 第 7 节）。
- 真实后端带副作用的流式链路（聊天流式回答、执行事件流、上传解析、订单查询、审批决策）：
  未在本轮执行（会产生模型消耗/业务写操作，未获授权）。
- 微信支付、快捷登录、审批正向批准：not-implemented，入口关闭。

## 模拟器逐页实测（2026-09-18 下午轮，真实后端 + 真实登录会话）

环境变化记录：验证中途原 dev 栈容器 `WeKnora-app` 被外部进程移除（`.orb.local` DNS 随之失效），
曾导致 account 分包页请求挂起与会话丢失假象；切换到 Up 栈
（`https://up-weknora-app.orb.local`，parity-up@local.dev 真实登录）后全部恢复——
判定为 blocked-env，非小程序缺陷。kb/upload 两页为 dev 栈会话（带真实 KB）所验，
其余 14 分包页与 4 Tab 为 Up 栈会话所验。

| 页面 | 结果 | 截图（evidence/sim-tour/） |
| --- | --- | --- |
| login | 表单渲染 + **真实登录链路通过**（输入→提交→自动进 workspace） | tour-login.png / login-result.png |
| workspace | 真实空间"parity-up's Workspace"选择→进入 home | tour-workspace.png |
| home (Tab) | 真实会话 + hero + Agent 数据 | tour-home-tab.png / home-after-login.png |
| tasks (Tab) | 列表端点 404 诚实降级 | tour-tasks-tab.png |
| knowledge (Tab) | 知识库列表（Up 栈空列表为真实空态） | tour-knowledge-tab.png |
| me (Tab) | 真实用户 parity-up；summary 404 降级不伪造 | tour-me-tab.png |
| agents / agent | Agent 列表 + 详情（builtin-quick-answer） | tour-agents.png / tour-agent.png |
| chat | 参数装配渲染（未发送消息，无副作用） | tour-chat.png |
| execution / approval / artifact | 无效 ID 诚实降级，不猜定位 | tour-execution.png 等 |
| kb / upload | dev 栈真实 KB 详情 + 上传表单 | （文字证据见矩阵，截图因 /tmp 清理未存档） |
| document | 文档页渲染（无效 doc 降级） | tour-document.png |
| usage | "暂时无法读取…当前部署尚未开放该能力" | tour-usage.png |
| checkout | 支付关闭说明，无虚构报价 | tour-checkout.png |
| order / invitations / states | 表单与静态说明渲染 | tour-order.png 等 |

## 模拟器实测补充（2026-09-18 上午轮，dev 栈 OrbStack 后端）

| 页面 | 实测结果 |
| --- | --- |
| home | 已登录真实会话，Agent/任务中心真实数据，Errors: 0 |
| tasks | 列表 404 诚实降级文案 + 降级提示；恢复按钮禁用态正确 |
| knowledge | 真实知识库（"AI 产品研究"，api 来源） |
| me | summary 404 → 错误条 + 重新加载，无伪造余额 |
