package types

import (
	"fmt"
	"time"
)

// T15（Issue #45）：有证据的知识问答。三类结论区分与逐引用版本/时间证据的生产
// wire 类型。CONTEXT.md「证据引用」：可打开的记录，包含来源和获取时间，并标明
// 结论属于原文事实、规则推导还是模型推断；避免把模型推断标成来源事实。

// EvidenceKind 标注一条结论/引用的知识来源类别（三类互斥，不得混标）。
type EvidenceKind string

const (
	// EvidenceKindFact 原文事实：直接来自获准知识源的检索命中行。
	EvidenceKindFact EvidenceKind = "fact"
	// EvidenceKindRuleDerived 规则推导：开发者注册的版本化规则链推得的结论（ADR-0002）。
	EvidenceKindRuleDerived EvidenceKind = "rule_derived"
	// EvidenceKindModelInferred 模型推断：LLM 生成或语义推理 model 模式的结论。
	EvidenceKindModelInferred EvidenceKind = "model_inferred"
)

func (k EvidenceKind) IsValid() bool {
	return k == EvidenceKindFact || k == EvidenceKindRuleDerived || k == EvidenceKindModelInferred
}

// AnswerEvidenceState 一次问答交付的证据状态。
type AnswerEvidenceState string

const (
	// EvidenceStateCited 有获准证据支撑（≥1 引用 + ≥1 结论）。
	EvidenceStateCited AnswerEvidenceState = "cited"
	// EvidenceStateNoEvidence 检索无命中：回答是 fallback，零引用零结论。
	EvidenceStateNoEvidence AnswerEvidenceState = "no_evidence"
	// EvidenceStateRevoked 交付前证据全部失权：本回答作废（ADR-0002）。
	EvidenceStateRevoked AnswerEvidenceState = "revoked"
)

func (s AnswerEvidenceState) IsValid() bool {
	return s == EvidenceStateCited || s == EvidenceStateNoEvidence || s == EvidenceStateRevoked
}

// EvidenceReasoningState 显式推理请求的执行状态。
type EvidenceReasoningState string

const (
	EvidenceReasoningNotRequested EvidenceReasoningState = "not_requested"
	EvidenceReasoningIncomplete   EvidenceReasoningState = "incomplete"
)

// EvidenceCitation 一条可打开的原文事实引用：知识文档、其所在知识库、
// chunk 编辑版本（revision）、检索时间与可选的原文坐标/引文。
type EvidenceCitation struct {
	CitationID      string       `json:"citation_id"`
	KnowledgeID     string       `json:"knowledge_id"`
	KnowledgeBaseID string       `json:"knowledge_base_id"`
	Title           string       `json:"title,omitempty"`
	Revision        int          `json:"revision"`
	StartAt         int          `json:"start_at,omitempty"`
	EndAt           int          `json:"end_at,omitempty"`
	Quote           string       `json:"quote,omitempty"`
	Kind            EvidenceKind `json:"kind"`
	RetrievedAt     string       `json:"retrieved_at"`
}

// EvidenceConclusion 一条结论的类别与审计锚点：规则推导必须带规则 ID，
// 模型推断必须带模型标识——否则与原文事实不可区分（AC1）。
type EvidenceConclusion struct {
	Kind        EvidenceKind `json:"kind"`
	ModelID     string       `json:"model_id,omitempty"`
	RuleIDs     []string     `json:"rule_ids,omitempty"`
	CitationIDs []string     `json:"citation_ids,omitempty"`
}

// EvidenceReasoning 显式推理请求的诚实状态：unavailable 时 state=incomplete
// 且 retryable=true，绝不用普通检索结果冒充推理成功（ADR-0002）。
type EvidenceReasoning struct {
	Requested bool                   `json:"requested"`
	Mode      string                 `json:"mode,omitempty"`
	State     EvidenceReasoningState `json:"state"`
	Reason    string                 `json:"reason,omitempty"`
	Retryable bool                   `json:"retryable"`
}

// AnswerEvidence 是 knowledge-chat SSE evidence 帧的载荷（信封）。
type AnswerEvidence struct {
	State             AnswerEvidenceState  `json:"state"`
	SemanticGraphUsed bool                 `json:"semantic_graph_used"`
	RetrievedAt       string               `json:"retrieved_at"`
	Citations         []EvidenceCitation   `json:"citations"`
	Conclusions       []EvidenceConclusion `json:"conclusions"`
	Reasoning         EvidenceReasoning    `json:"reasoning"`
}

// evidenceQuoteMaxRunes 限制单条引用的原文引文长度（信封不搬运整段正文）。
const evidenceQuoteMaxRunes = 200

// CitationsFromSearchResults 把检索命中行映射为原文事实引用：按 chunk ID 去重、
// 引文截断、统一盖检索时间戳；无原文坐标（或被 merge 重写）的行不伪造位置。
func CitationsFromSearchResults(results []*SearchResult, retrievedAt time.Time) []EvidenceCitation {
	stamp := retrievedAt.UTC().Format(time.RFC3339)
	seen := make(map[string]bool, len(results))
	citations := make([]EvidenceCitation, 0, len(results))
	for _, result := range results {
		if result == nil || result.ID == "" || result.KnowledgeID == "" || result.KnowledgeBaseID == "" {
			continue
		}
		if seen[result.ID] {
			continue
		}
		seen[result.ID] = true
		quote := []rune(result.Content)
		if len(quote) > evidenceQuoteMaxRunes {
			quote = quote[:evidenceQuoteMaxRunes]
		}
		citations = append(citations, EvidenceCitation{
			CitationID:      result.ID,
			KnowledgeID:     result.KnowledgeID,
			KnowledgeBaseID: result.KnowledgeBaseID,
			Title:           result.KnowledgeTitle,
			Revision:        result.ContentRevision,
			StartAt:         result.StartAt,
			EndAt:           result.EndAt,
			Quote:           string(quote),
			Kind:            EvidenceKindFact,
			RetrievedAt:     stamp,
		})
	}
	return citations
}

// ConclusionFromNativeAnswer 标注本地检索问答的答案结论：LLM 总结属模型推断。
func ConclusionFromNativeAnswer(modelID string, citationIDs []string) EvidenceConclusion {
	return EvidenceConclusion{Kind: EvidenceKindModelInferred, ModelID: modelID, CitationIDs: append([]string(nil), citationIDs...)}
}

// ConclusionFromSemanticReason 标注语义推理（Semantica）结论：rules 模式必须是
// 规则推导且携带非空规则 ID；model 模式必须是模型推断且携带模型版本。证据不足、
// 冲突或不可用态没有可标类的结论，一律报错（调用方按推理未完成处理）。
func ConclusionFromSemanticReason(mode SemanticReasoningMode, reason SemanticReasonResponse) (EvidenceConclusion, error) {
	if reason.Status != SemanticReasonStatusDerived || reason.Conclusion == nil || *reason.Conclusion == "" {
		return EvidenceConclusion{}, fmt.Errorf("semantic reason did not derive a conclusion (status=%s)", reason.Status)
	}
	switch mode {
	case SemanticReasoningModeRules:
		if len(reason.RuleIDs) == 0 {
			return EvidenceConclusion{}, fmt.Errorf("rule-derived conclusion requires non-empty rule ids")
		}
		return EvidenceConclusion{Kind: EvidenceKindRuleDerived, RuleIDs: append([]string(nil), reason.RuleIDs...)}, nil
	case SemanticReasoningModeModel:
		if reason.ModelVersion == nil || *reason.ModelVersion == "" {
			return EvidenceConclusion{}, fmt.Errorf("model-inferred conclusion requires a model version")
		}
		return EvidenceConclusion{Kind: EvidenceKindModelInferred, ModelID: *reason.ModelVersion}, nil
	default:
		return EvidenceConclusion{}, fmt.Errorf("unsupported reasoning mode %q", mode)
	}
}

// ValidateAnswerEvidence 是交付不变量：信封在发出（服务端）与渲染（客户端解析后）
// 两侧都必须自洽——cited 必须有引用与结论；no_evidence/revoked 必须零引用零结论；
// 引用行只能是原文事实；规则推导结论必须带规则 ID、模型推断结论必须带模型标识；
// 结论不得引用不存在的 citation；所有时间是 RFC3339。
func ValidateAnswerEvidence(e AnswerEvidence) error {
	if !e.State.IsValid() {
		return fmt.Errorf("evidence state %q is invalid", e.State)
	}
	if _, err := time.Parse(time.RFC3339, e.RetrievedAt); err != nil {
		return fmt.Errorf("evidence retrieved_at must be RFC3339")
	}
	if e.State == EvidenceStateCited && len(e.Citations) == 0 {
		return fmt.Errorf("cited evidence requires at least one citation")
	}
	if e.State != EvidenceStateCited && (len(e.Citations) > 0 || len(e.Conclusions) > 0) {
		return fmt.Errorf("state %q must not carry citations or conclusions", e.State)
	}
	if e.State == EvidenceStateCited && len(e.Conclusions) == 0 {
		return fmt.Errorf("cited evidence requires at least one conclusion")
	}
	citationIDs := make(map[string]bool, len(e.Citations))
	for i, citation := range e.Citations {
		if citation.CitationID == "" || citation.KnowledgeID == "" || citation.KnowledgeBaseID == "" {
			return fmt.Errorf("citations[%d] is not attributable to a knowledge source", i)
		}
		if citation.Kind != EvidenceKindFact {
			return fmt.Errorf("citations[%d].kind must be fact", i)
		}
		if citation.Revision < 0 {
			return fmt.Errorf("citations[%d].revision must not be negative", i)
		}
		if _, err := time.Parse(time.RFC3339, citation.RetrievedAt); err != nil {
			return fmt.Errorf("citations[%d].retrieved_at must be RFC3339", i)
		}
		if citationIDs[citation.CitationID] {
			return fmt.Errorf("citations[%d].citation_id is duplicated", i)
		}
		citationIDs[citation.CitationID] = true
	}
	for i, conclusion := range e.Conclusions {
		if !conclusion.Kind.IsValid() {
			return fmt.Errorf("conclusions[%d].kind %q is invalid", i, conclusion.Kind)
		}
		if conclusion.Kind == EvidenceKindRuleDerived && len(conclusion.RuleIDs) == 0 {
			return fmt.Errorf("conclusions[%d] rule-derived requires rule ids", i)
		}
		if conclusion.Kind == EvidenceKindModelInferred && conclusion.ModelID == "" {
			return fmt.Errorf("conclusions[%d] model-inferred requires a model id", i)
		}
		for _, id := range conclusion.CitationIDs {
			if !citationIDs[id] {
				return fmt.Errorf("conclusions[%d] references unknown citation %q", i, id)
			}
		}
	}
	if e.Reasoning.State == "" {
		return fmt.Errorf("reasoning state is required")
	}
	return nil
}
