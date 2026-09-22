# Pass A Batch A1 Integration Evidence（IA1）

集成线：`backend-mod-passa`（worktree `.worktrees/bm-passa-main`），基线 `703884315`（Foundation 终版）。

## 1. 集成顺序与提交

| 模块 | worker 分支终版 | merge 提交 | composition 切换提交 |
|---|---|---|---|
| A1 appconnector | 5bccdd7fc | b50ee126a | b0ef8895a（container.go:40/:54 切新路径，删 5 别名目录） |
| A2 commercial | d57e560a1 | 638d3eb60 | 10cc02f85（container.go 7 条 import 切换，删 7 别名目录） |
| A3 datasource | 0653bbd11 | d82b97a49 | ec609715c（container.go 11 条 import 切换，删 12 别名目录） |
| A4 channels | 8ddd4128b | 0b8041f84 | 1f5f89ee3（container.go im→modules/channels/im，删 10 别名目录） |

屏障工具/修复提交：b6fe61d6e + c8f17859e（architectureguard 精确路径 import 例外机制）、de7f15e61（modulemove 接受已集成搬迁）、cfca30943（基线注册 payment 预存 flake）、cabbe822c（changed-range lint 修复）。

## 2. 合并冲突（均为 A 系 import 修复相交，按"两侧新路径并集"解决）

- A2×A1（8 文件）：oc_recovery_test.go、open_connector.go、app_connector_oc_test.go、action.go、action_test.go、oc_integration_test.go、oc_limiter_test.go、oc_recovery.go
- A3×A1（2 文件）：datasource_service.go、datasource_credential_refresh_trigger_test.go

全部为 import 块并集，零逻辑取舍。

## 3. 集成后守卫发现与处置

- architectureguard 首次在真实模块布局上运行，暴露 4 处**预存**跨模块耦合（appconnector→commercial；Pass A 前为横向包互引，搬迁后显形）：oc_recovery.go、adapter.go、action.go（→ modules/commercial 根）与 oc_recovery.go（→ modules/commercial/service/commercial）。
- 处置：按 Spec §15 以精确路径+原因+删除批次注册 guard 例外（B-appconnector），通配符不支持；Pass B 收敛。
- route/worker/hook 计数：guard 实测 633 routes（564 literal + 69 apiKeyRoute）/ Redis 23 + Lite 23 / hooks 58 —— 与 F0 基线一致，零漂移。

## 4. 门禁结果（2026-09-21）

- `go build ./...` exit 0（每个模块切换后即时验证 + 终态复验）
- `go run ./tools/modulemove verify --all`：OK（16 manifests；de7f15e61 起接受已集成搬迁）
- `go run ./tools/architectureguard`：OK（0 violations）
- `golangci-lint run --new-from-rev=703884315 ./...`：0 issues（cabbe822c 修复 9 项：gofmt/gofumpt 对齐、2 处长行 wrap、1 处预存长 SQL 行 nolint:lll、craft_budget_test 重复 import 以删除 domain 别名+改 2 处 qualifier 根治、verify.go SA4004 重构）
- `go test ./internal/... -count=1 -timeout=25m`：**118 ok / 1 FAIL**，FAIL 为 `internal/agent/opencode`（1501s，goroutine 挂起超时）——F0 §2.5 已注册的 unstable blocker 签名，非新失败。payment 预存 flake 本轮未触发（cfca30943 已注册）。

## 5. 遗留与 Pass B 指向

- guard 例外 4 条 → B-appconnector（边界收紧时改走 commercial 公开门面后删除）。
- payment `TestProvidersFromEnvRejectsPartialAlipay` 测试断言 map 随机序 → B-commercial 观察项。
- 各模块旧路径别名已随切换全部删除（A1-A4 的 alias_obligations 提前完成，Pass B 无需再删）。

## 6. 批次结论

A1–A4 全部按 Integration Brief 接入完成，全部门禁不劣于 F0 基线，批次放行 Parallel Group A2（A5–A8）。
