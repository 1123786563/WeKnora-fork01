# Craft 运行时纵向验收（R07：两轮真实执行）

本文件是 R07 的验收证据落盘：同一工作区、同一 OpenCode 子 session，跨两轮委派持续修改——第一轮生成按月报告，第二轮改写为季度汇总；OC session 不变、文件摘要变化、两次 Run 分别可追溯。两份证据分开记录：受控模型协议证据（确定性端点，证明全链路协议与持久化）与真实模型产品证据（非脚本化生成，证明真实模型可用性）。两份证据的执行通道完全一致：真实锁定二进制 serve、真实 HTTP、真实 SQLite 迁移链与 G1 持久化入口；测试内没有任何"直接写 output 冒充生成"的路径。

## 执行环境（两份证据共用）

| 项 | 值 |
| --- | --- |
| 锁定二进制 | /Users/wuyongjun/.opencode/bin/opencode |
| 版本 / SHA256 | 1.18.4 / 9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98（与 testdata/protocol-lock.json、docker/craft/opencode.lock.json 一致，测试加载校验） |
| 隔离 | XDG_DATA_HOME / XDG_CONFIG_HOME / XDG_STATE_HOME 全部指向临时目录；环境变量中含 API_KEY / TOKEN / SECRET 的条目全部剔除；用户真实 opencode 配置、真实 GLM key、真实数据库零接触 |
| 持久化 | 真实 SQLite 迁移链（migrations/sqlite 000000–000042，含 000041_craft、000042_craft_versions）逐条应用；运行经 AgentRunStore.Admit/Claim/EnsureToolPlan/Finalize 生产 API 创建；委派经 CraftStore（PrepareTask/SaveResult/GetTask/GetResult） |
| 驱动入口 | internal/agent/opencode Executor（NewExecutor(client, store, emit)）真实 Execute；prompt 计数由注入 Client 的 RoundTripper 对真实 POST /session/{id}/prompt_async 计数 |

## RED 证据（Step 1–2）

先只落 brief 原文测试 TestLiveReportRejectsFreshSession（live_test.go），随后运行：

    $ go test ./internal/agent/opencode -run TestLiveReport -count=1
    internal/agent/opencode/live_test.go:6:7: undefined: LiveReport
    internal/agent/opencode/live_test.go:7:5: undefined: ValidTwoTurn
    internal/agent/opencode/live_test.go:11:6: undefined: ValidTwoTurn
    FAIL github.com/Tencent/WeKnora/internal/agent/opencode [build failed]
    EXIT=1

目标接口不存在导致编译失败——brief 预期的 RED（非依赖/网络故障）。

## 证据一：受控模型协议证据（两轮真实执行）

命令与退出码：

    $ CRAFT_LIVE=1 go test ./internal/agent/opencode -run TestLiveCraftTwoTurns -count=1 -v -timeout=12m
    --- PASS: TestLiveCraftTwoTurns (21.23s)      EXIT=0

（另一次独立运行 PASS 17.27s；R01 两个 live 测试同批亦 PASS，整批 ok 28.286s EXIT=0。）

受控端点：测试内 httptest 的 OpenAI-compatible 本地端点（@ai-sdk/openai-compatible provider 注入隔离 config，model=mock/mock-model，无任何真实 key）。每轮第一次响应流式返回 write 工具调用（真实 opencode 内置 write 工具落盘 report.md），工具结果回传后第二次响应返回最终文本。

关键断言全部通过（值取自最终运行日志，均为真实 HTTP / 持久化读回）：

| 断言 | 实测值 |
| --- | --- |
| OC session（两轮不变） | ses_f690a9ac2ffeZ8ZJQilARYUhHz（工作区绑定 revision 不变；另一次运行为 ses_f690bdaaaffeq8d6wNNg4mOrG4，行为一致） |
| prompt_async POST 计数 | 恰好 2 次，且只打到同一 session（RoundTripper 实测） |
| report.md 摘要变化 | 第一轮 sha256 f467b6119d2a752174c893fc4884a4726f714f8429930da07b31766e3fba485f → 第二轮 sha256 75d949cc7ca76fb16aef5d42f98ebc320008e492ae94f7c4cb0c73eac7dffbe6（受控内容确定性，两次运行摘要一致；第二轮内容含"季度"） |
| 两 Run 可追溯 | agent_runs 中 r-craft-live-a、r-craft-live-b 共 2 行；craft_delegations 中 call-live-a、call-live-b 状态 succeeded 共 2 行；GetTask/GetResult 各自读回：两个不同 TaskID，摘要分别为"已按月生成 report.md（2026-01 东区 100，2026-02 西区 200）。"与"已把 report.md 改写为季度汇总（2026-Q1 总计 300）。" |
| session 消息投影 | ≥4 条消息；两条 user 消息 ID 恰为持久化的两个 PromptMessageID（服务器采纳客户端升序 msg_ ID，第二个 > 第一个）；两轮 assistant 均 finish=stop 且文本引用 report.md |
| 运行事件 | 两个委派各自 emit delegation.started 与 delegation.finished |
| 模型端点往返 | 共 5 次调用；逐调用角色序列（真实请求解析）：call1: system,user,user；call2: system,user（opencode 自身发起的辅助请求，如实记录）；call3: system,user,assistant,tool；call4: system,user,assistant,tool,assistant,user；call5: system,user,assistant,tool,assistant,user,assistant,tool。call1→call3 完成第一轮（工具调用→工具结果→终文本），call4→call5 完成第二轮 |
| 一次一轮串行 | 第一轮 Execute 成功后经生产 API Finalize 关闭 r-craft-live-a 并释放 session 槽位，才 Admit 第二轮 run（session 单活跃 run 约束被真实遵守，未绕过） |

sales.csv fixture（testdata/sales.csv，复制进 serve 工作目录）：

    date,region,revenue
    2026-01-01,东区,100
    2026-02-01,西区,200

## 证据二：真实模型产品证据（非脚本化 CSV 生成）

命令同上（TestLiveCraftTwoTurns 内独立子通道 runLiveRealModelTurn；同一次 PASS 运行内完成）。第二套 XDG 全隔离 serve，无 provider 配置——默认模型为 opencode 内置免费模型；未配置、未读取、未消耗任何用户真实凭据（授权 key 缺席，按任务红线不消耗用户真实凭据）。

非脚本化 prompt（模型自行决定动作）："Read sales.csv in the current directory and write monthly-summary.md with a short revenue summary by month. Do not ask any question."

最终运行实测（两次运行均成功，产物非确定，证明确为真实模型输出）：

- 运行 A：真实写出 monthly-summary.md（sha256 前缀 2824bd05b9ed907e），模型回复 "monthly-summary.md written with revenue totals for January (100) and February (200), totaling 300."
- 运行 B：真实写出 monthly-summary.md（sha256 前缀 f5360be61d42df0b），模型回复 "Done. Wrote monthly-summary.md with revenue totals for Jan (100) and Feb (200) 2026."

数值与 sales.csv 一致（100 + 200 = 300），finish=stop，判定 check real_model_turn=passed。失败路径的诚实约定：若真实模型不可用（无网络/超时/未写产物），该 check 记为 not_run 并显式输出 REAL-MODEL EVIDENCE NOT COLLECTED，绝不默认通过——本次未触发。

## GREEN 与回归

| 命令 | 结果 |
| --- | --- |
| go test ./internal/agent/opencode -run TestLiveReport -count=1 | ok EXIT=0（TestLiveReportRejectsFreshSession PASS） |
| go test ./internal/agent/opencode -count=1 -timeout 480s | ok EXIT=0：42 PASS + 1 SKIP（TestLiveCraftTwoTurns 未设 CRAFT_LIVE，skip 原因显式打印 "live two-turn execution NOT verified"，不算通过） |
| CRAFT_LIVE=1 go test ./internal/agent/opencode -run 'TestLive' -count=1 -v -timeout=12m | ok EXIT=0：R01 两个 live + TestLiveReportRejectsFreshSession + TestLiveCraftTwoTurns 全 PASS |
| go vet ./internal/agent/opencode/ | EXIT=0 干净 |
| gofmt -l internal/agent/opencode/ | 空 |

R01–R04 既有 41 个测试全部保持 PASS，未破坏。

## 边界与限制（如实）

1. 本阶段验收的是执行纵向链路（委派→OC 子 session→真实工具落盘→持久化可追溯）；完整预览与用户浏览器交付仍依赖 W06。
2. 受控模型内容是确定性的（证明协议与链路，不证明模型质量）；真实模型证据由内置免费模型承担。若验收方要求"实际配置的授权模型"（如用户 GLM key），本环境无授权 key，按任务红线未消耗用户真实凭据，该项记为未执行。
3. 继承 R01 遗留：container_digest 仍为 pending（容器内 linux 二进制摘要未回填）；live 测试依赖本机锁定二进制绝对路径，CI 环境会显式 skip。
4. call2（system,user）为 opencode serve 自身发起的辅助请求（非两轮对话的一部分），受控端点对其做确定性应答；已如实记录在协议 trace 中。
