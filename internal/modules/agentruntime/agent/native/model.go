package native

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/types"
	"trpc.group/trpc-go/trpc-agent-go/model"
	modelopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// Credential is the short-lived result of a server-owned credential lookup.
// It is deliberately separate from ConfigBinding: bindings persist only the
// reference and version, never secret material.
type Credential struct {
	Value   string
	Version int64
}

// CredentialResolver authorizes and resolves a credential reference for one
// native model request. Implementations must not return a credential outside
// the supplied scope or version.
type CredentialResolver interface {
	ResolveCredential(context.Context, nativecontract.Scope, string, int64) (Credential, error)
}

// NativeModelConfig is the non-secret provider projection selected by the
// authoritative configuration source. Binding is the immutable identity that
// must match the admitted ConfigBinding exactly.
type NativeModelConfig struct {
	Binding             nativecontract.ConfigBinding
	Funding             nativecontract.FundingBinding
	Source              types.ModelSource
	Provider, ModelName string
	BaseURL             string
	Headers             map[string]string
	ExtraFields         map[string]any
}

// ModelConfigSource obtains the current server-owned provider projection.
// It must enforce tenant visibility before returning any endpoint or header.
type ModelConfigSource interface {
	ResolveNativeModel(context.Context, nativecontract.Scope, nativecontract.ConfigBinding) (NativeModelConfig, error)
}

// ModelResolver creates only native SDK provider models. It intentionally has
// no legacy chat.Chat or scripted-model fallback.
type ModelResolver struct {
	configs     ModelConfigSource
	credentials CredentialResolver
}

var _ nativecontract.ModelResolver = (*ModelResolver)(nil)

func NewModelResolver(configs ModelConfigSource, credentials CredentialResolver) *ModelResolver {
	return &ModelResolver{configs: configs, credentials: credentials}
}

func (r *ModelResolver) Resolve(
	ctx context.Context, scope nativecontract.Scope, binding nativecontract.ConfigBinding,
) (nativecontract.ModelBinding, error) {
	if err := ctx.Err(); err != nil {
		return nativecontract.ModelBinding{}, err
	}
	if r == nil || r.configs == nil || r.credentials == nil {
		return nativecontract.ModelBinding{}, fmt.Errorf("native model resolver dependencies are required")
	}
	if err := validateModelBinding(scope, binding); err != nil {
		return nativecontract.ModelBinding{}, err
	}
	configured, err := r.configs.ResolveNativeModel(ctx, scope, binding)
	if err != nil {
		return nativecontract.ModelBinding{}, fmt.Errorf("resolve native model configuration: %w", err)
	}
	if configured.Binding != binding {
		return nativecontract.ModelBinding{}, fmt.Errorf("native model configuration drift")
	}
	if err := validateNativeModelConfig(configured); err != nil {
		return nativecontract.ModelBinding{}, err
	}
	headers, err := cloneHeaders(configured.Headers)
	if err != nil {
		return nativecontract.ModelBinding{}, err
	}
	extra, err := cloneExtraFields(configured.ExtraFields)
	if err != nil {
		return nativecontract.ModelBinding{}, err
	}
	variant, err := nativeOpenAIVariant(configured.Provider)
	if err != nil {
		return nativecontract.ModelBinding{}, err
	}
	return nativecontract.ModelBinding{
		Model: &nativeProviderModel{
			scope: scope, binding: binding, credentials: r.credentials,
			provider: configured.Provider, modelName: configured.ModelName, baseURL: configured.BaseURL,
			headers: headers, extraFields: extra, variant: variant,
		},
		Config: binding, Funding: configured.Funding,
	}, nil
}

type nativeProviderModel struct {
	scope       nativecontract.Scope
	binding     nativecontract.ConfigBinding
	credentials CredentialResolver
	provider    string
	modelName   string
	baseURL     string
	headers     map[string]string
	extraFields map[string]any
	variant     modelopenai.Variant
}

var _ model.Model = (*nativeProviderModel)(nil)

func (m *nativeProviderModel) Info() model.Info { return model.Info{Name: m.modelName} }

func (m *nativeProviderModel) GenerateContent(ctx context.Context, request *model.Request) (<-chan *model.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, fmt.Errorf("native model request is required")
	}
	credential, err := m.credentials.ResolveCredential(ctx, m.scope, m.binding.CredentialRef, m.binding.CredentialVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve native model credential: %w", err)
	}
	if credential.Version != m.binding.CredentialVersion || strings.TrimSpace(credential.Value) == "" {
		return nil, fmt.Errorf("native model credential version drift")
	}
	provider := modelopenai.New(m.modelName,
		modelopenai.WithAPIKey(credential.Value),
		modelopenai.WithBaseURL(m.baseURL),
		modelopenai.WithHeaders(m.headers),
		modelopenai.WithExtraFields(m.extraFields),
		modelopenai.WithVariant(m.variant),
	)
	return provider.GenerateContent(ctx, request)
}

func validateModelBinding(scope nativecontract.Scope, binding nativecontract.ConfigBinding) error {
	if scope.TenantID == 0 {
		return fmt.Errorf("native model scope tenant is required")
	}
	if binding.SchemaVersion <= 0 || strings.TrimSpace(binding.SDKVersion) == "" ||
		strings.TrimSpace(binding.GraphVersion) == "" || strings.TrimSpace(binding.ConfigHash) == "" ||
		strings.TrimSpace(binding.ModelID) == "" || strings.TrimSpace(binding.ModelConfigVersion) == "" ||
		strings.TrimSpace(binding.CredentialRef) == "" || binding.CredentialVersion <= 0 {
		return fmt.Errorf("native model configuration identity is incomplete")
	}
	return nil
}

func validateNativeModelConfig(config NativeModelConfig) error {
	if config.Source != types.ModelSourceRemote || strings.TrimSpace(config.ModelName) == "" || strings.TrimSpace(config.BaseURL) == "" {
		return fmt.Errorf("native model provider configuration is incomplete")
	}
	return nil
}

func nativeOpenAIVariant(provider string) (modelopenai.Variant, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "generic", "openrouter", "litellm", "requesty", "siliconflow", "jina", "mimo", "gpustack", "modelscope", "qianfan", "qiniu", "longcat", "lkeap", "nvidia", "novita", "azure_openai":
		return modelopenai.VariantOpenAI, nil
	case "aliyun":
		return modelopenai.VariantQwen, nil
	case "zhipu":
		return modelopenai.VariantGLM, nil
	case "deepseek":
		return modelopenai.VariantDeepSeek, nil
	case "hunyuan":
		return modelopenai.VariantHunyuan, nil
	case "minimax":
		return modelopenai.VariantMiniMax, nil
	case "moonshot":
		return modelopenai.VariantKimi, nil
	default:
		return "", fmt.Errorf("unsupported native provider %q", provider)
	}
}

func cloneHeaders(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("native model header name is required")
		}
		out[key] = value
	}
	return out, nil
}

func cloneExtraFields(in map[string]any) (map[string]any, error) {
	if len(in) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("native model provider fields: %w", err)
	}
	out := make(map[string]any, len(in))
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("copy native model provider fields: %w", err)
	}
	return out, nil
}
