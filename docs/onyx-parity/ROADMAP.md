# Onyx 能力对齐路线图（craft + connectors + 平台运营面）

> **目标**：把 [GAP-MATRIX.md](./GAP-MATRIX.md) 盘点出的缺口全部实现（方案 A · 治理优先）。
> **纪律**：本文件是 Onyx 对齐线的唯一进度真相源。已 ✅ 条目不得重做，只做回归复核。条目编号引用矩阵（C-n = craft，K-n = connectors，P-n = platform 平台运营面）。
> **流程**：每个 SP 走 brainstorming 设计 → `docs/superpowers/specs/` spec → writing-plans 计划 → 实施 → 在此登记状态。

## 状态图例

⬜ 未开始 ｜ 🔄 设计/实施中 ｜ ✅ 已完成（含验证证据） ｜ ⏸ 暂缓

## 子项目分解与顺序

| SP | 主题 | 覆盖矩阵条目 | 状态 |
|----|------|--------------|------|
| SP1 | 清债接线：模型网关生产挂载、Stop 路由、前端 confluence/dingtalk 入口、幽灵声明清理 | C-25、C-3、K-26、K-5 | ✅（验证证据：各任务测试命令绿——go test 目标包 / pnpm test:craft:shared 113 pass / test:web 新用例绿 / frontend type-check+check-i18n 11/11；端到端冒烟为源码栈动作，留待合并后由用户环境执行） |
| SP2 | Connectors 治理补洞：attempt 级进度/心跳/取消、targeted reindex、删源级联清理、凭据动态续期 | K-11、K-12、K-29、K-3 | ⬜ |
| SP3 | Craft 定时任务：ScheduledTask CRUD+执行器+预授权目标 | C-23 | ⬜ |
| SP4 | User Library + 沙箱文件树 API/预览 | C-18、C-12 | ⬜ |
| SP5 | AGENTS.md 指令模板 + craft 内 MCP（桥接 appconnector 适配层） | C-15、C-16 | ⬜ |
| SP6 | Craft 管理面：admin 三页、onboarding、工作区/用户级开关 | C-27、C-28、C-26 | ⬜ |
| SP7 | Connectors 广度滚动 I：GitHub、Web 爬虫 | K-17、K-20 | ⬜ |
| SP8 | Connectors 广度滚动 II：IMAP/邮件、Google Drive | K-18、K-15 | ⬜ |
| SP9 | Connectors 广度滚动 III：Slack、SharePoint/OneDrive 等按需 | K-18、K-15 | ⬜ |
| SP10 | 长尾：能力体检、ingestion 直推 API、索引后层级浏览、cc_pair 详情页增强、状态总览聚合 | K-30、K-22、K-14、K-27、K-28 | ⬜ |
| SP11 | 平台运营面 · 反馈+分析：message_feedback 表+反馈 API、四组聚合端点、/platform/analytics 图表页（recharts）、消息气泡反馈按钮 | P-4、P-5 | ✅（[验收证据](../migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md)） |
| SP12 | 平台运营面 · 用量聚合：user_usage 日桶表+chat/craft 双写入点+commercial 计价、三个用量 API、用户 settings 分区+admin 用量 tab | P-1、P-2 | 🔄 设计定稿 |
| SP13 | 平台运营面 · 查询历史：admin 审计列表+快照+asynq CSV 导出+隐私三档、会话分享（租户内登录分享 token） | P-6~P-9 | 🔄 设计定稿 |
| SP14 | 平台运营面 · 轻项收割：commercial 四页挂路由+套餐入口、API key 面板 API 文档入口、per-user 默认模型偏好+解析链 | P-3、P-10、P-11 | 🔄 设计定稿 |

> 平台运营面四 SP 的设计定稿：`docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md`（关键决策：语义对齐+基建复用、分享=租户内登录态、聚合=用量日桶+分析读时聚合）。实施顺序 SP11 → SP12 → SP13，SP14 可穿插；SP13 依赖 SP11 的 feedback 表。

## 明确不排期（P3，需单独产品决策才启动）

- C-10 webapp 实时预览反代（与 W02 受控静态预览安全模型冲突）
- K-4 cc_pair 多对多重构（当前 1:1 内聚够用）
- K-23/24/25 文档权限同步三件套（改变"导入即共享"模型，牵动检索过滤链路）
- C-6 S3 沙箱休眠管道（现有 sweep+重放语义已覆盖治理目标）

## 依赖说明

- SP2 先于 SP7-9：attempt 进度/重索引基建就位后，新连接器直接受益，不返工。
- SP3 依赖 SP1：定时任务产物的模型调用走模型网关，网关先接线。
- SP5 的 craft MCP 复用 appconnector MCP 适配层（已存在），是桥接不是新造。
- SP10 可穿插在任何阶段按需提前。
- SP11–SP14 与 SP1–SP10 两条线相互独立，可并行；SP13 的审计详情依赖 SP11 的 message_feedback 表。

## 变更记录

- 2026-09-19：路线图创建（方案 A 确认）；SP1 进入设计。
- 2026-09-19：增补平台运营面线（SP11–SP14），设计定稿（用量/套餐/分析/查询历史/OpenAPI/对话偏好六域对齐，语义对齐+基建复用原则）。
- 2026-09-19：SP1 四项接线完成（commit 见 git log）。
