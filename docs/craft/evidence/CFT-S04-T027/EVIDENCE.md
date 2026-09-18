# CFT-S04-T027 证据（重生成轮）

日期：2026-09-18 · 主仓 main（恢复轮）
原验证提交：fac1f39e `test(craft): pin the spreadsheet chain — real recalculation, policy isolation (CFT-S04-T027)`。
**说明**：原 EVIDENCE.md 随 worktree 清理未及提交（任何分支均无此目录，见 DECISIONS D004）；本轮在 main 上真实重跑同一名义测试重生成证据，测试代码本体自原提交起未改动。

## 交付物（既有，HEAD 抽验在位）

- `internal/craft/spreadsheet_recalc_pin_test.go`（`TestCraftSpreadsheetRecalcPins`，3 子测试）
- `docs/craft/artifact-spreadsheet-contract.md`（本轮由 b08a67a6 恢复）

## 验收断言对照（本轮重跑实测）

- SUM 真重算 ✓（formula-only 工作簿无缓存值判 not-ready；LibreOffice 重算 fixture d02 答案 300）
- 预览数字 = 存储 XLSX 值 ✓（两轮 300/350，类型保真）
- 外链/宏政策 ✓（bracket refs、DDE、URL scheme、WEBSERVICE/HYPERLINK 族拒绝；普通 SUM 放行）

## 命令与退出码（本轮，main HEAD）

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/craft -run TestCraftSpreadsheetRecalcPins -count=1 -v` | 0 | 3/3 PASS |
| `go test ./internal/craft/... ./internal/application/service/ ./internal/handler/session/... ./internal/agent/opencode/... -run "TestCraft" -count=1 -v` | 0 | 119 PASS / 0 FAIL |

注：首轮重跑曾被 `migrations/sqlite` 000058 撞号阻断（harness 加载迁移失败）——修复见 DECISIONS D005 与本轮迁移提交。

## 回退

revert 本证据文件与迁移重编号提交；测试代码无需回退（自 fac1f39e 未改动）。
