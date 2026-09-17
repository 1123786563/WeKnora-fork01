# 上游功能对照台账（upstream-parity）

> **任务**：以 [Tencent/WeKnora 上游](https://github.com/Tencent/WeKnora) 源码为对照基准，确保本仓库（React 前端 + trpc-group/trpc-agent-go 智能体引擎 + gin 后端）的功能逻辑与上游一致；发现不一致则参考上游逻辑修复，但**保持 fork 技术栈不变**（不回退 trpc-agent-go、不重写 React）。
> **纪律**：本台账是唯一进度真相源。凡是「✅ 已完成」的条目，后续轮次**不得重做**，只做回归复核。

## 基线

| 项 | 值 |
|---|---|
| 上游克隆 | `/Users/wuyongjun/trea/weknora-upstream`，commit `2514e42`（2026-09-17 克隆，depth 60 历史可用 `git -C … show <sha>`） |
| 本仓库 | `/Users/wuyongjun/trea/WeKnora-fork01`（main），VERSION 0.8.0 与上游同源，go.mod 仅多 trpc-agent-go v1.10.0 / trpc-a2a-go |
| 差异总览（2026-09-17 首测） | internal/：上游独有 133 文件、fork 独有 196、内容不同 305（非测试 differ 202）；frontend/src：上游独有 49、fork 独有 23、不同 85 |
| fork 独有域（不适用对照） | commercial、workbench、payment、craft、execution、appconnector、connectorcontrol、metrics、agent/trpc（trpc-agent-go 引擎） |
| 关联任务 | React↔Vue UI 逐页对齐由独立 cron 自动化在 worktree `.worktrees/react-multiclient`（分支 codex/react-vue-parity-align）推进，台账 `docs/migrations/react/vue-react-parity-progress.md`。**本任务不动那个 worktree、不与其抢提交**。 |

## 状态图例

⬜ 待办 ｜ 🔄 进行中 ｜ ✅ 已完成（含验证证据） ｜ ⛔ 不适用/评估后不做 ｜ 🧩 已适配移植（代码有 fork 侧改造）

## A. 后端新功能移植

| # | 功能 | 上游依据 | fork 现状 | 状态 |
|---|---|---|---|---|
| A1 | Serply 网络搜索 provider | `e80f5df` internal/infrastructure/web_search/serply.go | 无 | ✅ 完成（本轮） |
| A2 | chat special_tokens 处理 | `internal/models/chat/special_tokens.go`（新文件，随近期演进加入） | 无 | ✅ 完成（本轮） |
| A3 | 知识库批量文件下载 | `a7d3545` internal/handler/knowledge_download.go | 无 | ✅ 完成（后端+Vue 前端+i18n；React 侧见 C7；swagger 的批量下载条目未 regen，后续补） |
| A4 | 文档原地替换（race-safe） | `419ff25`+`079e96a` knowledge_replace.go | 无 | ✅ 完成（service primitive，上游同样未暴露 handler） |
| A5 | 文档摘要生成可选开关 | `105c177` | 有 GenerateSummaryPrompt 但无开关 | ✅ 完成（本轮） |
| A6 | XMind 文件导入白名单 | `d812645` | docparser 已有 xmind 引擎，导入白名单待核 | ✅ 完成（本轮，fork 白名单本已含 xmind，补齐上传扩展名校验一致性） |
| A7 | 共享连接器文件名净化 | `820a14d` internal/datasource/filename.go | 各连接器各自实现 | ✅ 完成（本轮，新增共享包，原连接器实现保留） |
| A8 | Confluence 同步连接器 | `01f0700` internal/datasource/connector/confluence | 无 | ⬜ 待办（大条目） |
| A9 | 钉钉连接器（stream-only） | `internal/datasource/connector/dingtalk` + 迁移 000017/000096 | 无 | ⬜ 待办（大条目） |
| A10 | Milvus analyzer + 迁移工具 | `analyzer.go`/`migration.go` | 无 | ⬜ 待办 |
| A11 | 会话 fork（历史消息分叉） | `42e6163` session_fork 全家桶 + 迁移 000018/000019(上游编号) + 前端 forkPoint.ts | 无 | ⬜ 待办（大条目，需适配 trpc-agent-go checkpoint） |
| A12 | 记忆提取（memory extraction/lifecycle/vector） | `internal/application/repository/memory_*.go` + 迁移 | 仅旧 memory.go | ⬜ 待办（大条目） |
| A13 | BrowserSkill 0.3.0（浏览器技能） | `internal/browserskill/` + agent/tools/browserskill* + 迁移 | 无 | ⬜ 待办（最大条目，依赖 sandbox 基础设施） |
| A14 | 沙箱桌面（RFB/WS 远程桌面） | `sandbox_desktop_*` + handler/session/sandbox_desktop_* | 无 | ⬜ 待办（依赖 A13） |
| A15 | tool_images（agent 图片工具） | `internal/agent/tool_images.go` | 无（trpc-agent-go 引擎侧需评估等价物） | ⬜ 待办（需 trpc 适配） |
| A16 | shell_command_output（命令输出截断/持久化） | `internal/agent/tools/shell_command_output.go` + `internal/sandbox/command_output.go` | 无 | ⬜ 待办 |
| A17 | workspace_checkpointer / pinned_session_sandbox | `internal/application/service/` | 无 | ⬜ 待办（评估与 trpc checkpoint 关系） |
| A18 | im channel_security | `internal/im/channel_security.go` | 无 | ⬜ 待办 |
| A19 | agent_browser_preferences | `internal/application/service/agent_browser_preferences.go` | 无（依赖 A13） | ⬜ 待办 |
| A20 | embedpolicy 目录 | `internal/embedpolicy/` | 无 | ⬜ 待办（评估：fork embed-secure-mode 已有自有实现，可能 ⛔） |
| A21 | memory / session_fork / browser_authorization 数据库迁移 | 上游编号 000094-000099（versioned） | fork 编号已独立至 000135，需新编号追加 | ⬜ 随 A11/A12/A13 |

## B. 检索与数据修复（上游 bugfix 回移植）

| # | 修复 | 上游依据 | fork 现状 | 状态 |
|---|---|---|---|---|
| B1 | keyword-only BM25 分数在 rerank 前限幅 | `d8be87d` | 未修 | ✅ 完成（本轮） |
| B2 | 消息关键词搜索 SQLite/MySQL 兼容 | `0ad08d4` | 未修 | ✅ 完成（本轮） |
| B3 | 已删除行不再阻塞重复上传 | `55ec13a` | 未修 | ✅ 完成（本轮） |
| B4 | OBS 虚拟托管寻址 | `bca3a9f` | 未修 | ✅ 完成（本轮） |
| B5 | IM：agent 完成后错误不丢失 / QA 返回后 finalize | `e07e782`+`b23ae05` | e07e782 生产代码 fork 本已处于修复后状态（EventAgentComplete 不再 closeDone），仅缺测试；b23ae05 两处 goroutine 已补 defer + qa_exit_test.go 已加 | ✅ 完成（stream_exit_test.go 未移植——依赖上游测试基建且其对应生产修复本已在位） |
| B6 | KB 初始化创建模型补租户戳 | `c9d5987` | 待核 | ✅ 完成（本轮） |
| B7 | Milvus 保留关键词检索分数 | `38fee1e` | 待核（fork 用 postgres/paradedb 为主） | ✅ 完成（本轮） |
| B8 | sandbox：文件访问与 artifact 发布分离 | `5780cb0` | 已移植（18/22 文件补丁 + prompts.go/sandbox_ls.go/session_manager.go 手工改写 + 3 个旧行为测试重写 + prompts_shell_test 新增） | ✅ 完成 |
| B9 | sandbox 远程调用去重（perf） | `1c43934` | **回退**：依赖 fork 缺失的 `ExecShellCommandWithOutputSnapshot`/`connectRemoteSession`/`validateSessionSummary` 基建（属 A17 链：session_connect/session_file_operation/workspace_checkpointer/pinned_session_sandbox），已用 `git apply -R` 精确回退 | ⬜ 随 A17 一起做 |
| B10 | MCP 并发策略更新测试稳定（SQLite） | `ac4dd59` | 补丁干净应用 | ✅ 完成 |
| B11 | skill 后台安装竞态修复 | `fc7f37d` | 补丁干净应用 | ✅ 完成 |
| B12 | 客户端会话与集成请求处理对齐 | `d7ccd5b` | 91 文件大提交，涉 client SDK + dingtalk 迁移，需单独立项评估 | ⬜ 待办（大条目） |
| B13 | 前端 tag 过滤重构 + 引用弹层生命周期共享 | `559ad53`+`d645334` | fork Vue 未同步 | ⬜ 待办（影响 React parity，需双端同步评估） |

## C. 前端（Vue=上游基线 → React 同步）

> 原则：移植上游 Vue 新功能时，React 侧（apps/web）必须同步实现等价功能，否则违反「React 与上游一致」目标。小改动直接双端一起做；大功能（fork 按钮、BrowserSkill UI、桌面沙箱）单列条目待 A 类后端就绪后实施。

| # | 功能 | 状态 |
|---|---|---|
| C1 | 会话 fork 入口（forkPoint.ts + 消息操作） | ⬜ 等 A11 |
| C2 | BrowserSkill 聊天内 UI（BrowserTaskPreview/BrowserToolDetails） | ⬜ 等 A13 |
| C3 | SandboxDesktop 桌面组件 + Document Picture-in-Picture | ⬜ 等 A14 |
| C4 | KnowledgeTagFilter 组件化 | ⬜ 等 B13 |
| C5 | SandboxCommandProgress / pptxPreview / browserToolDisplay | ⬜ 等 A13/A14 |
| C6 | useCitationPopover / useFloatingPreviewDrag 共享化 | ⬜ 等 B13 |
| C7 | 批量文件下载 React 侧（apps/web 等价实现：选择多文档→批量下载入口→request blob 拼装） | ⬜ 待办（后端 /knowledge-bases/{kb_id}/knowledge/download 已就绪，上游 Vue 实现在 KnowledgeBase.vue + DocumentBatchBar.vue + request.ts + knowledgeDownloadFileName.ts） |

## D. 运行时对照（双栈实测）

| # | 项 | 状态 |
|---|---|---|
| D1 | fork 栈启动（OrbStack WeKnora-app :8080 + React :5181 + Vue :5180） | ✅ 完成（后端健康；注意镜像滞后于 main，验证本轮新功能需重建镜像） |
| D2 | 上游栈启动（weknora-upstream compose + override：`Up-WeKnora-*` 容器名，app :18080 / UI :18081 / minio :19000-19001；.env 由 example 生成+JWT_SECRET 随机+端口 sed；账号 parity-up@local.dev / Parity123456! tenant 10000） | ✅ 完成 |
| D3 | 核心流程一一对照逐项记录 | 🔄 进行中（API 级 + agent-chat 浏览器级已完成，2026-09-18 第 6 轮）。已完成：注册/登录/KB 创建列表/上传→解析→分块→启用/混合检索/聊天流式 SSE/agent-chat 智能体模式（上游基线+fork 缺陷修复 5e382d0b，见第 6 轮）。**结论汇总**：①信封分歧 D3-1（`success+data` vs `knowledge_base(s)`，fork 三端已适配，⛔ 不改）；②聊天事件序列 fork=上游超集：fork 多发 `session_title` 流内标题事件——**D3-2 fork 有意扩展**，Vue(index.vue:1525) 与 React(ChatRoutePage.tsx:345) 均已消费，⛔ 不改；③SSE 安全防护双栈一致（host.docker.internal / 直连 IP / 解析到私网的域名全部拒绝——白名单走 system_settings `ssrf.whitelist` 键，两栈已写入 `[parity-mock, 192.168.3.32]`）；④检索响应结构逐键一致、同一文档同一查询双栈命中一致。待做：更多知识格式、修复项端到端复验 |
| D4 | 对照账号 | fork 栈：parity-test@local.dev / Parity123456!（**实际 tenant 10001，记忆中 10000 已过时**）；上游栈：parity-up@local.dev / Parity123456!（tenant 10000） |
| D6 | 浏览器级页面一一对照 | 🔄 两轮完成（第 4 轮证据 `evidence/browser-d3/` 13 张；第 6 轮 agent-chat 证据 `evidence/browser-agent-chat/` 3 张）。**D6-1 已结案（2026-09-18 第 6 轮）**：冷加载白屏真因=ChatHeaderMenu hooks 顺序违规（见第 6 轮修复 1），当时的外部 WIP 污染只是掩盖了复现条件；回归测试 chat-header-hook-order.test.tsx 已锁死。其余对照点见第 4/6 轮记录 |
| D5 | 双栈共享 mock 模型服务 | ✅ 完成：容器 `parity-mock`（python:3.12-alpine 跑 /Users/wuyongjun/trea/parity-mock/mock_server.py，同时接入 weknora-upstream_WeKnora-network 与 react-multiclient_WeKnora-network 两网），OpenAI 兼容 /v1/embeddings(1024 维确定性)+/v1/chat/completions(流式 SSE)+/v1/rerank(确定性 relevance_score，2026-09-18 第 6 轮加入)。两栈 DB 已插 mock-llm(KnowledgeQA)/mock-embed(Embedding,1024)/mock-rerank(Rerank) 并 is_default（rerank 非 default）；fork tenant 10001 旧 rig 模型(parity-llm-mock 等)已同指 parity-mock。SSRF 白名单经 system_settings `ssrf.whitelist`（重启 app 生效）。**复跑入口：两栈 KB `parity-smoke*` 各传 parity-doc.md → batch-reparse → hybrid-search/knowledge-chat；agent-chat 用 智能推理 agent（需配 对话模型+重排模型 就绪）** |

## E. 完成记录

### 2026-09-17 第 5 轮（D6-1 溯源：复现被外部 WIP 污染，暂缓定性）
- 二分定位过程：空 content assistant 消息删除后冷加载恢复（rootChildren 0→1）→ 初判数据相关。
- 但随后发现 worktree `codex/react-vue-parity-align` 存在**他人未提交的路由迁移 WIP**（main.tsx 自研路由→@tanstack/react-router 半成品 + package.json 新依赖 + navigation.ts 改动），:5181 dev server 实时加载该中间态；cron 防重叠锁 stale（PID 48180 已死）。冷加载崩溃与 SPA 导航丢历史均发生在此中间态上，**不可归因于已提交代码**。
- 处置：遵守「外部 WIP 未清空不代解」纪律，不在 worktree 调试/修复/提交；D6-1 转为「待迁移落库后重测」，复现步骤与上游对照结论（Vue 容错成立）已留档。
- 本轮零代码改动；台账更新即全部产出。

### 2026-09-17 第 4 轮（浏览器级页面对照首轮）✅
- 证据：`docs/upstream-parity/evidence/browser-d3/` 12 张（up-*/fk-* 成对：login/kb-list/kb-detail/chat-session/chat-new/chat-emptymsg/sessionB）。
- 登录页：双端布局一致；**小差异**：上游邮箱/密码 label 带必填星号（`* 邮箱`），fork React 无星号；左侧轮播图初始帧不同（Rotation 时序，非缺陷）。
- KB 列表/详情：IA 与卡片结构一致；两栈同样显示「部分知识库尚未初始化」横幅（初始化门控行为一致——models 表有模型但 initialization 配置未走时两栈都提示）。fork 知识库设置弹层（分组 IA）为 fork 增强形态。日期筛选控件呈现不同（fork --/--/----- vs 上游 起始时间/结束时间）——属 React↔Vue parity lane 范畴。
- 聊天页：上游 `/platform/chat`（无会话）空态白屏为上游自身行为；点开会话正常。fork `/platform/creatChat` 正常（含新手引导浮层、模型 chip）。
- **P1 缺陷 D6-1（本轮最重要产出）**：fork React 对空 content assistant 消息整页崩溃，上游 Vue 容错。上游栈保留了一个空消息测试会话（e9bd0ca3 首答被置空）作为长期复现夹具。
- 工程注记：locator 点击在 fork React 上频繁 3s 超时，坐标点击（cua）全程可用；:5181 dev server 属常驻进程（非本会话启动，勿杀）。

### 2026-09-17 第 3 轮（D3 深流程 API 级对照）✅
- **mock 基建（D5）**：本地 OpenAI 兼容 mock 容器 `parity-mock` 双网接入；两栈 DB 插入相同模型行；SSRF 白名单经 system_settings 运行时键（重启 app 生效）。期间实测两栈 SSRF 防护行为完全一致（host.docker.internal/直连 IP/私网解析域名全拦）。
- **上传→解析→分块→启用**：同一 parity-doc.md 双栈 parse_status=completed、enable_status=enabled。
- **混合检索**：POST /knowledge-bases/:id/hybrid-search（body: query_text）同一查询双栈各命中 1 条，响应键逐一同构、内容命中一致。
- **聊天流式**：POST /knowledge-chat/:session_id SSE。上游 `agent_query→answer×2→complete`；fork `agent_query→answer×2→session_title→complete`。**D3-2：session_title 为 fork 有意扩展**（Vue/React 前端均消费，React SSE 解析为上游超集：tool_call/approval/steer/artifacts），⛔ 不改。内容差异为 mock 回声伪影（两侧 QA 提示词长度不同），非缺陷。
- **故障排查记录**：fork 对照账号实际 tenant=10001（记忆 10000 过时，已修正）；会话默认绑定旧 rig 模型（parity-llm-mock），需连同旧模型一起改 base_url。
- 遗留：浏览器级页面一一对照（双 UI 已可达：上游 :18081、fork React :5181 需起 dev server）、agent-chat 智能体流对照、更多文档格式。

### 2026-09-17 第 2 轮（B8 完整移植 + 双栈就绪 + D3 冒烟）
- **B8 完整移植**（`5780cb0` 文件访问与 artifact 发布分离）：18/22 文件补丁直用；prompts.go（ArtifactOutputDir 动态化 + artifact 链接指引两条新增）、sandbox_ls.go（移除 /workspace 白名单强制 + 描述/schema/注释同步 + 删 inspectablePathError/inspectableRootsDescription + path import）、session_manager.go（cleanSessionWorkspaceWritePath 放宽到整沙箱保留 input 只读；cleanSessionWorkDir 改绝对路径语义 + install 模式保留 workspace/skills 范围）三处手工改写；三个旧行为测试按上游重写（AcceptsSandboxSkillRoot/AllowsTemporaryWorkDir/SandboxPaths 写入矩阵）+ prompts_shell_test 增补 + registry_journal_test 用例更新。
  - **教训**：`git show --stat | awk` 会截断长文件路径为 `.../xxx`，用截断名做 `--include` 时 git apply 静默跳过（零文件应用也退出 0）——B8 的 `agent_service_install_shell_test.go` 因此漏应用，首轮全量测试才暴露（TestOrdinaryAgentKeepsTheUnprivilegedShell 失败）。补应用后修复。
- **B9 回退**：依赖 A17 链基建（connectRemoteSession/validateSessionSummary/ExecShellCommandWithOutputSnapshot），部分应用编译失败后 `git apply -R` 逐文件精确回退，保留 B8。条目改为「随 A17 一起做」。
- **B10/B11**（MCP SQLite 测试稳定 / skill 安装竞态）：干净应用。
- **验证**：`go build ./...` EXIT=0；`go test ./internal/...` 全量 EXIT=0（100 包 ok，0 FAIL）。
- **D1/D2 双栈就绪**：fork :8080 健康；上游栈 `Up-WeKnora-*` 容器组 app :18080 healthy / UI :18081。上游 .env 生成方式与账号已记录于 D2。
- **D3 冒烟对照**：注册/登录/KB 创建/列表双栈全通。发现有意分歧 D3-1（KB 响应信封 `data` vs `knowledge_base`/`knowledge_bases`）——fork 三端已适配，⛔ 不改。
- 遗留：D3 深流程（需给上游栈配模型）；A8-A21；B12；C1-C7。

### 2026-09-17 第 1 轮（静态对照 + 第一批移植）✅
- 基线盘点：上游克隆 `2514e42`，差异统计入档；台账建立。
- 应用补丁：d812645（XMind 白名单）、c9d5987（初始化模型租户戳）、0ad08d4（消息关键词搜索 sqlite/mysql）、bca3a9f（OBS 虚拟托管）、d8be87d（BM25 分数限幅）、e80f5df（Serply 全套：provider+registry+前端 union+swagger 手改）、a7d3545（批量下载后端+Vue 前端，docs/api/knowledge.md 与 swagger.yaml hunk 冲突跳过）、105c177（摘要生成可选：base/config/post_process+上传确认弹窗+i18n）、820a14d（连接器文件名净化共享包，排除 fork 不存在的 code-slimming-audit.md）、419ff25+079e96a（原地替换 primitive，顺序应用）。
- 手工移植：55ec13a 去重上传（NOT IN failed/deleting + 矩阵测试追加到 fork 的 knowledge_duplicate_test.go）；special_tokens.go 新建 + anthropic_tools/openai_request/ollama 三处接线；b23ae05 IM 两处 QA goroutine useAgent defer 关闭 + qa_exit_test.go。
- 验证：`go build ./...` EXIT=0；相关 8 组包 `go test` 42 包全 ok（repository/service/chat/im/web_search/datasource/types），EXIT=0（重定向后读 $?）。
- 遗留：A8-A21 大条目、B8-B12 待核、C1-C7 前端同步、D 双栈实测、批量下载 swagger 条目补 regen。

### 2026-09-18 第 6 轮（D3/D6 agent-chat 智能体模式浏览器对照 + 两处 fork 缺陷修复）🔄

> 纪律例外说明：本轮按用户新指令以 `.worktrees/react-multiclient` 为工作目录做浏览器对照并修复 React 侧缺陷（提交 `5e382d0b`，与 cron lane 文件不相交，无提交冲突；cron R460 同期正常落地）。后端 DI 修复也在该分支——main 分支的 newAgentRuntime 无 usage 参数、不受影响，无需回植。

**环境（本轮新增/变化）**：
- fork 后端 WeKnora-app 容器曾丢失，已从 worktree compose（项目 react-multiclient，.env 从主仓复制）重建。旧镜像（2026-09-03, 14953fcb）已被覆盖删除，无法回滚——本轮被迫重建镜像。
- parity-mock 扩展 `/v1/rerank`（确定性 relevance_score），容器已重启生效；双栈 DB（Up 10000 / fork 10001）均插入 mock-rerank（Rerank/remote → parity-mock:18090）。
- 镜像构建坑：① `go install migrate` 需 GOPROXY_ARG=https://goproxy.cn,direct；② WITH_ANYDOC=1 的 rustup 步骤网络失败（build context 变更后缓存失效），对照验证用 WITH_ANYDOC=0 规避；③ 管道 tail 吞退出码事故再现一次（教训重申：一律重定向后读 $?）。
- worktree 分支镜像启动 panic（DI 顺序，见下）曾致 :8080 crash-loop——修复后已恢复。

**D3/D6 agent-chat 对照结论（上游 Vue :18081 vs fork React :5181，parity-up / parity-test 账号）**：
- 上游基线流程：侧边栏智能体页（4 内置：快速问答/智能推理/维基问答/数据分析师；卡片点击=编辑弹窗）→ 新对话 `/platform/creatChat` 输入区「智能体芯片」→ 富选择面板（管理入口/内置分组/未就绪门禁：smart-reasoning 必须配 对话模型+重排模型，未就绪项禁选+提示）→ 切换 toast「已切换到智能推理」→ 发送 → `/platform/chat/:id` 流式渲染 agent_steps（thought+iteration+timestamp+tool_calls；mock 无 tool_calls）→ 消息工具栏 复制/添加到知识库/请求信息。上游消息结构：content + agent_steps[] + execution_context（agent_config_hash/question_suggestions/locale/langfuse）。
- 上游附带观察：agent 编辑器字段集（模型*/ReRank 模型*/温度/最大生成Token（默认4096/沙箱24576）/思考模式/输出来源引用/最大迭代次数/LLM 超时；左导航 基础/知识检索/能力扩展 三组，含 MCP 服务、技能、长期记忆、发布渠道 IM 集成/网页嵌入）——后续 React 智能体编辑器 parity 的基线清单。另：上游消息行渲染 3 个空 src 的 t-image 占位（图片无法显示，mock 数据诱发，上游侧行为，fork 未模仿——待上游定性，暂不判差异）。

**发现并修复的 fork 缺陷（提交 5e382d0b，TDD）**：
1. **React agent-chat 全页崩溃**：`ChatHeaderMenu`（packages/views/src/chat/page.tsx）在 `if (!session) return null` 之后才调用 rename 聚焦 useEffect。发送后立即跳转 /platform/chat/:id 时会话尚未进入 sessions 列表 → 首渲染 null、次渲染 hook 数 12→13 → React 抛「Rendered more hooks than during the previous render」白屏，并连带 abort 后端 SSE（assistant 留空消息、agent 运行中止）。D6-1 遗留的「空 content assistant 消息冷加载白屏」真因即此（当时归因被路由 WIP 污染）。修复=effect 前移至 bail-out 之前；回归测试 `apps/web/src/chat/chat-header-hook-order.test.tsx`（HEAD 复现崩溃→修复后通过）。门禁：typecheck:web 0 错、test:web 1666/1666（+1）。
2. **worktree 分支后端启动 panic**：container.go 把 CommercialGateway/ExecutionGate/RemoteUsageService 三个 Provider 注册在 `Invoke(registerCraftHTTPHandlers)` 之后——dig 惰性解析使 newAgentRuntime 的 *RemoteUsageService 缺失，boot panic（2026-09-18 重建镜像必现；main 无此问题）。修复=三 Provider 前移至 newAgentRuntime 注册之前；`go build ./internal/container` 0 错。`go test ./internal/container` 的 TestWireCraftInteractionRegistrarRegistersPendingInteractions 在基线（无我改动）即失败——分支预存问题，未代解。

**发现未修（下轮候选）**：
- **i18n 缺口（功能差异）**：fork 新对话页主体英文（Hi, I am WeKnora— / Ask questions directly to the model / Select Agent / Chat model / Send），上游同页全中文；内置智能体卡片名/描述英文（Quick Answer/Smart Reasoning/…），上游中文（快速问答/智能推理/…）。fork 侧边栏/登录/KB 页均为中文——聊天域 locale 未接线或键缺失。
- **智能体选择器功能差距**：fork=原生 combobox（Select Agent 下拉）；上游=富面板（分组/管理入口/未就绪门禁+去配置直达/特性徽章/详情卡）。上游就绪门禁（smart-reasoning 需对话模型+重排模型）fork 前端未实现——**后端已同样强制**（knowledge_search 启用时无 rerank_model_id 直接报错「rerank model is not configured」，SSE error 事件，fork 前端以 alert+Retry 正确呈现）；数据侧已给 fork builtin-smart-reasoning 配置 mock-rerank（custom_agents.config.rerank_model_id）。
- 后端旧镜像滞后的 migration error（db_version 135 failed）已随新镜像消失（新代码迁移集含 135 down 文件）。

**端到端复验（2026-09-18 02:20，修复镜像 c974350a + 5e382d0b）✅**：原崩溃会话冷加载正常渲染（2 消息行、header menu 在位、无崩溃）；同会话重发智能体消息 → 后端 agent 运行完成（assistant 92 字符内容 + 1 agent_step）→ 前端流式渲染 +「思考与工具」折叠时间线（含 ✓ 完成标记，对应上游 AgentStreamDisplay 结构）。证据 `evidence/browser-agent-chat/fk-04-agent-chat-fixed-e2e.png`（另有 fk-02 i18n 缺口、fk-03 修复前崩溃、up-01/02/03 上游基线）。

**遗留**：上游 t-image 空占位定性；i18n 与选择器差距排期（上两条未修项）。

## G. 纪律与教训

1. 本任务在**主仓库 main** 工作；绝不触碰 worktree `codex/react-vue-parity-align`（cron 自动化每 30 分钟一轮在跑，提交纪律：只 add 自己的文件，绝不 `git add -A`）。
2. 移植上游提交用 `git -C /Users/wuyongjun/trea/weknora-upstream show <sha>` 取 diff；上游 fetch depth=60，更早提交需再 fetch。
3. agent 相关上游文件（internal/agent/*）是原版自研循环，fork01 已换 trpc-agent-go（internal/agent/trpc/）——移植这类文件必须做 trpc 工具接口适配，不能直接拷贝。
4. 退出码纪律：管道会吞退出码，构建/测试一律重定向后读 `$?`。
5. 数据库迁移编号已分叉：给 fork 追加上游迁移时用 fork 序号（000136+），不得照抄上游编号。
