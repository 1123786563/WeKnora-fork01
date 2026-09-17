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
| B8 | sandbox：文件访问与 artifact 发布分离 | `5780cb0` | 待核 | ⬜ 待办 |
| B9 | sandbox 远程调用去重（perf） | `1c43934` | 待核 | ⬜ 待办 |
| B10 | MCP 并发策略更新测试稳定（SQLite） | `ac4dd59` | 待核 | ⬜ 待办 |
| B11 | skill 后台安装竞态修复 | `fc7f37d` | 待核 | ⬜ 待办 |
| B12 | 客户端会话与集成请求处理对齐 | `d7ccd5b` | 待核 | ⬜ 待办 |
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
| D1 | fork 栈启动（OrbStack WeKnora-app :8080 + React :5181 + Vue :5180） | ⬜ 待办（OrbStack 可能休眠，恢复：`open -a OrbStack && docker start WeKnora-postgres WeKnora-docreader WeKnora-app`） |
| D2 | 上游栈启动（weknora-upstream docker-compose，端口需错开：8080→18080 等） | ⬜ 待办 |
| D3 | 核心流程一一对照（登录/KB/上传/解析/检索/聊天/流式/设置/图谱）逐项记录 | ⬜ 待办（依赖 D1/D2） |
| D4 | 对照账号 | fork 栈：parity-test@local.dev / Parity123456!（tenant 10000）；上游栈需新建 |

## E. 完成记录

### 2026-09-17 第 1 轮（静态对照 + 第一批移植）✅
- 基线盘点：上游克隆 `2514e42`，差异统计入档；台账建立。
- 应用补丁：d812645（XMind 白名单）、c9d5987（初始化模型租户戳）、0ad08d4（消息关键词搜索 sqlite/mysql）、bca3a9f（OBS 虚拟托管）、d8be87d（BM25 分数限幅）、e80f5df（Serply 全套：provider+registry+前端 union+swagger 手改）、a7d3545（批量下载后端+Vue 前端，docs/api/knowledge.md 与 swagger.yaml hunk 冲突跳过）、105c177（摘要生成可选：base/config/post_process+上传确认弹窗+i18n）、820a14d（连接器文件名净化共享包，排除 fork 不存在的 code-slimming-audit.md）、419ff25+079e96a（原地替换 primitive，顺序应用）。
- 手工移植：55ec13a 去重上传（NOT IN failed/deleting + 矩阵测试追加到 fork 的 knowledge_duplicate_test.go）；special_tokens.go 新建 + anthropic_tools/openai_request/ollama 三处接线；b23ae05 IM 两处 QA goroutine useAgent defer 关闭 + qa_exit_test.go。
- 验证：`go build ./...` EXIT=0；相关 8 组包 `go test` 42 包全 ok（repository/service/chat/im/web_search/datasource/types），EXIT=0（重定向后读 $?）。
- 遗留：A8-A21 大条目、B8-B12 待核、C1-C7 前端同步、D 双栈实测、批量下载 swagger 条目补 regen。

## G. 纪律与教训

1. 本任务在**主仓库 main** 工作；绝不触碰 worktree `codex/react-vue-parity-align`（cron 自动化每 30 分钟一轮在跑，提交纪律：只 add 自己的文件，绝不 `git add -A`）。
2. 移植上游提交用 `git -C /Users/wuyongjun/trea/weknora-upstream show <sha>` 取 diff；上游 fetch depth=60，更早提交需再 fetch。
3. agent 相关上游文件（internal/agent/*）是原版自研循环，fork01 已换 trpc-agent-go（internal/agent/trpc/）——移植这类文件必须做 trpc 工具接口适配，不能直接拷贝。
4. 退出码纪律：管道会吞退出码，构建/测试一律重定向后读 `$?`。
5. 数据库迁移编号已分叉：给 fork 追加上游迁移时用 fork 序号（000136+），不得照抄上游编号。
