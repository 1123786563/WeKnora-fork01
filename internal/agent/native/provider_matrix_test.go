package native

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestProviderMappingOpenAICompatiblePreservesHeadersAndRequestFields(t *testing.T) {
	var request map[string]any
	var authorization, trace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization, trace = r.Header.Get("Authorization"), r.Header.Get("X-Trace-ID")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"response-1","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	binding := nativeModelBinding()
	cfg := nativeRemoteModelConfig(binding)
	cfg.BaseURL = server.URL
	cfg.Headers = map[string]string{"X-Trace-ID": "trace-1"}
	cfg.ExtraFields = map[string]any{"seed": 42, "tool_choice": "required", "parallel_tool_calls": false}
	source := &nativeModelConfigSourceFake{config: cfg}
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "provider-secret", Version: binding.CredentialVersion}}
	resolver := NewModelResolver(source, credentials)
	resolved, err := resolver.Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
	require.NoError(t, err)
	// The admitted provider projection is immutable for this model binding.
	// Later source-map mutation cannot rewrite a resumed request.
	cfg.Headers["X-Trace-ID"] = "changed"
	cfg.ExtraFields["seed"] = 99

	seed := 42
	requestModel := &model.Request{
		Messages:         []model.Message{model.NewUserMessage("hello")},
		GenerationConfig: model.GenerationConfig{MaxTokens: &seed},
		ExtraFields:      map[string]any{"request_flag": "kept"},
	}
	responses, err := resolved.Model.GenerateContent(context.Background(), requestModel)
	require.NoError(t, err)
	var terminal *model.Response
	for response := range responses {
		terminal = response
	}
	require.NotNil(t, terminal)
	require.Nil(t, terminal.Error)

	require.Equal(t, "Bearer provider-secret", authorization)
	require.Equal(t, "trace-1", trace)
	require.Equal(t, float64(42), request["seed"])
	require.Equal(t, "required", request["tool_choice"])
	require.Equal(t, false, request["parallel_tool_calls"])
	require.Equal(t, "kept", request["request_flag"])
	require.NotContains(t, string(mustJSON(t, request)), "provider-secret")
}

func TestProviderMappingRejectsUnsupportedProviderWithoutLegacyFallback(t *testing.T) {
	binding := nativeModelBinding()
	cfg := nativeRemoteModelConfig(binding)
	cfg.Provider = "anthropic"
	source := &nativeModelConfigSourceFake{config: cfg}
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "secret", Version: binding.CredentialVersion}}

	_, err := NewModelResolver(source, credentials).Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
	require.ErrorContains(t, err, "unsupported native provider")
	require.Zero(t, credentials.calls)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return b
}
