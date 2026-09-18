# CFT-S05-T031 证据（重生成轮 + 回归修复）

日期：2026-09-18 · 主仓 main（恢复轮）
原验证提交：288e8c8a `test(craft): pin the conservative lifecycle summary (CFT-S05-T031)`。
**说明**：原 EVIDENCE.md 未及提交即随 worktree 清理丢失（见 DECISIONS D004）；本轮重生成，并修复了重跑暴露的主线集成回归（D005）。

## 交付物（既有，HEAD 抽验在位）

- `internal/application/service/craft_lifecycle_guard_test.go`（`TestCraftLifecycleGuardSummary`，4 子测试，委托 `craft_lifecycle_test.go` 四个名义套件）

## 发现并修复的回归（本轮核心事实）

在 main HEAD 首轮重跑 `TestCraftLifecycleGuardSummary` **FAIL（exit=1，4/4 子测试失败）**：
`craft_snapshot_test.go:210` 迁移 harness 报 `failed to open source, "file://…/migrations/sqlite": duplicate migration file: 000058_mobile_devices.down.sql`。
根因：三条并行 lane 各自新增 000058 号迁移（`000058_mobile_devices` / `000058_paseo_control` / `000058_execution_registrations`），golang-migrate 拒绝加载，**所有依赖 sqlite 迁移 harness 的测试全部阻断**（不止 Craft）。

修复（内容零改动，仅重编号，`git mv`）：
- `000058_paseo_control.*` → `000077_paseo_control.*`（无后续迁移依赖其表）
- `000058_execution_registrations.*` → `000078_execution_registrations.*`（`000061/000062_execution_cleanup` 仅用自建表，已核无依赖）
- 保留 `000058_mobile_devices.*`（三者中最早提交，且 000059/000060 同链 ALTER 其后续表）

## 验收断言对照（修复后重跑实测）

- 被引用对象存活、孤儿回收（staging 回收窗）✓
- 活跃 work slot 不回收（live session/sandbox + generation 保护）✓
- GC 重试不扩围 ✓
- 中断 deleting 对象幂等重驱动、不扩围 ✓

## 命令与退出码（本轮，main HEAD）

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftLifecycleGuardSummary -count=1 -v`（修复前） | 1 | 4/4 FAIL（duplicate migration 000058） |
| 同上（修复后） | 0 | 4/4 PASS（1.19s） |
| `go test ./internal/craft/... ./internal/application/service/ ./internal/handler/session/... ./internal/agent/opencode/... -run "TestCraft" -count=1 -v` | 0 | 119 PASS / 0 FAIL |

## 残留注意

- 曾在本机长期存在的开发库若已按旧号应用过某一 lane 的 000058，重编号后 77/78 会被视为新迁移而 CREATE TABLE 撞已有表——此类库需重建；全新库与测试临时库不受影响。
- 生产/部署迁移目录同样受益于此修复（此前加载即失败）。

## 回退

revert 迁移重编号提交与本证据提交即可回到（已知的）阻断态；不建议。
