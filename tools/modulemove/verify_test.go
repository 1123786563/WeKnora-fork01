package main

import (
	"os"
	"path/filepath"
	"strings"
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

const demoManifestYAML = `module: demo
description: demo module for tests
move_packages:
  - from: internal/demo
    to: internal/modules/demo
alias_obligations:
  - old_import_path: internal/demo
    passb_task: B-demo
legacy_files:
  - path: internal/application/service/demo.go
    reason: trapped in horizontal service package
    navigation_label: Demo service
    passb_task: B-demo
owned_files:
  move_sources:
  - internal/demo
  move_targets:
    - internal/modules/demo
  importers:
  - internal/container
  module_files:
    - internal/modules/demo/README.md
    - internal/modules/demo/module.go
    - internal/modules/demo/legacy/README.md
test_commands:
  - go test ./internal/modules/demo/... -count=1
integration_points:
  routes: []
  workers: []
  lifecycle_hooks: []
forbidden_shared_files:
  - internal/router/router.go
  - internal/router/task.go
  - internal/router/sync_task.go
  - internal/container/container.go
  - go.mod
  - go.sum
  - migrations/
`

// demoFixtureFiles 是与 demoManifestYAML 匹配的最小仓库文件树。
func demoFixtureFiles() map[string]string {
	return map[string]string{
		"internal/demo/demo.go":                  "package demo\n",
		"internal/application/service/demo.go":   "package service\n",
		"internal/modules/demo/README.md":        "# demo\n",
		"internal/modules/demo/module.go":        "package demo\n",
		"internal/modules/demo/legacy/README.md": "# demo legacy\n",
		"internal/router/router.go":              "package router\n",
		"internal/router/task.go":                "package router\n",
		"internal/router/sync_task.go":           "package router\n",
		"internal/container/container.go":        "package container\n",
	}
}

// demoGoPackages 是与 demo fixture 匹配的 go list 结果。
func demoGoPackages(root string) map[string][]GoPackage {
	return map[string][]GoPackage{
		"internal/demo/...": {
			{ImportPath: "internal/demo", Dir: filepath.Join(root, "internal/demo"), Name: "demo"},
		},
	}
}

// fakeGoList 按精确 pattern 返回预置包集合；未登记的 pattern 返回空集。
func fakeGoList(byPattern map[string][]GoPackage) GoLister {
	return func(pattern string) ([]GoPackage, error) {
		return byPattern[pattern], nil
	}
}

// newDemoVerifier 构造针对 demo fixture 的 Verifier。
func newDemoVerifier(t *testing.T) (*Verifier, string) {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, demoFixtureFiles())
	v := &Verifier{Root: root, GoList: fakeGoList(demoGoPackages(root))}
	return v, root
}

func loadDemoManifest(t *testing.T) *MoveManifest {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := os.WriteFile(path, []byte(demoManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifestStrict(path)
	if err != nil {
		t.Fatalf("LoadManifestStrict: %v", err)
	}
	return m
}

func hasDiagnostic(ds []Diagnostic, check, substr string) bool {
	for _, d := range ds {
		if d.Check == check && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestVerifyManifestAcceptsValidFixture(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)

	ds := SortDiagnostics(v.VerifyManifest(m))
	if len(ds) != 0 {
		t.Fatalf("合法 fixture 不应产生诊断，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsAliasMoveMismatch(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.AliasObligations = nil // 1:1 被破坏：有搬迁却无别名义务

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "alias-1to1", "internal/demo") {
		t.Fatalf("缺少 alias-1to1 诊断，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsAliasOldImportPathNotEqualFrom(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.AliasObligations[0].OldImportPath = "internal/other"

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "alias-1to1", "internal/other") {
		t.Fatalf("old_import_path != from 应报告 alias-1to1，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMissingSourceDir(t *testing.T) {
	v, root := newDemoVerifier(t)
	m := loadDemoManifest(t)
	if err := os.RemoveAll(filepath.Join(root, "internal", "demo")); err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "source-package", "not found") && !hasDiagnostic(ds, "source-package", "internal/demo") {
		t.Fatalf("源目录缺失应报告 source-package 诊断，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMixedPackageClauseSource(t *testing.T) {
	v, root := newDemoVerifier(t)
	m := loadDemoManifest(t)
	writeTree(t, root, map[string]string{
		"internal/demo/extra.go": "package other\n",
	})

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "source-package", "package") {
		t.Fatalf("源目录出现多个 package 子句应报告 source-package（整体搬移前提被破坏），得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsPartialPackageMove(t *testing.T) {
	// internal/demo/sub 是仓库里的真实子包，但既不是本模块的 from，
	// 也不在本模块其他 from 的子树内 —— 整体搬移会把它遗漏/错位。
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/demo/demo.go":    "package demo\n",
		"internal/demo/sub/sub.go": "package sub\n",
	})
	writeTree(t, root, demoFixtureFiles())
	gpkgs := demoGoPackages(root)
	gpkgs["internal/demo/..."] = append(gpkgs["internal/demo/..."],
		GoPackage{ImportPath: "internal/demo/sub", Dir: filepath.Join(root, "internal/demo/sub"), Name: "sub"})
	v := &Verifier{Root: root, GoList: fakeGoList(gpkgs)}
	m := loadDemoManifest(t)

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "partial-move", "internal/demo/sub") {
		t.Fatalf("未纳入搬迁集的子包应报告 partial-move，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMissingForbiddenSharedFile(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.ForbiddenSharedFiles = []string{
		"internal/router/router.go",
		"internal/router/task.go",
		"internal/router/sync_task.go",
		// 缺 internal/container/container.go
	}

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "forbidden-shared-files", "internal/container/container.go") {
		t.Fatalf("缺少强制禁改文件应报告 forbidden-shared-files，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsWildcardForbiddenEntry(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.ForbiddenSharedFiles = append(m.ForbiddenSharedFiles, "internal/router/*")

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "forbidden-shared-files", "internal/router/*") {
		t.Fatalf("通配符禁改项必须被拒绝（仅允许精确路径），得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsEmptyTestCommands(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.TestCommands = nil

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "test-commands", "") {
		t.Fatalf("test_commands 为空应报告 test-commands 诊断，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMoveSourcesMismatch(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.OwnedFiles.MoveSources = nil // 与 move_packages.from 不一致

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "owned-move-sources", "internal/demo") {
		t.Fatalf("move_sources 与 from 不一致应报告 owned-move-sources，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMissingModuleFiles(t *testing.T) {
	v, root := newDemoVerifier(t)
	m := loadDemoManifest(t)
	if err := os.Remove(filepath.Join(root, "internal", "modules", "demo", "module.go")); err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "owned-module-files", "module.go") {
		t.Fatalf("module_files 不存在应报告 owned-module-files，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMissingLegacyFile(t *testing.T) {
	v, root := newDemoVerifier(t)
	m := loadDemoManifest(t)
	if err := os.Remove(filepath.Join(root, "internal", "application", "service", "demo.go")); err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "legacy-file", "demo.go") {
		t.Fatalf("legacy_files 指向的文件不存在应报告 legacy-file，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyRejectsMoveTargetOutsideModuleNamespace(t *testing.T) {
	v, _ := newDemoVerifier(t)
	m := loadDemoManifest(t)
	m.MovePackages[0].To = "internal/elsewhere"

	ds := SortDiagnostics(v.VerifyManifest(m))
	if !hasDiagnostic(ds, "move-target", "internal/modules/demo") {
		t.Fatalf("to 不在 internal/modules/<module> 下应报告 move-target，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyAllRejectsDuplicateOwnership(t *testing.T) {
	v, _ := newDemoVerifier(t)
	a := loadDemoManifest(t)

	bYAML := strings.Replace(demoManifestYAML, "module: demo", "module: beta", 1)
	bYAML = strings.Replace(bYAML, "B-demo", "B-beta", -1)
	bPath := filepath.Join(t.TempDir(), "beta.yaml")
	if err := os.WriteFile(bPath, []byte(bYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadManifestStrict(bPath)
	if err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyAll([]*MoveManifest{a, b}))
	if !hasDiagnostic(ds, "ownership-overlap", "internal/demo") {
		t.Fatalf("两份 manifest 声明同一 from 应报告 ownership-overlap，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyAllRejectsDuplicateTarget(t *testing.T) {
	v, _ := newDemoVerifier(t)
	a := loadDemoManifest(t)

	bYAML := strings.Replace(demoManifestYAML, "module: demo", "module: beta", 1)
	bYAML = strings.Replace(bYAML, "from: internal/demo", "from: internal/beta", 1)
	bYAML = strings.Replace(bYAML, "  - internal/demo\n", "  - internal/beta\n", 1)
	bYAML = strings.Replace(bYAML, "B-demo", "B-beta", -1)
	bPath := filepath.Join(t.TempDir(), "beta.yaml")
	if err := os.WriteFile(bPath, []byte(bYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadManifestStrict(bPath)
	if err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyAll([]*MoveManifest{a, b}))
	if !hasDiagnostic(ds, "ownership-overlap", "internal/modules/demo") {
		t.Fatalf("两份 manifest 声明同一 to 应报告 ownership-overlap，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyAllRejectsDuplicateLegacyPath(t *testing.T) {
	v, _ := newDemoVerifier(t)
	a := loadDemoManifest(t)

	bYAML := strings.Replace(demoManifestYAML, "module: demo", "module: beta", 1)
	bYAML = strings.Replace(bYAML, "from: internal/demo", "from: internal/beta", 1)
	bYAML = strings.Replace(bYAML, "  - internal/demo\n", "  - internal/beta\n", 1)
	bYAML = strings.Replace(bYAML, "B-demo", "B-beta", -1)
	bPath := filepath.Join(t.TempDir(), "beta.yaml")
	if err := os.WriteFile(bPath, []byte(bYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadManifestStrict(bPath)
	if err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyAll([]*MoveManifest{a, b}))
	if !hasDiagnostic(ds, "ownership-overlap", "internal/application/service/demo.go") {
		t.Fatalf("两份 manifest 声明同一 legacy 路径应报告 ownership-overlap，得到:\n%s", joinDiagnostics(ds))
	}
}

func TestVerifyAllRejectsNestedFromAcrossManifests(t *testing.T) {
	// from A 是 from B 的路径前缀：物理整树搬移会踩到另一模块声明的包。
	v, _ := newDemoVerifier(t)
	a := loadDemoManifest(t)

	bYAML := strings.Replace(demoManifestYAML, "from: internal/demo", "from: internal/demo/sub", 1)
	bYAML = strings.Replace(bYAML, "to: internal/modules/demo", "to: internal/modules/beta/sub", 1)
	bYAML = strings.Replace(bYAML, "  - internal/demo\n", "  - internal/demo/sub\n", 1)
	bYAML = strings.Replace(bYAML, "module: demo", "module: beta", 1)
	bYAML = strings.Replace(bYAML, "B-demo", "B-beta", -1)
	bPath := filepath.Join(t.TempDir(), "beta.yaml")
	if err := os.WriteFile(bPath, []byte(bYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadManifestStrict(bPath)
	if err != nil {
		t.Fatal(err)
	}

	ds := SortDiagnostics(v.VerifyAll([]*MoveManifest{a, b}))
	if !hasDiagnostic(ds, "ownership-overlap", "internal/demo/sub") {
		t.Fatalf("跨 manifest 的 from 嵌套应报告 ownership-overlap，得到:\n%s", joinDiagnostics(ds))
	}
}

func joinDiagnostics(ds []Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.String())
		b.WriteString("\n")
	}
	return b.String()
}
