package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic 是一条可读的校验诊断（排序后输出）。
type Diagnostic struct {
	Module  string // 所属 manifest 的 module id（跨 manifest 检查用 "*"）
	Check   string // 检查名，如 "alias-1to1"、"ownership-overlap"
	Message string // 具体违规描述
}

// String 输出形如 "modulemove: <module>: <check>: <message>"。
func (d Diagnostic) String() string {
	return fmt.Sprintf("modulemove: %s: %s: %s", d.Module, d.Check, d.Message)
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

// GoListError 镜像 go list -json 输出中的 Error 字段。
type GoListError struct {
	Err string `json:"Err"`
}

// GoPackage 是 go list -json 输出中本工具关心的字段子集。
type GoPackage struct {
	ImportPath string
	Dir        string
	Name       string
	Error      *GoListError `json:"Error,omitempty"`
}

// GoLister 对一个 go list pattern（如 "internal/agent/..."）在指定目录（仓库根）下
// 执行 `go list -e -json`。生产实现走真实 go 命令；测试注入假实现以保持用例离线、快速。
type GoLister func(dir, pattern string) ([]GoPackage, error)

// Verifier 持有校验所需的环境（仓库根 + 可注入的 go list）。
type Verifier struct {
	Root   string
	GoList GoLister
}

// goList 是生产环境的 GoLister：在 dir 下执行 `go list -e -json <pattern>`。
// -e 让不存在的包也产出带 Error 的对象，便于给出确定性诊断。
func goList(dir, pattern string) ([]GoPackage, error) {
	cmd := exec.Command("go", "list", "-e", "-json", pattern)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// go list -e 几乎总是 0 退出；非 0 属环境故障（如 go 不在 PATH）。
		return nil, fmt.Errorf("go list %s: %w: %s", pattern, err, strings.TrimSpace(stderr.String()))
	}
	var pkgs []GoPackage
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var p GoPackage
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode go list output for %s: %w", pattern, err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// mandatoryForbidden 是每份 manifest 都必须禁改的四个共享文件
// （moves/README.md §forbidden_shared_files；go.mod/go.sum/migrations/ 为额外项）。
var mandatoryForbidden = []string{
	"internal/router/router.go",
	"internal/router/task.go",
	"internal/router/sync_task.go",
	"internal/container/container.go",
}

// globMeta 是被禁止出现在 forbidden_shared_files 中的通配符字符
// （例外必须是精确路径，杜绝 "internal/router/*" 这类越权通配）。
const globMeta = "*?[]{\\"

// VerifyManifest 校验单个 manifest 的内部一致性。
func (v *Verifier) VerifyManifest(m *MoveManifest) []Diagnostic {
	var ds []Diagnostic
	d := func(check, format string, args ...any) {
		ds = append(ds, Diagnostic{Module: m.Module, Check: check, Message: fmt.Sprintf(format, args...)})
	}

	// --- move_packages 与 alias_obligations 1:1 ---
	aliasOf := map[string]string{} // old_import_path -> passb_task
	for _, a := range m.AliasObligations {
		aliasOf[a.OldImportPath] = a.PassBTask
	}
	froms := make([]string, 0, len(m.MovePackages))
	for _, mp := range m.MovePackages {
		froms = append(froms, mp.From)
		task, ok := aliasOf[mp.From]
		if !ok {
			d("alias-1to1", "move from %q 缺少对应 alias_obligation（old_import_path == from）", mp.From)
			continue
		}
		if task == "" {
			d("alias-1to1", "move from %q 的别名义务缺少 passb_task", mp.From)
		}
	}
	for _, a := range m.AliasObligations {
		if !containsString(froms, a.OldImportPath) {
			d("alias-1to1", "alias_obligation %q 没有对应的 move_packages[].from", a.OldImportPath)
		}
	}
	if len(aliasOf) != len(m.AliasObligations) {
		d("alias-1to1", "alias_obligations 内部存在重复 old_import_path")
	}

	// --- move_targets 必须落在 internal/modules/<module> 命名空间内 ---
	moduleNS := "internal/modules/" + m.Module
	for _, mp := range m.MovePackages {
		if mp.To == mp.From {
			d("move-target", "to == from (%q)", mp.To)
		}
		if mp.To != moduleNS && !strings.HasPrefix(mp.To, moduleNS+"/") {
			d("move-target", "to %q 不在模块命名空间 %s[/...] 下", mp.To, moduleNS)
		}
	}

	// --- passb_task 命名（B-<module>）---
	for _, a := range m.AliasObligations {
		if a.PassBTask != "" && a.PassBTask != "B-"+m.Module {
			d("passb-task", "别名义务 passb_task=%q, want B-%s", a.PassBTask, m.Module)
		}
	}
	for _, lf := range m.LegacyFiles {
		if lf.PassBTask != "" && lf.PassBTask != "B-"+m.Module {
			d("passb-task", "legacy %s passb_task=%q, want B-%s", lf.Path, lf.PassBTask, m.Module)
		}
	}

	// --- 源目录：存在、是 Go 包、package 子句唯一（整体搬移前提）---
	rooted := func(rel string) string { return filepath.Join(v.Root, filepath.FromSlash(rel)) }
	dirHasGo := func(rel string) ([]string, bool) {
		entries, err := os.ReadDir(rooted(rel))
		if err != nil {
			return nil, false
		}
		var goFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				goFiles = append(goFiles, e.Name())
			}
		}
		return goFiles, len(goFiles) > 0
	}
	// singlePackageClause 返回该目录的 package 名（全部非测试 .go 一致）。
	// 测试文件允许标准布局：package pkg 与 package pkg_test 共存；
	// 纯测试包（仅 _test.go，如 internal/agent/nativeprobe）要求同属一个 X / X_test 家族。
	singlePackageClause := func(rel string) (string, error) {
		goFiles, _ := dirHasGo(rel)
		pkgName := ""
		testPkgs := map[string]bool{}
		fset := token.NewFileSet()
		clause := func(name string) (string, error) {
			f, err := parser.ParseFile(fset, filepath.Join(rooted(rel), name), nil, parser.PackageClauseOnly)
			if err != nil {
				return "", fmt.Errorf("解析 %s/%s: %w", rel, name, err)
			}
			return f.Name.Name, nil
		}
		for _, name := range goFiles {
			got, err := clause(name)
			if err != nil {
				return "", err
			}
			if strings.HasSuffix(name, "_test.go") {
				testPkgs[got] = true
				continue
			}
			if pkgName == "" {
				pkgName = got
			} else if got != pkgName {
				return "", fmt.Errorf("%s 内出现多个 package 子句: %s 与 %s", rel, pkgName, got)
			}
		}
		if pkgName != "" {
			for tp := range testPkgs {
				if tp != pkgName && tp != pkgName+"_test" {
					return "", fmt.Errorf("%s 测试文件 package %q 与包 %q 不匹配", rel, tp, pkgName)
				}
			}
			return pkgName, nil
		}
		if len(testPkgs) > 0 {
			base := ""
			for tp := range testPkgs {
				b := strings.TrimSuffix(tp, "_test")
				if base == "" {
					base = b
				} else if b != base {
					return "", fmt.Errorf("%s 纯测试包出现多个 package 家族: %s 与 %s", rel, base, b)
				}
			}
			return base + " (test-only)", nil
		}
		return "", fmt.Errorf("%s 内没有任何 .go 文件", rel)
	}

	// goPackageListed 报告 rel 是否是现存的 Go 包：注入 GoList 时以 go list 为准；
	// 未注入时退化为文件系统检查。
	// pattern 用 "./"+rel 在主模块内解析（裸 internal/... 会被 go 当作 std 路径）。
	// 字面量单路径 pattern 至多产出一个对象：Error == nil 即存在；带 Error 或没有
	// 输出即不存在。（不能按 Dir 对齐 —— Root 为相对路径时 rooted(rel) 与 go list
	// 输出的绝对 Dir 恒不相等。）
	goPackageListed := func(rel string) (bool, string) {
		if v.GoList == nil {
			if _, ok := dirHasGo(rel); ok {
				return true, ""
			}
			return false, "目录不存在或不含 .go 文件"
		}
		pkgs, err := v.GoList(v.Root, "./"+rel)
		if err != nil {
			return false, fmt.Sprintf("go list %s 失败: %v", rel, err)
		}
		if len(pkgs) == 0 {
			return false, "go list 未找到该包"
		}
		if pkgs[0].Error == nil {
			return true, ""
		}
		return false, fmt.Sprintf("go list 报错: %s", pkgs[0].Error.Err)
	}

	// 搬迁已执行（集成后旧别名路径被删除）时 from 目录不复存在：此时放行的唯一条件是
	// to 侧已是现存 Go 包（校验转查 to 侧）；from 缺失且 to 也缺失仍是错误。
	for _, mp := range m.MovePackages {
		if _, ok := dirHasGo(mp.From); ok {
			if _, err := singlePackageClause(mp.From); err != nil {
				d("source-package", "%v（整体搬移要求 package 子句唯一）", err)
			}
			continue
		}
		if ok, why := goPackageListed(mp.To); !ok {
			d("source-package", "源目录 not found 或不含 .go 文件: %s，且搬迁目标 %s 不是现存 Go 包（%s）",
				mp.From, mp.To, why)
		}
	}

	// --- partial-move：from/... 下每个子包都必须自身是本 manifest 的一个 from
	// （manifest 约定整棵搬迁树逐包声明，保证每个旧 import path 都有别名）---
	// 仅在 from 仍存在（搬前状态）时适用；from 已删除（搬迁已执行）时跳过。
	if v.GoList != nil {
		for _, mp := range m.MovePackages {
			if _, ok := dirHasGo(mp.From); !ok {
				continue
			}
			subPkgs, err := v.GoList(v.Root, mp.From+"/...")
			if err != nil {
				d("partial-move", "go list %s/... 失败: %v", mp.From, err)
				continue
			}
			for _, p := range subPkgs {
				if p.Dir == rooted(mp.From) || p.ImportPath == mp.From {
					if p.Error != nil {
						d("partial-move", "go list 报告源包 %s 错误: %s", p.ImportPath, p.Error.Err)
					}
					continue
				}
				if !containsString(froms, p.ImportPath) {
					d("partial-move", "子包 %s（from %s 的子树）未被声明为独立的 move from：旧 import path 将失去别名",
						p.ImportPath, mp.From)
				}
			}
		}
	}

	// --- owned_files.move_sources 与 from 集合一致 ---
	if !sameStringSet(m.OwnedFiles.MoveSources, froms) {
		missing := subtract(froms, m.OwnedFiles.MoveSources)
		extra := subtract(m.OwnedFiles.MoveSources, froms)
		d("owned-move-sources", "move_sources != move_packages.from 集合: 缺少 %v, 多出 %v",
			sortStrings(missing), sortStrings(extra))
	}

	// --- owned_files.module_files 存在 ---
	for _, mf := range m.OwnedFiles.ModuleFiles {
		if _, err := os.Stat(rooted(mf)); err != nil {
			d("owned-module-files", "module file not found: %s", mf)
		}
	}

	// --- test_commands 非空 ---
	if len(m.TestCommands) == 0 {
		d("test-commands", "test_commands 不得为空（搬迁后证明本模块的精确命令）")
	}

	// --- forbidden_shared_files：强制四文件 + 精确路径（禁通配） ---
	for _, req := range mandatoryForbidden {
		if !containsString(m.ForbiddenSharedFiles, req) {
			d("forbidden-shared-files", "缺少强制禁改文件 %s", req)
		}
	}
	for _, f := range m.ForbiddenSharedFiles {
		if strings.ContainsAny(f, globMeta) {
			d("forbidden-shared-files", "禁改例外必须是精确路径，拒绝通配符 %q", f)
		}
	}

	// --- legacy_files：存在、非测试 .go ---
	for _, lf := range m.LegacyFiles {
		if !strings.HasSuffix(lf.Path, ".go") || strings.HasSuffix(lf.Path, "_test.go") {
			d("legacy-file", "%s 必须是非测试 .go 文件", lf.Path)
			continue
		}
		if _, err := os.Stat(rooted(lf.Path)); err != nil {
			d("legacy-file", "not found: %s", lf.Path)
		}
	}

	return SortDiagnostics(ds)
}

// VerifyAll 在逐 manifest 校验之上追加跨 manifest 所有权重叠检查
// （同一 from/to/legacy 路径不得被两份 manifest 声明；from 之间不得存在路径嵌套）。
func (v *Verifier) VerifyAll(mods []*MoveManifest) []Diagnostic {
	var ds []Diagnostic
	add := func(module, check, format string, args ...any) {
		ds = append(ds, Diagnostic{Module: module, Check: check, Message: fmt.Sprintf(format, args...)})
	}

	fromOwner := map[string]string{}
	toOwner := map[string]string{}
	legacyOwner := map[string]string{}

	for _, m := range mods {
		ds = append(ds, v.VerifyManifest(m)...)
		for _, mp := range m.MovePackages {
			if prev, dup := fromOwner[mp.From]; dup {
				add("*", "ownership-overlap", "from %s 被重复声明: %s 与 %s", mp.From, prev, m.Module)
			} else {
				fromOwner[mp.From] = m.Module
			}
			if prev, dup := toOwner[mp.To]; dup {
				add("*", "ownership-overlap", "to %s 被重复声明: %s 与 %s", mp.To, prev, m.Module)
			} else {
				toOwner[mp.To] = m.Module
			}
		}
		for _, lf := range m.LegacyFiles {
			if prev, dup := legacyOwner[lf.Path]; dup {
				add("*", "ownership-overlap", "legacy path %s 被重复声明: %s 与 %s", lf.Path, prev, m.Module)
			} else {
				legacyOwner[lf.Path] = m.Module
			}
		}
	}

	// from 路径嵌套：仅当嵌套双方属于不同模块时才冲突
	// （同一 manifest 内逐包声明嵌套 from 是 moves/README.md 确定性规则 1 的约定）。
	fromPaths := make([]string, 0, len(fromOwner))
	for f := range fromOwner {
		fromPaths = append(fromPaths, f)
	}
	sort.Strings(fromPaths)
	for i, a := range fromPaths {
		for j, b := range fromPaths {
			if i != j && fromOwner[a] != fromOwner[b] && isStrictPathPrefix(b, a) {
				add("*", "ownership-overlap",
					"from %s（%s）嵌套于 from %s（%s）之内：跨模块整树搬移冲突", a, fromOwner[a], b, fromOwner[b])
			}
		}
	}

	return SortDiagnostics(ds)
}

// containsString 报告 list 是否含 s。
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sameStringSet 忽略顺序与重复，比较两个集合。
func sameStringSet(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ma := map[string]bool{}
	for _, v := range a {
		ma[v] = true
	}
	mb := map[string]bool{}
	for _, v := range b {
		mb[v] = true
	}
	if len(ma) != len(mb) {
		return false
	}
	for v := range ma {
		if !mb[v] {
			return false
		}
	}
	return true
}

// subtract 返回在 a 不在 b 的元素。
func subtract(a, b []string) []string {
	mb := map[string]bool{}
	for _, v := range b {
		mb[v] = true
	}
	var out []string
	for _, v := range a {
		if !mb[v] {
			out = append(out, v)
		}
	}
	return out
}

// isStrictPathPrefix 报告 prefix 是否是 p 的严格路径前缀（"/" 边界对齐）。
func isStrictPathPrefix(prefix, p string) bool {
	return prefix != p && strings.HasPrefix(p, prefix+"/")
}
