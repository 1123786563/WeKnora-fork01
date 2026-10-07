package experts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pilot-library tests pin the four experts imported from Octop under
// config/experts at the repository root (see scripts/import_octop_experts.py).
// They assert the imported shape only; asset fidelity (byte-verbatim persona
// and skill files) is proven by the importer workflow, with one light content
// spot-check here.

// pilotRoot is the shipped builtin expert library at the repo root, reached
// from this package directory (internal/agent/experts).
func pilotRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "config", "experts")
}

// scanPilots scans config/experts and returns the experts keyed by ID,
// failing the test unless exactly the four pilots are present.
func scanPilots(t *testing.T) map[string]*Expert {
	t.Helper()
	got, err := ScanExperts(pilotRoot(t))
	if err != nil {
		t.Fatalf("ScanExperts(config/experts): %v", err)
	}
	wantOrder := []string{"general-assistant", "news-trend", "ops-engineer", "stock-assistant"}
	if len(got) != len(wantOrder) {
		t.Fatalf("want %d pilot experts, got %d", len(wantOrder), len(got))
	}
	byID := make(map[string]*Expert, len(got))
	for i, e := range got {
		if e.Manifest.ID != wantOrder[i] {
			t.Errorf("experts[%d].ID = %q, want %q (scanner orders by directory name)", i, e.Manifest.ID, wantOrder[i])
		}
		byID[e.Manifest.ID] = e
	}
	return byID
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pilotShape is the expected per-expert manifest shape. general-assistant
// ships 12 quick prompts in the Octop source while the other three ship 6;
// the importer passes all of them through (asset fidelity), so the counts
// pinned here are the real ones — the plan's "6 each" held for three of the
// four pilots only.
var pilotShape = map[string]struct {
	promptFiles  []string
	skills       []string
	quickPrompts int
}{
	"general-assistant": {promptFiles: []string{"SOUL.md"}, skills: nil, quickPrompts: 12},
	"news-trend":        {promptFiles: []string{"SOUL.md", "IDENTITY.md", "HEARTBEAT.md"}, skills: []string{"daily-hot-news"}, quickPrompts: 6},
	"ops-engineer":      {promptFiles: []string{"SOUL.md", "IDENTITY.md"}, skills: nil, quickPrompts: 6},
	"stock-assistant":   {promptFiles: []string{"SOUL.md", "IDENTITY.md", "HEARTBEAT.md"}, skills: []string{"stock-info"}, quickPrompts: 6},
}

// pilotAgentConfig is the fixed per-expert agent_config table applied by the
// importer. Unlisted fields stay zero ("leave CreateAgent defaults").
var pilotAgentConfig = map[string]ExpertAgentConfig{
	"general-assistant": {AgentMode: "smart-reasoning", KBSelectionMode: "all"},
	"ops-engineer":      {AgentMode: "smart-reasoning", KBSelectionMode: "all", WebSearchEnabled: true},
	"stock-assistant":   {AgentMode: "smart-reasoning", WebSearchEnabled: true},
	"news-trend":        {AgentMode: "smart-reasoning", WebSearchEnabled: true},
}

func TestPilotExpertsManifestShape(t *testing.T) {
	byID := scanPilots(t)
	for id, want := range pilotShape {
		e := byID[id]
		if e == nil {
			t.Fatalf("missing pilot expert %q", id)
		}
		m := e.Manifest
		if !eqStrings(m.PromptFiles, want.promptFiles) {
			t.Errorf("%s: PromptFiles = %v, want %v", id, m.PromptFiles, want.promptFiles)
		}
		if !eqStrings(m.Skills, want.skills) {
			t.Errorf("%s: Skills = %v, want %v", id, m.Skills, want.skills)
		}
		if len(m.QuickPrompts) != want.quickPrompts {
			t.Errorf("%s: QuickPrompts count = %d, want %d", id, len(m.QuickPrompts), want.quickPrompts)
		}
		for i, q := range m.QuickPrompts {
			for _, loc := range []string{"zh", "en"} {
				if strings.TrimSpace(q.Title[loc]) == "" {
					t.Errorf("%s: QuickPrompts[%d].Title[%s] empty", id, i, loc)
				}
				if strings.TrimSpace(q.Prompt[loc]) == "" {
					t.Errorf("%s: QuickPrompts[%d].Prompt[%s] empty", id, i, loc)
				}
			}
		}
		if m.AgentConfig != pilotAgentConfig[id] {
			t.Errorf("%s: AgentConfig = %+v, want %+v", id, m.AgentConfig, pilotAgentConfig[id])
		}
	}
}

func TestPilotExpertsPersonaFiles(t *testing.T) {
	byID := scanPilots(t)
	for id, want := range pilotShape {
		e := byID[id]
		if e == nil {
			t.Fatalf("missing pilot expert %q", id)
		}
		if len(e.PersonaFiles) != len(want.promptFiles) {
			t.Errorf("%s: %d persona files loaded, want %d", id, len(e.PersonaFiles), len(want.promptFiles))
		}
		for _, name := range want.promptFiles {
			content, ok := e.PersonaFiles[name]
			if !ok {
				t.Errorf("%s: PersonaFiles missing %s", id, name)
				continue
			}
			if len(content) == 0 {
				t.Errorf("%s: persona file %s is empty", id, name)
			}
		}
	}
	// Light verbatim spot-check: ops-engineer's SOUL opens with its identity
	// line, exactly as in the Octop source.
	ops := byID["ops-engineer"]
	if ops == nil {
		t.Fatal("missing pilot expert ops-engineer")
	}
	if soul := ops.PersonaFiles["SOUL.md"]; !strings.Contains(string(soul), "You are **Ops**") {
		t.Errorf("ops-engineer SOUL.md spot-check failed: %q", string(soul[:min(80, len(soul))]))
	}
}

func TestPilotSkillsOnDisk(t *testing.T) {
	scanPilots(t) // also fails if the pilot library is missing/malformed
	root := pilotRoot(t)
	// octop-assistant is Octop-product specific and excluded by the importer:
	// no skills/ subdirectory may be shipped for general-assistant.
	if _, err := os.Stat(filepath.Join(root, "general-assistant", "skills")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("general-assistant/skills should not exist, stat err = %v", err)
	}
	// The bundled skills that ARE shipped carry their SKILL.md.
	for _, tc := range []struct{ id, slug string }{
		{"stock-assistant", "stock-info"},
		{"news-trend", "daily-hot-news"},
	} {
		doc := filepath.Join(root, tc.id, "skills", tc.slug, "SKILL.md")
		if fi, err := os.Stat(doc); err != nil || fi.IsDir() {
			t.Errorf("%s: %s: err=%v dir=%v", tc.id, doc, err, fi != nil && fi.IsDir())
		}
	}
}
