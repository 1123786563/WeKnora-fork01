package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
)

// OpenAICompatibleProvider is the approved model entry adapter: one
// configurable base URL + key + model name, deadline and cancellation
// propagated, raw usage returned. It never logs credentials.
type OpenAICompatibleProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewModelProviderAdapter returns nil unless the approved entry is fully
// configured (fail closed: no half-configured model calls).
func NewModelProviderAdapter(cfg *config.SemanticConfig) types.SemanticModelProvider {
	if cfg == nil || cfg.ModelProvider != "openai-compatible" || cfg.ModelBaseURL == "" || cfg.ModelAPIKey == "" || cfg.ModelName == "" {
		return nil
	}
	return &OpenAICompatibleProvider{
		baseURL: cfg.ModelBaseURL,
		apiKey:  cfg.ModelAPIKey,
		model:   cfg.ModelName,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type chatRequest struct {
	Model     string                       `json:"model"`
	Messages  []types.SemanticModelMessage `json:"messages"`
	MaxTokens int                          `json:"max_tokens,omitempty"`
	Stream    bool                         `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	ID string `json:"id"`
}

func (p *OpenAICompatibleProvider) Invoke(ctx context.Context, req types.SemanticModelRequest) (types.SemanticModelResult, error) {
	payload, err := json.Marshal(chatRequest{
		Model: p.model, Messages: req.Messages, MaxTokens: req.MaxOutputTokens, Stream: false,
	})
	if err != nil {
		return types.SemanticModelResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return types.SemanticModelResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey) // never logged
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return types.SemanticModelResult{}, fmt.Errorf("provider transport: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return types.SemanticModelResult{}, fmt.Errorf("provider rejected with status %d", resp.StatusCode)
	}
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return types.SemanticModelResult{}, fmt.Errorf("provider response decode: %w", err)
	}
	text := ""
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}
	return types.SemanticModelResult{
		Text:              text,
		InputTokens:       parsed.Usage.PromptTokens,
		OutputTokens:      parsed.Usage.CompletionTokens,
		ProviderRequestID: parsed.ID,
		Status:            "completed",
	}, nil
}
