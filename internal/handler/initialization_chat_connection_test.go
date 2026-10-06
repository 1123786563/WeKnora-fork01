package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/api"
	"github.com/Tencent/WeKnora/internal/models/providers"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeModelGateway spins up a gateway that always replies with the given
// status and body, and returns a remote *types.Model pointing at it.
// checkChatModelConnection walks the exact production ConfigFromModel →
// NewChat path against it, so the error shapes under test are the real ones.
func fakeModelGateway(t *testing.T, status int, body string) *types.Model {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &types.Model{
		Name:   "test-model",
		Type:   types.ModelTypeKnowledgeQA,
		Source: types.ModelSourceRemote,
		Parameters: types.ModelParameters{
			BaseURL: server.URL,
			APIKey:  "sk-test",
		},
	}
}

// TestCheckChatModelConnection_BadRequestMeansReachable pins issue #3928: a
// minimal probe with MaxTokens: 1 gets a 400 from reasoning models (LiteLLM
// behind gpt-5.x rejects "Could not finish the message because max_tokens
// ..."), and that 400 must count as "endpoint reachable + auth OK".
func TestCheckChatModelConnection_BadRequestMeansReachable(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")

	const litellmBody = `{"error":{"type":"invalid_request_error","message":"Could not finish the message because max_tokens (1) is smaller than the minimum number of tokens required for the model to produce a response"}}`

	// The Anthropic path fmt.Errorf's the status into a flat error string
	// ("API request failed with status 400: ...") — the wording the legacy
	// "status code: 400" fallback used to miss.
	t.Run("anthropic gateway 400 chat-layer wording", func(t *testing.T) {
		h := &InitializationHandler{}
		model := fakeModelGateway(t, http.StatusBadRequest, litellmBody)
		model.Parameters.Provider = providers.AnthropicID

		ok, msg := h.checkChatModelConnection(context.Background(), model, "", "")
		assert.True(t, ok)
		assert.Equal(t, "连接正常，模型可用", msg)
	})

	// The OpenAI-compatible path surfaces the go-openai SDK error wording
	// ("error, status code: 400, ...") — the legacy fallback must keep
	// working for it.
	t.Run("openai-compatible gateway 400 legacy SDK wording", func(t *testing.T) {
		h := &InitializationHandler{}
		model := fakeModelGateway(t, http.StatusBadRequest, litellmBody)

		ok, msg := h.checkChatModelConnection(context.Background(), model, "", "")
		assert.True(t, ok)
		assert.Equal(t, "连接正常，模型可用", msg)
	})
}

// TestIsEndpointBadRequest pins the detection helper itself: the structured
// *api.HTTPError first, then both string wordings as fallbacks, and — just as
// important — that non-400 statuses and network errors are not misjudged.
func TestIsEndpointBadRequest(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "chat-layer flat wording",
			err:  errors.New("API request failed with status 400: Could not finish the message because max_tokens (1) is smaller"),
			want: true,
		},
		{
			name: "typed api.HTTPError wrapped in a chain",
			err:  fmt.Errorf("create chat completion: %w", &api.HTTPError{StatusCode: http.StatusBadRequest, Body: "bad request"}),
			want: true,
		},
		{
			name: "legacy go-openai wording",
			err:  errors.New("error, status code: 400, status: 400 Bad Request, message: max_tokens too small"),
			want: true,
		},
		{
			name: "legacy wording wrapped by the chat layer",
			err:  fmt.Errorf("create chat completion: %w", errors.New("error, status code: 400, status: 400 Bad Request, message: max_tokens too small")),
			want: true,
		},
		{
			name: "401 with the new wording is not 400",
			err:  errors.New("API request failed with status 401: invalid api key"),
			want: false,
		},
		{
			name: "typed api.HTTPError with 401 is not 400",
			err:  fmt.Errorf("create chat completion: %w", &api.HTTPError{StatusCode: http.StatusUnauthorized, Body: "invalid api key"}),
			want: false,
		},
		{
			name: "network error is not 400",
			err:  errors.New("send request: dial tcp 127.0.0.1:1: connect: connection refused"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isEndpointBadRequest(tc.err))
		})
	}
}

// TestCheckChatModelConnection_FailuresSurfaceHintAndRawError keeps the
// non-400 failure contract: available=false, a human-readable hint, and the
// upstream error verbatim.
func TestCheckChatModelConnection_FailuresSurfaceHintAndRawError(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")

	t.Run("401 surfaces auth hint and raw error", func(t *testing.T) {
		h := &InitializationHandler{}
		model := fakeModelGateway(t, http.StatusUnauthorized,
			`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
		model.Parameters.Provider = providers.AnthropicID

		ok, msg := h.checkChatModelConnection(context.Background(), model, "", "")
		assert.False(t, ok)
		assert.Contains(t, msg, "认证失败")
		assert.Contains(t, msg, "invalid x-api-key")
	})

	t.Run("unreachable endpoint surfaces connection hint", func(t *testing.T) {
		h := &InitializationHandler{}
		model := &types.Model{
			Name:   "test-model",
			Type:   types.ModelTypeKnowledgeQA,
			Source: types.ModelSourceRemote,
			Parameters: types.ModelParameters{
				BaseURL:  "http://127.0.0.1:1",
				APIKey:   "sk-test",
				Provider: providers.AnthropicID,
			},
		}

		ok, msg := h.checkChatModelConnection(context.Background(), model, "", "")
		require.False(t, ok)
		assert.Contains(t, msg, "无法连接到服务器")
	})
}
