package types

// FusedEvidence adds the RRF fusion score and provenance identity key
// (document/revision/chunk) to the shared SemanticEvidence.
type FusedEvidence struct {
	SemanticEvidence
	FusedScore float64 `json:"fused_score"`
	Identity   string  `json:"identity"`
}

// SemanticDocumentStatusWire is the per-document semantic state surfaced by
// the user API (W01). Revision stays a decimal string end-to-end (uint64
// precision across every client boundary).
type SemanticDocumentStatusWire struct {
	DocumentID       string `json:"document_id"`
	Revision         string `json:"revision"`
	SemanticStatus   string `json:"semantic_status"`
	ActiveGeneration string `json:"active_generation,omitempty"`
}

// SemanticSearchResponseWire is the wire search payload (mode is the ACTUAL
// execution mode - degraded runs say so, never fake reasoning).
type SemanticSearchResponseWire struct {
	Mode       string                 `json:"mode"`
	Generation string                 `json:"generation,omitempty"`
	Truncated  bool                   `json:"truncated"`
	Evidence   []SemanticEvidenceWire `json:"evidence"`
}

// SemanticEvidenceWire is the client-facing evidence item.
type SemanticEvidenceWire struct {
	EvidenceID  string `json:"evidence_id"`
	DocumentID  string `json:"document_id"`
	Revision    string `json:"revision"`
	ChunkID     string `json:"chunk_id"`
	ContentHash string `json:"content_hash,omitempty"`
	Quote       string `json:"quote,omitempty"`
}

// SemanticReasonResponseWire is the wire reason payload.
type SemanticReasonResponseWire struct {
	Mode        string   `json:"mode"`
	Status      string   `json:"status"`
	Conclusion  string   `json:"conclusion"`
	PremiseIDs  []string `json:"premise_ids"`
	RuleIDs     []string `json:"rule_ids,omitempty"`
	Limitations []string `json:"limitations,omitempty"`
}

// SemanticSearchResult is the unified query facade result: the ACTUAL
// mode is always reported (never fake a degraded run as reasoning).
type SemanticSearchResult struct {
	Mode     string          `json:"mode"`
	Evidence []FusedEvidence `json:"evidence"`
}

// SemanticReasonResult carries the fused retrieval plus the conclusion
// AFTER delivery validation passed.
type SemanticReasonResult struct {
	Mode       string          `json:"mode"`
	Evidence   []FusedEvidence `json:"evidence"`
	Conclusion string          `json:"conclusion,omitempty"`
	Status     string          `json:"status"`
}
