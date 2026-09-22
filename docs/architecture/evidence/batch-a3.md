# Pass A Batch A3 Integration Evidence（IA3，核心模块）

集成线：`backend-mod-passa`，基线 `918f90000`（batch a2 终版）。接入顺序严格为 Knowledge → Conversation → Agent Runtime（计划固定）。

## 1. 集成顺序与提交

| 模块 | worker 分支终版 | merge 提交 | composition 切换提交 |
|---|---|---|---|
| A9 knowledge | fb71b3084（含 fix round 1：retriever service 对齐 manifest 根路径） | 790be67fe | 见 ae37070e7 前：knowledge 切换提交 |
| A10 conversation | c2920997f | 17bbd2fe2 | 33c23e288 |
| A11 agentruntime | b6fd1b6e1 | 1812127bd | ae37070e7 |

屏障提交：
- `45fc6f9b1` guard 例外：batch-a3 新暴露 **76 条**预存耦合（精确路径：B-agentruntime×53 / B-conversation×17 / B-knowledge×4 / B-channels×2），全部为搬迁显形的横向互引，通配禁用
- `e24e67d04` importer 侧 lint 修复 15 项（lll nolint/换行、revive 文档注释）
- 集成中追加 1 条例外（channels/im/service.go → knowledge/docparser，翻转 docparser import 后显形）

## 2. 合并冲突（均为 A 系 import 修复相交，按"新路径并集/取 HEAD 新路径"解决）

- A10×knowledge 切换（1 文件）：extract.go（chatpipeline×retriever 双行并集）
- A11×K/C 集成（2 文件）：extract.go、session_knowledge_qa.go（A11 分叉早于 K/C 集成，其旧路径行弃用，取 HEAD 新路径；extract.go/session_knowledge_qa.go 各残留 1 条旧 agent/tools 行删除）

## 3. 关键裁决

- **A9 retriever 布局（Critical→已修）**：`service/retriever` 曾落位 `retriever/service/`，与 manifest `to: internal/modules/knowledge/retriever` 不符；控制器裁定 manifest 布局合法（retriever/ 根放 service 包文件 + doris/ 等子目录并存），代码对齐（fb71b3084，12 文件 R100 上移 + 24 处 import 重写 + 3 处文档更正），re-review ADDRESSED。
- **lint 债（MOVED 文件）**：变更范围扩大使 13 个预存 lint 问题（lll/revive/unused）在纯改名文件（internal/modules/agentruntime/**）中显形；控制器裁定不加 nolint（保 R100 纯移动证据链），记为在册 lint 债，Pass B B-agentruntime 收敛。importer 侧 15 项已修（e24e67d04）。

## 4. 门禁结果

- `go build ./...` exit 0（每模块切换后即时 + 终态）
- `go run ./tools/architectureguard`：OK（0 violations；633 routes = 564+69 / redis 23 / lite 23 / hooks 58 / modules 16 —— 与 F0 零漂移）
- `go run ./tools/modulemove verify --all`：OK（16 manifests）
- `golangci-lint run --new-from-rev=918f90000 ./...`：非 modules 文件 0 issues；internal/modules/** 13 条在册 lint 债（见 §3）
- 旧路径残留：全仓 grep 零命中（A9-A11 全部别名目录已删：knowledge 18 + conversation 1 + agentruntime 4 + retriever/tree 等）
- `go test ./internal/... -count=1 -timeout=25m`：见 §5
- 高风险面覆盖：knowledge 删除/索引（knowledge 包测试）、session/message 流式（handler/session + chat_pipeline characterization，A10 记录）、tenant 访问（RBAC 中间件测试）、agent run/tool/approval/recovery（agentruntime 包 + race 24.7s 0 DATA RACE，A11 记录）、Redis/Lite worker 一致（guard 双侧计数 + parity）

## 5. 批次结论

- `go test ./internal/... -count=1 -timeout=25m`（提交态 e24e67d04，2026-09-21）：**119 ok / 0 FAIL / EXIT=0** —— 完全干净，连续第二个批次优于 F0 基线（在册 unstable 均未触发）。

Knowledge → Conversation → Agent Runtime 按固定顺序接入完成；76+1 条预存耦合全部以精确路径例外在册；13 条 moved 文件 lint 债在册（B-agentruntime）；全部门禁不劣于基线，批次放行 Parallel Group A4（A12–A14）。

