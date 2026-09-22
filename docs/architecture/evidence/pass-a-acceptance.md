# Pass A 验收证据（IA4 Acceptance）

分支：`backend-mod-passa`，验收基点 F0 `7b1d3cf017a50cc8061da166d67befc6b51773a0`（main），IA4 终态见 git log。
验收日期：2026-09-21。Spec：docs/specs/2026-09-21-backend-domain-module-reorganization-design.md §17.1。

## 1. 交付完整性（Spec §17.1 逐项）

| 验收项 | 结果 | 证据 |
|---|---|---|
| 16 模块均有所有者、README、module.go、代码地图、迁移台账 | ✅ | internal/modules/<id>/{README.md,module.go,legacy/README.md} ×16（F1 35ee7e088 起，各 A 任务维护）；归属 docs/architecture/backend-modules.yaml |
| 所有服务端生产文件与测试都有目标模块归属 | ✅ | F1 覆盖恒等式：manifests 并集 + platform/bootstrap 残留（moves/README.md）= F0 库存 128 包；F0 已审 |
| 天然成块代码物理归位，剩余 legacy 有精确清单 | ✅ | 14 个 A 任务搬迁：A1 5 包/65 文件、A2 7/116、A3 12/97、A4 10/99、A5 3/142、A6 12/188、A7 0（真空）、A8 5/25、A9 18/142、A10 1/47、A11 20/371、A12 4/36、A13 1/47、A14 1/13；legacy_files 396 条全部带 passb_task（manifests） |
| router/container/Worker 按模块注册且无重复 | ✅ | architectureguard 唯一注册检查 0 violations；composition 全部指向 internal/modules/*（各 IA 证据） |
| 新业务代码不再进入旧横向业务目录 | ✅ | F2 legacy guard（service/repository/handler 三目录 + manifest 允许集）0 violations |
| 所有 Integration Barrier 构建/测试/守护/Review 不劣于基线 | ✅ | IA1 118 ok（+2 在册 unstable）/IA2 119 ok/IA3 119 ok/IA4 119 ok 全绿；四份 barrier review 均 Approved |
| cmd/desktop、docreader、client/SDK 未被改造 | ✅* | 零逻辑变更；4 个 cmd main.go（server/desktop/connector-control/saas-migrate）仅 import 路径行维护（1-2 行/文件，随内部包搬迁的机械性依赖修复，逐行核验纯 import）——controller 裁定：这不构成"改造" |

## 2. 基线比对（F0 → 终态）

| 指标 | F0 | 终态 | 判定 |
|---|---|---|---|
| 路由注册 | 633（564 literal + 69 apiKeyRoute） | 633（同分解） | ✅ 零漂移 |
| Redis/Lite worker | 23 任务类型 + 6 池 / 23 | 同 | ✅ 集合一致（guard 双侧计数） |
| container.Invoke hooks | 58 | 58 | ✅ |
| migrations | 537 文件（270 assets） | 537，git diff 零变更 | ✅ |
| 生产包 | 128（cmd/server + internal） | 139（F0 128 包全数迁移/索引 + 16 新模块根包 + internal/bootstrap，均有归属） | ✅ |
| 测试 | 118 ok + 2 flaky 注册 | 119 ok / 0 FAIL | ✅ 优于基线 |

## 3. Pass A 期间新增的治理资产

- `tools/modulemove`（严格 manifest 校验 + 已集成搬迁语义）与 `tools/architectureguard`（资产发现/唯一注册/跨模块禁导入/legacy 新文件拒绝），Make 目标 `verify-module-moves`、`check-backend-architecture`。
- guard 精确路径 import 例外共 **105 条**在册（a1:4 / a2:6 / a3:77 含 IA3 集成 1 条 / a4:18），全部绑定 Pass B 任务（B-appconnector/B-airesource/B-channels/B-execution/B-knowledge/B-conversation/B-agentruntime/B-workbench/B-craft），无通配。
- 冻结契约：docs/architecture/frozen-entrypoints-batch-a2.md（A9–A14 消费面）。
- Pass B 简报：docs/architecture/passb/（knowledge×4、conversation×2、agentruntime×4、workbench/craft/insights 各 1 + manifests 内 alias_obligations/legacy_files 义务）。

## 4. 在册债务与遗留（Pass B 输入）

- 13 条 moved 文件 lint 债（internal/modules/agentruntime/**，lll/revive/unused，预存显形；裁定不加 nolint 以保 R100 证据链）→ B-agentruntime。
- 预存横向耦合 105 条（见 §3）→ 对应模块 Pass B 边界收紧时改走公开门面后删除例外。
- A2 payment `TestProvidersFromEnvRejectsPartialAlipay`（map 序断言）→ B-commercial 修测试。
- system housekeeping hook 实现文件归 knowledge → B-knowledge/B-system 协调。
- 各模块 legacy_files（396 条）→ 对应 B-<module> 拆分。

## 5. 结论

Pass A（Move First）全部 17 个任务 + 4 个集成屏障完成并逐一独立审查通过；系统行为、外部契约、基线计数全程零漂移；16 模块归位完成，剩余 legacy 全部有精确清单与 Pass B 义务。**Pass A 验收通过**，Pass B 待另行规划。
