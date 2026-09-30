package main

import (
	"bytes"
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

// TestEventSchemaRequiresTenantScopedActorOrigin 覆盖 OCR R1
// #b0-ocr-r1-event-metadata-actor-origin：event-catalog.yaml 头注释声明
// "每条 tenant-scoped 事件强制 tenant_id/occurred_at/event_id/idempotency_key/
// actor_origin"，validate 的 tenant-scoped 级联清单必须与声明一致——
// required_metadata 缺 actor_origin 必须加载失败（此前级联清单只含
// occurred_at/event_id，缺 actor_origin 无人拦截）。
func TestEventSchemaRequiresTenantScopedActorOrigin(t *testing.T) {
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
    required_metadata: [tenant_id, occurred_at, event_id, idempotency_key]
`
	err := loadEventCatalogFixture(t, body)
	require.Error(t, err, "tenant-scoped 事件缺 actor_origin 必须被拒绝")
	require.Contains(t, err.Error(),
		`events[0].required_metadata: tenant-scoped event must also require "actor_origin"`)
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

// ---- OCR R1 #15：CheckEvents 事件目录漂移守卫（守卫路径补齐）----
//
// RunPassBGuard 此前只聚合 Ownership/Contracts/Ambiguity/Rulings/Brief，
// g.Events 仅剩 schema 校验与计数：目录删除或位点漂移不阻断 CI。
// CheckEvents 把事件目录与仓库发现对照：目录非空、producer 符号真实声明
// （DiscoverSymbol，含方法）、consumers 在 Go 树上、transport/replay/version
// 遵守 B0 冻结词汇。

// fixtureEventForCheck 构造一条锚定 contractrepo fixture 的合法事件。
func fixtureEventForCheck(id string) Event {
	return Event{
		ID:               EventID(id),
		Version:          1,
		Producer:         "internal/router/routes_tenant.go:RegisterTenantRoutes",
		Meaning:          "fixture 已发生的事实",
		Ordering:         "per-fixture 单调",
		Replay:           "source-query",
		Transport:        "in_process",
		Consumers:        []string{"internal/container/container.go"},
		RequiredMetadata: []string{"idempotency_key"},
	}
}

// TestCheckEventsHappyPathProducesNoDiagnostics：合法冻结事件零诊断。
func TestCheckEventsHappyPathProducesNoDiagnostics(t *testing.T) {
	root := contractRepoRoot(t)
	g := &Governance{Events: []Event{fixtureEventForCheck("conversation.message.appended")}}
	diags := CheckEvents(g, fixtureContractDiscovery(t, root))
	require.Empty(t, diags, "diags: %v", diags)
}

// TestCheckEventsDiagnostics 表驱动覆盖漂移面：producer 符号/文件缺失或形态
// 非法、consumer 不在 Go 树、transport/replay/version 越出 B0 冻结词汇。
func TestCheckEventsDiagnostics(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(e *Event)
		check string
		want  string
	}{
		{
			name: "producer symbol not declared",
			mut: func(e *Event) {
				e.Producer = "internal/router/routes_tenant.go:GhostRoutes"
			},
			check: "event-producer-missing",
			want:  "GhostRoutes",
		},
		{
			name: "producer file missing",
			mut: func(e *Event) {
				e.Producer = "internal/router/ghost.go:RegisterTenantRoutes"
			},
			check: "event-producer-missing",
			want:  "ghost.go",
		},
		{
			name: "producer malformed",
			mut: func(e *Event) {
				e.Producer = "internal/router/routes_tenant.go"
			},
			check: "event-producer-missing",
			want:  "file:Name",
		},
		{
			name: "consumer missing from go tree",
			mut: func(e *Event) {
				e.Consumers = []string{"internal/container/ghost.go"}
			},
			check: "event-consumer-missing",
			want:  "ghost.go",
		},
		{
			name: "transport outside frozen vocabulary",
			mut: func(e *Event) {
				e.Transport = "kafka"
			},
			check: "event-transport-frozen",
			want:  "kafka",
		},
		{
			name: "replay outside frozen vocabulary",
			mut: func(e *Event) {
				e.Replay = "event-sourced-broker"
			},
			check: "event-replay-frozen",
			want:  "source-query",
		},
		{
			name: "version beyond B0 freeze",
			mut: func(e *Event) {
				e.Version = 2
			},
			check: "event-version-frozen",
			want:  "version-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := contractRepoRoot(t)
			e := fixtureEventForCheck("conversation.message.appended")
			tc.mut(&e)
			g := &Governance{Events: []Event{e}}
			diags := CheckEvents(g, fixtureContractDiscovery(t, root))
			diag, ok := findDiag(diags, tc.check)
			require.True(t, ok, "want check %q, got diags: %v", tc.check, diags)
			require.Contains(t, diag.Message, tc.want)
		})
	}
}

// TestCheckEventsRequiresNonEmptyCatalog：目录为空（含整份 event-catalog.yaml
// 缺失按空集加载的形态）必须报 event-catalog-empty。
func TestCheckEventsRequiresNonEmptyCatalog(t *testing.T) {
	diags := CheckEvents(&Governance{}, &Discovery{})
	require.Len(t, diags, 1, "空目录只报一条结构性诊断，got: %v", diags)
	_, ok := findDiag(diags, "event-catalog-empty")
	require.True(t, ok, "空目录必须报 event-catalog-empty，got: %v", diags)
}

// TestCheckEventsProducerMayBeMethod：真实目录的 producer 绝大多数是带接收者
// 的方法（如 gate.go:RequestAndWait），DiscoverSymbol 必须以方法回退命中
// ——契约符号语义不变（顶层声明优先，方法仅回退）。
func TestCheckEventsProducerMayBeMethod(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/store/store.go", `package store

// Store 是 fixture 事件生产者宿主。
type Store struct{}

// AppendFact 是方法形态的 producer 符号。
func (s *Store) AppendFact(id string) error { return nil }
`)
	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	d := &Discovery{Root: root, GoFiles: files, Imports: imports}

	e := fixtureEventForCheck("x.fact.appended")
	e.Producer = "internal/store/store.go:AppendFact"
	e.Consumers = []string{"internal/store/store.go"}
	diags := CheckEvents(&Governance{Events: []Event{e}}, d)
	require.Empty(t, diags, "diags: %v", diags)

	// 方法同样参与存在性校验：方法被删除必须报 producer-missing。
	e.Producer = "internal/store/store.go:DropFact"
	diags = CheckEvents(&Governance{Events: []Event{e}}, d)
	_, ok := findDiag(diags, "event-producer-missing")
	require.True(t, ok, "方法符号缺失必须报 event-producer-missing，got: %v", diags)
}

// TestCheckEventsProducerOutsideGoTreeRejected 覆盖 OCR R2 f2：producer 文件
// 必须在发现的 Go 树上。DiscoverSymbol 只 os.Stat+ParseFile，不经 discoverGoTree
// 剪枝——testdata/ 点前缀目录下的文件磁盘存在且声明同名符号，但不参与构建，
// 不得作为 producer 锚点（与契约符号侧的 goFileSet 文件预检同构）。
func TestCheckEventsProducerOutsideGoTreeRejected(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/store/producer.go", `package store

// EmitFact 是正常树上的 fixture 事件生产者。
func EmitFact() {}
`)
	writeDiscoverFixture(t, root, "internal/store/testdata/producer.go", `package store

// EmitFact 在 testdata 下：磁盘存在且声明同名符号，但不参与构建。
func EmitFact() {}
`)
	writeDiscoverFixture(t, root, ".worktrees/passb-b0/producer.go", `package store

// EmitFact 在点前缀目录（git worktree 树）下：发现树整体剪枝。
func EmitFact() {}
`)

	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/store/producer.go"}, files,
		"fixture 前置：testdata 与点前缀目录必须被 discoverGoTree 剪枝")
	d := &Discovery{Root: root, GoFiles: files, Imports: imports}

	// 阳性对照：同一符号锚定正常树上文件零诊断——翻转结果的只有树成员性，
	// 不是符号存在性（三个文件声明完全同名符号）。
	e := fixtureEventForCheck("x.fact.emitted")
	e.Producer = "internal/store/producer.go:EmitFact"
	e.Consumers = []string{"internal/store/producer.go"}
	require.Empty(t, CheckEvents(&Governance{Events: []Event{e}}, d),
		"正常树上的 producer 必须零诊断")

	cases := []struct{ name, producer, want string }{
		{
			name:     "testdata directory",
			producer: "internal/store/testdata/producer.go:EmitFact",
			want:     "internal/store/testdata/producer.go",
		},
		{
			name:     "dot-prefixed directory",
			producer: ".worktrees/passb-b0/producer.go:EmitFact",
			want:     ".worktrees/passb-b0/producer.go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := fixtureEventForCheck("x.fact.emitted")
			e.Producer = tc.producer
			e.Consumers = []string{"internal/store/producer.go"}
			diags := CheckEvents(&Governance{Events: []Event{e}}, d)
			diag, ok := findDiag(diags, "event-producer-missing")
			require.True(t, ok,
				"树外 producer（磁盘存在且声明同名符号）必须报 event-producer-missing，got: %v", diags)
			require.Contains(t, diag.Message, tc.want)
		})
	}
}

// TestRealRepoCheckEventsZeroDiagnostics：真实仓库事件目录对照零诊断。
func TestRealRepoCheckEventsZeroDiagnostics(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	d, err := DiscoverPassB(root)
	require.NoError(t, err)
	require.NotEmpty(t, g.Events)
	diags := CheckEvents(g, d)
	require.Empty(t, diags, "diags: %v", diags)
}

// TestCLIBlocksOnEventCatalogDrift 覆盖 OCR R1 #15 验收的 CLI 路径：
// 消费方漂移到不存在的文件时守卫以 1 退出（make check-passb-readiness 阻断）。
func TestCLIBlocksOnEventCatalogDrift(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, filepath.Join(root, "docs/architecture/moves/knowledge.yaml"), `module: knowledge
description: fixture manifest for event drift
legacy_files:
  - path: internal/application/repository/widget.go
    reason: fixture legacy file
    passb_task: B-knowledge
`)
	writeMainTestFile(t, filepath.Join(root, "internal/application/repository/widget.go"),
		"package repository\n")
	writeMainTestFile(t, filepath.Join(root, "internal/application/repository/producer.go"),
		"package repository\n\n// EmitWidget 是 fixture 事件生产者。\nfunc EmitWidget() {}\n")
	writeMainTestFile(t, filepath.Join(root, "tools/architectureguard/check.go"), `package architectureguard

type importException struct {
	ImporterFile string
	ImportedPath string
	Reason       string
	PassBTask    string
}

var importExceptions = []importException{
	{
		ImporterFile: "internal/modules/agentruntime/agent/engine.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat",
		Reason:       "fixture 预存横向包耦合",
		PassBTask:    "B-agentruntime",
	},
}
`)
	writeMainTestFile(t, filepath.Join(root, filepath.FromSlash(GovernanceDir), "event-catalog.yaml"), `events:
  - id: widget.fact.emitted
    version: 1
    producer: internal/application/repository/producer.go:EmitWidget
    meaning: fixture 已发生的事实
    ordering: per-widget 单调
    replay: source-query
    transport: in_process
    consumers:
      - internal/application/repository/ghost_consumer.go
    required_metadata: [idempotency_key]
`)

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"-root", root})
	require.Equal(t, 1, code, "消费方漂移必须以 1 退出，stderr:\n%s", stderr.String())
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "event-consumer-missing: widget.fact.emitted:",
		"漂移诊断必须出现在 CLI 输出中")
}
