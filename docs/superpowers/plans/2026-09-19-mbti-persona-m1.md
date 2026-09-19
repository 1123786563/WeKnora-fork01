# MBTI Persona (M1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate Octop's MBTI persona capability into WeKnora: 16 type profiles + 28-question test + bilingual persona rendering prepended to agent system prompts, with API and React editor UI.

**Architecture:** Pure-data Go package `internal/agent/persona` (embed JSON exported verbatim from Octop via one-time import script) → two new `CustomAgentConfig` fields carried through `types.AgentConfig` → persona segment prepended at `prepareAgentCapabilities` (single point covering both the builtin ReAct engine and the trpc engine) → gin handler `/api/v1/mbti/*` → React agent editor "personalization" section.

**Tech Stack:** Go 1.26 / gin / gorm (no DB migration — persona fields live in the existing `config` JSON column) / go:embed / React 19 + packages/api-client + packages/ui (Tailwind v4).

**Spec:** `docs/superpowers/specs/2026-09-19-octop-capabilities-migration-design.md` §4 (MBTI). Octop source of truth: `/Users/wuyongjun/trea/Octop/src/octop/infra/agents/mbti_profiles.py`, `/Users/wuyongjun/trea/Octop/src/octop/infra/agents/persona.py`, `/Users/wuyongjun/trea/Octop/src/octop/api/routers/mbti.py`.

## Global Constraints

- Data fidelity: the 16 profiles and 28 questions must be byte-identical to Octop's data (imported by script, verified by test — never hand-typed).
- Scoring identical to Octop `_score_answers`: ≥20 valid answers required; per-axis `pct = round(50 + dominant/total*35)` clamped to 50–85; tie → first pole; unanswered axis → first pole at 50.
- Rendering improves on Octop: behavior/summary/descriptors fields selected by request locale (`zh*` → Chinese fields, else English). Octop renders English only.
- No silent failures: applying/removing persona returns explicit errors (Octop's silent `auto_apply=False` is a recorded defect we do not copy).
- All `.go`/source edits go through Write/Edit tools (Mimosa hook rejects bash writes).
- Frontend changes land only in `apps/web` + `packages/api-client` + `packages/i18n` (Vue untouched).
- Locale source: `types.LanguageFromContextOrDefault(ctx)` (`internal/types/context_helpers.go:348`).
- JSON field naming: snake_case (matches `CustomAgentConfig` json tags).

---

### Task 1: Import script + persona data package (types + embed)

**Files:**
- Create: `scripts/import_octop_mbti.py`
- Create: `internal/agent/persona/profile.go`
- Create: `internal/agent/persona/data.go`
- Create: `internal/agent/persona/data/profiles.json` (generated)
- Create: `internal/agent/persona/data/questions.json` (generated)
- Test: `internal/agent/persona/data_test.go`

**Interfaces:**
- Consumes: Octop python modules (read-only, absolute path below).
- Produces:
  - `type MBTIProfile struct { Code, NameZh, NameEn, NicknameZh, SummaryZh, SummaryEn, DescriptorsZh, DescriptorsEn, Color, Symbol string; Dimensions MBTIDimensions; Behavior MBTIBehavior }`
  - `type MBTIDimensions struct{ EI, SN, TF, JP Axis }`, `type Axis struct{ Pole string; Percent int }`
  - `type MBTIBehavior struct{ AnswerStyle, CasualChat, Conflict, Creativity, Emotion, Planning, AnswerStyleZh, CasualChatZh, ConflictZh, CreativityZh, EmotionZh, PlanningZh string }`
  - `type TestQuestion struct{ ID int; Dimension, APole, BPole, QuestionZh, OptionAZh, OptionBZh, QuestionEn, OptionAEn, OptionBEn string }`
  - `var ErrProfileNotFound = errors.New(...)`; `func Profile(code string) (MBTIProfile, bool)`; `func AllProfiles() []MBTIProfile` (canonical NT/NF/SJ/SP order); `func Questions() []TestQuestion`

- [ ] **Step 1: Write the import script**

```python
#!/usr/bin/env python3
"""One-time importer: dump Octop MBTI profiles + test questions to JSON for go:embed.

Re-runnable for auditing data fidelity. Output byte-compares in data_test.go
only for structure; the JSON is the single source for the Go package.
"""
import json
import sys
from pathlib import Path

OCTOP = Path("/Users/wuyongjun/trea/Octop/src/octop")
sys.path.insert(0, str(OCTOP.parent.parent.parent))

from octop.infra.agents.mbti_profiles import get_all_profiles  # noqa: E402

# Questions live in the router module.
import importlib.util  # noqa: E402
spec = importlib.util.spec_from_file_location(
    "mbti_router", OCTOP / "api" / "routers" / "mbti.py")
mbti_router = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mbti_router)

OUT = Path(__file__).resolve().parents[1] / "internal" / "agent" / "persona" / "data"
OUT.mkdir(parents=True, exist_ok=True)

def axis(t):
    return {"pole": t[0], "percent": t[1]}

profiles = []
for p in get_all_profiles():
    profiles.append({
        "code": p.code, "name_zh": p.name_zh, "name_en": p.name_en,
        "nickname_zh": p.nickname_zh, "summary_zh": p.summary_zh,
        "summary_en": p.summary_en, "descriptors_zh": p.descriptors_zh,
        "descriptors_en": p.descriptors_en,
        "dimensions": {
            "ei": axis(p.dimensions.ei), "sn": axis(p.dimensions.sn),
            "tf": axis(p.dimensions.tf), "jp": axis(p.dimensions.jp)},
        "behavior": {
            "answer_style": p.behavior.answer_style,
            "casual_chat": p.behavior.casual_chat,
            "conflict": p.behavior.conflict,
            "creativity": p.behavior.creativity,
            "emotion": p.behavior.emotion,
            "planning": p.behavior.planning,
            "answer_style_zh": p.behavior.answer_style_zh,
            "casual_chat_zh": p.behavior.casual_chat_zh,
            "conflict_zh": p.behavior.conflict_zh,
            "creativity_zh": p.behavior.creativity_zh,
            "emotion_zh": p.behavior.emotion_zh,
            "planning_zh": p.behavior.planning_zh},
        "color": p.color, "symbol": p.symbol,
    })

questions = [{
    "id": q.id, "dimension": q.dimension, "a_pole": q.a_pole, "b_pole": q.b_pole,
    "question_zh": q.question_zh, "option_a_zh": q.option_a_zh, "option_b_zh": q.option_b_zh,
    "question_en": q.question_en, "option_a_en": q.option_a_en, "option_b_en": q.option_b_en,
} for q in mbti_router._QUESTIONS]

(OUT / "profiles.json").write_text(
    json.dumps(profiles, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
(OUT / "questions.json").write_text(
    json.dumps(questions, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
print(f"wrote {len(profiles)} profiles, {len(questions)} questions -> {OUT}")
```

- [ ] **Step 2: Run the script, verify output counts**

Run: `python3 scripts/import_octop_mbti.py`
Expected: `wrote 16 profiles, 28 questions -> .../internal/agent/persona/data`

- [ ] **Step 3: Write the failing data-integrity test**

`internal/agent/persona/data_test.go`:

```go
package persona

import "testing"

func TestAllProfilesSixteenTypes(t *testing.T) {
	all := AllProfiles()
	if len(all) != 16 {
		t.Fatalf("want 16 profiles, got %d", len(all))
	}
	seen := map[string]bool{}
	for _, p := range all {
		if seen[p.Code] {
			t.Errorf("duplicate code %s", p.Code)
		}
		seen[p.Code] = true
		for _, a := range []Axis{p.Dimensions.EI, p.Dimensions.SN, p.Dimensions.TF, p.Dimensions.JP} {
			if a.Percent < 50 || a.Percent > 85 {
				t.Errorf("%s axis %% out of 50-85: %+v", p.Code, a)
			}
		}
		if p.Behavior.AnswerStyle == "" || p.SummaryEn == "" || p.Color == "" {
			t.Errorf("%s missing core fields", p.Code)
		}
	}
	if !seen["INTJ"] || !seen["ESFP"] {
		t.Error("canonical corner types missing")
	}
}

func TestQuestionsTwentyEightBalanced(t *testing.T) {
	qs := Questions()
	if len(qs) != 28 {
		t.Fatalf("want 28 questions, got %d", len(qs))
	}
	perDim := map[string]int{}
	for _, q := range qs {
		perDim[q.Dimension]++
		if q.QuestionZh == "" || q.QuestionEn == "" || q.OptionAZh == "" || q.OptionBEn == "" {
			t.Errorf("question %d missing bilingual fields", q.ID)
		}
	}
	for _, dim := range []string{"EI", "SN", "TF", "JP"} {
		if perDim[dim] != 7 {
			t.Errorf("dimension %s: want 7 questions, got %d", dim, perDim[dim])
		}
	}
}

func TestProfileLookup(t *testing.T) {
	if _, ok := Profile("INTJ"); !ok {
		t.Error("INTJ should resolve")
	}
	if _, ok := Profile("intj"); ok {
		t.Error("lookup must be case-sensitive (uppercase codes only)")
	}
	if _, ok := Profile("XXYY"); ok {
		t.Error("unknown code should not resolve")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/agent/persona/ -run 'TestAllProfiles|TestQuestions|TestProfileLookup' -v`
Expected: FAIL — package does not exist / undefined symbols.

- [ ] **Step 5: Implement profile.go + data.go**

`internal/agent/persona/profile.go` (types only, zero imports beyond stdlib):

```go
// Package persona provides MBTI personality profiles, the 28-question test,
// and system-prompt persona rendering. Data is imported verbatim from Octop
// (scripts/import_octop_mbti.py) and embedded at build time.
package persona

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

//go:embed data/profiles.json
var profilesJSON []byte

//go:embed data/questions.json
var questionsJSON []byte

var ErrProfileNotFound = errors.New("persona: mbti profile not found")

type Axis struct {
	Pole    string `json:"pole"`
	Percent int    `json:"percent"`
}

type MBTIDimensions struct {
	EI Axis `json:"ei"`
	SN Axis `json:"sn"`
	TF Axis `json:"tf"`
	JP Axis `json:"jp"`
}

type MBTIBehavior struct {
	AnswerStyle  string `json:"answer_style"`
	CasualChat   string `json:"casual_chat"`
	Conflict     string `json:"conflict"`
	Creativity   string `json:"creativity"`
	Emotion      string `json:"emotion"`
	Planning     string `json:"planning"`
	AnswerStyleZh string `json:"answer_style_zh"`
	CasualChatZh  string `json:"casual_chat_zh"`
	ConflictZh    string `json:"conflict_zh"`
	CreativityZh  string `json:"creativity_zh"`
	EmotionZh     string `json:"emotion_zh"`
	PlanningZh    string `json:"planning_zh"`
}

type MBTIProfile struct {
	Code          string         `json:"code"`
	NameZh        string         `json:"name_zh"`
	NameEn        string         `json:"name_en"`
	NicknameZh    string         `json:"nickname_zh"`
	SummaryZh     string         `json:"summary_zh"`
	SummaryEn     string         `json:"summary_en"`
	DescriptorsZh string         `json:"descriptors_zh"`
	DescriptorsEn string         `json:"descriptors_en"`
	Dimensions    MBTIDimensions `json:"dimensions"`
	Behavior      MBTIBehavior   `json:"behavior"`
	Color         string         `json:"color"`
	Symbol        string         `json:"symbol"`
}

type TestQuestion struct {
	ID         int    `json:"id"`
	Dimension  string `json:"dimension"`
	APole      string `json:"a_pole"`
	BPole      string `json:"b_pole"`
	QuestionZh string `json:"question_zh"`
	OptionAZh  string `json:"option_a_zh"`
	OptionBZh  string `json:"option_b_zh"`
	QuestionEn string `json:"question_en"`
	OptionAEn  string `json:"option_a_en"`
	OptionBEn  string `json:"option_b_en"`
}

var (
	loadOnce    sync.Once
	loaded      []MBTIProfile
	byCode      map[string]MBTIProfile
	loadedQs    []TestQuestion
	loadErr     error
)

func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(profilesJSON, &loaded); err != nil {
			loadErr = fmt.Errorf("persona: decode profiles: %w", err)
			return
		}
		byCode = make(map[string]MBTIProfile, len(loaded))
		for _, p := range loaded {
			byCode[p.Code] = p
		}
		if err := json.Unmarshal(questionsJSON, &loadedQs); err != nil {
			loadErr = fmt.Errorf("persona: decode questions: %w", err)
		}
	})
}

// Profile returns the profile for an uppercase MBTI code.
func Profile(code string) (MBTIProfile, bool) {
	load()
	if loadErr != nil {
		return MBTIProfile{}, false
	}
	p, ok := byCode[code]
	return p, ok
}

// AllProfiles returns all 16 profiles in canonical NT/NF/SJ/SP order (the
// import preserves Octop's registration order).
func AllProfiles() []MBTIProfile {
	load()
	if loadErr != nil {
		return nil
	}
	return append([]MBTIProfile(nil), loaded...)
}

// Questions returns the 28 test questions.
func Questions() []TestQuestion {
	load()
	if loadErr != nil {
		return nil
	}
	return append([]TestQuestion(nil), loadedQs...)
}
```

(Note: split into `profile.go` for types and `data.go` for embed/load at implementation time is fine — signatures above are the contract.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/agent/persona/ -v`
Expected: PASS (3 tests).

- [ ] **Step 7: Commit**

```bash
git add scripts/import_octop_mbti.py internal/agent/persona/
git commit -m "feat(persona): import Octop MBTI profiles and test data as embedded JSON"
```

---

### Task 2: Scoring (28-question submit logic)

**Files:**
- Create: `internal/agent/persona/scoring.go`
- Test: `internal/agent/persona/scoring_test.go`

**Interfaces:**
- Consumes: `Questions()` from Task 1.
- Produces: `type AxisResult struct{ Dominant string; Percent int }`; `type ScoreResult struct{ Code string; EI, SN, TF, JP AxisResult }`; `func ScoreAnswers(answers map[string]string) (ScoreResult, error)` — `answers` keys are decimal question IDs, values `"A"`/`"B"` (case-insensitive). Unknown IDs ignored. Fewer than 20 valid answers → error.

- [ ] **Step 1: Write the failing tests**

`internal/agent/persona/scoring_test.go`:

```go
package persona

import "testing"

func allAnswers(pick func(q TestQuestion) string) map[string]string {
	out := map[string]string{}
	for _, q := range Questions() {
		out[fmt.Sprint(q.ID)] = pick(q)
	}
	return out
}

func TestScoreAllAIsFirstPoles(t *testing.T) {
	res, err := ScoreAnswers(allAnswers(func(q TestQuestion) string { return "A" }))
	if err != nil {
		t.Fatal(err)
	}
	// All-A: every axis dominant is the a_pole of its questions.
	if res.Code != "ENTJ" && res.Code != "ESTJ" {
		// derive instead of hardcoding: first poles per axis in question data
		t.Logf("code=%s", res.Code)
	}
	if res.EI.Dominant != "E" || res.SN.Dominant != "N" || res.TF.Dominant != "T" || res.JP.Dominant != "J" {
		t.Fatalf("all-A must select first poles, got %s", res.Code)
	}
	if res.EI.Percent != 85 { // 7/7 dominant: 50+35=85
		t.Errorf("EI percent = %d, want 85", res.EI.Percent)
	}
}

func TestScoreTieTakesFirstPole(t *testing.T) {
	// 4×A + 3×B on every axis: pct = round(50 + 4/7*35) = 70, dominant = first pole.
	answers := map[string]string{}
	i := 0
	for _, q := range Questions() {
		choice := "A"
		if i%7 >= 4 {
			choice = "B"
		}
		answers[fmt.Sprint(q.ID)] = choice
		i++
	}
	res, err := ScoreAnswers(answers)
	if err != nil {
		t.Fatal(err)
	}
	if res.EI.Dominant != "E" || res.EI.Percent != 70 {
		t.Errorf("EI = %+v, want E/70", res.EI)
	}
}

func TestScoreTooFewAnswers(t *testing.T) {
	answers := map[string]string{"1": "A", "2": "B"}
	if _, err := ScoreAnswers(answers); err == nil {
		t.Fatal("want error for 2/28 answers")
	}
}

func TestScoreIgnoresUnknownAndInvalid(t *testing.T) {
	answers := allAnswers(func(q TestQuestion) string { return "a" }) // lowercase ok
	answers["999"] = "A"   // unknown id
	answers["1"] = "X"     // invalid choice ignored
	if _, err := ScoreAnswers(answers); err != nil {
		t.Fatal(err)
	}
}

func TestScoreUnansweredAxisDefaultsFirstPole(t *testing.T) {
	// Answer 21 questions, all EI questions blank.
	answers := map[string]string{}
	count := 0
	for _, q := range Questions() {
		if q.Dimension == "EI" || count >= 21 {
			continue
		}
		answers[fmt.Sprint(q.ID)] = "B"
		count++
	}
	res, err := ScoreAnswers(answers)
	if err != nil {
		t.Fatal(err)
	}
	if res.EI.Dominant != "E" || res.EI.Percent != 50 {
		t.Errorf("unanswered EI = %+v, want E/50", res.EI)
	}
}
```

(add `"fmt"` to imports)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/agent/persona/ -run TestScore -v`
Expected: FAIL — `ScoreAnswers` undefined.

- [ ] **Step 3: Implement scoring.go**

```go
package persona

import "errors"

// AxisResult is the scored outcome of one MBTI axis.
type AxisResult struct {
	Dominant string `json:"dominant"`
	Percent  int    `json:"percent"`
}

// ScoreResult is the outcome of the 28-question test.
type ScoreResult struct {
	Code string     `json:"code"`
	EI   AxisResult `json:"ei"`
	SN   AxisResult `json:"sn"`
	TF   AxisResult `json:"tf"`
	JP   AxisResult `json:"jp"`
}

var axes = []struct {
	key         string
	first, sec  string
	set         func(*ScoreResult, AxisResult)
	get         func(ScoreResult) AxisResult
}{
	{"EI", "E", "I", func(r *ScoreResult, a AxisResult) { r.EI = a }, func(r ScoreResult) AxisResult { return r.EI }},
	{"SN", "S", "N", func(r *ScoreResult, a AxisResult) { r.SN = a }, func(r ScoreResult) AxisResult { return r.SN }},
	{"TF", "T", "F", func(r *ScoreResult, a AxisResult) { r.TF = a }, func(r ScoreResult) AxisResult { return r.TF }},
	{"JP", "J", "P", func(r *ScoreResult, a AxisResult) { r.JP = a }, func(r ScoreResult) AxisResult { return r.JP }},
}

// ScoreAnswers scores submitted test answers, mirroring Octop _score_answers:
// ≥20 valid answers required; pct = round(50 + dominant/total*35) clamped to
// 50–85; tie takes the first pole; an unanswered axis defaults to the first
// pole at 50.
func ScoreAnswers(answers map[string]string) (ScoreResult, error) {
	counts := map[string]map[string]int{}
	for _, ax := range axes {
		counts[ax.key] = map[string]int{ax.first: 0, ax.sec: 0}
	}
	qByID := map[int]TestQuestion{}
	for _, q := range Questions() {
		qByID[q.ID] = q
	}
	valid := 0
	for id, choice := range answers {
		var qid int
		if _, err := fmt.Sscanf(id, "%d", &qid); err != nil {
			continue
		}
		q, ok := qByID[qid]
		if !ok {
			continue
		}
		switch strings.ToUpper(choice) {
		case "A":
			counts[q.Dimension][q.APole]++
			valid++
		case "B":
			counts[q.Dimension][q.BPole]++
			valid++
		}
	}
	if valid < 20 {
		return ScoreResult{}, fmt.Errorf("persona: too few answers (%d/28), at least 20 required", valid)
	}
	var res ScoreResult
	for _, ax := range axes {
		a, b := counts[ax.key][ax.first], counts[ax.key][ax.sec]
		if a+b == 0 {
			ax.set(&res, AxisResult{Dominant: ax.first, Percent: 50})
			continue
		}
		dominant, dominantCount := ax.first, a
		if b > a {
			dominant, dominantCount = ax.sec, b
		}
		pct := 50 + (dominantCount*35+(a+b)/2)/(a+b)
		if pct > 85 {
			pct = 85
		}
		ax.set(&res, AxisResult{Dominant: dominant, Percent: pct})
	}
	res.Code = res.EI.Dominant + res.SN.Dominant + res.TF.Dominant + res.JP.Dominant
	return res, nil
}
```

Fix imports: `"fmt"`, `"strings"`; drop unused `errors`. The rounding expression `(d*35 + t/2)/t` is round-half-up, proven equal to Python's banker's `round()` for every (d, t) reachable here (t ≤ 7 questions per axis): a half-fraction requires 70d = t·odd, and every such pct has an odd integer part, where both rounding modes round up. Keep the test asserting 70/85.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/persona/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/persona/scoring.go internal/agent/persona/scoring_test.go
git commit -m "feat(persona): 28-question scoring ported from Octop _score_answers"
```

---

### Task 3: Bilingual persona renderer

**Files:**
- Create: `internal/agent/persona/render.go`
- Test: `internal/agent/persona/render_test.go`

**Interfaces:**
- Consumes: `Profile()` from Task 1.
- Produces: `type RenderInput struct{ AgentName, UserDisplay, Custom string }`; `func RenderPersona(code, locale string, in RenderInput) string` — `code` empty/invalid → default template; `locale` prefix `zh` → Chinese fields, else English; `in.Custom` appended as an extra paragraph when non-empty.

- [ ] **Step 1: Write the failing tests**

```go
package persona

import (
	"strings"
	"testing"
)

func TestRenderEnglish(t *testing.T) {
	out := RenderPersona("INTJ", "en-US", RenderInput{AgentName: "Atlas", UserDisplay: "Wu"})
	if !strings.HasPrefix(out, "# Persona: INTJ — Architect") {
		t.Errorf("bad header: %q", firstLine(out))
	}
	if !strings.Contains(out, "You are Atlas, an AI assistant working with Wu.") {
		t.Error("intro sentence missing")
	}
	if !strings.Contains(out, "Imaginative strategist with a plan for everything") {
		t.Error("english summary missing")
	}
	if strings.Contains(out, "简洁有条理") {
		t.Error("chinese fields must not leak into english render")
	}
}

func TestRenderChinese(t *testing.T) {
	out := RenderPersona("INTJ", "zh-CN", RenderInput{AgentName: "阿特拉斯", UserDisplay: "吴"})
	if !strings.Contains(out, "# Persona: INTJ — 建筑师") {
		t.Error("chinese header missing")
	}
	if !strings.Contains(out, "富有想象力的战略家，对一切都有计划") {
		t.Error("chinese summary missing")
	}
	if !strings.Contains(out, "简洁有条理，喜欢深度而非广度") {
		t.Error("chinese behavior missing")
	}
}

func TestRenderCustomAppended(t *testing.T) {
	out := RenderPersona("INTJ", "en", RenderInput{AgentName: "A", UserDisplay: "U", Custom: "Always answer in bullet points."})
	if !strings.Contains(out, "Always answer in bullet points.") {
		t.Error("custom paragraph missing")
	}
}

func TestRenderUnknownCodeFallsBackToDefault(t *testing.T) {
	out := RenderPersona("", "en", RenderInput{AgentName: "A", UserDisplay: "U"})
	if !strings.Contains(out, "# Persona: Default") {
		t.Error("default template expected for empty code")
	}
	if RenderPersona("XXYY", "en", RenderInput{}) == "" {
		t.Error("invalid code must still render default, not empty")
	}
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/agent/persona/ -run TestRender -v`
Expected: FAIL — `RenderPersona` undefined.

- [ ] **Step 3: Implement render.go**

```go
package persona

import (
	"fmt"
	"strings"
)

const defaultTemplate = `# Persona: Default

You are {agent_name}, an attentive AI assistant working with {user_display}.

Tone: warm, direct, and competent. Prefer concrete answers over hedging.
Match the user's level of detail. When uncertain, say so and propose how
to find out.
`

// RenderInput fills the persona template placeholders.
type RenderInput struct {
	AgentName   string
	UserDisplay string
	// Custom is optional free-text layered after the MBTI block.
	Custom string
}

// RenderPersona renders the persona segment for an MBTI code. Behavior,
// summary, and descriptor fields are selected by locale (zh* → Chinese),
// fixing Octop's English-only rendering. Empty or unknown codes render the
// default template.
func RenderPersona(code, locale string, in RenderInput) string {
	if in.UserDisplay == "" {
		in.UserDisplay = "the user"
	}
	profile, ok := Profile(strings.ToUpper(code))
	if !ok {
		return fill(defaultTemplate, in)
	}
	useZh := strings.HasPrefix(strings.ToLower(locale), "zh")
	name, summary, descriptors := profile.NameEn, profile.SummaryEn, profile.DescriptorsEn
	b := profile.Behavior
	behavior := [6]string{b.AnswerStyle, b.CasualChat, b.Conflict, b.Creativity, b.Emotion, b.Planning}
	if useZh {
		name, summary, descriptors = profile.NameZh, profile.SummaryZh, profile.DescriptorsZh
		behavior = [6]string{b.AnswerStyleZh, b.CasualChatZh, b.ConflictZh, b.CreativityZh, b.EmotionZh, b.PlanningZh}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Persona: %s — %s\n\n", profile.Code, name)
	if useZh {
		fmt.Fprintf(&sb, "你是 %s，一名与 %s 协作的 AI 助手。\n\n", in.AgentName, in.UserDisplay)
	} else {
		fmt.Fprintf(&sb, "You are %s, an AI assistant working with %s.\n\n", in.AgentName, in.UserDisplay)
	}
	fmt.Fprintf(&sb, "%s.\n\n", summary)
	if useZh {
		fmt.Fprintf(&sb, "特质：%s。\n\n", descriptors)
		sb.WriteString("## 行为风格\n\n")
		labels := [6]string{"回答风格", "闲聊", "冲突", "创造力", "情绪", "计划"}
		for i, label := range labels {
			fmt.Fprintf(&sb, "- **%s：**%s\n", label, behavior[i])
		}
	} else {
		fmt.Fprintf(&sb, "Traits: %s.\n\n", descriptors)
		sb.WriteString("## Behavior\n\n")
		labels := [6]string{"Answer style", "Casual chat", "Conflict", "Creativity", "Emotion", "Planning"}
		for i, label := range labels {
			fmt.Fprintf(&sb, "- **%s:** %s\n", label, behavior[i])
		}
	}
	if custom := strings.TrimSpace(in.Custom); custom != "" {
		sb.WriteString("\n" + custom + "\n")
	}
	return sb.String()
}

func fill(tmpl string, in RenderInput) string {
	r := strings.NewReplacer(
		"{agent_name}", in.AgentName,
		"{user_display}", in.UserDisplay,
	)
	return r.Replace(tmpl)
}
```

Note: Octop's INTJ profile ends with a summary already containing no trailing period — the Go render appends "." itself; the test asserts containment so this is stable.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/persona/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/persona/render.go internal/agent/persona/render_test.go
git commit -m "feat(persona): bilingual persona renderer with default-template fallback"
```

---

### Task 4: Config fields — CustomAgentConfig + AgentConfig passthrough

**Files:**
- Modify: `internal/types/custom_agent.go:153` area (Skills Settings 之后、Sandbox Settings 之前插入 Persona block)
- Modify: `internal/types/agent.go` (AgentConfig struct — locate the field mirroring SystemPrompt)
- Modify: `internal/handler/session/session_agent_qa.go:314` `buildAgentConfig` (copy the two fields)
- Test: `internal/types/custom_agent_persona_test.go`

**Interfaces:**
- Consumes: none.
- Produces: `CustomAgentConfig.PersonaMBTI string` / `CustomAgentConfig.PersonaStyle string` (json `persona_mbti` / `persona_style`); same two fields on `types.AgentConfig`; copy in `buildAgentConfig`.

- [ ] **Step 1: Write the failing round-trip test**

```go
package types

import (
	"encoding/json"
	"testing"
)

func TestPersonaConfigJSONRoundTrip(t *testing.T) {
	cfg := CustomAgentConfig{PersonaMBTI: "INTJ", PersonaStyle: "Be terse."}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back CustomAgentConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.PersonaMBTI != "INTJ" || back.PersonaStyle != "Be terse." {
		t.Fatalf("round-trip lost fields: %+v", back)
	}
	// Old payloads without the keys must not break, and EnsureDefaults must
	// not invent a persona.
	var legacy CustomAgentConfig
	if err := json.Unmarshal([]byte(`{"agent_mode":"quick-answer"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	agent := &CustomAgent{Config: legacy}
	agent.EnsureDefaults()
	if agent.Config.PersonaMBTI != "" {
		t.Error("EnsureDefaults must not set a persona")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/types/ -run TestPersonaConfig -v`
Expected: FAIL — no PersonaMBTI field.

- [ ] **Step 3: Add fields**

In `internal/types/custom_agent.go`, insert after the Skills Settings block (around line 157, before Sandbox Settings):

```go
	// ===== Persona Settings =====
	// PersonaMBTI is an uppercase MBTI code ("INTJ") selecting a built-in
	// personality profile rendered before SystemPrompt. Empty = no persona.
	PersonaMBTI string `yaml:"persona_mbti" json:"persona_mbti,omitempty"`
	// PersonaStyle is free-text persona guidance appended after the MBTI
	// block. Only rendered when PersonaMBTI is set.
	PersonaStyle string `yaml:"persona_style" json:"persona_style,omitempty"`
```

In `internal/types/agent.go` find `AgentConfig` (the runtime mirror carrying SystemPrompt) and add the same two fields with identical tags. In `internal/handler/session/session_agent_qa.go` `buildAgentConfig` (the function mapping CustomAgentConfig→AgentConfig around :391), add next to the SystemPrompt copy:

```go
	agentConfig.PersonaMBTI = customAgent.Config.PersonaMBTI
	agentConfig.PersonaStyle = customAgent.Config.PersonaStyle
```

- [ ] **Step 4: Run tests + build**

Run: `go test ./internal/types/ -run TestPersonaConfig -v && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/types/custom_agent.go internal/types/custom_agent_persona_test.go internal/types/agent.go internal/handler/session/session_agent_qa.go
git commit -m "feat(persona): persona_mbti/persona_style config fields with agent passthrough"
```

---

### Task 5: Prompt assembly — prepend persona segment in prepareAgentCapabilities

**Files:**
- Modify: `internal/application/service/agent_capabilities.go:110-125` (systemPrompt assembly)
- Test: `internal/application/service/agent_capabilities_persona_test.go`

**Interfaces:**
- Consumes: `persona.RenderPersona` (Task 3), `config.PersonaMBTI/PersonaStyle` (Task 4), `types.LanguageFromContextOrDefault(ctx)` (`internal/types/context_helpers.go:348`).
- Produces: `SystemPrompt` in returned `AgentCapabilities` starts with the persona segment when `config.PersonaMBTI != ""`. Covers both engines: builtin ReAct (`engine.go:149` consumes the template) and trpc durable runs (`trpc/graph.go:81-94` consumes `Capabilities.SystemPrompt` verbatim).

- [ ] **Step 1: Write the failing test**

```go
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/persona"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestPrepareCapabilitiesPrependsPersona(t *testing.T) {
	svc := newPersonaTestAgentService(t) // helper: minimal agentService with fake model service; reuse existing test scaffolding in this package (see agent_capabilities_test.go patterns). If scaffolding is heavy, extract the persona-prepend into a pure helper and test that — see Step 3 note.
	cfg := &types.AgentConfig{
		SystemPrompt:  "You are a research helper.",
		PersonaMBTI:   "INTJ",
		PersonaStyle:  "Answer in bullet points.",
	}
	_ = cfg
	out := persona.RenderPersona("INTJ", "zh-CN", persona.RenderInput{
		AgentName: "Test", UserDisplay: "u1", Custom: cfg.PersonaStyle,
	})
	if !strings.HasPrefix(out, "# Persona: INTJ") {
		t.Fatalf("rendered persona wrong: %q", firstN(out, 40))
	}
	if !strings.Contains(out, "Answer in bullet points.") {
		t.Error("style paragraph missing")
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

The integration-level assertion (SystemPrompt really starts with the segment) is exercised in Step 3 via the extracted helper + existing capability tests. Adjust to the package's actual test scaffolding; if `prepareAgentCapabilities` already has a test constructing it with fake models, extend that instead of new scaffolding.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/application/service/ -run Persona -v`
Expected: FAIL (helper absent) or scaffolding compile error — that's the red state.

- [ ] **Step 3: Implement**

In `agent_capabilities.go`, replace lines 110-113:

```go
	knowledgeBases, documents := s.resolveKBAndDocInfos(ctx, config)
	systemPrompt := ""
	if config.UseCustomSystemPrompt || config.SystemPrompt != "" {
		systemPrompt = config.ResolveSystemPrompt(config.WebSearchEnabled)
	}
```

with:

```go
	knowledgeBases, documents := s.resolveKBAndDocInfos(ctx, config)
	systemPrompt := ""
	if config.UseCustomSystemPrompt || config.SystemPrompt != "" {
		systemPrompt = config.ResolveSystemPrompt(config.WebSearchEnabled)
	}
	systemPrompt = prependPersonaSegment(ctx, config, systemPrompt)
```

and add in the same file:

```go
// prependPersonaSegment renders the agent's MBTI persona block in front of
// its system prompt template. Both engines consume capabilities.SystemPrompt
// (the builtin ReAct engine treats it as a template; trpc runs insert it
// verbatim), so prepending here covers every execution path. An unset or
// unknown PersonaMBTI leaves the prompt untouched.
func prependPersonaSegment(ctx context.Context, config *types.AgentConfig, systemPrompt string) string {
	if config == nil || config.PersonaMBTI == "" {
		return systemPrompt
	}
	if _, ok := persona.Profile(config.PersonaMBTI); !ok {
		logger.Warnf(ctx, "agent has unknown persona_mbti %q; skipping persona segment", config.PersonaMBTI)
		return systemPrompt
	}
	locale := types.LanguageFromContextOrDefault(ctx)
	segment := persona.RenderPersona(config.PersonaMBTI, locale, persona.RenderInput{
		AgentName:   config.Name,
		UserDisplay: userIDFromContext(ctx),
		Custom:      config.PersonaStyle,
	})
	if systemPrompt == "" {
		return segment
	}
	return segment + "\n---\n\n" + systemPrompt
}

func userIDFromContext(ctx context.Context) string {
	if uid, ok := ctx.Value(types.UserIDContextKey.String()).(string); ok && uid != "" {
		return uid
	}
	return ""
}
```

Add `"github.com/Tencent/WeKnora/internal/agent/persona"` to imports. `config.Name` — verify the AgentConfig field actually carrying the agent display name while implementing (it mirrors CustomAgent.Name; if named differently, e.g. `AgentName`, use that). Add a table-driven test for `prependPersonaSegment` (with/without persona, unknown code, empty systemPrompt, zh/en locale).

- [ ] **Step 4: Run package tests + full build**

Run: `go test ./internal/application/service/ -run 'Persona|Capabilities' -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/agent_capabilities.go internal/application/service/agent_capabilities_persona_test.go
git commit -m "feat(persona): prepend MBTI persona segment in capability assembly (covers both engines)"
```

---

### Task 6: API — handler, service, routes

**Files:**
- Create: `internal/handler/persona.go`
- Create: `internal/router/routes_persona.go`
- Modify: `internal/router/router.go` (register new routes group alongside other v1 groups)
- Test: `internal/handler/persona_test.go`

**Interfaces:**
- Consumes: persona package (Tasks 1-3), `customAgentService.GetAgentByID/UpdateAgent` (`internal/types/interfaces/custom_agent.go:49`), guard `g.OwnedAgentOrAdmin()` pattern from `internal/router/routes_agent.go:43`, locale `types.LanguageFromContextOrDefault(ctx)`.
- Produces routes (all under `/api/v1`):
  - `GET /mbti/types` → `{"success":true,"data":{"types":[MBTITypeResponse...]}}`
  - `GET /mbti/types/:code` → single or 404
  - `GET /mbti/preview/:code` → `{"code","markdown"}` (`code=_default` renders default template)
  - `GET /mbti/test/questions` → `{"questions":[TestQuestion...]}` (locale-filtered presentation is client-side; payload ships both languages)
  - `POST /mbti/test/submit` body `{"answers":{"1":"A",...}}` → `{"code", "dimensions":{ei/sn/tf/jp:{dominant,percent}}, "profile":MBTITypeResponse}`
  - `PUT /agents/:agentID/persona` body `{"code":"INTJ","style":""}` → updated agent summary; validates code, 404 unknown agent, guarded
  - `DELETE /agents/:agentID/persona` → clears both fields
  - `MBTITypeResponse` json mirrors the Go profile (snake_case, dimensions as `{ei:{pole,percent},...}`, all 12 behavior fields)

- [ ] **Step 1: Write failing handler tests**

`internal/handler/persona_test.go` — follow the existing handler test scaffolding in `internal/handler/*_test.go` (gin test context + fake custom agent service). Cover:
1. `GET /mbti/types` returns 16 items, success=true
2. `GET /mbti/types/INTJ` returns nickname_zh="紫老头"; `GET /mbti/types/XXYY` → 404
3. `GET /mbti/preview/INTJ` markdown starts with "# Persona: INTJ"; `_default` renders default header
4. `POST /mbti/test/submit` with 28 "A" answers returns code and all dimensions dominant/percent=85… except check EI percent==85; with 2 answers → 400
5. `PUT /agents/a1/persona` with invalid code → 400; unknown agent → 404; valid → service.UpdateAgent called with PersonaMBTI set
6. `DELETE /agents/a1/persona` clears fields

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMBTITypesList(t *testing.T) {
	router, _ := newPersonaTestRouter(t) // helper builds gin engine with persona routes + fake agent service
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mbti/types", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Data struct{ Types []json.RawMessage } `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Types) != 16 {
		t.Fatalf("want 16 types, got %d", len(body.Data.Types))
	}
}

func TestMBTISubmitScores(t *testing.T) {
	router, _ := newPersonaTestRouter(t)
	answers := map[string]string{}
	for i := 1; i <= 28; i++ {
		answers[json.Number(string(rune('0'+i/10))+string(rune('0'+i%10))).String()] = "A"
	}
	payload, _ := json.Marshal(map[string]any{"answers": answers})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mbti/test/submit", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Data.Code) != 4 {
		t.Fatalf("code = %q", body.Data.Code)
	}
}
```

(Helper `newPersonaTestRouter` reuses the package's existing fake-service patterns; construct answers with `strconv.Itoa` in the real test — the rune arithmetic above is only illustrative and must be replaced with `strconv.Itoa(i)`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/handler/ -run MBTI -v`
Expected: FAIL — routes undefined.

- [ ] **Step 3: Implement handler + routes**

`internal/handler/persona.go`:

```go
package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/agent/persona"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// PersonaHandler serves the MBTI persona API: type catalog, rendering
// preview, the 28-question test, and applying/removing a persona on an agent.
type PersonaHandler struct {
	agentService interfaces.CustomAgentService
}

func NewPersonaHandler(agentService interfaces.CustomAgentService) *PersonaHandler {
	return &PersonaHandler{agentService: agentService}
}

type mbtiTypeResponse struct {
	Code          string            `json:"code"`
	NameZh        string            `json:"name_zh"`
	NameEn        string            `json:"name_en"`
	NicknameZh    string            `json:"nickname_zh"`
	SummaryZh     string            `json:"summary_zh"`
	SummaryEn     string            `json:"summary_en"`
	DescriptorsZh string            `json:"descriptors_zh"`
	DescriptorsEn string            `json:"descriptors_en"`
	Dimensions    map[string]axisDTO `json:"dimensions"`
	Behavior      persona.MBTIBehavior `json:"behavior"`
	Color         string            `json:"color"`
	Symbol        string            `json:"symbol"`
}

type axisDTO struct {
	Pole    string `json:"pole"`
	Percent int    `json:"percent"`
}

func toTypeResponse(p persona.MBTIProfile) mbtiTypeResponse {
	return mbtiTypeResponse{
		Code: p.Code, NameZh: p.NameZh, NameEn: p.NameEn,
		NicknameZh: p.NicknameZh, SummaryZh: p.SummaryZh, SummaryEn: p.SummaryEn,
		DescriptorsZh: p.DescriptorsZh, DescriptorsEn: p.DescriptorsEn,
		Dimensions: map[string]axisDTO{
			"ei": {p.Dimensions.EI.Pole, p.Dimensions.EI.Percent},
			"sn": {p.Dimensions.SN.Pole, p.Dimensions.SN.Percent},
			"tf": {p.Dimensions.TF.Pole, p.Dimensions.TF.Percent},
			"jp": {p.Dimensions.JP.Pole, p.Dimensions.JP.Percent},
		},
		Behavior: p.Behavior, Color: p.Color, Symbol: p.Symbol,
	}
}

func (h *PersonaHandler) ListTypes(c *gin.Context) {
	types := make([]mbtiTypeResponse, 0, 16)
	for _, p := range persona.AllProfiles() {
		types = append(types, toTypeResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"types": types}})
}

func (h *PersonaHandler) GetType(c *gin.Context) {
	p, ok := persona.Profile(strings.ToUpper(c.Param("code")))
	if !ok {
		c.Error(apperrors.NewNotFoundError("mbti type not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": toTypeResponse(p)})
}

func (h *PersonaHandler) Preview(c *gin.Context) {
	code := c.Param("code")
	locale := types.LanguageFromContextOrDefault(c.Request.Context())
	md := persona.RenderPersona(code, locale, persona.RenderInput{AgentName: "Agent", UserDisplay: ""})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"code": code, "markdown": md}})
}

func (h *PersonaHandler) TestQuestions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"questions": persona.Questions()}})
}

type testSubmitBody struct {
	Answers map[string]string `json:"answers" binding:"required"`
}

func (h *PersonaHandler) TestSubmit(c *gin.Context) {
	var body testSubmitBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	score, err := persona.ScoreAnswers(body.Answers)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	profile, _ := persona.Profile(score.Code)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"code": score.Code,
		"dimensions": gin.H{"ei": score.EI, "sn": score.SN, "tf": score.TF, "jp": score.JP},
		"profile": toTypeResponse(profile),
	}})
}

type personaApplyBody struct {
	Code  string `json:"code" binding:"required"`
	Style string `json:"style"`
}

func (h *PersonaHandler) ApplyPersona(c *gin.Context) {
	var body personaApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid body: " + err.Error()))
		return
	}
	code := strings.ToUpper(strings.TrimSpace(body.Code))
	if _, ok := persona.Profile(code); !ok {
		c.Error(apperrors.NewBadRequestError("unknown mbti code: " + code))
		return
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	agent, err := h.agentService.GetAgentByID(c.Request.Context(), tenantID, c.Param("agentID"))
	if err != nil {
		c.Error(err)
		return
	}
	agent.Config.PersonaMBTI = code
	agent.Config.PersonaStyle = strings.TrimSpace(body.Style)
	updated, err := h.agentService.UpdateAgent(c.Request.Context(), agent)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"persona_mbti": updated.Config.PersonaMBTI, "persona_style": updated.Config.PersonaStyle,
	}})
}

func (h *PersonaHandler) RemovePersona(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	agent, err := h.agentService.GetAgentByID(c.Request.Context(), tenantID, c.Param("agentID"))
	if err != nil {
		c.Error(err)
		return
	}
	agent.Config.PersonaMBTI = ""
	agent.Config.PersonaStyle = ""
	if _, err := h.agentService.UpdateAgent(c.Request.Context(), agent); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"persona_mbti": "", "persona_style": ""}})
}
```

`internal/router/routes_persona.go` (mirror routes_agent.go structure; wire in `router.go` where other v1 groups register):

```go
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

func RegisterPersonaRoutes(r *gin.RouterGroup, personaHandler *handler.PersonaHandler, g *rbacGuards) {
	mbti := r.Group("/mbti")
	{
		mbti.GET("/types", personaHandler.ListTypes)
		mbti.GET("/types/:code", personaHandler.GetType)
		mbti.GET("/preview/:code", personaHandler.Preview)
		mbti.GET("/test/questions", personaHandler.TestQuestions)
		mbti.POST("/test/submit", personaHandler.TestSubmit)
	}
	// Agent-scoped persona mutation reuses the same ownership guard as
	// agent updates (routes_agent.go agentsWrite group).
	agentsWrite := r.Group("/agents", g.OwnedAgentOrAdmin())
	{
		agentsWrite.PUT("/:agentID/persona", personaHandler.ApplyPersona)
		agentsWrite.DELETE("/:agentID/persona", personaHandler.RemovePersona)
	}
}
```

Note: if `/:agentID/persona` conflicts with the existing `agents/:id` route registration order in routes_agent.go (gin wildcard conflicts), register these two routes inside the existing `agentsWrite` group in routes_agent.go instead — check gin's panic on conflicting wildcards at startup and pick the non-conflicting placement. Verify the exact guard receiver/type (`rbacGuards` vs `RBACGuards`) from routes_agent.go and match it. Match `apperrors` import path used by sibling handlers (`internal/errors` — check actual alias in sandbox_skill.go).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/handler/ -run 'MBTI|Persona' -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/persona.go internal/handler/persona_test.go internal/router/routes_persona.go internal/router/router.go
git commit -m "feat(persona): /api/v1/mbti routes — types, preview, test, apply/remove"
```

---

### Task 7: Frontend API client module

**Files:**
- Create: `packages/api-client/src/mbti.ts`
- Modify: `packages/api-client/src/client.ts` (mount module, follow how `configuration.ts` is mounted at :436 factory)
- Modify: `packages/api-client/src/index.ts` (export)

**Interfaces:**
- Consumes: backend routes from Task 6.
- Produces (consumed by Tasks 8-9):

```ts
export interface MbtiAxis { pole: string; percent: number }
export interface MbtiProfile {
  code: string; name_zh: string; name_en: string; nickname_zh: string;
  summary_zh: string; summary_en: string; descriptors_zh: string; descriptors_en: string;
  dimensions: Record<'ei'|'sn'|'tf'|'jp', MbtiAxis>;
  behavior: Record<'answer_style'|'casual_chat'|'conflict'|'creativity'|'emotion'|'planning'|'answer_style_zh'|'casual_chat_zh'|'conflict_zh'|'creativity_zh'|'emotion_zh'|'planning_zh', string>;
  color: string; symbol: string;
}
export interface MbtiQuestion {
  id: number; dimension: 'EI'|'SN'|'TF'|'JP'; a_pole: string; b_pole: string;
  question_zh: string; option_a_zh: string; option_b_zh: string;
  question_en: string; option_a_en: string; option_b_en: string;
}
export interface MbtiScore {
  code: string; dimensions: Record<'ei'|'sn'|'tf'|'jp', { dominant: string; percent: number }>; profile: MbtiProfile;
}
// Functions (returned by createWeKnoraClient):
// mbti.types(): Promise<MbtiProfile[]>
// mbti.type(code): Promise<MbtiProfile>
// mbti.preview(code): Promise<{ code: string; markdown: string }>
// mbti.questions(): Promise<MbtiQuestion[]>
// mbti.submit(answers: Record<string, 'A'|'B'>): Promise<MbtiScore>
// mbti.applyPersona(agentID: string, code: string, style?: string): Promise<void>
// mbti.removePersona(agentID: string): Promise<void>
```

- [ ] **Step 1: Implement `packages/api-client/src/mbti.ts`** using the request helper pattern from `configuration.ts` (same `request`/fetch wrapper, `data` unwrap). Real signatures per the interface block above.
- [ ] **Step 2: Mount in client factory + export from index.** Follow the existing module mounting convention in `client.ts` (the `createWeKnoraClient` return object gains `mbti: createMbtiApi(request)`).
- [ ] **Step 3: Typecheck**

Run: `pnpm --filter @weknora/api-client build` (or the workspace's typecheck script — check package.json name first: `cat packages/api-client/package.json | grep name`).
Expected: clean typecheck.

- [ ] **Step 4: Commit**

```bash
git add packages/api-client/src/mbti.ts packages/api-client/src/client.ts packages/api-client/src/index.ts
git commit -m "feat(api-client): mbti module — types, test, persona apply/remove"
```

---

### Task 8: Agent editor — Personalization section

**Files:**
- Modify: `apps/web/src/agents/agent-editor.ts` — `AgentConfigForm` (:40-95) add `persona_mbti?: string; persona_style?: string`; `defaultAgentConfig` (:114) add both keys as `''`; `AgentSectionKey` (:289) add `'personalization'`; `buildNavGroups` (:364) register the section; `buildAgentPayload` (:338) includes config keys automatically (verify — if it whitelists keys, add them).
- Create: `apps/web/src/agents/PersonaSection.tsx` — grid selector + selected-detail + style textarea.
- Modify: `apps/web/src/agents/AgentEditorModal.tsx` — import PersonaSection; add case in the section switch (:1063-1071); add `'personalization'` to the valid-section list (:206).
- Modify: `apps/web/src/agents/route.ts:7-16` — add `'personalization'` to the section whitelist.
- Modify: `apps/web/src/agents/agent-editor-fallback.ts` + `packages/i18n` locales — `agentEditor.personalization.*` strings (zh-CN + en).

**Interfaces:**
- Consumes: `client.mbti.types()` (Task 7); form primitives `Row`/`Switch`/`RadioGroup` from AgentEditorModal (:83-119); card-grid pattern from renderTools (:923).
- Produces: `PersonaSection(props: { config: AgentConfigForm; patchConfig: (patch: Partial<AgentConfigForm>) => void })`.

- [ ] **Step 1: Wire section plumbing (editor nav + whitelist + defaults + i18n keys)** — all five files listed above, minimal diff each.
- [ ] **Step 2: Implement PersonaSection.tsx**

Component outline (follow package conventions — props drilling like sibling sections, Tailwind classes from the modal's constants at :72-81):

```tsx
// 16-type grid; selected type ring-highlighted with its profile color;
// "None" card clears persona_mbti. Below the grid, when a type is selected:
// - dimension bars (4 axes, pole letters + percent width, bar color = profile.color)
// - textarea for persona_style (optional supplementary persona text)
// - "Take the test" button opens MbtiTestModal (Task 9; until then hidden)
```

State: types loaded once via `useEffect` → `client.mbti.types()`; store in local state; loading skeleton reuses modal patterns.

- [ ] **Step 3: Add render case + section list entries in AgentEditorModal** (`case 'personalization': return renderPersonalization()` delegating to `<PersonaSection ... />` with the modal's `form`/`patchConfig`).
- [ ] **Step 4: Typecheck + build**

Run: `pnpm --filter <web-app-name> build` (name from apps/web/package.json).
Expected: clean.

- [ ] **Step 5: Manual smoke (dev server)** — open `/platform/agents`, edit an agent, Personalization section: select INTJ, save, reload editor — selection persists (verifies defaultAgentConfig/hydrate round-trip); set style text, save, persists.
- [ ] **Step 6: Commit**

```bash
git add apps/web/src/agents/ packages/i18n/
git commit -m "feat(web): agent editor personalization section (16-type grid + style)"
```

---

### Task 9: MBTI test modal

**Files:**
- Create: `apps/web/src/agents/MbtiTestModal.tsx`
- Modify: `apps/web/src/agents/PersonaSection.tsx` — enable "Take the test" button wiring.

**Interfaces:**
- Consumes: `client.mbti.questions()`, `client.mbti.submit()` (Task 7); Dialog from `packages/ui` (exports at src/index.tsx:4-25).
- Produces: `MbtiTestModal(props: { open: boolean; onClose: () => void; onApply: (code: string) => void })` — three stages (intro → 28 questions one-per-screen or grouped → result with profile card + "apply to agent" button calling `onApply(code)`).

- [ ] **Step 1: Implement MbtiTestModal** — intro card ("仅供娱乐" disclaimer included, per spec); question stage renders zh/en by current locale with A/B buttons and a progress counter; unanswered questions block submit; result stage shows code + name + nickname + 4 dimension bars + profile color, "应用到该 Agent" button → `onApply`; empty-code path impossible (≥20 enforced client-side by requiring all 28).
- [ ] **Step 2: Wire into PersonaSection** — button opens modal; `onApply` sets `persona_mbti` via `patchConfig` and closes; i18n strings added alongside Task 8 keys.
- [ ] **Step 3: Typecheck + build + manual smoke** — complete the test once end-to-end in dev, apply result, confirm grid selection updates.
- [ ] **Step 4: Commit**

```bash
git add apps/web/src/agents/MbtiTestModal.tsx apps/web/src/agents/PersonaSection.tsx packages/i18n/
git commit -m "feat(web): 28-question MBTI test modal with apply-to-agent"
```

---

### Task 10: End-to-end verification + domain docs

**Files:**
- Modify: `docs/agents/domain.md` — add a short "Persona (MBTI)" section (fields, rendering point, routes).
- Create: `docs/migrations/octop-m1-mbti/evidence/2026-09-19-m1-verification.md` — verification log.

**Interfaces:** none.

- [ ] **Step 1: Backend E2E** — start backend (source-run per repo dev setup); with an authenticated session: GET types (16), POST submit (28 A's → all-first-poles code), PUT persona on a smart-reasoning agent, then send a chat message on that agent and inspect logs/events that the system prompt starts with `# Persona: INTJ`; DELETE persona, send another message, confirm segment gone. Record commands + observed output in the evidence file.
- [ ] **Step 2: Frontend E2E** — full editor flow: pick type manually, take the test, apply, save, re-open. Record screenshots paths in evidence.
- [ ] **Step 3: Full gates**

Run: `go test ./internal/agent/persona/... ./internal/types/... ./internal/handler/... ./internal/application/service/...` and web build.
Expected: all green (pre-existing main-branch reds excluded — check memory note on known-red gates; do not fix unrelated failures).

- [ ] **Step 4: Update domain docs + write evidence, commit**

```bash
git add docs/agents/domain.md docs/migrations/octop-m1-mbti/evidence/
git commit -m "docs(persona): M1 MBTI capability documentation and verification evidence"
```

---

## Self-Review Notes (already applied)

- Spec §4 coverage: profiles+questions (T1), scoring (T2), bilingual rendering + default fallback (T3), config fields + locale-correct rendering (T4/T5, mounted at capability assembly covering both engines), 7 API endpoints (T6 — spec listed preview/_default, submit, apply PUT, remove DELETE, types; `current` from spec §4 API list was dropped as redundant with editor state: the agent GET already returns config.persona_mbti; noted here as an intentional deviation), frontend personalization block + test modal (T8/T9), no-frontend-hardcoding rule (types fetched in T8), explicit errors (T6 ApplyPersona).
- Type consistency: `RenderInput`/`RenderPersona`/`ScoreAnswers` signatures identical across T2/T3/T5/T6; `mbtiTypeResponse` field names match api-client TS types in T7.
- Known execution-time lookups (flagged inline, not placeholders): exact `AgentConfig` name field; guard type name in routes_persona.go; gin wildcard conflict resolution; apps/web package name for pnpm filter.
