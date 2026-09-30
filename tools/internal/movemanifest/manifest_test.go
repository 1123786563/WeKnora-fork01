package movemanifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/tools/internal/movemanifest"
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

// demoManifestYAML 是覆盖 LOCKED schema 全部字段的合法 manifest 样例。
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

func TestLoadManifestStrictAcceptsCompleteManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := os.WriteFile(path, []byte(demoManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := movemanifest.LoadManifestStrict(path)
	if err != nil {
		t.Fatalf("合法 manifest 不应报错: %v", err)
	}
	if m.Module != "demo" {
		t.Fatalf("module = %q, want demo", m.Module)
	}
	if len(m.MovePackages) != 1 || m.MovePackages[0].From != "internal/demo" ||
		m.MovePackages[0].To != "internal/modules/demo" {
		t.Fatalf("move_packages 解析错误: %+v", m.MovePackages)
	}
	if len(m.AliasObligations) != 1 || m.AliasObligations[0].OldImportPath != "internal/demo" {
		t.Fatalf("alias_obligations 解析错误: %+v", m.AliasObligations)
	}
	if len(m.LegacyFiles) != 1 || m.LegacyFiles[0].Path != "internal/application/service/demo.go" {
		t.Fatalf("legacy_files 解析错误: %+v", m.LegacyFiles)
	}
	if len(m.IntegrationPoints.Workers) != 0 || len(m.ForbiddenSharedFiles) != 7 {
		t.Fatalf("integration_points/forbidden_shared_files 解析错误: %+v / %+v",
			m.IntegrationPoints, m.ForbiddenSharedFiles)
	}
}

func TestLoadManifestStrictRejectsUnknownField(t *testing.T) {
	yaml := strings.Replace(demoManifestYAML, "description: demo module for tests",
		"description: demo module for tests\nbogus_field: nope", 1)
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := movemanifest.LoadManifestStrict(path)
	if err == nil {
		t.Fatal("schema 外字段（bogus_field）必须被 KnownFields(true) 拒绝")
	}
	if !strings.Contains(err.Error(), "bogus_field") {
		t.Fatalf("错误信息应指出未知字段名，得到: %v", err)
	}
}

func TestLoadManifestStrictRejectsUnknownNestedField(t *testing.T) {
	yaml := strings.Replace(demoManifestYAML,
		"  - old_import_path: internal/demo\n    passb_task: B-demo",
		"  - old_import_path: internal/demo\n    passb_task: B-demo\n    surprise: true", 1)
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := movemanifest.LoadManifestStrict(path)
	if err == nil {
		t.Fatal("嵌套 mapping 中的 schema 外字段同样必须被拒绝")
	}
	if !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("错误信息应指出嵌套未知字段名，得到: %v", err)
	}
}

func TestListManifestModulesSortedAndSkipsReadme(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/architecture/moves/README.md":      "# docs\n",
		"docs/architecture/moves/zeta.yaml":      demoManifestYAML,
		"docs/architecture/moves/alpha.yaml":     strings.Replace(demoManifestYAML, "module: demo", "module: alpha", 1),
		"docs/architecture/moves/not-a-move.txt": "skip me",
	})

	mods, err := movemanifest.ListManifestModules(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "zeta"}
	if len(mods) != len(want) {
		t.Fatalf("modules = %v, want %v", mods, want)
	}
	for i := range want {
		if mods[i] != want[i] {
			t.Fatalf("modules = %v, want %v", mods, want)
		}
	}
}
