# CFT-S05-T033 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_release_gate_test.go`（新增，2 tests，`TestCraftReleaseGate*`）：Gate 矩阵 pin（真实 store 全链）。
- `docs/craft/release-gates.md`（新增）：任务卡建议的灰度/回滚文档（四层 Gate、关闭语义、回滚步骤、商业门禁现状）。
- 实跑 `scripts/check-craft-release.py`（O05 商业门禁检查器）：**因证据 HEAD 与当前 HEAD 不符正确拒绝（exit 1）**——这本身就是"未过商业门禁不得收费开放"的活证据。

## 验收断言对照

- 关闭某类型后不接受新任务 ✓（`ClosedKindRefusesNewKeepsPublished` 前半：closed gate 的 Allows=false——服务端 Create/StartRun 强制（T005 HTTP pin：503）；UI 侧禁用+原因（T009）只是体验层）
- 已发布可读版本不因关闭类型删除 ✓（后半：真实 store 发布 v1 后，closed kinds 下 GET 同版本照常——version store 读取无 kind 维度；数据删除只属生命周期保守清扫（T031），与 Gate 无关）
- 活动 Run 按排空/取消明确策略 ✓（`ClosingAdmissionDrainsNotKills`：kinds 全关时 Allows=false（新任务拒）而读取路径不经闸（View/List 照常）；drain 语义由 admission_enabled 闸 + 生命周期套件（T031 排空/重驱动）+ T023 权威取消（不静默 kill）承载）
- 未过商业门禁不得收费开放 ✓（release-checker-refuses.log：证据 HEAD 不符 → exit 1 拒绝放行；检查器还强制 26 任务依赖祖先、场景证据非空含退出码、SQLite/Cube/E2B 独立能力、已知缺口必列 limitations——无证据的能力不存在）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftReleaseGate -count=1` | 0 | 2 PASS |
| `python3 scripts/check-craft-release.py docs/testing/craft/release-evidence.json` | **1（预期拒绝）** | head 不符——生产放行需在发布 commit 重新生成证据 |
| `go test -count=1 ./internal/application/service/ -run "TestCraftReleaseGate\|TestCraftHTTPCapabilities"` | 0 | ok |

## 未验证事项

- 生产环境灰度演练（真实部署的 KINDS 切换）不在本仓库模拟范围——`release-gates.md` 记录操作步骤；S05 不等于生产部署授权。

## 回退

revert 本提交（测试+文档，纯增量）。
