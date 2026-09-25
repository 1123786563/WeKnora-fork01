package types

import (
	"strings"
	"testing"
	"time"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"github.com/stretchr/testify/require"
)

func wireReasonRules(t *testing.T, ruleIDs []string) SemanticReasonResponse {
	t.Helper()
	conclusion := "A 间接依赖 C"
	decoded, err := SemanticReasonResponseFromWire(&semanticpb.ReasonResponse{
		Status:     semanticpb.ReasonStatus_REASON_STATUS_DERIVED,
		Conclusion: &conclusion,
		RuleIds:    ruleIDs,
	})
	require.NoError(t, err)
	return decoded
}

func TestConclusionFromSemanticReasonRulesModeRequiresRuleIDs(t *testing.T) {
	got, err := ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, []string{"dep-transitive"}))
	require.NoError(t, err)
	require.Equal(t, EvidenceKindRuleDerived, got.Kind)
	require.Equal(t, []string{"dep-transitive"}, got.RuleIDs)
	require.Empty(t, got.ModelID, "规则推导结论不得携带模型版本（三类不可混标）")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, nil))
	require.Error(t, err, "规则推导结论缺规则 ID 即无法与原文事实区分，必须拒绝")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, []string{"dep-transitive"}))
	require.NoError(t, err)
	insufficient := wireReasonRules(t, []string{"dep-transitive"})
	insufficient.Status = SemanticReasonStatusInsufficientEvidence
	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, insufficient)
	require.Error(t, err, "证据不足态没有结论可标类")
}

func TestConclusionFromSemanticReasonModelModeRequiresModelVersion(t *testing.T) {
	conclusion := "服务 A 的故障可能影响 B"
	modelVersion := "qwen3-32b"
	decoded, err := SemanticReasonResponseFromWire(&semanticpb.ReasonResponse{
		Status:       semanticpb.ReasonStatus_REASON_STATUS_DERIVED,
		Conclusion:   &conclusion,
		ModelVersion: &modelVersion,
	})
	require.NoError(t, err)

	got, err := ConclusionFromSemanticReason(SemanticReasoningModeModel, decoded)
	require.NoError(t, err)
	require.Equal(t, EvidenceKindModelInferred, got.Kind)
	require.Equal(t, "qwen3-32b", got.ModelID)
	require.Empty(t, got.RuleIDs, "模型推断结论不得携带规则 ID（三类不可混标）")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeModel, wireReasonRules(t, nil))
	require.Error(t, err, "模型推断结论缺模型版本即无法审计，必须拒绝")
}

func TestConclusionFromNativeAnswerIsModelInferred(t *testing.T) {
	got := ConclusionFromNativeAnswer("chat-model-1", []string{"c1", "c2"})
	require.Equal(t, EvidenceKindModelInferred, got.Kind)
	require.Equal(t, "chat-model-1", got.ModelID)
	require.Equal(t, []string{"c1", "c2"}, got.CitationIDs)
	require.Empty(t, got.RuleIDs)
}

func TestCitationsFromSearchResultsDeduplicatesCapsQuoteAndStampsTime(t *testing.T) {
	retrievedAt := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	results := []*SearchResult{
		{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own", KnowledgeTitle: "手册", ContentRevision: 3, StartAt: 10, EndAt: 40, Content: "第一段命中内容"},
		{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own", KnowledgeTitle: "手册", ContentRevision: 3, StartAt: 10, EndAt: 40, Content: "第一段命中内容"},
		{ID: "chunk-2", KnowledgeID: "doc-2", KnowledgeBaseID: "kb-shared", KnowledgeTitle: "指南", ContentRevision: 5, Content: strings.Repeat("长", 500)},
		nil,
	}
	citations := CitationsFromSearchResults(results, retrievedAt)
	require.Len(t, citations, 2, "重复 chunk 与 nil 行不得产生重复/空引用")
	first := citations[0]
	require.Equal(t, "chunk-1", first.CitationID)
	require.Equal(t, "doc-1", first.KnowledgeID)
	require.Equal(t, "kb-own", first.KnowledgeBaseID)
	require.Equal(t, 3, first.Revision)
	require.Equal(t, EvidenceKindFact, first.Kind, "检索命中行本身是原文事实")
	require.Equal(t, retrievedAt.Format(time.RFC3339), first.RetrievedAt)
	second := citations[1]
	require.Equal(t, 5, second.Revision)
	require.LessOrEqual(t, len([]rune(second.Quote)), evidenceQuoteMaxRunes, "引文超长必须截断")
	require.Equal(t, 0, second.StartAt+second.EndAt, "无原文坐标的行不得伪造位置")
}

func validEvidence() AnswerEvidence {
	return AnswerEvidence{
		State:             EvidenceStateCited,
		SemanticGraphUsed: false,
		RetrievedAt:       "2026-09-24T08:00:00Z",
		Citations: []EvidenceCitation{{
			CitationID: "c1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own",
			Revision: 3, Kind: EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}, {
			CitationID: "c2", KnowledgeID: "doc-2", KnowledgeBaseID: "kb-shared",
			Revision: 5, Kind: EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}},
		Conclusions: []EvidenceConclusion{{Kind: EvidenceKindModelInferred, ModelID: "chat-model-1", CitationIDs: []string{"c1", "c2"}}},
		Reasoning:   EvidenceReasoning{State: EvidenceReasoningNotRequested},
	}
}

func TestValidateAnswerEvidenceAcceptsMultiKnowledgeSourceEnvelope(t *testing.T) {
	require.NoError(t, ValidateAnswerEvidence(validEvidence()))
}

func TestValidateAnswerEvidenceRejectsMasqueradeAndInconsistency(t *testing.T) {
	noEvidence := validEvidence()
	noEvidence.State = EvidenceStateNoEvidence
	require.Error(t, ValidateAnswerEvidence(noEvidence), "无证据态携带引用即伪造证据")

	revoked := validEvidence()
	revoked.State = EvidenceStateRevoked
	require.Error(t, ValidateAnswerEvidence(revoked), "撤权作废态携带引用即伪造证据")

	cited := validEvidence()
	cited.State = EvidenceStateCited
	cited.Conclusions = nil
	require.Error(t, ValidateAnswerEvidence(cited), "cited 态必须至少一条结论")

	unknownKind := validEvidence()
	unknownKind.Conclusions[0].Kind = EvidenceKind("oracle")
	require.Error(t, ValidateAnswerEvidence(unknownKind), "白名单外类别必须拒绝")

	citationKind := validEvidence()
	citationKind.Citations[0].Kind = EvidenceKindModelInferred
	require.Error(t, ValidateAnswerEvidence(citationKind), "引用行只能是原文事实")

	dangling := validEvidence()
	dangling.Conclusions[0].CitationIDs = []string{"c1", "missing"}
	require.Error(t, ValidateAnswerEvidence(dangling), "结论不得引用不存在的 citation")

	negativeRevision := validEvidence()
	negativeRevision.Citations[0].Revision = -1
	require.Error(t, ValidateAnswerEvidence(negativeRevision))

	badTime := validEvidence()
	badTime.RetrievedAt = "2026-09-24 08:00:00"
	require.Error(t, ValidateAnswerEvidence(badTime), "检索时间必须是 RFC3339")
}
