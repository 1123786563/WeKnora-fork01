package trpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

type (
	existingModel   struct{ chat chat.Chat }
	modelAttemptKey struct{}
	attemptContext  struct {
		id     string
		record func(ModelAttempt) error
	}
)

// NewModel adapts an already configured Chat, retaining its provider credentials,
// concurrency controls, tracing, and provider-specific request handling.
func NewModel(c chat.Chat) model.Model { return &existingModel{chat: c} }

// WithModelAttempt binds stream events and the completed usage/response record
// to one attempt. The graph node records this result in State before committing
// its checkpoint; recorder failures prevent publication of a successful plan.
func WithModelAttempt(ctx context.Context, id string, record func(ModelAttempt) error) context.Context {
	return context.WithValue(ctx, modelAttemptKey{}, attemptContext{id: id, record: record})
}

func (m *existingModel) Info() model.Info { return model.Info{Name: m.chat.GetModelName()} }

func (m *existingModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil || m.chat == nil {
		return nil, fmt.Errorf("model request and chat are required")
	}
	messages, opts, err := convertModelRequest(req)
	if err != nil {
		return nil, err
	}
	attempt, _ := ctx.Value(modelAttemptKey{}).(attemptContext)
	if attempt.id == "" {
		attempt.id = uuid.NewString()
	}
	if !req.Stream {
		response, err := m.chat.Chat(ctx, messages, opts)
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if response == nil {
			return nil, fmt.Errorf("provider returned no response")
		}
		completed, err := m.complete(attempt, response)
		if err != nil {
			return nil, err
		}
		out := make(chan *model.Response, 1)
		out <- completed
		close(out)
		return out, nil
	}
	stream, err := m.chat.ChatStream(ctx, messages, opts)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, fmt.Errorf("provider returned no stream")
	}
	out := make(chan *model.Response, 1)
	go m.consumeStream(ctx, attempt, stream, out)
	return out, nil
}

func convertModelRequest(req *model.Request) ([]chat.Message, *chat.ChatOptions, error) {
	// Chat cannot represent these SDK-only options. Reject rather than silently
	// dropping a caller's generation constraint or credential/header override.
	if len(req.Stop) > 0 || req.ReasoningEffort != nil || req.ThinkingTokens != nil ||
		req.ThinkingLevel != nil || len(req.Headers) > 0 {
		return nil, nil, fmt.Errorf("request uses options unsupported by existing Chat")
	}
	opts := &chat.ChatOptions{Thinking: req.ThinkingEnabled}
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		opts.TopP = *req.TopP
	}
	if req.MaxTokens != nil {
		opts.MaxTokens = *req.MaxTokens
	}
	if req.FrequencyPenalty != nil {
		opts.FrequencyPenalty = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		opts.PresencePenalty = *req.PresencePenalty
	}
	if len(req.ExtraFields) > 0 {
		raw, err := json.Marshal(req.ExtraFields)
		if err != nil {
			return nil, nil, err
		}
		var extra struct {
			Seed                int             `json:"seed"`
			ToolChoice          string          `json:"tool_choice"`
			ParallelToolCalls   *bool           `json:"parallel_tool_calls"`
			MaxCompletionTokens int             `json:"max_completion_tokens"`
			Format              json.RawMessage `json:"format"`
		}
		if err := decodeStrict(raw, &extra); err != nil {
			return nil, nil, fmt.Errorf("model extra fields: %w", err)
		}
		opts.Seed, opts.ToolChoice, opts.ParallelToolCalls = extra.Seed, extra.ToolChoice, extra.ParallelToolCalls
		opts.MaxCompletionTokens, opts.Format = extra.MaxCompletionTokens, extra.Format
	}
	if req.StructuredOutput != nil {
		raw, err := json.Marshal(req.StructuredOutput)
		if err != nil {
			return nil, nil, err
		}
		opts.Format = raw
	}
	names := make([]string, 0, len(req.Tools))
	for name := range req.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t := req.Tools[name]
		if t == nil || t.Declaration() == nil {
			return nil, nil, fmt.Errorf("missing tool declaration for %s", name)
		}
		d := t.Declaration()
		if d.Name != name || d.InputSchema == nil {
			return nil, nil, fmt.Errorf("invalid tool declaration for %s", name)
		}
		schema, err := json.Marshal(d.InputSchema)
		if err != nil {
			return nil, nil, err
		}
		opts.Tools = append(opts.Tools, chat.Tool{Type: "function", Function: chat.FunctionDef{
			Name: d.Name, Description: d.Description, Parameters: schema,
		}})
	}
	messages := make([]chat.Message, 0, len(req.Messages))
	for _, in := range req.Messages {
		out, err := convertModelMessage(in)
		if err != nil {
			return nil, nil, err
		}
		messages = append(messages, out)
	}
	return messages, opts, nil
}

func convertModelMessage(in model.Message) (chat.Message, error) {
	out := chat.Message{
		Role: string(in.Role), Content: in.Content, Name: in.ToolName, ToolCallID: in.ToolID,
		ReasoningContent: in.ReasoningContent,
	}
	if !in.Role.IsValid() || in.ReasoningSignature != "" {
		return out, fmt.Errorf("unsupported message role or reasoning signature")
	}
	for _, call := range in.ToolCalls {
		metadata := make(types.ToolCallMetadata)
		for key, value := range call.ExtraFields {
			raw, err := json.Marshal(value)
			if err != nil {
				return out, err
			}
			metadata[key] = raw
		}
		out.ToolCalls = append(out.ToolCalls, chat.ToolCall{
			ID: call.ID, Type: call.Type,
			Function:         chat.FunctionCall{Name: call.Function.Name, Arguments: string(call.Function.Arguments)},
			ProviderMetadata: metadata,
		})
	}
	if len(in.ContentParts) > 0 && in.Content != "" {
		out.MultiContent = append(out.MultiContent, chat.MessageContentPart{Type: "text", Text: in.Content})
	}
	for _, part := range in.ContentParts {
		switch part.Type {
		case model.ContentTypeText:
			if part.Text == nil {
				return out, fmt.Errorf("missing text content")
			}
			out.MultiContent = append(out.MultiContent, chat.MessageContentPart{Type: "text", Text: *part.Text})
		case model.ContentTypeImage:
			if part.Image == nil {
				return out, fmt.Errorf("missing image content")
			}
			url := part.Image.URL
			if len(part.Image.Data) > 0 {
				format := strings.TrimPrefix(part.Image.Format, "image/")
				if format == "" {
					return out, fmt.Errorf("missing inline image format")
				}
				url = "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(part.Image.Data)
			}
			if url == "" {
				return out, fmt.Errorf("missing image URL or data")
			}
			out.MultiContent = append(out.MultiContent, chat.MessageContentPart{
				Type:     "image_url",
				ImageURL: &chat.ImageURL{URL: url, Detail: part.Image.Detail},
			})
		default:
			return out, fmt.Errorf("unsupported Chat content part %q", part.Type)
		}
	}
	return out, nil
}

func (m *existingModel) complete(attempt attemptContext, in *types.ChatResponse) (*model.Response, error) {
	if in.FinishReason == types.FinishReasonIncomplete {
		return nil, fmt.Errorf("incomplete model response")
	}
	message := model.Message{Role: model.RoleAssistant, Content: in.Content, ReasoningContent: in.ReasoningContent}
	seen := make(map[string]bool)
	for _, call := range in.ToolCalls {
		if call.ID == "" || seen[call.ID] || call.Function.Name == "" || !json.Valid([]byte(call.Function.Arguments)) {
			return nil, fmt.Errorf("incomplete or invalid tool plan")
		}
		seen[call.ID] = true
		extra := make(map[string]any)
		for key, raw := range call.ProviderMetadata {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, err
			}
			extra[key] = value
		}
		message.ToolCalls = append(message.ToolCalls, model.ToolCall{
			ID: call.ID, Type: call.Type,
			Function: model.FunctionDefinitionParam{
				Name: call.Function.Name, Arguments: []byte(call.Function.Arguments),
			},
			ExtraFields: extra,
		})
	}
	u := in.Usage
	response := &model.Response{
		ID: attempt.id, Model: m.chat.GetModelName(), Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{Message: message, FinishReason: &in.FinishReason}}, Usage: &model.Usage{
			PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, TotalTokens: u.TotalTokens,
			PromptTokensDetails: model.PromptTokensDetails{
				CachedTokens:    u.CachedTokens,
				CacheReadTokens: u.CacheReadTokens, CacheCreationTokens: u.CacheWriteTokens,
			},
		},
	}
	if attempt.record != nil {
		if err := attempt.record(ModelAttempt{Version: 1, Response: response, Usage: u}); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (m *existingModel) consumeStream(ctx context.Context, attempt attemptContext,
	stream <-chan types.StreamResponse, out chan<- *model.Response,
) {
	defer close(out)
	result := &types.ChatResponse{}
	completed := false
	send := func(response *model.Response) bool {
		select {
		case out <- response:
			return true
		case <-ctx.Done():
			return false
		}
	}
	fail := func(err error) {
		typ := model.ErrorTypeStreamError
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			typ = model.ErrorTypeCancelled
		}
		response := &model.Response{
			ID: attempt.id, Object: model.ObjectTypeError, Done: true,
			Error: &model.ResponseError{Type: typ, Message: err.Error()},
		}
		// A cancellation never publishes a successful response or tool plan.
		if ctx.Err() == nil {
			send(response)
			return
		}
		select {
		case out <- response:
		default:
		}
	}
	for {
		select {
		case <-ctx.Done():
			fail(ctx.Err())
			return
		case chunk, ok := <-stream:
			if !ok {
				if err := ctx.Err(); err != nil {
					fail(err)
					return
				}
				if !completed {
					fail(fmt.Errorf("provider stream closed before completion"))
					return
				}
				response, err := m.complete(attempt, result)
				if err != nil {
					fail(err)
					return
				}
				send(response)
				return
			}
			if chunk.ResponseType == types.ResponseTypeError {
				fail(fmt.Errorf("provider stream: %s", chunk.Content))
				return
			}
			if chunk.FinishReason == types.FinishReasonIncomplete {
				fail(fmt.Errorf("incomplete provider stream"))
				return
			}
			if chunk.FinishReason != "" {
				result.FinishReason = chunk.FinishReason
				completed = true
			}
			if chunk.Usage != nil {
				result.Usage = *chunk.Usage
			}
			if len(chunk.ToolCalls) > 0 {
				result.ToolCalls = mergeToolCalls(result.ToolCalls, chunk.ToolCalls, chunk.Done)
			}
			if chunk.Content == "" || (chunk.Data != nil && chunk.Data["source"] != nil) {
				continue
			}
			delta := model.Message{Role: model.RoleAssistant}
			if chunk.ResponseType == types.ResponseTypeThinking {
				result.ReasoningContent += chunk.Content
				delta.ReasoningContent = chunk.Content
			} else {
				result.Content += chunk.Content
				delta.Content = chunk.Content
			}
			if !send(&model.Response{
				ID: attempt.id, Model: m.chat.GetModelName(), Object: model.ObjectTypeChatCompletionChunk,
				IsPartial: true, Choices: []model.Choice{{Delta: delta}},
			}) {
				return
			}
		}
	}
}

func mergeToolCalls(current, incoming []types.LLMToolCall, final bool) []types.LLMToolCall {
	for i, call := range incoming {
		index := -1
		for j := range current {
			if call.ID != "" && current[j].ID == call.ID {
				index = j
				break
			}
		}
		if index < 0 && call.ID == "" && i < len(current) {
			index = i
		}
		if index < 0 {
			current = append(current, call)
			continue
		}
		old := &current[index]
		args := call.Function.Arguments
		// Existing providers publish complete snapshots in their terminal chunk.
		// Other Chat implementations can supply fragments with stable call IDs.
		if final || json.Valid([]byte(args)) || strings.HasPrefix(args, old.Function.Arguments) {
			if args != "" {
				old.Function.Arguments = args
			}
		} else {
			old.Function.Arguments += args
		}
		if call.ID != "" {
			old.ID = call.ID
		}
		if call.Type != "" {
			old.Type = call.Type
		}
		if call.Function.Name != "" {
			old.Function.Name = call.Function.Name
		}
		if len(call.ProviderMetadata) > 0 {
			if old.ProviderMetadata == nil {
				old.ProviderMetadata = make(types.ToolCallMetadata)
			}
			for key, value := range call.ProviderMetadata {
				old.ProviderMetadata[key] = value
			}
		}
	}
	return current
}
