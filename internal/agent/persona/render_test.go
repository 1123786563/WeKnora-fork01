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
