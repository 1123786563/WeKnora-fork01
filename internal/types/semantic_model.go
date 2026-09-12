package types

import "context"

// SemanticModelMessage is one chat message sent through the controlled
// model gateway (never a raw provider payload).
type SemanticModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SemanticModelRequest is the provider-facing call the gateway adapter
// issues after ledger claim and budget admission.
type SemanticModelRequest struct {
	InvocationID    string                  `json:"invocation_id"`
	OperationID     string                  `json:"operation_id"`
	ModelProfileRef string                  `json:"model_profile_ref"`
	Messages        []SemanticModelMessage  `json:"messages"`
	MaxOutputTokens int                     `json:"max_output_tokens"`
}

// SemanticModelResult carries the provider outcome plus RAW usage - no
// pricing interpretation (settlement rules belong to the billing track).
type SemanticModelResult struct {
	InvocationID      string `json:"invocation_id"`
	Text              string `json:"text"`
	InputTokens       int    `json:"input_tokens"`
	OutputTokens      int    `json:"output_tokens"`
	ProviderRequestID string `json:"provider_request_id"`
	Status            string `json:"status"`
}

// SemanticModelProvider is the adapter seam: every model call from the
// semantic pipeline goes through it (no direct provider URLs, no
// long-term keys, deadline and cancellation propagated).
type SemanticModelProvider interface {
	Invoke(ctx context.Context, request SemanticModelRequest) (SemanticModelResult, error)
}