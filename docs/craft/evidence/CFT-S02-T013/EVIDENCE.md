# CFT-S02-T013 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/agent/opencode/protocol_pin_test.go`（新增，3 tests，`TestProtocolPin*`）：钉住锁定协议的三组断言（离线 fixture + httptest 驱动真实 Client）。
- 既有 protocol.go/client.go/normalizer.go 无需修改——1.18.4 锁定方言（消息 ID 生成复刻源码 commit 49c69c5 的 48-bit 时间戳+计数器格式、parts 严格校验、端点表面）已在 R01/R03 实现，本任务补的是**契约钉死证据**而非重建。

## 验收断言对照

- 版本变化导致清晰失败而非猜字段 ✓（`VersionDriftFailsClearly`：漂移的 parts 形状（对象/null/字符串/空）全部明确报错；非锁定格式 message ID 被拒；漂移的 session status 类型（"v2-drifted-state"）明确失败而非映射成猜测状态）
- 事件/消息/取消协议与固定镜像一致 ✓（`EndpointSurface`：fixture serve 驱动真实 Client 走完整锁定端点面——POST /session、POST /session/{id}/prompt_async（204）、POST /session/{id}/abort、GET /session/status、GET /event（SSE 流）、POST /question/{id}/reply，逐一断言命中且拒绝未预期调用）
- 不将 /doc OpenAPI 版本当二进制版本 ✓（`BinaryVersionIsTheDigestNotTheOpenAPI`：Client 构造无版本探测；二进制身份=部署 runtime digest（CRAFT_OPENCODE_RUNTIME_DIGEST ↔ docker/craft/runtime-config.json）；消息 ID 形状钉死 1.18.4 格式）

## 真实服务兼容证据（固定二进制）

| 项 | 值 |
|---|---|
| 二进制 | `~/.opencode/bin/opencode` 1.18.4，sha256 `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`（与 docker/craft/runtime-config.json darwin-arm64-local-probe 一致） |
| 本轮实跑 | `CRAFT_LIVE=1 go test -run TestLiveCraftTwoTurns` **PASS（21.5s）**——真实 serve 两轮委派（live-two-turns.log）：同一工作区+同一 OpenCode 子会话跨两轮、真实 SQLite 持久化链、受控本地模型端点 |
| 历史证据 | W06 real 模式 e2e（docs/testing/craft/web-acceptance.md：真实免费模型两轮通过） |

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/agent/opencode -run TestProtocolPin -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/agent/opencode/...` | 0 | ok（13.97s，全包含既有 normalizer/client/executor 测试） |
| `CRAFT_LIVE=1 go test -run TestLiveCraftTwoTurns -timeout 600s` | 0 | ok（21.5s，真实 serve） |

## 双轨检查

`internal/craft/opencode/` 不存在——适配器唯一实现在 `internal/agent/opencode/`（无并行旧目录）。

## 回退

revert 本提交（单测试文件，纯增量）。
