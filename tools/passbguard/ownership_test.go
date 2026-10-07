package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/tools/internal/movemanifest"
)

// fixtureDiscovery 是最小真实形态的发现快照：1 个 knowledge manifest 事实、
// 1 条 alias 义务、1 条 architectureguard importException，供表驱动诊断用例改造。
func fixtureDiscovery() *Discovery {
	return &Discovery{
		Root: "/fixture-root",
		LegacyFiles: []LegacyFact{
			{Path: "internal/application/repository/widget.go", Module: "knowledge", PassBTask: "B-knowledge"},
		},
		Aliases: []movemanifest.AliasObligation{
			{OldImportPath: "internal/application/service/widgets", PassBTask: "B-knowledge"},
		},
		GoFiles: []string{
			"internal/application/repository/widget.go",
			"internal/modules/knowledge/widgets/alias.go",
			"internal/modules/agentruntime/agent/engine.go",
		},
		Imports: map[string][]string{
			"internal/modules/knowledge/widgets/alias.go": {},
			"internal/modules/agentruntime/agent/engine.go": {
				"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat",
			},
		},
		Exceptions: []DiscoveredException{
			{
				ImporterFile: "internal/modules/agentruntime/agent/engine.go",
				ImportedPath: "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat",
				Reason:       "预存横向包耦合",
				PassBTask:    "B-agentruntime",
			},
		},
	}
}

// fixtureGovernance 与 fixtureDiscovery 完全一致的合法治理集。
func fixtureGovernance() *Governance {
	return &Governance{
		Legacy: []LegacyOwnership{
			{
				Path:             "internal/application/repository/widget.go",
				Module:           "knowledge",
				Plan:             "24-knowledge-process",
				Destination:      "internal/modules/knowledge/process",
				IntegrationOwner: "ib2",
				DeleteBarrier:    "ib2",
			},
		},
		Aliases: []AliasOwnership{
			{OldImportPath: "internal/application/service/widgets", Plan: "20-knowledge-program", DeleteBarrier: "ib2"},
		},
		Exceptions: []Exception{
			{
				ID:       "exc-0001",
				From:     "internal/modules/agentruntime/agent/engine.go",
				To:       "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat",
				Plan:     "33-agentruntime-engine",
				RemoveAt: "ib3",
				Reason:   "预存横向包耦合",
			},
		},
	}
}

// findDiag 返回首个 check 命中的诊断；不存在返回 false。
func findDiag(diags []Diagnostic, check string) (Diagnostic, bool) {
	for _, d := range diags {
		if d.Check == check {
			return d, true
		}
	}
	return Diagnostic{}, false
}

func TestOwnershipHappyPathProducesNoDiagnostics(t *testing.T) {
	diags := CheckOwnership(fixtureGovernance(), fixtureDiscovery())
	require.Empty(t, diags, "diags: %v", diags)
}

func TestOwnershipDiagnostics(t *testing.T) {
	cases := []struct {
		name     string
		mutG     func(g *Governance)
		mutD     func(d *Discovery)
		check    string
		want     string
		wantPath string
	}{
		{
			// 1. 漏报一个 manifest legacy 文件（matrix 缺行）。
			name:     "missing legacy file",
			mutG:     func(g *Governance) { g.Legacy = nil },
			check:    "legacy-missing",
			want:     "no ownership-matrix owner",
			wantPath: "internal/application/repository/widget.go",
		},
		{
			// 2. 同一文件被两个计划重复认领。
			name: "duplicate owner overlap",
			mutG: func(g *Governance) {
				g.Legacy = append(g.Legacy, LegacyOwnership{
					Path:   "internal/application/repository/widget.go",
					Module: "workbench", Plan: "40-workbench",
					Destination:   "internal/modules/workbench/repository",
					DeleteBarrier: "ib4",
				})
			},
			check: "legacy-overlap",
			want:  "40-workbench",
		},
		{
			// 3. manifest 声明了路径但仓库树上不存在。
			name:  "nonexistent path",
			mutD:  func(d *Discovery) { d.GoFiles = nil },
			check: "legacy-nonexistent",
			want:  "does not exist",
		},
		{
			// 4a. plan 属主模块与行模块不符（knowledge 文件挂 identity 计划）。
			name:  "owner plan module mismatch",
			mutG:  func(g *Governance) { g.Legacy[0].Plan = "10-identity" },
			check: "legacy-plan-module",
			want:  "10-identity",
		},
		{
			// 4b. 目标包落在别的模块树下。
			name: "destination module mismatch",
			mutG: func(g *Governance) {
				g.Legacy[0].Destination = "internal/modules/conversation/service"
			},
			check: "legacy-destination",
			want:  "internal/modules/conversation/service",
		},
		{
			// 4c. 删除屏障早于属主计划阶段（B2 计划配 ib1）。
			name:  "delete barrier precedes plan phase",
			mutG:  func(g *Governance) { g.Legacy[0].DeleteBarrier = "ib1" },
			check: "legacy-barrier",
			want:  "ib2",
		},
		{
			// 5. 漏报一条 alias 义务。
			name:     "missing alias",
			mutG:     func(g *Governance) { g.Aliases = nil },
			check:    "alias-missing",
			want:     "no ownership-matrix owner",
			wantPath: "internal/application/service/widgets",
		},
		{
			// 6. ledger 里存在 guard 源未登记的例外（无属主例外）。
			name: "unowned exception",
			mutG: func(g *Governance) {
				g.Exceptions = append(g.Exceptions, Exception{
					ID:   "exc-0002",
					From: "internal/modules/agentruntime/agent/ghost.go",
					To:   "github.com/Tencent/WeKnora/internal/modules/commercial",
					Plan: "33-agentruntime-engine", RemoveAt: "ib3", Reason: "invented",
				})
			},
			check: "exception-unregistered",
			want:  "importExceptions",
		},
		{
			// 7. 例外删除期限早于其属主计划阶段（B3 计划配 ib2）。
			name:  "exception removed before its plan runs",
			mutG:  func(g *Governance) { g.Exceptions[0].RemoveAt = "ib2" },
			check: "exception-barrier",
			want:  "ib3",
		},
		{
			// 8. 例外被推给 b5 而模块计划本可更早删除。
			name:  "exception deferred to b5",
			mutG:  func(g *Governance) { g.Exceptions[0].RemoveAt = "b5" },
			check: "exception-b5-escape",
			want:  "b5",
		},
		{
			// 9. ledger 与 guard 源的 reason 漂移。
			name:  "exception reason drift",
			mutG:  func(g *Governance) { g.Exceptions[0].Reason = "改写过的原因" },
			check: "exception-reason-drift",
			want:  "预存横向包耦合",
		},
		{
			// 10. ledger 缺 guard 源在册的例外。
			name:  "missing exception entry",
			mutG:  func(g *Governance) { g.Exceptions = nil },
			check: "exception-missing",
			want:  "agent/engine.go",
		},
		{
			// 10b. 同一 (from,to) 边被两行不同 id 的例外重复认领
			// （OCR R1 #2：validate 只对相邻同 id 判重，边级判重必须由
			// CheckOwnership 兜底——与 legacy-overlap 双防护同构）。
			name: "duplicate exception edge",
			mutG: func(g *Governance) {
				dup := g.Exceptions[0]
				dup.ID = "exc-0099"
				g.Exceptions = append(g.Exceptions, dup)
			},
			check:    "exception-overlap",
			want:     "exc-0099",
			wantPath: "internal/modules/agentruntime/agent/engine.go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := fixtureGovernance()
			d := fixtureDiscovery()
			if tc.mutG != nil {
				tc.mutG(g)
			}
			if tc.mutD != nil {
				tc.mutD(d)
			}
			diags := CheckOwnership(g, d)
			diag, ok := findDiag(diags, tc.check)
			require.True(t, ok, "want check %q, got diags: %v", tc.check, diags)
			require.Contains(t, diag.Message, tc.want)
			if tc.wantPath != "" {
				require.Contains(t, diag.Path, tc.wantPath)
			}
		})
	}
}

func TestOwnershipDiagnosticsSortedAndDeduplicated(t *testing.T) {
	g := fixtureGovernance()
	g.Legacy = nil // missing + 其他诊断并存
	diags := CheckOwnership(g, fixtureDiscovery())
	require.NotEmpty(t, diags)
	for i := 1; i < len(diags); i++ {
		require.LessOrEqual(t, diags[i-1].String(), diags[i].String())
	}
	for i := 1; i < len(diags); i++ {
		require.NotEqual(t, diags[i-1].String(), diags[i].String())
	}
}

// repoRootFromTest 向上查找仓库根（含 go.mod 与 docs/architecture/moves）。
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "docs", "architecture", "moves")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到仓库根（go.mod + docs/architecture/moves）")
		}
		dir = parent
	}
}

// acceptanceLedgerCount 从 Pass A 验收台账解析基线计数（F5：期望值从台账参数化读取）。
func acceptanceLedgerCount(t *testing.T, root string) (legacy int, exceptions int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "evidence", "pass-a-acceptance.md"))
	require.NoError(t, err)
	text := string(data)
	m := regexp.MustCompile(`legacy_files (\d+) 条全部带 passb_task`).FindStringSubmatch(text)
	require.NotNil(t, m, "台账未记录 legacy_files 计数")
	legacy = atoi(t, m[1])
	m = regexp.MustCompile(`import 例外共 \*\*(\d+) 条\*\*在册`).FindStringSubmatch(text)
	require.NotNil(t, m, "台账未记录 import 例外计数")
	exceptions = atoi(t, m[1])
	return legacy, exceptions
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, r := range s {
		require.True(t, r >= '0' && r <= '9')
		n = n*10 + int(r-'0')
	}
	return n
}

// TestRealRepoOwnershipMatrixFreezesAllLegacy 覆盖 Review Focus：
// 遗漏/重复/悬空属主在真实仓库上必须零诊断，且矩阵计数与 manifest 发现值、
// Pass A 验收台账三方一致（conventions §8 / F5）。
func TestRealRepoOwnershipMatrixFreezesAllLegacy(t *testing.T) {
	root := repoRootFromTest(t)
	d, err := DiscoverPassB(root)
	require.NoError(t, err)
	g, err := LoadGovernance(root)
	require.NoError(t, err)

	diags := CheckOwnership(g, d)
	require.Empty(t, diags, "diags: %v", diags)

	// 三方一致：矩阵 == manifest 发现 == 台账。
	wantLegacy, wantExceptions := acceptanceLedgerCount(t, root)
	require.Equal(t, wantLegacy, len(d.LegacyFiles), "manifest 发现的 legacy 计数与台账不符")
	require.Equal(t, wantLegacy, len(g.Legacy), "ownership-matrix 行数与台账不符")
	require.Equal(t, wantExceptions, len(d.Exceptions), "guard 解析的例外计数与台账不符")
	require.Equal(t, wantExceptions, len(g.Exceptions), "exception-ledger 行数与台账不符")
	require.Equal(t, len(d.Aliases), len(g.Aliases), "alias 属主行数与 manifest alias_obligations 不符")

	// 框架要求的分模块冻结总量（freeze:150-155）。knowledge 84→79→77→76→53：
	// K1.1–K1.5 迁移删 9 行（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP，K1.6 窗口
	// 补录）+ 过渡 shim 成对补 4 行（Ruling
	// 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）；K4.2 迁移删 3 行
	// （write/index_content/task_options）+ 宿主 compat 成对补 1 行（同 Ruling）；
	// K4.3 迁移删 2 行（kb_access/task_progress_auth）+ 宿主 compat 成对补
	// 1 行（同 Ruling）；IB2 回写批（2026-09-27）——K2/K3 已迁残留 33 行删除
	// + K2/K3 宿主 compat 过渡 shim 10 行成对补行（K4.3 条目登记的 23 行 BASE
	// 漂移修复收口），76-33+10=53；纯计数修正，判定逻辑不变。
	// datasource 4→3：b2-datasource B2-DS.2 repository 迁移删行（matrix 已删、
	// 该分支测试期望未同步，IB2 集成侧修正）。appconnector 7→1：
	// Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP（B2-AC.2，2026-09-23）——6 行已迁
	// handler 删除后仅余 shim 行 app_connector.go。IB2 集成合并树实测 total=358
	//（matrix == manifests 三方一致）。
	// R2.5 上游布局回归收尾（2026-10-07）：删 11 条文件已删的 shim/compat 行
	//（agentcatalog −1 / agentruntime −2 / datasource −3 归零 / knowledge −5），
	// 并对齐分支实测预存漂移（agentruntime 45→40、insights 6→3，本表旧值自
	// IB2 后未随行删除同步）；matrix 实测 352→341。
	wantPerModule := map[string]int{
		"identity": 27, "airesource": 33, "commercial": 8, "execution": 21,
		"knowledge": 48, "agentcatalog": 54, "appconnector": 1,
		"agentruntime": 40, "conversation": 44, "channels": 7, "insights": 3,
		"workbench": 19, "craft": 30, "system": 5, "policy": 1,
	}
	gotPerModule := map[string]int{}
	for _, row := range g.Legacy {
		gotPerModule[row.Module]++
	}
	require.Equal(t, wantPerModule, gotPerModule)
}

// TestRealRepoKnowledgeRepositoryFileHasExplicitKOwner 落实 b0 节点裁定：
// internal/application/repository/knowledge.go 必须显式归属某个 K 计划（不得悬空）。
func TestRealRepoKnowledgeRepositoryFileHasExplicitKOwner(t *testing.T) {
	root := repoRootFromTest(t)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	for _, row := range g.Legacy {
		if row.Path == "internal/application/repository/knowledge.go" {
			kPlans := []PlanID{
				"21-knowledge-ingest", "22-knowledge-retrieval",
				"23-knowledge-wikifaq", "24-knowledge-process",
			}
			require.Contains(t, kPlans, row.Plan,
				"knowledge.go 必须显式归属 K1-K4 之一")
			return
		}
	}
	t.Fatal("ownership-matrix 缺少 internal/application/repository/knowledge.go 行")
}

// TestRealRepoBriefDisambiguatesKnowledgeRepositoryFile 校验 brief 消歧：
// 4 份 K brief 中必须能以全路径检索到 repository/knowledge.go 的显式归属说明。
func TestRealRepoBriefDisambiguatesKnowledgeRepositoryFile(t *testing.T) {
	root := repoRootFromTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "passb", "knowledge-process.md"))
	require.NoError(t, err)
	require.True(t, strings.Contains(string(data), "internal/application/repository/knowledge.go"),
		"knowledge-process.md 必须以全路径显式登记 repository/knowledge.go（b0 notes 审校补充发现）")
}

// TestRealRepoExceptionLedgerPlansMatchGuardTasks 校验例外属主计划与 guard 源
// PassBTask 的模块绑定（F1：字段级机器校验）。
func TestRealRepoExceptionLedgerPlansMatchGuardTasks(t *testing.T) {
	root := repoRootFromTest(t)
	d, err := DiscoverPassB(root)
	require.NoError(t, err)
	g, err := LoadGovernance(root)
	require.NoError(t, err)
	planOf := map[string]PlanID{}
	for _, x := range g.Exceptions {
		planOf[x.From+"→"+x.To] = x.Plan
	}
	for _, de := range d.Exceptions {
		p := planOf[de.ImporterFile+"→"+de.ImportedPath]
		info, ok := KnownPlans[p]
		require.True(t, ok, "例外 %s→%s 属主计划 %q 不在框架计划表", de.ImporterFile, de.ImportedPath, p)
		mod := PassBTaskModule[de.PassBTask]
		require.NotEmpty(t, mod, "guard 源 PassBTask %q 缺模块映射", de.PassBTask)
		require.Contains(t, info.Modules, mod,
			"例外 %s→%s 的属主计划 %q 与 guard PassBTask %q 模块不符", de.ImporterFile, de.ImportedPath, p, de.PassBTask)
	}
}
