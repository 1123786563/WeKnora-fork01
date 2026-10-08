package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 四份治理文件的合法模板（与 testdata/valid 同构），表驱动用例覆写单个文件
// 制造单一缺陷；覆写值为空串表示删除该文件。
const (
	tplOwnershipMatrix = `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
aliases:
  - old_import_path: internal/application/service/chat_pipeline
    plan: 35-conversation-program
    delete_barrier: ib2
`

	tplContracts = `contracts:
  - id: conversation-route-set
    owner: conversation
    kind: route-set
    symbol: internal/conversation/routes.go
    signature: ""
    stability: frozen
    consumers: []
    characterization_tests: []
    items:
      - "POST /api/v1/session/chat"
`

	tplEventCatalog = `events:
  - id: conversation.message.appended
    version: 1
    producer: conversation
    meaning: 会话已追加一条消息（事实记录，非命令）
    ordering: per-session append order
    replay: source-query
    transport: in_process
    consumers:
      - internal/insights
    required_metadata:
      - tenant_id
      - occurred_at
      - event_id
      - idempotency_key
      - actor_origin
`

	tplExceptionLedger = `exceptions:
  - id: exc-0001
    from: internal/modules/appconnector/adapter.go
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
`
)

// writeGovernanceRepo 在临时目录里布置一份治理文件集并返回仓库根。
func writeGovernanceRepo(t *testing.T, overrides map[string]string) string {
	t.Helper()
	files := map[string]string{
		"docs/architecture/passb/ownership-matrix.yaml": tplOwnershipMatrix,
		"docs/architecture/passb/contracts.yaml":        tplContracts,
		"docs/architecture/passb/event-catalog.yaml":    tplEventCatalog,
		"docs/architecture/passb/exception-ledger.yaml": tplExceptionLedger,
	}
	for name, content := range overrides {
		files[name] = content
	}
	root := t.TempDir()
	for name, content := range files {
		if content == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}

func TestLoadGovernanceAcceptsValidFixture(t *testing.T) {
	g, err := LoadGovernance("testdata/valid")
	require.NoError(t, err)

	require.Len(t, g.Legacy, 2)
	require.Len(t, g.Aliases, 1)
	require.Len(t, g.Contracts, 2)
	require.Len(t, g.Events, 1)
	require.Len(t, g.Exceptions, 1)

	// 按稳定 ID 排序：legacy 按路径升序（yaml 里故意倒序）。
	require.Equal(t, "internal/application/service/alpha_chat.go", g.Legacy[0].Path)
	require.Equal(t, "internal/application/service/zeta_widget.go", g.Legacy[1].Path)
	require.Equal(t, LegacyID("internal/application/service/alpha_chat.go"), g.Legacy[0].Key())

	// 字段完整落位。
	require.Equal(t, "conversation", g.Legacy[0].Module)
	require.Equal(t, PlanID("35-conversation-program"), g.Legacy[0].Plan)
	require.Equal(t, "internal/conversation/service", g.Legacy[0].Destination)
	require.Equal(t, "ib3", g.Legacy[0].DeleteBarrier)

	// contracts 按排序：conversation-route-set < workbench-task-port。
	require.Equal(t, ContractID("conversation-route-set"), g.Contracts[0].ID)
	require.Equal(t, "route-set", g.Contracts[0].Kind)
	require.Equal(t, EventID("conversation.message.appended"), g.Events[0].ID)
	require.Equal(t, 1, g.Events[0].Version)
	require.Equal(t, "in_process", g.Events[0].Transport)
	require.Equal(t, ExceptionID("exc-0001"), g.Exceptions[0].ID)
	require.Equal(t, "ib2", g.Exceptions[0].RemoveAt)
}

func TestLoadGovernanceRejectsUnknownField(t *testing.T) {
	_, err := LoadGovernance("testdata/unknown-field")
	require.ErrorContains(t, err, "field")
}

func TestLoadGovernanceRejectsWildcardPath(t *testing.T) {
	_, err := LoadGovernance("testdata/wildcard-path")
	require.ErrorContains(t, err, "exact repository path")
}

func TestLoadGovernanceRejectsDuplicateID(t *testing.T) {
	_, err := LoadGovernance("testdata/duplicate-id")
	require.ErrorContains(t, err, "duplicate")
}

func TestLoadGovernanceTreatsMissingFilesAsEmpty(t *testing.T) {
	g, err := LoadGovernance(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, g.Legacy)
	require.Empty(t, g.Aliases)
	require.Empty(t, g.Contracts)
	require.Empty(t, g.Events)
	require.Empty(t, g.Exceptions)
}

func TestLoadGovernanceNormalizesLeadingDotSlash(t *testing.T) {
	root := writeGovernanceRepo(t, map[string]string{
		"docs/architecture/passb/ownership-matrix.yaml": `legacy_files:
  - path: ./internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
	})
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	require.Len(t, g.Legacy, 1)
	require.Equal(t, "internal/application/service/alpha_chat.go", g.Legacy[0].Path)
}

func TestLoadGovernanceRejectsStructuralViolations(t *testing.T) {
	cases := []struct {
		name string
		file string
		body string
		want string
	}{
		{
			name: "legacy absolute path",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: /internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "absolute",
		},
		{
			name: "legacy parent traversal",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/../../alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: `".."`,
		},
		{
			name: "legacy backslash path",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal\application\service\alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "slash",
		},
		{
			name: "legacy empty path",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: ""
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "path",
		},
		{
			name: "legacy empty plan owner",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: ""
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "plan",
		},
		{
			name: "legacy unknown module id",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversations
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "unknown module",
		},
		{
			name: "legacy bad plan id form",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: conversation
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "plan id",
		},
		{
			name: "legacy bad delete barrier",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: someday
`,
			want: "delete_barrier",
		},
		{
			name: "legacy empty destination",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: ""
    integration_owner: ""
    delete_barrier: ib3
`,
			want: "destination",
		},
		{
			name: "alias wildcard path",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `aliases:
  - old_import_path: internal/application/service/*
    plan: 35-conversation-program
    delete_barrier: ib2
`,
			want: "exact repository path",
		},
		{
			name: "alias empty plan owner",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `aliases:
  - old_import_path: internal/application/service/chat_pipeline
    plan: ""
    delete_barrier: ib2
`,
			want: "plan",
		},
		{
			name: "contract unknown kind",
			file: "docs/architecture/passb/contracts.yaml",
			body: `contracts:
  - id: conversation-route-set
    owner: conversation
    kind: magic-port
    symbol: internal/conversation/routes.go
    signature: ""
    stability: frozen
`,
			want: "kind",
		},
		{
			name: "contract unknown owner module",
			file: "docs/architecture/passb/contracts.yaml",
			body: `contracts:
  - id: conversation-route-set
    owner: nope
    kind: route-set
    symbol: internal/conversation/routes.go
    signature: ""
    stability: frozen
`,
			want: "unknown module",
		},
		{
			name: "contract empty symbol",
			file: "docs/architecture/passb/contracts.yaml",
			body: `contracts:
  - id: conversation-route-set
    owner: conversation
    kind: route-set
    symbol: ""
    signature: ""
    stability: frozen
`,
			want: "symbol",
		},
		{
			name: "contract empty stability",
			file: "docs/architecture/passb/contracts.yaml",
			body: `contracts:
  - id: conversation-route-set
    owner: conversation
    kind: route-set
    symbol: internal/conversation/routes.go
    signature: ""
    stability: ""
`,
			want: "stability",
		},
		{
			name: "contract duplicate id",
			file: "docs/architecture/passb/contracts.yaml",
			body: `contracts:
  - id: conversation-route-set
    owner: conversation
    kind: route-set
    symbol: internal/conversation/routes.go
    signature: ""
    stability: frozen
  - id: conversation-route-set
    owner: workbench
    kind: route-set
    symbol: internal/modules/workbench/routes.go
    signature: ""
    stability: frozen
`,
			want: "duplicate",
		},
		{
			name: "event zero version",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 0
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "version",
		},
		{
			name: "event non-integer version",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: one
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "cannot unmarshal",
		},
		{
			name: "event missing idempotency metadata",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [tenant_id]
`,
			want: "idempotency_key",
		},
		{
			name: "tenant event missing occurred_at metadata",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [tenant_id, event_id, idempotency_key]
`,
			want: "occurred_at",
		},
		{
			name: "event empty consumers",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: conversation
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
			name: "event empty producer",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: ""
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "producer",
		},
		{
			name: "event imperative command id",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.send
    version: 1
    producer: conversation
    meaning: 命令式命名（应拒绝）
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "imperative",
		},
		{
			name: "event id not dot separated",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: messageappended
    version: 1
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "dot-separated",
		},
		{
			name: "event duplicate producer version",
			file: "docs/architecture/passb/event-catalog.yaml",
			body: `events:
  - id: conversation.message.appended
    version: 1
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
  - id: conversation.turn.completed
    version: 1
    producer: conversation
    meaning: 事实
    ordering: per-session
    replay: source-query
    transport: in_process
    consumers: [internal/insights]
    required_metadata: [idempotency_key]
`,
			want: "producer/version",
		},
		{
			name: "exception empty from",
			file: "docs/architecture/passb/exception-ledger.yaml",
			body: `exceptions:
  - id: exc-0001
    from: ""
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
`,
			want: "from",
		},
		{
			name: "exception from not go file",
			file: "docs/architecture/passb/exception-ledger.yaml",
			body: `exceptions:
  - id: exc-0001
    from: internal/modules/appconnector/adapter.txt
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
`,
			want: ".go",
		},
		{
			name: "exception wildcard to",
			file: "docs/architecture/passb/exception-ledger.yaml",
			body: `exceptions:
  - id: exc-0001
    from: internal/modules/appconnector/adapter.go
    to: "github.com/Tencent/WeKnora/internal/modules/*"
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
`,
			want: "exact repository path",
		},
		{
			name: "exception bad remove_at barrier",
			file: "docs/architecture/passb/exception-ledger.yaml",
			body: `exceptions:
  - id: exc-0001
    from: internal/modules/appconnector/adapter.go
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib0
    reason: fixture reason
`,
			want: "remove_at",
		},
		{
			name: "exception duplicate id",
			file: "docs/architecture/passb/exception-ledger.yaml",
			body: `exceptions:
  - id: exc-0001
    from: internal/modules/appconnector/adapter.go
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
  - id: exc-0001
    from: internal/modules/appconnector/action.go
    to: github.com/Tencent/WeKnora/internal/modules/commercial
    plan: 27-appconnector
    remove_at: ib2
    reason: fixture reason
`,
			want: "duplicate",
		},
		{
			name: "legacy duplicate path",
			file: "docs/architecture/passb/ownership-matrix.yaml",
			body: `legacy_files:
  - path: internal/application/service/alpha_chat.go
    module: conversation
    plan: 35-conversation-program
    destination: internal/conversation/service
    integration_owner: ""
    delete_barrier: ib3
  - path: internal/application/service/alpha_chat.go
    module: workbench
    plan: 40-workbench
    destination: internal/modules/workbench/service
    integration_owner: ""
    delete_barrier: ib4
`,
			want: "duplicate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeGovernanceRepo(t, map[string]string{tc.file: tc.body})
			_, err := LoadGovernance(root)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
