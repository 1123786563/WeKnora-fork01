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
		"internal/modules/appconnector/service/appconnector/oc_recovery.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
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
		"internal/modules/appconnector/service/appconnector/other_file.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
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
		"internal/modules/appconnector/service/appconnector/oc_recovery.go": `package appconnector

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
		"internal/modules/appconnector/adapter.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/commercial"
`,
		"internal/modules/appconnector/service/appconnector/action.go": `package appconnector

import _ "github.com/Tencent/WeKnora/internal/modules/commercial"
`,
		"internal/modules/appconnector/other_root.go": `package appconnector

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

func TestRunImportExceptionCoversBatchA2AiresourceChat(t *testing.T) {
	// batch A2：models/chat 搬入 airesource 后，usage.go 对 commercial 根包的
	// 预存横向耦合显形。命中豁免的精确 file→package 对放行；同包相邻文件
	// 不在豁免范围，必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/airesource/models/chat/usage.go": `package chat

import _ "github.com/Tencent/WeKnora/internal/modules/commercial"
`,
		"internal/modules/airesource/models/chat/usage_neighbor.go": `package chat

import _ "github.com/Tencent/WeKnora/internal/modules/commercial"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "airesource"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "chat/usage.go") {
		t.Fatalf("batch A2 命中豁免的 usage.go→commercial 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import", "usage_neighbor.go") {
		t.Fatalf("未列入豁免的相邻文件必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversBatchA2ChannelsAndExecution(t *testing.T) {
	// batch A2：channels/im 与 execution/sandbox 对 airesource/policy 的预存
	// 横向耦合显形。命中豁免的精确对放行；同一文件指向未列包的导入
	// （service.go→policy/ipclass 不在清单）仍须报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/channels/im/service.go": `package im

import (
	_ "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	_ "github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"
	_ "github.com/Tencent/WeKnora/internal/modules/policy/ratelimit"
	_ "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"
)
`,
		"internal/modules/channels/im/yunzhijia/url.go": `package yunzhijia

import _ "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"
`,
		"internal/modules/execution/sandbox/url_guard.go": `package sandbox

import _ "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"
`,
	})

	mods := []ManifestView{{Module: "channels"}, {Module: "execution"}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "airesource/mcp") ||
		hasCheck(rep.Diagnostics, "forbidden-import", "airesource/storageurl") ||
		hasCheck(rep.Diagnostics, "forbidden-import", "policy/ratelimit") {
		t.Fatalf("batch A2 命中豁免的 service.go 导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "yunzhijia/url.go") ||
		hasCheck(rep.Diagnostics, "forbidden-import", "url_guard.go") {
		t.Fatalf("batch A2 命中豁免的 url.go/url_guard.go→ipclass 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`service.go 导入了模块 policy 的内部包 "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"`) {
		t.Fatalf("未列入豁免的 service.go→policy/ipclass 必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversBatchA3KnowledgeAndAgentruntime(t *testing.T) {
	// batch A3：knowledge/agentruntime 搬入模块目录后，预存横向耦合显形。
	// 命中豁免的精确 file→package 对放行；同文件指向未列包的导入
	// （composite.go→policy/access 不在清单）仍须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/knowledge/retriever/composite.go": `package retriever

import (
	_ "github.com/Tencent/WeKnora/internal/modules/airesource/models/embedding"
	_ "github.com/Tencent/WeKnora/internal/modules/policy/access"
)
`,
		"internal/modules/agentruntime/agent/engine.go": `package agent

import _ "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
`,
	})

	mods := []ManifestView{{Module: "knowledge"}, {Module: "agentruntime"}}
	rep, err := Run(root, mods)
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import",
		`composite.go 导入了模块 airesource 的内部包`+
			` "github.com/Tencent/WeKnora/internal/modules/airesource/models/embedding"`) {
		t.Fatalf("batch A3 命中豁免的 composite.go→embedding 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if hasCheck(rep.Diagnostics, "forbidden-import",
		`engine.go 导入了模块 airesource 的内部包 "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"`) {
		t.Fatalf("batch A3 命中豁免的 engine.go→chat 不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`composite.go 导入了模块 policy 的内部包 "github.com/Tencent/WeKnora/internal/modules/policy/access"`) {
		t.Fatalf("未列入豁免的 composite.go→policy/access 必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
}

func TestRunImportExceptionCoversBatchA3ConversationChatPipeline(t *testing.T) {
	// batch A3：conversation 搬入模块目录后，chat_pipeline 对 airesource/knowledge
	// 的预存横向耦合显形。命中豁免的精确对放行；未列入清单的相邻文件
	// 必须照常报告。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/modules/conversation/chat_pipeline/common.go": `package chat_pipeline

import (
	_ "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	_ "github.com/Tencent/WeKnora/internal/modules/knowledge/searchutil"
)
`,
		"internal/modules/conversation/chat_pipeline/neighbor.go": `package chat_pipeline

import _ "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
`,
	})

	rep, err := Run(root, []ManifestView{{Module: "conversation"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasCheck(rep.Diagnostics, "forbidden-import", "common.go") {
		t.Fatalf("batch A3 命中豁免的 common.go 两对导入不应报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
	}
	if !hasCheck(rep.Diagnostics, "forbidden-import",
		`neighbor.go 导入了模块 airesource 的内部包 "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"`) {
		t.Fatalf("未列入豁免的 neighbor.go 必须照常报告 forbidden-import:\n%s", joinChecks(rep.Diagnostics))
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
	if len(mods) != 16 {
		t.Fatalf("应加载 16 份 manifest, got %d", len(mods))
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
