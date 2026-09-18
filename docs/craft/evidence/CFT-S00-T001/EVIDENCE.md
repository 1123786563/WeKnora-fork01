# CFT-S00-T001 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · HEAD `c606c765`（+ 本任务提交）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `bash scripts/test_craft_shared.sh` | 0 | 61 tests pass / 0 fail（test-craft-shared.log） |
| `pnpm run test:shared` | 0 | **573 tests pass / 0 fail**（test-shared.log；含 craft 五目录 glob，旧 110 基线只增未减） |
| `go test -count=1 -v ./internal/craft/...` | 0 | **103 PASS** / 0 FAIL（go-test-count.txt = 103，uncached） |
| `go test -count=1 ./internal/agent/opencode/... ./internal/container/...` | 0 | ok ×2（43.6s / 5.1s） |
| `bash e2e/craft-stack.sh up mock && run mock`（修复后） | 0 | **6 passed (25.7s)**：create/upload/generate/preview/modify/old-download、reopen、malicious blocked、cross-tenant 404、session-switch isolation、reload idempotency（playwright-run.log） |

## 本轮发现并修复的 e2e 入口回归（RED→GREEN）

首次 `run mock` 失败：`01 create` 240s 超时，`craft-main-status` 停在"等待执行"，DB 中 run 永远 `queued`、`agent_run_events` 0 行。逐层定位出两个独立缺口，均为 harness/装配层，未改业务代码：

1. **recovery worker 未启用**：craft `POST /craft/runs` → `AgentRunService.Submit` → `store.Admit`（仅持久化），唯一消费者 `AgentRunWorker.Run` 在 `cfg.Enabled=false`（`agent.recovery.enabled` 默认 false，config/config.yaml:131）时直接 return——run 永远 queued。09-16 paseo 合并后 worker 成为唯一消费路径，e2e 此后未复跑。修复：`craft-stack.sh` server env 增加 `WEKNORA_AGENT_RECOVERY_ENABLED=true WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED=true`。
2. **默认模型被 builtin 抢占**：`config/builtin_models.yaml` 的 `builtin-llm-mock`（base_url `http://192.168.3.30:18090/v1`，is_default=true）在 server 启动时入库；stack seed 的 `craft-main-fixture` 晚到，`craftChatModelID` 取"遍历中第一个 default"命中 builtin 行 → SSRF 校验拒绝直连 IP（`baseURL SSRF check failed`）。修复：seed SQL 追加 `UPDATE models SET is_default=0 WHERE id='builtin-llm-mock'`。

修复后同一命令 6/6 全绿。第一次失败现场保留在 /tmp 运行目录（craft-w06-mock.1jvR1M / .iPets0，重启即失，关键日志已摘录于上）。

## 截图（mock 全栈真实浏览器）

- `01-preview-monthly.png` 创建→上传→生成→独立 https 预览源（按月报告）
- `02-versions.png` 修改后两版本并存
- `03-malicious-blocked.png` 恶意 fixture 被沙箱/CSP 阻断
- `04-viewer-invisible.png` 非属主/跨租户不可见

## 环境身份

- OpenCode serve 1.18.4 sha256 `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`（harness 启动前复核）
- 主模型 fixture `apps/web/e2e/craft-mocks/main-model.mjs`（OpenAI 兼容，127.0.0.1 随机端口）
- Go 1.26.3 / Node v26.7.0 / pnpm 10.28.2 / Playwright 1.63.0（pnpm-lock 锁定）
- 测试凭据仅存 /tmp 运行目录（0600 creds.txt），不入库不入提交

## 未验证事项

- `real` 模式（OpenCode 内置免费模型）本轮未跑（W06 历史证据 01/02/04 通过；T024 里程碑时重跑）。
- `test:craft:web` 在 CI 环境的表现（本机 mock 全量为准）。
- 前端 `pnpm run typecheck:web` 本任务未执行（留给 T002-T006 各自验证轮）。

## 回退方式

`git revert` 本任务对 `apps/web/e2e/craft-stack.sh` 的提交即可恢复原 harness 行为（回到 run-forever-queued 状态）。
