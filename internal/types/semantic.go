package types

import (
	"fmt"
	"time"

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

type SemanticOperationRef struct {
	Scope       SemanticScopeKey
	OperationID string
}

type SemanticOperationState string

const (
	SemanticOperationStatePending   SemanticOperationState = "pending"
	SemanticOperationStateRunning   SemanticOperationState = "running"
	SemanticOperationStateSucceeded SemanticOperationState = "succeeded"
	SemanticOperationStateFailed    SemanticOperationState = "failed"
	SemanticOperationStateCancelled SemanticOperationState = "cancelled"
)

type SemanticOperation struct {
	OperationID      string
	Scope            SemanticScopeKey
	DocumentID       string
	Revision         uint64
	State            SemanticOperationState
	Stage            string
	LeaseToken       uint64
	ResultGeneration *string
	ErrorCode        *string
}

func SemanticOperationFromWire(wire *semanticpb.Operation) (SemanticOperation, error) {
	if wire == nil || wire.Scope == nil || wire.Scope.KbId == "" || wire.OperationId == "" || wire.DocumentId == "" || wire.Revision == 0 || wire.Stage == "" {
		return SemanticOperation{}, fmt.Errorf("semantic operation is incomplete")
	}
	states := map[semanticpb.OperationState]SemanticOperationState{semanticpb.OperationState_OPERATION_STATE_PENDING: SemanticOperationStatePending, semanticpb.OperationState_OPERATION_STATE_RUNNING: SemanticOperationStateRunning, semanticpb.OperationState_OPERATION_STATE_SUCCEEDED: SemanticOperationStateSucceeded, semanticpb.OperationState_OPERATION_STATE_FAILED: SemanticOperationStateFailed, semanticpb.OperationState_OPERATION_STATE_CANCELLED: SemanticOperationStateCancelled}
	state, ok := states[wire.State]
	if !ok {
		return SemanticOperation{}, fmt.Errorf("unsupported semantic operation state")
	}
	result := SemanticOperation{OperationID: wire.OperationId, Scope: SemanticScopeKey{TenantID: wire.Scope.TenantId, KBID: wire.Scope.KbId}, DocumentID: wire.DocumentId, Revision: wire.Revision, State: state, Stage: wire.Stage, LeaseToken: wire.LeaseToken}
	if wire.ResultGeneration != nil {
		result.ResultGeneration = wire.ResultGeneration
	}
	if wire.ErrorCode != nil {
		result.ErrorCode = wire.ErrorCode
	}
	return result, nil
}

func SemanticOperationToWire(value SemanticOperation) (*semanticpb.Operation, error) {
	if value.OperationID == "" || value.Scope.KBID == "" || value.DocumentID == "" || value.Revision == 0 || value.Stage == "" {
		return nil, fmt.Errorf("semantic operation is incomplete")
	}
	states := map[SemanticOperationState]semanticpb.OperationState{SemanticOperationStatePending: semanticpb.OperationState_OPERATION_STATE_PENDING, SemanticOperationStateRunning: semanticpb.OperationState_OPERATION_STATE_RUNNING, SemanticOperationStateSucceeded: semanticpb.OperationState_OPERATION_STATE_SUCCEEDED, SemanticOperationStateFailed: semanticpb.OperationState_OPERATION_STATE_FAILED, SemanticOperationStateCancelled: semanticpb.OperationState_OPERATION_STATE_CANCELLED}
	state, ok := states[value.State]
	if !ok {
		return nil, fmt.Errorf("unsupported semantic operation state")
	}
	wire := &semanticpb.Operation{OperationId: value.OperationID, Scope: &semanticpb.ScopeKey{TenantId: value.Scope.TenantID, KbId: value.Scope.KBID}, DocumentId: value.DocumentID, Revision: value.Revision, State: state, Stage: value.Stage, LeaseToken: value.LeaseToken}
	if value.ResultGeneration != nil {
		wire.ResultGeneration = value.ResultGeneration
	}
	if value.ErrorCode != nil {
		wire.ErrorCode = value.ErrorCode
	}
	return wire, nil
}

func SemanticOperationRefFromWire(wire *semanticpb.OperationRef) (SemanticOperationRef, error) {
	if wire == nil || wire.Scope == nil || wire.Scope.KbId == "" || wire.OperationId == "" {
		return SemanticOperationRef{}, fmt.Errorf("semantic operation reference is incomplete")
	}
	return SemanticOperationRef{Scope: SemanticScopeKey{TenantID: wire.Scope.TenantId, KBID: wire.Scope.KbId}, OperationID: wire.OperationId}, nil
}

func SemanticOperationRefToWire(value SemanticOperationRef) *semanticpb.OperationRef {
	return &semanticpb.OperationRef{Scope: &semanticpb.ScopeKey{TenantId: value.Scope.TenantID, KbId: value.Scope.KBID}, OperationId: value.OperationID}
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
	if _, err := time.Parse(time.RFC3339, wire.ExpiresAt); err != nil {
		return SemanticAccessScope{}, fmt.Errorf("semantic access scope expiry is invalid")
	}
	purposes := map[semanticpb.Purpose]string{semanticpb.Purpose_PURPOSE_SEARCH: "PURPOSE_SEARCH", semanticpb.Purpose_PURPOSE_REASON: "PURPOSE_REASON", semanticpb.Purpose_PURPOSE_INDEX: "PURPOSE_INDEX"}
	purpose, ok := purposes[wire.Purpose]
	if !ok {
		return SemanticAccessScope{}, fmt.Errorf("semantic access scope purpose is unsupported")
	}
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

type SemanticReasonStatus string

const (
	SemanticReasonStatusDerived              SemanticReasonStatus = "derived"
	SemanticReasonStatusInsufficientEvidence SemanticReasonStatus = "insufficient_evidence"
	SemanticReasonStatusConflict             SemanticReasonStatus = "conflict"
	SemanticReasonStatusUnavailable          SemanticReasonStatus = "unavailable"
)

type SemanticReasoningMode string

const (
	SemanticReasoningModeRules SemanticReasoningMode = "rules"
	SemanticReasoningModeModel SemanticReasoningMode = "model"
)

type SemanticReasonRequest struct {
	Search         SemanticSearchRequest
	ReasoningMode  SemanticReasoningMode
	RuleSetVersion string
}

func SemanticReasonRequestFromWire(wire *semanticpb.ReasonRequest) (SemanticReasonRequest, error) {
	if wire == nil || wire.RuleSetVersion == "" {
		return SemanticReasonRequest{}, fmt.Errorf("semantic reason request is incomplete")
	}
	search, err := SemanticSearchRequestFromWire(wire.Search)
	if err != nil {
		return SemanticReasonRequest{}, err
	}
	modes := map[semanticpb.ReasoningMode]SemanticReasoningMode{semanticpb.ReasoningMode_REASONING_MODE_RULES: SemanticReasoningModeRules, semanticpb.ReasoningMode_REASONING_MODE_MODEL: SemanticReasoningModeModel}
	mode, ok := modes[wire.ReasoningMode]
	if !ok {
		return SemanticReasonRequest{}, fmt.Errorf("unsupported reasoning mode")
	}
	return SemanticReasonRequest{Search: search, ReasoningMode: mode, RuleSetVersion: wire.RuleSetVersion}, nil
}

func SemanticReasonRequestToWire(value SemanticReasonRequest) (*semanticpb.ReasonRequest, error) {
	modes := map[SemanticReasoningMode]semanticpb.ReasoningMode{SemanticReasoningModeRules: semanticpb.ReasoningMode_REASONING_MODE_RULES, SemanticReasoningModeModel: semanticpb.ReasoningMode_REASONING_MODE_MODEL}
	mode, ok := modes[value.ReasoningMode]
	if !ok || value.RuleSetVersion == "" {
		return nil, fmt.Errorf("semantic reason request is invalid")
	}
	search, err := SemanticSearchRequestToWire(value.Search)
	if err != nil {
		return nil, err
	}
	return &semanticpb.ReasonRequest{Search: search, ReasoningMode: mode, RuleSetVersion: value.RuleSetVersion}, nil
}

type SemanticReasonResponse struct {
	Status         SemanticReasonStatus
	Conclusion     *string
	Retrieval      *SemanticSearchResponse
	ConclusionKind *string
	PremiseIDs     []string
	RuleIDs        []string
	ModelVersion   *string
	PromptVersion  *string
	Limitations    []string
}

type SemanticChunkSnapshot struct{ ChunkID, Text, ContentHash string }
type SemanticIndexConfig struct{ ConfigDigest, EngineVersion, ModelProfileRef, PromptVersion, RuleSetVersion, SchemaVersion string }
type SemanticApplyRequest struct {
	Document                    SemanticDocumentRevision
	Chunks                      []SemanticChunkSnapshot
	Config                      SemanticIndexConfig
	IdempotencyKey, PayloadHash string
	ManifestRef                 *string
}

type SemanticCapabilities struct {
	ProtocolVersion, EngineVersion string
	RetrievalModes                 []SemanticRetrievalMode
	ReasoningModes                 []SemanticReasoningMode
	Limits                         SemanticQueryLimits
	Limitations                    []string
	UnavailableReason              *string
}

func SemanticCapabilitiesFromWire(wire *semanticpb.Capabilities) (SemanticCapabilities, error) {
	if wire == nil || wire.ProtocolVersion == "" || wire.EngineVersion == "" || wire.Limits == nil {
		return SemanticCapabilities{}, fmt.Errorf("semantic capabilities are incomplete")
	}
	retrievalMap := map[semanticpb.RetrievalMode]SemanticRetrievalMode{semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG: SemanticRetrievalModeGraphRAG, semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON: SemanticRetrievalModeReason}
	reasoningMap := map[semanticpb.ReasoningMode]SemanticReasoningMode{semanticpb.ReasoningMode_REASONING_MODE_RULES: SemanticReasoningModeRules, semanticpb.ReasoningMode_REASONING_MODE_MODEL: SemanticReasoningModeModel}
	result := SemanticCapabilities{ProtocolVersion: wire.ProtocolVersion, EngineVersion: wire.EngineVersion, Limits: SemanticQueryLimits{MaxHops: wire.Limits.MaxHops, MaxNodes: wire.Limits.MaxNodes, MaxEdges: wire.Limits.MaxEdges, TopK: wire.Limits.TopK, MaxTokens: wire.Limits.MaxTokens, DeadlineMS: wire.Limits.DeadlineMs}, Limitations: append([]string(nil), wire.Limitations...)}
	for _, mode := range wire.RetrievalModes {
		mapped, ok := retrievalMap[mode]
		if !ok {
			return SemanticCapabilities{}, fmt.Errorf("unsupported capability retrieval mode")
		}
		result.RetrievalModes = append(result.RetrievalModes, mapped)
	}
	for _, mode := range wire.ReasoningModes {
		mapped, ok := reasoningMap[mode]
		if !ok {
			return SemanticCapabilities{}, fmt.Errorf("unsupported capability reasoning mode")
		}
		result.ReasoningModes = append(result.ReasoningModes, mapped)
	}
	if wire.UnavailableReason != nil {
		result.UnavailableReason = wire.UnavailableReason
	}
	return result, nil
}

func SemanticCapabilitiesToWire(value SemanticCapabilities) (*semanticpb.Capabilities, error) {
	if value.ProtocolVersion == "" || value.EngineVersion == "" {
		return nil, fmt.Errorf("semantic capabilities are incomplete")
	}
	retrievalMap := map[SemanticRetrievalMode]semanticpb.RetrievalMode{SemanticRetrievalModeGraphRAG: semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG, SemanticRetrievalModeReason: semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON}
	reasoningMap := map[SemanticReasoningMode]semanticpb.ReasoningMode{SemanticReasoningModeRules: semanticpb.ReasoningMode_REASONING_MODE_RULES, SemanticReasoningModeModel: semanticpb.ReasoningMode_REASONING_MODE_MODEL}
	wire := &semanticpb.Capabilities{ProtocolVersion: value.ProtocolVersion, EngineVersion: value.EngineVersion, Limits: &semanticpb.QueryLimits{MaxHops: value.Limits.MaxHops, MaxNodes: value.Limits.MaxNodes, MaxEdges: value.Limits.MaxEdges, TopK: value.Limits.TopK, MaxTokens: value.Limits.MaxTokens, DeadlineMs: value.Limits.DeadlineMS}, Limitations: append([]string(nil), value.Limitations...)}
	for _, mode := range value.RetrievalModes {
		mapped, ok := retrievalMap[mode]
		if !ok {
			return nil, fmt.Errorf("unsupported capability retrieval mode")
		}
		wire.RetrievalModes = append(wire.RetrievalModes, mapped)
	}
	for _, mode := range value.ReasoningModes {
		mapped, ok := reasoningMap[mode]
		if !ok {
			return nil, fmt.Errorf("unsupported capability reasoning mode")
		}
		wire.ReasoningModes = append(wire.ReasoningModes, mapped)
	}
	if value.UnavailableReason != nil {
		wire.UnavailableReason = value.UnavailableReason
	}
	return wire, nil
}

func SemanticApplyRequestFromWire(wire *semanticpb.ApplyRequest) (SemanticApplyRequest, error) {
	if wire == nil || wire.Document == nil || wire.Config == nil || wire.IdempotencyKey == "" || wire.PayloadHash == "" {
		return SemanticApplyRequest{}, fmt.Errorf("semantic apply request is incomplete")
	}
	document, err := SemanticDocumentRevisionFromWire(wire.Document)
	if err != nil {
		return SemanticApplyRequest{}, err
	}
	if document.Deleted {
		return SemanticApplyRequest{}, fmt.Errorf("semantic apply cannot use deleted revision")
	}
	config := SemanticIndexConfig{ConfigDigest: wire.Config.ConfigDigest, EngineVersion: wire.Config.EngineVersion, ModelProfileRef: wire.Config.ModelProfileRef, PromptVersion: wire.Config.PromptVersion, RuleSetVersion: wire.Config.RuleSetVersion, SchemaVersion: wire.Config.SchemaVersion}
	if config.ConfigDigest == "" || config.EngineVersion == "" || config.ModelProfileRef == "" || config.PromptVersion == "" || config.RuleSetVersion == "" || config.SchemaVersion == "" {
		return SemanticApplyRequest{}, fmt.Errorf("semantic index config is incomplete")
	}
	chunks := make([]SemanticChunkSnapshot, 0, len(wire.Chunks))
	for _, chunk := range wire.Chunks {
		if chunk == nil || chunk.ChunkId == "" || chunk.ContentHash == "" {
			return SemanticApplyRequest{}, fmt.Errorf("semantic chunk is incomplete")
		}
		chunks = append(chunks, SemanticChunkSnapshot{ChunkID: chunk.ChunkId, Text: chunk.Text, ContentHash: chunk.ContentHash})
	}
	result := SemanticApplyRequest{Document: document, Chunks: chunks, Config: config, IdempotencyKey: wire.IdempotencyKey, PayloadHash: wire.PayloadHash}
	if wire.ManifestRef != nil {
		result.ManifestRef = wire.ManifestRef
	}
	return result, nil
}

func SemanticApplyRequestToWire(value SemanticApplyRequest) (*semanticpb.ApplyRequest, error) {
	if value.Document.Deleted || value.IdempotencyKey == "" || value.PayloadHash == "" {
		return nil, fmt.Errorf("semantic apply request is invalid")
	}
	if value.Config.ConfigDigest == "" || value.Config.EngineVersion == "" || value.Config.ModelProfileRef == "" || value.Config.PromptVersion == "" || value.Config.RuleSetVersion == "" || value.Config.SchemaVersion == "" {
		return nil, fmt.Errorf("semantic index config is incomplete")
	}
	chunks := make([]*semanticpb.ChunkSnapshot, 0, len(value.Chunks))
	for _, chunk := range value.Chunks {
		if chunk.ChunkID == "" || chunk.ContentHash == "" {
			return nil, fmt.Errorf("semantic chunk is incomplete")
		}
		chunks = append(chunks, &semanticpb.ChunkSnapshot{ChunkId: chunk.ChunkID, Text: chunk.Text, ContentHash: chunk.ContentHash})
	}
	wire := &semanticpb.ApplyRequest{Document: SemanticDocumentRevisionToWire(value.Document), Chunks: chunks, Config: &semanticpb.IndexConfig{ConfigDigest: value.Config.ConfigDigest, EngineVersion: value.Config.EngineVersion, ModelProfileRef: value.Config.ModelProfileRef, PromptVersion: value.Config.PromptVersion, RuleSetVersion: value.Config.RuleSetVersion, SchemaVersion: value.Config.SchemaVersion}, IdempotencyKey: value.IdempotencyKey, PayloadHash: value.PayloadHash}
	if value.ManifestRef != nil {
		wire.ManifestRef = value.ManifestRef
	}
	return wire, nil
}

type SemanticQueryLimits struct{ MaxHops, MaxNodes, MaxEdges, TopK, MaxTokens, DeadlineMS uint32 }
type SemanticSearchRequest struct {
	QueryID, Query string
	AccessScope    SemanticAccessScope
	Limits         SemanticQueryLimits
	RequestedMode  SemanticRetrievalMode
}

func SemanticSearchRequestFromWire(wire *semanticpb.SearchRequest) (SemanticSearchRequest, error) {
	if wire == nil || wire.QueryId == "" || wire.Query == "" {
		return SemanticSearchRequest{}, fmt.Errorf("semantic search request is incomplete")
	}
	scope, err := SemanticAccessScopeFromWire(wire.AccessScope)
	if err != nil {
		return SemanticSearchRequest{}, err
	}
	modes := map[semanticpb.RetrievalMode]SemanticRetrievalMode{semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG: SemanticRetrievalModeGraphRAG, semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON: SemanticRetrievalModeReason}
	mode, ok := modes[wire.RequestedMode]
	if !ok {
		return SemanticSearchRequest{}, fmt.Errorf("unsupported requested retrieval mode")
	}
	if (mode == SemanticRetrievalModeGraphRAG && scope.Purpose != "PURPOSE_SEARCH") || (mode == SemanticRetrievalModeReason && scope.Purpose != "PURPOSE_REASON") {
		return SemanticSearchRequest{}, fmt.Errorf("semantic search request purpose is incompatible with requested mode")
	}
	if wire.Limits == nil {
		return SemanticSearchRequest{}, fmt.Errorf("semantic query limits are required")
	}
	return SemanticSearchRequest{QueryID: wire.QueryId, Query: wire.Query, AccessScope: scope, Limits: SemanticQueryLimits{MaxHops: wire.Limits.MaxHops, MaxNodes: wire.Limits.MaxNodes, MaxEdges: wire.Limits.MaxEdges, TopK: wire.Limits.TopK, MaxTokens: wire.Limits.MaxTokens, DeadlineMS: wire.Limits.DeadlineMs}, RequestedMode: mode}, nil
}

func SemanticSearchRequestToWire(value SemanticSearchRequest) (*semanticpb.SearchRequest, error) {
	modes := map[SemanticRetrievalMode]semanticpb.RetrievalMode{SemanticRetrievalModeGraphRAG: semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG, SemanticRetrievalModeReason: semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON}
	mode, ok := modes[value.RequestedMode]
	if !ok {
		return nil, fmt.Errorf("unsupported requested retrieval mode")
	}
	if value.QueryID == "" || value.Query == "" {
		return nil, fmt.Errorf("semantic search request is incomplete")
	}
	if (value.RequestedMode == SemanticRetrievalModeGraphRAG && value.AccessScope.Purpose != "PURPOSE_SEARCH") || (value.RequestedMode == SemanticRetrievalModeReason && value.AccessScope.Purpose != "PURPOSE_REASON") {
		return nil, fmt.Errorf("semantic search request purpose is incompatible with requested mode")
	}
	return &semanticpb.SearchRequest{QueryId: value.QueryID, Query: value.Query, AccessScope: SemanticAccessScopeToWire(value.AccessScope), Limits: &semanticpb.QueryLimits{MaxHops: value.Limits.MaxHops, MaxNodes: value.Limits.MaxNodes, MaxEdges: value.Limits.MaxEdges, TopK: value.Limits.TopK, MaxTokens: value.Limits.MaxTokens, DeadlineMs: value.Limits.DeadlineMS}, RequestedMode: mode}, nil
}

func SemanticReasonResponseFromWire(wire *semanticpb.ReasonResponse) (SemanticReasonResponse, error) {
	if wire == nil {
		return SemanticReasonResponse{}, fmt.Errorf("semantic reason response is nil")
	}
	statuses := map[semanticpb.ReasonStatus]SemanticReasonStatus{semanticpb.ReasonStatus_REASON_STATUS_DERIVED: SemanticReasonStatusDerived, semanticpb.ReasonStatus_REASON_STATUS_INSUFFICIENT_EVIDENCE: SemanticReasonStatusInsufficientEvidence, semanticpb.ReasonStatus_REASON_STATUS_CONFLICT: SemanticReasonStatusConflict, semanticpb.ReasonStatus_REASON_STATUS_UNAVAILABLE: SemanticReasonStatusUnavailable}
	status, ok := statuses[wire.Status]
	if !ok {
		return SemanticReasonResponse{}, fmt.Errorf("unsupported semantic reason status")
	}
	result := SemanticReasonResponse{Status: status, PremiseIDs: append([]string(nil), wire.PremiseIds...), RuleIDs: append([]string(nil), wire.RuleIds...), Limitations: append([]string(nil), wire.Limitations...)}
	if wire.Conclusion != nil {
		result.Conclusion = wire.Conclusion
	}
	if wire.ConclusionKind != nil {
		result.ConclusionKind = wire.ConclusionKind
	}
	if wire.ModelVersion != nil {
		result.ModelVersion = wire.ModelVersion
	}
	if wire.PromptVersion != nil {
		result.PromptVersion = wire.PromptVersion
	}
	if wire.Retrieval != nil {
		mapped, err := SemanticSearchResponseFromWire(wire.Retrieval)
		if err != nil {
			return SemanticReasonResponse{}, err
		}
		result.Retrieval = &mapped
	}
	return result, nil
}

func SemanticReasonResponseToWire(value SemanticReasonResponse) (*semanticpb.ReasonResponse, error) {
	statuses := map[SemanticReasonStatus]semanticpb.ReasonStatus{SemanticReasonStatusDerived: semanticpb.ReasonStatus_REASON_STATUS_DERIVED, SemanticReasonStatusInsufficientEvidence: semanticpb.ReasonStatus_REASON_STATUS_INSUFFICIENT_EVIDENCE, SemanticReasonStatusConflict: semanticpb.ReasonStatus_REASON_STATUS_CONFLICT, SemanticReasonStatusUnavailable: semanticpb.ReasonStatus_REASON_STATUS_UNAVAILABLE}
	status, ok := statuses[value.Status]
	if !ok {
		return nil, fmt.Errorf("unsupported semantic reason status")
	}
	wire := &semanticpb.ReasonResponse{Status: status, PremiseIds: append([]string(nil), value.PremiseIDs...), RuleIds: append([]string(nil), value.RuleIDs...), Limitations: append([]string(nil), value.Limitations...)}
	if value.Conclusion != nil {
		wire.Conclusion = value.Conclusion
	}
	if value.ConclusionKind != nil {
		wire.ConclusionKind = value.ConclusionKind
	}
	if value.ModelVersion != nil {
		wire.ModelVersion = value.ModelVersion
	}
	if value.PromptVersion != nil {
		wire.PromptVersion = value.PromptVersion
	}
	if value.Retrieval != nil {
		mapped, err := SemanticSearchResponseToWire(*value.Retrieval)
		if err != nil {
			return nil, err
		}
		wire.Retrieval = mapped
	}
	return wire, nil
}
