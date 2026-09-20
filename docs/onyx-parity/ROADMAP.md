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
| SP2 | Connectors 治理补洞：attempt 级进度/心跳/取消、targeted reindex、删源级联清理、凭据动态续期 | K-11、K-12、K-29、K-3 | ✅（**SP2-a 完成 2026-09-20**：K-11 心跳/stall/协作取消 + K-29 删源级联清理全链交付——sync_logs 生命周期列（heartbeat/asynq task id/cancel 标记）、双路径心跳、stall 窗口（超窗不阻塞再调度）、协作取消 API+React/Vue 双端 UI、删源 purge 级联（异步分批 drain 文档/向量/标签+审计）、documents-count 端点、双选删除面板（React Sheet / Vue TDesign Dialog 等价交互）。提交：`ee3a6ed0`/`13ea321b`/`86070810`/`cc424e71`/`cd56a458`/`14d56739`/`5369211d`/`aa63b3b0`/`d8bac0d3`（Task 10 收入 `161adf69`）。**SP2-b 完成 2026-09-21**：K-12 targeted reindex + K-3 凭据动态续期全链交付——TargetedFetcher 11 实装（ima 降级前置条件错误，Ruling P-1）、scoped reindex API（request_id 幂等/独立 SyncLog/ds 零写双写收敛）、SyncItemError 逐项失败样本（ExternalID+稳定 i18n 码）、机器凭据回写通道（三子句防覆盖守卫+写入集 {key,last_refreshed_at}）、CredentialsRefresher 续期触发（scoped 分支后，AuthVersion 递增+旧 cursor 失效）、credentials 字段级元数据（expires_at/last_refreshed_at/needs_reauthorization）、React 失败样本渲染+重试选中项、Vue 五语言 syncError 连接器码 18×5 照 React 抄写。提交：`ef2ccb24`(T1 收编)/`848f19d2`/`d292e17e`/`2d0e3dcb`(+`7e820411`/`aab41d5c` 收编)/`d86d244c`/`56ad6032`/`d63c2881`(+`de6ddd2d` 收编)/`1f693ea4`(T8 locale+spec 收编)+台账本笔。验证证据：`.superpowers/sdd/2026-09-20-sp2b-connectors-reindex-credentials/task-*-report.md` + [evidence](../migrations/react/evidence/onyx-parity/2026-09-20-sp2b-connectors-reindex-credentials.md)；本线包 go test 显式全绿 + 全量 `go test ./...` 零失败 + frontend type-check/check-i18n 11/11；专用 8085 栈冒烟 reindex 一轮+凭据元数据端点全过（共享 8084 为旧二进制未动）。真实 OAuth 连接器留 SP8） |
| SP3 | Craft 定时任务：ScheduledTask CRUD+执行器+预授权目标 | C-23 | ⬜ |
| SP4 | User Library + 沙箱文件树 API/预览 | C-18、C-12 | ⬜ |
| SP5 | AGENTS.md 指令模板 + craft 内 MCP（桥接 appconnector 适配层） | C-15、C-16 | ⬜ |
| SP6 | Craft 管理面：admin 三页、onboarding、工作区/用户级开关 | C-27、C-28、C-26 | ⬜ |
| SP7 | Connectors 广度滚动 I：GitHub、Web 爬虫 | K-17、K-20 | ⬜ |
| SP8 | Connectors 广度滚动 II：IMAP/邮件、Google Drive | K-18、K-15 | ⬜ |
| SP9 | Connectors 广度滚动 III：Slack、SharePoint/OneDrive 等按需 | K-18、K-15 | ⬜ |
| SP10 | 长尾：能力体检、ingestion 直推 API、索引后层级浏览、cc_pair 详情页增强、状态总览聚合 | K-30、K-22、K-14、K-27、K-28 | ⬜ |
| SP11 | 平台运营面 · 反馈+分析：message_feedback 表+反馈 API、四组聚合端点、/platform/analytics 图表页（recharts）、消息气泡反馈按钮 | P-4、P-5 | ✅（[验收证据](../migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md)） |
| SP12 | 平台运营面 · 用量聚合：user_usage 日桶表+chat/craft 双写入点+commercial 计价、三个用量 API、用户 settings 分区+admin 用量 tab | P-1、P-2 | ✅（[验收证据](../migrations/react/evidence/onyx-parity/2026-09-19-sp12-usage-aggregation.md)） |
| SP13 | 平台运营面 · 查询历史：admin 审计列表+快照+asynq CSV 导出+隐私三档、会话分享（租户内登录分享 token） | P-6~P-9 | ✅（[验收证据](../migrations/react/evidence/onyx-parity/2026-09-20-sp13-query-history.md)） |
| SP14 | 平台运营面 · 轻项收割：commercial 四页挂路由+套餐入口、API key 面板 API 文档入口、per-user 默认模型偏好+解析链 | P-3、P-10、P-11 | ✅（[验收证据](../migrations/react/evidence/onyx-parity/2026-09-20-sp14-lightweight.md)） |

> **六域线收官（2026-09-20）**：SP11–SP14 四个子项目全部 ✅，GAP-MATRIX 第三部分 P-1~P-11 十一项全绿——平台运营面六域（用量/套餐/分析/查询历史/OpenAPI/对话偏好）对齐线收官。收尾登记见 SP14 验收证据 §5。

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
- 2026-09-19：SP1 合并 main（merge 9c21612b）；合并时与并行 lane 迁移撞号（PG 000158 / sqlite 000079 双份 message_feedback），SP1 让号重排为 **PG 000161 / sqlite 000082**（修复 5c4d990e，测试引用同步）。合并后待办：端到端冒烟 4 项（无 secret 404 / 真 PG 网关链路 / 双前端测试连接 / 真 OC 栈停止收敛）回填证据。
- 2026-09-20：SP13 八任务完成（`c2e2bb04`…`e22b1a65`），回归+冒烟+证据收尾（[evidence](../migrations/react/evidence/onyx-parity/2026-09-20-sp13-query-history.md)）。
- 2026-09-20：SP14 四实现任务+收尾完成（`6a8c8f1a`/`eb49392f`/`c14c4501`/`f8ca453c`，净效果含并行 `e72acff9` 收编；收尾补 vite 别名一行修复），回归+冒烟+证据收尾（[evidence](../migrations/react/evidence/onyx-parity/2026-09-20-sp14-lightweight.md)）。**六域对齐线（SP11–SP14，P-1~P-11）就此收官**；冒烟另登记预存欠账：commercial summary 前后端形状错配（`de023ac9` 起，BillingPage 数据区/general 卡片/SP12 预算卡真实数据不可用）。
- 2026-09-20：**SP2-a（connectors 治理上半）收官**：Task 1-11 全部完成。K-11 ✅（sync_logs 生命周期列+心跳+stall 窗口+协作取消，cancel API/UI 双端）；K-29 ✅（删源可选级联 purge——异步分批 drain 文档/向量/标签+审计记录，DELETE `purge_documents=true` 严格匹配，双选面板默认不勾=保留文档承诺）。矩阵 K-11/K-29 行已更新；K-12（targeted reindex）/K-3（凭据动态续期）留 SP2-b。端到端冒烟 5 项留待合并后源码栈执行（计划 `.superpowers/sdd/2026-09-19-sp2a-connectors-governance/` brief 尾节）。
- 2026-09-21：**SP2-b（connectors 治理下半）收官——SP2 整体 ✅**：Task 1-8 全部完成。K-12 ✅（TargetedFetcher 11 连接器实装 + `POST /datasource/:id/reindex` scoped 重跑：request_id 幂等/独立 SyncLog/ds 零写 + SyncItemError 逐项失败样本带 ExternalID + React 失败项复选框重试 + Vue/React 五语言 syncError 连接器码）；K-3 ✅（凭据动态续期：AES 通道单 key 机器回写通道——三子句防覆盖守卫+写入集 {key,last_refreshed_at}+credential_auto_refreshed 审计、CredentialsRefresher 临期触发（<5min，scoped 分支后不轮换）、AuthVersion 递增使旧 cursor 失效、credentials 子资源字段级元数据 expires_at/last_refreshed_at/needs_reauthorization、mock OAuth 验收载体）。矩阵 K-12/K-3 行已更新；真实 OAuth 连接器（Google Drive 等）留 SP8。提交与验证证据：[evidence](../migrations/react/evidence/onyx-parity/2026-09-20-sp2b-connectors-reindex-credentials.md)。SP2-a 遗留的 5 项端到端冒烟仍在共享栈重启欠账清单（本线专用 8085 栈已覆盖 reindex/凭据元数据两项）。
