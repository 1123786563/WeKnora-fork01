package types

import (
	"testing"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

func TestSemanticDeleteRequiresDeletedRevision(t *testing.T) {
	_, err := SemanticDeleteDocumentRevisionFromWire(&semanticpb.DocumentRevision{Deleted: false})
	if err == nil {
		t.Fatal("expected delete revision validation error")
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
	wire := SemanticDocumentRevisionToWire(original)
	result, err := SemanticDocumentRevisionFromWire(wire)
	if err != nil || result != original {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticOperationRefWireRoundTrip(t *testing.T) {
	original := SemanticOperationRef{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, OperationID: "op"}
	wire := SemanticOperationRefToWire(original)
	result, err := SemanticOperationRefFromWire(wire)
	if err != nil || result != original {
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
	original := SemanticSearchResponse{QueryID: "q", Generation: "g", RequestedMode: SemanticRetrievalModeGraphRAG, ActualMode: SemanticRetrievalModeGraphRAG, Evidence: []SemanticEvidence{{EvidenceID: "e", DocumentID: "d", Revision: 1, ChunkID: "c", ContentHash: "h", Quote: "text"}}, AssertionIDs: []string{"a"}, Paths: [][]string{{"a", "b"}}, Partial: true}
	wire, err := SemanticSearchResponseToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticSearchResponseFromWire(wire)
	if err != nil || result.QueryID != original.QueryID || len(result.Evidence) != 1 || !result.Partial || len(result.Paths) != 1 {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonResponseWireRoundTripPreservesAbsentConclusion(t *testing.T) {
	original := SemanticReasonResponse{Status: SemanticReasonStatusInsufficientEvidence, Limitations: []string{"missing"}}
	wire, err := SemanticReasonResponseToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticReasonResponseFromWire(wire)
	if err != nil || result.Status != original.Status || result.Conclusion != nil || len(result.Limitations) != 1 {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticSearchRequestWireRoundTrip(t *testing.T) {
	scope := SemanticAccessScope{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: "PURPOSE_SEARCH", BudgetRef: "budget"}
	original := SemanticSearchRequest{QueryID: "q", Query: "query", AccessScope: scope, Limits: SemanticQueryLimits{MaxHops: 2, MaxNodes: 10, MaxEdges: 20, TopK: 3, MaxTokens: 100, DeadlineMS: 1000}, RequestedMode: SemanticRetrievalModeGraphRAG}
	wire, err := SemanticSearchRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticSearchRequestFromWire(wire)
	if err != nil || result.QueryID != original.QueryID || result.Limits.MaxNodes != 10 || result.RequestedMode != original.RequestedMode {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticReasonRequestWireRoundTrip(t *testing.T) {
	scope := SemanticAccessScope{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: "PURPOSE_REASON", BudgetRef: "budget"}
	search := SemanticSearchRequest{QueryID: "q", Query: "query", AccessScope: scope, Limits: SemanticQueryLimits{MaxHops: 2}, RequestedMode: SemanticRetrievalModeReason}
	original := SemanticReasonRequest{Search: search, ReasoningMode: SemanticReasoningModeRules, RuleSetVersion: "v1"}
	wire, err := SemanticReasonRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticReasonRequestFromWire(wire)
	if err != nil || result.ReasoningMode != original.ReasoningMode || result.RuleSetVersion != "v1" {
		t.Fatalf("round trip = %#v, %v", result, err)
	}
}

func TestSemanticApplyRequestWireRoundTrip(t *testing.T) {
	original := SemanticApplyRequest{Document: SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: 1, KBID: "kb"}, DocumentID: "doc", Revision: 1, ContentHash: "h"}, Chunks: []SemanticChunkSnapshot{{ChunkID: "c", Text: "text", ContentHash: "ch"}}, Config: SemanticIndexConfig{ConfigDigest: "d", EngineVersion: "e", ModelProfileRef: "m", PromptVersion: "p", RuleSetVersion: "r", SchemaVersion: "s"}, IdempotencyKey: "idem", PayloadHash: "payload"}
	wire, err := SemanticApplyRequestToWire(original)
	if err != nil {
		t.Fatalf("to wire: %v", err)
	}
	result, err := SemanticApplyRequestFromWire(wire)
	if err != nil || result.IdempotencyKey != original.IdempotencyKey || len(result.Chunks) != 1 || result.Document.Deleted {
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
	roundTripEvidence, err := SemanticEvidenceFromWire(SemanticEvidenceToWire(evidence))
	if err != nil || roundTripEvidence != evidence {
		t.Fatalf("evidence round trip = %#v, %v", roundTripEvidence, err)
	}
	scope, err := SemanticAccessScopeFromWire(&semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: 1, KbId: "kb"}, SubjectId: "user", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, ExpiresAt: "2026-09-20T00:00:00Z", Audience: "semantic", Purpose: semanticpb.Purpose_PURPOSE_SEARCH, BudgetRef: "budget"})
	if err != nil || scope.PermissionEpoch != 2 || scope.ExpiresAt != "2026-09-20T00:00:00Z" {
		t.Fatalf("scope = %#v, %v", scope, err)
	}
	roundTrip, err := SemanticAccessScopeFromWire(SemanticAccessScopeToWire(scope))
	if err != nil || roundTrip != scope {
		t.Fatalf("scope round trip = %#v, %v", roundTrip, err)
	}
}
