package main

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Diagnostic 是 passbguard 的一条治理违规。
type Diagnostic struct {
	Check   string
	Path    string
	Message string
}

// String 输出形如 "passbguard: <check>: <path>: <message>"（path 可空时省略）。
func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("passbguard: %s: %s", d.Check, d.Message)
	}
	return fmt.Sprintf("passbguard: %s: %s: %s", d.Check, d.Path, d.Message)
}

// sortDiagnosticsDiag 按 String 排序并去重。
func sortDiagnosticsDiag(ds []Diagnostic) []Diagnostic {
	out := append([]Diagnostic(nil), ds...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	dedup := make([]Diagnostic, 0, len(out))
	for i, d := range out {
		if i == 0 || d.String() != out[i-1].String() {
			dedup = append(dedup, d)
		}
	}
	return dedup
}

// PlanInfo 描述一个 Pass B 子计划：可属主的模块集合与阶段集成屏障。
type PlanInfo struct {
	Modules []string
	Barrier string
}

var allModules = append([]string(nil), KnownModules...)

// KnownPlans 是框架计划表（docs/plans/2026-09-23-backend-modularization-pass-b-framework.md）
// 中可拥有 legacy/alias/exception 的计划；IB 计划（19/29/39/49）不拥有文件，不在此列。
var KnownPlans = map[PlanID]PlanInfo{
	"10-identity":                     {Modules: []string{"identity"}, Barrier: "ib1"},
	"11-airesource":                   {Modules: []string{"airesource"}, Barrier: "ib1"},
	"12-commercial":                   {Modules: []string{"commercial"}, Barrier: "ib1"},
	"13-execution":                    {Modules: []string{"execution"}, Barrier: "ib1"},
	"20-knowledge-program":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"21-knowledge-ingest":             {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"22-knowledge-retrieval":          {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"23-knowledge-wikifaq":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"24-knowledge-process":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"25-agentcatalog-program":         {Modules: []string{"agentcatalog"}, Barrier: "ib2"},
	"26-datasource":                   {Modules: []string{"datasource"}, Barrier: "ib2"},
	"27-appconnector":                 {Modules: []string{"appconnector"}, Barrier: "ib2"},
	"30-agentruntime-program":         {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"31-agentruntime-memory":          {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"32-agentruntime-tools":           {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"33-agentruntime-engine":          {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"34-agentruntime-protocol":        {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"35-conversation-program":         {Modules: []string{"conversation"}, Barrier: "ib3"},
	"36-channels":                     {Modules: []string{"channels"}, Barrier: "ib3"},
	"37-insights":                     {Modules: []string{"insights"}, Barrier: "ib3"},
	"40-workbench":                    {Modules: []string{"workbench"}, Barrier: "ib4"},
	"41-craft":                        {Modules: []string{"craft"}, Barrier: "ib4"},
	"42-system-policy":                {Modules: []string{"system", "policy"}, Barrier: "ib4"},
	"50-final-cleanup-and-acceptance": {Modules: allModules, Barrier: "b5"},
}

// PassBTaskModule 把 guard 源 importExceptions 的模块级 PassBTask 映射到模块 id，
// 用于例外属主计划与 guard 源的模块绑定校验。
var PassBTaskModule = map[string]string{
	"B-agentcatalog": "agentcatalog",
	"B-agentruntime": "agentruntime",
	"B-airesource":   "airesource",
	"B-appconnector": "appconnector",
	"B-channels":     "channels",
	"B-conversation": "conversation",
	"B-craft":        "craft",
	"B-execution":    "execution",
	"B-knowledge":    "knowledge",
	"B-workbench":    "workbench",
	// 任务级绑定（IB2 集成侧补录）：b2-datasource B2-DS.5 登记例外行采用任务级
	// PassBTask（26-datasource §5 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY，
	// 分支侧已审查数据行保持逐字不动，集成侧补模块映射）。
	"B2-DS.5": "datasource",
}

// modulesDirPrefix 是所有模块目标包的强制前缀。
const modulesDirPrefix = "internal/modules/"

// CheckOwnership 把治理文件（ownership-matrix.yaml / exception-ledger.yaml 生成的
// Governance）与仓库发现（Discovery）对照，报告全部覆盖/重叠/属主/期限诊断。
// 返回值已排序去重；空切片即通过。
func CheckOwnership(g *Governance, d *Discovery) []Diagnostic {
	var ds []Diagnostic
	emit := func(check, path, format string, args ...any) {
		ds = append(ds, Diagnostic{Check: check, Path: path, Message: fmt.Sprintf(format, args...)})
	}

	manifestLegacy := map[string]string{} // path -> module
	for _, lf := range d.LegacyFiles {
		manifestLegacy[lf.Path] = lf.Module
	}
	goFileSet := make(map[string]bool, len(d.GoFiles))
	for _, p := range d.GoFiles {
		goFileSet[p] = true
	}

	// ---- legacy 覆盖 / 重叠 / 属主一致性 ----
	seenLegacy := map[string]PlanID{}
	for _, row := range g.Legacy {
		manifest, declared := manifestLegacy[row.Path]
		switch {
		case !declared:
			emit("legacy-undeclared", row.Path, "not declared by any manifest legacy_files")
		case manifest != row.Module:
			emit("legacy-manifest-module", row.Path,
				"matrix module %q disagrees with manifest owner %q", row.Module, manifest)
		}
		if !goFileSet[row.Path] {
			emit("legacy-nonexistent", row.Path, "path does not exist on disk")
		}
		if prev, dup := seenLegacy[row.Path]; dup {
			emit("legacy-overlap", row.Path, "claimed by two plans %q and %q", prev, row.Plan)
		} else {
			seenLegacy[row.Path] = row.Plan
		}
		info, ok := KnownPlans[row.Plan]
		if !ok {
			emit("legacy-plan-unknown", row.Path, "plan %q is not a framework plan that can own files", row.Plan)
			continue
		}
		if !containsModule(info.Modules, row.Module) {
			emit("legacy-plan-module", row.Path,
				"plan %q (modules %s) cannot own a %q file",
				row.Plan, strings.Join(info.Modules, "/"), row.Module)
		}
		wantPrefix := modulesDirPrefix + row.Module + "/"
		if !strings.HasPrefix(row.Destination, wantPrefix) {
			emit("legacy-destination", row.Path,
				"destination %q is outside owning module tree %q", row.Destination, wantPrefix)
		}
		if row.DeleteBarrier != info.Barrier {
			emit("legacy-barrier", row.Path,
				"delete_barrier %q does not match plan %q phase barrier %q",
				row.DeleteBarrier, row.Plan, info.Barrier)
		}
		if row.IntegrationOwner != "" && knownBarrier(row.IntegrationOwner) && row.IntegrationOwner != info.Barrier {
			emit("legacy-integration-owner", row.Path,
				"integration_owner %q must be the owning plan phase barrier %q when set to a barrier",
				row.IntegrationOwner, info.Barrier)
		}
	}
	for path, module := range manifestLegacy {
		if _, owned := seenLegacy[path]; !owned {
			emit("legacy-missing", path, "manifest legacy file of module %q has no ownership-matrix owner", module)
		}
	}

	// ---- alias 义务覆盖 ----
	aliasModule := map[string]string{} // old_import_path -> module
	for _, m := range d.Manifests {
		for _, a := range m.AliasObligations {
			aliasModule[a.OldImportPath] = m.Module
		}
	}
	for _, a := range d.Aliases {
		if _, ok := aliasModule[a.OldImportPath]; !ok {
			aliasModule[a.OldImportPath] = PassBTaskModule[a.PassBTask]
		}
	}
	seenAlias := map[string]bool{}
	for _, row := range g.Aliases {
		module, declared := aliasModule[row.OldImportPath]
		if !declared {
			emit("alias-undeclared", row.OldImportPath, "not declared by any manifest alias_obligations")
		}
		seenAlias[row.OldImportPath] = true
		info, ok := KnownPlans[row.Plan]
		if !ok {
			emit("alias-plan-unknown", row.OldImportPath,
				"plan %q is not a framework plan that can own files", row.Plan)
			continue
		}
		if declared && module != "" && !containsModule(info.Modules, module) {
			emit("alias-plan-module", row.OldImportPath,
				"plan %q (modules %s) cannot own an alias of module %q",
				row.Plan, strings.Join(info.Modules, "/"), module)
		}
		if row.DeleteBarrier != "" && row.DeleteBarrier != info.Barrier {
			emit("alias-barrier", row.OldImportPath,
				"delete_barrier %q does not match plan %q phase barrier %q",
				row.DeleteBarrier, row.Plan, info.Barrier)
		}
	}
	for path, module := range aliasModule {
		if !seenAlias[path] {
			emit("alias-missing", path, "manifest alias obligation of module %q has no ownership-matrix owner", module)
		}
	}

	// ---- exception 台账与 guard 源的精确对照（F1）----
	type excKey struct{ from, to string }
	discovered := map[excKey]DiscoveredException{}
	for _, de := range d.Exceptions {
		discovered[excKey{de.ImporterFile, de.ImportedPath}] = de
	}
	// seenEdge 是边级判重（OCR R1 #2）：同一 (from,to) 只允许一行例外——
	// validate 只对相邻同 ExceptionID 判重（排序后同边不同 id 不相邻），
	// 与 legacy-overlap 双防护同构地由这里兜底。
	seenEdge := map[excKey]ExceptionID{}
	ledger := map[excKey]Exception{}
	for _, x := range g.Exceptions {
		key := excKey{x.From, x.To}
		if prev, dup := seenEdge[key]; dup {
			emit("exception-overlap", x.From,
				"%s → %s claimed by two exceptions %q and %q", x.From, x.To, prev, x.ID)
		} else {
			seenEdge[key] = x.ID
		}
		ledger[key] = x
		de, registered := discovered[key]
		if !registered {
			emit("exception-unregistered", x.From,
				"%s → %s is not registered in %s importExceptions",
				x.From, x.To, architectureGuardCheckFile)
		} else {
			if x.Reason != de.Reason {
				emit("exception-reason-drift", x.From,
					"reason drift for %s → %s: ledger %q vs guard %q",
					x.From, x.To, x.Reason, de.Reason)
			}
			taskModule := PassBTaskModule[de.PassBTask]
			info, ok := KnownPlans[x.Plan]
			if !ok {
				emit("exception-plan-unknown", x.From,
					"plan %q is not a framework plan that can own exceptions", x.Plan)
			} else if taskModule == "" {
				emit("exception-task-module", x.From,
					"guard PassBTask %q has no module mapping", de.PassBTask)
			} else if !containsModule(info.Modules, taskModule) {
				emit("exception-plan-module", x.From,
					"plan %q (modules %s) cannot remove an exception bound to task %q",
					x.Plan, strings.Join(info.Modules, "/"), de.PassBTask)
			} else {
				if x.RemoveAt != info.Barrier {
					emit("exception-barrier", x.From,
						"remove_at %q does not match plan %q phase barrier %q",
						x.RemoveAt, x.Plan, info.Barrier)
				}
				if x.RemoveAt == "b5" && info.Barrier != "b5" {
					emit("exception-b5-escape", x.From,
						"exception deferred to b5 while plan %q can remove it at %q",
						x.Plan, info.Barrier)
				}
			}
		}
		if !goFileSet[x.From] {
			emit("exception-importer-missing", x.From, "importer file does not exist on disk")
		} else if imports, ok := d.Imports[x.From]; !ok || !containsExact(imports, x.To) {
			emit("exception-import-absent", x.From, "import %q no longer present in file", x.To)
		}
	}
	for key := range discovered {
		if _, owned := ledger[key]; !owned {
			emit("exception-missing", key.from,
				"no exception-ledger entry for %s → %s registered in %s",
				key.from, key.to, architectureGuardCheckFile)
		}
	}

	return sortDiagnosticsDiag(ds)
}

func containsModule(modules []string, want string) bool {
	for _, m := range modules {
		if m == want {
			return true
		}
	}
	return false
}

func containsExact(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// ---- B0.4：能力/组合契约校验 ----

// contractSymbolKinds 是以 file:Name 定位源码符号的契约 kind；
// 其余 kind（route-set/worker-set/lifecycle-set/module-construction）是组合面记录。
var contractSymbolKinds = map[string]bool{
	"capability-port": true,
	"wire-protocol":   true,
	"data-ownership":  true,
}

// moduleFaçadeOps 是 module-construction 契约可冻结的门面操作全集
// （freeze B0.4 Step 6：缺席必须显式记录，不得推断）。
var moduleFacadeOps = []string{"NewModule", "RegisterRoutes", "RegisterWorkers", "Start", "Stop"}

var (
	// entryFileRE 从 manifest routes/lifecycle_hooks 条目提取引用的 .go 文件
	// （形如 "— internal/router/routes_x.go:43"，行号可缺省）。引用必须带至少
	// 一个目录段（OCR R1 #f4）：条目括号内的交叉引用（"(func at container.go:2405)"）
	// 是裸文件名，不得算作消费方。
	entryFileRE = regexp.MustCompile(`((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+\.go)(:\d+)?`)
	// façade 注释提取：门面操作行与「当前 N 项」计数句式（module.go 包注释）。
	facadeOpRE = regexp.MustCompile(
		`(?m)^//\t(?:\(m \*Module\) )?(NewModule|RegisterRoutes|RegisterWorkers|Start|Stop)\(`)
	facadeRoutesRE  = regexp.MustCompile(`当前 (\d+) 项入口`)
	facadeWorkersRE = regexp.MustCompile(`当前 (\d+) 项，见 integration_points\.workers`)
	facadeHooksRE   = regexp.MustCompile(`当前 (\d+) 项生命周期挂点`)
)

// moduleImportBase 用于消费方跨模块导入检查的 import 路径前缀
// （与 architectureguard 同源常量）。
const moduleImportBase = repoModulePath + "/internal/modules/"

// compositionBaseline 是从 Pass A 验收台账解析出的组合基线计数（F5：期望值参数化）。
type compositionBaseline struct {
	RoutesTotal, RoutesLiteral, RoutesAPIKey int
	WorkerTypes, WorkerPools                 int
	WorkerRedis, WorkerLite                  int
	Hooks                                    int
}

// parseCompositionBaseline 从 pass-a-acceptance.md 的基线比对表解析计数。
func parseCompositionBaseline(path string) (compositionBaseline, error) {
	var b compositionBaseline
	data, err := os.ReadFile(path)
	if err != nil {
		return b, fmt.Errorf("read acceptance ledger: %w", err)
	}
	text := string(data)
	m := regexp.MustCompile(`路由注册 \| (\d+)（(\d+) literal \+ (\d+) apiKeyRoute）`).FindStringSubmatch(text)
	if m == nil {
		return b, fmt.Errorf("acceptance ledger missing 路由注册 baseline row")
	}
	b.RoutesTotal, b.RoutesLiteral, b.RoutesAPIKey = atoiSafe(m[1]), atoiSafe(m[2]), atoiSafe(m[3])
	// 台账 worker 行形态「N 任务类型 + M 池 / K」：N 是 Redis 任务类型数、
	// M 是池数、K 是 Lite 任务类型数（worker-parity 下 N==K）。三个数字
	// 各有捕获组（OCR R1 #f5：池数漂移不得静默丢失）。
	m = regexp.MustCompile(`Redis/Lite worker \| (\d+) 任务类型 \+ (\d+) 池 / (\d+)`).FindStringSubmatch(text)
	if m == nil {
		return b, fmt.Errorf("acceptance ledger missing Redis/Lite worker baseline row")
	}
	b.WorkerTypes, b.WorkerPools = atoiSafe(m[1]), atoiSafe(m[2])
	b.WorkerRedis, b.WorkerLite = atoiSafe(m[1]), atoiSafe(m[3])
	m = regexp.MustCompile(`container\.Invoke hooks \| (\d+)`).FindStringSubmatch(text)
	if m == nil {
		return b, fmt.Errorf("acceptance ledger missing container.Invoke hooks baseline row")
	}
	b.Hooks = atoiSafe(m[1])
	return b, nil
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// splitContractSymbol 拆解 "file:Name" 形态的契约符号地址。
func splitContractSymbol(symbol string) (file, name string, err error) {
	i := strings.LastIndex(symbol, ":")
	if i < 0 || i == len(symbol)-1 {
		return "", "", fmt.Errorf("symbol %q must be file:Name", symbol)
	}
	return symbol[:i], symbol[i+1:], nil
}

// CheckContracts 把 contracts.yaml 的冻结记录与仓库发现对照，报告全部
// 符号/签名/消费方/组合面/基线漂移诊断。返回值已排序去重；空切片即通过。
func CheckContracts(g *Governance, d *Discovery) []Diagnostic {
	var ds []Diagnostic
	emit := func(check, path, format string, args ...any) {
		ds = append(ds, Diagnostic{Check: check, Path: path, Message: fmt.Sprintf(format, args...)})
	}

	goFileSet := make(map[string]bool, len(d.GoFiles))
	for _, p := range d.GoFiles {
		goFileSet[p] = true
	}

	// ---- 组合基线：三行 composition.baseline-* 必须在册且与台账一致 ----
	// 台账不可解析时只报单条 contract-baseline-ledger 并跳过三行对照
	// （OCR R1 #04）：零值 led 的对照会产生「disagrees with pass-a ledger 0」
	// 的 drift/missing 级联噪声，误导排障方向。
	acceptanceLedgerRel := filepath.FromSlash("docs/architecture/evidence/pass-a-acceptance.md")
	led, ledErr := parseCompositionBaseline(filepath.Join(d.Root, acceptanceLedgerRel))
	if ledErr != nil {
		emit("contract-baseline-ledger", "", "cannot parameterize baseline counts: %v", ledErr)
	} else {
		baselineWant := map[string]map[string]int{
			"composition.baseline-routes": {
				"routes_total":   led.RoutesTotal,
				"routes_literal": led.RoutesLiteral,
				"routes_apikey":  led.RoutesAPIKey,
			},
			"composition.baseline-workers": {
				"worker_types": led.WorkerTypes,
				"worker_pools": led.WorkerPools,
				"worker_redis": led.WorkerRedis,
				"worker_lite":  led.WorkerLite,
			},
			"composition.baseline-lifecycle": {
				"hooks": led.Hooks,
			},
		}
		baselineRows := map[string]Contract{}
		for _, c := range g.Contracts {
			if strings.HasPrefix(string(c.ID), "composition.baseline-") {
				baselineRows[string(c.ID)] = c
			}
		}
		for id, want := range baselineWant {
			row, ok := baselineRows[id]
			if !ok {
				emit("contract-baseline-missing", id, "baseline row required (counts parameterized from pass-a ledger)")
				continue
			}
			got := map[string]string{}
			for _, item := range row.Items {
				kv := strings.SplitN(item, "=", 2)
				if len(kv) != 2 {
					emit("contract-baseline-drift", id, "item %q must be key=value", item)
					continue
				}
				got[kv[0]] = kv[1]
			}
			for key, wantVal := range want {
				gotVal, ok := got[key]
				if !ok {
					emit("contract-baseline-drift", id, "missing %s entry (want %d)", key, wantVal)
					continue
				}
				if gotVal != strconv.Itoa(wantVal) {
					emit("contract-baseline-drift", id, "%s=%s disagrees with pass-a ledger %d", key, gotVal, wantVal)
				}
			}
		}
	}

	// ---- 每个 manifest 模块：module-construction + 三种 set 契约恰一行 ----
	for _, m := range d.Manifests {
		rowsByKind := map[string][]Contract{}
		for _, c := range g.Contracts {
			if c.Owner != m.Module || strings.HasPrefix(string(c.ID), "composition.baseline-") {
				continue
			}
			rowsByKind[c.Kind] = append(rowsByKind[c.Kind], c)
		}
		for _, kind := range []string{"module-construction", "route-set", "worker-set", "lifecycle-set"} {
			switch n := len(rowsByKind[kind]); {
			case n == 0:
				emit("contract-set-missing", "docs/architecture/moves/"+m.Module+".yaml",
					"module %s has no %s contract row", m.Module, kind)
			case n > 1:
				emit("contract-set-duplicate", "docs/architecture/moves/"+m.Module+".yaml",
					"module %s has %d %s contract rows (want exactly 1)", m.Module, n, kind)
			}
		}

		// route-set / worker-set / lifecycle-set：items 与 manifest 精确集合相等。
		wantSets := map[string][]string{
			"route-set":     m.IntegrationPoints.Routes,
			"worker-set":    m.IntegrationPoints.Workers,
			"lifecycle-set": m.IntegrationPoints.LifecycleHooks,
		}
		for kind, rows := range rowsByKind {
			want, isSet := wantSets[kind]
			if !isSet || len(rows) != 1 {
				continue
			}
			row := rows[0]
			if diff := setDiff(row.Items, want); len(diff) > 0 {
				emit("contract-set-drift", string(row.ID),
					"items disagree with manifest integration_points: %s", strings.Join(diff, "; "))
			}
			// set 行消费方 = 注册/装配位点（见各 kind 语义），必须与发现值一致。
			// OCR R2 b0-ocr-r2-2：discovered 只含归一化（Clean/ToSlash）且磁盘
			// 验证过的位点；结构性问题（位点缺失/裸名不可推导）由
			// discoverSetConsumers 显式返回诊断，unrecorded/vanished 比较只在
			// 归一化且磁盘验证过的路径上进行。
			discovered, siteIssues := discoverSetConsumers(d, kind, want)
			for _, is := range siteIssues {
				emit(is.Check, string(row.ID), "%s", is.Message)
			}
			for _, p := range discovered {
				if !containsExact(row.Consumers, p) {
					emit("contract-consumer-unrecorded", string(row.ID),
						"registration file %s consumes this %s but is not recorded", p, kind)
				}
			}
			for _, p := range row.Consumers {
				if !goFileSet[p] {
					// 与符号契约 recorded consumer 的磁盘预检（check.go:608
					// contract-consumer-file-missing）同构：文件不存在时由
					// missing 层捕获，vanished 层不越界误报。
					emit("contract-consumer-file-missing", string(row.ID),
						"recorded consumer %s does not exist on disk", p)
					continue
				}
				if !containsExact(discovered, p) {
					emit("contract-consumer-vanished", string(row.ID),
						"recorded consumer %s no longer registers this %s", p, kind)
				}
			}
		}

		// module-construction：门面操作集合与 module.go 注释声明的形态一致，
		// 注释计数不得与 manifest 冻结登记数矛盾（freeze B0.4 文件清单）。
		for _, row := range rowsByKind["module-construction"] {
			moduleGo := "internal/modules/" + m.Module + "/module.go"
			if row.Symbol != moduleGo {
				emit("contract-facade-symbol-drift", string(row.ID),
					"symbol %q must be %s", row.Symbol, moduleGo)
			}
			data, err := os.ReadFile(filepath.Join(d.Root, filepath.FromSlash(moduleGo)))
			if err != nil {
				emit("contract-facade-file-missing", string(row.ID), "cannot read %s: %v", moduleGo, err)
				continue
			}
			text := string(data)
			declared := map[string]bool{}
			for _, mm := range facadeOpRE.FindAllStringSubmatch(text, -1) {
				declared[mm[1]] = true
			}
			for _, op := range moduleFacadeOps {
				inItems, inDeclared := containsExact(row.Items, op), declared[op]
				if inItems != inDeclared {
					emit("contract-facade-shape-drift", string(row.ID),
						"façade operation %s: frozen=%v but module.go declares %v "+
							"(absence must be explicit, not inferred)", op, inItems, inDeclared)
				}
			}
			for _, cc := range []struct {
				kind  string
				re    *regexp.Regexp
				count int
			}{
				{"routes", facadeRoutesRE, len(m.IntegrationPoints.Routes)},
				{"workers", facadeWorkersRE, len(m.IntegrationPoints.Workers)},
				{"lifecycle_hooks", facadeHooksRE, len(m.IntegrationPoints.LifecycleHooks)},
			} {
				mm := cc.re.FindStringSubmatch(text)
				if mm == nil {
					emit("contract-facade-count-drift", string(row.ID),
						"module.go facade comment lacks %s count sentence", cc.kind)
					continue
				}
				if got := atoiSafe(mm[1]); got != cc.count {
					emit("contract-facade-count-drift", string(row.ID),
						"module.go claims %d %s but manifest freezes %d", got, cc.kind, cc.count)
				}
			}
		}
	}

	// ---- 符号契约：符号存在、签名无漂移、消费方全量且无禁用导入 ----
	for _, c := range g.Contracts {
		if !contractSymbolKinds[c.Kind] {
			continue
		}
		file, name, err := splitContractSymbol(c.Symbol)
		if err != nil {
			emit("contract-symbol-missing", string(c.ID), "%v", err)
			continue
		}
		if !goFileSet[file] {
			emit("contract-symbol-missing", string(c.ID), "symbol file %s does not exist on disk", file)
			continue
		}
		fact, found, err := DiscoverSymbol(d.Root, file, name)
		if err != nil {
			emit("contract-symbol-missing", string(c.ID), "cannot parse %s: %v", file, err)
			continue
		}
		if !found {
			emit("contract-symbol-missing", string(c.ID), "%s no longer declares %s", file, name)
			continue
		}
		if fact.Signature != c.Signature {
			emit("contract-signature-drift", string(c.ID),
				"frozen signature %q != current %q (contract revision required)", c.Signature, fact.Signature)
		}
		if fact.DeclKind == "method" {
			// OCR R2 f5：契约符号按顶层语义校验——方法回退命中意味着顶层声明
			// 已消失（只剩同名方法）。方法引用形态 recv.Name() 的 SelectorExpr.X
			// 是变量而非定义包限定名，DiscoverSymbolConsumers 的限定名匹配对
			// 跨包消费方全部不命中 → consumers 恒空 → 已登记消费方全量误报
			// vanished。故报 no longer declares（签名对照已在上方保留），
			// 跳过消费方发现与后续消费方面校验。
			emit("contract-symbol-missing", string(c.ID),
				"%s no longer declares top-level %s (only a same-named method remains)", file, name)
			continue
		}
		consumers, characterization, err := DiscoverSymbolConsumers(d, fact)
		if err != nil {
			emit("contract-symbol-missing", string(c.ID), "consumer discovery failed: %v", err)
			continue
		}
		for _, p := range consumers {
			if !containsExact(c.Consumers, p) {
				emit("contract-consumer-unrecorded", string(c.ID),
					"production consumer %s references %s but is not recorded", p, name)
			}
		}
		for _, p := range c.Consumers {
			if !goFileSet[p] {
				emit("contract-consumer-file-missing", string(c.ID), "recorded consumer %s does not exist on disk", p)
				continue
			}
			if !containsExact(consumers, p) {
				emit("contract-consumer-vanished", string(c.ID),
					"recorded consumer %s no longer references %s", p, name)
			}
		}
		// OCR R1 ocr-r1-1：与消费方 vanished 同构的符号引用层。此前生产调用点
		// 丢弃 DiscoverSymbolConsumers 返回的 tests，登记特征化测试只剩文件
		// 存在一层校验——改名/被删尚有 missing 兜底，「不再引用符号」完全无感。
		// unrecorded 反方向（树上有测试引用符号但未登记）不在此强制。
		charTestSet := make(map[string]bool, len(characterization))
		for _, p := range characterization {
			charTestSet[p] = true
		}
		for _, p := range c.CharacterizationTests {
			if !goFileSet[p] {
				emit("contract-characterization-missing", string(c.ID),
					"characterization test %s does not exist on disk", p)
				continue
			}
			if !charTestSet[p] {
				emit("contract-characterization-drift", string(c.ID),
					"recorded test %s no longer references %s", p, name)
			}
		}

		// 消费方禁用导入：/adapters 子包与其他模块非公开子包（在册例外除外）。
		scanConsumerImports(g, c, consumers, d, emit)
	}

	// set 行消费方同样受禁用导入约束。
	for _, c := range g.Contracts {
		switch c.Kind {
		case "route-set", "worker-set", "lifecycle-set":
			if strings.HasPrefix(string(c.ID), "composition.baseline-") {
				continue
			}
			scanConsumerImports(g, c, c.Consumers, d, emit)
		}
	}

	return sortDiagnosticsDiag(ds)
}

// ---- B0.5/OCR R1 #15：事件目录漂移守卫 ----

// eventTransportsFrozen 是 B0 冻结的投递形态词汇：B0 不新增 broker，durable
// 仅限已存在的事件事实表，in_process 覆盖进程内总线/回调路径。
var eventTransportsFrozen = map[string]bool{"in_process": true, "durable": true}

// eventReplayFrozen 是 B0 冻结的重放规则：投影一律从权威源表重建。
const eventReplayFrozen = "source-query"

// eventVersionFrozen 是 B0 冻结的事件版本：只登记 version-1 记录。
const eventVersionFrozen = 1

// CheckEvents 把 event-catalog.yaml 的冻结事件与仓库发现对照（OCR R1 #15）：
// 目录非空（整份缺失按空集加载，必须显式报错而非静默通过）、producer 是
// file:Symbol、文件在发现的 Go 树上（OCR R2 f2：与契约符号侧同构的 goFileSet
// 预检——testdata/点前缀目录被 discoverGoTree 剪枝，不得作为锚点）且符号
// 真实声明于该文件（DiscoverSymbol，含方法回退——真实目录的 producer
// 绝大多数是带接收者的方法）、consumers 全部存在于当前 Go 树、
// transport/replay/version 遵守 B0 冻结词汇。
// 返回值已排序去重；空切片即通过。
func CheckEvents(g *Governance, d *Discovery) []Diagnostic {
	var ds []Diagnostic
	emit := func(check, path, format string, args ...any) {
		ds = append(ds, Diagnostic{Check: check, Path: path, Message: fmt.Sprintf(format, args...)})
	}

	if len(g.Events) == 0 {
		emit("event-catalog-empty", "",
			"event-catalog.yaml has no frozen events (B0.5 catalog is required, not optional)")
		return sortDiagnosticsDiag(ds)
	}

	goFileSet := make(map[string]bool, len(d.GoFiles))
	for _, p := range d.GoFiles {
		goFileSet[p] = true
	}

	for _, e := range g.Events {
		file, sym, err := splitContractSymbol(e.Producer)
		if err != nil {
			emit("event-producer-missing", string(e.ID), "producer %q: %v", e.Producer, err)
		} else if !goFileSet[file] {
			// OCR R2 f2：与契约符号侧的 goFileSet 文件预检同构。DiscoverSymbol
			// 只 os.Stat+ParseFile，不经 discoverGoTree 剪枝——testdata/点前缀
			// 目录下的文件磁盘存在且可解析（甚至声明同名符号），但不在 Go 树上，
			// producer 不得锚定树外文件。
			emit("event-producer-missing", string(e.ID),
				"producer file %s is not in the discovered Go tree "+
					"(dot-prefixed and testdata directories are pruned)", file)
		} else {
			_, found, err := DiscoverSymbol(d.Root, file, sym)
			if err != nil {
				emit("event-producer-missing", string(e.ID),
					"producer symbol %s: %v", e.Producer, err)
			} else if !found {
				emit("event-producer-missing", string(e.ID),
					"producer file %s no longer declares %s (event revision required)", file, sym)
			}
		}
		for _, p := range e.Consumers {
			if !goFileSet[p] {
				emit("event-consumer-missing", string(e.ID),
					"consumer %s does not exist on disk", p)
			}
		}
		if !eventTransportsFrozen[e.Transport] {
			emit("event-transport-frozen", string(e.ID),
				"transport %q must be one of {in_process, durable} (B0 adds no broker)", e.Transport)
		}
		if e.Replay != eventReplayFrozen {
			emit("event-replay-frozen", string(e.ID),
				"replay %q must be %q (projections rebuild from the authoritative source)",
				e.Replay, eventReplayFrozen)
		}
		if e.Version != eventVersionFrozen {
			emit("event-version-frozen", string(e.ID),
				"version %d beyond B0 freeze (only version-1 records; revisions go through contract review)",
				e.Version)
		}
	}
	return sortDiagnosticsDiag(ds)
}

// scanConsumerImports 检查一组消费方文件的导入面：
//   - 任何消费方都禁止导入其他模块的 /adapters 子包（同模块内部装配除外；
//     当前仓库零出现，本规则纯防 B1+ 新漂移）；
//   - 模块代码消费方（internal/modules/<X>/ 下）禁止导入其他模块非公开子包
//     （与 architectureguard forbidden-import 同口径；exception-ledger 在册对
//     除外——那是已有删除属主的已知债务，不是新漂移）。平台 legacy 文件导入
//     模块子包是 Pass A 已接受状态（guard 只扫 modules 目录），不在此报告。
func scanConsumerImports(
	g *Governance, c Contract, consumers []string, d *Discovery,
	emit func(check, path, format string, args ...any),
) {
	excepted := map[string]bool{}
	for _, x := range g.Exceptions {
		excepted[x.From+"→"+x.To] = true
	}
	for _, p := range consumers {
		imports := d.Imports[p]
		if imports == nil && !containsExact(d.GoFiles, p) {
			continue // 不在仓库树上的文件由其它 check 报告
		}
		for _, imp := range imports {
			if !strings.HasPrefix(imp, moduleImportBase) {
				continue
			}
			target := strings.TrimPrefix(imp, moduleImportBase)
			targetModule := target
			sub := ""
			if i := strings.Index(target, "/"); i >= 0 {
				targetModule, sub = target[:i], target[i+1:]
			}
			if sub == "" {
				continue // 模块根公共门面：合法
			}
			consumerModule := moduleOfPath(p)
			if consumerModule == targetModule {
				continue // 本模块内部子包
			}
			if strings.HasPrefix(sub, "adapters/") || sub == "adapters" {
				emit("contract-consumer-adapters-import", p,
					"consumer of %s imports non-public adapters subpackage %q "+
						"(route via module root façade or export a narrow port)", string(c.ID), imp)
				continue
			}
			if consumerModule != "" && !excepted[p+"→"+imp] {
				emit("contract-consumer-module-import", p,
					"consumer of %s imports module %s non-public subpackage %q without a ledgered exception",
					string(c.ID), targetModule, imp)
			}
		}
	}
}

// moduleOfPath 返回 internal/modules/<mod>/ 下文件的属主模块 id（否则空串）。
func moduleOfPath(p string) string {
	const prefix = "internal/modules/"
	if !strings.HasPrefix(p, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(p, prefix)
	if i := strings.Index(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// workerRegisterRootPrefix 是 B0 阶段 worker handler 注册根：B0 的任务注册
// 全部位于 internal/router/{task,sync_task}.go（17 条 worker-set 契约的登记
// consumers 去重后只有这 3 个 router 文件），裸标识符条目以该根为扫描范围。
const workerRegisterRootPrefix = "internal/router/"

// setSiteIssue 是 set 条目位点推导的结构性问题：discoverSetConsumers 内部
// 无法入集也不得静默跳过时返回，由调用方以契约行为 path emit。
type setSiteIssue struct {
	Check   string
	Message string
}

// setSiteCollector 收集归一化且磁盘验证过的 set 注册位点（OCR R2
// b0-ocr-r2-2）：三种 set kind 的位点入集共用同一规则——
// filepath.ToSlash(filepath.Clean(…)) 归一化后对照由 d.GoFiles 重建的磁盘
// Go 树（仓库相对 slash 路径，见 discoverGoTree）验证。
type setSiteCollector struct {
	goFileSet map[string]bool
	seen      map[string]bool
	files     []string
	issues    []setSiteIssue
}

func newSetSiteCollector(d *Discovery) *setSiteCollector {
	goFileSet := make(map[string]bool, len(d.GoFiles))
	for _, p := range d.GoFiles {
		goFileSet[p] = true
	}
	return &setSiteCollector{goFileSet: goFileSet, seen: map[string]bool{}}
}

// add 归一化并磁盘验证一个 entryFileRE 位点：合法则去重入集；不在磁盘上时
// 追加 contract-consumer-file-missing issue（与符号契约 recorded consumer
// 磁盘预检同构），不静默入集也不静默跳过。
func (c *setSiteCollector) add(site, entry string) {
	norm := filepath.ToSlash(filepath.Clean(site))
	if !c.goFileSet[norm] {
		c.issues = append(c.issues, setSiteIssue{
			Check: "contract-consumer-file-missing",
			Message: fmt.Sprintf(
				"registration site %s (from entry %q) does not exist on disk", norm, entry),
		})
		return
	}
	if !c.seen[norm] {
		c.seen[norm] = true
		c.files = append(c.files, norm)
	}
}

// addVerified 把已归一化且已知在磁盘上的发现文件（如回退扫描命中的
// d.GoFiles 成员）去重入集。
func (c *setSiteCollector) addVerified(file string) {
	if !c.seen[file] {
		c.seen[file] = true
		c.files = append(c.files, file)
	}
}

// sorted 返回去重排序后的位点集（只含归一化且磁盘验证过的路径）。
func (c *setSiteCollector) sorted() []string {
	out := append([]string(nil), c.files...)
	sort.Strings(out)
	return out
}

// discoverSetConsumers 发现组合面 set 的注册/装配位点：
//   - route-set / lifecycle-set：manifest 条目中引用的 .go 文件（去重排序）；
//   - worker-set：见下方 OCR R1 #b0-ocr-r1-7 义务说明——条目带注册位点后缀
//     时从条目推导；裸标识符条目（B0 形态）回退扫描 workerRegisterRootPrefix
//     下引用任一任务类型标识符的非测试文件。
//
// OCR R2 b0-ocr-r2-2：位点入集规则见 setSiteCollector——manifest 手写位点
// 的点段形态（"./x"、"x/./y.go"）归一化后照常入集，不在磁盘上的位点显式
// 返回 file-missing issue。返回的位点集只含归一化且磁盘验证过的路径，
// unrecorded/vanished 比较因此只在同一形态上进行。
func discoverSetConsumers(d *Discovery, kind string, items []string) ([]string, []setSiteIssue) {
	c := newSetSiteCollector(d)
	switch kind {
	case "route-set", "lifecycle-set":
		for _, entry := range items {
			for _, m := range entryFileRE.FindAllStringSubmatch(entry, -1) {
				c.add(m[1], entry)
			}
		}
		return c.sorted(), c.issues
	case "worker-set":
		// OCR R1 #b0-ocr-r1-7（B1+ 演进义务）：worker-set 消费方 = 任务
		// handler 注册位点。B0 口径正确的前提有二：RegisterWorkers 在 16 个
		// module.go 中均为注释形态（无模块树内注册位点），注册全在
		// internal/router；且树内/树外的 enqueue 生产方引用
		// （internal/application/service/*、internal/datasource/connector/moauth/scheduler.go 等）
		// 与注册位点无法用"标识符被引用"区分，只能靠已知注册根圈定。
		//
		// B1+ 落地 RegisterWorkers 到模块树时，必须同步把 manifest
		// integration_points.workers 条目更新为带注册位点后缀的形态
		// （"TypeX — internal/modules/<m>/workers.go:NN"，与 routes/
		// lifecycle_hooks 条目同构），本分支随即从条目推导位点。否则：
		//   - 只改代码不改 manifest：模块树内新注册位点静默漏报
		//     （contract-consumer-unrecorded 永不触发）；
		//   - router 旧注册删除而契约 consumers 未更新：已登记 consumers
		//     批量假 vanished（contract-consumer-vanished 误报）。
		var idents []string // 回退扫描的标识符集合
		for _, entry := range items {
			sites := entryFileRE.FindAllStringSubmatch(entry, -1)
			// OCR R2 b0-ocr-r2-1：裸名推导与位点提取共用 entryFileRE 这一
			// 单一事实源——以首个匹配索引为锚取位点前缀，剔除尾部非标识符
			// 字符（" — "/"–"/"——"/"—"/" - " 等任意分隔符形态一律适用），
			// 不再依赖 strings.Cut 的 " — " 字面量（此前分隔符漂移时完整
			// 条目串进扫描集合，fileReferencesIdent 只匹配 ast.Ident 名，
			// 永不命中 → 回退扫描静默失效）。位点命中但裸名不可推导的矛盾
			// 形态（锚前缀为空或非标识符，如 "— internal/x.go:1" 或
			// "TypeFoo-internal/…" 被正则整体吞并）显式 entry-drift 诊断。
			if loc := entryFileRE.FindStringIndex(entry); loc == nil {
				idents = append(idents, entry) // B0 裸标识符形态
			} else {
				bare := trimTrailingNonIdent(entry[:loc[0]])
				if !isGoIdent(bare) {
					c.issues = append(c.issues, setSiteIssue{
						Check: "contract-set-entry-drift",
						Message: fmt.Sprintf(
							"worker-set entry %q references registration site %q but its bare "+
								"task type cannot be derived from prefix %q (want a Go identifier)",
							entry, sites[0][1], bare),
					})
				} else {
					// 条目已带位点后缀：裸名也进扫描集合——迁移中间态
					// （新位点已登记、router 旧注册未删）两个真实引用
					// 位点都必须被发现。
					idents = append(idents, bare)
				}
			}
			for _, m := range sites {
				c.add(m[1], entry)
			}
		}
		// 2) 裸标识符回退：扫描 B0 注册根下引用任一标识符的非测试文件。
		for _, f := range d.GoFiles {
			if !strings.HasPrefix(f, workerRegisterRootPrefix) || strings.HasSuffix(f, "_test.go") {
				continue
			}
			relevant := false
			for _, taskType := range idents {
				if taskType == "" {
					continue
				}
				hits, err := fileReferencesIdent(d, f, taskType)
				if err != nil {
					// OCR R1 #b0-ocr-r1-fileReferencesIdent-swallow-error：
					// parse 失败与 ident 无关，收敛为显式诊断（经
					// check.go 500-503 既有 issue 通道以契约行为 path emit），
					// 该文件不得进 discovered 集（否则 unrecorded/vanished
					// 比较建立在未知基础上）。
					c.issues = append(c.issues, setSiteIssue{
						Check: "contract-set-scan-failed",
						Message: fmt.Sprintf(
							"worker-set fallback scan cannot parse %s: %v", f, err),
					})
					break
				}
				if hits {
					relevant = true
					break
				}
			}
			if relevant {
				c.addVerified(f) // f 本身来自 d.GoFiles：已归一化且在磁盘
			}
		}
		return c.sorted(), c.issues
	}
	return nil, c.issues
}

// isIdentRune 报告 r 是否可出现在 Go 标识符中（unicode letter / digit / '_'）。
func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// trimTrailingNonIdent 剔除 s 尾部所有非 Go 标识符字符（含 " — "/"–"/"——"
// 等多字节分隔符），用于从 entryFileRE 位点锚前缀推导裸任务类型名。
func trimTrailingNonIdent(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if isIdentRune(r) {
			break
		}
		s = s[:len(s)-size]
	}
	return s
}

// isGoIdent 报告 s 是否为完整合法的 Go 标识符（首字符 letter/'_'，其余
// letter/digit/'_'）——裸任务类型名必须能作为 ast.Ident 名被回退扫描命中。
func isGoIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if !isIdentRune(r) {
			return false
		}
	}
	return true
}

// fileReferencesIdent 判断文件 AST 内任意位置（含限定名 Sel）出现同名 Ident。
// parseFileFull 错误原样上抛（OCR R1 #b0-ocr-r1-fileReferencesIdent-swallow-error）：
// 与 DiscoverSymbolConsumers 的 parse 错误上抛口径一致，绝不静默 false——
// 静默会让真实注册位点从 discovered 集丢失而守卫毫无感知。
func fileReferencesIdent(d *Discovery, file, name string) (bool, error) {
	c, err := d.parseFileFull(file)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(c.file, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
			return false
		}
		return true
	})
	return found, nil
}

// setDiff 返回两个集合的单向差描述（仅出现在其中一个集合的元素）。
func setDiff(a, b []string) []string {
	count := map[string]int{}
	for _, x := range a {
		count[x]++
	}
	for _, x := range b {
		count[x]--
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		if count[k] > 0 {
			out = append(out, fmt.Sprintf("only-in-contract: %q", k))
		} else if count[k] < 0 {
			out = append(out, fmt.Sprintf("only-in-manifest: %q", k))
		}
	}
	return out
}
