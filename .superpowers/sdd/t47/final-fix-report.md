# T47 最终修复报告（整计划最终审查 3 项 minor 发现，一次批次全修）

- Worktree：`.worktrees/issue30-sweep-t47`（分支 `codex/issue30-t47`）
- 修复基线：进场 HEAD `289dd0861`（worktree clean，实跑 `git status` 确认）
- 修复范围：整计划最终审查 3 项发现，全部位于 `internal/handler/session/workbench_research.go`，**3/3 全部修复，零未修项**
- 结论：每项修复附新增回归测试与实跑输出；受影响包全量测试全绿（session 包 ok）、`go vet` 干净、`go build ./...` 通过。所有证据均为本会话亲自实跑。
- 历史批次：上一轮（mobile 2 项 important 发现）报告归档于同目录 `final-fix-report-mobile-round.md`（基线 `74e9c8959`，其修复已在提交历史中）。

---

## 发现 1（minor）：`researchDelegateInput.AgentID` 声明但从未使用（dead wire 字段）

审查原文：`internal/handler/session/workbench_research.go:168` ——「计划原文如此，委派由写 Run 内消化不绑定具体 agent；可在 #71 消费前清理或补语义」。

### 核实（本会话实读/实跑）

- 修复前代码（本会话 Read `workbench_research.go` 旧版 :165-169）：

  ```go
  type researchDelegateInput struct {
      Objective string   `json:"objective"`
      Sources   []string `json:"sources"`
      AgentID   string   `json:"agent_id"`
  }
  ```

  `AgentID` 在 `DelegateResearch`（旧版 :174-219）中无任何读写——纯 dead wire 字段。
- 计划原文确带此字段：`docs/plans/issue30-sweep/plans/plan-t47.md:1255`（`grep -n "agent_id\|AgentID" plan-t47.md` 唯一命中），且计划同样未消费它——是计划稿的复制残留而非语义要求。
- 消费面核查（本会话实跑 grep）：
  - 前端无引用：`grep -rn "research" --include="*.ts" --include="*.tsx" --include="*.vue" --include="*.js" -il web/` → 零文件；
  - spec/plans 无契约要求：`grep -rn "agent_id" docs/specs/ docs/plans/ | grep -i "research\|delegat"` → 零命中；
  - 仓库内 `DelegateResearch` 引用仅三处：handler 自身、其测试、路由（`internal/router/routes_workbench.go`）；`internal/application/repository/task_research_http_test.go` 中无 `agent_id`。

### 修复

删除 `AgentID` 字段，并留注释说明删除理由与兼容性（`internal/handler/session/workbench_research.go:165`）：```go
type researchDelegateInput struct {
	Objective string   `json:"objective"`
	Sources   []string `json:"sources"`
	// NOTE: the plan's original draft also carried an agent_id field, but a
	// delegation is consumed inside the owning write run and never binds to
	// a specific agent, so the field was dead wire — removed. Clients that
	// still send agent_id are unaffected: gin's JSON binding ignores
	// unknown keys.
}
```

选择「清理」而非「补语义」的理由：委派由写 Run 内消化、不绑定具体 agent 是审查确认的既定语义；补一个读取该字段的假语义反而制造新的假承诺。行为兼容性：gin `ShouldBindJSON` 默认忽略未知 JSON 键，删除字段前后客户端发送 `agent_id` 的请求行为完全一致（都被忽略、正常 201）——由下方回归测试锁定。

### 回归测试与输出

新增 `TestDelegateResearchIgnoresLegacyAgentIDKey`（`internal/handler/session/workbench_research_test.go:253`）：请求体携带 `agent_id` 键，断言 201 且委派行内容不受影响。

```
$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestAnnotateMaterial' -v | grep -E '^(=== RUN|--- PASS|--- FAIL|ok)'
=== RUN   TestDelegateResearchPersistsReadOnlyDelegation
--- PASS: TestDelegateResearchPersistsReadOnlyDelegation (0.00s)
=== RUN   TestDelegateResearchRejectsSourceOutsideTenantScope
--- PASS: TestDelegateResearchRejectsSourceOutsideTenantScope (0.00s)
=== RUN   TestDelegateResearchRequiresObjectiveAndSources
--- PASS: TestDelegateResearchRequiresObjectiveAndSources (0.00s)
=== RUN   TestDelegateResearchIgnoresLegacyAgentIDKey
--- PASS: TestDelegateResearchIgnoresLegacyAgentIDKey (0.00s)
...
ok  	github.com/Tencent/WeKnora/internal/handler/session	0.848s
```

---

## 发现 2（minor）：`AnnotateMaterial` 的 BaseVersion 比较前未 TrimSpace

审查原文：`internal/handler/session/workbench_research.go:327` ——「MaterialID 在 :310 有 trim；带空白的 base_version 得 409 而非 400，fail closed 方向正确，行为可接受」。

### 核实（本会话实读）

- 旧版 :310 仅 `input.MaterialID = strings.TrimSpace(input.MaterialID)`；
- 旧版 :327 `if artifactVersionOf(*matched) != input.BaseVersion` 直接比较未 trim 的 `input.BaseVersion`；`artifactVersionOf`（`workbench_artifacts.go:77`）内部对 ContentHash 做了 trim 后取前 16 位，返回值无空白——因此「`" 9a2f…\n"` 形式的当前版本号」会误判为 stale 而 409。

### 修复

与 MaterialID 的 trim 并列（`internal/handler/session/workbench_research.go:315`）：

```go
input.MaterialID = strings.TrimSpace(input.MaterialID)
input.BaseVersion = strings.TrimSpace(input.BaseVersion)
```

效果：带空白的当前版本号从 409 变为 201，且落库的 `BaseVersion` 是 trim 后的干净版本身份；真正 stale 的版本仍 409（既有测试 `TestAnnotateMaterialRejectsStaleBaseVersion` 锁定）。fail closed 语义未被削弱——空白只是传输噪声，不是版本漂移。

### 回归测试与输出

新增 `TestAnnotateMaterialTrimsBaseVersionBeforeCompare`（`workbench_research_test.go:366`）：`base_version` 带前后空白发送，断言 201、落库值为 trim 后的 `9a2f1c3d4e5f6a7b`。

```
=== RUN   TestAnnotateMaterialTrimsBaseVersionBeforeCompare
--- PASS: TestAnnotateMaterialTrimsBaseVersionBeforeCompare (0.00s)
```

（同命令全量输出见发现 1；stale 409 回归 `TestAnnotateMaterialRejectsStaleBaseVersion --- PASS` 亦在同轮输出中。）

---

## 发现 3（minor）：body 空值校验发生在 annotation 结构体构造之后

审查原文：`internal/handler/session/workbench_research.go:331-339` ——「顺序略反直觉；400 仍在任何落库之前，行为正确」。

### 核实（本会话实读）

旧版顺序：`annotation := types.TaskArtifactAnnotation{…}`（:331）构造在先，`if strings.TrimSpace(input.Body) == "" { 400; return }`（:336-339）在后——先构造注定丢弃的结构体，再校验。

### 修复

校验整体上移到构造之前（校验 `workbench_research.go:336-339`，构造 `:340`），构造后直达 `CreateAnnotation`。选择「紧邻构造点前移」而非「上移到 bind 之后」的保守位置：404（material 未命中）/409（版本冲突）/400（body 空）的相对优先序保持与修复前完全一致，零行为变化，纯顺序整理。

### 回归测试与输出

新增 `TestAnnotateMaterialRejectsBlankBodyBeforePersist`（`workbench_research_test.go:379`）：body 全空白，断言 400 `research_invalid_request` 且零落库。

```
=== RUN   TestAnnotateMaterialRejectsBlankBodyBeforePersist
--- PASS: TestAnnotateMaterialRejectsBlankBodyBeforePersist (0.00s)
```

---

## 全量回归（本会话实跑）

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| research 全部 handler 测试（含 3 项新增） | `go test ./internal/handler/session/ -run 'TestDelegateResearch\|TestAnnotateMaterial\|TestListResearch\|TestCompleteResearch\|TestListAnnotations' -v` | 全 PASS，`ok github.com/Tencent/WeKnora/internal/handler/session` |
| session 包全量 | `go test ./internal/handler/session/` | `ok github.com/Tencent/WeKnora/internal/handler/session 46.472s` |
| 静态检查 | `go vet ./internal/handler/session/` | 无输出（通过） |
| 全仓构建 | `go build ./...` | 通过（仅 `cmd/server`/`cmd/desktop` 链接器重复库 `-lc++` 警告，与本修复无关，修复前已存在） |

## 变更清单

- `internal/handler/session/workbench_research.go`：+10 −5（删 dead 字段 + BaseVersion trim + body 校验前移）
- `internal/handler/session/workbench_research_test.go`：+40（3 个回归测试）
- 本报告 + 上一轮报告归档 `final-fix-report-mobile-round.md`
