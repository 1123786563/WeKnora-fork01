package container

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentSecurityWiringRegistered(t *testing.T) {
	containerSrc := readRepoFile(t, "container.go")
	for _, want := range []string{
		"must(container.Provide(NewAgentSecurityService))",
		"must(container.Provide(NewAgentSecurityHandler))",
		"must(container.Invoke(wireAgentSecurityGates))",
	} {
		require.Truef(t, strings.Contains(containerSrc, want), "container.go 必须注册 %q——漏注册会让撤回路由/治理闸门静默消失", want)
	}
	routerSrc := readRepoFile(t, "../router/router.go")
	require.True(t, strings.Contains(routerSrc, "RegisterAgentSecurityRoutes(v1, params.AgentSecurityHandler, rbacGuards)"), "router.go 必须挂载安全撤回路由")
}
