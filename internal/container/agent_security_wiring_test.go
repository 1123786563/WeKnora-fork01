package container

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
)

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
	} {
		require.Truef(t, strings.Contains(helperSrc, want), "security helper 必须包含 %q", want)
	}
	require.NotContains(t, helperSrc, "repository.NewAgentRunStore", "AgentRunStore 已在核心注册，security helper 不得重复注册")
	routerSrc := readRepoFile(t, "../router/router.go")
	require.True(t, strings.Contains(routerSrc, "RegisterAgentSecurityRoutes(v1, params.AgentSecurityHandler, rbacGuards)"), "router.go 必须挂载安全撤回路由")
}

func TestAgentSecurityProvidersResolveWithExistingRunStore(t *testing.T) {
	container := dig.New()
	require.NoError(t, container.Provide(func() *gorm.DB { return &gorm.DB{} }))
	require.NoError(t, container.Provide(repository.NewAgentRunStore))
	require.NoError(t, container.Provide(func() *service.AgentAdoptionService { return nil }))
	require.NoError(t, container.Provide(func() *service.AgentUpgradeService { return nil }))

	provideAgentSecurity(container)

	var resolved *handler.AgentSecurityHandler
	require.NoError(t, container.Invoke(func(h *handler.AgentSecurityHandler) { resolved = h }))
	require.NotNil(t, resolved)
}
