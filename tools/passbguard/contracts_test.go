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

// ---- B0.4 Step 1：符号/消费方发现与契约校验的失败测试（先 RED）----

// contractRepoRoot 是最小契约 fixture 仓库（见 testdata/contractrepo）：
// 一个 TenantService 接口端口、两个生产消费方（其一带在册跨模块例外导入）、
// 一个不引用端口的 /adapters 导入文件、路由/挂点存在性 stub、
// 1 项入口/0 worker/1 挂点的 module.go 门面注释与迷你验收台账。
func contractRepoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "contractrepo")
}

// cloneContractRepo 把 fixture 仓库复制到独立临时目录，供需要改写磁盘状态的用例。
func cloneContractRepo(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src, err := filepath.Abs(contractRepoRoot(t))
	require.NoError(t, err)
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	require.NoError(t, err)
	return dst
}

// fixtureContractDiscovery 从 fixture 仓库磁盘发现 Go 树，并挂上迷你 identity manifest。
func fixtureContractDiscovery(t *testing.T, root string) *Discovery {
	t.Helper()
	goFiles, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	return &Discovery{
		Root:    root,
		GoFiles: goFiles,
		Imports: imports,
		Manifests: []*movemanifest.MoveManifest{
			{
				Module: "identity",
				IntegrationPoints: movemanifest.IntegrationPoints{
					Routes:         []string{"RegisterTenantRoutes — internal/router/routes_tenant.go:10"},
					Workers:        nil,
					LifecycleHooks: []string{"startThing — internal/container/container.go:20"},
				},
			},
		},
	}
}

// fixtureTenantSignature 读取 fixture TenantService 的真实渲染签名（渲染器输出即冻结语义）。
func fixtureTenantSignature(t *testing.T, root string) string {
	t.Helper()
	fact, found, err := DiscoverSymbol(root, "internal/types/interfaces/tenant.go", "TenantService")
	require.NoError(t, err)
	require.True(t, found)
	return fact.Signature
}

// fixtureContractGovernance 是与 fixture 仓库完全一致的合法契约集。
func fixtureContractGovernance(t *testing.T, root string) *Governance {
	t.Helper()
	return &Governance{
		Contracts: []Contract{
			{
				ID:        "identity.tenant-service",
				Owner:     "identity",
				Kind:      "capability-port",
				Symbol:    "internal/types/interfaces/tenant.go:TenantService",
				Signature: fixtureTenantSignature(t, root),
				Stability: "frozen",
				Consumers: []string{
					"internal/handler/tenant_api.go",
					"internal/modules/insights/rogue_cross.go",
				},
				CharacterizationTests: []string{"internal/handler/tenant_api_test.go"},
			},
			{
				ID:        "identity.facade",
				Owner:     "identity",
				Kind:      "module-construction",
				Symbol:    "internal/modules/identity/module.go",
				Signature: "façade",
				Stability: "frozen",
				Items:     []string{"NewModule", "RegisterRoutes", "RegisterWorkers", "Start", "Stop"},
			},
			{
				ID:        "identity.routes",
				Owner:     "identity",
				Kind:      "route-set",
				Symbol:    "docs/architecture/moves/identity.yaml",
				Signature: "integration_points.routes",
				Stability: "frozen",
				Items:     []string{"RegisterTenantRoutes — internal/router/routes_tenant.go:10"},
				Consumers: []string{"internal/router/routes_tenant.go"},
			},
			{
				ID:        "identity.workers",
				Owner:     "identity",
				Kind:      "worker-set",
				Symbol:    "docs/architecture/moves/identity.yaml",
				Signature: "integration_points.workers",
				Stability: "frozen",
			},
			{
				ID:        "identity.lifecycle",
				Owner:     "identity",
				Kind:      "lifecycle-set",
				Symbol:    "docs/architecture/moves/identity.yaml",
				Signature: "integration_points.lifecycle_hooks",
				Stability: "frozen",
				Items:     []string{"startThing — internal/container/container.go:20"},
				Consumers: []string{"internal/container/container.go"},
			},
			{
				ID:        "composition.baseline-routes",
				Owner:     "system",
				Kind:      "route-set",
				Symbol:    "docs/architecture/evidence/pass-a-acceptance.md",
				Signature: "baseline totals",
				Stability: "frozen",
				Items:     []string{"routes_total=7", "routes_literal=5", "routes_apikey=2"},
			},
			{
				ID:        "composition.baseline-workers",
				Owner:     "system",
				Kind:      "worker-set",
				Symbol:    "docs/architecture/evidence/pass-a-acceptance.md",
				Signature: "baseline totals",
				Stability: "frozen",
				Items:     []string{"worker_types=4", "worker_pools=6", "worker_redis=4", "worker_lite=4"},
			},
			{
				ID:        "composition.baseline-lifecycle",
				Owner:     "system",
				Kind:      "lifecycle-set",
				Symbol:    "docs/architecture/evidence/pass-a-acceptance.md",
				Signature: "baseline totals",
				Stability: "frozen",
				Items:     []string{"hooks=9"},
			},
		},
		Exceptions: []Exception{
			{
				ID:   "exc-fixture-1",
				From: "internal/modules/insights/rogue_cross.go",
				To:   "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat",
				Plan: "11-airesource", RemoveAt: "ib1", Reason: "fixture 在册例外",
			},
		},
	}
}

// TestContractRenderedSignatureNormalization 锚定签名渲染规范化形态：
// 接口渲染为单行 `interface { M(...); N(...) }`，参数与结果保持源序文本。
func TestContractRenderedSignatureNormalization(t *testing.T) {
	fact, found, err := DiscoverSymbol(contractRepoRoot(t), "internal/types/interfaces/tenant.go", "TenantService")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "interface", fact.DeclKind)
	require.Equal(t, "github.com/Tencent/WeKnora/internal/types/interfaces", fact.ImportPath)
	require.Equal(t,
		"interface { CreateTenant(ctx context.Context, name string) error; "+
			"GetTenantByID(ctx context.Context, id uint64) (string, error) }",
		fact.Signature)
}

// TestContractDiscoveryFindsProductionConsumersAndTests 验证消费方发现：
// 生产消费方（非测试、限定名引用、导入定义包）与特征化测试文件分开返回，
// 不引用符号的 rogue_adapters.go 不算消费方。
func TestContractDiscoveryFindsProductionConsumersAndTests(t *testing.T) {
	root := contractRepoRoot(t)
	fact, found, err := DiscoverSymbol(root, "internal/types/interfaces/tenant.go", "TenantService")
	require.NoError(t, err)
	require.True(t, found)
	consumers, tests, err := DiscoverSymbolConsumers(fixtureContractDiscovery(t, root), fact)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/handler/tenant_api.go", "internal/modules/insights/rogue_cross.go"}, consumers)
	require.Equal(t, []string{"internal/handler/tenant_api_test.go"}, tests)
}

// TestContractCheckHappyPathProducesNoDiagnostics：合法契约集零诊断。
func TestContractCheckHappyPathProducesNoDiagnostics(t *testing.T) {
	root := contractRepoRoot(t)
	diags := CheckContracts(fixtureContractGovernance(t, root), fixtureContractDiscovery(t, root))
	require.Empty(t, diags, "diags: %v", diags)
}

// TestContractDiscoveryQualifiesByDefinitionPackageClause 覆盖 OCR R1 #f12：
// 定义包 internal/version/ver 的包子句是 version（≠目录尾段 ver）。无别名
// 导入的消费方以真实包名（version.APIVersion）限定引用，发现必须命中——
// 默认限定名取定义文件 package 子句，不得按 import 路径尾段推导。
func TestContractDiscoveryQualifiesByDefinitionPackageClause(t *testing.T) {
	root := contractRepoRoot(t)
	fact, found, err := DiscoverSymbol(root, "internal/version/ver/version.go", "APIVersion")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "github.com/Tencent/WeKnora/internal/version/ver", fact.ImportPath)

	consumers, tests, err := DiscoverSymbolConsumers(fixtureContractDiscovery(t, root), fact)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/handler/version_consumer.go"}, consumers,
		"以包子句 version 限定引用的消费方必须被发现")
	require.Empty(t, tests)
}

// TestDiscoverSetConsumersRequiresDirectorySegment 覆盖 OCR R1 #f4：
// entryFileRE 只认带目录段的 .go 引用——manifest lifecycle_hooks 条目括号内
// 的交叉引用（如 "(func at container.go:2405)"，见 moves/agentcatalog.yaml）
// 是裸文件名，不得算作 set 消费方（曾冻结出 14 条幽灵 consumer）。
func TestDiscoverSetConsumersRequiresDirectorySegment(t *testing.T) {
	items := []string{
		"startTenantSkillReaper — internal/container/container.go:702 (func at container.go:2405)",
		"startChat — internal/container/container.go:2074 (func at routes_chat.go:88)",
		"startRouter — internal/router/router.go:940",
	}
	require.Equal(t,
		[]string{"internal/container/container.go", "internal/router/router.go"},
		discoverSetConsumers(&Discovery{}, "lifecycle-set", items))
	require.Empty(t, discoverSetConsumers(&Discovery{}, "lifecycle-set",
		[]string{"ghost — (func at bare.go:12)"}),
		"仅含裸文件名的条目不得产生任何消费方")
}

// TestContractCheckDiagnostics 表驱动覆盖 Review Focus：
// 符号缺失、签名漂移、未记录新消费方、消费方消失、消费方导入 /adapters 或
// 其他模块非公开子包、集合漂移/缺失、基线漂移、门面形态/计数矛盾、特征化测试缺失。
func TestContractCheckDiagnostics(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) (*Governance, *Discovery) // 默认基于 fixture，可换临时仓库
		check string
		want  string
	}{
		{
			// 1. 冻结符号在源码中不存在（改名）。
			name: "missing symbol",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Symbol = "internal/types/interfaces/tenant.go:GhostService"
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-symbol-missing",
			want:  "GhostService",
		},
		{
			// 2. 冻结符号文件不存在。
			name: "missing symbol file",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Symbol = "internal/types/interfaces/ghost.go:TenantService"
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-symbol-missing",
			want:  "ghost.go",
		},
		{
			// 3. 源码签名与冻结签名不一致。
			name: "changed signature",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Signature = "interface { WRONG() }"
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-signature-drift",
			want:  "interface { WRONG() }",
		},
		{
			// 4. 出现未记录的新消费方（冻结清单少了真实引用者）。
			name: "new unrecorded consumer",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Consumers = []string{"internal/handler/rogue_cross.go"}
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-unrecorded",
			want:  "internal/handler/tenant_api.go",
		},
		{
			// 5a. 冻结消费方在磁盘上已不引用符号。
			name: "vanished consumer",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Consumers = append(g.Contracts[0].Consumers, "internal/handler/rogue_adapters.go")
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-vanished",
			want:  "rogue_adapters.go",
		},
		{
			// 5b. 冻结消费方文件整个不存在。
			name: "consumer file missing",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Consumers = append(g.Contracts[0].Consumers, "internal/handler/ghost.go")
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-file-missing",
			want:  "ghost.go",
		},
		{
			// 6. 消费方导入其他模块的 /adapters 非公开子包。
			name: "consumer imports adapters subpackage",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := cloneContractRepo(t)
				err := os.WriteFile(filepath.Join(root, "internal", "handler", "bad_adapters.go"), []byte(
					`package handler

import (
	ifaces "github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/modules/identity/adapters/thing"
)

func BadAdapters(s ifaces.TenantService) { _ = thing.Adapter{} }
`), 0o644)
				require.NoError(t, err)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].Consumers = append(g.Contracts[0].Consumers, "internal/handler/bad_adapters.go")
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-adapters-import",
			want:  "identity/adapters",
		},
		{
			// 7. 消费方导入其他模块非公开子包且无在册例外。
			name: "consumer imports other module non-public subpackage",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Exceptions = nil // rogue_cross.go 的跨模块导入失去例外
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-module-import",
			want:  "airesource",
		},
		{
			// 8. route-set 与 manifest integration_points.routes 漂移。
			name: "route set drift",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[2].Items = []string{"RegisterGhostRoutes — internal/router/routes_tenant.go:10"}
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-set-drift",
			want:  "RegisterGhostRoutes",
		},
		{
			// 9. manifest 模块缺 worker-set 契约行。
			name: "missing set contract",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts = append(g.Contracts[:3], g.Contracts[4:]...) // 删 identity.workers
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-set-missing",
			want:  "worker-set",
		},
		{
			// 10. 组合基线与验收台账不一致。
			name: "baseline drift",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[5].Items = []string{"routes_total=8", "routes_literal=5", "routes_apikey=2"}
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-baseline-drift",
			want:  "routes_total",
		},
		{
			// 11. module.go 注释声明的门面操作与冻结形态不一致。
			name: "facade shape drift",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[1].Items = []string{"NewModule", "RegisterRoutes", "RegisterWorkers", "Start"}
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-facade-shape-drift",
			want:  "Stop",
		},
		{
			// 12. module.go 注释计数与 manifest 冻结登记数矛盾。
			name: "facade count contradicts manifest",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := cloneContractRepo(t)
				content := "// Package identity fixture（计数矛盾版）。\n" +
					"//\n" +
					"//\tNewModule(deps Dependencies) (*Module, error)\n" +
					"//\t(m *Module) RegisterRoutes(r RouteRegistrar)\n" +
					"//\t    注册本模块 HTTP 路由（当前 2 项入口，见 manifest integration_points.routes）。\n" +
					"//\t(m *Module) RegisterWorkers(mux WorkerRegistrar)\n" +
					"//\t    注册本模块 asynq/Lite 任务处理器（当前 0 项，见 integration_points.workers）。\n" +
					"//\t(m *Module) Start(ctx context.Context) error\n" +
					"//\t    启动后台任务（当前 1 项生命周期挂点，见 integration_points.lifecycle_hooks）。\n" +
					"//\t(m *Module) Stop(ctx context.Context) error\n" +
					"//\t    优雅停止。\n" +
					"package identity\n"
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "internal", "modules", "identity", "module.go"),
					[]byte(content), 0o644))
				return fixtureContractGovernance(t, root), fixtureContractDiscovery(t, root)
			},
			check: "contract-facade-count-drift",
			want:  "routes",
		},
		{
			// 13. 冻结的特征化测试路径不存在。
			name: "characterization test missing",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[0].CharacterizationTests = append(
					g.Contracts[0].CharacterizationTests, "internal/handler/ghost_test.go")
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-characterization-missing",
			want:  "ghost_test.go",
		},
		{
			// 14. worker 池数基线漂移（OCR R1 #f5：台账「N 任务类型 + M 池 / K」
			// 的池数 M 必须由 worker_pools 键承载并参与比对——fixture 台账池数
			// 为 6，登记 8 必须报 drift）。
			name: "worker pools baseline drift",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[6].Items = []string{
					"worker_types=4", "worker_pools=8", "worker_redis=4", "worker_lite=4",
				}
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-baseline-drift",
			want:  "worker_pools",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, d := tc.setup(t)
			diags := CheckContracts(g, d)
			diag, ok := findDiag(diags, tc.check)
			require.True(t, ok, "want check %q, got diags: %v", tc.check, diags)
			require.Contains(t, diag.Message, tc.want)
		})
	}
}

// ---- 真实仓库冻结验证（B0.4 Step 7）----

// TestRealRepoContractsFreezeCurrentSurfaces 覆盖 Review Focus「契约抽取必须含全部
// 生产消费方并拒绝未记录新消费方」+ Step 7 基线计数：真实仓库零诊断，
// 七种契约 kind 全部在册，B1 四模块与下游主题面齐备，基线与台账参数化一致。
func TestRealRepoContractsFreezeCurrentSurfaces(t *testing.T) {
	root := repoRootFromTest(t)
	d, err := DiscoverPassB(root)
	require.NoError(t, err)
	g, err := LoadGovernance(root)
	require.NoError(t, err)

	diags := CheckContracts(g, d)
	require.Empty(t, diags, "diags: %v", diags)

	// 七种 kind 全部冻结在册。
	kindCount := map[string]int{}
	for _, c := range g.Contracts {
		kindCount[c.Kind]++
	}
	for _, kind := range KnownContractKinds {
		require.NotZerof(t, kindCount[kind], "contracts.yaml 缺少 kind %s 的冻结记录", kind)
	}
	// 16 个 manifest 模块各有 module-construction + 三种 set 契约。
	for _, m := range d.Manifests {
		for _, kind := range []string{"module-construction", "route-set", "worker-set", "lifecycle-set"} {
			n := 0
			for _, c := range g.Contracts {
				if c.Owner == m.Module && c.Kind == kind && !strings.HasPrefix(string(c.ID), "composition.baseline-") {
					n++
				}
			}
			require.Equal(t, 1, n, "module %s 的 %s 契约行数应为 1", m.Module, kind)
		}
	}
	// B1 四模块能力面（Step 4）与下游主题面（Step 5）齐备。
	themeMin := map[string]int{
		"identity": 5, "airesource": 5, "commercial": 5, "execution": 3,
		"knowledge": 4, "conversation": 4, "agentcatalog": 3, "agentruntime": 5, "workbench": 3,
	}
	ownerSymbols := map[string]int{}
	for _, c := range g.Contracts {
		switch c.Kind {
		case "capability-port", "wire-protocol", "data-ownership":
			ownerSymbols[c.Owner]++
		}
	}
	for owner, min := range themeMin {
		require.GreaterOrEqual(t, ownerSymbols[owner], min,
			"%s 的能力/线协议/数据面契约不足 %d 条", owner, min)
	}

	// 基线行与台账三方一致（参数化读取，F5）。
	totalRoutes, totalWorkers, totalHooks := compositionBaselineFromLedger(t, root)
	gotRoutes, gotWorkers, gotHooks := 0, 0, 0
	for _, c := range g.Contracts {
		switch c.ID {
		case "composition.baseline-routes":
			gotRoutes = baselineItemValue(t, c.Items, "routes_total")
		case "composition.baseline-workers":
			gotWorkers = baselineItemValue(t, c.Items, "worker_types")
		case "composition.baseline-lifecycle":
			gotHooks = baselineItemValue(t, c.Items, "hooks")
		}
	}
	require.Equal(t, totalRoutes, gotRoutes, "routes 基线与台账不符")
	require.Equal(t, totalWorkers, gotWorkers, "worker 基线与台账不符")
	require.Equal(t, totalHooks, gotHooks, "hooks 基线与台账不符")
}

// compositionBaselineFromLedger 从 Pass A 验收台账解析组合基线计数（F5 参数化，
// 期望值不得写死字面量）。返回 routes 总数、worker 任务类型数、hooks 数。
func compositionBaselineFromLedger(t *testing.T, root string) (routes, workers, hooks int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "evidence", "pass-a-acceptance.md"))
	require.NoError(t, err)
	text := string(data)
	m := regexpFind(t, text, `路由注册 \| (\d+)（(\d+) literal \+ (\d+) apiKeyRoute）`)
	routes = atoi(t, m[1])
	m = regexpFind(t, text, `Redis/Lite worker \| (\d+) 任务类型 \+ (\d+) 池 / (\d+)`)
	workers = atoi(t, m[1])
	redisN, liteN := atoi(t, m[1]), atoi(t, m[3])
	require.Equal(t, redisN, liteN, "台账 Redis/Lite 任务类型数不一致")
	m = regexpFind(t, text, `container\.Invoke hooks \| (\d+)`)
	hooks = atoi(t, m[1])
	return routes, workers, hooks
}

func regexpFind(t *testing.T, text, pattern string) []string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	require.NotNil(t, m, "台账未匹配 %s", pattern)
	return m
}

func baselineItemValue(t *testing.T, items []string, key string) int {
	t.Helper()
	for _, it := range items {
		if strings.HasPrefix(it, key+"=") {
			return atoi(t, strings.TrimPrefix(it, key+"="))
		}
	}
	t.Fatalf("基线行缺 %s 项", key)
	return 0
}
