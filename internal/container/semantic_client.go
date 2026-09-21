package container

import (
	"fmt"
	"os"
	"strings"

	"github.com/Tencent/WeKnora/internal/config"
	semanticinfra "github.com/Tencent/WeKnora/internal/infrastructure/semantic"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// initSemanticClient creates an opt-in semantic client. A constructed lazy gRPC
// connection is not a readiness signal; callers must use RPC results for that.
func initSemanticClient(cfg *config.Config, cleaner interfaces.ResourceCleaner) (interfaces.SemanticClient, error) {
	if cfg.Semantic == nil || !cfg.Semantic.Enabled {
		return nil, nil
	}
	rootCA, err := os.ReadFile(strings.TrimSpace(cfg.Semantic.RootCAPath))
	if err != nil {
		return nil, fmt.Errorf("read semantic root CA: %w", err)
	}
	client, err := semanticinfra.NewClient(semanticinfra.SemanticClientConfig{
		Address: cfg.Semantic.Address, ServerName: cfg.Semantic.ServerName, RootCA: rootCA,
		ServiceToken: cfg.Semantic.ServiceToken, Audience: cfg.Semantic.Audience,
	})
	if err != nil {
		return nil, err
	}
	cleaner.RegisterWithName("SemanticClient", client.Close)
	return client, nil
}
