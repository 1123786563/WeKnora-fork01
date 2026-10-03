# Dev 库漂移清偿轮实施计划（/loop 第 2 循环）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清偿 WeKnora-postgres-dev 库的 15 张同型漂移表（迁移账本越过未执行所致，清单在 `.superpowers/sdd/2026-10-03-wb-graph/delivery-verify-report.md` 附录），区分「纯 DB 漂移」与「缺生产迁移的真代码缺口」，并从 HEAD 重启 :8084 后端。

**Architecture:** 漂移模式先例=000196 被账本 271 越过（上轮已治愈）；`workbench_device_registrations` 型缺口=有模型无迁移（上轮已补 000272/000191）。本轮对 15 表逐一同型判别：账本表内缺号→直灌后 `migrate up`；无建表方→补双链迁移+TDD（沿用上轮模式与命名纪律，versioned 续 000273+、sqlite 续 000192+）。

**Tech Stack:** psql（容器内）；仓库迁移双链（migrations/versioned + sqlite）；Go 迁移测试。

## Global Constraints

- dev 库为本地开发库，可直灌修复；**生产语义不改**（只补缺失迁移/对齐账本，不重编号既有迁移）。
- 每补一个迁移必须 TDD（先例：workbench_migration_test / migration_version_uniqueness 模式）。
- 重启后端从当前 HEAD 编译（先 kill 旧 :8084 进程——是本会话栈自有进程，安全）。
- 提交带 `(DEVDB)`；不 push（轮末统一）；密钥不入库。
- 工作区 `.superpowers/sdd/2026-10-03-devdb-drift/`。

---

### Task 1: 漂移审计与清偿（subagent）
- [ ] 读附录 15 表清单，逐表判别：账本内缺号（直灌+migrate up）/ 无生产建表方（补双链迁移+TDD）/ 模型废弃（记录不动）。
- [ ] 执行清偿；`migrate up` 至最新；全部 15 表 `\d` 存在且列集与模型对齐。
- [ ] kill 旧 :8084 → 从 HEAD 编译重启（同环境变量配方）→ /health 200 + 上轮三面复测（inbox/delivery/devices）+ bootsmoke。
- [ ] 证据与判别表 → task-1-report.md；提交补迁移（如有）。

### Task 2: 抽查判词（controller 或 opus）
- [ ] 补迁移 diff 抽查（必要性/命名/测试）；台账终局段写入本计划文档。

## Self-Review
覆盖：15 表判别→Task 1；重启验证→Task 1；抽查→Task 2；无占位。
