package semantic

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

// Wire mapping between the domain DTOs (internal/types) and the generated
// protobuf messages (semantic/proto). Unknown wire enum values degrade to
// the explicit "unspecified" domain value - never to a success value; unknown
// domain enum strings are errors rather than silent re-encoding.

func ScopeKeyToWire(scope types.SemanticScopeKey) *semanticpb.ScopeKey {
	return &semanticpb.ScopeKey{TenantId: scope.TenantID, KbId: scope.KBID}
}

func ScopeKeyFromWire(message *semanticpb.ScopeKey) (types.SemanticScopeKey, error) {
	if message == nil {
		return types.SemanticScopeKey{}, fmt.Errorf("nil scope key")
	}
	return types.SemanticScopeKey{TenantID: message.GetTenantId(), KBID: message.GetKbId()}, nil
}

// scopeFieldFromWire maps an optional scope sub-message; an absent scope maps
// to the zero value and is rejected later by domain validation (C03), keeping
// this layer purely mechanical.
func scopeFieldFromWire(message *semanticpb.ScopeKey) types.SemanticScopeKey {
	if message == nil {
		return types.SemanticScopeKey{}
	}
	return types.SemanticScopeKey{TenantID: message.GetTenantId(), KBID: message.GetKbId()}
}

func DocumentRevisionToWire(revision types.SemanticDocumentRevision) *semanticpb.DocumentRevision {
	return &semanticpb.DocumentRevision{
		Scope:       ScopeKeyToWire(revision.Scope),
		DocumentId:  revision.DocumentID,
		Revision:    revision.Revision,
		ContentHash: revision.ContentHash,
		Deleted:     revision.Deleted,
	}
}

func DocumentRevisionFromWire(message *semanticpb.DocumentRevision) (types.SemanticDocumentRevision, error) {
	if message == nil {
		return types.SemanticDocumentRevision{}, fmt.Errorf("nil document revision")
	}
	return types.SemanticDocumentRevision{
		Scope:       scopeFieldFromWire(message.GetScope()),
		DocumentID:  message.GetDocumentId(),
		Revision:    message.GetRevision(),
		ContentHash: message.GetContentHash(),
		Deleted:     message.GetDeleted(),
	}, nil
}

func OperationToWire(operation types.SemanticOperation) (*semanticpb.Operation, error) {
	state, err := operationStateToWire(operation.State)
	if err != nil {
		return nil, err
	}
	message := &semanticpb.Operation{
		OperationId: operation.OperationID,
		Scope:       ScopeKeyToWire(operation.Scope),
		DocumentId:  operation.DocumentID,
		Revision:    operation.Revision,
		State:       state,
		Stage:       operation.Stage,
		LeaseToken:  operation.LeaseToken,
	}
	if operation.ResultGeneration != nil {
		message.ResultGeneration = operation.ResultGeneration
	}
	if operation.ErrorCode != nil {
		message.ErrorCode = operation.ErrorCode
	}
	return message, nil
}

func OperationFromWire(message *semanticpb.Operation) (types.SemanticOperation, error) {
	if message == nil {
		return types.SemanticOperation{}, fmt.Errorf("nil operation")
	}
	operation := types.SemanticOperation{
		OperationID: message.GetOperationId(),
		Scope:       scopeFieldFromWire(message.GetScope()),
		DocumentID:  message.GetDocumentId(),
		Revision:    message.GetRevision(),
		State:       operationStateFromWire(message.GetState()),
		Stage:       message.GetStage(),
		LeaseToken:  message.GetLeaseToken(),
	}
	if message.ResultGeneration != nil {
		value := message.GetResultGeneration()
		operation.ResultGeneration = &value
	}
	if message.ErrorCode != nil {
		value := message.GetErrorCode()
		operation.ErrorCode = &value
	}
	return operation, nil
}

func EvidenceToWire(evidence types.SemanticEvidence) *semanticpb.Evidence {
	message := &semanticpb.Evidence{
		EvidenceId:  evidence.EvidenceID,
		DocumentId:  evidence.DocumentID,
		Revision:    evidence.Revision,
		ChunkId:     evidence.ChunkID,
		ContentHash: evidence.ContentHash,
		Quote:       evidence.Quote,
	}
	if evidence.StartChar != nil {
		message.StartChar = evidence.StartChar
	}
	if evidence.EndChar != nil {
		message.EndChar = evidence.EndChar
	}
	return message
}

func EvidenceFromWire(message *semanticpb.Evidence) (types.SemanticEvidence, error) {
	if message == nil {
		return types.SemanticEvidence{}, fmt.Errorf("nil evidence")
	}
	evidence := types.SemanticEvidence{
		EvidenceID:  message.GetEvidenceId(),
		DocumentID:  message.GetDocumentId(),
		Revision:    message.GetRevision(),
		ChunkID:     message.GetChunkId(),
		ContentHash: message.GetContentHash(),
		Quote:       message.GetQuote(),
	}
	if message.StartChar != nil {
		value := message.GetStartChar()
		evidence.StartChar = &value
	}
	if message.EndChar != nil {
		value := message.GetEndChar()
		evidence.EndChar = &value
	}
	return evidence, nil
}

func AssertionToWire(assertion types.SemanticAssertion) (*semanticpb.Assertion, error) {
	kind, err := assertionKindToWire(assertion.Kind)
	if err != nil {
		return nil, err
	}
	message := &semanticpb.Assertion{
		AssertionId: assertion.AssertionID,
		Scope:       ScopeKeyToWire(assertion.Scope),
		SubjectId:   assertion.SubjectID,
		Predicate:   assertion.Predicate,
		Kind:        kind,
		EvidenceIds: assertion.EvidenceIDs,
		PremiseIds:  assertion.PremiseIDs,
	}
	if assertion.ObjectID != nil {
		message.ObjectId = assertion.ObjectID
	}
	if assertion.Value != nil {
		message.Value = assertion.Value
	}
	if assertion.ValidFrom != nil {
		message.ValidFrom = assertion.ValidFrom
	}
	if assertion.ValidUntil != nil {
		message.ValidUntil = assertion.ValidUntil
	}
	return message, nil
}

func AssertionFromWire(message *semanticpb.Assertion) (types.SemanticAssertion, error) {
	if message == nil {
		return types.SemanticAssertion{}, fmt.Errorf("nil assertion")
	}
	assertion := types.SemanticAssertion{
		AssertionID: message.GetAssertionId(),
		Scope:       scopeFieldFromWire(message.GetScope()),
		SubjectID:   message.GetSubjectId(),
		Predicate:   message.GetPredicate(),
		Kind:        assertionKindFromWire(message.GetKind()),
		EvidenceIDs: message.GetEvidenceIds(),
		PremiseIDs:  message.GetPremiseIds(),
	}
	if message.ObjectId != nil {
		value := message.GetObjectId()
		assertion.ObjectID = &value
	}
	if message.Value != nil {
		value := message.GetValue()
		assertion.Value = &value
	}
	if message.ValidFrom != nil {
		value := message.GetValidFrom()
		assertion.ValidFrom = &value
	}
	if message.ValidUntil != nil {
		value := message.GetValidUntil()
		assertion.ValidUntil = &value
	}
	return assertion, nil
}

// ---------------------------------------------------------------------------
// Complete C01 surface: chunks, index config, apply, access, limits,
// search/reason and capabilities
// ---------------------------------------------------------------------------

func ChunkToWire(chunk types.SemanticChunkSnapshot) *semanticpb.ChunkSnapshot {
	return &semanticpb.ChunkSnapshot{ChunkId: chunk.ChunkID, Text: chunk.Text, ContentHash: chunk.ContentHash}
}

func ChunkFromWire(message *semanticpb.ChunkSnapshot) (types.SemanticChunkSnapshot, error) {
	if message == nil {
		return types.SemanticChunkSnapshot{}, fmt.Errorf("nil chunk")
	}
	return types.SemanticChunkSnapshot{ChunkID: message.GetChunkId(), Text: message.GetText(), ContentHash: message.GetContentHash()}, nil
}

func IndexConfigToWire(config types.SemanticIndexConfig) *semanticpb.IndexConfig {
	return &semanticpb.IndexConfig{
		ConfigDigest:    config.ConfigDigest,
		EngineVersion:   config.EngineVersion,
		ModelProfileRef: config.ModelProfileRef,
		PromptVersion:   config.PromptVersion,
		RuleSetVersion:  config.RuleSetVersion,
		SchemaVersion:   config.SchemaVersion,
	}
}

func IndexConfigFromWire(message *semanticpb.IndexConfig) (types.SemanticIndexConfig, error) {
	if message == nil {
		return types.SemanticIndexConfig{}, fmt.Errorf("nil index config")
	}
	return types.SemanticIndexConfig{
		ConfigDigest:    message.GetConfigDigest(),
		EngineVersion:   message.GetEngineVersion(),
		ModelProfileRef: message.GetModelProfileRef(),
		PromptVersion:   message.GetPromptVersion(),
		RuleSetVersion:  message.GetRuleSetVersion(),
		SchemaVersion:   message.GetSchemaVersion(),
	}, nil
}

func ApplyRequestToWire(request types.SemanticApplyRequest) (*semanticpb.ApplyRequest, error) {
	document := DocumentRevisionToWire(request.Document)
	message := &semanticpb.ApplyRequest{
		Document:       document,
		Chunks:         make([]*semanticpb.ChunkSnapshot, 0, len(request.Chunks)),
		IdempotencyKey: request.IdempotencyKey,
		PayloadHash:    request.PayloadHash,
	}
	if request.ManifestRef != nil {
		message.ManifestRef = request.ManifestRef
	}
	if request.Config != nil {
		message.Config = IndexConfigToWire(*request.Config)
	}
	for _, chunk := range request.Chunks {
		message.Chunks = append(message.Chunks, ChunkToWire(chunk))
	}
	return message, nil
}

func ApplyRequestFromWire(message *semanticpb.ApplyRequest) (types.SemanticApplyRequest, error) {
	if message == nil {
		return types.SemanticApplyRequest{}, fmt.Errorf("nil apply request")
	}
	document, err := DocumentRevisionFromWire(message.GetDocument())
	if err != nil {
		return types.SemanticApplyRequest{}, err
	}
	request := types.SemanticApplyRequest{
		Document:       document,
		Chunks:         make([]types.SemanticChunkSnapshot, 0, len(message.GetChunks())),
		IdempotencyKey: message.GetIdempotencyKey(),
		PayloadHash:    message.GetPayloadHash(),
	}
	if message.ManifestRef != nil {
		value := message.GetManifestRef()
		request.ManifestRef = &value
	}
	if message.GetConfig() != nil {
		config, err := IndexConfigFromWire(message.GetConfig())
		if err != nil {
			return types.SemanticApplyRequest{}, err
		}
		request.Config = &config
	}
	for _, chunk := range message.GetChunks() {
		parsed, err := ChunkFromWire(chunk)
		if err != nil {
			return types.SemanticApplyRequest{}, err
		}
		request.Chunks = append(request.Chunks, parsed)
	}
	return request, nil
}

func AccessScopeToWire(access types.SemanticAccessScope) (*semanticpb.AccessScope, error) {
	purpose, err := purposeToWire(access.Purpose)
	if err != nil {
		return nil, err
	}
	return &semanticpb.AccessScope{
		Scope:           ScopeKeyToWire(access.Scope),
		SubjectId:       access.SubjectID,
		ScopeRef:        access.ScopeRef,
		ScopeHash:       access.ScopeHash,
		PermissionEpoch: access.PermissionEpoch,
		ExpiresAt:       access.ExpiresAt,
		Audience:        access.Audience,
		Purpose:         purpose,
		BudgetRef:       access.BudgetRef,
	}, nil
}

func AccessScopeFromWire(message *semanticpb.AccessScope) (types.SemanticAccessScope, error) {
	if message == nil {
		return types.SemanticAccessScope{}, fmt.Errorf("nil access scope")
	}
	return types.SemanticAccessScope{
		Scope:           scopeFieldFromWire(message.GetScope()),
		SubjectID:       message.GetSubjectId(),
		ScopeRef:        message.GetScopeRef(),
		ScopeHash:       message.GetScopeHash(),
		PermissionEpoch: message.GetPermissionEpoch(),
		ExpiresAt:       message.GetExpiresAt(),
		Audience:        message.GetAudience(),
		Purpose:         purposeFromWire(message.GetPurpose()),
		BudgetRef:       message.GetBudgetRef(),
	}, nil
}

func QueryLimitsToWire(limits types.SemanticQueryLimits) *semanticpb.QueryLimits {
	return &semanticpb.QueryLimits{
		MaxHops:    limits.MaxHops,
		MaxNodes:   limits.MaxNodes,
		MaxEdges:   limits.MaxEdges,
		TopK:       limits.TopK,
		MaxTokens:  limits.MaxTokens,
		DeadlineMs: limits.DeadlineMS,
	}
}

func QueryLimitsFromWire(message *semanticpb.QueryLimits) (types.SemanticQueryLimits, error) {
	if message == nil {
		return types.SemanticQueryLimits{}, fmt.Errorf("nil query limits")
	}
	return types.SemanticQueryLimits{
		MaxHops:    message.GetMaxHops(),
		MaxNodes:   message.GetMaxNodes(),
		MaxEdges:   message.GetMaxEdges(),
		TopK:       message.GetTopK(),
		MaxTokens:  message.GetMaxTokens(),
		DeadlineMS: message.GetDeadlineMs(),
	}, nil
}

// SearchRequestToWire attaches the transport header; domain DTOs never carry
// it. ReasonRequest nesting rule: the OUTER header is authoritative and the
// nested search.header must stay unset (see semantic.proto).
func SearchRequestToWire(request types.SemanticSearchRequest, header *semanticpb.RequestHeader) (*semanticpb.SearchRequest, error) {
	message := &semanticpb.SearchRequest{
		Header:  header,
		QueryId: request.QueryID,
		Query:   request.Query,
		Mode:    request.Mode,
	}
	if request.AccessScope != nil {
		access, err := AccessScopeToWire(*request.AccessScope)
		if err != nil {
			// The authorization envelope must never be silently dropped.
			return nil, err
		}
		message.AccessScope = access
	}
	if request.Limits != nil {
		message.Limits = QueryLimitsToWire(*request.Limits)
	}
	return message, nil
}

func SearchRequestFromWire(message *semanticpb.SearchRequest) (types.SemanticSearchRequest, error) {
	if message == nil {
		return types.SemanticSearchRequest{}, fmt.Errorf("nil search request")
	}
	request := types.SemanticSearchRequest{
		QueryID: message.GetQueryId(),
		Query:   message.GetQuery(),
		Mode:    message.GetMode(),
	}
	if message.GetAccessScope() != nil {
		access, err := AccessScopeFromWire(message.GetAccessScope())
		if err != nil {
			return types.SemanticSearchRequest{}, err
		}
		request.AccessScope = &access
	}
	if message.GetLimits() != nil {
		limits, err := QueryLimitsFromWire(message.GetLimits())
		if err != nil {
			return types.SemanticSearchRequest{}, err
		}
		request.Limits = &limits
	}
	return request, nil
}

func SearchResponseToWire(response types.SemanticSearchResponse) *semanticpb.SearchResponse {
	message := &semanticpb.SearchResponse{
		QueryId:      response.QueryID,
		Generation:   response.Generation,
		Mode:         response.Mode,
		Evidence:     make([]*semanticpb.Evidence, 0, len(response.Evidence)),
		AssertionIds: response.AssertionIDs,
		Paths:        make([]*semanticpb.SearchResponse_Path, 0, len(response.Paths)),
		Stale:        response.Stale,
		Partial:      response.Partial,
		Truncated:    response.Truncated,
	}
	for _, item := range response.Evidence {
		message.Evidence = append(message.Evidence, EvidenceToWire(item))
	}
	for _, path := range response.Paths {
		message.Paths = append(message.Paths, &semanticpb.SearchResponse_Path{Nodes: path})
	}
	return message
}

func SearchResponseFromWire(message *semanticpb.SearchResponse) (types.SemanticSearchResponse, error) {
	if message == nil {
		return types.SemanticSearchResponse{}, fmt.Errorf("nil search response")
	}
	response := types.SemanticSearchResponse{
		QueryID:      message.GetQueryId(),
		Generation:   message.GetGeneration(),
		Mode:         message.GetMode(),
		Evidence:     make([]types.SemanticEvidence, 0, len(message.GetEvidence())),
		AssertionIDs: message.GetAssertionIds(),
		Paths:        make([][]string, 0, len(message.GetPaths())),
		Stale:        message.GetStale(),
		Partial:      message.GetPartial(),
		Truncated:    message.GetTruncated(),
	}
	for _, item := range message.GetEvidence() {
		parsed, err := EvidenceFromWire(item)
		if err != nil {
			return types.SemanticSearchResponse{}, err
		}
		response.Evidence = append(response.Evidence, parsed)
	}
	for _, path := range message.GetPaths() {
		response.Paths = append(response.Paths, path.GetNodes())
	}
	return response, nil
}

func ReasonRequestToWire(request types.SemanticReasonRequest, header *semanticpb.RequestHeader) (*semanticpb.ReasonRequest, error) {
	mode, err := reasoningModeToWire(request.ReasoningMode)
	if err != nil {
		return nil, err
	}
	search, err := SearchRequestToWire(request.Search, nil)
	if err != nil {
		return nil, err
	}
	return &semanticpb.ReasonRequest{
		Header:         header,
		Search:         search,
		ReasoningMode:  mode,
		RuleSetVersion: request.RuleSetVersion,
	}, nil
}

func ReasonRequestFromWire(message *semanticpb.ReasonRequest) (types.SemanticReasonRequest, error) {
	if message == nil {
		return types.SemanticReasonRequest{}, fmt.Errorf("nil reason request")
	}
	search, err := SearchRequestFromWire(message.GetSearch())
	if err != nil {
		return types.SemanticReasonRequest{}, err
	}
	return types.SemanticReasonRequest{
		Search:         search,
		ReasoningMode:  reasoningModeFromWire(message.GetReasoningMode()),
		RuleSetVersion: message.GetRuleSetVersion(),
	}, nil
}

func ReasonResponseToWire(response types.SemanticReasonResponse) (*semanticpb.ReasonResponse, error) {
	kind, err := conclusionKindToWire(response.ConclusionKind)
	if err != nil {
		return nil, err
	}
	message := &semanticpb.ReasonResponse{
		Retrieval:      SearchResponseToWire(response.Retrieval),
		Status:         response.Status,
		Conclusion:     response.Conclusion,
		ConclusionKind: kind,
		PremiseIds:     response.PremiseIDs,
		RuleIds:        response.RuleIDs,
		Limitations:    response.Limitations,
	}
	if response.ModelVersion != nil {
		message.ModelVersion = response.ModelVersion
	}
	if response.PromptVersion != nil {
		message.PromptVersion = response.PromptVersion
	}
	return message, nil
}

func ReasonResponseFromWire(message *semanticpb.ReasonResponse) (types.SemanticReasonResponse, error) {
	if message == nil {
		return types.SemanticReasonResponse{}, fmt.Errorf("nil reason response")
	}
	retrieval, err := SearchResponseFromWire(message.GetRetrieval())
	if err != nil {
		return types.SemanticReasonResponse{}, err
	}
	response := types.SemanticReasonResponse{
		Retrieval:      retrieval,
		Status:         message.GetStatus(),
		Conclusion:     message.GetConclusion(),
		ConclusionKind: conclusionKindFromWire(message.GetConclusionKind()),
		PremiseIDs:     message.GetPremiseIds(),
		RuleIDs:        message.GetRuleIds(),
		Limitations:    message.GetLimitations(),
	}
	if message.ModelVersion != nil {
		value := message.GetModelVersion()
		response.ModelVersion = &value
	}
	if message.PromptVersion != nil {
		value := message.GetPromptVersion()
		response.PromptVersion = &value
	}
	return response, nil
}

func CapabilitiesToWire(capabilities types.SemanticCapabilities) *semanticpb.GetCapabilitiesResponse {
	message := &semanticpb.GetCapabilitiesResponse{
		ProtocolVersion: capabilities.ProtocolVersion,
		EngineVersion:   capabilities.EngineVersion,
		Capabilities:    make([]*semanticpb.Capability, 0, len(capabilities.Capabilities)),
	}
	for _, item := range capabilities.Capabilities {
		message.Capabilities = append(message.Capabilities, &semanticpb.Capability{
			Mode: item.Mode, Available: item.Available, UnavailableReason: item.UnavailableReason,
		})
	}
	if capabilities.DefaultLimits != nil {
		message.DefaultLimits = QueryLimitsToWire(*capabilities.DefaultLimits)
	}
	return message
}

func CapabilitiesFromWire(message *semanticpb.GetCapabilitiesResponse) (types.SemanticCapabilities, error) {
	if message == nil {
		return types.SemanticCapabilities{}, fmt.Errorf("nil capabilities")
	}
	capabilities := types.SemanticCapabilities{
		ProtocolVersion: message.GetProtocolVersion(),
		EngineVersion:   message.GetEngineVersion(),
		Capabilities:    make([]types.SemanticCapability, 0, len(message.GetCapabilities())),
	}
	for _, item := range message.GetCapabilities() {
		capabilities.Capabilities = append(capabilities.Capabilities, types.SemanticCapability{
			Mode: item.GetMode(), Available: item.GetAvailable(), UnavailableReason: item.GetUnavailableReason(),
		})
	}
	if message.GetDefaultLimits() != nil {
		limits, err := QueryLimitsFromWire(message.GetDefaultLimits())
		if err != nil {
			return types.SemanticCapabilities{}, err
		}
		capabilities.DefaultLimits = &limits
	}
	return capabilities, nil
}

// ---------------------------------------------------------------------------
// Enum tables
// ---------------------------------------------------------------------------

var assertionKindsToWire = map[types.SemanticAssertionKind]semanticpb.Assertion_Kind{
	types.SemanticAssertionKindUnspecified: semanticpb.Assertion_KIND_UNSPECIFIED,
	types.SemanticAssertionKindSource:      semanticpb.Assertion_KIND_SOURCE,
	types.SemanticAssertionKindRule:        semanticpb.Assertion_KIND_RULE,
	types.SemanticAssertionKindModel:       semanticpb.Assertion_KIND_MODEL,
}

var assertionKindsFromWire = map[semanticpb.Assertion_Kind]types.SemanticAssertionKind{
	semanticpb.Assertion_KIND_UNSPECIFIED: types.SemanticAssertionKindUnspecified,
	semanticpb.Assertion_KIND_SOURCE:      types.SemanticAssertionKindSource,
	semanticpb.Assertion_KIND_RULE:        types.SemanticAssertionKindRule,
	semanticpb.Assertion_KIND_MODEL:       types.SemanticAssertionKindModel,
}

var operationStatesToWire = map[types.SemanticOperationState]semanticpb.Operation_State{
	types.SemanticOperationStateUnspecified: semanticpb.Operation_OPERATION_STATE_UNSPECIFIED,
	types.SemanticOperationStateAccepted:    semanticpb.Operation_OPERATION_STATE_ACCEPTED,
	types.SemanticOperationStateRunning:     semanticpb.Operation_OPERATION_STATE_RUNNING,
	types.SemanticOperationStateStaged:      semanticpb.Operation_OPERATION_STATE_STAGED,
	types.SemanticOperationStatePublishing:  semanticpb.Operation_OPERATION_STATE_PUBLISHING,
	types.SemanticOperationStateSucceeded:   semanticpb.Operation_OPERATION_STATE_SUCCEEDED,
	types.SemanticOperationStateFailed:      semanticpb.Operation_OPERATION_STATE_FAILED,
	types.SemanticOperationStateCancelled:   semanticpb.Operation_OPERATION_STATE_CANCELLED,
	types.SemanticOperationStateSuperseded:  semanticpb.Operation_OPERATION_STATE_SUPERSEDED,
}

var operationStatesFromWire = map[semanticpb.Operation_State]types.SemanticOperationState{
	semanticpb.Operation_OPERATION_STATE_UNSPECIFIED: types.SemanticOperationStateUnspecified,
	semanticpb.Operation_OPERATION_STATE_ACCEPTED:    types.SemanticOperationStateAccepted,
	semanticpb.Operation_OPERATION_STATE_RUNNING:     types.SemanticOperationStateRunning,
	semanticpb.Operation_OPERATION_STATE_STAGED:      types.SemanticOperationStateStaged,
	semanticpb.Operation_OPERATION_STATE_PUBLISHING:  types.SemanticOperationStatePublishing,
	semanticpb.Operation_OPERATION_STATE_SUCCEEDED:   types.SemanticOperationStateSucceeded,
	semanticpb.Operation_OPERATION_STATE_FAILED:      types.SemanticOperationStateFailed,
	semanticpb.Operation_OPERATION_STATE_CANCELLED:   types.SemanticOperationStateCancelled,
	semanticpb.Operation_OPERATION_STATE_SUPERSEDED:  types.SemanticOperationStateSuperseded,
}

var purposesToWire = map[types.SemanticAccessPurpose]semanticpb.AccessScope_Purpose{
	types.SemanticPurposeUnspecified: semanticpb.AccessScope_PURPOSE_UNSPECIFIED,
	types.SemanticPurposeSearch:      semanticpb.AccessScope_PURPOSE_SEARCH,
	types.SemanticPurposeReason:      semanticpb.AccessScope_PURPOSE_REASON,
	types.SemanticPurposeIndex:       semanticpb.AccessScope_PURPOSE_INDEX,
}

var purposesFromWire = map[semanticpb.AccessScope_Purpose]types.SemanticAccessPurpose{
	semanticpb.AccessScope_PURPOSE_UNSPECIFIED: types.SemanticPurposeUnspecified,
	semanticpb.AccessScope_PURPOSE_SEARCH:      types.SemanticPurposeSearch,
	semanticpb.AccessScope_PURPOSE_REASON:      types.SemanticPurposeReason,
	semanticpb.AccessScope_PURPOSE_INDEX:       types.SemanticPurposeIndex,
}

var reasoningModesToWire = map[types.SemanticReasoningMode]semanticpb.ReasonRequest_ReasoningMode{
	types.SemanticReasoningModeUnspecified: semanticpb.ReasonRequest_REASONING_MODE_UNSPECIFIED,
	types.SemanticReasoningModeRules:       semanticpb.ReasonRequest_REASONING_MODE_RULES,
	types.SemanticReasoningModeModel:       semanticpb.ReasonRequest_REASONING_MODE_MODEL,
}

var reasoningModesFromWire = map[semanticpb.ReasonRequest_ReasoningMode]types.SemanticReasoningMode{
	semanticpb.ReasonRequest_REASONING_MODE_UNSPECIFIED: types.SemanticReasoningModeUnspecified,
	semanticpb.ReasonRequest_REASONING_MODE_RULES:       types.SemanticReasoningModeRules,
	semanticpb.ReasonRequest_REASONING_MODE_MODEL:       types.SemanticReasoningModeModel,
}

var conclusionKindsToWire = map[types.SemanticConclusionKind]semanticpb.ReasonResponse_ConclusionKind{
	types.SemanticConclusionKindUnspecified: semanticpb.ReasonResponse_CONCLUSION_KIND_UNSPECIFIED,
	types.SemanticConclusionKindRule:        semanticpb.ReasonResponse_CONCLUSION_KIND_RULE,
	types.SemanticConclusionKindModel:       semanticpb.ReasonResponse_CONCLUSION_KIND_MODEL,
}

var conclusionKindsFromWire = map[semanticpb.ReasonResponse_ConclusionKind]types.SemanticConclusionKind{
	semanticpb.ReasonResponse_CONCLUSION_KIND_UNSPECIFIED: types.SemanticConclusionKindUnspecified,
	semanticpb.ReasonResponse_CONCLUSION_KIND_RULE:        types.SemanticConclusionKindRule,
	semanticpb.ReasonResponse_CONCLUSION_KIND_MODEL:       types.SemanticConclusionKindModel,
}

func purposeToWire(purpose types.SemanticAccessPurpose) (semanticpb.AccessScope_Purpose, error) {
	if value, ok := purposesToWire[purpose]; ok {
		return value, nil
	}
	return semanticpb.AccessScope_PURPOSE_UNSPECIFIED, fmt.Errorf("unknown access purpose %q", purpose)
}

func purposeFromWire(purpose semanticpb.AccessScope_Purpose) types.SemanticAccessPurpose {
	if value, ok := purposesFromWire[purpose]; ok {
		return value
	}
	return types.SemanticPurposeUnspecified
}

func reasoningModeToWire(mode types.SemanticReasoningMode) (semanticpb.ReasonRequest_ReasoningMode, error) {
	if value, ok := reasoningModesToWire[mode]; ok {
		return value, nil
	}
	return semanticpb.ReasonRequest_REASONING_MODE_UNSPECIFIED, fmt.Errorf("unknown reasoning mode %q", mode)
}

func reasoningModeFromWire(mode semanticpb.ReasonRequest_ReasoningMode) types.SemanticReasoningMode {
	if value, ok := reasoningModesFromWire[mode]; ok {
		return value
	}
	return types.SemanticReasoningModeUnspecified
}

func conclusionKindToWire(kind types.SemanticConclusionKind) (semanticpb.ReasonResponse_ConclusionKind, error) {
	if value, ok := conclusionKindsToWire[kind]; ok {
		return value, nil
	}
	return semanticpb.ReasonResponse_CONCLUSION_KIND_UNSPECIFIED, fmt.Errorf("unknown conclusion kind %q", kind)
}

func conclusionKindFromWire(kind semanticpb.ReasonResponse_ConclusionKind) types.SemanticConclusionKind {
	if value, ok := conclusionKindsFromWire[kind]; ok {
		return value
	}
	return types.SemanticConclusionKindUnspecified
}

func assertionKindToWire(kind types.SemanticAssertionKind) (semanticpb.Assertion_Kind, error) {
	if value, ok := assertionKindsToWire[kind]; ok {
		return value, nil
	}
	return semanticpb.Assertion_KIND_UNSPECIFIED, fmt.Errorf("unknown assertion kind %q", kind)
}

func assertionKindFromWire(kind semanticpb.Assertion_Kind) types.SemanticAssertionKind {
	if value, ok := assertionKindsFromWire[kind]; ok {
		return value
	}
	return types.SemanticAssertionKindUnspecified
}

func operationStateToWire(state types.SemanticOperationState) (semanticpb.Operation_State, error) {
	if value, ok := operationStatesToWire[state]; ok {
		return value, nil
	}
	return semanticpb.Operation_OPERATION_STATE_UNSPECIFIED, fmt.Errorf("unknown operation state %q", state)
}

func operationStateFromWire(state semanticpb.Operation_State) types.SemanticOperationState {
	if value, ok := operationStatesFromWire[state]; ok {
		return value
	}
	return types.SemanticOperationStateUnspecified
}
