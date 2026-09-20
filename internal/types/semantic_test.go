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
