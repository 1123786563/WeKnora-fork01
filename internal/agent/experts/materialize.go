// Skillset materialization (M4 Task 2): port of Octop's
// skillhub_market.py install path (_install_skillset_template_locked +
// _write_expert_template + _download_skill_into_template). A SkillHub
// skillset becomes the same on-disk shape ScanExperts already reads for
// builtin experts — manifest.yaml + persona documents + skills/<slug>/ —
// so the catalog, the detail view and Instantiate treat market experts and
// builtin experts identically from there on.
package experts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Tencent/WeKnora/internal/agent/skills/skillhub"
	"github.com/Tencent/WeKnora/internal/types"
)

// soulFileName is the single persona document a materialized expert ships;
// it is what manifest.yaml's prompt_files names.
const soulFileName = "SOUL.md"

// soulSummaryFallback mirrors Octop's _expert_soul default when the skillset
// carries no usable summary.
const soulSummaryFallback = "围绕该 SkillHub skillset 提供专家级工作流支持。"

// MaterializedExpert is the in-memory form of one installed skillset: the
// expert manifest, the persona documents and the per-skill package payloads
// (ParsePackage output). It is produced by the pure MaterializeSkillset and
// serialized by WriteMaterializedExpert; nothing here touches the network or
// the disk by itself.
type MaterializedExpert struct {
	// Manifest is the ExpertManifest the materialized directory ships.
	Manifest ExpertManifest
	// PersonaFiles maps the prompt-file base names to their contents; today
	// that is exactly SOUL.md.
	PersonaFiles map[string][]byte
	// SkillFiles maps each bundled skill slug to its validated package
	// payloads (workspace-relative member names).
	SkillFiles map[string]skillhub.ZipFiles
}

// MarketExpertSlugFromID splits a market expert ID back into its skillset
// slug; ok is false for every other ID (builtins included).
func MarketExpertSlugFromID(id string) (string, bool) {
	return skillhub.MarketExpertSlugFromID(id)
}

// MaterializeSkillset turns one skillset listing into a MaterializedExpert.
// It is PURE: fetchSkill supplies every per-skill package (the caller wires
// it to skillhub Download + ParsePackage), so the mapping is table-testable.
//
// Mapping (the binding M4 contract, a deliberate slimming of Octop's
// _expert_manifest):
//   - Manifest.ID = "skillhub-skillset-<slug>" (skillhub.MarketExpertID),
//     which cannot collide with builtin IDs;
//   - Label/Description carry the skillset's zh/en metadata (displayName /
//     displayNameEn / summary / summaryEn from Raw) when present, else the
//     typed Name/Description pass through to both locales;
//   - PromptFiles = ["SOUL.md"], built per Octop's SOUL template from the
//     skillset description and the bundled-skill list;
//   - Skills = the deduplicated, trimmed skill slugs;
//   - AgentConfig = minimal smart-reasoning.
//
// Octop additionally derives quick prompts, task examples and scene-based
// icon/color from the workflow prompt inside the skillset zip; this port
// does not download the skillset zip (the listing already carries the skill
// slugs), so those decorations stay out of the materialized manifest.
func MaterializeSkillset(ss skillhub.SkillsetSummary, fetchSkill func(string) (skillhub.ZipFiles, error)) (*MaterializedExpert, error) {
	slug, err := skillhub.ValidateSkillsetSlug(ss.Slug)
	if err != nil {
		return nil, fmt.Errorf("experts: materialize skillset: %w", err)
	}

	slugs := make([]string, 0, len(ss.SkillSlugs))
	seen := make(map[string]bool, len(ss.SkillSlugs))
	for _, raw := range ss.SkillSlugs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		if !isSafeBaseName(trimmed) {
			return nil, fmt.Errorf("experts: skillset %q: skill slug %q must be a plain directory name", slug, raw)
		}
		seen[trimmed] = true
		slugs = append(slugs, trimmed)
	}
	if len(slugs) == 0 {
		return nil, fmt.Errorf("experts: skillset %q has no skills", slug)
	}

	skillFiles := make(map[string]skillhub.ZipFiles, len(slugs))
	for _, skillSlug := range slugs {
		files, err := fetchSkill(skillSlug)
		if err != nil {
			return nil, fmt.Errorf("experts: skillset %q: fetch skill %q: %w", slug, skillSlug, err)
		}
		if !hasRootSkillDoc(files) {
			return nil, fmt.Errorf("experts: skillset %q: skill %q package has no root SKILL.md", slug, skillSlug)
		}
		skillFiles[skillSlug] = files
	}

	manifest := ExpertManifest{
		ID:          skillhub.MarketExpertID(slug),
		Label:       LocaleText{"zh": localizedOr(ss.Raw, "displayName", ss.Name), "en": localizedOr(ss.Raw, "displayNameEn", ss.Name)},
		Description: LocaleText{"zh": localizedOr(ss.Raw, "summary", ss.Description), "en": localizedOr(ss.Raw, "summaryEn", ss.Description)},
		PromptFiles: []string{soulFileName},
		Skills:      slugs,
		AgentConfig: ExpertAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}

	return &MaterializedExpert{
		Manifest:     manifest,
		PersonaFiles: map[string][]byte{soulFileName: soulMarkdown(manifest, slug)},
		SkillFiles:   skillFiles,
	}, nil
}

// localizedOr picks a truthy localized string out of the raw skillset entry,
// falling back to the typed passthrough value (Python `raw.get(k) or
// fallback` truthiness semantics over the original JSON value).
func localizedOr(raw map[string]any, key, fallback string) string {
	if value, ok := raw[key]; ok {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			return strconv.FormatFloat(v, 'g', -1, 64)
		case bool:
			if v {
				return "true"
			}
		}
	}
	return fallback
}

// soulMarkdown renders the persona document, following Octop's _expert_soul
// template minus its workflow-orchestration bullet (that bullet points at
// skills/<skillset-slug>/SKILL.md, a workflow document Octop extracts from
// the skillset zip this port does not download). Built by concatenation
// because the markdown's code spans need literal backticks, which a Go raw
// string cannot carry.
func soulMarkdown(m ExpertManifest, slug string) []byte {
	name := m.Label["zh"]
	if name == "" {
		name = m.ID
	}
	summary := m.Description["zh"]
	if strings.TrimSpace(summary) == "" {
		summary = soulSummaryFallback
	}
	list := make([]string, 0, len(m.Skills))
	for _, skillSlug := range m.Skills {
		list = append(list, "- "+codeSpan(skillSlug))
	}
	soul := strings.Join([]string{
		"# " + name,
		"",
		"你是「" + name + "」，来源于 SkillHub skillset " + codeSpan(slug) + "。",
		"",
		"## 专家定位",
		"",
		summary,
		"",
		"## 工作方式",
		"",
		"- 根据用户目标主动拆解步骤、识别输入缺口，并给出可执行产物。",
		"- 需要具体能力时，调用已安装的配套技能；不要把技能清单当作用户可见负担。",
		"- 输出时保持结构清晰，先给结论和下一步，再补充必要依据。",
		"",
		"## 配套技能",
		"",
		strings.Join(list, "\n"),
	}, "\n") + "\n"
	return []byte(soul)
}

// codeSpan wraps text in markdown backticks.
func codeSpan(text string) string {
	return "`" + text + "`"
}

// hasRootSkillDoc reports whether a validated package carries the root
// SKILL.md ScanExperts requires under skills/<slug>/.
func hasRootSkillDoc(files skillhub.ZipFiles) bool {
	for _, f := range files {
		if f.Name == "SKILL.md" {
			return true
		}
	}
	return false
}

// materializedFile pairs one output path (relative to the install directory)
// with its bytes; the sorted slice is the full deterministic tree both the
// snapshot hash and the disk writer consume.
type materializedFile struct {
	name    string
	content []byte
}

// fileTree renders the whole install directory: manifest.yaml, the persona
// documents, then skills/<slug>/<member> for every bundled skill. Paths are
// re-validated here so a hand-built MaterializedExpert can never smuggle an
// escaping member into tenant storage (the zip validator should have caught
// these already — this is the defense in depth).
func (me *MaterializedExpert) fileTree() ([]materializedFile, error) {
	manifestYAML, err := yaml.Marshal(me.Manifest)
	if err != nil {
		return nil, fmt.Errorf("experts: encode manifest.yaml: %w", err)
	}
	files := []materializedFile{{name: "manifest.yaml", content: manifestYAML}}

	for _, name := range me.Manifest.PromptFiles {
		content, ok := me.PersonaFiles[name]
		if !ok || !isSafeBaseName(name) {
			return nil, fmt.Errorf("experts: persona file %q is missing or unsafe", name)
		}
		files = append(files, materializedFile{name: name, content: content})
	}

	skillSlugs := make(map[string]bool, len(me.Manifest.Skills))
	for _, slug := range me.Manifest.Skills {
		skillSlugs[slug] = true
	}
	sorted := make([]string, 0, len(me.SkillFiles))
	for slug := range me.SkillFiles {
		sorted = append(sorted, slug)
	}
	sort.Strings(sorted)
	for _, slug := range sorted {
		if !skillSlugs[slug] || !isSafeBaseName(slug) {
			return nil, fmt.Errorf("experts: refusing skill payload %q outside the manifest skill list", slug)
		}
		for _, member := range me.SkillFiles[slug] {
			if !isSafeRelativePath(member.Name) {
				return nil, fmt.Errorf("experts: refusing unsafe materialized path %q", "skills/"+slug+"/"+member.Name)
			}
			files = append(files, materializedFile{name: "skills/" + slug + "/" + member.Name, content: member.Content})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

// isSafeRelativePath accepts clean, relative, separator-normal member names:
// no absolute paths, no escaping segments, no NUL bytes, no drive letters.
func isSafeRelativePath(name string) bool {
	if name == "" || strings.ContainsRune(name, '\x00') || strings.ContainsRune(name, '\\') {
		return false
	}
	if name != filepath.ToSlash(filepath.Clean(name)) || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ":") {
			return false
		}
	}
	return true
}

// SnapshotSHA256 is the content digest of the materialized tree: a stable
// hash over every (path, bytes) pair, in sorted path order. The install
// ledger stores it so a reinstall of unchanged content is recognizable
// without re-reading tenant storage.
func (me *MaterializedExpert) SnapshotSHA256() string {
	files, err := me.fileTree()
	if err != nil {
		return ""
	}
	hash := sha256.New()
	for _, f := range files {
		hash.Write([]byte(f.name))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.Itoa(len(f.content))))
		hash.Write([]byte{0})
		hash.Write(f.content)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// WriteMaterializedExpert serializes me under installDir in the ScanExperts
// layout, atomically: the tree lands in a scratch directory first, then
// swaps into place (port of Octop's _replace_dir), so a crash mid-write can
// never leave a half-written expert for the catalog to scan.
//
// The scratch and backup directories live OUTSIDE the tenant's scan root
// (one level up, next to the tenant directory, dot-prefixed): a leftover
// inside the tenant directory would carry a manifest.yaml, read as a
// duplicate expert ID, and take the tenant's whole catalog down.
func WriteMaterializedExpert(installDir string, me *MaterializedExpert) error {
	files, err := me.fileTree()
	if err != nil {
		return err
	}

	tenantDir := filepath.Dir(installDir)
	if err := os.MkdirAll(tenantDir, 0o755); err != nil {
		return fmt.Errorf("experts: create market dir %s: %w", tenantDir, err)
	}
	// dataRoot is the tenant directory's parent: <dataRoot>/<tenantID>/<slug>
	// is the layout MarketInstallDir builds, so scratch siblings never fall
	// under a directory ScanExperts reads.
	dataRoot := filepath.Dir(tenantDir)
	suffix := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	staging := filepath.Join(dataRoot, ".expert-market-staging-"+suffix)
	backup := filepath.Join(dataRoot, ".expert-market-backup-"+suffix)
	if err := writeTree(staging, files); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := swapDir(staging, backup, installDir); err != nil {
		_ = os.RemoveAll(staging)
		_ = os.RemoveAll(backup)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

// writeTree writes every file under root, creating parent directories.
func writeTree(root string, files []materializedFile) error {
	for _, f := range files {
		target := filepath.Join(root, filepath.FromSlash(f.name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("experts: create %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, f.content, 0o644); err != nil {
			return fmt.Errorf("experts: write %s: %w", target, err)
		}
	}
	return nil
}

// swapDir atomically moves src in as dest: an existing dest is moved aside
// to backup first, and restored when the swap fails (port of Octop's
// _replace_dir; the caller owns cleaning up src and backup). All three
// paths must sit on the same filesystem, which the dataRoot placement above
// guarantees.
func swapDir(src, backup, dest string) error {
	hasBackup := false
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			return fmt.Errorf("experts: stage replacement of %s: %w", dest, err)
		}
		hasBackup = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("experts: stat %s: %w", dest, err)
	}
	if err := os.Rename(src, dest); err != nil {
		if hasBackup {
			_ = os.Rename(backup, dest)
		}
		return fmt.Errorf("experts: install %s: %w", dest, err)
	}
	return nil
}

// marketBaseDirEnv is the local-data root every local-storage consumer in
// the server reads (the file service's local backend, the desktop build's
// data dir). The expert market nests under it so one deployment has one
// data root.
const marketBaseDirEnv = "LOCAL_STORAGE_BASE_DIR"

// defaultMarketBaseDir matches the server default the storage backend uses
// when the env var is unset.
const defaultMarketBaseDir = "/data/files"

// MarketDataRoot resolves the expert-market root:
// <$LOCAL_STORAGE_BASE_DIR>/expert-market (default /data/files/expert-market).
// This is the sanctioned M4 layout choice — ScanExperts needs a real
// directory tree, which the blob-oriented tenant FileService cannot express,
// so materialized experts live under the same local-data base dir every
// other on-disk consumer uses, tenant-scoped one level down.
func MarketDataRoot() string {
	base := strings.TrimSpace(os.Getenv(marketBaseDirEnv))
	if base == "" {
		base = defaultMarketBaseDir
	}
	return filepath.Join(base, "expert-market")
}

// MarketTenantDir is the directory holding one tenant's installed experts:
// <dataRoot>/<tenantID>. Market dataRoot comes from the caller (the
// container resolves it once via MarketDataRoot); each install is a
// slug-named subdirectory.
func MarketTenantDir(dataRoot string, tenantID uint64) string {
	return filepath.Join(dataRoot, strconv.FormatUint(tenantID, 10))
}

// MarketInstallDir is one skillset's install directory:
// <dataRoot>/<tenantID>/<slug>. The slug is validated [A-Za-z0-9_.-]+ by
// MaterializeSkillset, so it is a safe single path segment.
func MarketInstallDir(dataRoot string, tenantID uint64, slug string) string {
	return filepath.Join(MarketTenantDir(dataRoot, tenantID), slug)
}

// InstalledExperts scans one tenant's materialized experts with the same
// scanner the builtin library uses. It is degraded-mode by design: a missing
// directory is "nothing installed", any other scan trouble is logged and
// swallowed — installed experts are optional data and must never break the
// catalog (the same contract LoadBuiltinExperts upholds).
func InstalledExperts(dataRoot string, tenantID uint64) []*Expert {
	return loadBuiltinExpertsFrom(MarketTenantDir(dataRoot, tenantID))
}
