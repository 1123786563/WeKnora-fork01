# agentcatalog — Pass A 证据（Task A7）

分支 `bm-passa-a7`（worktree `.worktrees/bm-passa-a7`，基线 03032f890 "refactor:
integrate pass-a batch a1"）。Manifest：`docs/architecture/moves/agentcatalog.yaml`。
**结论：零搬迁模块** —— manifest `move_packages: []` / `alias_obligations: []` /
`owned_files.move_sources/move_targets/importers: []`，与 F0
`backend-modules.yaml:281`（`packages: []`）一致。行为零变更：本分支除两份文档外
未改任何文件。

## 1. Gate

- 搬迁前：`go run ./tools/modulemove verify --module agentcatalog` → `modulemove: OK (agentcatalog)`（exit 0）
- 结束时同命令复跑 → `modulemove: OK (agentcatalog)`（exit 0）

## 2. 基线测试（manifest test_commands 全量，仅 1 条）

```
go test ./internal/modules/agentcatalog/... -count=1
?   	github.com/Tencent/WeKnora/internal/modules/agentcatalog	[no test files]
EXIT=0
```

模块仅有 Pass A 骨架（README/module.go/legacy 索引），无测试文件；搬迁前后命令与
结果完全相同（pre == post：零搬迁）。直接消费方测试不适用（无 importers，
`owned_files.importers: []`）。

已知 flaky（F0 §2.4 / 基线 §2.5，均归 agentruntime / commercial，A7 未运行这些包）：
`internal/agent/opencode` 套件超时、`TestAgentRunDecisionConcurrentOnlyOneRevision`
并发偶败、payment `TestProvidersFromEnvRejectsPartialAlipay` 断言消息非确定性。

## 3. 零搬迁证明（简报 "move experts/persona/subagents/skills/catalog" 的裁定）

- manifest（绑定，KnownFields 严格加载）：move_packages 为空列表 —— F1 覆盖恒等式
  （moves/README.md「所有 manifest 之并 = F0 清单」）在 F0 对 agentcatalog 声明
  `packages: []` 的前提下自洽，说明这是设计而非遗漏。
- `internal/agent/{experts,persona,skills,subagents}` 全部出现在
  `docs/architecture/moves/agentruntime.yaml` 的 move_packages
  （→ `internal/modules/agentruntime/agent/...`，A11 领土）；`internal/agent/catalog`
  不存在。A7 未触碰 `internal/agent` 下任何文件（`git status` 全程仅两份新文档）。
- 本模块业务实现 = 55 个 legacy_files（repository 11 / service 31 / handler 13，
  Pass B `B-agentcatalog`）。

## 4. 构建 / 静态检查

- `go build ./...` → exit 0（仅 cmd/server、cmd/desktop 既有 ld "duplicate libraries"
  链接警告，与本任务无关）
- `go vet ./internal/modules/agentcatalog/...` → exit 0
- `go run ./tools/architectureguard` → `literal=564 apiKeyRoute=69 handle=0 total=633 |
  redis=23 lite=23 | hooks=58 | modules=16`，`OK (0 violations)` —— 与基线完全一致，
  **无新增禁改违例、无需记录暴露耦合**
- `go run ./tools/modulemove verify --module agentcatalog` → OK（两次）

## 5. 纯改名证据 / 禁改文件

- 无 MOVE / COMPILE-REPAIR 提交（零搬迁），故无 `git diff --summary` rename 清单、
  无别名包、无 import 修复。
- 禁改文件未触碰证明：`git diff --name-only 03032f890..HEAD -- internal/router/router.go
  internal/router/task.go internal/router/sync_task.go internal/container/container.go
  go.mod go.sum migrations/` → 空输出。
- 分支全部改动 = 新增 `docs/architecture/integration/agentcatalog.md` 与本文档。

## 6. 集成点复核（Integration Brief 输入，均在基线逐条验证）

- 11 个路由入口行号与 manifest 一致（routes_agent.go:22/61/76、routes_agent_versions.go:29、
  routes_agent_marketplace.go:21、routes_persona.go:24、routes_expert.go:21、
  routes_subagent.go:26、routes_skill_market.go:32、routes_tenant_skill_market.go:31、
  routes_tenant_expert_market.go:35）；逐入口统计 literal+apiKeyRoute 合计 **55 条**，
  与 F0 `by_owner.agentcatalog: 55` 精确吻合。
- 生命周期挂点 `startTenantSkillReaper`：container.go:702（Invoke）/ 2405（func 签名），
  与 manifest 一致。

## 7. 提交

| 提交 | 主题 |
|---|---|
| （本文档 + integration brief） | `docs(agentcatalog): record zero-move pass-a outcome with integration brief` |

## 8. Review 结果

**Approved**（final whole-branch review 回填；Review 范围：task-scoped gate review + batch barrier review）。
pass-a 全部 task 均 Approved、四道 barrier 均 Approved；唯一修复轮次为 A9 knowledge——retriever 路径对齐 fb71b3084，复 review ADDRESSED。
结论与证据台账见 `.superpowers/sdd/2026-09-21-backend-modularization-foundation-pass-a/progress.md` 与 `docs/architecture/evidence/batch-a*.md`。
