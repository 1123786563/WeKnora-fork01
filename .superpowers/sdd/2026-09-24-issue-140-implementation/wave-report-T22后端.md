# Wave 9 — T22 后端：求职数据导出与完整删除（Issue #162，implement）

- 状态：DONE
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t22-export/WeKnora-fork01`（独立、detached）
- BASE `10e2f30da` → HEAD `51b94cd46`（本地提交 1 个，未 push/merge）
- COMMIT：`feat(career): export and completely delete job search data`
- 详细任务报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t22-export/task-1-report.md`

## 交付内容

1. **export_career（封闭 intent）**：同步、owner-scoped 完整导出包——档案事实+提案、事实历史、原岗位快照（rawText）、申请事件（progress）、材料版本、投递记录；sha256 digest 可校验；requestId 幂等 replay；同 requestId 异 intent → 409。
2. **delete_career（封闭 intent）**：
   - 删除前边界清单 API（GET deletions/boundary）：in-space 分区+计数 vs 外部平台资料（不可撤回）+ 保留披露（holder/reason/status）。
   - 四步 durable 状态机（revoke exports → purge 24 张 career 表 → Workbench 投影移除 → finalize revision+1 & career_deleted 事件）。
   - 部分失败：status=partial + 审计 + 同 requestId 续删恢复；绝不声称已完全删除。
   - 删除后：旧 export/artifact 授权 fail-closed（grant 兑换重查 durable 行）；changes 流呈现 deletion 事件驱动客户端缓存失效。
3. **跨域删除端口**：`internal/types/interfaces.CareerApplicationTaskProjectionRemover`（T14 linker 同型）；Workbench coordinator 实现（scoped + 幂等）；container 接线。Career 不直写 Workbench 表。
4. **迁移**：versioned 000205 / sqlite 000126（实核空闲编号），表 career_data_exports + career_data_deletions，UNIQUE(tenant,user,request)。
5. **architectureguard**：612/681 → 617/686（+5 路由，精确值）。

## RED/GREEN 证据

- RED（实现前，9 个命名 office 测试）：`go test ./internal/modules/career/ -count=1` → `[build failed]`，`o.ExportCareer undefined`、`undefined: interfaces.CareerApplicationTaskProjection` 等（完整输出见任务报告）。
- GREEN（9 个命名测试全过）：`--- PASS: TestExportCareerIncludes...` 等 9 行 + `ok github.com/Tencent/WeKnora/internal/modules/career 3.086s`。
- 事实披露：`TestCareerExportMigrationUpAndDown` 写于迁移文件之后，首跑即 GREEN，无 RED 记录。

## 验证命令（全部实跑通过）

```
gofmt -w internal/modules/career internal/types/interfaces internal/container internal/router   # clean
go test ./internal/modules/career/... -count=1        # ok  12.638s
go test ./internal/modules/workbench/service/workbench/... -count=1  # ok 21.258s
go test ./internal/database/... -count=1              # ok  18.589s
go test ./internal/router/... -count=1                # ok   2.684s
go test ./tools/architectureguard/... -count=1        # ok   1.554s
git diff --check                                      # CLEAN
```
（附加 `go build ./...` 通过，仅基线同款 -lc++ 链接警告。）

## 文件清单

新增：internal/modules/career/career_export.go + career_export_test.go；internal/modules/workbench/service/workbench/application_task_removal.go + application_task_removal_test.go；migrations/versioned/000205_*.up/.down.sql；migrations/sqlite/000126_*.up/.down.sql。
修改：career/office.go、handler.go、handler_test.go；router/routes_career.go + routes_career_test.go；internal/types/interfaces/career_application_task.go；internal/container/container.go；internal/database/career_migration_test.go；tools/architectureguard/discovery_test.go。

## 自查与遗留

- 删除测试全部使用 task 级临时 SQLite（t.TempDir()），未触碰持久库（全局约束）。
- 隔离数据库、参数绑定（无 SQL 拼接；迁移测试中 pragma 查询参数化）、scope 从认证上下文派生——均已满足。
- 已知局限（详见任务报告）：导出同步内联未走文件存储；删除步骤间非单事务（外部调用在事务外，由 durable state 兜底）；partial 后新 requestId 另起审计行。
- Web 子任务合同：POST/GET /career/exports(/receipt)、GET /career/deletions/boundary、POST/GET /career/deletions(/receipt)；receipt/boundary JSON 结构见任务报告"接口冻结"节。

## 第 1 轮评审修复（R1）

### F1（critical）：迁移 SQL 被 .gitignore 吞掉未进提交 — 已修复

- 根因核实：`git check-ignore -v migrations/versioned/000205_career_exports_deletions.up.sql` → `.gitignore:96:migrations/`；`git ls-files` 尾列为 000204/000125（先例即被强制添加入库）。磁盘 4 文件存在但 51b94cd46 未跟踪它们。
- 修复：`git add -f` 4 个迁移文件，追加提交 `86c696618 fix(career): force-add T22 migrations ignored by .gitignore`（内容零改动，与评审时磁盘字节一致——`git status` 干净 + `git diff HEAD` 为空证明）。
- 提交证据：`git show --stat HEAD` 尾部含 4 个 .sql（000126 up 31 行/down 2 行、000205 up 34 行/down 2 行，共 69 insertions）；`git ls-files migrations/versioned|sqlite` 尾列变为 000205/000126。
- 测试有效性推理：工作区干净（无未提交改动）⇒ 下述重跑就是对提交内容的测试。

### R1 重跑验证（全部实跑）

```
$ gofmt -w internal/modules/career internal/types/interfaces internal/container internal/router
$ gofmt -l internal/modules/career internal/types/interfaces internal/container internal/router \
        internal/modules/workbench/service/workbench internal/database
（无输出）
$ go test ./internal/database/... -count=1
ok  	github.com/Tencent/WeKnora/internal/database	16.891s
$ go test ./internal/modules/career/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/career	10.948s
$ go test ./internal/modules/workbench/service/workbench/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench	19.895s
$ go test ./internal/router/... -count=1
ok  	github.com/Tencent/WeKnora/internal/router	1.923s
$ go test ./tools/architectureguard/... -count=1
ok  	github.com/Tencent/WeKnora/tools/architectureguard	0.522s
$ git diff --check
（无输出，退出 0）
```

### low 项处置（记录不修）

- F2（DeleteCareer 并发竞态回放可能返回 `ReceiptBody:"{}"` 解码出的空回执）：确认存在但方向 fail-safe（不虚报 deleted）；属罕见并发窗口的回执形态问题，留待后续任务细化（如对 status=deleting 的行返回 202/进行中语义）。
- F3（failDeletionStep 测试 seam 无赋值点）：确认死代码钩子（partial 失败实际由 fakeCareerTaskRemover.failErr 驱动）；保留 hook 但更正报告表述——它不是本测试套件的使用点，若后续无人使用应删除。
- F4（无单一 Office+Identity+Workbench 真实联合测试）：确认现状为 fake remover + coordinator 独立测试 + handler 合同测试的组合覆盖；与 T14 fake linker 先例同型，联合合同测试形态留待集成验证阶段。

### 提交历史（R1 后）

- `51b94cd46` feat(career): export and completely delete job search data
- `86c696618` fix(career): force-add T22 migrations ignored by .gitignore（HEAD）
