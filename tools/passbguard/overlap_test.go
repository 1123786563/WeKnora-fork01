package passbguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writePassbFile 在临时仓库根下写一份 docs/architecture/passb/<name> 文件。
func writePassbFile(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(GovernanceDir))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestAmbiguousScanDetectsForbiddenPhrases(t *testing.T) {
	cases := []struct {
		name   string
		phrase string
	}{
		{"english first merged wins", "first merged wins"},
		{"chinese first merger", "以先合并者为准"},
		{"chinese either-one executes", "删除动作二选一执行"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writePassbFile(t, root, "sample-brief.md",
				"# Brief\n\n两 brief 不得重复认领（"+tc.phrase+"）。\n")
			diags, err := CheckAmbiguity(root)
			require.NoError(t, err)
			require.Len(t, diags, 1)
			require.Equal(t, "ambiguous-phrase", diags[0].Check)
			require.Equal(t, filepath.ToSlash(filepath.Join(GovernanceDir, "sample-brief.md")), diags[0].Path)
			require.Contains(t, diags[0].Message, "line 3")
		})
	}
}

func TestAmbiguousScanCleanFilesPass(t *testing.T) {
	root := t.TempDir()
	writePassbFile(t, root, "ownership-matrix.yaml", "# 注释：无歧义表述\nlegacy_files: []\n")
	writePassbFile(t, root, "clean-brief.md", "# Brief\n\n唯一属主：34-agentruntime-protocol。\n")
	diags, err := CheckAmbiguity(root)
	require.NoError(t, err)
	require.Empty(t, diags, "diags: %v", diags)
}

// TestOverlapDuplicateMatrixClaimRejected 锚定 Step 1 的
// 「两个 plan 认领同一仓库路径」拒绝语义（matrix 侧经 CheckOwnership）。
func TestOverlapDuplicateMatrixClaimRejected(t *testing.T) {
	g := fixtureGovernance()
	g.Legacy = append(g.Legacy, LegacyOwnership{
		Path:   "internal/application/repository/widget.go",
		Module: "workbench", Plan: "40-workbench",
		Destination:   "internal/modules/workbench/repository",
		DeleteBarrier: "ib4",
	})
	diags := CheckOwnership(g, fixtureDiscovery())
	diag, ok := findDiag(diags, "legacy-overlap")
	require.True(t, ok, "want legacy-overlap, got: %v", diags)
	require.Contains(t, diag.Message, "claimed by two plans")
}

func TestRulingOwnersDetectMissingWrongAndPlatformClaims(t *testing.T) {
	rulings := map[string]PlanID{
		"internal/application/service/native_archive.go":  "34-agentruntime-protocol",
		"internal/application/service/native_recovery.go": "33-agentruntime-engine",
	}
	platform := []string{"internal/handler/list_pagination.go"}
	good := &Governance{Legacy: []LegacyOwnership{
		{
			Path:   "internal/application/service/native_archive.go",
			Module: "agentruntime", Plan: "34-agentruntime-protocol",
			Destination: "internal/modules/agentruntime/service", DeleteBarrier: "ib3",
		},
		{
			Path:   "internal/application/service/native_recovery.go",
			Module: "agentruntime", Plan: "33-agentruntime-engine",
			Destination: "internal/modules/agentruntime/service", DeleteBarrier: "ib3",
		},
	}}
	require.Empty(t, checkRulings(good, rulings, platform))

	t.Run("ruled path missing from matrix", func(t *testing.T) {
		g := good
		g.Legacy = g.Legacy[:1]
		diags := checkRulings(g, rulings, platform)
		diag, ok := findDiag(diags, "ruling-missing")
		require.True(t, ok, "want ruling-missing, got: %v", diags)
		require.Contains(t, diag.Path, "native_recovery.go")
	})

	t.Run("ruled path owned by wrong plan", func(t *testing.T) {
		bad := good
		bad.Legacy[0].Plan = "33-agentruntime-engine"
		diags := checkRulings(bad, rulings, platform)
		diag, ok := findDiag(diags, "ruling-owner")
		require.True(t, ok, "want ruling-owner, got: %v", diags)
		require.Contains(t, diag.Message, "34-agentruntime-protocol")
	})

	t.Run("platform preserved helper claimed by a plan", func(t *testing.T) {
		bad := good
		bad.Legacy = append(bad.Legacy, LegacyOwnership{
			Path:   "internal/handler/list_pagination.go",
			Module: "conversation", Plan: "35-conversation-program",
			Destination: "internal/modules/conversation/handler", DeleteBarrier: "ib3",
		})
		diags := checkRulings(bad, rulings, platform)
		diag, ok := findDiag(diags, "ruling-platform-claimed")
		require.True(t, ok, "want ruling-platform-claimed, got: %v", diags)
		require.Contains(t, diag.Path, "list_pagination.go")
	})
}

func TestBriefClaimChecks(t *testing.T) {
	rulings := map[string]PlanID{
		"internal/application/service/native_archive.go":  "34-agentruntime-protocol",
		"internal/application/service/native_recovery.go": "33-agentruntime-engine",
		"internal/application/service/agent_run_graph.go": "33-agentruntime-engine",
	}
	t.Run("owner brief claims pass and line references are not claims", func(t *testing.T) {
		root := t.TempDir()
		writePassbFile(t, root, "agentruntime-protocol.md",
			"# p\n\n迁入：`internal/application/service/native_archive.go`（独占）。\n"+
				"消费方：`agent_run_graph.go:22`（trpcagent import）。\n")
		writePassbFile(t, root, "agentruntime-engine.md",
			"# e\n\n迁入：native_recovery.go、agent_run_graph.go。\n")
		diags, err := checkBriefClaims(root, rulings)
		require.NoError(t, err)
		require.Empty(t, diags, "diags: %v", diags)
	})
	t.Run("owner brief missing claim", func(t *testing.T) {
		root := t.TempDir()
		writePassbFile(t, root, "agentruntime-engine.md",
			"# e\n\n迁入：agent_run_graph.go。\n")
		writePassbFile(t, root, "agentruntime-protocol.md",
			"# p\n\n（未声明任何归属文件）\n")
		diags, err := checkBriefClaims(root, rulings)
		require.NoError(t, err)
		diag, ok := findDiag(diags, "brief-claim-missing")
		require.True(t, ok, "want brief-claim-missing, got: %v", diags)
		require.Contains(t, diag.Path, "native_archive.go")
		require.Contains(t, diag.Message, "34-agentruntime-protocol")
	})
	t.Run("other plan brief claiming ruled file", func(t *testing.T) {
		root := t.TempDir()
		writePassbFile(t, root, "agentruntime-protocol.md",
			"# p\n\n迁入：`internal/application/service/native_archive.go`、`native_recovery.go`。\n")
		writePassbFile(t, root, "agentruntime-engine.md",
			"# e\n\n迁入：native_recovery.go、agent_run_graph.go、native_archive.go。\n")
		diags, err := checkBriefClaims(root, rulings)
		require.NoError(t, err)
		conflicts := map[string]bool{}
		for _, d := range diags {
			if d.Check == "brief-claim-conflict" {
				conflicts[d.Path] = true
			}
		}
		require.True(t, conflicts["agentruntime-protocol.md"], "want protocol conflict on native_recovery.go, got: %v", diags)
		require.True(t, conflicts["agentruntime-engine.md"], "want engine conflict on native_archive.go, got: %v", diags)
	})
	t.Run("substring basename is not a claim", func(t *testing.T) {
		root := t.TempDir()
		writePassbFile(t, root, "agentruntime-protocol.md",
			"# p\n\n迁入：`internal/application/service/native_archive.go`。\n")
		writePassbFile(t, root, "agentruntime-engine.md",
			"# e\n\n迁入：native_recovery.go、agent_run_graph.go；见 `agent_stream_handler.go` 与 `_handler.go` 族。\n")
		diags, err := checkBriefClaims(root, rulings)
		require.NoError(t, err)
		require.Empty(t, diags, "diags: %v", diags)
	})
}

// matrixOwners 返回 matrix 中 path -> plan（真实仓库）。
func matrixOwners(t *testing.T, g *Governance) map[string]PlanID {
	t.Helper()
	owners := make(map[string]PlanID, len(g.Legacy))
	for _, row := range g.Legacy {
		owners[row.Path] = row.Plan
	}
	return owners
}

// TestRealRepoBriefsContainNoAmbiguousPhrase 落实 B0.3 Interfaces：
// 治理/brief 文本零歧义标记（freeze:187）。
func TestRealRepoBriefsContainNoAmbiguousPhrase(t *testing.T) {
	root := repoRootFromTest(t)
	diags, err := CheckAmbiguity(root)
	require.NoError(t, err)
	require.Empty(t, diags, "diags: %v", diags)
}

// TestRealRepoAgentRuntimeNativeOwnersFollowRuling 落实 B0.3 Step 2 精确属主
// （freeze:189-196）：archive 双文件归 34-protocol；recovery 与 native repository
// 状态/lease/pending/usage 族归 33-engine。
func TestRealRepoAgentRuntimeNativeOwnersFollowRuling(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	owners := matrixOwners(t, g)
	for path, want := range AgentRuntimeNativeRuling {
		require.Equal(t, want, owners[path], "Agent Runtime 裁定文件 %s 属主漂移", path)
	}
	// 纯协议包（native/nativecontract/nativeprobe/trpc/opencode/recoverytest）
	// 的边界收敛归协议计划：协议 brief 必须逐一列名。
	protocolBrief, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GovernanceDir), "agentruntime-protocol.md"))
	require.NoError(t, err)
	for _, pkg := range ProtocolFamilyPackages {
		require.Contains(t, string(protocolBrief), pkg,
			"agentruntime-protocol.md 必须列名协议包 %s（B0.3 Step 2 裁定）", pkg)
	}
}

// TestRealRepoHandlerSessionOwnersFollowRuling 落实 B0.3 Step 3 共享宿主裁定：
// internal/handler/session 全部 36 个 legacy 文件逐一路径精确属主。
func TestRealRepoHandlerSessionOwnersFollowRuling(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	owners := matrixOwners(t, g)
	for path, want := range HandlerSessionRuling {
		require.Equal(t, want, owners[path], "handler/session 共享宿主裁定属主漂移：%s", path)
	}
	// 裁定表必须覆盖 handler/session 的全部 matrix 行（无遗漏）。
	for path := range owners {
		if isUnder(path, "internal/handler/session/") {
			_, ruled := HandlerSessionRuling[path]
			require.True(t, ruled, "handler/session 文件 %s 未纳入 B0.3 裁定表", path)
		}
	}
}

// TestRealRepoBriefClaimsFollowRuling 落实 B0.3 Step 1/2：
// brief 声明与裁定属主一致——属主 brief 必须认领，其他计划的 brief 不得认领。
func TestRealRepoBriefClaimsFollowRuling(t *testing.T) {
	root := repoRootFromTest(t)
	diags, err := CheckBriefClaims(root)
	require.NoError(t, err)
	require.Empty(t, diags, "diags: %v", diags)
}

// TestRealRepoHousekeepingRulingRecorded 落实 B0.3 Step 4：
// knowledge_housekeeping.go 业务清扫规则留 Knowledge（matrix 24-knowledge-process），
// System 经窄 port 拥有调度/生命周期调用，且不得出现第二套清扫实现。
func TestRealRepoHousekeepingRulingRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	owners := matrixOwners(t, g)
	require.Equal(t, PlanID("24-knowledge-process"), owners["internal/application/service/knowledge_housekeeping.go"],
		"knowledge_housekeeping.go 必须归 24-knowledge-process（B0.3 Step 4 裁定）")

	brief, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GovernanceDir), "knowledge-process.md"))
	require.NoError(t, err)
	text := string(brief)
	require.Contains(t, text, "KnowledgeHousekeeping",
		"knowledge-process.md 必须登记 System 侧窄 port 名称 KnowledgeHousekeeping（B0.3 Step 4）")
	require.Contains(t, text, "42-system-policy",
		"knowledge-process.md 必须显式标注调度/生命周期调用属主计划 42-system-policy（B0.3 Step 4）")
}

// TestRealRepoPlatformHelpersRemainUnowned 落实 B0.3 Step 3 末条：
// 通用分页/上传限制 helper 保持 platform（不进任何子计划矩阵行）。
func TestRealRepoPlatformHelpersRemainUnowned(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	owners := matrixOwners(t, g)
	for _, p := range PlatformPreservedFiles {
		_, claimed := owners[p]
		require.False(t, claimed, "platform helper %s 不得进入 ownership-matrix（B0.3 Step 3 裁定）", p)
	}
	brief, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GovernanceDir), "conversation-session.md"))
	require.NoError(t, err)
	for _, basename := range []string{"list_pagination.go", "upload_limit.go"} {
		require.Contains(t, string(brief), basename,
			"conversation-session.md 必须声明 %s 保持 platform（B0.3 Step 3 裁定）", basename)
	}
}

func isUnder(path, prefix string) bool {
	return len(path) > len(prefix) && path[:len(prefix)] == prefix
}
