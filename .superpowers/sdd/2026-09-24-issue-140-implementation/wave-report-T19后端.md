# Wave 10 — T19 后端 wave 报告：求职信与基于投递版的面试准备（Issue #156）

- **BASE**: `f6b14cf95`（集成 HEAD）
- **HEAD（实现 worktree）**: `481f4a745`（唯一提交；task-1-report.md 位于 worktree `.superpowers/` 下，该目录被 .gitignore 忽略，按简报要求以文件形式交付、未入库）
- **Worktree**: `/Users/wuyongjun/.codex/worktrees/issue-140-t19-preparation/WeKnora-fork01`（本地提交，未 push/merge）
- 完整报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t19-preparation/task-1-report.md`

## 交付内容（单提交 481f4a745，13 文件：2 新源文件 + 4 新迁移 + 7 修改）

| 文件 | 变更 |
|---|---|
| internal/modules/career/preparation.go | **新增**：GeneratePreparation/FindPreparationReceipt/ApplicationPreparations、PreparationGenerator seam、deterministicPreparationGenerator（生产默认，零外部 LLM）、career_preparations 持久层 |
| internal/modules/career/preparation_test.go | **新增**：9 个命名 RED→GREEN 测试 |
| internal/modules/career/office.go | 聚焦修改：preparationGenerator 字段+默认值+SetPreparationGenerator、models 列表+preparationRecord、SQLite schema 校验（列+唯一约束） |
| internal/modules/career/handler.go | 聚焦修改：3 个 handler（生成/列表/回放）、writeError 新增 preparation_version_unknown(409，含 applicationId/submissionRecorded) 与 preparation_generation_failed(500) |
| internal/modules/career/handler_test.go | 聚焦新增：TestCareerPreparationHandlersPromptReplayAndFailure |
| internal/router/routes_career.go | 3 条路由：POST/GET `/career/applications/:applicationId/preparations`、GET `/career/preparations/receipt` |
| internal/router/routes_career_test.go | 聚焦新增：TestCareerPreparationRoutesAreRegistered |
| migrations/versioned/000206_career_preparations.{up,down}.sql | **新增**（沿用 T22 先例 `git add -f`，migrations/ 在 .gitignore） |
| migrations/sqlite/000127_career_preparations.{up,down}.sql | **新增**（同上） |
| internal/database/career_migration_test.go | 19 处版本断言 126→127 + TestPreparationMigrationUpAndDown |
| tools/architectureguard/discovery_test.go | wantRouteLiteral 617→620、wantRouteTotal 686→689（T22 后实值为基的精确更新） |

## 验收要点落位

1. **版本锚定**：仅确认投递绑定版本；V2 已投递→引用 V2（V3 已存在也不引用）；未知/无投递→typed 提示态 409 `preparation_version_unknown`，不猜最新。
2. **只引用已确认事实和岗位快照**：生成产物经 `validateMaterialClaims`（T15 先例）+ 快照 digest 入 Sources；未确认主张 → typed 失败、零持久草稿。
3. **可审阅、修订、有来源**：草稿物化为 material 草稿行，经既有 EditMaterial/ConfirmMaterial 修订（新版本、不覆盖）；回执带完整来源链（投递版本+快照+事实键+profile revision）。
4. **零自动发送零承诺**：无 transport/linker/存储副作用，不改 submissions/progress；缺失=显式 needs_review 占位。
5. **失败保留可恢复**：seam 失败/主张被拒 → 行状态 failed + 失败码可读（同 request ID），重试同 ID 成功转 draft，无空白成功产物。
6. **house 语义**：request ID 精确回放/内容变化 409、expectedRevision CAS 返回当前版本、scope 拒绝他租户/他人、(tenant,user,request_id) 唯一。

## 迁移编号使用

**使用**了分配编号：versioned **000206** / sqlite **000127**（新表 career_preparations，未归还）。理由：投递锚点需可重读、失败态需可恢复，且 material.go 不在本任务文件所有权内无法扩展证据 pin —— 取舍详见 task-1-report.md「生成 seam 取舍披露」。

## RED/GREEN 证据摘要（真实输出）

- RED（office 9 测试）：`go test ./internal/modules/career/ -run TestPreparation -count=1` → `[build failed]`，9 类 undefined 符号（PreparationGenerationRequest/GeneratePreparationInput/preparationRecord/o.GeneratePreparation/PreparationFocus*/PreparationKind* 等）。
- RED（迁移）：移走 4 个迁移文件后 → `paired migration files must exist: migrations/versioned/000206_career_preparations.up.sql` FAIL（随后恢复）。
- GREEN：9/9 PASS + 迁移测试 ok。

## 验证命令（全部实跑，输出真实）

```
gofmt -w internal/modules/career internal/router          # clean（gofmt -l 无输出）
go test ./internal/modules/career/... -count=1            # ok  40.895s
go test ./internal/database/... -count=1                  # ok  69.345s
go test ./internal/router/... -count=1                    # ok   4.454s
go test ./tools/architectureguard/... -count=1            # ok   1.174s
git diff --check                                          # clean
go build ./...                                            # ok
go vet（4 个改动包）                                       # clean
```

## 自查与遗留

- **遗留 1（文件所有权限制）**：`career_export.go` 的 `careerPurgeTables`/删除边界视图不在本任务所有权内，`career_preparations` 未纳入 purge —— delete_career 会留下 preparation 孤儿行（scope 内可见，无用户可见泄露）。建议集成员补一行（purge 列表 + boundary InSpace 段）。
- **遗留 2（语义披露）**：生成期间 profile 前进不使已预约生成失败（reserve 时 CAS；finalize 以新 head 重校验 claims，事实只增不减）；回执 Revision 报预约时 head。
- **遗留 3**：同一申请允许多次生成（不同 request ID/focus 并存），spec 未禁止，未加唯一约束。
- 生成 seam 生产默认为本地确定性组装器，**零外部 LLM 依赖**；Web 子任务可依据本报告「接口冻结」一节（见 task-1-report.md）对接。
