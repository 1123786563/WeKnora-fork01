// internal/container/craft_model_gateway_test.go
package container

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type fakeModelSource struct {
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
