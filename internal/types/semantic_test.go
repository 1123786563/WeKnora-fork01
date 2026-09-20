package types

import (
	"testing"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

func TestSemanticDeleteRequiresDeletedRevision(t *testing.T) {
	_, err := SemanticDocumentRevisionFromWire(&semanticpb.DocumentRevision{Deleted: false})
	if err == nil {
		t.Fatal("expected delete revision validation error")
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
	scope, err := SemanticAccessScopeFromWire(&semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: 1, KbId: "kb"}, SubjectId: "user", ScopeRef: "ref", ScopeHash: "hash", PermissionEpoch: 2, Audience: "semantic", Purpose: semanticpb.Purpose_PURPOSE_SEARCH, BudgetRef: "budget"})
	if err != nil || scope.PermissionEpoch != 2 {
		t.Fatalf("scope = %#v, %v", scope, err)
	}
}
