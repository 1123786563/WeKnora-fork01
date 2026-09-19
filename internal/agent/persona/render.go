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
