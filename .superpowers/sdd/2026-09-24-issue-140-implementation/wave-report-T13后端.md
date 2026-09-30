# Wave 7 — T13 后端子任务报告：可控的持续找岗规则（Issue #154，implement）

- 状态：**DONE**（worktree 本地提交完成，待独立评审与集成）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t13-search-rule/WeKnora-fork01`
- BASE `067aa1d0e` → HEAD `d7eb419c8`（单提交 `feat(career): run user-controlled recurring search rules`，14 files，+1467/−39）
- 详细报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t13-search-rule/task-1-report.md`

## 交付概要

- **规则生命周期**：`SetRule`（request-ID 幂等 + expectedRevision=profile head 钉扎 + scope 隔离）创建/更新规则（条件=query、频率=intervalMinutes 1 分钟..30 天、状态 enabled/paused/disabled）；未开启/disabled 零触发、零入队、零 quota 咨询、零源访问。
- **触发机制（受约束设计）**：显式 seam `Office.TriggerDueRules(ctx, now)`（注入时钟），`set_rule` 排程走 `Office.searchRuleNow` 注入时钟；**无常驻 goroutine/定时器**（全仓无常驻调度器先例，如需真实调度由主控 T33 前另裁）。
- **paused 语义冻结**：取消当次 + 恢复点重排（paused 时 next_due=NULL，到期经过零执行；恢复后 next_due=恢复时刻+interval，不补跑）。
- **发现待办去重**：发现身份=岗位链接，UNIQUE(tenant,user,link)，同岗位跨周期/跨规则恰一个 open 待办。
- **对账**：每周期确定性 request ID `rule:<ruleID>:<period>`，复用 T11 SearchOnce 幂等（claim/lease/receipt）；(rule,period) 唯一约束防 run 重复。重复触发同 ID 对账不复制（有测试）。
- **可见状态**：预算不足→run 记 `blocked_no_quota`、来源不完整→`no_vetted_sources`（均带 note，GET /rules/:ruleId 历史可查），零静默跳过。额度真实状态属 T21，未虚构；预计消耗为规则参数确定性估算（triggers/day × 已核源数，basis 注明 "not a quota balance"）。
- **迁移**：versioned `000203` / sqlite `000124`（实核目录最大 202/123，编号空闲确认）；四张表 career_search_rules / _rule_receipts / _rule_runs / _discovery_todos；up/down/up 往返测试通过；sqlite head 123→124。
- **路由**：POST /career/rules、GET /career/rules/receipt、GET /career/rules/:ruleId（3 条）；architectureguard **606/675 → 609/678**（精确值，台账注释同步）。

## RED/GREEN 证据（实测）

- **RED**（实现前）：career 包 build failed（`o.SetRule undefined`、`undefined: SetRuleInput/RuleStatusEnabled/searchRuleRecord`、`o.searchRuleNow undefined`）；database 包 9 FAIL（13 处版本断言 123 失败 + TestSearchRuleMigrationUpAndDown 迁移文件/表缺失）。
- **GREEN**：10 个命名测试全部 PASS（-run 过滤 -v 实录，见 task-1-report.md §2）。

## 验证命令与真实输出（BASE 四套件基线先全绿）

```
gofmt -w internal/modules/career internal/router        # 无输出；gofmt -l 空
go test ./internal/modules/career/... -count=1          # ok  9.007s
go test ./internal/database/... -count=1                # ok  20.545s
go test ./internal/router/... -count=1                  # ok  3.857s
go test ./tools/architectureguard/... -count=1          # ok  1.061s
git diff --check                                        # clean
```

## 提交列表

- `d7eb419c8` feat(career): run user-controlled recurring search rules

## 文件清单

新增：`internal/modules/career/search_rule.go`、`search_rule_test.go`、`migrations/versioned/000203_career_search_rules.up/.down.sql`、`migrations/sqlite/000124_career_search_rules.up/.down.sql`
修改：`internal/modules/career/office.go`、`handler.go`、`handler_test.go`、`search_once_test.go`（越界披露见下）、`internal/router/routes_career.go`、`routes_career_test.go`、`internal/database/career_migration_test.go`、`tools/architectureguard/discovery_test.go`

## 自查与披露

1. **一处越界（语义保持）**：`search_once_test.go`（T11，不在简报 Files 列表）原断言"无 %rule% 表"与简报指令的 career_search_rules 迁移必然冲突；改为数据级断言（one-shot 搜索后规则行/到期计划/run/todo 全零），完整保留原语义。已在 task-1-report §7 披露，请评审确认。
2. migrations/ 目录在 .gitignore，与既有迁移一致以 `git add -f` 入库。
3. 已知局限：TriggerDueRules 串行且单规则 DB 错误中止 sweep（同 ID 下轮对账）；触发 expectedRevision 读 sweep 时 head，并发 profile 写冲突则本轮跳过下轮重试（未测该竞态）；todo 仅 'open' 无闭环（范围外）。
4. 绝未 push/merge；主仓库与集成分支未改动（报告文件除外）。
