package types

// FusedEvidence adds the RRF fusion score and provenance identity key
// (document/revision/chunk) to the shared SemanticEvidence.
type FusedEvidence struct {
	SemanticEvidence
	FusedScore float64 `json:"fused_score"`
	Identity   string  `json:"identity"`
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