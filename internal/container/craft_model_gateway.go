// internal/container/craft_model_gateway.go
package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

// craftGatewaySecretEnv enables the craft controlled model gateway (O02).
// Setting a secret is the enablement intent: a non-empty secret shorter than
// the handler's 16-byte minimum fails the boot (fail-closed, never a silent
// downgrade). Empty keeps the gateway unassembled.
const craftGatewaySecretEnv = "WEKNORA_CRAFT_GATEWAY_SECRET"

// craftGatewayBaseURLEnv optionally names the externally reachable gateway
// base URL stamped into issued credentials (default: relative path).
const craftGatewayBaseURLEnv = "WEKNORA_CRAFT_GATEWAY_BASE_URL"

// craftModelSource is the narrow consumer interface of the upstream
// resolver (consumer-defined interface, Go idiom): interfaces.ModelService
// satisfies it without taking the whole model surface here.
type craftModelSource interface {
	GetModelByID(ctx context.Context, id string) (*types.Model, error)
}

// craftUpstreamChatPath is the OpenAI-compatible path the gateway's Forward
// joins onto the resolved base URL (Forward does TrimSuffix(baseURL,"/")+Path).
const craftUpstreamChatPath = "/v1/chat/completions"

// craftUpstreamResolver resolves one model's managed upstream from the
// server's credential store ONLY (the model row's AES-GCM-encrypted api_key
// is decrypted transparently by ModelParameters.Scan). It fails closed on a
// missing model, a missing key or a missing base URL — the gateway header
// forbids any environment fallback.
func craftUpstreamResolver(models craftModelSource) handler.CraftUpstreamResolver {
	return func(ctx context.Context, model string) (handler.CraftUpstreamTarget, error) {
		m, err := models.GetModelByID(ctx, model)
		if err != nil {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s: %w", model, err)
		}
		if m == nil {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s not found", model)
		}
		if m.Parameters.APIKey == "" {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s has no configured api key", model)
		}
		if m.Parameters.BaseURL == "" {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s has no base url", model)
		}
		return handler.CraftUpstreamTarget{
			BaseURL: strings.TrimSuffix(m.Parameters.BaseURL, "/"),
			Path:    craftUpstreamChatPath,
			APIKey:  m.Parameters.APIKey,
		}, nil
	}
}
