# Pass B Brief — B-execution（Sandbox/Target/Workspace/Terminal/Browser 面共享宿主裁定）

> child plan：**13-execution**（B1-EX；B0.3 裁定标注，framework:71）。
> Manifest：`docs/architecture/moves/execution.yaml`（模块 execution）。
> 本文在 B0 冻结阶段登记 execution 模块在共享宿主包 `internal/handler/session`
> 内的逐文件精确属主与 `*Handler` 去方法化裁定；完整拆分义务
> （Sandbox/Target/Workspace/Terminal/Browser façade、21 个 legacy 文件、别名与
> 例外删除、特征化测试与差分证据）由 `docs/plans/passb/13-execution.md` 子计划
> 承接，本 brief 只冻结边界事实，不新增行为。

## Scope（共享宿主裁定，B0.3 Step 3）

**迁入（`internal/handler/session/` 内独占归 13-execution 的文件）**：

- `internal/handler/session/browserskill.go`（浏览器技能会话面）
- `internal/handler/session/sandbox_terminal_bridge.go`（终端桥接）
- `internal/handler/session/sandbox_terminal_ws.go`（终端 WebSocket）

三文件在共享 host 包内（plurality owner conversation，handler.go:24 `*Handler`）；
`internal/handler/session` 的其余文件分属 conversation-session（35）、
agentruntime-engine（33）、agentruntime-protocol（34）、knowledge-wikifaq（23）、
workbench（40）、craft（41），见 ownership-matrix.yaml 逐路径唯一属主。

## 边界目标

浏览器/终端会话能力随 execution 模块拆分收进 `internal/execution/`，
共享 host 包不再残留 execution 归属文件。

## 义务（B0 冻结口径）

1. **\*Handler 去方法化（B0.3 Step 3 + IB1 裁定，DAG b1-execution
   required_contracts）**：上述三文件挂在 conversation 属主 `*Handler`
   （handler.go:24，67 字段）上的方法，拆出时必须去方法化——自带
   receiver/独立构造，不得继续给共享 `*Handler` 增方法；确无法当场消除的
   残差登记 Integration Brief 推迟 IB1 收口，禁止遗留为 conversation handler
   的常驻方法（Handler 拆分裁定随 IB1 Integration Brief 执行）。
2. **行为零变化**：浏览器/终端会话端点的 wire 语义（WebSocket 协议、
   sandbox 生命周期、bridge 超时）为外部契约，拆分不改行为；差分证据按
   conventions §7 高风险面（session stream 邻接面）执行。
3. **验证**：`go test ./internal/execution/... -count=1`、
   `go build ./...`、`make check-backend-architecture` 全绿；route/worker/hook
   计数与 F0 基线一致（633/23+23/58）。
