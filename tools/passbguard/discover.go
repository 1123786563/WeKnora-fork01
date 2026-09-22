package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/tools/internal/movemanifest"
)

// architectureGuardCheckFile 是 importExceptions 的唯一在册事实源文件
// （F1 裁定：passbguard 用 go/ast 解析该复合字面量并与 exception-ledger
// 逐条对照，禁止两侧各自漂移）。
const architectureGuardCheckFile = "tools/architectureguard/check.go"

// repoModulePath 是仓库 go.mod 的 module path（ImportPath 推导用，
// 与 tools/architectureguard moduleImportBase 同源常量）。
const repoModulePath = "github.com/Tencent/WeKnora"

// LegacyFact 是一份 manifest legacy_files 条目的发现快照。
type LegacyFact struct {
	Path      string
	Module    string
	PassBTask string
}

// DiscoveredException 是 architectureguard check.go 源内 importExceptions
// 一条精确 (importer file → imported package) 豁免的发现快照。
type DiscoveredException struct {
	ImporterFile string
	ImportedPath string
	Reason       string
	PassBTask    string
}

// Discovery 是 DiscoverPassB 对真实仓库的只读发现结果：
// 16 份 manifest（legacy/alias 义务）、当前 .go 文件树、逐文件 import 面、
// 以及 guard 源码中的精确 import 例外。计数一律来自这里，不从散文证据解析。
type Discovery struct {
	Root        string
	Manifests   []*movemanifest.MoveManifest
	LegacyFiles []LegacyFact
	Aliases     []movemanifest.AliasObligation
	GoFiles     []string
	Imports     map[string][]string
	Exceptions  []DiscoveredException

	// astCache 是契约符号/消费方发现的惰性全量 AST 缓存（文件路径 → 解析结果）。
	// 未导出：测试与调用方按值/字面量构造 Discovery 时为零值，首次访问惰性初始化。
	// 进程内顺序使用（guard/测试均为单线程），不加锁。
	astCache map[string]*cachedAST
}

// cachedAST 是一个 .go 文件的完整 AST（含定位信息）。
type cachedAST struct {
	file *ast.File
	fset *token.FileSet
}

// DiscoverPassB 读取仓库事实：manifests（docs/architecture/moves/*.yaml）、
// 当前 .go 路径与 import、architectureguard 的 importExceptions。
func DiscoverPassB(root string) (*Discovery, error) {
	ids, err := movemanifest.ListManifestModules(root)
	if err != nil {
		return nil, fmt.Errorf("discover manifests: %w", err)
	}
	d := &Discovery{
		Root:    root,
		Imports: map[string][]string{},
	}
	for _, id := range ids {
		m, err := movemanifest.LoadManifestStrict(movemanifest.ManifestPath(root, id))
		if err != nil {
			return nil, fmt.Errorf("discover manifests: %w", err)
		}
		d.Manifests = append(d.Manifests, m)
		for _, lf := range m.LegacyFiles {
			d.LegacyFiles = append(d.LegacyFiles, LegacyFact{Path: lf.Path, Module: m.Module, PassBTask: lf.PassBTask})
		}
		d.Aliases = append(d.Aliases, m.AliasObligations...)
	}
	sort.Slice(d.LegacyFiles, func(i, j int) bool { return d.LegacyFiles[i].Path < d.LegacyFiles[j].Path })
	sort.Slice(d.Aliases, func(i, j int) bool { return d.Aliases[i].OldImportPath < d.Aliases[j].OldImportPath })

	goFiles, imports, err := discoverGoTree(root)
	if err != nil {
		return nil, err
	}
	d.GoFiles = goFiles
	d.Imports = imports

	excs, err := discoverImportExceptions(filepath.Join(root, filepath.FromSlash(architectureGuardCheckFile)))
	if err != nil {
		return nil, err
	}
	d.Exceptions = excs
	return d, nil
}

// discoverGoTree 枚举仓库内全部 .go 文件（跳过 .git 与 testdata——Go 工具链
// 惯例上 testdata 不参与构建，guard fixture 不得进入仓库发现集），并解析每个
// 文件的 import 面（含厂商路径原样保留）。返回 slash 分隔的仓库相对路径。
func discoverGoTree(root string) ([]string, map[string][]string, error) {
	var files []string
	imports := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		files = append(files, slash)

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse imports %s: %w", slash, err)
		}
		var paths []string
		for _, imp := range f.Imports {
			v, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("unquote import in %s: %w", slash, err)
			}
			paths = append(paths, v)
		}
		sort.Strings(paths)
		imports[slash] = paths
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk go tree: %w", err)
	}
	sort.Strings(files)
	return files, imports, nil
}

// discoverImportExceptions 用 go/ast 解析 architectureguard check.go 源码中的
// importExceptions 复合字面量（字段 ImporterFile/ImportedPath/Reason/PassBTask）。
// 不依赖该 main 包可导入——源码即接口（F1 裁定方案）。
func discoverImportExceptions(path string) ([]DiscoveredException, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read importExceptions source: %w", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse importExceptions source: %w", err)
	}
	var out []DiscoveredException
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "importExceptions" {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("importExceptions 不是复合字面量")
			}
			for _, elt := range lit.Elts {
				row := DiscoveredException{}
				kv, ok := elt.(*ast.CompositeLit)
				if !ok {
					return nil, fmt.Errorf("importExceptions 元素不是 struct 字面量")
				}
				for _, field := range kv.Elts {
					pair, ok := field.(*ast.KeyValueExpr)
					if !ok {
						return nil, fmt.Errorf("importExceptions 元素字段不是 key-value")
					}
					key, _ := pair.Key.(*ast.Ident)
					if key == nil {
						continue
					}
					value, err := constString(pair.Value)
					if err != nil {
						return nil, fmt.Errorf("importExceptions 字段 %s: %w", key.Name, err)
					}
					switch key.Name {
					case "ImporterFile":
						row.ImporterFile = value
					case "ImportedPath":
						row.ImportedPath = value
					case "Reason":
						row.Reason = value
					case "PassBTask":
						row.PassBTask = value
					}
				}
				out = append(out, row)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("importExceptions 在 %s 中未找到或为空", path)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ImporterFile != out[j].ImporterFile {
			return out[i].ImporterFile < out[j].ImporterFile
		}
		return out[i].ImportedPath < out[j].ImportedPath
	})
	return out, nil
}

// constString 求值字符串常量表达式（字面量与 + 拼接，含括号）。
func constString(expr ast.Expr) (string, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", fmt.Errorf("want string literal, got %s", e.Kind)
		}
		return strconv.Unquote(e.Value)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", fmt.Errorf("want string concatenation, got %s", e.Op)
		}
		left, err := constString(e.X)
		if err != nil {
			return "", err
		}
		right, err := constString(e.Y)
		if err != nil {
			return "", err
		}
		return left + right, nil
	case *ast.ParenExpr:
		return constString(e.X)
	default:
		return "", fmt.Errorf("unsupported constant expression %T", expr)
	}
}

// ---- B0.4：契约符号与消费方发现（go/parser AST；源码文本即冻结语义）----

// SymbolFact 是一个冻结契约符号（顶层导出声明）的发现快照。
// File 是仓库相对 slash 路径；Name 是顶层声明名；DeclKind ∈
// func|interface|struct|type|var|const；Signature 是规范化渲染签名
// （源码序文本、单行、空白折叠——冻结与校验共用同一渲染器，签名相等即无漂移）；
// ImportPath 是定义包完整 import 路径（module path + 目录）。
type SymbolFact struct {
	File       string
	Name       string
	DeclKind   string
	Signature  string
	ImportPath string
}

// parseFileFull 解析单个 .go 文件为完整 AST（缓存）。
func (d *Discovery) parseFileFull(rel string) (*cachedAST, error) {
	if d.astCache == nil {
		d.astCache = map[string]*cachedAST{}
	}
	if c, ok := d.astCache[rel]; ok {
		return c, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(d.Root, filepath.FromSlash(rel)), nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	c := &cachedAST{file: f, fset: fset}
	d.astCache[rel] = c
	return c, nil
}

// DiscoverSymbol 在仓库内定位 file:Name 顶层声明并渲染其当前签名。
// 找到返回 (fact, true, nil)；文件可解析但声明不存在返回 (nil, false, nil)；
// 文件不存在/不可解析返回错误。
func DiscoverSymbol(root, file, name string) (*SymbolFact, bool, error) {
	abs := filepath.Join(root, filepath.FromSlash(file))
	if _, err := os.Stat(abs); err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", file, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, abs, nil, 0)
	if err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", file, err)
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || d.Name.Name != name {
				continue
			}
			return &SymbolFact{
				File:       file,
				Name:       name,
				DeclKind:   "func",
				Signature:  "func " + name + renderFuncType(d.Type, fset),
				ImportPath: importPathOfDir(dirOf(file)),
			}, true, nil
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name != name {
						continue
					}
					kind, sig := renderTypeSpec(s, fset)
					return &SymbolFact{
						File:       file,
						Name:       name,
						DeclKind:   kind,
						Signature:  sig,
						ImportPath: importPathOfDir(dirOf(file)),
					}, true, nil
				case *ast.ValueSpec:
					if len(s.Names) != 1 || s.Names[0].Name != name {
						continue
					}
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					return &SymbolFact{
						File:       file,
						Name:       name,
						DeclKind:   kind,
						Signature:  "type " + name + " " + renderValueType(s, fset),
						ImportPath: importPathOfDir(dirOf(file)),
					}, true, nil
				}
			}
		}
	}
	return nil, false, nil
}

// DiscoverSymbolConsumers 发现一个符号的生产消费方与特征化测试文件。
// 消费方 = 满足以下全部条件的 .go 文件（不含定义文件自身）：
//   - 与定义文件同目录（同包裸引用），或 import 定义包（限定名引用）；
//   - 文件内出现对符号的引用：同包裸 Ident 命中，或以定义包任一限定名
//     （含别名）的 SelectorExpr.Sel 命中。
//
// 返回 consumers（非 _test.go，排序）与 tests（_test.go，排序）。
func DiscoverSymbolConsumers(d *Discovery, fact *SymbolFact) (consumers, tests []string, err error) {
	defDir := dirOf(fact.File)
	for _, f := range d.GoFiles {
		if f == fact.File {
			continue
		}
		sameDir := dirOf(f) == defDir
		importsDef := containsExact(d.Imports[f], fact.ImportPath)
		if !sameDir && !importsDef {
			continue
		}
		c, perr := d.parseFileFull(f)
		if perr != nil {
			return nil, nil, perr
		}
		qualifiers := qualifiersFor(c.file, fact.ImportPath)
		if referencesSymbol(c.file, fact.Name, sameDir, qualifiers) {
			if strings.HasSuffix(f, "_test.go") {
				tests = append(tests, f)
			} else {
				consumers = append(consumers, f)
			}
		}
	}
	sort.Strings(consumers)
	sort.Strings(tests)
	return consumers, tests, nil
}

// referencesSymbol 判断文件是否引用符号：同包时裸 Ident 命中即可；
// 跨包时须以定义包限定名（含别名）的 SelectorExpr.Sel 命中。
func referencesSymbol(f *ast.File, name string, sameDir bool, qualifiers map[string]bool) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		switch e := n.(type) {
		case *ast.SelectorExpr:
			if e.Sel.Name != name {
				return true
			}
			if id, ok := e.X.(*ast.Ident); ok && qualifiers[id.Name] {
				found = true
				return false
			}
		case *ast.Ident:
			if sameDir && e.Name == name {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// qualifiersFor 返回文件内指向 importPath 的全部限定名（别名或默认包名）。
func qualifiersFor(f *ast.File, importPath string) map[string]bool {
	out := map[string]bool{}
	for _, imp := range f.Imports {
		v, err := strconv.Unquote(imp.Path.Value)
		if err != nil || v != importPath {
			continue
		}
		if imp.Name != nil {
			out[imp.Name.Name] = true
			continue
		}
		if i := strings.LastIndex(v, "/"); i >= 0 {
			out[v[i+1:]] = true
		} else {
			out[v] = true
		}
	}
	return out
}

// dirOf 返回 slash 路径的目录部分（无分隔符返回 "."）。
func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

// importPathOfDir 由仓库相对目录推导完整 import 路径。
func importPathOfDir(dir string) string {
	if dir == "." {
		return repoModulePath
	}
	return repoModulePath + "/" + dir
}

// ---- 签名渲染（规范化：单行、空白折叠、源序文本）----

// renderFuncType 渲染函数类型（参数 + 结果），无结果省略、单匿名结果无括号。
func renderFuncType(ft *ast.FuncType, fset *token.FileSet) string {
	var b strings.Builder
	b.WriteString("(")
	b.WriteString(renderFieldList(ft.Params, fset))
	b.WriteString(")")
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return b.String()
	}
	res := renderFieldList(ft.Results, fset)
	unnamedSingle := len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0
	if unnamedSingle {
		b.WriteString(" " + res)
	} else {
		b.WriteString(" (" + res + ")")
	}
	return b.String()
}

// renderFieldList 渲染参数/结果字段清单：命名参数 "name1, name2 Type"，
// 匿名字段仅类型；逗号连接。
func renderFieldList(fl *ast.FieldList, fset *token.FileSet) string {
	if fl == nil {
		return ""
	}
	parts := make([]string, 0, len(fl.List))
	for _, field := range fl.List {
		t := exprText(field.Type, fset)
		if len(field.Names) == 0 {
			parts = append(parts, t)
			continue
		}
		names := make([]string, 0, len(field.Names))
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+t)
	}
	return strings.Join(parts, ", ")
}

// renderTypeSpec 渲染类型声明：interface/struct 展开成员集合，
// 其余（定义类型/别名）原样渲染右侧表达式。
func renderTypeSpec(s *ast.TypeSpec, fset *token.FileSet) (kind, signature string) {
	switch t := s.Type.(type) {
	case *ast.InterfaceType:
		methods := make([]string, 0, len(t.Methods.List))
		for _, m := range t.Methods.List {
			if len(m.Names) == 0 { // 嵌入接口
				methods = append(methods, exprText(m.Type, fset))
				continue
			}
			ft, ok := m.Type.(*ast.FuncType)
			if !ok {
				continue
			}
			methods = append(methods, m.Names[0].Name+renderFuncType(ft, fset))
		}
		return "interface", "interface { " + strings.Join(methods, "; ") + " }"
	case *ast.StructType:
		fields := make([]string, 0, len(t.Fields.List))
		for _, field := range t.Fields.List {
			entry := exprText(field.Type, fset)
			if len(field.Names) > 0 {
				names := make([]string, 0, len(field.Names))
				for _, n := range field.Names {
					names = append(names, n.Name)
				}
				entry = strings.Join(names, ", ") + " " + entry
			}
			if field.Tag != nil {
				entry += " " + field.Tag.Value
			}
			fields = append(fields, entry)
		}
		return "struct", "struct { " + strings.Join(fields, "; ") + " }"
	default:
		return "type", "type " + s.Name.Name + " " + exprText(s.Type, fset)
	}
}

// renderValueType 渲染 var/const 声明的类型或值表达式。
func renderValueType(s *ast.ValueSpec, fset *token.FileSet) string {
	if s.Type != nil {
		return exprText(s.Type, fset)
	}
	if len(s.Values) > 0 {
		return exprText(s.Values[0], fset)
	}
	return ""
}

// exprText 用 go/printer 输出表达式源码文本并折叠空白为单空格。
func exprText(e ast.Expr, fset *token.FileSet) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, e); err != nil {
		return ""
	}
	return collapseSpaces(buf.String())
}

// collapseSpaces 把制表符/换行/连续空白折叠为单个空格并去除首尾空白。
func collapseSpaces(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			space = b.Len() > 0
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
