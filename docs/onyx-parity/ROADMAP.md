# Onyx 能力对齐路线图（craft + connectors）

> **目标**：把 [GAP-MATRIX.md](./GAP-MATRIX.md) 盘点出的缺口全部实现（方案 A · 治理优先）。
> **纪律**：本文件是 Onyx 对齐线的唯一进度真相源。已 ✅ 条目不得重做，只做回归复核。条目编号引用矩阵（C-n = craft，K-n = connectors）。
> **流程**：每个 SP 走 brainstorming 设计 → `docs/superpowers/specs/` spec → writing-plans 计划 → 实施 → 在此登记状态。

## 状态图例

⬜ 未开始 ｜ 🔄 设计/实施中 ｜ ✅ 已完成（含验证证据） ｜ ⏸ 暂缓

## 子项目分解与顺序

| SP | 主题 | 覆盖矩阵条目 | 状态 |
|----|------|--------------|------|
| SP1 | 清债接线：模型网关生产挂载、Stop 路由、前端 confluence/dingtalk 入口、幽灵声明清理 | C-25、C-3、K-26、K-5 | 🔄 |
| SP2 | Connectors 治理补洞：attempt 级进度/心跳/取消、targeted reindex、删源级联清理、凭据动态续期 | K-11、K-12、K-29、K-3 | ⬜ |
| SP3 | Craft 定时任务：ScheduledTask CRUD+执行器+预授权目标 | C-23 | ⬜ |
| SP4 | User Library + 沙箱文件树 API/预览 | C-18、C-12 | ⬜ |
| SP5 | AGENTS.md 指令模板 + craft 内 MCP（桥接 appconnector 适配层） | C-15、C-16 | ⬜ |
| SP6 | Craft 管理面：admin 三页、onboarding、工作区/用户级开关 | C-27、C-28、C-26 | ⬜ |
| SP7 | Connectors 广度滚动 I：GitHub、Web 爬虫 | K-17、K-20 | ⬜ |
| SP8 | Connectors 广度滚动 II：IMAP/邮件、Google Drive | K-18、K-15 | ⬜ |
| SP9 | Connectors 广度滚动 III：Slack、SharePoint/OneDrive 等按需 | K-18、K-15 | ⬜ |
| SP10 | 长尾：能力体检、ingestion 直推 API、索引后层级浏览、cc_pair 详情页增强、状态总览聚合 | K-30、K-22、K-14、K-27、K-28 | ⬜ |

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

## 变更记录

- 2026-09-19：路线图创建（方案 A 确认）；SP1 进入设计。
