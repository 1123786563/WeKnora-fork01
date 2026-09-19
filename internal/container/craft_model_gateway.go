// internal/container/craft_model_gateway.go
package container

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	commercial "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
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

// defaultCraftBudgetPolicy is the non-configurable default admission policy
// of the controlled gateway. Credit values are micro-credits (the same unit
// as ReservationRow.UpperMicro): CallUpper 1 credit per forwarded call,
// TaskLimit 10 credits per admitted run, GrantWindow 24h, MaxCalls 1000.
func defaultCraftBudgetPolicy() service.CraftBudgetPolicy {
	return service.CraftBudgetPolicy{
		GrantWindow: 24 * time.Hour,
		MaxCalls:    1000,
		CallUpper:   commercial.Credits(1_000_000),
		TaskLimit:   commercial.Credits(10_000_000),
	}
}

// newCraftModelGatewayHandler assembles the O02 controlled model gateway.
// Enablement requires BOTH the OpenCode runtime dial (the gateway's only
// callers live inside the sandbox runtime) and a signing secret; a set-but
// short secret fails the boot (fail-closed). The budget service is built
// inline — it stays a private dependency of the gateway, not a container
// surface other consumers could accidentally depend on.
func newCraftModelGatewayHandler(
	db *gorm.DB,
	models interfaces.ModelService,
	usage *service.CraftUsageService,
) (*handler.CraftModelGateway, error) {
	secret := strings.TrimSpace(os.Getenv(craftGatewaySecretEnv))
	if secret == "" {
		return nil, nil
	}
	if strings.TrimSpace(os.Getenv(craftOpenCodeBaseURLEnv)) == "" {
		logger.Warnf(context.Background(),
			"[CraftModelGateway] %s set but %s is not: the gateway stays unassembled (no runtime caller)",
			craftGatewaySecretEnv, craftOpenCodeBaseURLEnv)
		return nil, nil
	}
	budget, err := service.NewCraftBudgetService(db, nil, defaultCraftBudgetPolicy())
	if err != nil {
		return nil, fmt.Errorf("craft model gateway budget service: %w", err)
	}
	gw, err := handler.NewCraftModelGateway(handler.CraftModelGatewayConfig{
		Secret:         []byte(secret),
		Budget:         budget,
		Recorder:       usage,
		Upstream:       craftUpstreamResolver(models),
		GatewayBaseURL: strings.TrimSpace(os.Getenv(craftGatewayBaseURLEnv)),
	})
	if err != nil {
		return nil, fmt.Errorf("craft model gateway: %w", err)
	}
	return gw, nil
}

// registerCraftModelGatewayHTTPHandlers installs the gateway for routing.
func registerCraftModelGatewayHTTPHandlers(gw *handler.CraftModelGateway) {
	if gw == nil {
		return
	}
	handler.RegisterCraftModelGateway(gw)
}
