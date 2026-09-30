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
// OCR R2 b0-ocr-r2-2：位点入集需归一化后对照磁盘 Go 树验证，fixture 改用
// 克隆仓库提供真实 d.GoFiles（含补写的 internal/router/router.go）。
func TestDiscoverSetConsumersRequiresDirectorySegment(t *testing.T) {
	root := cloneContractRepo(t)
	writeWorkerSiteFiles(t, root, "internal/router/router.go", "package router\n")
	d := workerSiteDiscovery(t, root)
	items := []string{
		"startTenantSkillReaper — internal/container/container.go:702 (func at container.go:2405)",
		"startChat — internal/container/container.go:2074 (func at routes_chat.go:88)",
		"startRouter — internal/router/router.go:940",
	}
	consumers, issues := discoverSetConsumers(d, "lifecycle-set", items)
	require.Equal(t,
		[]string{"internal/container/container.go", "internal/router/router.go"}, consumers)
	require.Empty(t, issues)
	ghost, issues := discoverSetConsumers(d, "lifecycle-set",
		[]string{"ghost — (func at bare.go:12)"})
	require.Empty(t, ghost, "仅含裸文件名的条目不得产生任何消费方")
	require.Empty(t, issues)

	// OCR R2 b0-ocr-r2-2（route/lifecycle-set 同构）：位点不在磁盘上时显式
	// file-missing 诊断，不静默入集也不静默跳过。
	ghost, issues = discoverSetConsumers(d, "lifecycle-set",
		[]string{"startGhost — internal/container/ghost.go:9"})
	require.Empty(t, ghost, "不在磁盘上的位点不得入集")
	require.Len(t, issues, 1)
	require.Equal(t, "contract-consumer-file-missing", issues[0].Check)
	require.Contains(t, issues[0].Message, "internal/container/ghost.go")
}

// TestDiscoverSetConsumersWorkerSetEntrySites 覆盖 OCR R1 #b0-ocr-r1-7 +
// OCR R2 b0-ocr-r2-1/b0-ocr-r2-2：
// worker-set 消费方 = 任务 handler 注册位点。B0 注册全在 internal/router
// （RegisterWorkers 在 16 个 module.go 中均为注释形态），故裸标识符条目回退
// 扫描该根；B1+ RegisterWorkers 落地模块树时，manifest integration_points.workers
// 条目必须同步演进为带注册位点后缀的形态（与 routes/lifecycle_hooks 条目同构），
// 守卫随即从条目推导位点——只改代码不改 manifest 时，模块树内新注册位点会
// 静默漏报（树内 enqueue 引用与注册位点无法用标识符引用区分，见
// internal/modules/datasource/scheduler.go），而 router 旧注册删除后已登记
// consumers 会批量假 vanished。
//
// OCR R2 追加锚定：裸名推导以 entryFileRE 首个匹配索引为单一事实源锚
// （任意分隔符形态可推导、矛盾形态显式诊断）；位点入集前一律
// Clean/ToSlash 归一化并对照磁盘 Go 树验证。
func TestDiscoverSetConsumersWorkerSetEntrySites(t *testing.T) {
	// 磁盘 fixture：router 旧注册未删 + 模块树内注册并存（迁移中间态形态）。
	root := cloneContractRepo(t)
	writeWorkerSiteFiles(t, root,
		"internal/router/task.go", "package router\n"+
			"\n"+
			"import \"github.com/Tencent/WeKnora/internal/types\"\n"+
			"\n"+
			"func RegisterTasks() { _ = types.TypeFoo }\n",
		"internal/modules/identity/workers.go", "package identity\n"+
			"\n"+
			"import \"github.com/Tencent/WeKnora/internal/types\"\n"+
			"\n"+
			"func RegisterWorkers() { _ = types.TypeFoo }\n",
	)
	d := workerSiteDiscovery(t, root)

	// 1. 条目带注册位点后缀：直接从条目推导（含模块树内位点），不依赖扫描根；
	//    裸名同步进回退扫描集合——迁移中间态（新位点已登记、router 旧注册
	//    未删）两个真实引用位点都必须被发现（防止迁移期假 vanished / 漏报）。
	consumers, issues := discoverSetConsumers(d, "worker-set",
		[]string{"TypeFoo — internal/modules/identity/workers.go:12"})
	require.Equal(t,
		[]string{"internal/modules/identity/workers.go", "internal/router/task.go"}, consumers,
		"带位点后缀的条目必须从条目推导注册位点")
	require.Empty(t, issues)

	// 2. B0 裸标识符条目：回退扫描 internal/router 注册根；模块树内裸标识符
	//    引用不是登记口径（现状行为回归锚，条目更新义务由注释登记）。
	consumers, issues = discoverSetConsumers(d, "worker-set", []string{"TypeFoo"})
	require.Equal(t, []string{"internal/router/task.go"}, consumers)
	require.Empty(t, issues)

	// 3. OCR R2 b0-ocr-r2-1：裸名推导不再依赖 " — " 字面分隔符——以
	//    entryFileRE 首个匹配索引为锚取位点前缀并剔除尾部非标识符字符。
	//    en dash / 全角破折号 / em dash 无空格 / 连字符带空格等手写形态下
	//    位点与裸名都必须照常推导，回退扫描（router/task.go 引用 TypeFoo）
	//    必须命中，不得因分隔符漂移把完整条目串静默塞进扫描集合。
	for _, entry := range []string{
		"TypeFoo – internal/modules/identity/workers.go:12", // en dash
		"TypeFoo——internal/modules/identity/workers.go:12",  // 全角破折号
		"TypeFoo—internal/modules/identity/workers.go:12",   // em dash 无空格
		"TypeFoo - internal/modules/identity/workers.go:12", // 连字符带空格
	} {
		got, entryIssues := discoverSetConsumers(d, "worker-set", []string{entry})
		require.Equal(t,
			[]string{"internal/modules/identity/workers.go", "internal/router/task.go"}, got,
			"条目 %q 的位点与裸名都必须照常推导", entry)
		require.Empty(t, entryIssues, "条目 %q 不得产生诊断", entry)
	}

	// 4. OCR R2 b0-ocr-r2-1 矛盾形态：连字符无空格时 entryFileRE 把
	//    "TypeFoo-internal" 整体吞并为首个匹配（锚前缀为空），裸名推导失败
	//    且位点不在磁盘——显式 entry-drift + file-missing 诊断，完整条目串
	//    绝不静默进入回退扫描集合。
	consumers, issues = discoverSetConsumers(d, "worker-set",
		[]string{"TypeFoo-internal/modules/identity/workers.go:12"})
	require.Empty(t, consumers, "矛盾形态位点不得入集")
	require.Len(t, issues, 2)
	checks := []string{issues[0].Check, issues[1].Check}
	require.ElementsMatch(t,
		[]string{"contract-set-entry-drift", "contract-consumer-file-missing"}, checks)
	for _, is := range issues {
		require.Contains(t, is.Message, "TypeFoo-internal/modules/identity/workers.go")
	}

	// 5. OCR R2 b0-ocr-r2-2：manifest 手写位点的点段形态（"./x"、"x/./y.go"）
	//    必须 filepath.Clean 归一化后与 d.GoFiles 同形照常入集。
	for _, entry := range []string{
		"TypeFoo — ./internal/modules/identity/workers.go:12",
		"TypeFoo — internal/modules/./identity/workers.go:12",
	} {
		got, entryIssues := discoverSetConsumers(d, "worker-set", []string{entry})
		require.Equal(t,
			[]string{"internal/modules/identity/workers.go", "internal/router/task.go"}, got,
			"条目 %q 的位点归一化后必须照常入集", entry)
		require.Empty(t, entryIssues, "条目 %q", entry)
	}

	// 6. OCR R2 b0-ocr-r2-2：位点不在磁盘上——显式 file-missing 诊断，不静默
	//    入集（否则归一化后字符串不等会同时误报 unrecorded + vanished）也不
	//    静默跳过；裸名仍进回退扫描集合（router 旧注册可被发现）。
	consumers, issues = discoverSetConsumers(d, "worker-set",
		[]string{"TypeFoo — internal/modules/identity/ghost.go:12"})
	require.Equal(t, []string{"internal/router/task.go"}, consumers,
		"幽灵位点不得入集，但裸名回退扫描仍须发现 router 旧注册")
	require.Len(t, issues, 1)
	require.Equal(t, "contract-consumer-file-missing", issues[0].Check)
	require.Contains(t, issues[0].Message, "internal/modules/identity/ghost.go")
}

// TestDiscoverSetConsumersWorkerSetScanFailure 覆盖 OCR R1
// #b0-ocr-r1-fileReferencesIdent-swallow-error：回退扫描中 parseFileFull
// 完整解析失败的文件必须显式 contract-set-scan-failed 诊断，且该文件不得
// 进入 discovered 集合。discoverGoTree 以 ImportsOnly 枚举 Go 树（import 段
// 之后残缺的文件仍可入集），而 parseFileFull 以 mode 0 完整解析——
// fileReferencesIdent 此前对该错误静默 return false，真实注册位点丢失而
// 守卫毫无感知，与 DiscoverSymbolConsumers 上抛 parse 错误的口径不一致。
func TestDiscoverSetConsumersWorkerSetScanFailure(t *testing.T) {
	root := cloneContractRepo(t)
	// import 段之后残缺（缺 '}'）：ImportsOnly 可解析 → 进 d.GoFiles；
	// mode 0 完整解析失败 → parseFileFull 报错。
	writeWorkerSiteFiles(t, root,
		"internal/router/broken.go", "package router\n"+
			"\n"+
			"import \"github.com/Tencent/WeKnora/internal/types\"\n"+
			"\n"+
			"func broken() {\n"+
			"\t_ = types.TypeFoo\n",
	)
	d := workerSiteDiscovery(t, root)

	consumers, issues := discoverSetConsumers(d, "worker-set", []string{"TypeFoo"})
	require.Empty(t, consumers, "parse 失败的文件不得静默入集")
	require.Len(t, issues, 1)
	require.Equal(t, "contract-set-scan-failed", issues[0].Check)
	require.Contains(t, issues[0].Message, "internal/router/broken.go",
		"诊断须含文件路径")
	require.Contains(t, issues[0].Message, "parse",
		"诊断须携带底层解析错误")
}

// writeWorkerSiteFiles 在克隆 fixture 中写入相对路径 rel 的文件（自动建目录）。
func writeWorkerSiteFiles(t *testing.T, root string, relContents ...string) {
	t.Helper()
	if len(relContents)%2 != 0 {
		t.Fatalf("relContents 必须是 (rel, content) 偶数项，got %d", len(relContents))
	}
	for i := 0; i < len(relContents); i += 2 {
		rel, content := relContents[i], relContents[i+1]
		abs := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
}

// workerSiteDiscovery 从磁盘重新发现 Go 树（含新写入的 worker 位点文件）。
func workerSiteDiscovery(t *testing.T, root string) *Discovery {
	t.Helper()
	goFiles, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	return &Discovery{Root: root, GoFiles: goFiles, Imports: imports}
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
		{
			// 15. 登记的特征化测试文件仍在 Go 树上但不再引用契约符号
			// （OCR R1 ocr-r1-1）：改名/被删由 contract-characterization-missing
			// 文件存在层兜底，「不再引用符号」此前完全无感——冻结既有特征化
			// 测试的承诺必须有符号引用层校验。
			name: "characterization test no longer references symbol",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := cloneContractRepo(t)
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "internal", "handler", "tenant_api_test.go"), []byte(
						`package handler

// TestSomethingElse 不再引用 TenantService（特征化锚点漂移形态）。
func TestSomethingElse() {}
`), 0o644))
				return fixtureContractGovernance(t, root), fixtureContractDiscovery(t, root)
			},
			check: "contract-characterization-drift",
			want:  "tenant_api_test.go",
		},
		{
			// 16. worker-set 条目位点不在磁盘（OCR R2 b0-ocr-r2-2）：manifest
			// 手写位点入集前必须归一化并对照 d.GoFiles 验证，缺失时显式
			// file-missing 诊断（与符号契约 recorded consumer 预检同构），
			// 不静默入集也不静默跳过。
			name: "worker-set entry site missing on disk",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				entry := "TypeFoo — internal/modules/identity/ghost.go:12"
				g.Contracts[3].Items = []string{entry}
				d := fixtureContractDiscovery(t, root)
				d.Manifests[0].IntegrationPoints.Workers = []string{entry}
				return g, d
			},
			check: "contract-consumer-file-missing",
			want:  "internal/modules/identity/ghost.go",
		},
		{
			// 17. worker-set 条目位点命中但裸名不可推导（OCR R2 b0-ocr-r2-1
			// 矛盾形态）：显式 entry-drift 诊断而非把复合串静默塞进回退扫描。
			name: "worker-set entry bare type underivable",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				entry := "— internal/router/routes_tenant.go:10"
				g.Contracts[3].Items = []string{entry}
				d := fixtureContractDiscovery(t, root)
				d.Manifests[0].IntegrationPoints.Workers = []string{entry}
				return g, d
			},
			check: "contract-set-entry-drift",
			want:  "bare task type",
		},
		{
			// 18. set 行登记消费方文件不存在（OCR R2 b0-ocr-r2-2）：与符号
			// 契约 recorded consumer 磁盘预检同构，file-missing 层捕获，
			// vanished 层不越界误报。
			name: "set recorded consumer file missing",
			setup: func(t *testing.T) (*Governance, *Discovery) {
				root := contractRepoRoot(t)
				g := fixtureContractGovernance(t, root)
				g.Contracts[2].Consumers = append(g.Contracts[2].Consumers, "internal/router/ghost.go")
				return g, fixtureContractDiscovery(t, root)
			},
			check: "contract-consumer-file-missing",
			want:  "recorded consumer internal/router/ghost.go",
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

// TestContractCharacterizationDriftLayering 覆盖 OCR R1 ocr-r1-1：登记特征化
// 测试的漂移形态分层归属。生产调用点此前丢弃 DiscoverSymbolConsumers 返回的
// tests（grep 证实仅 *_test.go 消费该返回值），对 c.CharacterizationTests 只做
// goFileSet 磁盘存在性检查——「冻结既有特征化测试」弱化为单层防线：
//   - 改名/被删（路径不在 Go 树上）：仍由 contract-characterization-missing
//     文件存在层捕获，drift 层不得越界误报；
//   - 不再引用符号（文件仍在树上）：由新的 contract-characterization-drift
//     符号引用层捕获，且不得同时误报 missing。
//
// unrecorded 反方向（树上有测试引用符号但未登记）按验收口径不强制。
func TestContractCharacterizationDriftLayering(t *testing.T) {
	t.Run("no longer references symbol triggers drift not missing", func(t *testing.T) {
		root := cloneContractRepo(t)
		require.NoError(t, os.WriteFile(
			filepath.Join(root, "internal", "handler", "tenant_api_test.go"), []byte(
				`package handler

// TestSomethingElse 不再引用 TenantService（特征化锚点漂移形态）。
func TestSomethingElse() {}
`), 0o644))
		diags := CheckContracts(fixtureContractGovernance(t, root), fixtureContractDiscovery(t, root))
		diag, ok := findDiag(diags, "contract-characterization-drift")
		require.True(t, ok, "登记测试仍在 Go 树上但不再引用符号必须报 drift，got: %v", diags)
		require.Equal(t,
			"recorded test internal/handler/tenant_api_test.go no longer references TenantService",
			diag.Message)
		_, missing := findDiag(diags, "contract-characterization-missing")
		require.False(t, missing, "文件仍在磁盘上不得误报 missing，got: %v", diags)
	})
	t.Run("renamed away stays with missing file layer", func(t *testing.T) {
		root := contractRepoRoot(t)
		g := fixtureContractGovernance(t, root)
		// 改名形态：登记路径已不在 Go 树上（测试文件被改名带走）。
		g.Contracts[0].CharacterizationTests = []string{"internal/handler/renamed_away_test.go"}
		diags := CheckContracts(g, fixtureContractDiscovery(t, root))
		diag, ok := findDiag(diags, "contract-characterization-missing")
		require.True(t, ok, "改名/被删形态必须仍由 missing 文件存在层捕获，got: %v", diags)
		require.Contains(t, diag.Message, "renamed_away_test.go")
		_, drift := findDiag(diags, "contract-characterization-drift")
		require.False(t, drift, "路径不在 Go 树上不得报 drift（那是 missing 层语义），got: %v", diags)
	})
}

// TestContractBaselineSkipsComparisonWhenLedgerUnparseable 覆盖 OCR R1 #04：
// Pass A 验收台账缺失/不可解析时，只报单条 contract-baseline-ledger 结构性
// 诊断，不得以零值 led 继续三行对照——那会产生「disagrees with pass-a
// ledger 0」的 drift 与 missing 级联噪声，污染 CI 输出并误导排障方向。
func TestContractBaselineSkipsComparisonWhenLedgerUnparseable(t *testing.T) {
	root := cloneContractRepo(t)
	require.NoError(t, os.Remove(filepath.Join(root,
		"docs", "architecture", "evidence", "pass-a-acceptance.md")))

	g := fixtureContractGovernance(t, root)
	d := fixtureContractDiscovery(t, root)
	diags := CheckContracts(g, d)

	led, ok := findDiag(diags, "contract-baseline-ledger")
	require.True(t, ok, "台账不可解析必须报 contract-baseline-ledger，got: %v", diags)
	require.Contains(t, led.Message, "cannot parameterize baseline counts")
	for _, check := range []string{"contract-baseline-drift", "contract-baseline-missing"} {
		_, hit := findDiag(diags, check)
		require.Falsef(t, hit,
			"台账不可解析时不得以零值基线产生 %s 级联诊断（实得 %v）", check, diags)
	}
}

// TestContractMethodFallbackFactSkipsConsumerDiscovery 覆盖 OCR R2 f5：
// DiscoverSymbol 的同名方法回退（DeclKind=="method"）命中时，契约按顶层语义
// 报 contract-symbol-missing「no longer declares」并保留签名对照，但不得把
// method fact 送入 DiscoverSymbolConsumers——方法引用形态 recv.Name() 的
// SelectorExpr.X 是变量而非定义包限定名，qualifiers 匹配对跨包消费方全部
// 不命中 → consumers 恒空 → 已登记消费方全量误报 contract-consumer-vanished。
func TestContractMethodFallbackFactSkipsConsumerDiscovery(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/store/store.go", `package store

// Store 是 fixture 契约符号宿主。
type Store struct{}

// AppendFact 是方法形态符号：顶层声明缺失，仅方法回退可命中。
func (s *Store) AppendFact(id string) error { return nil }
`)
	writeDiscoverFixture(t, root, "internal/handler/use.go", `package handler

import "github.com/Tencent/WeKnora/internal/store"

// UseAppendFact 以 recv.Name() 形态真实调用方法。
func UseAppendFact(s *store.Store) error { return s.AppendFact("x") }
`)
	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	d := &Discovery{Root: root, GoFiles: files, Imports: imports}

	// 前置：方法回退命中且 fact 是 method（R1 #15 的事件 producer 语义不变）。
	fact, found, err := DiscoverSymbol(root, "internal/store/store.go", "AppendFact")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "method", fact.DeclKind)

	newGov := func(signature string) *Governance {
		return &Governance{Contracts: []Contract{{
			ID:        "store.append-fact",
			Owner:     "store",
			Kind:      "capability-port",
			Symbol:    "internal/store/store.go:AppendFact",
			Signature: signature,
			Stability: "frozen",
			// use.go 真实调用 s.AppendFact——消费方登记是真实的。
			Consumers: []string{"internal/handler/use.go"},
		}}}
	}

	t.Run("method fact reports no longer declares without false vanished", func(t *testing.T) {
		g := newGov(fact.Signature) // 冻结签名 == 当前方法签名：无 drift
		diags := CheckContracts(g, d)
		diag, ok := findDiag(diags, "contract-symbol-missing")
		require.True(t, ok, "方法回退命中必须按契约顶层语义报 no longer declares，got: %v", diags)
		require.Contains(t, diag.Message, "no longer declares")
		require.Contains(t, diag.Message, "AppendFact")
		for _, check := range []string{
			"contract-consumer-vanished", "contract-consumer-unrecorded", "contract-signature-drift",
		} {
			_, hit := findDiag(diags, check)
			require.Falsef(t, hit,
				"方法 fact 不得产生 %s（use.go 真实调用 s.AppendFact，限定名匹配语义不适用于方法引用形态），got: %v",
				check, diags)
		}
	})

	t.Run("signature drift still reported for method fact", func(t *testing.T) {
		g := newGov("func AppendFact(id string) error") // 顶层形态的冻结签名
		diags := CheckContracts(g, d)
		_, ok := findDiag(diags, "contract-signature-drift")
		require.True(t, ok, "方法 fact 保留签名对照（drift 必须照报），got: %v", diags)
		_, ok = findDiag(diags, "contract-symbol-missing")
		require.True(t, ok, "got: %v", diags)
		_, hit := findDiag(diags, "contract-consumer-vanished")
		require.False(t, hit, "got: %v", diags)
	})
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
