// Package persona provides MBTI personality profiles, the 28-question test,
// and system-prompt persona rendering. Data is imported verbatim from Octop
// (scripts/import_octop_mbti.py) and embedded at build time.
package persona

// Axis is one end of an MBTI dimension: the dominant pole ("I", "N", ...)
// and its strength as a percentage (50-85).
type Axis struct {
	Pole    string `json:"pole"`
	Percent int    `json:"percent"`
}

// MBTIDimensions holds the four axis scores of a profile.
type MBTIDimensions struct {
	EI Axis `json:"ei"`
	SN Axis `json:"sn"`
	TF Axis `json:"tf"`
	JP Axis `json:"jp"`
}

// MBTIBehavior holds behaviour guidance strings (English + Chinese) for
// different interaction contexts.
type MBTIBehavior struct {
	AnswerStyle   string `json:"answer_style"`
	CasualChat    string `json:"casual_chat"`
	Conflict      string `json:"conflict"`
	Creativity    string `json:"creativity"`
	Emotion       string `json:"emotion"`
	Planning      string `json:"planning"`
	AnswerStyleZh string `json:"answer_style_zh"`
	CasualChatZh  string `json:"casual_chat_zh"`
	ConflictZh    string `json:"conflict_zh"`
	CreativityZh  string `json:"creativity_zh"`
	EmotionZh     string `json:"emotion_zh"`
	PlanningZh    string `json:"planning_zh"`
}

// MBTIProfile is the complete persona profile for a single MBTI type.
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

// TestQuestion is one forced-choice A/B item of the 28-question test.
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
