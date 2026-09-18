# CFT-S00-T005 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/domain/src/craft/capabilities.ts`（新增）：`craftViewCapabilities` 纯投影 + `craftKindDisabledReason`（禁用给原因、不清草稿）+ `CRAFT_CAPABILITY_REASONS` 统一枚举。全矩阵不变量：写=开闸∧有写权；读历史永不依赖闸/写权；kinds 精确镜像服务端；禁用面恰好一个 reason；未知上传上限保持 null 不造 0。
- `packages/domain/src/craft/capabilities.test.ts`（新增，5 tests）：含 2×2×3 全组合矩阵一致性断言。
- `internal/application/service/craft_session.go`：`CraftGateCapabilities` + `Capabilities()`（gate 快照拷贝投影）。
- `internal/handler/session/craft.go`：`CraftSessionAPI` 接口增加 `Capabilities()`；POST /craft/sessions 201 与 GET workspace 200 响应携带 `capabilities{enabled, allowed_kinds}`（snake_case wire）。
- `internal/handler/session/craft_test.go`：`TestCraftHTTPCapabilitiesProjection`。

## 验收断言对照

- 未启用时入口不可提交 ✓（零值 gate：domain 投影 canWrite=false + gateOff reason；服务端 `Allows` 全拒绝，Go 测试断言）
- 没有 write 时仍可按 read 查看历史 ✓（domain 不变量 2：canRead 恒 true；服务端 View 不经 gate——A06 恢复语义，e2e 04 既有覆盖）
- 不允许的类型被后端拒绝且前端保留草稿 ✓（服务端：closed kind → 503 "not enabled"（Unsupported 映射，宿主错误分类法），Go 测试断言；前端保留草稿由 domain 规则承载：`craftKindDisabledReason` 只投影原因不清空目标——T009 接线 home 选择器时消费）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/domain/src/craft/capabilities.test.ts` | 0 | 5 pass / 0 fail |
| `go test -count=1 ./internal/handler/session/ ./internal/application/service/` | 0 | ok ×2（含新测试） |
| `pnpm run test:craft:shared` | 0 | **78 pass / 0 fail**（73→78） |
| `pnpm run typecheck:shared` | 0 | 通过 |

## 未验证事项 / 回退

- 前端 home/workbench 对 `capabilities` 字段的实际消费（类型选择器按 allowed_kinds 禁用+原因）在 T009/T010 接线（本任务冻结合同与服务端字段）。
- e2e 全栈复跑未在本任务重复（响应为增量字段，contracts 解析器忽略未知字段，既有 6 spec 不受影响；T009 接线时一并回归）。
- 回退：revert 本提交（Go 接口方法与响应字段为增量；domain 两文件纯新增）。
