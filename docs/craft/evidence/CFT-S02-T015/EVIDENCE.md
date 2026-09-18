# CFT-S02-T015 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物（含一项真实缺口修复）

- **`internal/agent/opencode/executor.go` 修复**：预检快照（pre-flight snapshot）读取失败时，原实现把 `alreadyAccepted` 留为 false 并继续 Prompt POST——读端超时（远端可能已接受）时会**盲重发副作用**。现在预检失败直接 `unresolved`（ErrUnknown、可核对、不提交）：
  ```go
  } else {
      return e.unresolved(run, nil, "pre-flight snapshot failed, refusing to re-submit the prompt: %v", err)
  }
  ```
  该缺口由本任务测试注入抓到（见下），不是预设结论。
- `internal/agent/opencode/unknown_acceptance_test.go`（新增，3 tests，`TestUnknownAcceptance*`）。
- `executor_test.go`：runtimeFake 增加 `hangMessages` 故障注入（GET /message 挂起至请求取消）+ `setHangMessages`；memStore 增加 `lastPreparedPromptID`（重入复用同一委派身份）。

## 验收断言对照（故障注入证明副作用提交次数）

- 远端接受后 GET 超时不二次 POST ✓（`RemoteAcceptedReadTimeoutNoSecondPost`：hijack 响应丢失但消息列表含 prompt → 第一轮 unknown；重入①读超时（真实 800ms client timeout）→ unknown 且**未重发**（修复后）；重入②读恢复、远端仍在跑同一 prompt → 仍 unknown；**三次 Execute 全程 prompt POST 恰 1 次**）
- 断流不解释成成功 ✓（`StreamBreakIsNeverSuccess`：剪流+assistant 未完成 → ErrUnknown、零存储）
- 未知保持主 ToolCall 等待 ✓（两测试均断言 `savedStatuses()` 为空——unknown 不落结果，主 ToolCall 由 ErrUnknown 保持 pending）
- 达到核对截止时间有诊断状态 ✓（`PassedDeadlineReportsDiagnostics`：过期 deadline → unknown 且 Summary 含 "deadline"、**零副作用提交**）

## 验证命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/agent/opencode -run TestUnknownAcceptance -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/agent/opencode/...` | 0 | ok（17.5s，修复无既有回归） |
| `CRAFT_LIVE=1 go test -run TestLiveCraftTwoTurns -timeout 300s` | 0 | ok（50.8s，真实 serve 两轮——修复后的正常路径验证） |
| `go build ./...` | 0 | 通过 |

## 环境事件记录

首次 live 复验 600s 超时：根因是**前次超时 panic 遗留的两个测试 serve 进程**（t.Cleanup 未执行）占据 XDG 目录，新 serve 的 /doc 就绪探测（无超时的 http.Get）挂死。清理两个测试残留进程（pid 44976/45011，均为本仓库测试拉起）后复验通过。另注：live_test 的就绪探测单次 GET 无超时属测试 harness 弱点（deadline 检查在两次 Get 之间，单次挂死逃逸）——已在此留档，修复属测试基建，不在本任务锁定文件内（不扩权）。

## 回退

revert 本提交（executor 单分支修改 + 两测试文件增量；回退后恢复"预检失败继续 prompt"的旧行为——即恢复盲重发缺口）。
