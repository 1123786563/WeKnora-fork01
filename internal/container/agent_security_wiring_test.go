package container

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"context"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type agentSecurityWiringVersionService struct{}

func (agentSecurityWiringVersionService) FreezeAgentVersion(context.Context, uint64, string, string) (interfaces.AgentVersionView, error) {
	return interfaces.AgentVersionView{}, nil
}
func (agentSecurityWiringVersionService) GetAgentVersion(context.Context, uint64, string) (interfaces.AgentVersionSnapshot, error) {
	return interfaces.AgentVersionSnapshot{}, nil
}
func (agentSecurityWiringVersionService) ListAgentVersions(context.Context, uint64, string) ([]interfaces.AgentVersionView, error) {
	return nil, nil
}

func TestAgentSecurityWiringRegistered(t *testing.T) {
	containerSrc := readRepoFile(t, "container.go")
	require.Contains(t, containerSrc, "provideAgentSecurity(container)", "BuildContainer 必须调用 Agent Security provider helper")
	helperStart := strings.Index(containerSrc, "func provideAgentSecurity(")
	require.NotEqual(t, -1, helperStart, "必须定义 Agent Security provider helper")
	helperEnd := strings.Index(containerSrc[helperStart:], "\n}")
	require.NotEqual(t, -1, helperEnd, "Agent Security provider helper 必须闭合")
	helperSrc := containerSrc[helperStart : helperStart+helperEnd]
	for _, want := range []string{
		"container.Provide(NewAgentSecurityService)",
		"container.Provide(NewAgentSecurityHandler)",
		"container.Invoke(wireAgentSecurityGates)",
		"container.Invoke(wireAgentSecuritySessionClaims)",
	} {
		require.Truef(t, strings.Contains(helperSrc, want), "security helper 必须包含 %q", want)
	}
	require.NotContains(t, helperSrc, "repository.NewAgentRunStore", "AgentRunStore 已在核心注册，security helper 不得重复注册")
	routerSrc := readRepoFile(t, "../router/router.go")
	require.True(t, strings.Contains(routerSrc, "RegisterAgentSecurityRoutes(v1, params.AgentSecurityHandler, rbacGuards)"), "router.go 必须挂载安全撤回路由")
}

func TestAgentSecurityClaimDependenciesAreInjected(t *testing.T) {
	container := dig.New()
	require.NoError(t, container.Provide(func() *sessionhandler.Handler { return &sessionhandler.Handler{} }))
	require.NoError(t, container.Provide(func() *gorm.DB { return &gorm.DB{} }))
	require.NoError(t, container.Provide(repository.NewAgentChatTurnClaimRepository))
	require.NoError(t, container.Provide(func() interfaces.AgentVersionService { return agentSecurityWiringVersionService{} }))
	require.NoError(t, container.Provide(func() *service.AgentSecurityService { return service.NewAgentSecurityService(nil, nil) }))
	require.NoError(t, container.Invoke(wireAgentSecuritySessionClaims))
	var h *sessionhandler.Handler
	require.NoError(t, container.Invoke(func(handler *sessionhandler.Handler) { h = handler }))
	v := reflect.ValueOf(h).Elem()
	require.False(t, v.FieldByName("agentChatTurnClaimStore").IsNil())
	require.False(t, v.FieldByName("agentVersionService").IsNil())
}

func TestAgentSecurityProvidersResolveWithExistingRunStore(t *testing.T) {
	container := dig.New()
	require.NoError(t, container.Provide(func() *gorm.DB { return &gorm.DB{} }))
	require.NoError(t, container.Provide(repository.NewAgentRunStore))
	require.NoError(t, container.Provide(func() *service.AgentAdoptionService { return service.NewAgentAdoptionService(nil, nil, nil) }))
	require.NoError(t, container.Provide(func() *service.AgentUpgradeService { return service.NewAgentUpgradeService(nil) }))

	provideAgentSecurity(container)

	var resolved *handler.AgentSecurityHandler
	var adoption *service.AgentAdoptionService
	var upgrade *service.AgentUpgradeService
	require.NoError(t, container.Invoke(func(h *handler.AgentSecurityHandler, a *service.AgentAdoptionService, u *service.AgentUpgradeService) {
		resolved, adoption, upgrade = h, a, u
	}))
	require.NotNil(t, resolved)
	require.NotNil(t, adoption)
	require.NotNil(t, upgrade)
	require.False(t, reflect.ValueOf(adoption).Elem().FieldByName("releaseSecurityGate").IsNil())
	require.False(t, reflect.ValueOf(upgrade).Elem().FieldByName("releaseSecurityGate").IsNil())
}
