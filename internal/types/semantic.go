package types

import (
	"fmt"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

type SemanticScopeKey struct {
	TenantID uint64
	KBID     string
}

type SemanticDocumentRevision struct {
	Scope       SemanticScopeKey
	DocumentID  string
	Revision    uint64
	ContentHash string
	Deleted     bool
}

func SemanticDocumentRevisionFromWire(wire *semanticpb.DocumentRevision) (SemanticDocumentRevision, error) {
	if wire == nil || wire.Scope == nil || wire.Scope.KbId == "" || wire.DocumentId == "" || wire.Revision == 0 {
		return SemanticDocumentRevision{}, fmt.Errorf("semantic document revision is incomplete")
	}
	return SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: wire.Scope.TenantId, KBID: wire.Scope.KbId}, DocumentID: wire.DocumentId, Revision: wire.Revision, ContentHash: wire.ContentHash, Deleted: wire.Deleted}, nil
}

func SemanticDeleteDocumentRevisionFromWire(wire *semanticpb.DocumentRevision) (SemanticDocumentRevision, error) {
	revision, err := SemanticDocumentRevisionFromWire(wire)
	if err != nil {
		return SemanticDocumentRevision{}, err
	}
	if !revision.Deleted {
		return SemanticDocumentRevision{}, fmt.Errorf("semantic delete requires deleted=true")
	}
	return revision, nil
}

func SemanticDocumentRevisionToWire(value SemanticDocumentRevision) *semanticpb.DocumentRevision {
	return &semanticpb.DocumentRevision{
		Scope:       &semanticpb.ScopeKey{TenantId: value.Scope.TenantID, KbId: value.Scope.KBID},
		DocumentId:  value.DocumentID,
		Revision:    value.Revision,
		ContentHash: value.ContentHash,
		Deleted:     value.Deleted,
	}
}

type SemanticRetrievalMode string

const (
	SemanticRetrievalModeGraphRAG SemanticRetrievalMode = "graph_rag"
	SemanticRetrievalModeReason   SemanticRetrievalMode = "reason"
)

type SemanticSearchResponse struct {
	QueryID       string
	Generation    string
	RequestedMode SemanticRetrievalMode
	ActualMode    SemanticRetrievalMode
	Evidence      []SemanticEvidence
	AssertionIDs  []string
	Paths         [][]string
	Stale         bool
	Partial       bool
	Truncated     bool
}

type SemanticEvidence struct {
	EvidenceID, DocumentID, ChunkID, ContentHash, Quote string
	Revision                                            uint64
	StartChar, EndChar                                  *uint32
}

func SemanticEvidenceFromWire(wire *semanticpb.Evidence) (SemanticEvidence, error) {
	if wire == nil || wire.EvidenceId == "" || wire.DocumentId == "" || wire.ChunkId == "" || wire.ContentHash == "" || wire.Quote == "" || wire.Revision == 0 {
		return SemanticEvidence{}, fmt.Errorf("semantic evidence is incomplete")
	}
	if (wire.StartChar == nil) != (wire.EndChar == nil) || (wire.StartChar != nil && *wire.EndChar <= *wire.StartChar) {
		return SemanticEvidence{}, fmt.Errorf("semantic evidence span is invalid")
	}
	return SemanticEvidence{EvidenceID: wire.EvidenceId, DocumentID: wire.DocumentId, Revision: wire.Revision, ChunkID: wire.ChunkId, ContentHash: wire.ContentHash, Quote: wire.Quote, StartChar: wire.StartChar, EndChar: wire.EndChar}, nil
}

func SemanticEvidenceToWire(value SemanticEvidence) *semanticpb.Evidence {
	wire := &semanticpb.Evidence{EvidenceId: value.EvidenceID, DocumentId: value.DocumentID, Revision: value.Revision, ChunkId: value.ChunkID, ContentHash: value.ContentHash, Quote: value.Quote}
	if value.StartChar != nil {
		wire.StartChar = value.StartChar
		wire.EndChar = value.EndChar
	}
	return wire
}

type SemanticAccessScope struct {
	Scope                                                          SemanticScopeKey
	SubjectID, ScopeRef, ScopeHash, ExpiresAt, Audience, BudgetRef string
	PermissionEpoch                                                uint64
	Purpose                                                        string
}

func SemanticAccessScopeFromWire(wire *semanticpb.AccessScope) (SemanticAccessScope, error) {
	if wire == nil || wire.Scope == nil || wire.Scope.KbId == "" || wire.SubjectId == "" || wire.ScopeRef == "" || wire.ScopeHash == "" || wire.ExpiresAt == "" || wire.Audience == "" || wire.BudgetRef == "" || wire.Purpose == semanticpb.Purpose_PURPOSE_UNSPECIFIED {
		return SemanticAccessScope{}, fmt.Errorf("semantic access scope is incomplete")
	}
	purpose := wire.Purpose.String()
	return SemanticAccessScope{Scope: SemanticScopeKey{TenantID: wire.Scope.TenantId, KBID: wire.Scope.KbId}, SubjectID: wire.SubjectId, ScopeRef: wire.ScopeRef, ScopeHash: wire.ScopeHash, ExpiresAt: wire.ExpiresAt, PermissionEpoch: wire.PermissionEpoch, Audience: wire.Audience, BudgetRef: wire.BudgetRef, Purpose: purpose}, nil
}

func SemanticAccessScopeToWire(value SemanticAccessScope) *semanticpb.AccessScope {
	purposes := map[string]semanticpb.Purpose{
		"PURPOSE_SEARCH": semanticpb.Purpose_PURPOSE_SEARCH,
		"PURPOSE_REASON": semanticpb.Purpose_PURPOSE_REASON,
		"PURPOSE_INDEX":  semanticpb.Purpose_PURPOSE_INDEX,
	}
	purpose, ok := purposes[value.Purpose]
	if !ok {
		panic("unsupported semantic access scope purpose")
	}
	return &semanticpb.AccessScope{Scope: &semanticpb.ScopeKey{TenantId: value.Scope.TenantID, KbId: value.Scope.KBID}, SubjectId: value.SubjectID, ScopeRef: value.ScopeRef, ScopeHash: value.ScopeHash, PermissionEpoch: value.PermissionEpoch, ExpiresAt: value.ExpiresAt, Audience: value.Audience, Purpose: purpose, BudgetRef: value.BudgetRef}
}

func semanticRetrievalModeFromWire(mode semanticpb.RetrievalMode) (SemanticRetrievalMode, error) {
	switch mode {
	case semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG:
		return SemanticRetrievalModeGraphRAG, nil
	case semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON:
		return SemanticRetrievalModeReason, nil
	default:
		return "", fmt.Errorf("unsupported semantic retrieval mode: %s", mode.String())
	}
}

func SemanticSearchResponseFromWire(wire *semanticpb.SearchResponse) (SemanticSearchResponse, error) {
	if wire == nil {
		return SemanticSearchResponse{}, fmt.Errorf("semantic search response is nil")
	}
	requested, err := semanticRetrievalModeFromWire(wire.RequestedMode)
	if err != nil {
		return SemanticSearchResponse{}, err
	}
	actual, err := semanticRetrievalModeFromWire(wire.ActualMode)
	if err != nil {
		return SemanticSearchResponse{}, err
	}
	if requested == SemanticRetrievalModeGraphRAG && actual == SemanticRetrievalModeReason {
		return SemanticSearchResponse{}, fmt.Errorf("semantic search cannot implicitly upgrade graph_rag to reason")
	}
	evidence := make([]SemanticEvidence, 0, len(wire.Evidence))
	for _, item := range wire.Evidence {
		mapped, err := SemanticEvidenceFromWire(item)
		if err != nil {
			return SemanticSearchResponse{}, err
		}
		evidence = append(evidence, mapped)
	}
	paths := make([][]string, 0, len(wire.Paths))
	for _, path := range wire.Paths {
		paths = append(paths, append([]string(nil), path.Ids...))
	}
	return SemanticSearchResponse{QueryID: wire.QueryId, Generation: wire.Generation, RequestedMode: requested, ActualMode: actual, Evidence: evidence, AssertionIDs: append([]string(nil), wire.AssertionIds...), Paths: paths, Stale: wire.Stale, Partial: wire.Partial, Truncated: wire.Truncated}, nil
}

func SemanticSearchResponseToWire(value SemanticSearchResponse) (*semanticpb.SearchResponse, error) {
	modes := map[SemanticRetrievalMode]semanticpb.RetrievalMode{SemanticRetrievalModeGraphRAG: semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG, SemanticRetrievalModeReason: semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON}
	requested, ok := modes[value.RequestedMode]
	if !ok {
		return nil, fmt.Errorf("unsupported requested retrieval mode")
	}
	actual, ok := modes[value.ActualMode]
	if !ok {
		return nil, fmt.Errorf("unsupported actual retrieval mode")
	}
	if value.RequestedMode == SemanticRetrievalModeGraphRAG && value.ActualMode == SemanticRetrievalModeReason {
		return nil, fmt.Errorf("semantic search cannot implicitly upgrade graph_rag to reason")
	}
	evidence := make([]*semanticpb.Evidence, 0, len(value.Evidence))
	for _, item := range value.Evidence {
		evidence = append(evidence, SemanticEvidenceToWire(item))
	}
	paths := make([]*semanticpb.StringPath, 0, len(value.Paths))
	for _, path := range value.Paths {
		paths = append(paths, &semanticpb.StringPath{Ids: append([]string(nil), path...)})
	}
	return &semanticpb.SearchResponse{QueryId: value.QueryID, Generation: value.Generation, RequestedMode: requested, ActualMode: actual, Evidence: evidence, AssertionIds: append([]string(nil), value.AssertionIDs...), Paths: paths, Stale: value.Stale, Partial: value.Partial, Truncated: value.Truncated}, nil
}
