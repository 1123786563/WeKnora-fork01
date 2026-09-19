# tRPC Native Agent P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立完整功能基线，验证固定 SDK 的原生执行与恢复边界，交付有证据的版本和接口决策，解除下游规划依赖。

**Architecture:** 在隔离 worktree 内创建测试探针和文档；复用当前 checkpoint 探针观察 SDK，不切换生产执行链。以行为证据区分原生能力、必要业务扩展和环境阻塞。

**Tech Stack:** Go 1.26.0，当前 trpc-agent-go v1.10.0，Go testing，SQLite/PostgreSQL；现有 pnpm 工作区。

**Spec:** [已确认规格](../specs/2026-09-19-trpc-native-agent-migration-design.md)。完整工作包及依赖见[总计划](2026-09-19-trpc-native-agent-migration.md)。

## Global Constraints

- “首版保留现有全部业务功能；验收后通过维护窗口一次切换。”
- “旧会话不恢复执行，也不隐式导入新上下文。”
- “归档不等于删除；未批准任何历史数据、附件或审计记录的清除。”
- “适配器只处理业务边界或原生组件缺失能力，每个适配器记录原因、覆盖测试和可删除条件。”
- “缺少凭据、服务或设备的项目标记 `blocked-env`，不得计为通过；必需项存在阻塞或失败时不得上线。”
- P0 只新增文档与测试探针；不修改线上路由、旧表、用户数据或 SDK 版本。版本升级结论若产生，在后续受审任务中执行升级。
- 所有日志脱敏；测试文件只访问临时目录或指定隔离数据库。依赖下载失败单独记环境失败，不当作行为 RED。

## Review Focus

1. 两空间同名会话：Task 2 原生 Session 身份探针与 Task 4 隔离契约。
2. channel 结束但任务未完成：Task 3 的终态与中断测试。
3. 工具执行结果不明：Task 3 副作用恢复测试与 Task 4 门槛。
4. 事件落库失败被吞掉：Task 4 要求可靠投影扩展，不将当前 best-effort 行为照搬。
5. 真实 Provider 缺失但恢复测试跳过：Task 3 检查 JSON test action，Task 5 发布门槛禁止误报。

## Task 1: 固定功能基线与证据台账

**Files:**
- Create: `docs/superpowers/plans/trpc-native/baseline.md`
- Create: `docs/superpowers/plans/trpc-native/features.tsv`
- Create: `docs/superpowers/plans/trpc-native/progress.md`
- Read: `go.mod`、`package.json`、`internal/application/service/agent_service.go`、`session_agent_qa.go`、`agent_run_graph.go`、`internal/container/agent_runtime.go`、`frontend.md`。

**Interfaces:** Consumes 当前源码与已确认规格；Produces TSV 字段 `id, category, behavior, entry, consumers, baseline_test, required_environment, target_phase, evidence_status`（以 Tab 分隔）。一行一个可验证行为，不以整个目录作为一项功能。

- [ ] 使用 using-git-worktrees 技能建隔离 worktree，记录绝对路径、HEAD、分支和 dirty 状态；包含已确认文档，不复制未获授权的前端脏改动。
- [ ] 执行下列只读命令，读取结果并在 baseline.md 保存精简事实和具体入口：

```bash
git rev-parse HEAD
git status --short
rg -n 'trpc-agent-go|^go |^replace' go.mod
rg -n 'CreateAgentEngine|ExecuteDurableRun|EngineType' internal/application/service internal/container internal/handler
rg --files internal/agent/tools internal/agent/skills internal/models/chat internal/sandbox
rg --files packages apps cli client | rg '(agent|chat|stream|session|memory|package.json)'
```

- [ ] 从注册和调用路径逐项盘点 Provider、工具、MCP/Skills、知识、Sandbox、Connector、专业委派、会话/Memory、计费/审批/恢复与全部客户端；保留代码中已存在的配置开关和失败状态。阅读相关实现与测试，不仅复制文件列表。
- [ ] baseline.md 记录实际支持的数据库和各平台脚本；读所有活跃客户端 package.json，区分 apps/mobile 和 apps/mobile-next 是否仍有消费者。
- [ ] 建立 features.tsv；例如表头和首条采用以下格式，首条测试入口读代码后记录实际函数名：

```text
id	category	behavior	entry	consumers	baseline_test	required_environment	target_phase	evidence_status
REC-001	recovery	工具结果不明时等待用户	internal/agent/runtime/tool_recovery.go	Agent Run	internal/agent/runtime/tool_recovery_test.go	isolated database	P2/P7	unverified
```

- [ ] 每个注册入口都对应至少一项清单，每项都有责任工作包；无消费者条目记录查找范围和依据后才允许排除。文档任务以覆盖审查为完成条件，不制造 RED。
- [ ] 记录 Task 1 passed 仅表示盘点已审查，所有运行验收保持 unverified。提交指定文件：

```bash
git add docs/superpowers/plans/trpc-native/baseline.md docs/superpowers/plans/trpc-native/features.tsv docs/superpowers/plans/trpc-native/progress.md
git commit -m 'docs: inventory native agent migration baseline'
```

## Task 2: 验证原生 LLMAgent / Runner / Session 工具往返

**Files:**
- Create: `internal/agent/nativeprobe/runner_test.go`
- Create: `docs/superpowers/plans/trpc-native/sdk-probes.md`

**Interfaces:** Consumes SDK `model.Model`、`llmagent.New`、`runner.NewRunner`、`session.Service`；Produces `TestNativeRunnerToolRoundTrip` 和 `TestSessionScopeKeysSeparateTenants` 的行为证据。`scriptedModel` 仅为测试确定性 Provider，不进入产品模型工厂。

- [ ] 从本机 `go env GOMODCACHE` 定位固定版本源码，核对以下代码引用的接口。若基线已变化，记录差异而不是静默跟随最新版。
- [ ] 将下列完整测试写入 `runner_test.go`；这是现有 SDK 的特征探针，可以首次直接通过，不伪造 RED。后续产品适配变更才要求行为 RED → GREEN。

```go
package nativeprobe

import (
    "context"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    "trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
    "trpc.group/trpc-go/trpc-agent-go/model"
    "trpc.group/trpc-go/trpc-agent-go/runner"
    "trpc.group/trpc-go/trpc-agent-go/session"
    "trpc.group/trpc-go/trpc-agent-go/session/inmemory"
    "trpc.group/trpc-go/trpc-agent-go/tool"
    "trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type scriptedModel struct{}

func (*scriptedModel) Info() model.Info {
    return model.Info{Name: "native-contract-probe"}
}

func (*scriptedModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
    if err := ctx.Err(); err != nil { return nil, err }
    msg := model.Message{Role: model.RoleAssistant}
    for _, input := range req.Messages {
        if input.Role == model.RoleTool && input.ToolID == "call-1" {
            msg.Content = "finished"
        }
    }
    if msg.Content == "" {
        msg.ToolCalls = []model.ToolCall{{
            ID: "call-1", Type: "function",
            Function: model.FunctionDefinitionParam{Name: "count", Arguments: []byte(`{}`)},
        }}
    }
    out := make(chan *model.Response, 1)
    out <- &model.Response{
        Object: model.ObjectTypeChatCompletion, Done: true,
        Choices: []model.Choice{{Message: msg}},
    }
    close(out)
    return out, nil
}

func TestNativeRunnerToolRoundTrip(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    var calls atomic.Int32
    count := function.NewFunctionTool(func(context.Context, struct{}) (string, error) {
        calls.Add(1)
        return "counted", nil
    }, function.WithName("count"), function.WithDescription("Count one invocation"))
    ag := llmagent.New("native-probe", llmagent.WithModel(&scriptedModel{}),
        llmagent.WithTools([]tool.Tool{count}))
    r := runner.NewRunner("native-probe", ag,
        runner.WithSessionService(inmemory.NewSessionService()))
    defer r.Close()
    events, err := r.Run(ctx, "user-1", "session-1", model.NewUserMessage("count"))
    require.NoError(t, err)
    finished := false
    for ev := range events {
        if ev == nil || ev.Response == nil { continue }
        require.Nil(t, ev.Error)
        for _, choice := range ev.Choices {
            if choice.Message.Content == "finished" { finished = true }
        }
    }
    require.NoError(t, ctx.Err())
    require.True(t, finished)
    require.EqualValues(t, 1, calls.Load())
}

func TestSessionScopeKeysSeparateTenants(t *testing.T) {
    ctx := context.Background()
    svc := inmemory.NewSessionService()
    defer svc.Close()
    a := session.Key{AppName: "weknora/tenant/1", UserID: "same", SessionID: "same"}
    b := session.Key{AppName: "weknora/tenant/2", UserID: "same", SessionID: "same"}
    _, err := svc.CreateSession(ctx, a, session.StateMap{"marker": []byte("a")})
    require.NoError(t, err)
    _, err = svc.CreateSession(ctx, b, session.StateMap{"marker": []byte("b")})
    require.NoError(t, err)
    gotA, err := svc.GetSession(ctx, a)
    require.NoError(t, err)
    gotB, err := svc.GetSession(ctx, b)
    require.NoError(t, err)
    require.Equal(t, []byte("a"), gotA.State["marker"])
    require.Equal(t, []byte("b"), gotB.State["marker"])
}
```

- [ ] 执行：

```bash
gofmt -w internal/agent/nativeprobe/runner_test.go
GOWORK=off go test -race ./internal/agent/nativeprobe -count=1 -v
```

预期：两个测试通过，工具真实进入 SDK 调用一次，明确最终内容存在；若超时/失败，记录实际 SDK 行为并诊断，不能只断言 event channel 已关闭。

- [ ] sdk-probes.md 区分结果：确定性 SDK 往返已验证；真实 Provider 未验证；Session 名称隔离仅证明 SDK 键空间，不证明服务端授权或生产数据库隔离。
- [ ] 独立审查确认探针没有使用 `internal/models/chat` 或旧 AgentEngine，且失败不会被吞掉。提交两个文件及台账。

## Task 3: 校准恢复证据与运行语义

**Files:**
- Read: `internal/agent/trpc/compatibility_probe.go`、`compatibility_probe_test.go`、`internal/agent/runtime/tool_recovery_test.go`、`internal/agent/recoverytest/matrix_test.go`、`.github/workflows/agent-recovery.yml`。
- Modify documentation: `docs/superpowers/plans/trpc-native/sdk-probes.md`、`progress.md`。
- Create: `docs/superpowers/plans/trpc-native/recovery-gaps.md`。

**Interfaces:** Consumes 现有 `RunCheckpointProbe(context.Context, string, bool) (ProbeReport, error)`；Produces checkpoint 能力证据与迁移缺口，不宣称当前自定义图就是最终原生组合。

- [ ] 阅读现有探针，确认使用临时数据库。运行基线：

```bash
GOWORK=off go test ./internal/agent/trpc -run 'TestCheckpointProbe|TestSQLiteSaverPendingWrites' -count=1 -v
GOWORK=off go test -race ./internal/agent/runtime -count=1 -v
```

预期：中断不标为完成；重开数据库恢复 pending writes 和工具 ID；不明结果遵守恢复策略。失败记录为现有基线问题，不直接改生产代码来“完成 P0”。

- [ ] 比较 Task 2 普通 Runner 与现有 GraphAgent 探针：列出普通 Agent 内部工具循环在哪些位置可持久化、批准和恢复；读取 SDK callback/graph 源码指出确切方法及调用顺序。源码不能证明外部效果恰好一次。
- [ ] 单独运行故障 harness 并保存结构化结果到工作区外临时日志：

```bash
GOWORK=off go test -json ./internal/agent/recoverytest -count=1 > /tmp/weknora-native-recovery-p0.json
```

- [ ] 解析 `Action=skip` 和失败项，读取测试所需环境变量；无真实 Provider/数据库时标记 blocked-env。临时日志路径只用于本轮，提交前将脱敏证据复制到规范的证据目录并更新台账。
- [ ] recovery-gaps.md 必须对六个间隙逐条作判断：模型结果到计划提交；审批到调用；调用成功到结果落库；结果落库到 checkpoint；checkpoint 到 Session；事件落库到客户端发送。每条包含当前行为、目标行为、P1/P2/P3/P7 责任和验证方式。
- [ ] 记录不可查询且非幂等调用等待用户；现有吞掉事件 append 失败不能作为目标投影实现。提交文档，不扩大本任务为恢复重写。

## Task 4: 全部能力与扩展决策矩阵

**Files:**
- Create: `docs/superpowers/plans/trpc-native/sdk-capabilities.tsv`
- Create: `docs/superpowers/plans/trpc-native/interfaces.md`
- Modify: `docs/superpowers/plans/trpc-native/sdk-probes.md`、`progress.md`。

**Interfaces:** Consumes Task 1 功能清单、Task 2/3 行为证据和固定版本源码；Produces `capability, sdk_version, source_symbol, evidence_level, result, gap, decision, owner_phase` 八列矩阵。result 取 verified / source-only / blocked-env / incompatible；source-only 不能转写为运行通过。

- [ ] 逐项读取 SDK 与项目实现，覆盖下表。只读 API/源码核对不需要修改依赖。若需要比较新版，先固定候选 tag/commit，取得官方源码，不用浮动 main 作为实施版本。

| 能力 | 必查契约 | 需要的行为证据 / 下游门槛 |
| --- | --- | --- |
| Runner/Agent | 事件终态、取消、工具循环、资源关闭 | Task 2；P3 补真实 Provider |
| Graph 恢复 | saver、pending writes、interrupt/resume、嵌套 Agent | Task 3；P3/P7 补目标组合的进程级恢复 |
| Model | 全部现有 Provider 与特殊参数、图片、reasoning/tool metadata | 旧 model_test 特殊字段逐项映射；P3 必需真实调用 |
| Tools/MCP | 动态发现、schema、调用拦截、OAuth、输出、取消 | 拦截必须发生在调用前；P2/P4 真实计数与撤权测试 |
| Skills/Sandbox | 安装/加载/授权、执行环境、长任务身份 | 原生加载不能绕过现有沙箱；P4 行为验证 |
| Session | 创建/列表/删除、历史过滤/压缩、事件幂等、持久化 | Task 2 仅键空间；P1 数据库与投影故障测试 |
| Memory | 用户与空间范围、删除/开关、工具与后台抽取 | P1/P3 禁用和撤权后不能继续写入 |
| 多 Agent | 调用身份、预算、取消、并行写入与委派结果 | P2/P4 验证权限不扩大和父子预算 |
| 用量/事件 | callback 时机、attempt ID、重复流、durable append | P2/P5/P7 验证费用和文本去重 |

- [ ] interfaces.md 固定后续计划需要的业务契约：认证范围来源、Session/Memory 键映射、Run/ToolCall/attempt 身份、错误分类、日志和 checkpoint 提交协调、事件版本、归档只读。每项引用当前真实类型或写出完整新 Go/TS 类型定义，不留下未定义的跨任务类型。
- [ ] 对每个缺口给出“原生采用 / 最小业务扩展 / 固定候选升级 / 阻塞并修订规格”之一；保留扩展必须说明理由、测试和删除条件。
- [ ] 当前版本可用不等于必须升级；新版文档有功能不等于当前版本有功能。目标版本选择列出证据和未完成验收，禁止全部填 verified。
- [ ] 本任务只有当所有功能类别有矩阵行、所有下游接口有确定版本来源或明确阻塞时完成文档交付；存在关键 blocked 项不解锁对应实现。
- [ ] 审查与限定文件提交；不修改 go.mod 来假装版本决策已执行。

## Task 5: 审定 P0 门槛并展开首批产品实施计划

**Files:**
- Create: `docs/superpowers/plans/trpc-native/p0-decision.md`
- Create after prerequisite evidence: `docs/superpowers/plans/2026-09-19-trpc-native-agent-p1-storage.md`
- Create after prerequisite evidence: `docs/superpowers/plans/2026-09-19-trpc-native-agent-p2-governance.md`
- Modify: `docs/superpowers/plans/trpc-native/progress.md`。

**Interfaces:** Consumes Task 1–4 的清单、证据与精确契约；Produces P0 go/no-go 和使用真实接口的 P1/P2 可执行计划。

- [ ] 用矩阵逐项核对目标规格；缺失能力保留原有功能的实现路径必须明确，关键版本/恢复组合仍无解则判 no-go。
- [ ] 在 p0-decision.md 分开报告：已实测、仅源码确认、环境阻塞、行为不满足、选定版本及扩展。no-go 时继续可独立验证工作，不跨越对应门槛。
- [ ] 对已具备前置证据的 P1/P2 使用 writing-plans 展开精确文件、完整失败测试、接口签名、迁移 SQL、RED/GREEN/回归命令、审查和 commit；禁止将这里的工作包描述直接交给实现者当作完整代码计划。
- [ ] 数据库迁移编号读取当前 HEAD 后分配；SDK Service 方法从选定版本核验；将旧调用消费者盘点写入删除边界。
- [ ] 核对五项 Review Focus 均有责任任务和具体测试；存储与治理共享接口排队修改。
- [ ] 限定文件提交并交用户审阅后续计划，沿用用户已选择的执行方式；本阶段不开始生产切换。

## 验证与交接

Task 1 → Task 2/Task 3 可独立进行 → Task 4 → Task 5。Task 2/3 的 sdk-probes.md 合并由集成人串行完成，避免并行覆写。

本计划中的测试代码在规划阶段只做源码接口核对，尚未编译或执行。执行时记录首次结果；探针失败是一项有效发现，但不是目标功能验收通过。P0 不等于全面迁移完成，P1–P9 必须分别展开、审查和验收。
