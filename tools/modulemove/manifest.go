package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MoveManifest 与 docs/architecture/moves/README.md 的 LOCKED schema 一一对应。
// 字段加载必须走 KnownFields(true)（见 LoadManifestStrict）：schema 外字段即报错。
type MoveManifest struct {
	Module               string            `yaml:"module"`
	Description          string            `yaml:"description"`
	MovePackages         []MovePackage     `yaml:"move_packages"`
	AliasObligations     []AliasObligation `yaml:"alias_obligations"`
	LegacyFiles          []LegacyFile      `yaml:"legacy_files"`
	OwnedFiles           OwnedFiles        `yaml:"owned_files"`
	TestCommands         []string          `yaml:"test_commands"`
	IntegrationPoints    IntegrationPoints `yaml:"integration_points"`
	ForbiddenSharedFiles []string          `yaml:"forbidden_shared_files"`
}

// MovePackage 声明一个整体独占的 Go 包搬迁。
type MovePackage struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// AliasObligation 声明旧路径上的无逻辑转发别名包（Pass B 删除）。
type AliasObligation struct {
	OldImportPath string `yaml:"old_import_path"`
	PassBTask     string `yaml:"passb_task"`
}

// LegacyFile 声明横向 host 包内归本模块的单个遗留文件（非测试 .go）。
type LegacyFile struct {
	Path            string `yaml:"path"`
	Reason          string `yaml:"reason"`
	NavigationLabel string `yaml:"navigation_label"`
	PassBTask       string `yaml:"passb_task"`
}

// OwnedFiles 声明本模块 Pass A worker 可创建/修改的全部文件。
type OwnedFiles struct {
	MoveSources []string `yaml:"move_sources"`
	MoveTargets []string `yaml:"move_targets"`
	Importers   []string `yaml:"importers"`
	ModuleFiles []string `yaml:"module_files"`
}

// IntegrationPoints 声明 F0 归属本模块的路由/worker/生命周期挂点。
type IntegrationPoints struct {
	Routes         []string `yaml:"routes"`
	Workers        []string `yaml:"workers"`
	LifecycleHooks []string `yaml:"lifecycle_hooks"`
}

// LoadManifestStrict 以 KnownFields(true) 严格加载一份 manifest。
// 任何 schema 外字段、格式错误都返回错误。
func LoadManifestStrict(path string) (*MoveManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var m MoveManifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("strict decode %s: %w", path, err)
	}
	if m.Module == "" {
		return nil, fmt.Errorf("strict decode %s: module 字段为空", path)
	}
	return &m, nil
}

// ListManifestModules 返回 manifest 目录下全部模块 id（排序，剔除非 .yaml 文件）。
func ListManifestModules(root string) ([]string, error) {
	dir := filepath.Join(root, ManifestsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifests dir: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(ids)
	return ids, nil
}

// ManifestPath 返回模块 manifest 的仓库相对路径。
func ManifestPath(root, module string) string {
	return root + "/" + ManifestsDir + "/" + module + ".yaml"
}

// validModuleID 校验模块 id 形态（小写字母数字，可含连字符）。
func validModuleID(id string) bool {
	if id == "" || strings.ContainsAny(id, "/\\") {
		return false
	}
	for _, r := range id {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isDash := r == '-'
		if !isLower && !isDigit && !isDash {
			return false
		}
	}
	return true
}

// sortStrings 就地排序并返回（诊断排序复用）。
func sortStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
