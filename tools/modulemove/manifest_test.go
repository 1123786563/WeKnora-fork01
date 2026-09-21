package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestStrictAcceptsCompleteManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := os.WriteFile(path, []byte(demoManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifestStrict(path)
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

	_, err := LoadManifestStrict(path)
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

	_, err := LoadManifestStrict(path)
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

	mods, err := ListManifestModules(root)
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
