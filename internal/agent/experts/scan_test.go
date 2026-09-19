package experts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanRoot is the happy-path fixture root: sample-expert plus a
// manifest-less directory and a stray file that must both be ignored.
func scanRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "experts")
}

func brokenRoot(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name, "experts")
}

func TestScanExpertsHappyPath(t *testing.T) {
	got, err := ScanExperts(scanRoot(t))
	if err != nil {
		t.Fatalf("ScanExperts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 expert, got %d: %+v", len(got), got)
	}
	e := got[0]

	// Manifest fields round-trip.
	m := e.Manifest
	if m.ID != "sample-expert" {
		t.Errorf("ID = %q, want sample-expert", m.ID)
	}
	if m.Label["zh"] != "示例专家" || m.Label["en"] != "Sample Expert" {
		t.Errorf("Label = %v", m.Label)
	}
	if m.Description["zh"] != "用于扫描器测试的示例专家" || m.Description["en"] != "A sample expert used by scanner tests" {
		t.Errorf("Description = %v", m.Description)
	}
	if m.IconName != "sparkles" || m.Color != "#6366f1" || m.PersonaMBTI != "INTJ" {
		t.Errorf("icon/color/mbti = %q/%q/%q", m.IconName, m.Color, m.PersonaMBTI)
	}
	if len(m.PromptFiles) != 1 || m.PromptFiles[0] != "SOUL.md" {
		t.Errorf("PromptFiles = %v", m.PromptFiles)
	}
	if len(m.QuickPrompts) != 2 {
		t.Fatalf("QuickPrompts len = %d, want 2", len(m.QuickPrompts))
	}
	q := m.QuickPrompts[0]
	if q.Title["zh"] != "第一条" || q.Title["en"] != "First one" {
		t.Errorf("QuickPrompts[0].Title = %v", q.Title)
	}
	if q.Description["en"] != "First description" || q.Prompt["zh"] != "帮我做一件事" {
		t.Errorf("QuickPrompts[0].Description/Prompt = %v/%v", q.Description, q.Prompt)
	}
	if q.Color != "#22c55e" || q.IconName != "zap" {
		t.Errorf("QuickPrompts[0].color/icon = %q/%q", q.Color, q.IconName)
	}
	if len(m.Skills) != 1 || m.Skills[0] != "sample-skill" {
		t.Errorf("Skills = %v", m.Skills)
	}
	ac := m.AgentConfig
	if ac.AgentMode != "smart-reasoning" || ac.KBSelectionMode != "all" {
		t.Errorf("AgentMode/KBSelectionMode = %q/%q", ac.AgentMode, ac.KBSelectionMode)
	}
	if ac.Temperature != 0.7 || ac.MaxIterations != 5 {
		t.Errorf("Temperature/MaxIterations = %v/%d", ac.Temperature, ac.MaxIterations)
	}
	if ac.SystemPrompt != "You are a sample expert. Be precise." {
		t.Errorf("SystemPrompt = %q", ac.SystemPrompt)
	}
	if !ac.WebSearchEnabled || !ac.MultiTurnEnabled {
		t.Errorf("web/multi-turn = %v/%v", ac.WebSearchEnabled, ac.MultiTurnEnabled)
	}

	// Persona file loaded into memory.
	soul, ok := e.PersonaFiles["SOUL.md"]
	if !ok {
		t.Fatalf("PersonaFiles missing SOUL.md: %v", e.PersonaFiles)
	}
	if len(soul) == 0 || !strings.Contains(string(soul), "sample expert") {
		t.Errorf("SOUL.md content unexpected: %q", soul)
	}

	// Skill dir resolved to an absolute path containing SKILL.md.
	dir, ok := e.SkillDirs["sample-skill"]
	if !ok {
		t.Fatalf("SkillDirs missing sample-skill: %v", e.SkillDirs)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("SkillDirs entry not absolute: %q", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md under %q: %v", dir, err)
	}
}

func TestScanExpertsSkipsDirWithoutManifest(t *testing.T) {
	got, err := ScanExperts(scanRoot(t))
	if err != nil {
		t.Fatalf("ScanExperts: %v", err)
	}
	for _, e := range got {
		if e.Manifest.ID == "no-manifest" {
			t.Errorf("manifest-less directory was scanned: %+v", e.Manifest)
		}
	}
	if len(got) != 1 || got[0].Manifest.ID != "sample-expert" {
		t.Errorf("want only sample-expert, got %+v", got)
	}
}

func TestScanExpertsMissingPromptFile(t *testing.T) {
	_, err := ScanExperts(brokenRoot(t, "broken-missing-prompt"))
	if err == nil {
		t.Fatal("want error for missing prompt file, got nil")
	}
	if !strings.Contains(err.Error(), "ABSENT.md") {
		t.Errorf("error should mention ABSENT.md, got: %v", err)
	}
}

func TestScanExpertsMissingSkillDir(t *testing.T) {
	_, err := ScanExperts(brokenRoot(t, "broken-missing-skill"))
	if err == nil {
		t.Fatal("want error for skill without directory, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-skill") {
		t.Errorf("error should mention ghost-skill, got: %v", err)
	}
}

func TestScanExpertsDuplicateID(t *testing.T) {
	_, err := ScanExperts(brokenRoot(t, "broken-dup"))
	if err == nil {
		t.Fatal("want error for duplicate expert id, got nil")
	}
	if !strings.Contains(err.Error(), "dup-expert") {
		t.Errorf("error should mention dup-expert, got: %v", err)
	}
}

// TestScanExpertsRejectsPathTraversal pins the no-escape rule: prompt file
// names and skill slugs must be single path elements, even when the target
// happens to exist on disk outside the expert directory.
func TestScanExpertsRejectsPathTraversal(t *testing.T) {
	cases := []struct {
		name       string
		manifestID string
		promptFile string
		skill      string
	}{
		{"parent escape prompt", "escape-prompt", "../escape.md", ""},
		{"nested path prompt", "nested-prompt", "sub/dir.md", ""},
		{"parent escape skill", "escape-skill", "", "../stolen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			expertDir := filepath.Join(root, "expert")
			if err := os.MkdirAll(filepath.Join(expertDir, "skills"), 0o755); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			b.WriteString("id: " + tc.manifestID + "\n")
			if tc.promptFile != "" {
				b.WriteString("prompt_files:\n  - " + tc.promptFile + "\n")
			}
			if tc.skill != "" {
				b.WriteString("skills:\n  - " + tc.skill + "\n")
			}
			if err := os.WriteFile(filepath.Join(expertDir, "manifest.yaml"), []byte(b.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			// The escape targets DO exist (outside the expert dir) so the
			// rejection has to come from name validation, not file absence.
			if err := os.WriteFile(filepath.Join(root, "escape.md"), []byte("outside"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, "stolen"), 0o755); err != nil {
				t.Fatal(err)
			}

			if _, err := ScanExperts(root); err == nil {
				t.Fatalf("want error for %q, got nil", tc.name)
			}
		})
	}
}

func TestLoadBuiltinExpertsDegradedOnScanError(t *testing.T) {
	got := loadBuiltinExpertsFrom(brokenRoot(t, "broken-missing-prompt"))
	if got == nil || len(got) != 0 {
		t.Errorf("want non-nil empty slice on scan error, got %#v", got)
	}
}

func TestLoadBuiltinExpertsEmptyOnMissingDir(t *testing.T) {
	got := loadBuiltinExpertsFrom(filepath.Join("testdata", "no-such-experts-dir"))
	if got == nil || len(got) != 0 {
		t.Errorf("want non-nil empty slice on missing dir, got %#v", got)
	}
}
