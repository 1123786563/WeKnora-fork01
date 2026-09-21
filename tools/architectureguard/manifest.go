// manifest.go owns the strict backend module manifest loader: the YAML
// schema is modelled with explicit structs and decoded with
// KnownFields(true) so that any schema drift between the manifest producer
// and this guard fails loudly instead of being silently ignored.
package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Asset kinds recorded in docs/architecture/backend-modules.yaml.
const (
	KindRoute      = "route"
	KindWorker     = "worker"
	KindLifecycle  = "lifecycle"
	KindMigration  = "migration"
	KindPackage    = "package"
	PlatformOwner  = "platform"
	schemaVersion1 = 1
)

// Manifest is the parsed backend-modules.yaml.
type Manifest struct {
	SchemaVersion       int                  `yaml:"schema_version"`
	Scope               Scope                `yaml:"scope"`
	Modules             []Module             `yaml:"modules"`
	Platform            Platform             `yaml:"platform"`
	Assets              []Asset              `yaml:"assets"`
	TemporaryExceptions []TemporaryException `yaml:"temporary_exceptions"`

	// CurrentWave is provided by the CLI -wave flag, not by the YAML; it
	// drives temporary-exception expiry checks inside Check.
	CurrentWave int `yaml:"-"`
}

// Scope restricts filesystem discovery.
type Scope struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// Module is one business module with its target directory prefix.
type Module struct {
	ID           string `yaml:"id"`
	TargetPrefix string `yaml:"target_prefix"`
}

// Platform records the shared-platform owner and its prefixes.
type Platform struct {
	ID             string   `yaml:"id"`
	TargetPrefixes []string `yaml:"target_prefixes"`
}

// Asset is one owned backend asset. Symbol applies to route/lifecycle
// assets, TaskType to worker assets; migration/package assets use neither.
type Asset struct {
	Kind     string `yaml:"kind"`
	ID       string `yaml:"id"`
	Owner    string `yaml:"owner"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Symbol   string `yaml:"symbol"`
	TaskType string `yaml:"task_type"`
}

// TemporaryException permits one exact from-prefix -> to-prefix dependency
// until the wave named by RemoveInWave.
type TemporaryException struct {
	From         string `yaml:"from"`
	To           string `yaml:"to"`
	Reason       string `yaml:"reason"`
	RemoveInWave int    `yaml:"remove_in_wave"`
}

// LoadManifest reads and strictly validates the manifest at filename.
func LoadManifest(filename string) (*Manifest, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate enforces the manifest invariants: known schema version, sane
// module ids and prefixes, unique asset ids, known owners, non-empty
// sources/targets, per-kind required fields, and exact-prefix exceptions.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != schemaVersion1 {
		return fmt.Errorf("unsupported schema_version %d (want %d)", m.SchemaVersion, schemaVersion1)
	}
	if len(m.Scope.Include) == 0 {
		return fmt.Errorf("scope.include must not be empty")
	}
	for _, dir := range append(append([]string{}, m.Scope.Include...), m.Scope.Exclude...) {
		if _, err := normalizeRelPath(dir); err != nil {
			return fmt.Errorf("scope path %q: %w", dir, err)
		}
	}

	prefixes := map[string]string{} // owner id -> target prefix (modules only)
	seenModules := map[string]bool{}
	for _, mod := range m.Modules {
		if mod.ID == "" {
			return fmt.Errorf("empty module id")
		}
		if seenModules[mod.ID] {
			return fmt.Errorf("duplicate module id: %s", mod.ID)
		}
		seenModules[mod.ID] = true
		clean, err := normalizeRelPath(mod.TargetPrefix)
		if err != nil {
			return fmt.Errorf("module %s target_prefix: %w", mod.ID, err)
		}
		if clean == "" {
			return fmt.Errorf("module %s: empty target_prefix", mod.ID)
		}
		mod.TargetPrefix = clean
		prefixes[mod.ID] = clean
	}

	if m.Platform.ID == "" {
		return fmt.Errorf("platform id must not be empty")
	}
	if len(m.Platform.TargetPrefixes) == 0 {
		return fmt.Errorf("platform target_prefixes must not be empty")
	}
	for i, p := range m.Platform.TargetPrefixes {
		clean, err := normalizeRelPath(p)
		if err != nil {
			return fmt.Errorf("platform target_prefix %q: %w", p, err)
		}
		m.Platform.TargetPrefixes[i] = clean
	}

	seenAssets := map[string]bool{}
	for i := range m.Assets {
		a := &m.Assets[i]
		switch a.Kind {
		case KindRoute, KindWorker, KindLifecycle, KindMigration, KindPackage:
		default:
			return fmt.Errorf("asset %q: unknown kind %q", a.ID, a.Kind)
		}
		if a.ID == "" {
			return fmt.Errorf("asset #%d: empty id", i+1)
		}
		if seenAssets[a.ID] {
			return fmt.Errorf("duplicate asset id: %s", a.ID)
		}
		seenAssets[a.ID] = true
		if a.Owner == "" {
			return fmt.Errorf("asset %s: empty owner", a.ID)
		}
		if a.Owner != m.Platform.ID {
			if _, ok := prefixes[a.Owner]; !ok {
				return fmt.Errorf("asset %s: unknown owner: %s", a.ID, a.Owner)
			}
		}
		if a.Source == "" {
			return fmt.Errorf("asset %s: empty source", a.ID)
		}
		cleanSource, err := normalizeRelPath(a.Source)
		if err != nil {
			return fmt.Errorf("asset %s source: %w", a.ID, err)
		}
		a.Source = cleanSource
		if a.Target == "" {
			return fmt.Errorf("asset %s: empty target", a.ID)
		}
		cleanTarget, err := normalizeRelPath(a.Target)
		if err != nil {
			return fmt.Errorf("asset %s target: %w", a.ID, err)
		}
		a.Target = cleanTarget
		switch a.Kind {
		case KindRoute, KindLifecycle:
			if a.Symbol == "" {
				return fmt.Errorf("asset %s: kind %s requires symbol", a.ID, a.Kind)
			}
		case KindWorker:
			if a.TaskType == "" {
				return fmt.Errorf("asset %s: kind %s requires task_type", a.ID, a.Kind)
			}
		}
	}

	for i := range m.TemporaryExceptions {
		e := &m.TemporaryExceptions[i]
		if e.From == "" || e.To == "" {
			return fmt.Errorf("temporary exception #%d: empty from/to", i+1)
		}
		for _, p := range []string{e.From, e.To} {
			if strings.ContainsAny(p, "*?[") {
				return fmt.Errorf("exception paths must be exact prefixes: %s", p)
			}
		}
		cleanFrom, err := normalizeRelPath(e.From)
		if err != nil {
			return fmt.Errorf("temporary exception %s -> %s: %w", e.From, e.To, err)
		}
		e.From = cleanFrom
		cleanTo, err := normalizeRelPath(e.To)
		if err != nil {
			return fmt.Errorf("temporary exception %s -> %s: %w", e.From, e.To, err)
		}
		e.To = cleanTo
		if e.RemoveInWave < 1 {
			return fmt.Errorf("temporary exception %s -> %s: remove_in_wave must be >= 1", e.From, e.To)
		}
	}
	return nil
}

// OwnerPrefixes returns the target prefixes an owner's assets may point at.
func (m *Manifest) OwnerPrefixes(owner string) []string {
	if owner == m.Platform.ID {
		return m.Platform.TargetPrefixes
	}
	for _, mod := range m.Modules {
		if mod.ID == owner {
			return []string{mod.TargetPrefix}
		}
	}
	return nil
}

// SortedModuleIDs returns module ids in stable order.
func (m *Manifest) SortedModuleIDs() []string {
	ids := make([]string, 0, len(m.Modules))
	for _, mod := range m.Modules {
		ids = append(ids, mod.ID)
	}
	sort.Strings(ids)
	return ids
}

// normalizeRelPath validates a repository-relative slash path and returns
// its path.Clean form. Absolute paths and paths escaping the repository
// root are rejected.
func normalizeRelPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("absolute path %q is not repository-relative", p)
	}
	if strings.Contains(p, "\\") {
		return "", fmt.Errorf("path %q must use forward slashes", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf(`path %q escapes the repository root with ".."`, p)
	}
	if clean == "." {
		return "", fmt.Errorf("path %q resolves to the repository root", p)
	}
	return clean, nil
}
