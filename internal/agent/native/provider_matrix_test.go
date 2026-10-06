package native

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// TestProviderMappingCurrentNativeConfigurationsSerializeAtTheSDKBoundary is
// intentionally a deterministic SDK-boundary probe. It records the behavior
// of the currently supported native mappings; it is not a real-provider,
// proxy, concurrency, multimodal-stream, or product-acceptance test.
func TestProviderMappingCurrentNativeConfigurationsSerializeAtTheSDKBoundary(t *testing.T) {
	tests := []struct {
		provider        string
		variant         string
		serializesImage bool
	}{
		{provider: "openai", variant: "openai", serializesImage: true},
		{provider: "generic", variant: "openai", serializesImage: true},
		{provider: "openrouter", variant: "openai", serializesImage: true},
		{provider: "litellm", variant: "openai", serializesImage: true},
		{provider: "requesty", variant: "openai", serializesImage: true},
		{provider: "siliconflow", variant: "openai", serializesImage: true},
		{provider: "jina", variant: "openai", serializesImage: true},
		{provider: "mimo", variant: "openai", serializesImage: true},
		{provider: "gpustack", variant: "openai", serializesImage: true},
		{provider: "modelscope", variant: "openai", serializesImage: true},
		{provider: "qianfan", variant: "openai", serializesImage: true},
		{provider: "qiniu", variant: "openai", serializesImage: true},
		{provider: "longcat", variant: "openai", serializesImage: true},
		{provider: "lkeap", variant: "openai", serializesImage: true},
		{provider: "nvidia", variant: "openai", serializesImage: true},
		{provider: "novita", variant: "openai", serializesImage: true},
		{provider: "azure_openai", variant: "openai", serializesImage: true},
		{provider: "aliyun", variant: "qwen", serializesImage: true},
		{provider: "zhipu", variant: "glm", serializesImage: true},
		{provider: "deepseek", variant: "deepseek", serializesImage: false},
		{provider: "hunyuan", variant: "hunyuan", serializesImage: true},
		{provider: "minimax", variant: "minimax", serializesImage: true},
		{provider: "moonshot", variant: "kimi", serializesImage: true},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			variant, err := nativeOpenAIVariant(tc.provider)
			require.NoError(t, err)
			require.Equal(t, tc.variant, string(variant))

			captured := make(chan capturedNativeProviderRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read probe request: %v", err)
					return
				}
				captured <- capturedNativeProviderRequest{headers: r.Header.Clone(), body: body}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"probe-response","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
			}))
			defer server.Close()

			binding := nativeModelBinding()
			config := nativeRemoteModelConfig(binding)
			config.Provider = tc.provider
			config.BaseURL = server.URL
			config.Headers = map[string]string{"X-Config-Probe": tc.provider}
			config.ExtraFields = map[string]any{"config_probe": tc.provider}
			credentials := &nativeCredentialResolverFake{credential: Credential{Value: "provider-secret", Version: binding.CredentialVersion}}
			resolved, err := NewModelResolver(&nativeModelConfigSourceFake{config: config}, credentials).Resolve(
				context.Background(), nativecontract.Scope{TenantID: 9, ActorUserID: "actor-1"}, binding,
			)
			require.NoError(t, err)

			responses, err := resolved.Model.GenerateContent(context.Background(), nativeProviderProbeRequest())
			require.NoError(t, err)
			go func() {
				for range responses {
				}
			}()

			select {
			case got := <-captured:
				require.Equal(t, "Bearer provider-secret", got.headers.Get("Authorization"))
				require.Equal(t, tc.provider, got.headers.Get("X-Config-Probe"))
				require.Equal(t, "request", got.headers.Get("X-Request-Probe"))
				require.NotContains(t, string(got.body), "provider-secret")
				require.Contains(t, string(got.body), `"model":"gpt-test"`)
				require.Contains(t, string(got.body), `"config_probe":"`+tc.provider+`"`)
				require.Contains(t, string(got.body), `"request_probe":"preserved"`)
				if tc.serializesImage {
					require.Contains(t, string(got.body), "https://images.example.invalid/probe.png")
				} else {
					require.Contains(t, string(got.body), "Omitted non-text attachments for this provider: 1 image.")
				}
				require.Contains(t, string(got.body), `"id":"call-1"`)
				require.Contains(t, string(got.body), `"reasoning_content":"reasoning-probe"`)
			case <-time.After(2 * time.Second):
				t.Fatal("native provider probe did not reach deterministic server")
			}
			require.Equal(t, 1, credentials.calls)
		})
	}
}

// TestProviderMappingHTTPFailureUsesPinnedSDKRetryBudget records the current
// v1.11.0 SDK behavior. These are transport retries below the resolver, not
// authorized application attempts, billing events, or provider acceptance.
func TestProviderMappingHTTPFailureUsesPinnedSDKRetryBudget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "deterministic provider failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	binding := nativeModelBinding()
	config := nativeRemoteModelConfig(binding)
	config.BaseURL = server.URL
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "provider-secret", Version: binding.CredentialVersion}}
	resolved, err := NewModelResolver(&nativeModelConfigSourceFake{config: config}, credentials).Resolve(
		context.Background(), nativecontract.Scope{TenantID: 9}, binding,
	)
	require.NoError(t, err)

	responses, err := resolved.Model.GenerateContent(context.Background(), &model.Request{Messages: []model.Message{model.NewUserMessage("fail")}})
	require.NoError(t, err)
	var terminal *model.Response
	for response := range responses {
		terminal = response
	}
	require.NotNil(t, terminal)
	require.Error(t, terminal.Error)
	require.EqualValues(t, 3, calls.Load())
	require.Equal(t, 1, credentials.calls)
}

type capturedNativeProviderRequest struct {
	headers http.Header
	body    []byte
}

func nativeProviderProbeRequest() *model.Request {
	thinking := true
	image := model.NewUserMessage("inspect the supplied image")
	image.AddImageURL("https://images.example.invalid/probe.png", "low")
	return &model.Request{
		Messages: []model.Message{
			image,
			{
				Role:             model.RoleAssistant,
				Content:          "calling weather",
				ReasoningContent: "reasoning-probe",
				ToolCalls: []model.ToolCall{{
					ID: "call-1", Type: "function",
					Function: model.FunctionDefinitionParam{Name: "weather", Arguments: []byte(`{"city":"Shanghai"}`)},
				}},
			},
			model.NewToolMessage("call-1", "weather", `{"temperature":24}`),
		},
		GenerationConfig: model.GenerationConfig{ThinkingEnabled: &thinking},
		ExtraFields:      map[string]any{"request_probe": "preserved"},
		Headers:          map[string]string{"X-Request-Probe": "request"},
	}
}

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
