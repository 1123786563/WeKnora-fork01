package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Diagnostic 是一条守卫违规（排序后输出）。
type Diagnostic struct {
	Check   string
	Message string
}

// String 输出形如 "architectureguard: <check>: <message>"。
func (d Diagnostic) String() string {
	return fmt.Sprintf("architectureguard: %s: %s", d.Check, d.Message)
}

// SortDiagnostics 按 String 排序并去重。
func SortDiagnostics(ds []Diagnostic) []Diagnostic {
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

// Summary 汇总发现规模（与 F0 基线 §3.2 口径对照用）。
type Summary struct {
	RoutesLiteral  int
	RoutesAPIKey   int
	RoutesHandle   int
	RoutesTotal    int
	WorkerRedis    int
	WorkerLite     int
	Hooks          int
	ModulesScanned int
}

// ManifestView 是 architectureguard 需要的 manifest 只读子集
// （与 tools/modulemove 的 LOCKED schema 对齐；guard 不 import main 包，故最小重复）。
type ManifestView struct {
	Module               string
	MoveFroms            []string
	LegacyPaths          []string
	RouteEntries         []string // 原文（"RegisterX — file:line" 形态）
	WorkerTypes          []string
	LifecycleHookEntries []string // 原文（自由文本，识别符形态的才做强校验）
}

// platformRouteFiles 是 moves/README.md 记录的 platform 路由残留文件
// （/health、/swagger/*any、静态前端、files 授权/预签名、workbench HMAC 挂载点）。
var platformRouteFiles = map[string]bool{
	"internal/router/router.go": true,
	"internal/router/static.go": true,
	"internal/router/files.go":  true,
}

// platformHooks 是 moves/README.md 记录的 platform 生命周期/worker 启动残留。
var platformHooks = map[string]bool{
	"registerLangfuseCleanup": true,
	"registerPoolCleanup":     true,
	"RunAsynqServer":          true,
	"RegisterSyncHandlers":    true,
}

// platformLegacyFiles 是横向目录内归 platform 本体的 3 个文件（moves/README.md）。
var platformLegacyFiles = map[string]bool{
	"internal/handler/list_pagination.go":           true,
	"internal/handler/upload_limit.go":              true,
	"internal/application/repository/task_queue.go": true,
}

// platformPackageDirs 是 moves/README.md platform 包清单中位于横向目录下的整包豁免
// （internal/handler/dto、internal/application/service/file：platform 本体包，Pass B 处理）。
var platformPackageDirs = []string{
	"internal/handler/dto/",
	"internal/application/service/file/",
}

// importException 是一条精确路径（exact-path）的跨模块 import 豁免条目
// （规格 §15：例外必须精确路径、写明原因、指明 Pass B 删除任务；禁用通配）。
type importException struct {
	ImporterFile string // 导入方文件的仓库相对精确路径（slash 分隔）
	ImportedPath string // 被导入包的精确 import 路径
	Reason       string // 一行原因
	PassBTask    string // Pass B 删除任务 id
}

// importExceptions 是 Pass A 期间允许的跨模块 import 例外清单（与
// platformPackageDirs 同类的代码常量先例；对应 Pass B 任务落地时逐条删除）。
// 匹配规则：importer 文件路径与被导入包路径都必须完全相等——不做 glob、
// 前缀或子串匹配；一条豁免只压制该精确 file→package 对的 forbidden-import。
var importExceptions = []importException{
	{
		ImporterFile: "internal/modules/appconnector/service/appconnector/oc_recovery.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial",
		Reason:       "预存横向包耦合（Pass A 前双方均在 internal/application/service 下，oc_recovery 直接消费 commercial service），Pass A 不改边界",
		PassBTask:    "B-appconnector",
	},
	{
		ImporterFile: "internal/modules/appconnector/adapter.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/modules/commercial",
		Reason:       "预存横向包耦合（Pass A 前 appconnector 消费 internal/commercial 根包，A2 import 修复改写为模块路径），Pass A 不改边界",
		PassBTask:    "B-appconnector",
	},
	{
		ImporterFile: "internal/modules/appconnector/service/appconnector/action.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/modules/commercial",
		Reason:       "预存横向包耦合（Pass A 前 appconnector 消费 internal/commercial 根包，A2 import 修复改写为模块路径），Pass A 不改边界",
		PassBTask:    "B-appconnector",
	},
	{
		ImporterFile: "internal/modules/appconnector/service/appconnector/oc_recovery.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/modules/commercial",
		Reason:       "预存横向包耦合（Pass A 前 appconnector 消费 internal/commercial 根包，A2 import 修复改写为模块路径），Pass A 不改边界",
		PassBTask:    "B-appconnector",
	},
}

// importExcepted 报告 (importerFile, importedPath) 是否命中一条精确豁免。
func importExcepted(importerFile, importedPath string) bool {
	for _, e := range importExceptions {
		if e.ImporterFile == importerFile && e.ImportedPath == importedPath {
			return true
		}
	}
	return false
}

// horizontalDirs 是 Pass B 之前仍承载多模块遗留文件的水平业务目录。
var horizontalDirs = []string{
	"internal/application/service",
	"internal/application/repository",
	"internal/handler",
}

// moduleImportBase 用于模块间禁互导检查的 import 路径前缀。
const moduleImportBase = "github.com/Tencent/WeKnora/internal/modules/"

// hookIdentRE 匹配识别符形态的 hook 条目名（普通或点分）。
var hookIdentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// manifestFile 严格解码用的 schema 镜像（KnownFields(true) 要求字段与 LOCKED
// schema 完全一致——子集会在未知字段上报错；guard 不 import main 包，故最小重复）。
type manifestFile struct {
	Module       string `yaml:"module"`
	Description  string `yaml:"description"`
	MovePackages []struct {
		From string `yaml:"from"`
		To   string `yaml:"to"`
	} `yaml:"move_packages"`
	AliasObligations []struct {
		OldImportPath string `yaml:"old_import_path"`
		PassBTask     string `yaml:"passb_task"`
	} `yaml:"alias_obligations"`
	LegacyFiles []struct {
		Path            string `yaml:"path"`
		Reason          string `yaml:"reason"`
		NavigationLabel string `yaml:"navigation_label"`
		PassBTask       string `yaml:"passb_task"`
	} `yaml:"legacy_files"`
	OwnedFiles struct {
		MoveSources []string `yaml:"move_sources"`
		MoveTargets []string `yaml:"move_targets"`
		Importers   []string `yaml:"importers"`
		ModuleFiles []string `yaml:"module_files"`
	} `yaml:"owned_files"`
	TestCommands      []string `yaml:"test_commands"`
	IntegrationPoints struct {
		Routes         []string `yaml:"routes"`
		Workers        []string `yaml:"workers"`
		LifecycleHooks []string `yaml:"lifecycle_hooks"`
	} `yaml:"integration_points"`
	ForbiddenSharedFiles []string `yaml:"forbidden_shared_files"`
}

// LoadManifestViews 加载 manifest 目录下全部清单为守卫所需的只读子集
// （严格 KnownFields(true)，与 tools/modulemove 同一 schema 约束）。
func LoadManifestViews(root string) ([]ManifestView, error) {
	dir := filepath.Join(root, "docs", "architecture", "moves")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifests dir: %w", err)
	}
	var out []ManifestView
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		mf, err := loadManifestFileStrict(path)
		if err != nil {
			return nil, err
		}
		v := ManifestView{Module: mf.Module}
		for _, mp := range mf.MovePackages {
			v.MoveFroms = append(v.MoveFroms, mp.From)
		}
		for _, lf := range mf.LegacyFiles {
			v.LegacyPaths = append(v.LegacyPaths, lf.Path)
		}
		v.RouteEntries = mf.IntegrationPoints.Routes
		v.WorkerTypes = mf.IntegrationPoints.Workers
		v.LifecycleHookEntries = mf.IntegrationPoints.LifecycleHooks
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Module < out[j].Module })
	return out, nil
}

func loadManifestFileStrict(path string) (manifestFile, error) {
	var mf manifestFile
	f, err := os.Open(path)
	if err != nil {
		return mf, err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&mf); err != nil {
		return mf, fmt.Errorf("strict decode %s: %w", path, err)
	}
	return mf, nil
}

// Report 汇总一次守卫运行。
type Report struct {
	Summary     Summary
	Diagnostics []Diagnostic
}

// Run 执行全部守卫检查：所有权覆盖、注册唯一性、Redis/Lite 一致、
// 模块间禁互导、横向目录遗留文件守卫。
func Run(root string, mods []ManifestView) (Report, error) {
	rep := Report{Summary: Summary{ModulesScanned: len(mods)}}
	add := func(check, format string, args ...any) {
		rep.Diagnostics = append(rep.Diagnostics, Diagnostic{Check: check, Message: fmt.Sprintf(format, args...)})
	}

	routes, err := DiscoverRoutes(root)
	if err != nil {
		return rep, err
	}
	redis, lite, err := DiscoverWorkers(root)
	if err != nil {
		return rep, err
	}
	hooks, err := DiscoverHooks(root)
	if err != nil {
		return rep, err
	}
	funcDecls, err := FuncDeclNames(root, routeScanDirs...)
	if err != nil {
		return rep, err
	}

	// ---- 发现规模 ----
	for _, r := range routes {
		switch r.Kind {
		case "literal":
			rep.Summary.RoutesLiteral++
		case "apiKeyRoute":
			rep.Summary.RoutesAPIKey++
		case "handle":
			rep.Summary.RoutesHandle++
		}
	}
	rep.Summary.RoutesTotal = len(routes)
	rep.Summary.WorkerRedis = len(redis)
	rep.Summary.WorkerLite = len(lite)
	rep.Summary.Hooks = len(hooks)

	// ---- manifest 声明的资产索引 ----
	declaredRouteFiles := map[string]bool{}
	declaredWorkers := map[string]bool{} // 短名（TypeX）
	for _, m := range mods {
		for _, entry := range m.RouteEntries {
			if _, file := parseRouteEntry(entry); file != "" {
				declaredRouteFiles[file] = true
			}
		}
		for _, w := range m.WorkerTypes {
			declaredWorkers[w] = true
		}
	}

	// ---- route-entry-coverage（正向）：manifest 声明的入口必须真实存在 ----
	// 条目名称形态多样（"RegisterX"、"RegisterX (handler-side delegation; n routes)"、
	// "xxx groups (inside RegisterY)"、纯描述聚合如 "session 子路由 ×4"）：
	// 在整条文本（名称 + 文件标注）中抽取识别符形态的词，任一命中 router/handler
	// 函数声明即视为可定位；无识别符词、或其文件属于 platform 路由残留的条目
	// 不做机械校验（由 route-file-coverage 的 platform 侧覆盖）。
	for _, m := range mods {
		for _, entry := range m.RouteEntries {
			_, file := parseRouteEntry(entry)
			if platformRouteFiles[file] {
				continue
			}
			idents := entryIdentifiers(entry)
			if len(idents) == 0 {
				continue
			}
			found := false
			for _, id := range idents {
				if funcDecls[id] {
					found = true
					break
				}
			}
			if !found {
				add("route-entry-coverage", "%s 声明的路由入口 %q 未能在 internal/router|internal/handler 定位",
					m.Module, strings.TrimSpace(entry))
			}
		}
	}

	// ---- route-file-coverage（反向）：出现注册的路由文件必须被 manifest 或 platform 残留覆盖 ----
	for _, r := range routes {
		if declaredRouteFiles[r.File] || platformRouteFiles[r.File] {
			continue
		}
		add("route-file-coverage", "路由注册文件 %s 未被任何 manifest routes 条目或 platform 残留覆盖（%s:%d）",
			r.File, r.File, r.Line)
	}

	// ---- worker-coverage（正向）：manifest 声明的任务类型必须已注册 ----
	discoveredWorkerShort := map[string]bool{}
	for _, w := range redis {
		discoveredWorkerShort[workerShortName(w.TaskType)] = true
	}
	for _, w := range lite {
		discoveredWorkerShort[workerShortName(w.TaskType)] = true
	}
	for _, m := range mods {
		for _, w := range m.WorkerTypes {
			if !discoveredWorkerShort[w] {
				add("worker-coverage", "%s 声明的任务类型 %q 未在任何模式（Redis/Lite）注册", m.Module, w)
			}
		}
	}

	// ---- worker-reverse-coverage（反向）：每个注册的任务类型必须归某个 manifest ----
	for _, w := range redis {
		if !declaredWorkers[workerShortName(w.TaskType)] {
			add("worker-reverse-coverage", "任务类型 %s（%s:%d）未被任何 manifest 声明", w.TaskType, w.File, w.Line)
		}
	}
	for _, w := range lite {
		if !declaredWorkers[workerShortName(w.TaskType)] {
			add("worker-reverse-coverage", "任务类型 %s（%s:%d）未被任何 manifest 声明", w.TaskType, w.File, w.Line)
		}
	}

	// ---- worker-parity：Redis 与 Lite 任务类型集合必须一致 ----
	redisSet := map[string]bool{}
	for _, w := range redis {
		redisSet[w.TaskType] = true
	}
	liteSet := map[string]bool{}
	for _, w := range lite {
		liteSet[w.TaskType] = true
	}
	var onlyRedis, onlyLite []string
	for tt := range redisSet {
		if !liteSet[tt] {
			onlyRedis = append(onlyRedis, tt)
		}
	}
	for tt := range liteSet {
		if !redisSet[tt] {
			onlyLite = append(onlyLite, tt)
		}
	}
	sort.Strings(onlyRedis)
	sort.Strings(onlyLite)
	if len(onlyRedis) > 0 || len(onlyLite) > 0 {
		add("worker-parity", "Redis/Lite 任务集合不一致: only-in-redis=%v, only-in-lite=%v", onlyRedis, onlyLite)
	}

	// ---- unique-worker-registration：同一模式内任务类型不得重复 ----
	countDup := func(list []WorkerReg, mode string) {
		seen := map[string]int{}
		for _, w := range list {
			seen[w.TaskType]++
		}
		tts := make([]string, 0, len(seen))
		for tt, n := range seen {
			if n > 1 {
				tts = append(tts, tt)
			}
		}
		sort.Strings(tts)
		for _, tt := range tts {
			add("unique-worker-registration", "任务类型 %s 在 %s 模式注册了 %d 次", tt, mode, seen[tt])
		}
	}
	countDup(redis, "redis")
	countDup(lite, "lite")

	// ---- unique-route-registration：同一 method+静态解析路径不得重复（Key 见 discovery.go）----
	// 例外：同函数内 if/else 互斥回退注册（同一 if 作用域、一个在 body 一个在 else）
	// 运行时只注册其一，不构成重复（如 routes_workbench.go 的 W23 facade 回退）。
	seenRoute := map[string]RouteReg{}
	for _, r := range routes {
		if r.Key == "" {
			continue // 路径实参非字面量：无法参与静态查重
		}
		if prev, dup := seenRoute[r.Key]; dup {
			if prev.BranchScope != 0 && prev.BranchScope == r.BranchScope && prev.InElse != r.InElse {
				continue // if/else 互斥回退
			}
			add("unique-route-registration", "路由 %s %s 重复注册: %s:%d 与 %s:%d",
				r.Method, r.Path, prev.File, prev.Line, r.File, r.Line)
		} else {
			seenRoute[r.Key] = r
		}
	}

	// ---- hook-coverage（正向）：识别符形态的 hook 条目必须存在于代码 ----
	hookNames := map[string]bool{}
	for _, h := range hooks {
		hookNames[h.Name] = true
	}
	containerDecls, err := FuncDeclNames(root, "internal/container")
	if err != nil {
		return rep, err
	}
	for _, m := range mods {
		for _, entry := range m.LifecycleHookEntries {
			name := strings.TrimSpace(strings.Split(entry, " — ")[0])
			if !hookIdentRE.MatchString(name) {
				continue // 自由文本条目（"wire X (inline)"、插件聚合 ×N 等）不做机械校验
			}
			base := name
			if i := strings.LastIndex(name, "."); i >= 0 {
				base = name[i+1:]
			}
			if hookNames[name] || hookNames[base] || containerDecls[name] || containerDecls[base] ||
				platformHooks[name] || platformHooks[base] {
				continue
			}
			add("hook-coverage", "%s 声明的生命周期挂点 %q 未找到（%s）", m.Module, name, entry)
		}
	}

	// ---- forbidden-import：模块之间不得互相 import ----
	modRoot := filepath.Join(root, "internal", "modules")
	werr := filepath.WalkDir(modRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == modRoot {
				return nil // 目录缺失（测试 fixture）视为空集
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		relSlash := filepath.ToSlash(rel)
		owner := moduleOwnerOf(relSlash)
		if owner == "" {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(impPath, moduleImportBase) {
				continue
			}
			target := strings.TrimPrefix(impPath, moduleImportBase)
			if i := strings.Index(target, "/"); i >= 0 {
				target = target[:i]
			}
			if target != owner {
				if importExcepted(relSlash, impPath) {
					continue // §15 精确路径豁免：仅对此 file→package 对放行，Pass B 删除
				}
				add("forbidden-import", "%s 导入了模块 %s 的内部包 %q（跨模块只能经模块根公共门面）",
					relSlash, target, impPath)
			}
		}
		return nil
	})
	if werr != nil {
		return rep, werr
	}

	// ---- legacy-guard：横向目录内每个非测试 .go 必须有归属 ----
	moveFroms := map[string]bool{}
	legacyPaths := map[string]bool{}
	for _, m := range mods {
		for _, f := range m.MoveFroms {
			moveFroms[f+"/"] = true
		}
		for _, p := range m.LegacyPaths {
			legacyPaths[p] = true
		}
	}
	for _, dir := range horizontalDirs {
		absDir := filepath.Join(root, filepath.FromSlash(dir))
		werr := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if path == absDir {
					return nil // 目录缺失（测试 fixture）视为空集
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			if r, rerr := filepath.Rel(root, path); rerr == nil {
				rel = filepath.ToSlash(r)
			}
			if legacyPaths[rel] || platformLegacyFiles[rel] {
				return nil
			}
			for _, pkgDir := range platformPackageDirs {
				if strings.HasPrefix(rel, pkgDir) {
					return nil // platform 包整包豁免
				}
			}
			for from := range moveFroms {
				if strings.HasPrefix(rel, from) {
					return nil // 位于整包搬迁树内的文件随树搬迁
				}
			}
			add("legacy-guard", "横向目录出现无归属的新生产文件 %s（须列入某 manifest 的 legacy_files/move_packages，或属 platform）", rel)
			return nil
		})
		if werr != nil {
			return rep, werr
		}
	}

	rep.Diagnostics = SortDiagnostics(rep.Diagnostics)
	return rep, nil
}

// ---- 小工具 ----

// parseRouteEntry 拆 "Name — file:line (note)" → (Name, file)。
// file 剥离 ":line" 或 ":line-line" 后缀（manifest 用区间标注多处注册）。
func parseRouteEntry(entry string) (name, file string) {
	parts := strings.SplitN(entry, " — ", 2)
	name = strings.TrimSpace(parts[0])
	if len(parts) < 2 {
		return name, ""
	}
	rest := strings.TrimSpace(parts[1])
	tok := rest
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		tok = rest[:i]
	}
	if i := strings.LastIndex(tok, ":"); i > 0 && isLineSuffix(tok[i+1:]) {
		tok = tok[:i]
	}
	return name, tok
}

// isLineSuffix 接受 "18" 或 "116-154" 形态的行号后缀。
func isLineSuffix(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, "-") {
		if part == "" || !isDigits(part) {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// goIdentRE 匹配独立的 Go 识别符（下划线开头/结尾、驼峰函数名等）。
var goIdentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// isIdentRune 报告 r 是否为 Go 识别符字符。
func isIdentRune(r rune) bool {
	isLower := r >= 'a' && r <= 'z'
	isUpper := r >= 'A' && r <= 'Z'
	isDigit := r >= '0' && r <= '9'
	return isLower || isUpper || isDigit || r == '_'
}

// entryIdentifiers 抽取条目全文中识别符形态的词（含括号内引用，如
// "(inside RegisterSessionRoutes)"；跳过中文描述、行号、×N 聚合等非识别符词）。
func entryIdentifiers(entry string) []string {
	words := strings.FieldsFunc(entry, func(r rune) bool {
		return !isIdentRune(r)
	})
	var out []string
	for _, w := range words {
		if goIdentRE.MatchString(w) && !isDigits(string(w[0])) {
			out = append(out, w)
		}
	}
	return out
}

// workerShortName 从 "types.TypeX" / "TypeX" 取短名；空串表示不可解析。
func workerShortName(taskType string) string {
	taskType = strings.TrimSpace(taskType)
	if taskType == "" {
		return ""
	}
	if i := strings.LastIndex(taskType, "."); i >= 0 {
		return taskType[i+1:]
	}
	return taskType
}

// moduleOwnerOf 从 "internal/modules/<owner>/..." 取 owner；非模块文件返回 ""。
func moduleOwnerOf(rel string) string {
	prefix := "internal/modules/"
	if !strings.HasPrefix(rel, prefix) {
		return ""
	}
	rest := rel[len(prefix):]
	if i := strings.Index(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}
