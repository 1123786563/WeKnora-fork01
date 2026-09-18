# CFT 执行基线报告（CFT-S00-T001）

日期：2026-09-18 · 执行人：CFT 总控（ZCode 会话） · 状态：本轮已完成源码与验证入口核查

## 1. HEAD、环境与工作区

| 项 | 值 |
|---|---|
| 执行 worktree | `.worktrees/craft-cft`（分支 `craft/cft-execution`，基于 `main` b04722cf） |
| 执行 HEAD | `c606c765`（0f2270f8 设计包 + c606c765 脚本计数修正，均为本分支提交） |
| 设计审阅基线 | `0cba2f85…` 存在于仓库历史；当前 HEAD 领先该基线（A11/A12/A8/A9/A18 等多轮 upstream-parity 已合入） |
| 主工作区 | 停留在 `main` b04722cf，含其他会话的未提交改动（Miniapp Kit 删除、untracked 设计包目录）——本轮不触碰，按约束改用专用 worktree |
| Node / pnpm | v26.7.0 / 10.28.2（pnpm-lock.yaml 在 worktree 重新安装，exit 0） |
| Go | go1.26.3 darwin/arm64 |
| Playwright | @playwright/test 1.63.0（pnpm-lock 锁定；chromium 本机缓存） |
| OpenCode | `~/.opencode/bin/opencode` 1.18.4，sha256 `9449af91…2a8de3f98`（与 `docker/craft/runtime-config.json` darwin-arm64-local-probe 一致） |
| DESIGN_ROOT | `docs/design/weknora-craft-hifi/`（解压目录与建议路径 `docs/craft/design/` 不同，已在分支提交 0f2270f8 固化；以实际目录为准） |

## 2. 服务注册与功能 Gate

- Gate：`WEKNORA_CRAFT_ENABLED`（`internal/container/container.go:2052-2057`），未设置即不装配 craft，默认关闭；`internal/container/craft_runtime.go` 提供 `NewUnavailableCraftExecutor` fail-closed 语义，有测试钉死。
- 执行链 env：`CRAFT_OPENCODE_BASE_URL` / `CRAFT_OPENCODE_WORK_DIR` / `CRAFT_OPENCODE_OUTPUT_DIR` / `CRAFT_OPENCODE_RUNTIME_DIGEST`；未设 BASE_URL 时保持 fail-closed。
- 路由注册：`internal/router/routes_chat.go:120-150` 在 sessions 组内注册 craft 全部端点；handler 为 nil（未装配）时路由整体跳过，不产生半注册状态。

## 3. REST/SSE、DTO、取消与人工交互入口（真实存在）

| 能力 | 入口 | 状态 |
|---|---|---|
| 创建/列出会话 | POST/GET `/api/v1/craft/sessions`（幂等 request_id） | 已注册 |
| 权威快照 | GET `/sessions/:id/craft` | 已注册 |
| 关联输入 | POST `/sessions/:id/craft/inputs` | 已注册 |
| 提交 Run | POST `/sessions/:id/craft/runs`（request_id 幂等 + live-run guard） | 已注册 |
| 事件流 | GET `/sessions/:id/runs/:rid/events?after=N`（既有 RunEvent SSE） | 已注册 |
| 版本/文件 | GET `.../craft/versions[/:vid][/files/*path]`（manifest 路径校验、拒绝 `..`/`\`） | 已注册 |
| 预览票据 | POST `.../craft/versions/:vid/preview` | 已注册 |
| 快照/恢复 | GET `.../craft/snapshots`、POST `.../craft/restore` | 已注册 |
| 用量 | GET `.../craft/usage` | 已注册 |
| 人工交互 decide | POST `/sessions/:session_id/craft/interactions/:interaction_id/decide`（`craft_interaction.go:66`，含 ExpectedRevision CAS） | 已注册（W06 报告"未注册"已过时） |
| 取消 | 复用既有 session run 取消路由（非 craft 专属） | 既有 |

DTO 现状（`packages/api-client/src/craft/index.ts` + `internal/handler/session/craft.go`）：
- `submit` 实际字段：`request_id / prompt / input_refs[] / knowledge_scope / base_version_id`。
- 与目标 `DraftIntent`（command-bridge.types.ts）差异：`text→prompt`；`inputs:InputRef[]→input_refs:string[]`；`knowledge:KnowledgeSelection[]→knowledge_scope:string`；**`expectedWorkspaceRevision` 后端 run DTO 未支持**（CAS 目前只在 interaction decide 与 restore 上有）。此差异移交 T002 冻结处理。
- cancel/decide/restore/refreshPreview 未收敛进 `createCraftApi`（restore/decide 有 HTTP 路由，api-client 无方法；views 侧经 `client.request` 或专用 client 调用）——"缺接线"类，T002/T008 处理。

## 4. assistant-ui 真实状态

**未安装 `@assistant-ui/react`**（apps/web、views package.json 与 node_modules 均无）。`packages/views/src/craft/workbench.tsx:3-20` 注释明说：旧计划 G2 决策以纯 React 19 + `useSyncExternalStore` 实现"同 ExternalStoreRuntime 语义"的消息日志。本轮设计（ADR-C02、T006）要求锁版安装并真实挂载 Provider/Thread/Composer——**缺实现，S00-T006 执行**（决策记录见 DECISIONS.md D001）。

## 5. Craft 测试收集（本轮实际执行，非零收集）

| 命令 | 结果 | 退出码 |
|---|---|---|
| `bash scripts/test_craft_shared.sh` | 61 pass / 0 fail（contracts+api-client+domain+core+views 五目录 craft 测试） | 0 |
| `pnpm run test:shared` | **573 pass / 0 fail**（全量含 craft glob；旧基线 110 为 W06 时点部分值，只增未减） | 0 |
| `go test -count=1 -v ./internal/craft/...` | **103 pass** / 0 fail（uncached） | 0 |
| `go test -count=1 ./internal/agent/opencode/... ./internal/container/...` | ok ×2 | 0 |
| `pnpm run test:craft:web`（playwright，craft-stack.sh mock 全栈） | **修复后 6 passed (25.7s)，exit 0**——首轮复验捕获真实回归（run 永远 queued），两处 harness 修复后全绿，详见 evidence/CFT-S00-T001/ | 0 |

- `test:craft:shared` / `test:craft:web` 均已存在于 package.json（任务卡"可能只是计划建议"的怀疑已核实：**它们已真实存在**，分别是 tsx --test 显式 glob 与 playwright -c playwright.craft.config.ts）。
- `typecheck:shared` 显式含 craft 五目录文件；`typecheck:web` 的 tsconfig 有 craft paths 映射。类型检查包含 Craft。

## 6. 设计建议文件 → 真实文件映射

| 设计建议 | 真实落点 | 备注 |
|---|---|---|
| `packages/views/src/craft/assistant-runtime.tsx`（新增） | 不存在 | T006 创建 |
| `packages/views/src/craft/thread.tsx`（新增） | 不存在（对话渲染内嵌于 workbench.tsx） | T006/T007 处理 |
| `packages/core/src/craft/command-bridge.ts`（新增/收敛） | 不存在（命令串联内嵌于 `apps/web/src/features/craft/routes.tsx`） | T002/T008 决定收敛方式 |
| `packages/views/src/craft/workbench.tsx`（现有改造） | 存在（W05 双栏工作台） | 令牌化在 T003、结构在 T010 |
| `packages/core/src/craft/controller.ts` | 存在（快照/seq/退避/epoch 语义齐备，有测试） | 直接复用 |
| `packages/domain/src/craft/` | 存在（state/reconnect/usage，纯投影） | 直接复用 |
| `apps/web/src/features/craft/routes.tsx` | 存在（宿主装配，两路径 /craft、/craft/:sessionId） | 直接复用 |
| `internal/craft/contracts.go` | 存在（Scope/Workspace/Task/Version/File/Check/Result 全部与设计 §3 一致） | 直接复用 |
| `internal/agent/opencode/` | 存在（client/executor/protocol/normalizer + 测试） | 直接复用，无双轨 |
| `internal/craft/opencode/`（禁止双轨） | 不存在 | 保持 |
| `scripts/test_craft_shared.sh` | 存在（c606c765 修正计数 102→101：原 ls 对 mobile-* 双重列出） | 已提交 |
| `docs/craft/baseline.md` | 本文件即基线（落点 `docs/craft/execution/BASELINE.md`） | — |

## 7. 能力分类总表

| 分类 | 项 |
|---|---|
| 已实现且本轮验证 | craft 五包共享测试（61）；全量 shared（573）；Go internal/craft（103）；opencode/container Go 测试；test/typecheck 入口含 craft；Feature Gate fail-closed；REST+ SSE 12+ 端点注册 |
| 有代码但本轮未逐项验证 | `real` 模式 e2e（OpenCode 内置免费模型，W06 历史证据通过，T024 重跑）；restore/decide/preview/usage 浏览器端到端路径（有服务级测试 + mock e2e 覆盖，历史证据在 docs/testing/craft/web-acceptance.md） |
| 缺接线 | api-client 未收敛 cancel/decide/restore/refreshPreview 端口；submit 无 expectedWorkspaceRevision 字段（后端 DTO 同缺） |
| 缺实现 | assistant-ui 锁版与真实挂载（T006）；`.wk-craft` 作用域 + `--craft-*` 语义令牌别名（craft.css 现为硬编码色值，T003）；统一 command-bridge 模块（T002/T008） |
| 环境阻塞 | 无（OpenCode 二进制、Playwright chromium、SQLite 全栈 harness 均本机可用） |

## 8. 文档漂移与兼容策略

1. 设计包基线 0cba2f85 落后当前 main：该基线时点 `test:shared` 不含 craft glob（设计 §10 [R05]），**现已含**（573 pass）——采用策略：以真实代码为准，设计要求视为已满足，不重复补 glob。
2. W06 验收报告 §8"interaction decide 后端路由仍未注册"**已过时**：`craft_interaction.go:66` 已注册并有 ExpectedRevision CAS。
3. 设计 `DraftIntent.expectedWorkspaceRevision` 与后端 `craftRunRequestDTO` 不一致：不盲改后端；T002 冻结合同时决定（加字段 or 记录目标降级为后续演进）。
4. 原设计包路径建议 `docs/craft/design/weknora-craft-hifi/` 与实际 `docs/design/weknora-craft-hifi/` 不同：以实际提交路径为 DESIGN_ROOT，不搬移。
5. 旧八份 superpowers 计划（craft-01~05 等 27 项）已全部实施合入 main（见 docs/superpowers/plans/ 与 W01-W06/R01-R07 报告）；本轮 CFT 36 项是"对齐高保真设计 + 补真实缺口"，旧计划能力优先复用。
