package passbguard

import (
	"fmt"
	"go/ast"
	"go/parser"
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

// discoverGoTree 枚举仓库内全部 .go 文件（跳过 .git），并解析每个文件的
// import 面（含厂商路径原样保留）。返回 slash 分隔的仓库相对路径。
func discoverGoTree(root string) ([]string, map[string][]string, error) {
	var files []string
	imports := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
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
