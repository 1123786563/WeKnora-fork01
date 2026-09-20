package types

import "time"

// SemanticModelWireRequest is the complete Python-to-Go model body. All
// authority and provider selection is carried in the signed capability.
type SemanticModelWireRequest struct {
	CapabilityToken string                  `json:"capability_token"`
	Messages        []SemanticModelMessage  `json:"messages"`
	Parameters      SemanticModelParameters `json:"parameters"`
}
type SemanticModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type SemanticModelParameters struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int64    `json:"max_output_tokens"`
}

// SemanticModelCapability is Go-only authority reconstructed from the signed
// token and a fresh A01 resolution. It is deliberately not a wire type.
type SemanticModelCapability struct {
	OwnerTenantID, RequesterTenantID                               uint64
	KBID, ScopeRef, ScopeHash, SubjectID, Purpose, Audience        string
	PolicyVersion                                                  uint64
	ModelID, Funding, PriceVersion, RunID, CallID                  string
	MaxInputTokensPerCall, MaxOutputTokensPerCall                  int64
	MaxCallsPerTask, MaxInputTokensPerTask, MaxOutputTokensPerTask int64
	PerCallUpperMicro                                              int64
	TaskUpperMicro                                                 *int64
	ExpiresAt, Deadline                                            time.Time
}
type SemanticModelIssuedCapability struct {
	Token           string    `json:"token"`
	MaxInputBytes   int64     `json:"max_input_bytes"`
	MaxOutputTokens int64     `json:"max_output_tokens"`
	ExpiresAt       time.Time `json:"expires_at"`
}
type SemanticModelInvocationResult struct {
	Result                    []byte
	InputTokens, OutputTokens int64
}
