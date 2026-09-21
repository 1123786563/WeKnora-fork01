package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Discovery holds the assets found in a repository. All slices are sorted
// so that repeated runs and diagnostics are byte-stable.
type Discovery struct {
	// ModulePath is the Go module path read from go.mod; import paths are
	// translated to repository-relative directories against it.
	ModulePath string

	Routes         []RouteAsset
	Workers        []WorkerAsset
	Lifecycles     []LifecycleAsset
	Migrations     []string // migration asset ids
	Packages       []string // package directories (slash paths)
	RedisTaskTypes []string
	LiteTaskTypes  []string
	Imports        []ImportEdge
}

// RouteAsset is one discovered route registration call.
type RouteAsset struct {
	// ID is the stable display id, e.g. "router.RegisterFooRoutes".
	ID string
	// Symbol is the callee function name used for manifest matching.
	Symbol string
	// Source is the file containing the call.
	Source string
}

// WorkerAsset is one discovered Redis-side task registration.
type WorkerAsset struct {
	// ID is the task type name used for manifest matching.
	ID string
	// Source is the file containing the registration.
	Source string
}

// LifecycleAsset is one discovered container.Invoke wiring call.
type LifecycleAsset struct {
	// ID is the invoked function symbol (or inline:<name>) used for
	// manifest matching.
	ID string
	// Source is the file containing the call.
	Source string
}

// ImportEdge is one package -> package import, both as repository-relative
// slash directories.
type ImportEdge struct {
	Package string
	Path    string
}

// Canonical locations the guard inspects. The server router package, the
// dependency-injection container, the migration trees, and the Go source
// roots are fixed by the repository layout.
var (
	discoverySourceRoots = []string{"internal", "cmd/server"}
	routerDir            = filepath.Join("internal", "router")
	containerDir         = filepath.Join("internal", "container")
	migrationDirs        = []string{
		filepath.Join("migrations", "versioned"),
		filepath.Join("migrations", "sqlite"),
	}
)

var routeSymbolRe = regexp.MustCompile(`^Register[A-Za-z0-9]+Routes$`)

var goBuiltins = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "copy": true,
	"delete": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true, "recover": true,
}

// Discover enumerates the real backend assets under root: route
// registration calls and worker task registrations in the server router,
// container.Invoke lifecycle wirings, migration files, Go packages, and
// package imports. Imports and call sites are extracted through go/parser
// and go/ast, never through regular expressions over source text.
func Discover(root string) (*Discovery, error) {
	root = filepath.Clean(root)
	modulePath, err := readModulePath(root)
	if err != nil {
		return nil, err
	}
	d := &Discovery{ModulePath: modulePath}

	if err := discoverRoutes(root, d); err != nil {
		return nil, err
	}
	if err := discoverWorkers(root, d); err != nil {
		return nil, err
	}
	if err := discoverLifecycles(root, d); err != nil {
		return nil, err
	}
	discoverMigrations(root, d)
	if err := discoverPackages(root, d); err != nil {
		return nil, err
	}
	sortDiscovery(d)
	return d, nil
}

// readModulePath extracts the module declaration from <root>/go.mod.
func readModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after), nil
		}
	}
	return "", fmt.Errorf("go.mod has no module directive")
}

// discoverRoutes parses the server router package and records every
// Register*Routes call: bare package-local calls and package-qualified
// selector calls (e.g. session.RegisterCraftSessionRoutes). Method calls on
// values (g.authorizer.RegisteredRoutes()) are not route registrations and
// are skipped.
func discoverRoutes(root string, d *Discovery) error {
	return forEachGoFile(root, routerDir, func(rel string, file *ast.File) {
		for _, inc := range inspectCalls(file) {
			switch fun := inc.call.Fun.(type) {
			case *ast.Ident:
				if !routeSymbolRe.MatchString(fun.Name) {
					continue
				}
				d.Routes = append(d.Routes, RouteAsset{
					ID:     file.Name.Name + "." + fun.Name,
					Symbol: fun.Name,
					Source: rel,
				})
			case *ast.SelectorExpr:
				pkg, ok := fun.X.(*ast.Ident)
				if !ok || !routeSymbolRe.MatchString(fun.Sel.Name) {
					continue
				}
				d.Routes = append(d.Routes, RouteAsset{
					ID:     pkg.Name + "." + fun.Sel.Name,
					Symbol: fun.Sel.Name,
					Source: rel,
				})
			}
		}
	})
}

// discoverWorkers extracts the Redis-side (HandleFunc in task.go) and
// Lite-side (RegisterHandler in sync_task.go) task type sets.
func discoverWorkers(root string, d *Discovery) error {
	redisSide := func(rel string, file *ast.File) {
		for _, inc := range inspectCalls(file) {
			if !inc.selectorNamed("HandleFunc") {
				continue
			}
			if name, ok := taskTypeName(inc.call); ok {
				d.RedisTaskTypes = append(d.RedisTaskTypes, name)
				d.Workers = append(d.Workers, WorkerAsset{ID: name, Source: rel})
			}
		}
	}
	liteSide := func(rel string, file *ast.File) {
		for _, inc := range inspectCalls(file) {
			if !inc.selectorNamed("RegisterHandler") {
				continue
			}
			if name, ok := taskTypeName(inc.call); ok {
				d.LiteTaskTypes = append(d.LiteTaskTypes, name)
			}
		}
	}
	if err := forEachGoFile(root, filepath.Join(routerDir, "task.go"), redisSide); err != nil {
		return err
	}
	return forEachGoFile(root, filepath.Join(routerDir, "sync_task.go"), liteSide)
}

// taskTypeName extracts the worker identity from the first argument of a
// registration call: a string literal is used verbatim, identifiers and
// selectors (types.TypeFoo) contribute their local name.
func taskTypeName(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	switch arg := call.Args[0].(type) {
	case *ast.BasicLit:
		if arg.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(arg.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.Ident:
		return arg.Name, true
	case *ast.SelectorExpr:
		return arg.Sel.Name, true
	}
	return "", false
}

// discoverLifecycles extracts container.Invoke wirings. The invoked symbol
// is the argument's function name; inline function literals contribute
// "inline:<action>", where the action is the first plain function call in
// the literal body and, failing that, the first method call.
func discoverLifecycles(root string, d *Discovery) error {
	return forEachGoFile(root, containerDir, func(rel string, file *ast.File) {
		for _, inc := range inspectCalls(file) {
			if !inc.selectorNamed("Invoke") || len(inc.call.Args) == 0 {
				continue
			}
			arg := inc.call.Args[0]
			var symbol string
			switch a := arg.(type) {
			case *ast.Ident:
				symbol = a.Name
			case *ast.SelectorExpr:
				if pkg, ok := a.X.(*ast.Ident); ok {
					symbol = pkg.Name + "." + a.Sel.Name
				} else {
					symbol = a.Sel.Name
				}
			case *ast.FuncLit:
				symbol = inlineInvokeSymbol(a)
			default:
				continue
			}
			d.Lifecycles = append(d.Lifecycles, LifecycleAsset{ID: symbol, Source: rel})
		}
	})
}

// inlineInvokeSymbol derives the lifecycle symbol for a function literal
// passed to container.Invoke.
func inlineInvokeSymbol(lit *ast.FuncLit) string {
	var methodCalls []string
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		stmt, ok := n.(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if !goBuiltins[fun.Name] {
				methodCalls = append(methodCalls, "\x00"+fun.Name)
			}
		case *ast.SelectorExpr:
			methodCalls = append(methodCalls, fun.Sel.Name)
		}
		return true
	})
	for _, name := range methodCalls {
		if strings.HasPrefix(name, "\x00") {
			return strings.TrimPrefix(name, "\x00")
		}
	}
	for _, name := range methodCalls {
		if !strings.HasPrefix(name, "\x00") {
			return "inline:" + name
		}
	}
	return fmt.Sprintf("inline:funclit%d", lit.Pos())
}

// discoverMigrations enumerates versioned and SQLite migration trees. Only
// the ordered .up.sql files are recorded; each is expected to have a
// .down.sql sibling, and nothing here ever rewrites them.
func discoverMigrations(root string, d *Discovery) {
	for _, dir := range migrationDirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".up.sql") {
				continue
			}
			base := strings.TrimSuffix(name, ".up.sql")
			leaf := filepath.Base(dir)
			d.Migrations = append(d.Migrations, "migration."+leaf+"."+base)
		}
	}
}

// discoverPackages enumerates Go package directories (directories holding
// at least one non-test .go file) under the canonical source roots and
// collects every intra-module import edge.
func discoverPackages(root string, d *Discovery) error {
	seen := map[string]bool{}
	for _, src := range discoverySourceRoots {
		err := filepath.WalkDir(filepath.Join(root, src), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if path != filepath.Join(root, src) && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata") {
				return filepath.SkipDir
			}
			prefix := slash(root) + "/"
			dir := strings.TrimPrefix(slash(filepath.Clean(path)), prefix)
			if seen[dir] {
				return nil
			}
			files, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			var goFiles []string
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") && !strings.HasSuffix(f.Name(), "_test.go") {
					goFiles = append(goFiles, filepath.Join(path, f.Name()))
				}
			}
			if len(goFiles) == 0 {
				return nil
			}
			seen[dir] = true
			d.Packages = append(d.Packages, dir)
			return collectImports(dir, goFiles, d)
		})
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// collectImports adds the intra-module import edges of one package.
func collectImports(dir string, files []string, d *Discovery) error {
	for _, file := range files {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse imports %s: %w", file, err)
		}
		for _, spec := range parsed.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(value, d.ModulePath+"/") {
				continue
			}
			target := slash(value[len(d.ModulePath)+1:])
			if target == "" || target == dir {
				continue
			}
			d.Imports = append(d.Imports, ImportEdge{Package: dir, Path: target})
		}
	}
	return nil
}

type callSite struct {
	call *ast.CallExpr
}

// inspectCalls returns every call expression in a parsed file.
func inspectCalls(file *ast.File) []callSite {
	var calls []callSite
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, callSite{call: call})
		}
		return true
	})
	return calls
}

// selectorNamed reports whether the callee is X.<name> for any X.
func (c callSite) selectorNamed(name string) bool {
	sel, ok := c.call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}

// forEachGoFile parses every non-test .go file under root+dir (or the
// single file when root+dir has a .go extension) with a full AST and
// invokes fn with the root-relative slash path.
func forEachGoFile(root, dir string, fn func(rel string, file *ast.File)) error {
	absDir := filepath.Join(root, dir)
	var files []string
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		files = []string{absDir}
	} else {
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				files = append(files, filepath.Join(absDir, entry.Name()))
			}
		}
	}
	prefix := slash(root) + "/"
	for _, file := range files {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", file, err)
		}
		fn(strings.TrimPrefix(slash(file), prefix), parsed)
	}
	return nil
}

func sortDiscovery(d *Discovery) {
	sort.Slice(d.Routes, func(i, j int) bool { return d.Routes[i].ID < d.Routes[j].ID })
	sort.Slice(d.Workers, func(i, j int) bool { return d.Workers[i].ID < d.Workers[j].ID })
	sort.Slice(d.Lifecycles, func(i, j int) bool { return d.Lifecycles[i].ID < d.Lifecycles[j].ID })
	sort.Strings(d.Migrations)
	sort.Strings(d.Packages)
	sort.Strings(d.RedisTaskTypes)
	sort.Strings(d.LiteTaskTypes)
	sort.Slice(d.Imports, func(i, j int) bool {
		if d.Imports[i].Package != d.Imports[j].Package {
			return d.Imports[i].Package < d.Imports[j].Package
		}
		return d.Imports[i].Path < d.Imports[j].Path
	})
	d.Imports = dedupEdges(d.Imports)
}

func dedupEdges(edges []ImportEdge) []ImportEdge {
	out := edges[:0]
	var prev ImportEdge
	for i, e := range edges {
		if i > 0 && e == prev {
			continue
		}
		out = append(out, e)
		prev = e
	}
	return out
}

func slash(p string) string { return filepath.ToSlash(p) }
