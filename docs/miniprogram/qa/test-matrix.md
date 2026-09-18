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

## 模拟器交互层实测（2026-09-18 晚间轮，Up 栈后端 + parity-up 真实会话）

automator（cli auto :9420 + connect）逐场景驱动，断言 24/25 通过；唯一 FAIL 为脚本断言错位
（见下"审批空态"行，实际行为更严格）。截图：[evidence/sim-interactions/](./evidence/sim-interactions/)。

| 场景 | 断言 | 结果 |
| --- | --- | --- |
| 四 Tab switchTab 往返 | currentPage 随切换正确（首次读值抖动复验 3/3 稳定） | PASS |
| home→agents 二级导航（真实 tap"查看全部"）+ navigateBack | 进入分包页、返回回 home | PASS |
| agents 搜索过滤（输入"问答"） | 列表 4→2 收窄 | PASS |
| agents 无匹配关键词 | 空态文案渲染，cards=0 | PASS |
| knowledge 搜索无匹配 | 空态文案渲染 | PASS |
| login 表单：空表单/填表未勾选 | 提交按钮 disabled（attr=true） | PASS |
| login 表单：填写+勾选 consent | 提交按钮 enabled | PASS |
| tasks 输入无效 run ID →"读取任务状态" | 跳转执行详情页，"正在读取服务端快照…此资源不存在"诚实降级，不重启任务 | PASS |
| approval 无效 run | **无任何批准入口渲染**（比按钮禁用更安全）+ 全局"仅支持安全拒绝"提示 + 降级条 | PASS（脚本原断言找禁用按钮，实际列表空即不渲染，行为更严） |
| checkout | "暂不可购买"disabled（attr=true）+ 渠道未接入说明 | PASS |
| me 退出登录（mock showModal → cancel） | 会话保持，页面不跳转 | PASS |
| chat 长中文+emoji 输入（不发送） | 167 字符保留、发送按钮出现 | PASS |
| 冷启动会话恢复（重建 Up origin bundle 重载） | bootstrap 从 storage 恢复 ready，home 显示真实空间名（曾疑 D8"重启丢会话"，查实为 dist 被并行会话替换成 placeholder/localhost origin 所致的预期 key 不匹配，非缺陷） | PASS |
| 尺寸转换链路 | tokens.json dimension → build-tokens ×2 → tokens.mini.scss rpx（一次性）；手写 px(375 画板) → pxtransform ×2 → rpx；fixedPx（阴影/发丝线）刻意不缩放；dist 抽样 14px→28rpx/16px→32rpx/20px→40rpx 逐一对应，**无重复乘二**；实测 .wk-card 左缘 20px = 40rpx×(390/750)≈20.8px（取整内） | PASS |
| 前后台切换（useDidHide stopSubscriptions） | automator 无对应 API，未自动化 | —（复验：手动切后台/前台后回聊天页观察订阅停止与恢复） |

## 明确未验证（blocked-env / not-implemented）

- Android/iOS 真机：未执行（复验步骤见 report.md 第 7 节）。
- 真实后端带副作用的流式链路（聊天流式回答、执行事件流、上传解析、订单查询、审批决策）：
  未在本轮执行（会产生模型消耗/业务写操作，未获授权）。
- 微信支付、快捷登录、审批正向批准：not-implemented，入口关闭。

## 模拟器实测补充（2026-09-18，真实 OrbStack 后端）

| 页面 | 实测结果 |
| --- | --- |
| home | 已登录真实会话，Agent/任务中心真实数据，Errors: 0 |
| tasks | 列表 404 诚实降级文案 + 降级提示；恢复按钮禁用态正确 |
| knowledge | 真实知识库（"AI 产品研究"，api 来源） |
| me | summary 404 → 错误条 + 重新加载，无伪造余额 |
