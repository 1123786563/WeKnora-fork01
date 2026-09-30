# Pass B IB1 基础能力集成屏障计划

> **For agentic workers:** 本计划是 barrier 节点 `ib1`（`execution-dag.json`）的实施计划，由集成工程师独占执行。共享装配文件（router/container/bootstrap 全局注册/migration 序列/go.mod）为集成工程师禁改独占区（conventions §3/§4）。

**Goal:** 把 B1 四支（B1-ID→B1-AI→B1-CM→B1-EX）逐支合并进 `codex/passb-integration`，校验全库门禁与计数奇偶，登记本屏障拥有的例外状态；为 B2/B3/B4 冻结可消费的 B1 四门面。

**Architecture:** 依据框架计划 `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` IB1 节（:100-107）：一次一支串行合并；共享装配仅从**已评审的 Integration Brief** 切换；运行 tenant/RBAC、capability、payment/usage、sandbox/target 差分套件；冻结 B2/B3/B4 消费的具体门面并更新例外台账；跑 architecture guard、build、全量 internal 测试、变更域 lint、计数奇偶。

**Tech Stack:** Go 1.26, git merge（ort），make 门禁，Testify。

## 0. 现实基线（本屏障派发时实测，2026-09-23）

B1 四节点的**实施产出均止于子计划文档，无生产代码变更**（调度台账 20:07/20:14/20:17/20:22 各条目留痕，ib1 派发条目 20:23 亦确认）：

| 支 | 分支 | head | 实施产出 |
|---|---|---|---|
| B1-ID | `codex/passb-b1-identity` | `8db61f8ae` | `docs/plans/passb/10-identity.md`（500 行） |
| B1-AI | `codex/passb-b1-airesource` | `d57a2fa70`（=base，零提交） | `11-airesource.md` **untracked 未入库**（其 worktree 内） |
| B1-CM | `codex/passb-b1-commercial` | `4e481a63b` | `docs/plans/passb/12-commercial.md`（676 行） |
| B1-EX | `codex/passb-b1-execution` | `0044f54fc` | `docs/plans/passb/13-execution.md`（331 行） |

四模块 `internal/modules/{identity,airesource,commercial,execution}/module.go` 仍为零逻辑注释骨架（仅声明 NewModule/RegisterRoutes/RegisterWorkers/Start/Stop 契约，无实现无导入——DAG ib1 notes F2 裁定口径）。因此：

- **本屏障可执行**：逐支合并（计划入库）、模块与直接消费者测试、全库门禁、计数奇偶、例外现状核验与登记；
- **本屏障不可执行（依赖 B1 实施提交落地，须后续补足，见 §4）**：router/container/全局注册装配切换、contracts.yaml 四门面 current 化、exception-ledger `remove_at: ib1` 删行、四组差分套件。这些步骤在本屏障内**跳过而非伪造**——把零逻辑骨架接入装配或把 planned 门面伪称 current 都会破坏 633/23+23/58 计数契约与行为等价性。

## 1. 逐支合并（一次一支，已执行）

| 序 | 支 | 合并提交 | 结果 | 合并后验证（build + 模块/直接消费者测试 `-count=1`） |
|---|---|---|---|---|
| 1 | B1-ID | `1900e038a` | +`10-identity.md` | `go build ./...` OK；`go test ./internal/modules/identity/...` ok（骨架包 no test files） |
| 2 | B1-AI | （no-op，"Already up to date"） | 无变化 | `go test ./internal/modules/airesource/...` 12 包（10 ok + 2 no test files）；消费者：agentruntime 20 ok、conversation 1 ok、knowledge 15 ok |
| 3 | B1-CM | `f66dfb188` | +`12-commercial.md` | `go test ./internal/modules/commercial/...` 7 包 ok；消费者：appconnector+airesource-chat+workbench 10 ok、agentruntime/agent 族 18 ok |
| 4 | B1-EX | `ca38afb7e` | +`13-execution.md` | `go test ./internal/modules/execution/... ./internal/modules/policy/...` 8 包 ok；消费者：agentruntime/agent/tools 1 ok |

直接消费者名单依据：`docs/architecture/passb/exception-ledger.yaml` 中 `from` 指向对应模块的在案耦合（agentruntime→airesource/commercial/execution、conversation→airesource、knowledge→airesource、appconnector→commercial、workbench→commercial、airesource/models/chat→commercial、execution/sandbox→policy/ipclass）。identity 无 ledger 在案消费者（最底层公共用例）。

**冲突裁定**：四支合并零冲突（ort 自动合并；各支仅新增不相交的计划文件，符合 framework:95 不相交前提）。b1-airesource 的 untracked `11-airesource.md` 不属于已审查分支内容，本屏障不代为提交，维持在其 worktree 原状（调度台账 20:07 条目已建议提交入库，待调度方/实施者动作）。

## 2. 门禁与计数奇偶（已执行）

- `go build ./...`：每支合并后 exit 0（仅 cmd/desktop、cmd/server 既有 `-lc++` 重复库链接警告，非错误）；
- `make check-backend-architecture`：见台账本屏障条目（预期 `OK (0 violations)`，基线 633/23+23/58）；
- `make verify-module-moves`：见台账本屏障条目（预期 `modulemove: OK (16 manifests verified)`）；
- `make check-passb-readiness`：防措辞/治理回归（`go run ./tools/passbguard -root .`）。

## 3. 例外台账状态（已核验）

`remove_at: ib1` 的例外共 2 条，其 import **均仍在磁盘**（B1 实施未落地，删除条件未满足），本屏障**不删行**：

| 例外 | import 位点 | 现状核验 |
|---|---|---|
| exc-0057 | `internal/modules/airesource/models/chat/usage.go:9` → `internal/modules/commercial` | import 在（plan 11-airesource） |
| exc-0087 | `internal/modules/execution/sandbox/url_guard.go:31` → `internal/modules/policy/ipclass` | import 在（plan 13-execution） |

## 4. 待 B1 实施落地后的屏障收尾动作（后续 barrier 动作，不在本次范围）

1. **装配切换**（router.go/container.go/bootstrap/{routes,workers,lifecycle}.go，集成工程师独占）：按各 B1 子计划 §Integration Brief（10/11/12/13-*.md）评审通过后，将四模块 NewModule/RegisterRoutes/RegisterWorkers/Start/Stop 接入全局装配；每支一次一支、切换后立即跑该模块+消费者测试与计数奇偶；
2. **contracts.yaml current 化**：四门面（identity.facade/airesource.facade/commercial.facade/execution.facade 及配套 capability-port）实现落地后从声明态转 current（consumers/characterization_tests 回填）；
3. **例外删行**：exc-0057、exc-0087 对应 import 被"消费方窄端口 + 旧路径桥接适配器"替换后删除行（`tools/modulemove` 与 passbguard 校验通过为前提）；
4. **差分套件**：tenant/RBAC（B1-ID）、capability（B1-AI）、payment/usage（B1-CM）、sandbox/target（B1-EX）特征化与差分证据，各支实施时产出，本屏障汇总核验；
5. **全量门禁补跑**：`go test ./internal/... -count=1 -timeout=25m` 与 `golangci-lint run --new-from-rev="$PASSB_BASE_SHA" ./...`（DAG ib1 gates 全集），在装配切换发生的那次屏障收尾执行。

## 5. 验收标准

- [x] 四支按 B1-ID→B1-AI→B1-CM→B1-EX 顺序逐支合并，零冲突；
- [x] 每支合并后 `go build ./...` + 模块与直接消费者测试绿；
- [x] `make check-backend-architecture` / `make verify-module-moves` / `make check-passb-readiness` 绿，计数 633/23+23/58 奇偶不漂移；
- [x] exc-0057/exc-0087 现状核验并登记（不删行）；
- [x] 本计划文件写盘（本文件）；
- [ ] 装配切换/契约 current 化/例外删行/差分套件/全量 25m 测试+变更域 lint —— **依赖 B1 实施落地，本屏障如实上报缺位**（调度台账同步登记）。
