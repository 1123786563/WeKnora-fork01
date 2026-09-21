package types

import (
	"reflect"
	"testing"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

func TestSemanticDeleteRequiresDeletedRevision(t *testing.T) {
	_, err := SemanticDeleteDocumentRevisionFromWire(&semanticpb.DocumentRevision{Deleted: false})
	if err == nil {
		t.Fatal("expected delete revision validation error")
	}
}

func TestSemanticDeleteToWireRequiresTombstone(t *testing.T) {
	_, err := SemanticDeleteDocumentRevisionToWire(SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, DocumentID: "doc", Revision: 1, Deleted: false})
	if err == nil {
		t.Fatal("expected outbound delete tombstone validation")
	}
}

func TestSemanticApplyAllowsNonDeletedRevision(t *testing.T) {
	revision, err := SemanticDocumentRevisionFromWire(&semanticpb.DocumentRevision{Scope: &semanticpb.ScopeKey{TenantId: 1, KbId: "kb"}, DocumentId: "doc", Revision: 1, ContentHash: "hash", Deleted: false})
	if err != nil || revision.Deleted {
		t.Fatalf("apply revision = %#v, %v", revision, err)
	}
}

func TestSemanticDocumentRevisionWireRoundTrip(t *testing.T) {
	original := SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, DocumentID: "doc", Revision: 1, ContentHash: "hash", Deleted: false}
	wire, err := SemanticDocumentRevisionToWire(original)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SemanticDocumentRevisionFromWire(wire)
	if err != nil || result != original {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticOperationRefWireRoundTrip(t *testing.T) {
	original := SemanticOperationRef{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, OperationID: "op"}
	wire, err := SemanticOperationRefToWire(original)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SemanticOperationRefFromWire(wire)
	if err != nil || result != original {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticOperationWireRoundTrip(t *testing.T) {
	resultGeneration := "g"
	errorCode := "ERR"
	original := SemanticOperation{OperationID: "op", Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, DocumentID: "doc", Revision: 1, State: SemanticOperationStateRunning, Stage: "index", LeaseToken: 2, ResultGeneration: &resultGeneration, ErrorCode: &errorCode}
	wire, err := SemanticOperationToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticOperationFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticSearchResponseEchoesBothModes(t *testing.T) {
	response, err := SemanticSearchResponseFromWire(&semanticpb.SearchResponse{
		RequestedMode: semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG,
		ActualMode:    semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG,
	})
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	if response.RequestedMode != SemanticRetrievalModeGraphRAG || response.ActualMode != SemanticRetrievalModeGraphRAG {
		t.Fatalf("mode echo = %#v", response)
	}
}

func TestSemanticSearchResponseWireRoundTrip(t *testing.T) {
	start, end := uint32(2), uint32(5)
	original := SemanticSearchResponse{QueryID: "q", Generation: "g", RequestedMode: SemanticRetrievalModeGraphRAG, ActualMode: SemanticRetrievalModeGraphRAG, Evidence: []SemanticEvidence{{EvidenceID: "e", DocumentID: "d", Revision: 1, ChunkID: "c", ContentHash: "h", Quote: "text", StartChar: &start, EndChar: &end}}, AssertionIDs: []string{"a"}, Paths: [][]string{{"a", "b"}}, Stale: true, Partial: true, Truncated: true}
	wire, err := SemanticSearchResponseToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticSearchResponseFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonResponseWireRoundTripPreservesAbsentConclusion(t *testing.T) {
	version, kind := "model-v1", "rule"
	retrieval := SemanticSearchResponse{QueryID: "q", Generation: "g", RequestedMode: SemanticRetrievalModeReason, ActualMode: SemanticRetrievalModeReason}
	original := SemanticReasonResponse{Status: SemanticReasonStatusInsufficientEvidence, Retrieval: &retrieval, ConclusionKind: &kind, PremiseIDs: []string{"p1"}, RuleIDs: []string{"r1"}, ModelVersion: &version, Limitations: []string{"missing"}}
	wire, err := SemanticReasonResponseToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticReasonResponseFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) || result.Conclusion != nil {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonResponseWireRoundTripPreservesPresentOptionals(t *testing.T) {
	conclusion, kind, model, prompt := "A→C", "rule", "model-v1", "prompt-v1"
	retrieval := SemanticSearchResponse{QueryID: "q", Generation: "g", RequestedMode: SemanticRetrievalModeReason, ActualMode: SemanticRetrievalModeReason, AssertionIDs: []string{"a1", "a2"}}
	original := SemanticReasonResponse{Status: SemanticReasonStatusDerived, Conclusion: &conclusion, Retrieval: &retrieval, ConclusionKind: &kind, PremiseIDs: []string{"a1", "a2"}, RuleIDs: []string{"r1"}, ModelVersion: &model, PromptVersion: &prompt, Limitations: []string{"bounded"}}
	wire, err := SemanticReasonResponseToWire(original)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SemanticReasonResponseFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticSearchRequestWireRoundTrip(t *testing.T) {
	scope := SemanticAccessScope{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: SemanticAccessPurposeSearch, BudgetRef: "budget"}
	original := SemanticSearchRequest{QueryID: "q", Query: "query", AccessScope: scope, Limits: SemanticQueryLimits{MaxHops: 2, MaxNodes: 10, MaxEdges: 20, TopK: 3, MaxTokens: 100, DeadlineMS: 1000}, RequestedMode: SemanticRetrievalModeGraphRAG}
	wire, err := SemanticSearchRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticSearchRequestFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonRequestWireRoundTrip(t *testing.T) {
	scope := SemanticAccessScope{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: SemanticAccessPurposeReason, BudgetRef: "budget"}
	search := SemanticSearchRequest{QueryID: "q", Query: "query", AccessScope: scope, Limits: SemanticQueryLimits{MaxHops: 2}, RequestedMode: SemanticRetrievalModeReason}
	original := SemanticReasonRequest{Search: search, ReasoningMode: SemanticReasoningModeRules, RuleSetVersion: "v1"}
	wire, err := SemanticReasonRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticReasonRequestFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonRequestRejectsSearchScope(t *testing.T) {
	scope := SemanticAccessScope{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 1, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: SemanticAccessPurposeSearch, BudgetRef: "budget"}
	_, err := SemanticReasonRequestToWire(SemanticReasonRequest{Search: SemanticSearchRequest{QueryID: "q", Query: "query", AccessScope: scope, RequestedMode: SemanticRetrievalModeGraphRAG}, ReasoningMode: SemanticReasoningModeRules, RuleSetVersion: "v1"})
	if err == nil {
		t.Fatal("expected reason request scope/mode rejection")
	}
}

func TestSemanticApplyRequestWireRoundTrip(t *testing.T) {
	manifest := "manifest-v1"
	original := SemanticApplyRequest{Document: SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, DocumentID: "doc", Revision: 1, ContentHash: "h"}, Chunks: []SemanticChunkSnapshot{{ChunkID: "c", Text: "text", ContentHash: "ch"}}, Config: SemanticIndexConfig{ConfigDigest: "d", EngineVersion: "e", ModelProfileRef: "m", PromptVersion: "p", RuleSetVersion: "r", SchemaVersion: "s"}, IdempotencyKey: "idem", PayloadHash: "payload", ManifestRef: &manifest}
	wire, err := SemanticApplyRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticApplyRequestFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticCapabilitiesWireRoundTrip(t *testing.T) {
	original := SemanticCapabilities{ProtocolVersion: "v1", EngineVersion: "engine", RetrievalModes: []SemanticRetrievalMode{SemanticRetrievalModeGraphRAG}, ReasoningModes: []SemanticReasoningMode{SemanticReasoningModeRules}, Limits: SemanticQueryLimits{MaxNodes: 2}, Limitations: []string{"limit"}}
	wire, err := SemanticCapabilitiesToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticCapabilitiesFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticCapabilitiesWireRoundTripPreservesUnavailableReason(t *testing.T) {
	unavailable := "reasoning disabled"
	original := SemanticCapabilities{ProtocolVersion: "v1", EngineVersion: "engine", RetrievalModes: []SemanticRetrievalMode{SemanticRetrievalModeGraphRAG, SemanticRetrievalModeReason}, ReasoningModes: []SemanticReasoningMode{SemanticReasoningModeRules, SemanticReasoningModeModel}, Limits: SemanticQueryLimits{MaxHops: 2, MaxNodes: 10, MaxEdges: 20, TopK: 3, MaxTokens: 100, DeadlineMS: 1000}, Limitations: []string{"bounded"}, UnavailableReason: &unavailable}
	wire, err := SemanticCapabilitiesToWire(original)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SemanticCapabilitiesFromWire(wire)
	if err != nil || !reflect.DeepEqual(result, original) {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticSearchResponseRejectsImplicitReasonUpgrade(t *testing.T) {
	_, err := SemanticSearchResponseFromWire(&semanticpb.SearchResponse{
		RequestedMode: semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG,
		ActualMode:    semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON,
	})
	if err == nil {
		t.Fatal("expected implicit reason upgrade rejection")
	}
}

func TestSemanticEvidencePreservesAbsentSpanAndAccessScope(t *testing.T) {
	evidence, err := SemanticEvidenceFromWire(&semanticpb.Evidence{EvidenceId: "e", DocumentId: "d", Revision: 1, ChunkId: "c", ContentHash: "h", Quote: "text"})
	if err != nil || evidence.StartChar != nil || evidence.EndChar != nil {
		t.Fatalf("evidence = %#v, %v", evidence, err)
	}
	evidenceWire, err := SemanticEvidenceToWire(evidence)
	if err != nil {
		t.Fatal(err)
	}
	roundTripEvidence, err := SemanticEvidenceFromWire(evidenceWire)
	if err != nil || roundTripEvidence != evidence {
		t.Fatalf("evidence round trip = %#v, %v", roundTripEvidence, err)
	}
	scope, err := SemanticAccessScopeFromWire(&semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: 1, KbId: "kb"}, SubjectId: "user", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: semanticpb.Purpose_PURPOSE_SEARCH, BudgetRef: "budget"})
	if err != nil || scope.PermissionEpoch != 2 || scope.ExpiresAt != "2026-09-20T00:00:00Z" {
		t.Fatalf("scope = %#v, %v", scope, err)
	}
	accessWire, err := SemanticAccessScopeToWire(scope)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := SemanticAccessScopeFromWire(accessWire)
	if err != nil || roundTrip != scope {
		t.Fatalf("scope round trip = %#v, %v", roundTrip, err)
	}
}

func TestSemanticEvidenceToWireRejectsMalformed(t *testing.T) {
	if _, err := SemanticEvidenceToWire(SemanticEvidence{Revision: 1}); err == nil {
		t.Fatal("expected outbound evidence validation")
	}
}

func TestSemanticAccessScopeRejectsMalformedExpiryAndUnknownPurpose(t *testing.T) {
	base := &semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: 1, KbId: "kb"}, SubjectId: "user", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "not-a-time", Audience: "semantic", Purpose: semanticpb.Purpose_PURPOSE_SEARCH, BudgetRef: "budget"}
	if _, err := SemanticAccessScopeFromWire(base); err == nil {
		t.Fatal("expected malformed expiry rejection")
	}
	base.ExpiresAt = "2026-09-20T00:00:00Z"
	base.Purpose = semanticpb.Purpose(99)
	if _, err := SemanticAccessScopeFromWire(base); err == nil {
		t.Fatal("expected unknown purpose rejection")
	}
}

func TestSemanticInboundScopeRejectsZeroTenant(t *testing.T) {
	if _, err := SemanticDocumentRevisionFromWire(&semanticpb.DocumentRevision{Scope: &semanticpb.ScopeKey{TenantId: 0, KbId: "kb"}, DocumentId: "doc", Revision: 1}); err == nil {
		t.Fatal("expected document zero tenant rejection")
	}
	if _, err := SemanticOperationRefFromWire(&semanticpb.OperationRef{Scope: &semanticpb.ScopeKey{TenantId: 0, KbId: "kb"}, OperationId: "op"}); err == nil {
		t.Fatal("expected operation ref zero tenant rejection")
	}
	if _, err := SemanticAccessScopeFromWire(&semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: 0, KbId: "kb"}, SubjectId: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 1, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: semanticpb.Purpose_PURPOSE_SEARCH, BudgetRef: "budget"}); err == nil {
		t.Fatal("expected scope zero tenant rejection")
	}
}
