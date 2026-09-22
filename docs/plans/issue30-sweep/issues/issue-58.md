# Issue #58 — T28: Tenant Catalog 不可变 Release 发布闭环

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #58 |
| 标题 | T28: Tenant Catalog 不可变 Release 发布闭环 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/58 |
| 状态 | closed |
| 关闭时间 | 2026-09-20T19:37:42Z |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:42:27Z |
| 更新时间 | 2026-09-20T19:37:42Z |
| 评论数 | 4（API 字段 4，timeline commented 事件 4，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | None（正文声明可立即开始）（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #59、#60 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

Agent Author 从确定 Agent Version 提交脱敏 Release，Tenant Reviewer 审核 Manifest、Dependency Lock、许可和摘要后生成不可变 Release 与 Listing。

## Acceptance criteria

- [ ] 重新发布创建新 Release，不覆盖旧快照。
- [ ] KB、模型、凭据、Sandbox、Memory 和 Task 内容不会进入 Release。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- None (can start immediately).

## 任务清单（正文 checkbox 原文）

- [ ] 重新发布创建新 Release，不覆盖旧快照。
- [ ] KB、模型、凭据、Sandbox、Memory 和 Task 内容不会进入 Release。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

### 评论 1 — 1123786563 @ 2026-09-20T19:28:59Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/58#issuecomment-5752105376
- 摘要: 已实现并集成到 feature/mobile-office（集成 HEAD 0c5a6bdc）。独立任务评审与全分支评审通过后修复完成。证据包括 SQLite 后端 HTTP 生命周期的不可变重发布、净化 canonical bundle 检查、role/tenant 授权与精确错误映射。Wave 检查全部通过：go test ./...、test:shared、test:web（2260 tests）、test:mobile（9 tests）、AgentVersion/Tenant Release 契约与 API 测试（11）、shared/web/mobile 类型检查、iOS/Android Expo 导出。本环境无法执行 PostgreSQL 运行时迁移；PostgreSQL schema 迁移已配对评审，SQLite 迁移/回滚检查通过。

### 评论 2 — 1123786563 @ 2026-09-20T19:34:15Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/58#issuecomment-5752139301
- 摘要: wave 级验收评审后重开：实现与代码评审干净、SQLite 生命周期/迁移检查通过，但已批准实施计划的 Task 8 还要求 PostgreSQL/SQLite 迁移对等与回滚证据；因无 PostgreSQL DSN，PostgreSQL 迁移只评审未执行。保持开放直至 PostgreSQL 升级/回滚与约束行为验证完成；未报告生产代码缺陷。

### 评论 3 — 1123786563 @ 2026-09-20T19:37:40Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/58#issuecomment-5752159718
- 摘要: 验收补充完成：PostgreSQL 15.2 运行时检查应用迁移 000178/000179，插入完整 version/listing/submission/review/release 数据图，证明并发对立评审在 uq_agent_release_review_submission 约束上失败，并验证 000179 回滚保留 AgentVersion、在 000178 回滚前移除 Marketplace schema；临时本地容器已清理。与通过的 SQLite 迁移/回滚及生命周期测试一起，关闭此前记录的 PostgreSQL 门禁。

### 评论 4 — 1123786563 @ 2026-09-20T19:37:42Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/58#issuecomment-5752159974
- 摘要: Ticket #58 验收门禁全部完成；实现已集成于 feature/mobile-office，PostgreSQL/SQLite 升级、回滚、不可变性、脱敏与竞争评审约束均有证据。（随后 Issue 于 19:37:43Z 关闭）


## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
