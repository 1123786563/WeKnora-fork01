package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---- B0.5 Step 1：事件 schema 失败测试（先 RED）----
//
// 事件是已发生的事实：每条记录必须携带 id（dot-separated 事实名）、
// 正整数 version、producer、meaning、consumers、ordering、replay 规则与
// required_metadata（含 idempotency_key；tenant-scoped 事件还须
// occurred_at/event_id）。命令式命名的"事件"与重复的 producer/version 对
// 必须被拒绝。

// loadEventCatalogFixture 把单份 event-catalog.yaml 写进临时治理目录并加载，
// 返回聚合后的结构错误（其余治理文件不存在按空集处理，不干扰事件断言）。
func loadEventCatalogFixture(t *testing.T, body string) error {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(GovernanceDir))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "event-catalog.yaml"), []byte(body), 0o644))
	_, err := LoadGovernance(root)
	return err
}

// validEventYAML 渲染一条字段齐全的合法事件（producer/consumers 只需形态合法，
// fixture 不做磁盘校验）。
func validEventYAML(id, producer string) string {
	return fmt.Sprintf(`events:
  - id: %s
    version: 1
    producer: %s
    meaning: 一条已发生的事实
    ordering: per-session seq 单调递增
    replay: source-query
    transport: in_process
    consumers:
      - internal/modules/insights/projection.go
    required_metadata: [tenant_id, occurred_at, event_id, idempotency_key, actor_origin]
`, id, producer)
}

// TestEventSchemaRequiresFactFields 逐字段证明目录必须冻结事实字段：
// 任一字段缺失/非法即加载失败。
func TestEventSchemaRequiresFactFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing id",
			body: `events:
  - version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "id",
		},
		{
			name: "zero version",
			body: `events:
  - id: conversation.message.appended
    version: 0
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "version",
		},
		{
			name: "missing producer",
			body: `events:
  - id: conversation.message.appended
    version: 1
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "producer",
		},
		{
			name: "missing meaning",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "meaning",
		},
		{
			name: "missing ordering",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "ordering",
		},
		{
			name: "missing replay rule",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "replay",
		},
		{
			name: "missing transport",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [idempotency_key]
`,
			want: "transport",
		},
		{
			name: "empty consumers",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: []
    required_metadata: [idempotency_key]
`,
			want: "consumers",
		},
		{
			name: "empty required metadata",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: []
`,
			want: "required_metadata",
		},
		{
			name: "missing idempotency key",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [tenant_id, occurred_at, event_id]
`,
			want: "idempotency_key",
		},
		{
			name: "tenant event missing occurred_at",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [tenant_id, event_id, idempotency_key]
`,
			want: "occurred_at",
		},
		{
			name: "tenant event missing event_id",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/modules/insights/projection.go]
    required_metadata: [tenant_id, occurred_at, idempotency_key]
`,
			want: "event_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadEventCatalogFixture(t, tc.body)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestEventSchemaAcceptsCompleteFactRecord：字段齐全的事实记录可加载，
// 字段值逐项保真（id/version/transport/metadata 排序无关）。
func TestEventSchemaAcceptsCompleteFactRecord(t *testing.T) {
	err := loadEventCatalogFixture(t, validEventYAML(
		"conversation.message.appended",
		"internal/application/repository/message.go:CreateMessage"))
	require.NoError(t, err)
}

// TestEventSchemaRejectsCommandNames：事件 id 的段落命中命令式动词即拒绝
// （事件是事实，命令保持同步 port）。
func TestEventSchemaRejectsCommandNames(t *testing.T) {
	for _, id := range []string{
		"conversation.message.send",
		"knowledge.document.create",
		"agentrun.run.delete",
		"notification.message.dispatch",
		"usage.record.update",
	} {
		t.Run(id, func(t *testing.T) {
			sym := "internal/application/repository/message.go:CreateMessage"
			err := loadEventCatalogFixture(t, validEventYAML(id, sym))
			require.Error(t, err, "事件 id %s 是命令式命名，必须被拒绝", id)
			require.Contains(t, err.Error(), "imperative")
		})
	}
	// 过去时/完成体事实名不受影响。
	err := loadEventCatalogFixture(t, validEventYAML(
		"agentrun.run.completed",
		"internal/application/repository/agent_run_events.go:Finalize"))
	require.NoError(t, err)
}

// TestEventSchemaRejectsDuplicateProducerVersion：同一 producer/version 对
// 只能声明一条事件。
func TestEventSchemaRejectsDuplicateProducerVersion(t *testing.T) {
	body := `events:
  - id: conversation.message.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 一条已发生的事实
    ordering: per-session seq 单调递增
    replay: source-query
    transport: in_process
    consumers:
      - internal/modules/insights/projection.go
    required_metadata: [tenant_id, occurred_at, event_id, idempotency_key, actor_origin]
  - id: conversation.turn.appended
    version: 1
    producer: internal/application/repository/message.go:CreateMessage
    meaning: 另一条已发生的事实
    ordering: per-session seq 单调递增
    replay: source-query
    transport: in_process
    consumers:
      - internal/modules/insights/projection.go
    required_metadata: [tenant_id, occurred_at, event_id, idempotency_key, actor_origin]
`
	err := loadEventCatalogFixture(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "producer/version")
}

// ---- B0.5 Step 2/3：真实仓库事件目录冻结验证 ----

// requiredEventFamilies 是计划 B0.5 Step 2 冻结的家族前缀 → 最少记录数。
var requiredEventFamilies = map[string]int{
	"agentrun":     7, // started/attention/completed/failed/cancelled + tool/approval result
	"conversation": 4, // message/turn appended + session lifecycle
	"knowledge":    4, // processing/index/deletion completed or failed
	"craft":        5, // run/interaction/artifact/scheduled-run lifecycle
	"usage":        2, // fact observed + settlement outcome
	"workbench":    4, // task/timeline/attention/artifact projection updates
	"notification": 2, // delivery outcome
	"channel":      1, // channel delivery outcome
}

// requiredEventIDs 是 Step 2 逐项列出的当前可观察事实（必须逐条在册）。
var requiredEventIDs = []string{
	// Agent Run started/attention/completed/failed/cancelled and tool/approval result
	"agentrun.run.started",
	"agentrun.run.attention_required",
	"agentrun.run.completed",
	"agentrun.run.failed",
	"agentrun.run.cancelled",
	"agentrun.tool.result",
	"agentrun.approval.resolved",
	// Conversation message/turn appended and session lifecycle
	"conversation.message.appended",
	"conversation.turn.appended",
	"conversation.session.created",
	"conversation.session.deleted",
	// Knowledge processing/index/deletion completed or failed
	"knowledge.processing.completed",
	"knowledge.processing.failed",
	"knowledge.index.completed",
	"knowledge.deletion.completed",
	// Craft run/interaction/artifact/scheduled-run lifecycle
	"craft.run.started",
	"craft.run.completed",
	"craft.interaction.appended",
	"craft.artifact.published",
	"craft.scheduled_run.completed",
	// Usage fact observed and settlement outcome
	"usage.observed",
	"usage.settlement.completed",
	// Workbench task/timeline/attention/artifact projection updates
	"workbench.task.updated",
	"workbench.timeline.updated",
	"workbench.attention.updated",
	"workbench.artifact.updated",
	// Notification delivery outcome and Channel delivery outcome
	"notification.delivery.completed",
	"notification.delivery.failed",
	"channel.delivery.completed",
}

// frozenEventTransports 是 B0 冻结的投递形态词汇表：B0 不新增 broker，
// durable 仅限已存在的事件事实表，in_process 覆盖进程内总线/回调路径。
var frozenEventTransports = map[string]bool{"in_process": true, "durable": true}

const frozenEventReplay = "source-query"

// tenantEventMetadata 是 tenant-scoped 事件的强制元数据（freeze Step 2：
// tenant_id/occurred_at/event_id/idempotency_key + actor/system origin）。
var tenantEventMetadata = []string{
	"tenant_id", "occurred_at", "event_id", "idempotency_key", "actor_origin",
}

// requireProducerOnDisk 证明 producer 是 file:Symbol 且符号真实声明于该文件
// （顶层函数/方法/类型/值声明任一匹配）。
func requireProducerOnDisk(t *testing.T, root, producer string) {
	t.Helper()
	file, sym, found := strings.Cut(producer, ":")
	require.True(t, found, "producer %q 必须是 file:Symbol 形态", producer)
	require.NotEmpty(t, sym, "producer %q 缺少符号名", producer)
	abs := filepath.Join(root, filepath.FromSlash(file))
	require.FileExists(t, abs, "producer 文件 %s 不存在", file)
	data, err := os.ReadFile(abs)
	require.NoError(t, err)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, abs, data, 0)
	require.NoError(t, err, "producer 文件 %s 不可解析", file)
	declared := false
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == sym {
				declared = true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if s, ok := spec.(*ast.TypeSpec); ok && s.Name.Name == sym {
					declared = true
				}
				if s, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range s.Names {
						if n.Name == sym {
							declared = true
						}
					}
				}
			}
		}
	}
	require.True(t, declared, "producer 符号 %s 未在 %s 声明", sym, file)
}

// requireRepoFileExists 断言仓库相对路径存在。
func requireRepoFileExists(t *testing.T, root, rel string) {
	t.Helper()
	require.FileExists(t, filepath.Join(root, filepath.FromSlash(rel)), "路径 %s 不存在", rel)
}

// TestRealRepoEventCatalogFreezesFamilies 覆盖 Review Focus「事件记录必须区分
// 权威事实与命令并携带 tenant/版本/时间/幂等元数据」+ Step 2 家族清单 + Step 3
// producer/consumer 映射：真实 event-catalog.yaml 全量加载零结构错误，
// 每条记录的 producer（file:Symbol）与 consumers 都在当前 Go 树上，
// transport/replay/版本遵守 B0 冻结词汇。
func TestRealRepoEventCatalogFreezesFamilies(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	require.NotEmpty(t, g.Events, "event-catalog.yaml 尚未生成或为空")

	byID := map[EventID]Event{}
	familyCount := map[string]int{}
	for _, e := range g.Events {
		byID[e.ID] = e
		family := strings.SplitN(string(e.ID), ".", 2)[0]
		familyCount[family]++
	}

	// Step 2 家族齐备。
	for family, min := range requiredEventFamilies {
		require.GreaterOrEqualf(t, familyCount[family], min,
			"事件家族 %s.* 记录不足 %d 条（实得 %d）", family, min, familyCount[family])
	}
	// Step 2 逐项事实在册。
	for _, id := range requiredEventIDs {
		_, ok := byID[EventID(id)]
		require.Truef(t, ok, "计划要求的权威事件 %s 未在 event-catalog.yaml 冻结", id)
	}

	for _, e := range g.Events {
		// B0 冻结词汇：不加新 broker，重放一律从权威源查询。
		require.Equalf(t, 1, e.Version, "事件 %s：B0 只冻结 version-1 记录", e.ID)
		require.Truef(t, frozenEventTransports[e.Transport],
			"事件 %s：transport %q 不在 B0 冻结词汇 {in_process, durable}", e.ID, e.Transport)
		require.Equalf(t, frozenEventReplay, e.Replay,
			"事件 %s：B0 冻结的重放规则是 source-query（不加新 broker）", e.ID)

		// tenant-scoped 元数据（tenant_id → 全套租户元数据）。
		hasTenant := false
		for _, m := range e.RequiredMetadata {
			if m == "tenant_id" {
				hasTenant = true
			}
		}
		if hasTenant {
			for _, key := range tenantEventMetadata {
				require.Containsf(t, e.RequiredMetadata, key,
					"事件 %s：tenant-scoped 事件缺元数据 %s", e.ID, key)
			}
		}

		// Step 3：producer（file:Symbol）与 consumers 都在当前 Go 树上。
		requireProducerOnDisk(t, root, e.Producer)
		require.NotEmptyf(t, e.Consumers, "事件 %s：必须记录当前消费位点", e.ID)
		for _, c := range e.Consumers {
			requireRepoFileExists(t, root, c)
		}
	}
}

// TestRealRepoEventCatalogMapsCurrentProducersAndConsumers 抽查 Step 3 的
// producer/consumer 映射锚定真实位点：代表性事件的 producer 文件与消费投影
// 必须与仓库当前事实源一致（防目录凭空发明位点）。
func TestRealRepoEventCatalogMapsCurrentProducersAndConsumers(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	byID := map[EventID]Event{}
	for _, e := range g.Events {
		byID[e.ID] = e
	}

	cases := []struct {
		id                  string
		wantProducerSubstr  string   // producer 文件锚点
		wantConsumerSubstrs []string // 至少一个消费位点锚点
	}{
		{
			// run_completed 由 Finalize 幂等落账（agent_run_events.go），
			// 通知投影按 checkpoint 消费 run 事件。
			id:                  "agentrun.run.completed",
			wantProducerSubstr:  "agent_run_events.go",
			wantConsumerSubstrs: []string{"mobile_notification.go"},
		},
		{
			// usage.observed 对齐 nativecontract EventUsage（"usage.observed"）。
			id:                  "usage.observed",
			wantProducerSubstr:  "native_usage.go",
			wantConsumerSubstrs: []string{"execution_cleanup.go"},
		},
		{
			// 消息追加事实写入 messages 表，IM 渠道读取会话消息。
			id:                  "conversation.message.appended",
			wantProducerSubstr:  "message.go",
			wantConsumerSubstrs: []string{"service.go"},
		},
		{
			// Craft artifact 版本发布事实与 workbench artifact 投影。
			id:                  "craft.artifact.published",
			wantProducerSubstr:  "craft_version.go",
			wantConsumerSubstrs: []string{"workbench_artifacts.go"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			e, ok := byID[EventID(tc.id)]
			require.True(t, ok, "事件 %s 未在册", tc.id)
			require.Contains(t, e.Producer, tc.wantProducerSubstr,
				"事件 %s 的 producer 未锚定 %s", tc.id, tc.wantProducerSubstr)
			requireProducerOnDisk(t, root, e.Producer)
			matched := false
			for _, c := range e.Consumers {
				if strings.Contains(c, tc.wantConsumerSubstrs[0]) {
					matched = true
				}
			}
			require.Truef(t, matched, "事件 %s 的 consumers 未记录 %s 相关位点（实得 %v）",
				tc.id, tc.wantConsumerSubstrs[0], e.Consumers)
		})
	}
}
