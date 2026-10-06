package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type agentCapabilitiesMCPService struct {
	interfaces.MCPServiceService
	service  *types.MCPService
	metadata *types.MCPMetadata
}

func (s *agentCapabilitiesMCPService) ListMCPServices(context.Context, uint64) ([]*types.MCPService, error) {
	return []*types.MCPService{s.service}, nil
}

func (s *agentCapabilitiesMCPService) GetMCPServiceByID(
	_ context.Context, _ uint64, id string,
) (*types.MCPService, error) {
	if id != s.service.ID {
		return nil, types.ErrMCPServiceNotFound
	}
	return s.service, nil
}

func (s *agentCapabilitiesMCPService) GetMCPMetadata(
	context.Context, uint64, string,
) (*types.MCPMetadata, error) {
	return s.metadata, nil
}

func (*agentCapabilitiesMCPService) PersistMCPMetadata(
	context.Context, uint64, string, []*types.MCPTool, string,
) error {
	return nil
}

func (*agentCapabilitiesMCPService) RefreshMCPMetadata(
	context.Context, uint64, string,
) (*types.MCPMetadata, error) {
	return nil, nil
}

func TestAgentCapabilitiesUseIndependentRegistriesPerSession(t *testing.T) {
	now := time.Now()
	mcpService := &agentCapabilitiesMCPService{
		service: &types.MCPService{
			ID:        "calendar",
			TenantID:  7,
			Name:      "Calendar",
			Enabled:   true,
			UpdatedAt: now,
		},
		metadata: &types.MCPMetadata{
			ServiceID: "calendar",
			Tools: []*types.MCPTool{{
				Name:        "create_event",
				Description: "Create a calendar event",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			}},
		},
	}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	svc := &agentService{mcpServiceService: mcpService, mcpManager: manager}
	ctx := types.WithPrincipal(
		context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7)),
		types.Principal{Type: types.PrincipalWebUser, ID: "user-1"},
	)
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	chatModel := &fakeAgentChatModel{}

	first, err := svc.prepareAgentCapabilities(ctx, config, chatModel, nil, nil, "session-1", "message-1")
	require.NoError(t, err)
	second, err := svc.prepareAgentCapabilities(ctx, config, chatModel, nil, nil, "session-2", "message-2")
	require.NoError(t, err)
	require.NotSame(t, first.Tools, second.Tools)
	require.Equal(t, second.Tools.GetModelFunctionDefinitions(), first.Tools.GetModelFunctionDefinitions())

	result, err := first.Tools.ExecuteTool(
		ctx,
		tools.ToolDiscoverMCPTools,
		json.RawMessage(`{"mode":"describe","server_id":"calendar","tool_name":"create_event"}`),
	)
	require.NoError(t, err)
	require.True(t, result.Success, result.Error)
	first.Tools.RefreshMCPTools(ctx)

	require.Greater(t, len(first.Tools.GetModelFunctionDefinitions()), len(second.Tools.GetModelFunctionDefinitions()))
	require.Equal(t, []string{tools.ToolDiscoverMCPTools, tools.ToolThinking}, modelToolNames(second.Tools))
}

func TestAgentCapabilitiesSnapshotPreservesDeferredMCPAndSkills(t *testing.T) {
	ctx := types.WithPrincipal(
		context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7)),
		types.Principal{Type: types.PrincipalWebUser, ID: "user-1"},
	)
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	mcpService := &agentCapabilitiesMCPService{
		service:  &types.MCPService{ID: "calendar", TenantID: 7, Enabled: true},
		metadata: &types.MCPMetadata{ServiceID: "calendar"},
	}
	svc := &agentService{mcpServiceService: mcpService, mcpManager: manager}
	caps, err := svc.prepareAgentCapabilities(ctx, &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}, &fakeAgentChatModel{}, nil, nil, "session-1", "message-1")
	require.NoError(t, err)
	snapshot := caps.CapabilitySnapshot()
	require.Contains(t, snapshot.ToolIdentities, tools.ToolThinking)
	// The deferred set is derived from the same registry, so a capability
	// snapshot cannot accidentally advertise a tool that execution lacks.
	for _, name := range snapshot.DeferredNames {
		require.Contains(t, snapshot.ToolIdentities, name)
	}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotEmpty(t, raw)
}

func modelToolNames(registry *tools.ToolRegistry) []string {
	definitions := registry.GetModelFunctionDefinitions()
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}
