package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeTree 在临时目录中落一批文件（目录自动创建）。
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
}

func TestDiscoverRoutesFindsLiteralAPIKeyAndHandle(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/fixture.go": `package router

import "net/http"

func RegisterFixtureA(g gtype, grp grptype) {
	g.GET("/alpha", h)
	g.POST("/alpha", h)
	g.apiKeyRoute(grp, http.MethodGet, "/beta", policy, h)
	g.Handle(http.MethodPut, "/gamma", h)
}

func RegisterFixtureB(g gtype) {
	g.Any("/catchall", h)
	g.DELETE("/delta", h)
}
`,
	})

	routes, err := DiscoverRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ kind, method, path string }
	got := map[key]bool{}
	for _, r := range routes {
		got[key{r.Kind, r.Method, r.Path}] = true
	}
	want := []key{
		{"literal", "GET", "/alpha"},
		{"literal", "POST", "/alpha"},
		{"apiKeyRoute", "GET", "/beta"},
		{"handle", "PUT", "/gamma"},
		{"literal", "ANY", "/catchall"},
		{"literal", "DELETE", "/delta"},
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("缺少发现项 %+v，实际发现 %d 条: %+v", w, len(routes), routes)
		}
	}
}

func TestDiscoverRoutesSkipsTestFilesAndComments(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/fixture.go": `package router

// func RegisterCommented(g gtype) { g.GET("/commented", h) }

func RegisterReal(g gtype) {
	g.GET("/real", h) // g.GET("/trailing", h)
}
`,
		"internal/router/fixture_test.go": `package router

func TestNothing(t *testing.T) {
	g.GET("/in-test", h)
}
`,
	})

	routes, err := DiscoverRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Path != "/real" {
		t.Fatalf("注释与 _test.go 中的调用不得计入，得到: %+v", routes)
	}
}

func TestDiscoverWorkersSeparatesRedisAndLite(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/task.go": `package router

func registerFixtureWorkers(mux muxtype) {
	mux.HandleFunc(types.TypeChunkExtract, h)
	mux.HandleFunc(types.TypeKBDelete, h)
}
`,
		"internal/router/sync_task.go": `package router

func RegisterSyncHandlers(params ptype) {
	params.Executor.RegisterHandler(types.TypeChunkExtract, h)
	params.Executor.RegisterHandler(types.TypeMemoryExtract, h)
}
`,
	})

	redis, lite, err := DiscoverWorkers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(redis) != 2 || len(lite) != 2 {
		t.Fatalf("redis=%d lite=%d, want 2/2: %+v / %+v", len(redis), len(lite), redis, lite)
	}
	if redis[0].TaskType != "types.TypeChunkExtract" || redis[1].TaskType != "types.TypeKBDelete" {
		t.Fatalf("redis 发现错误: %+v", redis)
	}
	if lite[1].TaskType != "types.TypeMemoryExtract" {
		t.Fatalf("lite 发现错误: %+v", lite)
	}
	if redis[0].Mode != "redis" || lite[0].Mode != "lite" {
		t.Fatalf("Mode 标注错误: %+v %+v", redis[0], lite[0])
	}
}

func TestDiscoverHooksFindsContainerInvoke(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/container/container.go": `package container

func boot() {
	must(container.Invoke(registerPoolCleanup))
	must(container.Invoke(chatpipeline.NewPluginSearch))
}
`,
	})

	hooks, err := DiscoverHooks(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, h := range hooks {
		names = append(names, h.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "chatpipeline.NewPluginSearch" || names[1] != "registerPoolCleanup" {
		t.Fatalf("hook 发现错误: %+v", names)
	}
}

func TestFuncDeclNamesRecursive(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/router/a.go":          "package router\nfunc RegisterA() {}\n",
		"internal/handler/session/b.go": "package session\nfunc RegisterB() {}\n",
		"internal/handler/b_test.go":    "package handler\nfunc TestB() {}\n",
	})

	names, err := FuncDeclNames(root, "internal/router", "internal/handler")
	if err != nil {
		t.Fatal(err)
	}
	if !names["RegisterA"] || !names["RegisterB"] {
		t.Fatalf("应递归发现顶层函数且跳过测试文件: %v", names)
	}
	if names["TestB"] {
		t.Fatalf("_test.go 中的函数不得计入: %v", names)
	}
}

// ---- 真实仓库基线对照。F0 §3.2 记录 632 = 563 literal + 69 apiKeyRoute + 0 handle；
// 实测（代码为事实源）：564 literal + 69 + 0 = 633 —— 文档的 line-sweep 在
// routes_knowledge.go 少数 1 个 GET（该文件在 f4acb2154 基线修正时即为 90 处调用，
// 文档表记 89；全仓仅 GET 222 vs 文档 221 一处之差，其余方法全部吻合）。
// worker 23+23 两侧一致；hooks 58。 ----

const (
	wantRouteLiteral  = 564
	wantRouteAPIKey   = 69
	wantRouteHandle   = 0
	wantRouteTotal    = 633
	wantWorkersPerMix = 23
	wantHooks         = 58
)

func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(abs, "docs", "architecture", "moves")); err != nil || !fi.IsDir() {
		t.Fatalf("仓库根定位失败: %s (%v)", abs, err)
	}
	return abs
}

func TestDiscoverRealRepoRouteTotals(t *testing.T) {
	root := repoRoot(t)
	routes, err := DiscoverRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	var lit, api, handle int
	for _, r := range routes {
		switch r.Kind {
		case "literal":
			lit++
		case "apiKeyRoute":
			api++
		case "handle":
			handle++
		}
	}
	if lit != wantRouteLiteral || api != wantRouteAPIKey || handle != wantRouteHandle {
		t.Errorf("路由计数偏离基线（代码实测）: literal=%d(%d) apiKeyRoute=%d(%d) handle=%d(%d)",
			lit, wantRouteLiteral, api, wantRouteAPIKey, handle, wantRouteHandle)
	}
	if lit+api+handle != wantRouteTotal {
		t.Errorf("路由总数 = %d, want %d", lit+api+handle, wantRouteTotal)
	}
}

func TestDiscoverRealRepoWorkersParity(t *testing.T) {
	root := repoRoot(t)
	redis, lite, err := DiscoverWorkers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(redis) != wantWorkersPerMix || len(lite) != wantWorkersPerMix {
		t.Errorf("worker 注册数偏离 F0 基线: redis=%d lite=%d, want %d/%d",
			len(redis), len(lite), wantWorkersPerMix, wantWorkersPerMix)
	}
	set := func(rs []WorkerReg) map[string]bool {
		m := map[string]bool{}
		for _, r := range rs {
			m[r.TaskType] = true
		}
		return m
	}
	rs, ls := set(redis), set(lite)
	for tt := range rs {
		if !ls[tt] {
			t.Errorf("类型 %s 仅在 Redis 侧注册", tt)
		}
	}
	for tt := range ls {
		if !rs[tt] {
			t.Errorf("类型 %s 仅在 Lite 侧注册", tt)
		}
	}
}

func TestDiscoverRealRepoHooks(t *testing.T) {
	root := repoRoot(t)
	hooks, err := DiscoverHooks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != wantHooks {
		t.Errorf("container.Invoke 挂点 = %d, want %d（F0 基线）", len(hooks), wantHooks)
	}
}
