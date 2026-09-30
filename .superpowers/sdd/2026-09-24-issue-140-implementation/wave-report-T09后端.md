# Wave 报告：T09 后端子任务（Career Source Adapter And Durable URL Evidence）

- 状态：DONE（实现完成，留在 worktree 等独立评审，未集成、未宣布 T09 完成）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t09-source-import/WeKnora-fork01`（detached，基线 `21df162a24d73d9ec68d012458577050c18fd59f`）
- HEAD：`483bcfe38`，提交 `feat(career): record URL source evidence`（1 个本地提交，未 push）
- 评审包：`/Users/wuyongjun/.codex/worktrees/issue-140-t09-source-import/WeKnora-fork01/.superpowers/sdd/2026-09-24-issue-140-implementation/review-21df162a2..483bcfe38.diff`（95009 字节）
- Worktree 内详细报告：`.superpowers/sdd/2026-09-24-issue-140-t09-source-import/task-1-report.md`（含 RED/GREEN 证据原文与设计取舍）

## RED→GREEN 证据（真实运行输出）

RED（先写九个命名测试，实现不存在）：
- `go test ./internal/modules/career/... -count=1` → build failed（`undefined: ApprovedSource / SourcePolicy / SourceFetchResult / transportDialOptions` 等）
- `go test ./internal/database/... -count=1` → 4 FAIL（迁移文件不存在、新列缺失、版本 116≠119）
- `go test ./internal/router/... -count=1` → `TestCareerSourceImportRoutesAreRegistered` FAIL（路由未注册）

GREEN（Step 4 全量 VERIFY，逐条照跑）：
- `gofmt -w internal/modules/career internal/router` → 无输出
- `go test ./internal/modules/career/... -count=1` → `ok 2.319s`
- `go test ./internal/database/... -count=1` → `ok 7.424s`
- `go test ./internal/router/... -count=1` → `ok 1.315s`
- `go test ./tools/architectureguard/... -count=1` → `ok 0.310s`
- `git diff --check` → clean
- 九个命名测试逐一 `-v` 确认 PASS；并发测试额外 `-count=5` 复跑稳定；`go vet` 四包无告警。

## 提交与文件清单

commit `483bcfe38`（14 files changed, 1731 insertions, 15 deletions）：
- 新建 `internal/modules/career/source_import.go`、`source_import_test.go`
- 修改 `internal/modules/career/opportunity.go`（模型 8 新列、ImportJD owner-scoped append）、`office.go`（Office sourcePolicy/sourceTransport + SQLite shape 校验）、`handler.go`（ImportURL/OpportunityObservations handler）
- 修改 `internal/router/routes_career.go`（+2 路由）、`routes_career_test.go`
- 新建迁移 versioned `000198_source_import_observations.{up,down}.sql`、sqlite `000119_source_import_observations.{up,down}.sql`（唯一分配编号；migrations/ 在 .gitignore 中但按 T08/T10 先例 `git add -f` 跟踪）
- 修改 `internal/database/career_migration_test.go`（迁移测试 + 版本断言 116→119 + 新列清单）
- 修改 `tools/architectureguard/discovery_test.go`（581→583 / 650→652）与 `docs/architecture/moves/README.md`（历史段）

## 关键行为（对齐 Review Focus）

1. 未核准主机：零网络尝试（transport 调用计数为 0），返回 `policy_unverified` + `source_unverified` + `needsUserJD=true`，仍持久化观察证据（空文本快照 + 空字节 SHA-256 + needs_review）。
2. 重定向：production transport 在重定向处逐跳 `policy.VerifyRedirect` + 字面私网 IP 拒绝；httptest 实测未核准/私网目标 0 次命中（在 redirect 处拒绝，不回溯信任）；细节（私网 IP/原始错误）不出现在任何结果 JSON、收据体或错误消息中。
3. 含学历/届别词的摘要页：所有硬字段恒 `unknown`（即使注入了会提取的 extractor，URL 路径也不调用）。
4. URL 失败后手工补充 JD：同 opportunity 下 append 新观察+新快照（distinct snapshot IDs），原 URL 观察不变、两条均开放、观察列表可追溯。
5. 幂等与并发：同一 request ID 换 URL → `idempotency_conflict`；8 并发同请求 → 恰 1 条 observation/snapshot/receipt（claim→fetch→reconcile 三段式，网络 I/O 严格在事务外；busy 冲突走收据对账返回类型化 outcome_unknown）。

## 自查与遗留

- 网络安全约束满足：仅 http/https、拨号前解析并拒绝 localhost/环回/私网/保留地址、DNS 钉 IP、重定向逐跳校验；DB 查询全部参数绑定（gorm 占位符）；迁移测试 INSERT 已按 Mimosa 要求参数化。
- 已知局限（设计使然）：生产 allowlist 起始为空（所有生产导入 policy_unverified，真实来源核验归 T33）；完整性判据为保守启发式；未跑 PostgreSQL 实测（无 PG 环境，SQLite 树已 up/down/up 实测，versioned 语法与 T08/T10 同构）。
- 未做（本轮范围外）：Web 子任务（Task 2）未写任何 Web 文件，等本合同 reviewed 并集成后另行派发。
