package experts

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"gopkg.in/yaml.v3"
)

const (
	manifestFileName = "manifest.yaml"
	skillsDirName    = "skills"
	skillDocFileName = "SKILL.md"
)

// ScanExperts scans dir for expert packages laid out as
// <dir>/<id>/manifest.yaml (+ persona documents referenced by
// prompt_files + optional skills/<slug>/SKILL.md directories).
//
// Contract:
//   - Subdirectories without a manifest.yaml are skipped, not an error —
//     the scanner stays log-free so the caller can decide policy.
//   - Non-directory entries at the scan root are ignored.
//   - Duplicate manifest IDs are an error.
//   - Every prompt_files entry must be a plain base name (no separators,
//     no "..") and must exist with non-empty content; contents are loaded
//     into Expert.PersonaFiles here.
//   - Every skills entry must be a plain base name with a matching
//     skills/<slug>/ directory containing SKILL.md; SkillDirs carries the
//     absolute directory paths.
//
// The returned experts are ordered by directory name. An unreadable root
// directory (including a missing one) is returned as an error.
func ScanExperts(dir string) ([]*Expert, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("experts: scan %s: %w", dir, err)
	}

	var experts []*Expert
	seen := make(map[string]string) // expert ID → directory it came from
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		expertDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(expertDir, manifestFileName)
		if _, err := os.Stat(manifestPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // not an expert package; caller decides policy
			}
			return nil, fmt.Errorf("experts: stat %s: %w", manifestPath, err)
		}
		e, err := scanExpertDir(expertDir, manifestPath)
		if err != nil {
			return nil, err
		}
		if prevDir, dup := seen[e.Manifest.ID]; dup {
			return nil, fmt.Errorf("experts: duplicate expert id %q in %s (already seen in %s)",
				e.Manifest.ID, expertDir, prevDir)
		}
		seen[e.Manifest.ID] = expertDir
		experts = append(experts, e)
	}
	return experts, nil
}

// scanExpertDir loads and validates one expert package directory.
func scanExpertDir(expertDir, manifestPath string) (*Expert, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("experts: read %s: %w", manifestPath, err)
	}
	var m ExpertManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("experts: parse %s: %w", manifestPath, err)
	}
	if m.ID == "" {
		return nil, fmt.Errorf("experts: %s: manifest id is empty", manifestPath)
	}

	e := &Expert{
		Manifest:     m,
		PersonaFiles: make(map[string][]byte, len(m.PromptFiles)),
		SkillDirs:    make(map[string]string, len(m.Skills)),
	}

	for _, name := range m.PromptFiles {
		if !isSafeBaseName(name) {
			return nil, fmt.Errorf("experts: %s: prompt_files entry %q must be a plain file name", manifestPath, name)
		}
		content, err := os.ReadFile(filepath.Join(expertDir, name))
		if err != nil {
			return nil, fmt.Errorf("experts: %s: prompt file %q: %w", manifestPath, name, err)
		}
		if len(content) == 0 {
			return nil, fmt.Errorf("experts: %s: prompt file %q is empty", manifestPath, name)
		}
		e.PersonaFiles[name] = content
	}

	for _, slug := range m.Skills {
		if !isSafeBaseName(slug) {
			return nil, fmt.Errorf("experts: %s: skills entry %q must be a plain directory name", manifestPath, slug)
		}
		skillDir := filepath.Join(expertDir, skillsDirName, slug)
		if err := requireSkillDir(skillDir); err != nil {
			return nil, fmt.Errorf("experts: %s: skill %q: %w", manifestPath, slug, err)
		}
		abs, err := filepath.Abs(skillDir)
		if err != nil {
			return nil, fmt.Errorf("experts: %s: skill %q: %w", manifestPath, slug, err)
		}
		e.SkillDirs[slug] = abs
	}

	return e, nil
}

// requireSkillDir validates that skillDir is a directory containing SKILL.md.
func requireSkillDir(skillDir string) error {
	info, err := os.Stat(skillDir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no directory %s", skillDir)
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", skillDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", skillDir)
	}
	doc := filepath.Join(skillDir, skillDocFileName)
	docInfo, err := os.Stat(doc)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("missing %s in %s", skillDocFileName, skillDir)
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", doc, err)
	}
	if docInfo.IsDir() {
		return fmt.Errorf("%s is not a file", doc)
	}
	return nil
}

// isSafeBaseName reports whether name is a single, non-escaping path
// element: non-empty, not "." or "..", no path separators (either slash
// convention, so a windows-authored manifest cannot escape on unix), and no
// NUL bytes.
func isSafeBaseName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	return filepath.Base(name) == name
}

var (
	builtinOnce    sync.Once
	builtinExperts []*Expert
)

// LoadBuiltinExperts returns the process-level cached scan of the builtin
// expert library at <ConfigDir()>/experts. The scan runs on the first call,
// never at import time. A missing directory is the normal "no experts
// shipped" case; any other scan error is logged and swallowed — builtin
// experts are optional data and must never block startup (degraded mode:
// an empty, non-nil slice).
//
// The returned Experts are shared; treat them as read-only.
func LoadBuiltinExperts() []*Expert {
	builtinOnce.Do(func() {
		builtinExperts = loadBuiltinExpertsFrom(filepath.Join(config.ConfigDir(), "experts"))
	})
	return builtinExperts
}

// loadBuiltinExpertsFrom performs the degraded-mode load of dir used by
// LoadBuiltinExperts (exposed separately for tests).
func loadBuiltinExpertsFrom(dir string) []*Expert {
	experts, err := ScanExperts(dir)
	if err != nil {
		// Absence is normal; anything else is worth a warning but not fatal.
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warnf(context.Background(), "[Experts] builtin experts unavailable (dir=%s): %v", dir, err)
		}
		return []*Expert{}
	}
	if experts == nil {
		return []*Expert{}
	}
	return experts
}
