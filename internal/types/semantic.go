package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Semantic domain DTOs (C01). They mirror semantic/proto/semantic.proto
// field-by-field but are plain Go types: protobuf objects never leak past
// the mapping layer in internal/infrastructure/semantic.
//
// JSON public boundary: every 64-bit integer is a decimal string (json tag
// ",string" or custom marshalling), so TS/JS consumers never round uint64
// through float64. Optional scalars use pointers so "absent" stays distinct
// from zero values.

type SemanticAssertionKind string

const (
	SemanticAssertionKindUnspecified SemanticAssertionKind = "unspecified"
	SemanticAssertionKindSource      SemanticAssertionKind = "source"
	SemanticAssertionKindRule        SemanticAssertionKind = "rule"
	SemanticAssertionKindModel       SemanticAssertionKind = "model"
)

type SemanticAccessPurpose string

const (
	SemanticPurposeUnspecified SemanticAccessPurpose = "unspecified"
	SemanticPurposeSearch      SemanticAccessPurpose = "search"
	SemanticPurposeReason      SemanticAccessPurpose = "reason"
	SemanticPurposeIndex       SemanticAccessPurpose = "index"
)

type SemanticReasoningMode string

const (
	SemanticReasoningModeUnspecified SemanticReasoningMode = "unspecified"
	SemanticReasoningModeRules       SemanticReasoningMode = "rules"
	SemanticReasoningModeModel       SemanticReasoningMode = "model"
)

type SemanticConclusionKind string

const (
	SemanticConclusionKindUnspecified SemanticConclusionKind = "unspecified"
	SemanticConclusionKindRule        SemanticConclusionKind = "rule"
	SemanticConclusionKindModel       SemanticConclusionKind = "model"
)

type SemanticOperationState string

const (
	SemanticOperationStateUnspecified SemanticOperationState = "unspecified"
	SemanticOperationStateAccepted    SemanticOperationState = "accepted"
	SemanticOperationStateRunning     SemanticOperationState = "running"
	SemanticOperationStateStaged      SemanticOperationState = "staged"
	SemanticOperationStatePublishing  SemanticOperationState = "publishing"
	SemanticOperationStateSucceeded   SemanticOperationState = "succeeded"
	SemanticOperationStateFailed      SemanticOperationState = "failed"
	SemanticOperationStateCancelled   SemanticOperationState = "cancelled"
	SemanticOperationStateSuperseded  SemanticOperationState = "superseded"
)

type SemanticScopeKey struct {
	TenantID uint64 `json:"tenant_id,string"`
	KBID     string `json:"kb_id"`
}

type SemanticDocumentRevision struct {
	Scope       SemanticScopeKey `json:"scope"`
	DocumentID  string           `json:"document_id"`
	Revision    uint64           `json:"revision,string"`
	ContentHash string           `json:"content_hash"`
	Deleted     bool             `json:"deleted"`
}

type SemanticChunkSnapshot struct {
	ChunkID     string `json:"chunk_id"`
	Text        string `json:"text"`
	ContentHash string `json:"content_hash"`
}

type SemanticIndexConfig struct {
	ConfigDigest    string `json:"config_digest"`
	EngineVersion   string `json:"engine_version"`
	ModelProfileRef string `json:"model_profile_ref"`
	PromptVersion   string `json:"prompt_version"`
	RuleSetVersion  string `json:"rule_set_version"`
	SchemaVersion   string `json:"schema_version"`
}

type SemanticApplyRequest struct {
	Document       SemanticDocumentRevision `json:"document"`
	Chunks         []SemanticChunkSnapshot  `json:"chunks"`
	ManifestRef    *string                  `json:"manifest_ref"`
	Config         *SemanticIndexConfig     `json:"config"`
	IdempotencyKey string                   `json:"idempotency_key"`
	PayloadHash    string                   `json:"payload_hash"`
}

type SemanticOperation struct {
	OperationID      string                 `json:"operation_id"`
	Scope            SemanticScopeKey       `json:"scope"`
	DocumentID       string                 `json:"document_id"`
	Revision         uint64                 `json:"revision,string"`
	State            SemanticOperationState `json:"state"`
	Stage            string                 `json:"stage"`
	LeaseToken       uint64                 `json:"lease_token,string"`
	ResultGeneration *string                `json:"result_generation"`
	ErrorCode        *string                `json:"error_code"`
}

// SemanticEvidence carries an optional Unicode codepoint span; an absent
// span stays nil (never silently zero). Custom JSON keeps the shared
// boundary: revision as decimal string, spans as decimal strings or null.
type SemanticEvidence struct {
	EvidenceID  string
	DocumentID  string
	Revision    uint64
	ChunkID     string
	ContentHash string
	Quote       string
	StartChar   *uint32
	EndChar     *uint32
}

type semanticEvidenceJSON struct {
	EvidenceID  string  `json:"evidence_id"`
	DocumentID  string  `json:"document_id"`
	Revision    string  `json:"revision"`
	ChunkID     string  `json:"chunk_id"`
	ContentHash string  `json:"content_hash"`
	Quote       string  `json:"quote"`
	StartChar   *string `json:"start_char"`
	EndChar     *string `json:"end_char"`
}

func (e SemanticEvidence) MarshalJSON() ([]byte, error) {
	shadow := semanticEvidenceJSON{
		EvidenceID:  e.EvidenceID,
		DocumentID:  e.DocumentID,
		Revision:    strconv.FormatUint(e.Revision, 10),
		ChunkID:     e.ChunkID,
		ContentHash: e.ContentHash,
		Quote:       e.Quote,
	}
	if e.StartChar != nil {
		encoded := strconv.FormatUint(uint64(*e.StartChar), 10)
		shadow.StartChar = &encoded
	}
	if e.EndChar != nil {
		encoded := strconv.FormatUint(uint64(*e.EndChar), 10)
		shadow.EndChar = &encoded
	}
	return json.Marshal(shadow)
}

func (e *SemanticEvidence) UnmarshalJSON(data []byte) error {
	var shadow semanticEvidenceJSON
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	revision, err := strconv.ParseUint(shadow.Revision, 10, 64)
	if err != nil {
		return fmt.Errorf("evidence revision: %w", err)
	}
	e.EvidenceID = shadow.EvidenceID
	e.DocumentID = shadow.DocumentID
	e.Revision = revision
	e.ChunkID = shadow.ChunkID
	e.ContentHash = shadow.ContentHash
	e.Quote = shadow.Quote
	e.StartChar, err = parseOptionalUint32String(shadow.StartChar)
	if err != nil {
		return err
	}
	e.EndChar, err = parseOptionalUint32String(shadow.EndChar)
	return err
}

func parseOptionalUint32String(value *string) (*uint32, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := strconv.ParseUint(*value, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("span value %q: %w", *value, err)
	}
	casted := uint32(parsed)
	return &casted, nil
}

type SemanticAssertion struct {
	AssertionID string                `json:"assertion_id"`
	Scope       SemanticScopeKey      `json:"scope"`
	SubjectID   string                `json:"subject_id"`
	Predicate   string                `json:"predicate"`
	ObjectID    *string               `json:"object_id"`
	Value       *string               `json:"value"`
	Kind        SemanticAssertionKind `json:"kind"`
	EvidenceIDs []string              `json:"evidence_ids"`
	PremiseIDs  []string              `json:"premise_ids"`
	ValidFrom   *string               `json:"valid_from"`
	ValidUntil  *string               `json:"valid_until"`
}

type SemanticAccessScope struct {
	Scope           SemanticScopeKey      `json:"scope"`
	SubjectID       string                `json:"subject_id"`
	ScopeRef        string                `json:"scope_ref"`
	ScopeHash       string                `json:"scope_hash"`
	PermissionEpoch uint64                `json:"permission_epoch,string"`
	ExpiresAt       string                `json:"expires_at"`
	Audience        string                `json:"audience"`
	Purpose         SemanticAccessPurpose `json:"purpose"`
	BudgetRef       string                `json:"budget_ref"`
}

type SemanticQueryLimits struct {
	MaxHops    uint32 `json:"max_hops"`
	MaxNodes   uint32 `json:"max_nodes"`
	MaxEdges   uint32 `json:"max_edges"`
	TopK       uint32 `json:"top_k"`
	MaxTokens  uint32 `json:"max_tokens"`
	DeadlineMS uint32 `json:"deadline_ms"`
}

type SemanticSearchRequest struct {
	QueryID     string               `json:"query_id"`
	Query       string               `json:"query"`
	AccessScope *SemanticAccessScope `json:"access_scope"`
	Limits      *SemanticQueryLimits `json:"limits"`
	Mode        string               `json:"mode"`
}

type SemanticSearchResponse struct {
	QueryID      string             `json:"query_id"`
	Generation   string             `json:"generation"`
	Mode         string             `json:"mode"`
	Evidence     []SemanticEvidence `json:"evidence"`
	AssertionIDs []string           `json:"assertion_ids"`
	Paths        [][]string         `json:"paths"`
	Stale        bool               `json:"stale"`
	Partial      bool               `json:"partial"`
	Truncated    bool               `json:"truncated"`
}

type SemanticReasonRequest struct {
	Search         SemanticSearchRequest `json:"search"`
	ReasoningMode  SemanticReasoningMode `json:"reasoning_mode"`
	RuleSetVersion string                `json:"rule_set_version"`
}

type SemanticReasonResponse struct {
	Retrieval      SemanticSearchResponse `json:"retrieval"`
	Status         string                 `json:"status"`
	Conclusion     string                 `json:"conclusion"`
	ConclusionKind SemanticConclusionKind `json:"conclusion_kind"`
	PremiseIDs     []string               `json:"premise_ids"`
	RuleIDs        []string               `json:"rule_ids"`
	ModelVersion   *string                `json:"model_version"`
	PromptVersion  *string                `json:"prompt_version"`
	Limitations    []string               `json:"limitations"`
}

type SemanticCapability struct {
	Mode              string `json:"mode"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason"`
}

type SemanticCapabilities struct {
	ProtocolVersion string               `json:"protocol_version"`
	EngineVersion   string               `json:"engine_version"`
	Capabilities    []SemanticCapability `json:"capabilities"`
	DefaultLimits   *SemanticQueryLimits `json:"default_limits"`
}
