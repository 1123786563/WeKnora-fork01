package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// RouteReg 是一次 HTTP 路由注册的静态发现结果。
// Kind: "literal"（gin 方法字面调用）、"apiKeyRoute"（rbac 助手）、"handle"（直连 .Handle( 且方法为字面量）。
// 与 F0 基线 §3.2 的计数口径一致：internal/router/*.go 与 internal/handler/**/*.go 非测试文件，
// AST 解析天然排除注释文本。
type RouteReg struct {
	File   string // 仓库相对路径
	Line   int
	Func   string // 所属顶层函数名（不在函数体内时为 "<file>"）
	Method string // GET/POST/.../ANY（Any 记为 ANY）
	Path   string // 调用点的字面路径参数（不可解析时为 ""）
	Kind   string
	// Key 是唯一性查重键：method + 静态可解析的组前缀 + 字面路径。
	// 组前缀按 (函数内 var → Group 链字面量) 解析；挂载基座为函数参数（跨函数
	// 不可静态解析）时退化用 "<func>#<var>" 命名空间，避免跨挂载点误报。
	Key string
	// BranchScope / InElse 记录注册点是否位于某个 if 语句的 body/else 分支
	// （同函数内 if/else 互斥回退注册——如 W23 facade 回退——不构成重复注册）。
	BranchScope int  // 所在最内层 if 语句的起始行；不在分支内为 0
	InElse      bool // 是否位于该 if 的 else 分支
}

// WorkerReg 是一次任务处理器注册的静态发现结果。
// Mode: "redis"（internal/router 侧 mux.HandleFunc）或 "lite"（RegisterHandler 侧）。
type WorkerReg struct {
	TaskType string // 首参表达式文本，如 "types.TypeChunkExtract"
	File     string
	Line     int
	Func     string
	Mode     string
}

// HookReg 是一次 container.Invoke(<fn>) 生命周期挂点注册的静态发现结果。
type HookReg struct {
	Name string // 首参表达式文本（如 "registerPoolCleanup" 或 "chatpipeline.NewPluginSearch"）
	File string
	Line int
}

// ginMethodSet 是 F0 基线 §3.2 口径的 gin 路由注册方法（字面调用）。
// Any 记为 ANY（注册全部方法）。
var ginMethodSet = map[string]string{
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE",
	"PATCH": "PATCH", "HEAD": "HEAD", "OPTIONS": "OPTIONS", "Any": "ANY",
}

// httpMethodConsts 镜像 net/http 的 Method* 常量名（http.MethodX → X）。
var httpMethodConsts = map[string]string{
	"MethodGet": "GET", "MethodPost": "POST", "MethodPut": "PUT",
	"MethodDelete": "DELETE", "MethodPatch": "PATCH", "MethodHead": "HEAD",
	"MethodOptions": "OPTIONS",
}

// routeScanDirs 是基线 §3.2 的路由扫描范围：internal/router/*.go 与 internal/handler/**。
var routeScanDirs = []string{"internal/router", "internal/handler"}

// DiscoverRoutes 扫描 internal/router 与 internal/handler 下的非测试 .go，
// 按基线 §3.2 口径发现全部路由注册点：
//   - literal: gin 方法字面调用 g.GET("/x", ...) / g.Any(...)（基线对这两个目录内
//     该形态的调用全量计数，注释文本由 AST 解析天然排除）
//   - apiKeyRoute: rbac 助手 g.apiKeyRoute(grp, http.MethodX, "/y", ...)（每次恰注册一条路由）
//   - handle: 直连 .Handle( 且方法实参为字面量（基线口径：helper 定义内传变量的调用不计）
func DiscoverRoutes(root string) ([]RouteReg, error) {
	var out []RouteReg
	err := forEachGoFile(root, routeScanDirs, func(rel string, f *ast.File, fset *token.FileSet) error {
		ranges := funcDeclRanges(f, fset)
		branches := ifBranchRanges(f, fset)
		chains := newGroupChainTracker(ranges, fset)
		ast.Inspect(f, func(n ast.Node) bool {
			// 先于调用处理：记录 `x := y.Group("/base")` 链（源码序保证先建组后注册）。
			chains.observe(n)
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			reg := RouteReg{File: rel, Line: fset.Position(call.Pos()).Line}
			reg.Func = funcNameAt(ranges, reg.Line)
			reg.BranchScope, reg.InElse = branchAt(branches, reg.Line)
			var recvPrefix string
			switch {
			case ginMethodSet[sel.Sel.Name] != "":
				reg.Kind = "literal"
				reg.Method = ginMethodSet[sel.Sel.Name]
				reg.Path = stringLit(call.Args, 0)
				recvPrefix = chains.prefixOf(sel.X, reg.Func)
			case sel.Sel.Name == "apiKeyRoute" && len(call.Args) >= 3:
				reg.Kind = "apiKeyRoute"
				reg.Method = methodArg(call.Args, 1)
				reg.Path = stringLit(call.Args, 2)
				recvPrefix = chains.prefixOf(call.Args[0], reg.Func)
			case sel.Sel.Name == "Handle" && len(call.Args) >= 2:
				m := methodArg(call.Args, 0)
				if m == "" {
					return true // 方法实参非字面量：基线口径下不计（rbac helper 定义内部）
				}
				reg.Kind = "handle"
				reg.Method = m
				reg.Path = stringLit(call.Args, 1)
				recvPrefix = chains.prefixOf(sel.X, reg.Func)
			default:
				return true
			}
			reg.Key = dedupKey(reg.Method, recvPrefix, reg.Path)
			out = append(out, reg)
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// groupChainTracker 在单个文件的 AST 遍历中跟踪每个函数内
// `x := y.Group("/base")` 的组前缀链，并给出注册调用接收者的静态前缀。
type groupChainTracker struct {
	ranges []funcRange
	fset   *token.FileSet
	chains map[string]string // "func|var" -> prefix（含不可解析标记）
}

func newGroupChainTracker(ranges []funcRange, fset *token.FileSet) *groupChainTracker {
	return &groupChainTracker{ranges: ranges, fset: fset, chains: map[string]string{}}
}

// unknownPrefix 是参数组（挂载基座跨函数不可解析）的命名空间标记。
const unknownPrefix = "\x00"

func (t *groupChainTracker) scopeOf(n ast.Node) string {
	return funcNameAt(t.ranges, t.fset.Position(n.Pos()).Line)
}

// observe 记录赋值语句中的 Group 链。
func (t *groupChainTracker) observe(n ast.Node) {
	assign, ok := n.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Group" || len(call.Args) < 1 {
		return
	}
	base := stringLit(call.Args, 0)
	if base == "" {
		return
	}
	scope := t.scopeOf(n)
	recv := t.prefixOf(sel.X, scope)
	t.chains[scope+"|"+lhs.Name] = joinPrefix(recv, base)
}

// prefixOf 返回接收者表达式的静态前缀：
// 有跟踪链用链值；标识符参数退化 "<unknown>#func#var"；其余表达式同退化（无 var 名）。
func (t *groupChainTracker) prefixOf(recv ast.Expr, scope string) string {
	switch x := recv.(type) {
	case *ast.Ident:
		if p, ok := t.chains[scope+"|"+x.Name]; ok {
			return p
		}
		return unknownPrefix + scope + "#" + x.Name
	case *ast.SelectorExpr:
		return t.prefixOf(x.X, scope) + "." + x.Sel.Name
	case nil:
	}
	return unknownPrefix + scope + "#?"
}

// joinPrefix 拼接组前缀与相对路径。
func joinPrefix(prefix, rel string) string {
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(rel, "/")
}

// dedupKey 组装唯一性键；path 为空（非字面量）时返回 ""（不参与查重）。
func dedupKey(method, recvPrefix, path string) string {
	if path == "" {
		return ""
	}
	return method + "|" + recvPrefix + path
}

// DiscoverWorkers 扫描 internal/router 下的非测试 .go：
//   - redis 模式: mux.HandleFunc(types.TypeX, ...)（asynq ServeMux）
//   - lite 模式: executor.RegisterHandler(types.TypeX, ...)（SyncTaskExecutor）
//
// 首参取标识符/选择器的源码文本（如 "types.TypeChunkExtract"）。
func DiscoverWorkers(root string) (redis, lite []WorkerReg, err error) {
	err = forEachGoFile(root, []string{"internal/router"}, func(rel string, f *ast.File, fset *token.FileSet) error {
		ranges := funcDeclRanges(f, fset)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || len(call.Args) < 1 {
				return true
			}
			var mode string
			switch sel.Sel.Name {
			case "HandleFunc":
				mode = "redis"
			case "RegisterHandler":
				mode = "lite"
			default:
				return true
			}
			line := fset.Position(call.Pos()).Line
			reg := WorkerReg{
				TaskType: exprText(f, call.Args[0]),
				File:     rel,
				Line:     line,
				Func:     funcNameAt(ranges, line),
				Mode:     mode,
			}
			if mode == "redis" {
				redis = append(redis, reg)
			} else {
				lite = append(lite, reg)
			}
			return true
		})
		return nil
	})
	return redis, lite, err
}

// DiscoverHooks 扫描 internal/container 下的非测试 .go，
// 发现全部 container.Invoke(<fn>) 挂点（含 must(container.Invoke(...)) 包裹形态）。
func DiscoverHooks(root string) ([]HookReg, error) {
	var out []HookReg
	err := forEachGoFile(root, []string{"internal/container"},
		func(rel string, f *ast.File, fset *token.FileSet) error {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Invoke" || len(call.Args) < 1 {
					return true
				}
				// 限定 container.Invoke（X 为标识符 container），避免误报其它 .Invoke。
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "container" {
					return true
				}
				out = append(out, HookReg{
					Name: exprText(f, call.Args[0]),
					File: rel,
					Line: fset.Position(call.Pos()).Line,
				})
				return true
			})
			return nil
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FuncDeclNames 返回指定仓库相对目录（递归）内全部顶层函数名（非测试 .go）。
func FuncDeclNames(root string, dirs ...string) (map[string]bool, error) {
	names := map[string]bool{}
	err := forEachGoFile(root, dirs, func(_ string, f *ast.File, _ *token.FileSet) error {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				names[fd.Name.Name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// ---- 内部工具 ----

// forEachGoFile 对 dirs（仓库相对，递归）内每个非测试 .go 做语法解析并回调。
func forEachGoFile(root string, dirs []string, visit func(rel string, f *ast.File, fset *token.FileSet) error) error {
	fset := token.NewFileSet()
	for _, dir := range dirs {
		absDir := filepath.Join(root, filepath.FromSlash(dir))
		err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if path == absDir { // 扫描根缺失视为空集（测试 fixture 场景）
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return fmt.Errorf("parse %s: %w", path, perr)
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			return visit(filepath.ToSlash(rel), f, fset)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// funcRange 是一个顶层函数声明的 [start,end] 行区间。
type funcRange struct {
	name       string
	start, end int
}

// funcDeclRanges 预收集文件内全部顶层函数声明的行区间。
func funcDeclRanges(f *ast.File, fset *token.FileSet) []funcRange {
	var out []funcRange
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		out = append(out, funcRange{
			name:  fd.Name.Name,
			start: fset.Position(fd.Pos()).Line,
			end:   fset.Position(fd.End()).Line,
		})
	}
	return out
}

// ifBranch 是一个 if 语句的行区间（含 body 与 else 的边界）。
type ifBranch struct {
	scopeLine          int // if 语句起始行（分支作用域标识）
	bodyStart, bodyEnd int // then 块区间
	hasElse            bool
	elseStart, elseEnd int // else 块区间
}

// ifBranchRanges 预收集文件内全部 if 语句的分支区间（用于互斥回退注册识别）。
func ifBranchRanges(f *ast.File, fset *token.FileSet) []ifBranch {
	var out []ifBranch
	ast.Inspect(f, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		b := ifBranch{scopeLine: fset.Position(ifStmt.Pos()).Line}
		b.bodyStart = fset.Position(ifStmt.Body.Pos()).Line
		b.bodyEnd = fset.Position(ifStmt.Body.End()).Line
		if ifStmt.Else != nil {
			b.hasElse = true
			b.elseStart = fset.Position(ifStmt.Else.Pos()).Line
			b.elseEnd = fset.Position(ifStmt.Else.End()).Line
		}
		out = append(out, b)
		return true
	})
	return out
}

// branchAt 返回行号所在最内层 if 的 (起始行, 是否 else 分支)；不在分支内返回 (0, false)。
func branchAt(branches []ifBranch, line int) (int, bool) {
	best := -1
	inElse := false
	for i, b := range branches {
		if line >= b.bodyStart && line <= b.bodyEnd {
			if best == -1 || containsBranch(branches[best], branches[i]) {
				best, inElse = i, false
			}
		}
		if b.hasElse && line >= b.elseStart && line <= b.elseEnd {
			if best == -1 || containsBranch(branches[best], branches[i]) {
				best, inElse = i, true
			}
		}
	}
	if best == -1 {
		return 0, false
	}
	return branches[best].scopeLine, inElse
}

// containsBranch 报告 outer 的区间是否包含 inner 的起始行（用于取最内层）。
func containsBranch(outer, inner ifBranch) bool {
	start := inner.bodyStart
	if inner.hasElse {
		start = inner.elseStart
	}
	end := inner.bodyEnd
	if inner.hasElse && inner.elseEnd > end {
		end = inner.elseEnd
	}
	oStart, oEnd := outer.bodyStart, outer.bodyEnd
	if outer.hasElse && outer.elseEnd > oEnd {
		oEnd = outer.elseEnd
	}
	return oStart <= start && end <= oEnd
}

// funcNameAt 返回行号所在函数名；不在任何函数内（如 var 注册块）时记 "<file>"。
func funcNameAt(ranges []funcRange, line int) string {
	best := "<file>"
	for _, r := range ranges {
		if r.start <= line && line <= r.end {
			return r.name
		}
		// 不在函数体内的调用归属 "<file>"；这里仅处理区间前的位置以保持确定性。
		if r.start > line {
			return best
		}
		best = r.name
	}
	return "<file>"
}

// stringLit 返回 args[i] 的字符串字面量值（非字面量或越界返回 ""）。
func stringLit(args []ast.Expr, i int) string {
	if i >= len(args) {
		return ""
	}
	if bl, ok := args[i].(*ast.BasicLit); ok && bl.Kind == token.STRING {
		return strings.Trim(bl.Value, "`\"")
	}
	return ""
}

// methodArg 解析方法实参：http.MethodX 选择器或字符串字面量；其余返回 ""。
func methodArg(args []ast.Expr, i int) string {
	if i >= len(args) {
		return ""
	}
	switch a := args[i].(type) {
	case *ast.SelectorExpr:
		if x, ok := a.X.(*ast.Ident); ok && x.Name == "http" {
			return httpMethodConsts[a.Sel.Name]
		}
	case *ast.BasicLit:
		if a.Kind == token.STRING {
			v := strings.Trim(a.Value, "`\"")
			if strings.EqualFold(v, "any") {
				return "ANY"
			}
			return strings.ToUpper(v)
		}
	}
	return ""
}

// exprText 返回表达式的紧凑源码文本（Ident/Selector）；其余返回 ""。
func exprText(_ *ast.File, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	}
	return ""
}
