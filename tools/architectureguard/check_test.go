package main

import (
	"strings"
	"testing"
)

func hasCheck(ds []Diagnostic, check, substr string) bool {
	for _, d := range ds {
		if d.Check == check && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestRunRejectsDuplicateRouteRegistration(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/fixture.go": `package router

func RegisterFixtureDup(g gtype) {
	g.GET("/dup", h1)
	g.GET("/dup", h2)
}
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "unique-route-registration", "/dup") {
		t.Fatalf("同一 path+method 注册两次必须报告 unique-route-registration:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsDuplicateWorkerRegistration(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/task.go": `package router

func registerWorkers(mux muxtype) {
	mux.HandleFunc(types.TypeDocumentProcess, h)
	mux.HandleFunc(types.TypeDocumentProcess, h2)
}
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "unique-worker-registration", "types.TypeDocumentProcess") {
		t.Fatalf("同一任务类型注册两次必须报告 unique-worker-registration:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsWorkerParityMismatch(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/task.go": `package router

func registerWorkers(mux muxtype) {
	mux.HandleFunc(types.TypeChunkExtract, h)
}
`,
		"internal/router/sync_task.go": `package router

func RegisterSyncHandlers(params ptype) {
	params.Executor.RegisterHandler(types.TypeMemoryExtract, h)
}
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "worker-parity", "types.TypeChunkExtract") ||
		!hasCheck(rep.Diagnostics, "worker-parity", "types.TypeMemoryExtract") {
		t.Fatalf("Redis/Lite 任务集合不一致必须报告 worker-parity:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsModuleToModuleImport(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/alpha/module.go": `package alpha

import _ "github.com/Tencent/WeKnora/internal/modules/beta"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "internal/modules/beta") {
		t.Fatalf("模块间横向 import 必须被拒绝:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionSuppressesExactPair(t *testing.T) {
	// §15：例外必须精确到 file→package 对。命中豁免的精确组合不得报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/appconnector/service/appconnector/oc_recovery.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "appconnector"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "oc_recovery.go") {
		t.Fatalf("命中精确豁免的 file→package 对不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionDoesNotCoverOtherFiles(t *testing.T) {
	// 同一 package、不同文件的 import 不在豁免范围内，必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/appconnector/service/appconnector/other_file.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "appconnector"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "other_file.go") {
		t.Fatalf("豁免只对精确 importer 文件生效，其他文件必须照常报告:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionDoesNotCoverOtherPackages(t *testing.T) {
	// 同一文件 import 其他模块的包不在豁免范围内，必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/appconnector/service/appconnector/oc_recovery.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/othermod/service/othermod"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "appconnector"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "othermod") {
		t.Fatalf("豁免只对精确 imported 路径生效，其他包必须照常报告:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversCommercialRootImports(t *testing.T) {
	// 同一批预存耦合的另一形态：import commercial 模块根（原 internal/commercial）。
	// 命中豁免的 adapter.go/action.go 根导入被放行；同文件指向其他模块根的
	// 导入不在豁免范围，仍须报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/appconnector/adapter.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/commercial"
`,
		"internal/appconnector/service/appconnector/action.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/commercial"
`,
		"internal/appconnector/other_root.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/othermod"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "appconnector"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "adapter.go") ||
		hasCheck(rep.Diagnostics, "forbidden-import", "action.go") {
		t.Fatalf("命中精确豁免的 commercial 根导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "other_root.go") {
		t.Fatalf("豁免只覆盖列出的精确 file→package 对，其他模块根导入必须照常报告:\n%s", joinChecks(rep.Diagnostics))
	}
}

// R2.5（上游布局回归收尾，2026-10-07）：原 batch A2/A3 时代登记的多条例外随
// importer 文件移出 internal/modules 判定面删除（见 check.go importExceptions
// R2.5 注记）。以下用例的语义意图不变——例外精确压制 + 未列文件/未列包照常
// 报告——夹具改锚定删后仍存活的在册条目。
func TestRunImportExceptionCoversKnowledgeRetrievalCommercialRoot(t *testing.T) {
	// K2.3 搬迁显形（exc-0112）：knowledge/retrieval/app 的 semantic 面消费
	// commercial 根包。命中豁免的精确 file→package 对放行；同包相邻文件
	// 不在豁免范围，必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/knowledge/retrieval/app/semantic_model_capability.go": `package app

import _ "github.com/Tencent/WeKnora/internal/commercial"
`,
		"internal/knowledge/retrieval/app/semantic_capability_neighbor.go": `package app

import _ "github.com/Tencent/WeKnora/internal/commercial"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "knowledge"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "semantic_model_capability.go") {
		t.Fatalf("K2.3 命中豁免的 semantic_model_capability.go→commercial 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "semantic_capability_neighbor.go") {
		t.Fatalf("未列入豁免的相邻文件必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversAppconnectorOcRecoveryMixedImports(t *testing.T) {
	// 同一文件的成对在册豁免（exc-0060/0061）：oc_recovery.go 对 commercial
	// 根包与 commercial/service/commercial 内部包的导入均放行；同文件指向
	// 未列包（execution 模块根不在清单）仍须报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/appconnector/service/appconnector/oc_recovery.go": `package appconnector

import (
	_ "github.com/Tencent/WeKnora/internal/commercial"
	_ "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	_ "github.com/Tencent/WeKnora/internal/execution"
)
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "appconnector"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "commercial") {
		t.Fatalf("命中豁免的 oc_recovery.go 两条 commercial 导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`oc_recovery.go 导入了模块 execution 的内部包 "github.com/Tencent/WeKnora/internal/execution"`) {
		t.Fatalf("未列入豁免的 oc_recovery.go→execution 必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversCodedeliveryAndWorkbench(t *testing.T) {
	// guard-only 存活边（无 ledger 行、两侧均未删）：codedelivery 消费
	// appconnector 根包/内部包（×2 在册）与 workbench 命令队列消费
	// agentruntime 模块根（在册）放行；同文件指向未列包
	// （codedelivery/service.go→agentruntime/agent 不在清单）仍须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/codedelivery/service.go": `package codedelivery

import (
	_ "github.com/Tencent/WeKnora/internal/appconnector"
	_ "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	_ "github.com/Tencent/WeKnora/internal/agentruntime/agent"
)
`,
		"internal/workbench/service/workbench/command_queue_next.go": `package workbench

import _ "github.com/Tencent/WeKnora/internal/agentruntime"
`,
	})

	mods := []ManifestView{{Module: "codedelivery"}, {Module: "workbench"}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "appconnector") {
		t.Fatalf("在册的 codedelivery→appconnector 两条导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "command_queue_next.go") {
		t.Fatalf("在册的 command_queue_next.go→agentruntime 模块根导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`service.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/agentruntime/agent"`) {
		t.Fatalf("未列入豁免的 service.go→agentruntime/agent 必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversWorkbenchRemoteUsagePairs(t *testing.T) {
	// 同一文件两条在册对（exc-0103/0104）：remote_usage.go 消费 commercial
	// 根包与 commercial/repository/commercial 内部包均放行；未列入清单的
	// 相邻文件必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/workbench/service/workbench/remote_usage.go": `package workbench

import (
	_ "github.com/Tencent/WeKnora/internal/commercial"
	_ "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
)
`,
		"internal/workbench/service/workbench/remote_usage_neighbor.go": `package workbench

import _ "github.com/Tencent/WeKnora/internal/commercial"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "workbench"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "remote_usage.go") {
		t.Fatalf("在册的 remote_usage.go 两对导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "remote_usage_neighbor.go") {
		t.Fatalf("未列入豁免的相邻文件必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversBatchA4Workbench(t *testing.T) {
	// batch A4 搬迁显形的预存横向耦合：命中豁免的精确 file→package 对放行
	// （workbench→commercial 根包、commercial/repository 内部包与 execution
	// 模块根三种形态；R2.5 后 agentruntime/agent/runtime 家族已随上游布局
	// 归位删除，不再列入）。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/workbench/service/workbench/admission.go": `package workbench

import (
	_ "github.com/Tencent/WeKnora/internal/commercial"
	_ "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	_ "github.com/Tencent/WeKnora/internal/execution"
)
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "workbench"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "admission.go") {
		t.Fatalf("batch A4 命中豁免的精确对不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionBatchA4DoesNotCoverUnlistedPairs(t *testing.T) {
	// batch A4 豁免只覆盖清单中的精确对：同一文件指向未列包的导入
	// （admission.go→commercial/service 不在清单）与未列入清单的相邻文件
	// 仍须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/workbench/service/workbench/admission.go": `package workbench

import (
	_ "github.com/Tencent/WeKnora/internal/commercial"
	_ "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
)
`,
		"internal/workbench/service/workbench/neighbor.go": `package workbench

import _ "github.com/Tencent/WeKnora/internal/agentruntime/agent/approval"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "workbench"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import",
		`admission.go 导入了模块 commercial 的内部包 `+
			`"github.com/Tencent/WeKnora/internal/commercial"`) {
		t.Fatalf("batch A4 命中豁免的精确对 admission.go 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`admission.go 导入了模块 commercial 的内部包 `+
			`"github.com/Tencent/WeKnora/internal/commercial/service/commercial"`) {
		t.Fatalf("未列入豁免的 admission.go 指向 commercial/service 的导入必须照常报告:\n%s",
			joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "neighbor.go") {
		t.Fatalf("未列入豁免的相邻文件必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunAllowsSelfModuleImport(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/alpha/module.go":  "package alpha\n",
		"internal/modules/alpha/sub/sub.go": "package sub\n",
		"internal/modules/alpha/user.go": `package alpha

import _ "github.com/Tencent/WeKnora/internal/modules/alpha/sub"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "") {
		t.Fatalf("模块内部子包 import 不应被拒绝:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsNewFileInHorizontalDir(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/application/service/listed.go":   "package service\n",
		"internal/application/service/newthing.go": "package service\n",
	})

	mods := []ManifestView{{
		Module:      "demo",
		LegacyPaths: []string{"internal/application/service/listed.go"},
	}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "legacy-guard", "internal/application/service/newthing.go") {
		t.Fatalf("横向目录新文件必须被拒绝:\n%s", joinChecks(rep.Diagnostics))
	}
	if hasCheck(rep.Diagnostics, "legacy-guard", "listed.go") {
		t.Fatalf("manifest 声明的遗留文件不应被拒绝:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunAllowsPlatformLegacyFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/handler/list_pagination.go":           "package handler\n",
		"internal/handler/upload_limit.go":              "package handler\n",
		"internal/application/repository/task_queue.go": "package repository\n",
		"internal/handler/dto/pagination.go":            "package dto\n",
	})

	rep, err := Run(root, []ManifestView{{Module: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "legacy-guard", "") {
		t.Fatalf("platform 本体文件与 platform 包不应被拒绝:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsUnownedRouteFile(t *testing.T) {
	// 反向覆盖：出现注册的路由文件必须被某 manifest 的 routes 条目或 platform 残留覆盖。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/routes_newthing.go": `package router

func RegisterNewthingRoutes(g gtype) {
	g.GET("/newthing", h)
}
`,
	})

	mods := []ManifestView{{
		Module:       "demo",
		RouteEntries: []string{"RegisterNewthingRoutes — internal/router/routes_newthing.go:2"},
	}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "route-file-coverage", "") {
		t.Fatalf("被 manifest 覆盖的路由文件不应报告 route-file-coverage:\n%s", joinChecks(rep.Diagnostics))
	}

	rep2, err := Run(root, []ManifestView{{Module: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep2.Diagnostics, "route-file-coverage", "internal/router/routes_newthing.go") {
		t.Fatalf("未被覆盖的路由文件必须报告 route-file-coverage:\n%s", joinChecks(rep2.Diagnostics))
	}
}

func TestRunRejectsUnknownRouteEntry(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/routes_known.go": `package router

func RegisterKnownRoutes(g gtype) {
	g.GET("/known", h)
}
`,
	})

	mods := []ManifestView{{
		Module:       "demo",
		RouteEntries: []string{"RegisterGhostRoutes — internal/router/routes_known.go:3"},
	}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "route-entry-coverage", "RegisterGhostRoutes") {
		t.Fatalf("manifest 声明的不存在路由入口必须报告 route-entry-coverage:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunRejectsManifestWorkerTypeMissingFromDiscovery(t *testing.T) {
	root := t.TempDir()
	mods := []ManifestView{{
		Module:      "demo",
		WorkerTypes: []string{"TypeGhost"},
	}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(rep.Diagnostics, "worker-coverage", "TypeGhost") {
		t.Fatalf("manifest 声明但代码未注册的 worker 必须报告 worker-coverage:\n%s", joinChecks(rep.Diagnostics))
	}
}

// TestGuardCleanAtHead 是 F2 的总闸：在未搬迁的当前 HEAD 上（真实 16 manifest），
// 守卫必须零诊断退出。任何后续 Pass A 搬迁引入的重复注册/越界 import/新横向文件都会打破它。
func TestGuardCleanAtHead(t *testing.T) {
	root := repoRoot(t)
	mods, err := LoadManifestViews(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 17 {
		t.Fatalf("应加载 17 份 manifest（含 career）, got %d", len(mods))
	}

	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Diagnostics) != 0 {
		t.Fatalf("HEAD 上守卫应零违规，得到 %d 条:\n%s", len(rep.Diagnostics), joinChecks(rep.Diagnostics))
	}
	if rep.Summary.RoutesTotal != wantRouteTotal || rep.Summary.WorkerRedis != wantWorkersPerMix ||
		rep.Summary.WorkerLite != wantWorkersPerMix || rep.Summary.Hooks != wantHooks {
		t.Errorf("发现规模偏离基线: %+v", rep.Summary)
	}
}

func joinChecks(ds []Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.String())
		b.WriteString("\n")
	}
	return b.String()
}
