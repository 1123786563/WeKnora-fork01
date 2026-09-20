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
