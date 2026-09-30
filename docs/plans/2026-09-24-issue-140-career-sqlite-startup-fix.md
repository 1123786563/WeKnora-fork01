# Issue #140 T03 SQLite Startup Integration Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 T03 Career Office 在隔离 SQLite Lite 服务的实际启动顺序中完成迁移并注册路由，使 #141 浏览器验收能够运行。

**Architecture:** 保持 Career 数据模型及 `000112`/`000191` 与 T04 `000113`/`000192` 的迁移顺序。复现 `internal/database` 版本化 SQL 迁移后 `career.NewOffice` 的 GORM schema check；根据复现定位表约束解析或重复 AutoMigrate 的根因，在最窄的 Career 迁移/初始化边界修复。对已有应用过 SQL 迁移的数据库也要有效，不只修新库。

**Tech Stack:** Go 1.26、GORM、SQLite Lite、现有迁移器。

**Sources:** #141 Issue 快照 `docs/plans/issue-140/issues/issue-141.md`、批准求职 Spec、ADR-0015/0017、T03 原计划及 `task-3-integrated-backend-validation.md`。启动证据：一次性 SQLite 路径运行 `go run ./cmd/server`，在 `NewOffice` 报 `table career_facts__temp has no column named UNIQUE`；原始迁移有表级 `UNIQUE (...)`，`NewOffice` 调用 `AutoMigrate`。临时库在 `/tmp/weknora-t03-browser.I3xI7y`，不含用户数据。

## Global Constraints

- 单独 Worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t03-sqlite-fix/WeKnora-fork01`，BASE `a4bb32ed3`；用户授权本地提交，不推送/部署。
- 仅修改 `internal/modules/career/`、相关 Career/数据库集成测试和必要的 Career 迁移 SQL；不修改 T04 授权业务或共享开发数据库。
- 不以删除唯一性约束换取启动成功；`(tenant,user,key)` 事实、`(tenant,user,revision)` 事件、`(tenant,user,requestID)` 收据仍必须在 SQLite 和 PostgreSQL 强制唯一。
- 保留已应用 `000112`/`000191` 的升级路径；若需要新增迁移，顺接当前最高版本并测试上下行，不能改写已部署库的历史事实。

## Review Focus

1. 失败先行测试覆盖真实 Lite 顺序：版本化 SQL up → `career.NewOffice` → 再次打开/重启；首次运行的 `UNIQUE` 报错必须先复现再消失。
2. 新库与已迁移库都能启动，T04 `000113` 与 Career `000112` 连续应用后仍可启动。
3. 唯一约束、事实历史、收据和 revision CAS 不因迁移修复弱化。
4. 实际一次性 Lite 服务启动、注册/登录及 `/api/v1/career/open` 的只读探针在修复集成后运行，不复用共享 DB 或现有账号。

### Task 1: 修复 Career SQLite SQL 迁移与 AutoMigrate 交互

**Depends:** T03 backend 已独立审查并集成，T04 已顺序集成；Web T03 修复可在独立 Worktree 并行。**Owner/validator:** backend_implementer / backend_validator。**Files:** `internal/modules/career/office.go` 或相邻初始化文件、`internal/database/career_migration_test.go`、必要的 `migrations/sqlite/` Career 增量；只在需要时修改相邻测试。**Consumes:** 现有 versioned/SQLite schema、GORM Career models。**Produces:** 可在空库与已迁移 SQLite 库启动的 Career handler，保留数据库唯一约束。**Parallel:** 与 T03 Web 不共享文件/数据库/端口；集成由主控串行。

- [ ] **Step 1:** 在隔离临时 SQLite 上写一个使用仓库真实迁移器依次应用至 `000113` 后调用 `career.NewOffice` 的测试，先运行并保存 `career_facts__temp ... UNIQUE` 的 RED 输出；同时检查 `career_changes`/`career_receipts` 是否同类潜在错误。
- [ ] **Step 2:** 跟踪 GORM 的 schema 读取/重建触发点，确认是表级 UNIQUE 解析还是模型差异导致的重复 AutoMigrate；选择最小修复，使历史已迁移库、新空库和 PostgreSQL 路径一致。不得静默丢弃约束或忽略初始化错误。
- [ ] **Step 3:** GREEN：运行新集成测试、`go test ./internal/modules/career/... ./internal/database/... ./internal/router/...`，检查事实/事件/收据唯一冲突测试及迁移上下行，`git diff --check`。在独立本地提交并记录命令/结果。
- [ ] **Step 4:** 后端验证者在同一提交 SHA 上独立重跑迁移→NewOffice 与唯一样例；reviewer 给出 Spec/质量双结论。主控集成后再用一次性 Lite 服务验证实际启动和受认证 Career HTTP，不把仅测试通过当浏览器验收通过。

**Failure handling:** 如问题涉及 GORM 对已部署 SQLite 表不可逆重建，保留失败库并记录 schema/SQL；改用显式增量迁移或避免重复 AutoMigrate 的稳定边界，重新审查，不让失败路径被 `IF EXISTS`/忽略错误掩盖。
