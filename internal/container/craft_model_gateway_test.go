// internal/container/craft_model_gateway_test.go
package container

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The embedded interface lets the same fake satisfy the dig provider's full
// interfaces.ModelService parameter; only GetModelByID is ever exercised by
// these tests (any other call would panic on the nil embed — fail loud).
type fakeModelSource struct {
	interfaces.ModelService
	model *types.Model
	err   error
}

func (f fakeModelSource) GetModelByID(ctx context.Context, id string) (*types.Model, error) {
	return f.model, f.err
}

func withChatModel(id, baseURL, apiKey string) *types.Model {
	m := &types.Model{ID: id}
	m.Parameters.BaseURL = baseURL
	m.Parameters.APIKey = apiKey
	return m
}

func TestCraftUpstreamResolverResolvesManagedUpstream(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "https://api.example.com/", "sk-managed")})
	target, err := resolver(context.Background(), "m1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.BaseURL != "https://api.example.com" || target.Path != "/v1/chat/completions" || target.APIKey != "sk-managed" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestCraftUpstreamResolverFailsClosedWithoutKey(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "https://api.example.com", "")})
	if _, err := resolver(context.Background(), "m1"); err == nil || !strings.Contains(err.Error(), "no configured api key") {
		t.Fatalf("expected fail-closed no-key error, got %v", err)
	}
}

func TestCraftUpstreamResolverFailsClosedWithoutBaseURL(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "", "sk-managed")})
	if _, err := resolver(context.Background(), "m1"); err == nil || !strings.Contains(err.Error(), "no base url") {
		t.Fatalf("expected fail-closed no-base-url error, got %v", err)
	}
}

func TestCraftModelGatewayAssemblyFailsClosedOnShortSecret(t *testing.T) {
	t.Setenv(craftGatewaySecretEnv, "short")
	t.Setenv(craftOpenCodeBaseURLEnv, "http://127.0.0.1:9090")
	// A real sqlite handle lets the inline budget service construct, so the
	// failure surfaced is the gateway's own 16-byte secret check — not the
	// db-missing shortcut a nil handle would take (detection power over the
	// secret threshold, per the round-1 review finding).
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	_, err = newCraftModelGatewayHandler(db, fakeModelSource{}, nil)
	if err == nil || !strings.Contains(err.Error(), "at least 16 bytes") {
		t.Fatalf("expected the 16-byte secret fail-closed error, got %v", err)
	}
}

func TestCraftModelGatewayAssemblyDisabledByDefault(t *testing.T) {
	t.Setenv(craftGatewaySecretEnv, "")
	if gw, err := newCraftModelGatewayHandler(nil, fakeModelSource{}, nil); err != nil || gw != nil {
		t.Fatalf("expected (nil, nil) when no secret is set, got (%v, %v)", gw, err)
	}
}
